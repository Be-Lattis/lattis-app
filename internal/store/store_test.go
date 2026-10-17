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
