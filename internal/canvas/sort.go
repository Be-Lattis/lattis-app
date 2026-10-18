package canvas

import (
	"context"
	"fmt"
	"time"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/grid"
)

// SortTable trie les lignes d'un tableau selon la colonne i, comme le tri
// d'Excel : l'ordre des lignes change pour de bon (les clés d'ordre sont
// réécrites), les cellules vides et les saisies en erreur vont à la fin, et
// les égalités gardent l'ordre précédent. Une colonne calculée ne se trie
// pas (pour l'instant) : son calcul dépend de l'ordre.
func (m *Model) SortTable(ctx context.Context, table string, i int, desc bool) error {
	return m.Do(ctx, "Trier", func(t *tx) error {
		// Tri local permis sur un tableau publié en lecture seule (classeur
		// figé, §5.4) ; ailleurs, le journal logique décide.
		tb, err := t.viewTable(table)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(tb.Cols) {
			return fmt.Errorf("colonne %d hors du tableau", i)
		}
		c := tb.Cols[i]
		if c.Formula != "" {
			return fmt.Errorf("« %s » est une colonne calculée : triez sur une colonne saisie", c.Name)
		}
		col := colSQL(c.ID)
		if c.Type == grid.Text {
			col += " COLLATE NOCASE"
		}
		dir := "ASC"
		if desc {
			dir = "DESC"
		}
		stamp := time.Now().UnixNano()
		trash := fmt.Sprintf("_lattis_trash_sort_%d_%d", tb.ID, stamp)
		rank := fmt.Sprintf("_lattis_rank_%d_%d", tb.ID, stamp)
		// Nouvelles clés : le rang écrit sur 12 chiffres, suivi d'une lettre
		// (une clé ne finit jamais par 0) ; l'ordre lexicographique est celui
		// du rang, et l'on peut toujours insérer entre deux clés.
		return t.ApplyStep(command.Step{
			Do: []stmt{
				st(`CREATE TABLE ` + trash + ` (id INTEGER PRIMARY KEY, pos TEXT NOT NULL)`),
				st(`INSERT INTO ` + trash + ` SELECT id, _pos FROM ` + tb.sql()),
				st(`CREATE TABLE ` + rank + ` (id INTEGER PRIMARY KEY, k TEXT NOT NULL)`),
				st(`INSERT INTO ` + rank + ` SELECT id, printf('%012dV', row_number() OVER (ORDER BY ` +
					colSQL(c.ID) + ` IS NULL, ` + col + ` ` + dir + `, _pos)) FROM ` + tb.sql()),
				// Index de _pos retiré pendant la réécriture : le reconstruire d'un
				// coup est bien plus rapide que de le tenir à jour ligne à ligne.
				st(`DROP INDEX ` + tb.sql() + `_pos`),
				st(`UPDATE ` + tb.sql() + ` SET _pos = r.k FROM ` + rank + ` AS r WHERE r.id = ` + tb.sql() + `.id`),
				st(`CREATE INDEX ` + tb.sql() + `_pos ON ` + tb.sql() + `(_pos)`),
				st(`DROP TABLE ` + rank),
			},
			Undo: []stmt{
				st(`DROP INDEX ` + tb.sql() + `_pos`),
				st(`UPDATE ` + tb.sql() + ` SET _pos = o.pos FROM ` + trash + ` AS o WHERE o.id = ` + tb.sql() + `.id`),
				st(`CREATE INDEX ` + tb.sql() + `_pos ON ` + tb.sql() + `(_pos)`),
				st(`DROP TABLE ` + trash),
			},
			Purge: []stmt{st(`DROP TABLE IF EXISTS ` + trash)},
		})
	})
}
