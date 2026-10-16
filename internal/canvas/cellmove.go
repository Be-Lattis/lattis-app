package canvas

import (
	"context"
	"fmt"
)

// MoveCells déplace les cellules libres du rectangle [r0, r1] × [c0, c1]
// de (dr, dc), comme en glissant la bordure de la sélection dans Excel :
// valeurs, mise en forme et noms suivent ; ce qui occupait la destination
// est remplacé ; les adresses Cell_A1 qui visaient les cellules déplacées
// suivent (celles qui visaient une cellule remplacée deviennent #REF!).
func (m *Model) MoveCells(ctx context.Context, sheet int64, r0, c0, r1, c1, dr, dc int) error {
	if dr == 0 && dc == 0 {
		return nil
	}
	if r0 > r1 || c0 > c1 || r0+dr < 0 || c0+dc < 0 {
		return fmt.Errorf("%w : déplacement hors de la feuille", ErrTable)
	}
	return m.Do(ctx, "Déplacer les cellules", func(t *tx) error {
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		tr0, tr1, tc0, tc1 := r0+dr, r1+dr, c0+dc, c1+dc
		for _, tb := range tables {
			if tb.Row <= r1 && tb.Bottom() >= r0 && tb.Col <= c1 && tb.Right() >= c0 {
				return fmt.Errorf("%w : la sélection contient une partie du tableau « %s » ; sélectionnez le tableau entier pour le déplacer", ErrTable, tb.Name)
			}
			if touches(tr0, tr1, tc0, tc1, tb) {
				return fmt.Errorf("%w : la destination touche le tableau « %s »", ErrTable, tb.Name)
			}
		}
		inSource := `row BETWEEN ? AND ? AND col BETWEEN ? AND ?`
		for _, tbl := range []string{"_slitable_cells", "_slitable_cell_styles", "_slitable_names"} {
			// La destination est remplacée (hors cellules déplacées elles-mêmes).
			if err := t.deleteWhere(tbl, `sheet = ? AND `+inSource+` AND NOT (`+inSource+`)`,
				sheet, tr0, tr1, tc0, tc1, r0, r1, c0, c1); err != nil {
				return err
			}
			// Deux temps : pas de collision de clé quand source et destination se recouvrent.
			away := `UPDATE ` + tbl + ` SET row = -(row + ?) - 1, col = -(col + ?) - 1 WHERE sheet = ? AND ` + inSource
			back := `UPDATE ` + tbl + ` SET row = -row - 1, col = -col - 1 WHERE sheet = ? AND row < 0`
			if err := t.apply(
				[]stmt{st(away, dr, dc, sheet, r0, r1, c0, c1), st(back, sheet)},
				[]stmt{st(away, -dr, -dc, sheet, tr0, tr1, tc0, tc1), st(back, sheet)},
			); err != nil {
				return err
			}
		}
		return t.rewriteCellRefs(sheet, func(r, c int) (int, int, bool) {
			switch {
			case r >= r0 && r <= r1 && c >= c0 && c <= c1:
				return r + dr, c + dc, true
			case r >= tr0 && r <= tr1 && c >= tc0 && c <= tc1:
				return 0, 0, false
			}
			return r, c, true
		})
	})
}
