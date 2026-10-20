package api

import (
	"github.com/lalamefine/lattis/internal/msg"
)

// Onglet à choix (canvas/choice.go) : une feuille porte une catégorie
// (« Année ») et une valeur choisie sur son onglet (« Ventes · 2025 ▾ »),
// qui filtre ses tableaux qui ont cette colonne.

// SheetChoiceView décrit l'onglet à choix d'une feuille.
type SheetChoiceView struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Table  string   `json:"table,omitempty"`
	Value  string   `json:"value"`
	All    bool     `json:"all"`
	Values []string `json:"values"`
	// Formula : catégorie calculée par une formule (sheets.choose_by_formula).
	Formula string `json:"formula,omitempty"`
}

func init() {
	register(
		&Command{
			Name:    "sheets.choice",
			Summary: "Décrit l'onglet à choix d'une feuille : catégorie (colonne ou formule), valeur choisie et valeurs proposées ; nil si la feuille n'en a pas.",
			Details: "Le choix filtre les tableaux de la feuille qui ont la colonne de catégorie, ou les colonnes de sa formule (formula, catégorie calculée) ; all : « (Toutes) », aucun filtre. Le SQL et les scripts voient toujours toutes les lignes.",
			Returns: "{name, type, table, formula, value, all, values} ou nil",
			Params:  []Param{pSheet},
			Example: `local c = sheets.choice("Feuille1")
if c then print(c.name .. " : " .. c.value) end`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				ch, err := d.SheetChoice(c, s.ID)
				if err != nil || ch == nil {
					return nil, err
				}
				vals, err := d.SheetChoiceValues(c, s.ID)
				if err != nil {
					return nil, err
				}
				out := SheetChoiceView{Name: ch.Name, Type: string(ch.Type), Table: ch.Table, Value: ch.Value, All: ch.All, Values: []string{}, Formula: ch.Formula}
				for _, v := range vals {
					out.Values = append(out.Values, v.Text)
				}
				return out, nil
			},
		},
		&Command{
			Name:    "sheets.set_choice",
			Label:   "Choix de l'onglet",
			Summary: "Règle l'onglet à choix d'une feuille sur une valeur (l'année, le mois, la ville affichés) ; sans valeur, sur « (Toutes) ».",
			Details: "C'est de la navigation, comme changer d'onglet : pas d'étape d'annulation. La feuille doit déjà avoir un onglet à choix (sheets.choose_by).",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "feuille à choix"}, {Name: "value", Type: Value, Desc: "valeur à afficher ; absente : toutes", Optional: true}},
			Write:   true,
			Example: `sheets.choose_by("Commandes", "Client")
sheets.set_choice("Feuille1", "Durand")`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				if !c.Has("value") {
					return nil, d.ChooseSheetValue(c, s.ID, "", true)
				}
				return nil, d.ChooseSheetValue(c, s.ID, raw(c.Raw("value")), false)
			},
		},
		&Command{
			Name:    "sheets.choose_by",
			Label:   "Choisir par",
			Summary: "Fait d'une feuille un onglet à choix par une colonne d'un de ses tableaux (« Choisir par… », « Filtrer l'onglet par cette colonne ») : la flèche de l'onglet choisit la valeur affichée.",
			Details: "Tous les tableaux de la feuille qui ont une colonne de ce nom suivent le choix, ainsi que ses tableaux croisés dont le tableau source a cette colonne. Avec sheet, l'onglet à choix est posé sur cette feuille-là : c'est « Filtrer l'onglet par ce champ » d'un tableau croisé posé ailleurs que son tableau source ; le TCD suit le choix, le tableau source (sur sa feuille) n'est pas filtré. Sans valeur, l'onglet montre toutes les valeurs.",
			Params: []Param{
				{Name: "table", Type: Ref, Desc: "tableau de la colonne (pour un tableau croisé : son tableau source)"},
				{Name: "column", Type: String, Desc: "colonne de catégorie (Année, Mois, Ville…)"},
				{Name: "value", Type: Value, Desc: "valeur affichée d'abord ; absente : toutes", Optional: true},
				{Name: "sheet", Type: Ref, Desc: "feuille de l'onglet à choix (celle d'un tableau croisé) ; absente : celle du tableau", Optional: true},
			},
			Write: true,
			Example: `sheets.choose_by("Commandes", "Client", "Durand")
sheets.add("Synthèse")
sheets.choose_by{table = "Commandes", column = "Client", sheet = "Synthèse"}`,
			Run: func(c *Call) (any, error) {
				tb, err := c.table("table")
				if err != nil {
					return nil, err
				}
				col, err := c.Str("column")
				if err != nil {
					return nil, err
				}
				sheet := tb.Sheet.ID
				if c.Has("sheet") {
					s, err := c.sheet("sheet")
					if err != nil {
						return nil, err
					}
					sheet = s.ID
				}
				d, _ := c.Doc()
				return nil, d.SetSheetChoice(c, sheet, tb.Name, col, raw(c.Raw("value")))
			},
		},
		&Command{
			Name:    "sheets.choose_by_formula",
			Label:   "Choisir par une formule",
			Summary: "Fait d'une feuille un onglet à choix par une formule sur les colonnes de chaque ligne (« Choisir par › Une formule… ») : =ANNEE([Date]), =TEXTE([Date];\"mmmm\"), =SI([Montant]>1000;\"Gros\";\"Petit\").",
			Details: "Les tableaux de la feuille qui ont toutes les colonnes citées par la formule suivent le choix (ainsi que ses tableaux croisés dont le tableau source les a) : seules les lignes dont la formule donne la valeur choisie restent affichées. Pas de colonne à remplir : une ligne ajoutée ne reçoit rien d'office, elle reste affichée jusqu'au prochain changement de valeur. « Nouvelle valeur » (sheets.new_choice) ne s'applique pas. Une étape d'annulation.",
			Params: []Param{
				{Name: "sheet", Type: Ref, Desc: "feuille de l'onglet à choix"},
				{Name: "formula", Type: String, Desc: "formule en français, colonnes entre crochets ([Date])"},
				{Name: "name", Type: String, Desc: "nom de la catégorie (Année, Mois, Taille…)"},
				{Name: "value", Type: Value, Desc: "valeur affichée d'abord ; absente : toutes", Optional: true},
			},
			Write: true,
			Example: `sheets.choose_by_formula("Feuille1", "=SI([Montant]>1000;\"Gros\";\"Petit\")", "Taille", "Gros")
print(sheets.choice("Feuille1").value)`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				f, err := c.Str("formula")
				if err != nil {
					return nil, err
				}
				name, err := c.Str("name")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.SetSheetChoiceFormula(c, s.ID, name, f, raw(c.Raw("value")))
			},
		},
		&Command{
			Name:    "sheets.new_choice",
			Label:   "Nouvelle valeur de l'onglet",
			Summary: "Ajoute une valeur à l'onglet à choix (« Nouvelle année… ») et l'affiche : la feuille paraît vide, prête à être remplie.",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "feuille à choix"}, {Name: "value", Type: Value, Desc: "nouvelle valeur (2026, Décembre…)"}},
			Write:   true,
			Example: `sheets.choose_by("Commandes", "Client")
sheets.new_choice("Feuille1", "Petit")`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				if !c.Has("value") {
					return nil, msg.New("api.valueExpected", nil)
				}
				d, _ := c.Doc()
				return nil, d.NewSheetChoiceValue(c, s.ID, raw(c.Raw("value")))
			},
		},
		&Command{
			Name:    "tables.follow_choice",
			Label:   "Suivre l'onglet à choix",
			Summary: "Coche ou décoche « Suivre l'année » (le mois, la ville…) sur un tableau d'une feuille à onglet à choix : un tableau qui suit ne montre que les lignes de la valeur choisie, et une ligne ajoutée reçoit cette valeur.",
			Details: "follow = true (par défaut) : ajoute la colonne de catégorie en dernier (masquée tant qu'une valeur est choisie), remplie avec la valeur affichée. follow = false : retire la colonne si elle n'a qu'une valeur (rien ne se perd) ; sinon la garde, et le tableau ne suit plus le choix (réglage gardé avec le tableau). La colonne source de l'onglet à choix ne se décoche pas. Une étape d'annulation. Rend le message de la barre d'état.",
			Returns: "message",
			Params:  []Param{{Name: "table", Type: Ref, Desc: "tableau de la feuille à choix"}, {Name: "follow", Type: Boolean, Desc: "true : suivre ; false : ne plus suivre", Optional: true}},
			Write:   true,
			Example: `sheets.choose_by("Commandes", "Client", "Durand")
cells.paste(10, 1, {{"Produit", "Prix"}, {"Pomme", 2}, {"Poire", 3}})
local t = sheets.get().tables
print(tables.follow_choice(t[#t].name))`,
			Run: func(c *Call) (any, error) {
				tb, err := c.table("table")
				if err != nil {
					return nil, err
				}
				follow := true
				if c.Has("follow") {
					if follow, err = c.Bool("follow"); err != nil {
						return nil, err
					}
				}
				d, _ := c.Doc()
				return d.SetTableFollow(c, tb.Name, follow)
			},
		},
		&Command{
			Name:    "sheets.clear_choice",
			Label:   "Ne plus choisir",
			Summary: "Retire l'onglet à choix d'une feuille : ses tableaux montrent de nouveau toutes leurs lignes.",
			Params:  []Param{pSheet},
			Write:   true,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.ClearSheetChoice(c, s.ID)
			},
		},
	)
}
