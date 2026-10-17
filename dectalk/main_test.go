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
	"testing"
)

// With FAKE_SAY set, the test binary stands in for DECtalk's say: it checks the args,
// logs stdin to $FAKE_SAY and writes one 16-bit sample per input byte to stdout.
func TestMain(m *testing.M) {
	if log := os.Getenv("FAKE_SAY"); log != "" {
		if !slices.Equal(os.Args[1:], []string{"-c", "-fo", "stdout:raw", "-"}) {
			os.Stderr.WriteString("bad args")
			os.Exit(2)
		}
		in, _ := io.ReadAll(os.Stdin)
		os.WriteFile(log, in, 0o644)
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
	log = filepath.Join(dist, "stdin.log")
	t.Setenv("FAKE_SAY", log)
	return log
}

func TestLoadVoices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "options.json")
	os.WriteFile(path, []byte(`{"voice":"robot","rate":220,"custom_voices":[
		{"name":"robot","base":"paul","settings":"pr 0"},
		{"name":"betty","base":"betty","settings":"hs 110","rate":150}]}`), 0o644)
	vs, def, err := loadVoices(path)
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
	os.WriteFile(path, []byte(`{"voice":"nobody"}`), 0o644)
	if _, _, err := loadVoices(path); err == nil {
		t.Fatal("expected error for unknown default voice")
	}
}

// exchange sends events like Home Assistant and collects replies until stop.
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

func TestDescribe(t *testing.T) {
	// Python wyoming sends data as a separate blob after the header ("{}" + exchange's newline).
	types, _ := exchange(t, []string{`{"type":"describe","data_length":3}` + "\n{}"}, "info")
	if !slices.Equal(types, []string{"info"}) {
		t.Fatal(types)
	}
}

func TestSynthesize(t *testing.T) {
	log := fakeSay(t)
	types, audio := exchange(t, []string{`{"type":"synthesize","data":{"text":"hello","voice":{"name":"kit"}}}`}, "audio-stop")
	want := "[:name kit] hello"
	if got, _ := os.ReadFile(log); string(got) != want {
		t.Fatalf("say got %q", got)
	}
	if !slices.Equal(types, []string{"audio-start", "audio-chunk", "audio-stop"}) || audio != 2*len(want) {
		t.Fatalf("%v, %d bytes", types, audio)
	}
}

func TestStreaming(t *testing.T) {
	log := fakeSay(t)
	// Home Assistant's order: start, chunks, a compatibility synthesize, stop.
	types, audio := exchange(t, []string{
		`{"type":"synthesize-start","data":{"voice":{"name":"betty"}}}`,
		`{"type":"synthesize-chunk","data":{"text":"Hello there. "}}`,
		`{"type":"synthesize-chunk","data":{"text":"General Kenobi."}}`,
		`{"type":"synthesize","data":{"text":"Hello there. General Kenobi."}}`,
		`{"type":"synthesize-stop"}`,
	}, "synthesize-stopped")
	want := "[:name betty] Hello there. General Kenobi."
	if got, _ := os.ReadFile(log); string(got) != want {
		t.Fatalf("say got %q", got)
	}
	if !slices.Equal(types, []string{"audio-start", "audio-chunk", "audio-stop", "synthesize-stopped"}) || audio != 2*len(want) {
		t.Fatalf("%v, %d bytes", types, audio)
	}
}

func TestSayFailure(t *testing.T) {
	dist = t.TempDir() // no say here
	types, _ := exchange(t, []string{`{"type":"synthesize","data":{"text":"hi"}}`}, "audio-stop")
	if !slices.Equal(types, []string{"error"}) {
		t.Fatal(types)
	}
}
