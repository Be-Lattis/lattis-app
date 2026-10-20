package canvas

import (
	"context"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Tableau tapé sans en-têtes (demande de Louis, décision 0005) : quand la
// 1re ligne d'un îlot tapé ressemble aux suivantes (que des textes partout,
// par exemple), toutes les lignes tapées sont des données et l'en-tête se
// place juste au-dessus, avec les noms « Colonne 1 », « Colonne 2 »… (gris
// italique dans la grille ; les heuristiques qui les comptent comme donnés
// par Lattis, autoColRe, valent pour eux). Taper dans l'en-tête renomme la
// colonne ; clic droit › « Utiliser la première ligne comme en-têtes »
// (PromoteHeader) fait de la 1re ligne l'en-tête, à sa place. Sans place
// libre au-dessus, la 1re ligne reste l'en-tête, comme avant.

// autoHeaders : les noms « Colonne 1 » … « Colonne n » d'un en-tête sans nom.
func autoHeaders(n int) []string {
	return uniqueColumnNames(make([]string, n))
}

// allAuto : toutes les colonnes de tb ont un nom donné par Lattis
// (« Colonne n »).
func allAuto(tb Table) bool {
	for _, c := range tb.Cols {
		if !autoColRe.MatchString(c.Name) {
			return false
		}
	}
	return len(tb.Cols) > 0
}

// firstRowDiffers : la 1re ligne d'un îlot tapé se distingue des suivantes,
// c'est un en-tête : dans une colonne, un texte au-dessus de valeurs d'un
// autre type (typedOver), ou un en-tête que le collage reconnaît (hasHeader :
// années ou dates au-dessus de nombres, textes de longueurs différentes de
// valeurs de même longueur…).
func firstRowDiffers(g [][]string) bool {
	w := 0
	for _, r := range g {
		w = max(w, len(r))
	}
	return typedOver(g) || hasHeader(g, w)
}

// typedOver : dans au moins une colonne, la 1re ligne a un texte et les
// lignes suivantes, au moins une valeur, que des valeurs d'un autre type
// (nombres, dates, booléens, heures, pourcentages). Une colonne vide sous
// son premier texte ne dit rien.
func typedOver(g [][]string) bool {
	if len(g) < 2 {
		return false
	}
	for c, h := range g[0] {
		if h = strings.TrimSpace(h); h == "" || !allText([]string{h}) {
			continue
		}
		n, typed := 0, true
		for _, r := range g[1:] {
			if c >= len(r) {
				continue
			}
			v := strings.TrimSpace(r[c])
			if v == "" {
				continue
			}
			n++
			typed = typed && (Infer(v) != grid.Text || isTimeOrPercent(v))
		}
		if n > 0 && typed {
			return true
		}
	}
	return false
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

// reheadYoung : un tableau tout juste né sans en-têtes (que des noms
// « Colonne n », au plus quelques lignes) dont la 1re ligne se distingue
// maintenant des suivantes, même règle qu'à la naissance (un texte
// au-dessus de nombres tapés ensuite : « Nom | Prénom | Âge » puis
// « Dupont | Jean | 42 ») : elle devient l'en-tête, à sa place, et la barre
// d'état le dit.
func (t *tx) reheadYoung(sheet int64, row, col int) error {
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	for _, tb := range tables {
		if row <= tb.Row || row > tb.Bottom() || col < tb.Col || col > tb.Right() {
			continue
		}
		if tb.Rows < 2 || tb.Rows > 6 || tb.Filtered() || tb.TwoLevel || !allAuto(tb) {
			return nil
		}
		for _, c := range tb.Cols {
			if c.Formula != "" {
				return nil
			}
		}
		if t.lockTable(tb.ID) != nil {
			return nil
		}
		if locked, err := t.structureLocked(sheet); err != nil || locked {
			return err
		}
		if q, err := t.queryLoaded(tb.ID); err != nil || q {
			return err
		}
		rows := make([][]string, tb.Rows)
		for n := range rows {
			rows[n] = make([]string, len(tb.Cols))
			for i := range tb.Cols {
				if rows[n][i], _, err = t.rawAt(sheet, tb.Row+1+n, tb.Col+i); err != nil {
					return err
				}
			}
		}
		if !firstRowDiffers(rows) {
			return nil
		}
		if err := t.promoteInPlace(tb); err != nil {
			return err
		}
		t.notes = append(t.notes, fmt.Sprintf("1re ligne de « %s » prise comme en-têtes : elle se distingue maintenant des suivantes (Ctrl+Z pour annuler)", tb.Name))
		return nil
	}
	return nil
}
