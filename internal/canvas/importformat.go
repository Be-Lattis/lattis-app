package canvas

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// Mise en forme d'un classeur importé (fichier .xlsx ou .ods) : styles des
// cellules, des lignes et des colonnes, largeurs et hauteurs, onglet, volets
// figés, mise en forme conditionnelle. Les feuilles et les tableaux étant
// neufs, les styles s'écrivent en bloc (une instruction par table de styles,
// les lignes en JSON) et s'annulent par une suppression : 100 000 cellules
// mises en forme ne font pas 100 000 entrées dans le journal.

// ImportStyle est la mise en forme d'une cellule : cellule libre (Row, Col
// de la feuille) ou cellule de tableau (Row : rang de la ligne de données,
// 0 la première ; Col : rang de la colonne).
type ImportStyle struct {
	Row, Col int
	Style    Style
}

// ImportLine est la mise en forme d'une ligne ou d'une colonne entière.
type ImportLine struct {
	Index int
	Style Style
}

// ImportLineRules : règles conditionnelles d'une ligne ou d'une colonne.
type ImportLineRules struct {
	Index int
	Rules []Rule
}

// ImportFormat est la mise en forme d'une feuille importée.
type ImportFormat struct {
	Color      string // onglet (#rrggbb)
	Hidden     bool
	FreezeRows int
	FreezeCols int
	Zoom       int
	Widths     []ColWidth
	Heights    []RowHeight
	RowStyles  []ImportLine
	ColStyles  []ImportLine
	// Cells : cellules libres (fusions mr/mc, notes nt, validations dv et
	// règles cf comprises).
	Cells    []ImportStyle
	RowRules []ImportLineRules
	ColRules []ImportLineRules
}

// TableFormat est la mise en forme d'un tableau importé : par colonne, son
// en-tête et toutes ses lignes ; puis les cellules qui s'en écartent, et
// les règles conditionnelles de chaque colonne.
type TableFormat struct {
	Head  []Style
	Cols  []Style
	Cells []ImportStyle
	Rules [][]Rule
}

// bulkRows insère des lignes en une instruction (JSON), undo les retire.
func (t *tx) bulkRows(table string, cols []string, rows [][]any, undo stmt) error {
	if len(rows) == 0 {
		return nil
	}
	sel := make([]string, len(cols))
	for i := range cols {
		sel[i] = fmt.Sprintf("json_extract(value, '$[%d]')", i)
	}
	q := `INSERT INTO ` + table + ` (` + strings.Join(cols, ", ") + `) SELECT ` + strings.Join(sel, ", ") + ` FROM json_each(?)`
	var do []stmt
	const chunk = 50_000
	for i := 0; i < len(rows); i += chunk {
		b, err := json.Marshal(rows[i:min(i+chunk, len(rows))])
		if err != nil {
			return err
		}
		do = append(do, st(q, string(b)))
	}
	return t.apply(do, []stmt{undo})
}

// encodeImported encode un style (vide : "").
func encodeImported(s Style) string {
	if len(s) == 0 {
		return ""
	}
	out, err := encodeStyle(s)
	if err != nil {
		return ""
	}
	return out
}

// validCF garde les règles valides de la propriété cf de chaque cellule ;
// rend le nombre de règles retirées.
func validCF(f *ImportFormat) int {
	if f == nil {
		return 0
	}
	n := 0
	for _, c := range f.Cells {
		rs, ok := c.Style["cf"].([]Rule)
		if !ok {
			continue
		}
		var out []Rule
		for _, r := range rs {
			if checkRules([]Rule{r}) == nil {
				out = append(out, r)
			}
		}
		n += len(rs) - len(out)
		if len(out) == 0 {
			delete(c.Style, "cf")
		} else {
			c.Style["cf"] = out
		}
	}
	return n
}

