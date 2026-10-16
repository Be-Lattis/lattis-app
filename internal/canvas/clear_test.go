package canvas

import (
	"context"
	"strings"
	"testing"
)

// Supprimer sur des cellules libres vide tout : valeurs, mise en forme,
// fusions et notes ; une fusion qui déborde est défaite ; Ctrl+Z rétablit.
func TestClearCells(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.SetCell(ctx, 1, 1, 1, "bonjour"); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 0, 0, 3, 3, Style{"bg": "#ff0000"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.MergeCells(ctx, 1, 1, 1, 2, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := d.MergeCells(ctx, 1, 5, 0, 5, 3); err != nil {
		t.Fatal(err)
	}
	if err := d.SetNote(ctx, 1, 0, 0, "note"); err != nil {
		t.Fatal(err)
	}
	if err := d.ClearCells(ctx, 1, 0, 0, 5, 1); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Cells) != 0 {
		t.Fatalf("valeurs restantes : %+v", l.Cells)
	}
	for _, s := range l.Styles {
		if s.Row <= 5 && s.Col <= 1 {
			t.Fatalf("mise en forme restante en %d,%d : %v", s.Row, s.Col, s.Style)
		}
		if strings.Contains(s.Style, `"mr"`) {
			t.Fatalf("fusion restante en %d,%d", s.Row, s.Col)
		}
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if l, _ := d.Layout(ctx, 1); len(l.Cells) != 1 {
		t.Fatalf("annulation : %+v", l.Cells)
	}
}
