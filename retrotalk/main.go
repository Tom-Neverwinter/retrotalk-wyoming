// Wyoming TTS server for the in-process formant speech engine in ./tts.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Tom-Neverwinter/retrotalk-wyoming/tts"
)

const (
	maxDataLength = 1 << 20            // client-supplied JSON is tiny; refuse anything absurd
	maxHeaderLine = 64 << 10           // bufio buffer size = longest header line accepted
	maxConns      = 32                 // concurrent connections; more are closed at once
	maxPending    = 2000               // streamed text held while waiting for punctuation
	chunkBytes    = tts.SampleRate * 2 // send audio in chunks of at most 1 s
)

var format = map[string]int{"rate": tts.SampleRate, "width": 2, "channels": 1}

// Timeouts drop clients that stop sending or stop reading, so they can't hold a connection
// (and its buffers) forever. Variables so tests can shorten them.
var (
	firstTimeout = 10 * time.Second // Home Assistant asks right after connecting
	idleTimeout  = 2 * time.Minute  // between events of a stream (an LLM may pause)
	writeTimeout = 30 * time.Second
)

// voice is a named speaker: a built-in preset plus optional settings, rate and language.
type voice struct {
	Name     string `json:"name"`
	Base     string `json:"base"`
	Settings string `json:"settings"`
	Rate     int    `json:"rate"`
	Language string `json:"language"` // us uk gr sp la fr (or a BCP 47 tag); "" = us
	desc     string
	tts      tts.Voice
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// loadVoices reads the add-on options. Custom voices with a built-in's name replace it.
// A missing file (running outside Home Assistant) means defaults.
func loadVoices(path string) ([]voice, voice, error) {
	o := struct {
		Voice        string   `json:"voice"`
		Rate         int      `json:"rate"`
		Languages    []string `json:"languages"`
		CustomVoices []voice  `json:"custom_voices"`
	}{Voice: "paul", Languages: []string{"us"}}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		b, err = []byte("{}"), nil
	}
	if err != nil {
		return nil, voice{}, err
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return nil, voice{}, fmt.Errorf("%s: %w", path, err)
	}
	names := make([]string, 0, len(tts.Presets))
	for n := range tts.Presets {
		names = append(names, n)
	}
	sort.Strings(names)
	// Built-ins for each enabled language: paul, betty... for US English; paul_gr... otherwise.
	var vs []voice
	for _, code := range o.Languages {
		if tts.LanguageTag(code) == "" {
			return nil, voice{}, fmt.Errorf("unknown language %q", code)
		}
		for _, n := range names {
			name := n
			if code != "us" {
				name += "_" + code
			}
			vs = append(vs, voice{Name: name, Base: n, Language: code, desc: n})
		}
	}
	for _, c := range o.CustomVoices {
		c.desc = fmt.Sprintf("%s (%s %s)", c.Name, c.Base, c.Settings)
		if i := slices.IndexFunc(vs, func(v voice) bool { return v.Name == c.Name }); i >= 0 {
			vs[i] = c
		} else {
			vs = append(vs, c)
		}
	}
	for i := range vs {
		v := &vs[i]
		base, ok := tts.Presets[v.Base]
		if !ok {
			return nil, voice{}, fmt.Errorf("voice %q: unknown base voice %q", v.Name, v.Base)
		}
		if v.tts, err = base.With(v.Settings); err != nil {
			return nil, voice{}, fmt.Errorf("voice %q: %w", v.Name, err)
		}
		if rate := cmp(v.Rate, o.Rate); rate > 0 {
			v.tts.Rate = float64(rate)
		}
		if v.tts.Lang = tts.LanguageTag(cmpStr(v.Language, "us")); v.tts.Lang == "" {
			return nil, voice{}, fmt.Errorf("voice %q: unknown language %q", v.Name, v.Language)
		}
	}
	i := slices.IndexFunc(vs, func(v voice) bool { return v.Name == o.Voice })
	if i < 0 {
		return nil, voice{}, fmt.Errorf("default voice %q is not a built-in or custom voice", o.Voice)
	}
	return vs, vs[i], nil
}

func cmpStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// cmp returns the first non-zero value.
func cmp(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func info(vs []voice) map[string]any {
	attr := map[string]string{"name": "Retrotalk", "url": "https://github.com/Tom-Neverwinter/retrotalk-wyoming"}
	tv := make([]map[string]any, len(vs))
	for i, v := range vs {
		tv[i] = map[string]any{"name": v.Name, "description": v.desc, "attribution": attr,
			"installed": true, "version": "1", "languages": []string{v.tts.Lang}}
	}
	none := []any{}
	return map[string]any{
		"tts": []any{map[string]any{"name": "retrotalk", "description": "Retrotalk formant speech synthesizer", "attribution": attr,
			"installed": true, "version": "1", "voices": tv, "supports_synthesize_streaming": true}},
		"asr": none, "handle": none, "intent": none, "wake": none, "mic": none, "snd": none,
	}
}

// readEvent reads one Wyoming event: a JSON header line, optional data JSON, optional payload.
// It returns the type and the data blobs (inline and/or trailing); payloads are discarded.
func readEvent(r *bufio.Reader) (string, [][]byte, error) {
	line, err := r.ReadSlice('\n') // ErrBufferFull caps header size
	if err != nil {
		return "", nil, err
	}
	var h struct {
		Type          string          `json:"type"`
		Data          json.RawMessage `json:"data"`
		DataLength    int             `json:"data_length"`
		PayloadLength int64           `json:"payload_length"`
	}
	if err := json.Unmarshal(line, &h); err != nil {
		return "", nil, err
	}
	if h.DataLength < 0 || h.DataLength > maxDataLength || h.PayloadLength < 0 {
		return "", nil, fmt.Errorf("bad lengths in header: %s", line)
	}
	var data [][]byte
	if len(h.Data) > 0 {
		data = append(data, h.Data)
	}
	if h.DataLength > 0 {
		b := make([]byte, h.DataLength)
		if _, err := io.ReadFull(r, b); err != nil {
			return "", nil, err
		}
		data = append(data, b)
	}
	if _, err := io.CopyN(io.Discard, r, h.PayloadLength); err != nil {
		return "", nil, err
	}
	return h.Type, data, nil
}

// writeEvent frames an event the way the Python wyoming library does. Write errors
// stick in the bufio.Writer and surface at Flush.
func writeEvent(w *bufio.Writer, typ string, data any, payload []byte) {
	d, _ := json.Marshal(data)
	h := map[string]any{"type": typ, "version": "1.5.4", "data_length": len(d)}
	if len(payload) > 0 {
		h["payload_length"] = len(payload)
	}
	hb, _ := json.Marshal(h)
	w.Write(hb)
	w.WriteByte('\n')
	w.Write(d)
	w.Write(payload)
}

type conn struct {
	nc      net.Conn
	w       *bufio.Writer
	started bool // audio-start sent for the current message
}

func (c *conn) send(typ string, data any, payload []byte) error {
	writeEvent(c.w, typ, data, payload)
	c.nc.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.w.Flush()
}

// speak synthesizes text and streams it phrase by phrase, opening with audio-start once.
func (c *conn) speak(sess *tts.Session, text string) error {
	return sess.Speak(text, func(pcm []byte) error {
		if !c.started {
			if err := c.send("audio-start", format, nil); err != nil {
				return err
			}
			c.started = true
		}
		for len(pcm) > 0 {
			n := min(len(pcm), chunkBytes)
			if err := c.send("audio-chunk", format, pcm[:n]); err != nil {
				return err
			}
			pcm = pcm[n:]
		}
		return nil
	})
}

type server struct {
	voices  []voice
	def     voice
	info    map[string]any
	speaker *tts.Speaker // all voices for [:name x], plus user pronunciations
}

func newServer(vs []voice, def voice, lexicon map[string][]string) *server {
	sp := &tts.Speaker{Voices: map[string]tts.Voice{}, Lexicon: lexicon}
	for _, v := range vs {
		sp.Voices[v.Name] = v.tts
	}
	return &server{voices: vs, def: def, info: info(vs), speaker: sp}
}

// loadLexicon reads the pronunciations option: [{"word": "Nguyen", "say": "new win"}, ...].
// "say" is a respelling in ordinary words or phonemes in brackets ("[n w ih1 n]").
func loadLexicon(path string) (map[string][]string, error) {
	var o struct {
		Pronunciations []struct{ Word, Say string } `json:"pronunciations"`
	}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &o); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	lex := map[string][]string{}
	for _, p := range o.Pronunciations {
		phones, err := tts.Pronounce(p.Say)
		if err != nil {
			return nil, fmt.Errorf("pronunciation for %q: %w", p.Word, err)
		}
		lex[strings.ToLower(p.Word)] = phones
	}
	return lex, nil
}

type synthReq struct {
	Text  string `json:"text"`
	Voice *struct {
		Name string `json:"name"`
	} `json:"voice"`
}

func decode(data [][]byte) (synthReq, error) {
	var req synthReq
	for _, d := range data {
		if err := json.Unmarshal(d, &req); err != nil {
			return req, err
		}
	}
	return req, nil
}

