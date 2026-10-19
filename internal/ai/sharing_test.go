package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/doc"
)

// secrets : valeurs du classeur de valuesHost, qui ne doivent jamais
// parvenir au modèle sans partage des données.
var secrets = []string{"Zorglub", "Quaxine", "Brimbelle", "Vrouzac", "Pelmouche", "Tarbouille", "Grunlieu", "Secretissime", "Jeton42"}

// valuesHost : un tableau Clients aux valeurs reconnaissables, un TCD avec
// le champ Nom en colonnes, une cellule libre et une requête web avec un
// jeton dans ses en-têtes et un filtre sur une valeur.
func valuesHost(t *testing.T) *host {
	t.Helper()
	ctx := context.Background()
	d, err := doc.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	h := &host{d: d, sel: api.Selection{Sheet: 1}}
	call := func(name string, args map[string]any) any {
		t.Helper()
		res, err := api.Invoke(ctx, h, name, args)
		if err != nil {
			t.Fatalf("%s : %v", name, err)
		}
		return res
	}
	rows := [][]any{{"Nom", "Ville", "Montant"}}
	for i, n := range secrets[:5] {
		rows = append(rows, []any{n, secrets[5+i%2], float64(i + 1)})
	}
	call("cells.paste", map[string]any{"row": 1, "col": 1, "values": rows})
	call("tables.rename", map[string]any{"table": firstTable(t, h), "name": "Clients"})
	call("cells.set", map[string]any{"row": 20, "col": 10, "value": "Secretissime"})
	call("pivots.create", map[string]any{"config": map[string]any{"name": "TCD1", "source": "Clients", "rowFields": []any{"Ville"}, "cols": "Nom", "values": []any{map[string]any{"field": "Montant", "agg": "sum"}}}})
	call("queries.save", map[string]any{"name": "API",
		"source": map[string]any{"kind": "web", "url": "https://api.exemple.fr/v1?cle=Jeton42", "headers": []any{map[string]any{"name": "X-Api-Key", "value": "Jeton42"}}},
		"steps":  []any{map[string]any{"op": "filter", "conditions": []any{map[string]any{"column": "Nom", "cmp": "=", "value": "Zorglub"}}}}})
	return h
}

func TestToolsFullSharing(t *testing.T) {
	full := toolNames(Tools(Full))
	for _, n := range []string{"tables_list", "tables_rows", "db_query", "cells_read", "sheets_find", "cells_dependencies", profileTool, showTool} {
		if !full[n] {
			t.Errorf("%s absent en partage complet", n)
		}
	}
	for _, n := range []string{"tables_delete", "sheets_delete", "doc_save", "doc_export_csv", "http_get", "scripts_run", "db_execute"} {
		if full[n] {
			t.Errorf("%s proposé", n)
		}
	}
	// sheets_find (positions d'un texte cherché) : à partir de l'échantillon ;
	// cells_dependencies (noms et positions) et les demandes courantes (I8) :
	// dans tous les partages.
	schema := toolNames(Tools(SchemaOnly))
	if schema["sheets_find"] || !toolNames(Tools(Sample))["sheets_find"] {
		t.Error("sheets_find")
	}
	for _, n := range []string{"cells_dependencies", "columns_delete", "charts_delete", "pivots_delete", "tables_duplicates", "tables_remove_duplicates",
		"tables_clear_filters", "cells_note", "sheets_hide_lines", "tables_extract_columns", "sheets_replace", layoutTool} {
		if !schema[n] {
			t.Errorf("%s absent sans partage", n)
		}
	}
	if !strings.Contains(System(Full), "tout le classeur") || strings.Contains(System(SchemaOnly), "tout le classeur") {
		t.Error("consigne")
	}
}

