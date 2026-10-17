package tts

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Hostile inputs found while attacking the engine; each must stay bounded.
func TestHostileInput(t *testing.T) {
	for _, c := range []struct {
		name, text, lang string
		maxSecs          float64 // audio produced
		tooLong          bool    // must stop with ErrTooLong
	}{
		{"malformed money once panicked", "Cuesta $1..2 hoy.", "es-ES", 10, false},
		{"malformed percent", "Il fait 1,,5% ici.", "fr-FR", 10, false},
		{"huge sung note", "[ah<100000000,10>]", "", 4, false},
		{"8000-letter word", strings.Repeat("ab", 4000), "de-DE", 10, false},
		{"8000-letter word", strings.Repeat("ab", 4000), "fr-FR", 10, false},
		{"2000-digit dial", "[:dial " + strings.Repeat("5", 2000) + "]", "", 12, false},
		{"bracket of 200 long tones", "[" + strings.Repeat(":tone 440,5000 ", 200) + "]", "", maxAudioSecs, true},
		{"1 MB without punctuation", strings.Repeat("door ", 200000), "", maxAudioSecs, true},
	} {
		v := Presets["paul"]
		v.Lang = c.lang
		start, n := time.Now(), 0
		err := Speak(c.text, v, func(b []byte) error { n += len(b); return nil })
		secs := float64(n) / 2 / SampleRate
		if secs > c.maxSecs || time.Since(start) > 3*time.Second || errors.Is(err, ErrTooLong) != c.tooLong {
			t.Errorf("%s (%s): %.1f s of audio in %v, err %v", c.name, c.lang, secs, time.Since(start), err)
		}
	}
}
