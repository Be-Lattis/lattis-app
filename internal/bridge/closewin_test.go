package bridge

import (
	"context"
	"testing"
	"time"
)

// quitter est un hôte qui compte les fermetures de l'application.
type quitter struct {
	emitter
	quits int
}

func (q *quitter) Window() WindowControl { return quitWindow{q} }

type quitWindow struct{ q *quitter }

func (quitWindow) SetTitle(string) {}
func (w quitWindow) Quit()         { w.q.quits++ }

func (q *quitter) asked() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, e := range q.events {
		if e.event == CloseRequestEvent {
			n++
		}
	}
	return n
}

// Fermer la fenêtre d'un classeur modifié pose la question à l'interface
// (audit 6, I1) ; intact, la fenêtre se ferme ; une interface bloquée
// n'empêche pas de fermer au deuxième clic.
func TestWindowClosing(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	s := &DocumentService{}
	q := &quitter{}
	UseHost(q, nil, s)
	if got := s.windowClosing(time.Now()); got != closeAllow {
		t.Fatalf("sans classeur : %v", got)
	}
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	if got := s.windowClosing(time.Now()); got != closeAllow || q.asked() != 0 {
		t.Fatalf("classeur intact : %v, %d questions", got, q.asked())
	}
	d, _ := s.current()
	if _, err := d.SetCell(ctx, 1, 0, 0, "x"); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	if got := s.windowClosing(t0); got != closeAsked || q.asked() != 1 {
		t.Fatalf("classeur modifié : %v, %d questions", got, q.asked())
	}
	// Interface qui a pris la demande : on attend sa réponse, même longtemps.
	s.CloseRequestSeen()
	if got := s.windowClosing(t0.Add(time.Minute)); got != closeAsked {
		t.Fatalf("question en cours : %v", got)
	}
	// Interface muette depuis plus de closeAckTimeout : on ferme.
	if got := s.windowClosing(t0.Add(time.Minute + closeAckTimeout + time.Second)); got != closeAllow {
		t.Fatalf("interface bloquée : %v", got)
	}
	// Réponse de l'interface : la fenêtre se ferme, plus de question.
	s.QuitApp()
	if q.quits != 1 {
		t.Fatalf("%d fermetures", q.quits)
	}
	if got := s.windowClosing(time.Now()); got != closeAllow {
		t.Fatalf("après QuitApp : %v", got)
	}
}
