package sharing

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/query"
)

// Chaque commande de lecture du serveur MCP est rangée : structure,
// lecture de valeurs, ou sans valeur du classeur. Une commande nouvelle
// doit être classée ici avant d'être proposée hors partage complet.
func TestEveryReadClassified(t *testing.T) {
	neutral := []string{
		"cells.dependencies", "controls.get", "controls.list", "controls.listeners",
		"doc.new", "doc.open", "doc.redo", "doc.save", "doc.state", "doc.undo",
		"events.enable", "events.enabled", "events.list", "http.get", "http.post", "http.request",
		"scripts.check", "scripts.list", "tables.duplicates",
		"ui.alert", "ui.confirm", "ui.dialog", "ui.notify", "ui.prompt", "ui.select", "ui.select_cells",
		"ui.select_object", "ui.selection", "ui.show_sheet", "ui.show_view",
	}
	for _, c := range api.Commands() {
		if c.Write || c.NoMCP {
			continue
		}
		if !slices.Contains(Structure, c.Name) && !values[c.Name] && !slices.Contains(neutral, c.Name) {
			t.Errorf("%s : lecture non classée (Structure, values ou neutre)", c.Name)
		}
	}
	for _, list := range [][]string{Structure, Reads, FullReads} {
		for _, n := range list {
			if api.Lookup(n) == nil {
				t.Errorf("commande inconnue : %s", n)
			}
		}
	}
	for n := range values {
		if api.Lookup(n) == nil {
			t.Errorf("commande inconnue : %s", n)
		}
	}
}

func TestExposed(t *testing.T) {
	for _, c := range []string{"tables.rows", "db.query", "cells.read", "scripts.run", "doc.export_csv", "sheets.find", "pages.get"} {
		if Exposed(c, SchemaOnly) {
			t.Errorf("%s proposé sans partage des données", c)
		}
		if !Exposed(c, Full) {
			t.Errorf("%s absent en partage complet", c)
		}
	}
	if !Exposed("tables.rows", Sample) || Exposed("scripts.run", Sample) || Exposed("doc.export_xlsx", Sample) {
		t.Error("mode sample : lectures bornées seulement")
	}
	for _, c := range []string{"tables.list", "sheets.get", "cells.set", "db.execute", "doc.save", "ui.select"} {
		if !Exposed(c, SchemaOnly) {
			t.Errorf("%s absent sans partage des données", c)
		}
	}
	if Mode("") != SchemaOnly || Mode("tout") != SchemaOnly || Mode(Full) != Full {
		t.Error("Mode")
	}
}

// Hors partage complet, sheets.get ne rend que la disposition : positions
// et nature des cellules, pas leur contenu.
func TestLayoutOnly(t *testing.T) {
	src := api.SheetView{
		Cells:  []api.CellView{{Row: 1, Col: 1, Raw: "Salaire de Léa"}, {Row: 2, Col: 1, Raw: "=SOMME(A1:A3)", Value: "42"}, {Row: 3, Col: 1, Raw: "1 200,5"}},
		Pivots: []api.ObjectView{{Name: "TCD", Row: 5, Col: 1, Config: map[string]any{"filters": []string{"Léa"}}}},
	}
	for _, mode := range []string{SchemaOnly, Sample} {
		v := Filter("sheets.get", src, mode).(api.SheetView)
		got := []string{v.Cells[0].Raw, v.Cells[1].Raw, v.Cells[2].Raw}
		if strings.Join(got, "|") != "(texte)|(formule)|(nombre)" || v.Cells[1].Value != "" || v.Pivots[0].Config != nil || v.Pivots[0].Row != 5 {
			t.Fatalf("%s : disposition %+v", mode, v)
		}
	}
	if src.Cells[0].Raw != "Salaire de Léa" || src.Pivots[0].Config == nil {
		t.Fatal("résultat d'origine modifié")
	}
	if v := Filter("sheets.get", src, Full).(api.SheetView); v.Cells[0].Raw != "Salaire de Léa" {
		t.Fatal("partage complet : contenu retiré")
	}
}

