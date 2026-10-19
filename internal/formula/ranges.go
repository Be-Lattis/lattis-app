package formula

import (
	"fmt"
	"strings"
)

// Plages : Cell[A1:B3] (cellules libres) et Tableau[Col][1:3] (lignes 1 à 3
// d'une colonne, par rang d'affichage). Elles se lisent comme une colonne de
// tableau dans les agrégats (SOMME, MOYENNE, NB, MAX, MIN…) et dans les
// fonctions de parcours (SOMME.SI, NB.SI, SOMMEPROD, EQUIV…), seules ou dans
// une expression (Cell[A1:A3]*Cell[B1:B3]). Plusieurs plages lues ensemble
// doivent avoir la même forme ; une plage ne se mêle pas à une colonne
// entière de tableau (#VALEUR!).

// walkCols appelle f sur chaque lecture de colonne d'une expression sur
// colonnes : Tableau[Col] entier, Tableau[Col][1:3] et Cell[A1:B3]. Les
// agrégats et parcours imbriqués lisent leurs propres colonnes : seuls leurs
// paramètres sont visités.
func walkCols(n Node, f func(Node)) {
	switch n := n.(type) {
	case Ref:
		if !n.Row {
			f(n)
		}
	case RowRange, CellRange:
		f(n)
	case Unary:
		walkCols(n.X, f)
	case Binary:
		walkCols(n.L, f)
		walkCols(n.R, f)
	case Call:
		fn, ok := lookupFunc(n.Fn)
		if !ok {
			return
		}
		args := n.Args
		switch fn.kind {
		case spill:
			return
		case aggregate:
			args = args[:min(aggLead[fn.EN], len(args))]
		case scan:
			rs, ok := scans[fn.EN].roles(len(args))
			if !ok || fn.EN == "RANK" {
				return
			}
			var ps []Node
			for i, a := range args {
				if rs[i] == param {
					ps = append(ps, a)
				}
			}
			args = ps
		case special:
			switch fn.EN {
			case "REFERENCE", "REFERENCE.APPROX":
				args = args[min(2, len(args)):]
			case "EXISTS":
				args = args[min(1, len(args)):]
			case "SUBTOTAL", "VLOOKUP", "XLOOKUP", "HLOOKUP", "ROW", "COLUMN", "ROWS", "COLUMNS":
				return // ROW… ne lisent que la forme de leur référence
			}
		}
		for _, a := range args {
			walkCols(a, f)
		}
	}
}

// span décrit ce que parcourt une expression qui lit des plages : les lignes
// From à To d'un tableau, ou les positions de plages de cellules de même
// forme.
type span struct {
	table    *Table
	from, to int
	ranges   []CellRange
}

// rows donne le nombre de positions parcourues par des plages de cellules.
func (s span) positions() int {
	if len(s.ranges) == 0 {
		return 0
	}
	r, c := s.ranges[0].Size()
	return r * c
}

// cells dit si le parcours lit des cellules libres (et non des lignes de
// tableau) : leurs textes sont des libellés, ignorés par les fonctions
// numériques comme dans Excel (décision 0013).
func (s span) cells() bool { return s.table == nil && len(s.ranges) > 0 }

// index donne le rang d'une plage dans s.ranges, ou -1.
func (s span) index(r CellRange) int {
	for i, x := range s.ranges {
		if x == r {
			return i
		}
	}
	return -1
}

// spanOf cherche les plages lues par des expressions sur colonnes. ok est
// faux si elles n'en lisent aucune (le parcours ordinaire s'applique). Des
// plages de formes différentes, ou mêlées à des colonnes entières, donnent
// #VALEUR!.
func spanOf(nodes []Node, s *Schema, current *Table) (sp span, ok bool, e ErrCode) {
	var whole bool // une colonne entière de tableau est lue
	var rows bool
	for _, n := range nodes {
		walkCols(n, func(x Node) {
			switch x := x.(type) {
			case Ref:
				whole = true
			case RowRange:
				t := current
				if x.Ref.Table != "" && s != nil {
					t, _ = s.Table(x.Ref.Table)
				}
				if t == nil {
					e = ErrRef
					return
				}
				if rows && (sp.table != t || sp.from != x.From || sp.to != x.To) {
					e = ErrValue
				}
				rows, sp.table, sp.from, sp.to = true, t, x.From, x.To
			case CellRange:
				if len(sp.ranges) > 0 {
					r0, c0 := sp.ranges[0].Size()
					if r1, c1 := x.Size(); r0 != r1 || c0 != c1 {
						e = ErrValue
					}
				}
				if sp.index(x) < 0 {
					sp.ranges = append(sp.ranges, x)
				}
			}
		})
	}
	if !rows && len(sp.ranges) == 0 {
		return span{}, false, 0
	}
	if e == 0 && (whole || rows && len(sp.ranges) > 0) {
		e = ErrValue
	}
	return sp, true, e
}

