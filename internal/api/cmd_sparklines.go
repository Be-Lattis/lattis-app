package api

import (
	"encoding/json"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Graphiques sparkline d'une feuille (canvas/sparkline.go).

var pSparkline = Param{Name: "sparkline", Type: Ref, Desc: "groupe de sparklines (identifiant ou emplacement)"}

const sparklineHelp = `config : {kind, table, cols, at, source, location, high, low, first, last, negative, markers, color, negColor, markerColor, highColor, lowColor, firstColor, lastColor, axis, same, min, max, weight}. kind : "line" (courbe), "column" (histogramme) ou "winloss" (positif/négatif). Tiré d'un tableau : table, cols (colonnes de la même ligne, dans l'ordre : {"Jan", "Fév", …}) et at (colonne du tableau où dessiner ; absente : la colonne juste à droite du tableau) ; une ligne du tableau, un graphique, lignes ajoutées comprises. Tiré de cellules libres : source ("B2:M5") et location ("N2:N5"), un graphique par ligne (ou colonne) de la source. high, low, first, last, negative : points mis en valeur ; markers : tous les points d'une courbe ; couleurs "#rrggbb" ; axis : axe à zéro ; same : même échelle pour tout le groupe ; min, max : bornes fixes ; weight : épaisseur de la courbe (pixels, 1,5 par défaut). Un groupe posé sur les cellules d'un autre le remplace.`

func init() {
	register(
		&Command{
			Name:    "sparklines.create",
			Label:   "Insérer des graphiques sparkline",
			Summary: "Pose un groupe de graphiques sparkline (petits graphiques dans les cellules) et renvoie son identifiant.",
			Details: sparklineHelp,
			Params:  []Param{{Name: "config", Type: Object, Desc: "type, source et emplacement des graphiques"}, pSheet},
			Write:   true,
			Example: `sparklines.create({kind = "column", table = "Commandes", cols = {"Montant"}, high = true})
sparklines.create({kind = "column", source = "B2:M5", location = "N2:N5"})`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				var cfg canvas.SparklineConfig
				if err := c.Decode("config", &cfg); err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return d.CreateSparkline(c, s.ID, cfg)
			},
		},
		&Command{
			Name:    "sparklines.set",
			Label:   "Modifier les graphiques sparkline",
			Summary: "Modifie un groupe de graphiques sparkline : seuls les champs donnés changent.",
			Details: sparklineHelp,
			Params:  []Param{pSparkline, {Name: "config", Type: Object, Desc: "champs à changer"}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				id, err := c.object("_lattis_sparklines", "location", "sparkline", "sparkline")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				cfg, err := sparkConfig(c, id)
				if err != nil {
					return nil, err
				}
				if err := c.Decode("config", &cfg); err != nil {
					return nil, err
				}
				return nil, d.SetSparkline(c, id, cfg)
			},
		},
		&Command{
			Name:    "sparklines.delete",
			Label:   "Supprimer les graphiques sparkline",
			Summary: "Supprime un groupe de graphiques sparkline.",
			Params:  []Param{pSparkline},
			Write:   true,
			Run: func(c *Call) (any, error) {
				id, err := c.object("_lattis_sparklines", "location", "sparkline", "sparkline")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.DeleteSparkline(c, id)
			},
		},
		&Command{
			Name:    "sparklines.list",
			Summary: "Liste les groupes de graphiques sparkline d'une feuille, avec leur configuration.",
			Returns: "{ {id, config, error}, ... }",
			Params:  []Param{pSheet},
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				list, err := d.Sparklines(c, s.ID)
				if err != nil {
					return nil, err
				}
				type view struct {
					ID     int64                  `json:"id"`
					Config canvas.SparklineConfig `json:"config"`
					Error  string                 `json:"error,omitempty"`
				}
				out := make([]view, 0, len(list))
				for _, g := range list {
					out = append(out, view{g.ID, g.Config, g.Error})
				}
				return out, nil
			},
		},
	)
}

// sparkConfig : configuration en place d'un groupe (mise à jour partielle).
func sparkConfig(c *Call, id int64) (canvas.SparklineConfig, error) {
	var cfg canvas.SparklineConfig
	d, err := c.Doc()
	if err != nil {
		return cfg, err
	}
	var raw string
	if err := d.DB().Read().QueryRowContext(c, `SELECT config FROM _lattis_sparklines WHERE id = ?`, id).Scan(&raw); err != nil {
		return cfg, nil // groupe introuvable : SetSparkline le dira
	}
	return cfg, json.Unmarshal([]byte(raw), &cfg)
}
