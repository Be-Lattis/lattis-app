package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrSheet signale une opération impossible sur un onglet.
var ErrSheet = errors.New("onglet")

// AddSheet ajoute un onglet à la fin et renvoie son identifiant.
func (m *Model) AddSheet(ctx context.Context) (int64, error) {
	var id int64
	err := m.Do(ctx, "Nouvelle feuille", func(t *tx) (err error) {
		id, err = t.addSheet()
		return err
	})
	return id, err
}

// InsertSheet ajoute un onglet juste avant l'onglet before (Insérer, dans le
// menu d'un onglet) et renvoie son identifiant.
func (m *Model) InsertSheet(ctx context.Context, before int64) (int64, error) {
	var id int64
	err := m.Do(ctx, "Insérer une feuille", func(t *tx) error {
		ids, err := t.sheetOrder()
		if err != nil {
			return err
		}
		at := slices.Index(ids, before)
		if at < 0 {
			return fmt.Errorf("%w : feuille introuvable", ErrSheet)
		}
		if id, err = t.addSheet(); err != nil {
			return err
		}
		return t.moveSheet(id, at)
	})
	return id, err
}

// MoveSheet place l'onglet sheet au rang to (0 : premier), les autres
// onglets gardant leur ordre.
func (m *Model) MoveSheet(ctx context.Context, sheet int64, to int) error {
	return m.Do(ctx, "Déplacer la feuille", func(t *tx) error { return t.moveSheet(sheet, to) })
}

func (t *tx) addSheet() (int64, error) {
	if err := t.guardWorkbook(); err != nil {
		return 0, err
	}
	var id int64
	var pos int
	if err := t.row(`SELECT coalesce(max(id), 0) + 1, coalesce(max(pos), -1) + 1 FROM _slitable_sheets`).Scan(&id, &pos); err != nil {
		return 0, err
	}
	name := ""
	for n := int(id); ; n++ {
		name = fmt.Sprintf("Feuille%d", n)
		var k int
		if err := t.row(`SELECT count(*) FROM _slitable_sheets WHERE name = ?`, name).Scan(&k); err != nil {
			return 0, err
		}
		if k == 0 {
			break
		}
	}
	return id, t.apply(
		[]stmt{st(`INSERT INTO _slitable_sheets (id, name, pos, gridlines) VALUES (?, ?, ?, 0)`, id, name, pos)},
		[]stmt{st(`DELETE FROM _slitable_sheets WHERE id = ?`, id)},
	)
}

