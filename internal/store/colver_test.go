package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
)

func TestSetColumns(t *testing.T) {
	for _, c := range []struct {
		q     string
		table string
		cols  []string
	}{
		{`UPDATE t_3 SET c_5 = ? WHERE id = ?`, "t_3", []string{"c_5"}},
		{`update main.t_3 set "c_1" = 'a, b', c_2 = coalesce(?, c_2) where id = ?`, "t_3", []string{"c_1", "c_2"}},
		{`UPDATE t_3 SET c_2 = s.v FROM _lattis_trash_1 AS s WHERE s.id = t_3.id`, "t_3", []string{"c_2"}},
		{`UPDATE t_3 SET c_2 = NULL`, "t_3", []string{"c_2"}},
		{`UPDATE t_3 SET c_2 = 'WHERE ; x'`, "t_3", []string{"c_2"}},
		{`UPDATE t_3 SET _pos = r.k FROM _lattis_rank_3_1 AS r WHERE r.id = t_3.id`, "t_3", []string{"_pos"}},
	} {
		table, cols, ok := setColumns(c.q)
		if !ok || table != c.table || !slices.Equal(cols, c.cols) {
			t.Errorf("%s : %s %v %v", c.q, table, cols, ok)
		}
	}
	for _, q := range []string{
		`UPDATE t_3 SET id = 4 WHERE id = ?`,
		`UPDATE t_3 SET (c_1, c_2) = (1, 2)`,
		`UPDATE t_3 SET c_1 = 1; UPDATE t_3 SET c_2 = 2`,
		`UPDATE Commandes SET c_1 = 1`,
		`INSERT INTO t_3 (c_1) VALUES (1)`,
		`DELETE FROM t_3`,
	} {
		if table, cols, ok := setColumns(q); ok {
			t.Errorf("%s : rangée par colonne (%s %v)", q, table, cols)
		}
	}
}

