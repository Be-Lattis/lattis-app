package bridge

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/work"
)

// workHost relève les états des opérations longues (WorkEvent) ; onStep
// peut réagir à chacun (bouton « Annuler »).
type workHost struct {
	NoHost
	mu     sync.Mutex
	steps  []work.Step
	onStep func(work.Step)
}

func (h *workHost) Emit(event string, data any) bool {
	if event == WorkEvent {
		st := data.(work.Step)
		h.mu.Lock()
		h.steps = append(h.steps, st)
		on := h.onStep
		h.mu.Unlock()
		if on != nil {
			on(st)
		}
	}
	return true
}

// TestCancelWorkFromStatusBar : la barre d'état reçoit l'action et ses
// étapes ; « Annuler » (CancelWork) interrompt la suppression d'une colonne,
// le classeur reste tel qu'avant, avec un message clair.
func TestCancelWorkFromStatusBar(t *testing.T) {
	d, e := work.Delay, work.Every
	work.Delay, work.Every = 0, 0
	t.Cleanup(func() { work.Delay, work.Every = d, e })
	ctx := context.Background()
	s := &DocumentService{}
	h := &workHost{}
	UseHost(h, nil, s)
	t.Cleanup(func() { s.CloseDocument(ctx) })
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	block := [][]string{{"Produit", "Prix", "Qté"}}
	for i := range 300 {
		block = append(block, []string{"P" + strconv.Itoa(i), strconv.Itoa(i), "1"})
	}
	if _, err := s.Paste(ctx, 1, 0, 0, block); err != nil {
		t.Fatal(err)
	}
	l, err := s.Layout(ctx, 1)
	if err != nil || len(l.Tables) != 1 {
		t.Fatalf("%v %+v", err, l.Tables)
	}
	table := l.Tables[0].Name
	h.mu.Lock()
	h.steps = nil
	h.onStep = func(st work.Step) {
		if st.Phase == "rewrite" && !st.End && !s.CancelWork(st.ID) {
			t.Error("CancelWork refusé")
		}
	}
	h.mu.Unlock()
	_, err = s.DeleteColumn(ctx, table, 1)
	if !errors.Is(err, errWorkCancelled) {
		t.Fatalf("erreur : %v", err)
	}
	h.mu.Lock()
	h.onStep = nil
	steps := append([]work.Step(nil), h.steps...)
	h.mu.Unlock()
	for len(steps) > 0 && steps[0].Op == "" {
		steps = steps[1:] // émise avant que l'action soit nommée (Delay nul)
	}
	if len(steps) == 0 || steps[0].Op != "deleteColumn" || steps[0].Name != "Prix" || !steps[0].Cancel {
		t.Fatalf("étapes : %+v", steps)
	}
	if last := steps[len(steps)-1]; !last.End {
		t.Fatalf("fin non émise : %+v", last)
	}
	l, err = s.Layout(ctx, 1)
	if err != nil || len(l.Tables[0].Columns) != 3 || l.Tables[0].Rows != 300 {
		t.Fatalf("classeur modifié : %v %+v", err, l.Tables[0])
	}
	// Opération finie : plus rien à annuler.
	if s.CancelWork(steps[0].ID) {
		t.Fatal("opération finie annulée")
	}
	// Sans interruption : faite.
	if _, err := s.DeleteColumn(ctx, table, 1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	l, _ = s.Layout(ctx, 1)
	if len(l.Tables[0].Columns) != 2 {
		t.Fatalf("colonnes : %+v", l.Tables[0].Columns)
	}
}
