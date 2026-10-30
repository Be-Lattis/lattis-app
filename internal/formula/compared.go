package formula

import (
	stdcmp "cmp"
	"slices"
	"strings"
)

// Critères de comparaison par ligne pour la famille .SI.
//
// SOMME.SI(T[Qté]; ">"&[@Qté]; T[Qté]) compare chaque ligne à toutes les
// autres : un agrégat de cette colonne sur 30 000 lignes demandait près d'un
// milliard de tests. Quand un seul critère qui dépend de la ligne est une
// comparaison (<, <=, >, >=), les lignes retenues par les autres critères
// sont rangées par la valeur comparée (une liste triée par sorte : nombres
// et dates, textes sans casse, booléens), avec les cumuls de l'agrégat dans
// les deux sens : chaque ligne lit son résultat par une recherche
// dichotomique. Les autres critères qui dépendent de la ligne restent des
// égalités simples (totaux par valeur, grouped.go). Tout autre cas (critère
// "<>", joker, vide, erreur) reprend le parcours complet.
//
// La sémantique est celle de criterion.compare : un nombre ne se compare
// qu'aux nombres et dates, un texte qu'aux textes, un booléen qu'aux
// booléens ; les cellules vides et les erreurs ne sont jamais retenues.

// comparison dit si un critère est écrit comme une comparaison avec la
// valeur de la ligne : ">"&[@Qté], "<="&[@Date]&"".
func comparison(n Node) bool {
	for {
		b, ok := n.(Binary)
		if !ok {
			break
		}
		if b.Op != "&" {
			return false
		}
		n = b.L
	}
	s, ok := n.(StrLit)
	return ok && (strings.HasPrefix(s.V, "<") || strings.HasPrefix(s.V, ">")) && !strings.HasPrefix(s.V, "<>")
}

// rankable : agrégats que les cumuls savent combiner.
func rankable(c *cond) bool {
	if c.value == -1 {
		return true
	}
	switch c.agg.Fn {
	case "SUM", "AVERAGE", "MIN", "MAX":
		return true
	}
	return false
}

// acc : l'état d'un Agg utile à SOMME, MOYENNE, MIN, MAX et au compte, sans
// ses listes (les cumuls en gardent deux par valeur distincte).
type acc struct {
	count     int
	sum, comp float64
	n, dates  int
	min, max  float64
	err       ErrCode
}

func (a acc) agg(fn string, r numRule) Agg {
	return Agg{Fn: fn, rule: r, Sum: a.sum, Comp: a.comp, N: a.n, Dates: a.dates, Min: a.min, Max: a.max, Err: a.err}
}

func accOf(g Agg, count int) acc {
	return acc{count: count, sum: g.Sum, comp: g.Comp, n: g.N, dates: g.Dates, min: g.Min, max: g.Max, err: g.Err}
}

// merge combine deux états comme Agg.Merge.
func (a acc) merge(b acc, fn string) acc {
	x := a.agg(fn, numRule{})
	x.Merge(b.agg(fn, numRule{}))
	return accOf(x, a.count+b.count)
}

// sortedAccs : valeurs distinctes triées, avec les cumuls jusqu'à chaque
// valeur (pre) et depuis chaque valeur (suf).
type sortedAccs[K stdcmp.Ordered] struct {
	m        map[K]acc // pendant le parcours
	keys     []K
	pre, suf []acc
}

func (s *sortedAccs[K]) add(k K, a acc, fn string) {
	if s.m == nil {
		s.m = map[K]acc{}
	}
	old, ok := s.m[k]
	if ok {
		a = old.merge(a, fn)
	}
	s.m[k] = a
}

func (s *sortedAccs[K]) finish(fn string) {
	s.keys = make([]K, 0, len(s.m))
	for k := range s.m {
		s.keys = append(s.keys, k)
	}
	slices.Sort(s.keys)
	n := len(s.keys)
	s.pre, s.suf = make([]acc, n), make([]acc, n)
	for i, k := range s.keys {
		s.pre[i] = s.m[k]
		if i > 0 {
			s.pre[i] = s.pre[i-1].merge(s.pre[i], fn)
		}
	}
	for i := n - 1; i >= 0; i-- {
		s.suf[i] = s.m[s.keys[i]]
		if i < n-1 {
			s.suf[i] = s.suf[i].merge(s.suf[i+1], fn)
		}
	}
	s.m = nil
}

