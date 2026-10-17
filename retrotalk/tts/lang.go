package tts

import (
	"regexp"
	"strconv"
	"strings"
)

// language bundles what changes per language: text normalization, pronunciation and a few
// acoustic habits.
type language struct {
	tag       string                     // BCP 47, as Home Assistant groups voices
	normalize func(string) string        // symbols and numbers to words
	pronounce func(word string) []string // phonemes with stress digits on vowels
	function  map[string]bool            // unstressed words
	digits    []string                   // 0-9 spoken
	aspirate  bool                       // aspirate p/t/k (English, German) or not (Spanish, French)
	reduce    float64                    // unstressed vowel duration factor; 0 = tuned English value
	english   bool                       // English abbreviations and acronym spelling apply
	wh        map[string]bool            // question words: their questions fall instead of rise
}

// Languages are the supported languages, keyed by BCP 47 tag.
var Languages = map[string]*language{}

// langAliases maps DECtalk's language codes and short tags to Languages keys.
var langAliases = map[string]string{"us": "en-US", "en": "en-US", "uk": "en-GB", "gb": "en-GB",
	"sp": "es-ES", "es": "es-ES", "la": "es-419", "gr": "de-DE", "de": "de-DE", "fr": "fr-FR"}

// LanguageTag resolves "gr", "de" or "de-DE" to a supported tag ("" if unsupported).
func LanguageTag(code string) string {
	if _, ok := Languages[code]; ok {
		return code
	}
	return langAliases[strings.ToLower(code)]
}

func langFor(tag string) *language {
	if l, ok := Languages[LanguageTag(tag)]; ok {
		return l
	}
	return Languages["en-US"]
}

