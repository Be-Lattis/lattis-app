package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/lalamefine/slitable/internal/doc"
)

type fakeHost struct {
	d       *doc.Document
	sel     Selection
	changed int
	dialogs []Dialog
	answer  map[string]any
	prints  []string
	notices []string
}

func (h *fakeHost) Doc() (*doc.Document, error)           { return h.d, nil }
func (h *fakeHost) Changed(context.Context)               { h.changed++ }
func (h *fakeHost) Selection() Selection                  { return h.sel }
func (h *fakeHost) Notify(m string)                       { h.notices = append(h.notices, m) }
func (h *fakeHost) Print(l string)                        { h.prints = append(h.prints, l) }
func (h *fakeHost) NewDoc(context.Context) error          { return nil }
func (h *fakeHost) OpenDoc(context.Context, string) error { return nil }
func (h *fakeHost) SaveDoc(context.Context, string) error { return nil }
func (h *fakeHost) ShowView(_ context.Context, v string) error {
	h.sel.View = v
	return nil
}
func (h *fakeHost) Select(_ context.Context, s Selection) error {
	h.sel = s
	return nil
}
func (h *fakeHost) Dialog(_ context.Context, d Dialog) (map[string]any, bool, error) {
	h.dialogs = append(h.dialogs, d)
	return h.answer, h.answer != nil, nil
}

func newHost(t *testing.T) *fakeHost {
	t.Helper()
	d, err := doc.New(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return &fakeHost{d: d, sel: Selection{Sheet: 1}}
}

func call(t *testing.T, h Host, name string, args map[string]any) any {
	t.Helper()
	res, err := Invoke(context.Background(), h, name, args)
	if err != nil {
		t.Fatalf("%s : %v", name, err)
	}
	return res
}

func asJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func run(t *testing.T, h *fakeHost, src string) ScriptResult {
	t.Helper()
	res, err := RunScript(h, "Test", src, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("%s\n%s", res.Error, src)
	}
	return res
}

func TestScriptIsOneUndoStep(t *testing.T) {
	h := newHost(t)
	run(t, h, `
local f = sheets.add("Ventes")
cells.paste{sheet = "Ventes", row = 2, col = 2, values = {{"Produit", "Prix"}, {"Café", 3.5}, {"Thé", 2.8}}}
local tb = sheets.get("Ventes").tables[1]
tables.rename(tb.name, "Produits")
columns.rename("Produits", "Prix", "Prix TTC")
db.insert("Produits", {Produit = "Cacao", ["Prix TTC"] = 4})
sheets.rename("Feuille1", "Accueil")
print(#sheets.list(), db.value("SELECT sum(\"Prix TTC\") FROM Produits"))
`)
	if got := strings.Join(h.prints, ";"); got != "2\t10.3" {
		t.Fatalf("sortie %q", got)
	}
	st := call(t, h, "doc.state", nil).(DocState)
	if st.Undo != "Script « Test »" {
		t.Fatalf("étape %q", st.Undo)
	}
	call(t, h, "doc.undo", nil)
	if got := asJSON(call(t, h, "sheets.list", nil)); !strings.Contains(got, `"Feuille1"`) || strings.Contains(got, "Ventes") {
		t.Fatalf("après une annulation : %s", got)
	}
}

func TestSheetsCRUD(t *testing.T) {
	h := newHost(t)
	call(t, h, "sheets.add", map[string]any{"name": "B"})
	call(t, h, "sheets.add", map[string]any{"name": "A", "before": "B"})
	call(t, h, "sheets.move", map[string]any{"sheet": "Feuille1", "position": int64(3)})
	call(t, h, "sheets.rename", map[string]any{"sheet": "b", "name": "Bilan"})
	if got := asJSON(call(t, h, "sheets.list", nil)); !strings.Contains(got, `"name":"A"},{"id":2,"name":"Bilan"},{"id":1,"name":"Feuille1"}`) {
		t.Fatalf("feuilles %s", got)
	}
	call(t, h, "sheets.delete", map[string]any{"sheet": "A"})
	if _, err := Invoke(context.Background(), h, "sheets.delete", map[string]any{"sheet": "Nulle"}); err == nil {
		t.Fatal("feuille inconnue acceptée")
	}
	if _, err := Invoke(context.Background(), h, "sheets.rename", map[string]any{"sheet": "Bilan"}); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("paramètre manquant : %v", err)
	}
}

