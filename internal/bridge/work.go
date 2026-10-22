package bridge

import (
	"context"
	"sync"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/work"
)

// WorkEvent porte l'état d'une opération longue (work.Step) : la barre
// d'état dit ce qui se passe (« Suppression de la colonne Prix :
// réécriture du tableau ») au lieu de « Traitement… », avec « Annuler »
// quand l'opération peut l'être. Rien n'est émis pour une opération
// courte (work.Delay).
const WorkEvent = "work:step"

var errWorkCancelled = msg.New("work.cancelled", nil)

// workState : opérations suivies en cours, que CancelWork peut viser.
type workState struct {
	mu      sync.Mutex
	running map[int64]*work.Tracker
}

// track commence le suivi d'une opération ; cancellable : elle tient dans
// une seule transaction (Model.Do, lecture), qu'une interruption défait en
// entier. Dans une opération déjà suivie, rien ne change (t nil).
func (s *DocumentService) track(ctx context.Context, cancellable bool) (context.Context, *work.Tracker) {
	ctx, t := work.Start(ctx, func(st work.Step) { s.sys().Emit(WorkEvent, st) }, cancellable)
	if t != nil {
		w := &s.work
		w.mu.Lock()
		if w.running == nil {
			w.running = map[int64]*work.Tracker{}
		}
		w.running[t.ID()] = t
		w.mu.Unlock()
	}
	return ctx, t
}

// untrack termine le suivi ; une opération interrompue par CancelWork
// rend un message clair (rien n'a été modifié) au lieu de l'erreur de
// SQLite.
func (s *DocumentService) untrack(t *work.Tracker, err *error) {
	if t == nil {
		return
	}
	w := &s.work
	w.mu.Lock()
	delete(w.running, t.ID())
	w.mu.Unlock()
	t.End()
	if err != nil && *err != nil && t.Cancelled() {
		*err = errWorkCancelled
	}
}

// CancelWork interrompt l'opération id (bouton « Annuler » de la barre
// d'état) : sa transaction est défaite, le classeur reste tel qu'avant.
// Faux si elle est finie ou ne peut pas être interrompue.
func (s *DocumentService) CancelWork(id int64) bool {
	w := &s.work
	w.mu.Lock()
	t := w.running[id]
	w.mu.Unlock()
	return t.Cancel()
}

// actw est act pour une opération suivie dans la barre d'état ; fn reçoit
// le contexte du suivi. cancellable : fn fait une seule transaction
// (Model.Do), CancelWork peut l'interrompre.
func (s *DocumentService) actw(ctx context.Context, cancellable bool, fn func(context.Context, *doc.Document) error) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	wctx, t := s.track(ctx, cancellable)
	err = fn(wctx, d)
	s.untrack(t, &err)
	if err != nil {
		return DocState{}, err
	}
	// Contexte de l'appel, pas celui du suivi : l'opération est faite, la
	// relecture de l'état ne doit pas être interrompue.
	return s.changed(ctx)
}
