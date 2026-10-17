package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Tom-Neverwinter/retrotalk-wyoming/tts"
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
	s := newServer(vs, def, nil)
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
	s := newServer(vs, def, nil)
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

func TestLanguages(t *testing.T) {
	vs, _, err := loadVoices(writeOptions(t, `{"languages":["us","gr"],
		"custom_voices":[{"name":"heidi","base":"betty","language":"gr"}]}`))
	if err != nil || len(vs) != 19 {
		t.Fatalf("got %d voices, %v", len(vs), err)
	}
	tags := map[string]string{}
	for _, v := range info(vs)["tts"].([]any)[0].(map[string]any)["voices"].([]map[string]any) {
		tags[v["name"].(string)] = v["languages"].([]string)[0]
	}
	if tags["paul"] != "en-US" || tags["paul_gr"] != "de-DE" || tags["heidi"] != "de-DE" {
		t.Fatal(tags)
	}
	for _, bad := range []string{`{"languages":["xx"]}`, `{"custom_voices":[{"name":"x","base":"paul","language":"xx"}]}`} {
		if _, _, err := loadVoices(writeOptions(t, bad)); err == nil {
			t.Fatalf("expected error for %s", bad)
		}
	}
}

func TestLoadLexicon(t *testing.T) {
	lex, err := loadLexicon(writeOptions(t, `{"pronunciations":[
		{"word":"Nguyen","say":"new win"},{"word":"Hisense","say":"[hh ay1 s eh2 n s]"}]}`))
	if err != nil || strings.Join(lex["nguyen"], " ") != "N UW1 W IH1 N" || len(lex["hisense"]) != 6 {
		t.Fatalf("%v %v", lex, err)
	}
	if _, err := loadLexicon(writeOptions(t, `{"pronunciations":[{"word":"x","say":""}]}`)); err == nil {
		t.Fatal("expected error for an empty pronunciation")
	}
}

// A [:rate] command in one streamed chunk still applies to the next chunk.
func TestStreamingKeepsCommandState(t *testing.T) {
	stream := func(first string) int {
		_, audio := exchange(t, []string{
			`{"type":"synthesize-start","data":{"voice":{"name":"paul"}}}`,
			`{"type":"synthesize-chunk","data":{"text":"` + first + `Hello there. "}}`,
			`{"type":"synthesize-chunk","data":{"text":"This sentence comes in a later chunk."}}`,
			`{"type":"synthesize-stop"}`,
		}, "synthesize-stopped")
		return audio
	}
	if normal, fast := stream(""), stream("[:rate 400] "); float64(fast) > 0.7*float64(normal) {
		t.Fatalf("rate did not carry over: %d vs %d bytes", fast, normal)
	}
}

// FuzzReadEvent: arbitrary bytes off the wire never panic the protocol reader.
//
//	go test -fuzz FuzzReadEvent -fuzztime 60s .
func FuzzReadEvent(f *testing.F) {
	for _, seed := range []string{
		`{"type":"describe"}` + "\n",
		`{"type":"synthesize","data_length":2}` + "\n{}",
		`{"type":"x","data_length":-1,"payload_length":-9}` + "\n",
		`{"type":"x","payload_length":99999999999}` + "\n",
		`{"type":"synthesize","data":{"text":7,"voice":"paul"}}` + "\n",
		"\n\n\n", `{"type":`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		r := bufio.NewReaderSize(bytes.NewReader(b), maxHeaderLine)
		for i := 0; i < 100; i++ {
			typ, data, err := readEvent(r)
			if err != nil {
				return
			}
			if typ == "synthesize" || typ == "synthesize-chunk" {
				decode(data) // must not panic on any JSON shape
			}
		}
	})
}

// A client that connects and says nothing is dropped, freeing its connection slot.
func TestSilentClientDropped(t *testing.T) {
	old := firstTimeout
	firstTimeout = 100 * time.Millisecond
	defer func() { firstTimeout = old }()
	vs, def, _ := loadVoices("")
	client, server := net.Pipe()
	done := make(chan struct{})
	go func() { newServer(vs, def, nil).handle(server); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("silent connection was not dropped")
	}
	client.Close()
}

// Streamed text with no punctuation is still spoken as it arrives, not buffered forever.
func TestUnpunctuatedStreamIsSpoken(t *testing.T) {
	vs, def, _ := loadVoices("")
	client, server := net.Pipe()
	go newServer(vs, def, nil).handle(server)
	defer client.Close()
	chunk := `{"type":"synthesize-chunk","data":{"text":"` + strings.Repeat("door ", 1000) + `"}}` + "\n"
	go client.Write([]byte(`{"type":"synthesize-start"}` + "\n" + chunk))
	if ev := readReply(t, bufio.NewReader(client)); ev.Type != "audio-start" {
		t.Fatalf("got %q before the stream ended, want audio-start", ev.Type)
	}
}

// A message longer than the audio budget ends with audio-stop instead of a dropped connection.
func TestOverlongMessageEndsCleanly(t *testing.T) {
	// Far more than 10 minutes of text, sent as Home Assistant does: a separate data block
	// (headers are capped at 64 kB). exchange appends a newline, counted in data_length.
	data := `{"text":"` + strings.Repeat("The front door is open. ", 6000) + `"}`
	header := fmt.Sprintf(`{"type":"synthesize","data_length":%d}`, len(data)+1)
	types, audio := exchange(t, []string{header + "\n" + data}, "audio-stop")
	if !slices.Equal(types, []string{"audio-start", "audio-chunk", "audio-stop"}) || audio > 10*60*tts.SampleRate*2 {
		t.Fatalf("%v, %d bytes", types, audio)
	}
}
