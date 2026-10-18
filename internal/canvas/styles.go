package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Style est la mise en forme d'une cellule (JSON compact) : b, i, u, x
// (gras, italique, souligné, barré), ff et fs (police et taille en points),
// fg et bg (couleurs #rrggbb), al (left, center, right), va (top, middle,
// bottom), wr (renvoyer à la ligne), nf (format : "0", "0,00", "%", "€",
// "date", "time", "text"), dp (décimales), ng (sans séparateur de
// milliers), df (motif de date ou d'heure : "jj/mm/aaaa", "hh:mm"…), bt,
// bb, bl, br (bordures) et bc (leur couleur). Notes, fusions, masquage et
// validation sont aussi des propriétés de style : voir cellformat.go.
//
// Un seul système de mise en forme, en couches, de la moins à la plus
// précise (comme Excel : cellule > ligne > colonne) :
//
//	colonne de la feuille   _lattis_col_styles (sheet, col)
//	ligne de la feuille     _lattis_row_styles (sheet, row)
//	colonne d'un tableau    _lattis_styles (tbl, rid = 0, col)
//	en-tête d'un tableau    _lattis_styles (tbl, rid = -1, col)
//	cellule d'un tableau    _lattis_styles (tbl, rid, col)
//	cellule libre ou de TCD _lattis_cell_styles (sheet, row, col)
//
// Une couche plus précise remplace propriété par propriété les couches
// au-dessous. Une propriété à null (JSON) est retirée explicitement : elle
// masque la valeur des couches au-dessous (par exemple une cellule non
// grasse dans une ligne en gras).
type Style map[string]any

// MaxStyleCells borne une mise en forme cellule par cellule.
const MaxStyleCells = 20_000

// MaxStyleLines borne une mise en forme de lignes ou colonnes entières.
const MaxStyleLines = 10_000

// empty indique une valeur qui retire la propriété (false, "", nil).
func empty(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case bool:
		return !v
	case string:
		return v == ""
	}
	return false
}

func parseStyle(s string) Style {
	st := Style{}
	if s != "" {
		if err := json.Unmarshal([]byte(s), &st); err != nil {
			return Style{}
		}
	}
	return st
}

func encodeStyle(s Style) (string, error) {
	if len(s) == 0 {
		return "", nil
	}
	b, err := json.Marshal(s) // encoding/json trie les clés : sortie stable
	return string(b), err
}

// over superpose des couches de style (la dernière l'emporte) ; les
// propriétés retirées (null) restent, pour masquer les couches suivantes.
func over(layers ...Style) Style {
	out := Style{}
	for _, l := range layers {
		for k, v := range l {
			out[k] = v
		}
	}
	return out
}

// merge applique un correctif à un style encodé. Une valeur vide (false,
// "", nil) retire la propriété ; si une couche au-dessous (inh) la définit,
// elle est retirée explicitement (null) pour ne pas réapparaître.
// Renvoie le style encodé ("" s'il ne reste rien).
func merge(old string, patch, inh Style) (string, error) {
	s := parseStyle(old)
	for k, v := range patch {
		switch {
		case !empty(v):
			s[k] = v
		case !empty(inh[k]):
			s[k] = nil
		default:
			delete(s, k)
		}
	}
	return encodeStyle(s)
}

// strip retire d'un style encodé les propriétés du correctif : la cellule
// reprend alors la valeur des couches au-dessous.
func strip(old string, patch Style) (string, error) {
	s := parseStyle(old)
	for k := range patch {
		delete(s, k)
	}
	return encodeStyle(s)
}

// sheetLines est la mise en forme des lignes et colonnes entières d'une
// feuille.
type sheetLines struct {
	rows, cols map[int]Style
}

