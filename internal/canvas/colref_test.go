package canvas

import (
	"context"
	"fmt"
	"testing"
)

func refsText(t *testing.T, d *fixture) string {
	t.Helper()
	refs, err := d.References(context.Background())
	must(t, err)
	return fmt.Sprint(refs)
}

// Une validation de toute une colonne tirée de la clé d'un autre tableau en
// fait une référence (décision 0016) : la ligne visée se retrouve sans
// casse, les lignes qui citent une valeur se comptent ; la référence suit
// le renommage du tableau visé, disparaît avec sa clé ou la validation, et
// une colonne qui n'est pas la clé n'en fait pas une.
func TestReferenceFromValidation(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom", "Famille"}, {"Pie", "Corvidés"}, {"Merle", "Turdidés"}})
	must(t, err)
	must(t, d.RenameTable(ctx, "Feuille1", "Especes"))
	_, err = d.Paste(ctx, 1, 0, 5, [][]string{{"Nid", "Espece", "Note"}, {"N1", "Pie", "a"}, {"N2", "pie", "b"}, {"N3", "Geai", "c"}})
	must(t, err)
	all, err := d.AllTables(ctx)
	must(t, err)
	for _, tb := range all {
		if tb.Name != "Especes" {
			must(t, d.RenameTable(ctx, tb.Name, "Nids"))
		}
	}
	// Sans clé sur Especes : pas de référence.
	must(t, d.ApplyStyle(ctx, 1, 1, 6, 3, 6, Style{"dv": "=Especes[Nom]"}))
	if got := refsText(t, d); got != "[]" {
		t.Fatalf("sans clé : %s", got)
	}
	must(t, d.SetKey(ctx, "Especes", 0))
	if got := refsText(t, d); got != "[{Nids Espece Especes Nom}]" {
		t.Fatalf("références : %s", got)
	}
	// Colonne qui n'est pas la clé : pas une référence.
	must(t, d.ApplyStyle(ctx, 1, 1, 7, 3, 7, Style{"dv": "=Especes[Famille]"}))
	if got := refsText(t, d); got != "[{Nids Espece Especes Nom}]" {
		t.Fatalf("colonne non clé : %s", got)
	}
	if r, err := d.ReferenceRow(ctx, "Especes[Famille]", "Corvidés"); err != nil || r.Table != "" {
		t.Fatalf("source non clé : %+v %v", r, err)
	}
	// Basculer sur : ligne retrouvée sans casse ; valeur absente.
	r, err := d.ReferenceRow(ctx, "Especes[Nom]", "PIE")
	must(t, err)
	if r.Table != "Especes" || r.Key != "Nom" || !r.Found {
		t.Fatalf("ligne visée : %+v", r)
	}
	spot, err := d.LocateRow(ctx, r.Table, r.ID)
	must(t, err)
	if spot.Row != 1 || spot.Col != 0 {
		t.Fatalf("ligne située : %+v", spot)
	}
	if r, _ := d.ReferenceRow(ctx, "Especes[Nom]", "Geai"); r.Found || r.Table != "Especes" {
		t.Fatalf("valeur absente : %+v", r)
	}
	// Lignes qui citent « Pie » : deux (sans casse), la première est N1.
	cs, err := d.Citations(ctx, "Especes", "Pie")
	must(t, err)
	if len(cs) != 1 || cs[0].Table != "Nids" || cs[0].Column != "Espece" || cs[0].Count != 2 {
		t.Fatalf("citations : %+v", cs)
	}
	spot, err = d.LocateRow(ctx, "Nids", cs[0].First)
	must(t, err)
	if spot.Row != 1 {
		t.Fatalf("première ligne qui cite : %+v", spot)
	}
	// Renommage du tableau visé : la référence suit.
	must(t, d.RenameTable(ctx, "Especes", "Oiseaux"))
	if got := refsText(t, d); got != "[{Nids Espece Oiseaux Nom}]" {
		t.Fatalf("après renommage : %s", got)
	}
	// Clé retirée : plus de référence ; Ctrl+Z la rend.
	must(t, d.ClearKey(ctx, "Oiseaux"))
	if got := refsText(t, d); got != "[]" {
		t.Fatalf("clé retirée : %s", got)
	}
	_, err = d.bus.Undo(ctx)
	must(t, err)
	if got := refsText(t, d); got != "[{Nids Espece Oiseaux Nom}]" {
		t.Fatalf("clé rendue : %s", got)
	}
	// Validation retirée : plus de référence.
	must(t, d.ApplyStyle(ctx, 1, 1, 6, 3, 6, Style{"dv": nil}))
	if got := refsText(t, d); got != "[]" {
		t.Fatalf("validation retirée : %s", got)
	}
}
