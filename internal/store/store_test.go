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
	exec(`CREATE TABLE _slitable_columns (x)`)
	s.TakeTouched()
	exec(`INSERT INTO t_2 VALUES (1)`)
	names, all := s.TakeTouched()
	if all || !names["t_2"] || names["t_1"] {
		t.Fatalf("après t_2 : %v %v", names, all)
	}
	if names, all = s.TakeTouched(); all || len(names) != 0 {
		t.Fatalf("remise à zéro : %v %v", names, all)
	}
	exec(`INSERT INTO _slitable_columns VALUES (1)`)
	if _, all = s.TakeTouched(); !all {
		t.Fatal("métadonnées des colonnes : tous les tableaux concernés")
	}
}
