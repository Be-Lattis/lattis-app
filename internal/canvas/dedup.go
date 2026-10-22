package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/work"
)

// RemoveDuplicates supprime les lignes dont les colonnes cols (toutes si
// vide) répètent celles d'une ligne précédente, comme « Supprimer les
// doublons » d'Excel : la première occurrence reste. Renvoie le nombre de
// lignes supprimées ; une seule étape d'annulation, quelle que soit la taille
// (les lignes partent dans une corbeille que l'annulation remet).
func (m *Model) RemoveDuplicates(ctx context.Context, table string, cols []int) (int, error) {
	n := 0
	err := m.Do(ctx, "Supprimer les doublons", func(t *tx) error {
		tb, err := t.dataTable(table)
		if err != nil {
			return err
		}
		work.Op(ctx, "dedup", tb.Name)
		if tb.Filtered() {
			return tb.errFiltered(false)
		}
		dups, err := dupQuery(tb, cols)
		if err != nil || dups == "" {
			return err
		}
		// Les doublons ne sont cherchés qu'une fois (un parcours trié du
		// tableau) : ils vont à la corbeille, puis on les compte ; aucun :
		// la transaction est abandonnée, sans étape d'annulation.
		trash := fmt.Sprintf("_lattis_trash_dup_%d_%d", tb.ID, uniq())
		err = t.ApplyStep(command.Step{
			Do: []stmt{
				st(`CREATE TABLE ` + trash + ` AS SELECT * FROM ` + tb.sql() + ` WHERE id IN (` + dups + `)`),
				st(`CREATE INDEX ` + trash + `_id ON ` + trash + `(id)`),
				st(`CREATE TABLE `+trash+`_inv AS SELECT * FROM _lattis_invalid WHERE tbl = ? AND rid IN (SELECT id FROM `+trash+`)`, tb.ID),
				st(`CREATE TABLE `+trash+`_sty AS SELECT * FROM _lattis_styles WHERE tbl = ? AND rid IN (SELECT id FROM `+trash+`)`, tb.ID),
				st(`DELETE FROM _lattis_invalid WHERE tbl = ? AND rid IN (SELECT id FROM `+trash+`)`, tb.ID),
				st(`DELETE FROM _lattis_styles WHERE tbl = ? AND rid IN (SELECT id FROM `+trash+`)`, tb.ID),
				st(`DELETE FROM ` + tb.sql() + ` WHERE id IN (SELECT id FROM ` + trash + `)`),
			},
			Undo: []stmt{
				st(`INSERT INTO ` + tb.sql() + ` SELECT * FROM ` + trash),
				st(`INSERT INTO _lattis_invalid SELECT * FROM ` + trash + `_inv`),
				st(`INSERT INTO _lattis_styles SELECT * FROM ` + trash + `_sty`),
				st(`DROP TABLE ` + trash),
				st(`DROP TABLE ` + trash + `_inv`),
				st(`DROP TABLE ` + trash + `_sty`),
			},
			Purge: []stmt{
				st(`DROP TABLE IF EXISTS ` + trash),
				st(`DROP TABLE IF EXISTS ` + trash + `_inv`),
				st(`DROP TABLE IF EXISTS ` + trash + `_sty`),
			},
		})
		if err != nil {
			return err
		}
		if err := t.row(`SELECT count(*) FROM ` + trash).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return errNoDuplicates
		}
		return nil
	})
	if errors.Is(err, errNoDuplicates) {
		return 0, nil
	}
	return n, err
}

// errNoDuplicates abandonne une suppression des doublons qui n'en trouve pas.
var errNoDuplicates = msg.New("dedup.none", nil)

// DuplicatesPreview compte les lignes que RemoveDuplicates retirerait.
func (m *Model) DuplicatesPreview(ctx context.Context, table string, cols []int) (int, error) {
	n := 0
	err := m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		dups, err := dupQuery(tb, cols)
		if err != nil || dups == "" {
			return err
		}
		return t.row(`SELECT count(*) FROM (` + dups + `)`).Scan(&n)
	})
	return n, err
}

// dupQuery donne la requête des lignes en double selon les colonnes cols
// (toutes si vide), la première occurrence exceptée ; "" sans colonne.
func dupQuery(tb Table, cols []int) (string, error) {
	if len(cols) == 0 {
		for i := range tb.Cols {
			cols = append(cols, i)
		}
	}
	var key []string
	for _, i := range cols {
		if i < 0 || i >= len(tb.Cols) {
			return "", msg.New("table.columnIndexOutOfRange", msg.Args{"n": i})
		}
		c := tb.Cols[i]
		if c.Formula != "" {
			continue
		}
		// Texte sans tenir compte de la casse (Excel) ; les autres types par
		// leur valeur.
		if c.Type == grid.Text {
			key = append(key, "lower("+colSQL(c.ID)+")")
		} else {
			key = append(key, colSQL(c.ID))
		}
	}
	if len(key) == 0 {
		return "", nil
	}
	return `SELECT id FROM (SELECT id, row_number() OVER (PARTITION BY ` + strings.Join(key, ", ") + ` ORDER BY _pos) AS rn FROM ` +
		tb.sql() + `) WHERE rn > 1`, nil
}
