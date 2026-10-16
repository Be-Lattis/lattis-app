package formula

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Fonctions de texte. Les positions et longueurs comptent des caractères
// (et non des octets), et les majuscules gèrent les accents.

func textArg(v Value) (string, Value) { return ToText(v) }

// countArg lit un nombre de caractères facultatif (défaut 1, négatif refusé).
func countArg(a []Value, i int, def int) (int, Value) {
	if i >= len(a) || a[i].IsEmpty() && def >= 0 {
		return def, Value{}
	}
	f, e := ToNumber(a[i])
	if e.IsErr() {
		return 0, e
	}
	if f < 0 {
		return 0, Err(ErrValue)
	}
	return int(math.Trunc(f)), Value{}
}

func fnLeft(a []Value) Value {
	s, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	n, e := countArg(a, 1, 1)
	if e.IsErr() {
		return e
	}
	r := []rune(s)
	return Str(string(r[:min(n, len(r))]))
}

func fnRight(a []Value) Value {
	s, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	n, e := countArg(a, 1, 1)
	if e.IsErr() {
		return e
	}
	r := []rune(s)
	return Str(string(r[len(r)-min(n, len(r)):]))
}

func fnMid(a []Value) Value {
	s, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	start, e := ToNumber(a[1])
	if e.IsErr() {
		return e
	}
	n, e := countArg(a, 2, 0)
	if e.IsErr() {
		return e
	}
	if start < 1 {
		return Err(ErrValue)
	}
	r := []rune(s)
	i := min(int(start)-1, len(r))
	return Str(string(r[i:min(i+n, len(r))]))
}

func fnProper(a []Value) Value {
	return textFn(a[0], func(s string) string {
		var b strings.Builder
		prev := false // le caractère précédent est une lettre
		for _, r := range s {
			if prev {
				b.WriteRune(unicode.ToLower(r))
			} else {
				b.WriteRune(unicode.ToUpper(r))
			}
			prev = unicode.IsLetter(r)
		}
		return b.String()
	})
}

// fnTrim retire les espaces au début et à la fin, et réduit les espaces
// internes à un seul, comme SUPPRESPACE.
func fnTrim(a []Value) Value {
	return textFn(a[0], func(s string) string {
		return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == ' ' }), " ")
	})
}

func fnSubstitute(a []Value) Value {
	var s [3]string
	for i := range s {
		var e Value
		if s[i], e = textArg(a[i]); e.IsErr() {
			return e
		}
	}
	text, old, repl := s[0], s[1], s[2]
	if old == "" {
		return Str(text)
	}
	if len(a) < 4 || a[3].IsEmpty() {
		return Str(strings.ReplaceAll(text, old, repl))
	}
	nf, e := ToNumber(a[3])
	if e.IsErr() {
		return e
	}
	n := int(nf)
	if n < 1 {
		return Err(ErrValue)
	}
	idx := 0
	for k := 1; ; k++ {
		j := strings.Index(text[idx:], old)
		if j < 0 {
			return Str(text)
		}
		if k == n {
			return Str(text[:idx+j] + repl + text[idx+j+len(old):])
		}
		idx += j + len(old)
	}
}

func fnReplace(a []Value) Value {
	s, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	start, e := ToNumber(a[1])
	if e.IsErr() {
		return e
	}
	n, e := countArg(a, 2, 0)
	if e.IsErr() {
		return e
	}
	repl, e := textArg(a[3])
	if e.IsErr() {
		return e
	}
	if start < 1 {
		return Err(ErrValue)
	}
	r := []rune(s)
	i := min(int(start)-1, len(r))
	j := min(i+n, len(r))
	return Str(string(r[:i]) + repl + string(r[j:]))
}

// find : TROUVE (sensible à la casse, sans joker) et CHERCHE (insensible, avec
// jokers). Position en caractères à partir de 1 ; #VALEUR! si absent.
func find(caseSensitive bool) func([]Value) Value {
	return func(a []Value) Value {
		needle, e := textArg(a[0])
		if e.IsErr() {
			return e
		}
		hay, e := textArg(a[1])
		if e.IsErr() {
			return e
		}
		from := 1
		if len(a) > 2 && !a[2].IsEmpty() {
			f, e := ToNumber(a[2])
			if e.IsErr() {
				return e
			}
			from = int(f)
		}
		r := []rune(hay)
		if from < 1 || from > len(r)+1 {
			return Err(ErrValue)
		}
		if caseSensitive {
			j := strings.Index(string(r[from-1:]), needle)
			if j < 0 {
				return Err(ErrValue)
			}
			return Num(float64(from + utf8.RuneCountInString(string(r[from-1:])[:j])))
		}
		j := searchText(needle, hay, from-1)
		if j < 0 {
			return Err(ErrValue)
		}
		return Num(float64(j + 1))
	}
}

func fnRept(a []Value) Value {
	s, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	n, e := countArg(a, 1, 0)
	if e.IsErr() {
		return e
	}
	if utf8.RuneCountInString(s)*n > 32767 {
		return Err(ErrValue)
	}
	return Str(strings.Repeat(s, n))
}

func fnExact(a []Value) Value {
	x, e := textArg(a[0])
	if e.IsErr() {
		return e
	}
	y, e := textArg(a[1])
	if e.IsErr() {
		return e
	}
	return Boolean(x == y)
}

// fnValue : CNUM convertit un texte en nombre (locale française ou point
// décimal, pourcentage, date).
func fnValue(a []Value) Value {
	v := a[0]
	switch v.Kind {
	case Number, Date:
		return Num(v.Num)
	case Empty:
		return Num(0)
	case Error:
		return v
	case Bool:
		return Err(ErrValue)
	}
	s := strings.TrimSpace(v.Str)
	if strings.HasSuffix(s, "%") {
		if f, ok := parseNumber(strings.TrimSuffix(s, "%")); ok {
			return Num(f / 100)
		}
	}
	if f, ok := parseNumber(s); ok {
		return Num(f)
	}
	if f, ok := ParseDate(s); ok {
		return Num(f)
	}
	return Err(ErrValue)
}

func fnConcat(a []Value) Value {
	out := Str("")
	for _, v := range a {
		if out = Concat(out, v); out.IsErr() {
			return out
		}
	}
	return out
}
