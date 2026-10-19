package meta

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/store"
)

func open(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "m.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func exec(t *testing.T, db *store.Store, qs ...string) {
	t.Helper()
	err := db.Write(context.Background(), func(tx *sql.Tx) error {
		for _, q := range qs {
			if _, err := tx.Exec(q); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// schema donne le schéma (hors données) d'une base, pour comparer.
func schema(t *testing.T, db *store.Store) string {
	t.Helper()
	rows, err := db.Read().Query(`SELECT type, name, coalesce(sql, '') FROM sqlite_master WHERE name NOT LIKE 't\_%' ESCAPE '\' ORDER BY type, name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var typ, name, q string
		rows.Scan(&typ, &name, &q)
		b.WriteString(typ + " " + name + " " + strings.Join(strings.Fields(q), " ") + "\n")
	}
	return b.String()
}

func version(t *testing.T, db *store.Store) int {
	t.Helper()
	var v int
	db.Read().QueryRow(`PRAGMA user_version`).Scan(&v)
	return v
}

// TestMigrateEveryVersion : un classeur de chaque format, du plus ancien au
// précédent, avec des données, passe au format courant ; ses données
// restent et son schéma est celui d'un classeur neuf.
func TestMigrateEveryVersion(t *testing.T) {
	ctx := context.Background()
	fresh := open(t)
	if err := Migrate(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	want := schema(t, fresh)
	for v := 1; v < Version; v++ {
		db := open(t)
		if err := MigrateTo(ctx, db, v); err != nil {
			t.Fatalf("format %d : %v", v, err)
		}
		// Données du format 1 : feuilles, tableau, colonne, cellules, journal.
		exec(t, db,
			`INSERT INTO _lattis_sheets (id, name, pos) VALUES (2, 'Données', 1)`,
			`INSERT INTO _lattis_tables (id, sheet, name, row, col) VALUES (1, 2, 'Ventes', 0, 0)`,
			`INSERT INTO _lattis_columns (tbl, id, name, type, pos) VALUES (1, 1, 'Prix', 'number', 0)`,
			`CREATE TABLE t_1 (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 REAL)`,
			`INSERT INTO t_1 (_pos, c_1) VALUES ('a', 1.5), ('b', 2.5)`,
			`INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (1, 5, 5, 'libre')`,
			`INSERT INTO _lattis_journal (name, steps) VALUES ('Saisie', '[]')`,
		)
		from, err := MigrateFrom(ctx, db)
		if err != nil || from != v || version(t, db) != Version {
			t.Fatalf("format %d : origine %d, version %d, %v", v, from, version(t, db), err)
		}
		if got := schema(t, db); got != want {
			t.Fatalf("format %d migré : schéma\n%s\nattendu\n%s", v, got, want)
		}
		var sheets, rows, cells, journal int
		var grid int
		db.Read().QueryRow(`SELECT count(*) FROM _lattis_sheets`).Scan(&sheets)
		db.Read().QueryRow(`SELECT count(*) FROM t_1`).Scan(&rows)
		db.Read().QueryRow(`SELECT count(*) FROM _lattis_cells`).Scan(&cells)
		db.Read().QueryRow(`SELECT count(*) FROM _lattis_journal`).Scan(&journal)
		db.Read().QueryRow(`SELECT gridlines FROM _lattis_sheets WHERE id = 2`).Scan(&grid)
		if sheets != 2 || rows != 2 || cells != 1 || journal != 1 {
			t.Fatalf("format %d migré : %d feuilles, %d lignes, %d cellules, %d commandes", v, sheets, rows, cells, journal)
		}
		// Un classeur existant garde son quadrillage (seul un neuf n'en a pas).
		if grid != 1 {
			t.Fatalf("format %d migré : quadrillage %d", v, grid)
		}
		var id string
		if err := db.Read().QueryRow(`SELECT value FROM _lattis_info WHERE key = 'id'`).Scan(&id); err != nil || len(id) != 32 {
			t.Fatalf("format %d migré : identifiant %q %v", v, id, err)
		}
	}
}

// TestMigrateAtomic : une étape qui échoue annule toute la migration ; la
// base reste à son format, sans aucune des étapes précédentes.
func TestMigrateAtomic(t *testing.T) {
	ctx := context.Background()
	db := open(t)
	if err := MigrateTo(ctx, db, 2); err != nil {
		t.Fatal(err)
	}
	// La migration 3 → 4 crée _lattis_names : déjà là, elle échoue.
	exec(t, db, `CREATE TABLE _lattis_names (x)`)
	before := schema(t, db)
	_, err := MigrateFrom(ctx, db)
	if err == nil || !strings.Contains(err.Error(), "format 3 au format 4") {
		t.Fatalf("migration en échec : %v", err)
	}
	if v := version(t, db); v != 2 {
		t.Fatalf("version %d après un échec, attendu 2", v)
	}
	if got := schema(t, db); got != before {
		t.Fatalf("schéma modifié par une migration en échec :\n%s\navant\n%s", got, before)
	}
}

// TestMigrateRefuses : une base d'un format plus récent ou étrangère à
// Lattis est refusée, sans être touchée.
func TestMigrateRefuses(t *testing.T) {
	ctx := context.Background()
	db := open(t)
	exec(t, db, `PRAGMA user_version = 99`)
	var tooNew ErrTooNew
	if _, err := MigrateFrom(ctx, db); !errors.As(err, &tooNew) || !strings.Contains(err.Error(), "plus récente") {
		t.Fatalf("format 99 : %v", err)
	}
	if version(t, db) != 99 || schema(t, db) != "" {
		t.Fatal("base trop récente modifiée")
	}

	db = open(t)
	exec(t, db, `CREATE TABLE clients (nom TEXT)`)
	if _, err := MigrateFrom(ctx, db); !errors.Is(err, ErrForeign) {
		t.Fatalf("base étrangère : %v", err)
	}
	if version(t, db) != 0 || strings.Contains(schema(t, db), "_lattis") {
		t.Fatal("base étrangère modifiée")
	}
}

// TestMigrateCollaboration : le format 28 ajoute les tables des classeurs
// partagés, vides ; les tableaux existants ne changent pas (leurs
// identifiants partagés ne sont posés qu'à la publication).
func TestMigrateCollaboration(t *testing.T) {
	ctx := context.Background()
	db := open(t)
	if err := MigrateTo(ctx, db, 27); err != nil {
		t.Fatal(err)
	}
	exec(t, db,
		`INSERT INTO _lattis_tables (id, sheet, name, row, col) VALUES (1, 1, 'Ventes', 0, 0)`,
		`CREATE TABLE t_1 (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 REAL) STRICT`,
		`INSERT INTO t_1 (_pos, c_1) VALUES ('a', 1.5)`,
	)
	before := tableSQL(t, db, "t_1")
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if got := tableSQL(t, db, "t_1"); got != before {
		t.Fatalf("tableau modifié par la migration : %s", got)
	}
	for _, name := range []string{"_lattis_remote_tables", "_lattis_row_ids", "_lattis_outbox", "_lattis_conflicts", "_lattis_capabilities"} {
		var n int
		if err := db.Read().QueryRow(`SELECT count(*) FROM ` + name).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s : %d lignes, %v", name, n, err)
		}
	}
	err := db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO _lattis_outbox (kind, batch, ops, state) VALUES ('do', 1, '[]', 'perdu')`)
		return err
	})
	if err == nil {
		t.Fatal("état de la file d'envoi inconnu accepté")
	}
	exec(t, db,
		`INSERT INTO _lattis_row_ids (tbl, rid, lattis_id) VALUES (1, 1, '0192aaaa-0000-7000-8000-000000000001')`)
	err = db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO _lattis_row_ids (tbl, rid, lattis_id) VALUES (1, 2, '0192aaaa-0000-7000-8000-000000000001')`)
		return err
	})
	if err == nil {
		t.Fatal("identifiant partagé en double accepté")
	}
}

func tableSQL(t *testing.T, db *store.Store, name string) string {
	t.Helper()
	var q string
	if err := db.Read().QueryRow(`SELECT sql FROM sqlite_master WHERE name = ?`, name).Scan(&q); err != nil {
		t.Fatal(err)
	}
	return q
}

// TestMigrateOpenDropsTriggers : les déclencheurs du fichier (Lattis n'en
// crée aucun de persistant) sont supprimés avant la migration ; vues et
// index inconnus restent.
func TestMigrateOpenDropsTriggers(t *testing.T) {
	ctx := context.Background()
	db := open(t)
	if err := MigrateTo(ctx, db, Version-1); err != nil {
		t.Fatal(err)
	}
	exec(t, db,
		`CREATE TRIGGER "piège ""1""" AFTER UPDATE ON _lattis_sheets BEGIN SELECT RAISE(ABORT, 'piège'); END`,
		`CREATE TRIGGER piege2 BEFORE INSERT ON _lattis_cells BEGIN SELECT RAISE(ABORT, 'piège'); END`,
		`CREATE VIEW v_inconnue AS SELECT 1`,
	)
	r, err := MigrateOpen(ctx, db)
	if err != nil || r.From != Version-1 || version(t, db) != Version {
		t.Fatalf("origine %d, %v", r.From, err)
	}
	if strings.Join(r.Dropped, "|") != `piege2|piège "1"` {
		t.Fatalf("supprimés : %q", r.Dropped)
	}
	if s := schema(t, db); strings.Contains(s, "TRIGGER") || !strings.Contains(s, "v_inconnue") {
		t.Fatalf("schéma :\n%s", s)
	}
	exec(t, db, `UPDATE _lattis_sheets SET name = name`, `INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (1, 0, 0, 'x')`)
	// Une base saine : rien à signaler.
	if r, err := MigrateOpen(ctx, db); err != nil || len(r.Dropped) != 0 {
		t.Fatalf("réouverture : %v %v", r.Dropped, err)
	}
}
