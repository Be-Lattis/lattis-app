package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Démo publique : récents = démonstrations (la liste du poste n'est ni lue
// ni écrite), pas de Serveur Lattis, pas d'assistant, pas de récupération.
func TestPublicDemoServices(t *testing.T) {
	list := filepath.Join(t.TempDir(), "recent.json")
	old := recentFile
	recentFile = func() (string, error) { return list, nil }
	t.Cleanup(func() { recentFile = old; UsePublicDemo(false) })
	os.WriteFile(list, []byte(`["`+filepath.ToSlash(list)+`"]`), 0o644)

	UsePublicDemo(true)
	ctx := context.Background()
	s := &DocumentService{}
	got := s.Recent(ctx)
	if len(got) != len(DemoPresets) || got[0].Demo != "sml" || got[1].Demo != "med" || got[0].Path != "" {
		t.Fatalf("Recent = %+v", got)
	}
	before, _ := os.ReadFile(list)
	addRecent(filepath.Join(t.TempDir(), "x.lattis"))
	if after, _ := os.ReadFile(list); string(after) != string(before) {
		t.Fatalf("liste des récents écrite : %s", after)
	}
	if _, err := lattisOn(); !errors.Is(err, errDemo) {
		t.Fatalf("lattisOn = %v", err)
	}
	if _, err := sessions(); err == nil {
		t.Fatal("sessions : Serveur Lattis joignable en démo")
	}
	if st := s.LattisState(); st.Enabled {
		t.Fatal("LattisState.Enabled en démo")
	}
	if _, _, err := s.aiTurn(); !errors.Is(err, errDemo) {
		t.Fatalf("aiTurn = %v", err)
	}
	if o, err := s.Orphans(); err != nil || len(o) != 0 {
		t.Fatalf("Orphans = %v, %v", o, err)
	}
	if _, err := s.OpenPreset(ctx, "big"); !errors.Is(err, errDemo) {
		t.Fatalf("OpenPreset(big) = %v", err)
	}

	UsePublicDemo(false)
	if _, err := s.OpenPreset(ctx, "sml"); !errors.Is(err, errDemo) {
		t.Fatalf("OpenPreset hors démo = %v", err)
	}
}
