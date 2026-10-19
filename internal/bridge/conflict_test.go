package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/doc"
)

// isolateRecent garde la liste des fichiers récents du test hors du profil.
func isolateRecent(t *testing.T) {
	old := recentFile
	dir := t.TempDir()
	recentFile = func() (string, error) { return filepath.Join(dir, "recent.json"), nil }
	t.Cleanup(func() { recentFile = old })
}

func savedFile(t *testing.T, name, value string) string {
	t.Helper()
	ctx := context.Background()
	d, err := doc.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 0, value); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// Fichier modifié par un autre poste depuis l'ouverture : Enregistrer
// n'écrase rien et pose la question (DocState.Conflict) ; « Écraser »
// enregistre.
func TestSaveConflict(t *testing.T) {
	workRoot(t)
	isolateRecent(t)
	ctx := context.Background()
	path := savedFile(t, "commun.lattis", "moi")
	other, err := os.ReadFile(savedFile(t, "autre.lattis", "autre poste"))
	if err != nil {
		t.Fatal(err)
	}
	s := &DocumentService{}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	if r, err := s.Open(ctx, path); err != nil || r.Locked != nil {
		t.Fatalf("ouverture : %+v %v", r, err)
	}
	if _, err := s.SetCell(ctx, 1, 1, 0, "ajout"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, other, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := s.Save(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Conflict == nil || !strings.Contains(st.Conflict.Message, "modifié par ailleurs") || !st.Dirty {
		t.Fatalf("question attendue : %+v", st)
	}
	if got, _ := os.ReadFile(path); string(got) != string(other) {
		t.Fatal("fichier de l'autre poste écrasé sans question")
	}
	st, err = s.ResolveSave(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if st.Conflict != nil || st.Dirty {
		t.Fatalf("après « Écraser » : %+v", st)
	}
}

// Classeur ouvert sur un autre poste : Open pose la question
// (OpenResult.Locked) ; OpenLocked l'ouvre en lecture seule.
func TestOpenLocked(t *testing.T) {
	workRoot(t)
	isolateRecent(t)
	ctx := context.Background()
	path := savedFile(t, "partage.lattis", "x")
	b, _ := json.Marshal(archive.Holder{User: "Jean Dupont", Machine: "PC-COMPTA", PID: 1, Since: time.Now()})
	if err := os.WriteFile(archive.LockPath(path), b, 0o644); err != nil {
		t.Fatal(err)
	}
	s := &DocumentService{}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	r, err := s.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Locked == nil || !strings.Contains(r.Locked.Message, "Jean Dupont") || r.State.Open {
		t.Fatalf("question attendue : %+v", r)
	}
	r, err = s.OpenLocked(ctx, path, true)
	if err != nil {
		t.Fatal(err)
	}
	if !r.State.ReadOnly || !strings.HasSuffix(r.State.Title, "(lecture seule)") {
		t.Fatalf("lecture seule attendue : %+v", r.State)
	}
}
