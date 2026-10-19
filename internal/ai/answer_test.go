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
	// Ni valeur ni nombre de lignes (M1).
	if best := shown[0].Rows[0][0]; strings.Contains(res, best) || strings.Contains(res, "1 ligne") || !strings.Contains(res, "Résultat montré") {
		t.Fatalf("rendu au modèle : %s", res)
	}
	// Nombre de requêtes borné par réponse (M1).
	exec := Exec(h, SchemaOnly)
	for i := range MaxBlindShows + 1 {
		if _, isErr := exec(ctx, showTool, json.RawMessage(q)); isErr != (i == MaxBlindShows) {
			t.Fatalf("requête %d : erreur %v", i+1, isErr)
		}
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
	// Requête plafonnée à la limite de lecture : le total est un minimum (M8).
	shown = nil
	many := `{"sql":"WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM n WHERE x < 10005) SELECT x FROM n"}`
	if res, _ := Exec(h, Full)(ctx, showTool, json.RawMessage(many)); !strings.Contains(res, `"total":"plus de 10000"`) || len(shown) != 1 || !shown[0].More || !shown[0].Truncated {
		t.Fatalf("total plafonné : %.200s / %+v", res, shown)
	}
	if s := Structure(context.Background(), h, SchemaOnly); !strings.Contains(s, "« Ventes »") || !strings.Contains(s, "Montant") || strings.Contains(s, "C1") {
		t.Fatalf("structure : %s", s)
	}
}
