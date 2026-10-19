package canvas

import (
	"github.com/lalamefine/lattis/internal/formula"
)

// MaxSpillRows borne le nombre de lignes déversées par une cellule.
const MaxSpillRows = 10000

// ErrSpillText est la valeur d'une cellule dont le tableau dynamique ne peut
// pas se déverser (cellules déjà remplies ou tableau sur le chemin).
const ErrSpillText = "#DEVERSEMENT!"

// spillFree calcule une cellule libre dont la formule est un tableau dynamique
// (FILTRE, TRIER, UNIQUE…) ou une colonne entière d'un tableau
// (=Ventes[Montant], voir formula.IsFreeSpill). ok est faux si la formule
// n'en est pas un.
func (t *tx) spillFreeRaw(sheet int64, raw string) (rows [][]string, ok bool, errV formula.Value) {
	n, err := formula.Parse(raw, formula.FR)
	if err != nil {
		return nil, false, formula.Value{}
	}
	_, column := n.(formula.Ref)
	if !column && !formula.IsSpill(n) {
		return nil, false, formula.Value{}
	}
	base, err := t.schema()
	if err != nil {
		return nil, true, formula.Err(formula.ErrValue)
	}
	// Tableaux de la feuille filtrés par son onglet à choix (schemaFor).
	schema, err := t.schemaFor(sheet)
	if err != nil {
		return nil, true, formula.Err(formula.ErrValue)
	}
	if column && !formula.IsFreeSpill(n, schema) {
		return nil, false, formula.Value{} // [Col], colonne inconnue (#REF!)
	}
	if n, _, err = formula.Convert(n, schema, noRanges{}, raw); err != nil {
		return nil, true, formula.Err(formula.ErrName)
	}
	n = formula.QualifyCells(n, sheet)
	names, err := t.nameValues(base)
	if err != nil {
		return nil, true, formula.Err(formula.ErrValue)
	}
	vals, e := t.spillRows(n, schema, names)
	if e.IsErr() {
		return nil, true, e
	}
	for _, r := range vals {
		row := make([]string, len(r))
		for i, v := range r {
			row[i] = v.String()
		}
		rows = append(rows, row)
	}
	return rows, true, formula.Value{}
}

// spillRows calcule un tableau dynamique (formule déjà rattachée à sa
// feuille) avec les valeurs des noms et cellules names.
func (t *tx) spillRows(n formula.Node, schema *formula.Schema, names map[string]formula.Value) ([][]formula.Value, formula.Value) {
	hoist, _ := t.hoister()
	sp, e := formula.CompileSpillHoisted(n, schema, names, hoist)
	if e.IsErr() {
		return nil, e
	}
	rs, err := t.query(`SELECT * FROM (`+sp.SQL+`) LIMIT ?`, append(sp.Args, MaxSpillRows)...)
	if err != nil {
		return nil, formula.Err(formula.ErrValue)
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, formula.Err(formula.ErrValue)
	}
	var rows [][]formula.Value
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, formula.Err(formula.ErrValue)
		}
		row := make([]formula.Value, len(cols))
		for i, v := range vals {
			row[i] = formula.FromResult(v)
		}
		rows = append(rows, row)
	}
	if rs.Err() != nil {
		return nil, formula.Err(formula.ErrValue)
	}
	return rows, formula.Value{}
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
