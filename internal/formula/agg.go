package formula

import (
	"math"
	"slices"
	"strings"
)

// Agg accumule un agrégat. Comme dans Excel, une colonne entière ne compte
// que ses nombres (texte, booléens et vides ignorés), alors qu'une valeur
// passée directement est convertie (« 3 » compte, « abc » donne #VALEUR!).
// Une erreur rencontrée est propagée, sauf par NB, NBVAL et NB.VIDE.
//
// Fn est le nom anglais de l'agrégat : SUM, AVERAGE, COUNT, COUNTA,
// COUNTBLANK, MIN, MAX, MEDIAN, MODE, STDEV, STDEVP, VAR, VARP (et leurs
// formes .S et .P), PRODUCT, TEXTJOIN.
type Agg struct {
	Fn       string
	Sum      float64 // somme compensée (Neumaier) : Sum + Comp
	Comp     float64
	N        int // nombres
	Nonempty int // valeurs non vides
	Blank    int // vides et textes vides (NB.VIDE)
	Dates    int // nombres qui étaient des dates (MIN et MAX rendent alors une date)
	Min, Max float64
	Prod     float64
	Vals     []float64 // nombres gardés pour MEDIANE et les écarts types
	Items    []string  // textes de JOINDRE.TEXTE ("" pour une valeur vide)
	Err      ErrCode
}

func (a *Agg) keepsVals() bool {
	switch a.Fn {
	case "MEDIAN", "STDEV", "STDEVP", "STDEV.S", "STDEV.P", "VAR", "VARP", "VAR.S", "VAR.P", "MODE", "MODE.SNGL":
		return true
	}
	return false
}

func (a *Agg) countsErrors() bool {
	return a.Fn != "COUNT" && a.Fn != "COUNTA" && a.Fn != "COUNTBLANK"
}

// addSum ajoute à la somme compensée : la somme d'un million de parts vaut 1
// et non 1,00000000000007.
func (a *Agg) addSum(f float64) {
	t := a.Sum + f
	if math.Abs(a.Sum) >= math.Abs(f) {
		a.Comp += (a.Sum - t) + f
	} else {
		a.Comp += (f - t) + a.Sum
	}
	a.Sum = t
}

func (a *Agg) num(f float64, date bool) {
	if a.N == 0 || f < a.Min {
		a.Min = f
	}
	if a.N == 0 || f > a.Max {
		a.Max = f
	}
	if a.N == 0 {
		a.Prod = f
	} else {
		a.Prod *= f
	}
	if date {
		a.Dates++
	}
	if a.keepsVals() {
		a.Vals = append(a.Vals, f)
	}
	a.addSum(f)
	a.N++
}

func blank(v Value) bool { return v.Kind == Empty || v.Kind == Text && v.Str == "" }

// textItem ajoute une valeur à JOINDRE.TEXTE.
func (a *Agg) textItem(v Value) {
	if v.IsErr() {
		a.Err = v.Err
		return
	}
	a.Items = append(a.Items, v.String())
}

// AddRange ajoute une valeur lue dans une colonne.
func (a *Agg) AddRange(v Value) {
	if a.Err != 0 {
		return
	}
	if a.Fn == "TEXTJOIN" {
		a.textItem(v)
		return
	}
	if v.Kind != Empty {
		a.Nonempty++
	}
	if blank(v) {
		a.Blank++
	}
	switch {
	case v.Kind == Number || v.Kind == Date:
		a.num(v.Num, v.Kind == Date)
	case v.IsErr() && a.countsErrors():
		a.Err = v.Err
	}
}

// AddValue ajoute une valeur passée directement en argument.
func (a *Agg) AddValue(v Value) {
	if a.Err != 0 {
		return
	}
	switch a.Fn {
	case "TEXTJOIN":
		a.textItem(v)
		return
	case "COUNTBLANK":
		if blank(v) {
			a.Blank++
		}
		return
	}
	if v.Kind == Empty {
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
	a.num(f, v.Kind == Date)
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
		if a.N == 0 {
			a.Prod = b.Prod
		} else {
			a.Prod *= b.Prod
		}
	}
	a.addSum(b.Sum)
	a.Comp += b.Comp
	a.N += b.N
	a.Nonempty += b.Nonempty
	a.Blank += b.Blank
	a.Dates += b.Dates
	a.Vals = append(a.Vals, b.Vals...)
	a.Items = append(a.Items, b.Items...)
}

