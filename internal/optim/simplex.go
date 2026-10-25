package optim

import "math"

// Op : sens d'une contrainte linéaire.
type Op int

const (
	LE Op = iota // <=
	GE           // >=
	EQ           // =
)

// Status : issue d'une résolution.
type Status int

const (
	Optimal Status = iota
	Infeasible
	Unbounded
	// Limit : nombre de nœuds (variables entières) ou d'itérations dépassé ;
	// la meilleure solution trouvée est rendue, s'il y en a une.
	Limit
)

// LP : programme linéaire. Maximize ou minimiser C·x, sous A[i]·x Ops[i]
// B[i], Lo <= x <= Hi (±Inf : pas de borne), Int[j] : x[j] entier.
type LP struct {
	Maximize bool
	C        []float64
	A        [][]float64
	Ops      []Op
	B        []float64
	Lo, Hi   []float64
	Int      []bool
}

const eps = 1e-9

// SolveLP résout le programme : simplexe en deux phases (règle de Bland,
// sans cyclage), puis séparation et évaluation pour les variables entières.
func SolveLP(p LP) ([]float64, float64, Status) {
	n := len(p.C)
	lo, hi := fill(p.Lo, n, math.Inf(-1)), fill(p.Hi, n, math.Inf(1))
	for j := 0; j < n; j++ {
		if j < len(p.Int) && p.Int[j] {
			lo[j], hi[j] = math.Ceil(lo[j]-eps), math.Floor(hi[j]+eps)
		}
	}
	x, obj, st := relaxed(p, lo, hi)
	if st != Optimal || !hasInt(p.Int) {
		return x, obj, st
	}
	// Séparation et évaluation, en profondeur d'abord.
	var best []float64
	bestObj := math.Inf(1)
	if p.Maximize {
		bestObj = math.Inf(-1)
	}
	better := func(v float64) bool {
		if best == nil {
			return true
		}
		if p.Maximize {
			return v > bestObj+1e-9*math.Max(1, math.Abs(bestObj))
		}
		return v < bestObj-1e-9*math.Max(1, math.Abs(bestObj))
	}
	nodes := 0
	limited := false
	var branch func(lo, hi []float64)
	branch = func(lo, hi []float64) {
		if nodes >= 20000 {
			limited = true
			return
		}
		nodes++
		x, obj, st := relaxed(p, lo, hi)
		if st == Unbounded {
			limited = true
			return
		}
		if st != Optimal || !better(obj) {
			return
		}
		k, frac := -1, 0.0
		for j := 0; j < n; j++ {
			if j < len(p.Int) && p.Int[j] {
				if f := math.Abs(x[j] - math.Round(x[j])); f > 1e-7 && f > frac {
					k, frac = j, f
				}
			}
		}
		if k < 0 {
			for j := 0; j < n; j++ {
				if j < len(p.Int) && p.Int[j] {
					x[j] = math.Round(x[j])
				}
			}
			best, bestObj = x, obj
			return
		}
		down := append([]float64(nil), hi...)
		down[k] = math.Floor(x[k])
		up := append([]float64(nil), lo...)
		up[k] = math.Ceil(x[k])
		// Le côté le plus proche d'abord.
		if x[k]-math.Floor(x[k]) < 0.5 {
			branch(lo, down)
			branch(up, hi)
		} else {
			branch(up, hi)
			branch(lo, down)
		}
	}
	branch(lo, hi)
	if best == nil {
		if limited {
			return nil, 0, Limit
		}
		return nil, 0, Infeasible
	}
	if limited {
		return best, bestObj, Limit
	}
	return best, bestObj, Optimal
}

func hasInt(b []bool) bool {
	for _, v := range b {
		if v {
			return true
		}
	}
	return false
}

func fill(v []float64, n int, def float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = def
		if i < len(v) {
			out[i] = v[i]
		}
	}
	return out
}

// relaxed résout le programme sans les contraintes d'intégrité, avec les
// bornes lo, hi.
func relaxed(p LP, lo, hi []float64) ([]float64, float64, Status) {
	n := len(p.C)
	for j := 0; j < n; j++ {
		if lo[j] > hi[j]+eps {
			return nil, 0, Infeasible
		}
	}
	// Variables y >= 0 : x[j] = off[j] + Σ coef·y.
	type term struct {
		y    int
		coef float64
	}
	off := make([]float64, n)
	terms := make([][]term, n)
	ny := 0
	type bound struct {
		y int
		v float64
	}
	var ups []bound
	for j := 0; j < n; j++ {
		switch {
		case !math.IsInf(lo[j], -1):
			off[j] = lo[j]
			terms[j] = []term{{ny, 1}}
			if !math.IsInf(hi[j], 1) {
				ups = append(ups, bound{ny, hi[j] - lo[j]})
			}
			ny++
		case !math.IsInf(hi[j], 1):
			off[j] = hi[j]
			terms[j] = []term{{ny, -1}}
			ny++
		default:
			terms[j] = []term{{ny, 1}, {ny + 1, -1}}
			ny += 2
		}
	}
	// Lignes en y.
	var rows []lrow
	for i, ai := range p.A {
		r := lrow{a: make([]float64, ny), op: p.Ops[i], b: p.B[i]}
		for j := 0; j < n && j < len(ai); j++ {
			r.b -= ai[j] * off[j]
			for _, t := range terms[j] {
				r.a[t.y] += ai[j] * t.coef
			}
		}
		rows = append(rows, r)
	}
	for _, u := range ups {
		r := lrow{a: make([]float64, ny), op: LE, b: u.v}
		r.a[u.y] = 1
		rows = append(rows, r)
	}
	cy := make([]float64, ny)
	sign := 1.0
	if p.Maximize {
		sign = -1 // le simplexe minimise
	}
	for j := 0; j < n; j++ {
		for _, t := range terms[j] {
			cy[t.y] += sign * p.C[j] * t.coef
		}
	}
	y, st := simplex(rows, cy, ny)
	if st != Optimal {
		return nil, 0, st
	}
	x := make([]float64, n)
	obj := 0.0
	for j := 0; j < n; j++ {
		x[j] = off[j]
		for _, t := range terms[j] {
			x[j] += t.coef * y[t.y]
		}
		obj += p.C[j] * x[j]
	}
	return x, obj, Optimal
}