func words(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

func init() {
	enWh := words("what where when why who whom whose which how")
	Languages["en-US"] = &language{tag: "en-US", normalize: normalize, pronounce: pronounce,
		function: functionWords, digits: ones[:10], aspirate: true, english: true, wh: enWh}
	Languages["en-GB"] = &language{tag: "en-GB", normalize: normalize, pronounce: britishPronounce,
		function: functionWords, digits: ones[:10], aspirate: true, english: true, wh: enWh}

	es := &spellout{units: strings.Fields(`cero uno dos tres cuatro cinco seis siete ocho nueve diez
		once doce trece catorce quince dieciséis diecisiete dieciocho diecinueve veinte veintiuno
		veintidós veintitrés veinticuatro veinticinco veintiséis veintisiete veintiocho veintinueve`),
		tens:     strings.Fields("_ _ veinte treinta cuarenta cincuenta sesenta setenta ochenta noventa"),
		tensJoin: " y ", hundreds: strings.Fields(`_ ciento doscientos trescientos cuatrocientos quinientos
		seiscientos setecientos ochocientos novecientos`), hundredExact: "cien",
		thousand: "mil", oneThousand: "mil", million: "millón", millions: "millones", oneMillion: "un millón"}
	esFunc := words("el la los las un una unos unas de del a al y o u en con por para que se lo le les su sus es me te nos mi tu")
	for _, tag := range []string{"es-ES", "es-419"} {
		castilian := tag == "es-ES"
		Languages[tag] = &language{tag: tag, function: esFunc, digits: es.units[:10], reduce: 0.8,
			wh: words("qué que dónde donde cuándo cuando por cómo como quién quien cuál cual cuánto cuánta"),
			normalize: func(s string) string {
				return normalizeOther(s, es, otherWords{point: "coma", minus: "menos", percent: "por ciento",
					degrees: "grados", and: "y", plus: "más", euros: "euros", dollars: "dólares",
					clock: func(h, m string) string {
						if m == "00" {
							return es.say(h) + " en punto"
						}
						return es.say(h) + " y " + es.say(m)
					}})
			},
			pronounce: func(w string) []string { return spanishG2P(w, castilian) }}
	}

	de := &spellout{units: strings.Fields(`null eins zwei drei vier fünf sechs sieben acht neun zehn
		elf zwölf dreizehn vierzehn fünfzehn sechzehn siebzehn achtzehn neunzehn`),
		tens:            strings.Fields("_ _ zwanzig dreißig vierzig fünfzig sechzig siebzig achtzig neunzig"),
		unitsBeforeTens: true, unitsJoin: "und", hundreds: strings.Fields(`_ einhundert zweihundert dreihundert
		vierhundert fünfhundert sechshundert siebenhundert achthundert neunhundert`),
		thousand: "tausend", oneThousand: "eintausend", million: "Million", millions: "Millionen", oneMillion: "eine Million"}
	Languages["de-DE"] = &language{tag: "de-DE", digits: de.units[:10], aspirate: true, reduce: 0.6,
		wh:       words("was wo wann warum wer wen wem wessen wie welche welcher welches woher wohin"),
		function: words("der die das den dem des ein eine einen einem einer und oder in im an am auf mit von zu zum zur ist es sie er ich wir ihr bei aus"),
		normalize: func(s string) string {
			return normalizeOther(s, de, otherWords{point: "Komma", minus: "minus", percent: "Prozent",
				degrees: "Grad", and: "und", plus: "plus", euros: "Euro", dollars: "Dollar",
				clock: func(h, m string) string {
					if m == "00" {
						return de.say(h) + " Uhr"
					}
					return de.say(h) + " Uhr " + de.say(m)
				}})
		},
		pronounce: germanG2P}

	fr := &spellout{units: strings.Fields(`zéro un deux trois quatre cinq six sept huit neuf dix onze
		douze treize quatorze quinze seize dix-sept dix-huit dix-neuf`),
		tens:     strings.Fields("_ _ vingt trente quarante cinquante soixante soixante quatre-vingt quatre-vingt"),
		tensJoin: "-", french: true, hundreds: strings.Fields(`_ cent deux-cent trois-cent quatre-cent
		cinq-cent six-cent sept-cent huit-cent neuf-cent`),
		thousand: "mille", oneThousand: "mille", million: "million", millions: "millions", oneMillion: "un million"}
	Languages["fr-FR"] = &language{tag: "fr-FR", digits: fr.units[:10], reduce: 0.8,
		wh:       words("que quoi où quand pourquoi qui comment quel quelle quels quelles combien"),
		function: words("le la les l un une des de du d et ou en au aux à que qui ne pas se ce il elle est je tu nous vous"),
		normalize: func(s string) string {
			return normalizeOther(s, fr, otherWords{point: "virgule", minus: "moins", percent: "pour cent",
				degrees: "degrés", and: "et", plus: "plus", euros: "euros", dollars: "dollars",
				clock: func(h, m string) string {
					if m == "00" {
						return fr.say(h) + " heures"
					}
					return fr.say(h) + " heures " + fr.say(m)
				}})
		},
		pronounce: frenchG2P}
}

// spellout says integers in a language whose number words follow the usual European pattern.
type spellout struct {
	units, tens, hundreds         []string // units 0..19 (or 0..29), tens 20..90, hundreds 100..900
	tensJoin, unitsJoin           string   // "treinta y uno"; German "einundzwanzig"
	unitsBeforeTens, french       bool
	hundredExact                  string // Spanish "cien"
	thousand, oneThousand         string
	million, millions, oneMillion string
}

func (s *spellout) below100(n int) string {
	if n < len(s.units) {
		return s.units[n]
	}
	t, u := n/10, n%10
	switch {
	case s.french && (t == 7 || t == 9): // soixante-dix, quatre-vingt-dix
		if u == 1 && t == 7 {
			return "soixante et onze"
		}
		return s.tens[t] + "-" + s.units[10+u]
	case u == 0:
		if s.french && t == 8 {
			return "quatre-vingts"
		}
		return s.tens[t]
	case s.unitsBeforeTens:
		unit := s.units[u]
		if u == 1 {
			unit = "ein"
		}
		return unit + s.unitsJoin + s.tens[t]
	case s.french && u == 1 && t != 8:
		return s.tens[t] + " et un"
	}
	return s.tens[t] + s.tensJoin + s.units[u]
}

func (s *spellout) below1000(n int) string {
	h, r := n/100, n%100
	var w []string
	switch {
	case h == 1 && r == 0 && s.hundredExact != "":
		return s.hundredExact
	case h > 0:
		w = append(w, s.hundreds[h])
	}
	if r > 0 || h == 0 {
		w = append(w, s.below100(r))
	}
	return strings.Join(w, " ")
}

// say spells a digit string as an integer.
func (s *spellout) say(digits string) string {
	n, err := strconv.Atoi(strings.TrimLeft(digits, "0"))
	if err != nil || n == 0 {
		if strings.Trim(digits, "0") == "" && digits != "" {
			return s.units[0]
		}
		return digits
	}
	var w []string
	if m := n / 1000000; m > 0 {
		if m == 1 {
			w = append(w, s.oneMillion)
		} else {
			w = append(w, s.below1000(m%1000)+" "+s.millions)
		}
		n %= 1000000
	}
	if t := n / 1000; t > 0 {
		if t == 1 {
			w = append(w, s.oneThousand)
		} else {
			w = append(w, s.below1000(t)+" "+s.thousand)
		}
		n %= 1000
	}
	if n > 0 {
		w = append(w, s.below1000(n))
	}
	return strings.ReplaceAll(strings.Join(w, " "), "-", " ")
}

type otherWords struct {
	point, minus, percent, degrees, and, plus, euros, dollars string
	clock                                                     func(h, m string) string
}

var (
	reMoneyAny = regexp.MustCompile(`([$€])\s?(\d[\d.,]*)|(\d[\d.,]*)\s?([$€])`)
	// Comma or point decimals: 21,5°C and 21.5 % both occur in Europe.
	reDegreeEU  = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*°\s*(?:([FfCc])\b)?`)
	rePercentEU = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*%`)
	reNumberEU  = regexp.MustCompile(`\d+(?:[.,]\d+)?`)
)

