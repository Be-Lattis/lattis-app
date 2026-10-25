package api

import (
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// Valeur cible (Données › Analyse de scénarios), pour les scripts.
func init() {
	register(&Command{
		Name:    "cells.goal_seek",
		Label:   "Valeur cible",
		Summary: "Valeur cible : cherche la valeur de la cellule changing (une constante, ou une cellule saisie d'un tableau) pour laquelle la formule de la cellule target vaut value, et la pose (sauf keep = false).",
		Details: "Sécante, puis encadrement et méthode de Brent (300 calculs au plus). found est faux quand aucune valeur ne donne exactement value : value est alors la plus proche trouvée, posée seulement si keep = true est donné.",
		Returns: "{found, value, result}",
		Params: []Param{
			{Name: "target", Type: String, Desc: "cellule à définir, avec une formule (\"B5\")"},
			{Name: "value", Type: Number, Desc: "valeur à atteindre"},
			{Name: "changing", Type: String, Desc: "cellule à modifier (\"B1\")"},
			{Name: "keep", Type: Boolean, Desc: "faux : seulement chercher, sans rien changer", Optional: true},
			pSheet,
		},
		Write: true,
		Example: `cells.set(50, 20, 4)
cells.set(50, 22, "=T50*3+2")
local r = cells.goal_seek("V50", 20, "T50")
print(r.found, r.value, r.result) -- true 6 20`,
		Run: func(c *Call) (any, error) {
			s, err := c.sheet("sheet")
			if err != nil {
				return nil, err
			}
			target, err := c.Str("target")
			if err != nil {
				return nil, err
			}
			changing, err := c.Str("changing")
			if err != nil {
				return nil, err
			}
			goal := raw(c.Raw("value"))
			if f, ok := c.Raw("value").(float64); ok {
				goal = formula.FormatNumber(f)
			}
			d, _ := c.Doc()
			r, err := d.GoalSeek(c, s.ID, target, goal, changing)
			if err != nil {
				return nil, err
			}
			keep := r.Found
			if v, ok := c.Raw("keep").(bool); ok {
				keep = v
			}
			if keep {
				if err := d.SetWhatIfValues(c, s.ID, "goal", []string{r.Cell}, []string{r.Raw}); err != nil {
					return nil, err
				}
			}
			v, _ := strconv.ParseFloat(strings.Replace(r.Raw, ",", ".", 1), 64)
			return map[string]any{"found": r.Found, "value": v, "result": r.Result}, nil
		},
	})
}
