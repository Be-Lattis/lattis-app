package api

import (
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/script"
)

// SelectionView est la sélection de la grille (positions à partir de 1).
type SelectionView struct {
	Sheet   string `json:"sheet"`
	SheetID int64  `json:"sheet_id"`
	Row     int    `json:"row"`
	Col     int    `json:"col"`
	Row2    int    `json:"row2"`
	Col2    int    `json:"col2"`
	Table   string `json:"table,omitempty"`  // tableau de la cellule active
	Column  string `json:"column,omitempty"` // sa colonne
	View    string `json:"view"`
}

func (c *Call) selectRange(s sheetRef, r0, c0, r1, c1 int) (any, error) {
	sel := Selection{Sheet: s.ID, R0: min(r0, r1), C0: min(c0, c1), R1: max(r0, r1), C1: max(c0, c1), View: "grid"}
	if err := c.Host.Select(c, sel); err != nil {
		return nil, err
	}
	return c.selection()
}

func (c *Call) selection() (SelectionView, error) {
	sel := c.Host.Selection()
	v := SelectionView{SheetID: sel.Sheet, Row: sel.R0 + 1, Col: sel.C0 + 1, Row2: sel.R1 + 1, Col2: sel.C1 + 1, View: sel.View}
	if v.View == "" {
		v.View = "grid"
	}
	d, err := c.Doc()
	if err != nil {
		return v, err
	}
	s, err := c.sheet("")
	if err != nil {
		return v, err
	}
	v.Sheet, v.SheetID = s.Name, s.ID
	l, err := d.Layout(c, s.ID)
	if err != nil {
		return v, err
	}
	if t := tableAt(l, sel.R0, sel.C0); t != nil {
		v.Table, v.Column = t.Name, t.Columns[sel.C0-t.Col].Name
	}
	return v, nil
}