func (a *Agg) variance(sample bool) (float64, bool) {
	n := len(a.Vals)
	if n == 0 || sample && n < 2 {
		return 0, false
	}
	mean := 0.0
	for _, v := range a.Vals {
		mean += v
	}
	mean /= float64(n)
	ss := 0.0
	for _, v := range a.Vals {
		ss += (v - mean) * (v - mean)
	}
	if sample {
		return ss / float64(n-1), true
	}
	return ss / float64(n), true
}

// mode : la valeur la plus fréquente ; à égalité, celle qui apparaît la
// première (dans l'ordre du tableau). #N/A si aucune valeur ne se répète.
func (a *Agg) mode() Value {
	count := map[float64]int{}
	hits := 1
	for _, v := range a.Vals {
		count[v]++
		hits = max(hits, count[v])
	}
	if hits < 2 {
		return Err(ErrNA)
	}
	for _, v := range a.Vals {
		if count[v] == hits {
			return Num(v)
		}
	}
	return Err(ErrNA)
}

// Result donne le résultat de l'agrégat.
func (a *Agg) Result() Value {
	if a.Err != 0 {
		return Err(a.Err)
	}
	switch a.Fn {
	case "SUM":
		return number(a.Sum + a.Comp)
	case "AVERAGE":
		if a.N == 0 {
			return Err(ErrDiv0)
		}
		return number((a.Sum + a.Comp) / float64(a.N))
	case "COUNT":
		return Num(float64(a.N))
	case "COUNTA":
		return Num(float64(a.Nonempty))
	case "COUNTBLANK":
		return Num(float64(a.Blank))
	case "MIN", "MAX":
		v := a.Min
		if a.Fn == "MAX" {
			v = a.Max
		}
		if a.N > 0 && a.Dates == a.N {
			return DateSerial(v)
		}
		return Num(v)
	case "PRODUCT":
		if a.N == 0 {
			return Num(0)
		}
		return number(a.Prod)
	case "MEDIAN":
		if a.N == 0 {
			return Err(ErrNum)
		}
		s := slices.Clone(a.Vals)
		slices.Sort(s)
		if len(s)%2 == 1 {
			return Num(s[len(s)/2])
		}
		return number((s[len(s)/2-1] + s[len(s)/2]) / 2)
	case "MODE", "MODE.SNGL":
		return a.mode()
	case "STDEV", "STDEVP", "STDEV.S", "STDEV.P", "VAR", "VARP", "VAR.S", "VAR.P":
		sample := a.Fn == "STDEV" || a.Fn == "STDEV.S" || a.Fn == "VAR" || a.Fn == "VAR.S"
		v, ok := a.variance(sample)
		if !ok {
			return Err(ErrDiv0)
		}
		if strings.HasPrefix(a.Fn, "STDEV") {
			v = math.Sqrt(v)
		}
		return number(v)
	}
	return Err(ErrName)
}

// TextResult donne le résultat de JOINDRE.TEXTE(délimiteur; ignorer_vides; …).
func (a *Agg) TextResult(delim, ignore Value) Value {
	if a.Err != 0 {
		return Err(a.Err)
	}
	d, e := ToText(delim)
	if e.IsErr() {
		return e
	}
	skip, e := Truth(ignore)
	if e.IsErr() {
		return e
	}
	items := a.Items
	if skip {
		items = slices.DeleteFunc(slices.Clone(items), func(s string) bool { return s == "" })
	}
	s := strings.Join(items, d)
	if len([]rune(s)) > 32767 {
		return Err(ErrValue) // limite d'une cellule Excel
	}
	return Str(s)
}

// subtotalFn donne l'agrégat d'un code de SOUS.TOTAL (1 à 11, ou 101 à 111).
func subtotalFn(code float64) (string, bool) {
	c := int(code)
	if float64(c) != code {
		return "", false
	}
	if c > 100 {
		c -= 100
	}
	fns := []string{"", "AVERAGE", "COUNT", "COUNTA", "MAX", "MIN", "PRODUCT", "STDEV", "STDEVP", "SUM", "VAR", "VARP"}
	if c < 1 || c >= len(fns) {
		return "", false
	}
	return fns[c], true
}
