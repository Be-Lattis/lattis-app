package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/ai"
	"github.com/lalamefine/lattis/internal/config"
)

// Erreur au deuxième tour : « Réessayer » reprend la même réponse sans
// renvoyer le message (I2), et son annulation reste d'un seul tenant ; une
// erreur au premier tour retire le message de la conversation et de
// l'affichage (M3) ; une écriture sans effet ne fait pas de bilan (M5).
func TestAIResumeAndUnsent(t *testing.T) {
	workRoot(t)
	defer config.SetCurrent(config.Current())
	aiConfig(false)
	p := &scriptProvider{}
	ai.Register(p)
	ctx := context.Background()
	s := &DocumentService{}
	h := &aiHost{}
	UseHost(h, nil, s)
	t.Cleanup(func() { s.CloseDocument(ctx) })
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}

	// Premier tour en échec : message retiré partout, évènement unsent.
	if err := s.AISend(ctx, 1, "Bonjour", AIContext{}); err == nil {
		t.Fatal("erreur attendue")
	}
	if len(s.AITranscript()) != 0 || s.AIState().Messages != 0 {
		t.Fatalf("message gardé : %+v", s.AITranscript())
	}
	h.mu.Lock()
	last := h.events[len(h.events)-1]
	h.mu.Unlock()
	if last.Run != 1 || last.Kind != ai.EventUnsent {
		t.Fatalf("dernier évènement : %+v", last)
	}

	// Erreur au deuxième tour, après une écriture.
	p.replies = addSheet("Analyse")[:1]
	if err := s.AISend(ctx, 2, "Crée une feuille", AIContext{}); err == nil {
		t.Fatal("erreur attendue")
	}
	msgs := s.AIState().Messages
	p.replies = addSheet("Analyse")[1:]
	if err := s.AIResume(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if n := s.AIState().Messages; n != msgs+1 {
		t.Fatalf("%d messages après la reprise (avant : %d)", n, msgs)
	}
	users, errs, dones := 0, 0, 0
	for _, e := range s.AITranscript() {
		switch {
		case e.User != "":
			users++
		case e.Event.Kind == ai.EventNotice && e.Event.Error:
			errs++
		case e.Event.Kind == ai.EventDone:
			dones++
		}
	}
	if users != 1 || errs != 0 || dones != 1 {
		t.Fatalf("affichage : %d messages, %d erreurs, %d fins", users, errs, dones)
	}
	if d := h.lastDone(2); d.Changes != 1 || !d.Undo {
		t.Fatalf("done : %+v", d)
	}
	if err := s.AIResume(ctx, 2); !errors.Is(err, ai.ErrNothingToResume) {
		t.Fatalf("reprise d'une réponse complète : %v", err)
	}
	if _, err := s.AIUndoRun(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sheetNames(t, s), "Analyse") {
		t.Fatal("feuille gardée")
	}

	// Écriture sans effet (volets déjà libres) : aucun bilan.
	p.replies = []ai.Reply{
		{Stop: ai.StopToolUse, Message: ai.Message{Role: ai.Assistant, Content: []ai.Block{
			{Type: ai.ToolUseBlock, ID: "f", Name: "sheets_freeze", Input: json.RawMessage(`{"rows":0,"cols":0}`)},
		}}},
		{Stop: ai.StopEnd, Message: ai.Message{Role: ai.Assistant, Content: []ai.Block{{Type: ai.TextBlock, Text: "Fait."}}}},
	}
	if err := s.AISend(ctx, 3, "Libère les volets", AIContext{}); err != nil {
		t.Fatal(err)
	}
	if d := h.lastDone(3); d.Changes != 0 || d.Undo {
		t.Fatalf("done : %+v", d)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, e := range h.events {
		if e.Kind == ai.EventToolDone && e.ID == "f" && (e.Error || !e.Write) {
			t.Fatalf("outil : %+v", e.Event)
		}
	}
}

// blockProvider attend l'arrêt (AIStop) avant de répondre, après un début
// de texte.
type blockProvider struct{ started chan struct{} }

func (p *blockProvider) Info() ai.Info {
	return ai.Info{ID: "essai-arret", Name: "Essai", Available: true, DefaultModel: "m"}
}

func (p *blockProvider) Turn(ctx context.Context, _ ai.Credentials, _ ai.Request, emit func(ai.Event)) (ai.Reply, error) {
	emit(ai.Event{Kind: ai.EventText, Text: "Je commen"})
	close(p.started)
	<-ctx.Done()
	return ai.Reply{}, ctx.Err()
}

// Échap pendant le premier tour : le message, jamais reçu par le modèle,
// quitte la conversation et l'affichage, sans erreur (M3, M14) ; une
// conversation reprise de la liste n'a plus de réponse à annuler.
func TestAIStopFirstTurnAndReopen(t *testing.T) {
	workRoot(t)
	defer config.SetCurrent(config.Current())
	config.SetCurrent(config.Resolve(context.Background(), config.Options{}, &config.TreeSource{N: "essai", R: config.RankUser,
		T: config.Tree{"ai": config.Tree{"enabled": true, "provider": "essai-arret"}}}))
	bp := &blockProvider{started: make(chan struct{})}
	ai.Register(bp)
	ctx := context.Background()
	s := &DocumentService{}
	h := &aiHost{}
	UseHost(h, nil, s)
	t.Cleanup(func() { s.CloseDocument(ctx) })
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	go func() {
		<-bp.started
		s.AIStop()
	}()
	if err := s.AISend(ctx, 1, "Bonjour", AIContext{}); err != nil {
		t.Fatalf("arrêt rendu en erreur : %v", err)
	}
	if len(s.AITranscript()) != 0 || s.AIState().Messages != 0 {
		t.Fatalf("message gardé : %+v", s.AITranscript())
	}
	h.mu.Lock()
	last := h.events[len(h.events)-1]
	h.mu.Unlock()
	if last.Kind != ai.EventUnsent {
		t.Fatalf("dernier évènement : %+v", last)
	}

	// Conversation mise de côté puis reprise : ses réponses ne s'annulent
	// plus d'un clic (Ctrl+Z reste possible).
	aiConfig(false)
	p := &scriptProvider{replies: addSheet("Analyse")}
	ai.Register(p)
	if err := s.AISend(ctx, 2, "Crée une feuille", AIContext{}); err != nil {
		t.Fatal(err)
	}
	if err := s.AINewConversation(); err != nil {
		t.Fatal(err)
	}
	if err := s.AIOpenConversation(s.AIConversations()[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AIUndoRun(ctx, 2); err == nil {
		t.Fatal("réponse d'une conversation reprise annulée")
	}
	for _, e := range s.AITranscript() {
		if e.Event != nil && e.Event.Kind == ai.EventDone && e.Event.Undo {
			t.Fatalf("réponse annulable après reprise : %+v", e.Event)
		}
	}
	if !strings.Contains(sheetNames(t, s), "Analyse") {
		t.Fatal("feuille perdue")
	}
}

// lastDone : le dernier évènement done de la réponse run.
func (h *aiHost) lastDone(run int) ai.Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.events) - 1; i >= 0; i-- {
		if e := h.events[i]; e.Run == run && e.Kind == ai.EventDone {
			return e.Event
		}
	}
	return ai.Event{}
}