// rangeValues donne les valeurs des cellules d'une plage (vides si absentes).
func rangeValues(r CellRange, names map[string]Value, home int64) []Value {
	keys := r.keys(home)
	out := make([]Value, len(keys))
	for i, k := range keys {
		out[i] = names[k]
	}
	return out
}

// --- SQL ----------------------------------------------------------------------

// spanSource donne la source SQL d'un parcours de plages : les lignes du
// tableau restreintes à leur rang, ou une table de valeurs (VALUES) dont la
// colonne v<i> est la plage n° i. Les lignes gardent un _pos pour l'ordre.
func (c *compiler) spanSource(sp span, sc scope) string {
	if sp.table != nil {
		return fmt.Sprintf("(SELECT * FROM %s ORDER BY _pos LIMIT %d OFFSET %d)", sp.table.SQL, sp.to-sp.from+1, sp.from-1)
	}
	var home int64
	if sc.table != nil {
		home = sc.table.Sheet
	}
	cols := make([][]Value, len(sp.ranges))
	for i, r := range sp.ranges {
		cols[i] = rangeValues(r, c.names, home)
	}
	var b strings.Builder
	b.WriteString("(SELECT column1 AS _pos")
	for i := range sp.ranges {
		fmt.Fprintf(&b, ", column%d AS v%d", i+2, i)
	}
	b.WriteString(" FROM (VALUES ")
	for p := range sp.positions() {
		if p > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "(%d", p)
		for i := range sp.ranges {
			v := cols[i][p]
			if v.Kind == Empty {
				b.WriteString(", NULL")
			} else {
				b.WriteString(", " + c.param(ToSQL(v)))
			}
		}
		b.WriteString(")")
	}
	b.WriteString("))")
	return b.String()
}

// inSpan donne la portée d'une expression évaluée sur chaque position d'un
// parcours de plages (alias de la source).
func (sc scope) inSpan(sp span, alias string) scope {
	out := sc.inArray(sp.table, alias)
	out.ranges = sp.ranges
	return out
}

// rowValue compile Tableau[Col][k:k] hors d'un parcours : la valeur de la
// ligne de rang k.
func (c *compiler) rowValue(n RowRange, sc scope) string {
	if n.From != n.To {
		return errSQL(ErrValue)
	}
	t, col, e := c.resolve(n.Ref, sc)
	if e != 0 {
		return errSQL(e)
	}
	al := c.newAlias()
	src := fmt.Sprintf("(SELECT * FROM %s ORDER BY _pos LIMIT 1 OFFSET %d)", t.SQL, n.From-1)
	// Ligne absente : #REF!, comme l'évaluateur Go.
	return fmt.Sprintf("slt_pick(EXISTS (SELECT 1 FROM %s), (SELECT %s FROM %s AS %s), %s)", src, c.column(t, col, al), src, al, errSQL(ErrRef))
}

// --- Go ------------------------------------------------------------------------

// eachSpan appelle f avec le contexte de chaque position d'un parcours.
func (env *Env) eachSpan(sp span, f func(*Env)) {
	if sp.table != nil {
		rows := len(env.Data[sp.table])
		for r := sp.from - 1; r < min(sp.to, rows); r++ {
			f(env.inArray(sp.table, r))
		}
		return
	}
	var home int64
	if env.Table != nil {
		home = env.Table.Sheet
	}
	vals := make([][]Value, len(sp.ranges))
	for i, r := range sp.ranges {
		vals[i] = rangeValues(r, env.Names, home)
	}
	for p := range sp.positions() {
		sub := *env
		sub.arr, sub.ranges, sub.rangeVals, sub.rangePos = nil, sp.ranges, vals, p
		f(&sub)
	}
}

// rowValue évalue Tableau[Col][k:k] hors d'un parcours.
func (env *Env) rowValue(n RowRange) Value {
	if n.From != n.To {
		return Err(ErrValue)
	}
	t, i, err := env.resolve(n.Ref)
	if err.IsErr() {
		return err
	}
	return env.cell(t, n.From-1, i)
}
