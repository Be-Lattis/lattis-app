package canvas

import (
	"context"

	"github.com/lalamefine/slitable/internal/grid"
)

// SheetInfo décrit un onglet.
type SheetInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ColumnInfo décrit une colonne de tableau pour l'interface.
type ColumnInfo struct {
	ID     int64     `json:"id"`
	Errors int       `json:"errors"` // saisies hors type
	Name   string    `json:"name"`
	SQL    string    `json:"sql"`
	Type   grid.Type `json:"type"`
	Fixed  bool      `json:"fixed"`
	// Colonne calculée : formule telle que l'utilisateur la lit (français).
	Formula string `json:"formula"`
}

// TableInfo décrit un tableau posé sur une feuille.
type TableInfo struct {
	ID      int64        `json:"id"`
	Name    string       `json:"name"`
	SQL     string       `json:"sql"`
	Row     int          `json:"row"` // ancre : ligne d'en-tête
	Col     int          `json:"col"`
	Rows    int          `json:"rows"` // lignes de données
	Columns []ColumnInfo `json:"columns"`
}

// CellInfo est une cellule libre.
type CellInfo struct {
	Row int    `json:"row"`
	Col int    `json:"col"`
	Raw string `json:"raw"`
	// Cellule avec formule : valeur calculée, affichée à la place de Raw.
	Value string `json:"value"`
	Error bool   `json:"error"`
}

// Layout est la mise en page d'une feuille : ses tableaux et ses cellules
// libres. Les valeurs des tableaux se lisent ensuite par fenêtres.
type Layout struct {
	Sheets []SheetInfo `json:"sheets"`
	Sheet  int64       `json:"sheet"`
	Tables []TableInfo `json:"tables"`
	Cells  []CellInfo  `json:"cells"`
	Widths []ColWidth  `json:"widths"` // largeurs choisies par l'utilisateur
}

// ColWidth est la largeur choisie pour une colonne de la feuille.
type ColWidth struct {
	Col   int `json:"col"`
	Width int `json:"width"`
}

// Layout lit la mise en page d'une feuille.
func (m *Model) Layout(ctx context.Context, sheet int64) (Layout, error) {
	l := Layout{Sheet: sheet, Tables: []TableInfo{}, Cells: []CellInfo{}, Widths: []ColWidth{}}
	err := m.read(ctx, func(t *tx) error {
		rs, err := t.query(`SELECT id, name FROM _slitable_sheets ORDER BY pos, id`)
		if err != nil {
			return err
		}
		for rs.Next() {
			var s SheetInfo
			if err := rs.Scan(&s.ID, &s.Name); err != nil {
				rs.Close()
				return err
			}
			l.Sheets = append(l.Sheets, s)
		}
		rs.Close()
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			ti := TableInfo{ID: tb.ID, Name: tb.Name, SQL: tb.sql(), Row: tb.Row, Col: tb.Col, Rows: tb.Rows}
			errs := map[int64]int{}
			rs, err := t.query(`SELECT col, count(*) FROM _slitable_invalid WHERE tbl = ? GROUP BY col`, tb.ID)
			if err != nil {
				return err
			}
			for rs.Next() {
				var col int64
				var n int
				if err := rs.Scan(&col, &n); err != nil {
					rs.Close()
					return err
				}
				errs[col] = n
			}
			rs.Close()
			for _, c := range tb.Cols {
				ti.Columns = append(ti.Columns, ColumnInfo{ID: c.ID, Errors: errs[c.ID], Name: c.Name, SQL: colSQL(c.ID), Type: c.Type, Fixed: c.Fixed, Formula: displayFormula(c.Formula)})
			}
			l.Tables = append(l.Tables, ti)
		}
		rs, err = t.query(`SELECT col, width FROM _slitable_col_widths WHERE sheet = ? ORDER BY col`, sheet)
		if err != nil {
			return err
		}
		for rs.Next() {
			var w ColWidth
			if err := rs.Scan(&w.Col, &w.Width); err != nil {
				rs.Close()
				return err
			}
			l.Widths = append(l.Widths, w)
		}
		rs.Close()
		cells, err := t.cells(sheet)
		if err != nil {
			return err
		}
		for _, c := range cells {
			ci := CellInfo{Row: c.Row, Col: c.Col, Raw: c.Raw}
			if isFormula(c.Raw) {
				v := t.evalFree(c.Raw)
				ci.Value, ci.Error = v.String(), v.IsErr()
			}
			l.Cells = append(l.Cells, ci)
		}
		return nil
	})
	return l, err
}
