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
	Rate       float64 // words per minute
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
	"kit":    {Pitch: 270, PitchRange: 120, HeadSize: 78, Breath: 10, Smooth: 40, Rich: 50, Assert: 60, Quick: 60, Rate: 180},
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
