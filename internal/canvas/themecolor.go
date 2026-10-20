package canvas

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Couleurs du thème dans les styles (décision 0017) : une couleur de police,
// de remplissage ou de bordure prise dans les palettes du thème est gardée
// comme référence au thème, « theme:<emplacement>[:<±pourcentage>] »
// (« theme:accent1:+40 » : Accent 1, plus clair 40 % ; « theme:dk2:-25 » :
// Texte 2, plus foncé 25 %), pour que changer le thème du classeur recolore
// ces cellules, comme Excel. Les autres couleurs restent #rrggbb.
// L'interface résout ces références (frontend/src/lib/grid/colors.ts) ;
// l'export les écrit résolues (ResolveColor), avec les mêmes calculs.

// ThemeSlots : emplacements des couleurs du thème, dans l'ordre des palettes.
var ThemeSlots = [...]string{"lt1", "dk1", "lt2", "dk2", "accent1", "accent2", "accent3", "accent4", "accent5", "accent6"}

// builtinThemes : thèmes prédéfinis (comme BUILTIN_THEMES de theme.ts, que
// vérifie TestBuiltinThemesMatchFrontend) ; leurs accents valent pour un
// classeur qui a gardé leur thème sans accents (d'avant les palettes).
var builtinThemes = []Theme{
	{ID: "vert-bleu", Name: "Vert et bleu", Table: "#2f7d55", Source: "#3b6fb6", Accents: []string{"#e07b24", "#7b4fb0", "#c99a1e", "#b83b5e"}, Dark: "#34495e", Light: "#e8eeea"},
	{ID: "bleu-orange", Name: "Bleu et orange", Table: "#2563eb", Source: "#ea580c", Accents: []string{"#16a34a", "#9333ea", "#ca8a04", "#db2777"}, Dark: "#1e293b", Light: "#e2e8f0"},
	{ID: "violet-sarcelle", Name: "Violet et sarcelle", Table: "#7c3aed", Source: "#0d9488", Accents: []string{"#f59e0b", "#e11d48", "#2563eb", "#65a30d"}, Dark: "#3b3355", Light: "#ede9f6"},
	{ID: "sarcelle-violet", Name: "Sarcelle et violet", Table: "#0d9488", Source: "#7c3aed", Accents: []string{"#f59e0b", "#e11d48", "#2563eb", "#65a30d"}, Dark: "#134e4a", Light: "#e6f2f1"},
	{ID: "gris-bleu", Name: "Gris et bleu", Table: "#64748b", Source: "#2563eb", Accents: []string{"#0d9488", "#d97706", "#dc2626", "#7c3aed"}, Dark: "#1e293b", Light: "#e2e8f0"},
	{ID: "orange-bleu", Name: "Orange et bleu", Table: "#c2410c", Source: "#2563eb", Accents: []string{"#15803d", "#a16207", "#7c3aed", "#be123c"}, Dark: "#3b2f2a", Light: "#f3ebe5"},
	{ID: "bordeaux-gris", Name: "Bordeaux et gris", Table: "#9f1239", Source: "#64748b", Accents: []string{"#b45309", "#0f766e", "#1d4ed8", "#6d28d9"}, Dark: "#3f1d2b", Light: "#efe7ea"},
}