func init() {
	register(
		&Command{
			Name:    "ui.selection",
			Summary: "Donne la feuille affichée et la plage sélectionnée dans la grille.",
			Details: "row, col est le coin haut gauche de la sélection, row2, col2 le coin bas droit (à partir de 1). table et column désignent le tableau et la colonne de la cellule en haut à gauche, s'il y en a un.",
			Returns: "{sheet, sheet_id, row, col, row2, col2, table, column, view}",
			Example: `local s = ui.selection()
print(s.sheet, s.row, s.col, s.table or "hors tableau")`,
			Run: func(c *Call) (any, error) { return c.selection() },
		},
		&Command{
			Name:    "ui.select",
			Summary: "Affiche une feuille et y sélectionne une plage (ou une cellule).",
			Returns: "la nouvelle sélection, comme ui.selection",
			Params:  []Param{pRow, pCol, pRow2, pCol2, pSheet},
			Example: `ui.select(1, 1, 10, 3)   -- de la ligne 1, colonne 1 à la ligne 10, colonne 3`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				r0, c0, r1, c1, err := c.rect()
				if err != nil {
					return nil, err
				}
				return c.selectRange(s, r0, c0, r1, c1)
			},
		},
		&Command{
			Name:    "ui.select_object",
			Summary: "Affiche et sélectionne un objet par son nom : tableau, tableau croisé, graphique (titre), bouton (libellé) ou cellule nommée.",
			Details: "Le tableau est sélectionné en entier, en-tête compris. Renvoie la sélection, avec kind : \"table\", \"pivot\", \"chart\", \"button\" ou \"name\".",
			Returns: "{kind, sheet, row, col, row2, col2, ...}",
			Params:  []Param{{Name: "name", Type: String, Desc: "nom de l'objet"}},
			Example: `ui.select_object("Commandes")`,
			Run: func(c *Call) (any, error) {
				name, _ := c.Str("name")
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				sheets, err := sheetList(c, d)
				if err != nil {
					return nil, err
				}
				type hit struct {
					kind           string
					s              sheetRef
					r0, c0, r1, c1 int
				}
				var found *hit
				for _, s := range sheets {
					l, err := d.Layout(c, s.ID)
					if err != nil {
						return nil, err
					}
					if h := findObject(l, name, s); h != nil {
						found = &hit{h.kind, s, h.r0, h.c0, h.r1, h.c1}
						if h.exact {
							break
						}
					}
				}
				if found == nil {
					return nil, msg.New("api.objectNotFoundNamed", msg.Args{"name": name})
				}
				sel, err := c.selectRange(found.s, found.r0, found.c0, found.r1, found.c1)
				if err != nil {
					return nil, err
				}
				return struct {
					Kind string `json:"kind"`
					SelectionView
				}{found.kind, sel.(SelectionView)}, nil
			},
		},
		&Command{
			Name:    "ui.select_cells",
			Summary: "Sélectionne des cellules d'un tableau par colonne et numéro de ligne.",
			Details: "Sans row, toute la colonne (lignes de données) ; column2 et row2 étendent la plage vers la droite et vers le bas. Les lignes se comptent à partir de 1 sous l'en-tête.",
			Returns: "la nouvelle sélection, comme ui.selection",
			Params: []Param{
				{Name: "table", Type: String, Desc: "nom du tableau"},
				{Name: "column", Type: Ref, Desc: "colonne (nom ou rang)"},
				{Name: "row", Type: Integer, Desc: "ligne de données, à partir de 1 (absente : toute la colonne)", Optional: true},
				{Name: "column2", Type: Ref, Desc: "dernière colonne de la plage", Optional: true},
				{Name: "row2", Type: Integer, Desc: "dernière ligne de la plage", Optional: true},
			},
			Example: `ui.select_cells("Commandes", "Montant")          -- toute la colonne
ui.select_cells("Commandes", "Client", 2, "Montant", 3)`,
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				k0, err := c.column(t, "column")
				if err != nil {
					return nil, err
				}
				k1 := k0
				if c.Has("column2") {
					if k1, err = c.column(t, "column2"); err != nil {
						return nil, err
					}
				}
				r0, r1 := 0, t.Rows-1
				if c.Has("row") {
					if r0, err = c.Pos("row", 0); err != nil {
						return nil, err
					}
					if r1, err = c.Pos("row2", r0); err != nil {
						return nil, err
					}
				}
				if t.Rows == 0 {
					r0, r1 = -1, -1 // en-tête seul
				}
				if max(r0, r1) >= t.Rows {
					return nil, msg.New("api.tableRowsOnly", msg.Args{"table": t.Name, "rows": t.Rows})
				}
				base := t.Row + 1
				return c.selectRange(t.Sheet, base+r0, t.Col+k0, base+r1, t.Col+k1)
			},
		},
		&Command{
			Name:    "ui.show_sheet",
			Summary: "Affiche une feuille (vue grille), en gardant la sélection de la cellule en haut à gauche.",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "feuille (nom ou identifiant)"}},
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				return c.selectRange(s, 0, 0, 0, 0)
			},
		},
		&Command{
			Name:    "ui.show_view",
			Summary: "Affiche une vue de l'application : \"grid\" (feuilles), \"sql\" ou \"script\".",
			Params:  []Param{{Name: "view", Type: String, Desc: "grid, sql ou script"}},
			Run: func(c *Call) (any, error) {
				v, _ := c.Str("view")
				if v != "grid" && v != "sql" && v != "script" {
					return nil, msg.New("api.unknownView", msg.Args{"view": fmt.Sprintf("%q", v)})
				}
				return nil, c.Host.ShowView(c, v)
			},
		},
		&Command{
			Name:    "ui.notify",
			Summary: "Affiche un message dans la barre d'état, sans interrompre le script.",
			Params:  []Param{{Name: "message", Type: String, Desc: "texte du message"}},
			Run: func(c *Call) (any, error) {
				m, _ := c.Str("message")
				c.Host.Notify(m)
				return nil, nil
			},
		},
		&Command{
			Name:    "ui.dialog",
			Summary: "Ouvre une boîte de dialogue avec des champs de saisie et attend la réponse ; renvoie { nom = valeur } ou nil si l'utilisateur annule.",
			Details: dialogHelp,
			Returns: "{ nom = valeur, ... } ou nil",
			Params: []Param{
				{Name: "title", Type: String, Desc: "titre de la boîte"},
				{Name: "fields", Type: Array, Items: Object, Desc: "champs : { {name, label, type, default, options, required, placeholder, min, max}, ... }"},
				{Name: "message", Type: String, Desc: "texte affiché au-dessus des champs", Optional: true},
				{Name: "ok", Type: String, Desc: "libellé du bouton de validation (OK par défaut)", Optional: true},
				{Name: "cancel", Type: String, Desc: "libellé du bouton d'annulation (Annuler par défaut)", Optional: true},
			},
			Example: `local r = ui.dialog{
  title = "Nouvelle commande",
  fields = {
    {name = "client", label = "Client", required = true},
    {name = "montant", label = "Montant", type = "number", min = 0},
    {name = "paye", label = "Payée", type = "checkbox"},
    {name = "mode", label = "Règlement", type = "select", options = {"Carte", "Virement", "Chèque"}},
    {name = "echeance", label = "Échéance", type = "date"},
  },
}
if r then
  db.insert("Commandes", {Client = r.client, Montant = r.montant, ["Payé"] = r.paye})
end`,
			Run: func(c *Call) (any, error) {
				var d Dialog
				d.Title, _ = c.Str("title")
				d.Message, _ = c.Str("message")
				d.OK, _ = c.Str("ok")
				d.Cancel, _ = c.Str("cancel")
				if d.Cancel == "" {
					d.Cancel = "Annuler"
				}
				if err := c.Decode("fields", &d.Fields); err != nil {
					return nil, err
				}
				return c.dialog(d)
			},
		},
		&Command{
			Name:    "ui.alert",
			Summary: "Affiche un message dans une boîte de dialogue et attend que l'utilisateur la ferme.",
			Params:  []Param{{Name: "message", Type: String, Desc: "texte du message"}, {Name: "title", Type: String, Desc: "titre (Lattis par défaut)", Optional: true}},
			Example: `local n = db.value("SELECT count(*) FROM Commandes")
ui.alert("Le tableau compte " .. n .. " commandes")`,
			Run: func(c *Call) (any, error) {
				d := Dialog{Fields: []Field{}}
				d.Message, _ = c.Str("message")
				d.Title, _ = c.Str("title")
				_, err := c.dialog(d)
				return nil, err
			},
		},
		&Command{
			Name:    "ui.confirm",
			Summary: "Pose une question oui / non ; renvoie true si l'utilisateur confirme.",
			Params:  []Param{{Name: "message", Type: String, Desc: "question posée"}, {Name: "title", Type: String, Desc: "titre (Lattis par défaut)", Optional: true}},
			Example: `if ui.confirm("Supprimer les commandes payées ?") then
  db.execute([[DELETE FROM Commandes WHERE "Payé"]])
end`,
			Run: func(c *Call) (any, error) {
				d := Dialog{Fields: []Field{}, OK: "Oui", Cancel: "Non"}
				d.Message, _ = c.Str("message")
				d.Title, _ = c.Str("title")
				v, err := c.dialog(d)
				return v != nil, err
			},
		},
		&Command{
			Name:    "ui.prompt",
			Summary: "Demande un texte à l'utilisateur ; renvoie le texte saisi, ou nil s'il annule.",
			Params: []Param{
				{Name: "message", Type: String, Desc: "question posée"},
				{Name: "default", Type: String, Desc: "valeur proposée", Optional: true},
				{Name: "title", Type: String, Desc: "titre (Lattis par défaut)", Optional: true},
			},
			Example: `local nom = ui.prompt("Nom de la nouvelle feuille", "Synthèse")
if nom then sheets.add(nom) end`,
			Run: func(c *Call) (any, error) {
				d := Dialog{Cancel: "Annuler"}
				d.Title, _ = c.Str("title")
				msg, _ := c.Str("message")
				def, _ := c.Str("default")
				d.Fields = []Field{{Name: "value", Label: msg, Default: def}}
				v, err := c.dialog(d)
				if v == nil || err != nil {
					return nil, err
				}
				return v["value"], nil
			},
		},
	)
}

