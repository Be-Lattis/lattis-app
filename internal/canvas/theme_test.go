package canvas

import (
	"context"
	"testing"
)

func TestWorkbookTheme(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if th, err := d.Theme(ctx); err != nil || th != nil {
		t.Fatalf("classeur neuf : %+v %v", th, err)
	}
	must(t, d.SetTheme(ctx, &Theme{ID: "bleu-orange", Name: " Bleu et orange ", Table: "#2563EB", Source: "#ea580c"}))
	th, err := d.Theme(ctx)
	must(t, err)
	if th == nil || th.Name != "Bleu et orange" || th.Table != "#2563eb" || th.Source != "#ea580c" || th.ID != "bleu-orange" {
		t.Fatalf("thème : %+v", th)
	}
	if err := d.SetTheme(ctx, &Theme{Name: "x", Table: "vert", Source: "#000000"}); err == nil {
		t.Fatal("couleur invalide acceptée")
	}
	// Retirer le thème, puis annuler : il revient.
	must(t, d.SetTheme(ctx, nil))
	if th, _ := d.Theme(ctx); th != nil {
		t.Fatalf("thème retiré : %+v", th)
	}
	d.bus.Undo(ctx)
	if th, _ := d.Theme(ctx); th == nil || th.ID != "bleu-orange" {
		t.Fatalf("retrait annulé : %+v", th)
	}
	d.bus.Undo(ctx)
	if th, _ := d.Theme(ctx); th != nil {
		t.Fatalf("choix annulé : %+v", th)
	}
	if ParseTheme(`{"name":"x","table":"#zzzzzz","source":"#000000"}`) != nil || ParseTheme("pas du json") != nil {
		t.Fatal("thème illisible accepté")
	}
}
