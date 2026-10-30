package formula

import (
	"math"
	"regexp"
	"sync"
)

// Expressions régulières (audit des formules du 30/10/2026) : REGEXTEST,
// REGEXEXTRACT et REGEXREPLACE, sous le même nom dans toutes les langues
// comme dans le tableur. Syntaxe RE2 de Go : celle du tableur (PCRE2) sans
// les assertions avant / arrière ni les références arrière (\1), qui
// donnent #VALEUR!. Fonctions scalaires : même code pour les deux moteurs.

// regexCache garde les motifs compilés (une colonne calculée applique le
// même motif à chaque ligne) ; vidé au-delà de regexCacheMax motifs.
var (
	regexMu    sync.Mutex
	regexCache = map[string]*regexp.Regexp{}
)

const regexCacheMax = 256

// regexArg compile le motif ; sensibilité 0 (ou omise) : respecte la casse,
// 1 : l'ignore. Motif invalide ou sensibilité inconnue : #VALEUR!.
func regexArg(pat, sens Value) (*regexp.Regexp, Value) {
	p, e := textArg(pat)
	if e.IsErr() {
		return nil, e
	}
	if !sens.IsEmpty() {
		f, e := ToNumber(sens)
		if e.IsErr() {
			return nil, e
		}
		switch math.Trunc(f) {
		case 0:
		case 1:
			p = "(?i)" + p
		default:
			return nil, Err(ErrValue)
		}
	}
	regexMu.Lock()
	defer regexMu.Unlock()
	if re, ok := regexCache[p]; ok {
		return re, Value{}
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, Err(ErrValue)
	}
	if len(regexCache) >= regexCacheMax {
		clear(regexCache)
	}
	regexCache[p] = re
	return re, Value{}
}

// fnRegexTest : REGEXTEST(texte; motif; [sensibilité]).
func fnRegexTest(a []Value) Value {
	s, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	re, e := regexArg(a[1], optParam(a, 2))
	if e.IsErr() {
		return e
	}
	return Boolean(re.MatchString(s))
}

// fnRegexExtract : REGEXEXTRACT(texte; motif; [mode]; [sensibilité]). Mode
// 0 : la première correspondance (#N/A sans correspondance). Les modes 1
// (toutes) et 2 (groupes) rendent un tableau, que le moteur ne déverse pas
// encore : #VALEUR!.
func fnRegexExtract(a []Value) Value {
	s, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	re, e := regexArg(a[1], optParam(a, 3))
	if e.IsErr() {
		return e
	}
	if mode, e := modeArg(optParam(a, 2), 0, 0); e.IsErr() || mode != 0 {
		return Err(ErrValue)
	}
	loc := re.FindStringIndex(s)
	if loc == nil {
		return Err(ErrNA)
	}
	return Str(s[loc[0]:loc[1]])
}

// dollarGroup : $1 à $99 dans un remplacement, écrits ${1} pour Go (où $1a
// désignerait le groupe nommé « 1a »).
var dollarGroup = regexp.MustCompile(`\$(\d{1,2})`)

// fnRegexReplace : REGEXREPLACE(texte; motif; remplacement; [occurrence];
// [sensibilité]). Occurrence 0 (ou omise) : toutes ; n : la n-ième ;
// négative : en partant de la fin. $1, $2… reprennent les groupes.
func fnRegexReplace(a []Value) Value {
	s, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	re, e := regexArg(a[1], optParam(a, 4))
	if e.IsErr() {
		return e
	}
	repl, e := textArg(a[2])
	if e.IsErr() {
		return e
	}
	repl = dollarGroup.ReplaceAllString(repl, "$${$1}")
	n := 0
	if v := optParam(a, 3); !v.IsEmpty() {
		f, e := ToNumber(v)
		if e.IsErr() {
			return e
		}
		n = int(math.Trunc(f))
	}
	matches := re.FindAllStringSubmatchIndex(s, -1)
	if n < 0 {
		n += len(matches) + 1
		if n <= 0 {
			return Str(s)
		}
	}
	out := make([]byte, 0, len(s))
	at := 0
	for i, m := range matches {
		if n != 0 && i+1 != n {
			continue
		}
		out = append(out, s[at:m[0]]...)
		out = re.ExpandString(out, repl, s, m)
		at = m[1]
	}
	return Str(string(append(out, s[at:]...)))
}
