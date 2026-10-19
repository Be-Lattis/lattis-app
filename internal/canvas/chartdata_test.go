package canvas

import (
	"context"
	"fmt"
	"strings"
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

// Renommer une colonne ou le tableau suit dans la colonne de couleur d'une
// carte et dans les graphiques et TCD d'un onglet Document.
func TestChartRenameColorByAndDoc(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Lat", "Lon", "Région"}, {"Paris", "48.85", "2.35", "IDF"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateChart(ctx, 1, 0, 6, ChartConfig{Source: "Feuille1", Kind: "map", X: "Ville", Series: []string{"Lat", "Lon"}, ColorBy: "Région"}); err != nil {
		t.Fatal(err)
	}
	doc, err := d.AddDoc(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	src := `{"ops":[{"insert":"Texte\n"},{"insert":{"lattisChart":{"source":"Feuille1","kind":"bar","x":"Ville","series":["Lat"],"colorBy":"Région"}}},` +
		`{"insert":{"lattisPivot":{"source":"Feuille1","rowFields":["Région"],"values":[{"field":"Lat","agg":"sum"}]}}},{"insert":"\n"}]}`
	if err := d.SetPage(ctx, doc, src); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 3, "Zone"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 0, "Commune"); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameTable(ctx, "Feuille1", "Lieux"); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if c := l.Charts[0]; c.Error != "" || c.Config.ColorBy != "Zone" || c.Config.Source != "Lieux" {
		t.Fatalf("carte : %+v", c)
	}
	page, err := d.Page(ctx, doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"source":"Lieux"`, `"colorBy":"Zone"`, `"x":"Commune"`, `"rowFields":["Zone"]`} {
		if !strings.Contains(page, want) {
			t.Fatalf("document : %s manque dans %s", want, page)
		}
	}
	if strings.Contains(page, "Région") || strings.Contains(page, "Feuille1") {
		t.Fatalf("document : ancien nom resté : %s", page)
	}
	// Le document revient avec l'annulation du renommage du tableau.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if page, _ := d.Page(ctx, doc); !strings.Contains(page, `"source":"Feuille1"`) {
		t.Fatalf("annulation : %s", page)
	}
}
