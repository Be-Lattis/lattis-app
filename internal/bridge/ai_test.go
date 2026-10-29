package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/ai"
	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/config"
)

// scriptProvider rejoue des réponses écrites d'avance et garde le dernier
// message de l'utilisateur reçu.
type scriptProvider struct {
	mu      sync.Mutex
	replies []ai.Reply
	last    string
}

func (p *scriptProvider) Info() ai.Info {
	return ai.Info{ID: "essai-pont", Name: "Essai", Available: true, DefaultModel: "claude-sonnet-5-5"}
}

func (p *scriptProvider) Turn(_ context.Context, _ ai.Credentials, req ai.Request, emit func(ai.Event)) (ai.Reply, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	m := req.Messages[len(req.Messages)-1]
	for _, b := range m.Content {
		if b.Type == ai.TextBlock {
			p.last = b.Text
		}
	}
	if len(p.replies) == 0 {
		return ai.Reply{}, errors.New("plus de réponse prévue")
	}
	r := p.replies[0]
	p.replies = p.replies[1:]
	for _, b := range r.Message.Content {
		if b.Type == ai.TextBlock {
			emit(ai.Event{Kind: ai.EventText, Text: b.Text})
		}
	}
	return r, nil
}

// addSheet : réponse qui ajoute une feuille, puis réponse finale.
func addSheet(name string) []ai.Reply {
	return []ai.Reply{
		{Stop: ai.StopToolUse, Usage: ai.Usage{InputTokens: 1000, OutputTokens: 10}, Message: ai.Message{Role: ai.Assistant, Content: []ai.Block{
			{Type: ai.TextBlock, Text: "J'ajoute "},
			{Type: ai.TextBlock, Text: "la feuille."},
			{Type: ai.ToolUseBlock, ID: "t-" + name, Name: "sheets_add", Input: json.RawMessage(`{"name":"` + name + `"}`)},
		}}},
		{Stop: ai.StopEnd, Usage: ai.Usage{InputTokens: 1100, OutputTokens: 5}, Message: ai.Message{Role: ai.Assistant, Content: []ai.Block{{Type: ai.TextBlock, Text: "Fait."}}}},
	}
}

// aiHost relève les évènements de l'assistant.
type aiHost struct {
	NoHost
	mu     sync.Mutex
	events []AIEventData
	resets int
}

func (h *aiHost) Emit(event string, data any) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch event {
	case AIEvent:
		h.events = append(h.events, data.(AIEventData))
	case AIResetEvent:
		h.resets++
	}
	return true
}

func (h *aiHost) done(run int) ai.Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, e := range h.events {
		if e.Run == run && e.Kind == ai.EventDone {
			return e.Event
		}
	}
	return ai.Event{}
}

func aiConfig(keep bool) {
	config.SetCurrent(config.Resolve(context.Background(), config.Options{}, &config.TreeSource{N: "essai", R: config.RankUser,
		T: config.Tree{"ai": config.Tree{"enabled": true, "provider": "essai-pont", "keepConversation": keep}}}))
}

func sheetNames(t *testing.T, s *DocumentService) string {
	t.Helper()
	res, err := api.Invoke(context.Background(), host{s}, "sheets.list", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res)
	return string(b)
}

