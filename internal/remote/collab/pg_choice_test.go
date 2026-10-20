package collab_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Onglet à choix contre un vrai PostgreSQL : la catégorie posée par la
// propriétaire après la publication part dans la mise en page (save_meta,
// jsonb opaque) et arrive chez l'éditeur, qui ne la change pas (mise en
// page aux concepteurs, décision 0018).
func TestPostgresChoiceTab(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	must(t, alice.d.Model.SetSheetChoice(ctx, 1, "Ventes", "Produit", "Café"))
	must(t, alice.e.PublishChoices(ctx, false))
	rep, err := bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if !rep.Choices {
		t.Fatal("catégorie d'Alice pas reçue par Bob")
	}
	wantTab(t, "Bob", bob, "Produit", "Chocolat")
	must(t, bob.d.Model.ChooseSheetValue(ctx, 1, "Thé", false))
	// La catégorie est de la mise en page : refusée en local à l'éditeur
	// (décision 0018) ; sa valeur choisie lui reste.
	if err := bob.d.Model.SetSheetChoice(ctx, 1, "Ventes", "Quantité", "3"); !errors.Is(err, workbook.ErrLayout) {
		t.Fatalf("catégorie posée par un éditeur : %v", err)
	}
	must(t, bob.e.PublishChoices(ctx, true))
	_, err = alice.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	wantTab(t, "Alice", alice, "Produit", "Café")
	wantTab(t, "Bob, sa valeur", bob, "Produit", "Thé")
}

// Mise en page en direct contre un vrai PostgreSQL (jsonb : nombres et
// clés réécrits par le serveur) : cellule libre, formule et largeur de la
// propriétaire reçues par l'éditeur ; rien ne revient en écho.
func TestPostgresLayoutLive(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	_, err := alice.d.SetCell(ctx, 1, 10, 5, "test")
	must(t, err)
	_, err = alice.d.SetCell(ctx, 1, 11, 5, "=2*21")
	must(t, err)
	must(t, alice.d.Model.SetColumnWidth(ctx, 1, 5, 140))
	alice.syncLayout(false)
	if rep := alice.syncLayout(true); rep.Changed() {
		t.Fatal("écho chez Alice")
	}
	bob.syncLayout(true)
	wantFree(t, "Bob", bob, 1, 10, 5, "test", "test")
	wantFree(t, "Bob", bob, 1, 11, 5, "=2*21", "42")
	if w := width(t, bob, 1, 5); w != 140 {
		t.Fatalf("largeur chez Bob : %d", w)
	}
	if rep := bob.syncLayout(true); rep.Changed() {
		t.Fatal("reçu deux fois")
	}
}