func TestSelection(t *testing.T) {
	h := newHost(t)
	call(t, h, "cells.paste", map[string]any{"row": int64(3), "col": int64(2), "values": []any{[]any{"Nom", "Âge"}, []any{"Léa", int64(31)}, []any{"Tom", int64(8)}}})
	tables := call(t, h, "sheets.get", nil).(SheetView).Tables
	if len(tables) != 1 {
		t.Fatalf("tableaux %+v", tables)
	}
	name := tables[0].Name
	sel := call(t, h, "ui.select_object", map[string]any{"name": name})
	if got := asJSON(sel); !strings.Contains(got, `"kind":"table"`) || !strings.Contains(got, `"row":3,"col":2,"row2":5,"col2":3`) {
		t.Fatalf("sélection %s", got)
	}
	sel = call(t, h, "ui.select_cells", map[string]any{"table": name, "column": "Âge", "row": int64(2)})
	if v := sel.(SelectionView); v.Row != 5 || v.Col != 3 || v.Column != "Âge" {
		t.Fatalf("sélection %+v", v)
	}
	c := call(t, h, "cells.get", map[string]any{"row": int64(5), "col": int64(3)}).(CellValue)
	if c.Value != int64(8) || c.Line != 2 || c.Table != name {
		t.Fatalf("cellule %+v", c)
	}
	call(t, h, "ui.select", map[string]any{"row": int64(10), "col": int64(1), "row2": int64(1)})
	if h.sel.R0 != 0 || h.sel.R1 != 9 {
		t.Fatalf("plage %+v", h.sel)
	}
}

func TestDialog(t *testing.T) {
	h := newHost(t)
	h.answer = map[string]any{"client": "Durand", "montant": "12,5", "paye": true}
	run(t, h, `
local r = ui.dialog{title = "Saisie", fields = {
  {name = "client", required = true},
  {name = "montant", type = "number"},
  {name = "paye", type = "checkbox"},
  {name = "mode", type = "select", options = {"Carte", "Chèque"}},
}}
print(r.client, r.montant, r.paye, r.mode)
print(ui.confirm("Sûr ?"))
`)
	if got := strings.Join(h.prints, ";"); got != "Durand\t12.5\ttrue\tnil;true" {
		t.Fatalf("sortie %q", got)
	}
	if len(h.dialogs) != 2 || h.dialogs[0].Fields[0].Label != "client" || h.dialogs[1].OK != "Oui" {
		t.Fatalf("boîtes %+v", h.dialogs)
	}
	h.answer = nil
	h.prints = nil
	run(t, h, `print(ui.prompt("Nom ?") == nil, ui.confirm("Sûr ?"))`)
	if got := strings.Join(h.prints, ";"); got != "true\tfalse" {
		t.Fatalf("sortie %q", got)
	}
	if res, _ := RunScript(h, "", `ui.dialog{title = "x", fields = {{name = "a", type = "slider"}}}`, ""); !strings.Contains(res.Error, "slider") {
		t.Fatalf("type inconnu accepté : %q", res.Error)
	}
}

func TestButtons(t *testing.T) {
	h := newHost(t)
	call(t, h, "scripts.save", map[string]any{"name": "Actions", "source": `
function bonjour()
  sheets.add("Bonjour")
  print("ok")
end`})
	id := call(t, h, "buttons.create", map[string]any{"row": int64(2), "col": int64(2), "label": "Dire bonjour", "script": "Actions", "function": "bonjour"}).(int64)
	res, err := RunButton(h, id)
	if err != nil || res.Error != "" || len(res.Output) != 1 {
		t.Fatalf("clic %+v %v", res, err)
	}
	if st := call(t, h, "doc.state", nil).(DocState); st.Undo != "Script « Actions »" {
		t.Fatalf("étape %q", st.Undo)
	}
	call(t, h, "buttons.set", map[string]any{"button": "Dire bonjour", "label": "Saluer"})
	call(t, h, "buttons.place", map[string]any{"button": "Saluer", "row": int64(4), "col": int64(1), "width": int64(150)})
	list := call(t, h, "buttons.list", nil).([]ButtonView)
	if len(list) != 1 || list[0].Label != "Saluer" || list[0].Row != 4 || list[0].Function != "bonjour" {
		t.Fatalf("boutons %+v", list)
	}
	call(t, h, "buttons.delete", map[string]any{"button": id})
	call(t, h, "doc.undo", nil)
	if list := call(t, h, "buttons.list", nil).([]ButtonView); len(list) != 1 {
		t.Fatalf("annulation de la suppression : %+v", list)
	}
}

