package doc

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Un fichier piégé (déclencheurs écrits hors de Lattis) : à l'ouverture,
// les déclencheurs sont supprimés avant toute écriture du moteur, et
// l'utilisateur est prévenu une fois. Le fichier ne change qu'à
// l'enregistrement ; les vues et index inconnus restent.
func TestUnknownTriggersDropped(t *testing.T) {
	useRoot(t)
	otherMachine(t)
	ctx := context.Background()
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "1")
	// Écrits directement dans la base, comme le ferait un outil SQLite.
	must(t, d.DB().Write(ctx, func(tx *sql.Tx) error {
		for _, q := range []string{
			`CREATE TRIGGER evil_cells AFTER INSERT ON _lattis_cells BEGIN INSERT OR REPLACE INTO _lattis_info (key, value) VALUES ('pirate', 'cellule'); END`,
			`CREATE TRIGGER "evil ""sheets""" AFTER UPDATE ON _lattis_sheets BEGIN INSERT OR REPLACE INTO _lattis_info (key, value) VALUES ('pirate', 'feuille'); END`,
			`CREATE VIEW vue_inconnue AS SELECT 1 AS x`,
			`CREATE INDEX index_inconnu ON _lattis_cells (raw)`,
		} {
			if _, err := tx.Exec(q); err != nil {
				return err
			}
		}
		return nil
	}))
	path := filepath.Join(t.TempDir(), "piege.lattis")
	must(t, d.Save(ctx, path))
	must(t, d.Close())
	before, err := os.ReadFile(path)
	must(t, err)

	d, err = Open(ctx, path)
	must(t, err)
	msg := d.TakeOpenNotice()
	if !strings.Contains(msg, "2 déclencheurs inconnus supprimés du classeur (sécurité)") || !strings.Contains(msg, "evil_cells") {
		t.Fatalf("message : %q", msg)
	}
	if again := d.TakeOpenNotice(); again != "" {
		t.Fatalf("message répété : %q", again)
	}
	count := func(typ string) int {
		var n int
		must(t, d.DB().Read().QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = ?`, typ).Scan(&n))
		return n
	}
	if count("trigger") != 0 {
		t.Fatal("déclencheurs restés")
	}
	if count("view") == 0 || count("index") == 0 {
		t.Fatal("vue ou index inconnus supprimés")
	}
	set(t, d, 1, 1, "2")
	var pirate int
	must(t, d.DB().Read().QueryRowContext(ctx, `SELECT count(*) FROM _lattis_info WHERE key = 'pirate'`).Scan(&pirate))
	if pirate != 0 {
		t.Fatal("un déclencheur du fichier s'est exécuté")
	}
	must(t, d.Close())
	after, err := os.ReadFile(path)
	must(t, err)
	if !bytes.Equal(before, after) {
		t.Fatal("fichier modifié sans enregistrement")
	}

	// Le classeur est à enregistrer ; enregistré, il n'en a plus.
	d, err = Open(ctx, path)
	must(t, err)
	if dirty, _ := d.Dirty(ctx); !dirty {
		t.Fatal("classeur nettoyé non marqué à enregistrer")
	}
	d.TakeOpenNotice()
	must(t, d.Save(ctx, ""))
	must(t, d.Close())
	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	if msg := d.TakeOpenNotice(); msg != "" {
		t.Fatalf("enregistré : %q", msg)
	}
}

// Un classeur créé par Lattis (cellules, script, page web) s'ouvre sans
// message ni modification.
func TestLattisWorkbookNoTriggerNotice(t *testing.T) {
	useRoot(t)
	otherMachine(t)
	ctx := context.Background()
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "1")
	_, err = d.SaveScript(ctx, 0, "Calcul", `print(1)`)
	must(t, err)
	_, err = d.AddPage(ctx, 0)
	must(t, err)
	path := filepath.Join(t.TempDir(), "sain.lattis")
	must(t, d.Save(ctx, path))
	must(t, d.Close())
	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	if msg := d.TakeOpenNotice(); msg != "" {
		t.Fatalf("message : %q", msg)
	}
	if dirty, _ := d.Dirty(ctx); dirty {
		t.Fatal("classeur sain marqué modifié")
	}
}
