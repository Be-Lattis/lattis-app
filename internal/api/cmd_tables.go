package api

import (
	"fmt"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/grid"
)

var (
	pTable  = Param{Name: "table", Type: String, Desc: "nom du tableau"}
	pColumn = Param{Name: "column", Type: Ref, Desc: "colonne (nom ou rang à partir de 1)"}
)

// MaxRows borne tables.rows.
const MaxRows = 10_000

// Rows est un extrait d'un tableau.
type Rows struct {
	Table   string   `json:"table"`
	Columns []string `json:"columns"`
	First   int      `json:"first"` // première ligne renvoyée, à partir de 1
	Total   int      `json:"total"` // lignes affichées du tableau
	IDs     []int64  `json:"ids"`   // clés _id des lignes
	Rows    [][]any  `json:"rows"`
}

// textColumnsHelp décrit les options de « Convertir ».
const textColumnsHelp = `options : {delims, merge, quote, fixed, breaks, columns, decimal, thousands, dest}. delims : caractères séparateurs (";", ",", " ", "	"…) ; merge = true : séparateurs consécutifs comptés pour un seul ; quote : qualificateur de texte ("\"" par défaut côté assistant, "'" ou "" pour aucun). fixed = true : largeur fixe, breaks donne les positions des coupures (en caractères). columns : format de chaque colonne produite, {format = "" (standard), "text", "date" ou "skip" (non importée), order = "DMY", "MDY", "YMD"… pour les dates}. decimal et thousands : séparateurs des nombres ("," et " " par défaut). dest : cellule de destination (« E2 ») dans des cellules libres.`

// tableCmd : commande sur un tableau, sans autre paramètre.
func tableCmd(name, label, summary string, write bool, run func(c *Call, t tableRef) (any, error)) *Command {
	return &Command{Name: name, Label: label, Summary: summary, Write: write, Params: []Param{pTable},
		Run: func(c *Call) (any, error) {
			t, err := c.table("table")
			if err != nil {
				return nil, err
			}
			return run(c, t)
		}}
}

// columnCmd : commande sur une colonne d'un tableau.
func columnCmd(cmd *Command, run func(c *Call, t tableRef, i int) (any, error)) *Command {
	cmd.Params = append([]Param{pTable, pColumn}, cmd.Params...)
	cmd.Run = func(c *Call) (any, error) {
		t, err := c.table("table")
		if err != nil {
			return nil, err
		}
		i, err := c.column(t, "column")
		if err != nil {
			return nil, err
		}
		return run(c, t, i)
	}
	return cmd
}

// TableEntry est un tableau ou un tableau croisé de tables.list.
type TableEntry struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"` // table ou pivot
	Sheet   string   `json:"sheet"`
	Rows    int      `json:"rows"`
	Columns []string `json:"columns"`
	Types   []string `json:"types"`
	// ColsField : champ en colonnes d'un tableau croisé ; ses colonnes, à
	// partir de la ColsFrom-ième (comptée depuis 1), portent ses valeurs.
	ColsField string `json:"colsField,omitempty"`
	ColsFrom  int    `json:"colsFrom,omitempty"`
}