// lrow : une ligne a·y op b.
type lrow struct {
	a  []float64
	op Op
	b  float64
}

// simplex minimise c·y sous les lignes, y >= 0 (deux phases, tableau dense).
func simplex(rows []lrow, c []float64, ny int) ([]float64, Status) {
	m := len(rows)
	// Colonnes : y, écarts, artificielles ; dernière : second membre.
	nSlack, nArt := 0, 0
	for i := range rows {
		if rows[i].b < 0 {
			for k := range rows[i].a {
				rows[i].a[k] = -rows[i].a[k]
			}
			rows[i].b = -rows[i].b
			switch rows[i].op {
			case LE:
				rows[i].op = GE
			case GE:
				rows[i].op = LE
			}
		}
		switch rows[i].op {
		case LE:
			nSlack++
		case GE:
			nSlack++
			nArt++
		case EQ:
			nArt++
		}
	}
	cols := ny + nSlack + nArt
	rhs := cols
	T := make([][]float64, m+1)
	for i := range T {
		T[i] = make([]float64, cols+1)
	}
	basis := make([]int, m)
	art := make([]bool, cols)
	s, a := ny, ny+nSlack
	for i, r := range rows {
		copy(T[i], r.a)
		T[i][rhs] = r.b
		switch r.op {
		case LE:
			T[i][s] = 1
			basis[i] = s
			s++
		case GE:
			T[i][s] = -1
			s++
			T[i][a] = 1
			art[a] = true
			basis[i] = a
			a++
		case EQ:
			T[i][a] = 1
			art[a] = true
			basis[i] = a
			a++
		}
	}
	setObj := func(cost []float64) {
		z := T[m]
		for j := range z {
			z[j] = 0
		}
		for j := 0; j < cols; j++ {
			z[j] = cost[j]
		}
		for i := 0; i < m; i++ {
			cb := cost[basis[i]]
			if cb == 0 {
				continue
			}
			for j := 0; j <= cols; j++ {
				z[j] -= cb * T[i][j]
			}
		}
	}
	pivot := func(pr, pc int) {
		pv := T[pr][pc]
		for j := 0; j <= cols; j++ {
			T[pr][j] /= pv
		}
		for i := 0; i <= m; i++ {
			if i == pr {
				continue
			}
			f := T[i][pc]
			if f == 0 {
				continue
			}
			for j := 0; j <= cols; j++ {
				T[i][j] -= f * T[pr][j]
			}
		}
		basis[pr] = pc
	}
	run := func(allowed func(int) bool) Status {
		for it := 0; it < 50000; it++ {
			pc := -1
			for j := 0; j < cols; j++ {
				if allowed(j) && T[m][j] < -eps {
					pc = j // Bland : le plus petit indice
					break
				}
			}
			if pc < 0 {
				return Optimal
			}
			pr := -1
			best := math.Inf(1)
			for i := 0; i < m; i++ {
				if T[i][pc] > eps {
					r := T[i][rhs] / T[i][pc]
					if r < best-eps || (r < best+eps && pr >= 0 && basis[i] < basis[pr]) {
						best, pr = r, i
					}
				}
			}
			if pr < 0 {
				return Unbounded
			}
			pivot(pr, pc)
		}
		return Limit
	}
	// Phase 1 : somme des artificielles.
	if nArt > 0 {
		cost := make([]float64, cols)
		for j := range cost {
			if art[j] {
				cost[j] = 1
			}
		}
		setObj(cost)
		if st := run(func(int) bool { return true }); st == Limit {
			return nil, Limit
		}
		if -T[m][rhs] > 1e-7 {
			return nil, Infeasible
		}
		// Artificielles restées dans la base (à zéro) : sorties si possible.
		for i := 0; i < m; i++ {
			if !art[basis[i]] {
				continue
			}
			for j := 0; j < ny+nSlack; j++ {
				if math.Abs(T[i][j]) > eps {
					pivot(i, j)
					break
				}
			}
		}
	}
	cost := make([]float64, cols)
	copy(cost, c)
	setObj(cost)
	st := run(func(j int) bool { return !art[j] })
	if st != Optimal {
		return nil, st
	}
	y := make([]float64, ny)
	for i := 0; i < m; i++ {
		if basis[i] < ny {
			y[basis[i]] = T[i][rhs]
		}
	}
	return y, Optimal
}
