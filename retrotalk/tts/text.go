package tts

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// phrase is a run of words spoken under one intonation contour, ended by punctuation.
type phrase struct {
	words []string // original spelling (case kept for acronym detection)
	end   byte     // '.', '?', '!', ',' or 0
}

var (
	reCommand  = regexp.MustCompile(`\[:[^\]]*\]`) // DECtalk-style inline commands: ignored
	reDotted   = regexp.MustCompile(`\b(?:[A-Za-z]\.){2,}`)
	reTimeAmPm = regexp.MustCompile(`(?i)\b(\d{1,2})(?::(\d{2}))?\s*([ap])\.?\s?m\b\.?`)
	reTime     = regexp.MustCompile(`\b(\d{1,2}):(\d{2})\b`)
	reMoney    = regexp.MustCompile(`\$\s?(\d[\d,]*)(?:\.(\d{2}))?`)
	reDegree   = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*°\s*(?:([FfCc])\b)?`)
	rePercent  = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)
	reOrdinal  = regexp.MustCompile(`(?i)\b(\d+)(st|nd|rd|th)\b`)
	reMinus    = regexp.MustCompile(`(^|[\s(])-(\d)`)
	reNumber   = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?`)
	reToken    = regexp.MustCompile(`[A-Za-z]+(?:'[A-Za-z]+)*|\d|[.,;:!?]`)
)

var fold = strings.NewReplacer(
	"’", "'", "‘", "'", "“", `"`, "”", `"`, "–", ", ", "—", ", ", "…", ". ",
	"&", " and ", "+", " plus ", "=", " equals ", "@", " at ", "#", " number ", "/", " slash ",
)

var accents = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'æ': "ae", 'ç': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ì': "i", 'í': "i", 'î': "i", 'ï': "i",
	'ñ': "n", 'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'œ': "oe",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ý': "y", 'ÿ': "y", 'ß': "ss",
}

var abbreviations = map[string]string{
	"mr": "mister", "mrs": "missus", "ms": "miz", "dr": "doctor", "st": "street", "jr": "junior",
	"sr": "senior", "vs": "versus", "etc": "etcetera", "approx": "approximately", "mph": "miles per hour",
	"kph": "kilometers per hour", "kmh": "kilometers per hour", "kwh": "kilowatt hours", "kw": "kilowatts",
	"ft": "feet", "lbs": "pounds", "oz": "ounces", "hr": "hour", "hrs": "hours", "eg": "for example",
	"ie": "that is",
}

// normalize rewrites symbols, numbers and abbreviations as words.
func normalize(s string) string {
	s = reCommand.ReplaceAllString(s, " ")
	s = fold.Replace(s)
	var b strings.Builder
	for _, r := range s {
		switch lr := unicode.ToLower(r); {
		case r < 128 || r == '°':
			b.WriteRune(r)
		case accents[lr] != "":
			a := accents[lr]
			if r != lr {
				a = strings.ToUpper(a)
			}
			b.WriteString(a)
		default:
			b.WriteByte(' ')
		}
	}
	s = b.String()
	s = reDotted.ReplaceAllStringFunc(s, func(m string) string { return strings.ReplaceAll(m, ".", "") + " " })
	s = reTimeAmPm.ReplaceAllStringFunc(s, func(m string) string {
		p := reTimeAmPm.FindStringSubmatch(m)
		return " " + clock(p[1], p[2], true) + " " + strings.ToUpper(p[3]) + "M " // spelled: A M / P M
	})
	s = reTime.ReplaceAllStringFunc(s, func(m string) string {
		p := reTime.FindStringSubmatch(m)
		return " " + clock(p[1], p[2], false) + " "
	})
	s = reMoney.ReplaceAllStringFunc(s, func(m string) string {
		p := reMoney.FindStringSubmatch(m)
		return " " + money(p[1], p[2]) + " "
	})
	s = reDegree.ReplaceAllStringFunc(s, func(m string) string {
		p := reDegree.FindStringSubmatch(m)
		unit := map[string]string{"f": " fahrenheit", "c": " celsius"}[strings.ToLower(p[2])]
		deg := " degrees"
		if p[1] == "1" {
			deg = " degree"
		}
		return " " + numberWords(p[1], false) + deg + unit + " "
	})
	s = rePercent.ReplaceAllStringFunc(s, func(m string) string {
		return " " + numberWords(rePercent.FindStringSubmatch(m)[1], false) + " percent "
	})
	s = reOrdinal.ReplaceAllStringFunc(s, func(m string) string {
		return " " + ordinal(cardinal(reOrdinal.FindStringSubmatch(m)[1])) + " "
	})
	s = reMinus.ReplaceAllString(s, "${1}minus $2")
	s = reNumber.ReplaceAllStringFunc(s, func(m string) string { return " " + numberWords(m, true) + " " })
	return s
}

