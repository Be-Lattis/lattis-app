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
