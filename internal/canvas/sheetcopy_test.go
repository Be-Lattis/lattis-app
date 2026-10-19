package canvas

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestCopySheet(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 2, "Double"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 2, "=[Ventes]*2"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 8, 6, "note"); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 8, 6, 8, 6, Style{"bold": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateChart(ctx, 1, 0, 8, ChartConfig{Source: "Feuille1", Kind: "bar", X: "Pays", Series: []string{"Ventes"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSheetColor(ctx, 1, "#ff0000"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	id, err := d.CopySheet(ctx, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "copie de feuille")
	l, err := d.Layout(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if l.Sheets[0].ID != id || l.Sheets[0].Name != "Feuille1 (2)" || l.Sheets[0].Color != "#ff0000" {
		t.Fatalf("onglets : %+v", l.Sheets)
	}
	if len(l.Tables) != 1 || l.Tables[0].Name != "Feuille1_2" || l.Tables[0].Rows != 2 || l.Tables[0].Columns[2].Formula != "=[Ventes]*2" {
		t.Fatalf("tableaux de la copie : %+v", l.Tables)
	}
	if len(l.Cells) != 1 || l.Cells[0].Raw != "note" || len(l.Styles) != 1 {
		t.Fatalf("cellules de la copie : %+v %+v", l.Cells, l.Styles)
	}
	if len(l.Charts) != 1 || l.Charts[0].Config.Source != "Feuille1_2" || fmt.Sprint(l.Charts[0].Labels) != "[FR IT]" {
		t.Fatalf("graphique de la copie : %+v", l.Charts)
	}
	// La copie est indépendante de l'original.
	if _, err := d.SetCell(ctx, id, 1, 1, "99"); err != nil {
		t.Fatal(err)
	}
	orig, _ := d.Layout(ctx, 1)
	if *orig.Charts[0].Series[0].Values[0] != 10 {
		t.Fatalf("l'original a changé : %+v", orig.Charts[0].Series)
	}
	// Une deuxième copie prend le nom suivant.
	id2, err := d.CopySheet(ctx, id, 99)
	if err != nil {
		t.Fatal(err)
	}
	l2, _ := d.Layout(ctx, id2)
	if l2.Sheets[2].Name != "Feuille1 (3)" || l2.Tables[0].Name != "Feuille1_3" {
		t.Fatalf("deuxième copie : %+v %s", l2.Sheets, l2.Tables[0].Name)
	}
	check(t, d, "deuxième copie")
	for range 3 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
	// Rétablir refait la copie.
	d.bus.Redo(ctx)
	check(t, d, "rétablissement")
	if l, _ := d.Layout(ctx, id); len(l.Tables) != 1 || l.Tables[0].Rows != 2 {
		t.Fatalf("après rétablissement : %+v", l.Tables)
	}
}

func TestHiddenSheets(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	before := dump(t, d)
	if err := d.SetSheetHidden(ctx, 1, true); !errors.Is(err, ErrSheet) {
		t.Fatalf("dernière feuille visible masquée : %v", err)
	}
	id, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetSheetHidden(ctx, 1, true); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, id)
	if !l.Sheets[0].Hidden || l.Sheets[1].Hidden {
		t.Fatalf("onglets : %+v", l.Sheets)
	}
	if err := d.SetSheetHidden(ctx, id, true); !errors.Is(err, ErrSheet) {
		t.Fatalf("toutes les feuilles masquées : %v", err)
	}
	if err := d.DeleteSheet(ctx, id); !errors.Is(err, ErrSheet) {
		t.Fatalf("dernière feuille visible supprimée : %v", err)
	}
	if err := d.SetSheetColor(ctx, id, "rouge"); !errors.Is(err, ErrSheet) {
		t.Fatalf("couleur invalide acceptée : %v", err)
	}
	if err := d.SetSheetHidden(ctx, 1, false); err != nil {
		t.Fatal(err)
	}
	// Supprimer puis annuler rend la couleur et l'état masqué.
	if err := d.SetSheetColor(ctx, id, "#00ff00"); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSheet(ctx, id); err != nil {
		t.Fatal(err)
	}
	d.bus.Undo(ctx)
	if l, _ := d.Layout(ctx, id); l.Sheets[1].Color != "#00ff00" {
		t.Fatalf("couleur perdue : %+v", l.Sheets)
	}
	check(t, d, "feuilles masquées")
	for range 4 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}
