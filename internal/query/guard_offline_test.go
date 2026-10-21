package query

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// Guard.Offline (démo publique) : aucune connexion, même vers un hôte de
// la liste ou public ; les fichiers de Dir restent lisibles.
func TestGuardOffline(t *testing.T) {
	dir := t.TempDir()
	g := &Guard{Dir: dir, Hosts: []string{"interne.exemple"}, Offline: true}
	ctx := context.Background()
	for _, h := range []string{"interne.exemple", "93.184.216.34:443", "db.exemple:5432", ""} {
		if err := g.Check(ctx, Access{Host: h}); !errors.Is(err, ErrDenied) {
			t.Errorf("Check(%q) = %v", h, err)
		}
	}
	if err := g.Check(ctx, Access{Path: filepath.Join(dir, "a.csv")}); err != nil {
		t.Errorf("fichier de Dir : %v", err)
	}
	if err := g.Check(ctx, Access{Path: filepath.Join(t.TempDir(), "b.csv")}); err == nil {
		t.Error("fichier hors de Dir permis")
	}

	// Même un serveur de bouclage qui répond : la connexion n'est pas tentée.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("connexion établie")
	}))
	defer srv.Close()
	g.Hosts = append(g.Hosts, srv.Listener.Addr().String())
	if _, err := (&http.Client{Transport: g.Transport()}).Get(srv.URL); !errors.Is(err, ErrDenied) {
		t.Errorf("GET : %v", err)
	}

	// Une base de données de la connexion : refusée avant le pilote.
	_, _, err := connectDB(WithGuard(ctx, g), NewSecrets(false, ""), Connection{Name: "pg", Kind: KindPostgres, Config: DBConfig{Host: "db.exemple"}})
	if !errors.Is(err, ErrDenied) {
		t.Errorf("connectDB : %v", err)
	}
}
