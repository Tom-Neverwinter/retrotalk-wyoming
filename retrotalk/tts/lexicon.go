package tts

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"strings"
	"sync"
)

// cmudict.txt.gz is the CMU Pronouncing Dictionary (BSD licence, see CMUDICT-LICENSE),
// one pronunciation per word: "word PH1 ON0 EMES".
//
//go:embed cmudict.txt.gz
var cmudictGz []byte

var (
	dictOnce sync.Once
	dict     map[string]string
)

// LoadDictionary decompresses the dictionary; call it at startup to avoid a first-use delay.
func LoadDictionary() {
	dictOnce.Do(func() {
		dict = make(map[string]string, 130000)
		zr, err := gzip.NewReader(bytes.NewReader(cmudictGz))
		if err != nil {
			panic(err) // embedded at build time; can't be corrupt unless the build is
		}
		sc := bufio.NewScanner(zr)
		for sc.Scan() {
			if w, p, ok := strings.Cut(sc.Text(), " "); ok {
				dict[w] = p
			}
		}
	})
}

var letterNames = map[byte]string{
	'a': "EY1", 'b': "B IY1", 'c': "S IY1", 'd': "D IY1", 'e': "IY1", 'f': "EH1 F", 'g': "JH IY1",
	'h': "EY1 CH", 'i': "AY1", 'j': "JH EY1", 'k': "K EY1", 'l': "EH1 L", 'm': "EH1 M", 'n': "EH1 N",
	'o': "OW1", 'p': "P IY1", 'q': "K Y UW1", 'r': "AA1 R", 's': "EH1 S", 't': "T IY1", 'u': "Y UW1",
	'v': "V IY1", 'w': "D AH1 B AH0 L Y UW0", 'x': "EH1 K S", 'y': "W AY1", 'z': "Z IY1",
}

