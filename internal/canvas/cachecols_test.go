package canvas

import (
	"context"
	"encoding/json"
	"testing"
)

// TestPivotChartColumnCache : le cache d'un TCD et d'un graphique ne dépend
// que des colonnes qu'ils lisent ; après chaque opération sur la source, ce
// qu'ils affichent est ce qu'un calcul complet (cache vidé, F9) donne.
func TestPivotChartColumnCache(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"Pays", "Ville", "Ventes", "Note", "Qté"},
		{"FR", "Lyon", "10", "a", "1"}, {"IT", "Rome", "5", "b", "2"}, {"FR", "Paris", "7", "c", "3"},
		{"DE", "Berlin", "4", "d", "4"}, {"IT", "Milan", "8", "e", "5"},
	})
	must(t, err)
	sheet, err := d.CreatePivot(ctx, PivotConfig{Source: "Feuille1", RowFields: []string{"Pays"}, Cols: "Ville",
		Values: []PivotValue{{Field: "Ventes", Agg: "sum"}, {Agg: "count"}}})
	must(t, err)
	_, err = d.CreateChart(ctx, sheet, 0, 20, ChartConfig{Source: "Feuille1", Kind: "bar", X: "Pays", Series: []string{"Ventes"}})
	must(t, err)
	sheet2, err := d.CreatePivot(ctx, PivotConfig{Source: "Feuille1", RowFields: []string{"Pays"}, Values: []PivotValue{{Field: "Ventes", Agg: "sum"}}})
	must(t, err)
	_, err = d.CreateChart(ctx, sheet, 30, 20, ChartConfig{Source: "TCD2", Kind: "bar", X: "Pays", Series: []string{"Somme de Ventes"}})
	must(t, err)
	shown := func() string {
		t.Helper()
		l, err := d.Layout(ctx, sheet)
		must(t, err)
		l2, err := d.Layout(ctx, sheet2)
		must(t, err)
		b, _ := json.Marshal([]any{l.Pivots, l.Charts, l2.Pivots})
		return string(b)
	}
	check := func(step string) {
		t.Helper()
		cached := shown()
		d.Recalc()
		if fresh := shown(); cached != fresh {
			t.Fatalf("%s : affiché %s\nattendu %s", step, cached, fresh)
		}
	}
	check("départ")
	shown() // en cache
	steps := []struct {
		name string
		do   func() error
	}{
		{"saisie dans Note (non lue)", func() error { _, err := d.SetCell(ctx, 1, 2, 3, "zz"); return err }},
		{"saisie dans Ventes", func() error { _, err := d.SetCell(ctx, 1, 2, 2, "50"); return err }},
		{"saisie dans Pays", func() error { _, err := d.SetCell(ctx, 1, 3, 0, "ES"); return err }},
		{"saisie dans Ville", func() error { _, err := d.SetCell(ctx, 1, 1, 1, "Nice"); return err }},
		{"annuler", func() error { _, err := d.bus.Undo(ctx); return err }},
		{"ligne insérée", func() error { return d.InsertRow(ctx, "Feuille1", 1) }},
		{"saisie dans la nouvelle ligne", func() error { _, err := d.SetCell(ctx, 1, 2, 0, "PT"); return err }},
		{"ligne supprimée", func() error { return d.DeleteRow(ctx, "Feuille1", 3) }},
		{"tri par Qté (ordre des étiquettes)", func() error { return d.SortTable(ctx, "Feuille1", 4, true) }},
		{"filtre sur Note (graphique)", func() error {
			return d.SetFilter(ctx, "Feuille1", 3, &ColumnFilter{Values: []string{"a", "b", "zz"}})
		}},
		{"saisie dans Note filtrée", func() error { _, err := d.SetCell(ctx, 1, 1, 3, "q"); return err }},
		{"saisie dans Qté (non lue)", func() error { _, err := d.SetCell(ctx, 1, 1, 4, "9"); return err }},
	}
	for _, s := range steps {
		must(t, s.do())
		check(s.name)
	}
}