const dialogHelp = `Types de champs (type) : "text" (par défaut), "textarea", "password", "number", "integer", "checkbox", "select" et "radio" (avec options = {...}), "date" (renvoyée en "aaaa-mm-jj") et "color" ("#rrggbb"). label est le libellé affiché (name par défaut), default la valeur proposée, required interdit de valider un champ vide, min et max bornent un nombre. Une case à cocher renvoie true ou false, un nombre un nombre, les autres champs un texte. La durée maximale du script ne court pas pendant que la boîte est ouverte.`

// dialog valide une boîte de dialogue, l'ouvre et renvoie les valeurs
// saisies (nil : annulée).
func (c *Call) dialog(d Dialog) (map[string]any, error) {
	if d.Title == "" {
		d.Title = "Lattis"
	}
	if d.OK == "" {
		d.OK = "OK"
	}
	seen := map[string]bool{}
	for i := range d.Fields {
		f := &d.Fields[i]
		if f.Name == "" {
			return nil, msg.New("api.fieldNoName", msg.Args{"field": i + 1})
		}
		if seen[f.Name] {
			return nil, msg.New("api.fieldDuplicate", msg.Args{"field": fmt.Sprintf("%q", f.Name)})
		}
		seen[f.Name] = true
		if f.Type == "" {
			f.Type = "text"
		}
		if !fieldTypes[f.Type] {
			return nil, msg.New("api.fieldUnknownType", msg.Args{"field": fmt.Sprintf("%q", f.Name), "type": fmt.Sprintf("%q", f.Type)})
		}
		if (f.Type == "select" || f.Type == "radio") && len(f.Options) == 0 {
			return nil, msg.New("api.fieldNoOptions", msg.Args{"field": fmt.Sprintf("%q", f.Name)})
		}
		if f.Label == "" {
			f.Label = f.Name
		}
	}
	defer script.Pause(c)()
	vals, ok, err := c.Host.Dialog(c, d)
	if err != nil || !ok {
		return nil, err
	}
	out := map[string]any{}
	for _, f := range d.Fields {
		v := vals[f.Name]
		switch f.Type {
		case "checkbox":
			b, _ := v.(bool)
			out[f.Name] = b
		case "number", "integer":
			if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
				v = nil
			}
			if v == nil {
				continue
			}
			n, ok := number(v)
			if !ok {
				return nil, msg.New("api.fieldNumber", msg.Args{"field": fmt.Sprintf("%q", f.Name)})
			}
			out[f.Name] = n
		default:
			if v != nil {
				out[f.Name] = fmt.Sprint(v)
			}
		}
	}
	return out, nil
}

