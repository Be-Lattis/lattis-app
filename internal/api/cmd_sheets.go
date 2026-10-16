package api

import (
	"fmt"

	"github.com/lalamefine/slitable/internal/canvas"
)

var pSheet = Param{Name: "sheet", Type: Ref, Desc: "feuille (nom ou identifiant) ; absente : la feuille affichée", Optional: true}

// SheetView est le contenu d'une feuille.
type SheetView struct {
	ID      int64        `json:"id"`
	Name    string       `json:"name"`
	Tables  []Table      `json:"tables"`
	Cells   []CellView   `json:"cells"`
	Names   []NamedCell  `json:"names"`
	Pivots  []ObjectView `json:"pivots"`
	Charts  []ObjectView `json:"charts"`
	Buttons []ButtonView `json:"buttons"`
}

// CellView est une cellule libre (hors tableau).
type CellView struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Raw   string `json:"raw"`
	Value string `json:"value,omitempty"` // valeur calculée d'une formule
	Error bool   `json:"error,omitempty"`
}

// NamedCell est une cellule nommée.
type NamedCell struct {
	Name string `json:"name"`
	Row  int    `json:"row"`
	Col  int    `json:"col"`
}

// ObjectView est un tableau croisé ou un graphique.
type ObjectView struct {
	ID     int64  `json:"id"`
	Name   string `json:"name,omitempty"`
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Config any    `json:"config"`
	Error  string `json:"error,omitempty"`
}

// ButtonView est un bouton d'action.
type ButtonView struct {
	ID       int64  `json:"id"`
	Label    string `json:"label"`
	Script   string `json:"script"`
	Function string `json:"function,omitempty"`
	Row      int    `json:"row"`
	Col      int    `json:"col"`
}

