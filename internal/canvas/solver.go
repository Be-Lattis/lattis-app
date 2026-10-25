package canvas

import (
	"context"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/optim"
)

// Solveur (Données › Analyse de scénarios › Solveur) : maximiser, minimiser
// ou amener à une valeur une cellule objectif en changeant des cellules
// variables, sous contraintes (<=, >=, =, entier, binaire). Le problème est
// d'abord sondé (objectif et contraintes évalués en n + 3 points) : s'il est
// linéaire, le simplexe donne l'optimum exact (séparation et évaluation pour
// les entiers) ; sinon Nelder–Mead borné, avec pénalités pour les
// contraintes, donne une solution approchée. Rien ne change dans le classeur
// avant « Conserver » (SetWhatIfValues).

// SolverConstraint : Ref (cellule ou plage) Op RHS. Op : "<=", ">=", "=",
// "int" ou "bin" (ces deux-là sur des cellules variables, sans RHS). RHS :
// un nombre (langue de la session) ou une adresse (une cellule, ou une
// plage de même taille que Ref).
type SolverConstraint struct {
	Ref string `json:"ref"`
	Op  string `json:"op"`
	RHS string `json:"rhs,omitempty"`
}

// SolverParams : le problème. Goal : "max", "min" ou "value" (Value).
// NonNegative : les variables sans borne inférieure sont >= 0.
type SolverParams struct {
	Objective   string             `json:"objective"`
	Goal        string             `json:"goal"`
	Value       string             `json:"value,omitempty"`
	Variables   string             `json:"variables"`
	Constraints []SolverConstraint `json:"constraints"`
	NonNegative bool               `json:"nonNegative"`
}

// SolverResult : Status "optimal" (linéaire, exact), "approx" (non
// linéaire), "infeasible", "unbounded" ou "limit" (rien de mieux dans le
// temps imparti). Cells et Raws (forme gardée) vont à SetWhatIfValues ;
// Values et Objective : affichés.
type SolverResult struct {
	Status    string   `json:"status"`
	Linear    bool     `json:"linear"`
	Cells     []string `json:"cells"`
	Raws      []string `json:"raws"`
	Values    []string `json:"values"`
	Objective string   `json:"objective"`
}

// solverRow : une contrainte sur une cellule : lhs - rhs op 0.
type solverRow struct {
	lhs    whatCell
	op     optim.Op
	rhs    float64
	rhsRef *whatCell
}

