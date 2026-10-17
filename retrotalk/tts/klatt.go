package tts

import "math"

// Formant synthesizer after Klatt (1980) "Software for a cascade/parallel formant
// synthesizer" (JASA 67) with a KLGLOTT88-style voicing source (Klatt & Klatt 1990):
// voicing and aspiration excite a cascade of resonators; frication excites a parallel
// resonator plus a flat bypass.

const (
	SampleRate = 16000
	frameMs    = 5
	frameLen   = SampleRate * frameMs / 1000
)

// resonator is Klatt's two-pole digital resonator, normalized to unity gain at 0 Hz.
type resonator struct{ a, b, c, y1, y2 float64 }

func (r *resonator) set(f, bw float64) {
	f = math.Min(f, 0.45*SampleRate)
	t := 1.0 / SampleRate
	r.c = -math.Exp(-2 * math.Pi * bw * t)
	r.b = 2 * math.Exp(-math.Pi*bw*t) * math.Cos(2*math.Pi*f*t)
	r.a = 1 - r.b - r.c
}

func (r *resonator) step(x float64) float64 {
	y := r.a*x + r.b*r.y1 + r.c*r.y2
	r.y2, r.y1 = r.y1, y
	return y
}

// antiresonator is the inverse of a resonator: a pair of zeros (nasal antiformant).
type antiresonator struct{ a, b, c, x1, x2 float64 }

func (r *antiresonator) set(f, bw float64) {
	var p resonator
	p.set(f, bw)
	r.a, r.b, r.c = 1/p.a, -p.b/p.a, -p.c/p.a
}

func (r *antiresonator) step(x float64) float64 {
	y := r.a*x + r.b*r.x1 + r.c*r.x2
	r.x2, r.x1 = r.x1, x
	return y
}

// bandpass has unity gain at f and none at 0 Hz, so noise sources keep their place cues
// (an [s] above 4 kHz, a [t] burst high, a [k] burst mid) instead of leaking lows.
type bandpass struct{ b0, a1, a2, x1, x2, y1, y2 float64 }

func (r *bandpass) set(f, bw float64) {
	w := 2 * math.Pi * math.Min(f, 0.45*SampleRate) / SampleRate
	alpha := math.Sin(w) / (2 * f / bw)
	a0 := 1 + alpha
	r.b0, r.a1, r.a2 = alpha/a0, -2*math.Cos(w)/a0, (1-alpha)/a0
}

func (r *bandpass) step(x float64) float64 {
	y := r.b0*(x-r.x2) - r.a1*r.y1 - r.a2*r.y2
	r.x2, r.x1 = r.x1, x
	r.y2, r.y1 = r.y1, y
	return y
}

// frame is one 5 ms set of synthesizer controls.
type frame struct {
	f0         float64 // Hz; 0 = no voicing
	av, ah, af float64 // voicing, aspiration, frication amplitudes (0..1)
	f, bw      [5]float64
	nasalZero  float64
	noise      noiseShape
	spread     int // formant smoothing half-width in frames
}

// glottis produces the derivative of a KLGLOTT88 flow pulse: u(x) = x^2 - x^3 over the
// open phase, so the output (2x - 3x^2) ends in a sharp negative step at closure.
type glottis struct{ pos, period, open, gain float64 }

func (g *glottis) next(f0, openQuotient, jitter float64, rnd *rng) float64 {
	if g.pos >= g.period {
		g.pos -= g.period
		if f0 <= 0 {
			g.period, g.open = frameLen, 0
		} else {
			g.period = SampleRate / f0 * (1 + jitter*rnd.signed())
			g.open = openQuotient * g.period
			// Each pulse's closure excites the tract equally, so power grows with pitch;
			// scaling by sqrt(120/f0) keeps high voices from sounding louder than low ones.
			g.gain = math.Sqrt(120/f0) * (1 + 0.5*jitter*rnd.signed()) // shimmer goes with jitter
		}
	}
	x := g.pos / g.open
	g.pos++
	if g.open == 0 || x >= 1 {
		return 0
	}
	return (2*x - 3*x*x) * g.gain
}

func (g *glottis) isOpen() bool { return g.open > 0 && g.pos < g.open }

// rng is a small xorshift generator so output is deterministic.
type rng struct{ s uint32 }

func (r *rng) signed() float64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 17
	r.s ^= r.s << 5
	return float64(r.s)/float64(1<<31) - 1
}

