package collab_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Écriture optimiste (décision 0012) : une saisie ou un changement de
// structure d'un classeur collaboratif est fait et visible en local sans
// attendre le serveur ; la confirmation (ou le refus) arrive ensuite par
// l'envoi de la file.

// slowServer est un serveur lent : chaque écriture attend delay.
type slowServer struct {
	workbook.Server
	delay  time.Duration
	writes atomic.Int32
}

func (s *slowServer) Write(ctx context.Context, fn func(workbook.Writer) error) error {
	s.writes.Add(1)
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.Server.Write(ctx, fn)
}

// TestOptimisticSlowServer : un serveur qui met 0,8 s à répondre ne retarde
// ni les saisies ni les changements de structure, alors même qu'un envoi
// est en cours ; tout part ensuite, dans l'ordre.
func TestOptimisticSlowServer(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	slow := &slowServer{Server: f.As("alice"), delay: 800 * time.Millisecond}
	alice.e.SetServer(slow)

	timed := func(what string, fn func() error) {
		t.Helper()
		start := time.Now()
		must(t, fn())
		if d := time.Since(start); d > 400*time.Millisecond {
			t.Fatalf("%s : %v, le serveur a été attendu", what, d)
		}
	}
	timed("saisie", func() error { return alice.set(0, 1, "7") })
	// Envoi en cours (bloqué sur le serveur lent) pendant les suivantes.
	sent := make(chan collab.Report, 1)
	go func() {
		r, err := alice.e.Send(ctx)
		if err != nil {
			t.Error(err)
		}
		sent <- r
	}()
	waitFor(t, "envoi commencé", func() bool { return slow.writes.Load() > 0 })
	timed("colonne ajoutée", func() error { return alice.d.InsertColumn(ctx, "Ventes", 3, "Remise") })
	timed("saisie dans la nouvelle colonne", func() error { return alice.set(1, 3, "10 %") })
	timed("conversion", func() error { return alice.d.SetType(ctx, "Ventes", 2, grid.Text) })
	// Visible en local tout de suite.
	eq(t, "Alice", colsOf(alice, "Ventes"), []string{"Produit:text", "Quantité:integer", "Prix:text", "Remise:text"})
	eq(t, "Alice Quantité", alice.col("Quantité"), []string{"7", "10", "1"})
	eq(t, "Alice Remise", alice.col("Remise"), []string{"", "10 %", ""})
	if p, _, _, _, err := alice.e.Counts(ctx); err != nil || p == 0 {
		t.Fatalf("file : %d, %v", p, err)
	}
	<-sent
	r, err := alice.e.Send(ctx)
	must(t, err)
	if r.Rejected != 0 {
		t.Fatalf("envoi : %+v", r)
	}
	cols, _ := serverCols(f, id, "Ventes")
	eq(t, "serveur", cols, []string{"Produit:text", "Quantité:integer", "Prix:text", "Remise:text"})
	if v, _, _, _ := f.Value(id, "Ventes", rowID(t, alice.d, 1), "Remise"); v != "10 %" {
		t.Fatalf("Remise sur le serveur : %v", v)
	}
	tbl := map[int64]bool{tableID(t, bob.d): true}
	_, err = bob.e.ReceiveStructure(ctx, nil, tbl)
	must(t, err)
	eq(t, "Bob Remise", bob.col("Remise"), []string{"", "10 %", ""})
	eq(t, "Bob Quantité", bob.col("Quantité"), []string{"7", "10", "1"})
}

// TestOptimisticStructureWaitsForReceive : tant qu'un changement de
// structure attend le serveur, la réception laisse le tableau tel quel
// (sa correspondance ne le reflète pas encore) ; il est relu après
// l'envoi.
func TestOptimisticStructureWaitsForReceive(t *testing.T) {
	ctx := context.Background()
	_, alice, bob, _ := setup(t)
	must(t, alice.d.DeleteColumn(ctx, "Ventes", 2))
	bob.mustSet(0, 1, "42")
	bob.send()
	r, err := alice.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if len(r.Notices) != 0 || r.Changed() {
		t.Fatalf("réception pendant l'attente : %+v", r)
	}
	eq(t, "Alice sans Prix", colsOf(alice, "Ventes"), []string{"Produit:text", "Quantité:integer"})
	eq(t, "Alice Quantité (pas encore reçue)", alice.col("Quantité"), []string{"3", "10", "1"})
	alice.send()
	_, err = alice.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	eq(t, "Alice", colsOf(alice, "Ventes"), []string{"Produit:text", "Quantité:integer"})
	eq(t, "Alice Quantité", alice.col("Quantité"), []string{"42", "10", "1"})
}

// TestOptimisticSyncStatus : la barre d'état voit la file (en attente,
// envoi en cours) sans relire l'état de partage.
func TestOptimisticSyncStatus(t *testing.T) {
	ctx := context.Background()
	_, alice, _, _ := setup(t)
	if s := alice.d.Share.Sync(); !s.Shared || !s.Online || s.Pending != 0 || s.Sending {
		t.Fatalf("au départ : %+v", s)
	}
	alice.mustSet(0, 1, "5")
	alice.mustSet(1, 1, "6")
	_, _, state, sync, err := alice.e.Counts(ctx)
	must(t, err)
	if state || !sync || alice.d.Share.Sync().Pending != 2 {
		t.Fatalf("file : state %v, sync %v, %+v", state, sync, alice.d.Share.Sync())
	}
	alice.send()
	_, _, state, sync, err = alice.e.Counts(ctx)
	must(t, err)
	if state || !sync || alice.d.Share.Sync().Pending != 0 {
		t.Fatalf("après l'envoi : state %v, sync %v, %+v", state, sync, alice.d.Share.Sync())
	}
	// Hors ligne : la file ne change que la barre d'état (le classeur
	// reste modifiable, décision 0014 : pas de relecture de la mise en page
	// à chaque saisie).
	alice.d.Share.SetOnline(false, "coupure")
	if state, sync := alice.d.Share.SetQueue(3, 0); state || !sync {
		t.Fatalf("hors ligne : state %v, sync %v", state, sync)
	}
	if s := alice.d.Share.Sync(); s.Online || s.Pending != 3 || alice.d.Share.State().Offline != "coupure" {
		t.Fatalf("hors ligne : %+v", s)
	}
}
