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
	} {
		table, cols, ok := setColumns(c.q)
		if !ok || table != c.table || !slices.Equal(cols, c.cols) {
			t.Errorf("%s : %s %v %v", c.q, table, cols, ok)
		}
	}
	for _, q := range []string{
		`UPDATE t_3 SET _pos = ? WHERE id = ?`,
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
