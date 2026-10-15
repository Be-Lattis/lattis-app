package canvas

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lalamefine/slitable/internal/command"
	"github.com/lalamefine/slitable/internal/grid"
	"github.com/lalamefine/slitable/internal/order"
)

// --- Opérations élémentaires (chacune enregistre son inverse) ---------------

// setFreeCell pose ou efface (raw vide) une cellule libre.
func (t *tx) setFreeCell(sheet int64, row, col int, raw string) error {
	var old string
	err := t.row(`SELECT raw FROM _slitable_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&old)
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if !exists && raw == "" || exists && old == raw {
		return nil
	}
	del := st(`DELETE FROM _slitable_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col)
	put := func(v string) stmt {
		return st(`INSERT INTO _slitable_cells (sheet, row, col, raw) VALUES (?, ?, ?, ?)`, sheet, row, col, v)
	}
	do := []stmt{del}
	if raw != "" {
		do = append(do, put(raw))
	}
	undo := []stmt{del}
	if exists {
		undo = append(undo, put(old))
	}
	return t.apply(do, undo)
}

// setValue écrit une valeur typée dans une cellule de tableau ; invalid
// (non vide) enregistre une saisie hors type, la cellule valant alors NULL.
func (t *tx) setValue(tb Table, rid int64, c Column, v any, invalid string) error {
	var old any
	if err := t.row(`SELECT `+colSQL(c.ID)+` FROM `+tb.sql()+` WHERE id = ?`, rid).Scan(&old); err != nil {
		return err
	}
	var oldInvalid string
	err := t.row(`SELECT raw FROM _slitable_invalid WHERE tbl = ? AND rid = ? AND col = ?`, tb.ID, rid, c.ID).Scan(&oldInvalid)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if b, ok := old.([]byte); ok {
		old = string(b)
	}
	if old == v && oldInvalid == invalid {
		return nil
	}
	write := func(v any, inv string) []stmt {
		s := []stmt{
			st(`UPDATE `+tb.sql()+` SET `+colSQL(c.ID)+` = ? WHERE id = ?`, v, rid),
			st(`DELETE FROM _slitable_invalid WHERE tbl = ? AND rid = ? AND col = ?`, tb.ID, rid, c.ID),
		}
		if inv != "" {
			s = append(s, st(`INSERT INTO _slitable_invalid (tbl, rid, col, raw) VALUES (?, ?, ?, ?)`, tb.ID, rid, c.ID, inv))
		}
		return s
	}
	return t.apply(write(v, invalid), write(old, oldInvalid))
}

// insertRow ajoute une ligne vide de clé id à la position pos.
func (t *tx) insertRow(tb Table, id int64, pos string) error {
	return t.apply(
		[]stmt{st(`INSERT INTO `+tb.sql()+` (id, _pos) VALUES (?, ?)`, id, pos)},
		[]stmt{st(`DELETE FROM `+tb.sql()+` WHERE id = ?`, id)},
	)
}

// deleteRow supprime une ligne ; l'inverse la réinsère à l'identique (même
// clé, même position, mêmes valeurs et saisies invalides).
func (t *tx) deleteRow(tb Table, rid int64) error {
	names := []string{"id", "_pos"}
	for _, c := range tb.Cols {
		names = append(names, colSQL(c.ID))
	}
	vals := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := t.row(`SELECT `+strings.Join(names, ", ")+` FROM `+tb.sql()+` WHERE id = ?`, rid).Scan(ptrs...); err != nil {
		return err
	}
	undo := []stmt{st(`INSERT INTO `+tb.sql()+` (`+strings.Join(names, ", ")+`) VALUES (`+marks(len(names))+`)`, normalize(vals)...)}
	rs, err := t.query(`SELECT col, raw FROM _slitable_invalid WHERE tbl = ? AND rid = ?`, tb.ID, rid)
	if err != nil {
		return err
	}
	for rs.Next() {
		var col int64
		var raw string
		if err := rs.Scan(&col, &raw); err != nil {
			rs.Close()
			return err
		}
		undo = append(undo, st(`INSERT INTO _slitable_invalid (tbl, rid, col, raw) VALUES (?, ?, ?, ?)`, tb.ID, rid, col, raw))
	}
	rs.Close()
	return t.apply([]stmt{
		st(`DELETE FROM _slitable_invalid WHERE tbl = ? AND rid = ?`, tb.ID, rid),
		st(`DELETE FROM `+tb.sql()+` WHERE id = ?`, rid),
	}, undo)
}

