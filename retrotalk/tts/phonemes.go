package tts

// Phoneme classes.
const (
	vowel = 1 << iota
	voiced
	stop
	fricative
	nasal
	sonorant // L R W Y
	aspirate // HH
	affricate
)

// noiseShape is the spectrum of frication or a stop burst: one resonance plus a flat bypass.
type noiseShape struct{ f, bw, amp, bypass float64 }

// phoneme holds targets for an adult male speaker (head size 100). Formants follow the
// tables in Klatt (1980) and Peterson & Barney (1952); durations (inherent, minimum; ms)
// follow Klatt's (1979) duration rules as described in Allen, Hunnicutt & Klatt (1987).
type phoneme struct {
	flags     int
	f         [3]float64 // F1-F3 targets (stops: loci)
	glide     [3]float64 // diphthongs end here; zero = no glide
	dur, min  float64
	av        float64 // voicing amplitude
	af        float64 // frication amplitude
	noise     noiseShape
	nasalZero float64 // nasal antiformant (nasals only)
}

// Noise spectra, set from tune by applyTune.
var sibilant, palatal, labioDent, dental, labial, alveolar, velar noiseShape

// Fixed spectra for the ach-Laut (velar), ich-Laut (palatal) and uvular r fricatives.
var (
	velarFric   = noiseShape{f: 1500, bw: 1200, amp: 1.2, bypass: 0.05}
	palatalFric = noiseShape{f: 3600, bw: 1500, amp: 1.4, bypass: 0.03}
	uvularFric  = noiseShape{f: 1500, bw: 1600, amp: 1.0, bypass: 0.05} // French/German r
)

// schwa replaces AH, IH and ER targets in unstressed syllables.
var schwa = [3]float64{500, 1400, 2300}

