package tts

import (
	"math"
	"strings"
)

// seg is one phoneme in context, after duration and pitch rules.
type seg struct {
	p        *phoneme
	name     string
	stress   int // vowels: 0, 1 or 2; consonants: -1
	function bool
	dur      float64 // ms
	f0       float64 // pitch target at the middle of a vowel, Hz
}

// segments turns a phrase into phonemes with stress.
func segments(ph phrase) []seg {
	var out []seg
	for _, w := range ph.words {
		fn := functionWords[strings.ToLower(w)]
		for _, p := range pronounce(w) {
			name := strings.TrimRight(p, "012")
			pp := phonemes[name]
			if pp == nil {
				continue
			}
			stress := -1
			if pp.flags&vowel != 0 {
				stress = 0
				if d := p[len(p)-1]; d == '1' || d == '2' {
					stress = int(d - '0')
				}
				if fn {
					stress = 0
				}
			}
			out = append(out, seg{p: pp, name: name, stress: stress, function: fn})
		}
	}
	return out
}

// timeAndPitch applies Klatt-style duration rules and a hat-pattern pitch contour.
func timeAndPitch(segs []seg, end byte, v Voice) {
	lastVowel := -1
	for i, s := range segs {
		if s.stress >= 0 {
			lastVowel = i
		}
	}
	tempo := 180 / v.Rate
	final := end != 0
	for i := range segs {
		s := &segs[i]
		pct := 1.0
		if s.stress >= 0 {
			if s.stress == 0 {
				pct *= 0.5
			}
		} else if (i > 0 && segs[i-1].stress < 0) || (i+1 < len(segs) && segs[i+1].stress < 0) {
			pct *= 0.7 // consonant cluster
		}
		if final && i >= lastVowel && lastVowel >= 0 {
			pct *= 1.4 // phrase-final lengthening
		}
		s.dur = (s.p.min + (s.p.dur-s.p.min)*pct) * tempo
	}

	// Pitch: declining baseline, accents on stressed content words, phrase-final contour.
	total := 0.0
	for _, s := range segs {
		total += s.dur
	}
	rng := v.PitchRange / 100
	t, accents := 0.0, 0
	for i := range segs {
		s := &segs[i]
		mid := (t + s.dur/2) / math.Max(total, 1)
		t += s.dur
		if s.stress < 0 {
			continue
		}
		base := v.Pitch * (1 + (0.06-0.16*mid)*rng)
		switch {
		case s.stress == 1 && !s.function:
			lift := 0.22
			if accents == 0 {
				lift = 0.3 // first accent is the highest (hat rise)
			}
			accents++
			base += v.Pitch * lift * rng
		case s.stress == 2:
			base += v.Pitch * 0.08 * rng
		}
		if i == lastVowel {
			switch end {
			case '?':
				base = v.Pitch * (1 + 0.45*rng)
			case ',':
				base = v.Pitch * (1 + 0.05*rng)
			default:
				base = v.Pitch * (1 - (0.1+0.25*v.Assert/100)*rng)
			}
		}
		s.f0 = base
	}
}

// pauseMs is the silence after a phrase.
func pauseMs(end byte) float64 {
	switch end {
	case ',':
		return 180
	case 0:
		return 60
	}
	return 380
}

