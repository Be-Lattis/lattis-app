package bridge

import (
	"context"
	"testing"

	"github.com/lalamefine/lattis/internal/work"
)

// Édition web : un onglet qui arrive voit l'opération en cours, sans attendre
// qu'elle change d'étape.
func TestReplayWork(t *testing.T) {
	d, e := work.Delay, work.Every
	work.Delay, work.Every = 0, 0
	t.Cleanup(func() { work.Delay, work.Every = d, e })
	s := &DocumentService{}
	h := &workHost{}
	UseHost(h, nil, s)
	wctx, tr := s.track(context.Background(), true)
	work.Op(wctx, "openRemote", "")
	work.Loop(wctx, "connect", 0)
	h.mu.Lock()
	h.steps = nil
	h.mu.Unlock()
	ReplayWork(s)
	h.mu.Lock()
	got := append([]work.Step(nil), h.steps...)
	h.steps = nil
	h.mu.Unlock()
	if len(got) != 1 || got[0].Op != "openRemote" || got[0].Phase != "connect" || !got[0].Cancel {
		t.Fatalf("redit : %+v", got)
	}
	var err error
	s.untrack(tr, &err)
	ReplayWork(s)
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, st := range h.steps {
		if !st.End {
			t.Fatalf("opération finie redite : %+v", st)
		}
	}
}