// normalizeOther is normalization for languages other than English: numbers, times,
// temperatures, money and symbols become words; accented letters are kept.
func normalizeOther(s string, sp *spellout, w otherWords) string {
	s = reCommand.ReplaceAllString(s, " ")
	s = strings.NewReplacer("’", "'", "‘", "'", "“", `"`, "”", `"`, "«", `"`, "»", `"`, "–", ", ", "—", ", ",
		"…", ". ", "&", " "+w.and+" ", "+", " "+w.plus+" ").Replace(s)
	num := func(m string) string {
		whole, frac, found := strings.Cut(strings.ReplaceAll(m, ".", ","), ",")
		if found && len(frac) == 3 { // 1.250 or 1,250 as a thousands separator
			return sp.say(whole + frac)
		}
		out := sp.say(whole)
		if found {
			var d []string
			for _, c := range frac {
				if c >= '0' && c <= '9' { // "1..2" or "1,,5": skip the stray separators
					d = append(d, sp.units[c-'0'])
				}
			}
			out += " " + w.point + " " + strings.Join(d, " ")
		}
		return out
	}
	s = reTime.ReplaceAllStringFunc(s, func(m string) string {
		p := reTime.FindStringSubmatch(m)
		return " " + w.clock(p[1], p[2]) + " "
	})
	s = reMoneyAny.ReplaceAllStringFunc(s, func(m string) string {
		p := reMoneyAny.FindStringSubmatch(m)
		amount, cur := p[2]+p[3], p[1]+p[4]
		unit := w.euros
		if cur == "$" {
			unit = w.dollars
		}
		return " " + num(amount) + " " + unit + " "
	})
	s = reMinus.ReplaceAllString(s, "${1}"+w.minus+" $2")
	s = reDegreeEU.ReplaceAllStringFunc(s, func(m string) string {
		p := reDegreeEU.FindStringSubmatch(m)
		unit := map[string]string{"f": " Fahrenheit", "c": " Celsius"}[strings.ToLower(p[2])]
		return " " + num(p[1]) + " " + w.degrees + unit + " "
	})
	s = rePercentEU.ReplaceAllStringFunc(s, func(m string) string {
		return " " + num(rePercentEU.FindStringSubmatch(m)[1]) + " " + w.percent + " "
	})
	return reNumberEU.ReplaceAllStringFunc(s, func(m string) string { return " " + num(m) + " " })
}

// stressNucleus marks one vowel primary (1) and the rest unstressed (0). idx counts vowels;
// negative counts from the end (-1 = last).
func stressNucleus(ph []string, idx int) []string {
	var vowels []int
	for i, p := range ph {
		if q := phonemes[p]; q != nil && q.flags&vowel != 0 {
			vowels = append(vowels, i)
		}
	}
	if len(vowels) == 0 {
		return ph
	}
	if idx < 0 {
		idx += len(vowels)
	}
	idx = clamp2(idx, 0, len(vowels)-1)
	for k, i := range vowels {
		if k == idx {
			ph[i] += "1"
		} else {
			ph[i] += "0"
		}
	}
	return ph
}

