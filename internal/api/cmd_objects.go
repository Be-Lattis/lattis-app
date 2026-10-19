package api

import (
	"cmp"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// object résout un objet flottant (tableau croisé, graphique, bouton) par
// identifiant ou par nom (champ JSON key de sa configuration).
func (c *Call) object(table, key, kind, param string) (int64, error) {
	d, err := c.Doc()
	if err != nil {
		return 0, err
	}
	v := c.Raw(param)
	if f, ok := number(v); ok {
		if _, isStr := v.(string); !isStr {
			return int64(f), nil
		}
	}
	name, _ := c.Str(param)
	rs, err := d.DB().Read().QueryContext(c, `SELECT id, config FROM `+table+` ORDER BY id`)
	if err != nil {
		return 0, err
	}
	defer rs.Close()
	var loose int64
	for rs.Next() {
		var id int64
		var cfg string
		if err := rs.Scan(&id, &cfg); err != nil {
			return 0, err
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(cfg), &m); err != nil {
			continue // configuration illisible : pas de nom à comparer (l'identifiant reste valable)
		}
		n, _ := m[key].(string)
		if n == name {
			return id, nil
		}
		if loose == 0 && strings.EqualFold(n, name) {
			loose = id
		}
	}
	if err := rs.Err(); err != nil {
		return 0, err
	}
	if loose != 0 {
		return loose, nil
	}
	if f, ok := number(v); ok {
		return int64(f), nil
	}
	return 0, fmt.Errorf("%s « %s » introuvable", kind, name)
}

var (
	pChart  = Param{Name: "chart", Type: Ref, Desc: "graphique (identifiant ou titre)"}
	pPivot  = Param{Name: "pivot", Type: Ref, Desc: "tableau croisé (identifiant ou nom)"}
	pButton = Param{Name: "button", Type: Ref, Desc: "bouton (identifiant ou libellé)"}
)

const chartHelp = `config : {source, kind, x, series, title, …}. source est un tableau ou un tableau croisé, x la colonne des abscisses, series la liste des colonnes tracées. kind : "bar" (histogramme : barres verticales), "column" (barres horizontales), "line", "area", "pie", "doughnut", "scatter", "combo" (barres et courbes : seriesKinds donne la forme de chaque série, "bar", "line" ou "area"), "bar3d" et "pie3d" (en perspective), "map" (carte OpenStreetMap), "waterfall" (cascade), "treemap" (compartimentage), "radar", "funnel" (entonnoir), "boxplot" (boîte à moustaches), "histogram" (fréquences, bins classes) ou "gauge" (jauge : dernière valeur de la première série, entre yMin et yMax).
Présentation : title ; legend ("bottom", "top", "right" ou "none") ; xTitle, yTitle, y2Title (titres des axes) ; labels (étiquettes de données) ; noGrid (sans quadrillage) ; stacked (empilé : barres, aires) ; yMin, yMax (bornes de l'axe des valeurs) ; colors (couleur de chaque série, "#rrggbb" ; par catégorie pour secteurs, anneau, entonnoir et compartimentage ; hausse, baisse et total pour la cascade) ; secondary (liste de booléens : séries sur l'axe de droite) ; blanks (courbes et aires : cellule vide "gap" en intervalle, par défaut, "zero" ou "span" pour relier les points) ; hidden (lit aussi les lignes masquées par le filtre du tableau) ; filters (graphique tiré d'un tableau : {{field, values}, ...}, ne lit que les lignes dont la colonne field affiche l'une des valeurs, comme le filtre de rapport d'un TCD ; sur la colonne d'un onglet à choix, il l'emporte sur le choix) ; width, height (pixels) et offX, offY (décalage dans la cellule d'ancrage, pixels).
Carte : series = {latitude, longitude, valeur facultative} (mapCoords "pair", par défaut) ou {coordonnées, valeur facultative} (mapCoords "single" : une cellule « 48.85, 2.35 ») ; x le nom des points ; mapMode "points" ou "heat" (densité) ; colorBy, colonne dont chaque valeur prend sa couleur ; mapView {lat, lon, zoom} fixe la vue.`

const pivotHelp = `config : {source, rowFields, cols, values, filters, name, noGrandTotal, subtotals, collapsed, collapseAll, expanded}. source est un tableau, rowFields la liste des champs en lignes, cols le champ en colonnes, values une liste de {field, agg, show} (agg : "sum", "count", "countDistinct", "avg", "min" ou "max" ; show : "", "pctTotal", "pctRow" ou "pctCol"), filters une liste de {field, values}. Avec plusieurs champs en lignes, le tableau s'affiche en arborescence : subtotals place les sous-totaux des groupes ("" sur la ligne du groupe, "bottom" en dessous, "none" aucun) ; collapsed liste les groupes pliés, chacun par ses étiquettes ({"RH"}, {"RH", "Paris"}) ; collapseAll plie tout, sauf les groupes d'expanded.`

func init() {
	register(
		// --- Graphiques ---------------------------------------------------------
		&Command{
			Name:    "charts.create",
			Label:   "Insérer un graphique",
			Summary: "Pose un graphique sur une feuille, coin haut gauche en row, col, et renvoie son identifiant.",
			Details: chartHelp,
			Params:  []Param{pRow, pCol, {Name: "config", Type: Object, Desc: "configuration du graphique"}, pSheet},
			Write:   true,
			Example: `charts.create(2, 8, {source = "Commandes", kind = "bar", x = "Client", series = {"Montant"}, title = "Par client"})`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				var cfg canvas.ChartConfig
				if err := c.Decode("config", &cfg); err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return d.CreateChart(c, s.ID, row, col, cfg)
			},
		},
		&Command{
			Name:    "charts.set",
			Label:   "Modifier le graphique",
			Summary: "Modifie la configuration d'un graphique : seuls les champs donnés changent (replace = true remplace toute la configuration).",
			Details: chartHelp,
			Params:  []Param{pChart, {Name: "config", Type: Object, Desc: "champs à changer"}, {Name: "replace", Type: Boolean, Desc: "remplacer toute la configuration (taille, couleurs, vue de carte… reviennent par défaut)", Optional: true}},
			Write:   true,
			Example: `local g = charts.create(2, 8, {source = "Commandes", kind = "bar", x = "Client", series = {"Montant"}})
charts.set(g, {title = "Ventes par client", labels = true})`,
			Run: func(c *Call) (any, error) {
				id, err := c.object("_lattis_charts", "title", "graphique", "chart")
				if err != nil {
					return nil, err
				}
				replace, err := c.Bool("replace")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				var cfg canvas.ChartConfig
				if !replace {
					// Mise à jour partielle : les champs donnés recouvrent la configuration en place.
					if _, _, cfg, err = d.ChartPlace(c, id); err != nil {
						return nil, err
					}
				}
				if err := c.Decode("config", &cfg); err != nil {
					return nil, err
				}
				return nil, d.SetChart(c, id, cfg)
			},
		},
		&Command{
			Name:    "charts.place",
			Label:   "Déplacer le graphique",
			Summary: "Déplace un graphique (coin haut gauche en row, col) et, si donnés, le redimensionne (pixels, au moins 160 × 120) ; sinon il garde sa taille.",
			Params:  []Param{pChart, pRow, pCol, {Name: "width", Type: Integer, Desc: "largeur en pixels", Optional: true}, {Name: "height", Type: Integer, Desc: "hauteur en pixels", Optional: true}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				id, err := c.object("_lattis_charts", "title", "graphique", "chart")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				r0, c0, cfg, err := d.ChartPlace(c, id)
				if err != nil {
					return nil, err
				}
				w, err := c.Int("width", cmp.Or(cfg.Width, 480))
				if err != nil {
					return nil, err
				}
				h, err := c.Int("height", cmp.Or(cfg.Height, 288))
				if err != nil {
					return nil, err
				}
				// Décalage dans la cellule : gardé si le graphique reste ancré à la même cellule.
				offX, offY := 0, 0
				if r0 == row && c0 == col {
					offX, offY = cfg.OffX, cfg.OffY
				}
				return nil, d.PlaceChart(c, id, row, col, offX, offY, w, h)
			},
		},
		&Command{
			Name:    "charts.delete",
			Label:   "Supprimer le graphique",
			Summary: "Supprime un graphique.",
			Params:  []Param{pChart},
			Write:   true,
			Run: func(c *Call) (any, error) {
				id, err := c.object("_lattis_charts", "title", "graphique", "chart")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.DeleteChart(c, id)
			},
		},

		// --- Tableaux croisés ---------------------------------------------------
		&Command{
			Name:    "pivots.create",
			Label:   "Tableau croisé dynamique",
			Summary: "Crée un tableau croisé dynamique sur une nouvelle feuille et renvoie cette feuille.",
			Details: pivotHelp,
			Returns: "{id, name} (la feuille)",
			Params:  []Param{{Name: "config", Type: Object, Desc: "configuration du tableau croisé"}},
			Write:   true,
			Example: `local f = pivots.create({source = "Commandes", rowFields = {"Client"}, values = {{field = "Montant", agg = "sum"}}})
ui.show_sheet(f.name)`,
			Run: func(c *Call) (any, error) {
				var cfg canvas.PivotConfig
				if err := c.Decode("config", &cfg); err != nil {
					return nil, err
				}
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				id, err := d.CreatePivot(c, cfg)
				if err != nil {
					return nil, err
				}
				list, _ := sheetList(c, d)
				for _, s := range list {
					if s.ID == id {
						return s, nil
					}
				}
				return sheetRef{ID: id}, nil
			},
		},
		&Command{
			Name:    "pivots.set",
			Label:   "Modifier le tableau croisé",
			Summary: "Remplace la configuration d'un tableau croisé.",
			Details: pivotHelp,
			Params:  []Param{pPivot, {Name: "config", Type: Object, Desc: "nouvelle configuration"}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				id, err := c.object("_lattis_pivots", "name", "tableau croisé", "pivot")
				if err != nil {
					return nil, err
				}
				var cfg canvas.PivotConfig
				if err := c.Decode("config", &cfg); err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.SetPivot(c, id, cfg)
			},
		},
		&Command{
			Name:    "pivots.delete",
			Label:   "Supprimer le tableau croisé",
			Summary: "Supprime un tableau croisé (sa feuille reste).",
			Params:  []Param{pPivot},
			Write:   true,
			Run: func(c *Call) (any, error) {
				id, err := c.object("_lattis_pivots", "name", "tableau croisé", "pivot")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.DeletePivot(c, id)
			},
		},

		// --- Boutons d'action ---------------------------------------------------
		&Command{
			Name:    "buttons.create",
			Label:   "Insérer un bouton",
			Summary: "Pose un bouton d'action sur une feuille ; un clic exécute un script de la bibliothèque, ou l'une de ses fonctions. Renvoie l'identifiant du bouton.",
			Details: "script est le nom d'un script enregistré (onglet Script). Avec function, le script est chargé puis la fonction globale de ce nom est appelée : un même script peut servir plusieurs boutons. Tout ce que fait un clic s'annule en une fois.",
			Params: []Param{
				pRow, pCol,
				{Name: "label", Type: String, Desc: "texte du bouton"},
				{Name: "script", Type: String, Desc: "nom du script à exécuter"},
				{Name: "function", Type: String, Desc: "fonction du script à appeler", Optional: true},
				pSheet,
			},
			Write: true,
			Example: `scripts.save("Relances", [==[
function relancer()
  local n = db.value([[SELECT count(*) FROM Commandes WHERE NOT "Payé"]])
  ui.alert(n .. " commande(s) à relancer")
end
]==])
buttons.create(1, 6, "Relancer", "Relances", "relancer")`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				var cfg canvas.ButtonConfig
				cfg.Label, _ = c.Str("label")
				cfg.Script, _ = c.Str("script")
				cfg.Function, _ = c.Str("function")
				d, _ := c.Doc()
				return d.CreateButton(c, s.ID, row, col, cfg)
			},
		},
		&Command{
			Name:    "buttons.set",
			Label:   "Modifier le bouton",
			Summary: "Change le libellé, le script ou la fonction d'un bouton (les paramètres absents ne changent pas).",
			Params: []Param{
				pButton,
				{Name: "label", Type: String, Desc: "texte du bouton", Optional: true},
				{Name: "script", Type: String, Desc: "nom du script", Optional: true},
				{Name: "function", Type: String, Desc: "fonction du script (\"\" : le script seul)", Optional: true},
			},
			Write: true,
			Run: func(c *Call) (any, error) {
				id, err := c.controlID("button")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				b, err := d.Button(c, id)
				if err != nil {
					return nil, err
				}
				cfg := b.Config
				if c.Has("label") {
					cfg.Label, _ = c.Str("label")
				}
				if c.Has("script") {
					cfg.Script, _ = c.Str("script")
				}
				if c.Has("function") {
					cfg.Function, _ = c.Str("function")
				}
				return nil, d.SetButton(c, id, cfg)
			},
		},
		&Command{
			Name:    "buttons.place",
			Label:   "Déplacer le bouton",
			Summary: "Déplace un bouton (coin haut gauche en row, col) et, si donnés, le redimensionne (pixels).",
			Params:  []Param{pButton, pRow, pCol, {Name: "width", Type: Integer, Desc: "largeur en pixels", Optional: true}, {Name: "height", Type: Integer, Desc: "hauteur en pixels", Optional: true}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				id, err := c.controlID("button")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				b, err := d.Button(c, id)
				if err != nil {
					return nil, err
				}
				w, err := c.Int("width", b.Config.Width)
				if err != nil {
					return nil, err
				}
				h, err := c.Int("height", b.Config.Height)
				if err != nil {
					return nil, err
				}
				return nil, d.PlaceButton(c, id, row, col, 0, 0, w, h)
			},
		},
		&Command{
			Name:    "buttons.delete",
			Label:   "Supprimer le bouton",
			Summary: "Supprime un bouton d'action.",
			Params:  []Param{pButton},
			Write:   true,
			Run: func(c *Call) (any, error) {
				id, err := c.controlID("button")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.DeleteButton(c, id)
			},
		},
		&Command{
			Name:    "buttons.list",
			Summary: "Liste les boutons d'action d'une feuille.",
			Returns: "{ {id, name, kind, label, script, function, row, col}, ... }",
			Params:  []Param{pSheet},
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				l, err := d.Layout(c, s.ID)
				if err != nil {
					return nil, err
				}
				return describeSheet(s, l).Buttons, nil
			},
		},
		&Command{
			Name:    "buttons.run",
			Summary: "Exécute l'action d'un bouton, comme un clic : tout ce qu'elle modifie forme une seule étape d'annulation.",
			Returns: "{output, writes, durationMs, error}",
			Params:  []Param{pButton},
			NoLua:   true, // un script ne lance pas d'autre script
			Run: func(c *Call) (any, error) {
				id, err := c.controlID("button")
				if err != nil {
					return nil, err
				}
				return RunButton(c.Host, id)
			},
		},
	)
}