func (t *tx) sheetLines(sheet int64) (sheetLines, error) {
	l := sheetLines{rows: map[int]Style{}, cols: map[int]Style{}}
	for _, q := range []struct {
		sql string
		to  map[int]Style
	}{
		{`SELECT row, style FROM _lattis_row_styles WHERE sheet = ?`, l.rows},
		{`SELECT col, style FROM _lattis_col_styles WHERE sheet = ?`, l.cols},
	} {
		rs, err := t.query(q.sql, sheet)
		if err != nil {
			return l, err
		}
		for rs.Next() {
			var i int
			var s string
			if err := rs.Scan(&i, &s); err != nil {
				rs.Close()
				return l, err
			}
			q.to[i] = parseStyle(s)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return l, err
		}
	}
	return l, nil
}

// at est le style hérité de la feuille en (row, col) : colonne puis ligne.
func (l sheetLines) at(row, col int) Style { return over(l.cols[col], l.rows[row]) }

// rowsIn superpose les styles des lignes [r0, r1] (pour savoir si une
// propriété est posée sur l'une d'elles).
func (l sheetLines) rowsIn(r0, r1 int) Style {
	out := Style{}
	for r, s := range l.rows {
		if r >= r0 && r <= r1 {
			for k, v := range s {
				if !empty(v) {
					out[k] = v
				}
			}
		}
	}
	return out
}

// anyCol superpose les styles de toutes les colonnes de la feuille.
func (l sheetLines) anyCol() Style {
	out := Style{}
	for _, s := range l.cols {
		for k, v := range s {
			if !empty(v) {
				out[k] = v
			}
		}
	}
	return out
}

// tableStyle lit le style d'une colonne de tableau (rid 0) ou de son
// en-tête (rid -1).
func (t *tx) tableStyle(tbl, rid, col int64) (Style, error) {
	var s string
	err := t.row(`SELECT style FROM _lattis_styles WHERE tbl = ? AND rid = ? AND col = ?`, tbl, rid, col).Scan(&s)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return parseStyle(s), nil
}

// ridsAt donne les lignes (clés techniques) affichées aux lignes [first,
// last] de la feuille.
func (t *tx) ridsAt(tb *Table, first, last int) ([]int64, error) {
	rs, err := t.query(`SELECT id FROM `+tb.view()+` ORDER BY _pos LIMIT ? OFFSET ?`, last-first+1, first-tb.Row-1)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var ids []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rs.Err()
}

