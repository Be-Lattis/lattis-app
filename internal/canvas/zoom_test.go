package canvas

import (
	"context"
	"testing"
)

func TestZoom(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	l, _ := d.Layout(ctx, 1)
	if l.Sheets[0].Zoom != 100 {
		t.Fatalf("zoom par défaut : %d", l.Sheets[0].Zoom)
	}
	before := d.ProtectionChanges()
	if err := d.SetZoom(ctx, 1, 150); err != nil {
		t.Fatal(err)
	}
	if l, _ = d.Layout(ctx, 1); l.Sheets[0].Zoom != 150 {
		t.Fatalf("zoom : %d", l.Sheets[0].Zoom)
	}
	// Hors du journal, mais le classeur est « modifié ».
	if d.ProtectionChanges() == before {
		t.Fatal("le zoom ne rend pas le classeur modifié")
	}
	if d.bus.Undo(ctx); func() int { l, _ := d.Layout(ctx, 1); return l.Sheets[0].Zoom }() != 150 {
		t.Fatal("le zoom s'est annulé")
	}
	// Même zoom : rien ne change.
	n := d.ProtectionChanges()
	if err := d.SetZoom(ctx, 1, 150); err != nil || d.ProtectionChanges() != n {
		t.Fatalf("même zoom : %v", err)
	}
	for _, bad := range []int{0, 9, 401} {
		if err := d.SetZoom(ctx, 1, bad); err == nil {
			t.Fatalf("zoom %d accepté", bad)
		}
	}
	if err := d.SetZoom(ctx, 99, 120); err == nil {
		t.Fatal("feuille inconnue acceptée")
	}

	// La copie garde le zoom ; une feuille supprimée le retrouve à l'annulation.
	cp, err := d.CopySheet(ctx, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if l, _ = d.Layout(ctx, cp); l.Sheets[1].Zoom != 150 {
		t.Fatalf("zoom de la copie : %+v", l.Sheets)
	}
	if err := d.DeleteSheet(ctx, cp); err != nil {
		t.Fatal(err)
	}
	d.bus.Undo(ctx)
	if l, _ = d.Layout(ctx, cp); len(l.Sheets) != 2 || l.Sheets[1].Zoom != 150 {
		t.Fatalf("zoom de la feuille rendue : %+v", l.Sheets)
	}
	check(t, d, "zoom")
}
