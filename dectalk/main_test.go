package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// With FAKE_SAY set, the test binary stands in for DECtalk's say: it logs
// "<args>|<stdin>" to $FAKE_SAY and writes one 16-bit sample per input byte to stdout.
func TestMain(m *testing.M) {
	if log := os.Getenv("FAKE_SAY"); log != "" {
		in, _ := io.ReadAll(os.Stdin)
		os.WriteFile(log, []byte(strings.Join(os.Args[1:], " ")+"|"+string(in)), 0o644)
		os.Stdout.Write(make([]byte, 2*len(in)))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeSay(t *testing.T) (log string) {
	dist = t.TempDir()
	exe, _ := os.Executable()
	b, _ := os.ReadFile(exe)
	name := "say"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	os.WriteFile(filepath.Join(dist, name), b, 0o755)
	log = filepath.Join(dist, "say.log")
	t.Setenv("FAKE_SAY", log)
	return log
}

func writeOptions(t *testing.T, opts string) string {
	path := filepath.Join(t.TempDir(), "options.json")
	os.WriteFile(path, []byte(opts), 0o644)
	return path
}

func TestLoadVoices(t *testing.T) {
	vs, def, err := loadVoices(writeOptions(t, `{"voice":"robot","rate":220,"custom_voices":[
		{"name":"robot","base":"paul","settings":"pr 0"},
		{"name":"betty","base":"betty","settings":"hs 110","rate":150}]}`))
	if err != nil || len(vs) != len(builtins)+1 {
		t.Fatalf("got %d voices, %v", len(vs), err)
	}
	if got := def.prefix(); got != "[:name paul][:dv pr 0][:rate 220] " {
		t.Fatalf("robot prefix %q", got)
	}
	if got := vs[1].prefix(); got != "[:name betty][:dv hs 110][:rate 150] " {
		t.Fatalf("overridden betty prefix %q", got)
	}
	if _, def, _ := loadVoices(filepath.Join(t.TempDir(), "missing.json")); def.prefix() != "[:name paul] " {
		t.Fatalf("defaults prefix %q", def.prefix())
	}
	for _, bad := range []string{`{"voice":"nobody"}`, `{"languages":["xx"]}`,
		`{"custom_voices":[{"name":"x","base":"paul","language":"xx"}]}`} {
		if _, _, err := loadVoices(writeOptions(t, bad)); err == nil {
			t.Fatalf("expected error for %s", bad)
		}
	}
}

func TestLanguages(t *testing.T) {
	vs, _, err := loadVoices(writeOptions(t, `{"languages":["us","gr"],
		"custom_voices":[{"name":"heidi","base":"betty","language":"gr"}]}`))
	if err != nil || len(vs) != 2*len(builtins)+1 {
		t.Fatalf("got %d voices, %v", len(vs), err)
	}
	tags := map[string]string{}
	for _, v := range info(vs)["tts"].([]any)[0].(map[string]any)["voices"].([]map[string]any) {
		tags[v["name"].(string)] = v["languages"].([]string)[0]
	}
	if tags["paul"] != "en-US" || tags["paul_gr"] != "de-DE" || tags["heidi"] != "de-DE" {
		t.Fatal(tags)
	}
}

// exchange runs a server with the given options file ("" = defaults), sends events
// like Home Assistant and collects reply types until stop.
func exchange(t *testing.T, opts string, send []string, stop string) (types []string, audio int) {
	t.Helper()
	vs, def, err := loadVoices(opts)
	if err != nil {
		t.Fatal(err)
	}
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
		h, err := r.ReadBytes('\n')
		if err != nil {
			t.Fatalf("read after %v: %v", types, err)
		}
		var ev struct {
			Type          string `json:"type"`
			DataLength    int    `json:"data_length"`
			PayloadLength int    `json:"payload_length"`
		}
		json.Unmarshal(h, &ev)
		io.CopyN(io.Discard, r, int64(ev.DataLength))
		io.CopyN(io.Discard, r, int64(ev.PayloadLength))
		if ev.PayloadLength%2 != 0 {
			t.Fatalf("odd audio chunk %d", ev.PayloadLength)
		}
		if len(types) == 0 || types[len(types)-1] != ev.Type {
			types = append(types, ev.Type) // collapse runs of audio-chunk
		}
		audio += ev.PayloadLength
		if ev.Type == stop || ev.Type == "error" {
			return types, audio
		}
	}
}

// checkSay compares what the fake say received and the audio that came back.
func checkSay(t *testing.T, log, wantArgs, wantText string, types []string, audio int, wantTypes ...string) {
	t.Helper()
	if got, _ := os.ReadFile(log); string(got) != wantArgs+"|"+wantText {
		t.Fatalf("say got %q", got)
	}
	if !slices.Equal(types, wantTypes) || audio != 2*len(wantText) {
		t.Fatalf("%v, %d bytes", types, audio)
	}
}

func TestDescribe(t *testing.T) {
	// Python wyoming sends data as a separate blob after the header ("{}" + exchange's newline).
	types, _ := exchange(t, "", []string{`{"type":"describe","data_length":3}` + "\n{}"}, "info")
	if !slices.Equal(types, []string{"info"}) {
		t.Fatal(types)
	}
}

func TestSynthesize(t *testing.T) {
	log := fakeSay(t)
	types, audio := exchange(t, "", []string{`{"type":"synthesize","data":{"text":"hello","voice":{"name":"kit"}}}`}, "audio-stop")
	checkSay(t, log, "-c -fo stdout:raw -", "[:name kit] hello", types, audio,
		"audio-start", "audio-chunk", "audio-stop")
}

func TestSynthesizeLanguage(t *testing.T) {
	log := fakeSay(t)
	opts := writeOptions(t, `{"languages":["us","gr"]}`)
	types, audio := exchange(t, opts, []string{`{"type":"synthesize","data":{"text":"Hallo","voice":{"name":"paul_gr"}}}`}, "audio-stop")
	checkSay(t, log, "-c -fo stdout:raw -l gr -", "[:name paul] Hallo", types, audio,
		"audio-start", "audio-chunk", "audio-stop")
}

func TestStreaming(t *testing.T) {
	log := fakeSay(t)
	// Home Assistant's order: start, chunks, a compatibility synthesize, stop.
	types, audio := exchange(t, "", []string{
		`{"type":"synthesize-start","data":{"voice":{"name":"betty"}}}`,
		`{"type":"synthesize-chunk","data":{"text":"Hello there. "}}`,
		`{"type":"synthesize-chunk","data":{"text":"General Kenobi."}}`,
		`{"type":"synthesize","data":{"text":"Hello there. General Kenobi."}}`,
		`{"type":"synthesize-stop"}`,
	}, "synthesize-stopped")
	checkSay(t, log, "-c -fo stdout:raw -", "[:name betty] Hello there. General Kenobi.", types, audio,
		"audio-start", "audio-chunk", "audio-stop", "synthesize-stopped")
}

func TestSayFailure(t *testing.T) {
	dist = t.TempDir() // no say here
	types, _ := exchange(t, "", []string{`{"type":"synthesize","data":{"text":"hi"}}`}, "audio-stop")
	if !slices.Equal(types, []string{"error"}) {
		t.Fatal(types)
	}
}