// addColumn ajoute une colonne au rang pos.
func (t *tx) addColumn(tb Table, id int64, name string, typ grid.Type, pos int) error {
	return t.apply([]stmt{
		st(`ALTER TABLE ` + tb.sql() + ` ADD COLUMN ` + colSQL(id) + ` ` + sqlType(typ)),
		st(`UPDATE _slitable_columns SET pos = pos + 1 WHERE tbl = ? AND pos >= ?`, tb.ID, pos),
		st(`INSERT INTO _slitable_columns (tbl, id, name, type, pos) VALUES (?, ?, ?, ?, ?)`, tb.ID, id, name, string(typ), pos),
	}, []stmt{
		st(`DELETE FROM _slitable_invalid WHERE tbl = ? AND col = ?`, tb.ID, id),
		st(`DELETE FROM _slitable_columns WHERE tbl = ? AND id = ?`, tb.ID, id),
		st(`UPDATE _slitable_columns SET pos = pos - 1 WHERE tbl = ? AND pos > ?`, tb.ID, pos),
		st(`ALTER TABLE ` + tb.sql() + ` DROP COLUMN ` + colSQL(id)),
	})
}

// dropColumn supprime une colonne. Ses valeurs partent dans une table de
// corbeille, d'où l'annulation les remet : l'inverse reste en quelques
// instructions quelle que soit la taille du tableau.
func (t *tx) dropColumn(tb Table, c Column, pos int) error {
	trash := fmt.Sprintf("_slitable_trash_%d_%d_%d", tb.ID, c.ID, time.Now().UnixNano())
	col := colSQL(c.ID)
	// Supprimer la colonne clé retire d'abord la clé (SQLite refuse de
	// supprimer une colonne indexée) ; l'annulation la rétablit.
	var unkey, rekey []stmt
	if c.ID == tb.Key {
		unkey = []stmt{
			st(`DROP INDEX ` + tb.sql() + `_key`),
			st(`UPDATE _slitable_tables SET key_col = NULL WHERE id = ?`, tb.ID),
		}
		rekey = []stmt{
			st(`CREATE UNIQUE INDEX ` + tb.sql() + `_key ON ` + tb.sql() + `(` + keyExpr(c.ID, c.Type) + `)`),
			st(`UPDATE _slitable_tables SET key_col = ? WHERE id = ?`, c.ID, tb.ID),
		}
	}
	return t.ApplyStep(command.Step{Do: append(unkey, []stmt{
		st(`CREATE TABLE ` + trash + ` AS SELECT id, ` + col + ` AS v FROM ` + tb.sql()),
		st(`CREATE TABLE `+trash+`_inv AS SELECT rid, raw FROM _slitable_invalid WHERE tbl = ? AND col = ?`, tb.ID, c.ID),
		st(`DELETE FROM _slitable_invalid WHERE tbl = ? AND col = ?`, tb.ID, c.ID),
		st(`ALTER TABLE ` + tb.sql() + ` DROP COLUMN ` + col),
		st(`DELETE FROM _slitable_columns WHERE tbl = ? AND id = ?`, tb.ID, c.ID),
		st(`UPDATE _slitable_columns SET pos = pos - 1 WHERE tbl = ? AND pos > ?`, tb.ID, pos),
	}...), Undo: append([]stmt{
		st(`UPDATE _slitable_columns SET pos = pos + 1 WHERE tbl = ? AND pos >= ?`, tb.ID, pos),
		st(`INSERT INTO _slitable_columns (tbl, id, name, type, fixed, pos) VALUES (?, ?, ?, ?, ?, ?)`, tb.ID, c.ID, c.Name, string(c.Type), c.Fixed, pos),
		st(`ALTER TABLE ` + tb.sql() + ` ADD COLUMN ` + col + ` ` + sqlType(c.Type)),
		st(`UPDATE ` + tb.sql() + ` SET ` + col + ` = (SELECT v FROM ` + trash + ` WHERE ` + trash + `.id = ` + tb.sql() + `.id)`),
		st(`INSERT INTO _slitable_invalid (tbl, rid, col, raw) SELECT ?, rid, ?, raw FROM `+trash+`_inv`, tb.ID, c.ID),
		st(`DROP TABLE ` + trash),
		st(`DROP TABLE ` + trash + `_inv`),
	}, rekey...), Purge: []stmt{
		// La commande sort de l'historique : la corbeille ne servira plus.
		st(`DROP TABLE IF EXISTS ` + trash),
		st(`DROP TABLE IF EXISTS ` + trash + `_inv`),
	}})
}

