package archive

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// Travail à plusieurs : positions des journaux dans le manifeste, et
// « modifié par ailleurs » ignoré quand le fichier ne contient que ce qui
// est déjà connu (Known).
func TestJournalManifest(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "a.lattis")
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	exec1(t, d, `CREATE TABLE t (x)`)
	d.Journal = func(ctx context.Context, db *sql.DB) (map[string]int64, error) {
		return map[string]int64{"PC-a-1-ab12-1": 12, "PC \"b\"-2": 3}, nil
	}
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Journal) != 2 || m.Journal["PC-a-1-ab12-1"] != 12 || m.Journal["PC \"b\"-2"] != 3 {
		t.Fatalf("positions relues : %v", m.Journal)
	}

	// Un autre poste enregistre par-dessus.
	useRoot(t)
	o, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	o.Journal = func(ctx context.Context, db *sql.DB) (map[string]int64, error) {
		return map[string]int64{"PC-a-1-ab12-1": 20}, nil
	}
	exec1(t, o, `INSERT INTO t VALUES (1)`)
	if err := o.Save(ctx, ""); err != nil {
		t.Fatal(err)
	}
	o.Close()
	exec1(t, d, `INSERT INTO t VALUES (2)`)
	var changed *ChangedError
	if err := d.Save(ctx, ""); !errors.As(err, &changed) {
		t.Fatalf("sans Known : %v", err)
	}
	d.Known = func(m Manifest) bool { return m.Journal["PC-a-1-ab12-1"] <= 20 }
	if err := d.Save(ctx, ""); err != nil {
		t.Fatalf("avec Known : %v", err)
	}
	d.Known = func(m Manifest) bool { return false }
	exec1(t, d, `INSERT INTO t VALUES (3)`)
	if err := d.Save(ctx, ""); err != nil {
		t.Fatalf("fichier inchangé depuis le dernier enregistrement : %v", err)
	}
}
