package tts

import (
	"fmt"
	"strconv"
	"strings"
)

// Voice shapes the speaker. Field comments give the setting code used in voice strings.
type Voice struct {
	Pitch      float64 // ap: average pitch, Hz
	PitchRange float64 // pr: pitch range, % (0 = monotone)
	HeadSize   float64 // hs: %, scales formants (bigger head = deeper)
	Breath     float64 // br: breathiness, dB 0-72
	Smooth     float64 // sm: %, softer and less buzzy
	Rich       float64 // ri: %, brighter
	Creak      float64 // la: laryngealization (creaky, rough), %
	Assert     float64 // as: %, how far statements fall at the end
	Quick      float64 // qu: %, how fast pitch moves
	Loud       float64 // lo: dB relative to normal
	Nasal      float64 // na: %, nasal resonance in vowels (twang)
	Steps      float64 // pq: %, snap pitch to whole tones (autotune: flat notes, sudden jumps)

	// Intonation shape in Hz; 0 = automatic (scaled from pitch and pitch range).
	Fall       float64 // bf: baseline fall over a phrase
	Hat        float64 // hr: extra rise on a phrase's first accent
	StressRise float64 // sr: rise on an accented syllable

	// Source levels in dB relative to normal.
	GainVoice float64 // gv: voicing
	GainAsp   float64 // gh: aspiration (h, breathy release)
	GainFric  float64 // gf: frication (s, f, sh...)
	GainNasal float64 // gn: nasal murmur (m, n, ng)

	// Upper formants and source shape; 0 = automatic.
	F4, B4, F5, B5 float64 // f4 b4 f5 b5: Hz
	Lax            float64 // lx: %, breath noise even while the glottis is closed
	OpenQuotient   float64 // nf: %, share of each glottal cycle spent open
	Rate           float64 // words per minute
	Lang           string  // language tag ("es-ES"); "" = US English
}

// Presets are the built-in voices.
var Presets = map[string]Voice{
	"paul":   {Pitch: 122, PitchRange: 100, HeadSize: 100, Smooth: 30, Rich: 60, Assert: 65, Quick: 40, Rate: 180},
	"harry":  {Pitch: 95, PitchRange: 80, HeadSize: 110, Smooth: 20, Rich: 80, Assert: 90, Quick: 30, Rate: 170},
	"frank":  {Pitch: 135, PitchRange: 60, HeadSize: 100, Breath: 30, Smooth: 50, Rich: 40, Creak: 40, Assert: 50, Quick: 30, Rate: 165},
	"dennis": {Pitch: 110, PitchRange: 90, HeadSize: 105, Breath: 25, Smooth: 60, Rich: 40, Assert: 55, Quick: 40, Rate: 175},
	"betty":  {Pitch: 210, PitchRange: 110, HeadSize: 85, Breath: 15, Smooth: 40, Rich: 50, Assert: 65, Quick: 50, Rate: 180},
	"ursula": {Pitch: 190, PitchRange: 120, HeadSize: 88, Breath: 5, Smooth: 30, Rich: 80, Assert: 90, Quick: 60, Rate: 185},
	"rita":   {Pitch: 175, PitchRange: 80, HeadSize: 88, Breath: 10, Smooth: 30, Rich: 50, Creak: 45, Assert: 70, Quick: 40, Rate: 175},
	"wendy":  {Pitch: 210, PitchRange: 90, HeadSize: 85, Breath: 70, Smooth: 70, Rich: 30, Assert: 50, Quick: 40, Rate: 170},
	"kit":    {Pitch: 255, PitchRange: 120, HeadSize: 80, Breath: 20, Smooth: 65, Rich: 30, Assert: 60, Quick: 60, Rate: 175},
}

// With applies "code value" pairs such as "ap 90 pr 0 hs 110" on top of v.
func (v Voice) With(settings string) (Voice, error) {
	f := strings.Fields(settings)
	if len(f)%2 != 0 {
		return v, fmt.Errorf("settings %q: expected code/value pairs", settings)
	}
	for i := 0; i < len(f); i += 2 {
		n, err := strconv.ParseFloat(f[i+1], 64)
		if err != nil {
			return v, fmt.Errorf("settings %q: %s is not a number", settings, f[i+1])
		}
		switch f[i] {
		case "ap":
			v.Pitch = clamp(n, 40, 500)
		case "pr":
			v.PitchRange = clamp(n, 0, 250)
		case "hs":
			v.HeadSize = clamp(n, 50, 160)
		case "br":
			v.Breath = clamp(n, 0, 72)
		case "sm":
			v.Smooth = clamp(n, 0, 100)
		case "ri":
			v.Rich = clamp(n, 0, 100)
		case "la":
			v.Creak = clamp(n, 0, 100)
		case "as":
			v.Assert = clamp(n, 0, 100)
		case "qu":
			v.Quick = clamp(n, 0, 100)
		case "lo":
			v.Loud = clamp(n, -30, 12)
		case "na":
			v.Nasal = clamp(n, 0, 100)
		case "pq":
			v.Steps = clamp(n, 0, 100)
		case "bf":
			v.Fall = clamp(n, 0, 200)
		case "hr":
			v.Hat = clamp(n, 0, 200)
		case "sr":
			v.StressRise = clamp(n, 0, 300)
		case "gv":
			v.GainVoice = clamp(n, -30, 20)
		case "gh":
			v.GainAsp = clamp(n, -30, 20)
		case "gf":
			v.GainFric = clamp(n, -30, 20)
		case "gn":
			v.GainNasal = clamp(n, -30, 20)
		case "f4":
			v.F4 = clamp(n, 2500, 5000)
		case "b4":
			v.B4 = clamp(n, 50, 1000)
		case "f5":
			v.F5 = clamp(n, 3000, 6500)
		case "b5":
			v.B5 = clamp(n, 50, 1500)
		case "lx":
			v.Lax = clamp(n, 0, 100)
		case "nf":
			v.OpenQuotient = clamp(n, 0, 95)
		case "sx":
			if n == 0 {
				v.HeadSize = 85
			} else {
				v.HeadSize = 100
			}
		default:
			return v, fmt.Errorf("settings %q: unknown code %q", settings, f[i])
		}
	}
	return v, nil
}

func clamp(x, lo, hi float64) float64 { return max(lo, min(hi, x)) }
