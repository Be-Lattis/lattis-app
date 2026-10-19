package canvas

import (
	"context"
	"fmt"
	"testing"
)

// Un graphique ne montre que les lignes affichées par le filtre du tableau,
// sauf avec Hidden ; trier le tableau ensuite est suivi.
func TestChartFollowsFilter(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}, {"ES", "7"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateChart(ctx, 1, 0, 5, ChartConfig{Source: "Feuille1", Kind: "bar", X: "Pays", Series: []string{"Ventes"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateChart(ctx, 1, 20, 5, ChartConfig{Source: "Feuille1", Kind: "bar", X: "Pays", Series: []string{"Ventes"}, Hidden: true}); err != nil {
		t.Fatal(err)
	}
	labels := func(i int) string {
		l, err := d.Layout(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprint(l.Charts[i].Labels)
	}
	if got := labels(0); got != "[FR IT ES]" {
		t.Fatalf("avant le filtre : %s", got)
	}
	if err := d.SetFilter(ctx, "Feuille1", 0, &ColumnFilter{Values: []string{"FR", "ES"}}); err != nil {
		t.Fatal(err)
	}
	if got := labels(0); got != "[FR ES]" {
		t.Fatalf("filtré : %s", got)
	}
	if got := labels(1); got != "[FR IT ES]" {
		t.Fatalf("lignes masquées demandées : %s", got)
	}
	if err := d.SetFilter(ctx, "Feuille1", 0, nil); err != nil {
		t.Fatal(err)
	}
	if got := labels(0); got != "[FR IT ES]" {
		t.Fatalf("filtre retiré : %s", got)
	}
}
