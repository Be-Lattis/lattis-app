package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResetUser(t *testing.T) {
	defer SetCurrent(nil)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "Lattis")
	file := filepath.Join(dir, "config.json")
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
	paths := []string{"features.sharedWorkbooks", "features.coEditing", "ui.language", "ui.theme", "limits.undoDepth", "ai.enabled"}

	// Pas de fichier : rien à faire, rien d'écrit.
	if _, err := l.ResetUser(ctx, paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatal("fichier créé par la réinitialisation")
	}

	os.MkdirAll(dir, 0o755)
	os.WriteFile(file, []byte(`{
  "organization": "ACME",
  "features": {"SharedWorkbooks": false, "coEditing": false},
  "ui": {"language": "de", "theme": "mine", "themes": {"mine": {"name": "Le mien", "table": "#112233", "source": "#445566"}}},
  "limits": {"undoDepth": 50},
  "ai": {"enabled": true, "provider": "anthropic", "model": "claude-x"},
  "connections": {"acme": {"kind": "lattis", "config": {"url": "https://lattis.acme.fr"}}}
}`), 0o600)
	other := filepath.Join(dir, "device-id")
	os.WriteFile(other, []byte("abc"), 0o600)
	policy = Tree{"limits": Tree{"undoDepth": float64(30)}}

	c, err := l.ResetUser(ctx, paths)
	if err != nil {
		t.Fatal(err)
	}
	def := Resolve(ctx, Options{}, src("défaut", RankDefault, false, Defaults()))
	for _, p := range []string{"features.sharedWorkbooks", "features.coEditing", "ui.language", "ui.theme", "ai.enabled"} {
		got, _ := c.Lookup(p)
		want, _ := def.Lookup(p)
		if got != want {
			t.Fatalf("%s = %v, défaut %v", p, got, want)
		}
	}
	if Current() != c {
		t.Fatal("configuration courante non mise à jour")
	}
	// La stratégie l'emporte toujours ; la valeur de l'utilisateur sous le
	// verrou n'est pas touchée.
	if v := Limit(c, "limits.undoDepth"); v != 30 || !c.Locked("limits.undoDepth") {
		t.Fatalf("limits.undoDepth = %v", v)
	}
	b, _ := os.ReadFile(file)
	tr, err := parseTree(b)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := lookup(tr, "limits.undoDepth"); v != float64(50) {
		t.Fatalf("valeur sous stratégie retirée : %s", b)
	}
	// Ce qui n'est pas un réglage de la fenêtre reste : thèmes créés,
	// serveurs, fournisseur et modèle de l'IA, autres clés.
	for _, p := range []string{"organization", "ui.themes.mine.name", "connections.acme.config.url", "ai.provider", "ai.model"} {
		if _, ok := lookup(tr, p); !ok {
			t.Fatalf("%s retiré : %s", p, b)
		}
	}
	for _, p := range []string{"features", "ui.language", "ui.theme", "ai.enabled"} {
		if _, ok := lookup(tr, p); ok {
			t.Fatalf("%s gardé : %s", p, b)
		}
	}
	if b, _ := os.ReadFile(other); string(b) != "abc" {
		t.Fatal("autre fichier touché")
	}

	// Clé hors de la liste ou thème : refusés, rien d'écrit.
	before, _ := os.ReadFile(file)
	for _, bad := range []string{"connections", "ui.themes", "ui.themes.mine", "configUrl"} {
		if _, err := l.ResetUser(ctx, []string{"ui.language", bad}); err == nil {
			t.Fatalf("%s accepté", bad)
		}
	}
	if after, _ := os.ReadFile(file); string(after) != string(before) {
		t.Fatal("fichier modifié par un refus")
	}

	// Fichier cassé : pas écrasé.
	os.WriteFile(file, []byte(`{cassé`), 0o600)
	if _, err := l.ResetUser(ctx, paths); err == nil {
		t.Fatal("fichier cassé accepté")
	}
	if b, _ := os.ReadFile(file); string(b) != `{cassé` {
		t.Fatalf("fichier modifié : %s", b)
	}
}

func TestDeletePath(t *testing.T) {
	tr := Tree{"a": Tree{"B": Tree{"c": 1.0}, "d": 2.0}}
	if !deletePath(tr, "a.b.C") || deletePath(tr, "a.b.c") || deletePath(tr, "x.y") || deletePath(tr, "a.d.e") {
		t.Fatal("deletePath")
	}
	if _, ok := tr["a"].(Tree)["B"]; ok {
		t.Fatal("objet vide gardé")
	}
	if !deletePath(tr, "a.d") {
		t.Fatal("a.d")
	}
	if len(tr) != 0 {
		t.Fatalf("reste %v", tr)
	}
}