// Sans partage des données, aucune valeur du classeur ne parvient au
// modèle : structure jointe, outils de structure, disposition, requêtes,
// profil, résultat montré à l'utilisateur.
func TestSchemaOnlyLeaksNoValue(t *testing.T) {
	h := valuesHost(t)
	ctx := WithShow(context.Background(), func(Result) {})
	exec := Exec(h, SchemaOnly)
	outs := map[string]string{"structure": Structure(ctx, h, SchemaOnly)}
	calls := map[string]string{
		"tables_list":      `{}`,
		"tables_get":       `{"table":"Clients"}`,
		"sheets_list":      `{}`,
		"sheets_get":       `{}`,
		"queries_list":     `{}`,
		"connections_list": `{}`,
		"buttons_list":     `{}`,
		profileTool:        `{"table":"Clients"}`,
		errorsTool:         `{"table":"Clients"}`,
		showTool:           `{"sql":"SELECT * FROM \"TCD1\""}`,
	}
	for name, in := range calls {
		res, isErr := exec(ctx, name, json.RawMessage(in))
		if isErr {
			t.Fatalf("%s : %s", name, res)
		}
		outs[name] = res
	}
	// La disposition de chaque feuille (celle du TCD comprise).
	for _, s := range []string{"1", "2"} {
		res, _ := exec(ctx, "sheets_get", json.RawMessage(`{"sheet":`+s+`}`))
		outs["sheets_get "+s] = res
	}
	for name, out := range outs {
		for _, s := range secrets {
			if strings.Contains(out, s) {
				t.Errorf("%s transmet « %s » : %s", name, s, out)
			}
		}
	}
	if !strings.Contains(outs["structure"], "colonne(s) de valeurs du champ « Nom »") || !strings.Contains(outs["tables_list"], "valeurs du champ") {
		t.Errorf("TCD : %s", outs["structure"])
	}
	// En partage complet, les mêmes outils voient les valeurs.
	full := execRaw(h, Full)
	for name, want := range map[string]string{"tables_list": "Zorglub", "queries_list": "Jeton42", "sheets_get": "Secretissime"} {
		if res, _ := full(ctx, name, json.RawMessage(`{}`)); !strings.Contains(res, want) {
			t.Errorf("partage complet, %s : %s", name, res)
		}
	}
}

// En sample, la borne de 20 lignes tient sur chaque lecture : group_concat,
// first, cells_read large, sheets_get.
func TestSampleBoundEveryRead(t *testing.T) {
	h := newHost(t)
	ctx := WithShow(context.Background(), func(Result) {})
	exec := execRaw(h, Sample)
	long := strings.Repeat("x", 1000)
	if _, err := api.Invoke(ctx, h, "cells.set", map[string]any{"row": 40, "col": 1, "value": long}); err != nil {
		t.Fatal(err)
	}
	res, _ := exec(ctx, "db_query", json.RawMessage(`{"sql":"SELECT group_concat(\"Client\" || hex(zeroblob(200)), ';') FROM \"Ventes\""}`))
	if len(res) > 1000 {
		t.Errorf("group_concat : %d caractères", len(res))
	}
	res, _ = exec(ctx, "tables_rows", json.RawMessage(`{"table":"Ventes","first":5,"count":100}`))
	var r struct{ Rows [][]any }
	json.Unmarshal([]byte(res), &r)
	if len(r.Rows) > SampleRows {
		t.Errorf("tables_rows : %d lignes", len(r.Rows))
	}
	res, _ = exec(ctx, "cells_read", json.RawMessage(`{"row":1,"col":1,"row2":40,"col2":3}`))
	var cells [][]any
	if json.Unmarshal([]byte(res), &cells) != nil || len(cells) > SampleRows {
		t.Errorf("cells_read : %s", res)
	}
	res, _ = exec(ctx, "sheets_get", json.RawMessage(`{}`))
	if strings.Contains(res, long[:300]) || !strings.Contains(res, "(texte)") {
		t.Errorf("sheets_get : %s", res)
	}
	res, _ = exec(ctx, showTool, json.RawMessage(`{"sql":"SELECT * FROM \"Ventes\""}`))
	var q struct{ Rows [][]any }
	json.Unmarshal([]byte(res), &q)
	if len(q.Rows) != SampleRows {
		t.Errorf("query_show : %d lignes", len(q.Rows))
	}
	// Partage complet : sans borne.
	res, _ = execRaw(h, Full)(ctx, "tables_rows", json.RawMessage(`{"table":"Ventes","count":100}`))
	json.Unmarshal([]byte(res), &r)
	if len(r.Rows) != 30 {
		t.Errorf("partage complet : %d lignes", len(r.Rows))
	}
}
