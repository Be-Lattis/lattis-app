package formula

import (
	"math"
	"slices"
)

// Statistiques qui parcourent une colonne (CENTILE, QUARTILE) ou deux
// colonnes ligne à ligne (COEFFICIENT.CORRELATION, PENTE, ORDONNEE.ORIGINE).
// Comme GRANDE.VALEUR, elles ne lisent que les nombres : textes, booléens et
// vides sont ignorés, une erreur est propagée.

// quantile : CENTILE(matrice; k) et QUARTILE(matrice; quart), formes
// incluse (.INCLURE, par défaut) et exclue (.EXCLURE).
type quantile struct {
	p       Value
	quart   bool // QUARTILE : p est un numéro de quartile
	exclude bool
	vals    []float64
	err     Value
	numRule
}

func newQuantile(quart, exclude bool) func([]Value) scanner {
	return func(p []Value) scanner { return &quantile{p: p[0], quart: quart, exclude: exclude} }
}

func (q *quantile) add(cols []Value) {
	switch v := q.coerce(cols[0]); v.Kind {
	case Number, Date:
		q.vals = append(q.vals, v.Num)
	case Error:
		if !q.err.IsErr() {
			q.err = v
		}
	}
}

func (q *quantile) result() Value {
	if q.err.IsErr() {
		return q.err
	}
	p, e := ToNumber(q.p)
	if e.IsErr() {
		return e
	}
	if q.quart {
		k := math.Trunc(p)
		if k < 0 || k > 4 || q.exclude && (k < 1 || k > 3) {
			return Err(ErrNum)
		}
		p = k / 4
	}
	n := float64(len(q.vals))
	if n == 0 || p < 0 || p > 1 {
		return Err(ErrNum)
	}
	slices.Sort(q.vals)
	h := (n - 1) * p // rang (à partir de 0) de la forme incluse
	if q.exclude {
		h = (n+1)*p - 1
		if h < 0 || h > n-1 {
			return Err(ErrNum)
		}
	}
	i := int(math.Floor(h))
	if i+1 >= len(q.vals) {
		return Num(q.vals[i])
	}
	return number(q.vals[i] + (h-float64(i))*(q.vals[i+1]-q.vals[i]))
}

// pairStats : COEFFICIENT.CORRELATION(matrice1; matrice2), PENTE(y; x) et
// ORDONNEE.ORIGINE(y; x). Seules comptent les lignes où les deux valeurs sont
// des nombres.
type pairStats struct {
	fn  string
	xs  []float64
	ys  []float64
	err Value
	numRule
}

func (s *pairStats) add(cols []Value) {
	if s.err.IsErr() {
		return
	}
	for _, v := range cols {
		if v.IsErr() {
			s.err = v
			return
		}
	}
	a, b := s.coerce(cols[0]), s.coerce(cols[1])
	if (a.Kind == Number || a.Kind == Date) && (b.Kind == Number || b.Kind == Date) {
		// CORREL(x; y), mais PENTE(y; x) : le premier argument est y.
		s.ys, s.xs = append(s.ys, a.Num), append(s.xs, b.Num)
	}
}

func mean(v []float64) float64 {
	t := 0.0
	for _, x := range v {
		t += x
	}
	return t / float64(len(v))
}

func (s *pairStats) result() Value {
	if s.err.IsErr() {
		return s.err
	}
	if len(s.xs) == 0 {
		return Err(ErrDiv0)
	}
	mx, my := mean(s.xs), mean(s.ys)
	var sxy, sxx, syy float64
	for i := range s.xs {
		dx, dy := s.xs[i]-mx, s.ys[i]-my
		sxy += dx * dy
		sxx += dx * dx
		syy += dy * dy
	}
	switch s.fn {
	case "CORREL":
		if sxx == 0 || syy == 0 {
			return Err(ErrDiv0)
		}
		return number(sxy / math.Sqrt(sxx*syy))
	case "SLOPE":
		if sxx == 0 {
			return Err(ErrDiv0)
		}
		return number(sxy / sxx)
	}
	if sxx == 0 {
		return Err(ErrDiv0)
	}
	return number(my - sxy/sxx*mx)
}