// Solve résout le problème ; rien n'est changé dans le classeur.
func (m *Model) Solve(ctx context.Context, sheet int64, p SolverParams) (SolverResult, error) {
	var res SolverResult
	obj, err := oneCell(p.Objective)
	if err != nil {
		return res, err
	}
	vars, err := parseCells(p.Variables)
	if err != nil {
		return res, err
	}
	if len(vars) == 0 {
		return res, msg.New("whatif.badRef", msg.Args{"ref": p.Variables})
	}
	var target float64
	switch p.Goal {
	case "max", "min":
	case "value":
		if target, err = m.whatNumber(ctx, p.Value); err != nil {
			return res, err
		}
	default:
		p.Goal = "max"
	}
	n := len(vars)
	index := map[whatCell]int{}
	for i, v := range vars {
		index[v] = i
	}
	lo, hi := make([]float64, n), make([]float64, n)
	isInt := make([]bool, n)
	for i := range lo {
		lo[i], hi[i] = math.Inf(-1), math.Inf(1)
		if p.NonNegative {
			lo[i] = 0
		}
	}
	var rows []solverRow
	for _, c := range p.Constraints {
		bad := msg.New("whatif.badConstraint", msg.Args{"ref": strings.TrimSpace(c.Ref + " " + c.Op + " " + c.RHS)})
		lhs, err := parseCells(c.Ref)
		if err != nil || len(lhs) == 0 {
			return res, bad
		}
		if c.Op == "int" || c.Op == "bin" {
			for _, l := range lhs {
				i, ok := index[l]
				if !ok {
					return res, msg.New("whatif.intNotVar", msg.Args{"ref": l.addr()})
				}
				isInt[i] = true
				if c.Op == "bin" {
					lo[i], hi[i] = math.Max(lo[i], 0), math.Min(hi[i], 1)
				}
			}
			continue
		}
		var op optim.Op
		switch c.Op {
		case "<=":
			op = optim.LE
		case ">=":
			op = optim.GE
		case "=":
			op = optim.EQ
		default:
			return res, bad
		}
		rhs := strings.TrimSpace(c.RHS)
		var refs []whatCell
		var num float64
		if f, err := m.whatNumber(ctx, rhs); err == nil {
			num = f
		} else if refs, err = parseCells(rhs); err != nil || (len(refs) != 1 && len(refs) != len(lhs)) {
			return res, bad
		}
		for k, l := range lhs {
			r := solverRow{lhs: l, op: op, rhs: num}
			if len(refs) > 0 {
				rr := refs[min(k, len(refs)-1)]
				r.rhsRef = &rr
			}
			// Une borne simple sur une variable (B1 <= 10) : dans les bornes.
			if i, ok := index[l]; ok && r.rhsRef == nil {
				switch op {
				case optim.LE:
					hi[i] = math.Min(hi[i], num)
				case optim.GE:
					lo[i] = math.Max(lo[i], num)
				case optim.EQ:
					lo[i], hi[i] = math.Max(lo[i], num), math.Min(hi[i], num)
				}
				continue
			}
			rows = append(rows, r)
		}
	}
	res.Cells = make([]string, n)
	for i, v := range vars {
		res.Cells[i] = v.addr()
	}
	err = m.trial(ctx, func(t *tx) error {
		for _, v := range vars {
			if err := t.guardCells(sheet, Rect{v.Row, v.Col, v.Row, v.Col}, false); err != nil {
				return err
			}
		}
		if err := t.formulaCell(sheet, obj); err != nil {
			return err
		}
		ins := make([]whatInput, n)
		x0 := make([]float64, n)
		for i, v := range vars {
			if ins[i], err = t.inputCell(sheet, v); err != nil {
				return err
			}
			if f, ok, err := t.cellNumber(sheet, v); err != nil {
				return err
			} else if ok {
				x0[i] = math.Min(math.Max(f, lo[i]), hi[i])
			}
		}
		deadline := time.Now().Add(20 * time.Second)
		var failed error
		// eval : objectif puis lhs - rhs de chaque contrainte ; faux si une
		// cellule est en erreur.
		eval := func(x []float64) ([]float64, bool) {
			if failed != nil {
				return nil, false
			}
			if err := ctx.Err(); err != nil {
				failed = err
				return nil, false
			}
			for i, in := range ins {
				if err := t.setNumber(in, x[i]); err != nil {
					failed = err
					return nil, false
				}
			}
			out := make([]float64, 1+len(rows))
			f, ok, err := t.cellNumber(sheet, obj)
			if err != nil {
				failed = err
				return nil, false
			}
			out[0] = f
			for k, r := range rows {
				l, ok2, err := t.cellNumber(sheet, r.lhs)
				if err != nil {
					failed = err
					return nil, false
				}
				rv := r.rhs
				if r.rhsRef != nil {
					var ok3 bool
					if rv, ok3, err = t.cellNumber(sheet, *r.rhsRef); err != nil {
						failed = err
						return nil, false
					}
					ok2 = ok2 && ok3
				}
				out[1+k] = l - rv
				ok = ok && ok2
			}
			return out, ok
		}
		// 1. Sondage : linéaire ?
		base, ok := eval(x0)
		if failed != nil {
			return failed
		}
		linear := ok
		grads := make([][]float64, 1+len(rows))
		for k := range grads {
			grads[k] = make([]float64, n)
		}
		for i := 0; linear && i < n; i++ {
			x := append([]float64(nil), x0...)
			x[i]++
			y, ok := eval(x)
			if !ok {
				linear = false
				break
			}
			for k := range grads {
				grads[k][i] = y[k] - base[k]
			}
		}
		rng := rand.New(rand.NewPCG(1, 2))
		for probe := 0; linear && probe < 2; probe++ {
			x := make([]float64, n)
			d := make([]float64, n)
			for i := range x {
				d[i] = (rng.Float64()*5 - 2) * math.Max(1, math.Abs(x0[i]))
				x[i] = x0[i] + d[i]
			}
			y, ok := eval(x)
			if !ok {
				linear = false
				break
			}
			for k := range grads {
				want := base[k]
				scale := math.Abs(base[k]) + math.Abs(y[k])
				for i := range d {
					want += grads[k][i] * d[i]
					scale += math.Abs(grads[k][i] * d[i])
				}
				if math.Abs(y[k]-want) > 1e-7*math.Max(1, scale) {
					linear = false
				}
			}
		}
		if failed != nil {
			return failed
		}
		res.Linear = linear
		var x []float64
		if linear {
			lp := optim.LP{Maximize: p.Goal == "max", C: grads[0], Lo: lo, Hi: hi, Int: isInt}
			for k, r := range rows {
				g := grads[1+k]
				b := -base[1+k]
				for i := range g {
					b += g[i] * x0[i]
				}
				lp.A = append(lp.A, g)
				lp.Ops = append(lp.Ops, r.op)
				lp.B = append(lp.B, b)
			}
			if p.Goal == "value" {
				b := target - base[0]
				for i := range grads[0] {
					b += grads[0][i] * x0[i]
				}
				lp.A = append(lp.A, grads[0])
				lp.Ops = append(lp.Ops, optim.EQ)
				lp.B = append(lp.B, b)
				lp.C = make([]float64, n)
			}
			sol, _, st := optim.SolveLP(lp)
			switch st {
			case optim.Infeasible:
				res.Status = "infeasible"
				return nil
			case optim.Unbounded:
				res.Status = "unbounded"
				return nil
			case optim.Limit:
				if sol == nil {
					res.Status = "limit"
					return nil
				}
				res.Status = "limit"
			default:
				res.Status = "optimal"
			}
			x = sol
		} else {
			// 2. Non linéaire : pénalités croissantes, Nelder–Mead borné.
			sign := 1.0
			if p.Goal == "max" {
				sign = -1
			}
			viol := func(y []float64) float64 {
				s := 0.0
				for k, r := range rows {
					g := y[1+k]
					switch r.op {
					case optim.LE:
						g = math.Max(g, 0)
					case optim.GE:
						g = math.Min(g, 0)
					}
					s += g * g
				}
				return s
			}
			scale := 1.0
			if ok {
				scale += math.Abs(base[0])
			}
			cur := append([]float64(nil), x0...)
			lo2, hi2 := append([]float64(nil), lo...), append([]float64(nil), hi...)
			solve := func(rounds int) {
				for k, mu := 0, 1e2*scale; k < rounds && failed == nil && time.Now().Before(deadline); k, mu = k+1, mu*100 {
					f := func(x []float64) float64 {
						if time.Now().After(deadline) {
							return math.Inf(1)
						}
						y, ok := eval(x)
						if !ok {
							return math.Inf(1)
						}
						o := sign * y[0]
						if p.Goal == "value" {
							o = (y[0] - target) * (y[0] - target)
						}
						return o + mu*viol(y)
					}
					cur, _, _ = optim.Minimize(f, cur, lo2, hi2, 400)
				}
			}
			solve(4)
			// Entiers : arrondis, puis le reste réajusté.
			if hasInts(isInt) && failed == nil {
				for i := range cur {
					if isInt[i] {
						cur[i] = math.Round(cur[i])
						lo2[i], hi2[i] = cur[i], cur[i]
					}
				}
				solve(2)
			}
			if failed != nil {
				return failed
			}
			x = cur
			res.Status = "approx"
		}
		// Le point trouvé, arrondi comme il sera posé : objectif et
		// contraintes relus.
		res.Raws = make([]string, n)
		res.Values = make([]string, n)
		px := make([]float64, n)
		u := t.ui()
		for i := range x {
			if isInt[i] {
				x[i] = math.Round(x[i])
			}
			res.Raws[i] = numberRaw(x[i])
			res.Values[i] = u.valueOut(res.Raws[i])
			px[i], _ = pasteNumber(res.Raws[i])
		}
		y, ok := eval(px)
		if failed != nil {
			return failed
		}
		if !linear && (!ok || !feasible(rows, y)) {
			res.Status = "infeasible"
		}
		v, err := t.cellValue(sheet, obj)
		if err != nil {
			return err
		}
		res.Objective = t.showValue(v)
		return nil
	})
	return res, err
}

func hasInts(b []bool) bool {
	for _, v := range b {
		if v {
			return true
		}
	}
	return false
}

// feasible : les contraintes tiennent au point (y : sortie de eval).
func feasible(rows []solverRow, y []float64) bool {
	for k, r := range rows {
		g := y[1+k]
		tol := 1e-6 * math.Max(1, math.Abs(r.rhs))
		switch {
		case r.op == optim.LE && g > tol, r.op == optim.GE && g < -tol, r.op == optim.EQ && math.Abs(g) > tol:
			return false
		}
	}
	return true
}