// span donne le cumul des valeurs v telles que « v op k ».
func (s *sortedAccs[K]) span(op string, k K) acc {
	lo, _ := slices.BinarySearch(s.keys, k) // première valeur >= k
	hi := lo
	if hi < len(s.keys) && s.keys[hi] == k {
		hi++ // première valeur > k
	}
	switch op {
	case "<":
		if lo > 0 {
			return s.pre[lo-1]
		}
	case "<=":
		if hi > 0 {
			return s.pre[hi-1]
		}
	case ">":
		if hi < len(s.keys) {
			return s.suf[hi]
		}
	case ">=":
		if lo < len(s.keys) {
			return s.suf[lo]
		}
	}
	return acc{}
}

// rankedGroup : les lignes d'une combinaison des clés d'égalité, rangées par
// la valeur du critère de comparaison.
type rankedGroup struct {
	nums  sortedAccs[float64] // nombres et dates
	texts sortedAccs[string]  // textes, en minuscules
	bools sortedAccs[float64] // 0 : FAUX, 1 : VRAI
}

func (g *grouped) addRanked(combos [][]string, cols []Value) {
	t := g.tmpl
	v := cols[t.cols[g.rowDep[g.cmp]]]
	// La contribution de la ligne, calculée comme par cond.add.
	one := acc{}
	if t.value == -1 {
		one.count = 1
	} else {
		a := newAgg(t.agg.Fn, t.agg.rule)
		if t.value == -2 {
			a.AddRange(cols[len(cols)-1])
		} else {
			a.AddRange(cols[t.value])
		}
		one = accOf(a, 0)
	}
	for _, c := range combos {
		k := tupleKey(c)
		r, ok := g.ranked[k]
		if !ok {
			r = &rankedGroup{}
			g.ranked[k] = r
		}
		switch v.Kind {
		case Number, Date:
			r.nums.add(v.Num+0, one, t.agg.Fn)
		case Text:
			r.texts.add(strings.ToLower(v.Str), one, t.agg.Fn)
		case Bool:
			r.bools.add(float64(boolInt(v.Bool)), one, t.agg.Fn)
		}
	}
}

// finish trie les valeurs et calcule les cumuls, une fois le parcours fini.
func (g *grouped) finish() {
	fn := g.tmpl.agg.Fn
	for _, r := range g.ranked {
		r.nums.finish(fn)
		r.texts.finish(fn)
		r.bools.finish(fn)
	}
}

// lookupRanked donne le résultat d'une ligne : les combinaisons de ses clés
// d'égalité, et son critère de comparaison crit.
func (g *grouped) lookupRanked(combos [][]string, crit Value) (Value, bool) {
	c := parseCriterion(crit)
	if c.err.IsErr() {
		return Value{}, false
	}
	switch c.op {
	case "<", "<=", ">", ">=":
	default:
		return Value{}, false
	}
	t := g.tmpl
	fn := t.agg.Fn
	var out acc
	for _, combo := range combos {
		r, ok := g.ranked[tupleKey(combo)]
		if !ok {
			continue
		}
		var a acc
		switch c.val.Kind {
		case Number, Date:
			a = r.nums.span(c.op, c.val.Num+0)
		case Text:
			a = r.texts.span(c.op, strings.ToLower(c.val.Str))
		case Bool:
			a = r.bools.span(c.op, float64(boolInt(c.val.Bool)))
		default:
			return Value{}, false // vide, erreur : parcours complet
		}
		out = out.merge(a, fn)
	}
	if t.value == -1 {
		return Num(float64(out.count)), true
	}
	res := out.agg(fn, t.agg.rule)
	return res.Result(), true
}
