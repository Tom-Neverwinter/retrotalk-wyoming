package tts

import "fmt"

// tune holds the engine's acoustic constants in one place so they can be optimized against
// speech recognizers (TestWriteSamples reads overrides from $TTS_TUNE). The values here are
// the defaults the engine ships with.
var tune = map[string]float64{
	// Frication and burst spectra: center, bandwidth, level (bandpass) and flat bypass.
	"sib_f": 5720, "sib_bw": 960, "sib_amp": 2.24, "sib_bypass": 0.0275, // s z
	"pal_f": 3500, "pal_bw": 600, "pal_amp": 1.65, "pal_bypass": 0.03, // sh zh ch jh
	"labdent_bypass": 0.192,                     // f v
	"dent_amp":       0.08, "dent_bypass": 0.12, // th dh
	"burst_lab": 0.8, "burst_alv": 2.2, "burst_vel": 3.2, // p b / t d / k g
	"asp_gain": 0.176, // aspiration noise level

	// Voicing levels.
	"vfric_av":    0.144, // z v dh zh: low murmur under the noise
	"vfric_af":    0.68,  // z v dh zh: noise level
	"voicebar_av": 0.0864,
	"nasal_av":    1.2, "nasal_zero": 792,
	"sonorant_av": 0.8,

	// Timing (Klatt-style duration percentages) and voice onset time.
	"unstressed_pct": 0.375, "cluster_pct": 0.896, "final_pct": 1.4, "vot_scale": 0.75,

	// Formant transition half-lengths, in 5 ms frames.
	"spread_vowel": 7, "spread_stop": 5, "spread_sonorant": 10,

	// Unstressed IH target (reduced but front).
	"ih0_f1": 420, "ih0_f2": 1750,
}

// applyTune copies tune into the noise shapes and phoneme table.
func applyTune() {
	t := tune
	sibilant = noiseShape{f: t["sib_f"], bw: t["sib_bw"], amp: t["sib_amp"], bypass: t["sib_bypass"]}
	palatal = noiseShape{f: t["pal_f"], bw: t["pal_bw"], amp: t["pal_amp"], bypass: t["pal_bypass"]}
	labioDent = noiseShape{bypass: t["labdent_bypass"]}
	dental = noiseShape{f: 6000, bw: 2000, amp: t["dent_amp"], bypass: t["dent_bypass"]}
	labial = noiseShape{bypass: t["burst_lab"]}
	alveolar = noiseShape{f: 4000, bw: 1500, amp: t["burst_alv"]}
	velar = noiseShape{f: 1900, bw: 600, amp: t["burst_vel"]}
	for name, p := range phonemes {
		switch {
		case p.flags&nasal != 0:
			p.av, p.nasalZero = t["nasal_av"], t["nasal_zero"]
			if name == "NG" {
				p.nasalZero *= 2 // velar nasal: antiformant sits higher than for m, n
			}
		case p.flags&sonorant != 0:
			p.av = t["sonorant_av"]
		case p.flags&(fricative|voiced) == fricative|voiced:
			p.av, p.af = t["vfric_av"], t["vfric_af"]
		}
		switch name {
		case "S", "Z", "TS":
			p.noise = sibilant
		case "SH", "ZH", "CH", "JH":
			p.noise = palatal
		case "F", "V":
			p.noise = labioDent
		case "TH", "DH":
			p.noise = dental
		case "P", "B":
			p.noise = labial
		case "T", "D":
			p.noise = alveolar
		case "X":
			p.noise = velarFric
		case "CX":
			p.noise = palatalFric
		case "RU":
			p.noise = uvularFric
		case "PF":
			p.noise = labioDent
		case "DX":
			p.noise = alveolar
		case "K", "G":
			p.noise = velar
		}
	}
}

func init() { applyTune() }

// Override replaces tune constants (for example the result of an optimization run) and
// reapplies them. Call it before synthesizing; it is not safe during synthesis.
func Override(m map[string]float64) error {
	for k := range m {
		if _, ok := tune[k]; !ok {
			return fmt.Errorf("unknown tune constant %q", k)
		}
	}
	for k, v := range m {
		tune[k] = v
	}
	applyTune()
	return nil
}
