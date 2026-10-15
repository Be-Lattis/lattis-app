package formula

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

type fnKind int

const (
	scalar    fnKind = iota // arguments évalués, implémentation Go commune aux deux backends
	aggregate               // arguments : colonnes entières ou valeurs
	special                 // traité par chaque backend (REFERENCE, RECHERCHEV)
)

type funcDef struct {
	EN, FR   string
	Min, Max int // Max < 0 : nombre d'arguments libre
	kind     fnKind
	impl     func([]Value) Value
}

func (f funcDef) name(l Locale) string {
	if l == EN {
		return f.EN
	}
	return f.FR
}

func (f funcDef) arity() string {
	switch {
	case f.Max < 0:
		return fmt.Sprintf("au moins %d argument(s)", f.Min)
	case f.Min == f.Max:
		return fmt.Sprintf("%d argument(s)", f.Min)
	}
	return fmt.Sprintf("de %d à %d arguments", f.Min, f.Max)
}

var funcs = []funcDef{
	{"IF", "SI", 2, 3, scalar, fnIf},
	{"IFERROR", "SIERREUR", 2, 2, scalar, func(a []Value) Value {
		if a[0].IsErr() {
			return a[1]
		}
		return a[0]
	}},
	{"IFNA", "SI.NON.DISP", 2, 2, scalar, func(a []Value) Value {
		if a[0].IsErr() && a[0].Err == ErrNA {
			return a[1]
		}
		return a[0]
	}},
	{"AND", "ET", 1, -1, scalar, func(a []Value) Value { return logic(a, true) }},
	{"OR", "OU", 1, -1, scalar, func(a []Value) Value { return logic(a, false) }},
	{"NOT", "NON", 1, 1, scalar, func(a []Value) Value {
		b, e := Truth(a[0])
		if e.IsErr() {
			return e
		}
		return Boolean(!b)
	}},
	{"ISERROR", "ESTERREUR", 1, 1, scalar, func(a []Value) Value { return Boolean(a[0].IsErr()) }},
	{"ISBLANK", "ESTVIDE", 1, 1, scalar, func(a []Value) Value { return Boolean(a[0].IsEmpty()) }},
	{"ISNUMBER", "ESTNUM", 1, 1, scalar, func(a []Value) Value { return Boolean(a[0].Kind == Number) }},
	{"ISTEXT", "ESTTEXTE", 1, 1, scalar, func(a []Value) Value { return Boolean(a[0].Kind == Text) }},
	{"ROUND", "ARRONDI", 2, 2, scalar, fnRound},
	{"ABS", "ABS", 1, 1, scalar, func(a []Value) Value {
		x, e := ToNumber(a[0])
		if e.IsErr() {
			return e
		}
		return Num(math.Abs(x))
	}},
	{"CONCAT", "CONCAT", 1, -1, scalar, func(a []Value) Value {
		out := Str("")
		for _, v := range a {
			if out = Concat(out, v); out.IsErr() {
				return out
			}
		}
		return out
	}},
	{"LEN", "NBCAR", 1, 1, scalar, func(a []Value) Value {
		s, e := ToText(a[0])
		if e.IsErr() {
			return e
		}
		return Num(float64(utf8.RuneCountInString(s)))
	}},
	{"UPPER", "MAJUSCULE", 1, 1, scalar, func(a []Value) Value { return textFn(a[0], strings.ToUpper) }},
	{"LOWER", "MINUSCULE", 1, 1, scalar, func(a []Value) Value { return textFn(a[0], strings.ToLower) }},
	{"SUM", "SOMME", 1, -1, aggregate, nil},
	{"AVERAGE", "MOYENNE", 1, -1, aggregate, nil},
	{"COUNT", "NB", 1, -1, aggregate, nil},
	{"COUNTA", "NBVAL", 1, -1, aggregate, nil},
	{"MIN", "MIN", 1, -1, aggregate, nil},
	{"MAX", "MAX", 1, -1, aggregate, nil},
	{"REFERENCE", "REFERENCE", 3, 3, special, nil},
	{"VLOOKUP", "RECHERCHEV", 3, 4, special, nil},
}

var funcIndex = func() map[string]funcDef {
	m := map[string]funcDef{}
	for _, f := range funcs {
		m[f.EN] = f
		m[f.FR] = f
	}
	return m
}()

