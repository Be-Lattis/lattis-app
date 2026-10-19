package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
)

// sheets_layout, permis sans partage des données, montre de quoi réparer
// une feuille (titre, total, nature des cellules) sans aucune valeur (I4).
func TestSheetsLayoutWithoutValues(t *testing.T) {
	h := newHost(t)
	ctx := context.Background()
	steps := []struct {
		cmd  string
		args map[string]any
	}{
		{"sheets.add", map[string]any{"name": "Brut"}},
		{"cells.paste", map[string]any{"sheet": "Brut", "row": 4, "col": 1, "values": [][]any{
			{"Client", "Montant"}, {"Dupontel", 12.5}, {"Martinez", 30.0}, {"Total général", 42.5},
		}}},
		{"cells.set", map[string]any{"sheet": "Brut", "row": 1, "col": 1, "value": "Rapport annuel"}},
		{"cells.style", map[string]any{"sheet": "Brut", "row": 1, "col": 1, "style": map[string]any{"bold": true}}},
	}
	for _, s := range steps {
		if _, err := api.Invoke(ctx, h, s.cmd, s.args); err != nil {
			t.Fatalf("%s : %v", s.cmd, err)
		}
	}
	tools := map[string]bool{}
	for _, tl := range Tools(SchemaOnly) {
		tools[tl.Name] = true
	}
	if !tools[layoutTool] || tools["tables_rows"] {
		t.Fatal("sheets_layout absent, ou lecture de valeurs proposée sans partage")
	}
	res, isErr := Exec(h, SchemaOnly)(ctx, layoutTool, json.RawMessage(`{"sheet":"Brut"}`))
	if isErr {
		t.Fatal(res)
	}
	for _, v := range []string{"Rapport", "Dupontel", "Martinez", "général", "12", "42"} {
		if strings.Contains(res, v) {
			t.Fatalf("valeur %q transmise : %s", v, res)
		}
	}
	var out struct {
		Tables []layoutTable `json:"tables"`
		Cells  []layoutCell  `json:"cells"`
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(res, dataOpen+"\n"), "\n"+dataClose)
	if err := json.Unmarshal([]byte(inner), &out); err != nil {
		t.Fatal(err, res)
	}
	// La ligne de total reste hors du tableau (détection), en cellules libres.
	if len(out.Tables) != 1 || out.Tables[0].Row != 4 || out.Tables[0].Rows != 2 {
		t.Fatalf("tableaux : %+v", out.Tables)
	}
	if k := out.Tables[0].Last; len(k) != 2 || k[0] != "texte" || k[1] != "nombre" {
		t.Fatalf("dernière ligne : %v", k)
	}
	hints := map[int]string{}
	for _, c := range out.Cells {
		if c.Col == 1 {
			hints[c.Row] = c.Hint
		}
		if c.Row == 1 && (!c.Bold || c.Kind != "texte") {
			t.Fatalf("titre : %+v", c)
		}
	}
	if hints[1] != "titre" || hints[7] != "total" {
		t.Fatalf("cellules : %+v", out.Cells)
	}
	for row, want := range map[string]bool{"Sous-total": true, "TOTAL": true, "Somme des ventes": true, "Paris": false, "Totalement": false} {
		if got := rowLooksTotal([]any{nil, row, 3.0}); got != want {
			t.Errorf("%q : total %v", row, got)
		}
	}
}

func TestKindOf(t *testing.T) {
	for v, want := range map[any]string{
		nil: "vide", "": "vide", 3.0: "nombre", "1 234,50 €": "nombre", "12/03/2026": "date", "=A1": "formule",
		"#DIV/0!": "erreur", "#NOM?": "erreur", "VRAI": "booléen", true: "booléen", "Paris": "texte",
	} {
		if got := kindOf(v); got != want {
			t.Errorf("%v : %s, attendu %s", v, got, want)
		}
	}
}
