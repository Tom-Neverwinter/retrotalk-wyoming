// Wyoming TTS server that shells out to DECtalk's `say`.
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"
)

const (
	maxDataLength = 1 << 20  // client-supplied JSON is tiny; refuse anything absurd
	maxHeaderLine = 64 << 10 // bufio buffer size = longest header line accepted
)

var (
	dist     = envOr("DECTALK_DIR", "/dectalk")
	builtins = []string{"paul", "betty", "harry", "frank", "dennis", "kit", "ursula", "rita", "wendy"}
)

// voice is a DECtalk speaker: a built-in base voice plus optional [:dv] tweaks and rate.
type voice struct {
	Name     string `json:"name"`
	Base     string `json:"base"`
	Settings string `json:"settings"`
	Rate     int    `json:"rate"`
}

// prefix is the DECtalk command string that selects this voice.
func (v voice) prefix() string {
	p := "[:name " + v.Base + "]"
	if v.Settings != "" {
		p += "[:dv " + v.Settings + "]"
	}
	if v.Rate > 0 {
		p += fmt.Sprintf("[:rate %d]", v.Rate)
	}
	return p + " "
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
		Voice        string  `json:"voice"`
		Rate         int     `json:"rate"`
		CustomVoices []voice `json:"custom_voices"`
	}{Voice: "paul"}
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
	vs := make([]voice, 0, len(builtins)+len(o.CustomVoices))
	for _, n := range builtins {
		vs = append(vs, voice{Name: n, Base: n, Rate: o.Rate})
	}
	for _, c := range o.CustomVoices {
		if c.Rate == 0 {
			c.Rate = o.Rate
		}
		if i := slices.IndexFunc(vs, func(v voice) bool { return v.Name == c.Name }); i >= 0 {
			vs[i] = c
		} else {
			vs = append(vs, c)
		}
	}
	i := slices.IndexFunc(vs, func(v voice) bool { return v.Name == o.Voice })
	if i < 0 {
		return nil, voice{}, fmt.Errorf("default voice %q is not a built-in or custom voice", o.Voice)
	}
	return vs, vs[i], nil
}

func info(vs []voice) map[string]any {
	attr := map[string]string{"name": "DECtalk", "url": "https://github.com/dectalk/dectalk"}
	tv := make([]map[string]any, len(vs))
	for i, v := range vs {
		desc := v.Name
		if v.Name != v.Base || v.Settings != "" {
			desc = fmt.Sprintf("%s (%s %s)", v.Name, v.Base, v.Settings)
		}
		tv[i] = map[string]any{"name": v.Name, "description": desc, "attribution": attr,
			"installed": true, "version": "4.61", "languages": []string{"en-US"}}
	}
	none := []any{}
	return map[string]any{
		"tts": []any{map[string]any{"name": "dectalk", "description": "DECtalk", "attribution": attr,
			"installed": true, "version": "4.61", "voices": tv}},
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

// ponytail: one `say` process + temp WAV per request; link libtts via cgo if spawn cost matters.
func synth(text string, v voice) (rate, width, channels int, pcm []byte, err error) {
	dir, err := os.MkdirTemp("", "dectalk") // /tmp is tmpfs in the add-on, so no SD card writes
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "out.wav")
	cmd := exec.Command("./say", "-fo", out, "-a", v.prefix()+text)
	cmd.Dir = dist
	if b, e := cmd.CombinedOutput(); e != nil {
		err = fmt.Errorf("say: %w: %s", e, b)
		return
	}
	wav, err := os.ReadFile(out)
	if err != nil {
		return
	}
	return parseWAV(wav)
}

func parseWAV(b []byte) (rate, width, channels int, pcm []byte, err error) {
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return 0, 0, 0, nil, errors.New("not a WAV file")
	}
	for i := 12; i+8 <= len(b); {
		size := int(binary.LittleEndian.Uint32(b[i+4:]))
		body := b[i+8 : min(i+8+size, len(b))]
		switch string(b[i : i+4]) {
		case "fmt ":
			if len(body) < 16 {
				return 0, 0, 0, nil, errors.New("short fmt chunk")
			}
			channels = int(binary.LittleEndian.Uint16(body[2:]))
			rate = int(binary.LittleEndian.Uint32(body[4:]))
			width = int(binary.LittleEndian.Uint16(body[14:])) / 8
		case "data":
			pcm = body
		}
		i += 8 + size + size&1
	}
	if rate == 0 || width == 0 || channels == 0 || pcm == nil {
		return 0, 0, 0, nil, errors.New("WAV missing fmt or data chunk")
	}
	return
}

type server struct {
	voices []voice
	def    voice
	info   map[string]any
}

func (s *server) speak(w *bufio.Writer, data [][]byte) error {
	var req struct {
		Text  string `json:"text"`
		Voice *struct {
			Name string `json:"name"`
		} `json:"voice"`
	}
	for _, d := range data {
		if err := json.Unmarshal(d, &req); err != nil {
			return err
		}
	}
	v := s.def
	if req.Voice != nil {
		if i := slices.IndexFunc(s.voices, func(x voice) bool { return x.Name == req.Voice.Name }); i >= 0 {
			v = s.voices[i]
		}
	}
	rate, width, ch, pcm, err := synth(req.Text, v)
	if err != nil {
		writeEvent(w, "error", map[string]string{"text": err.Error(), "code": "synthesis-failed"}, nil)
		w.Flush()
		return err
	}
	f := map[string]int{"rate": rate, "width": width, "channels": ch}
	writeEvent(w, "audio-start", f, nil)
	step := rate * width * ch // 1s chunks
	for i := 0; i < len(pcm); i += step {
		writeEvent(w, "audio-chunk", f, pcm[i:min(i+step, len(pcm))])
	}
	writeEvent(w, "audio-stop", map[string]any{}, nil)
	return nil
}

func (s *server) handle(c net.Conn) {
	defer c.Close()
	r, w := bufio.NewReaderSize(c, maxHeaderLine), bufio.NewWriter(c)
	for {
		typ, data, err := readEvent(r)
		if err == nil {
			switch typ {
			case "describe":
				writeEvent(w, "info", s.info, nil)
			case "synthesize":
				err = s.speak(w, data)
			}
		}
		if err == nil {
			err = w.Flush()
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("%s: %v", c.RemoteAddr(), err)
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
	vs, def, err := loadVoices(envOr("OPTIONS_PATH", "/data/options.json"))
	if err != nil {
		log.Fatal(err)
	}
	s := &server{voices: vs, def: def, info: info(vs)}
	port := envOr("PORT", "10205")
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("DECtalk Wyoming server on %s, %d voices, default %q", ln.Addr(), len(vs), def.Name)
	go discover(port)
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go s.handle(c)
	}
}