func lookupFunc(name string) (funcDef, bool) {
	f, ok := funcIndex[strings.ToUpper(name)]
	return f, ok
}

func fnIf(a []Value) Value {
	b, e := Truth(a[0])
	if e.IsErr() {
		return e
	}
	if b {
		return a[1]
	}
	if len(a) > 2 {
		return a[2]
	}
	return Boolean(false)
}

func logic(a []Value, and bool) Value {
	for _, v := range a {
		b, e := Truth(v)
		if e.IsErr() {
			return e
		}
		if b != and {
			return Boolean(!and)
		}
	}
	return Boolean(and)
}

// fnRound arrondit « au plus loin de zéro » comme Excel, après correction de
// la représentation binaire (2,675 arrondi à 2 décimales donne 2,68).
func fnRound(a []Value) Value {
	x, e := ToNumber(a[0])
	if e.IsErr() {
		return e
	}
	d, e := ToNumber(a[1])
	if e.IsErr() {
		return e
	}
	p := math.Pow(10, math.Trunc(d))
	v := x * p
	v, _ = parseNumber(FormatNumber(v)) // 15 chiffres significatifs, comme Excel
	return number(math.Copysign(math.Floor(math.Abs(v)+0.5), v) / p)
}

func textFn(v Value, f func(string) string) Value {
	s, e := ToText(v)
	if e.IsErr() {
		return e
	}
	return Str(f(s))
}

// Agg accumule un agrégat. Comme dans Excel, une colonne entière ne compte
// que ses nombres (texte, booléens et vides ignorés), alors qu'une valeur
// passée directement est convertie (« 3 » compte, « abc » donne #VALEUR!).
// Une erreur rencontrée est propagée.
type Agg struct {
	Fn       string
	Sum      float64
	N        int // nombres
	Nonempty int // valeurs non vides
	Min, Max float64
	Err      ErrCode
}

func (a *Agg) num(f float64) {
	if a.N == 0 || f < a.Min {
		a.Min = f
	}
	if a.N == 0 || f > a.Max {
		a.Max = f
	}
	a.Sum += f
	a.N++
}

// AddRange ajoute une valeur lue dans une colonne.
func (a *Agg) AddRange(v Value) {
	if a.Err != 0 {
		return
	}
	if v.Kind != Empty {
		a.Nonempty++
	}
	switch {
	case v.Kind == Number:
		a.num(v.Num)
	case v.IsErr() && a.Fn != "COUNT" && a.Fn != "COUNTA":
		a.Err = v.Err // NB et NBVAL ne propagent pas les erreurs
	}
}

// AddValue ajoute une valeur passée directement en argument.
func (a *Agg) AddValue(v Value) {
	if a.Err != 0 || v.Kind == Empty {
		return
	}
	a.Nonempty++
	switch a.Fn {
	case "COUNTA":
		return
	case "COUNT":
		if _, e := ToNumber(v); !v.IsErr() && !e.IsErr() {
			a.N++
		}
		return
	}
	f, e := ToNumber(v)
	if e.IsErr() {
		a.Err = e.Err
		return
	}
	a.num(f)
}

// Merge combine deux accumulateurs (agrégat sur plusieurs arguments).
func (a *Agg) Merge(b Agg) {
	if a.Err != 0 {
		return
	}
	if b.Err != 0 {
		a.Err = b.Err
		return
	}
	if b.N > 0 {
		if a.N == 0 || b.Min < a.Min {
			a.Min = b.Min
		}
		if a.N == 0 || b.Max > a.Max {
			a.Max = b.Max
		}
	}
	a.Sum += b.Sum
	a.N += b.N
	a.Nonempty += b.Nonempty
}

// Result donne le résultat de l'agrégat.
func (a *Agg) Result() Value {
	if a.Err != 0 {
		return Err(a.Err)
	}
	switch a.Fn {
	case "SUM":
		return number(a.Sum)
	case "AVERAGE":
		if a.N == 0 {
			return Err(ErrDiv0)
		}
		return number(a.Sum / float64(a.N))
	case "COUNT":
		return Num(float64(a.N))
	case "COUNTA":
		return Num(float64(a.Nonempty))
	case "MIN":
		return Num(a.Min)
	case "MAX":
		return Num(a.Max)
	}
	return Err(ErrName)
}
