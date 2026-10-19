package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// Une panique de SQLite pendant une écriture (mémoire épuisée…) devient une
// erreur : l'écriture est annulée et la base reste utilisable.
func TestWritePanic(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	err = s.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE t (x)`); err != nil {
			return err
		}
		panic("sqlite3: out of memory")
	})
	if !errors.Is(err, ErrEngine) {
		t.Fatalf("erreur attendue, %v", err)
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE t (x)`)
		return err
	}); err != nil {
		t.Fatalf("base inutilisable après la panique : %v", err)
	}
}

// TakeTouched rend les tableaux modifiés depuis l'appel précédent : une
// écriture sur t_2 laisse t_1 valable ; une écriture sur les métadonnées des
// colonnes concerne tous les tableaux.
func TestTouched(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	exec := func(q string) {
		t.Helper()
		if err := s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec(q); return err }); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE t_1 (x)`)
	exec(`CREATE TABLE t_2 (x)`)
	exec(`CREATE TABLE _lattis_columns (x)`)
	s.TakeTouched()
	exec(`INSERT INTO t_2 VALUES (1)`)
	exec(`UPDATE t_1 SET x = 2`)
	content, structure, all := s.TakeTouched()
	if all || !structure["t_2"] || structure["t_1"] || !content["t_1"] {
		t.Fatalf("après t_2 et t_1 : %v %v %v", content, structure, all)
	}
	if content, structure, all = s.TakeTouched(); all || len(content)+len(structure) != 0 {
		t.Fatalf("remise à zéro : %v %v %v", content, structure, all)
	}
	exec(`INSERT INTO _lattis_columns VALUES (1)`)
	if _, _, all = s.TakeTouched(); !all {
		t.Fatal("métadonnées des colonnes : tous les tableaux concernés")
	}
}

// Versions par tableau : une saisie ne change pas celle de ses lignes ; une
// écriture ailleurs ne change rien.
func TestVersion(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	exec := func(q string) {
		t.Helper()
		if err := s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec(q); return err }); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE t_1 (x)`)
	exec(`CREATE TABLE t_2 (x)`)
	all, rows := s.Version("t_1"), s.Version(RowsOf("t_1"))
	exec(`INSERT INTO t_2 VALUES (1)`)
	if s.Version("t_1") != all || s.Version(RowsOf("t_1")) != rows {
		t.Fatal("une écriture dans t_2 change la version de t_1")
	}
	exec(`UPDATE t_1 SET x = 2`)
	if s.Version("t_1") == all || s.Version(RowsOf("t_1")) != rows {
		t.Fatal("une mise à jour de valeurs : version des valeurs seulement")
	}
	exec(`INSERT INTO t_1 VALUES (3)`)
	if s.Version(RowsOf("t_1")) == rows {
		t.Fatal("un ajout de ligne change la version des lignes")
	}
	if s.Quiet() < 0 {
		t.Fatal("aucune écriture ne tourne")
	}
}
