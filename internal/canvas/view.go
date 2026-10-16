package canvas

import (
	"context"
	"encoding/json"

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
	// Mise en forme de toute la colonne et de son en-tête (JSON, voir Style).
	Style       string `json:"style"`
	HeaderStyle string `json:"headerStyle"`
	// Mise en forme conditionnelle (JSON : liste de Rule).
	Rules string `json:"rules"`
}

// TableInfo décrit un tableau posé sur une feuille.
type TableInfo struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	SQL   string `json:"sql"`
	Row   int    `json:"row"` // ancre : ligne d'en-tête
	Col   int    `json:"col"`
	Rows  int    `json:"rows"`  // lignes de données affichées (filtre)
	Total int    `json:"total"` // lignes de données, masquées comprises
	// Filtre (JSON : identifiant de colonne → ColumnFilter), vide sans filtre.
	Filter string `json:"filter"`
	// Source SQL des lignes affichées (la table, ou une requête filtrée).
	Source  string       `json:"source"`
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

// NameInfo est une cellule nommée de la feuille.
type NameInfo struct {
	Name string `json:"name"`
	Row  int    `json:"row"`
	Col  int    `json:"col"`
}

// Layout est la mise en page d'une feuille : ses tableaux et ses cellules
// libres. Les valeurs des tableaux se lisent ensuite par fenêtres.
type Layout struct {
	Sheets []SheetInfo `json:"sheets"`
	Sheet  int64       `json:"sheet"`
	Tables []TableInfo `json:"tables"`
	Cells  []CellInfo  `json:"cells"`
	Widths []ColWidth  `json:"widths"` // largeurs choisies par l'utilisateur
	Names  []NameInfo  `json:"names"`  // cellules nommées de la feuille
	Styles []CellStyle `json:"styles"` // mise en forme hors tableaux
}

// CellStyle est la mise en forme d'une cellule hors tableau.
type CellStyle struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Style string `json:"style"`
}

// ColWidth est la largeur choisie pour une colonne de la feuille.
type ColWidth struct {
	Col   int `json:"col"`
	Width int `json:"width"`
}

// Layout lit la mise en page d'une feuille.
func (m *Model) Layout(ctx context.Context, sheet int64) (Layout, error) {
	l := Layout{Sheet: sheet, Tables: []TableInfo{}, Cells: []CellInfo{}, Widths: []ColWidth{}, Names: []NameInfo{}, Styles: []CellStyle{}}
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
			ti := TableInfo{ID: tb.ID, Name: tb.Name, SQL: tb.sql(), Row: tb.Row, Col: tb.Col, Rows: tb.Shown, Total: tb.Rows, Source: tb.view()}
			if len(tb.Filter) > 0 {
				b, err := json.Marshal(tb.Filter)
				if err != nil {
					return err
				}
				ti.Filter = string(b)
			}
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
			colStyles := map[[2]int64]string{}
			rs, err = t.query(`SELECT rid, col, style FROM _slitable_styles WHERE tbl = ? AND rid <= 0`, tb.ID)
			if err != nil {
				return err
			}
			for rs.Next() {
				var rid, col int64
				var style string
				if err := rs.Scan(&rid, &col, &style); err != nil {
					rs.Close()
					return err
				}
				colStyles[[2]int64{rid, col}] = style
			}
			rs.Close()
			for _, c := range tb.Cols {
				rules, err := t.columnRules(tb.ID, c.ID)
				if err != nil {
					return err
				}
				ti.Columns = append(ti.Columns, ColumnInfo{ID: c.ID, Errors: errs[c.ID], Name: c.Name, SQL: colSQL(c.ID), Type: c.Type, Fixed: c.Fixed, Formula: displayFormula(c.Formula),
					Style: colStyles[[2]int64{0, c.ID}], HeaderStyle: colStyles[[2]int64{-1, c.ID}], Rules: rules})
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
		rs, err = t.query(`SELECT name, row, col FROM _slitable_names WHERE sheet = ? ORDER BY name`, sheet)
		if err != nil {
			return err
		}
		for rs.Next() {
			var n NameInfo
			if err := rs.Scan(&n.Name, &n.Row, &n.Col); err != nil {
				rs.Close()
				return err
			}
			l.Names = append(l.Names, n)
		}
		rs.Close()
		rs, err = t.query(`SELECT row, col, style FROM _slitable_cell_styles WHERE sheet = ?`, sheet)
		if err != nil {
			return err
		}
		for rs.Next() {
			var c CellStyle
			if err := rs.Scan(&c.Row, &c.Col, &c.Style); err != nil {
				rs.Close()
				return err
			}
			l.Styles = append(l.Styles, c)
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

// TableNames décrit un tableau pour l'aide à la saisie des formules.
type TableNames struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// AllTables liste les tableaux de toutes les feuilles et leurs colonnes.
func (m *Model) AllTables(ctx context.Context) ([]TableNames, error) {
	var out []TableNames
	err := m.read(ctx, func(t *tx) error {
		rs, err := t.query(`SELECT id FROM _slitable_tables ORDER BY name`)
		if err != nil {
			return err
		}
		var ids []int64
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return err
			}
			ids = append(ids, id)
		}
		rs.Close()
		for _, id := range ids {
			tb, err := t.table(id)
			if err != nil {
				return err
			}
			tn := TableNames{Name: tb.Name}
			for _, c := range tb.Cols {
				tn.Columns = append(tn.Columns, c.Name)
			}
			out = append(out, tn)
		}
		return nil
	})
	return out, err
}
