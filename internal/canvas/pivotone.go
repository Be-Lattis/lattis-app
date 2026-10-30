package canvas

import (
	"bytes"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Tableau croisé en une seule lecture de la source (audit des performances
// du 30/10, suite) : une passe groupée au niveau le plus fin (tous les
// champs des lignes et des colonnes) rend, pour chaque groupe, la première
// ligne (min(_pos)) et des agrégats partiels ; les niveaux au-dessus
// (sous-totaux, totaux) en sont déduits ici, sans relire la source :
//   - nombre : somme des nombres ;
//   - somme : somme exacte des sommes entières, somme compensée (Kahan-
//     Babuska-Neumaier, comme SQLite) des sommes réelles ;
//   - moyenne : somme et nombre des valeurs, divisés à la fin, comme SQLite ;
//   - min, max : comparés comme SQLite (nombres, puis textes, puis données
//     binaires).
// Le nombre distinct ne se déduit pas : calcul en plusieurs passes
// (pivotcube.go), comme le reste des cas rares (dépassement d'un entier,
// deux étiquettes qui s'écrivent pareil, plus de lignes ou de colonnes que
// permis : l'erreur vient du calcul en plusieurs passes).
//
// Sans tri choisi, les lignes et les colonnes de détail sont rangées ici
// comme le fait axisOrder (première apparition de chaque groupe, vides en
// dernier) ; avec un tri, les deux passes ordonnées restent en SQL et
// celle-ci donne les autres niveaux.
//
// 1 M de lignes, TCD Pays × Ville (somme, nombre) : 4 passes → 1.

// pivotOnePass : calcul en une passe permis (l'essai de comparaison
// l'éteint pour obtenir le calcul en plusieurs passes).
var pivotOnePass = true

// pivotOneFits dit si le TCD se calcule en une passe.
func pivotOneFits(p pivotPlan, cb *pivotCube) bool {
	if !pivotOnePass || cb.nr+cb.nc == 0 {
		return false
	}
	if p.sorted() && (cb.nr == 0 || cb.nc == 0) {
		return false // la passe ordonnée est déjà le niveau le plus fin
	}
	for _, v := range cb.vals {
		if v.Agg == "countDistinct" {
			return false
		}
	}
	return true
}

// pivotPartial : les colonnes partielles d'une valeur dans la passe.
func pivotPartial(agg, v string) []string {
	switch agg {
	case "avg":
		return []string{"sum(" + v + ")", "count(" + v + ")"}
	}
	return []string{agg + "(" + v + ")"}
}

// pivotAcc : l'agrégat d'une valeur pour un groupe, en cours.
type pivotAcc struct {
	isum      int64
	fsum, err float64
	real      bool // une somme partielle réelle : somme compensée
	some      bool // une somme partielle non vide
	cnt       int64
	best      any
}

// addSum ajoute une somme partielle ; false : dépassement d'un entier.
func (a *pivotAcc) addSum(x any) bool {
	switch n := x.(type) {
	case nil:
		return true
	case int64:
		a.some = true
		if a.real {
			a.kbn(float64(n))
			return true
		}
		s := a.isum + n
		if (s > a.isum) != (n > 0) {
			return false
		}
		a.isum = s
	case float64:
		a.some = true
		if !a.real {
			a.real = true
			a.fsum, a.err = 0, 0
			a.kbn(float64(a.isum))
		}
		a.kbn(n)
	default:
		return false // inattendu : calcul en plusieurs passes
	}
	return true
}

// kbn : un pas de la somme compensée de SQLite (kahanBabuskaNeumaierStep).
func (a *pivotAcc) kbn(x float64) {
	t := a.fsum + x
	if math.Abs(a.fsum) >= math.Abs(x) {
		a.err += (a.fsum - t) + x
	} else {
		a.err += (x - t) + a.fsum
	}
	a.fsum = t
}

func (a *pivotAcc) sum() any {
	if !a.some {
		return nil
	}
	if !a.real {
		return a.isum
	}
	r := a.fsum
	if !math.IsInf(a.err, 0) && !math.IsNaN(a.err) {
		r += a.err
	}
	return r
}

// addCount ajoute un nombre partiel ; false : inattendu.
func (a *pivotAcc) addCount(x any) bool {
	n, ok := x.(int64)
	a.cnt += n
	return ok
}

// keep garde x s'il passe avant (min) ou après (max) le meilleur.
func (a *pivotAcc) keep(x any, max bool) {
	if x == nil {
		return
	}
	if a.best == nil {
		a.best = x
		return
	}
	if c := sqlCompare(a.best, x); (max && c < 0) || (!max && c > 0) {
		a.best = x
	}
}

// sqlCompare compare deux valeurs non vides comme SQLite (sqlite3MemCompare,
// classement BINARY) : nombres, puis textes, puis données binaires.
func sqlCompare(a, b any) int {
	class := func(v any) int {
		switch v.(type) {
		case int64, float64:
			return 0
		case string:
			return 1
		}
		return 2
	}
	ca, cb := class(a), class(b)
	if ca != cb {
		return ca - cb
	}
	switch ca {
	case 0:
		return numCompare(a, b)
	case 1:
		return strings.Compare(a.(string), b.(string))
	}
	ba, _ := a.([]byte)
	bb, _ := b.([]byte)
	return bytes.Compare(ba, bb)
}

// numCompare compare deux nombres (entier ou réel) exactement.
func numCompare(a, b any) int {
	ai, aInt := a.(int64)
	bi, bInt := b.(int64)
	switch {
	case aInt && bInt:
		return cmpOf(ai, bi)
	case aInt:
		return -floatIntCompare(b.(float64), ai)
	case bInt:
		return floatIntCompare(a.(float64), bi)
	}
	return cmpOf(a.(float64), b.(float64))
}

// floatIntCompare compare un réel et un entier sans perte.
func floatIntCompare(f float64, i int64) int {
	if f < -9223372036854775808.0 {
		return -1
	}
	if f >= 9223372036854775808.0 {
		return 1
	}
	t := int64(f)
	if t != i {
		return cmpOf(t, i)
	}
	return cmpOf(f, float64(t))
}

func cmpOf[T int64 | float64](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// rawKey : une étiquette lue en base, sans perte (type compris), pour
// vérifier que deux groupes n'ont pas la même étiquette affichée.
func rawKey(v any) string {
	if s, ok := v.(string); ok {
		return "s" + s
	}
	return fmt.Sprintf("%T:%v", v, v)
}

// pivotOneGroup : un groupe d'un niveau (un agrégat par valeur).
type pivotOneGroup struct{ accs []pivotAcc }

// pivotOne calcule le cube en une passe ; ordered : les étiquettes de
// détail sont rangées ici (sans tri choisi) ; sinon cb.rows, cb.cols et les
// niveaux (nr, 0) et (0, nc) viennent des passes ordonnées. false : calcul
// en plusieurs passes (voir plus haut) ; cb n'est alors pas touché.
func (t *tx) pivotOne(p pivotPlan, cb *pivotCube, where string, rowNames, colNames []string, ordered bool) (bool, error) {
	var sel, group []string
	for i, c := range p.rows {
		sel = append(sel, p.text(c)+" AS "+rowNames[i])
		group = append(group, rowNames[i])
	}
	for i, c := range p.cols {
		sel = append(sel, p.text(c)+" AS "+colNames[i])
		group = append(group, colNames[i])
	}
	nk := len(sel)
	sel = append(sel, "min(_pos)")
	// first[vi] : la première colonne partielle de la valeur vi.
	first := make([]int, len(cb.vals))
	for vi, v := range cb.vals {
		e := "1"
		if c := p.vals[vi]; c != nil {
			e = colSQL(c.ID)
		}
		first[vi] = len(sel) - nk - 1
		sel = append(sel, pivotPartial(v.Agg, e)...)
	}
	q := `SELECT ` + strings.Join(sel, ", ") + ` FROM ` + p.from() + where + ` GROUP BY ` + strings.Join(group, ", ")
	rs, err := t.query(q, p.args...)
	if err != nil {
		return false, err
	}
	defer rs.Close()

	nr, nc := cb.nr, cb.nc
	// levels[dr][dc] : groupes du niveau, par cubeKey.
	levels := make([][]map[string]*pivotOneGroup, nr+1)
	for dr := range levels {
		levels[dr] = make([]map[string]*pivotOneGroup, nc+1)
		for dc := range levels[dr] {
			levels[dr][dc] = map[string]*pivotOneGroup{}
		}
	}
	// raw[axe][pathKey des étiquettes] : le chemin exact (rawKey) ; deux
	// chemins différents de même étiquette : plusieurs passes.
	raw := [2]map[string]string{{}, {}}
	// minPos[axe][chemin exact] : première ligne de chaque groupe d'un axe.
	minPos := [2]map[string]string{{}, {}}
	// detail[axe] : chemins de détail (étiquettes et chemin exact).
	type path struct {
		labels []string
		keys   []any
		raw    string
	}
	var detail [2][]path
	seenDetail := [2]map[string]bool{{}, {}}
	vals := make([]any, len(sel))
	ptrs := make([]any, len(sel))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rs.Next() {
		clear(vals)
		if err := rs.Scan(ptrs...); err != nil {
			return false, err
		}
		pos, ok := vals[nk].(string)
		if !ok {
			return false, nil
		}
		var labels [2][]string
		var keys [2][]any
		for axis, n := range [2]int{nr, nc} {
			off := 0
			if axis == 1 {
				off = nr
			}
			labels[axis] = make([]string, n)
			keys[axis] = vals[off : off+n : off+n]
			rk := ""
			for i := range n {
				labels[axis][i] = pivotKeyLabel(vals[off+i])
				rk += rawKey(vals[off+i]) + "\x00"
				lk := pathKey(labels[axis][:i+1])
				if prev, ok := raw[axis][lk]; !ok {
					raw[axis][lk] = rk
				} else if prev != rk {
					return false, nil
				}
				if m, ok := minPos[axis][rk]; !ok || pos < m {
					minPos[axis][rk] = pos
				}
			}
			if n > 0 && !seenDetail[axis][rk] {
				seenDetail[axis][rk] = true
				detail[axis] = append(detail[axis], path{labels[axis], append([]any{}, keys[axis]...), rk})
				if (axis == 0 && len(detail[0]) > MaxPivotRows) || (axis == 1 && len(detail[1]) > MaxPivotCols) {
					return false, nil
				}
			}
		}
		part := vals[nk+1:]
		for dr := 0; dr <= nr; dr++ {
			for dc := 0; dc <= nc; dc++ {
				k := cubeKey(labels[0][:dr], labels[1][:dc])
				g := levels[dr][dc][k]
				if g == nil {
					g = &pivotOneGroup{accs: make([]pivotAcc, len(cb.vals))}
					levels[dr][dc][k] = g
				}
				for vi, v := range cb.vals {
					a := &g.accs[vi]
					x := part[first[vi]]
					ok := true
					switch v.Agg {
					case "count":
						ok = a.addCount(x)
					case "sum":
						ok = a.addSum(x)
					case "avg":
						ok = a.addSum(x) && a.addCount(part[first[vi]+1])
					case "min", "max":
						a.keep(x, v.Agg == "max")
					default:
						ok = false
					}
					if !ok {
						return false, nil
					}
				}
			}
		}
	}
	if err := rs.Err(); err != nil {
		return false, err
	}
	// Le total général existe même sans ligne (agrégats d'un ensemble vide).
	if len(levels[0][0]) == 0 {
		levels[0][0][cubeKey(nil, nil)] = &pivotOneGroup{accs: make([]pivotAcc, len(cb.vals))}
	}
	cells := map[string][]any{}
	for dr := 0; dr <= nr; dr++ {
		for dc := 0; dc <= nc; dc++ {
			if !ordered && ((dr == nr && dc == 0) || (dr == 0 && dc == nc)) {
				continue // déjà lu par la passe ordonnée
			}
			if dr == 0 && dc == 0 && len(cb.vals) == 0 {
				continue // ni étiquette ni valeur : aucune passe
			}
			for k, g := range levels[dr][dc] {
				out := make([]any, len(cb.vals))
				for vi, v := range cb.vals {
					a := &g.accs[vi]
					switch v.Agg {
					case "count":
						out[vi] = a.cnt
					case "sum":
						out[vi] = a.sum()
					case "avg":
						if a.cnt > 0 {
							switch s := a.sum().(type) {
							case int64:
								out[vi] = float64(s) / float64(a.cnt)
							case float64:
								out[vi] = s / float64(a.cnt)
							}
						}
					default:
						out[vi] = a.best
					}
				}
				cells[k] = out
			}
		}
	}
	if ordered {
		// Rangement d'axisOrder : pour chaque champ, les étiquettes vides
		// en dernier, puis la première apparition du groupe ; enfin la
		// première ligne du chemin entier.
		for axis := range 2 {
			list := detail[axis]
			blank := func(v any) int {
				switch v {
				case nil:
					return 0 // NULL = '' : NULL, en tête
				case "":
					return 2
				}
				return 1
			}
			prefix := func(pa path, i int) string {
				rk := ""
				for _, k := range pa.keys[:i+1] {
					rk += rawKey(k) + "\x00"
				}
				return rk
			}
			slices.SortFunc(list, func(a, b path) int {
				for i := range a.keys {
					if c := blank(a.keys[i]) - blank(b.keys[i]); c != 0 {
						return c
					}
					if c := strings.Compare(minPos[axis][prefix(a, i)], minPos[axis][prefix(b, i)]); c != 0 {
						return c
					}
				}
				return strings.Compare(minPos[axis][a.raw], minPos[axis][b.raw])
			})
			var out [][]string // nil sans étiquette, comme la passe ordonnée
			for _, pa := range list {
				out = append(out, pa.labels)
			}
			if axis == 0 {
				cb.rows = out
			} else {
				cb.cols = out
			}
		}
	}
	for k, v := range cells {
		cb.cells[k] = v
	}
	return true, nil
}
