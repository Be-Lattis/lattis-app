package optim

import (
	"math"
	"sort"
)

// Minimize cherche un minimum de f dans la boîte [lo, hi] (±Inf : pas de
// borne) par la méthode de Nelder–Mead, à partir de x0, en maxEval
// évaluations au plus ; un point hors de la boîte est ramené sur son bord.
// f peut rendre +Inf (point sans valeur). Solution approchée : un minimum
// local, sans garantie.
func Minimize(f func([]float64) float64, x0, lo, hi []float64, maxEval int) ([]float64, float64, int) {
	n := len(x0)
	lo, hi = fill(lo, n, math.Inf(-1)), fill(hi, n, math.Inf(1))
	evals := 0
	clamp := func(x []float64) []float64 {
		for i := range x {
			x[i] = math.Min(math.Max(x[i], lo[i]), hi[i])
		}
		return x
	}
	eval := func(x []float64) float64 {
		evals++
		v := f(x)
		if math.IsNaN(v) {
			return math.Inf(1)
		}
		return v
	}
	type vertex struct {
		x []float64
		f float64
	}
	bestX := clamp(append([]float64(nil), x0...))
	bestF := eval(bestX)
	if n == 0 {
		return bestX, bestF, evals
	}
	// Deux départs : le second repart du meilleur point avec un simplexe neuf
	// (Nelder–Mead s'enlise parfois sur un simplexe aplati).
	for round := 0; round < 3 && evals < maxEval; round++ {
		simplex := []vertex{{append([]float64(nil), bestX...), bestF}}
		for i := 0; i < n; i++ {
			x := append([]float64(nil), bestX...)
			step := 0.1 * math.Max(math.Abs(x[i]), 1)
			if round > 0 {
				step *= 0.25
			}
			if x[i]+step > hi[i] {
				step = -step
			}
			x[i] += step
			clamp(x)
			simplex = append(simplex, vertex{x, eval(x)})
		}
		for evals < maxEval {
			sort.SliceStable(simplex, func(a, b int) bool { return simplex[a].f < simplex[b].f })
			b, w := simplex[0], simplex[n]
			// Convergence : valeurs et sommets resserrés.
			size := 0.0
			for _, v := range simplex[1:] {
				for i := range v.x {
					size = math.Max(size, math.Abs(v.x[i]-b.x[i])/math.Max(1, math.Abs(b.x[i])))
				}
			}
			if size < 1e-10 || (!math.IsInf(w.f, 1) && math.Abs(w.f-b.f) <= 1e-12*math.Max(1, math.Abs(b.f)) && size < 1e-6) {
				break
			}
			c := make([]float64, n)
			for _, v := range simplex[:n] {
				for i := range c {
					c[i] += v.x[i] / float64(n)
				}
			}
			at := func(t float64) []float64 {
				x := make([]float64, n)
				for i := range x {
					x[i] = c[i] + t*(w.x[i]-c[i])
				}
				return clamp(x)
			}
			xr := at(-1)
			fr := eval(xr)
			switch {
			case fr < b.f:
				xe := at(-2)
				if fe := eval(xe); fe < fr {
					simplex[n] = vertex{xe, fe}
				} else {
					simplex[n] = vertex{xr, fr}
				}
			case fr < simplex[n-1].f:
				simplex[n] = vertex{xr, fr}
			default:
				t := 0.5
				if fr < w.f {
					t = -0.5
				}
				xc := at(t)
				if fc := eval(xc); fc < math.Min(fr, w.f) {
					simplex[n] = vertex{xc, fc}
				} else {
					// Rétrécissement vers le meilleur sommet.
					for k := 1; k <= n; k++ {
						for i := range simplex[k].x {
							simplex[k].x[i] = b.x[i] + 0.5*(simplex[k].x[i]-b.x[i])
						}
						simplex[k].f = eval(clamp(simplex[k].x))
					}
				}
			}
		}
		sort.SliceStable(simplex, func(a, b int) bool { return simplex[a].f < simplex[b].f })
		if simplex[0].f <= bestF {
			bestX, bestF = simplex[0].x, simplex[0].f
		}
	}
	return bestX, bestF, evals
}