func lerp3(a, b [3]float64, t float64) [3]float64 {
	return [3]float64{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

// frames lays segments out as 5 ms synthesizer frames with smoothed transitions.
func frames(segs []seg, end byte, v Voice) []frame {
	var out []frame
	targetOf := func(s seg) [3]float64 {
		if s.stress == 0 && (s.name == "AH" || s.name == "IH" || s.name == "ER") {
			if s.name == "ER" {
				return [3]float64{470, 1270, 1600}
			}
			return schwa
		}
		return s.p.f
	}
	blank := func() frame {
		return frame{f: [5]float64{500, 1500, 2500, 3300, 3850}, bw: [5]float64{80, 100, 150, 250, 200}}
	}
	out = append(out, blank(), blank()) // short lead-in
	starts := []int{0}                  // first frame of each segment, for transitions
	vot := 0                            // aspirated frames owed to this segment by a preceding p/t/k
	for i, s := range segs {
		starts = append(starts, len(out))
		n := max(2, int(math.Round(s.dur/frameMs)))
		devoice := min(vot, n*3/5)
		vot = 0
		next := seg{p: &phoneme{}}
		if i+1 < len(segs) {
			next = segs[i+1]
		}
		nextVoiced := next.p.flags&(vowel|sonorant|nasal) != 0
		if s.p.flags&(stop|voiced) == stop && nextVoiced {
			// Voice onset time: aspiration devoices the start of the next sound (Klatt).
			ms := map[string]float64{"P": 55, "T": 70, "K": 80}[s.name] * 180 / v.Rate
			if next.stress == 0 {
				ms /= 2
			}
			vot = int(ms / frameMs)
		}
		locus, noise := targetOf(s), s.p.noise
		if s.name == "K" || s.name == "G" {
			// Velars take their place from the neighbouring vowel: F2 high before "ee",
			// low before "ah"/"oo", with F3 close above it; the burst sits at that F2.
			f2 := 1800.0
			if vw, ok := nearVowel(segs, i); ok {
				f2 = clamp(targetOf(vw)[1]+150, 1300, 2300)
			}
			locus = [3]float64{200, f2, f2 + 500}
			noise.f = f2
		}
		for k := 0; k < n; k++ {
			fr := blank()
			fr.spread = 4
			if s.p.flags&stop != 0 {
				fr.spread = 5 // stop place cues live in the formant transitions
			}
			if s.p.flags&sonorant != 0 {
				fr.spread = 14 // glides and liquids move slowly
			}
			pos := float64(k) / float64(n)
			tg := locus
			if s.p.glide[0] > 0 {
				tg = lerp3(tg, s.p.glide, clamp((pos-0.35)/0.55, 0, 1)) // hold the onset, then glide
			}
			switch {
			case s.p.flags&aspirate != 0:
				tg = targetOf(next) // [h] takes the shape of the following vowel
				fr.ah = 0.9
			case s.p.flags&stop != 0:
				bl := map[string]int{"P": 1, "B": 1, "T": 2, "D": 2}[s.name] // burst: labial 5 ms, alveolar 10, velar 15
				if bl == 0 {
					bl = 3
				}
				if k < max(n-bl, 1) { // closure
					if s.p.flags&voiced != 0 {
						fr.av = 0.18                                  // voice bar: low murmur only,
						fr.bw = [5]float64{80, 700, 1000, 1000, 1000} // walls of the closed tract damp F2 and up
					}
				} else {
					fr.af, fr.noise = s.p.af, noise
				}
			case s.p.flags&affricate != 0:
				if k >= n*9/20 { // closure, then an abrupt frication onset
					fr.af, fr.noise = s.p.af, s.p.noise
					if s.p.flags&voiced != 0 {
						fr.av = 0.4
					}
				}
			default:
				fr.av, fr.af, fr.noise = s.p.av, s.p.af, s.p.noise
				if s.stress == 0 {
					fr.av *= 0.85
				}
			}
			if k < devoice {
				fr.av, fr.ah = 0, 1
			}
			fr.f[0], fr.f[1], fr.f[2] = tg[0], tg[1], tg[2]
			fr.nasalZero = s.p.nasalZero
			if s.p.flags&nasal != 0 {
				fr.bw[0] = 100
			}
			fr.f0 = s.f0 // vowels only for now; filled in below
			out = append(out, fr)
		}
	}
	// Silence holds the neighbouring formants so fading voicing doesn't glide into a vowel.
	tail := max(4, int(pauseMs(end)/frameMs))
	starts = append(starts, len(out))
	for k := 0; k < tail; k++ {
		fr := blank()
		fr.f = out[len(out)-1].f
		out = append(out, fr)
	}
	for k := 0; k < 2; k++ {
		out[k].f = out[2].f
	}
	transitions(out, starts)
	smoothAmplitudes(out)
	pitchTrack(out, v)
	return out
}

// nearVowel finds the vowel a consonant at i coarticulates with: the next one within two
// segments, else the previous one.
func nearVowel(segs []seg, i int) (seg, bool) {
	for j := i + 1; j < len(segs) && j <= i+2; j++ {
		if segs[j].stress >= 0 {
			return segs[j], true
		}
	}
	for j := i - 1; j >= 0; j-- {
		if segs[j].stress >= 0 {
			return segs[j], true
		}
	}
	return seg{}, false
}

// transitions draws linear formant transitions across each segment boundary (Klatt's
// target/transition model). A transition lasts the longer of the two segments' spreads but
// never more than half of either, so every segment still reaches its own target.
func transitions(fr []frame, starts []int) {
	src := make([][3]float64, len(fr))
	for i := range fr {
		src[i] = [3]float64{fr[i].f[0], fr[i].f[1], fr[i].f[2]}
	}
	for b := 1; b < len(starts); b++ {
		B := starts[b]
		end := len(fr)
		if b+1 < len(starts) {
			end = starts[b+1]
		}
		if B <= 0 || B >= len(fr) {
			continue
		}
		half := max(fr[B-1].spread, fr[B].spread)
		l, r := min(half, (B-starts[b-1])/2), min(half, (end-B)/2)
		a, z := B-l, B+r // fr[a-1] and fr[z] keep their targets; frames between move linearly
		if a < 1 || z >= len(fr) || z <= a {
			continue
		}
		from, to := src[a-1], src[z]
		for j := a; j < z; j++ {
			t := float64(j-a+1) / float64(z-a+1)
			for d := 0; d < 3; d++ {
				fr[j].f[d] = from[d] + (to[d]-from[d])*t
			}
		}
	}
}

// smoothAmplitudes ramps source amplitudes over ~10 ms to avoid clicks.
func smoothAmplitudes(fr []frame) {
	type amp struct{ av, ah, af float64 }
	src := make([]amp, len(fr))
	for i, f := range fr {
		src[i] = amp{f.av, f.ah, f.af}
	}
	for i := range fr {
		a, b, c := src[max(i-1, 0)], src[i], src[min(i+1, len(fr)-1)]
		fr[i].av = (a.av + 2*b.av + c.av) / 4
		fr[i].ah = (a.ah + 2*b.ah + c.ah) / 4
		fr[i].af = max(b.af, (a.af+2*b.af+c.af)/4) // keep bursts crisp
	}
}

// pitchTrack interpolates between vowel pitch targets and smooths by quickness.
func pitchTrack(fr []frame, v Voice) {
	var anchors []int
	for i := range fr {
		if fr[i].f0 > 0 {
			anchors = append(anchors, i)
		}
	}
	if len(anchors) == 0 {
		for i := range fr {
			fr[i].f0 = v.Pitch
		}
	} else {
		raw := make([]float64, len(fr))
		for i := range fr {
			raw[i] = fr[i].f0
		}
		a := 0
		for i := range fr {
			for a+1 < len(anchors) && anchors[a+1] <= i {
				a++
			}
			switch {
			case i <= anchors[0]:
				fr[i].f0 = raw[anchors[0]]
			case a+1 >= len(anchors):
				fr[i].f0 = raw[anchors[len(anchors)-1]]
			default:
				l, r := anchors[a], anchors[a+1]
				fr[i].f0 = raw[l] + (raw[r]-raw[l])*float64(i-l)/float64(r-l)
			}
		}
	}
	// One-pole smoothing: lower quickness = lazier pitch.
	k := 0.15 + 0.6*v.Quick/100
	p := fr[0].f0
	for i := range fr {
		p += (fr[i].f0 - p) * k
		fr[i].f0 = p
	}
}