// Sans partage des données, les colonnes d'un TCD tirées des valeurs de son
// champ en colonnes sont résumées.
func TestPivotColumns(t *testing.T) {
	list := []api.TableEntry{{Name: "TCD1", Kind: "pivot", Columns: []string{"Pays", "Dupont SA", "Martin SARL", "Total"}, Types: []string{"text", "decimal", "decimal", "decimal"}, ColsField: "Client", ColsFrom: 2}}
	got := Filter("tables.list", list, SchemaOnly).([]api.TableEntry)
	if s := strings.Join(got[0].Columns, "|"); s != "Pays|3 colonne(s) de valeurs du champ « Client »" || len(got[0].Types) != 2 {
		t.Fatalf("colonnes : %s %v", s, got[0].Types)
	}
	if list[0].Columns[1] != "Dupont SA" {
		t.Fatal("résultat d'origine modifié")
	}
	if got := Filter("tables.list", list, Sample).([]api.TableEntry); got[0].Columns[1] != "Dupont SA" {
		t.Fatal("mode sample : colonnes masquées")
	}
	if h := ValueColumns(list); !h["dupont sa"] || h["pays"] {
		t.Fatalf("ValueColumns : %v", h)
	}
}

// queries.list : en-têtes HTTP et paramètres d'adresse masqués hors partage
// complet, littéraux des étapes sans partage des données.
func TestMaskQueries(t *testing.T) {
	qs := []query.Query{{Name: "API", Source: query.Source{Kind: "web", URL: "https://api.exemple.fr/v1?cle=Jeton42&x=1", Headers: []query.Header{{Name: "X-Api-Key", Value: "Jeton42"}}},
		Steps: []query.Step{{Op: "filter", Conditions: []query.Cond{{Column: "Client", Cmp: "=", Value: "Dupont"}}}, {Op: "replace", Column: "Ville", Value: "Pariss", With: "Paris"}}}}
	b, _ := json.Marshal(Filter("queries.list", qs, SchemaOnly))
	for _, s := range []string{"Jeton42", "Dupont", "Pariss", "Paris\""} {
		if strings.Contains(string(b), s) {
			t.Errorf("%s transmis : %s", s, b)
		}
	}
	if !strings.Contains(string(b), "X-Api-Key") || !strings.Contains(string(b), "cle=") {
		t.Errorf("structure perdue : %s", b)
	}
	b, _ = json.Marshal(Filter("queries.list", qs, Sample))
	if strings.Contains(string(b), "Jeton42") || !strings.Contains(string(b), "Dupont") {
		t.Errorf("sample : %s", b)
	}
	if qs[0].Source.Headers[0].Value != "Jeton42" {
		t.Fatal("résultat d'origine modifié")
	}
}

// En sample, chaque lecture est bornée : lignes, valeurs longues
// (group_concat), taille totale.
func TestEncodeLimits(t *testing.T) {
	long := strings.Repeat("a@b.fr;", 2000)
	out := Encode(map[string]any{"columns": []string{"x"}, "rows": [][]any{{long}}})
	if len(out) > SampleBytes || strings.Contains(out, long[:SampleText+10]) {
		t.Fatalf("valeur longue : %d octets", len(out))
	}
	var rows [][]any
	for range 500 {
		row := make([]any, 400)
		for j := range row {
			row[j] = "valeur"
		}
		rows = append(rows, row)
	}
	out = Encode(rows) // cells.read
	if len(out) > SampleBytes+200 || !strings.Contains(out, "tronqué") {
		t.Fatalf("plage large : %d octets", len(out))
	}
	args := Args("cells.read", map[string]any{"row": 1.0, "col": 1.0, "row2": 500.0, "col2": 3.0}, Sample)
	if args["row2"] != float64(SampleRows) {
		t.Fatalf("cells.read : %v", args)
	}
	if args := Args("tables.rows", nil, Sample); args["count"] != float64(SampleRows) {
		t.Fatalf("tables.rows : %v", args)
	}
	if args := Args("tables.rows", map[string]any{"count": 1000.0}, Full); args["count"] != 1000.0 {
		t.Fatal("partage complet borné")
	}
}