var phonemes = map[string]*phoneme{
	"IY": {flags: vowel | voiced, f: [3]float64{310, 2020, 2960}, dur: 155, min: 55, av: 1},
	"IH": {flags: vowel | voiced, f: [3]float64{400, 1800, 2570}, dur: 135, min: 40, av: 1},
	"EY": {flags: vowel | voiced, f: [3]float64{480, 1720, 2520}, glide: [3]float64{330, 2200, 2600}, dur: 190, min: 100, av: 1},
	"EH": {flags: vowel | voiced, f: [3]float64{530, 1680, 2500}, dur: 150, min: 70, av: 1},
	"AE": {flags: vowel | voiced, f: [3]float64{620, 1660, 2430}, dur: 230, min: 80, av: 1},
	"AA": {flags: vowel | voiced, f: [3]float64{700, 1220, 2600}, dur: 240, min: 100, av: 1},
	"AO": {flags: vowel | voiced, f: [3]float64{600, 990, 2570}, dur: 240, min: 100, av: 1},
	"OW": {flags: vowel | voiced, f: [3]float64{540, 1100, 2300}, glide: [3]float64{450, 900, 2300}, dur: 220, min: 80, av: 1},
	"UH": {flags: vowel | voiced, f: [3]float64{450, 1100, 2350}, dur: 160, min: 60, av: 1},
	"UW": {flags: vowel | voiced, f: [3]float64{350, 1250, 2200}, glide: [3]float64{320, 900, 2200}, dur: 210, min: 70, av: 1},
	"AH": {flags: vowel | voiced, f: [3]float64{620, 1220, 2550}, dur: 140, min: 60, av: 1},
	"ER": {flags: vowel | voiced, f: [3]float64{470, 1270, 1540}, dur: 180, min: 80, av: 1},
	"AY": {flags: vowel | voiced, f: [3]float64{660, 1200, 2550}, glide: [3]float64{400, 1880, 2500}, dur: 250, min: 150, av: 1},
	"AW": {flags: vowel | voiced, f: [3]float64{640, 1230, 2550}, glide: [3]float64{420, 940, 2350}, dur: 260, min: 100, av: 1},
	"OY": {flags: vowel | voiced, f: [3]float64{550, 960, 2400}, glide: [3]float64{360, 1820, 2450}, dur: 280, min: 150, av: 1},

	"W": {flags: sonorant | voiced, f: [3]float64{290, 610, 2150}, dur: 80, min: 60},
	"Y": {flags: sonorant | voiced, f: [3]float64{260, 2070, 3020}, dur: 80, min: 40},
	"R": {flags: sonorant | voiced, f: [3]float64{310, 1060, 1380}, dur: 80, min: 30},
	"L": {flags: sonorant | voiced, f: [3]float64{310, 1050, 2880}, dur: 80, min: 40},

	"M":  {flags: nasal | voiced, f: [3]float64{480, 1270, 2130}, dur: 70, min: 60},
	"N":  {flags: nasal | voiced, f: [3]float64{480, 1340, 2470}, dur: 60, min: 50},
	"NG": {flags: nasal | voiced, f: [3]float64{480, 2000, 2900}, dur: 95, min: 60},

	"B": {flags: stop | voiced, f: [3]float64{200, 900, 2100}, dur: 85, min: 60, af: 0.7},
	"D": {flags: stop | voiced, f: [3]float64{200, 1600, 2600}, dur: 75, min: 50, af: 0.7},
	"G": {flags: stop | voiced, f: [3]float64{200, 1990, 2850}, dur: 80, min: 60, af: 0.7},
	"P": {flags: stop, f: [3]float64{200, 900, 2100}, dur: 85, min: 50, af: 1},
	"T": {flags: stop, f: [3]float64{200, 1600, 2600}, dur: 75, min: 50, af: 1},
	"K": {flags: stop, f: [3]float64{200, 1990, 2850}, dur: 80, min: 60, af: 1},

	"F":  {flags: fricative, f: [3]float64{340, 1100, 2080}, dur: 100, min: 80, af: 0.8},
	"V":  {flags: fricative | voiced, f: [3]float64{340, 1100, 2080}, dur: 60, min: 40},
	"TH": {flags: fricative, f: [3]float64{320, 1290, 2540}, dur: 80, min: 60, af: 0.8},
	"DH": {flags: fricative | voiced, f: [3]float64{320, 1290, 2540}, dur: 50, min: 30},
	"S":  {flags: fricative, f: [3]float64{320, 1390, 2530}, dur: 105, min: 60, af: 1},
	"Z":  {flags: fricative | voiced, f: [3]float64{320, 1390, 2530}, dur: 75, min: 40},
	"SH": {flags: fricative, f: [3]float64{300, 1840, 2750}, dur: 105, min: 80, af: 1},
	"ZH": {flags: fricative | voiced, f: [3]float64{300, 1840, 2750}, dur: 70, min: 40},
	"CH": {flags: affricate, f: [3]float64{300, 1840, 2750}, dur: 70, min: 50, af: 1},
	"JH": {flags: affricate | voiced, f: [3]float64{300, 1840, 2750}, dur: 70, min: 50, af: 0.6},

	"HH": {flags: aspirate, dur: 80, min: 20},

	"_": {dur: 100, min: 100}, // silence (phonemic input)

	// Other languages. Pure (monophthong) vowels for Spanish, German and French.
	"A": {flags: vowel | voiced, f: [3]float64{750, 1350, 2500}, dur: 180, min: 80, av: 1},
	"E": {flags: vowel | voiced, f: [3]float64{420, 2000, 2600}, dur: 160, min: 70, av: 1},
	"I": {flags: vowel | voiced, f: [3]float64{290, 2250, 2950}, dur: 150, min: 60, av: 1},
	"O": {flags: vowel | voiced, f: [3]float64{440, 850, 2450}, dur: 170, min: 70, av: 1},
	"U": {flags: vowel | voiced, f: [3]float64{320, 750, 2300}, dur: 160, min: 60, av: 1},
	// Front rounded vowels (German ü/ö, French u/eu).
	"UE": {flags: vowel | voiced, f: [3]float64{290, 1650, 2100}, dur: 150, min: 60, av: 1},
	"OE": {flags: vowel | voiced, f: [3]float64{430, 1450, 2300}, dur: 160, min: 70, av: 1},
	// French nasal vowels: the nasal antiformant moves off the pole during the vowel.
	"AN": {flags: vowel | voiced, f: [3]float64{650, 1050, 2500}, dur: 200, min: 90, av: 1, nasalZero: 700},
	"ON": {flags: vowel | voiced, f: [3]float64{450, 800, 2500}, dur: 200, min: 90, av: 1, nasalZero: 650},
	"IN": {flags: vowel | voiced, f: [3]float64{600, 1600, 2500}, dur: 200, min: 90, av: 1, nasalZero: 750},
	// British vowels: LOT, NURSE, GOAT.
	"OH": {flags: vowel | voiced, f: [3]float64{580, 880, 2450}, dur: 130, min: 50, av: 1},
	"UR": {flags: vowel | voiced, f: [3]float64{500, 1450, 2400}, dur: 190, min: 90, av: 1},
	"OU": {flags: vowel | voiced, f: [3]float64{520, 1300, 2400}, glide: [3]float64{420, 1000, 2350}, dur: 210, min: 80, av: 1},
	// Fricatives: German ach-Laut (also Spanish j) and ich-Laut.
	"X":  {flags: fricative, f: [3]float64{300, 1300, 2500}, dur: 100, min: 70, af: 0.8},
	"CX": {flags: fricative, f: [3]float64{300, 2100, 2900}, dur: 100, min: 70, af: 0.8},
	// Affricates: German z/tz and pf.
	"TS": {flags: affricate, f: [3]float64{300, 1600, 2600}, dur: 90, min: 60, af: 1},
	"PF": {flags: affricate, f: [3]float64{300, 1000, 2100}, dur: 90, min: 60, af: 1},
	// Spanish tap (pero) and trill (perro); uvular r (German, French).
	"DX": {flags: stop | voiced, f: [3]float64{300, 1600, 2600}, dur: 25, min: 20, af: 0.15},
	"RR": {flags: sonorant | voiced, f: [3]float64{300, 1500, 2500}, dur: 90, min: 60},
	"RU": {flags: fricative | voiced, f: [3]float64{500, 1250, 2400}, dur: 70, min: 40}, // voiced uvular fricative (levels tuned with the other voiced fricatives)
}
