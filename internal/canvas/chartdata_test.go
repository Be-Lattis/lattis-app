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

// Au-delà de MaxChartPoints : la coupure est signalée (Total, Truncated),
// une courbe est réduite en gardant ses extrêmes, et boîtes et fréquences
// portent sur toutes les lignes, quel que soit l'ordre du tableau.
func TestChartBeyondMaxPoints(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	const n = 5000
	rows := [][]string{{"N", "V", "Cat"}}
	for i := 1; i <= n; i++ {
		// V décroît : les 2 000 premières lignes ne sont que les grandes valeurs.
		rows = append(rows, []string{fmt.Sprint(i), fmt.Sprint(n + 1 - i), []string{"A", "B"}[i%2]})
	}
	if _, err := d.Paste(ctx, 1, 0, 0, rows); err != nil {
		t.Fatal(err)
	}
	chart := func(cfg ChartConfig) ChartInfo {
		t.Helper()
		cfg.Source = "Feuille1"
		info := d.ChartData(ctx, cfg)
		if info.Error != "" {
			t.Fatalf("%s : %s", cfg.Kind, info.Error)
		}
		return info
	}
	bar := chart(ChartConfig{Kind: "bar", X: "N", Series: []string{"V"}})
	if bar.Total != n || !bar.Truncated || len(bar.Labels) != MaxChartPoints || len(bar.XValues) != MaxChartPoints || *bar.XValues[0] != 1 {
		t.Fatalf("histogramme : total %d, coupé %v, %d points", bar.Total, bar.Truncated, len(bar.Labels))
	}
	byCat := chart(ChartConfig{Kind: "bar", X: "Cat", Series: []string{"V"}})
	if byCat.Total != 2 || byCat.Truncated || fmt.Sprint(byCat.Labels) != "[B A]" {
		t.Fatalf("regroupé : %+v", byCat.Labels)
	}
	line := chart(ChartConfig{Kind: "line", X: "N", Series: []string{"V"}})
	got := line.Series[0].Values
	if !line.Sampled || line.Truncated || len(got) > MaxChartPoints || len(got) < MaxChartPoints/2 || *got[0] != n || *got[len(got)-1] != 1 || line.Labels[len(line.Labels)-1] != fmt.Sprint(n) {
		t.Fatalf("courbe réduite : %d points, réduite %v", len(got), line.Sampled)
	}
	box := chart(ChartConfig{Kind: "boxplot", X: "N", Series: []string{"V"}})
	b := box.Boxes[0][0]
	if fmt.Sprint(box.Labels) != "[V]" || b.N != n || b.Min != 1 || b.Max != n || b.Median != 2500.5 || b.Q1 != 1250.75 || b.Q3 != 3750.25 || b.Mean != 2500.5 || b.Out != 0 {
		t.Fatalf("boîte : %+v", b)
	}
	// Par catégorie (Cat se répète) : A a les V impairs… et une valeur aberrante ajoutée.
	if _, err := d.SetCell(ctx, 1, n+1, 0, fmt.Sprint(n+1)); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, n+1, 1, "1000000"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, n+1, 2, "A"); err != nil {
		t.Fatal(err)
	}
	boxes := chart(ChartConfig{Kind: "boxplot", X: "Cat", Series: []string{"V"}})
	if fmt.Sprint(boxes.Labels) != "[B A]" || boxes.Boxes[1][0].N != n/2+1 || boxes.Boxes[1][0].Out != 1 || fmt.Sprint(boxes.Boxes[1][0].Outliers) != "[1e+06]" || boxes.Boxes[1][0].Max != n-1 {
		t.Fatalf("boîtes par catégorie : %v %+v %+v", boxes.Labels, boxes.Boxes[0][0], boxes.Boxes[1][0])
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	hist := chart(ChartConfig{Kind: "histogram", X: "N", Series: []string{"V"}, Bins: 10})
	if fmt.Sprint(hist.Edges) != "[0 500 1000 1500 2000 2500 3000 3500 4000 4500 5000]" || len(hist.Labels) != 10 {
		t.Fatalf("classes : %v %v", hist.Edges, hist.Labels)
	}
	sum := 0.0
	for _, v := range hist.Series[0].Values {
		sum += *v
	}
	if sum != n || *hist.Series[0].Values[0] != 499 || *hist.Series[0].Values[9] != 501 {
		t.Fatalf("effectifs : %v", hist.Series[0].Values)
	}
}

// Un texte dans une colonne de nombres ne compte pas pour 0 ; une colonne
// calculée de texte en X regroupe ; dates en X : valeurs et axe de temps ;
// format de nombre de chaque série.
func TestChartTypesAndFormats(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Jour", "Pays", "Ventes"}, {"2026-01-15", "FR", "10"}, {"2026-03-01", "IT", "5"}, {"2026-02-01", "FR", "7"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 3, "Zone"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 3, `=[Pays]&"!"`); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 1, 2, 3, 2, Style{"nf": "€"}); err != nil {
		t.Fatal(err)
	}
	zone := d.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "bar", X: "Zone", Series: []string{"Ventes"}})
	if zone.Error != "" || fmt.Sprint(zone.Labels) != "[FR! IT!]" || *zone.Series[0].Values[0] != 17 || zone.Formats[0] != `{"nf":"€"}` {
		t.Fatalf("X calculé : %+v", zone)
	}
	sc := d.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "scatter", X: "Jour", Series: []string{"Ventes"}})
	if sc.Error != "" || !sc.XDate || len(sc.XValues) != 3 || sc.XValues[0] == nil || *sc.XValues[1]-*sc.XValues[0] != 45 || sc.XFormat != `{"nf":"date"}` {
		t.Fatalf("dates : %+v", sc)
	}
	// Nuage à X de texte : les lignes une à une, sans regrouper.
	if p := d.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "scatter", X: "Pays", Series: []string{"Ventes"}}); len(p.Labels) != 3 || p.XValues != nil {
		t.Fatalf("nuage, X de texte : %+v", p)
	}
}
