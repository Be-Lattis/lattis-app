package formula

import (
	"math"
	"time"
)

// Fonctions courantes du tableur ajoutées à l'audit des formules du
// 30/10/2026 : FACT, COMBIN, PERMUTATION, PAIR, IMPAIR, ATAN2,
// PLAFOND.MATH, PLANCHER.MATH, ESTERR, JOURS360. Scalaires : même code pour
// les deux moteurs.

// fnFact : FACT(nombre), factorielle de la partie entière ; négatif :
// #NOMBRE!.
func fnFact(a []Value) Value {
	f, e := ToNumber(a[0])
	if e.IsErr() {
		return e
	}
	if f < 0 || f >= 171 {
		return Err(ErrNum)
	}
	r := 1.0
	for i := 2.0; i <= math.Trunc(f); i++ {
		r *= i
	}
	return number(r)
}

// combin : nombre de combinaisons (ou d'arrangements) de k parmi n, parties
// entières ; k > n ou négatif : #NOMBRE!.
func combin(perm bool) func([]Value) Value {
	return func(a []Value) Value {
		v, e := intArgs(a[:2])
		if e.IsErr() {
			return e
		}
		n, k := float64(v[0]), float64(v[1])
		if n < 0 || k < 0 || k > n {
			return Err(ErrNum)
		}
		r := 1.0
		for i := 0.0; i < k; i++ {
			r *= n - i
			if !perm {
				r /= i + 1
			}
		}
		return number(math.Round(r))
	}
}

// parityRound : PAIR / IMPAIR, arrondi en s'éloignant de zéro à l'entier pair (ou
// impair) le plus proche.
func parityRound(odd bool) func([]Value) Value {
	return func(a []Value) Value {
		f, e := ToNumber(a[0])
		if e.IsErr() {
			return e
		}
		sign := 1.0
		if f < 0 {
			sign, f = -1, -f
		}
		r := math.Ceil(f)
		if odd {
			if math.Mod(r, 2) == 0 {
				r++
			}
		} else if math.Mod(r, 2) == 1 {
			r++
		}
		return number(sign * r)
	}
}

// fnAtan2 : ATAN2(x; y), angle du point (x; y) ; (0; 0) : #DIV/0!.
func fnAtan2(a []Value) Value {
	x, e := ToNumber(a[0])
	if e.IsErr() {
		return e
	}
	y, e := ToNumber(a[1])
	if e.IsErr() {
		return e
	}
	if x == 0 && y == 0 {
		return Err(ErrDiv0)
	}
	return number(math.Atan2(y, x))
}

// mathRound : PLAFOND.MATH / PLANCHER.MATH(nombre; [précision]; [mode]).
// La précision compte en valeur absolue (1 si omise, 0 : 0) ; un nombre
// négatif s'arrondit vers zéro (PLAFOND) ou en s'en éloignant (PLANCHER),
// sens inversé par un mode non nul.
func mathRound(up bool) func([]Value) Value {
	return func(a []Value) Value {
		x, e := ToNumber(a[0])
		if e.IsErr() {
			return e
		}
		sig := 1.0
		if v := optParam(a, 1); !v.IsEmpty() {
			if sig, e = ToNumber(v); e.IsErr() {
				return e
			}
		}
		mode := 0.0
		if v := optParam(a, 2); !v.IsEmpty() {
			if mode, e = ToNumber(v); e.IsErr() {
				return e
			}
		}
		sig = math.Abs(sig)
		if sig == 0 {
			return Num(0)
		}
		q, _ := parseNumber(FormatNumber(x / sig))
		toward := up // vers +∞
		if x < 0 && mode != 0 {
			toward = !up // s'éloigne de zéro (PLAFOND) ou s'en approche (PLANCHER)
		}
		if toward {
			return number(math.Ceil(q) * sig)
		}
		return number(math.Floor(q) * sig)
	}
}

// fnIsErr : ESTERR(valeur), toute erreur sauf #N/A.
func fnIsErr(a []Value) Value {
	return Boolean(a[0].IsErr() && a[0].Err != ErrNA)
}

// fnDays360 : JOURS360(début; fin; [méthode]), jours entre deux dates sur une
// année de 360 jours. Méthode FAUX ou omise (américaine, NASD) : un début au
// dernier jour de février ou au 31 compte pour le 30 ; une fin au 31 compte
// pour le 30 si le début est au 30 ou au 31, sinon pour le 1er du mois
// suivant. VRAI (européenne) : tout 31 compte pour le 30.
func fnDays360(a []Value) Value {
	s, e := dateArg(a[0])
	if e.IsErr() {
		return e
	}
	f, e := dateArg(a[1])
	if e.IsErr() {
		return e
	}
	euro := false
	if v := optParam(a, 2); !v.IsEmpty() {
		if euro, e = Truth(v); e.IsErr() {
			return e
		}
	}
	sd, ed := s.Day(), f.Day()
	em, ey := int(f.Month()), f.Year()
	if euro {
		sd, ed = min(sd, 30), min(ed, 30)
	} else {
		if sd == 31 || s.Month() == time.February && s.AddDate(0, 0, 1).Month() == time.March {
			sd = 30
		}
		if ed == 31 {
			if sd < 30 {
				ed, em = 1, em+1
			} else {
				ed = 30
			}
		}
	}
	return Num(float64((ey-s.Year())*360 + (em-int(s.Month()))*30 + ed - sd))
}
