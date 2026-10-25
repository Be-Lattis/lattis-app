package transfer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Les graphiques exportés en graphiques natifs se relisent comme
// graphiques de Lattis : type, séries, catégories, titres, axe secondaire,
// légende, couleurs, taille.
func TestXLSXChartRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := exportFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ymax := 10.0
	cfgs := []canvas.ChartConfig{
		{Source: "Ventes", Kind: "bar", X: "Produit", Series: []string{"Prix", "Total"}, Title: "Ventes", XTitle: "Produit", YTitle: "Euros",
			Legend: "right", Colors: []string{"#c00000", "#0070c0"}, YMax: &ymax, Width: 500, Height: 300, Stacked: true},
		{Source: "Ventes", Kind: "combo", X: "Produit", Series: []string{"Prix", "Total"}, Secondary: []bool{false, true}, Y2Title: "Total", Labels: true},
		{Source: "Ventes", Kind: "pie", X: "Produit", Series: []string{"Prix"}, Legend: "none"},
		{Source: "Ventes", Kind: "scatter", X: "Prix", Series: []string{"Total"}},
		{Source: "Ventes", Kind: "column", X: "Produit", Series: []string{"Prix"}},
		{Source: "Ventes", Kind: "treemap", X: "Produit", Series: []string{"Prix"}}, // sans équivalent
	}
	for i, c := range cfgs {
		_, err := d.CreateChart(ctx, 1, 20+i*16, 0, c)
		must(err)
	}
	p := filepath.Join(t.TempDir(), "rt.xlsx")
	rep, err := ExportXLSXFile(ctx, d.Model, p)
	must(err)
	if rep.Sheets[0].Charts != 1 {
		t.Errorf("graphiques non exportés : %d", rep.Sheets[0].Charts)
	}
	parts := readZip(t, p)
	c1 := parts["xl/charts/chart1.xml"]
	for _, want := range []string{`<c:barDir val="col"/><c:grouping val="stacked"/>`, `<c:f>'Données'!$B$2:$B$3</c:f>`, `<c:f>'Données'!$A$2:$A$3</c:f>`, `<a:t>Euros</a:t>`, `<c:max val="10"/>`, `<a:srgbClr val="C00000"/>`, `<c:legendPos val="r"/>`} {
		if !strings.Contains(c1, want) {
			t.Errorf("chart1.xml : %s attendu dans\n%s", want, c1)
		}
	}
	if c2 := parts["xl/charts/chart2.xml"]; !strings.Contains(c2, `<c:axPos val="r"/>`) || !strings.Contains(c2, `<c:lineChart>`) {
		t.Errorf("chart2.xml : %s", c2)
	}
	if _, ok := parts["xl/charts/chart6.xml"]; ok {
		t.Error("treemap exporté")
	}
	d2 := open(t)
	res, err := ImportXLSX(ctx, d2.Model, p)
	must(err)
	check(t, d2, "après réimport")
	for _, w := range res.Warnings {
		if strings.Contains(w, "graphique") {
			t.Errorf("avertissement : %s", w)
		}
	}
	l, err := d2.Layout(ctx, res.Sheets[0])
	must(err)
	if len(l.Charts) != 5 {
		t.Fatalf("%d graphiques", len(l.Charts))
	}
	g := l.Charts[0]
	c := g.Config
	if g.Row != 20 || g.Col != 0 || c.Kind != "bar" || c.Source != "Ventes" || c.X != "Produit" || strings.Join(c.Series, ",") != "Prix,Total" ||
		c.Title != "Ventes" || c.XTitle != "Produit" || c.YTitle != "Euros" || c.Legend != "right" || !c.Stacked || c.YMax == nil || *c.YMax != 10 ||
		c.Width != 500 || c.Height != 300 || strings.Join(c.Colors, ",") != "#c00000,#0070c0" {
		t.Errorf("graphique 1 : %+v", c)
	}
	if c := l.Charts[1].Config; c.Kind != "combo" || len(c.Secondary) != 2 || !c.Secondary[1] || c.Y2Title != "Total" || !c.Labels || strings.Join(c.SeriesKinds, ",") != "bar,line" {
		t.Errorf("graphique 2 : %+v", c)
	}
	if c := l.Charts[2].Config; c.Kind != "pie" || c.Legend != "none" {
		t.Errorf("graphique 3 : %+v", c)
	}
	if c := l.Charts[3].Config; c.Kind != "scatter" || c.X != "Prix" {
		t.Errorf("graphique 4 : %+v", c)
	}
	if c := l.Charts[4].Config; c.Kind != "column" {
		t.Errorf("graphique 5 : %+v", c)
	}
}