func init() {
	register(
		&Command{
			Name:    "sheets.list",
			Summary: "Liste les feuilles du classeur, dans l'ordre des onglets.",
			Details: "protected vaut true pour une feuille protégée (Révision › Protéger la feuille).",
			Returns: "{ {id, name, protected}, ... }",
			Example: `for _, f in ipairs(sheets.list()) do
  print(f.id, f.name)
end`,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				l, err := sheetList(c, d)
				if l == nil {
					l = []sheetRef{}
				}
				return l, err
			},
		},
		&Command{
			Name:    "sheets.get",
			Summary: "Décrit le contenu d'une feuille : tableaux, cellules libres, noms, tableaux croisés, graphiques et boutons.",
			Details: "Les positions (row, col) se comptent à partir de 1. Les valeurs des tableaux se lisent avec tables.rows ou db.query.",
			Returns: "{id, name, tables, cells, names, pivots, charts, buttons}",
			Params:  []Param{pSheet},
			Example: `local f = sheets.get()
for _, t in ipairs(f.tables) do
  print(t.name, t.rows .. " lignes", "en " .. t.row .. "," .. t.col)
end`,
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
				return describeSheet(s, l), nil
			},
		},
		&Command{
			Name:    "sheets.add",
			Label:   "Ajouter une feuille",
			Summary: "Ajoute une feuille et renvoie {id, name}.",
			Details: "Sans before, la feuille s'ajoute après le dernier onglet. Sans nom, elle s'appelle comme dans la grille (« Feuille2 »…).",
			Returns: "{id, name}",
			Params: []Param{
				{Name: "name", Type: String, Desc: "nom de la nouvelle feuille", Optional: true},
				{Name: "before", Type: Ref, Desc: "feuille devant laquelle l'insérer", Optional: true},
			},
			Write: true,
			Example: `local f = sheets.add("Synthèse")
ui.show_sheet(f.name)`,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				var id int64
				if c.Has("before") {
					b, err := c.sheet("before")
					if err != nil {
						return nil, err
					}
					id, err = d.InsertSheet(c, b.ID)
					if err != nil {
						return nil, err
					}
				} else if id, err = d.AddSheet(c); err != nil {
					return nil, err
				}
				if name, _ := c.Str("name"); name != "" {
					if err := d.RenameSheet(c, id, name); err != nil {
						return nil, err
					}
				}
				list, err := sheetList(c, d)
				for _, s := range list {
					if s.ID == id {
						return s, nil
					}
				}
				return sheetRef{ID: id}, err
			},
		},
		&Command{
			Name:    "sheets.rename",
			Label:   "Renommer la feuille",
			Summary: "Renomme une feuille.",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "feuille à renommer"}, {Name: "name", Type: String, Desc: "nouveau nom"}},
			Write:   true,
			Example: `sheets.rename("Feuille1", "Ventes")`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				name, _ := c.Str("name")
				d, _ := c.Doc()
				return nil, d.RenameSheet(c, s.ID, name)
			},
		},
		&Command{
			Name:    "sheets.delete",
			Label:   "Supprimer la feuille",
			Summary: "Supprime une feuille sans tableau (ses cellules, graphiques et boutons partent avec elle).",
			Details: "Une feuille qui contient des tableaux est refusée : supprimez-les d'abord avec tables.delete. Le classeur garde au moins une feuille.",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "feuille à supprimer"}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.DeleteSheet(c, s.ID)
			},
		},
		&Command{
			Name:    "sheets.move",
			Label:   "Déplacer la feuille",
			Summary: "Place une feuille au rang position parmi les onglets (1 : premier).",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "feuille à déplacer"}, {Name: "position", Type: Integer, Desc: "rang, à partir de 1"}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				to, err := c.Pos("position", 0)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.MoveSheet(c, s.ID, to)
			},
		},
		&Command{
			Name:    "sheets.copy",
			Label:   "Copier la feuille",
			Summary: "Copie une feuille (tableaux, cellules, mise en forme, tableaux croisés, graphiques, boutons) et renvoie {id, name} de la copie.",
			Details: "Comme Déplacer ou copier… › Créer une copie. Les tableaux copiés reçoivent un nom libre (Ventes → Ventes_2) ; un graphique ou un TCD de la feuille qui lisait un tableau copié lit sa copie. Sans position, la copie se place juste après la feuille copiée.",
			Returns: "{id, name}",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "feuille à copier"}, {Name: "position", Type: Integer, Desc: "rang de la copie parmi les onglets, à partir de 1", Optional: true}},
			Write:   true,
			Example: `local copie = sheets.copy("Feuille1")
print(copie.name) -- Feuille1 (2)`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				list, err := sheetList(c, d)
				if err != nil {
					return nil, err
				}
				to := len(list)
				for i, x := range list {
					if x.ID == s.ID {
						to = i + 1
					}
				}
				if c.Has("position") {
					if to, err = c.Pos("position", 0); err != nil {
						return nil, err
					}
				}
				id, err := d.CopySheet(c, s.ID, to)
				if err != nil {
					return nil, err
				}
				list, err = sheetList(c, d)
				for _, x := range list {
					if x.ID == id {
						return x, nil
					}
				}
				return sheetRef{ID: id}, err
			},
		},
		&Command{
			Name:    "sheets.hide",
			Label:   "Masquer la feuille",
			Summary: "Masque une feuille (Masquer) ou l'affiche de nouveau (hidden = false).",
			Details: "Une feuille masquée n'apparaît plus dans les onglets mais garde son contenu ; le classeur garde au moins une feuille visible.",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "feuille à masquer ou afficher"}, {Name: "hidden", Type: Boolean, Desc: "true (défaut) : masquer ; false : afficher", Optional: true}},
			Write:   true,
			Example: `sheets.add("Calculs")
sheets.hide("Calculs")
sheets.hide("Calculs", false)`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				hidden := true
				if c.Has("hidden") {
					if hidden, err = c.Bool("hidden"); err != nil {
						return nil, err
					}
				}
				d, _ := c.Doc()
				return nil, d.SetSheetHidden(c, s.ID, hidden)
			},
		},
		&Command{
			Name:    "sheets.color",
			Label:   "Couleur d'onglet",
			Summary: "Fixe la couleur de l'onglet d'une feuille (« #rrggbb » ; vide : aucune couleur).",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "feuille"}, {Name: "color", Type: String, Desc: "couleur #rrggbb, ou vide", Optional: true}},
			Write:   true,
			Example: `sheets.color("Feuille1", "#217346")`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				color, _ := c.Str("color")
				d, _ := c.Doc()
				return nil, d.SetSheetColor(c, s.ID, color)
			},
		},
		&Command{
			Name:    "sheets.protect",
			Label:   "Protéger la feuille",
			Summary: "Protège une feuille, comme Révision › Protéger la feuille : toute modification de son contenu est ensuite refusée.",
			Details: "Restent permis la lecture, la sélection, la copie, les filtres et les requêtes SQL de lecture. Le mot de passe est facultatif ; il n'est pas stocké (seulement une empreinte salée). Protéger ne s'annule pas.",
			Params:  []Param{pSheet, {Name: "password", Type: String, Desc: "mot de passe pour ôter la protection", Optional: true}},
			Write:   true,
			Example: `sheets.protect("Feuille1", "secret")
print(sheets.list()[1].protected) -- true`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				pw, _ := c.Str("password")
				d, _ := c.Doc()
				return nil, d.ProtectSheet(c, s.ID, pw)
			},
		},
		&Command{
			Name:    "sheets.unprotect",
			Label:   "Ôter la protection de la feuille",
			Summary: "Ôte la protection d'une feuille (avec son mot de passe s'il y en a un).",
			Params:  []Param{pSheet, {Name: "password", Type: String, Desc: "mot de passe donné en protégeant", Optional: true}},
			Write:   true,
			Example: `sheets.protect("Feuille1", "secret")
sheets.unprotect("Feuille1", "secret")`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				pw, _ := c.Str("password")
				d, _ := c.Doc()
				return nil, d.UnprotectSheet(c, s.ID, pw)
			},
		},
		&Command{
			Name:    "sheets.protect_workbook",
			Label:   "Protéger le classeur",
			Summary: "Protège la structure du classeur, comme Révision › Protéger le classeur : ajouter, supprimer, renommer ou déplacer des feuilles est ensuite refusé.",
			Details: "Le contenu des feuilles reste modifiable (voir sheets.protect). Le mot de passe est facultatif. Protéger ne s'annule pas.",
			Params:  []Param{{Name: "password", Type: String, Desc: "mot de passe pour ôter la protection", Optional: true}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				pw, _ := c.Str("password")
				return nil, d.ProtectWorkbook(c, pw)
			},
		},
		&Command{
			Name:    "sheets.unprotect_workbook",
			Label:   "Ôter la protection du classeur",
			Summary: "Ôte la protection de la structure du classeur (avec son mot de passe s'il y en a un).",
			Params:  []Param{{Name: "password", Type: String, Desc: "mot de passe donné en protégeant", Optional: true}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				pw, _ := c.Str("password")
				return nil, d.UnprotectWorkbook(c, pw)
			},
		},
		&Command{
			Name:    "sheets.set_column_width",
			Label:   "Largeur de colonne",
			Summary: "Fixe la largeur (pixels) d'une colonne de la feuille ; 0 revient à la largeur par défaut.",
			Params:  []Param{{Name: "col", Type: Integer, Desc: "colonne de la feuille, à partir de 1"}, {Name: "width", Type: Integer, Desc: "largeur en pixels"}, pSheet},
			Write:   true,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				w, err := c.Int("width", 0)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.SetColumnWidth(c, s.ID, col, w)
			},
		},
		&Command{
			Name:    "sheets.set_row_height",
			Label:   "Hauteur de ligne",
			Summary: "Fixe la hauteur (pixels) d'une ligne de la feuille ; 0 revient à la hauteur par défaut.",
			Params:  []Param{{Name: "row", Type: Integer, Desc: "ligne de la feuille, à partir de 1"}, {Name: "height", Type: Integer, Desc: "hauteur en pixels"}, pSheet},
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
				h, err := c.Int("height", 0)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.SetRowHeight(c, s.ID, row, h)
			},
		},
		&Command{
			Name:    "sheets.find",
			Summary: "Cherche un texte dans une feuille (sans tenir compte de la casse) et renvoie les cellules trouvées.",
			Returns: "{ {row, col}, ... }",
			Params:  []Param{{Name: "text", Type: String, Desc: "texte cherché"}, pSheet},
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				text, _ := c.Str("text")
				d, _ := c.Doc()
				ms, err := d.Find(c, s.ID, text)
				out := make([]NamedCell, 0, len(ms))
				for _, m := range ms {
					out = append(out, NamedCell{Row: m.Row + 1, Col: m.Col + 1})
				}
				return out, err
			},
		},
		&Command{
			Name:    "sheets.replace",
			Label:   "Remplacer",
			Summary: "Remplace un texte dans toute une feuille et renvoie le nombre de cellules modifiées.",
			Params:  []Param{{Name: "text", Type: String, Desc: "texte cherché"}, {Name: "by", Type: String, Desc: "texte de remplacement"}, pSheet},
			Write:   true,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				text, _ := c.Str("text")
				by, _ := c.Str("by")
				d, _ := c.Doc()
				return d.Replace(c, s.ID, text, by)
			},
		},
	)
}

