package config

import (
	"errors"
	"slices"
	"strings"
)

// Thèmes de couleur des tableaux (Affichage › Thème, décision 0017). Le
// thème choisi vit dans le classeur ; la configuration donne le thème par
// défaut (ui.theme, celui des classeurs qui n'en ont pas) et la liste
// proposée : les thèmes prédéfinis (ui.builtinThemes) et ceux de
// ui.themes, un objet par identifiant. Les objets se fusionnant couche par
// couche, l'organisation fournit les siens (fichier machine, configuration
// distante, stratégie) et l'utilisateur ajoute les siens à côté.

// DefaultTheme : identifiant du thème prédéfini « Vert et bleu ».
const DefaultTheme = "vert-bleu"

// UI règle l'apparence.
type UI struct {
	// Theme : identifiant du thème par défaut des classeurs.
	Theme string `json:"theme,omitempty"`
	// BuiltinThemes : les thèmes prédéfinis sont proposés (vrai par
	// défaut) ; faux : seuls ceux de Themes.
	BuiltinThemes *bool `json:"builtinThemes,omitempty"`
	// Themes : thèmes ajoutés, par identifiant.
	Themes map[string]ThemeDef `json:"themes,omitempty"`
}

// ThemeDef est un thème : un nom et deux couleurs de base #rrggbb, celle
// des tableaux ordinaires et celle des tableaux d'une source externe.
type ThemeDef struct {
	Name   string `json:"name"`
	Table  string `json:"table"`
	Source string `json:"source"`
}

// themesPrefix : chemin d'un thème ajouté (ui.themes.<id>).
const themesPrefix = "ui.themes."

// themePath : chemin d'un thème de ui.themes (identifiant sans point).
func themePath(path string) bool {
	if len(path) <= len(themesPrefix) || !strings.EqualFold(path[:len(themesPrefix)], themesPrefix) {
		return false
	}
	return isIdent(path[len(themesPrefix):]) == nil && !strings.Contains(path[len(themesPrefix):], ".")
}

func hexColor(v any) bool {
	s, ok := v.(string)
	if !ok || len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

// isTheme : {"name": "…", "table": "#rrggbb", "source": "#rrggbb"}.
func isTheme(v any) error {
	t, ok := v.(Tree)
	if !ok {
		return errors.New(`thème attendu : {"name": "…", "table": "#rrggbb", "source": "#rrggbb"}`)
	}
	get := func(k string) any {
		if key, ok := findKey(t, k); ok {
			return t[key]
		}
		return nil
	}
	name, _ := get("name").(string)
	if strings.TrimSpace(name) == "" || len([]rune(name)) > 60 {
		return errors.New("nom de thème attendu (60 caractères au plus)")
	}
	if !hexColor(get("table")) || !hexColor(get("source")) {
		return errors.New("couleurs table et source attendues (#rrggbb)")
	}
	return nil
}

// isThemes : un objet de thèmes (chacun vérifié à part, ui.themes.*).
func isThemes(v any) error {
	if _, ok := v.(Tree); !ok {
		return errors.New("objet de thèmes attendu, par identifiant")
	}
	return nil
}

// themeOrigins donne les identifiants des thèmes de ui.themes qui ne
// viennent pas du fichier de l'utilisateur (organisation, Registre…) :
// l'interface ne propose pas de les supprimer.
func themeOrigins(c *Config) []string {
	v, _ := c.Lookup("ui.themes")
	t, _ := v.(Tree)
	var out []string
	for id := range t {
		user := false
		for _, k := range []string{"name", "table", "source"} {
			if o, ok := c.Origin(themesPrefix + id + "." + k); ok && o.Rank == RankUser {
				user = true
			}
		}
		if !user {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}
