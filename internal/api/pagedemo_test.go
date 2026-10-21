package api

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/demo"
	"github.com/lalamefine/lattis/internal/doc"
)

// La page « Tableau de bord » de la démonstration (preset med) fonctionne
// sans scripting (version web, démo publique) : chacune de ses lectures
// (lattis.query d'une requête favorite, lattis.sql) passe en lecture seule.
func TestDemoDashboardWithoutScripting(t *testing.T) {
	ctx := context.Background()
	d, _, err := demo.New(ctx, "med", demo.Options{Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	h := &fakeHost{d: d}
	src, err := PageReadCall(ctx, h, "pages.get", map[string]any{"sheet": "Tableau de bord"})
	if err != nil {
		t.Fatal(err)
	}
	page := src.(string)
	// Fabriquée ici, la page s'affiche sans demander l'accord (comme les scripts).
	if !doc.PageAllowed(page) {
		t.Fatal("page de la démonstration soumise à l'accord")
	}
	queries := regexp.MustCompile(`lattis\.query\("([^"]+)"\)`).FindAllStringSubmatch(page, -1)
	sqls := regexp.MustCompile(`lattis\.sql\('([^']+)', \[(\d+)\]\)`).FindAllStringSubmatch(page, -1)
	if len(queries) == 0 || len(sqls) == 0 || strings.Contains(page, "lattis.call(") {
		t.Fatalf("lectures de la page : %v %v", queries, sqls)
	}
	for _, q := range queries {
		res, err := PageReadCall(ctx, h, "favorites.run", map[string]any{"name": q[1]})
		if err != nil {
			t.Fatalf("lattis.query(%q) : %v", q[1], err)
		}
		if got := asJSON(res); !strings.Contains(got, `"columns":["ca","nb","panier","paye"]`) {
			t.Fatalf("%s : %s", q[1], got)
		}
	}
	for _, q := range sqls {
		res, err := PageReadCall(ctx, h, "db.query", map[string]any{"sql": q[1], "params": []any{float64(8)}}) // nombre JSON de la page
		if err != nil {
			t.Fatalf("lattis.sql(%q) : %v", q[1], err)
		}
		if got := asJSON(res); !strings.Contains(got, `"columns":["Ville","Région","ca"]`) {
			t.Fatalf("%s : %s", q[1], got)
		}
	}
}
