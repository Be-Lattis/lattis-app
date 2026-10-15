package meta

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/lalamefine/slitable/internal/store"
)

func TestMigrate(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "m.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for range 2 { // idempotent
		if err := Migrate(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	var v, sheets int
	db.Read().QueryRow(`PRAGMA user_version`).Scan(&v)
	db.Read().QueryRow(`SELECT count(*) FROM _slitable_sheets`).Scan(&sheets)
	if v != Version || sheets != 1 {
		t.Fatalf("version %d, %d feuilles", v, sheets)
	}
	db.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec(`PRAGMA user_version = 99`); return err })
	var tooNew ErrTooNew
	if err := Migrate(ctx, db); !errors.As(err, &tooNew) || tooNew.Found != 99 {
		t.Fatalf("format trop récent accepté : %v", err)
	}
}