func (s *server) voice(req synthReq) voice {
	if req.Voice != nil {
		if i := slices.IndexFunc(s.voices, func(v voice) bool { return v.Name == req.Voice.Name }); i >= 0 {
			return s.voices[i]
		}
	}
	return s.def
}

func (s *server) handle(nc net.Conn) {
	defer nc.Close()
	defer func() { // one bad message must not take the whole add-on down
		if r := recover(); r != nil {
			log.Printf("%s: recovered from panic: %v", nc.RemoteAddr(), r)
		}
	}()
	c := &conn{nc: nc, w: bufio.NewWriter(nc)}
	r := bufio.NewReaderSize(nc, maxHeaderLine)
	var stream *tts.Session // set between synthesize-start and synthesize-stop
	var pending string      // streamed text not yet ending a phrase
	wait := firstTimeout    // silent connections can't hold one of the few slots for long
	for {
		nc.SetReadDeadline(time.Now().Add(wait))
		wait = idleTimeout
		typ, data, err := readEvent(r)
		if err == nil {
			var req synthReq
			switch typ {
			case "describe":
				err = c.send("info", s.info, nil)
			case "synthesize":
				if stream != nil {
					break // inside a stream this is only a compatibility copy of the chunks
				}
				if req, err = decode(data); err == nil {
					c.started = false
					if err = c.speak(s.speaker.Session(s.voice(req).tts), req.Text); err == nil {
						err = c.send("audio-stop", map[string]any{}, nil)
					}
				}
			case "synthesize-start":
				if req, err = decode(data); err == nil {
					stream, pending, c.started = s.speaker.Session(s.voice(req).tts), "", false
				}
			case "synthesize-chunk":
				if req, err = decode(data); err == nil && stream != nil {
					// Speak each completed phrase as soon as it arrives.
					var done string
					done, pending = tts.SplitPhrases(pending + req.Text)
					if len(pending) > maxPending { // no punctuation in sight: speak up to a space
						cut := strings.LastIndexByte(pending[:maxPending], ' ') + 1
						if cut == 0 {
							cut = maxPending
						}
						done, pending = done+pending[:cut], pending[cut:]
					}
					err = c.speak(stream, done)
				}
			case "synthesize-stop":
				if stream != nil {
					if err = c.speak(stream, pending); err == nil {
						err = c.send("audio-stop", map[string]any{}, nil)
					}
					stream, pending = nil, ""
				}
				if err == nil {
					err = c.send("synthesize-stopped", map[string]any{}, nil)
				}
			}
		}
		if errors.Is(err, tts.ErrTooLong) {
			// Over-long message: what was spoken so far is delivered; end it normally.
			log.Printf("%s: %v", nc.RemoteAddr(), err)
			if typ == "synthesize" {
				err = c.send("audio-stop", map[string]any{}, nil)
			} else {
				err = nil // keep the stream open; further text is ignored by the spent session
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("%s: %v", nc.RemoteAddr(), err)
			}
			return
		}
	}
}

// discover tells the Supervisor where we listen so Home Assistant offers the Wyoming integration.
func discover(port string) {
	token := os.Getenv("SUPERVISOR_TOKEN")
	if token == "" {
		return // not running as an add-on
	}
	host, _ := os.Hostname()
	body, _ := json.Marshal(map[string]any{"service": "wyoming",
		"config": map[string]string{"uri": "tcp://" + host + ":" + port}})
	req, _ := http.NewRequest(http.MethodPost, "http://supervisor/discovery", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		log.Printf("discovery: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("discovery: supervisor returned %s", resp.Status)
		return
	}
	log.Printf("discovery: announced tcp://%s:%s", host, port)
}

func main() {
	opts := envOr("OPTIONS_PATH", "/data/options.json")
	vs, def, err := loadVoices(opts)
	if err != nil {
		log.Fatal(err)
	}
	lexicon, err := loadLexicon(opts)
	if err != nil {
		log.Fatal(err)
	}
	tts.LoadDictionary()
	s := newServer(vs, def, lexicon)
	port := envOr("PORT", "10205")
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Wyoming TTS server on %s, %d voices, default %q, %d pronunciations", ln.Addr(), len(vs), def.Name, len(lexicon))
	go discover(port)
	slots := make(chan struct{}, maxConns)
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		select {
		case slots <- struct{}{}:
			go func() {
				defer func() { <-slots }()
				s.handle(c)
			}()
		default:
			log.Printf("%s: too many connections, closing", c.RemoteAddr())
			c.Close()
		}
	}
}