// ApplyStyle met en forme le rectangle [r0, r1] × [c0, c1] d'une feuille.
// Dans un tableau, une sélection qui couvre toutes les lignes d'une colonne
// met en forme la colonne entière (y compris les lignes ajoutées ensuite) ;
// sinon chaque cellule suit sa ligne (tri, insertion). Hors tableau (cellules
// libres, tableaux croisés), la mise en forme suit la position. Une seule
// étape d'annulation.
func (m *Model) ApplyStyle(ctx context.Context, sheet int64, r0, c0, r1, c1 int, patch Style) error {
	if (r1-r0+1)*(c1-c0+1) > MaxStyleCells*4 {
		return fmt.Errorf("mise en forme limitée à %d cellules", MaxStyleCells)
	}
	return m.Do(ctx, "Mise en forme", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		lines, err := t.sheetLines(sheet)
		if err != nil {
			return err
		}
		cells := 0
		for c := c0; c <= c1; c++ {
			var tb *Table
			for i := range tables {
				if c >= tables[i].Col && c <= tables[i].Right() && r1 >= tables[i].Row && r0 <= tables[i].Bottom() {
					tb = &tables[i]
				}
			}
			// Cellules libres (et cellules vides hors tableau).
			for r := r0; r <= r1; r++ {
				if tb != nil && r >= tb.Row && r <= tb.Bottom() {
					continue // traité avec le tableau
				}
				if cells++; cells > MaxStyleCells {
					return fmt.Errorf("mise en forme limitée à %d cellules", MaxStyleCells)
				}
				if err := t.styleFree(sheet, r, c, patch, lines.at(r, c)); err != nil {
					return err
				}
			}
			if tb == nil {
				continue
			}
			colID := tb.Cols[c-tb.Col].ID
			if r0 <= tb.Row && tb.Row <= r1 {
				if err := t.styleTable(tb.ID, -1, colID, patch, lines.at(tb.Row, c)); err != nil { // en-tête
					return err
				}
			}
			first, last := max(r0, tb.Row+1), min(r1, tb.Bottom())
			if first > last {
				continue
			}
			if first == tb.Row+1 && last == tb.Bottom() {
				// Toute la colonne : style de colonne, et les styles de cellule
				// pour les mêmes propriétés s'effacent.
				if err := t.styleTableColumn(tb, colID, c, patch, lines); err != nil {
					return err
				}
				continue
			}
			ids, err := t.ridsAt(tb, first, last)
			if err != nil {
				return err
			}
			colStyle, err := t.tableStyle(tb.ID, 0, colID)
			if err != nil {
				return err
			}
			for i, id := range ids {
				if cells++; cells > MaxStyleCells {
					return fmt.Errorf("mise en forme limitée à %d cellules", MaxStyleCells)
				}
				if err := t.styleTable(tb.ID, id, colID, patch, over(lines.at(first+i, c), colStyle)); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// styleTableColumn met en forme toute une colonne de tableau (lignes
// ajoutées ensuite comprises) ; les styles de cellule pour les mêmes
// propriétés s'effacent.
func (t *tx) styleTableColumn(tb *Table, colID int64, c int, patch Style, lines sheetLines) error {
	if err := t.styleTable(tb.ID, 0, colID, patch, over(lines.cols[c], lines.rowsIn(tb.Row+1, tb.Bottom()))); err != nil {
		return err
	}
	return t.clearCellStyles(tb.ID, colID, patch)
}

// ApplyLineStyle met en forme des lignes (rows) ou des colonnes entières
// [i0, i1] de la feuille, comme Excel : la mise en forme vaut pour toutes
// leurs cellules, présentes et à venir, cellules de tableau et de tableau
// croisé comprises. Les propriétés posées l'emportent sur celles des
// cellules qu'elles traversent (elles y sont effacées ou recopiées). Une
// seule étape d'annulation.
func (m *Model) ApplyLineStyle(ctx context.Context, sheet int64, rows bool, i0, i1 int, patch Style) error {
	if i0 > i1 {
		i0, i1 = i1, i0
	}
	if i0 < 0 || i1-i0+1 > MaxStyleLines {
		return fmt.Errorf("mise en forme limitée à %d lignes ou colonnes", MaxStyleLines)
	}
	label := "Mise en forme des colonnes"
	if rows {
		label = "Mise en forme des lignes"
	}
	return m.Do(ctx, label, func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		lines, err := t.sheetLines(sheet)
		if err != nil {
			return err
		}
		cells := 0
		count := func() error {
			if cells++; cells > MaxStyleCells {
				return fmt.Errorf("mise en forme limitée à %d cellules", MaxStyleCells)
			}
			return nil
		}
		if rows {
			cols := lines.anyCol()
			for r := i0; r <= i1; r++ {
				if err := t.styleLine(sheet, true, r, patch, cols); err != nil {
					return err
				}
				// Les cellules libres de la ligne reprennent la mise en forme
				// de la ligne.
				if err := t.stripFree(sheet, `row = ?`, r, patch); err != nil {
					return err
				}
			}
			// Les tableaux l'emportent sur la ligne de la feuille : leurs
			// cellules traversées reçoivent la mise en forme.
			for i := range tables {
				tb := &tables[i]
				if !tb.spansRows(i0, i1) {
					continue
				}
				for k, col := range tb.Cols {
					c := tb.Col + k
					if i0 <= tb.Row && tb.Row <= i1 {
						if err := count(); err != nil {
							return err
						}
						if err := t.styleTable(tb.ID, -1, col.ID, patch, lines.cols[c]); err != nil {
							return err
						}
					}
					first, last := max(i0, tb.Row+1), min(i1, tb.Bottom())
					if first > last {
						continue
					}
					ids, err := t.ridsAt(tb, first, last)
					if err != nil {
						return err
					}
					colStyle, err := t.tableStyle(tb.ID, 0, col.ID)
					if err != nil {
						return err
					}
					for _, id := range ids {
						if err := count(); err != nil {
							return err
						}
						if err := t.styleTable(tb.ID, id, col.ID, patch, over(lines.cols[c], colStyle)); err != nil {
							return err
						}
					}
				}
			}
			return nil
		}
		for c := i0; c <= i1; c++ {
			if err := t.styleLine(sheet, false, c, patch, nil); err != nil {
				return err
			}
			if err := t.stripFree(sheet, `col = ?`, c, patch); err != nil {
				return err
			}
			// Une ligne mise en forme l'emporte sur la colonne : à leur
			// croisement, la cellule reçoit la mise en forme de la colonne.
			for r, rs := range lines.rows {
				sub := Style{}
				for k, v := range patch {
					if _, ok := rs[k]; ok {
						sub[k] = v
					}
				}
				if len(sub) == 0 || inTable(tables, r, c) {
					continue
				}
				if err := count(); err != nil {
					return err
				}
				if err := t.styleFree(sheet, r, c, sub, rs); err != nil {
					return err
				}
			}
			for i := range tables {
				tb := &tables[i]
				if c < tb.Col || c > tb.Right() {
					continue
				}
				colID := tb.Cols[c-tb.Col].ID
				if err := t.styleTable(tb.ID, -1, colID, patch, lines.rows[tb.Row]); err != nil {
					return err
				}
				if err := t.styleTable(tb.ID, 0, colID, patch, lines.rowsIn(tb.Row+1, tb.Bottom())); err != nil {
					return err
				}
				if err := t.clearCellStyles(tb.ID, colID, patch); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func inTable(tables []Table, r, c int) bool {
	for _, tb := range tables {
		if r >= tb.Row && r <= tb.Bottom() && c >= tb.Col && c <= tb.Right() {
			return true
		}
	}
	return false
}

// setStyle remplace un style (ligne de la table de styles désignée par
// where/args) par fn(ancien), avec son annulation.
func (t *tx) setStyle(table, where string, cols string, args []any, fn func(string) (string, error)) error {
	var old string
	err := t.row(`SELECT style FROM `+table+` WHERE `+where, args...).Scan(&old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	neu, err := fn(old)
	if err != nil || neu == old {
		return err
	}
	marks := "?"
	for range args {
		marks += ", ?"
	}
	set := func(s string) []stmt {
		out := []stmt{st(`DELETE FROM `+table+` WHERE `+where, args...)}
		if s != "" {
			out = append(out, st(`INSERT INTO `+table+` (`+cols+`, style) VALUES (`+marks+`)`, append(append([]any{}, args...), s)...))
		}
		return out
	}
	return t.apply(set(neu), set(old))
}

func (t *tx) styleFree(sheet int64, row, col int, patch, inh Style) error {
	return t.setStyle(`_lattis_cell_styles`, `sheet = ? AND row = ? AND col = ?`, `sheet, row, col`, []any{sheet, row, col},
		func(old string) (string, error) { return merge(old, patch, inh) })
}

func (t *tx) styleTable(tbl, rid, col int64, patch, inh Style) error {
	return t.setStyle(`_lattis_styles`, `tbl = ? AND rid = ? AND col = ?`, `tbl, rid, col`, []any{tbl, rid, col},
		func(old string) (string, error) { return merge(old, patch, inh) })
}

func (t *tx) styleLine(sheet int64, row bool, i int, patch, inh Style) error {
	table, where, cols := `_lattis_col_styles`, `sheet = ? AND col = ?`, `sheet, col`
	if row {
		table, where, cols = `_lattis_row_styles`, `sheet = ? AND row = ?`, `sheet, row`
	}
	return t.setStyle(table, where, cols, []any{sheet, i}, func(old string) (string, error) { return merge(old, patch, inh) })
}

// stripFree retire les propriétés du correctif des cellules libres d'une
// ligne ou d'une colonne (cond : "row = ?" ou "col = ?").
func (t *tx) stripFree(sheet int64, cond string, i int, patch Style) error {
	rs, err := t.query(`SELECT row, col FROM _lattis_cell_styles WHERE sheet = ? AND `+cond, sheet, i)
	if err != nil {
		return err
	}
	var pos [][2]int
	for rs.Next() {
		var p [2]int
		if err := rs.Scan(&p[0], &p[1]); err != nil {
			rs.Close()
			return err
		}
		pos = append(pos, p)
	}
	rs.Close()
	for _, p := range pos {
		err := t.setStyle(`_lattis_cell_styles`, `sheet = ? AND row = ? AND col = ?`, `sheet, row, col`, []any{sheet, p[0], p[1]},
			func(old string) (string, error) { return strip(old, patch) })
		if err != nil {
			return err
		}
	}
	return nil
}

// clearCellStyles retire des styles de cellule d'une colonne les propriétés
// que l'on vient de poser sur toute la colonne.
func (t *tx) clearCellStyles(tbl, col int64, patch Style) error {
	rs, err := t.query(`SELECT rid FROM _lattis_styles WHERE tbl = ? AND col = ? AND rid > 0`, tbl, col)
	if err != nil {
		return err
	}
	var rids []int64
	for rs.Next() {
		var r int64
		if err := rs.Scan(&r); err != nil {
			rs.Close()
			return err
		}
		rids = append(rids, r)
	}
	rs.Close()
	for _, r := range rids {
		err := t.setStyle(`_lattis_styles`, `tbl = ? AND rid = ? AND col = ?`, `tbl, rid, col`, []any{tbl, r, col},
			func(old string) (string, error) { return strip(old, patch) })
		if err != nil {
			return err
		}
	}
	return nil
}

// LineStyle est la mise en forme d'une ligne ou d'une colonne entière.
type LineStyle struct {
	Index int    `json:"index"`
	Style string `json:"style"`
}

// lineStyles lit les styles de lignes et de colonnes d'une feuille.
func (t *tx) lineStyles(sheet int64) (rows, cols []LineStyle, err error) {
	rows, cols = []LineStyle{}, []LineStyle{}
	for _, q := range []struct {
		sql string
		to  *[]LineStyle
	}{
		{`SELECT row, style FROM _lattis_row_styles WHERE sheet = ? ORDER BY row`, &rows},
		{`SELECT col, style FROM _lattis_col_styles WHERE sheet = ? ORDER BY col`, &cols},
	} {
		rs, err := t.query(q.sql, sheet)
		if err != nil {
			return nil, nil, err
		}
		for rs.Next() {
			var l LineStyle
			if err := rs.Scan(&l.Index, &l.Style); err != nil {
				rs.Close()
				return nil, nil, err
			}
			*q.to = append(*q.to, l)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return nil, nil, err
		}
	}
	return rows, cols, nil
}

// TableCellStyles donne la mise en forme propre à des cellules d'un tableau
// (export) : Row est le rang de la ligne dans l'ordre d'affichage (0 :
// première ligne de données), Col le rang de la colonne.
func (m *Model) TableCellStyles(ctx context.Context, tbl int64) ([]CellStyle, error) {
	var out []CellStyle
	err := m.read(ctx, func(t *tx) error {
		tb, err := t.table(tbl)
		if err != nil {
			return err
		}
		idx := map[int64]int{}
		for i, c := range tb.Cols {
			idx[c.ID] = i
		}
		rs, err := t.query(`SELECT r.n, s.col, s.style FROM _lattis_styles s
			JOIN (SELECT id, row_number() OVER (ORDER BY _pos) - 1 AS n FROM `+tb.sql()+`) r ON r.id = s.rid
			WHERE s.tbl = ? AND s.rid > 0`, tbl)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var n int
			var col int64
			var style string
			if err := rs.Scan(&n, &col, &style); err != nil {
				return err
			}
			if j, ok := idx[col]; ok {
				out = append(out, CellStyle{Row: n, Col: j, Style: style})
			}
		}
		return rs.Err()
	})
	return out, err
}
