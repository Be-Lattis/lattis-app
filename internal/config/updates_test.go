package config

import (
	"context"
	"path/filepath"
	"testing"
)

// Recherche des nouvelles versions (updates.check) : activée par défaut,
// réglable par l'utilisateur, coupée et verrouillée par une stratégie ou
// une variable d'environnement.
func TestUpdatesCheck(t *testing.T) {
	defer SetCurrent(nil)
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "Lattis", "config.json")
	var policy Tree
	l := &Loader{
		Environ:  func() []string { return nil },
		UserFile: file,
		Local: func() []Source {
			out := []Source{&FileSource{Path: file, R: RankUser}}
			if policy != nil {
				out = append(out, src("stratégie", RankPolicy, true, policy))
			}
			return out
		},
	}
	if !UpdateCheck(l.Load(ctx)) {
		t.Fatal("activée par défaut")
	}
	c, err := l.SetUser(ctx, "updates.check", false)
	if err != nil || UpdateCheck(c) {
		t.Fatalf("coupée par l'utilisateur : %v", err)
	}
	if _, err := l.SetUser(ctx, "updates.check", "peut-être"); err == nil {
		t.Error("valeur non booléenne acceptée")
	}
	// Variable d'environnement (DWORD ou texte : 0/1, true/false).
	ce := resolve(t, Options{}, src("défaut", RankDefault, false, Defaults()),
		&EnvSource{Environ: func() []string { return []string{"LATTIS_UPDATES__CHECK=0"} }})
	if UpdateCheck(ce) {
		t.Error("LATTIS_UPDATES__CHECK=0 sans effet")
	}
	// Stratégie : imposée et verrouillée.
	if _, err := l.SetUser(ctx, "updates.check", true); err != nil {
		t.Fatal(err)
	}
	policy = Tree{"updates": Tree{"check": false}}
	c = l.Load(ctx)
	if UpdateCheck(c) || !c.Locked("updates.check") {
		t.Fatal("stratégie non appliquée")
	}
	if _, err := l.SetUser(ctx, "updates.check", true); err == nil {
		t.Fatal("réglage imposé modifié")
	}
	for _, s := range UserSettings(c) {
		if s.Path == "updates.check" && !s.Locked {
			t.Error("réglage non montré verrouillé")
		}
	}
}
