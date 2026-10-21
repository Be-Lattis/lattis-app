package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Loader.Memory (démo publique) : les réglages de l'utilisateur restent en
// mémoire ; son config.json n'est ni lu ni écrit.
func TestMemoryUser(t *testing.T) {
	defer SetCurrent(nil)
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("APPDATA", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("HOME", home)
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Skip(err)
	}
	file := filepath.Join(dir, "Lattis", "config.json")
	os.MkdirAll(filepath.Dir(file), 0o755)
	if err := os.WriteFile(file, []byte(`{"ui": {"language": "de"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &Loader{Memory: true, Environ: func() []string { return nil }}
	if v := l.Load(ctx).String("ui.language", ""); v == "de" {
		t.Fatal("config.json de l'utilisateur lu en mémoire seule")
	}
	c, err := l.SetUser(ctx, "ui.language", "en")
	if err != nil {
		t.Fatal(err)
	}
	if v := c.String("ui.language", ""); v != "en" {
		t.Fatalf("ui.language = %q", v)
	}
	if v := l.Load(ctx).String("ui.language", ""); v != "en" {
		t.Fatalf("rechargé : ui.language = %q", v)
	}
	if b, _ := os.ReadFile(file); string(b) != `{"ui": {"language": "de"}}` {
		t.Fatalf("config.json modifié : %s", b)
	}
	if c, err = l.ResetUser(ctx, []string{"ui.language"}); err != nil {
		t.Fatal(err)
	}
	if v := c.String("ui.language", ""); v == "en" {
		t.Fatalf("réinitialisé : ui.language = %q", v)
	}
	entries, _ := os.ReadDir(filepath.Dir(file))
	if len(entries) != 1 {
		t.Fatalf("fichiers écrits : %v", entries)
	}
}
