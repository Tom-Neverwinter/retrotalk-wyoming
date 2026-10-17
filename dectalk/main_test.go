package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestParseWAV(t *testing.T) {
	fmtChunk := make([]byte, 16)
	binary.LittleEndian.PutUint16(fmtChunk[0:], 1)     // PCM
	binary.LittleEndian.PutUint16(fmtChunk[2:], 1)     // channels
	binary.LittleEndian.PutUint32(fmtChunk[4:], 11025) // rate
	binary.LittleEndian.PutUint16(fmtChunk[14:], 16)   // bits
	var b bytes.Buffer
	b.WriteString("RIFF\x00\x00\x00\x00WAVE")
	chunk := func(id string, body []byte) {
		b.WriteString(id)
		binary.Write(&b, binary.LittleEndian, uint32(len(body)))
		b.Write(body)
		if len(body)%2 == 1 {
			b.WriteByte(0)
		}
	}
	chunk("fmt ", fmtChunk)
	chunk("LIST", []byte("odd")) // odd-sized chunk must be skipped with its pad byte
	chunk("data", []byte{1, 2, 3, 4})

	rate, width, ch, pcm, err := parseWAV(b.Bytes())
	if err != nil || rate != 11025 || width != 2 || ch != 1 || !bytes.Equal(pcm, []byte{1, 2, 3, 4}) {
		t.Fatalf("got %d %d %d %v %v", rate, width, ch, pcm, err)
	}
	if _, _, _, _, err := parseWAV([]byte("nope")); err == nil {
		t.Fatal("expected error for non-WAV")
	}
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

func TestDescribeRoundTrip(t *testing.T) {
	vs, def, _ := loadVoices("")
	s := &server{voices: vs, def: def, info: info(vs)}
	client, server := net.Pipe()
	go s.handle(server)
	defer client.Close()

	// Python wyoming sends data as a separate blob after the header.
	client.Write([]byte(`{"type":"describe","data_length":2}` + "\n{}"))
	typ, data, err := readEvent(bufio.NewReader(client))
	if err != nil || typ != "info" || len(data) != 1 {
		t.Fatalf("got %q %v %v", typ, data, err)
	}
	var got struct {
		TTS []struct {
			Name   string                  `json:"name"`
			Voices []struct{ Name string } `json:"voices"`
		} `json:"tts"`
	}
	if err := json.Unmarshal(data[0], &got); err != nil || got.TTS[0].Name != "dectalk" || len(got.TTS[0].Voices) != len(builtins) {
		t.Fatalf("bad info: %s (%v)", data[0], err)
	}
}
