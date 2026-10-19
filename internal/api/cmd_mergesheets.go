package api

import (
	"errors"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Regrouper les onglets aux mêmes colonnes
// (docs/conception/regrouper-onglets-semblables.md) : détection et fusion.

// mergeOptions : ce que l'on peut préciser d'un regroupement (MergePlan
// sans ses tableaux).
type mergeOptions struct {
	Category   *canvas.MergeCategory `json:"category"`
	NoCategory bool                  `json:"noCategory"`
	Columns    []canvas.MergeColumn  `json:"columns"`
	Sheet      string                `json:"sheet"`
	Name       string                `json:"name"`
	Old        string                `json:"old"`
}

func init() {
	register(
		&Command{
			Name:    "tables.similar",
			Summary: "Cherche les onglets semblables : des tableaux de feuilles différentes aux mêmes colonnes (un onglet par mois, par année, par ville…), à regrouper en un seul tableau avec tables.merge_sheets. Ne lit que la structure (noms de feuilles, de tableaux et de colonnes).",
			Details: "Chaque groupe donne ses tableaux (dans l'ordre des onglets), la correspondance des colonnes (from : la colonne de chaque tableau, \"\" si absente ; paired : appariée par un nom proche), la catégorie tirée des noms d'onglets (kind : month, month_year, year, quarter, week, date, number, letter, default, city, shape ou other ; column : nom proposé, Mois, Année, Lot… ; type ; values), la clé commune, le nom proposé, l'indice de confiance (score, propose : assez haut pour le proposer de soi-même) et dismissed (« Ne plus proposer » choisi). Le plus grand groupe d'abord.",
			Returns: "{ {id, tables = { {table, sheet, sheetName, rows, key, hidden}, ... }, columns = { {name, type, from, paired, text}, ... }, category = {kind, column, type, values, labels, common}, key, name, score, propose, identical, dismissed, summary, notes}, ... }",
			Example: `for _, g in ipairs(tables.similar()) do
  print(#g.tables .. " onglets semblables, catégorie " .. g.category.column)
end`,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				return d.SimilarSheets(c)
			},
		},
		&Command{
			Name:  "tables.merge_sheets",
			Label: "Regrouper des onglets",
			Summary: "Regroupe des tableaux aux mêmes colonnes, chacun sur son onglet (un par mois, par année, par ville…), en un seul tableau sur une nouvelle feuille, avec en tête une colonne de catégorie tirée des noms d'onglets (Mois, Année, Ville…) et sa liste déroulante. Une seule étape d'annulation.",
			Details: "tables : les tableaux, dans l'ordre (absent : le plus grand groupe trouvé par tables.similar). options : {name (nom du tableau, proposé sinon), sheet (nom de la feuille), category = {name, type = \"date\" | \"integer\" | \"text\", values = {une valeur par tableau}} (déduite des noms d'onglets sinon), noCategory = true (pas de colonne de catégorie), columns (correspondance, comme celle de tables.similar), old = \"delete\" | \"hide\"}. " +
				"Colonnes réunies (cases vides là où une colonne manquait), types élargis, mise en forme et saisies invalides gardées. La clé est gardée si aucune valeur ne se répète d'un onglet à l'autre, sinon le tableau commun n'a pas de clé (keyNote donne un exemple). Les formules, tableaux croisés et graphiques qui citaient les anciens tableaux ne sont pas réécrits : ils sont listés dans pending, et les anciens onglets sont alors gardés masqués (tableaux renommés Janvier_ancien) plutôt que supprimés. Refusé pour un classeur partagé sur un serveur, une feuille protégée, un tableau filtré ou chargé par une requête.",
			Returns: "{sheet, sheetName, table, rows, old, keyKept, keyNote, pending = { {kind, sheetName, where, formula}, ... }, notes, notice}",
			Params: []Param{
				{Name: "tables", Type: Array, Items: String, Desc: "tableaux à regrouper, dans l'ordre (absent : le plus grand groupe d'onglets semblables)", Optional: true},
				{Name: "options", Type: Object, Desc: "{name, sheet, category = {name, type, values}, noCategory, columns, old}", Optional: true},
			},
			Write:   true,
			Example: `local noms = {}
for i, mois in ipairs({"Janvier", "Février", "Mars"}) do
  local f = sheets.add(mois)
  cells.paste(1, 1, {{"Client", "Montant"}, {"Durand", i * 10}}, nil, f.name)
  table.insert(noms, sheets.get(f.name).tables[1].name)
end
local r = tables.merge_sheets(noms, {name = "Ventes", category = {name = "Mois"}})
print(r.notice)`,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				var o mergeOptions
				if c.Has("options") {
					if err := c.Decode("options", &o); err != nil {
						return nil, err
					}
				}
				plan := canvas.MergePlan{Category: o.Category, NoCategory: o.NoCategory, Columns: o.Columns, Sheet: o.Sheet, Name: o.Name, Old: o.Old}
				if c.Has("tables") {
					list, ok := c.Raw("tables").([]any)
					if !ok {
						list = []any{c.Raw("tables")}
					}
					for _, v := range list {
						t, err := findTable(c, c, raw(v))
						if err != nil {
							return nil, err
						}
						plan.Tables = append(plan.Tables, t.Name)
					}
				} else {
					gs, err := d.SimilarSheets(c)
					if err != nil {
						return nil, err
					}
					if len(gs) == 0 {
						return nil, errors.New("aucun groupe d'onglets semblables : précisez les tableaux à regrouper")
					}
					for _, t := range gs[0].Tables {
						plan.Tables = append(plan.Tables, t.Table)
					}
					if len(plan.Columns) == 0 {
						plan.Columns = gs[0].Columns
					}
				}
				return d.MergeSheets(c, plan)
			},
		},
	)
}
