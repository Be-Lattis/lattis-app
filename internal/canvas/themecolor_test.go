package canvas

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Mêmes valeurs que colors.test.ts : l'export calcule comme l'interface.
func TestTintShadeLikeExcel(t *testing.T) {
	for _, c := range []struct {
		hex  string
		pct  int
		want string
	}{
		{"#2f7d55", 0, "#2f7d55"},
		{"#2f7d55", 80, "#ceeddd"},
		{"#2f7d55", 40, "#6cc898"},
		{"#2f7d55", -25, "#235e40"},
		{"#2f7d55", -50, "#183f2b"},
		{"#000000", 50, "#808080"},
		{"#ffffff", -5, "#f2f2f2"},
		{"#ffffff", -50, "#808080"},
	} {
		if got := TintShade(c.hex, c.pct); got != c.want {
			t.Errorf("TintShade(%s, %d) = %s, attendu %s", c.hex, c.pct, got, c.want)
		}
	}
}

func TestThemeColors(t *testing.T) {
	// Sans thème : « Vert et bleu » et ses accents.
	def := ThemeColors(nil)
	if def["accent1"] != "#2f7d55" || def["accent2"] != "#3b6fb6" || def["accent3"] != "#e07b24" || def["dk2"] != "#34495e" || def["lt1"] != "#ffffff" || def["dk1"] != "#000000" {
		t.Fatalf("défaut : %v", def)
	}
	// Thème prédéfini gardé sans accents (classeur d'avant les palettes) : ses accents.
	old := ThemeColors(&Theme{ID: "bleu-orange", Name: "Bleu et orange", Table: "#2563eb", Source: "#ea580c"})
	if old["accent3"] != "#16a34a" || old["accent6"] != "#db2777" || old["lt2"] != "#e2e8f0" {
		t.Fatalf("prédéfini sans accents : %v", old)
	}
	// Thème personnalisé sans accents : déduits des deux bases (comme theme.ts).
	own := ThemeColors(&Theme{Name: "x", Table: "#123456", Source: "#654321"})
	want := map[string]string{"accent3": "#be2873", "accent4": "#28be73", "accent5": "#73be28", "accent6": "#7328be", "dk2": "#53402d", "lt2": "#e9edf2"}
	for k, v := range want {
		if own[k] != v {
			t.Errorf("%s = %s, attendu %s (%v)", k, own[k], v, own)
		}
	}
	// Accents donnés : gardés, les autres déduits.
	part := ThemeColors(&Theme{Name: "x", Table: "#123456", Source: "#654321", Accents: []string{"#FF0000"}, Dark: "#111111"})
	if part["accent3"] != "#ff0000" || part["accent4"] != own["accent4"] || part["dk2"] != "#111111" || part["lt2"] != own["lt2"] {
		t.Fatalf("accents en partie : %v", part)
	}
}

func TestThemeRefs(t *testing.T) {
	for _, c := range []struct {
		v    string
		slot string
		pct  int
		ok   bool
	}{
		{"theme:accent1", "accent1", 0, true},
		{"theme:accent6:+40", "accent6", 40, true},
		{"theme:dk2:-25", "dk2", -25, true},
		{"theme:lt1:-5", "lt1", -5, true},
		{"theme:accent7", "", 0, false},
		{"theme:accent1:40", "", 0, false},
		{"theme:accent1:+101", "", 0, false},
		{"#ff0000", "", 0, false},
	} {
		slot, pct, ok := ParseThemeRef(c.v)
		if slot != c.slot || pct != c.pct || ok != c.ok {
			t.Errorf("ParseThemeRef(%q) = %q %d %v", c.v, slot, pct, ok)
		}
	}
	colors := ThemeColors(&Theme{Name: "Bleu et orange", Table: "#2563eb", Source: "#ea580c"})
	if got := ResolveColor("theme:accent1", colors); got != "#2563eb" {
		t.Errorf("accent1 = %s", got)
	}
	if got := ResolveColor("theme:accent3:+40", colors); got != TintShade("#16a34a", 40) {
		t.Errorf("accent3 +40 = %s", got)
	}
	if got := ResolveColor("#C00000", colors); got != "#C00000" {
		t.Errorf("couleur ordinaire changée : %s", got)
	}
	if got := ResolveColor("theme:accent1", nil); got != "#2f7d55" {
		t.Errorf("sans thème : %s", got)
	}
}

func TestThemeAccentsRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.SetTheme(ctx, &Theme{Name: "Mien", Table: "#123456", Source: "#654321", Accents: []string{"#FF0000", " #00ff00 "}, Dark: "#111111", Light: "#EEEEEE"}))
	th, err := d.Theme(ctx)
	must(t, err)
	if th == nil || strings.Join(th.Accents, ",") != "#ff0000,#00ff00" || th.Dark != "#111111" || th.Light != "#eeeeee" {
		t.Fatalf("thème relu : %+v", th)
	}
	for _, bad := range []*Theme{
		{Name: "x", Table: "#123456", Source: "#654321", Accents: []string{"rouge"}},
		{Name: "x", Table: "#123456", Source: "#654321", Accents: []string{"#000000", "#000000", "#000000", "#000000", "#000000"}},
		{Name: "x", Table: "#123456", Source: "#654321", Dark: "#12"},
	} {
		if err := d.SetTheme(ctx, bad); err == nil {
			t.Errorf("thème invalide accepté : %+v", bad)
		}
	}
	// Thème d'avant les palettes (sans accents) : toujours lisible.
	if p := ParseTheme(`{"id":"vert-bleu","name":"Vert et bleu","table":"#2f7d55","source":"#3b6fb6"}`); p == nil || p.Accents != nil {
		t.Fatalf("ancien thème : %+v", p)
	}
}

// Les thèmes prédéfinis sont les mêmes ici et dans l'interface (theme.ts).
func TestBuiltinThemesMatchFrontend(t *testing.T) {
	b, err := os.ReadFile("../../frontend/src/lib/grid/theme.ts")
	if err != nil {
		t.Skip("theme.ts introuvable :", err)
	}
	re := regexp.MustCompile(`\{ id: '([^']+)', name: '([^']+)', table: '(#[0-9a-f]{6})', source: '(#[0-9a-f]{6})', accents: \[([^\]]*)\], dark: '(#[0-9a-f]{6})', light: '(#[0-9a-f]{6})'`)
	found := re.FindAllStringSubmatch(string(b), -1)
	if len(found) != len(builtinThemes) {
		t.Fatalf("%d thèmes prédéfinis dans theme.ts, %d ici", len(found), len(builtinThemes))
	}
	for i, m := range found {
		g := builtinThemes[i]
		acc := strings.ReplaceAll(strings.ReplaceAll(m[5], "'", ""), " ", "")
		if m[1] != g.ID || m[2] != g.Name || m[3] != g.Table || m[4] != g.Source || acc != strings.Join(g.Accents, ",") || m[6] != g.Dark || m[7] != g.Light {
			t.Errorf("thème %s différent : theme.ts %v, Go %+v", g.ID, m[1:], g)
		}
	}
}
