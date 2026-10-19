package bridge

import (
	"context"
	"errors"
	"testing"

	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/query"
)

// Fonction coupée par la configuration (elle est active par défaut).
func TestLattisOff(t *testing.T) {
	defer config.SetCurrent(config.Current())
	off := config.Defaults()
	off["features"].(config.Tree)["sharedWorkbooks"] = false
	config.SetCurrent(config.Resolve(context.Background(), config.Options{}, &config.TreeSource{N: "essai", R: config.RankDefault, T: off}))
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
