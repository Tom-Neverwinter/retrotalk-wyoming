package tts

import (
	"math"
	"strings"
	"testing"
)

func texts(ph phrase) string {
	var w []string
	for _, x := range ph.words {
		w = append(w, x.text)
	}
	return strings.Join(w, " ")
}

func TestPhonemicInput(t *testing.T) {
	w, ok := parsePhonemes("hh ah1 l ow")
	if !ok || strings.Join(w.phones, " ") != "HH AH1 L OW" {
		t.Fatalf("arpabet: %v %v", w.phones, ok)
	}
	w, ok = parsePhonemes("dey<600,22>")
	if !ok || strings.Join(w.phones, " ") != "D EY" || w.dur[1] != 600 || math.Abs(w.f0[1]-220) > 1 {
		t.Fatalf("singing: %+v %v", w, ok) // note 22 = A3 = 220 Hz
	}
	w, ok = parsePhonemes("ax bawt")
	if !ok || strings.Join(w.phones, " ") != "AH0 B AW T" {
		t.Fatalf("decTalk aliases: %v %v", w.phones, ok)
	}
	for _, text := range []string{"sic", "hello", "ok", ""} {
		if _, ok := parsePhonemes(text); ok {
			t.Errorf("%q should stay text", text)
		}
	}
}

// run applies a command bracket and returns the audio it produced.
func run(s *Session, body string) []byte {
	var pcm []byte
	s.command(body, func(b []byte) error { pcm = append(pcm, b...); return nil })
	return pcm
}

func TestCommands(t *testing.T) {
	sp := &Speaker{Voices: map[string]Voice{"hal": {Pitch: 85, PitchRange: 100, HeadSize: 96, Rate: 235}}}
	s := sp.Session(Presets["paul"])
	run(s, ":rate 250 :volume 100 :cp 300 :pp 900 :punct all")
	if s.st.v.Rate != 250 || math.Abs(s.st.v.Loud-6.02) > 0.1 || s.st.commaMs != 300 || s.st.periodMs != 900 || s.st.punct != "all" {
		t.Fatalf("state %+v", s.st)
	}
	run(s, ":name hal")
	if s.st.v.Pitch != 85 {
		t.Fatalf("custom voice not selected: %+v", s.st.v)
	}
	run(s, ":nb")
	if s.st.v.Pitch != Presets["betty"].Pitch {
		t.Fatal("[:nb] should select betty")
	}
	run(s, ":dv ap 150 pq 50")
	if s.st.v.Pitch != 150 || s.st.v.Steps != 50 {
		t.Fatalf("dv: %+v", s.st.v)
	}
	run(s, ":mode spell on")
	if !s.st.spell {
		t.Fatal("spell mode")
	}
	if got := len(run(s, ":tone 440,500")); got != 2*8000 {
		t.Fatalf("tone: %d bytes", got)
	}
	if got := len(run(s, ":dial 55")); got != 2*2*(1600+960) {
		t.Fatalf("dial: %d bytes", got)
	}
	if got := len(run(s, ":pause 250")); got != 2*4000 {
		t.Fatalf("pause: %d bytes", got)
	}
}

func TestSplitBrackets(t *testing.T) {
	parts := splitBrackets("Hi [:rate 200] there [sic].")
	if len(parts) != 5 || !parts[1].bracket || parts[1].s != ":rate 200" || parts[3].s != "sic" {
		t.Fatalf("%+v", parts)
	}
	if c, r := SplitPhrases("Beep [:tone 440, 500] now. Next"); c != "Beep [:tone 440, 500] now." || r != " Next" {
		t.Fatalf("split inside brackets: %q %q", c, r)
	}
}

func TestEmphasisPunctSpell(t *testing.T) {
	ps := phrases(normalize("It is *really* hot"), false, Languages["en-US"])
	if len(ps) != 1 || !ps[0].words[2].emph || ps[0].words[1].emph {
		t.Fatalf("emphasis: %+v", ps)
	}
	if got := texts(phrases(normalize("Yes, no."), true, Languages["en-US"])[0]); got != "Yes comma" {
		t.Fatalf("punct all: %q", got)
	}
	if sp := spellPhrases("ab1"); len(sp) != 1 || len(sp[0].words) != 3 || sp[0].words[2].text != "one" {
		t.Fatalf("spell: %+v", sp)
	}
}

func TestLexiconAndPronounce(t *testing.T) {
	if p, err := Pronounce("[n w ih1 n]"); err != nil || strings.Join(p, " ") != "N W IH1 N" {
		t.Fatalf("phonemic: %v %v", p, err)
	}
	if p, err := Pronounce("new win"); err != nil || strings.Join(p, " ") != "N UW1 W IH1 N" {
		t.Fatalf("respelling: %v %v", p, err)
	}
	ph := phrases(normalize("Nguyen"), false, Languages["en-US"])[0]
	segs := segments(ph, map[string][]string{"nguyen": {"N", "W", "IH1", "N"}}, Languages["en-US"])
	if len(segs) != 4 || segs[2].name != "IH" {
		t.Fatalf("lexicon: %+v", segs)
	}
}

func TestSinging(t *testing.T) {
	// Two sung notes of 400 ms each, plus the 60 ms pause, plus lead-in.
	var n int
	(&Speaker{}).Speak("[ah<400,22> ah<400,26>]", Presets["paul"], func(b []byte) error { n += len(b); return nil })
	if ms := float64(n) / 2 / SampleRate * 1000; ms < 800 || ms > 1000 {
		t.Fatalf("sung length %.0f ms", ms)
	}
}

// Every voice setting must change the sound, and an empty setting string must not.
func TestEverySettingHasAnEffect(t *testing.T) {
	render := func(settings string) string {
		v, err := Presets["paul"].With(settings)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		Speak("She sells sea shells, my mother hums.", v, func(p []byte) error { b.Write(p); return nil })
		return b.String()
	}
	base := render("")
	if render("") != base {
		t.Fatal("synthesis is not deterministic")
	}
	for _, s := range []string{"ap 150", "pr 20", "hs 120", "sx 0", "br 40", "sm 90", "ri 10", "la 60",
		"as 10", "qu 90", "lo -6", "na 10", "pq 80", "bf 60", "hr 60", "sr 80", "gv -6", "gh 10",
		"gf -10", "gn 10", "f4 3800", "b4 600", "f5 4500", "b5 900", "lx 50", "nf 40"} {
		if render(s) == base {
			t.Errorf("%q has no effect", s)
		}
	}
}
