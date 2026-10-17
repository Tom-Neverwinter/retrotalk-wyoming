package tts

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Speaker speaks text containing DECtalk-style inline commands and phonemic input.
//
// Commands (several may share one bracket, e.g. "[:np :ra 200]"):
//
//	[:rate N] [:ra N]           words per minute (75-600)
//	[:volume N] [:vo N]         0-100, 50 = normal
//	[:name V] [:np] [:nb] ...   switch voice (built-in, or one of Speaker.Voices)
//	[:dv ap 90 pr 0 ...]        change voice settings (see Voice.With)
//	[:pause MS] [:cp MS] [:pp MS]  insert silence; comma and period pause lengths
//	[:mode spell on|off]        spell words letter by letter
//	[:punct all|some|none]      read punctuation aloud (all) or not
//	[:tone HZ,MS]  [:dial DIGITS]  sine tone; telephone (DTMF) digits
//	[:lang es|de|fr|uk|us|la]   switch language (also BCP 47 tags like es-419)
//	[:phoneme on|off]           accepted for compatibility; phonemes always work
//
// Phonemic input: "[hh ah1 l ow]" (ARPAbet, as in CMUdict) or DECtalk-style codes such as
// "[dey<600,22>zi<600,19>]". "<ms>" sets a phoneme's duration, "<ms,note>" also its pitch as
// a note number (1 = C2 ... 37 = C5) for singing; "_" is silence; ' or a trailing 1/2 marks
// stress. *Asterisks* around words emphasize them.
type Speaker struct {
	Voices  map[string]Voice    // extra voices for [:name x]; built-in presets always work
	Lexicon map[string][]string // lower-case word -> phonemes, overriding the dictionary
}

// Session speaks a sequence of texts, keeping command state (voice, modes) between calls:
// a stream of text chunks is one Session.
type Session struct {
	sp    *Speaker
	st    state
	spent int // bytes of audio emitted so far, against maxAudio
}

// emit passes audio on while the session is within its budget.
func (s *Session) emit(pcm []byte, out func([]byte) error) error {
	if s.spent+len(pcm) > maxAudio {
		return ErrTooLong
	}
	s.spent += len(pcm)
	return out(pcm)
}

// Limits on what one session (one message or one stream) may produce.
const (
	maxPhonemeMs = 3000    // longest phonemic-input duration (a long sung note)
	maxDialKeys  = 64      // [:dial] digits
	maxAudioSecs = 10 * 60 // total audio per session
	maxAudio     = maxAudioSecs * SampleRate * 2
)

// ErrTooLong stops a session that has produced its maximum amount of audio.
var ErrTooLong = errors.New("message too long: more than 10 minutes of audio")

type state struct {
	v                 Voice
	spell             bool
	punct             string // "some" (default), "all" (read aloud) or "none"
	commaMs, periodMs float64
	downstep          int // clauses since the last sentence end
}

// Session starts a session with voice v.
func (sp *Speaker) Session(v Voice) *Session {
	return &Session{sp: sp, st: state{v: v, punct: "some", commaMs: 180, periodMs: 380}}
}

// Speak speaks one text in a fresh session.
func (sp *Speaker) Speak(text string, v Voice, emit func(pcm []byte) error) error {
	return sp.Session(v).Speak(text, emit)
}

// Speak synthesizes text, calling emit with 16-bit PCM for each phrase, tone or pause as soon
// as it is ready. It stops at the first emit error.
func (s *Session) Speak(text string, emit func(pcm []byte) error) error {
	for _, part := range splitBrackets(text) {
		if !part.bracket {
			if err := s.speakText(part.s, emit); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(part.s), ":") {
			if err := s.command(part.s, emit); err != nil {
				return err
			}
			continue
		}
		if w, ok := parsePhonemes(part.s); ok {
			if err := s.speakPhrase(phrase{words: []word{w}}, 60, emit); err != nil {
				return err
			}
			continue
		}
		if err := s.speakText(part.s, emit); err != nil { // "[sic]": just text
			return err
		}
	}
	return nil
}

func (s *Session) speakText(text string, emit func([]byte) error) error {
	var ps []phrase
	if s.st.spell {
		ps = spellPhrases(text)
	} else {
		L := langFor(s.st.v.Lang)
		ps = phrases(L.normalize(text), s.st.punct == "all", L)
	}
	L := langFor(s.st.v.Lang)
	for _, ph := range ps {
		pause := s.pause(ph.end)
		ph.end = contour(ph, L)
		if err := s.speakPhrase(ph, pause, emit); err != nil {
			return err
		}
		// Downstep: each clause after a comma starts a little lower, until the sentence ends.
		if ph.end == ',' {
			s.st.downstep = min(s.st.downstep+1, 3)
		} else {
			s.st.downstep = 0
		}
	}
	return nil
}

func (s *Session) pause(end byte) float64 {
	switch end {
	case ',':
		return s.st.commaMs
	case 0:
		return 60
	}
	return s.st.periodMs
}

