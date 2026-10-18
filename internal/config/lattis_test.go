package config

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLattisServers(t *testing.T) {
	c := resolve(t, Options{}, src("défaut", RankDefault, false, Defaults()))
	if SharedWorkbooks(c) || !AllowUserLattisServers(c) || len(LattisServers(c)) != 0 {
		t.Fatal("valeurs par défaut des classeurs partagés")
	}
	policy := Tree{
		"features": Tree{"sharedWorkbooks": true, "allowUserLattisServers": false},
		"connections": Tree{
			"acme": Tree{"kind": "lattis", "name": "Serveur Acme", "config": Tree{"url": "https://lattis.acme.fr"}, "auth": Tree{"provider": "acme-kc"}},
			"erp":  Tree{"kind": "postgres", "config": Tree{"host": "erp"}},
			"vide": Tree{"kind": "lattis"},
		},
	}
	c = resolve(t, Options{}, src("défaut", RankDefault, false, Defaults()), src("stratégie", RankPolicy, true, policy))
	if !SharedWorkbooks(c) || AllowUserLattisServers(c) || !c.Locked("features.allowUserLattisServers") {
		t.Fatal("stratégie des classeurs partagés ignorée")
	}
	ls := LattisServers(c)
	if len(ls) != 1 || ls[0] != (LattisServer{ID: "acme", Name: "Serveur Acme", URL: "https://lattis.acme.fr", Provider: "acme-kc"}) {
		t.Fatalf("LattisServers = %+v", ls)
	}
	c = resolve(t, Options{}, src("user", RankUser, false, Tree{"connections": Tree{"x": Tree{"kind": "oracle"}}}))
	if len(c.Warnings) != 1 {
		t.Fatalf("genre de connexion inconnu accepté : %v", c.Warnings)
	}
}

func TestSharedWorkbooksUserSetting(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "config.json")
	l := &Loader{
		Environ:  func() []string { return nil },
		UserFile: file,
		Local:    func() []Source { return []Source{&FileSource{Path: file, R: RankUser}} },
	}
	if !Editable("features.sharedWorkbooks") || Editable("features.allowUserLattisServers") {
		t.Fatal("réglages modifiables depuis l'interface")
	}
	c, err := l.SetUser(ctx, "features.sharedWorkbooks", true)
	if err != nil || !SharedWorkbooks(c) {
		t.Fatalf("SetUser = %v", err)
	}
}
