package canvas

import (
	"math"
	"strings"

	"github.com/lalamefine/slitable/internal/formula"
	"github.com/lalamefine/slitable/internal/grid"
)

// Type numérique du résultat d'une colonne calculée, deviné sur sa formule :
// =[Prix]*1,2 est décimal, =[Qté]*2 entier. Une colonne calculée décimale
// s'affiche comme une colonne Décimal (au moins deux décimales sur toutes ses
// lignes) au lieu de « 1,44 / 3 / 0,96 ».

// colLookup donne la colonne d'un tableau ("" : le tableau de la formule).
type colLookup func(table, col string) (Column, bool)

// computedType donne grid.Decimal ou grid.Integer quand le résultat est
// sûrement numérique, "" sinon (texte, date, booléen ou inconnu).
func computedType(n formula.Node, look colLookup) grid.Type {
	return kindOf(n, look, 0)
}

// computedResult devine le type du résultat de la colonne calculée c de tb.
func (t *tx) computedResult(tb Table, c Column) grid.Type {
	n, err := formula.Parse(c.Formula, formula.EN)
	if err != nil {
		return ""
	}
	tables := map[string]Table{strings.ToLower(tb.Name): tb}
	return computedType(n, func(table, col string) (Column, bool) {
		if table == "" {
			table = tb.Name
		}
		o, ok := tables[strings.ToLower(table)]
		if !ok {
			var err error
			if o, err = t.tableByName(table); err != nil {
				return Column{}, false
			}
			tables[strings.ToLower(table)] = o
		}
		for _, oc := range o.Cols {
			if strings.EqualFold(oc.Name, col) {
				return oc, true
			}
		}
		return Column{}, false
	})
}

func numJoin(a, b grid.Type) grid.Type {
	switch {
	case a == "" || b == "":
		return ""
	case a == grid.Decimal || b == grid.Decimal:
		return grid.Decimal
	}
	return grid.Integer
}

func kindOf(n formula.Node, look colLookup, depth int) grid.Type {
	if depth > 8 {
		return ""
	}
	k := func(x formula.Node) grid.Type { return kindOf(x, look, depth+1) }
	switch n := n.(type) {
	case formula.NumLit:
		if n.V == math.Trunc(n.V) {
			return grid.Integer
		}
		return grid.Decimal
	case formula.Ref:
		c, ok := look(n.Table, n.Col)
		if !ok {
			return ""
		}
		if c.Formula != "" {
			if n.Table != "" {
				return "" // colonne calculée d'un autre tableau : ses [Col] ne sont pas les nôtres
			}
			f, err := formula.Parse(c.Formula, formula.EN)
			if err != nil {
				return ""
			}
			return k(f)
		}
		switch c.Type {
		case grid.Integer, grid.Decimal:
			return c.Type
		}
		return ""
	case formula.Unary:
		if n.Op == "%" {
			if k(n.X) == "" {
				return ""
			}
			return grid.Decimal
		}
		return k(n.X)
	case formula.Binary:
		l, r := k(n.L), k(n.R)
		switch n.Op {
		case "+", "-", "*":
			return numJoin(l, r)
		case "/", "^":
			if numJoin(l, r) == "" {
				return ""
			}
			return grid.Decimal
		}
		return ""
	case formula.Call:
		switch strings.ToUpper(n.Fn) {
		case "ITEM":
			if len(n.Args) > 0 {
				return k(n.Args[0])
			}
		case "SUMIF":
			// SOMME.SI(plage; critère; [plage à sommer])
			if len(n.Args) == 3 {
				return k(n.Args[2])
			}
			if len(n.Args) > 0 {
				return k(n.Args[0])
			}
		case "SUMIFS":
			// SOMME.SI.ENS(plage à sommer; plage; critère…)
			if len(n.Args) > 0 {
				return k(n.Args[0])
			}
		case "SUM", "MIN", "MAX", "ABS", "PRODUCT":
			if len(n.Args) == 0 {
				return ""
			}
			t := grid.Integer
			for _, a := range n.Args {
				t = numJoin(t, k(a))
			}
			return t
		case "AVERAGE", "AVERAGEIF", "AVERAGEIFS", "MEDIAN", "STDEV":
			return grid.Decimal
		case "COUNT", "COUNTA", "COUNTBLANK", "COUNTIF", "COUNTIFS", "INT":
			return grid.Integer
		case "ROUND", "ROUNDUP", "ROUNDDOWN":
			if len(n.Args) == 2 {
				if d, ok := n.Args[1].(formula.NumLit); ok && d.V <= 0 {
					return grid.Integer
				}
			}
			return grid.Decimal
		case "IF":
			if len(n.Args) == 3 {
				return numJoin(k(n.Args[1]), k(n.Args[2]))
			}
		case "IFERROR":
			if len(n.Args) == 2 {
				return numJoin(k(n.Args[0]), k(n.Args[1]))
			}
		}
	}
	return ""
}
