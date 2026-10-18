package formula

import (
	"slices"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/trace"
)

// Deps donne les cellules nommées et les tableaux dont dépend une formule.
func Deps(n Node) (names, tables []string) {
	seen := map[*[]string]map[string]bool{&names: {}, &tables: {}}
	add := func(list *[]string, s string) {
		s = strings.ToLower(s)
		if !seen[list][s] {
			seen[list][s] = true
			*list = append(*list, s)
		}
	}
	var walk func(Node)
	walk = func(n Node) {
		switch n := n.(type) {
		case Name:
			add(&names, n.Name)
		case CellRange:
			for _, k := range n.keys(0) {
				add(&names, k)
			}
		case RowRange:
			if n.Ref.Table != "" {
				add(&tables, n.Ref.Table)
			}
		case Ref:
			if n.Table != "" {
				add(&tables, n.Table)
			}
		case Unary:
			walk(n.X)
		case Binary:
			walk(n.L)
			walk(n.R)
		case Call:
			args := n.Args
			switch {
			case (n.Fn == "REFERENCE" || n.Fn == "REFERENCE.APPROX") && len(args) == 3:
				if t, ok := nameOf(args[0]); ok {
					add(&tables, t)
				}
				args = args[2:] // le 2e argument est un nom de colonne
			case IsSpill(n) && len(args) > 0:
				// FILTRE(Ventes; …) : la source nommée est un tableau ;
				// GROUPER.PAR(…; …; SOMME) nomme une fonction.
				if t, ok := args[0].(Name); ok {
					add(&tables, t.Name)
					args = args[1:]
				}
				if n.Fn == "GROUPBY" && len(args) == 3 {
					args = args[:2]
				}
			case n.Fn == "EXISTS" && len(args) == 2:
				if t, ok := nameOf(args[0]); ok {
					add(&tables, t)
				}
				args = args[1:]
			}
			for _, a := range args {
				walk(a)
			}
		}
	}
	walk(n)
	return names, tables
}

// Names regroupe les cellules nommées d'un document (clés en minuscules) et
// leurs valeurs calculées.
type Names struct {
	Formulas map[string]Node
	Values   map[string]Value
	// Extra : dépendances en plus de celles de la formule (les noms lus par
	// les colonnes calculées des tableaux qu'elle cite).
	Extra map[string][]string
}

// order donne l'ordre de calcul des noms (dépendances d'abord) et la liste des
// noms pris dans une référence circulaire.
func (ns *Names) order() (order, cyclic []string) {
	const (
		todo = iota
		active
		done
		broken // dans un cycle, ou dépend d'un cycle
	)
	state := map[string]int{}
	var visit func(string) bool
	visit = func(n string) bool {
		switch state[n] {
		case active:
			return false
		case done:
			return true
		case broken:
			return false
		}
		state[n] = active
		ok := true
		deps, _ := Deps(ns.Formulas[n])
		for _, d := range append(deps, ns.Extra[n]...) {
			if _, isName := ns.Formulas[d]; isName && !visit(d) {
				ok = false
			}
		}
		if ok {
			state[n] = done
			order = append(order, n)
		} else {
			state[n] = broken
			cyclic = append(cyclic, n)
		}
		return ok
	}
	keys := make([]string, 0, len(ns.Formulas))
	for k := range ns.Formulas {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		visit(k)
	}
	return order, cyclic
}

// Recalc recalcule les noms dans l'ordre des dépendances. Une référence
// circulaire donne #CIRC! à tous les noms qui en dépendent.
func (ns *Names) Recalc(env *Env) {
	if trace.On(3, trace.Calc) {
		defer trace.Since(3, trace.Calc, time.Now(), nil, "%d nom(s) recalculé(s)", len(ns.Formulas))
	}
	ns.Values = map[string]Value{}
	env.Names = ns.Values
	order, cyclic := ns.order()
	for _, n := range cyclic {
		ns.Values[n] = Err(ErrCirc)
	}
	for _, n := range order {
		ns.Values[n] = Eval(ns.Formulas[n], env)
	}
}

// Affected donne, dans l'ordre de calcul, les noms à recalculer quand des
// tableaux ou des noms changent.
func (ns *Names) Affected(tables, names []string) []string {
	dirty := map[string]bool{}
	for _, n := range names {
		dirty[strings.ToLower(n)] = true
	}
	order, _ := ns.order()
	var out []string
	for _, n := range order {
		deps, tbls := Deps(ns.Formulas[n])
		hit := dirty[n]
		for _, d := range deps {
			hit = hit || dirty[d]
		}
		for _, t := range tbls {
			hit = hit || slices.ContainsFunc(tables, func(x string) bool { return strings.EqualFold(x, t) })
		}
		if hit {
			dirty[n] = true
			out = append(out, n)
		}
	}
	return out
}

// Order donne l'ordre de calcul des noms (dépendances d'abord) et les noms
// pris dans une référence circulaire.
func (ns *Names) Order() (order, cyclic []string) { return ns.order() }
