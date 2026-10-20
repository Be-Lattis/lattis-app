package config

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

func setting(st []UserSetting, path string) UserSetting {
	for _, s := range st {
		if s.Path == path {
			return s
		}
	}
	return UserSetting{}
}

// Thèmes : défaut, liste de l'organisation, thèmes de l'utilisateur à côté,
// thème par défaut imposé.
func TestThemeSettings(t *testing.T) {
	defer SetCurrent(nil)
	ctx := context.Background()
	c := resolve(t, Options{}, src("défaut", RankDefault, false, Defaults()))
	if c.String("ui.theme", "") != DefaultTheme || !c.Bool("ui.builtinThemes", false) {
		t.Fatalf("défauts : %v", c.Explain())
	}
	file := filepath.Join(t.TempDir(), "Lattis", "config.json")
	machine := Tree{"ui": Tree{"builtinThemes": false, "theme": "acme", "themes": Tree{
		"acme":  Tree{"name": "ACME", "table": "#0b5394", "source": "#e69138"},
		"casse": Tree{"name": "Cassé", "table": "bleu", "source": "#000000"},
	}}}
	var policy Tree
	l := &Loader{
		Environ:  func() []string { return nil },
		UserFile: file,
		Local: func() []Source {
			out := []Source{&FileSource{Path: file, R: RankUser}, src("machine", RankMachine, true, machine)}
			if policy != nil {
				out = append(out, src("stratégie", RankPolicy, true, policy))
			}
			return out
		},
	}
	c = l.Load(ctx)
	if c.Bool("ui.builtinThemes", true) || c.String("ui.theme", "") != "acme" {
		t.Fatalf("organisation : %v", c.Explain())
	}
	if _, ok := c.Lookup("ui.themes.casse"); ok {
		t.Fatal("thème invalide gardé")
	}
	// Un thème de l'utilisateur s'ajoute à ceux de l'organisation.
	c, err := l.SetUser(ctx, "ui.themes.perso", map[string]any{"name": "Perso", "table": "#7c3aed", "source": "#0d9488"})
	if err != nil {
		t.Fatal(err)
	}
	st := setting(UserSettings(c), "ui.themes")
	m, _ := st.Value.(Tree)
	if len(m) != 2 || !slices.Equal(st.Managed, []string{"acme"}) || st.Locked {
		t.Fatalf("ui.themes = %+v", st)
	}
	if !setting(UserSettings(c), "ui.builtinThemes").ReadOnly {
		t.Fatal("ui.builtinThemes écrit par l'interface")
	}
	for _, bad := range []any{map[string]any{"name": "", "table": "#000000", "source": "#000000"}, map[string]any{"name": "x", "table": "#00000", "source": "#000000"}, "rouge"} {
		if _, err := l.SetUser(ctx, "ui.themes.mauvais", bad); err == nil {
			t.Fatalf("thème invalide accepté : %v", bad)
		}
	}
	if _, err := l.SetUser(ctx, "ui.themes", Tree{}); err == nil {
		t.Fatal("ui.themes écrit d'un bloc")
	}
	if _, err := l.SetUser(ctx, "ui.themes.a.b", nil); err == nil {
		t.Fatal("chemin de thème invalide accepté")
	}
	// Retirer : celui de l'utilisateur part, celui de l'organisation reste.
	if c, err = l.SetUser(ctx, "ui.themes.perso", nil); err != nil {
		t.Fatal(err)
	}
	if c, err = l.SetUser(ctx, "ui.themes.acme", nil); err != nil {
		t.Fatal(err)
	}
	if m, _ := setting(UserSettings(c), "ui.themes").Value.(Tree); len(m) != 1 || m["acme"] == nil {
		t.Fatalf("après retrait : %v", m)
	}
	// Thème par défaut choisi par l'utilisateur, puis imposé.
	if c, err = l.SetUser(ctx, "ui.theme", "violet-sarcelle"); err != nil || c.String("ui.theme", "") != "violet-sarcelle" {
		t.Fatalf("ui.theme : %v", err)
	}
	policy = Tree{"ui": Tree{"theme": "acme"}}
	if _, err := l.SetUser(ctx, "ui.theme", "vert-bleu"); err == nil {
		t.Fatal("thème imposé modifié")
	}
	if s := setting(UserSettings(l.Load(ctx)), "ui.theme"); !s.Locked || s.Value != "acme" {
		t.Fatalf("ui.theme imposé = %+v", s)
	}
}

// Les autres couleurs des palettes (Accent 3 à 6, Texte 2, Arrière-plan 2)
// sont facultatives, mais valides quand elles sont données.
func TestThemeAccentsSetting(t *testing.T) {
	base := func(extra Tree) Tree {
		m := Tree{"name": "ACME", "table": "#0b5394", "source": "#e69138"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	for _, ok := range []Tree{base(nil), base(Tree{"accents": []any{"#ff0000", "#00B050"}, "dark": "#1e293b", "light": "#e2e8f0"})} {
		if err := isTheme(ok); err != nil {
			t.Errorf("thème refusé : %v (%v)", ok, err)
		}
	}
	for _, bad := range []Tree{
		base(Tree{"accents": []any{"rouge"}}),
		base(Tree{"accents": "#ff0000"}),
		base(Tree{"accents": []any{"#000000", "#000000", "#000000", "#000000", "#000000"}}),
		base(Tree{"dark": "#12"}),
		base(Tree{"light": 3}),
	} {
		if err := isTheme(bad); err == nil {
			t.Errorf("thème invalide accepté : %v", bad)
		}
	}
}
