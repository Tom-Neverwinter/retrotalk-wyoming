// Wyoming TTS server that shells out to DECtalk's `say`.
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
)

const maxDataLength = 1 << 20 // client-supplied JSON is tiny; refuse anything absurd

var (
	dist   = envOr("DECTALK_DIR", "/dectalk")
	voices = []string{"paul", "betty", "harry", "frank", "dennis", "kit", "ursula", "rita", "wendy"}
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func info() map[string]any {
	attr := map[string]string{"name": "DECtalk", "url": "https://github.com/dectalk/dectalk"}
	vs := make([]map[string]any, len(voices))
	for i, v := range voices {
		vs[i] = map[string]any{"name": v, "description": v, "attribution": attr,
			"installed": true, "version": "4.61", "languages": []string{"en-US"}}
	}
	none := []any{}
	return map[string]any{
		"tts": []any{map[string]any{"name": "dectalk", "description": "DECtalk", "attribution": attr,
			"installed": true, "version": "4.61", "voices": vs}},
		"asr": none, "handle": none, "intent": none, "wake": none, "mic": none, "snd": none,
	}
}

// readEvent reads one Wyoming event: a JSON header line, optional data JSON, optional payload.
// It returns the type and the data blobs (inline and/or trailing); payloads are discarded.
func readEvent(r *bufio.Reader) (string, [][]byte, error) {
	line, err := r.ReadBytes('\n')
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

func synth(text, voice string) (rate, width, channels int, pcm []byte, err error) {
	dir, err := os.MkdirTemp("", "dectalk")
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "out.wav")
	cmd := exec.Command("./say", "-fo", out, "-a", "[:name "+voice+"] "+text)
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

func speak(w *bufio.Writer, data [][]byte) error {
	var s struct {
		Text  string `json:"text"`
		Voice *struct {
			Name string `json:"name"`
		} `json:"voice"`
	}
	for _, d := range data {
		if err := json.Unmarshal(d, &s); err != nil {
			return err
		}
	}
	voice := "paul"
	if s.Voice != nil && slices.Contains(voices, s.Voice.Name) {
		voice = s.Voice.Name
	}
	rate, width, ch, pcm, err := synth(s.Text, voice)
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

func handle(c net.Conn) {
	defer c.Close()
	r, w := bufio.NewReader(c), bufio.NewWriter(c)
	for {
		typ, data, err := readEvent(r)
		if err == nil {
			switch typ {
			case "describe":
				writeEvent(w, "info", info(), nil)
			case "synthesize":
				err = speak(w, data)
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

func main() {
	ln, err := net.Listen("tcp", ":"+envOr("PORT", "10205"))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("DECtalk Wyoming server listening on %s", ln.Addr())
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go handle(c)
	}
}
