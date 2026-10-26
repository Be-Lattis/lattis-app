package canvas

import (
	"github.com/lalamefine/lattis/internal/formula"
)

// « Afficher les valeurs » d'un tableau croisé, comme un tableur : chaque
// cellule de mesure rapportée à un autre agrégat du cube (pivotcube.go) :
//
//   - pctTotal, pctCol, pctRow : % du total général, de sa colonne, de sa
//     ligne ;
//   - pctParentRow, pctParentCol : % du total de la ligne (colonne) parente,
//     le groupe du niveau au-dessus ; pctParent : % du total de l'élément
//     qui la contient dans le champ de base ;
//   - pctOf, diff, pctDiff : % de, différence, % de différence par rapport
//     à l'élément de base (ou au précédent, au suivant) du champ de base,
//     les autres étiquettes égales ;
//   - runTotal, pctRunTotal : résultat cumulé (et sa part du total) le long
//     des éléments du champ de base, dans l'ordre affiché ;
//   - rankAsc, rankDesc : rang parmi les éléments du champ de base (du plus
//     petit, du plus grand ; ex aequo au même rang) ;
//   - index : (valeur × total général) / (total de la ligne × total de la
//     colonne).
//
// Les agrégats viennent tous du SQL (sous-totaux compris) : seule
// l'arithmétique sur le résultat se fait ici. Une cellule sans référence
// (base absente, total général pour un calcul le long d'un champ) reste vide.

// num : la valeur vi de (r, c) en nombre.
func (cb *pivotCube) num(vi int, r, c []string) (float64, bool) {
	f := formula.FromResult(cb.value(vi, r, c))
	return f.Num, f.Kind == formula.Number
}

// text : la cellule affichée de la valeur vi pour les lignes r et les
// colonnes c (chemins d'étiquettes ; vides : total).
func (cb *pivotCube) text(vi int, r, c []string) string {
	v := cb.vals[vi]
	x := cb.value(vi, r, c)
	if v.Show == "" {
		return formula.FromResult(x).String()
	}
	f, ok := cb.num(vi, r, c)
	if !ok {
		return formula.FromResult(x).String()
	}
	ratio := func(a, b float64, okb bool) string {
		if !okb || b == 0 {
			return ""
		}
		return percent(a / b)
	}
	number := func(n float64) string { return formula.FromResult(n).String() }
	switch v.Show {
	case "pctTotal":
		b, okb := cb.num(vi, nil, nil)
		return ratio(f, b, okb)
	case "pctCol":
		b, okb := cb.num(vi, nil, c)
		return ratio(f, b, okb)
	case "pctRow":
		b, okb := cb.num(vi, r, nil)
		return ratio(f, b, okb)
	case "pctParentRow":
		if len(r) == 0 {
			return ratio(f, f, true)
		}
		b, okb := cb.num(vi, r[:len(r)-1], c)
		return ratio(f, b, okb)
	case "pctParentCol":
		if len(c) == 0 {
			return ratio(f, f, true)
		}
		b, okb := cb.num(vi, r, c[:len(c)-1])
		return ratio(f, b, okb)
	case "index":
		g, ok1 := cb.num(vi, nil, nil)
		rt, ok2 := cb.num(vi, r, nil)
		ct, ok3 := cb.num(vi, nil, c)
		if !ok1 || !ok2 || !ok3 || rt*ct == 0 {
			return ""
		}
		return number(f * g / (rt * ct))
	}
	// Calculs le long du champ de base.
	b := cb.bases[vi]
	if b.level < 0 {
		return ""
	}
	path, axis := r, 0
	if b.col {
		path, axis = c, 1
	}
	i := b.level
	if len(path) <= i {
		return "" // total, ou groupe au-dessus du champ de base
	}
	// at : la cellule du même endroit pour l'élément s du champ de base.
	at := func(s string) (float64, bool) {
		p := append([]string{}, path...)
		p[i] = s
		if axis == 0 {
			return cb.num(vi, p, c)
		}
		return cb.num(vi, r, p)
	}
	sibs := cb.children(axis, path[:i])
	switch v.Show {
	case "pctParent":
		p := path[:i+1]
		if axis == 0 {
			bv, okb := cb.num(vi, p, c)
			return ratio(f, bv, okb)
		}
		bv, okb := cb.num(vi, r, p)
		return ratio(f, bv, okb)
	case "pctOf", "diff", "pctDiff":
		item := v.BaseItem
		if v.BasePos == "prev" || v.BasePos == "next" {
			item = ""
			for k, s := range sibs {
				if s != path[i] {
					continue
				}
				if v.BasePos == "prev" && k > 0 {
					item = sibs[k-1]
				}
				if v.BasePos == "next" && k+1 < len(sibs) {
					item = sibs[k+1]
				}
			}
		}
		if item == "" {
			return ""
		}
		if item == path[i] {
			if v.Show == "pctOf" {
				return ratio(f, f, true)
			}
			return ""
		}
		bv, okb := at(item)
		if !okb {
			return ""
		}
		switch v.Show {
		case "pctOf":
			return ratio(f, bv, true)
		case "diff":
			return number(f - bv)
		}
		return ratio(f-bv, bv, true)
	case "runTotal", "pctRunTotal":
		run, all, done := 0.0, 0.0, false
		for _, s := range sibs {
			x, okx := at(s)
			if !okx {
				x = 0
			}
			all += x
			if !done {
				run += x
			}
			if s == path[i] {
				done = true
			}
		}
		if v.Show == "runTotal" {
			return number(run)
		}
		return ratio(run, all, true)
	case "rankAsc", "rankDesc":
		rank := 1
		for _, s := range sibs {
			x, okx := at(s)
			if okx && ((v.Show == "rankAsc" && x < f) || (v.Show == "rankDesc" && x > f)) {
				rank++
			}
		}
		return number(float64(rank))
	}
	return ""
}
