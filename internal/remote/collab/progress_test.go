package collab_test

import (
	"context"
	"sync"
	"testing"

	"github.com/lalamefine/lattis/internal/work"
)

// Réception suivie dans la barre d'état : un tableau relu en entier dit ses
// lignes ; la lecture d'un tableau sur notification, couverte par ses
// deltas, ne montre rien.
func TestReceiveProgress(t *testing.T) {
	ctx := context.Background()
	d, e := work.Delay, work.Every
	work.Delay, work.Every = 0, 0
	t.Cleanup(func() { work.Delay, work.Every = d, e })
	f, alice, bob, _ := setup(t)

	var mu sync.Mutex
	var steps []work.Step
	tracked := 0
	bob.e.SetTrack(func(ctx context.Context) (context.Context, func()) {
		mu.Lock()
		tracked++
		mu.Unlock()
		wctx, tr := work.Start(ctx, func(st work.Step) {
			mu.Lock()
			steps = append(steps, st)
			mu.Unlock()
		}, false)
		return wctx, tr.End
	})

	// Sur notification, deltas : aucun suivi.
	alice.mustSet(0, 1, "4")
	alice.send()
	if _, err := bob.e.Receive(ctx, map[int64]bool{tableID(t, bob.d): true}); err != nil {
		t.Fatal(err)
	}
	if tracked != 0 {
		t.Fatalf("suivi d'une simple réception : %d", tracked)
	}

	// Pierres tombales purgées : le tableau est relu en entier, suivi.
	if err := alice.d.DeleteRow(ctx, "Ventes", 2); err != nil {
		t.Fatal(err)
	}
	alice.send()
	f.PurgeTombstones()
	if r, err := bob.e.Receive(ctx, map[int64]bool{tableID(t, bob.d): true}); err != nil || r.Reloaded != 1 {
		t.Fatalf("relecture : %+v %v", r, err)
	}
	mu.Lock()
	defer mu.Unlock()
	var rows work.Step
	for _, st := range steps {
		if st.Phase == "rows" {
			rows = st
		}
	}
	if tracked != 1 || rows.Op != "receive" || rows.Item != "Ventes" || rows.Done != 2 || rows.Cancel {
		t.Fatalf("suivi de la relecture : %d, %+v", tracked, rows)
	}
}