// ThemeColors rend les dix couleurs (#rrggbb) d'un thème, par emplacement.
// nil : le thème prédéfini « Vert et bleu » (défaut d'un poste sans
// réglage). Ce que le thème ne donne pas se déduit de ses deux bases, et un
// thème prédéfini gardé sans accents reprend les siens.
func ThemeColors(th *Theme) map[string]string {
	t := builtinThemes[0]
	if th != nil && IsHexColor(th.Table) && IsHexColor(th.Source) {
		t = *th
		if len(t.Accents) == 0 && t.Dark == "" && t.Light == "" {
			for _, b := range builtinThemes {
				if strings.EqualFold(b.Table, t.Table) && strings.EqualFold(b.Source, t.Source) {
					t.Accents, t.Dark, t.Light = b.Accents, b.Dark, b.Light
					break
				}
			}
		}
	}
	table, source := strings.ToLower(t.Table), strings.ToLower(t.Source)
	h1, s1, _ := toHSL(table)
	h2, s2, _ := toHSL(source)
	s := math.Min(0.75, math.Max(0.45, math.Max(s1, s2)))
	turn := func(h, d float64) string { return fromHSL(math.Mod(math.Mod(h+d, 360)+360, 360), s, 0.45) }
	acc := []string{turn(h1, 120), turn(h2, 120), turn(h1, 240), turn(h2, 240)}
	for i, a := range t.Accents {
		if i < 4 && IsHexColor(a) {
			acc[i] = strings.ToLower(a)
		}
	}
	out := map[string]string{
		"lt1": "#ffffff", "dk1": "#000000",
		"lt2": fromHSL(h1, math.Min(s1, 0.25), 0.93), "dk2": fromHSL(h2, math.Min(s2, 0.3), 0.25),
		"accent1": table, "accent2": source,
		"accent3": acc[0], "accent4": acc[1], "accent5": acc[2], "accent6": acc[3],
	}
	if IsHexColor(t.Light) {
		out["lt2"] = strings.ToLower(t.Light)
	}
	if IsHexColor(t.Dark) {
		out["dk2"] = strings.ToLower(t.Dark)
	}
	return out
}

var themeRefRe = regexp.MustCompile(`^theme:(lt1|dk1|lt2|dk2|accent[1-6])(?::([+-]\d{1,3}))?$`)

// ParseThemeRef lit une référence au thème : emplacement et pourcentage
// (positif plus clair, négatif plus foncé). ok faux : pas une référence.
func ParseThemeRef(v string) (slot string, pct int, ok bool) {
	m := themeRefRe.FindStringSubmatch(v)
	if m == nil {
		return "", 0, false
	}
	if m[2] != "" {
		pct, _ = strconv.Atoi(m[2])
	}
	if pct < -100 || pct > 100 {
		return "", 0, false
	}
	return m[1], pct, true
}

// IsThemeRef : couleur prise dans le thème (« theme:accent1:+40 »).
func IsThemeRef(v string) bool {
	_, _, ok := ParseThemeRef(v)
	return ok
}

// ResolveColor rend la couleur #rrggbb d'une couleur de style : une
// référence au thème est résolue avec les couleurs du thème (ThemeColors ;
// nil : celles du thème par défaut), une autre valeur est rendue telle quelle.
func ResolveColor(v string, colors map[string]string) string {
	slot, pct, ok := ParseThemeRef(v)
	if !ok {
		return v
	}
	if colors == nil {
		colors = ThemeColors(nil)
	}
	return TintShade(colors[slot], pct)
}

// TintShade varie une couleur d'un pourcentage, comme Excel (luminosité
// TSL) : plus claire (L + (1 − L)·t) pour t > 0, plus foncée (L·(1 + t))
// pour t < 0. Même calcul que tintShade (colors.ts).
func TintShade(hex string, pct int) string {
	if pct == 0 {
		return strings.ToLower(hex)
	}
	h, s, l := toHSL(hex)
	t := math.Max(-100, math.Min(100, float64(pct))) / 100
	if t > 0 {
		l += (1 - l) * t
	} else {
		l *= 1 + t
	}
	return fromHSL(h, s, l)
}

// toHSL : teinte (0–360), saturation et luminosité (0–1), comme toHsl (theme.ts).
func toHSL(hex string) (h, s, l float64) {
	n, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil || len(hex) != 7 {
		return 0, 0, 0
	}
	r, g, b := float64(n>>16&255)/255, float64(n>>8&255)/255, float64(n&255)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h * 60, s, l
}

// fromHSL : comme fromHsl (theme.ts), arrondi compris.
func fromHSL(h, s, l float64) string {
	k := func(n float64) float64 { return math.Mod(n+h/30, 12) }
	a := s * math.Min(l, 1-l)
	f := func(n float64) float64 {
		return l - a*math.Max(-1, math.Min(k(n)-3, math.Min(9-k(n), 1)))
	}
	c := func(v float64) int { return int(math.Floor(math.Min(255, math.Max(0, v)) + 0.5)) }
	return fmt.Sprintf("#%02x%02x%02x", c(f(0)*255), c(f(8)*255), c(f(4)*255))
}
