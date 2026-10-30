package canvas

import (
	"context"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/work"
)

// SortTable trie les lignes d'un tableau selon la colonne i, comme le tri
// d'Excel : l'ordre des lignes change pour de bon (les clés d'ordre sont
// réécrites), les cellules vides et les saisies en erreur vont à la fin, et
// les égalités gardent l'ordre précédent. Une colonne calculée se trie sur
// ses valeurs du moment (comme Excel : une formule qui dépend de l'ordre
// des lignes est recalculée dans le nouvel ordre).
func (m *Model) SortTable(ctx context.Context, table string, i int, desc bool) error {
	return m.Do(ctx, "Trier", func(t *tx) error {
		// Tri local permis sur un tableau publié en lecture seule (classeur
		// figé, §5.4) ; ailleurs, le journal logique décide.
		tb, err := t.sortTable(table)
		if err != nil {
			return err
		}
		if i >= 0 && i < len(tb.Cols) {
			work.Op(ctx, "sort", tb.Cols[i].Name)
		}
		if err := t.sortRows(tb, i, desc, nil); err != nil {
			return err
		}
		// Un tri ordinaire remplace le tri personnalisé ré-appliqué à
		// l'actualisation.
		return t.setAppliedOrder(tb.Name, "")
	})
}

// sortRows réécrit l'ordre des lignes du tableau selon la colonne i ; order
// (non nil) est un ordre personnalisé : ses valeurs d'abord, dans l'ordre
// de la liste (inverse avec desc), puis les autres dans l'ordre naturel
// croissant, les vides à la fin.
func (t *tx) sortRows(tb Table, i int, desc bool, order []string) error {
	if i < 0 || i >= len(tb.Cols) {
		return msg.New("table.columnIndexOutOfRange", msg.Args{"n": i})
	}
	c := tb.Cols[i]
	src := tb.sql()
	var err error
	if c.Formula != "" {
		if src, c, err = t.computedView(tb, c); err != nil {
			return err
		}
	}
	col := colSQL(c.ID)
	if c.Type == grid.Text {
		col += " COLLATE NOCASE"
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	stamp := uniq()
	trash := fmt.Sprintf("_lattis_trash_sort_%d_%d", tb.ID, stamp)
	rank := fmt.Sprintf("_lattis_rank_%d_%d", tb.ID, stamp)
	seq := rank + "_n"
	var do, after []stmt
	by := colSQL(c.ID) + ` IS NULL, ` + col + ` ` + dir + `, _pos`
	from := src
	if order != nil {
		// Table d'ordre (valeur repliée → rang) jointe au tableau : un
		// seul passage, rapide sur un grand tableau, quelle que soit la
		// longueur de la liste.
		ord := fmt.Sprintf("_lattis_order_%d_%d", tb.ID, stamp)
		do = append(do, st(`CREATE TABLE `+ord+` (v TEXT PRIMARY KEY, r INTEGER NOT NULL) WITHOUT ROWID`))
		do = append(do, orderInserts(ord, order)...)
		after = append(after, st(`DROP TABLE `+ord))
		from = src + ` LEFT JOIN ` + ord + ` AS o ON o.v = ` + FoldFunc + `(` + textExpr(c) + `)`
		by = colSQL(c.ID) + ` IS NULL, o.r IS NULL, o.r ` + dir + `, ` + col + ` ASC, _pos`
	}
	do = append(do,
		st(`CREATE TABLE `+trash+` (id INTEGER PRIMARY KEY, pos TEXT NOT NULL)`),
		st(`INSERT INTO `+trash+` SELECT id, _pos FROM `+tb.sql()),
		// Rang de chaque ligne : les identifiants rangés dans l'ordre du tri
		// reçoivent les numéros 1, 2, 3… (rowid d'une table vide, dans
		// l'ordre de l'INSERT ; by finit par _pos : aucune égalité), puis
		// sont relus dans l'ordre des identifiants. Tables temporaires
		// (hors du fichier et du journal WAL). Deux tris simples sont deux
		// fois plus rapides qu'un row_number() OVER (…) écrit dans une table
		// rangée par identifiant (insertions dans le désordre) : 1 M de
		// lignes, 10 s → 3 s, et la mise à jour qui suit lit une table
		// compacte (8 s → 3,5 s).
		st(`CREATE TEMP TABLE `+seq+` (n INTEGER PRIMARY KEY, id INTEGER NOT NULL)`),
		st(`INSERT INTO `+seq+` (id) SELECT id FROM `+from+` ORDER BY `+by),
		st(`CREATE TEMP TABLE `+rank+` (id INTEGER PRIMARY KEY, k TEXT NOT NULL)`),
		// Nouvelles clés : le rang écrit sur 12 chiffres, suivi d'une
		// lettre (une clé ne finit jamais par 0) ; l'ordre lexicographique
		// est celui du rang, et l'on peut toujours insérer entre deux clés.
		st(`INSERT INTO `+rank+` SELECT id, printf('%012dV', n) FROM `+seq+` ORDER BY id`),
		st(`DROP TABLE `+seq),
	)
	do = append(do, after...)
	do = append(do,
		// Index de _pos retiré pendant la réécriture : le reconstruire d'un
		// coup est bien plus rapide que de le tenir à jour ligne à ligne.
		st(`DROP INDEX `+tb.sql()+`_pos`),
		st(`UPDATE `+tb.sql()+` SET _pos = r.k FROM `+rank+` AS r WHERE r.id = `+tb.sql()+`.id`),
		st(`CREATE INDEX `+tb.sql()+`_pos ON `+tb.sql()+`(_pos)`),
		st(`DROP TABLE `+rank),
	)
	return t.ApplyStep(command.Step{
		Do: do,
		Undo: []stmt{
			st(`DROP INDEX ` + tb.sql() + `_pos`),
			st(`UPDATE ` + tb.sql() + ` SET _pos = o.pos FROM ` + trash + ` AS o WHERE o.id = ` + tb.sql() + `.id`),
			st(`CREATE INDEX ` + tb.sql() + `_pos ON ` + tb.sql() + `(_pos)`),
			st(`DROP TABLE ` + trash),
		},
		Purge: []stmt{st(`DROP TABLE IF EXISTS ` + trash)},
	})
}

// orderInserts remplit la table d'ordre : chaque valeur repliée (FoldFunc)
// et son rang ; un doublon après repli garde son premier rang.
func orderInserts(table string, values []string) []stmt {
	const chunk = 400
	var out []stmt
	for lo := 0; lo < len(values); lo += chunk {
		hi := min(lo+chunk, len(values))
		marks := make([]string, 0, hi-lo)
		args := make([]any, 0, 2*(hi-lo))
		for k := lo; k < hi; k++ {
			marks = append(marks, "(?, ?)")
			args = append(args, Fold(values[k]), k)
		}
		out = append(out, st(`INSERT OR IGNORE INTO `+table+` (v, r) VALUES `+strings.Join(marks, ", "), args...))
	}
	return out
}
