package collab_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// « Ajouter au classeur partagé » contre un vrai PostgreSQL :
// lattis.create_table sur la version publiée (déclencheurs, droits),
// lignes copiées par INSERT sous la RLS, clé, mise en page ; un éditeur
// reçoit le tableau et ses lignes, les écritures passent dans les deux
// sens ; un participant ne l'ajoute pas (refusé en local), un éditeur non
// plus.
func TestPostgresAddTable(t *testing.T) {
	p := pgSetupClients(t)
	pgHas0025(t, p)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	tbl := newLocalTable(t, alice, "Stock", 10)
	must(t, alice.d.SetKey(ctx, "Stock", 0))
	_, err := alice.e.AddTable(ctx, tbl)
	must(t, err)
	eq(t, "serveur", pgColumns(t, p, "Stock"), []string{"Article:text", "Stock:integer"})
	if r := alice.receive(); r.Applied.Updated+r.Applied.Inserted != 0 {
		t.Fatalf("écho chez Alice : %+v", r.Applied)
	}

	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	eq(t, "Bob Article", values(t, bob, "Stock", "Article"), []string{"Vis", "Écrou"})
	if keyOf(bob, "Stock") != "Article" {
		t.Fatalf("clé chez Bob : %q", keyOf(bob, "Stock"))
	}
	_, err = alice.d.SetCell(ctx, 1, 11, 1, "130")
	must(t, err)
	if r := alice.send(); r.Sent != 1 || r.Rejected != 0 {
		t.Fatalf("envoi d'Alice : %+v", r)
	}
	bob.receive()
	eq(t, "Bob après Alice", values(t, bob, "Stock", "Stock"), []string{"130", "75"})
	_, err = bob.d.SetCell(ctx, 1, 12, 1, "80")
	must(t, err)
	if r := bob.send(); r.Sent != 1 || r.Rejected != 0 {
		t.Fatalf("envoi de Bob : %+v", r)
	}
	alice.receive()
	eq(t, "Alice après Bob", values(t, alice, "Stock", "Stock"), []string{"130", "80"})

	// Le participant le reçoit aussi (lecture, et ses propres lignes).
	_, err = p.dave.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if !hasTable(p.dave, "Stock") {
		t.Fatal("Stock absent chez Dave")
	}
	// Un éditeur n'ajoute pas de tableau.
	if _, err := bob.e.AddTable(ctx, tableOf(t, bob, "Stock")); !errors.Is(err, workbook.ErrAddDesign) {
		t.Fatalf("éditeur : %v", err)
	}
}