func TestMCPOnlyCommandsHiddenFromLua(t *testing.T) {
	h := newHost(t)
	res, _ := RunScript(h, "", `doc.undo()`, "")
	if res.Error == "" {
		t.Fatal("doc.undo disponible dans un script")
	}
	run(t, h, `assert(db.query("SELECT 1 AS x")[1].x == 1)`) // db.query reste celui de Lua
}

func TestDBQuery(t *testing.T) {
	h := newHost(t)
	call(t, h, "cells.paste", map[string]any{"row": int64(1), "col": int64(1), "values": []any{[]any{"Nom", "Âge"}, []any{"Léa", int64(31)}, []any{"Tom", int64(8)}}})
	name := call(t, h, "sheets.get", nil).(SheetView).Tables[0].Name
	call(t, h, "db.execute", map[string]any{"sql": `UPDATE "` + name + `" SET "Âge" = "Âge" + 1 WHERE Nom = ?`, "params": []any{"Tom"}})
	res := call(t, h, "db.query", map[string]any{"sql": `SELECT Nom, "Âge" FROM "` + name + `" ORDER BY "Âge"`})
	if got := asJSON(res); !strings.Contains(got, `[["Tom",9],["Léa",31]]`) {
		t.Fatalf("requête %s", got)
	}
}

// Chaque exemple de la référence tourne sur le classeur d'exemple de
// docs/scripting-lua.md.
func TestExamplesRun(t *testing.T) {
	for _, c := range Commands() {
		if c.Example == "" || c.NoLua {
			continue
		}
		t.Run(c.Name, func(t *testing.T) { run(t, sample(t), c.Example) })
	}
}

// sample est le classeur d'exemple (tableau Commandes), avec des réponses
// toutes prêtes aux boîtes de dialogue.
func sample(t *testing.T) *fakeHost {
	h := newHost(t)
	call(t, h, "cells.paste", map[string]any{"row": int64(1), "col": int64(1), "values": []any{
		[]any{"Client", "Montant", "Payé", "Échéance"},
		[]any{"Durand", int64(120), true, "01/10/2026"},
		[]any{"Martin", 80.5, false, "20/10/2026"},
		[]any{"Durand", int64(40), false, "05/09/2026"},
	}})
	name := call(t, h, "sheets.get", nil).(SheetView).Tables[0].Name
	call(t, h, "tables.rename", map[string]any{"table": name, "name": "Commandes"})
	h.answer = map[string]any{"client": "Petit", "montant": int64(15), "paye": false, "echeance": "2026-11-02", "value": "Synthèse"}
	return h
}

// Les exemples des sections 5 à 7 du guide tournent aussi.
func TestGuideExamplesRun(t *testing.T) {
	b, err := os.ReadFile("../../docs/scripting-lua.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	doc = doc[strings.Index(doc, "## 5. Piloter"):strings.Index(doc, "## 8. Annuler")]
	blocks := regexp.MustCompile("(?s)```lua\n(.*?)```").FindAllStringSubmatch(doc, -1)
	if len(blocks) < 4 {
		t.Fatalf("%d exemples seulement", len(blocks))
	}
	for i, m := range blocks {
		t.Run(fmt.Sprint(i+1), func(t *testing.T) {
			h := sample(t)
			h.sel = Selection{Sheet: 1, R0: 1, C0: 1, R1: 1, C1: 1}
			run(t, h, m[1])
		})
	}
}