func TestAIConversation(t *testing.T) {
	workRoot(t)
	defer config.SetCurrent(config.Current())
	aiConfig(true)
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
	if h.resets != 1 {
		t.Fatalf("%d ai:reset", h.resets)
	}

	// 1. Contexte joint au message, invisible dans l'affichage.
	p.replies = addSheet("Analyse")
	if err := s.AISend(ctx, 1, "Crée une feuille", AIContext{Sheet: "Feuille1", Selection: "B2:D5"}); err != nil {
		t.Fatal(err)
	}
	if p.last != "[Contexte : feuille active « Feuille1 », sélection B2:D5, aujourd'hui "+time.Now().Format("2006-01-02")+"]\n\nCrée une feuille" {
		t.Fatalf("message envoyé : %q", p.last)
	}
	if d := h.done(1); d.Changes != 1 || !d.Undo {
		t.Fatalf("done : %+v", d)
	}
	st := s.AIState()
	// Tarif public d'Anthropic : pas pour un autre fournisseur, même avec
	// un modèle de même nom (M11).
	if st.Usage.InputTokens != 2100 || st.Usage.Cost != 0 || !st.KeepConversation || st.Messages != 4 {
		t.Fatalf("état : %+v", st)
	}
	tr := s.AITranscript()
	if len(tr) == 0 || tr[0].User != "Crée une feuille" || tr[0].Run != 1 {
		t.Fatalf("transcription : %+v", tr)
	}
	// Morceaux de texte réunis.
	var texts []string
	for _, e := range tr {
		if e.Event != nil && e.Event.Kind == ai.EventText {
			texts = append(texts, e.Event.Text)
		}
	}
	if len(texts) != 2 || texts[0] != "J'ajoute la feuille." {
		t.Fatalf("textes : %q", texts)
	}
	for _, e := range tr {
		if e.Event != nil && e.Event.Kind == ai.EventToolDone && (!e.Event.Write || e.Event.Target != "Analyse") {
			t.Fatalf("outil : %+v", e.Event)
		}
	}

	// 2. « Annuler cette réponse », signalé au modèle au message suivant.
	if _, err := s.AIUndoRun(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sheetNames(t, s), "Analyse") {
		t.Fatal("feuille gardée")
	}
	if _, err := s.AIUndoRun(ctx, 1); err == nil {
		t.Fatal("annulée deux fois")
	}
	if tr := s.AITranscript(); tr[len(tr)-1].Event.Kind != ai.EventDone || tr[len(tr)-1].Event.Undo {
		t.Fatalf("réponse annulée encore annulable : %+v", tr[len(tr)-1].Event)
	}
	p.replies = addSheet("Bilan")
	if err := s.AISend(ctx, 2, "Fais plutôt un bilan", AIContext{}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.last, "[Remarque : L'utilisateur a annulé les modifications de votre réponse précédente.]\n[Contexte : aujourd'hui ") || !strings.HasSuffix(p.last, "]\n\nFais plutôt un bilan") {
		t.Fatalf("message envoyé : %q", p.last)
	}

	// 3. Ctrl+Z de l'utilisateur, repéré au message suivant.
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	p.replies = []ai.Reply{{Stop: ai.StopEnd, Message: ai.Message{Role: ai.Assistant, Content: []ai.Block{{Type: ai.TextBlock, Text: "D'accord."}}}}}
	if err := s.AISend(ctx, 3, "Et maintenant ?", AIContext{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.last, "annulé (Ctrl+Z) les modifications de votre réponse précédente") {
		t.Fatalf("message envoyé : %q", p.last)
	}
	if d := h.done(3); d.Changes != 0 || d.Undo {
		t.Fatalf("done : %+v", d)
	}

	// 4. Une modification de l'utilisateur après la réponse : plus
	// d'annulation de la réponse seule.
	p.replies = addSheet("Essai")
	if err := s.AISend(ctx, 4, "Ajoute Essai", AIContext{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetCell(ctx, 1, 0, 0, "42"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AIUndoRun(ctx, 4); !errors.Is(err, errAIUndoLater) {
		t.Fatalf("err %v", err)
	}

	// 5. Conversation gardée avec le classeur, restaurée à l'ouverture.
	path := filepath.Join(t.TempDir(), "essai.lattis")
	if err := (host{s}).SaveDoc(ctx, path); err != nil {
		t.Fatal(err)
	}
	usage := s.AIState().Usage
	if err := s.CloseDocument(ctx); err != nil {
		t.Fatal(err)
	}
	if len(s.AITranscript()) != 0 || s.AIState().Messages != 0 || h.resets != 2 {
		t.Fatalf("fermeture : %d ai:reset, %d entrées", h.resets, len(s.AITranscript()))
	}
	reopen(t, s, path)
	tr = s.AITranscript()
	if len(tr) == 0 || tr[0].User != "Crée une feuille" || s.AIState().Messages == 0 || s.AIState().Usage != usage {
		t.Fatalf("restauration : %d entrées, %+v", len(tr), s.AIState())
	}
	// La conversation reprend dans la forme neutre.
	p.replies = []ai.Reply{{Stop: ai.StopEnd, Message: ai.Message{Role: ai.Assistant, Content: []ai.Block{{Type: ai.TextBlock, Text: "Rebonjour."}}}}}
	if err := s.AISend(ctx, 5, "Tu te souviens ?", AIContext{}); err != nil {
		t.Fatal(err)
	}

	// 6. Réglage coupé : rien n'est restauré, et l'enregistrement retire la
	// conversation du fichier.
	aiConfig(false)
	s.CloseDocument(ctx)
	reopen(t, s, path)
	if len(s.AITranscript()) != 0 || s.AIState().Messages != 0 {
		t.Fatal("conversation restaurée sans le réglage")
	}
	if _, err := s.SetCell(ctx, 1, 1, 1, "x"); err != nil {
		t.Fatal(err)
	}
	if err := (host{s}).SaveDoc(ctx, path); err != nil {
		t.Fatal(err)
	}
	s.CloseDocument(ctx)
	aiConfig(true)
	reopen(t, s, path)
	if len(s.AITranscript()) != 0 {
		t.Fatal("conversation gardée dans le fichier sans le réglage")
	}
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.AIReset(); err != nil {
		t.Fatal(err)
	}
}

// reopen rouvre path. Fermé juste avant, le classeur peut laisser une
// session vue comme à récupérer (selon l'horloge du fichier) : on la jette,
// comme l'utilisateur qui rouvre le fichier, au lieu de continuer sans
// classeur ouvert (M9).
func reopen(t *testing.T, s *DocumentService, path string) {
	t.Helper()
	ctx := context.Background()
	res, err := s.Open(ctx, path)
	if err == nil && res.Recoverable != nil {
		if err = s.Discard(res.Recoverable.Dir); err == nil {
			res, err = s.Open(ctx, path)
		}
	}
	if err != nil || res.Recoverable != nil || res.Damaged != nil {
		t.Fatalf("réouverture : %v %+v %+v", err, res.Recoverable, res.Damaged)
	}
}

// Une réponse qui démarre empêche de vider la conversation (M13).
func TestAIResetBusy(t *testing.T) {
	s := &DocumentService{}
	s.ai.cancel = func() {}
	if err := s.AIReset(); !errors.Is(err, ai.ErrBusy) {
		t.Fatalf("err %v", err)
	}
	if err := s.AIDeleteConversation(0); !errors.Is(err, ai.ErrBusy) {
		t.Fatalf("err %v", err)
	}
	s.ai.cancel = nil
	if err := s.AIReset(); err != nil {
		t.Fatal(err)
	}
}

func TestAISetKeyChecked(t *testing.T) {
	defer config.SetCurrent(config.Current())
	aiConfig(false)
	s := &DocumentService{}
	if _, err := s.AISetKey(context.Background(), "inconnu", "x"); err == nil {
		t.Fatal("fournisseur inconnu accepté")
	}
	if _, err := s.AISetKey(context.Background(), "anthropic", "  "); err == nil {
		t.Fatal("clé vide acceptée")
	}
}

// Nouvelle conversation : l'ancienne reste dans la liste et se reprend ;
// une conversation se supprime.
func TestAIConversations(t *testing.T) {
	workRoot(t)
	defer config.SetCurrent(config.Current())
	aiConfig(true)
	p := &scriptProvider{}
	ai.Register(p)
	ctx := context.Background()
	s := &DocumentService{}
	UseHost(&aiHost{}, nil, s)
	t.Cleanup(func() { s.CloseDocument(ctx) })
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	if l := s.AIConversations(); len(l) != 0 {
		t.Fatalf("liste de départ : %+v", l)
	}
	p.replies = addSheet("Analyse")
	if err := s.AISend(ctx, 1, "Crée une feuille d'analyse", AIContext{}); err != nil {
		t.Fatal(err)
	}
	if err := s.AINewConversation(); err != nil {
		t.Fatal(err)
	}
	l := s.AIConversations()
	if len(l) != 1 || l[0].Current || l[0].Title != "Crée une feuille d'analyse" || len(s.AITranscript()) != 0 {
		t.Fatalf("après Nouvelle conversation : %+v", l)
	}
	if err := s.AIOpenConversation(l[0].ID); err != nil {
		t.Fatal(err)
	}
	if tr := s.AITranscript(); len(tr) == 0 || tr[0].User != "Crée une feuille d'analyse" {
		t.Fatalf("conversation reprise : %+v", tr)
	}
	if l := s.AIConversations(); len(l) != 1 || !l[0].Current {
		t.Fatalf("liste après reprise : %+v", l)
	}
	if err := s.AIDeleteConversation(0); err != nil {
		t.Fatal(err)
	}
	if l := s.AIConversations(); len(l) != 0 {
		t.Fatalf("après suppression : %+v", l)
	}
}