func init() {
	register(
		&Command{
			Name:    "tables.list",
			Summary: "Liste les tableaux et les tableaux croisés de toutes les feuilles, avec leurs colonnes.",
			Returns: "{ {name, kind, sheet, rows, columns, types, colsField, colsFrom}, ... } (colsField : champ en colonnes d'un tableau croisé, dont les colonnes à partir de la colsFrom-ième portent les valeurs)",
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				all, err := d.AllTables(c)
				if err != nil {
					return nil, err
				}
				sheets, err := sheetList(c, d)
				if err != nil {
					return nil, err
				}
				names := map[int64]string{}
				for _, s := range sheets {
					names[s.ID] = s.Name
				}
				out := []TableEntry{}
				for _, t := range all {
					out = append(out, TableEntry{t.Name, t.Kind, names[t.Sheet], t.Rows, t.Columns, t.Types, t.ColsField, t.ColsFrom})
				}
				return out, nil
			},
		},
		tableCmd("tables.get", "", "Décrit un tableau : feuille, position, nombre de lignes et colonnes (type, formule, clé).", false,
			func(c *Call, t tableRef) (any, error) { return describeTable(t.TableInfo, t.Sheet.Name), nil }),
		&Command{
			Name:    "tables.rows",
			Summary: "Lit des lignes d'un tableau telles qu'elles s'affichent (filtre et colonnes calculées compris).",
			Details: "first compte à partir de 1 ; count vaut 100 par défaut (10 000 au plus). Pour filtrer ou agréger, préférez db.query.",
			Returns: "{table, columns, first, total, ids, rows}",
			Params:  []Param{pTable, {Name: "first", Type: Integer, Desc: "première ligne, à partir de 1", Optional: true}, {Name: "count", Type: Integer, Desc: "nombre de lignes (100 par défaut)", Optional: true}},
			Example: `local r = tables.rows("Commandes", 1, 5)
for i, ligne in ipairs(r.rows) do
  print(r.ids[i], table.concat(ligne, " | "))
end`,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				first, err := c.Pos("first", 0)
				if err != nil {
					return nil, err
				}
				count, err := c.Int("count", 100)
				if err != nil {
					return nil, err
				}
				count = max(0, min(count, MaxRows))
				out := Rows{Table: t.Name, First: first + 1, Total: t.Rows, IDs: []int64{}, Rows: [][]any{}}
				for _, col := range t.Columns {
					out.Columns = append(out.Columns, col.Name)
				}
				if first >= t.Rows || count == 0 {
					return out, nil
				}
				d, _ := c.Doc()
				g, err := LoadGrid(c, d, t.TableInfo)
				if err != nil {
					return nil, err
				}
				w, err := g.Window(c, first, min(count, t.Rows-first))
				if err != nil {
					return nil, err
				}
				for _, bad := range w.Invalid {
					w.Cells[bad.Row][bad.Col] = bad.Raw
				}
				out.IDs, out.Rows = w.IDs, w.Cells
				return out, nil
			},
		},
		&Command{
			Name:    "tables.rename",
			Label:   "Renommer le tableau",
			Summary: "Renomme un tableau ; les formules qui le citent suivent.",
			Params:  []Param{pTable, {Name: "name", Type: String, Desc: "nouveau nom"}},
			Write:   true,
			Example: `tables.rename("Commandes", "Ventes")`,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				name, _ := c.Str("name")
				d, _ := c.Doc()
				return nil, d.RenameTable(c, t.Name, name)
			},
		},
		&Command{
			Name:    "tables.delete",
			Label:   "Supprimer le tableau",
			Summary: "Supprime un ou plusieurs tableaux entiers (données comprises).",
			Params:  []Param{{Name: "tables", Type: Array, Items: String, Desc: "nom du tableau, ou liste de noms"}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				list, ok := c.Raw("tables").([]any)
				if !ok {
					list = []any{c.Raw("tables")}
				}
				var names []string
				for _, v := range list {
					t, err := findTable(c, c, raw(v))
					if err != nil {
						return nil, err
					}
					names = append(names, t.Name)
				}
				d, _ := c.Doc()
				return nil, d.DeleteTables(c, names)
			},
		},
		tableCmd("tables.to_range", "Convertir en plage",
			"Convertit un tableau en plage de cellules libres (comme Excel) : en-tête et valeurs restent au même endroit, avec leur mise en forme ; une colonne calculée donne sa formule en adresses de cellules (Cell[B2]*Cell[C2]) si elle se traduit simplement, sinon ses valeurs. Les formules qui citaient le tableau dans un agrégat visent ses cellules (Cell[B2:B9]) ; les autres donnent #REF!. La zone reste une plage : y écrire, ou juste à côté, ne refait pas de tableau.",
			true, func(c *Call, t tableRef) (any, error) {
				d, _ := c.Doc()
				return nil, d.ConvertToRange(c, t.Name)
			}),
		&Command{
			Name:    "tables.from_range",
			Label:   "Mettre sous forme de tableau",
			Summary: "Refait un tableau d'une plage (tableau converti par tables.to_range) qui contient la cellule : sa première ligne donne les en-têtes. Renvoie le nom du tableau.",
			Params:  []Param{pRow, pCol, pSheet},
			Write:   true,
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
				d, _ := c.Doc()
				return d.FormatAsTable(c, s.ID, row, col)
			},
		},
		&Command{
			Name:    "tables.move",
			Label:   "Déplacer le tableau",
			Summary: "Déplace un tableau sur sa feuille (coin haut gauche de l'en-tête en row, col). Posé côte à côte d'un autre tableau, il le rejoint.",
			Params:  []Param{pTable, pRow, pCol},
			Write:   true,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
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
				return nil, d.MoveTable(c, t.Name, row, col)
			},
		},
		&Command{
			Name:    "tables.sort",
			Label:   "Trier",
			Summary: "Trie un tableau selon une colonne.",
			Params:  []Param{pTable, pColumn, {Name: "descending", Type: Boolean, Desc: "ordre décroissant", Optional: true}},
			Write:   true,
			Example: `tables.sort("Commandes", "Montant", true)`,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				i, err := c.column(t, "column")
				if err != nil {
					return nil, err
				}
				desc, err := c.Bool("descending")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.SortTable(c, t.Name, i, desc)
			},
		},
		columnCmd(&Command{Name: "tables.sort_custom", Label: "Tri personnalisé",
			Summary: "Trie un tableau selon une colonne dans un ordre personnalisé (liste de valeurs ou liste nommée, comme « Mois » ou « Jours »).",
			Details: "Les valeurs de la liste viennent d'abord, dans son ordre (à rebours avec descending), comparées au texte affiché sans tenir compte de la casse ; " +
				"les autres suivent dans l'ordre croissant, les cellules vides à la fin. L'ordre est enregistré pour la colonne et ré-appliqué à l'actualisation " +
				"d'un tableau chargé par une requête. Listes prédéfinies : Jours, Jours abrégés, Mois, Mois abrégés ; les autres se définissent avec tables.set_sort_lists.",
			Params: []Param{{Name: "values", Type: Array, Items: String, Desc: "valeurs dans l'ordre voulu", Optional: true},
				{Name: "list", Type: String, Desc: "nom d'une liste personnalisée (à la place de values)", Optional: true},
				{Name: "descending", Type: Boolean, Desc: "liste à rebours", Optional: true}},
			Write: true, Example: `tables.sort_custom("Commandes", "Client", {"Martin", "Durand"})
tables.sort_custom("Commandes", "Client", nil, "Jours", true) -- liste nommée, à rebours`},
			func(c *Call, t tableRef, i int) (any, error) {
				var o canvas.CustomOrder
				if c.Has("values") {
					var vals []any
					if err := c.Decode("values", &vals); err != nil {
						return nil, err
					}
					for _, v := range vals {
						o.Values = append(o.Values, raw(v))
					}
				}
				if c.Has("list") {
					l, err := c.Str("list")
					if err != nil {
						return nil, err
					}
					o.List = l
				}
				o.Desc, _ = c.Bool("descending")
				d, _ := c.Doc()
				return nil, d.SortTableCustom(c, t.Name, i, o)
			}),
		columnCmd(&Command{Name: "columns.sort_order", Summary: "Rend l'ordre personnalisé enregistré d'une colonne (nil : aucun).",
			Returns: "{list, values = {...}, desc, applied} : applied si c'est le dernier tri du tableau"},
			func(c *Call, t tableRef, i int) (any, error) {
				d, _ := c.Doc()
				o, ok, applied, err := d.ColumnOrder(c, t.Name, i)
				if err != nil || !ok {
					return nil, err
				}
				return map[string]any{"list": o.List, "values": o.Values, "desc": o.Desc, "applied": applied}, nil
			}),
		&Command{
			Name:    "tables.sort_lists",
			Summary: "Liste les listes personnalisées de tri : prédéfinies (Jours, Mois…) puis celles du classeur.",
			Returns: "{ {name, values = {...}, builtin}, ... }",
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				return d.CustomLists(c)
			},
		},
		&Command{
			Name:    "tables.set_sort_lists",
			Label:   "Listes personnalisées",
			Summary: "Remplace les listes personnalisées du classeur (les listes prédéfinies ne changent pas).",
			Params:  []Param{{Name: "lists", Type: Array, Items: Object, Desc: "listes : { {name = \"Tailles\", values = {\"S\", \"M\", \"L\"}}, ... }"}},
			Write:   true,
			Example: `tables.set_sort_lists({ {name = "Tailles", values = {"S", "M", "L", "XL"}} })`,
			Run: func(c *Call) (any, error) {
				var lists []canvas.CustomList
				if err := c.Decode("lists", &lists); err != nil {
					return nil, err
				}
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				return nil, d.SetCustomLists(c, lists)
			},
		},
		&Command{
			Name:    "tables.insert_row",
			Label:   "Insérer une ligne",
			Summary: "Insère une ligne vide avant la ligne de données row (absente : à la fin du tableau).",
			Params:  []Param{pTable, {Name: "row", Type: Integer, Desc: "ligne de données, à partir de 1", Optional: true}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				n, err := c.Pos("row", t.Rows)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.InsertRow(c, t.Name, n)
			},
		},
		&Command{
			Name:    "tables.delete_row",
			Label:   "Supprimer une ligne",
			Summary: "Supprime la ligne de données row ; le tableau rétrécit.",
			Details: "Pour supprimer une ligne par sa clé, utilisez db.delete.",
			Params:  []Param{pTable, {Name: "row", Type: Integer, Desc: "ligne de données, à partir de 1"}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				n, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.DeleteRow(c, t.Name, n)
			},
		},
		tableCmd("tables.clear_filters", "Effacer les filtres", "Retire tous les filtres d'un tableau.", true,
			func(c *Call, t tableRef) (any, error) {
				d, _ := c.Doc()
				return nil, d.ClearFilters(c, t.Name)
			}),
		&Command{
			Name:    "tables.duplicates",
			Summary: "Compte les lignes en double d'un tableau, selon des colonnes (absentes : toutes).",
			Params:  []Param{pTable, {Name: "columns", Type: Array, Items: String, Desc: "colonnes comparées", Optional: true}},
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				cols, err := c.columns(t, "columns")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return d.DuplicatesPreview(c, t.Name, cols)
			},
		},
		&Command{
			Name:    "tables.remove_duplicates",
			Label:   "Supprimer les doublons",
			Summary: "Supprime les lignes en double d'un tableau (la première est gardée) et renvoie leur nombre.",
			Params:  []Param{pTable, {Name: "columns", Type: Array, Items: String, Desc: "colonnes comparées (absentes : toutes)", Optional: true}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				cols, err := c.columns(t, "columns")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return d.RemoveDuplicates(c, t.Name, cols)
			},
		},
		columnCmd(&Command{
			Name:    "tables.text_to_columns",
			Label:   "Convertir",
			Summary: "Convertit le texte d'une colonne en colonnes, comme l'assistant Convertir d'Excel : la première partie reste dans la colonne, les suivantes deviennent de nouvelles colonnes à droite. Renvoie le nombre de colonnes importées.",
			Details: textColumnsHelp,
			Params:  []Param{{Name: "options", Type: Object, Desc: "{delims, merge, quote, fixed, breaks, columns, decimal, thousands}"}},
			Write:   true,
			Example: `tables.text_to_columns("Commandes", "Échéance", {delims = "/", columns = {{format = "text"}}})`,
		}, func(c *Call, t tableRef, i int) (any, error) {
			var o canvas.TextColumnsOptions
			if err := c.Decode("options", &o); err != nil {
				return nil, err
			}
			d, _ := c.Doc()
			return d.TextToColumns(c, t.Sheet.ID, t.Row+1, t.Row+t.Rows, t.Col+i, o)
		}),
		tableCmd("tables.header_to_title", "En-tête en titre", "L'en-tête du tableau était un titre : il devient une cellule libre au-dessus, et la première ligne de données devient l'en-tête.", true,
			func(c *Call, t tableRef) (any, error) {
				d, _ := c.Doc()
				return nil, d.HeaderToTitle(c, t.Name)
			}),
		tableCmd("tables.last_row_out", "Sortir la dernière ligne", "La dernière ligne de données sort du tableau : elle reste juste dessous en cellules libres, gardées hors du tableau (une note, ou un total si elle commence par « Total »).", true,
			func(c *Call, t tableRef) (any, error) {
				d, _ := c.Doc()
				if t.Total < 1 {
					return nil, fmt.Errorf("le tableau « %s » n'a pas de ligne de données", t.Name)
				}
				return nil, d.RowToNote(c, t.Name, t.Total-1)
			}),
		tableCmd("tables.promote_header", "Ligne d'en-tête", "Prend la première ligne de données comme ligne d'en-têtes.", true,
			func(c *Call, t tableRef) (any, error) {
				d, _ := c.Doc()
				return nil, d.PromoteHeader(c, t.Name)
			}),
		&Command{
			Name:  "tables.two_level_headers",
			Label: "Groupes d'en-têtes",
			Summary: "Affiche (on = true) les en-têtes du tableau sur deux lignes : le nom « Groupe.Sous » d'une colonne montre son groupe dans une bande au-dessus de l'en-tête, fusionnée sur les colonnes voisines du même groupe ; les formules et le SQL gardent le nom complet (T[Groupe.Sous]). " +
				"À l'activation, les cellules libres de la ligne au-dessus de l'en-tête (fusionnées ou non) deviennent les groupes des colonnes qu'elles couvrent ; on = false garde les noms complets sur une ligne. " +
				"Une colonne au nom simple (sans point) est sur un niveau : sa case de la bande et son en-tête ne font qu'une cellule. À l'activation, un groupe seul au-dessus d'un en-tête automatique (« Colonne 17 ») devient le nom de la colonne, un groupe qui répète l'en-tête disparaît. " +
				"Pour ajuster : columns.rename (« Groupe.Sous » ou nom simple), cells.merge / cells.unmerge sur la bande (voir cells.merge), cells.set sur la bande pour poser ou renommer un groupe.",
			Params:  []Param{pTable, {Name: "on", Type: Boolean, Desc: "true : deux lignes ; false : une ligne"}},
			Write:   true,
			Example: `tables.two_level_headers("Commandes", true)`,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				on, err := c.Bool("on")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.SetTwoLevel(c, t.Name, on)
			},
		},
		columnCmd(&Command{Name: "tables.key_check", Summary: "Dit si une colonne peut devenir la clé du tableau : renvoie \"\" si oui, sinon la raison."},
			func(c *Call, t tableRef, i int) (any, error) {
				d, _ := c.Doc()
				return d.KeyCheck(c, t.Name, i)
			}),
		columnCmd(&Command{Name: "tables.set_key", Label: "Clé du tableau", Summary: "Fait d'une colonne aux valeurs uniques la clé du tableau (index UNIQUE).", Write: true},
			func(c *Call, t tableRef, i int) (any, error) {
				d, _ := c.Doc()
				return nil, d.SetKey(c, t.Name, i)
			}),
		tableCmd("tables.clear_key", "Retirer la clé", "Retire la clé d'un tableau.", true,
			func(c *Call, t tableRef) (any, error) {
				d, _ := c.Doc()
				return nil, d.ClearKey(c, t.Name)
			}),
		tableCmd("tables.indexes", "", "Liste les index posés sur le serveur pour un tableau partagé d'un classeur collaboratif (propriétaire ou concepteur).", false,
			func(c *Call, t tableRef) (any, error) {
				d, _ := c.Doc()
				x, err := d.Indexer()
				if err != nil {
					return nil, err
				}
				return x.Indexes(c, t.ID)
			}),
		&Command{
			Name:  "tables.add_index",
			Label: "Ajouter un index",
			Summary: "Pose sur le serveur un index sur des colonnes d'un tableau partagé d'un classeur collaboratif (propriétaire ou concepteur), " +
				"pour accélérer les filtres et les recherches des autres outils ; unique : les valeurs ne se répètent pas. Ne s'annule pas par Ctrl+Z : tables.drop_index le retire. Renvoie son nom.",
			Write:  true,
			Params: []Param{pTable, {Name: "columns", Type: Array, Items: String, Desc: "colonnes de l'index (1 à 8)"}, {Name: "unique", Type: Boolean, Desc: "index unique", Optional: true}},
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				if !c.Has("columns") {
					return nil, fmt.Errorf("columns : au moins une colonne")
				}
				idx, err := c.columns(t, "columns")
				if err != nil {
					return nil, err
				}
				unique, err := c.Bool("unique")
				if err != nil {
					return nil, err
				}
				cols := make([]int64, len(idx))
				for k, i := range idx {
					cols[k] = t.Columns[i].ID
				}
				d, _ := c.Doc()
				x, err := d.Indexer()
				if err != nil {
					return nil, err
				}
				return x.CreateIndex(c, t.ID, cols, unique)
			},
		},
		&Command{
			Name:    "tables.drop_index",
			Label:   "Retirer un index",
			Summary: "Retire un index posé sur le serveur (nom rendu par tables.add_index ou tables.indexes).",
			Write:   true,
			Params:  []Param{pTable, {Name: "name", Type: String, Desc: "nom de l'index (lattis_ix_…)"}},
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				name, err := c.Str("name")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				x, err := d.Indexer()
				if err != nil {
					return nil, err
				}
				return nil, x.DropIndex(c, t.ID, name)
			},
		},
		tableCmd("tables.keep_row_numbers", "Garder les numéros de ligne",
			"Le tableau, sans clé, désigne ses lignes par leur numéro (1 = première, dans l'ordre du tableau ; il change si l'on trie ou insère des lignes) : "+
				"REFERENCE(T[Col]; n) lit la n-ième ligne, EXISTE(T; n) dit si elle existe, une RECHERCHEV devient INDEX/EQUIV (première ligne trouvée, doublons permis). "+
				"Lattis ne propose plus de clé pour ce tableau. L'autre choix est tables.set_key.", true,
			func(c *Call, t tableRef) (any, error) {
				d, _ := c.Doc()
				return nil, d.DismissKeyHint(c, t.Name)
			}),
		&Command{
			Name:    "tables.extract_columns",
			Label:   "Extraire des colonnes",
			Summary: "Sort les colonnes column à column2 d'un tableau vers un nouveau tableau posé en row, col.",
			Params:  []Param{pTable, pColumn, {Name: "column2", Type: Ref, Desc: "dernière colonne extraite"}, pRow, pCol},
			Write:   true,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				i0, err := c.column(t, "column")
				if err != nil {
					return nil, err
				}
				i1, err := c.column(t, "column2")
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
				return nil, d.ExtractColumns(c, t.Name, min(i0, i1), max(i0, i1), row, col)
			},
		},

		// --- Colonnes ---------------------------------------------------------
		&Command{
			Name:    "columns.insert",
			Label:   "Insérer une colonne",
			Summary: "Insère une colonne dans un tableau, avant la colonne position (absente : à la fin).",
			Params:  []Param{pTable, {Name: "name", Type: String, Desc: "nom de la colonne", Optional: true}, {Name: "position", Type: Integer, Desc: "rang, à partir de 1", Optional: true}},
			Write:   true,
			Example: `columns.insert("Commandes", "TVA")`,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				i, err := c.Pos("position", len(t.Columns))
				if err != nil {
					return nil, err
				}
				name, _ := c.Str("name")
				d, _ := c.Doc()
				return nil, d.InsertColumn(c, t.Name, i, name)
			},
		},
		columnCmd(&Command{Name: "columns.delete", Label: "Supprimer une colonne", Summary: "Supprime une colonne d'un tableau (un tableau garde au moins une colonne).", Write: true},
			func(c *Call, t tableRef, i int) (any, error) {
				d, _ := c.Doc()
				return nil, d.DeleteColumn(c, t.Name, i)
			}),
		columnCmd(&Command{Name: "columns.rename", Label: "Renommer la colonne", Summary: "Renomme une colonne (son en-tête).", Write: true,
			Params: []Param{{Name: "name", Type: String, Desc: "nouveau nom"}}, Example: `columns.rename("Commandes", "Montant", "Montant HT")`},
			func(c *Call, t tableRef, i int) (any, error) {
				name, _ := c.Str("name")
				if name == "" {
					return nil, fmt.Errorf("nom de colonne vide")
				}
				d, _ := c.Doc()
				_, err := d.SetCell(c, t.Sheet.ID, t.Row, t.Col+i, name)
				return nil, err
			}),
		columnCmd(&Command{Name: "columns.set_type", Label: "Changer le type", Summary: "Fixe le type d'une colonne ; les valeurs qui ne s'y convertissent pas restent visibles, marquées en erreur.", Write: true,
			Params:  []Param{{Name: "type", Type: String, Desc: "integer, decimal, text, boolean ou date"}},
			Example: `columns.set_type("Commandes", "Échéance", "date")`},
			func(c *Call, t tableRef, i int) (any, error) {
				typ, err := c.gridType()
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.SetType(c, t.Name, i, typ)
			}),
		columnCmd(&Command{Name: "columns.type_preview", Summary: "Compte les valeurs d'une colonne qui ne se convertiraient pas au type donné.",
			Params: []Param{{Name: "type", Type: String, Desc: "integer, decimal, text, boolean ou date"}}},
			func(c *Call, t tableRef, i int) (any, error) {
				typ, err := c.gridType()
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return d.ConvertPreview(c, t.Name, i, typ)
			}),
		columnCmd(&Command{Name: "columns.type_mix", Summary: "Pour une colonne passée en texte par élargissement : type majoritaire et nombre de valeurs qui n'en sont pas.", Returns: "{type, off, total}"},
			func(c *Call, t tableRef, i int) (any, error) {
				d, _ := c.Doc()
				return d.TypeMix(c, t.Name, i)
			}),
		columnCmd(&Command{Name: "columns.fix_type", Label: "Fixer le type", Summary: "Fixe le type actuel d'une colonne : il ne s'élargira plus, une saisie hors type sera marquée en erreur.", Write: true},
			func(c *Call, t tableRef, i int) (any, error) {
				d, _ := c.Doc()
				return nil, d.FixType(c, t.Name, i)
			}),
		columnCmd(&Command{Name: "columns.values", Summary: "Liste les valeurs distinctes d'une colonne avec leur nombre d'occurrences (1 000 au plus).", Returns: "{ {text, count}, ... }"},
			func(c *Call, t tableRef, i int) (any, error) {
				d, _ := c.Doc()
				return d.FilterValues(c, t.Name, i)
			}),
		columnCmd(&Command{Name: "columns.set_filter", Label: "Filtrer", Summary: "Filtre un tableau sur une colonne : seules les lignes dont la valeur est dans values restent affichées.",
			Details: "values est une liste de textes tels qu'affichés (\"\" pour les cellules vides) ; errors = true ne garde que les saisies en erreur. " +
				"conds filtre par condition, comme les « Filtres textuels » et « Filtres numériques » d'Excel : chaque condition est {op, value, to} avec op parmi " +
				"\"eq\", \"ne\" (jokers * et ?), \"begins\", \"ends\", \"contains\", \"notcontains\", \"gt\", \"ge\", \"lt\", \"le\", \"between\" (value et to compris), " +
				"\"top\" et \"bottom\" (value : nombre d'éléments), \"above\" et \"below\" (moyenne). Les conditions doivent toutes être vérifiées, ou l'une d'elles avec any = true. " +
				"Sans values, errors ni conds, le filtre de la colonne est retiré.",
			Params: []Param{{Name: "values", Type: Array, Items: String, Desc: "valeurs gardées", Optional: true}, {Name: "errors", Type: Boolean, Desc: "ne garder que les erreurs", Optional: true},
				{Name: "conds", Type: Array, Items: Object, Desc: "conditions : { {op = \">\", value = \"100\"}, ... }", Optional: true},
				{Name: "any", Type: Boolean, Desc: "une seule condition suffit", Optional: true}},
			Write: true, Example: `columns.set_filter("Commandes", "Client", {"Durand", "Petit"})
columns.set_filter("Commandes", "Montant", nil, false, {{op = "between", value = "100", to = "500"}})`},
			func(c *Call, t tableRef, i int) (any, error) {
				var f canvas.ColumnFilter
				if c.Has("values") {
					var vals []any
					if err := c.Decode("values", &vals); err != nil {
						return nil, err
					}
					for _, v := range vals {
						f.Values = append(f.Values, raw(v))
					}
				}
				f.Errors, _ = c.Bool("errors")
				if c.Has("conds") {
					var raws []struct {
						Op    string `json:"op"`
						Value any    `json:"value"`
						To    any    `json:"to"`
					}
					if err := c.Decode("conds", &raws); err != nil {
						return nil, err
					}
					for _, r := range raws {
						f.Conds = append(f.Conds, canvas.FilterCond{Op: r.Op, Value: raw(r.Value), To: raw(r.To)})
					}
				}
				f.Or, _ = c.Bool("any")
				d, _ := c.Doc()
				if len(f.Values) == 0 && !f.Errors && len(f.Conds) == 0 {
					return nil, d.SetFilter(c, t.Name, i, nil)
				}
				return nil, d.SetFilter(c, t.Name, i, &f)
			}),
		columnCmd(&Command{Name: "columns.set_rules", Label: "Mise en forme conditionnelle", Summary: "Remplace les règles de mise en forme conditionnelle d'une colonne (liste vide : aucune) ; la première règle vérifiée l'emporte.",
			Details: "Chaque règle est {op, value, to, style}. op : \">\", \">=\", \"<\", \"<=\", \"=\", \"<>\", \"between\" (entre value et to), \"contains\", \"empty\", \"notempty\" ou \"error\". " + styleHelp,
			Params:  []Param{{Name: "rules", Type: Array, Items: Object, Desc: "règles : { {op = \">\", value = \"100\", style = {fill = \"#c6efce\"}}, ... }"}},
			Write:   true,
			Example: `columns.set_rules("Commandes", "Montant", {
  {op = ">", value = "100", style = {fill = "#c6efce", bold = true}},
  {op = "<", value = "0", style = {color = "#c00000"}},
})`},
			func(c *Call, t tableRef, i int) (any, error) {
				var raws []struct {
					Op    string         `json:"op"`
					Value any            `json:"value"`
					To    any            `json:"to"`
					Style map[string]any `json:"style"`
				}
				if err := c.Decode("rules", &raws); err != nil {
					return nil, err
				}
				rules := []canvas.Rule{}
				for _, r := range raws {
					st, err := toStyle(r.Style)
					if err != nil {
						return nil, err
					}
					rules = append(rules, canvas.Rule{Op: r.Op, Value: raw(r.Value), To: raw(r.To), Style: st})
				}
				d, _ := c.Doc()
				return nil, d.SetRules(c, t.Name, i, rules)
			}),
	)
}

func (c *Call) gridType() (grid.Type, error) {
	s, _ := c.Str("type")
	switch grid.Type(s) {
	case grid.Integer, grid.Decimal, grid.Text, grid.Boolean, grid.Date:
		return grid.Type(s), nil
	}
	return "", fmt.Errorf("type %q inconnu : integer, decimal, text, boolean ou date", s)
}
