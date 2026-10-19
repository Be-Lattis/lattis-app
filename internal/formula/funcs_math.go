package formula

import "math"

// Fonctions mathématiques ajoutées pour couvrir l'usage courant d'Excel
// (LOG, trigonométrie, ARRONDI.AU.MULTIPLE, EST.PAIR…).

func numArgs(a []Value) ([]float64, Value) {
	out := make([]float64, len(a))
	for i, v := range a {
		f, e := ToNumber(v)
		if e.IsErr() {
			return nil, e
		}
		out[i] = f
	}
	return out, Value{}
}

// fnLog : LOG(nombre; [base]), base 10 si omise.
func fnLog(a []Value) Value {
	x, e := numArgs(a)
	if e.IsErr() {
		return e
	}
	base := 10.0
	if len(a) > 1 && !a[1].IsEmpty() {
		base = x[1]
	}
	if x[0] <= 0 || base <= 0 {
		return Err(ErrNum)
	}
	if base == 1 {
		return Err(ErrDiv0)
	}
	return number(math.Log(x[0]) / math.Log(base))
}

// fnMRound : ARRONDI.AU.MULTIPLE(nombre; multiple), au plus loin de zéro
// pour la moitié ; nombre et multiple de signes contraires : #NOMBRE!.
func fnMRound(a []Value) Value {
	x, e := numArgs(a)
	if e.IsErr() {
		return e
	}
	n, m := x[0], x[1]
	if m == 0 {
		return Num(0)
	}
	if n*m < 0 {
		return Err(ErrNum)
	}
	q, _ := parseNumber(FormatNumber(n / m)) // 15 chiffres, comme ARRONDI
	r, _ := parseNumber(FormatNumber(math.Round(q) * m))
	return number(r)
}

// parity : EST.PAIR et EST.IMPAIR, sur la partie entière. Un booléen donne
// #VALEUR!, comme dans Excel.
func parity(even bool) func([]Value) Value {
	return func(a []Value) Value {
		if a[0].Kind == Bool {
			return Err(ErrValue)
		}
		f, e := ToNumber(a[0])
		if e.IsErr() {
			return e
		}
		odd := math.Mod(math.Trunc(math.Abs(f)), 2) == 1
		return Boolean(odd != even)
	}
}

// fnQuotient : QUOTIENT(numérateur; dénominateur), partie entière de la
// division.
func fnQuotient(a []Value) Value {
	x, e := numArgs(a)
	if e.IsErr() {
		return e
	}
	if x[1] == 0 {
		return Err(ErrDiv0)
	}
	return number(math.Trunc(x[0] / x[1]))
}
