package canvas

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// Anciens onglets qui portent autre chose que le tableau regroupé (un
// second tableau) : « Supprimer » les garde, et le dit (aperçu, barre
// d'état) ; les autres partent ; l'onglet gardé se supprime ensuite à la
// main.
func TestMergeSheetsKeptSheets(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	var tables []string
	var sheets []int64
	for i, s := range []string{"Janvier", "Février", "Mars"} {
		id := sheetWith(t, d, s, [][]string{{"Client", "Montant"}, {"A", "1"}, {"B", "2"}})
		sheets = append(sheets, id)
		tables = append(tables, tableOn(t, d, id))
		if i == 0 {
			if _, err := d.Paste(ctx, id, 0, 5, [][]string{{"Remise", "Taux"}, {"Or", "5"}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Une couleur déjà posée est remplacée par l'orange ; l'annulation la rend.
	must(t, d.SetSheetColor(ctx, sheets[0], "#0070c0"))
	plan := MergePlan{Tables: tables, Name: "Ventes", Old: MergeOldDelete}
	p, err := d.MergePreview(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Kept) != 1 || !strings.Contains(p.Kept[0], "« Janvier » gardée (onglet en orange)") || !strings.Contains(p.Kept[0], "le tableau Tableau1") {
		t.Fatalf("aperçu, onglets gardés : %q", p.Kept)
	}
	res, err := d.MergeSheets(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "regroupement")
	if !strings.Contains(res.Notice, "« Janvier » gardée") || !slices.Contains(res.Notes, p.Kept[0]) {
		t.Fatalf("explication : %q %q", res.Notice, res.Notes)
	}
	l, _ := d.Layout(ctx, res.Sheet)
	var names []string
	for _, s := range l.Sheets {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"Ventes", "Janvier"}) {
		t.Fatalf("onglets : %v", names)
	}
	if s := l.Sheets[1]; s.Color != KeptSheetColor || s.Hidden {
		t.Fatalf("onglet gardé : %+v", s)
	}
	_, err = d.bus.Undo(ctx)
	must(t, err)
	l, _ = d.Layout(ctx, sheets[0])
	for _, s := range l.Sheets {
		if s.ID == sheets[0] && s.Color != "#0070c0" {
			t.Fatalf("couleur après annulation : %q", s.Color)
		}
	}
	_, err = d.bus.Redo(ctx)
	must(t, err)
	if err := d.DeleteSheet(ctx, sheets[0]); err != nil {
		t.Fatalf("suppression à la main de l'onglet gardé : %v", err)
	}
	check(t, d, "suppression à la main")
}
