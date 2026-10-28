package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Réglages imposés par l'organisation (stratégie) ou posés plus haut que
// l'utilisateur (LATTIS_*, --set, Registre) : signalés à l'interface,
// refusés à l'écriture, gardés par « Réinitialiser ».
func TestImposedSettings(t *testing.T) {
	defer SetCurrent(nil)
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "Lattis", "config.json")
	policy := Tree{
		"features": Tree{"whatIf": true},
		"ai":       Tree{"enabled": true, "provider": "openai", "model": "gpt-x"},
		"limits":   Tree{"scriptSeconds": float64(5)},
		"updates":  Tree{"check": false},
	}
	env := Tree{"features": Tree{"sparklines": true}}
	l := &Loader{
		Environ:  func() []string { return nil },
		UserFile: file,
		Local: func() []Source {
			return []Source{&FileSource{Path: file, R: RankUser}, src("variables LATTIS_*", RankEnv, false, env), src("stratégie", RankPolicy, true, policy)}
		},
	}
	os.MkdirAll(filepath.Dir(file), 0o755)
	os.WriteFile(file, []byte(`{"features":{"sparklines":false,"coEditing":true},"limits":{"scriptSeconds":60}}`), 0o600)

	locked := []string{"features.whatIf", "ai.enabled", "ai.provider", "ai.model", "limits.scriptSeconds", "updates.check"}
	state := map[string]UserSetting{}
	for _, s := range UserSettings(l.Load(ctx)) {
		state[s.Path] = s
	}
	for _, p := range locked {
		if s := state[p]; !s.Locked || s.Fixed {
			t.Fatalf("%s : %+v, attendu verrouillé", p, s)
		}
	}
	if s := state["features.sparklines"]; s.Locked || !s.Fixed || s.Source != "variables LATTIS_*" {
		t.Fatalf("features.sparklines : %+v, attendu fixé par l'environnement", s)
	}
	for _, p := range []string{"features.coEditing", "ai.dataSharing", "limits.undoDepth"} {
		if s := state[p]; s.Locked || s.Fixed {
			t.Fatalf("%s : %+v, attendu libre", p, s)
		}
	}

	// Écriture refusée, fichier intact.
	before, _ := os.ReadFile(file)
	for p, v := range map[string]any{"features.whatIf": false, "ai.enabled": false, "ai.provider": "anthropic", "ai.model": "autre", "limits.scriptSeconds": float64(10), "updates.check": true, "features.sparklines": false} {
		if _, err := l.SetUser(ctx, p, v); err == nil {
			t.Fatalf("%s modifié malgré l'organisation", p)
		}
	}
	if after, _ := os.ReadFile(file); string(after) != string(before) {
		t.Fatalf("fichier modifié par un refus : %s", after)
	}
	if _, err := l.SetUser(ctx, "features.coEditing", false); err != nil {
		t.Fatal(err)
	}

	// Réinitialiser : les valeurs imposées restent, celles de l'utilisateur
	// sous elles aussi ; les autres reviennent au défaut.
	c, err := l.ResetUser(ctx, []string{"features.whatIf", "features.sparklines", "features.coEditing", "limits.scriptSeconds", "updates.check", "ai.enabled"})
	if err != nil {
		t.Fatal(err)
	}
	if !c.Bool("features.whatIf", false) || !c.Bool("features.sparklines", false) || c.Bool("updates.check", true) || Limit(c, "limits.scriptSeconds") != 5 {
		t.Fatalf("valeurs après réinitialisation : %v", c.Explain())
	}
	b, _ := os.ReadFile(file)
	tr, err := parseTree(b)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := lookup(tr, "features.sparklines"); v != false {
		t.Fatalf("valeur sous LATTIS_* retirée : %s", b)
	}
	if v, _ := lookup(tr, "limits.scriptSeconds"); v != float64(60) {
		t.Fatalf("valeur sous stratégie retirée : %s", b)
	}
	if _, ok := lookup(tr, "features.coEditing"); ok {
		t.Fatalf("features.coEditing gardé : %s", b)
	}
}