func clamp2(x, lo, hi int) int { return max(lo, min(hi, x)) }

// hasRunes reports whether r[i:] starts with p, without copying the rest of the word.
func hasRunes(r []rune, i int, p string) bool {
	for _, c := range p {
		if i >= len(r) || r[i] != c {
			return false
		}
		i++
	}
	return true
}

// spanishG2P: Spanish spelling is close to phonemic; stress falls on the accented vowel, else
// on the penultimate syllable for words ending in a vowel, n or s, else on the last.
func spanishG2P(w string, castilian bool) []string {
	r := []rune(strings.ToLower(w))
	at := func(i int) rune {
		if i >= 0 && i < len(r) {
			return r[i]
		}
		return 0
	}
	isV := func(c rune) bool { return strings.ContainsRune("aeiouáéíóúü", c) }
	soft := func(c rune) bool { return strings.ContainsRune("eiéí", c) }
	var out []string
	accent := -1 // vowel index carrying a written accent
	nuc := 0
	vowel := func(p string, acc bool) {
		if acc {
			accent = nuc
		}
		nuc++
		out = append(out, p)
	}
	for i := 0; i < len(r); i++ {
		c, n := r[i], at(i+1)
		switch c {
		case 'a', 'á':
			vowel("A", c == 'á')
		case 'e', 'é':
			vowel("E", c == 'é')
		case 'o', 'ó':
			vowel("O", c == 'ó')
		case 'i', 'í':
			if c == 'i' && isV(n) { // diphthong onglide: "tiene"
				out = append(out, "Y")
			} else {
				vowel("I", c == 'í')
			}
		case 'u', 'ú', 'ü':
			switch {
			case c == 'u' && (at(i-1) == 'q' || (at(i-1) == 'g' && soft(n))): // que, guerra: silent
			case c != 'ú' && isV(n): // "bueno"
				out = append(out, "W")
			default:
				vowel("U", c == 'ú')
			}
		case 'y':
			if i == len(r)-1 || !isV(n) {
				vowel("I", false)
			} else {
				out = append(out, "Y")
			}
		case 'b', 'v':
			out = append(out, "B")
		case 'c':
			switch {
			case n == 'h':
				out = append(out, "CH")
				i++
			case soft(n) && castilian:
				out = append(out, "TH")
			case soft(n):
				out = append(out, "S")
			default:
				out = append(out, "K")
			}
		case 'g':
			if soft(n) {
				out = append(out, "X")
			} else {
				out = append(out, "G")
			}
		case 'h':
		case 'j':
			out = append(out, "X")
		case 'l':
			if n == 'l' {
				out = append(out, "Y")
				i++
			} else {
				out = append(out, "L")
			}
		case 'ñ':
			out = append(out, "N", "Y")
		case 'q', 'k':
			out = append(out, "K")
		case 'r':
			if i == 0 || n == 'r' || strings.ContainsRune("nls", at(i-1)) {
				out = append(out, "RR")
				if n == 'r' {
					i++
				}
			} else {
				out = append(out, "DX")
			}
		case 'x':
			out = append(out, "K", "S")
		case 'z':
			if castilian {
				out = append(out, "TH")
			} else {
				out = append(out, "S")
			}
		case 'd', 'f', 'm', 'n', 'p', 's', 't', 'w':
			out = append(out, map[rune]string{'d': "D", 'f': "F", 'm': "M", 'n': "N", 'p': "P", 's': "S", 't': "T", 'w': "W"}[c])
		}
	}
	idx := -1
	switch last := at(len(r) - 1); {
	case accent >= 0:
		idx = accent
	case isV(last) || last == 'n' || last == 's':
		idx = -2
	}
	return stressNucleus(out, idx)
}

