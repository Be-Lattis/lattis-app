package collab_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// Retours du serveur sur la synchronisation (migration 0008 de
// be-lattis-server, docs/collaboration/synchronisation.md §4) : échos
// écartés, refus d'une seule opération, point de reprise non couvert,
// clés d'ordre distinctes.

func TestEchoesSkipped(t *testing.T) {
	f, _, bob, _ := setup(t)
	bob.mustSet(0, 1, "7")
	if r := bob.send(); r.Sent != 1 {
		t.Fatalf("envoi : %+v", r)
	}
	before := f.Calls["changes_rows"]
	if r := bob.receive(); r.Changed() {
		t.Fatalf("réception : %+v", r)
	}
	if n := f.Calls["changes_rows"] - before; n != 0 {
		t.Fatalf("%d ligne(s) renvoyée(s) au poste qui les a écrites", n)
	}
	// Serveur sans la migration 0008 : l'écho revient, sans effet.
	f.Legacy = true
	bob.mustSet(0, 1, "8")
	bob.send()
	before = f.Calls["changes_rows"]
	if r := bob.receive(); r.Changed() {
		t.Fatalf("réception (serveur ancien) : %+v", r)
	}
	if f.Calls["changes_rows"] == before {
		t.Fatal("serveur ancien : l'écho devait revenir")
	}
	eq(t, "Bob", bob.col("Quantité"), []string{"8", "10", "1"})
}

func TestForeignChangeUnderOwnWrite(t *testing.T) {
	_, alice, bob, _ := setup(t)
	alice.mustSet(0, 1, "5")
	alice.send()
	// Bob écrit la même ligne sans avoir reçu la modification d'Alice : sa
	// transaction devient la dernière de la ligne, écartée comme écho ; la
	// ligne est relue à l'envoi.
	bob.mustSet(0, 2, "9")
	if r := bob.send(); r.Sent != 1 || r.Conflicts != 0 {
		t.Fatalf("envoi : %+v", r)
	}
	eq(t, "Bob après envoi", bob.col("Quantité"), []string{"5", "10", "1"})
	bob.receive()
	eq(t, "Bob Quantité", bob.col("Quantité"), []string{"5", "10", "1"})
	eq(t, "Bob Prix", bob.col("Prix"), []string{"9.0", "2.0", "3.25"})
	alice.receive()
	eq(t, "Alice Prix", alice.col("Prix"), []string{"9.0", "2.0", "3.25"})
}

func TestOwnInsertAuthor(t *testing.T) {
	_, _, bob, _ := setup(t)
	bob.mustSet(3, 0, "Tisane")
	bob.send()
	var author string
	must(t, bob.d.DB().Read().QueryRow(`SELECT coalesce(lattis_author, '') FROM _lattis_row_ids WHERE lattis_id = ?`, rowID(t, bob.d, 3)).Scan(&author))
	if author != "bob" {
		t.Fatalf("auteur de sa propre ligne, sans écho : %q", author)
	}
}

func TestWriteFilterRefusesOneOperation(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	f.Forbid(id, "Ventes", "Produit", "Interdit")
	// Une étape, deux cellules : seule la valeur refusée par le filtre
	// d'écriture est défaite.
	_, err := bob.d.Paste(ctx, 1, 1, 0, [][]string{{"Interdit"}, {"Tisane"}})
	must(t, err)
	r := bob.send()
	if r.Rejected != 1 || len(r.Notices) == 0 || !strings.Contains(r.Notices[0], "Droits insuffisants") {
		t.Fatalf("envoi : %+v", r)
	}
	eq(t, "Bob", bob.col("Produit"), []string{"Café", "Tisane", "Chocolat"})
	if v, _, _, _ := f.Value(id, "Ventes", rowID(t, bob.d, 1), "Produit"); v != "Tisane" {
		t.Fatalf("serveur : %v", v)
	}
	// Deux lignes ajoutées d'un coup : la ligne refusée disparaît, l'autre
	// reste.
	_, err = bob.d.Paste(ctx, 1, 4, 0, [][]string{{"Interdit"}, {"Lait"}})
	must(t, err)
	if r := bob.send(); r.Rejected < 1 {
		t.Fatalf("lignes ajoutées : %+v", r)
	}
	eq(t, "Bob lignes", bob.col("Produit"), []string{"Café", "Tisane", "Chocolat", "Lait"})
	alice.receive()
	eq(t, "Alice", alice.col("Produit"), []string{"Café", "Tisane", "Chocolat", "Lait"})
}

func TestSinceNotCovered(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	must(t, alice.d.DeleteRow(ctx, "Ventes", 2)) // Chocolat
	alice.mustSet(0, 1, "4")
	alice.send()
	// Les pierres tombales sont purgées avant que Bob ne lise : ses deltas
	// seraient incomplets, le tableau est relu en entier. Sa ligne pas
	// encore envoyée reste.
	f.PurgeTombstones()
	bob.mustSet(3, 0, "Tisane")
	r := bob.receive()
	if r.Reloaded != 1 || !r.Changed() {
		t.Fatalf("réception : %+v", r)
	}
	eq(t, "Bob", bob.col("Produit"), []string{"Café", "Thé", "Tisane"})
	eq(t, "Bob Quantité", bob.col("Quantité"), []string{"4", "10", ""})
	if r := bob.send(); r.Sent != 1 {
		t.Fatalf("envoi : %+v", r)
	}
	// Point de reprise de nouveau couvert : deltas.
	if r := bob.receive(); r.Reloaded != 0 {
		t.Fatalf("seconde réception : %+v", r)
	}
	alice.receive()
	eq(t, "Alice", alice.col("Produit"), []string{"Café", "Thé", "Tisane"})
}

func TestSinceCoveredUnknown(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	f.Legacy = true
	must(t, alice.d.DeleteRow(ctx, "Ventes", 2))
	alice.send()
	f.PurgeTombstones()
	// Serveur sans since_covered : le point de reprise est supposé couvert
	// (la suppression purgée n'arrive pas).
	if r := bob.receive(); r.Reloaded != 0 || f.Calls["since_covered"] != 0 {
		t.Fatalf("réception : %+v", r)
	}
	eq(t, "Bob", bob.col("Produit"), []string{"Café", "Thé", "Chocolat"})
}

func TestConcurrentAppendSameOrder(t *testing.T) {
	_, alice, bob, _ := setup(t)
	// Les deux postes ajoutent une ligne en fin de tableau en même temps :
	// sans suffixe, les deux clés d'ordre seraient égales et chaque poste
	// mettrait la sienne en premier.
	alice.mustSet(3, 0, "Sucre")
	bob.mustSet(3, 0, "Lait")
	alice.send()
	bob.send()
	alice.receive()
	bob.receive()
	a, b := alice.col("Produit"), bob.col("Produit")
	if len(a) != 5 {
		t.Fatalf("Alice : %q", a)
	}
	eq(t, "même ordre", b, a)
	tbl := strconv.FormatInt(tableID(t, alice.d), 10)
	var n, distinct int
	must(t, alice.d.DB().Read().QueryRow(`SELECT count(*), count(DISTINCT _pos) FROM t_`+tbl).Scan(&n, &distinct))
	if n != distinct {
		t.Fatalf("clés d'ordre égales : %d lignes, %d clés", n, distinct)
	}
}