func describeSheet(s sheetRef, l canvas.Layout) SheetView {
	v := SheetView{ID: s.ID, Name: s.Name, Tables: []Table{}, Cells: []CellView{}, Names: []NamedCell{}, Pivots: []ObjectView{}, Charts: []ObjectView{}, Buttons: []ButtonView{}}
	for _, t := range l.Tables {
		v.Tables = append(v.Tables, describeTable(t, s.Name))
	}
	for _, c := range l.Cells {
		cv := CellView{Row: c.Row + 1, Col: c.Col + 1, Raw: c.Raw, Error: c.Error}
		if c.Value != c.Raw {
			cv.Value = c.Value
		}
		v.Cells = append(v.Cells, cv)
	}
	for _, n := range l.Names {
		v.Names = append(v.Names, NamedCell{Name: n.Name, Row: n.Row + 1, Col: n.Col + 1})
	}
	for _, p := range l.Pivots {
		v.Pivots = append(v.Pivots, ObjectView{ID: p.ID, Name: p.Config.Name, Row: p.Row + 1, Col: p.Col + 1, Config: p.Config, Error: p.Error})
	}
	for _, ch := range l.Charts {
		v.Charts = append(v.Charts, ObjectView{ID: ch.ID, Name: ch.Config.Title, Row: ch.Row + 1, Col: ch.Col + 1, Config: ch.Config, Error: ch.Error})
	}
	for _, b := range l.Buttons {
		v.Buttons = append(v.Buttons, describeButton(b))
	}
	return v
}

func describeButton(b canvas.ButtonInfo) ButtonView {
	return ButtonView{ID: b.ID, Label: b.Config.Label, Script: b.Config.Script, Function: b.Config.Function, Row: b.Row + 1, Col: b.Col + 1}
}

var _ = fmt.Sprint
