package canvas

import (
	"context"
	"fmt"
	"testing"
)

// Une colonne calculée sert de champ de TCD (en valeurs comme en lignes) ;
// la somme garde son format (€).
func TestPivotComputedColumn(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix", "Qté"}, {"Clavier", "25,5", "2"}, {"Souris", "10", "3"}, {"Clavier", "25,5", "1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 3, "Montant"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 3, "=[@Prix]*[@Qté]"); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 1, 3, 3, 3, Style{"nf": "€"}); err != nil {
		t.Fatal(err)
	}
	sheet, err := d.CreatePivot(ctx, PivotConfig{Source: "Feuille1", RowFields: []string{"Produit"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}})
	if err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, sheet)
	p := l.Pivots[0]
	if p.Error != "" {
		t.Fatal(p.Error)
	}
	if got := fmt.Sprint(p.Cells); got != "[[Produit Somme de Montant] [Clavier 76,5] [Souris 30] [Total 106,5]]" {
		t.Fatalf("TCD : %s", got)
	}
	if fmt.Sprint(p.Formats) != `[ {"nf":"€"}]` {
		t.Fatalf("format : %v", p.Formats)
	}
	// En lignes, et deux fois la même colonne (lignes et valeurs).
	if err := d.SetPivot(ctx, p.ID, PivotConfig{Source: "Feuille1", RowFields: []string{"Montant"}, Values: []PivotValue{{Field: "Montant", Agg: "count"}}}); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, sheet)
	if p := l.Pivots[0]; p.Error != "" || len(p.Cells) != 5 {
		t.Fatalf("Montant en lignes : %q %v", p.Error, p.Cells)
	}
}