// germanG2P: German spelling rules for consonant clusters, final devoicing and vowel length;
// stress on the first syllable unless it is an unstressed prefix.
func germanG2P(w string) []string {
	r := []rune(strings.ToLower(w))
	at := func(i int) rune {
		if i >= 0 && i < len(r) {
			return r[i]
		}
		return 0
	}
	isV := func(c rune) bool { return strings.ContainsRune("aeiouäöüy", c) }
	has := func(i int, s string) bool { return hasRunes(r, i, s) }
	// short vowel: followed by two or more consonants (or ck, tz, ss).
	short := func(i int) bool {
		a, b := at(i+1), at(i+2)
		return a != 'h' && a != 0 && !isV(a) && b != 0 && !isV(b)
	}
	end := len(r)
	var out []string
	for i := 0; i < end; i++ {
		c, n := r[i], at(i+1)
		final := i == end-1 || (i == end-2 && (n == 's' || n == 't'))
		switch {
		case has(i, "tsch"):
			out, i = append(out, "CH"), i+3
		case has(i, "sch"):
			out, i = append(out, "SH"), i+2
		case i == 0 && (has(i, "sp") || has(i, "st")):
			out = append(out, "SH")
		case has(i, "chs"):
			out, i = append(out, "K", "S"), i+2
		case has(i, "ch"):
			if strings.ContainsRune("aou", at(i-1)) {
				out = append(out, "X")
			} else {
				out = append(out, "CX")
			}
			i++
		case has(i, "ig") && i == end-2:
			out, i = append(out, "IH", "CX"), i+1
		case has(i, "ck"):
			out, i = append(out, "K"), i+1
		case has(i, "tz"), c == 'z':
			out = append(out, "TS")
			if c == 't' {
				i++
			}
		case has(i, "pf"):
			out, i = append(out, "PF"), i+1
		case has(i, "ph"):
			out, i = append(out, "F"), i+1
		case has(i, "qu"):
			out, i = append(out, "K", "V"), i+1
		case has(i, "th"), has(i, "dt"):
			out, i = append(out, "T"), i+1
		case has(i, "ng"):
			out, i = append(out, "NG"), i+1
		case has(i, "nk"):
			out, i = append(out, "NG", "K"), i+1
		case has(i, "ie"):
			out, i = append(out, "I"), i+1
		case has(i, "ei"), has(i, "ai"):
			out, i = append(out, "AY"), i+1
		case has(i, "eu"), has(i, "äu"):
			out, i = append(out, "OY"), i+1
		case has(i, "au"):
			out, i = append(out, "AW"), i+1
		case c == 'e' && (i == end-1 || (i == end-2 && strings.ContainsRune("nlrms", n))) && i > 0:
			out = append(out, "AH0") // final -e, -en, -el, -er: schwa
			if n == 'r' {
				i++ // -er: vocalized r
			}
		case isV(c):
			long := !short(i) || n == 'h' || n == c
			q := map[rune][2]string{'a': {"A", "A"}, 'e': {"E", "EH"}, 'i': {"I", "IH"}, 'o': {"O", "AO"},
				'u': {"U", "UH"}, 'ä': {"E", "EH"}, 'ö': {"OE", "OE"}, 'ü': {"UE", "UE"}, 'y': {"UE", "UE"}}[c]
			if long {
				out = append(out, q[0])
			} else {
				out = append(out, q[1])
			}
			if n == 'h' || n == c {
				i++ // length marker
			}
		case c == 'h':
			out = append(out, "HH")
		case c == 's':
			switch {
			case n == 's' || c == 'ß':
				out = append(out, "S")
				if n == 's' {
					i++
				}
			case isV(n) && (i == 0 || isV(at(i-1))):
				out = append(out, "Z")
			default:
				out = append(out, "S")
			}
		case c == 'ß':
			out = append(out, "S")
		case c == 'r':
			if isV(n) || i == 0 {
				out = append(out, "RU")
			} else {
				out = append(out, "AH0") // vocalized r after a vowel
			}
		case c == 'b' || c == 'd' || c == 'g':
			voiced, unvoiced := map[rune]string{'b': "B", 'd': "D", 'g': "G"}[c], map[rune]string{'b': "P", 'd': "T", 'g': "K"}[c]
			if final {
				out = append(out, unvoiced) // final devoicing
			} else {
				out = append(out, voiced)
			}
		default:
			if p, ok := map[rune]string{'c': "K", 'f': "F", 'j': "Y", 'k': "K", 'l': "L", 'm': "M", 'n': "N", 'p': "P",
				't': "T", 'v': "F", 'w': "V", 'x': "KS"}[c]; ok {
				if p == "KS" {
					out = append(out, "K", "S")
				} else {
					out = append(out, p)
				}
			}
		}
	}
	// Schwas are already marked; stress the first full vowel, or the second after a prefix.
	idx := 0
	for _, p := range []string{"be", "ge", "er", "ver", "zer", "ent", "emp"} {
		if strings.HasPrefix(string(r), p) && len(r) > len(p)+2 {
			idx = 1
			break
		}
	}
	var full []string
	for _, p := range out {
		full = append(full, strings.TrimSuffix(p, "0"))
	}
	marked := stressNucleus(full, idx)
	for k, p := range out {
		if strings.HasSuffix(p, "0") {
			marked[k] = p // keep schwas unstressed
		}
	}
	return marked
}

