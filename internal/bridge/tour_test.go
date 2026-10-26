package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// useTourFile place le témoin de la visite guidée dans dir.
func useTourFile(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "config", "Lattis", "tour")
	old := tourFile
	tourFile = func() (string, error) { return p, nil }
	t.Cleanup(func() { tourFile = old })
	return p
}

func TestTourState(t *testing.T) {
	useTourFile(t, t.TempDir())
	s := &DocumentService{}
	UseHost(NoHost{}, nil, s)
	ctx := context.Background()
	if got := s.TourState(ctx); got != "" {
		t.Fatalf("poste neuf : %q, attendu vide", got)
	}
	s.SetTourState(ctx, "n'importe quoi")
	if got := s.TourState(ctx); got != "" {
		t.Fatalf("valeur inconnue retenue : %q", got)
	}
	s.SetTourState(ctx, "skipped")
	if got := s.TourState(ctx); got != "skipped" {
		t.Fatalf("après Passer : %q", got)
	}
	s.SetTourState(ctx, "done")
	if got := s.TourState(ctx); got != "done" {
		t.Fatalf("après la visite : %q", got)
	}
}

func TestTourStateWeb(t *testing.T) {
	p := useTourFile(t, t.TempDir())
	s := &DocumentService{}
	UseHost(webLike{}, nil, s)
	s.SetTourState(context.Background(), "done")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("édition web : témoin écrit (%v)", err)
	}
	if got := s.TourState(context.Background()); got != "" {
		t.Fatalf("édition web : %q, attendu vide", got)
	}
}

func TestTourStateDamaged(t *testing.T) {
	p := useTourFile(t, t.TempDir())
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("???\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &DocumentService{}
	UseHost(NoHost{}, nil, s)
	if got := s.TourState(context.Background()); got != "" {
		t.Fatalf("témoin illisible : %q, attendu vide", got)
	}
}
