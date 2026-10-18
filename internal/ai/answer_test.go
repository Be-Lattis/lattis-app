package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// query_show répond sans modifier le classeur : le résultat va à
// l'utilisateur ; en partage « structure seulement », le modèle n'en voit
// pas les valeurs.
func TestShowResult(t *testing.T) {
	h := newHost(t)
	before := h.changed
	var shown []Result
	ctx := WithShow(context.Background(), func(r Result) { shown = append(shown, r) })
	q := `{"sql":"SELECT \"Client\", sum(\"Montant\") AS total FROM \"Ventes\" GROUP BY 1 ORDER BY 2 DESC LIMIT 1","title":"Meilleur client"}`
	res, isErr := Exec(h, SchemaOnly)(ctx, showTool, json.RawMessage(q))
	if isErr {
		t.Fatal(res)
	}
	if len(shown) != 1 || shown[0].Title != "Meilleur client" || len(shown[0].Rows) != 1 || shown[0].Rows[0][0] == "" {
		t.Fatalf("montré : %+v", shown)
	}
	if best := shown[0].Rows[0][0]; strings.Contains(res, best) || !strings.Contains(res, "1 ligne") {
		t.Fatalf("rendu au modèle : %s", res)
	}
	if h.changed != before {
		t.Fatal("classeur modifié")
	}
	res, _ = Exec(h, Sample)(ctx, showTool, json.RawMessage(q))
	if !strings.Contains(res, shown[0].Rows[0][0]) {
		t.Fatalf("partage par échantillon : %s", res)
	}
	if res, isErr := Exec(h, SchemaOnly)(ctx, showTool, json.RawMessage(`{"sql":"DELETE FROM \"Ventes\""}`)); !isErr {
		t.Fatalf("écriture acceptée : %s", res)
	}
	if s := Structure(context.Background(), h); !strings.Contains(s, "« Ventes »") || !strings.Contains(s, "Montant") || strings.Contains(s, "C1") {
		t.Fatalf("structure : %s", s)
	}
}