// sheetOrder donne les onglets dans l'ordre affiché.
func (t *tx) sheetOrder() ([]int64, error) {
	rs, err := t.query(`SELECT id FROM _slitable_sheets ORDER BY pos, id`)
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

// moveSheet renumérote les rangs pour placer sheet au rang to ; l'annulation
// rend les rangs d'origine.
func (t *tx) moveSheet(sheet int64, to int) error {
	if err := t.guardWorkbook(); err != nil {
		return err
	}
	ids, err := t.sheetOrder()
	if err != nil {
		return err
	}
	from := slices.Index(ids, sheet)
	if from < 0 {
		return fmt.Errorf("%w : feuille introuvable", ErrSheet)
	}
	to = max(0, min(to, len(ids)-1))
	if from == to {
		return nil
	}
	old := map[int64]int{}
	rs, err := t.query(`SELECT id, pos FROM _slitable_sheets`)
	if err != nil {
		return err
	}
	for rs.Next() {
		var id int64
		var pos int
		if err := rs.Scan(&id, &pos); err != nil {
			rs.Close()
			return err
		}
		old[id] = pos
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	q := `UPDATE _slitable_sheets SET pos = ? WHERE id = ?`
	var do, undo []stmt
	ids = slices.Insert(slices.Delete(ids, from, from+1), to, sheet)
	for i, id := range ids {
		do = append(do, st(q, i, id))
		undo = append(undo, st(q, old[id], id))
	}
	return t.apply(do, undo)
}

// RenameSheet renomme un onglet.
func (m *Model) RenameSheet(ctx context.Context, sheet int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w : nom vide", ErrSheet)
	}
	return m.Do(ctx, "Renommer la feuille", func(t *tx) error {
		if err := t.guardWorkbook(); err != nil {
			return err
		}
		var old string
		if err := t.row(`SELECT name FROM _slitable_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
			return err
		}
		if old == name {
			return nil
		}
		var k int
		if err := t.row(`SELECT count(*) FROM _slitable_sheets WHERE name = ? AND id <> ?`, name, sheet).Scan(&k); err != nil {
			return err
		}
		if k > 0 {
			return fmt.Errorf("%w : « %s » existe déjà", ErrSheet, name)
		}
		q := `UPDATE _slitable_sheets SET name = ? WHERE id = ?`
		return t.apply([]stmt{st(q, name, sheet)}, []stmt{st(q, old, sheet)})
	})
}

// DeleteSheet supprime un onglet sans tableau (ses cellules libres partent
// avec lui ; l'annulation les rend). Le dernier onglet ne se supprime pas.
func (m *Model) DeleteSheet(ctx context.Context, sheet int64) error {
	return m.Do(ctx, "Supprimer la feuille", func(t *tx) error {
		if err := t.guardWorkbook(); err != nil {
			return err
		}
		var n, tables int
		var name, color string
		var pos, grid, heads, hidden, frows, fcols int
		if err := t.row(`SELECT count(*) FROM _slitable_sheets`).Scan(&n); err != nil {
			return err
		}
		if n <= 1 {
			return fmt.Errorf("%w : un classeur garde au moins une feuille", ErrSheet)
		}
		var others int
		if err := t.row(`SELECT count(*) FROM _slitable_sheets WHERE hidden = 0 AND id <> ?`, sheet).Scan(&others); err != nil {
			return err
		}
		if others == 0 {
			return fmt.Errorf("%w : un classeur garde au moins une feuille visible", ErrSheet)
		}
		if err := t.row(`SELECT count(*) FROM _slitable_tables WHERE sheet = ?`, sheet).Scan(&tables); err != nil {
			return err
		}
		if tables > 0 {
			return fmt.Errorf("%w : la feuille contient des tableaux, supprimez-les d'abord", ErrSheet)
		}
		if err := t.row(`SELECT name, pos, gridlines, headings, color, hidden, freeze_rows, freeze_cols FROM _slitable_sheets WHERE id = ?`, sheet).Scan(&name, &pos, &grid, &heads, &color, &hidden, &frows, &fcols); err != nil {
			return err
		}
		cells, err := t.cells(sheet)
		if err != nil {
			return err
		}
		for _, c := range cells {
			if err := t.setFreeCell(sheet, c.Row, c.Col, ""); err != nil {
				return err
			}
		}
		pivots, err := t.pivotRows(sheet)
		if err != nil {
			return err
		}
		charts, err := t.chartRows(sheet)
		if err != nil {
			return err
		}
		buttons, err := t.buttonRows(sheet)
		if err != nil {
			return err
		}
		pivots = append(append(pivots, charts...), buttons...)
		// Mise en forme, largeurs et hauteurs de la feuille.
		do := []stmt{st(`DELETE FROM _slitable_pivots WHERE sheet = ?`, sheet), st(`DELETE FROM _slitable_charts WHERE sheet = ?`, sheet), st(`DELETE FROM _slitable_buttons WHERE sheet = ?`, sheet)}
		// Protection de la feuille : supprimée avec elle, rendue par l'annulation.
		prot, err := t.snapshot(`_slitable_protection`, sheet)
		if err != nil {
			return err
		}
		do = append(do, st(`DELETE FROM _slitable_protection WHERE sheet = ?`, sheet))
		pivots = append(pivots, prot...)
		for _, tbl := range sheetPositioned {
			rows, err := t.snapshot(tbl, sheet)
			if err != nil {
				return err
			}
			do = append(do, st(`DELETE FROM `+tbl+` WHERE sheet = ?`, sheet))
			pivots = append(pivots, rows...)
		}
		return t.apply(
			append(do, st(`DELETE FROM _slitable_sheets WHERE id = ?`, sheet)),
			append([]stmt{st(`INSERT INTO _slitable_sheets (id, name, pos, gridlines, headings, color, hidden, freeze_rows, freeze_cols) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, sheet, name, pos, grid, heads, color, hidden, frows, fcols)}, pivots...),
		)
	})
}

