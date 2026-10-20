package config

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/msg"
)

// Langue de l'interface : auto par défaut, cinq langues, réglable depuis
// Fichier › Paramètres et imposable par la stratégie.
func TestLanguageSetting(t *testing.T) {
	defer SetCurrent(nil)
	ctx := context.Background()
	c := resolve(t, Options{}, src("défaut", RankDefault, false, Defaults()))
	if c.String("ui.language", "") != "auto" {
		t.Fatalf("défaut : %v", c.Explain())
	}
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
	for _, v := range []string{"fr", "en", "de", "es", "zh", "auto"} {
		if c, err := l.SetUser(ctx, "ui.language", v); err != nil || c.String("ui.language", "") != v {
			t.Fatalf("ui.language = %s : %v", v, err)
		}
	}
	for _, v := range []any{"it", "", "en-US", true} {
		if _, err := l.SetUser(ctx, "ui.language", v); err == nil {
			t.Fatalf("ui.language = %v accepté", v)
		}
	}
	if _, err := l.SetUser(ctx, "ui.language", "de"); err != nil {
		t.Fatal(err)
	}
	policy = Tree{"ui": Tree{"language": "en"}}
	_, err := l.SetUser(ctx, "ui.language", "fr")
	var me *msg.Error
	if !errors.As(err, &me) || me.ID != "config.locked" || err.Error() != "ui.language : réglage imposé par la stratégie de votre organisation" {
		t.Fatalf("langue imposée modifiée : %v", err)
	}
	if s := setting(UserSettings(l.Load(ctx)), "ui.language"); !s.Locked || s.Value != "en" {
		t.Fatalf("ui.language imposé = %+v", s)
	}
}
