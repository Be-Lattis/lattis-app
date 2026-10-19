package bridge

import (
	"context"
	"testing"
)

// Classeur d'exemple de l'accueil (audit 6, I8) : la vitrine, ouverte sans
// rien à enregistrer ni à récupérer, sans toucher à la démonstration
// demandée au lancement.
func TestOpenSample(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	s := &DocumentService{}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	r, err := s.OpenSample(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !r.State.Open || r.State.Dirty || r.Sheet == 0 {
		t.Fatalf("classeur d'exemple : %+v", r)
	}
	d, err := s.current()
	if err != nil {
		t.Fatal(err)
	}
	if !d.Pristine() {
		t.Fatal("classeur d'exemple proposé à la récupération")
	}
	if s.DemoRequested() != "" {
		t.Fatal("l'exemple passe pour une démonstration demandée au lancement")
	}
	if p := s.DemoProgress(); p.Preset != "" {
		t.Fatalf("avancement après l'ouverture : %+v", p)
	}
	l, err := s.Layout(ctx, r.Sheet)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Sheets) < 5 {
		t.Fatalf("%d onglets seulement", len(l.Sheets))
	}
}
