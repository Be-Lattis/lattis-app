package api

import (
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
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
	// Lot 4 : source de la catégorie, autres familles de tableaux des
	// onglets, cellules libres non reprises.
	CategoryFrom string               `json:"categoryFrom"`
	Families     []canvas.MergeFamily `json:"families"`
	NoCells      bool                 `json:"noCells"`
}

func init() {
	register(
		&Command{
			Name:    "tables.similar",
			Summary: "Cherche les onglets semblables : des tableaux de feuilles différentes aux mêmes colonnes (un onglet par mois, par année, par ville…), à regrouper en un seul tableau avec tables.merge_sheets. Ne lit que la structure (noms de feuilles, de tableaux et de colonnes).",
			Details: "Chaque groupe donne ses tableaux (dans l'ordre des onglets), la correspondance des colonnes (from : la colonne de chaque tableau, \"\" si absente ; paired : appariée par un nom proche), la catégorie tirée des noms d'onglets (kind : month, month_year, year, quarter, week, date, number, letter, default, city, shape ou other ; column : nom proposé, Mois, Année, Lot… ; type ; values), la clé commune, le nom proposé, l'indice de confiance (score, propose : assez haut pour le proposer de soi-même) et dismissed (« Ne plus proposer » choisi). Le plus grand groupe d'abord. " +
				"On regroupe des feuilles entières : sheets (les onglets, copied : copié d'un autre onglet du groupe ; copies : leur nombre) ; families : les autres tableaux de ces onglets, appariés famille par famille, chacun avec ses tableaux, ses colonnes, same (mêmes données partout, une liste recopiée) et le mode proposé (merge : un tableau commun qui suit la catégorie ; once : un seul exemplaire ; leave : reste sur son onglet, s'il n'est que sur un onglet). sources : d'où peut venir la catégorie (sheet : noms d'onglets ; title : titre au-dessus de chaque tableau ; cell:A1 : une cellule libre qui change d'un onglet à l'autre), categoryFrom : celle proposée (décrite par category). stacked : tableaux semblables empilés sur une même feuille sous des titres. dataColumn : une colonne date porte déjà la catégorie (ses valeurs tombent dans le mois de chaque onglet) : la colonne de catégorie est facultative.",
			Returns: "{ {id, tables = { {table, sheet, sheetName, rows, key, hidden}, ... }, columns = { {name, type, from, paired, text}, ... }, category = {kind, column, type, values, labels, common}, key, name, score, propose, identical, dismissed, summary, notes, sheets = { {sheet, name, hidden, copied}, ... }, families = { {tables, columns, key, name, same, mode, note}, ... }, sources = { {source, label, category}, ... }, categoryFrom, stacked, dataColumn, copies}, ... }",
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
			Name:    "tables.merge_sheets",
			Label:   "Regrouper des onglets",
			Summary: "Regroupe des tableaux aux mêmes colonnes, chacun sur son onglet (un par mois, par année, par ville…), en un seul tableau sur une nouvelle feuille, avec en tête une colonne de catégorie tirée des noms d'onglets (Mois, Année, Ville…) et sa liste déroulante. Une seule étape d'annulation.",
			Details: "tables : les tableaux, dans l'ordre (absent : le plus grand groupe trouvé par tables.similar). options : {name (nom du tableau, proposé sinon), sheet (nom de la feuille), category = {name, type = \"date\" | \"integer\" | \"text\", values = {une valeur par tableau}} (déduite des noms d'onglets sinon), noCategory = true (pas de colonne de catégorie), columns (correspondance, comme celle de tables.similar), old = \"delete\" | \"hide\", categoryFrom = \"sheet\" | \"title\" | \"cell:A1\", families = { {tables, mode = \"merge\" | \"once\" | \"leave\", name, columns}, ... }, noCells = true}. " +
				"On regroupe des feuilles entières : chaque autre famille de tableaux des onglets (families ; absent avec tables absent : celles du groupe trouvé, avec le mode proposé) devient aussi un tableau commun sur la même feuille (merge : avec la colonne de catégorie, il suit l'onglet à choix ; once : un seul exemplaire, celui du dernier onglet, le même pour toutes les valeurs ; leave : il reste sur son onglet, alors gardé). La disposition reprend celle de l'onglet le plus récent. Les cellules libres des onglets (titres, totaux, objectifs) sont reprises : une cellule identique partout une fois, une cellule qui change d'un onglet à l'autre devient une cellule par valeur (elle montre celle de la valeur choisie). " +
				"Colonnes réunies (cases vides là où une colonne manquait), types élargis, mise en forme et saisies invalides gardées. La clé est gardée si aucune valeur ne se répète d'un onglet à l'autre, sinon le tableau commun n'a pas de clé (keyNote donne un exemple). Ce qui citait les anciens tableaux est repris sur le tableau commun filtré par la catégorie, dans la même étape d'annulation, et listé dans rewritten (formula : avant, after : après) : SOMME(Janvier[Montant]) devient SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; \"=Janvier\") (de même MOYENNE, MAX, MIN ; NBVAL devient NB.SI.ENS sur les cases non vides, NB compte les nombres de la colonne filtrée, les .SI et .SI.ENS gagnent le critère du mois, LIGNES(Janvier) devient NB.SI.ENS(Ventes[Mois]; \"=Janvier\"), une autre colonne entière lue comme une plage devient FILTRE(Ventes[X]; Ventes[Mois]=\"Janvier\")) ; un tableau croisé passe sur le tableau commun avec un filtre de rapport Mois = Janvier, un graphique avec un filtre (config.filters). Ce qui n'a pas d'équivalent exact (une ligne précise Janvier[X][3], REFERENCE, RECHERCHEV, EXISTE, SOUS.TOTAL, le tableau entier, un contrôle, une requête ; le texte SQL et les scripts Lua ne sont pas repérés) est listé dans pending avec la raison (why), et les anciens onglets sont alors gardés masqués (tableaux renommés Janvier_ancien) plutôt que supprimés ; sans colonne de catégorie, rien n'est repris. Refusé pour un classeur partagé sur un serveur, une feuille protégée, un tableau filtré ou chargé par une requête.",
			Returns: "{sheet, sheetName, table, rows, old, keyKept, keyNote, rewritten = { {kind, sheetName, where, formula, after}, ... }, pending = { {kind, sheetName, where, formula, why}, ... }, notes, notice, families = { {name, mode, tables, rows, follows, keyNote}, ... }, cells, perValue}",
			Params: []Param{
				{Name: "tables", Type: Array, Items: String, Desc: "tableaux à regrouper, dans l'ordre (absent : le plus grand groupe d'onglets semblables)", Optional: true},
				{Name: "options", Type: Object, Desc: "{name, sheet, category = {name, type, values}, noCategory, columns, old, categoryFrom, families, noCells}", Optional: true},
			},
			Write: true,
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
				plan := canvas.MergePlan{Category: o.Category, NoCategory: o.NoCategory, Columns: o.Columns, Sheet: o.Sheet, Name: o.Name, Old: o.Old,
					CategoryFrom: o.CategoryFrom, Families: o.Families, NoCells: o.NoCells}
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
						return nil, msg.New("api.noSimilarSheets", nil)
					}
					for _, t := range gs[0].Tables {
						plan.Tables = append(plan.Tables, t.Table)
					}
					if len(plan.Columns) == 0 {
						plan.Columns = gs[0].Columns
					}
					// Les autres familles des onglets, avec le mode proposé, et la
					// source de catégorie proposée.
					if o.Families == nil {
						for _, f := range gs[0].Families {
							mf := canvas.MergeFamily{Mode: f.Mode, Name: f.Name, Columns: f.Columns}
							for _, tb := range f.Tables {
								mf.Tables = append(mf.Tables, tb.Table)
							}
							plan.Families = append(plan.Families, mf)
						}
					}
					if plan.CategoryFrom == "" && plan.Category == nil {
						plan.CategoryFrom = gs[0].CategoryFrom
					}
				}
				return d.MergeSheets(c, plan)
			},
		},
	)
}
