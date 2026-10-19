package query

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/remote/auth"
)

func TestLattisConnection(t *testing.T) {
	c := Connection{Name: "Acme", Kind: KindLattis, Config: DBConfig{URL: "https://Lattis.Acme.fr/", Label: "Serveur Acme"}}
	def, err := EncodeConnection(c)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeConnection(def)
	if err != nil || back.Config.Label != "Serveur Acme" || back.Config.URL != "https://Lattis.Acme.fr/" {
		t.Fatalf("DecodeConnection = %+v, %v", back, err)
	}
	if back.Account() != "lattis://lattis.acme.fr" || KindLabel(KindLattis) != "Serveur Lattis" {
		t.Fatalf("compte = %s", back.Account())
	}
	for _, bad := range []string{"", "http://lattis.acme.fr", "ftp://x"} {
		if _, err := EncodeConnection(Connection{Name: "x", Kind: KindLattis, Config: DBConfig{URL: bad}}); err == nil {
			t.Errorf("adresse %q acceptée", bad)
		}
	}
	if !dbSource(SourceLattis) || !strings.HasPrefix(SourceLabel(Source{Kind: SourceLattis, Connection: "Acme"}), "Serveur Lattis") {
		t.Fatal("source Serveur Lattis")
	}
}

func TestConnectLattis(t *testing.T) {
	old := LattisOpen
	t.Cleanup(func() { LattisOpen = old })
	c := Connection{Name: "Acme", Kind: KindLattis, Config: DBConfig{URL: "lattis.acme.fr"}}
	ctx := context.Background()

	LattisOpen = nil
	if _, _, err := connectLattis(ctx, c); err == nil || !strings.Contains(err.Error(), "désactivés") {
		t.Fatalf("fonction coupée : %v", err)
	}

	var gotServer, gotUsage string
	LattisOpen = func(_ context.Context, server, usage string) (*sql.DB, error) {
		gotServer, gotUsage = server, usage
		return nil, &auth.ErrLoginRequired{Server: server, Name: "Serveur Acme"}
	}
	err := TestConnection(ctx, NewSecrets(false, ""), c, "", false)
	var pe *ErrPassword
	if !errors.As(err, &pe) || pe.Lattis != "https://lattis.acme.fr" || !strings.Contains(pe.Message, "connectez-vous") {
		t.Fatalf("connexion requise : %v", err)
	}
	if gotServer != "https://lattis.acme.fr" || gotUsage != "power_query" {
		t.Fatalf("LattisOpen(%q, %q)", gotServer, gotUsage)
	}
	if n := needPassword(err); n == nil || n.Lattis != "https://lattis.acme.fr" {
		t.Fatalf("PasswordNeed = %+v", n)
	}
}

func TestLattisTokens(t *testing.T) {
	s := NewSecrets(false, "")
	ts := LattisTokens(s.For("classeur-1"))
	if err := ts.Set("lattis://acme/alice", "rt"); err != nil {
		t.Fatal(err)
	}
	// Pas d'accord par classeur : la session est celle du poste.
	if v, ok := LattisTokens(s.For("classeur-2")).Get("lattis://acme/alice"); !ok || v != "rt" {
		t.Fatalf("Get = %q, %v", v, ok)
	}
	ts.Forget("lattis://acme/alice")
	if _, ok := ts.Get("lattis://acme/alice"); ok {
		t.Fatal("jeton gardé")
	}
}
