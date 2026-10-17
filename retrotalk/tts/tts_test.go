package tts

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"It is 72°F":           "It is seventy two degrees fahrenheit",
		"at 7:30 pm":           "at seven thirty PM",
		"at 7:05":              "at seven oh five",
		"at 7:00":              "at seven o'clock",
		"costs $12.50":         "costs twelve dollars and fifty cents",
		"humidity 45%":         "humidity forty five percent",
		"the 21st":             "the twenty first",
		"in 1984":              "in nineteen eighty four",
		"1,250 watts":          "one thousand two hundred fifty watts",
		"-5 degrees":           "minus five degrees",
		"it is -5°F":           "it is minus five degrees fahrenheit",
		"3.14":                 "three point one four",
		"don’t":                "don't",
		"café":                 "cafe",
		"[:rate 200] hi":       "hi",
		"Tom & Jerry":          "Tom and Jerry",
		"the U.S. army":        "the US army",
		"room 2048 has 1000 W": "room twenty forty eight has one thousand W",
	} {
		if got := strings.Join(strings.Fields(normalize(in)), " "); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPhrases(t *testing.T) {
	ps := phrases(normalize("Dr. Smith is here, is he? Yes!"), false, Languages["en-US"])
	if len(ps) != 3 || ps[0].end != ',' || ps[1].end != '?' || ps[2].end != '!' ||
		texts(ps[0]) != "doctor Smith is here" {
		t.Fatalf("%+v", ps)
	}
	if c, r := SplitPhrases("Hello there. How are y"); c != "Hello there." || r != " How are y" {
		t.Fatalf("%q %q", c, r)
	}
}

func TestPronounce(t *testing.T) {
	for word, want := range map[string]string{
		"hello":   "HH AH0 L OW1",  // dictionary
		"TV":      "T IY1 V IY1",   // initialism
		"dogs's":  "",              // not checked, just must not panic
		"blorpt":  "B L AO1 R P T", // letter-to-sound: or -> AO R
		"zinking": "Z IH1 NG K IH0 NG",
	} {
		got := strings.Join(pronounce(word), " ")
		if want != "" && got != want {
			t.Errorf("pronounce(%q) = %q, want %q", word, got, want)
		}
	}
}

// The resonator's impulse response must ring at its center frequency.
func TestResonator(t *testing.T) {
	var r resonator
	r.set(1000, 50)
	crossings, prev := 0, 0.0
	for n := 0; n < SampleRate/10; n++ { // 100 ms
		x := 0.0
		if n == 0 {
			x = 1
		}
		y := r.step(x)
		if (y > 0) != (prev > 0) && n > 0 {
			crossings++
		}
		prev = y
	}
	if f := float64(crossings) / 2 / 0.1; math.Abs(f-1000) > 30 {
		t.Fatalf("rings at %.0f Hz", f)
	}
}

func pcm(t *testing.T, text string, v Voice) []int16 {
	var out []int16
	err := Speak(text, v, func(b []byte) error {
		for i := 0; i+1 < len(b); i += 2 {
			out = append(out, int16(binary.LittleEndian.Uint16(b[i:])))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSpeak(t *testing.T) {
	samples := pcm(t, "Hello world. The front door is open.", Presets["paul"])
	seconds := float64(len(samples)) / SampleRate
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	rms := math.Sqrt(sum / float64(len(samples)))
	if seconds < 1.5 || seconds > 5 || rms < 500 || rms > 12000 {
		t.Fatalf("%.2fs of audio, rms %.0f", seconds, rms)
	}
	fast := Presets["paul"]
	fast.Rate = 360
	if f := float64(len(pcm(t, "Hello world. The front door is open.", fast))) / SampleRate; f > seconds*0.75 {
		t.Fatalf("rate 360 gave %.2fs vs %.2fs at 180", f, seconds)
	}
}

// TTS_SAMPLES=dir writes WAVs for listening and intelligibility checks:
// one per "name|voice|text" line of $TTS_SAMPLES/sentences.txt.
func TestWriteSamples(t *testing.T) {
	dir := os.Getenv("TTS_SAMPLES")
	if dir == "" {
		t.Skip("set TTS_SAMPLES to write samples")
	}
	lines, err := os.ReadFile(filepath.Join(dir, "sentences.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// TTS_TUNE=file.json overrides tune constants, for optimizing against recognizers.
	if path := os.Getenv("TTS_TUNE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var over map[string]float64
		if err := json.Unmarshal(b, &over); err != nil {
			t.Fatal(err)
		}
		if err := Override(over); err != nil {
			t.Fatal(err)
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(lines)), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 3)
		v, err := sampleVoice(parts[1])
		if err != nil {
			t.Fatal(err)
		}
		writeWAV(t, filepath.Join(dir, parts[0]+".wav"), pcm(t, parts[2], v))
	}
}

// sampleVoice parses a voice field: a preset name, settings for paul ("ap 90 pr 0"), or
// "base:settings@rate" (e.g. "betty:ap 150 pq 80@190"), optionally ending "#lang" ("paul#es-ES").
func sampleVoice(field string) (Voice, error) {
	field, lang, _ := strings.Cut(field, "#")
	if v, ok := Presets[field]; ok {
		v.Lang = lang
		return v, nil
	}
	v, err := sampleVoiceSettings(field)
	v.Lang = lang
	return v, err
}

func sampleVoiceSettings(field string) (Voice, error) {
	field, rate, _ := strings.Cut(field, "@")
	base, settings, found := strings.Cut(field, ":")
	if !found {
		base, settings = "paul", field
	}
	v, err := Presets[base].With(settings)
	if r, e := strconv.ParseFloat(rate, 64); e == nil {
		v.Rate = r
	}
	return v, err
}

func writeWAV(t *testing.T, path string, s []int16) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := func(v any) { binary.Write(f, binary.LittleEndian, v) }
	f.WriteString("RIFF")
	w(uint32(36 + 2*len(s)))
	f.WriteString("WAVEfmt ")
	w(uint32(16))
	w(uint16(1))
	w(uint16(1))
	w(uint32(SampleRate))
	w(uint32(SampleRate * 2))
	w(uint16(2))
	w(uint16(16))
	f.WriteString("data")
	w(uint32(2 * len(s)))
	w(s)
}
