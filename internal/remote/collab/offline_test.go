package collab_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Édition hors ligne d'un classeur collaboratif (décision 0014) : tout est
// permis avec les derniers droits connus, la file part à la reconnexion,
// dans l'ordre, avec la détection de conflit et les refus habituels.

// TestOfflineStructure : une colonne ajoutée puis remplie hors ligne part à
// la reconnexion (structure avant les données) ; un renommage devenu
// interdit pendant la coupure (colonne citée par une restriction posée
// entre-temps) est refusé par le serveur et défait en local.
func TestOfflineStructure(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	tid, _ := remoteID(t, alice, "Produit")

	f.SetDown(true)
	alice.d.Share.SetOnline(false, "serveur injoignable")
	if st := alice.d.Share.State(); st.ReadOnly != "" {
		t.Fatalf("hors ligne : %+v", st)
	}
	must(t, alice.d.InsertColumn(ctx, "Ventes", 3, "Note"))
	alice.mustSet(0, 3, "à revoir")
	alice.mustSet(1, 2, "2,5")
	must(t, alice.header(1, "Qté"))
	if alice.states()["pending"] != 4 {
		t.Fatalf("file hors ligne : %v", alice.states())
	}
	if _, err := alice.e.Send(ctx); err != nil {
		t.Fatalf("envoi hors ligne : %v", err) // rien ne part, sans erreur
	}
	if alice.states()["pending"] != 4 {
		t.Fatalf("file après un envoi hors ligne : %v", alice.states())
	}

	// Pendant la coupure (pour Alice), une restriction cite « Quantité ».
	f.SetDown(false)
	_, err := f.As("alice").AddRestriction(ctx, id, workbook.RestrictionRule{TableID: tid, Command: "select", Predicate: `"Quantité" > 0`, Label: "En stock"})
	must(t, err)

	// Reconnexion : tout part dans l'ordre ; le renommage est refusé.
	alice.d.Share.SetOnline(true, "")
	r := alice.send()
	all := strings.Join(r.Notices, " ; ")
	if r.Rejected != 1 || !strings.Contains(all, "restriction « En stock »") || !strings.Contains(all, "la modification a été annulée") {
		t.Fatalf("reconnexion : %+v", r)
	}
	if alice.states()["pending"] != 0 {
		t.Fatalf("file après la reconnexion : %v", alice.states())
	}
	eq(t, "Alice", colsOf(alice, "Ventes"), []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Note:text"})
	cols, _ := serverCols(f, id, "Ventes")
	eq(t, "serveur", cols, []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Note:text"})
	eq(t, "Note", alice.col("Note"), []string{"à revoir", "", ""})

	// Bob reçoit la colonne et les valeurs.
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	eq(t, "Bob", colsOf(bob, "Ventes"), []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Note:text"})
	eq(t, "Bob : Note", bob.col("Note"), []string{"à revoir", "", ""})
	eq(t, "Bob : Prix", bob.col("Prix"), []string{"4.5", "2.5", "3.25"})
}

// TestOfflineRowDeletedMeanwhile : une ligne modifiée hors ligne, supprimée
// entre-temps par un collègue : la modification n'est pas gardée (la ligne
// reste supprimée), sans bloquer le reste de la file.
func TestOfflineRowDeletedMeanwhile(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	f.SetDown(true)
	bob.d.Share.SetOnline(false, "serveur injoignable")
	bob.mustSet(0, 1, "33")
	bob.mustSet(2, 1, "11")
	f.SetDown(false)
	// Alice supprime la première ligne pendant la coupure de Bob.
	must(t, alice.d.DeleteRow(ctx, "Ventes", 0)) // Café
	alice.send()

	bob.d.Share.SetOnline(true, "")
	_, err := bob.e.Sync(ctx)
	must(t, err)
	if bob.states()["pending"] != 0 {
		t.Fatalf("file : %v", bob.states())
	}
	eq(t, "Bob", bob.col("Quantité"), []string{"10", "11"})
	alice.receive()
	eq(t, "Alice", alice.col("Quantité"), []string{"10", "11"})
}

// TestOfflineLongOutage : coupure longue, le serveur a purgé ses pierres
// tombales entre-temps (point de reprise plus couvert) : la file part
// d'abord (conflit sur la cellule changée des deux côtés, ligne ajoutée
// hors ligne envoyée), puis le tableau est relu en entier (ligne supprimée
// par la collègue retirée), sans perdre ce que le conflit garde.
func TestOfflineLongOutage(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	f.SetDown(true)
	bob.d.Share.SetOnline(false, "serveur injoignable")
	bob.mustSet(1, 1, "31")
	must(t, bob.d.InsertRow(ctx, "Ventes", 3))
	bob.mustSet(3, 0, "Sucre")
	bob.mustSet(3, 1, "7")
	f.SetDown(false)
	alice.mustSet(1, 1, "40")
	must(t, alice.d.DeleteRow(ctx, "Ventes", 2)) // Chocolat
	alice.send()
	f.PurgeTombstones()

	bob.d.Share.SetOnline(true, "")
	f.Calls["since_covered"] = 0
	r, err := bob.e.Sync(ctx)
	must(t, err)
	if r.Conflicts != 1 || f.Calls["since_covered"] == 0 {
		t.Fatalf("reconnexion : %+v, %v", r, f.Calls)
	}
	eq(t, "Bob Produit", bob.col("Produit"), []string{"Café", "Thé", "Sucre"})
	eq(t, "Bob Quantité", bob.col("Quantité"), []string{"3", "31", "7"})
	cs, err := bob.e.Conflicts(ctx)
	must(t, err)
	if len(cs) != 1 {
		t.Fatalf("conflits : %+v", cs)
	}
	alice.receive()
	eq(t, "Alice", alice.col("Produit"), []string{"Café", "Thé", "Sucre"})
	eq(t, "Alice Quantité", alice.col("Quantité"), []string{"3", "40", "7"})
}

// capsDown : relecture des droits en échec (connexion morte) tant que down.
type capsDown struct {
	workbook.Server
	down  atomic.Bool
	tries atomic.Int32
}

func (c *capsDown) Capabilities(ctx context.Context, id string) (workbook.Capabilities, error) {
	if c.down.Load() {
		c.tries.Add(1)
		return workbook.Capabilities{}, errNet
	}
	return c.Server.Capabilities(ctx, id)
}

// TestOfflineRightsUnreadAtReconnect : à la reprise, la relecture des
// droits échoue (connexion morte du pool, coupure) : rien ne part avec les
// droits d'avant la coupure ; au tour suivant, les droits réduits pendant
// la coupure sont lus et la saisie est refusée en local, avec un message.
func TestOfflineRightsUnreadAtReconnect(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 10 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	f, _, bob, id := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var notices []string
	srv := &capsDown{Server: f.As("bob")}
	bob.follow(ctx, srv, 20*time.Millisecond, func(ev collab.Event) {
		mu.Lock()
		notices = append(notices, ev.Notices...)
		mu.Unlock()
	})
	waitOnline(t, bob)
	f.SetDown(true)
	waitFor(t, "Bob hors ligne", func() bool { return !bob.d.Share.Online() })
	bob.mustSet(0, 1, "33")
	f.Revoke(id, "bob", "editor")
	must(t, f.Grant(id, "bob", "reader"))
	srv.down.Store(true)
	writes := f.Count("write")
	f.SetDown(false)
	waitFor(t, "relecture des droits essayée", func() bool { return srv.tries.Load() > 0 })
	time.Sleep(100 * time.Millisecond)
	if n := f.Count("write"); n != writes || bob.states()["pending"] != 1 {
		t.Fatalf("envoyé sans droits relus : %d écritures, file %v", n-writes, bob.states())
	}
	srv.down.Store(false)
	waitFor(t, "file de Bob vidée", func() bool { return bob.states()["pending"] == 0 })
	eq(t, "Bob", bob.col("Quantité"), []string{"3", "10", "1"})
	// Le message part à la fin du tour, après la file vidée.
	waitFor(t, "message des droits changés", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return strings.Contains(strings.Join(notices, " ; "), "droits")
	})
}
