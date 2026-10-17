// Wyoming TTS server that streams audio from DECtalk's `say`.
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
	"os/exec"
	"slices"
	"sync"
	"time"
)

const (
	maxDataLength = 1 << 20  // client-supplied JSON is tiny; refuse anything absurd
	maxHeaderLine = 64 << 10 // bufio buffer size = longest header line accepted
	maxSession    = 5 * time.Minute

	// say's default encoding (-e 1): 16-bit mono 11025 Hz, host byte order (little-endian on amd64/aarch64).
	sampleRate, sampleWidth, sampleChannels = 11025, 2, 1
)

var (
	dist     = envOr("DECTALK_DIR", "/dectalk")
	builtins = []string{"paul", "betty", "harry", "frank", "dennis", "kit", "ursula", "rita", "wendy"}
	format   = map[string]int{"rate": sampleRate, "width": sampleWidth, "channels": sampleChannels}
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
			"installed": true, "version": "4.61", "voices": tv, "supports_synthesize_streaming": true}},
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

// conn serializes writes: during a stream the audio pump writes while the handler reads.
type conn struct {
	mu sync.Mutex
	w  *bufio.Writer
}

func (c *conn) send(typ string, data any, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	writeEvent(c.w, typ, data, payload)
	return c.w.Flush()
}

func (c *conn) fail(err error) error {
	c.send("error", map[string]string{"text": err.Error(), "code": "synthesis-failed"}, nil)
	return err
}

// session is one running `say`: text goes in on stdin, raw PCM streams out of stdout.
// ponytail: one process per message; link libtts via cgo if spawn cost ever matters.
type session struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	done  chan error
}

func (c *conn) start(v voice) (*session, error) {
	// -c: clause mode, DECtalk speaks each clause as soon as it's complete. "-" must be last.
	cmd := exec.Command("./say", "-c", "-fo", "stdout:raw", "-")
	cmd.Dir = dist
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("say: %w", err)
	}
	s := &session{cmd: cmd, stdin: stdin, done: make(chan error, 1)}
	watchdog := time.AfterFunc(maxSession, func() { cmd.Process.Kill() }) // a hung say must not pin the connection
	go func() {
		err := c.pump(stdout)
		if err != nil {
			cmd.Process.Kill() // client is gone; don't let say block on a full pipe
		}
		werr := cmd.Wait()
		watchdog.Stop()
		if werr != nil && err == nil {
			err = fmt.Errorf("say: %w: %s", werr, bytes.TrimSpace(stderr.Bytes()))
		}
		s.done <- err
	}()
	if _, err := io.WriteString(stdin, v.prefix()); err != nil {
		s.kill()
		return nil, err
	}
	return s, nil
}

// pump relays PCM as say produces it, keeping chunks sample-aligned.
func (c *conn) pump(stdout io.Reader) error {
	buf := make([]byte, 8192)
	started, carry := false, 0
	for {
		n, rerr := stdout.Read(buf[carry:])
		n += carry
		if even := n &^ (sampleWidth - 1); even > 0 {
			if !started {
				if err := c.send("audio-start", format, nil); err != nil {
					return err
				}
				started = true
			}
			if err := c.send("audio-chunk", format, buf[:even]); err != nil {
				return err
			}
			carry = copy(buf, buf[even:n])
		} else {
			carry = n
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

// finish closes the text input, waits for the audio to drain and ends it.
func (c *conn) finish(s *session) error {
	s.stdin.Close()
	if err := <-s.done; err != nil {
		return c.fail(err)
	}
	return c.send("audio-stop", map[string]any{}, nil)
}

func (s *session) kill() {
	s.cmd.Process.Kill()
	s.stdin.Close()
	<-s.done
}

type server struct {
	voices []voice
	def    voice
	info   map[string]any
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
	c := &conn{w: bufio.NewWriter(nc)}
	r := bufio.NewReaderSize(nc, maxHeaderLine)
	var stream *session // set between synthesize-start and synthesize-stop
	defer func() {
		nc.Close()
		if stream != nil {
			stream.kill()
		}
	}()
	for {
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
					var one *session
					if one, err = c.start(s.voice(req)); err != nil {
						err = c.fail(err)
					} else if _, err = io.WriteString(one.stdin, req.Text); err != nil {
						one.kill()
					} else {
						err = c.finish(one)
					}
				}
			case "synthesize-start":
				if stream != nil {
					stream.kill()
				}
				if req, err = decode(data); err == nil {
					if stream, err = c.start(s.voice(req)); err != nil {
						err = c.fail(err)
					}
				}
			case "synthesize-chunk":
				if req, err = decode(data); err == nil && stream != nil {
					_, err = io.WriteString(stream.stdin, req.Text)
				}
			case "synthesize-stop":
				if stream != nil {
					err = c.finish(stream)
					stream = nil
				}
				if err == nil {
					err = c.send("synthesize-stopped", map[string]any{}, nil)
				}
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