// frenchWords are short words that don't follow the general rules.
var frenchWords = map[string]string{"est": "EH1", "les": "L EH1", "des": "D EH1", "mes": "M EH1",
	"tes": "T EH1", "ses": "S EH1", "ces": "S EH1", "et": "E1", "es": "EH1", "aux": "O1"}

// frenchG2P: silent final letters, nasal vowels, vowel digraphs; stress on the last syllable.
func frenchG2P(w string) []string {
	s := strings.ToLower(w)
	if p, ok := frenchWords[s]; ok {
		return strings.Fields(p)
	}
	short := map[string]bool{"le": true, "je": true, "me": true, "te": true, "se": true, "de": true,
		"ne": true, "que": true, "ce": true}
	switch {
	case strings.HasSuffix(s, "er") && len(s) > 3, strings.HasSuffix(s, "ez") && len(s) > 2:
		s = s[:len(s)-2] + "é"
	case s == "et":
		s = "é"
	}
	for len(s) > 2 && !short[s] && strings.ContainsAny(s[len(s)-1:], "sxtdpz") {
		s = s[:len(s)-1] // silent final consonants
	}
	r := []rune(s)
	silent := -1 // a final e is silent but stays as context: machine, garage, cuisine
	if len(s) > 2 && !short[s] && strings.HasSuffix(s, "e") {
		silent = len(r) - 1
	}
	at := func(i int) rune {
		if i >= 0 && i < len(r) {
			return r[i]
		}
		return 0
	}
	isV := func(c rune) bool { return strings.ContainsRune("aeiouyàâéèêëîïôûùœ", c) }
	has := func(i int, p string) bool { return hasRunes(r, i, p) }
	nasal := func(i int) bool { // n/m at i closes a nasal vowel (not before a vowel or doubled)
		c, n := at(i), at(i+1)
		return (c == 'n' || c == 'm') && !isV(n) && n != 'n' && n != 'm'
	}
	var out []string
	for i := 0; i < len(r); i++ {
		c, n := r[i], at(i+1)
		if i == silent {
			break
		}
		switch {
		case has(i, "ch"):
			out, i = append(out, "SH"), i+1
		case has(i, "gn"):
			out, i = append(out, "N", "Y"), i+1
		case has(i, "qu"):
			out, i = append(out, "K"), i+1
		case has(i, "ph"):
			out, i = append(out, "F"), i+1
		case has(i, "th"):
			out, i = append(out, "T"), i+1
		case has(i, "ill") && i > 0:
			out, i = append(out, "I", "Y"), i+2
		case has(i, "eau"):
			out, i = append(out, "O"), i+2
		case has(i, "au"):
			out, i = append(out, "O"), i+1
		case has(i, "ou"), has(i, "où"), has(i, "oû"):
			out, i = append(out, "U"), i+1
		case has(i, "oi"), has(i, "oy"):
			out, i = append(out, "W", "A"), i+1
		case (has(i, "ai") || has(i, "ei")) && nasal(i+2):
			out, i = append(out, "IN"), i+2
		case has(i, "ai"), has(i, "ei"), has(i, "aî"):
			out, i = append(out, "EH"), i+1
		case has(i, "eu"), has(i, "œu"):
			out, i = append(out, "OE"), i+1
		case (c == 'a' || c == 'e') && nasal(i+1):
			out, i = append(out, "AN"), i+1
		case c == 'o' && nasal(i+1):
			out, i = append(out, "ON"), i+1
		case (c == 'i' || c == 'u' || c == 'y') && nasal(i+1):
			out, i = append(out, "IN"), i+1
		case c == 'é':
			out = append(out, "E")
		case c == 'è' || c == 'ê' || c == 'ë':
			out = append(out, "EH")
		case c == 'e':
			if n != 0 && !isV(n) && !isV(at(i+2)) {
				out = append(out, "EH") // e before two consonants
			} else {
				out = append(out, "AH0")
			}
		case c == 'a' || c == 'à' || c == 'â':
			out = append(out, "A")
		case c == 'i' || c == 'î' || c == 'ï' || c == 'y':
			if isV(n) {
				out = append(out, "Y")
			} else {
				out = append(out, "I")
			}
		case c == 'o' || c == 'ô':
			out = append(out, "O")
		case c == 'u' || c == 'û' || c == 'ù':
			out = append(out, "UE")
		case c == 'h':
		case c == 'ç':
			out = append(out, "S")
		case c == 'c':
			if strings.ContainsRune("eiyéèê", n) {
				out = append(out, "S")
			} else {
				out = append(out, "K")
			}
		case c == 'g':
			if strings.ContainsRune("eiyéèê", n) {
				out = append(out, "ZH")
			} else {
				out = append(out, "G")
			}
		case c == 'j':
			out = append(out, "ZH")
		case c == 's':
			if isV(at(i-1)) && isV(n) {
				out = append(out, "Z")
			} else {
				out = append(out, "S")
			}
		case c == 'x':
			out = append(out, "K", "S")
		case c == 'r':
			out = append(out, "RU")
		case c == 'w':
			out = append(out, "V")
		default:
			if p, ok := map[rune]string{'b': "B", 'd': "D", 'f': "F", 'k': "K", 'l': "L", 'm': "M", 'n': "N",
				'p': "P", 't': "T", 'v': "V", 'z': "Z"}[c]; ok && (len(out) == 0 || out[len(out)-1] != p) {
				out = append(out, p) // doubled consonants are single
			}
		}
	}
	var full []string
	for _, p := range out {
		full = append(full, strings.TrimSuffix(p, "0"))
	}
	// Stress the last full vowel; schwas stay unstressed.
	last := -1
	for k, p := range out {
		if q := phonemes[p]; q != nil && q.flags&vowel != 0 {
			last = k
		}
	}
	marked := stressNucleus(full, -1)
	for k, p := range out {
		if strings.HasSuffix(p, "0") && k != last {
			marked[k] = p
		}
	}
	return marked
}