// rename change le nom affiché d'une colonne.
func (t *tx) rename(tb Table, c Column, name string) error {
	if name == c.Name {
		return nil
	}
	q := `UPDATE _slitable_columns SET name = ? WHERE tbl = ? AND id = ?`
	return t.apply([]stmt{st(q, name, tb.ID, c.ID)}, []stmt{st(q, c.Name, tb.ID, c.ID)})
}

// rebuildStmts reconstruit la table avec la colonne colID au type to (SQLite
// ne sait pas changer le type d'une colonne) : nouvelle table, copie avec
// conversion, suppression de l'ancienne, renommage, index. Les colonnes
// gardent leur ordre physique (cols), pour que l'annulation redonne
// exactement le même schéma.
func rebuildStmts(tb Table, physical []Column, colID int64, from, to grid.Type) []stmt {
	name, tmp := tb.sql(), tb.sql()+"_rebuild"
	defs := []string{"id INTEGER PRIMARY KEY", "_pos TEXT NOT NULL"}
	cols := []string{"id", "_pos"}
	sel := []string{"id", "_pos"}
	for _, c := range physical {
		typ, expr := c.Type, colSQL(c.ID)
		if c.ID == colID {
			typ, expr = to, convertExpr(colSQL(c.ID), from, to)
		}
		defs = append(defs, colSQL(c.ID)+" "+sqlType(typ))
		cols = append(cols, colSQL(c.ID))
		sel = append(sel, expr)
	}
	out := []stmt{
		st(`CREATE TABLE ` + tmp + ` (` + strings.Join(defs, ", ") + `) STRICT`),
		st(`INSERT INTO ` + tmp + ` (` + strings.Join(cols, ", ") + `) SELECT ` + strings.Join(sel, ", ") + ` FROM ` + name),
		st(`DROP TABLE ` + name),
		st(`ALTER TABLE ` + tmp + ` RENAME TO ` + name),
		st(`CREATE INDEX ` + name + `_pos ON ` + name + `(_pos)`),
	}
	// La clé du tableau (REFERENCE) garde son index UNIQUE.
	for _, c := range physical {
		if c.ID == tb.Key {
			typ := c.Type
			if c.ID == colID {
				typ = to
			}
			out = append(out, st(`CREATE UNIQUE INDEX `+name+`_key ON `+name+`(`+keyExpr(c.ID, typ)+`)`))
		}
	}
	return append(out, st(`UPDATE _slitable_columns SET type = ? WHERE tbl = ? AND id = ?`, string(to), tb.ID, colID))
}

// keyExpr est l'expression de l'index de la clé : sans tenir compte de la
// casse pour un texte, comme la recherche.
func keyExpr(id int64, t grid.Type) string {
	if t == grid.Text || t == grid.Date {
		return colSQL(id) + " COLLATE NOCASE"
	}
	return colSQL(id)
}