// SetGridlines affiche ou masque le quadrillage de la feuille.
func (m *Model) SetGridlines(ctx context.Context, sheet int64, show bool) error {
	return m.Do(ctx, "Quadrillage", func(t *tx) error {
		var old bool
		if err := t.row(`SELECT gridlines <> 0 FROM _slitable_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
			return err
		}
		if old == show {
			return nil
		}
		q := `UPDATE _slitable_sheets SET gridlines = ? WHERE id = ?`
		return t.apply([]stmt{st(q, show, sheet)}, []stmt{st(q, old, sheet)})
	})
}

// SetHeadings affiche toujours (ou seulement pendant la saisie d'une
// formule) les lettres de colonnes et numéros de lignes de la feuille.
func (m *Model) SetHeadings(ctx context.Context, sheet int64, show bool) error {
	return m.Do(ctx, "En-têtes A1", func(t *tx) error {
		var old bool
		if err := t.row(`SELECT headings <> 0 FROM _slitable_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
			return err
		}
		if old == show {
			return nil
		}
		q := `UPDATE _slitable_sheets SET headings = ? WHERE id = ?`
		return t.apply([]stmt{st(q, show, sheet)}, []stmt{st(q, old, sheet)})
	})
}

// SetColumnWidth fixe la largeur (en pixels) d'une colonne de la feuille ;
// 0 revient à la largeur par défaut.
func (m *Model) SetColumnWidth(ctx context.Context, sheet int64, col, width int) error {
	return m.Do(ctx, "Largeur de colonne", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		var old int
		err := t.row(`SELECT width FROM _slitable_col_widths WHERE sheet = ? AND col = ?`, sheet, col).Scan(&old)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if old == width {
			return nil
		}
		set := func(w int) []stmt {
			s := []stmt{st(`DELETE FROM _slitable_col_widths WHERE sheet = ? AND col = ?`, sheet, col)}
			if w > 0 {
				s = append(s, st(`INSERT INTO _slitable_col_widths (sheet, col, width) VALUES (?, ?, ?)`, sheet, col, w))
			}
			return s
		}
		return t.apply(set(width), set(old))
	})
}

// SetRowHeight fixe la hauteur (en pixels) d'une ligne de la feuille ; 0
// revient à la hauteur par défaut.
func (m *Model) SetRowHeight(ctx context.Context, sheet int64, row, height int) error {
	return m.Do(ctx, "Hauteur de ligne", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		var old int
		err := t.row(`SELECT height FROM _slitable_row_heights WHERE sheet = ? AND row = ?`, sheet, row).Scan(&old)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if old == height {
			return nil
		}
		set := func(h int) []stmt {
			s := []stmt{st(`DELETE FROM _slitable_row_heights WHERE sheet = ? AND row = ?`, sheet, row)}
			if h > 0 {
				s = append(s, st(`INSERT INTO _slitable_row_heights (sheet, row, height) VALUES (?, ?, ?)`, sheet, row, h))
			}
			return s
		}
		return t.apply(set(height), set(old))
	})
}

// sheetPositioned sont les tables de métadonnées repérées par feuille et
// position (sans identifiant propre), supprimées avec la feuille.
var sheetPositioned = []string{`_slitable_cell_styles`, `_slitable_row_styles`, `_slitable_col_styles`, `_slitable_row_rules`, `_slitable_col_rules`, `_slitable_col_widths`, `_slitable_row_heights`}

// snapshot renvoie les insertions qui recréent les lignes d'une feuille dans
// une table de métadonnées (pour l'annulation).
func (t *tx) snapshot(table string, sheet int64) ([]stmt, error) {
	rs, err := t.query(`SELECT * FROM `+table+` WHERE sheet = ?`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	q := `INSERT INTO ` + table + ` (` + strings.Join(cols, ", ") + `) VALUES (?` + strings.Repeat(", ?", len(cols)-1) + `)`
	var out []stmt
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		out = append(out, st(q, vals...))
	}
	return out, rs.Err()
}
