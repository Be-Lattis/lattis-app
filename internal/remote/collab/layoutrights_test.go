package collab_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Cellules libres et mise en page d'un classeur collaboratif : aux
// propriétaires et concepteurs (décision 0018). Un éditeur est refusé en
// local, tout de suite, avec le même message ; la saisie dans les tableaux
// reste permise selon ses droits.
func TestLayoutReservedToDesigners(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	refused := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, workbook.ErrLayout) {
			t.Fatalf("%s par un éditeur : %v", what, err)
		}
	}
	_, err := bob.d.SetCell(ctx, 1, 10, 5, "test")
	refused("cellule libre", err)
	_, err = bob.d.SetCell(ctx, 1, 10, 5, "=1+1")
	refused("formule libre", err)
	refused("largeur", bob.d.Model.SetColumnWidth(ctx, 1, 5, 120))
	_, err = bob.d.Model.AddSheet(ctx)
	refused("feuille ajoutée", err)
	refused("feuille renommée", bob.d.Model.RenameSheet(ctx, 1, "Bob"))
	refused("feuille supprimée", bob.d.Model.DeleteSheet(ctx, 1))
	// Saisie dans le tableau : permise, et partie au serveur.
	bob.mustSet(0, 1, "7")
	bob.send()
	alice.receive()
	eq(t, "Alice", alice.col("Quantité"), []string{"7", "10", "1"})
	// Propriétaire : permis.
	_, err = alice.d.SetCell(ctx, 1, 10, 5, "test")
	must(t, err)
	// Devenu concepteur (droits relus), Bob modifie la mise en page.
	must(t, f.Grant(id, "bob", "designer"))
	_, err = workbook.RefreshCapabilities(ctx, f.As("bob"), bob.d.Share)
	must(t, err)
	_, err = bob.d.SetCell(ctx, 1, 10, 6, "de Bob")
	must(t, err)
}
