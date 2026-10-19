package ai

import (
	"context"
	"encoding/json"
	"testing"
)

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
