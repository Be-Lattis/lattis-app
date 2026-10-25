package optim

import (
	"math"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestRoot(t *testing.T) {
	lin := func(x float64) (float64, bool) { return 3*x + 2 - 11, true }
	if r := Root(lin, 0, 100, 1e-9); !r.Found || !near(r.X, 3, 1e-9) {
		t.Errorf("linéaire : %+v", r)
	}
	sq := func(x float64) (float64, bool) { return x*x - 2, true }
	if r := Root(sq, 5, 200, 1e-12); !r.Found || !near(r.X, math.Sqrt2, 1e-9) {
		t.Errorf("x² = 2 : %+v", r)
	}
	// Pente nulle au départ : l'encadrement prend le relais.
	flat := func(x float64) (float64, bool) {
		if x < 10 {
			return -1, true
		}
		return x - 11, true
	}
	if r := Root(flat, 0, 300, 1e-9); !r.Found || !near(r.X, 11, 1e-6) {
		t.Errorf("plateau : %+v", r)
	}
	// Erreur sur une partie du domaine (racine carrée).
	sqrt := func(x float64) (float64, bool) {
		if x < 0 {
			return 0, false
		}
		return math.Sqrt(x) - 4, true
	}
	if r := Root(sqrt, 1, 300, 1e-9); !r.Found || !near(r.X, 16, 1e-6) {
		t.Errorf("racine : %+v", r)
	}
	none := func(x float64) (float64, bool) { return x*x + 1, true }
	if r := Root(none, 3, 300, 1e-9); r.Found {
		t.Errorf("sans solution : %+v", r)
	} else if !near(r.X, 0, 0.1) {
		t.Errorf("sans solution, meilleur point : %+v", r)
	}
}

func TestSimplex(t *testing.T) {
	// max 3x + 5y, x <= 4, 2y <= 12, 3x + 2y <= 18 : (2, 6), 36.
	p := LP{Maximize: true, C: []float64{3, 5},
		A:   [][]float64{{1, 0}, {0, 2}, {3, 2}},
		Ops: []Op{LE, LE, LE}, B: []float64{4, 12, 18},
		Lo: []float64{0, 0}}
	x, obj, st := SolveLP(p)
	if st != Optimal || !near(obj, 36, 1e-9) || !near(x[0], 2, 1e-9) || !near(x[1], 6, 1e-9) {
		t.Errorf("max : %v %v %v", x, obj, st)
	}
	// min x + y, x + 2y >= 4, 3x + y >= 6 : (1,6 ; 1,2), 2,8.
	p = LP{C: []float64{1, 1}, A: [][]float64{{1, 2}, {3, 1}}, Ops: []Op{GE, GE}, B: []float64{4, 6}, Lo: []float64{0, 0}}
	if x, obj, st := SolveLP(p); st != Optimal || !near(obj, 2.8, 1e-9) || !near(x[0], 1.6, 1e-9) {
		t.Errorf("min : %v %v %v", x, obj, st)
	}
	// Égalité et variables libres : min x, x - y = -3, -1 <= y <= 1 → x = -4.
	p = LP{C: []float64{1, 0}, A: [][]float64{{1, -1}, {0, 1}, {0, 1}}, Ops: []Op{EQ, LE, GE}, B: []float64{-3, 1, -1}}
	if x, _, st := SolveLP(p); st != Optimal || !near(x[0], -4, 1e-9) {
		t.Errorf("libre : %v %v", x, st)
	}
	// Irréalisable.
	p = LP{C: []float64{1}, A: [][]float64{{1}, {1}}, Ops: []Op{GE, LE}, B: []float64{5, 3}, Lo: []float64{0}}
	if _, _, st := SolveLP(p); st != Infeasible {
		t.Errorf("irréalisable : %v", st)
	}
	// Non borné.
	p = LP{Maximize: true, C: []float64{1}, A: [][]float64{{-1}}, Ops: []Op{LE}, B: []float64{3}, Lo: []float64{0}}
	if _, _, st := SolveLP(p); st != Unbounded {
		t.Errorf("non borné : %v", st)
	}
	// Entiers : max 5x + 4y, 6x + 4y <= 24, x + 2y <= 6 ; relâché (3 ; 1,5)
	// 21, entier (4 ; 0) 20.
	p = LP{Maximize: true, C: []float64{5, 4}, A: [][]float64{{6, 4}, {1, 2}}, Ops: []Op{LE, LE}, B: []float64{24, 6}, Lo: []float64{0, 0}, Int: []bool{true, true}}
	if x, obj, st := SolveLP(p); st != Optimal || !near(obj, 20, 1e-9) || x[0] != 4 || x[1] != 0 {
		t.Errorf("entiers : %v %v %v", x, obj, st)
	}
	// Binaires : sac à dos, poids 5 4 3 <= 8, valeurs 10 7 5 → 1 0 1 (15).
	p = LP{Maximize: true, C: []float64{10, 7, 5}, A: [][]float64{{5, 4, 3}}, Ops: []Op{LE}, B: []float64{8},
		Lo: []float64{0, 0, 0}, Hi: []float64{1, 1, 1}, Int: []bool{true, true, true}}
	if x, obj, st := SolveLP(p); st != Optimal || !near(obj, 15, 1e-9) || x[0] != 1 || x[1] != 0 || x[2] != 1 {
		t.Errorf("binaires : %v %v %v", x, obj, st)
	}
}

func TestMinimize(t *testing.T) {
	rosen := func(x []float64) float64 { return 100*math.Pow(x[1]-x[0]*x[0], 2) + math.Pow(1-x[0], 2) }
	x, f, _ := Minimize(rosen, []float64{-1.2, 1}, nil, nil, 5000)
	if f > 1e-6 || !near(x[0], 1, 1e-3) || !near(x[1], 1, 1e-3) {
		t.Errorf("Rosenbrock : %v %v", x, f)
	}
	// Borné : min (x - 5)² avec x <= 2 → 2.
	x, _, _ = Minimize(func(x []float64) float64 { return (x[0] - 5) * (x[0] - 5) }, []float64{0}, nil, []float64{2}, 500)
	if !near(x[0], 2, 1e-6) {
		t.Errorf("borné : %v", x)
	}
}