func (s *Session) speakPhrase(ph phrase, pauseMs float64, emit func([]byte) error) error {
	v := s.st.v
	v.Pitch *= 1 - 0.04*float64(s.st.downstep)
	L := langFor(v.Lang)
	segs := segments(ph, s.sp.Lexicon, L)
	if len(segs) == 0 {
		return nil
	}
	timeAndPitch(segs, ph.end, v, L)
	return s.emit(newSynth(v).render(frames(segs, pauseMs, v, L.aspirate)), emit)
}

// command applies one bracket of commands; tones and pauses are emitted one at a time, so a
// bracket of many tones never sits in memory at once.
func (s *Session) command(body string, emit func([]byte) error) error {
	for _, c := range strings.Split(body, ":")[1:] {
		var pcm []byte
		f := strings.Fields(strings.ReplaceAll(c, ",", " "))
		if len(f) == 0 {
			continue
		}
		arg := func(i int) float64 {
			if i < len(f) {
				n, _ := strconv.ParseFloat(f[i], 64)
				return n
			}
			return 0
		}
		switch name := strings.ToLower(f[0]); {
		case name == "rate" || name == "ra":
			s.st.v.Rate = clamp(arg(1), 75, 600)
		case name == "volume" || name == "vo":
			s.st.v.Loud = clamp(20*math.Log10(math.Max(arg(1), 1)/50), -30, 12)
		case name == "name" && len(f) > 1:
			s.setVoice(strings.ToLower(f[1]))
		case len(name) == 2 && name[0] == 'n' && shortNames[name] != "":
			s.setVoice(shortNames[name])
		case name == "dv":
			if v, err := s.st.v.With(strings.Join(f[1:], " ")); err == nil {
				s.st.v = v
			}
		case name == "pause":
			pcm = append(pcm, silence(arg(1))...)
		case name == "cp":
			s.st.commaMs = clamp(arg(1), 0, 5000)
		case name == "pp":
			s.st.periodMs = clamp(arg(1), 0, 5000)
		case name == "mode" && len(f) > 2 && strings.ToLower(f[1]) == "spell":
			s.st.spell = strings.ToLower(f[2]) == "on"
		case (name == "punct" || name == "pu") && len(f) > 1:
			s.st.punct = strings.ToLower(f[1])
		case name == "tone":
			pcm = append(pcm, tone(clamp(arg(1), 20, 7000), clamp(arg(2), 1, 5000), 0)...)
		case (name == "lang" || name == "language") && len(f) > 1:
			if tag := LanguageTag(f[1]); tag != "" {
				s.st.v.Lang = tag
			}
		case name == "dial" && len(f) > 1:
			keys := strings.Join(f[1:], "")
			pcm = append(pcm, dial(keys[:min(len(keys), maxDialKeys)])...)
		}
		if len(pcm) > 0 {
			if err := s.emit(pcm, emit); err != nil {
				return err
			}
		}
	}
	return nil
}

// shortNames are DECtalk's two-letter voice commands ([:np] = paul, ...).
var shortNames = map[string]string{"np": "paul", "nb": "betty", "nh": "harry", "nf": "frank",
	"nd": "dennis", "nk": "kit", "nu": "ursula", "nr": "rita", "nw": "wendy"}

func (s *Session) setVoice(name string) {
	if v, ok := s.sp.Voices[name]; ok {
		s.st.v = v
	} else if v, ok := Presets[name]; ok {
		s.st.v = v
	}
}

type bracketPart struct {
	s       string
	bracket bool
}

// splitBrackets splits text into plain runs and the contents of [...] groups.
func splitBrackets(text string) []bracketPart {
	var out []bracketPart
	for {
		i := strings.IndexByte(text, '[')
		j := strings.IndexByte(text[max(i, 0):], ']') + max(i, 0)
		if i < 0 || j < i {
			if text != "" {
				out = append(out, bracketPart{s: text})
			}
			return out
		}
		if i > 0 {
			out = append(out, bracketPart{s: text[:i]})
		}
		out = append(out, bracketPart{s: text[i+1 : j], bracket: true})
		text = text[j+1:]
	}
}

func silence(ms float64) []byte {
	return make([]byte, 2*int(clamp(ms, 0, 10000)*SampleRate/1000))
}

// tone renders a sine (plus an optional second frequency, for DTMF) with 5 ms fades.
func tone(hz, ms, hz2 float64) []byte {
	n := int(ms * SampleRate / 1000)
	fade := SampleRate / 200
	out := make([]byte, 0, 2*n)
	for i := 0; i < n; i++ {
		t := float64(i) / SampleRate
		x := math.Sin(2 * math.Pi * hz * t)
		if hz2 > 0 {
			x = (x + math.Sin(2*math.Pi*hz2*t)) / 2
		}
		g := math.Min(1, float64(min(i, n-1-i))/float64(fade))
		y := int16(x * g * 9000)
		out = append(out, byte(y), byte(y>>8))
	}
	return out
}

