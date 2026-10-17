package tts

import (
	"strings"
	"testing"
)

func TestPronunciationRules(t *testing.T) {
	for _, c := range []struct{ lang, word, want string }{
		{"es-ES", "hola", "O1 L A0"},
		{"es-ES", "perro", "P E1 RR O0"},
		{"es-ES", "pero", "P E1 DX O0"},
		{"es-ES", "corazón", "K O0 DX A0 TH O1 N"},
		{"es-419", "corazón", "K O0 DX A0 S O1 N"},
		{"es-ES", "ciudad", "TH Y U0 D A1 D"},
		{"es-ES", "guitarra", "G I0 T A1 RR A0"},
		{"es-ES", "año", "A1 N Y O0"},
		{"de-DE", "Haus", "HH AW1 S"},
		{"de-DE", "ich", "IH1 CX"},
		{"de-DE", "Bach", "B A1 X"},
		{"de-DE", "Straße", "SH T RU A1 S AH0"},
		{"de-DE", "Tag", "T A1 K"},
		{"de-DE", "schön", "SH OE1 N"},
		{"de-DE", "zehn", "TS E1 N"},
		{"de-DE", "Lieder", "L I1 D AH0"},
		{"fr-FR", "bonjour", "B ON0 ZH U1 RU"},
		{"fr-FR", "maison", "M EH0 Z ON1"},
		{"fr-FR", "chat", "SH A1"},
		{"fr-FR", "beaucoup", "B O0 K U1"},
		{"fr-FR", "parler", "P A0 RU L E1"},
		{"fr-FR", "fille", "F I1 Y"},
		{"en-GB", "car", "K AA1"},
		{"en-GB", "bird", "B UR1 D"},
		{"en-GB", "go", "G OU1"},
		{"en-GB", "hot", "HH OH1 T"},
		{"en-GB", "bath", "B AA1 TH"},
	} {
		if got := strings.Join(Languages[c.lang].pronounce(c.word), " "); got != c.want {
			t.Errorf("%s %q = %q, want %q", c.lang, c.word, got, c.want)
		}
	}
}

func TestNumbersAndNormalization(t *testing.T) {
	for _, c := range []struct{ lang, in, want string }{
		{"es-ES", "21", "veintiuno"},
		{"es-ES", "100", "cien"},
		{"es-ES", "175", "ciento setenta y cinco"},
		{"es-ES", "2026", "dos mil veintiséis"},
		{"es-ES", "a las 7:30, 21,5°C", "a las siete y treinta, veintiuno coma cinco grados Celsius"},
		{"es-419", "cuesta $40", "cuesta cuarenta dólares"},
		{"de-DE", "21", "einundzwanzig"},
		{"de-DE", "1999", "eintausend neunhundert neunundneunzig"},
		{"de-DE", "um 7:00 bei 50%", "um sieben Uhr bei fünfzig Prozent"},
		{"fr-FR", "71", "soixante et onze"},
		{"fr-FR", "80", "quatre vingts"},
		{"fr-FR", "99", "quatre vingt dix neuf"},
		{"fr-FR", "21", "vingt et un"},
		{"fr-FR", "il fait -3°", "il fait moins trois degrés"},
	} {
		got := strings.ReplaceAll(strings.Join(strings.Fields(Languages[c.lang].normalize(c.in)), " "), " ,", ",")
		if got != c.want {
			t.Errorf("%s %q = %q, want %q", c.lang, c.in, got, c.want)
		}
	}
}

func TestLanguageSwitching(t *testing.T) {
	if LanguageTag("gr") != "de-DE" || LanguageTag("es-419") != "es-419" || LanguageTag("xx") != "" {
		t.Fatal("language tags")
	}
	s := (&Speaker{}).Session(Presets["paul"])
	run(s, ":lang sp")
	if s.st.v.Lang != "es-ES" {
		t.Fatalf("[:lang sp] -> %q", s.st.v.Lang)
	}
	for tag := range Languages {
		v := Presets["paul"]
		v.Lang = tag
		var n int
		Speak("Hola, guten Tag, bonjour, 21.", v, func(b []byte) error { n += len(b); return nil })
		if n < SampleRate { // at least half a second of audio
			t.Errorf("%s produced only %d bytes", tag, n)
		}
	}
}

func TestFrenchSilentE(t *testing.T) {
	for word, want := range map[string]string{"machine": "M A0 SH I1 N", "garage": "G A0 RU A1 ZH",
		"cuisine": "K UE0 I0 Z I1 N", "toutes": "T U1 T", "les": "L EH1", "est": "EH1"} {
		if got := strings.Join(frenchG2P(word), " "); got != want {
			t.Errorf("%q = %q, want %q", word, got, want)
		}
	}
}

// The last stressed vowel's pitch: yes/no questions end high, wh-questions and statements
// low, list items (comma) a little high.
func TestIntonation(t *testing.T) {
	v, L := Presets["paul"], Languages["en-US"]
	final := func(text string) float64 {
		ph := phrases(L.normalize(text), false, L)[0]
		segs := segments(ph, nil, L)
		timeAndPitch(segs, contour(ph, L), v, L)
		last := 0.0
		for _, s := range segs {
			if s.stress >= 0 {
				last = s.f0
			}
		}
		return last
	}
	if q := final("Is the door open?"); q < v.Pitch*1.3 {
		t.Errorf("yes/no question ends at %.0f Hz, want a rise", q)
	}
	if q := final("Where is the door?"); q > v.Pitch {
		t.Errorf("wh-question ends at %.0f Hz, want a fall", q)
	}
	if c := final("Red, green and blue."); c < v.Pitch*1.1 {
		t.Errorf("list item ends at %.0f Hz, want a continuation rise", c)
	}
}