// Une saisie dans une colonne ne change pas la version des autres ; une
// écriture qui n'en nomme pas les colonnes (ajout, vue, déclencheur) les
// change toutes.
func TestColumnVersion(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	exec := func(q string, args ...any) {
		t.Helper()
		if err := s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec(q, args...); return err }); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE t_1 (id INTEGER PRIMARY KEY, c_1, c_2)`)
	exec(`INSERT INTO t_1 VALUES (1, 'a', 1), (2, 'b', 2)`)
	c1 := func() string { return s.Version(ColumnOf("t_1", "c_1")) }
	v := c1()
	exec(`UPDATE t_1 SET c_2 = ? WHERE id = ?`, 5, 1)
	exec(`UPDATE t_1 SET c_2 = c_2 + 1`)
	if c1() != v {
		t.Fatal("une mise à jour de c_2 change la version de c_1")
	}
	exec(`UPDATE t_1 SET c_1 = ? WHERE id = ?`, "z", 1)
	if c1() == v {
		t.Fatal("une mise à jour de c_1 garde sa version")
	}
	v = c1()
	exec(`INSERT INTO t_1 VALUES (3, 'c', 3)`)
	if c1() == v {
		t.Fatal("un ajout de ligne garde la version de c_1")
	}
	v = c1()
	exec(`CREATE TEMP VIEW vue AS SELECT id, c_1 AS a FROM t_1`)
	exec(`CREATE TEMP TRIGGER vue_u INSTEAD OF UPDATE ON vue BEGIN UPDATE t_1 SET c_1 = new.a WHERE id = old.id; END`)
	exec(`UPDATE vue SET a = 'y' WHERE id = 2`)
	if c1() == v {
		t.Fatal("une écriture par une vue garde la version de c_1")
	}
	v = c1()
	// Dans une même tâche, une mise à jour de c_2 puis une écriture par la
	// vue : la seconde compte encore.
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE t_1 SET c_2 = ? WHERE id = ?`, 9, 2); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE vue SET a = 'x' WHERE id = 2`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if c1() == v {
		t.Fatal("une écriture par la vue après une mise à jour de c_2 garde la version de c_1")
	}
}

// Changements de structure (demande de Louis du 22/10) : la version de
// c_1, sans les métadonnées (NoMeta), ne change que si les lignes changent
// en nombre ou si c_1 elle-même change ; celle du nombre de lignes
// (CountOf) seulement si des lignes sont ajoutées ou supprimées ; celle de
// l'ordre (_pos) à un tri.
func TestColumnVersionStructure(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// task : une tâche de l'écrivain (une action, ou son annulation).
	task := func(qs ...string) {
		t.Helper()
		if err := s.Write(ctx, func(tx *sql.Tx) error {
			for _, q := range qs {
				if _, err := tx.Exec(q); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	task(`CREATE TABLE _lattis_columns (tbl, id, name, type, pos)`,
		`CREATE TABLE _lattis_tables (id, key_col, no_key_hint)`,
		`CREATE TABLE t_1 (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 INTEGER, c_2 TEXT, c_3 REAL) STRICT`,
		`CREATE INDEX t_1_pos ON t_1(_pos)`,
		`INSERT INTO t_1 VALUES (1, 'a', 1, 'x', 1.5), (2, 'b', 2, 'y', 2.5), (3, 'c', 3, NULL, NULL)`,
		`CREATE TABLE t_2 (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1)`)
	c1 := func() string { return s.Version(NoMeta, ColumnOf("t_1", "c_1")) }
	count := func() string { return s.Version(NoMeta, CountOf("t_1")) }
	pos := func() string { return s.Version(NoMeta, ColumnOf("t_1", "_pos")) }
	// rebuild : reconstruction de t_1 (canvas/ops.go), cols et sel dans
	// l'ordre du schéma.
	rebuild := func(defs, cols, sel, from string) []string {
		return []string{
			`CREATE TABLE t_1_rebuild (` + defs + `) STRICT`,
			`INSERT INTO t_1_rebuild (` + cols + `) SELECT ` + sel + ` FROM ` + from,
			`DROP TABLE t_1`,
			`ALTER TABLE t_1_rebuild RENAME TO t_1`,
			`CREATE INDEX t_1_pos ON t_1(_pos)`,
		}
	}
	keep := func(what string, qs ...string) {
		t.Helper()
		v, n, p := c1(), count(), pos()
		task(qs...)
		if c1() != v {
			t.Errorf("%s : la version de c_1 change", what)
		}
		if count() != n {
			t.Errorf("%s : la version du nombre de lignes change", what)
		}
		if pos() != p {
			t.Errorf("%s : la version de l'ordre change", what)
		}
	}
	change := func(what string, qs ...string) {
		t.Helper()
		v := c1()
		task(qs...)
		if c1() == v {
			t.Errorf("%s : la version de c_1 ne change pas", what)
		}
	}

	keep("métadonnées", `INSERT INTO _lattis_columns VALUES (1, 4, 'D', 'text', 3)`, `UPDATE _lattis_tables SET no_key_hint = 0`)
	if v := s.Version(ColumnOf("t_1", "c_1")); func() bool {
		task(`UPDATE _lattis_columns SET name = 'E'`)
		return s.Version(ColumnOf("t_1", "c_1")) == v
	}() {
		t.Error("sans NoMeta, les métadonnées ne comptent plus")
	}
	keep("ajout d'une colonne", `ALTER TABLE t_1 ADD COLUMN c_4 TEXT`)
	keep("suppression d'une autre colonne (corbeille)",
		`CREATE TABLE _lattis_trash_1_4_1 (id INTEGER PRIMARY KEY, v)`,
		`INSERT INTO _lattis_trash_1_4_1 (id, v) SELECT id, c_4 FROM t_1 WHERE c_4 IS NOT NULL`,
		`ALTER TABLE t_1 DROP COLUMN c_4`)
	keep("annulation de la suppression",
		`ALTER TABLE t_1 ADD COLUMN c_4 TEXT`,
		`UPDATE t_1 SET c_4 = _lattis_trash_1_4_1.v FROM _lattis_trash_1_4_1 WHERE _lattis_trash_1_4_1.id = t_1.id`,
		`DROP TABLE _lattis_trash_1_4_1`)
	keep("saisie dans une autre colonne", `UPDATE t_1 SET c_2 = 'z' WHERE id = 1`)
	keep("autre tableau", `INSERT INTO t_2 VALUES (1, 'a', 1)`, `DELETE FROM t_2`)
	keep("index", `CREATE UNIQUE INDEX t_1_key ON t_1(c_1)`, `DROP INDEX t_1_key`)
	keep("conversion d'une autre colonne (reconstruction)", rebuild(
		`id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 INTEGER, c_2 TEXT, c_3 TEXT, c_4 TEXT`,
		`id, _pos, c_1, c_2, c_3, c_4`,
		`id, _pos, c_1, c_2, CAST(c_3 AS TEXT), c_4`, `t_1`)...)
	keep("suppression de plusieurs colonnes (reconstruction)", append([]string{
		`CREATE TABLE _lattis_trash_1_cols_2 (id INTEGER PRIMARY KEY, c_3, c_4)`,
		`INSERT INTO _lattis_trash_1_cols_2 (id, c_3, c_4) SELECT id, c_3, c_4 FROM t_1 WHERE (c_3 IS NOT NULL OR c_4 IS NOT NULL)`,
	}, rebuild(`id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 INTEGER, c_2 TEXT`,
		`id, _pos, c_1, c_2`, `t_1.id, t_1._pos, t_1.c_1, t_1.c_2`, `t_1`)...)...)
	keep("annulation (reconstruction avec la corbeille)", rebuild(
		`id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 INTEGER, c_2 TEXT, c_3 TEXT, c_4 TEXT`,
		`id, _pos, c_1, c_2, c_3, c_4`,
		`t_1.id, t_1._pos, t_1.c_1, t_1.c_2, _lattis_trash_1_cols_2.c_3, _lattis_trash_1_cols_2.c_4`,
		`t_1 LEFT JOIN _lattis_trash_1_cols_2 ON _lattis_trash_1_cols_2.id = t_1.id`)...)

	// Tri : l'ordre change, pas c_1 ni le nombre de lignes.
	v, n, p := c1(), count(), pos()
	task(`CREATE TABLE _lattis_trash_sort_1_1 (id INTEGER PRIMARY KEY, pos TEXT NOT NULL)`,
		`INSERT INTO _lattis_trash_sort_1_1 SELECT id, _pos FROM t_1`,
		`CREATE TABLE _lattis_rank_1_1 (id INTEGER PRIMARY KEY, k TEXT NOT NULL)`,
		`INSERT INTO _lattis_rank_1_1 SELECT id, printf('%012dV', row_number() OVER (ORDER BY c_1 IS NULL, c_1 DESC, _pos)) FROM t_1`,
		`DROP INDEX t_1_pos`,
		`UPDATE t_1 SET _pos = r.k FROM _lattis_rank_1_1 AS r WHERE r.id = t_1.id`,
		`CREATE INDEX t_1_pos ON t_1(_pos)`,
		`DROP TABLE _lattis_rank_1_1`)
	if c1() != v || count() != n || pos() == p {
		t.Error("tri : seule la version de l'ordre doit changer")
	}

	change("saisie dans c_1", `UPDATE t_1 SET c_1 = 9 WHERE id = 1`)
	change("ajout d'une ligne", `INSERT INTO t_1 (id, _pos, c_1) VALUES (4, 'd', 4)`)
	change("suppression d'une ligne", `DELETE FROM t_1 WHERE id = 4`)
	change("conversion de c_1", rebuild(
		`id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT, c_2 TEXT, c_3 TEXT, c_4 TEXT`,
		`id, _pos, c_1, c_2, c_3, c_4`,
		`id, _pos, CAST(c_1 AS TEXT), c_2, c_3, c_4`, `t_1`)...)
	change("reconstruction modifiée avant le renommage", `CREATE TABLE t_1_rebuild (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT, c_2 TEXT, c_3 TEXT, c_4 TEXT) STRICT`,
		`INSERT INTO t_1_rebuild (id, _pos, c_1, c_2, c_3, c_4) SELECT id, _pos, c_1, c_2, c_3, c_4 FROM t_1`,
		`UPDATE t_1_rebuild SET c_1 = 'w'`,
		`DROP TABLE t_1`,
		`ALTER TABLE t_1_rebuild RENAME TO t_1`)
	change("reconstruction filtrée", rebuild(
		`id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT, c_2 TEXT, c_3 TEXT, c_4 TEXT`,
		`id, _pos, c_1, c_2, c_3, c_4`,
		`id, _pos, c_1, c_2, c_3, c_4`, `t_1 WHERE id > 1`)...)
	change("suppression de c_1", `ALTER TABLE t_1 DROP COLUMN c_1`)
	change("ajout de c_1", `ALTER TABLE t_1 ADD COLUMN c_1 TEXT`)
	change("tableau vidé", `DELETE FROM t_1`)
	task(`CREATE TEMP TABLE file (v)`,
		`CREATE TEMP TRIGGER file_i AFTER INSERT ON file BEGIN INSERT INTO t_1 (_pos, c_1) VALUES ('z', new.v); END`)
	change("ajout dans une autre table qui écrit t_1 (déclencheur)", `INSERT INTO file VALUES ('q')`)
	n = count()
	task(`UPDATE t_1 SET c_1 = 'r'`)
	if count() != n {
		t.Error("une saisie change la version du nombre de lignes")
	}
	task(`INSERT INTO t_1 (_pos, c_1) VALUES ('y', 'p')`)
	if count() == n {
		t.Error("un ajout de ligne garde la version du nombre de lignes")
	}
}
