package api

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPages(t *testing.T) {
	h := sample(t)
	ctx := context.Background()
	run(t, h, `
local p = pages.add("Tableau de bord", "<h1>Bonjour</h1>")
print(p.name, p.kind)
pages.set("Tableau de bord", pages.get("Tableau de bord") .. "<p>suite</p>")
`)
	if got := strings.Join(h.prints, ";"); got != "Tableau de bord\thtml" {
		t.Fatalf("sortie %q", got)
	}
	if got := asJSON(call(t, h, "sheets.list", nil)); !strings.Contains(got, `"name":"Tableau de bord","kind":"html"`) || strings.Contains(got, `"Feuille1","kind"`) {
		t.Fatalf("feuilles %s", got)
	}
	if got := call(t, h, "pages.get", map[string]any{"sheet": "Tableau de bord"}); got != "<h1>Bonjour</h1><p>suite</p>" {
		t.Fatalf("page %q", got)
	}
	if _, err := Invoke(ctx, h, "pages.get", map[string]any{"sheet": "Feuille1"}); err == nil || !strings.Contains(err.Error(), "page HTML") {
		t.Fatalf("feuille lue comme page : %v", err)
	}

	// Modifier la page s'annule.
	call(t, h, "pages.set", map[string]any{"sheet": "Tableau de bord", "source": "<p>v2</p>"})
	call(t, h, "doc.undo", nil)
	if got := call(t, h, "pages.get", map[string]any{"sheet": "Tableau de bord"}); got != "<h1>Bonjour</h1><p>suite</p>" {
		t.Fatalf("après annulation %q", got)
	}

	// Copier l'onglet emporte la page ; le supprimer puis annuler la rend.
	cp := call(t, h, "sheets.copy", map[string]any{"sheet": "Tableau de bord"}).(sheetRef)
	if cp.Kind != "html" {
		t.Fatalf("copie %+v", cp)
	}
	call(t, h, "sheets.delete", map[string]any{"sheet": "Tableau de bord"})
	if _, err := Invoke(ctx, h, "pages.get", map[string]any{"sheet": "Tableau de bord"}); err == nil {
		t.Fatal("page supprimée encore lisible")
	}
	call(t, h, "doc.undo", nil)
	if got := call(t, h, "pages.get", map[string]any{"sheet": "Tableau de bord"}); got != "<h1>Bonjour</h1><p>suite</p>" {
		t.Fatalf("après annulation de la suppression %q", got)
	}

	// Tout le script s'annule d'un coup : plus de page.
	for range 3 {
		call(t, h, "doc.undo", nil)
	}
	if got := asJSON(call(t, h, "sheets.list", nil)); strings.Contains(got, "html") {
		t.Fatalf("après annulation du script : %s", got)
	}
}

// Le pont des pages HTML passe par les commandes de l'API.
func TestPageCall(t *testing.T) {
	h := sample(t)
	ctx := context.Background()
	res, err := PageCall(ctx, h, "db.query", map[string]any{"sql": `SELECT sum(Montant) FROM Commandes`})
	if err != nil {
		t.Fatal(err)
	}
	if got := asJSON(res); !strings.Contains(got, `"rows":[[240.5]]`) {
		t.Fatalf("db.query %s", got)
	}
	// Une écriture forme une étape d'annulation.
	if _, err := PageCall(ctx, h, "cells.set", map[string]any{"row": float64(10), "col": float64(1), "value": "note"}); err != nil {
		t.Fatal(err)
	}
	if st := call(t, h, "doc.state", nil).(DocState); st.Undo == "" {
		t.Fatalf("écriture d'une page sans étape d'annulation : %+v", st)
	}
	if _, err := PageCall(ctx, h, "db.execute", map[string]any{"sql": `UPDATE Commandes SET Montant = 0`}); err != nil {
		t.Fatal(err)
	}
	// Fichiers, annulation et scripts restent à l'utilisateur.
	for _, name := range []string{"doc.save", "doc.undo", "scripts.run", "doc.open"} {
		if _, err := PageCall(ctx, h, name, nil); err == nil || !strings.Contains(err.Error(), "non disponible") {
			t.Fatalf("%s accepté : %v", name, err)
		}
	}
	if _, err := PageCall(ctx, h, "rien.du.tout", nil); !errors.Is(err, ErrUnknown) {
		t.Fatalf("commande inconnue : %v", err)
	}
}
