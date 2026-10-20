package canvas

import (
	"context"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Tableau tapé sans en-têtes (décision 0005, règle de Louis du
// 20/10/2026) : la 1re ligne tapée d'un îlot est l'en-tête, sauf si elle a
// une valeur qui n'est pas un texte (nombre, date, heure, booléen,
// pourcentage, montant : textRow, actions.go). Alors toutes les lignes
// tapées sont des données et l'en-tête se place juste au-dessus, avec les
// noms « Colonne 1 », « Colonne 2 »… (gris italique dans la grille ; les
// heuristiques qui les comptent comme donnés par Lattis, autoColRe, valent
// pour eux). Taper dans l'en-tête renomme la colonne ; clic droit ›
// « Utiliser la première ligne comme en-têtes » (PromoteHeader) fait de la
// 1re ligne l'en-tête, à sa place. Sans place libre au-dessus, les cellules
// restent libres.

// autoHeaders : les noms « Colonne 1 » … « Colonne n » d'un en-tête sans nom.
func autoHeaders(n int) []string {
	return uniqueColumnNames(make([]string, n))
}

// headRoom : la ligne au-dessus de row, sur les colonnes c0..c1 (et une de
// part et d'autre), est libre pour un en-tête : ni cellule, ni tableau (zone
// grisée comprise), ni tableau croisé, ni fusion, ni plage. Faux en ligne 1.
func (t *tx) headRoom(sheet int64, row, c0, c1 int) (bool, error) {
	r := row - 1
	if r < 0 {
		return false, nil
	}
	if free, err := t.areaFree(sheet, r, c0-1, r, c1+1); err != nil || !free {
		return false, err
	}
	pivots, err := t.pivots(sheet)
	if err != nil {
		return false, err
	}
	for _, p := range pivots {
		if z := pivotRect(p); r >= z.R0 && r <= z.R1 && z.C0 <= c1+1 && z.C1 >= c0-1 {
			return false, nil
		}
	}
	merges, err := t.merges(sheet)
	if err != nil {
		return false, err
	}
	for _, g := range merges {
		if g.overlaps(r, c0-1, r, c1+1) {
			return false, nil
		}
	}
	marks, err := t.rangeMarks(sheet)
	if err != nil {
		return false, err
	}
	for c := c0 - 1; c <= c1+1; c++ {
		if marks[[2]int{r, c}] {
			return false, nil
		}
	}
	return true, nil
}

// PromoteHeader : « Utiliser la première ligne comme en-têtes ». La 1re
// ligne de données donne les noms des colonnes (une cellule vide garde le
// nom ; doublon : « Nom 2 ») et devient l'en-tête à sa place : l'ancien
// en-tête (« Colonne n » ou autres noms) disparaît, le tableau descend
// d'une ligne, son bas ne bouge pas ; la grille reste telle qu'elle a été
// tapée. Les formules suivent les renommages, les colonnes en texte à cause
// de cette ligne reprennent leur type (« Nom | 3 | 4 » : entier). Une
// étape.
func (m *Model) PromoteHeader(ctx context.Context, table string) error {
	return m.Do(ctx, "Utiliser la première ligne comme en-têtes", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if tb.Rows < 1 {
			return msg.Wrap(ErrTable, "table.noDataRow", msg.Args{"table": tb.Name})
		}
		if tb.Filtered() {
			return tb.errFiltered(false)
		}
		if tb.TwoLevel {
			return msg.Wrap(ErrTable, "table.twoLevelHeaders", msg.Args{"table": tb.Name})
		}
		if q, err := t.queryLoaded(tb.ID); err != nil || q {
			if err == nil {
				err = msg.Wrap(ErrTable, "table.queryLoadedRename", msg.Args{"table": tb.Name})
			}
			return err
		}
		return t.promoteInPlace(tb)
	})
}

// promoteInPlace : la 1re ligne de données devient l'en-tête, à sa place
// (voir PromoteHeader).
func (t *tx) promoteInPlace(tb Table) error {
	if err := t.promote(tb); err != nil {
		return err
	}
	tb, err := t.table(tb.ID)
	if err != nil {
		return err
	}
	if err := t.moveTable(tb, 1, 0); err != nil {
		return err
	}
	if tb, err = t.table(tb.ID); err != nil {
		return err
	}
	return t.narrowTypes(tb)
}

// narrowTypes : une colonne au type inféré passée en texte dont les valeurs
// sont maintenant toutes d'un autre type (son texte était l'en-tête) le
// reprend, comme à la saisie.
func (t *tx) narrowTypes(tb Table) error {
	for i, c := range tb.Cols {
		if c.Type != grid.Text || c.Fixed || c.Formula != "" {
			continue
		}
		rs, err := t.query(`SELECT ` + colSQL(c.ID) + ` FROM ` + tb.sql() + ` WHERE ` + colSQL(c.ID) + ` IS NOT NULL AND ` + colSQL(c.ID) + ` <> ''`)
		if err != nil {
			return err
		}
		var vals []string
		for rs.Next() {
			var v string
			if err := rs.Scan(&v); err != nil {
				rs.Close()
				return err
			}
			vals = append(vals, v)
		}
		rs.Close()
		if len(vals) == 0 {
			continue
		}
		if to := InferAll(vals); to != grid.Text {
			if tb, err = t.retype(tb, i, to); err != nil {
				return err
			}
		}
	}
	return nil
}
