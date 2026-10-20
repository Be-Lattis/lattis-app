package collab_test

import (
	"context"
	"testing"
)

// Convertir en valeurs contre un vrai PostgreSQL : la colonne calculée
// (mise en page) devient une colonne du tableau sur le serveur, remplie ;
// Bob la reçoit à la place de sa colonne calculée ; l'annulation la retire.
func TestPostgresConvertToValues(t *testing.T) {
	p := pgSetupClients(t)
	pgHas0025(t, p)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	_, err := alice.d.SetCell(ctx, 1, 1, 3, "=[@Quantité]*[@Prix]")
	must(t, err)
	must(t, alice.header(3, "Total"))
	alice.syncLayout(false)
	bob.syncLayout(true)
	if formulaOf(t, bob, "Total") == "" {
		t.Fatalf("Bob : pas de colonne calculée Total : %v", colsOf(bob, "Ventes"))
	}
	eq(t, "serveur avant", pgColumns(t, p, "Ventes"), []string{"Produit:text", "Quantité:integer", "Prix:decimal"})

	must(t, alice.d.ColumnsToValues(ctx, "Ventes", []int{3}))
	if r := alice.send(); r.Rejected != 0 {
		t.Fatalf("envoi : %+v", r)
	}
	want := []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Total:decimal"}
	eq(t, "serveur", pgColumns(t, p, "Ventes"), want)
	alice.syncLayout(false)
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	bob.syncLayout(true)
	eq(t, "Bob", colsOf(bob, "Ventes"), want)
	eq(t, "Bob : Total", bob.col("Total"), []string{"13.5", "20.0", "3.25"})
	if formulaOf(t, bob, "Total") != "" {
		t.Fatal("Bob : Total encore calculée")
	}

	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	if r := alice.send(); r.Rejected != 0 {
		t.Fatalf("envoi de l'annulation : %+v", r)
	}
	eq(t, "serveur après l'annulation", pgColumns(t, p, "Ventes"), want[:3])
	alice.syncLayout(false)
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	bob.syncLayout(true)
	if formulaOf(t, bob, "Total") == "" || len(colsOf(bob, "Ventes")) != 4 {
		t.Fatalf("Bob après l'annulation : %v", colsOf(bob, "Ventes"))
	}
}