type objectHit struct {
	kind           string
	r0, c0, r1, c1 int
	exact          bool
}

// findObject cherche un objet nommé sur une feuille (la casse exacte
// d'abord).
func findObject(l canvas.Layout, name string, _ sheetRef) *objectHit {
	var loose *objectHit
	try := func(n, kind string, r0, c0, r1, c1 int) *objectHit {
		if n == name {
			return &objectHit{kind, r0, c0, r1, c1, true}
		}
		if loose == nil && strings.EqualFold(n, name) {
			loose = &objectHit{kind, r0, c0, r1, c1, false}
		}
		return nil
	}
	for _, t := range l.Tables {
		if h := try(t.Name, "table", t.Row, t.Col, t.Row+t.Rows, t.Col+len(t.Columns)-1); h != nil {
			return h
		}
	}
	for _, p := range l.Pivots {
		rows, cols := max(len(p.Cells), 1), 1
		if len(p.Cells) > 0 {
			cols = max(len(p.Cells[0]), 1)
		}
		if h := try(p.Config.Name, "pivot", p.Row, p.Col, p.Row+rows-1, p.Col+cols-1); h != nil {
			return h
		}
	}
	for _, ch := range l.Charts {
		if h := try(ch.Config.Title, "chart", ch.Row, ch.Col, ch.Row, ch.Col); h != nil {
			return h
		}
	}
	for _, b := range l.Buttons {
		if h := try(b.Config.Label, "button", b.Row, b.Col, b.Row, b.Col); h != nil {
			return h
		}
	}
	for _, n := range l.Names {
		if h := try(n.Name, "name", n.Row, n.Col, n.Row, n.Col); h != nil {
			return h
		}
	}
	return loose
}
