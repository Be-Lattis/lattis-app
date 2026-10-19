package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/command"
)

// ErrTable signale une opération impossible sur un tableau entier
// (renommer, déplacer, fusionner, scinder).
var ErrTable = errors.New("tableau")

// a1Name : une adresse de cellule de la grille (colonnes A à ZZZZ, lignes 1
// à 999 999 999), ou une adresse L1C1.
var a1Name = regexp.MustCompile(`^(?i:[a-z]{1,4}[1-9][0-9]{0,8}|r[1-9][0-9]*c[1-9][0-9]*)$`)

// A1Like dit si un nom se lit comme une adresse de cellule (F10, AB12,
// R1C1) : refusé pour un tableau, comme dans Excel, car la zone de nom y
// irait et les formules ne sauraient pas lequel des deux on veut dire.
func A1Like(name string) bool { return a1Name.MatchString(name) }

// RenameTable renomme un tableau. Comme dans Excel, le nom est un
// identifiant (lettres, chiffres, _), pour s'écrire tel quel dans une
// formule, et pas une adresse de cellule.
func (m *Model) RenameTable(ctx context.Context, old, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || identifier(name) != name {
		return fmt.Errorf("%w : « %s » n'est pas un nom de tableau (lettres, chiffres et _, sans espace)", ErrTable, name)
	}
	if A1Like(name) {
		return fmt.Errorf("%w : « %s » est une adresse de cellule, ambiguë comme nom de tableau (essayez « %s_ »)", ErrTable, name, name)
	}
	return m.Do(ctx, "Renommer le tableau", func(t *tx) error {
		tb, err := t.editTable(old)
		if err != nil {
			return err
		}
		if tb.Name == name {
			return nil
		}
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE name = ? COLLATE NOCASE AND id <> ?`, name, tb.ID).Scan(&k); err != nil {
			return err
		}
		if k > 0 {
			return fmt.Errorf("%w : « %s » existe déjà", ErrTable, name)
		}
		return t.renameTable(tb, name)
	})
}

// renameTable renomme un tableau ; ce qui le cite suit (formules, TCD,
// graphiques, requêtes).
func (t *tx) renameTable(tb Table, name string) error {
	if err := t.rewriteRefs(tableChange(tb.Name, name)); err != nil {
		return err
	}
	q := `UPDATE _lattis_tables SET name = ? WHERE id = ?`
	return t.apply([]stmt{st(q, name, tb.ID)}, []stmt{st(q, tb.Name, tb.ID)})
}

// DeleteTables supprime des tableaux entiers (une seule étape d'annulation).
func (m *Model) DeleteTables(ctx context.Context, names []string) error {
	label := "Supprimer le tableau"
	if len(names) > 1 {
		label = "Supprimer les tableaux"
	}
	return m.Do(ctx, label, func(t *tx) error {
		for _, n := range names {
			tb, err := t.editTable(n)
			if err != nil {
				return err
			}
			if err := t.deleteTable(tb); err != nil {
				return err
			}
			// Suppression voulue : un tableau partagé est aussi supprimé du
			// serveur (un tableau qui disparaît autrement, actualisé,
			// déplacé, fusionné, est refusé : oplog.Change).
			t.Record(command.Op{Kind: command.OpMeta, Table: tb.ID, Meta: &command.MetaChange{Object: "table", Action: "drop"}})
		}
		return nil
	})
}

// deleteTable retire un tableau : ses lignes, saisies invalides et styles
// partent dans des tables de corbeille que l'annulation remet en place.
func (t *tx) deleteTable(tb Table) error {
	trash := fmt.Sprintf("_lattis_trash_t%d_%d", tb.ID, time.Now().UnixNano())
	var key, filter sql.NullString
	var noHint, twoLevel int
	if err := t.row(`SELECT key_col, filter, no_key_hint, two_level FROM _lattis_tables WHERE id = ?`, tb.ID).Scan(&key, &filter, &noHint, &twoLevel); err != nil {
		return err
	}
	undo := []stmt{
		st(`INSERT INTO _lattis_tables (id, sheet, name, row, col, key_col, filter, no_key_hint, two_level) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			tb.ID, tb.Sheet, tb.Name, tb.Row, tb.Col, nullable(key), nullable(filter), noHint, twoLevel),
		st(`INSERT INTO _lattis_columns SELECT * FROM ` + trash + `_col`),
		st(`INSERT INTO _lattis_invalid SELECT * FROM ` + trash + `_inv`),
		st(`INSERT INTO _lattis_styles SELECT * FROM ` + trash + `_sty`),
		st(`ALTER TABLE ` + trash + ` RENAME TO ` + tb.sql()),
		st(`CREATE INDEX ` + tb.sql() + `_pos ON ` + tb.sql() + `(_pos)`),
	}
	do := []stmt{
		st(`CREATE TABLE `+trash+`_col AS SELECT * FROM _lattis_columns WHERE tbl = ?`, tb.ID),
		st(`CREATE TABLE `+trash+`_inv AS SELECT * FROM _lattis_invalid WHERE tbl = ?`, tb.ID),
		st(`CREATE TABLE `+trash+`_sty AS SELECT * FROM _lattis_styles WHERE tbl = ?`, tb.ID),
		st(`DELETE FROM _lattis_columns WHERE tbl = ?`, tb.ID),
		st(`DELETE FROM _lattis_invalid WHERE tbl = ?`, tb.ID),
		st(`DELETE FROM _lattis_styles WHERE tbl = ?`, tb.ID),
		st(`DELETE FROM _lattis_tables WHERE id = ?`, tb.ID),
		// Les index portent le nom de la table : ils partent, pour qu'un
		// nouveau tableau puisse reprendre l'identifiant.
		st(`DROP INDEX ` + tb.sql() + `_pos`),
	}
	if tb.Key != 0 {
		do = append(do, st(`DROP INDEX `+tb.sql()+`_key`))
		for _, c := range tb.Cols {
			if c.ID == tb.Key {
				undo = append(undo, st(`CREATE UNIQUE INDEX `+tb.sql()+`_key ON `+tb.sql()+`(`+keyExpr(c.ID, c.Type)+`)`))
			}
		}
	}
	do = append(do, st(`ALTER TABLE `+tb.sql()+` RENAME TO `+trash))
	undo = append(undo, st(`DROP TABLE `+trash+`_col`), st(`DROP TABLE `+trash+`_inv`), st(`DROP TABLE `+trash+`_sty`))
	return t.ApplyStep(command.Step{Do: do, Undo: undo, Purge: []stmt{
		st(`DROP TABLE IF EXISTS ` + trash),
		st(`DROP TABLE IF EXISTS ` + trash + `_col`),
		st(`DROP TABLE IF EXISTS ` + trash + `_inv`),
		st(`DROP TABLE IF EXISTS ` + trash + `_sty`),
	}})
}

