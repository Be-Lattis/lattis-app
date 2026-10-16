package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrSheet signale une opération impossible sur un onglet.
var ErrSheet = errors.New("onglet")

// AddSheet ajoute un onglet à la fin et renvoie son identifiant.
func (m *Model) AddSheet(ctx context.Context) (int64, error) {
	var id int64
	err := m.Do(ctx, "Nouvelle feuille", func(t *tx) error {
		var pos int
		if err := t.row(`SELECT coalesce(max(id), 0) + 1, coalesce(max(pos), -1) + 1 FROM _slitable_sheets`).Scan(&id, &pos); err != nil {
			return err
		}
		name := ""
		for n := int(id); ; n++ {
			name = fmt.Sprintf("Feuille %d", n)
			var k int
			if err := t.row(`SELECT count(*) FROM _slitable_sheets WHERE name = ?`, name).Scan(&k); err != nil {
				return err
			}
			if k == 0 {
				break
			}
		}
		return t.apply(
			[]stmt{st(`INSERT INTO _slitable_sheets (id, name, pos) VALUES (?, ?, ?)`, id, name, pos)},
			[]stmt{st(`DELETE FROM _slitable_sheets WHERE id = ?`, id)},
		)
	})
	return id, err
}

// RenameSheet renomme un onglet.
func (m *Model) RenameSheet(ctx context.Context, sheet int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w : nom vide", ErrSheet)
	}
	return m.Do(ctx, "Renommer la feuille", func(t *tx) error {
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
		var n, tables int
		var name string
		var pos int
		if err := t.row(`SELECT count(*) FROM _slitable_sheets`).Scan(&n); err != nil {
			return err
		}
		if n <= 1 {
			return fmt.Errorf("%w : un classeur garde au moins une feuille", ErrSheet)
		}
		if err := t.row(`SELECT count(*) FROM _slitable_tables WHERE sheet = ?`, sheet).Scan(&tables); err != nil {
			return err
		}
		if tables > 0 {
			return fmt.Errorf("%w : la feuille contient des tableaux, supprimez-les d'abord", ErrSheet)
		}
		if err := t.row(`SELECT name, pos FROM _slitable_sheets WHERE id = ?`, sheet).Scan(&name, &pos); err != nil {
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
		pivots = append(pivots, charts...)
		return t.apply(
			[]stmt{st(`DELETE FROM _slitable_pivots WHERE sheet = ?`, sheet), st(`DELETE FROM _slitable_charts WHERE sheet = ?`, sheet), st(`DELETE FROM _slitable_sheets WHERE id = ?`, sheet)},
			append([]stmt{st(`INSERT INTO _slitable_sheets (id, name, pos) VALUES (?, ?, ?)`, sheet, name, pos)}, pivots...),
		)
	})
}

// SetColumnWidth fixe la largeur (en pixels) d'une colonne de la feuille ;
// 0 revient à la largeur par défaut.
func (m *Model) SetColumnWidth(ctx context.Context, sheet int64, col, width int) error {
	return m.Do(ctx, "Largeur de colonne", func(t *tx) error {
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
