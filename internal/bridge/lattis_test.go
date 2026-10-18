package bridge

import (
	"context"
	"errors"
	"testing"

	"github.com/lalamefine/lattis/internal/query"
)

func TestLattisOffByDefault(t *testing.T) {
	s := &DocumentService{}
	if st := s.LattisState(); st.Enabled || len(st.Servers) != 0 {
		t.Fatalf("LattisState = %+v", st)
	}
	if _, err := s.LattisDiscover(context.Background(), "https://lattis.acme.fr"); !errors.Is(err, errSharedOff) {
		t.Fatalf("LattisDiscover = %v", err)
	}
	if err := checkLattisConnection(query.Connection{Kind: query.KindLattis, Config: query.DBConfig{URL: "https://lattis.acme.fr"}}); !errors.Is(err, errSharedOff) {
		t.Fatalf("connexion lattis enregistrée sans la fonction : %v", err)
	}
	if err := checkLattisConnection(query.Connection{Kind: query.KindPostgres}); err != nil {
		t.Fatal(err)
	}
	if _, err := lattisOpen(context.Background(), "https://lattis.acme.fr", "sql"); !errors.Is(err, errSharedOff) {
		t.Fatalf("lattisOpen = %v", err)
	}
}
