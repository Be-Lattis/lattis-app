package bridge

import (
	"testing"
)

// Bordure épaisse en encadré (ruban) : les quatre bords de la sélection en
// une seule étape d'annulation, styles « style couleur » gardés tels quels.
func TestApplyStyleParts(t *testing.T) {
	s, ctx := eventsService(t)
	styles := func() map[[2]int]string {
		l, err := s.Layout(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		out := map[[2]int]string{}
		for _, c := range l.Styles {
			out[[2]int{c.Row, c.Col}] = c.Style
		}
		return out
	}
	parts := []StylePart{
		{R0: 0, C0: 0, R1: 0, C1: 1, Patch: map[string]any{"bt": "thick #FF0000"}},
		{R0: 1, C0: 0, R1: 1, C1: 1, Patch: map[string]any{"bb": "thick #FF0000"}},
		{R0: 0, C0: 0, R1: 1, C1: 0, Patch: map[string]any{"bl": "thick #FF0000"}},
		{R0: 0, C0: 1, R1: 1, C1: 1, Patch: map[string]any{"br": "thick #FF0000", "nc": `0,00 "kg"`}},
	}
	if _, err := s.ApplyStyleParts(ctx, 1, parts); err != nil {
		t.Fatal(err)
	}
	got := styles()
	if got[[2]int{0, 0}] != `{"bl":"thick #FF0000","bt":"thick #FF0000"}` || got[[2]int{1, 1}] != `{"bb":"thick #FF0000","br":"thick #FF0000","nc":"0,00 \"kg\""}` {
		t.Fatalf("styles : %v", got)
	}
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := styles(); len(got) != 0 {
		t.Fatalf("une seule annulation : %v", got)
	}
}