// phrases splits normalized text into phrases of words.
func phrases(s string) []phrase {
	var out []phrase
	cur := phrase{}
	afterAbbrev := false // "Dr." : that period isn't a sentence end
	for _, tok := range reToken.FindAllString(s, -1) {
		wasAbbrev := afterAbbrev
		afterAbbrev = false
		switch c := tok[0]; {
		case strings.ContainsRune(".,;:!?", rune(c)):
			if len(cur.words) == 0 || (c == '.' && wasAbbrev) {
				continue
			}
			cur.end = map[byte]byte{'.': '.', '!': '!', '?': '?', ',': ',', ';': ',', ':': ','}[c]
			out = append(out, cur)
			cur = phrase{}
		case c >= '0' && c <= '9':
			cur.words = append(cur.words, ones[c-'0'])
		default:
			if a, ok := abbreviations[strings.ToLower(tok)]; ok && tok != strings.ToUpper(tok) {
				cur.words = append(cur.words, strings.Fields(a)...)
				afterAbbrev = true
			} else {
				cur.words = append(cur.words, tok)
			}
		}
	}
	if len(cur.words) > 0 {
		out = append(out, cur)
	}
	return out
}

// SplitPhrases returns the longest prefix of text that ends a phrase (punctuation followed
// by whitespace) and the rest, so streamed text can be spoken a phrase at a time.
func SplitPhrases(text string) (complete, rest string) {
	for i := len(text) - 2; i >= 0; i-- {
		if strings.IndexByte(".,;:!?\n", text[i]) >= 0 && unicode.IsSpace(rune(text[i+1])) {
			return text[:i+1], text[i+1:]
		}
	}
	return "", text
}

var (
	ones  = []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	tens  = []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
	scale = []string{"", "thousand", "million", "billion", "trillion"}
)

func below1000(n int) string {
	var w []string
	if n >= 100 {
		w = append(w, ones[n/100], "hundred")
		n %= 100
	}
	switch {
	case n >= 20:
		w = append(w, tens[n/10])
		if n%10 > 0 {
			w = append(w, ones[n%10])
		}
	case n > 0:
		w = append(w, ones[n])
	}
	return strings.Join(w, " ")
}

// cardinal spells a non-negative integer; very long digit strings are read digit by digit.
func cardinal(digits string) string {
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || n >= 1e15 {
		return spellDigits(digits)
	}
	if n == 0 {
		return "zero"
	}
	var parts []string
	for i := 0; n > 0; i++ {
		if g := int(n % 1000); g > 0 {
			parts = append([]string{strings.TrimSpace(below1000(g) + " " + scale[i])}, parts...)
		}
		n /= 1000
	}
	return strings.Join(parts, " ")
}

func spellDigits(d string) string {
	var w []string
	for _, c := range d {
		w = append(w, ones[c-'0'])
	}
	return strings.Join(w, " ")
}

// numberWords spells "1,234.5"; with years set, 1100-1999 and 2010-2099 read as years.
func numberWords(s string, years bool) string {
	years = years && !strings.Contains(s, ",") // 1,250 is a quantity, not a year
	s = strings.ReplaceAll(s, ",", "")
	whole, frac, _ := strings.Cut(s, ".")
	words := cardinal(whole)
	if n, err := strconv.Atoi(whole); err == nil && years && len(whole) == 4 && frac == "" &&
		((n >= 1100 && n <= 1999) || (n >= 2010 && n <= 2099)) {
		hi, lo := n/100, n%100
		switch {
		case lo == 0:
			words = below1000(hi) + " hundred"
		case lo < 10:
			words = below1000(hi) + " oh " + ones[lo]
		default:
			words = below1000(hi) + " " + below1000(lo)
		}
	}
	if frac != "" {
		words += " point " + spellDigits(frac)
	}
	return words
}

func ordinal(words string) string {
	i := strings.LastIndexByte(words, ' ') + 1
	last := words[i:]
	irregular := map[string]string{"one": "first", "two": "second", "three": "third", "five": "fifth",
		"eight": "eighth", "nine": "ninth", "twelve": "twelfth"}
	switch {
	case irregular[last] != "":
		last = irregular[last]
	case strings.HasSuffix(last, "y"):
		last = last[:len(last)-1] + "ieth"
	default:
		last += "th"
	}
	return words[:i] + last
}

func clock(h, m string, ampm bool) string {
	w := cardinal(h)
	switch {
	case m == "" || (m == "00" && ampm):
	case m == "00" && len(h) == 2 && h > "12":
		w += " hundred" // 19:00
	case m == "00":
		w += " o'clock"
	case m[0] == '0':
		w += " oh " + ones[m[1]-'0']
	default:
		w += " " + cardinal(m)
	}
	return w
}

func money(dollars, cents string) string {
	d := strings.ReplaceAll(dollars, ",", "")
	var w []string
	if d != "0" || cents == "" {
		unit := " dollars"
		if d == "1" {
			unit = " dollar"
		}
		w = append(w, cardinal(d)+unit)
	}
	if c := strings.TrimLeft(cents, "0"); c != "" {
		unit := " cents"
		if c == "1" {
			unit = " cent"
		}
		w = append(w, cardinal(c)+unit)
	}
	return strings.Join(w, " and ")
}
