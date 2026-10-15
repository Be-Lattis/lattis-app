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
	Name  string    `json:"name"`
	SQL   string    `json:"sql"`
	Type  grid.Type `json:"type"`
	Fixed bool      `json:"fixed"`
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
}

// Layout est la mise en page d'une feuille : ses tableaux et ses cellules
// libres. Les valeurs des tableaux se lisent ensuite par fenêtres.
type Layout struct {
	Sheets []SheetInfo `json:"sheets"`
	Sheet  int64       `json:"sheet"`
	Tables []TableInfo `json:"tables"`
	Cells  []CellInfo  `json:"cells"`
}

// Layout lit la mise en page d'une feuille.
func (m *Model) Layout(ctx context.Context, sheet int64) (Layout, error) {
	l := Layout{Sheet: sheet, Tables: []TableInfo{}, Cells: []CellInfo{}}
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
			for _, c := range tb.Cols {
				ti.Columns = append(ti.Columns, ColumnInfo{Name: c.Name, SQL: colSQL(c.ID), Type: c.Type, Fixed: c.Fixed})
			}
			l.Tables = append(l.Tables, ti)
		}
		cells, err := t.cells(sheet)
		if err != nil {
			return err
		}
		for _, c := range cells {
			l.Cells = append(l.Cells, CellInfo{c.Row, c.Col, c.Raw})
		}
		return nil
	})
	return l, err
}
