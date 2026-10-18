package bridge

import (
	"slices"
	"testing"
)

// Plusieurs onglets sélectionnés : supprimés ou rangés en une seule étape
// d'annulation.
func TestSheetsGroup(t *testing.T) {
	s, ctx := eventsService(t)
	ids := []int64{}
	for range 3 {
		id, err := s.AddSheet(ctx)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	order := func() []int64 {
		l, err := s.Layout(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		var out []int64
		for _, sh := range l.Sheets {
			out = append(out, sh.ID)
		}
		return out
	}
	start := order() // 1, a, b, c
	want := append(slices.Clone(start[2:]), start[:2]...)
	if _, err := s.OrderSheets(ctx, want); err != nil {
		t.Fatal(err)
	}
	if got := order(); !slices.Equal(got, want) {
		t.Fatalf("ordre : %v, attendu %v", got, want)
	}
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := order(); !slices.Equal(got, start) {
		t.Fatalf("après annulation : %v, attendu %v", got, start)
	}
	if _, err := s.DeleteSheets(ctx, ids[:2]); err != nil {
		t.Fatal(err)
	}
	if got := order(); len(got) != len(start)-2 {
		t.Fatalf("après suppression : %v", got)
	}
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := order(); !slices.Equal(got, start) {
		t.Fatalf("suppression annulée : %v, attendu %v", got, start)
	}
}
