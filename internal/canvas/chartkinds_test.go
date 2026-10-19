package canvas

import (
	"context"
	"fmt"
	"testing"
)

func TestChartNewKinds(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes", "Coût"}, {"FR", "10", "4"}, {"IT", "5", "2"}, {"FR", "7", "1"}}); err != nil {
		t.Fatal(err)
	}
	bad := []ChartConfig{
		{Source: "Feuille1", Kind: "combo", X: "Pays", Series: []string{"Ventes"}, SeriesKinds: []string{"pie"}},
		{Source: "Feuille1", Kind: "map", X: "Pays", Series: []string{"Ventes"}},
		{Source: "Feuille1", Kind: "map", X: "Pays", Series: []string{"Ventes", "Coût"}, MapMode: "relief"},
		{Source: "Feuille1", Kind: "histogram", X: "Pays", Series: []string{"Ventes"}, Bins: 500},
	}
	for _, c := range bad {
		if _, err := d.CreateChart(ctx, 1, 0, 5, c); err == nil {
			t.Fatalf("accepté : %+v", c)
		}
	}
	for k := range chartKinds {
		if err := checkChart(ChartConfig{Kind: k, Series: []string{"a", "b"}}); err != nil {
			t.Fatalf("%s : %v", k, err)
		}
	}
	// Combiné : regroupé par catégorie comme un histogramme.
	if _, err := d.CreateChart(ctx, 1, 0, 5, ChartConfig{Source: "Feuille1", Kind: "combo", X: "Pays", Series: []string{"Ventes", "Coût"}, SeriesKinds: []string{"bar", "line"}}); err != nil {
		t.Fatal(err)
	}
	// Distribution : toutes les lignes, sans regrouper.
	if _, err := d.CreateChart(ctx, 1, 10, 5, ChartConfig{Source: "Feuille1", Kind: "boxplot", X: "Pays", Series: []string{"Ventes"}}); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	if c := l.Charts[0]; fmt.Sprint(c.Labels) != "[FR IT]" || *c.Series[0].Values[0] != 17 {
		t.Fatalf("combiné : %+v", c)
	}
	// Boîtes calculées par le classeur : une par pays (FR se répète).
	if c := l.Charts[1]; fmt.Sprint(c.Labels) != "[FR IT]" || c.Boxes[0][0].Median != 8.5 || c.Boxes[1][0].N != 1 {
		t.Fatalf("boîte à moustaches : %+v", c)
	}
}