// importFormat pose la mise en forme d'une feuille neuve.
func (t *tx) importFormat(sheet int64, f *ImportFormat) error {
	if f == nil {
		return nil
	}
	color := ""
	if hexColor.MatchString(f.Color) {
		color = f.Color
	}
	fr, fc := min(max(f.FreezeRows, 0), 1000), min(max(f.FreezeCols, 0), 1000)
	zoom := f.Zoom
	if zoom != 0 {
		zoom = min(max(zoom, 10), 400)
	}
	if color != "" || f.Hidden || fr > 0 || fc > 0 || zoom != 0 && zoom != 100 {
		var oz int
		if err := t.row(`SELECT zoom FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&oz); err != nil {
			return err
		}
		if zoom == 0 {
			zoom = oz
		}
		q := `UPDATE _lattis_sheets SET color = ?, hidden = ?, freeze_rows = ?, freeze_cols = ?, zoom = ? WHERE id = ?`
		if err := t.apply([]stmt{st(q, color, f.Hidden, fr, fc, zoom, sheet)}, []stmt{st(q, "", false, 0, 0, oz, sheet)}); err != nil {
			return err
		}
	}
	var rows [][]any
	for _, w := range f.Widths {
		if w.Col >= 0 && w.Width > 0 {
			rows = append(rows, []any{sheet, w.Col, w.Width})
		}
	}
	if err := t.bulkRows(`_lattis_col_widths`, []string{"sheet", "col", "width"}, rows, st(`DELETE FROM _lattis_col_widths WHERE sheet = ?`, sheet)); err != nil {
		return err
	}
	rows = nil
	for _, h := range f.Heights {
		if h.Row >= 0 && h.Height > 0 {
			rows = append(rows, []any{sheet, h.Row, h.Height})
		}
	}
	if err := t.bulkRows(`_lattis_row_heights`, []string{"sheet", "row", "height"}, rows, st(`DELETE FROM _lattis_row_heights WHERE sheet = ?`, sheet)); err != nil {
		return err
	}
	for _, q := range []struct {
		table, at string
		lines     []ImportLine
	}{{`_lattis_row_styles`, "row", f.RowStyles}, {`_lattis_col_styles`, "col", f.ColStyles}} {
		rows = nil
		seen := map[int]bool{}
		for _, l := range q.lines {
			if s := encodeImported(l.Style); s != "" && l.Index >= 0 && !seen[l.Index] {
				seen[l.Index] = true
				rows = append(rows, []any{sheet, l.Index, s})
			}
		}
		if err := t.bulkRows(q.table, []string{"sheet", q.at, "style"}, rows, st(`DELETE FROM `+q.table+` WHERE sheet = ?`, sheet)); err != nil {
			return err
		}
	}
	rows = nil
	seen := map[[2]int]bool{}
	for _, c := range f.Cells {
		k := [2]int{c.Row, c.Col}
		if s := encodeImported(c.Style); s != "" && c.Row >= 0 && c.Col >= 0 && !seen[k] {
			seen[k] = true
			rows = append(rows, []any{sheet, c.Row, c.Col, s})
		}
	}
	return t.bulkRows(`_lattis_cell_styles`, []string{"sheet", "row", "col", "style"}, rows, st(`DELETE FROM _lattis_cell_styles WHERE sheet = ?`, sheet))
}

// importTableFormat pose la mise en forme d'un tableau neuf (ses lignes ont
// pour identifiant leur rang + 1, voir importTable).
func (t *tx) importTableFormat(tb Table, f *TableFormat) error {
	if f == nil {
		return nil
	}
	var rows [][]any
	add := func(rid int64, col int, s Style) {
		if col < 0 || col >= len(tb.Cols) {
			return
		}
		if js := encodeImported(s); js != "" {
			rows = append(rows, []any{tb.ID, rid, tb.Cols[col].ID, js})
		}
	}
	for j, s := range f.Head {
		add(-1, j, s)
	}
	for j, s := range f.Cols {
		add(0, j, s)
	}
	for _, c := range f.Cells {
		if c.Row >= 0 && c.Row < tb.Rows {
			add(int64(c.Row+1), c.Col, c.Style)
		}
	}
	return t.bulkRows(`_lattis_styles`, []string{"tbl", "rid", "col", "style"}, rows, st(`DELETE FROM _lattis_styles WHERE tbl = ?`, tb.ID))
}

// importRules pose les règles conditionnelles importées : colonnes des
// tableaux puis lignes et colonnes de la feuille ; une règle refusée (formule
// illisible) est passée et rend un avertissement.
func (m *Model) importRules(t *tx, sheet int64, name string, tables []Table, fmts []*TableFormat, f *ImportFormat, dropped int) ([]string, error) {
	var warn []string
	keep := func(rs []Rule) []Rule {
		var out []Rule
		for _, r := range rs {
			if checkRules([]Rule{r}) == nil {
				out = append(out, r)
			} else {
				dropped++
			}
		}
		return out
	}
	for i, tf := range fmts {
		if tf == nil {
			continue
		}
		for j, rs := range tf.Rules {
			if rs = keep(rs); len(rs) == 0 || j >= len(tables[i].Cols) {
				continue
			}
			if err := t.setColumnRules(tables[i], j, rs); err != nil {
				if !userError(err) {
					return nil, err
				}
				dropped += len(rs)
			}
		}
	}
	if f != nil {
		for _, q := range []struct {
			rows  bool
			lines []ImportLineRules
		}{{true, f.RowRules}, {false, f.ColRules}} {
			for _, l := range q.lines {
				if rs := keep(l.Rules); len(rs) > 0 && l.Index >= 0 {
					if err := t.setLineRules(sheet, q.rows, l.Index, l.Index, rs); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if dropped > 0 {
		warn = append(warn, m.say("import.rulesDropped", msg.Args{"sheet": name, "n": dropped}))
	}
	return warn, nil
}
