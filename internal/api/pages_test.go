package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/config"
)

// withHTMLPages active les pages web (features.htmlPages) le temps d'un essai.
func withHTMLPages(t *testing.T) {
	t.Helper()
	old := config.Current()
	config.SetCurrent(config.Resolve(context.Background(), config.Options{},
		&config.TreeSource{N: "défaut", R: config.RankDefault, T: config.Defaults()},
		&config.TreeSource{N: "user", R: config.RankUser, T: config.Tree{"features": config.Tree{"htmlPages": true}}}))
	t.Cleanup(func() { config.SetCurrent(old) })
}

// Pages web coupées (par défaut) : ni création, ni modification, ni appel
// d'une page ; les documents restent, et une page déjà là se lit encore.
func TestPagesDisabledByDefault(t *testing.T) {
	if HTMLPagesConfigured() {
		t.Fatal("pages web activées par défaut")
	}
	h := sample(t)
	ctx := context.Background()
	for _, args := range []map[string]any{nil, {"kind": "html"}, {"name": "P", "source": "<h1>x</h1>"}} {
		if _, err := Invoke(ctx, h, "pages.add", args); !errors.Is(err, ErrPagesDisabled) {
			t.Fatalf("pages.add %v : %v", args, err)
		}
	}
	if _, err := PageCall(ctx, h, "db.query", map[string]any{"sql": "SELECT 1"}); !errors.Is(err, ErrPagesDisabled) {
		t.Fatalf("PageCall : %v", err)
	}
	doc := call(t, h, "pages.add", map[string]any{"kind": "doc", "name": "Notes"}).(sheetRef)
	if doc.Kind != "doc" {
		t.Fatalf("document %+v", doc)
	}
	call(t, h, "pages.set", map[string]any{"sheet": "Notes", "source": `{"ops":[{"insert":"x\n"}]}`})
	// Une page d'un classeur existant : gardée, lisible, pas modifiable.
	id, err := h.d.AddPage(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.d.SetPage(ctx, id, "<h1>avant</h1>"); err != nil {
		t.Fatal(err)
	}
	if _, err := Invoke(ctx, h, "pages.set", map[string]any{"sheet": float64(id), "source": "<h1>après</h1>"}); !errors.Is(err, ErrPagesDisabled) {
		t.Fatalf("pages.set : %v", err)
	}
	if got := call(t, h, "pages.get", map[string]any{"sheet": float64(id)}); got != "<h1>avant</h1>" {
		t.Fatalf("page %q", got)
	}
}

func TestPages(t *testing.T) {
	withHTMLPages(t)
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
	if _, err := Invoke(ctx, h, "pages.get", map[string]any{"sheet": "Feuille1"}); err == nil || !strings.Contains(err.Error(), "page web") {
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
	withHTMLPages(t)
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
	// Liste autorisée : chaque nom est une commande ; fichiers, annulation,
	// code, structure et protections restent à l'utilisateur.
	for name := range pageAllowed {
		if registry[name] == nil {
			t.Errorf("commande autorisée aux pages inconnue : %s", name)
		}
	}
	for _, name := range []string{"doc.save", "doc.undo", "scripts.run", "doc.open",
		"scripts.save", "pages.set", "pages.add", "events.on", "controls.on", "buttons.set", "buttons.create",
		"sheets.delete", "sheets.unprotect", "sheets.protect_workbook", "queries.save", "connections.list", "tables.delete"} {
		if _, err := PageCall(ctx, h, name, nil); err == nil || !strings.Contains(err.Error(), "non disponible") {
			t.Fatalf("%s accepté : %v", name, err)
		}
	}
	if _, err := PageCall(ctx, h, "rien.du.tout", nil); !errors.Is(err, ErrUnknown) {
		t.Fatalf("commande inconnue : %v", err)
	}
}
