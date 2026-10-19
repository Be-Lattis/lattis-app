package ai

import (
	"context"
	"encoding/json"
	"testing"
)

// Arrêt à MaxSteps, puis reprise de la même réponse sans nouveau message de
// l'utilisateur (M4, M14).
func TestResumeAfterMaxSteps(t *testing.T) {
	call := Reply{Stop: StopToolUse, Message: Message{Role: Assistant, Content: []Block{{Type: ToolUseBlock, ID: "t", Name: "x", Input: json.RawMessage(`{}`)}}}}
	f := &fakeProvider{}
	for range MaxSteps {
		f.replies = append(f.replies, call)
	}
	f.replies = append(f.replies, Reply{Stop: StopEnd, Message: Message{Role: Assistant, Content: []Block{{Type: TextBlock, Text: "Fini."}}}})
	var s Session
	turn := Turn{Provider: f, Exec: func(context.Context, string, json.RawMessage) (string, bool) { return "ok", false }}
	limit := false
	if err := s.Send(context.Background(), turn, "Longue tâche", func(e Event) { limit = limit || e.Limit }); err != nil || !limit {
		t.Fatalf("err %v, arrêt signalé : %v", err, limit)
	}
	n := s.Len()
	if err := s.Resume(context.Background(), turn, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if len(f.seen) != MaxSteps+1 || s.Len() != n+1 {
		t.Fatalf("%d tours, %d messages (avant : %d)", len(f.seen), s.Len(), n)
	}
	// Le tour repris part des derniers résultats, sans texte ajouté.
	last := f.seen[MaxSteps].Messages
	if m := last[len(last)-1]; m.Role != User || len(m.Content) != 1 || m.Content[0].Type != ToolResultBlock {
		t.Fatalf("dernier message envoyé : %+v", m)
	}
	if err := s.Resume(context.Background(), turn, func(Event) {}); err != ErrNothingToResume {
		t.Fatalf("reprise d'une réponse complète : %v", err)
	}
}

// Réponse coupée à la longueur maximale : l'appel à l'entrée incomplète
// n'est pas exécuté et ne reste pas dans la conversation (M12).
func TestTruncatedToolCallDropped(t *testing.T) {
	f := &fakeProvider{replies: []Reply{
		{Stop: StopMaxTokens, Usage: Usage{InputTokens: 1000, OutputTokens: 100}, Message: Message{Role: Assistant, Provider: "fake", Native: "natif", Content: []Block{
			{Type: TextBlock, Text: "Je colle les valeurs."},
			{Type: ToolUseBlock, ID: "t1", Name: "cells_paste", Input: json.RawMessage(`{"row":1,"values":[[1,2],[3`)},
		}}},
	}}
	ran := 0
	var s Session
	turn := Turn{Provider: f, Model: "claude-sonnet-5-5", NoPrice: true, Exec: func(context.Context, string, json.RawMessage) (string, bool) {
		ran++
		return "ok", false
	}}
	var notices int
	if err := s.Send(context.Background(), turn, "Colle", func(e Event) {
		if e.Kind == EventNotice {
			notices++
		}
	}); err != nil {
		t.Fatal(err)
	}
	if ran != 0 || notices != 1 || len(f.seen) != 1 {
		t.Fatalf("%d appels exécutés, %d avis, %d tours", ran, notices, len(f.seen))
	}
	s.mu.Lock()
	last := s.history[len(s.history)-1]
	s.mu.Unlock()
	if len(last.Content) != 1 || last.Content[0].Type != TextBlock || last.Native != nil {
		t.Fatalf("message gardé : %+v", last)
	}
	// NoPrice : aucun coût calculé (M11).
	if u := s.Usage(); u.Cost != 0 {
		t.Fatalf("coût : %+v", u)
	}
}
