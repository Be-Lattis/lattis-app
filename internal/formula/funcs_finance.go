package formula

import "math"

// Fonctions financières, avec les conventions d'Excel : une somme versée
// est négative, une somme reçue positive ; type 0 (défaut) : paiements en
// fin de période, 1 : en début de période.

// finArgs lit des arguments numériques ; les facultatifs absents valent 0.
func finArgs(a []Value, n int) ([]float64, Value) {
	out := make([]float64, n)
	for i := range out {
		if i >= len(a) {
			break
		}
		f, e := ToNumber(a[i])
		if e.IsErr() {
			return nil, e
		}
		out[i] = f
	}
	return out, Value{}
}

// payType : type de paiement, 0 ou 1 (toute valeur non nulle vaut 1).
func payType(t float64) float64 {
	if t != 0 {
		return 1
	}
	return 0
}

func pmt(r, n, pv, fv, t float64) float64 {
	if r == 0 {
		return -(pv + fv) / n
	}
	g := math.Pow(1+r, n)
	return -(r * (fv + pv*g)) / ((1 + r*t) * (g - 1))
}

func fv(r, n, p, pv, t float64) float64 {
	if r == 0 {
		return -(pv + p*n)
	}
	g := math.Pow(1+r, n)
	return -(pv*g + p*(1+r*t)*(g-1)/r)
}

// fnPmt : VPM(taux; npm; va; [vc]; [type]).
func fnPmt(a []Value) Value {
	x, e := finArgs(a, 5)
	if e.IsErr() {
		return e
	}
	if x[1] == 0 {
		return Err(ErrNum)
	}
	return number(pmt(x[0], x[1], x[2], x[3], payType(x[4])))
}

// fnPv : VA(taux; npm; vpm; [vc]; [type]).
func fnPv(a []Value) Value {
	x, e := finArgs(a, 5)
	if e.IsErr() {
		return e
	}
	r, n, p, f, t := x[0], x[1], x[2], x[3], payType(x[4])
	if r == 0 {
		return number(-(f + p*n))
	}
	g := math.Pow(1+r, n)
	return number(-(f + p*(1+r*t)*(g-1)/r) / g)
}

// fnFv : VC(taux; npm; vpm; [va]; [type]).
func fnFv(a []Value) Value {
	x, e := finArgs(a, 5)
	if e.IsErr() {
		return e
	}
	return number(fv(x[0], x[1], x[2], x[3], payType(x[4])))
}

// fnNper : NPM(taux; vpm; va; [vc]; [type]).
func fnNper(a []Value) Value {
	x, e := finArgs(a, 5)
	if e.IsErr() {
		return e
	}
	r, p, pv, f, t := x[0], x[1], x[2], x[3], payType(x[4])
	if r == 0 {
		if p == 0 {
			return Err(ErrNum)
		}
		return number(-(pv + f) / p)
	}
	k := p * (1 + r*t)
	num, den := k-f*r, k+pv*r
	if den == 0 || num/den <= 0 || r <= -1 {
		return Err(ErrNum)
	}
	return number(math.Log(num/den) / math.Log(1+r))
}

// rateEq est l'équation de TAUX : nulle au taux cherché.
func rateEq(r, n, p, pv, f, t float64) float64 {
	if math.Abs(r) < 1e-12 {
		return pv + p*n + f
	}
	g := math.Pow(1+r, n)
	return pv*g + p*(1+r*t)*(g-1)/r + f
}

// fnRate : TAUX(npm; vpm; va; [vc]; [type]; [estimation]), par la méthode
// de Newton (#NOMBRE! si elle ne converge pas).
func fnRate(a []Value) Value {
	x, e := finArgs(a, 6)
	if e.IsErr() {
		return e
	}
	n, p, pv, f, t := x[0], x[1], x[2], x[3], payType(x[4])
	r := 0.1
	if len(a) > 5 && !a[5].IsEmpty() {
		r = x[5]
	}
	if n <= 0 {
		return Err(ErrNum)
	}
	return solve(r, func(r float64) float64 { return rateEq(r, n, p, pv, f, t) })
}

// solve cherche un zéro de f près de r par la méthode de Newton (dérivée
// numérique).
func solve(r float64, f func(float64) float64) Value {
	for range 100 {
		if r <= -1 {
			return Err(ErrNum)
		}
		y := f(r)
		h := 1e-7 * math.Max(1, math.Abs(r))
		d := (f(r+h) - y) / h
		if d == 0 || math.IsNaN(d) || math.IsInf(d, 0) {
			return Err(ErrNum)
		}
		next := r - y/d
		if math.Abs(next-r) < 1e-12 {
			return number(next)
		}
		r = next
	}
	return Err(ErrNum)
}

// ipmt : intérêts de la période per (de 1 à npm), comme Excel.
func ipmt(r, per, n, pv, f, t float64) (float64, float64) {
	p := pmt(r, n, pv, f, t)
	var i float64
	switch {
	case per == 1 && t == 1:
		i = 0
	case per == 1:
		i = -pv
	case t == 1:
		i = fv(r, per-2, p, pv, 1) - p
	default:
		i = fv(r, per-1, p, pv, 0)
	}
	return i * r, p
}

// fnIpmtPpmt : INTPER et PRINCPER(taux; pér; npm; va; [vc]; [type]).
func fnIpmtPpmt(principal bool) func([]Value) Value {
	return func(a []Value) Value {
		x, e := finArgs(a, 6)
		if e.IsErr() {
			return e
		}
		r, per, n, pv, f, t := x[0], x[1], x[2], x[3], x[4], payType(x[5])
		if per < 1 || per > n || n == 0 {
			return Err(ErrNum)
		}
		i, p := ipmt(r, per, n, pv, f, t)
		if principal {
			return number(p - i)
		}
		return number(i)
	}
}

// npv : VAN(taux; valeur1; …), valeurs actualisées à partir de la fin de la
// première période, dans l'ordre du tableau.
func npv(rate Value, vals []float64) Value {
	r, e := ToNumber(rate)
	if e.IsErr() {
		return e
	}
	if r == -1 {
		return Err(ErrDiv0)
	}
	t := 0.0
	for i, v := range vals {
		t += v / math.Pow(1+r, float64(i+1))
	}
	return number(t)
}

// irr : TRI(valeurs; [estimation]), taux qui annule la valeur actuelle des
// flux (le premier n'est pas actualisé). Il faut au moins un flux positif et
// un négatif.
type irr struct {
	guess Value
	vals  []float64
	err   Value
	numRule
}

func (s *irr) add(cols []Value) {
	switch v := s.coerce(cols[0]); v.Kind {
	case Number, Date:
		s.vals = append(s.vals, v.Num)
	case Error:
		if !s.err.IsErr() {
			s.err = v
		}
	}
}

func (s *irr) result() Value {
	if s.err.IsErr() {
		return s.err
	}
	r := 0.1
	if !s.guess.IsEmpty() {
		g, e := ToNumber(s.guess)
		if e.IsErr() {
			return e
		}
		r = g
	}
	pos, neg := false, false
	for _, v := range s.vals {
		pos = pos || v > 0
		neg = neg || v < 0
	}
	if !pos || !neg {
		return Err(ErrNum)
	}
	return solve(r, func(r float64) float64 {
		t := 0.0
		for i, v := range s.vals {
			t += v / math.Pow(1+r, float64(i))
		}
		return t
	})
}
