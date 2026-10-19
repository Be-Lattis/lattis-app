package formula

import (
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Fonctions de texte ajoutées : CAR, CODE, UNICAR, UNICODE, EPURAGE,
// TEXTE.AVANT, TEXTE.APRES.

// cp1252 donne les caractères 128 à 159 de la page de codes Windows-1252,
// celle de CAR et CODE dans Excel sous Windows (0 : non attribué). Les
// autres codes de 1 à 255 sont ceux d'Unicode.
var cp1252 = [32]rune{
	'€', 0, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0, 'Ž', 0,
	0, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0, 'ž', 'Ÿ',
}

// intArg lit un entier (partie entière) ; #VALEUR! hors de [lo ; hi].
func intArg(v Value, lo, hi float64) (int, Value) {
	f, e := ToNumber(v)
	if e.IsErr() {
		return 0, e
	}
	f = math.Trunc(f)
	if f < lo || f > hi {
		return 0, Err(ErrValue)
	}
	return int(f), Value{}
}

// fnChar : CAR(code), de 1 à 255 (Windows-1252).
func fnChar(a []Value) Value {
	n, e := intArg(a[0], 1, 255)
	if e.IsErr() {
		return e
	}
	r := rune(n)
	if n >= 128 && n < 160 && cp1252[n-128] != 0 {
		r = cp1252[n-128]
	}
	return Str(string(r))
}

// fnCode : CODE(texte), code Windows-1252 du premier caractère (63, « ? »,
// pour un caractère absent de cette page).
func fnCode(a []Value) Value {
	s, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	r, _ := utf8.DecodeRuneInString(s)
	switch {
	case s == "":
		return Err(ErrValue)
	case r < 128 || r >= 160 && r < 256:
		return Num(float64(r))
	}
	if i := slices.Index(cp1252[:], r); i >= 0 {
		return Num(float64(128 + i))
	}
	return Num(63)
}

// fnUnichar : UNICAR(code), caractère Unicode.
func fnUnichar(a []Value) Value {
	n, e := intArg(a[0], 1, unicode.MaxRune)
	if e.IsErr() {
		return e
	}
	if !utf8.ValidRune(rune(n)) {
		return Err(ErrValue)
	}
	return Str(string(rune(n)))
}

// fnUnicode : UNICODE(texte), code Unicode du premier caractère.
func fnUnicode(a []Value) Value {
	s, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	if s == "" {
		return Err(ErrValue)
	}
	r, _ := utf8.DecodeRuneInString(s)
	return Num(float64(r))
}

// fnClean : EPURAGE(texte) retire les caractères de contrôle (codes 0 à 31).
func fnClean(a []Value) Value {
	return textFn(a[0], func(s string) string {
		return strings.Map(func(r rune) rune {
			if r < 32 {
				return -1
			}
			return r
		}, s)
	})
}

// textAround : TEXTE.AVANT et TEXTE.APRES(texte; délimiteur; [no_instance];
// [mode_correspondance]; [fin_correspondance]; [si_non_trouvé]).
func textAround(after bool) func([]Value) Value {
	return func(a []Value) Value {
		s, e := textArg(a[0])
		if e.IsErr() {
			return e
		}
		d, e := textArg(a[1])
		if e.IsErr() {
			return e
		}
		inst := 1
		if len(a) > 2 && !a[2].IsEmpty() {
			f, e := ToNumber(a[2])
			if e.IsErr() {
				return e
			}
			inst = int(math.Trunc(f))
		}
		var noCase, atEnd bool
		for i, p := range []*bool{&noCase, &atEnd} {
			if len(a) > 3+i {
				b, e := Truth(a[3+i])
				if e.IsErr() {
					return e
				}
				*p = b
			}
		}
		text := []rune(s)
		if inst == 0 || abs(inst) > len(text) && len(text) > 0 {
			return Err(ErrValue)
		}
		hay, delim := text, []rune(d)
		if noCase {
			hay, delim = lowerRunes(text), lowerRunes(delim)
		}
		// Positions des délimiteurs, de gauche à droite, sans chevauchement.
		var pos []int
		for i := 0; i <= len(hay)-len(delim); {
			if slices.Equal(hay[i:i+len(delim)], delim) {
				pos = append(pos, i)
				i += max(len(delim), 1)
				continue
			}
			i++
		}
		size := make([]int, len(pos))
		for i := range size {
			size[i] = len(delim)
		}
		if atEnd {
			// La fin du texte (le début pour une instance négative) compte
			// comme un délimiteur.
			if inst > 0 {
				pos, size = append(pos, len(text)), append(size, 0)
			} else {
				pos, size = append([]int{0}, pos...), append([]int{0}, size...)
			}
		}
		k := inst - 1
		if inst < 0 {
			k = len(pos) + inst
		}
		if k < 0 || k >= len(pos) {
			if len(a) > 5 {
				return a[5]
			}
			return Err(ErrNA)
		}
		if after {
			return Str(string(text[pos[k]+size[k]:]))
		}
		return Str(string(text[:pos[k]]))
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func lowerRunes(r []rune) []rune {
	out := make([]rune, len(r))
	for i, c := range r {
		out[i] = unicode.ToLower(c)
	}
	return out
}