// retype change le type de stockage d'une colonne (reconstruction).
func (t *tx) retype(tb Table, i int, to grid.Type) (Table, error) {
	c := tb.Cols[i]
	physical, err := t.physical(tb)
	if err != nil {
		return tb, err
	}
	if err := t.apply(rebuildStmts(tb, physical, c.ID, c.Type, to), rebuildStmts(tb, physical, c.ID, to, c.Type)); err != nil {
		return tb, err
	}
	tb.Cols = append([]Column(nil), tb.Cols...)
	tb.Cols[i].Type = to
	return tb, nil
}

// createTable crée un tableau à partir de saisies : en-têtes et lignes de
// données, types inférés colonne par colonne.
func (t *tx) createTable(sheet int64, name string, row, col int, headers []string, data [][]string) (Table, error) {
	var id int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _slitable_tables`).Scan(&id); err != nil {
		return Table{}, err
	}
	tb := Table{ID: id, Sheet: sheet, Name: name, Row: row, Col: col, Rows: len(data)}
	defs := []string{"id INTEGER PRIMARY KEY", "_pos TEXT NOT NULL"}
	names := []string{"id", "_pos"}
	do := []stmt{st(`INSERT INTO _slitable_tables (id, sheet, name, row, col) VALUES (?, ?, ?, ?, ?)`, id, sheet, name, row, col)}
	for i, h := range headers {
		raws := make([]string, len(data))
		for r := range data {
			raws[r] = data[r][i]
		}
		c := Column{ID: int64(i + 1), Name: h, Type: InferAll(raws)}
		if strings.TrimSpace(c.Name) == "" {
			c.Name = fmt.Sprintf("Colonne %d", i+1)
		}
		tb.Cols = append(tb.Cols, c)
		defs = append(defs, colSQL(c.ID)+" "+sqlType(c.Type))
		names = append(names, colSQL(c.ID))
		do = append(do, st(`INSERT INTO _slitable_columns (tbl, id, name, type, pos) VALUES (?, ?, ?, ?, ?)`, id, c.ID, c.Name, string(c.Type), i))
	}
	do = append([]stmt{
		st(`CREATE TABLE ` + tb.sql() + ` (` + strings.Join(defs, ", ") + `) STRICT`),
		st(`CREATE INDEX ` + tb.sql() + `_pos ON ` + tb.sql() + `(_pos)`),
	}, do...)
	keys := order.Spread(len(data))
	for r, raws := range data {
		args := []any{int64(r + 1), keys[r]}
		for i, raw := range raws {
			v, err := grid.Parse(tb.Cols[i].Type, raw)
			if err != nil {
				return Table{}, err
			}
			args = append(args, v)
		}
		do = append(do, st(`INSERT INTO `+tb.sql()+` (`+strings.Join(names, ", ")+`) VALUES (`+marks(len(names))+`)`, args...))
	}
	undo := []stmt{
		st(`DELETE FROM _slitable_invalid WHERE tbl = ?`, id),
		st(`DELETE FROM _slitable_columns WHERE tbl = ?`, id),
		st(`DELETE FROM _slitable_tables WHERE id = ?`, id),
		st(`DROP TABLE ` + tb.sql()),
	}
	return tb, t.apply(do, undo)
}

func marks(n int) string { return strings.TrimSuffix(strings.Repeat("?, ", n), ", ") }

func normalize(vals []any) []any {
	for i, v := range vals {
		if b, ok := v.([]byte); ok {
			vals[i] = string(b)
		}
	}
	return vals
}

// physical donne les colonnes dans l'ordre du schéma SQL (l'ordre d'ajout),
// qui diffère de l'ordre d'affichage après une insertion de colonne.
func (t *tx) physical(tb Table) ([]Column, error) {
	byName := map[string]Column{}
	for _, c := range tb.Cols {
		byName[colSQL(c.ID)] = c
	}
	rs, err := t.query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, tb.sql())
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []Column
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			return nil, err
		}
		if c, ok := byName[n]; ok {
			out = append(out, c)
		}
	}
	return out, rs.Err()
}
