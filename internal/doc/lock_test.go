package doc

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
)

// Classeur ouvert sur un autre poste : Open le passe en lecture seule avec
// un message qui nomme le détenteur ; OpenAnyway l'ouvre sans toucher à son
// verrou, OpenReadOnly en lecture seule.
func TestOpenLockedByOther(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "partage.lattis")
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "x")
	must(t, d.Save(ctx, path))
	must(t, d.Close())

	b, _ := json.Marshal(archive.Holder{User: "Jean Dupont", Login: "jdupont", Machine: "PC-COMPTA", PID: 1, Since: time.Now()})
	lp := archive.LockPath(path)
	must(t, os.WriteFile(lp, b, 0o644))

	d, err = Open(ctx, path)
	must(t, err)
	if !d.ReadOnly() {
		t.Fatal("classeur verrouillé ouvert en écriture")
	}
	if n := d.TakeOpenNotice(); !strings.Contains(n, "Jean Dupont") || !strings.Contains(n, "lecture seule") {
		t.Fatalf("message %q", n)
	}
	set(t, d, 1, 0, "y")
	if err := d.Save(ctx, ""); !errors.Is(err, archive.ErrReadOnly) {
		t.Fatalf("enregistrement en lecture seule : %v", err)
	}
	must(t, d.Close())
	if got, _ := os.ReadFile(lp); string(got) != string(b) {
		t.Fatal("verrou de l'autre poste touché")
	}

	d, err = OpenAs(ctx, path, OpenAnyway)
	must(t, err)
	if d.ReadOnly() || d.TakeOpenNotice() != "" {
		t.Fatal("ouvert quand même : lecture seule ou message")
	}
	set(t, d, 1, 0, "y")
	must(t, d.Save(ctx, ""))
	must(t, d.Close())
	if got, _ := os.ReadFile(lp); string(got) != string(b) {
		t.Fatal("verrou de l'autre poste touché par « ouvrir quand même »")
	}

	d, err = OpenAs(ctx, path, OpenReadOnly)
	must(t, err)
	defer d.Close()
	if !d.ReadOnly() {
		t.Fatal("OpenReadOnly modifiable")
	}
}

// Verrou impossible à poser (dossier en lecture seule) : le classeur
// s'ouvre normalement, avec un avertissement unique.
func TestOpenLockImpossible(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bloque.lattis")
	d, err := New(ctx)
	must(t, err)
	must(t, d.Save(ctx, path))
	must(t, d.Close())
	must(t, os.MkdirAll(filepath.Join(archive.LockPath(path), "x"), 0o755))

	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	if d.ReadOnly() {
		t.Fatal("lecture seule sans verrou d'un autre")
	}
	if n := d.TakeOpenNotice(); !strings.Contains(n, "Impossible de poser le verrou") {
		t.Fatalf("message %q", n)
	}
	if n := d.TakeOpenNotice(); n != "" {
		t.Fatalf("message répété : %q", n)
	}
}
