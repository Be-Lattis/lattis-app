package collab_test

import (
	"context"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// waitGone attend que le classeur de c soit noté disparu (gone).
func waitGone(t *testing.T, c *client, gone string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for c.d.Share.Gone() != gone {
		if time.Now().After(deadline) {
			t.Fatalf("%s : %+v, attendu %q", c.name, c.d.Share.State(), gone)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Classeur supprimé du serveur pendant le suivi : ni « hors ligne » ni
// nouveaux essais, le suivi s'arrête ; la modification pas encore
// envoyée reste dans la file (rien n'est défait) ; les suivantes ne s'y
// ajoutent plus.
func TestFollowStopsWhenDeleted(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 10 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	f, _, bob, id := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		bob.e.Follow(ctx, f.As("bob"), 10*time.Millisecond, nil)
		close(done)
	}()
	waitOnline(t, bob)
	f.DropWorkbook(id)
	bob.mustSet(0, 1, "7")
	bob.e.Kick()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("suivi pas arrêté")
	}
	waitGone(t, bob, workbook.GoneDeleted)
	if st := bob.d.Share.State(); st.Online || st.Offline != "" || st.ReadOnly != "" {
		t.Fatalf("état : %+v", st)
	}
	calls := f.Count("my_capabilities") + f.Count("listen")
	time.Sleep(150 * time.Millisecond)
	if n := f.Count("my_capabilities") + f.Count("listen"); n != calls {
		t.Fatalf("nouveaux essais : %d appels de plus", n-calls)
	}
	p, _, _, _, err := bob.e.Counts(ctx)
	must(t, err)
	if p != 1 {
		t.Fatalf("file : %d", p)
	}
	// Données locales désormais : la saisie passe, rien ne part plus.
	bob.mustSet(1, 1, "8")
	if p, _, _, _, _ = bob.e.Counts(ctx); p != 1 {
		t.Fatalf("file après la suppression : %d", p)
	}
	eq(t, "Bob", bob.col("Quantité"), []string{"7", "8", "1"})
	// Rouvert plus tard : Follow ne repart pas.
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	start := time.Now()
	bob.e.Follow(ctx2, f.As("bob"), 10*time.Millisecond, nil)
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("Follow d'un classeur supprimé")
	}
}

// Accès retiré : noté (gone « revoked »), en ligne, la file reste ; accès
// rendu (*acl) : de nouveau suivi normalement, la file part.
func TestFollowRevokedKeepsQueue(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 10 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	f, _, bob, id := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bob.e.Follow(ctx, f.As("bob"), 10*time.Millisecond, nil)
	waitOnline(t, bob)
	as := f.As("alice")
	editor := roleID(t, as, id, "editor")
	must(t, as.RevokeRole(ctx, editor, "bob"))
	waitGone(t, bob, workbook.GoneRevoked)
	if st := bob.d.Share.State(); !st.Online || st.Offline != "" {
		t.Fatalf("état : %+v", st)
	}
	must(t, as.GrantRole(ctx, editor, "bob"))
	waitGone(t, bob, "")
	bob.mustSet(0, 1, "11")
	bob.e.Kick()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if p, _, _, _, _ := bob.e.Counts(ctx); p == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("file jamais partie")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
