package tts

import (
	"testing"
	"time"
)

// FuzzSpeak feeds arbitrary text, commands and phonemes through every language. Nothing may
// panic, hang, or produce more than the per-session audio budget.
//
//	go test -fuzz FuzzSpeak -fuzztime 60s ./tts
func FuzzSpeak(f *testing.F) {
	for _, seed := range []string{
		"The front door is open.", "Cuesta $1..2 hoy.", "Il fait 1,,5% ici.", "[ah<100000000,10>]",
		"[:dial 5551234][:tone 440,5000]", "[:mode spell on]ABC 123", "*really* hot?",
		"[:name hal][:lang gr]Guten Tag, 21,5°C.", "It is -5°F at 7:30 pm, $12.50, 45%.",
		"[d ey<600,32> z iy<600,29>]", "[:dv ap 9999 pr -5 hs 0]x", "[[[:]]]", "1,,,.,.,2e9",
	} {
		f.Add(seed)
	}
	langs := []string{"en-US", "en-GB", "es-ES", "es-419", "de-DE", "fr-FR"}
	f.Fuzz(func(t *testing.T, text string) {
		for _, lang := range langs {
			v := Presets["paul"]
			v.Lang = lang
			start, n := time.Now(), 0
			Speak(text, v, func(b []byte) error { n += len(b); return nil })
			if n > maxAudio {
				t.Fatalf("%s: %d bytes exceeds the budget", lang, n)
			}
			if d := time.Since(start); d > 5*time.Second*raceSlowdown {
				t.Fatalf("%s: took %v for %d chars", lang, d, len(text))
			}
		}
	})
}

// FuzzParsePhonemes: phonemic input never panics and stays within duration limits.
func FuzzParsePhonemes(f *testing.F) {
	for _, seed := range []string{"hh ah1 l ow", "dey<600,22>", "ax<1e99,99>", "_<-5>", "<<>>,,", "zz9<"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		w, ok := parsePhonemes(s)
		if !ok {
			return
		}
		for _, d := range w.dur {
			if d < 0 || d > maxPhonemeMs {
				t.Fatalf("duration %v out of range for %q", d, s)
			}
		}
	})
}
