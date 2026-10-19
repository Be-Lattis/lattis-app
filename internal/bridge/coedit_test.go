package bridge

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/coedit"
	"github.com/lalamefine/lattis/internal/doc"
)

// coeditHost relève les états du travail à plusieurs.
type coeditHost struct {
	NoHost
	events chan coedit.State
}

func (h *coeditHost) Emit(event string, data any) bool {
	if event == CoEditEvent {
		select {
		case h.events <- data.(coedit.State):
		default:
		}
	}
	return true
}

// Deux postes (deux services, deux dossiers de travail) sur un même
// fichier : Bob rejoint Alice, sa saisie arrive chez elle, la structure
// est refusée tant qu'ils sont deux.
func TestCoEditTwoServices(t *testing.T) {
	ctx := context.Background()
	old := coeditTiming
	coeditTiming = coedit.Timing{Poll: 10 * time.Millisecond, Watch: 10 * time.Millisecond, Beat: 20 * time.Millisecond, Stale: 2 * time.Second, JoinWait: 5 * time.Second, Compact: 50 * time.Millisecond}
	t.Cleanup(func() { coeditTiming = old })
	oldRoot := archive.Root
	t.Cleanup(func() { archive.Root = oldRoot })
	root := func(name string) {
		dir := filepath.Join(t.TempDir(), name)
		archive.Root = func() (string, error) { return dir, nil }
	}

	root("maker")
	d, err := doc.New(ctx)
	mustShare(t, err)
	_, err = d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4,5"}, {"Thé", "2"}})
	mustShare(t, err)
	path := filepath.Join(t.TempDir(), "Prix.lattis")
	mustShare(t, d.Save(ctx, path))
	d.Close()

	service := func() (*DocumentService, *coeditHost) {
		s := &DocumentService{}
		h := &coeditHost{events: make(chan coedit.State, 64)}
		UseHost(h, nil, s)
		t.Cleanup(func() { s.CloseDocument(context.Background()) })
		return s, h
	}
	alice, ha := service()
	root("alice")
	r, err := alice.Open(ctx, path)
	mustShare(t, err)
	if r.Locked != nil || !r.State.Open {
		t.Fatalf("ouverture d'Alice : %+v", r)
	}

	bob, _ := service()
	root("bob")
	r, err = bob.OpenTogether(ctx, path)
	mustShare(t, err)
	if !r.State.Open {
		t.Fatalf("ouverture de Bob : %+v", r)
	}
	if st := bob.CoEditState(); !st.Active || len(st.Peers) != 1 {
		t.Fatalf("Bob : %+v", st)
	}
	for st := range ha.events {
		if st.Active && len(st.Peers) == 1 {
			break
		}
	}

	_, err = bob.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	ad, _ := alice.current()
	waitFor(t, "la saisie de Bob chez Alice", func() bool {
		var v float64
		ad.DB().Read().QueryRow(`SELECT c_2 FROM t_1 ORDER BY _pos LIMIT 1`).Scan(&v)
		return v == 5
	})
	var name string
	mustShare(t, ad.DB().Read().QueryRow(`SELECT name FROM _lattis_tables`).Scan(&name))
	_, err = bob.RenameTable(ctx, name, "Prix")
	var te coedit.TogetherError
	if !errors.As(err, &te) {
		t.Fatalf("structure à deux : %v", err)
	}
	// Alice enregistre, Bob aussi par-dessus : rien n'est demandé.
	_, err = alice.Save(ctx, false)
	mustShare(t, err)
	_, err = bob.SetCell(ctx, 1, 2, 1, "3")
	mustShare(t, err)
	st, err := bob.Save(ctx, false)
	mustShare(t, err)
	if st.Conflict != nil {
		t.Fatalf("enregistrement de Bob : %+v", st.Conflict)
	}
}
