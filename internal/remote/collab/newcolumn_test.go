package collab_test

import (
	"context"
	"testing"
)

// Valeurs saisies ou collées dans une colonne vide d'un tableau partagé, qui
// en changent le type dans la même étape (la colonne prend le type de sa
// première valeur) : la conversion part au serveur, puis les valeurs, au
// nouveau type. Auparavant, seule la conversion partait.

// newColumnScenario : alice conçoit la structure (propriétaire), bob la
// reçoit.
func newColumnScenario(t *testing.T, alice, bob *client) {
	ctx := context.Background()
	follow := func(what string) {
		t.Helper()
		if r := alice.send(); r.Rejected != 0 || r.Conflicts != 0 {
			t.Fatalf("%s : envoi %+v", what, r)
		}
		if n := alice.states()["pending"]; n != 0 {
			t.Fatalf("%s : file %v", what, alice.states())
		}
		_, err := bob.e.ReceiveStructure(ctx, nil, nil)
		must(t, err)
		bob.receive()
	}
	types := func(what string, c *client, name, local, remote string) {
		t.Helper()
		if got := localType(t, c.d, name); got != local {
			t.Fatalf("%s : type local de %s %q, attendu %q", what, name, got, local)
		}
		if got := remoteTypes(t, c)[name]; got != remote {
			t.Fatalf("%s : type serveur de %s %q, attendu %q", what, name, got, remote)
		}
	}

	// Nombre tapé dans une nouvelle colonne (texte, vide) : entière.
	must(t, alice.d.InsertColumn(ctx, "Ventes", 3, "Lot"))
	follow("colonne Lot")
	alice.mustSet(0, 3, "12")
	follow("saisie de 12")
	types("saisie", alice, "Lot", "integer", "integer")
	types("Bob", bob, "Lot", "integer", "integer")
	eq(t, "Bob Lot", bob.col("Lot"), []string{"12", "", ""})

	// Annulation de l'étape : la colonne redevient du texte vide pour tous ;
	// rétablissement : 12 revient.
	_, err := alice.d.Bus.Undo(ctx)
	must(t, err)
	if got := localType(t, alice.d, "Lot"); got != "text" {
		t.Fatalf("annulation : type local de Lot %q", got)
	}
	follow("annulation")
	types("Bob annulation", bob, "Lot", "text", "text")
	eq(t, "Bob Lot annulé", bob.col("Lot"), []string{"", "", ""})
	_, err = alice.d.Bus.Redo(ctx)
	must(t, err)
	follow("rétablissement")
	types("Bob rétablissement", bob, "Lot", "integer", "integer")
	eq(t, "Bob Lot rétabli", bob.col("Lot"), []string{"12", "", ""})
	bob.mustSet(1, 3, "7")
	if r := bob.send(); r.Sent != 1 || r.Rejected != 0 {
		t.Fatalf("Bob Lot : %+v", r)
	}
	alice.receive()
	eq(t, "Alice Lot", alice.col("Lot"), []string{"12", "7", ""})

	// Bloc collé dans une colonne vide, avec une ligne de plus que le
	// tableau : décimale, la ligne ajoutée a sa valeur.
	must(t, alice.d.InsertColumn(ctx, "Ventes", 4, "Remise"))
	follow("colonne Remise")
	ev, err := alice.d.Paste(ctx, 1, 1, 4, [][]string{{"1,5"}, {"2"}, {"3,25"}, {"4"}})
	must(t, err)
	if ev.RefusedCount != 0 {
		t.Fatalf("collage : %+v", ev)
	}
	follow("collage")
	types("collage", alice, "Remise", "decimal", "decimal")
	types("Bob collage", bob, "Remise", "decimal", "decimal")
	eq(t, "Bob Remise", bob.col("Remise"), []string{"1.5", "2.0", "3.25", "4.0"})
	eq(t, "Bob Lot (ligne ajoutée)", bob.col("Lot"), []string{"12", "7", "", ""})

	// Date tapée dans une colonne vide.
	must(t, alice.d.InsertColumn(ctx, "Ventes", 5, "Livraison"))
	follow("colonne Livraison")
	alice.mustSet(1, 5, "23/10/2026")
	follow("date")
	types("Bob date", bob, "Livraison", "date", "date")
	eq(t, "Bob Livraison", bob.col("Livraison"), []string{"", "2026-10-23", "", ""})

	// Date avec heure tapée dans une colonne vide : datetime sur le serveur.
	must(t, alice.d.InsertColumn(ctx, "Ventes", 6, "Départ"))
	follow("colonne Départ")
	alice.mustSet(2, 6, "24/10/2026 14:30")
	follow("date et heure")
	types("Bob date et heure", bob, "Départ", "date", "datetime")
	eq(t, "Bob Départ", bob.col("Départ"), []string{"", "", "2026-10-24 14:30", ""})
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	follow("annulation de la date et heure")
	types("Bob annulation date et heure", bob, "Départ", "text", "text")
	eq(t, "Bob Départ annulé", bob.col("Départ"), []string{"", "", "", ""})
	_, err = alice.d.Bus.Redo(ctx)
	must(t, err)
	follow("rétablissement de la date et heure")
	types("Bob rétablissement date et heure", bob, "Départ", "date", "datetime")
	eq(t, "Bob Départ rétabli", bob.col("Départ"), []string{"", "", "2026-10-24 14:30", ""})

	// Hors ligne : la colonne et sa première valeur attendent dans la file,
	// partent dans l'ordre à la reconnexion.
	alice.d.Share.SetOnline(false, "serveur injoignable")
	must(t, alice.d.InsertColumn(ctx, "Ventes", 7, "Stock"))
	alice.mustSet(0, 7, "40")
	alice.mustSet(3, 7, "41")
	if n := alice.states()["pending"]; n < 3 {
		t.Fatalf("file hors ligne : %v", alice.states())
	}
	alice.d.Share.SetOnline(true, "")
	follow("reconnexion")
	types("Bob hors ligne", bob, "Stock", "integer", "integer")
	eq(t, "Bob Stock", bob.col("Stock"), []string{"40", "", "", "41"})
}

func TestNewColumnTypedByEntry(t *testing.T) {
	_, alice, bob, _ := setup(t)
	newColumnScenario(t, alice, bob)
}

func TestPostgresNewColumnTypedByEntry(t *testing.T) {
	p := pgSetupClients(t)
	pgHas0025(t, p)
	newColumnScenario(t, p.alice, p.bob)
}
