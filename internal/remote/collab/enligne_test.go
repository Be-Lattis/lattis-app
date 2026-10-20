package collab_test

// Essais tirés de l'audit du mode en ligne (docs/audits/audit-en-ligne-2026-10-20.md),
// faux serveur : chacun est un défaut corrigé, gardé en non-régression.

import (
	"context"
	"testing"
)

// Constat 1 : une colonne renommée par une collègue pendant la coupure, alors
// qu'une saisie hors ligne la vise : l'envoi échoue (42703, errStale), la
// réception (qui relirait la correspondance) ne vient qu'après l'envoi.
func TestStaleColumnAfterColleague(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	f.SetDown(true)
	bob.d.Share.SetOnline(false, "serveur injoignable")
	bob.mustSet(0, 2, "9") // Prix
	f.SetDown(false)
	must(t, alice.header(2, "Tarif"))
	alice.send()

	bob.d.Share.SetOnline(true, "")
	for i := 0; i < 3; i++ {
		_, err := bob.e.Sync(ctx)
		t.Logf("essai %d : err=%v, file=%v", i+1, err, bob.states())
	}
	if bob.states()["pending"] > 0 {
		t.Fatalf("la file ne part pas (%v) ; colonnes de Bob : %v", bob.states(), colsOf(bob, "Ventes"))
	}
	// La saisie de Bob arrive sous le nouveau nom de la colonne.
	alice.receive()
	if got := alice.col("Tarif"); len(got) == 0 || got[0] != "9.0" {
		t.Errorf("Tarif chez Alice : %v, attendu 9.0 en premier", got)
	}
}

// Constat 1 : même chose avec un tableau supprimé par une conceptrice.
func TestStaleTableAfterColleague(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	f.SetDown(true)
	bob.d.Share.SetOnline(false, "serveur injoignable")
	bob.mustSet(0, 1, "99")
	f.SetDown(false)
	must(t, alice.d.DeleteTables(ctx, []string{"Ventes"}))
	alice.send()

	bob.d.Share.SetOnline(true, "")
	for i := 0; i < 3; i++ {
		_, err := bob.e.Sync(ctx)
		t.Logf("essai %d : err=%v, file=%v", i+1, err, bob.states())
	}
	if bob.states()["pending"] > 0 {
		t.Errorf("la file ne part pas (%v)", bob.states())
	}
}

// Même chose avec le faux serveur (l'erreur 42703/42P01 arrive à
// l'exécution, sans le problème du point de sauvegarde).
func TestStructureDuringInputFake(t *testing.T) {
	_, alice, bob, _ := setup(t)
	ctx := context.Background()
	bob.mustSet(1, 2, "8")
	must(t, alice.d.Model.RenameTable(ctx, "Ventes", "Achats"))
	alice.send()
	_, err1 := bob.e.Sync(ctx)
	_, err2 := bob.e.Sync(ctx)
	if err1 != nil || err2 != nil || bob.states()["pending"] != 0 {
		t.Fatalf("Bob : %v / %v, file %v", err1, err2, bob.states())
	}
}