func nullable(s sql.NullString) any {
	if s.Valid {
		return s.String
	}
	return nil
}

// touches indique si deux rectangles (avec la ligne et la colonne vides qui
// séparent chaque tableau du reste) se touchent.
func touches(r0, r1, c0, c1 int, b Table) bool {
	return r0 <= b.Bottom()+1 && b.Top() <= r1+1 && c0 <= b.Right()+1 && b.Col <= c1+1
}

// place fusionne un tableau posé en (row, col) avec un tableau voisin dont
// l'en-tête est sur la même ligne (côte à côte) ; les autres tableaux qu'il
// touche s'écartent vers la droite (settle). Il ne recouvre jamais de
// cellules remplies. Renvoie le tableau obtenu.
func (t *tx) place(tb Table) (Table, error) {
	tables, err := t.tables(tb.Sheet)
	if err != nil {
		return tb, err
	}
	var left, right *Table
	for i := range tables {
		o := tables[i]
		if o.ID == tb.ID || !touches(tb.Top(), tb.Bottom(), tb.Col, tb.Right(), o) {
			continue
		}
		// Côte à côte, en-têtes sur la même ligne : fusion (voir aussi
		// joinAdjacent, qui écarte les autres cas).
		overlap := o.Row == tb.Row && o.TwoLevel == tb.TwoLevel && !o.Filtered() && !tb.Filtered()
		switch {
		case overlap && o.Right()+1 == tb.Col && left == nil:
			left = &tables[i]
		case overlap && tb.Right()+1 == o.Col && right == nil:
			right = &tables[i]
		}
	}
	cells, err := t.cells(tb.Sheet)
	if err != nil {
		return tb, err
	}
	for _, c := range cells {
		if c.Row >= tb.Top() && c.Row <= tb.Bottom()+1 && c.Col >= tb.Col && c.Col <= tb.Right()+1 {
			return tb, fmt.Errorf("%w : la place recouvre des cellules remplies", ErrTable)
		}
	}
	if left != nil {
		if err := t.joinTables(*left, tb); err != nil {
			return tb, err
		}
		if tb, err = t.table(left.ID); err != nil {
			return tb, err
		}
	}
	if right != nil {
		if r, err := t.table(right.ID); err == nil {
			right = &r
		}
		if err := t.joinTables(tb, *right); err != nil {
			return tb, err
		}
		if tb, err = t.table(tb.ID); err != nil {
			return tb, err
		}
	}
	if err := t.settle(tb.Sheet, tb.ID); err != nil {
		return tb, err
	}
	return t.table(tb.ID)
}

