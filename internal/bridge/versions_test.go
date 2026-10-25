package bridge

import (
	"context"
	"path/filepath"
	"testing"
)

// cellRaw lit la cellule libre (row, col) de la feuille 1 du classeur ouvert.
func cellRaw(t *testing.T, s *DocumentService, row, col int) string {
	t.Helper()
	d, err := s.current()
	if err != nil {
		t.Fatal(err)
	}
	var raw string
	d.DB().Read().QueryRow(`SELECT raw FROM _lattis_cells WHERE sheet = 1 AND row = ? AND col = ?`, row, col).Scan(&raw)
	return raw
}

// Historique des versions d'un classeur local : une version à chaque
// enregistrement, une version nommée, ouverture en lecture seule,
// restauration (défaite en restaurant la version d'avant).
func TestVersionHistory(t *testing.T) {
	ctx := context.Background()
	p := filepath.Join(t.TempDir(), "Budget.lattis")
	s := &DocumentService{}
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	if v, _ := s.Versions(ctx); v.Reason == "" {
		t.Fatal("classeur jamais enregistré : pas d'historique attendu")
	}
	set := func(raw string) {
		t.Helper()
		if _, err := s.SetCell(ctx, 1, 0, 0, raw); err != nil {
			t.Fatal(err)
		}
	}
	set("premier")
	d, _ := s.current()
	if err := saveDoc(ctx, d, p); err != nil {
		t.Fatal(err)
	}
	set("jalon")
	if _, err := s.SaveVersion(ctx, "Jalon"); err != nil {
		t.Fatal(err)
	}
	set("dernier")
	if err := saveDoc(ctx, d, ""); err != nil {
		t.Fatal(err)
	}
	v, err := s.Versions(ctx)
	if err != nil || v.Reason != "" || len(v.Entries) != 2 || v.Entries[1].Name != "Jalon" || v.Entries[0].Name != "" {
		t.Fatalf("historique : %+v %v", v, err)
	}
	jalon := v.Entries[1].ID
	// Ouvrir la version nommée en lecture seule.
	r, err := s.OpenVersion(ctx, jalon)
	if err != nil || !r.State.ReadOnly || r.State.Notice == "" {
		t.Fatalf("version ouverte : %+v %v", r.State, err)
	}
	if got := cellRaw(t, s, 0, 0); got != "jalon" {
		t.Fatalf("contenu de la version : %q", got)
	}
	if v, _ = s.Versions(ctx); v.Viewing != jalon || len(v.Entries) != 2 {
		t.Fatalf("historique vu depuis la version : %+v", v)
	}
	// La restaurer : le classeur rouvert la montre, la version d'avant est gardée.
	if r, err = s.RestoreVersion(ctx, jalon); err != nil || r.State.ReadOnly || r.State.Path != p {
		t.Fatalf("restauration : %+v %v", r.State, err)
	}
	if got := cellRaw(t, s, 0, 0); got != "jalon" {
		t.Fatalf("après restauration : %q", got)
	}
	v, _ = s.Versions(ctx)
	if len(v.Entries) != 2 || v.Entries[0].Name != "Avant restauration" {
		t.Fatalf("version d'avant la restauration : %+v", v.Entries)
	}
	// Défaire : restaurer la version d'avant.
	if _, err := s.RestoreVersion(ctx, v.Entries[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := cellRaw(t, s, 0, 0); got != "dernier" {
		t.Fatalf("restauration défaite : %q", got)
	}
}
