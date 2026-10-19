package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func authStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	err = s.Write(ctx, func(tx *sql.Tx) error {
		for _, q := range []string{
			`CREATE TABLE t_1 (id INTEGER PRIMARY KEY, c_1)`,
			`INSERT INTO t_1 VALUES (1, 'a'), (2, 'b')`,
			`CREATE TABLE _lattis_info (key TEXT PRIMARY KEY, value)`,
			`INSERT INTO _lattis_info VALUES ('id', 'secret')`,
		} {
			if _, err := tx.Exec(q); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Une requête de lecture de l'utilisateur ne lit que les tableaux : ni les
// tables internes, ni un autre fichier, ni un PRAGMA qui change la
// connexion ; le moteur garde, lui, un accès complet.
func TestUserReadAuthorizer(t *testing.T) {
	ctx := context.Background()
	s := authStore(t)
	dir := t.TempDir()
	read := func(q string) error {
		c, err := UserConn(ctx, s.Read())
		if err != nil {
			t.Fatal(err)
		}
		defer ReleaseUserConn(c)
		rs, err := c.QueryContext(ctx, q)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
		}
		return rs.Err()
	}
	for _, q := range []string{
		`SELECT count(*) FROM t_1`,
		`WITH x AS (SELECT c_1 FROM main.t_1) SELECT * FROM x`,
		`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 3) SELECT * FROM n`,
		`SELECT name FROM sqlite_schema`,
		`SELECT * FROM pragma_table_info('t_1')`,
		`PRAGMA table_info(t_1)`,
		`SELECT upper(c_1), json_array(1, 2) FROM t_1`,
	} {
		if err := read(q); err != nil {
			t.Errorf("refusé : %s : %v", q, err)
		}
	}
	for _, q := range []string{
		`SELECT * FROM _lattis_info`,
		`SELECT * FROM t_1 WHERE c_1 IN (SELECT value FROM _LATTIS_INFO)`,
		`ATTACH DATABASE '` + filepath.Join(dir, "a.db") + `' AS a`,
		`PRAGMA query_only = 0`,
		`PRAGMA writable_schema = 1`,
		`SELECT load_extension('x')`,
		`CREATE TEMP TABLE x (a)`,
		`SELECT * FROM temp.sqlite_master`,
		`BEGIN`,
	} {
		if err := read(q); err == nil {
			t.Errorf("accepté : %s", q)
		} else if !AuthDenied(err) && !strings.Contains(err.Error(), "load_extension") {
			t.Logf("%s : %v", q, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "a.db")); err == nil {
		t.Fatal("ATTACH a créé un fichier")
	}
	// La connexion rendue au pool n'a plus d'autorisateur.
	for range 8 {
		var v string
		if err := s.Read().QueryRowContext(ctx, `SELECT value FROM _lattis_info`).Scan(&v); err != nil || v != "secret" {
			t.Fatalf("moteur : %q %v", v, err)
		}
	}
}

// Une écriture de l'utilisateur (AsUser) touche les tableaux, pas les
// tables internes ni la structure ; les déclencheurs du moteur, eux,
// s'exécutent.
func TestUserWriteAuthorizer(t *testing.T) {
	ctx := context.Background()
	s := authStore(t)
	dir := t.TempDir()
	write := func(q string) error {
		return s.Write(ctx, func(tx *sql.Tx) error {
			for _, st := range []string{
				`CREATE TEMP TABLE _slt_log (id)`,
				`CREATE TEMP TRIGGER _slt_t AFTER UPDATE ON main.t_1 BEGIN INSERT INTO _slt_log VALUES (new.id); UPDATE _lattis_info SET value = 'vu' WHERE key = 'id'; END`,
				`CREATE TEMP VIEW "Tableau" (c) AS SELECT c_1 FROM main.t_1`,
			} {
				if _, err := tx.Exec(st); err != nil {
					return err
				}
			}
			defer func() {
				for _, st := range []string{`DROP TRIGGER temp._slt_t`, `DROP TABLE temp._slt_log`, `DROP VIEW temp."Tableau"`} {
					tx.Exec(st)
				}
			}()
			return s.AsUser(func() error {
				_, err := tx.ExecContext(ctx, q)
				return err
			})
		})
	}
	if err := write(`UPDATE t_1 SET c_1 = 'z' WHERE id = 1`); err != nil {
		t.Fatalf("écriture refusée : %v", err)
	}
	var v string
	if err := s.Read().QueryRowContext(ctx, `SELECT value FROM _lattis_info`).Scan(&v); err != nil || v != "vu" {
		t.Fatalf("déclencheur du moteur : %q %v", v, err)
	}
	if err := write(`DELETE FROM t_1 WHERE c_1 IN (SELECT c FROM "Tableau" WHERE c = 'b')`); err != nil {
		t.Fatalf("vue temporaire du moteur : %v", err)
	}
	for _, q := range []string{
		`UPDATE _lattis_info SET value = 'x'`,
		`INSERT INTO t_1 (c_1) SELECT value FROM _lattis_info`,
		`DELETE FROM temp._slt_log`,
		`ATTACH DATABASE '` + filepath.Join(dir, "b.db") + `' AS b`,
		`CREATE TEMP TRIGGER pirate AFTER INSERT ON main.t_1 BEGIN DELETE FROM t_1; END`,
		`CREATE TABLE x (a)`,
		`DROP TABLE t_1`,
		`ALTER TABLE t_1 ADD COLUMN c_2`,
		`COMMIT`,
		`SAVEPOINT s`,
		`PRAGMA foreign_keys = 0`,
		`UPDATE sqlite_master SET sql = 'x'`,
		`DELETE FROM t_1; ATTACH DATABASE '` + filepath.Join(dir, "c.db") + `' AS c`,
	} {
		if err := write(q); err == nil {
			t.Errorf("accepté : %s", q)
		}
	}
	for _, f := range []string{"b.db", "c.db"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Fatalf("ATTACH a créé %s", f)
		}
	}
	// L'écrivain retrouve un accès complet.
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE _lattis_info SET value = 'moteur'`)
		return err
	}); err != nil {
		t.Fatalf("moteur : %v", err)
	}
}
