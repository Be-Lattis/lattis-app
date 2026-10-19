package collab_test

import (
	"context"
	"testing"
)

// Onglet à choix contre un vrai PostgreSQL : la catégorie posée par la
// propriétaire après la publication part dans la mise en page (save_meta,
// jsonb opaque) et arrive chez l'éditeur ; celle de l'éditeur reste sur son
// poste (save_meta refusé, sans erreur).
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
	must(t, bob.d.Model.SetSheetChoice(ctx, 1, "Ventes", "Quantité", "3"))
	must(t, bob.e.PublishChoices(ctx, true))
	_, err = alice.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	wantTab(t, "Alice", alice, "Produit", "Café")
	wantTab(t, "Bob, sa catégorie", bob, "Quantité", "3")
}