// functionWords are never accented, whatever the dictionary says.
var functionWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a an the of to in on at by for with from and or but nor so if
		as than then is are was were be been am do does did has have had will would can could
		shall should may might must it its i me my we us our you your he him his she her they
		them their this that these those not no`) {
		functionWords[w] = true
	}
}

// pronounce returns ARPAbet phonemes with stress digits on vowels.
func pronounce(word string) []string {
	LoadDictionary()
	lw := strings.ToLower(word)
	upper := word == strings.ToUpper(word) && len(word) > 1
	// Short all-caps words are initialisms (TV, USB, AM); longer ones are read if known (NASA).
	if upper && len(word) <= 3 {
		return spell(lw)
	}
	if p, ok := dict[lw]; ok {
		return strings.Fields(p)
	}
	if upper && len(word) <= 5 {
		return spell(lw)
	}
	if base, ok := strings.CutSuffix(lw, "'s"); ok {
		if p, ok := dict[base]; ok {
			ph := strings.Fields(p)
			return append(ph, strings.Fields(pluralS(ph[len(ph)-1]))...)
		}
	}
	return letterToSound(strings.ReplaceAll(lw, "'", ""))
}

func spell(w string) []string {
	var out []string
	for i := 0; i < len(w); i++ {
		out = append(out, strings.Fields(letterNames[w[i]])...)
	}
	return out
}

func pluralS(last string) string {
	switch strings.TrimRight(last, "012") {
	case "S", "Z", "SH", "ZH", "CH", "JH":
		return "IH0 Z" // simplified; rare in practice
	case "P", "T", "K", "F", "TH":
		return "S"
	}
	return "Z"
}

// Letter-to-sound rules for words missing from the dictionary. Each rule is
// "left[match]right=PHONEMES"; the first rule for the current letter whose contexts match wins.
// Context symbols: '#' one or more vowels, ':' zero or more consonants, '^' one consonant,
// '+' a front vowel (e, i, y), '.' a voiced consonant, ' ' a word boundary.
var ltsRules = map[byte][]string{
	'a': {"[a] =AH", " [are] =AA R", "[ai]=EY", "[ay]=EY", "[au]=AO", "[aw]=AO", "[all]=AO L",
		"[alk]=AO K", "[ar]=AA R", "[a]^e =EY", "[a]^+#=EY", "[a]=AE"},
	'b': {"[bb]=B", "[b]=B"},
	'c': {"[ch]=CH", "[ck]=K", "[cc]+=K S", "[c]+=S", "[c]=K"},
	'd': {"[dd]=D", "[dg]=JH", "[d]=D"},
	'e': {"#:[e] =", " ^[e] =IY", "[e] =IY", "[ee]=IY", "[ea]=IY", "[ey] =IY", "[ey]=EY", "[ew]=UW",
		"t[ed] =IH D", "d[ed] =IH D", "#:[ed] =D", "#:[es] =Z", "c[ei]=IY", "[ei]=EY", "[er]=ER",
		"[e]^e =IY", "[e]=EH"},
	'f': {"[ff]=F", "[f]=F"},
	'g': {" [gh]=G", "[gh]=", "[gg]=G", "[g]+=JH", "[g]=G"},
	'h': {"[h] =", "[h]=HH"},
	'i': {"[igh]=AY", "[ie] =AY", "[ie]=IY", "[ir]=ER", "[ing]=IH NG", "[i]^e =AY", "[i]=IH"},
	'j': {"[j]=JH"},
	'k': {" [k]n=", "[k]=K"},
	'l': {"[ll]=L", "[l]=L"},
	'm': {"[mm]=M", "[m]=M"},
	'n': {"[ng]=NG", "[nk]=NG K", "[nn]=N", "[n]=N"},
	'o': {"[ough]=AO", "[oo]k=UH", "[oo]=UW", "[ou]=AW", "[ow] =OW", "[ow]=AW", "[oy]=OY", "[oi]=OY",
		"[oa]=OW", "[or]=AO R", "[old]=OW L D", "[o]^e =OW", "[o] =OW", "[o]=AA"},
	'p': {"[ph]=F", "[pp]=P", "[p]=P"},
	'q': {"[qu]=K W", "[q]=K"},
	'r': {"[rr]=R", "[r]=R"},
	's': {"[sh]=SH", "[sion]=ZH AH N", "[ss]=S", "#[s]#=Z", "t[s] =S", "k[s] =S", "p[s] =S",
		"f[s] =S", "#:[s] =Z", "[s]=S"},
	't': {" [the] =DH AH", "[th]=TH", "[tion]=SH AH N", "[tch]=CH", "[tt]=T", "[t]=T"},
	'u': {"[ue] =UW", "[ui]=UW", "[ur]=ER", "[u]^e =UW", "[u]=AH"},
	'v': {"[v]=V"},
	'w': {"[wh]=W", "[wr]=R", "[w]=W"},
	'x': {" [x]=Z", "[x]=K S"},
	'y': {" [y]=Y", "#:[y] =IY", "[y] =AY", "[y]#=Y", "[y]=IH"},
	'z': {"[zz]=Z", "[z]=Z"},
}

func isVowelLetter(c byte) bool { return strings.IndexByte("aeiouy", c) >= 0 }

// matchContext matches pattern against text, scanning right (dir=1) or left (dir=-1)
// from position i. It returns whether everything matched.
func matchContext(pattern, text string, i, dir int) bool {
	pi, step := 0, 1
	if dir < 0 {
		pi, step = len(pattern)-1, -1
	}
	at := func(i int) byte {
		if i < 0 || i >= len(text) {
			return ' '
		}
		return text[i]
	}
	for ; pi >= 0 && pi < len(pattern); pi += step {
		c := at(i)
		switch p := pattern[pi]; p {
		case '#':
			if !isVowelLetter(c) {
				return false
			}
			for isVowelLetter(at(i + dir)) {
				i += dir
			}
		case ':':
			for at(i) != ' ' && !isVowelLetter(at(i)) {
				i += dir
			}
			continue // consumed zero or more; don't advance again
		case '^':
			if c == ' ' || isVowelLetter(c) {
				return false
			}
		case '+':
			if c != 'e' && c != 'i' && c != 'y' {
				return false
			}
		case '.':
			if strings.IndexByte("bdvgjlmnrwz", c) < 0 {
				return false
			}
		default:
			if c != p {
				return false
			}
		}
		i += dir
	}
	return true
}

func letterToSound(w string) []string {
	var out []string
	for i := 0; i < len(w); {
		rules := ltsRules[w[i]]
		matched := false
		for _, r := range rules {
			lb, rb := strings.IndexByte(r, '['), strings.IndexByte(r, ']')
			eq := strings.IndexByte(r, '=')
			left, match, right, phones := r[:lb], r[lb+1:rb], r[rb+1:eq], r[eq+1:]
			if !strings.HasPrefix(w[i:], match) ||
				!matchContext(left, w, i-1, -1) || !matchContext(right, w, i+len(match), 1) {
				continue
			}
			out = append(out, strings.Fields(phones)...)
			i += len(match)
			matched = true
			break
		}
		if !matched {
			i++ // letters without rules (digits, symbols) are skipped
		}
	}
	// Stress: first vowel primary, the rest unstressed.
	stressed := false
	for k, p := range out {
		if ph := phonemes[p]; ph != nil && ph.flags&vowel != 0 {
			if stressed {
				out[k] = p + "0"
			} else {
				out[k] = p + "1"
				stressed = true
			}
		}
	}
	return out
}
