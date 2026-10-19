package bridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/ai"
	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/config"
)

// La structure change hors des réponses (tableau collé par l'utilisateur) :
// elle est jointe de nouveau au message suivant ; les modifications de
// l'assistant lui-même ne la font pas renvoyer (I7).
func TestAIStructureResent(t *testing.T) {
	workRoot(t)
	defer config.SetCurrent(config.Current())
	aiConfig(false)
	p := &scriptProvider{}
	ai.Register(p)
	ctx := context.Background()
	s := &DocumentService{}
	UseHost(&aiHost{}, nil, s)
	t.Cleanup(func() { s.CloseDocument(ctx) })
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	final := func(text string) ai.Reply {
		return ai.Reply{Stop: ai.StopEnd, Message: ai.Message{Role: ai.Assistant, Content: []ai.Block{{Type: ai.TextBlock, Text: text}}}}
	}
	send := func(run int, text string) string {
		t.Helper()
		if err := s.AISend(ctx, run, text, AIContext{}); err != nil {
			t.Fatal(err)
		}
		return p.last
	}

	p.replies = []ai.Reply{final("Bonjour.")}
	send(1, "Bonjour")
	if _, err := api.Invoke(ctx, host{s}, "cells.paste", map[string]any{"row": 1, "col": 1, "values": [][]any{{"Ville", "CA"}, {"Lyon", 3.0}, {"Nantes", 4.0}}}); err != nil {
		t.Fatal(err)
	}
	p.replies = []ai.Reply{final("Vu.")}
	if m := send(2, "Analyse le nouveau tableau"); !strings.HasPrefix(m, "[Structure modifiée") || !strings.Contains(m, "Ville (text)") {
		t.Fatalf("message envoyé : %q", m)
	}
	p.replies = []ai.Reply{final("Rien de neuf.")}
	if m := send(3, "Et ensuite ?"); strings.Contains(m, "[Structure") {
		t.Fatalf("structure renvoyée sans changement : %q", m)
	}
	// Tableau ajouté par l'assistant : connu de lui, pas renvoyé.
	p.replies = []ai.Reply{
		{Stop: ai.StopToolUse, Message: ai.Message{Role: ai.Assistant, Content: []ai.Block{
			{Type: ai.ToolUseBlock, ID: "c", Name: "cells_paste", Input: json.RawMessage(`{"row":1,"col":5,"values":[["Client","Montant"],["A",1]]}`)},
		}}},
		final("Tableau créé."),
	}
	send(4, "Crée un tableau Clients")
	p.replies = []ai.Reply{final("D'accord.")}
	if m := send(5, "Merci"); strings.Contains(m, "[Structure") {
		t.Fatalf("structure renvoyée après une modification de l'assistant : %q", m)
	}
}
