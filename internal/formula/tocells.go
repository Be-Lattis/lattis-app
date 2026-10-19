package formula

import (
	"math"
	"strings"
)

// Conversion d'un tableau en plage (comme « Convertir en plage » d'Excel) :
// les références structurées au tableau converti deviennent des adresses de
// cellules libres, quand la traduction est simple et sûre.

// TableCells décrit où sont tombées les cellules d'un tableau converti en
// plage.
type TableCells struct {
	Table string         // nom du tableau converti
	Sheet string         // feuille du tableau, "" : celle de la formule
	Row   int            // ligne de l'en-tête ; les données suivent
	Rows  int            // lignes de données
	Cols  map[string]int // nom de colonne (minuscules) → colonne de la feuille
	// Self : formule d'une colonne calculée du tableau converti, posée sur
	// la ligne At de la feuille ([@Col] y désigne la cellule de cette ligne).
	Self bool
	At   int
}

// ToCells réécrit les références au tableau décrit par tc en adresses de
// cellules : [@Col] → Cell[B3] (ligne de la formule), Tableau[Col] dans un
// agrégat ou un parcours → Cell[B2:B9], Tableau[Col][k] → Cell[Bk]. Faux si
// une référence au tableau ne se traduit pas simplement (recherche dans le
// tableau, calculs entre lignes, tableau dynamique, colonne entière hors
// agrégat…) : la formule ne doit pas être réécrite.
func ToCells(n Node, tc TableCells) (Node, bool) {
	return toCells(n, tc, false)
}

func (tc TableCells) own(r Ref) bool {
	if r.Table == "" {
		return tc.Self
	}
	return strings.EqualFold(r.Table, tc.Table)
}

// cell donne l'adresse d'une cellule de la plage (qualifiée par la feuille).
func (tc TableCells) cell(row, col int) Node {
	name := CellName(row, col)
	if tc.Sheet != "" {
		name = tc.Sheet + "." + name
	}
	return Name{name}
}

// span donne les lignes de données from à to (rangs à partir de 1) d'une
// colonne.
func (tc TableCells) span(col, from, to int) (Node, bool) {
	if from < 1 || to > tc.Rows || from > to || to-from+1 > MaxRangeCells {
		return nil, false
	}
	if from == to {
		return tc.cell(tc.Row+from, col), true
	}
	return CellRange{Sheet: tc.Sheet, R1: tc.Row + from, C1: col, R2: tc.Row + to, C2: col}, true
}

func (tc TableCells) col(name string) (int, bool) {
	c, ok := tc.Cols[strings.ToLower(strings.TrimSpace(name))]
	return c, ok
}

// toCells : inAgg vrai dans les arguments d'un agrégat ou d'un parcours, où
// une colonne entière se lit comme une plage.
func toCells(n Node, tc TableCells, inAgg bool) (Node, bool) {
	switch v := n.(type) {
	case Ref:
		if !tc.own(v) {
			return v, true
		}
		col, ok := tc.col(v.Col)
		if !ok {
			return nil, false
		}
		if v.Row {
			if !tc.Self {
				return nil, false
			}
			return tc.cell(tc.At, col), true
		}
		if !inAgg {
			return nil, false
		}
		return tc.span(col, 1, tc.Rows)
	case RowRange:
		if !tc.own(v.Ref) {
			return v, true
		}
		col, ok := tc.col(v.Ref.Col)
		if !ok || !inAgg {
			return nil, false
		}
		return tc.span(col, v.From, v.To)
	case Name:
		if strings.EqualFold(v.Name, tc.Table) {
			return nil, false
		}
		return v, true
	case Unary:
		x, ok := toCells(v.X, tc, inAgg)
		if !ok {
			return nil, false
		}
		v.X = x
		return v, true
	case Binary:
		l, ok := toCells(v.L, tc, inAgg)
		if !ok {
			return nil, false
		}
		r, ok := toCells(v.R, tc, inAgg)
		if !ok {
			return nil, false
		}
		v.L, v.R = l, r
		return v, true
	case Call:
		if !mentions(v, tc) {
			return v, true
		}
		if strings.EqualFold(v.Fn, "ITEM") && len(v.Args) == 2 {
			// Tableau[Col][k] : la k-ième ligne de données.
			if r, ok := v.Args[0].(Ref); ok && tc.own(r) {
				col, ok := tc.col(r.Col)
				k, isNum := v.Args[1].(NumLit)
				if !ok || !isNum || k.V != math.Trunc(k.V) {
					return nil, false
				}
				return tc.span(col, int(k.V), int(k.V))
			}
		}
		fn, known := lookupFunc(v.Fn)
		agg := false
		if known {
			switch fn.kind {
			case aggregate:
				agg = true
			case scan:
				if fn.EN == "RANK" {
					return nil, false
				}
				agg = true
			case special:
				// Une recherche peut prendre une valeur de la ligne pour clé
				// (REFERENCE(Autre; [@Code]; …)), pas une colonne du tableau.
				switch fn.EN {
				case "REFERENCE", "REFERENCE.APPROX", "EXISTS":
				default:
					return nil, false
				}
			case spill:
				return nil, false
			}
		}
		args := make([]Node, len(v.Args))
		for i, a := range v.Args {
			x, ok := toCells(a, tc, agg)
			if !ok {
				return nil, false
			}
			args[i] = x
		}
		v.Args = args
		return v, true
	}
	return n, true
}

// mentions : la formule cite-t-elle le tableau ?
func mentions(n Node, tc TableCells) bool {
	switch v := n.(type) {
	case Ref:
		return tc.own(v)
	case RowRange:
		return tc.own(v.Ref)
	case Name:
		return strings.EqualFold(v.Name, tc.Table)
	case Unary:
		return mentions(v.X, tc)
	case Binary:
		return mentions(v.L, tc) || mentions(v.R, tc)
	case Call:
		for _, a := range v.Args {
			if mentions(a, tc) {
				return true
			}
		}
	}
	return false
}

// Mentions dit si une formule cite le tableau décrit par tc.
func Mentions(n Node, tc TableCells) bool { return mentions(n, tc) }
