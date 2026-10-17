package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeOptions(t *testing.T, opts string) string {
	path := filepath.Join(t.TempDir(), "options.json")
	os.WriteFile(path, []byte(opts), 0o644)
	return path
}

func TestLoadVoices(t *testing.T) {
	vs, def, err := loadVoices(writeOptions(t, `{"voice":"robot","rate":220,"custom_voices":[
		{"name":"robot","base":"paul","settings":"pr 0 ap 90"},
		{"name":"betty","base":"betty","settings":"hs 90","rate":150}]}`))
	if err != nil || len(vs) != 10 {
		t.Fatalf("got %d voices, %v", len(vs), err)
	}
	if def.Name != "robot" || def.tts.PitchRange != 0 || def.tts.Pitch != 90 || def.tts.Rate != 220 {
		t.Fatalf("robot = %+v", def.tts)
	}
	betty := vs[slices.IndexFunc(vs, func(v voice) bool { return v.Name == "betty" })]
	if betty.tts.HeadSize != 90 || betty.tts.Rate != 150 {
		t.Fatalf("overridden betty = %+v", betty.tts)
	}
	if _, def, _ := loadVoices(filepath.Join(t.TempDir(), "missing.json")); def.Name != "paul" {
		t.Fatalf("default without options = %q", def.Name)
	}
	for _, bad := range []string{`{"voice":"nobody"}`,
		`{"custom_voices":[{"name":"x","base":"nobody"}]}`,
		`{"custom_voices":[{"name":"x","base":"paul","settings":"zz 1"}]}`} {
		if _, _, err := loadVoices(writeOptions(t, bad)); err == nil {
			t.Fatalf("expected error for %s", bad)
		}
	}
}

type event struct {
	Type          string `json:"type"`
	DataLength    int    `json:"data_length"`
	PayloadLength int    `json:"payload_length"`
}

func readReply(t *testing.T, r *bufio.Reader) event {
	t.Helper()
	h, err := r.ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var ev event
	json.Unmarshal(h, &ev)
	io.CopyN(io.Discard, r, int64(ev.DataLength))
	io.CopyN(io.Discard, r, int64(ev.PayloadLength))
	if ev.PayloadLength%2 != 0 {
		t.Fatalf("odd audio chunk %d", ev.PayloadLength)
	}
	return ev
}

// exchange sends events like Home Assistant and collects reply types (runs of audio-chunk
// collapsed) and audio bytes until stop.
func exchange(t *testing.T, send []string, stop string) (types []string, audio int) {
	t.Helper()
	vs, def, _ := loadVoices("")
	s := &server{voices: vs, def: def, info: info(vs)}
	client, server := net.Pipe()
	go s.handle(server)
	defer client.Close()
	go func() {
		for _, e := range send {
			client.Write([]byte(e + "\n"))
		}
	}()
	r := bufio.NewReader(client)
	for {
		ev := readReply(t, r)
		if len(types) == 0 || types[len(types)-1] != ev.Type {
			types = append(types, ev.Type)
		}
		audio += ev.PayloadLength
		if ev.Type == stop || ev.Type == "error" {
			return types, audio
		}
	}
}

func TestDescribe(t *testing.T) {
	// Python wyoming sends data as a separate blob after the header ("{}" + exchange's newline).
	types, _ := exchange(t, []string{`{"type":"describe","data_length":3}` + "\n{}"}, "info")
	if !slices.Equal(types, []string{"info"}) {
		t.Fatal(types)
	}
}

func TestSynthesize(t *testing.T) {
	types, audio := exchange(t, []string{`{"type":"synthesize","data":{"text":"The front door is open.","voice":{"name":"kit"}}}`}, "audio-stop")
	if !slices.Equal(types, []string{"audio-start", "audio-chunk", "audio-stop"}) || audio < 2*16000 {
		t.Fatalf("%v, %d bytes", types, audio)
	}
}

func TestStreaming(t *testing.T) {
	// Home Assistant's order: start, chunks, a compatibility synthesize, stop.
	types, audio := exchange(t, []string{
		`{"type":"synthesize-start","data":{"voice":{"name":"betty"}}}`,
		`{"type":"synthesize-chunk","data":{"text":"Hello there. "}}`,
		`{"type":"synthesize-chunk","data":{"text":"General Kenobi."}}`,
		`{"type":"synthesize","data":{"text":"Hello there. General Kenobi."}}`,
		`{"type":"synthesize-stop"}`,
	}, "synthesize-stopped")
	if !slices.Equal(types, []string{"audio-start", "audio-chunk", "audio-stop", "synthesize-stopped"}) || audio < 2*16000 {
		t.Fatalf("%v, %d bytes", types, audio)
	}
}

// A completed phrase is spoken before the stream ends.
func TestStreamingSpeaksEarly(t *testing.T) {
	vs, def, _ := loadVoices("")
	s := &server{voices: vs, def: def, info: info(vs)}
	client, server := net.Pipe()
	go s.handle(server)
	defer client.Close()
	go client.Write([]byte(`{"type":"synthesize-start"}` + "\n" +
		`{"type":"synthesize-chunk","data":{"text":"The door is open. The win"}}` + "\n"))
	r := bufio.NewReader(client)
	if ev := readReply(t, r); ev.Type != "audio-start" {
		t.Fatalf("first reply %q", ev.Type)
	}
	if ev := readReply(t, r); ev.Type != "audio-chunk" {
		t.Fatalf("second reply %q", ev.Type)
	}
}
