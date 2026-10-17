package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"
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

func TestDescribeRoundTrip(t *testing.T) {
	client, server := net.Pipe()
	go handle(server)
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
	if err := json.Unmarshal(data[0], &got); err != nil || got.TTS[0].Name != "dectalk" || len(got.TTS[0].Voices) != len(voices) {
		t.Fatalf("bad info: %s (%v)", data[0], err)
	}
}
