package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/sharing"
)

// secrets : valeurs du classeur de valuesServer.
var secrets = []string{"Zorglub", "Quaxine", "Brimbelle", "Vrouzac", "Pelmouche", "Tarbouille", "Grunlieu", "Secretissime", "Jeton42"}

// valuesServer : un tableau Clients aux valeurs reconnaissables, un TCD
// avec le champ Nom en colonnes, une cellule libre et une requête web avec
// un jeton ; le partage est lu dans *mode.
func valuesServer(t *testing.T, mode *string) *Server {
	t.Helper()
	s := newServer(t)
	s.Sharing = func() string { return *mode }
	ctx := context.Background()
	call := func(name string, args map[string]any) any {
		t.Helper()
		res, err := api.Invoke(ctx, s.Host, name, args)
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
	b, _ := json.Marshal(call("tables.list", nil))
	var l []struct{ Name string }
	json.Unmarshal(b, &l)
	call("tables.rename", map[string]any{"table": l[0].Name, "name": "Clients"})
	call("cells.set", map[string]any{"row": 20, "col": 10, "value": "Secretissime"})
	call("pivots.create", map[string]any{"config": map[string]any{"name": "TCD1", "source": "Clients", "rowFields": []any{"Ville"}, "cols": "Nom", "values": []any{map[string]any{"field": "Montant", "agg": "sum"}}}})
	call("queries.save", map[string]any{"name": "API",
		"source": map[string]any{"kind": "web", "url": "https://api.exemple.fr/v1?cle=Jeton42", "headers": []any{map[string]any{"name": "X-Api-Key", "value": "Jeton42"}}},
		"steps":  []any{map[string]any{"op": "filter", "conditions": []any{map[string]any{"column": "Nom", "cmp": "=", "value": "Zorglub"}}}}})
	return s
}

func toolList(t *testing.T, s *Server) map[string]string {
	t.Helper()
	r := rpc(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	out := map[string]string{}
	for _, x := range r["result"].(map[string]any)["tools"].([]any) {
		m := x.(map[string]any)
		out[m["name"].(string)] = m["description"].(string)
	}
	return out
}

// callTool appelle un outil et rend son texte et son état d'erreur.
func callTool(t *testing.T, s *Server, name, args string) (string, bool) {
	t.Helper()
	r := rpc(t, s, fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, name, args))
	res := r["result"].(map[string]any)
	return res["content"].([]any)[0].(map[string]any)["text"].(string), res["isError"] == true
}

// Le réglage de partage de l'assistant s'applique aux outils MCP.
func TestToolsBySharing(t *testing.T) {
	mode := sharing.SchemaOnly
	s := valuesServer(t, &mode)
	reads := []string{"tables_rows", "db_query", "cells_read", "cells_get", "columns_values", "sheets_find"}
	hidden := []string{"scripts_run", "buttons_run", "controls_fire", "doc_export_csv", "doc_export_xlsx", "queries_preview", "pages_get"}
	always := []string{"tables_list", "sheets_get", "cells_set", "db_execute", "doc_save", "ui_select", "queries_list"}

	tools := toolList(t, s)
	for _, n := range append(append([]string{}, reads...), hidden...) {
		if _, ok := tools[n]; ok {
			t.Errorf("schema-only : %s proposé", n)
		}
	}
	for _, n := range always {
		if _, ok := tools[n]; !ok {
			t.Errorf("schema-only : %s absent", n)
		}
	}
	// Un outil caché est refusé même appelé directement.
	if text, isErr := callTool(t, s, "db_query", `{"sql":"SELECT * FROM \"Clients\""}`); !isErr || strings.Contains(text, "Zorglub") {
		t.Errorf("db_query permis : %s", text)
	}

	mode = sharing.Sample
	tools = toolList(t, s)
	for _, n := range reads {
		if !strings.Contains(tools[n], "Partage limité") {
			t.Errorf("sample : %s absent ou sans borne", n)
		}
	}
	for _, n := range hidden {
		if _, ok := tools[n]; ok {
			t.Errorf("sample : %s proposé", n)
		}
	}

	mode = sharing.Full
	tools = toolList(t, s)
	for _, n := range append(append(reads, hidden...), always...) {
		if _, ok := tools[n]; !ok {
			t.Errorf("partage complet : %s absent", n)
		}
	}
	// Tous, sauf les pages web (coupées par défaut : features.htmlPages).
	all := len(Tools()) - len(api.PageCommands)
	if len(tools) != all {
		t.Errorf("partage complet : %d outils sur %d", len(tools), all)
	}
	for _, n := range []string{"pages_add", "pages_set"} {
		if _, ok := tools[n]; ok {
			t.Errorf("%s proposé, pages web coupées", n)
		}
	}
	// Sans réglage (serveur de test, intégration) : partage complet.
	if len(toolList(t, newServer(t))) != all {
		t.Error("serveur sans réglage")
	}
}

// Sans partage des données, aucune valeur du classeur ne parvient au client
// par les outils proposés ; en sample, chaque lecture est bornée.
func TestSchemaOnlyLeaksNoValue(t *testing.T) {
	mode := sharing.SchemaOnly
	s := valuesServer(t, &mode)
	r := rpc(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"t"}}}`)
	if in := r["result"].(map[string]any)["instructions"].(string); !strings.Contains(in, "Structure seulement") {
		t.Errorf("instructions : %s", in)
	}
	calls := map[string]string{
		"tables_list": `{}`, "tables_get": `{"table":"Clients"}`, "sheets_list": `{}`, "sheets_get": `{}`,
		"sheets_get 2": `{"sheet":2}`, "queries_list": `{}`, "connections_list": `{}`, "controls_list": `{}`,
		"tables_key_check": `{"table":"Clients","column":"Nom"}`, "tables_duplicates": `{"table":"Clients"}`,
	}
	for name, args := range calls {
		tool, _, _ := strings.Cut(name, " ")
		text, isErr := callTool(t, s, tool, args)
		if isErr {
			t.Fatalf("%s : %s", name, text)
		}
		for _, v := range secrets {
			if strings.Contains(text, v) {
				t.Errorf("%s transmet « %s » : %s", name, v, text)
			}
		}
	}
	if text, _ := callTool(t, s, "tables_list", `{}`); !strings.Contains(text, "valeurs du champ « Nom »") {
		t.Errorf("TCD : %s", text)
	}

	mode = sharing.Sample
	text, _ := callTool(t, s, "db_query", `{"sql":"SELECT group_concat(\"Nom\" || hex(zeroblob(500)), ';') FROM \"Clients\""}`)
	if len(text) > 1000 || !strings.Contains(text, "Zorglub") {
		t.Errorf("sample, group_concat : %d caractères", len(text))
	}
	if text, _ := callTool(t, s, "sheets_get", `{}`); strings.Contains(text, "Secretissime") {
		t.Errorf("sample, sheets_get : %s", text)
	}
	if text, _ := callTool(t, s, "queries_list", `{}`); strings.Contains(text, "Jeton42") {
		t.Errorf("sample, en-têtes : %s", text)
	}

	mode = sharing.Full
	for name, want := range map[string]string{"tables_list": "Zorglub", "sheets_get": "Secretissime", "queries_list": "Jeton42"} {
		if text, _ := callTool(t, s, name, `{}`); !strings.Contains(text, want) {
			t.Errorf("partage complet, %s : %s", name, text)
		}
	}
}

// « Demander avant de modifier » couvre aussi les fichiers écrits :
// enregistrer sous, exports, import.
func TestWritesFiles(t *testing.T) {
	cases := map[string]struct {
		args map[string]any
		want bool
	}{
		"doc.save":        {nil, false},
		"doc.export_csv":  {map[string]any{"path": "x.csv"}, true},
		"doc.export_xlsx": {map[string]any{"path": "x.xlsx"}, true},
		"doc.export_ods":  {map[string]any{"path": "x.ods"}, true},
		"doc.import":      {map[string]any{"path": "x.csv"}, true},
		"doc.state":       {nil, false},
	}
	for name, c := range cases {
		if got := Writes(api.Lookup(name), c.args); got != c.want {
			t.Errorf("%s : %v", name, got)
		}
	}
	if !Writes(api.Lookup("doc.save"), map[string]any{"path": `C:\x.lattis`}) {
		t.Error("doc.save avec path : pas une écriture")
	}
}