// MoveTable déplace un tableau, l'en-tête en (row, col). Posé côte à côte
// d'un autre tableau, en-têtes sur la même ligne, il fusionne avec lui.
func (m *Model) MoveTable(ctx context.Context, name string, row, col int) error {
	if row < 0 || col < 0 {
		return fmt.Errorf("%w : hors de la feuille", ErrTable)
	}
	return m.Do(ctx, "Déplacer le tableau", func(t *tx) error {
		tb, err := t.editTable(name)
		if err != nil {
			return err
		}
		if tb.TwoLevel && row < 1 {
			// La bande des groupes reste sur la feuille.
			row = 1
		}
		if tb.Row == row && tb.Col == col {
			return nil
		}
		if err := t.moveTable(tb, row-tb.Row, col-tb.Col); err != nil {
			return err
		}
		tb.Row, tb.Col = row, col
		_, err = t.place(tb)
		return err
	})
}

// mergeRight fusionne b à droite de a, en-têtes sur la même ligne : les
// colonnes de b s'ajoutent à a, chaque ligne de b dans la ligne de a qui est
// sur la même ligne de la feuille ; le plus court des deux est complété de
// lignes vides. b disparaît : ce qui le citait (formules, colonnes calculées,
// règles, TCD, graphiques, requêtes) cite a, sous les nouveaux noms de ses
// colonnes (« Prix 2 » quand a a déjà un « Prix »). Sa clé devient celle de
// a si a n'en a pas.
func (t *tx) mergeRight(a, b Table) (Table, error) {
	if a.Filtered() || b.Filtered() {
		return a, fmt.Errorf("%w : effacez le filtre avant de fusionner « %s » et « %s »", ErrTable, a.Name, b.Name)
	}
	if a.Row != b.Row {
		return a, fmt.Errorf("%w : les en-têtes de « %s » et « %s » ne sont pas sur la même ligne", ErrTable, a.Name, b.Name)
	}
	if a.TwoLevel != b.TwoLevel {
		return a, fmt.Errorf("%w : « %s » et « %s » n'ont pas tous deux leurs en-têtes sur deux lignes", ErrTable, a.Name, b.Name)
	}
	// Noms des colonnes de b dans a.
	names := map[string]bool{}
	for _, c := range a.Cols {
		names[strings.ToLower(c.Name)] = true
	}
	rc := tableChange(b.Name, a.Name)
	rc.kept = map[string][]string{}
	newNames := make([]string, len(b.Cols))
	for k, c := range b.Cols {
		name := c.Name
		for n := 2; names[strings.ToLower(name)]; n++ {
			name = fmt.Sprintf("%s %d", c.Name, n)
		}
		names[strings.ToLower(name)] = true
		newNames[k] = name
		rc.column(b.Name, c.Name, name)
		rc.kept[strings.ToLower(b.Name)] = append(rc.kept[strings.ToLower(b.Name)], name)
	}
	// Formules d'abord (avec les noms d'avant) : les colonnes calculées de b
	// sont réécrites sur place, puis recopiées.
	if err := t.rewriteRefs(rc); err != nil {
		return a, err
	}
	b, err := t.table(b.ID)
	if err != nil {
		return a, err
	}
	if b.Rows > a.Rows {
		if err := t.appendRows(a, b.Rows-a.Rows); err != nil {
			return a, err
		}
	}
	// Mêmes lignes d'en-tête : la n-ième ligne de données de chacun est sur
	// la même ligne de la feuille.
	m := fmt.Sprintf("temp._slt_merge_%d", time.Now().UnixNano())
	if err := t.apply([]stmt{
		st(`CREATE TABLE ` + m + ` AS SELECT a.id AS aid, b.id AS bid FROM
			(SELECT id, row_number() OVER (ORDER BY _pos) AS rn FROM ` + a.sql() + `) a JOIN
			(SELECT id, row_number() OVER (ORDER BY _pos) AS rn FROM ` + b.sql() + `) b USING (rn)`),
		st(`CREATE INDEX ` + m + `_aid ON ` + strings.TrimPrefix(m, "temp.") + `(aid)`),
	}, nil); err != nil {
		return a, err
	}
	var next int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_columns WHERE tbl = ?`, a.ID).Scan(&next); err != nil {
		return a, err
	}
	var key *Column
	for k, c := range b.Cols {
		id := next + int64(k)
		if err := t.addColumn(a, id, newNames[k], c.Type, len(a.Cols)+k); err != nil {
			return a, err
		}
		if c.ID == b.Key && a.Key == 0 {
			key = &Column{ID: id, Type: c.Type, Name: newNames[k]}
		}
		rules, err := t.columnRules(b.ID, c.ID)
		if err != nil {
			return a, err
		}
		if err := t.apply([]stmt{
			st(`UPDATE _lattis_columns SET fixed = ?, formula = ?, rules = ? WHERE tbl = ? AND id = ?`,
				c.Fixed, nullIfEmpty(c.Formula), nullIfEmpty(rules), a.ID, id),
			st(`UPDATE ` + a.sql() + ` SET ` + colSQL(id) + ` = (SELECT ` + colSQL(c.ID) + ` FROM ` + b.sql() +
				` WHERE ` + b.sql() + `.id = (SELECT bid FROM ` + m + ` WHERE aid = ` + a.sql() + `.id))`),
			st(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) SELECT ?, m.aid, ?, i.raw
				FROM _lattis_invalid i JOIN `+m+` m ON m.bid = i.rid WHERE i.tbl = ? AND i.col = ?`, a.ID, id, b.ID, c.ID),
			st(`INSERT INTO _lattis_styles (tbl, rid, col, style) SELECT ?, m.aid, ?, s.style
				FROM _lattis_styles s JOIN `+m+` m ON m.bid = s.rid WHERE s.tbl = ? AND s.col = ?`, a.ID, id, b.ID, c.ID),
			st(`INSERT INTO _lattis_styles (tbl, rid, col, style) SELECT ?, rid, ?, style
				FROM _lattis_styles WHERE tbl = ? AND col = ? AND rid <= 0`, a.ID, id, b.ID, c.ID),
		}, []stmt{
			st(`DELETE FROM _lattis_styles WHERE tbl = ? AND col = ?`, a.ID, id),
		}); err != nil {
			return a, err
		}
	}
	if err := t.apply([]stmt{st(`DROP TABLE ` + m)}, nil); err != nil {
		return a, err
	}
	if err := t.deleteTable(b); err != nil {
		return a, err
	}
	// La clé de b (REFERENCE) devient celle de a, qui n'en avait pas.
	if key != nil {
		index := a.sql() + "_key"
		if err := t.apply([]stmt{
			st(`CREATE UNIQUE INDEX ` + index + ` ON ` + a.sql() + `(` + keyExpr(key.ID, key.Type) + `)`),
			st(`UPDATE _lattis_tables SET key_col = ? WHERE id = ?`, key.ID, a.ID),
		}, []stmt{
			st(`UPDATE _lattis_tables SET key_col = NULL WHERE id = ?`, a.ID),
			st(`DROP INDEX ` + index),
		}); err != nil {
			return a, err
		}
	} else if b.Key != 0 {
		t.notes = append(t.notes, fmt.Sprintf("La clé de « %s » n'en est plus une : « %s » garde la sienne", b.Name, a.Name))
	}
	merged, err := t.table(a.ID)
	if err != nil {
		return a, err
	}
	// Le tableau fusionné occupe la place des deux : ce qui touche sa
	// nouvelle étendue s'écarte.
	if err := t.ensureGapRight(a.Sheet, a.ID, a.Right(), merged.Right(), merged.Row, merged.Bottom()); err != nil {
		return merged, err
	}
	return t.table(a.ID)
}