// dial renders telephone keypad tones: 100 ms per key, 60 ms gaps.
func dial(digits string) []byte {
	rows, cols := []float64{697, 770, 852, 941}, []float64{1209, 1336, 1477, 1633}
	keys := []string{"123A", "456B", "789C", "*0#D"}
	var out []byte
	for _, d := range strings.ToUpper(digits) {
		for r, row := range keys {
			if c := strings.IndexRune(row, d); c >= 0 {
				out = append(out, tone(rows[r], 100, cols[c])...)
				out = append(out, silence(60)...)
			}
		}
	}
	return out
}

// arpabet is the phonemic input inventory (English, as in CMUdict). Language-specific
// phonemes come from each language's rules, so bracketed words like [ok] stay text.
var arpabet = map[string]bool{}

func init() {
	for _, p := range strings.Fields(`AA AE AH AO AW AY B CH D DH EH ER EY F G HH IH IY JH K L M N NG
		OW OY P R S SH T TH UH UW V W Y Z ZH`) {
		arpabet[p] = true
	}
}

// Phonemic input codes: ARPAbet (as in CMUdict) plus DECtalk spellings that differ.
var phonemeAliases = map[string][]string{
	"ax": {"AH0"}, "ix": {"IH0"}, "rr": {"ER1"}, "yu": {"Y", "UW1"}, "el": {"AH0", "L"},
	"en": {"AH0", "N"}, "lx": {"L"}, "rx": {"R"}, "dx": {"D"}, "q": {"_"}, "_": {"_"},
}

// parsePhonemes reads "[hh ah1 l ow]" or "[dey<600,22>zi<600,19>]"; ok is false if s isn't
// phonemic input (so "[sic]" stays text).
func parsePhonemes(s string) (word, bool) {
	var w word
	s = strings.ToLower(s)
	if strings.TrimSpace(s) == "" {
		return w, false
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '-':
			i++
			continue
		case c == '\'' || c == '`':
			i++ // stress mark before a vowel: handled by giving the next vowel stress 1
			continue
		}
		var codes []string
		n := 0
		for _, l := range []int{2, 1} { // greedy: two-letter codes first
			if i+l > len(s) {
				continue
			}
			tok := s[i : i+l]
			if a, ok := phonemeAliases[tok]; ok {
				codes, n = a, l
				break
			}
			if arpabet[strings.ToUpper(tok)] {
				codes, n = []string{strings.ToUpper(tok)}, l
				break
			}
		}
		if n == 0 {
			return w, false
		}
		i += n
		if i < len(s) && s[i] >= '0' && s[i] <= '2' { // ARPAbet stress digit
			if p := phonemes[strings.TrimRight(codes[len(codes)-1], "012")]; p != nil && p.flags&vowel != 0 {
				codes = append(codes[:len(codes)-1:len(codes)-1], strings.TrimRight(codes[len(codes)-1], "012")+string(s[i]))
			}
			i++
		}
		dur, f0 := 0.0, 0.0
		if i < len(s) && s[i] == '<' { // <ms> or <ms,note>
			j := strings.IndexByte(s[i:], '>')
			if j < 0 {
				return w, false
			}
			parts := strings.Split(s[i+1:i+j], ",")
			dur, _ = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			dur = clamp(dur, 0, maxPhonemeMs)
			if len(parts) > 1 {
				note, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if note >= 1 && note <= 37 {
					f0 = 65.41 * math.Pow(2, (note-1)/12) // 1 = C2
				}
			}
			i += j + 1
		}
		for k, code := range codes {
			w.phones = append(w.phones, code)
			d, f := 0.0, f0
			if k == len(codes)-1 {
				d = dur
			}
			w.dur, w.f0 = append(w.dur, d), append(w.f0, f)
		}
	}
	return w, len(w.phones) > 0
}

// Pronounce turns a pronunciation into phonemes: phonemic input ("[n w ih1 n]" or
// "n w ih1 n") or a respelling in ordinary words ("new win").
func Pronounce(say string) ([]string, error) {
	inner := strings.Trim(strings.TrimSpace(say), "[]")
	if w, ok := parsePhonemes(inner); ok && strings.HasPrefix(strings.TrimSpace(say), "[") {
		return w.phones, nil
	}
	var out []string
	for _, ph := range phrases(normalize(say), false, Languages["en-US"]) {
		for _, wd := range ph.words {
			out = append(out, pronounce(wd.text)...)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("pronunciation %q has no sounds", say)
	}
	return out, nil
}

// contour is the phrase-final pitch movement: wh-questions ("Where is it?") fall like
// statements; yes/no questions ("Is it open?") rise.
func contour(ph phrase, L *language) byte {
	if ph.end == '?' && len(ph.words) > 0 && L.wh[strings.ToLower(ph.words[0].text)] {
		return '.'
	}
	return ph.end
}