// gauss approximates Gaussian noise (sum of uniforms).
func (r *rng) gauss() float64 { return (r.signed() + r.signed() + r.signed() + r.signed()) / 2 }

type synth struct {
	v                      Voice
	glot                   glottis
	rnd                    rng
	tilt                   float64
	nasalPole              resonator
	nasalZero              antiresonator
	cascade                [5]resonator
	fricRes                bandpass
	openQuotient, tiltCoef float64
	breath, jitter, gain   float64
	aspiration             float64
	gv, gf, lax            float64
}

func newSynth(v Voice) *synth {
	s := &synth{v: v, rnd: rng{s: 0x2545F491}}
	s.openQuotient = 0.65 - 0.25*v.Rich/100 + 0.15*v.Smooth/100
	if v.OpenQuotient > 0 {
		s.openQuotient = v.OpenQuotient / 100
	}
	s.tiltCoef = 0.15 + 0.6*v.Smooth/100
	s.breath = v.Breath / 72
	s.jitter = 0.005 + 0.06*v.Creak/100
	// Nasality's antiformant costs about 3 dB once it leaves the nasal pole; give it back.
	nasalComp := 3 * min(1, v.Nasal/20)
	s.gain = 0.27 * math.Pow(10, (v.Loud+nasalComp)/20)
	s.aspiration = tune["asp_gain"] * math.Pow(10, v.GainAsp/20)
	s.gv, s.gf = math.Pow(10, v.GainVoice/20), math.Pow(10, v.GainFric/20)
	s.lax = v.Lax / 100
	s.nasalPole.set(270, 100)
	return s
}

// render turns frames into 16-bit little-endian PCM.
func (s *synth) render(frames []frame) []byte {
	out := make([]byte, 0, len(frames)*frameLen*2)
	scale := 100 / s.v.HeadSize
	for i, fr := range frames {
		next := fr
		if i+1 < len(frames) {
			next = frames[i+1]
		}
		for k := range s.cascade {
			f, bw := fr.f[k]*scale, fr.bw[k]*math.Sqrt(scale)
			if k == 3 && s.v.F4 > 0 {
				f = s.v.F4
			}
			if k == 3 && s.v.B4 > 0 {
				bw = s.v.B4
			}
			if k == 4 && s.v.F5 > 0 {
				f = s.v.F5
			}
			if k == 4 && s.v.B5 > 0 {
				bw = s.v.B5
			}
			s.cascade[k].set(f, bw)
		}
		if fr.nasalZero > 0 {
			s.nasalZero.set(fr.nasalZero*scale, 100)
		} else {
			// Without nasality the zero cancels the nasal pole. Nasality moves it away, coupling
			// in the nasal resonance (Klatt's vowel nasalization).
			s.nasalZero.set(270+2.5*s.v.Nasal, 100)
		}
		if fr.noise.f > 0 {
			s.fricRes.set(fr.noise.f, fr.noise.bw)
		}
		for n := 0; n < frameLen; n++ {
			t := float64(n) / frameLen
			f0 := fr.f0 + (next.f0-fr.f0)*t
			av := (fr.av + (next.av-fr.av)*t) * s.gv
			ah := fr.ah + (next.ah-fr.ah)*t
			af := (fr.af + (next.af-fr.af)*t) * s.gf
			if fr.f0 <= 0 || next.f0 <= 0 {
				f0 = math.Max(fr.f0, next.f0)
			}

			noise := s.rnd.gauss()
			src := s.glot.next(f0, s.openQuotient, s.jitter, &s.rnd) * av
			if s.glot.isOpen() {
				src += noise * av * s.breath * 0.6 // breathy voice: turbulence while the glottis is open
			} else {
				src += noise * av * s.lax * 0.3 // lax voice: the glottis never quite closes
			}
			s.tilt = src*(1-s.tiltCoef) + s.tilt*s.tiltCoef
			x := s.tilt + noise*ah*s.aspiration

			x = s.nasalZero.step(s.nasalPole.step(x))
			for k := len(s.cascade) - 1; k >= 0; k-- {
				x = s.cascade[k].step(x)
			}
			if af > 0 {
				fn := noise * af
				if fr.noise.f > 0 {
					x += s.fricRes.step(fn) * fr.noise.amp * 0.6
				}
				x += fn * fr.noise.bypass * 0.4
			}

			y := math.Tanh(x*s.gain) * 30000 // soft clip
			out = append(out, byte(int16(y)), byte(int16(y)>>8))
		}
	}
	return out
}
