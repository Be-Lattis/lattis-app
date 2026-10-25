// Package optim rassemble les calculs numériques de l'analyse de scénarios
// (canvas/whatif.go) : recherche d'une racine pour la valeur cible
// (sécante, puis encadrement et méthode de Brent), simplexe en deux phases
// avec séparation et évaluation pour le solveur linéaire, et Nelder–Mead
// borné pour le solveur non linéaire (solution approchée).
//
// Rien ici ne connaît le classeur : les fonctions reçoivent une fonction à
// évaluer (une évaluation peut échouer : cellule en erreur).
package optim

import "math"

// RootResult : point trouvé (ou le meilleur essayé), valeur de f en ce point,
// et nombre d'évaluations.
type RootResult struct {
	X, F  float64
	Found bool
	Evals int
}

// Root cherche x tel que |f(x)| <= tol, à partir de x0, en maxEval
// évaluations au plus. ok faux : f n'a pas de valeur en x (erreur).
//
// D'abord la sécante (un problème linéaire se résout en un pas) ; dès que
// deux points encadrent un changement de signe, la méthode de Brent, qui
// converge toujours sur un intervalle encadrant ; sans encadrement après la
// sécante, une recherche de part et d'autre de x0 par pas doublés.
func Root(f func(float64) (float64, bool), x0 float64, maxEval int, tol float64) RootResult {
	if maxEval <= 0 {
		maxEval = 200
	}
	best := RootResult{X: x0, F: math.Inf(1)}
	eval := func(x float64) (float64, bool) {
		if best.Evals >= maxEval || math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, false
		}
		best.Evals++
		y, ok := f(x)
		if !ok || math.IsNaN(y) || math.IsInf(y, 0) {
			return 0, false
		}
		if math.Abs(y) < math.Abs(best.F) {
			best.X, best.F = x, y
		}
		return y, true
	}
	done := func() bool { return math.Abs(best.F) <= tol }
	finish := func() RootResult {
		best.Found = done()
		if math.IsInf(best.F, 0) {
			best.X, best.F = x0, math.NaN()
		}
		return best
	}

	f0, ok0 := eval(x0)
	if ok0 && done() {
		return finish()
	}
	h := 0.01 * math.Max(math.Abs(x0), 1)
	// 1. Sécante.
	if ok0 {
		xa, fa := x0, f0
		xb := x0 + h
		fb, ok := eval(xb)
		for i := 0; ok && i < 30 && !done(); i++ {
			if fa*fb < 0 {
				brent(eval, xa, xb, fa, fb, tol, done)
				return finish()
			}
			if fb == fa {
				break
			}
			xn := xb - fb*(xb-xa)/(fb-fa)
			// Pas démesuré : la sécante part à l'infini (fonction plate).
			if math.Abs(xn-xb) > 1e6*math.Max(math.Abs(xb), 1) {
				break
			}
			fn, okn := eval(xn)
			if !okn {
				break
			}
			xa, fa, xb, fb = xb, fb, xn, fn
		}
		if done() {
			return finish()
		}
	}
	// 2. Encadrement : pas doublés de part et d'autre de x0 (et du meilleur
	// point de la sécante).
	for _, c := range uniq(x0, best.X) {
		fc, okc := eval(c)
		if !okc {
			fc, okc = f0, ok0
		}
		prevL, prevR := c, c
		fl, fr := fc, fc
		okl, okr := okc, okc
		step := 0.1 * math.Max(math.Abs(c), 1)
		for k := 0; k < 60 && best.Evals < maxEval && !done(); k++ {
			for _, dir := range []float64{1, -1} {
				x := c + dir*step
				y, ok := eval(x)
				if !ok {
					continue
				}
				pf, pok, px := fr, okr, prevR
				if dir < 0 {
					pf, pok, px = fl, okl, prevL
				}
				if pok && pf*y < 0 {
					brent(eval, px, x, pf, y, tol, done)
					return finish()
				}
				if dir > 0 {
					prevR, fr, okr = x, y, true
				} else {
					prevL, fl, okl = x, y, true
				}
			}
			step *= 2
		}
		if done() {
			break
		}
	}
	return finish()
}

func uniq(a, b float64) []float64 {
	if a == b {
		return []float64{a}
	}
	return []float64{a, b}
}

// brent resserre [a, b], où f change de signe (méthode de Brent :
// dichotomie, sécante et interpolation quadratique inverse).
func brent(eval func(float64) (float64, bool), a, b, fa, fb, tol float64, done func() bool) {
	c, fc := b, fb
	var d, e float64
	for i := 0; i < 200 && !done(); i++ {
		if (fb > 0 && fc > 0) || (fb < 0 && fc < 0) {
			c, fc = a, fa
			d = b - a
			e = d
		}
		if math.Abs(fc) < math.Abs(fb) {
			a, b, c = b, c, b
			fa, fb, fc = fb, fc, fb
		}
		tol1 := 2*1e-15*math.Abs(b) + 1e-300
		xm := 0.5 * (c - b)
		if math.Abs(xm) <= tol1 || fb == 0 {
			return
		}
		if math.Abs(e) >= tol1 && math.Abs(fa) > math.Abs(fb) {
			s := fb / fa
			var p, q float64
			if a == c {
				p = 2 * xm * s
				q = 1 - s
			} else {
				q = fa / fc
				r := fb / fc
				p = s * (2*xm*q*(q-r) - (b-a)*(r-1))
				q = (q - 1) * (r - 1) * (s - 1)
			}
			if p > 0 {
				q = -q
			}
			p = math.Abs(p)
			if 2*p < math.Min(3*xm*q-math.Abs(tol1*q), math.Abs(e*q)) {
				e = d
				d = p / q
			} else {
				d = xm
				e = d
			}
		} else {
			d = xm
			e = d
		}
		a, fa = b, fb
		if math.Abs(d) > tol1 {
			b += d
		} else {
			b += math.Copysign(tol1, xm)
		}
		y, ok := eval(b)
		if !ok {
			return
		}
		fb = y
	}
}
