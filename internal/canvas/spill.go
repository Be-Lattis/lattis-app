package canvas

import (
	"github.com/lalamefine/slitable/internal/formula"
)

// MaxSpillRows borne le nombre de lignes déversées par une cellule.
const MaxSpillRows = 10000

// ErrSpillText est la valeur d'une cellule dont le tableau dynamique ne peut
// pas se déverser (cellules déjà remplies ou tableau sur le chemin).
const ErrSpillText = "#DEVERSEMENT!"

// spillFree calcule une cellule libre dont la formule est un tableau dynamique
// (FILTRE, TRIER, UNIQUE…). ok est faux si la formule n'en est pas un.
func (t *tx) spillFreeRaw(sheet int64, raw string) (rows [][]string, ok bool, errV formula.Value) {
	n, err := formula.Parse(raw, formula.FR)
	if err != nil || !formula.IsSpill(n) {
		return nil, false, formula.Value{}
	}
	schema, err := t.schema()
	if err != nil {
		return nil, true, formula.Err(formula.ErrValue)
	}
	if n, _, err = formula.Convert(n, schema, noRanges{}, raw); err != nil {
		return nil, true, formula.Err(formula.ErrName)
	}
	n = formula.QualifyCells(n, sheet)
	names, err := t.nameValues(schema)
	if err != nil {
		return nil, true, formula.Err(formula.ErrValue)
	}
	sp, e := formula.CompileSpill(n, schema, names)
	if e.IsErr() {
		return nil, true, e
	}
	rs, err := t.query(`SELECT * FROM (`+sp.SQL+`) LIMIT ?`, append(sp.Args, MaxSpillRows)...)
	if err != nil {
		return nil, true, formula.Err(formula.ErrValue)
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, true, formula.Err(formula.ErrValue)
	}
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, true, formula.Err(formula.ErrValue)
		}
		row := make([]string, len(cols))
		for i, v := range vals {
			row[i] = formula.FromResult(v).String()
		}
		rows = append(rows, row)
	}
	if rs.Err() != nil {
		return nil, true, formula.Err(formula.ErrValue)
	}
	return rows, true, formula.Value{}
}

// spillBlocked indique si le déversement de rows depuis (row, col) recouvre
// une autre cellule remplie ou un tableau.
func spillBlocked(row, col int, rows [][]string, cells []cell, tables []Table) bool {
	if len(rows) == 0 {
		return false
	}
	r1, c1 := row+len(rows)-1, col+len(rows[0])-1
	for _, c := range cells {
		if (c.Row != row || c.Col != col) && c.Row >= row && c.Row <= r1 && c.Col >= col && c.Col <= c1 {
			return true
		}
	}
	for _, tb := range tables {
		if tb.Row <= r1 && tb.Bottom() >= row && tb.Col <= c1 && tb.Right() >= col {
			return true
		}
	}
	return false
}
