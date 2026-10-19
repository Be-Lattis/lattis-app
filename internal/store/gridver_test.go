package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// GridVersion suit les lignes écrites, même quand le texte de la requête ne
// nomme pas le tableau (déclencheur, vue de l'onglet SQL) ou la colonne du
// choix (feuille rétablie avec son choix) ; un réglage de feuille (zoom) ne
// change rien.
func TestGridVersion(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	exec := func(qs ...string) {
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
	exec(`CREATE TABLE t_1 (id INTEGER PRIMARY KEY, x, _pos)`, `CREATE TABLE t_2 (id INTEGER PRIMARY KEY, x)`,
		`CREATE TABLE _lattis_sheets (id INTEGER PRIMARY KEY, zoom, choice)`, `INSERT INTO _lattis_sheets VALUES (1, 100, NULL)`,
		`CREATE TABLE _lattis_cells (raw)`, `CREATE TABLE autre (x)`)
	deps := []string{"t_1"}
	rows, values, data := s.GridVersion(deps)
	step := func(what string, wantRows, wantValues, wantData bool, qs ...string) {
		t.Helper()
		exec(qs...)
		r, v, d := s.GridVersion(deps)
		if (r != rows) != wantRows || (v != values) != wantValues || (d != data) != wantData {
			t.Fatalf("%s : lignes %v, valeurs %v, données %v", what, r != rows, v != values, d != data)
		}
		rows, values, data = r, v, d
	}
	// Ligne ajoutée à t_1 par un déclencheur sur une autre table.
	step("déclencheur", true, true, true,
		`CREATE TEMP TRIGGER tr AFTER INSERT ON autre BEGIN INSERT INTO t_1 (x, _pos) VALUES (new.x, 'a'); END`,
		`INSERT INTO autre VALUES (1)`, `DROP TRIGGER tr`)
	step("valeur", false, true, true, `UPDATE t_1 SET x = 2`)
	step("ordre", true, true, true, `UPDATE t_1 SET _pos = 'b'`)
	step("autre tableau", false, false, true, `INSERT INTO t_2 (x) VALUES (1)`)
	step("cellule libre", false, false, true, `INSERT INTO _lattis_cells VALUES ('1')`)
	step("zoom", false, false, false, `UPDATE _lattis_sheets SET zoom = 120`)
	step("choix", true, false, false, `UPDATE _lattis_sheets SET zoom = 100, choice = '{}'`)
	step("feuille rétablie avec son choix", true, false, false, `INSERT INTO _lattis_sheets (id, zoom, choice) VALUES (2, 100, '{}')`)
	// Une lecture pendant une écriture ne sert plus après elle.
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM t_1`)
		r, _, _ := s.GridVersion(deps)
		rows = r
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if r, _, _ := s.GridVersion(deps); r == rows {
		t.Fatal("version lue pendant l'écriture encore valable après")
	}
}
