package config

import (
	"context"
	"path/filepath"
	"testing"
)

// Limites réglables (audit 6, M14) : valeurs par défaut, bornes, réglage
// de l'utilisateur, stratégie et environnement.
func TestLimits(t *testing.T) {
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
	c := l.Load(ctx)
	for path, want := range map[string]int{"limits.undoDepth": 200, "limits.scriptSeconds": 30, "limits.sqlReadSeconds": 30, "limits.sqlWriteSeconds": 120} {
		if got := Limit(c, path); got != want {
			t.Errorf("%s = %d, attendu %d", path, got, want)
		}
	}
	if c, err := l.SetUser(ctx, "limits.undoDepth", float64(500)); err != nil || Limit(c, "limits.undoDepth") != 500 {
		t.Fatalf("profondeur d'annulation : %v", err)
	}
	for _, bad := range []any{float64(5), float64(20000), 12.5, "beaucoup"} {
		if _, err := l.SetUser(ctx, "limits.undoDepth", bad); err == nil {
			t.Errorf("valeur %v acceptée", bad)
		}
	}
	// Un texte qui est un nombre (Registre REG_SZ, --set) est accepté.
	if c, err := l.SetUser(ctx, "limits.scriptSeconds", "90"); err != nil || Limit(c, "limits.scriptSeconds") != 90 {
		t.Fatalf("durée des scripts en texte : %v", err)
	}
	// Variable d'environnement (la clé arrive en minuscules).
	ce := resolve(t, Options{}, src("défaut", RankDefault, false, Defaults()),
		&EnvSource{Environ: func() []string { return []string{"LATTIS_LIMITS__SQLREADSECONDS=300"} }})
	if got := Limit(ce, "limits.sqlReadSeconds"); got != 300 {
		t.Fatalf("environnement : %d", got)
	}
	// Stratégie qui verrouille.
	policy = Tree{"limits": Tree{"scriptSeconds": float64(10)}}
	c = l.Load(ctx)
	if Limit(c, "limits.scriptSeconds") != 10 || !c.Locked("limits.scriptSeconds") {
		t.Fatal("stratégie non appliquée")
	}
	if _, err := l.SetUser(ctx, "limits.scriptSeconds", float64(60)); err == nil {
		t.Fatal("réglage imposé modifié")
	}
	// Une valeur hors bornes venue d'ailleurs est ignorée (défaut).
	policy = Tree{"limits": Tree{"sqlWriteSeconds": float64(999999)}}
	if got := Limit(l.Load(ctx), "limits.sqlWriteSeconds"); got != 120 {
		t.Fatalf("valeur hors bornes : %d", got)
	}
}
