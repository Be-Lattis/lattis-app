package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Thème de couleur du classeur (Affichage › Thème, décision 0017) : la
// couleur des tableaux ordinaires et celle des tableaux chargés par une
// source externe. Gardé dans le classeur (_lattis_info, clé « theme ») avec
// ses deux couleurs de base et son nom : il s'affiche pareil sur un poste
// qui ne connaît pas ce thème. Sans thème, l'interface prend le thème par
// défaut des réglages (ui.theme). Les nuances (en-tête, texte, bordure)
// se déduisent des deux couleurs dans l'interface (theme.ts). Les palettes
// de couleur (police, remplissage) prennent dix couleurs au thème, comme
// Excel : les deux bases (Accent 1 et 2), Accent 3 à 6, Texte 2 et
// Arrière-plan 2, facultatives (déduites des bases, voir themecolor.go).

// ThemeKey : clé du thème dans _lattis_info (publiée avec la mise en page
// d'un classeur partagé, voir remote/workbook).
const ThemeKey = "theme"

// Theme est le thème du classeur. ID : identifiant du thème choisi
// (prédéfini, de l'organisation ou de l'utilisateur), vide pour des
// couleurs personnalisées ; Table et Source : couleurs de base #rrggbb ;
// Accents : Accent 3 à 6 (quatre au plus), Dark et Light : Texte 2 et
// Arrière-plan 2 (absents : déduits de Table et Source).
type Theme struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	Table   string   `json:"table"`
	Source  string   `json:"source"`
	Accents []string `json:"accents,omitempty"`
	Dark    string   `json:"dark,omitempty"`
	Light   string   `json:"light,omitempty"`
}

// IsHexColor : couleur #rrggbb.
func IsHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

// Clean vérifie un thème et le met en forme (couleurs en minuscules).
func (th Theme) Clean() (Theme, error) {
	th.ID = strings.TrimSpace(th.ID)
	th.Name = strings.TrimSpace(th.Name)
	th.Table = strings.ToLower(strings.TrimSpace(th.Table))
	th.Source = strings.ToLower(strings.TrimSpace(th.Source))
	if th.Name == "" {
		th.Name = "Personnalisé"
	}
	if len([]rune(th.Name)) > 60 || len(th.ID) > 60 {
		return th, fmt.Errorf("nom de thème trop long")
	}
	if !IsHexColor(th.Table) || !IsHexColor(th.Source) {
		return th, fmt.Errorf("couleur de thème invalide (#rrggbb attendu)")
	}
	if len(th.Accents) > 4 {
		return th, fmt.Errorf("quatre accents au plus (Accent 3 à 6)")
	}
	var acc []string
	for _, a := range th.Accents {
		a = strings.ToLower(strings.TrimSpace(a))
		if !IsHexColor(a) {
			return th, fmt.Errorf("accent de thème invalide (#rrggbb attendu)")
		}
		acc = append(acc, a)
	}
	th.Accents = acc
	th.Dark = strings.ToLower(strings.TrimSpace(th.Dark))
	th.Light = strings.ToLower(strings.TrimSpace(th.Light))
	if th.Dark != "" && !IsHexColor(th.Dark) || th.Light != "" && !IsHexColor(th.Light) {
		return th, fmt.Errorf("Texte 2 ou Arrière-plan 2 du thème invalide (#rrggbb attendu)")
	}
	return th, nil
}

// ParseTheme lit un thème gardé (nil : absent ou illisible).
func ParseTheme(raw string) *Theme {
	if raw == "" {
		return nil
	}
	var th Theme
	if json.Unmarshal([]byte(raw), &th) != nil {
		return nil
	}
	th, err := th.Clean()
	if err != nil {
		return nil
	}
	return &th
}

// Theme rend le thème du classeur (nil : thème par défaut des réglages).
func (m *Model) Theme(ctx context.Context) (*Theme, error) {
	var out *Theme
	err := m.read(ctx, func(t *tx) error {
		raw, _, err := t.infoValue(ThemeKey)
		out = ParseTheme(raw)
		return err
	})
	return out, err
}

// SetTheme change le thème du classeur (nil : retiré, le thème par défaut
// s'applique), en une étape d'annulation.
func (m *Model) SetTheme(ctx context.Context, th *Theme) error {
	now := ""
	if th != nil {
		c, err := th.Clean()
		if err != nil {
			return err
		}
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		now = string(b)
	}
	return m.Do(ctx, "Thème", func(t *tx) error { return t.setInfo(ThemeKey, now) })
}