// ExtractColumns sort les colonnes i0..i1 d'un tableau pour en faire un
// nouveau tableau, l'en-tête en (row, col) ; toutes les colonnes : le
// tableau se déplace. Posé côte à côte d'un autre tableau, il fusionne.
func (m *Model) ExtractColumns(ctx context.Context, name string, i0, i1, row, col int) error {
	if row < 0 || col < 0 {
		return fmt.Errorf("%w : hors de la feuille", ErrTable)
	}
	var whole bool
	err := m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(name)
		if err != nil {
			return err
		}
		if i0 < 0 || i1 >= len(tb.Cols) || i0 > i1 {
			return fmt.Errorf("%w : colonnes hors du tableau", ErrTable)
		}
		whole = i0 == 0 && i1 == len(tb.Cols)-1
		return nil
	})
	if err != nil {
		return err
	}
	if whole {
		return m.MoveTable(ctx, name, row, col)
	}
	return m.Do(ctx, "Déplacer des colonnes", func(t *tx) error {
		tb, err := t.editTable(name)
		if err != nil {
			return err
		}
		return t.extractColumns(tb, i0, i1, tb.Sheet, row, col)
	})
}

// extractColumns sort les colonnes i0..i1 (pas toutes) de tb vers un nouveau
// tableau de la feuille sheet, l'en-tête en (row, col), puis le pose (fusion
// côte à côte, écartement des voisins).
func (t *tx) extractColumns(tb Table, i0, i1 int, sheet int64, row, col int) error {
	if tb.Filtered() {
		return fmt.Errorf("%w : effacez le filtre avant de déplacer des colonnes", ErrTable)
	}
	cols := tb.Cols[i0 : i1+1]
	for _, c := range cols {
		if c.Formula != "" {
			return fmt.Errorf("%w : la colonne calculée « %s » ne se déplace pas", ErrTable, c.Name)
		}
	}
	var id int64
	if err := t.row(nextTableID).Scan(&id); err != nil {
		return err
	}
	newName, err := t.tableName(sheet, row, col)
	if err != nil {
		return err
	}
	nt := Table{ID: id, Sheet: sheet, Name: newName, Row: row, Col: col}
	defs := []string{"id INTEGER PRIMARY KEY", "_pos TEXT NOT NULL"}
	src := []string{"id", "_pos"}
	dst := []string{"id", "_pos"}
	do := []stmt{st(`INSERT INTO _lattis_tables (id, sheet, name, row, col) VALUES (?, ?, ?, ?, ?)`, id, sheet, newName, row, col)}
	for k, c := range cols {
		nid := int64(k + 1)
		defs = append(defs, colSQL(nid)+" "+sqlType(c.Type))
		src = append(src, colSQL(c.ID))
		dst = append(dst, colSQL(nid))
		rules, err := t.columnRules(tb.ID, c.ID)
		if err != nil {
			return err
		}
		do = append(do,
			st(`INSERT INTO _lattis_columns (tbl, id, name, type, fixed, pos, rules) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				id, nid, c.Name, string(c.Type), c.Fixed, k, nullIfEmpty(rules)),
			st(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) SELECT ?, rid, ?, raw FROM _lattis_invalid WHERE tbl = ? AND col = ?`, id, nid, tb.ID, c.ID),
			st(`INSERT INTO _lattis_styles (tbl, rid, col, style) SELECT ?, rid, ?, style FROM _lattis_styles WHERE tbl = ? AND col = ?`, id, nid, tb.ID, c.ID))
	}
	// Mêmes clés et mêmes positions : les styles et saisies invalides
	// suivent sans renumérotation.
	do = append([]stmt{
		st(`CREATE TABLE ` + nt.sql() + ` (` + strings.Join(defs, ", ") + `) STRICT`),
		st(`CREATE INDEX ` + nt.sql() + `_pos ON ` + nt.sql() + `(_pos)`),
		st(`INSERT INTO ` + nt.sql() + ` (` + strings.Join(dst, ", ") + `) SELECT ` + strings.Join(src, ", ") + ` FROM ` + tb.sql()),
	}, do...)
	undo := []stmt{
		st(`DELETE FROM _lattis_styles WHERE tbl = ?`, id),
		st(`DELETE FROM _lattis_invalid WHERE tbl = ?`, id),
		st(`DELETE FROM _lattis_columns WHERE tbl = ?`, id),
		st(`DELETE FROM _lattis_tables WHERE id = ?`, id),
		st(`DROP TABLE ` + nt.sql()),
	}
	if err := t.apply(do, undo); err != nil {
		return err
	}
	for i := i1; i >= i0; i-- {
		cur, err := t.table(tb.ID)
		if err != nil {
			return err
		}
		if err := t.dropColumn(cur, cur.Cols[i], i); err != nil {
			return err
		}
	}
	nt, err = t.table(id)
	if err != nil {
		return err
	}
	_, err = t.place(nt)
	return err
}