// britishPronounce turns the American dictionary pronunciation into southern British:
// non-rhotic r, NURSE, GOAT and LOT vowels, and broad BATH a.
func britishPronounce(word string) []string {
	us := pronounce(word)
	lw := strings.ToLower(word)
	isVowel := func(p string) bool {
		q := phonemes[strings.TrimRight(p, "012")]
		return q != nil && q.flags&vowel != 0
	}
	var out []string
	for i, p := range us {
		name, stress := strings.TrimRight(p, "012"), strings.TrimLeft(p, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
		next := ""
		if i+1 < len(us) {
			next = us[i+1]
		}
		switch {
		case name == "R" && (next == "" || !isVowel(next)):
			if len(out) > 0 {
				switch prev := strings.TrimRight(out[len(out)-1], "012"); prev {
				case "IH", "EH", "UH": // near, square, cure: centring diphthongs
					out = append(out, "AH0")
				}
			}
			continue // non-rhotic: r only before a vowel
		case name == "ER" && stress != "0":
			p = "UR" + stress
		case name == "ER":
			p = "AH0"
		case name == "OW":
			p = "OU" + stress
		case name == "AA" && strings.Contains(lw, "o") && !strings.Contains(lw, "a"):
			p = "OH" + stress // LOT
		case name == "AE" && strings.ContainsAny(lw, "a") && (next == "F" || next == "TH" || next == "S" ||
			(next == "N" && i+2 < len(us) && (us[i+2] == "T" || us[i+2] == "S" || us[i+2] == "CH"))):
			p = "AA" + stress // BATH
		}
		out = append(out, p)
	}
	return out
}
