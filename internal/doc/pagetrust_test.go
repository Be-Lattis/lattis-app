package doc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/trust"
)

// Une page web créée ou modifiée sur ce poste y est autorisée ; reçue sur un
// autre poste, chaque version demande l'accord, sauf pour un ancien
// classeur de ce poste (repris une fois).
func TestPageVersionsAllowed(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	here := otherMachine(t)
	d, err := New(ctx)
	must(t, err)
	id, err := d.AddPage(ctx, 0)
	must(t, err)
	if !PageAllowed(canvas.PageTemplate) {
		t.Fatal("modèle de page non autorisé")
	}
	const src = `<h1>Ventes</h1><script>lattis.call("sheets.list")</script>`
	must(t, d.SetPage(ctx, id, src))
	if !here.PageAllowed(trust.Hash(src)) || !PageAllowed(src+"") {
		t.Fatal("page écrite ici non autorisée")
	}
	// Un Document n'est pas une page web : rien à autoriser.
	doc, err := d.AddDoc(ctx, 0)
	must(t, err)
	const delta = `{"ops":[{"insert":"x\n"}]}`
	must(t, d.SetPage(ctx, doc, delta))
	if here.PageAllowed(trust.Hash(delta)) {
		t.Fatal("document autorisé comme une page")
	}
	path := filepath.Join(t.TempDir(), "pages.lattis")
	must(t, d.Save(ctx, path))
	must(t, d.Close())

	// Reçu : la page ne s'exécute qu'avec l'accord de l'utilisateur.
	otherMachine(t)
	d, err = Open(ctx, path)
	must(t, err)
	if PageAllowed(src) {
		t.Fatal("page reçue autorisée d'office")
	}
	must(t, d.AllowPage(ctx, id, src))
	if !PageAllowed(src) || PageAllowed(src+" ") {
		t.Fatal("accord d'une version")
	}
	must(t, d.Close())

	// Ancien classeur de ce poste (récent au moment de la mise à jour).
	otherMachine(t, path)
	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	if !PageAllowed(src) {
		t.Fatal("pages d'un ancien classeur de ce poste non reprises")
	}
}
