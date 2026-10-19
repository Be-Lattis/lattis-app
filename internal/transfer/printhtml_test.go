package transfer

import (
	"context"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Les graphiques ne sont pas imprimés : la page le dit.
func TestPrintHTMLCharts(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Pomme", "1,5"}, {"Poire", "2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateChart(ctx, 1, 5, 0, canvas.ChartConfig{Source: "Feuille1", Kind: "bar", X: "Produit", Series: []string{"Prix"}}); err != nil {
		t.Fatal(err)
	}
	html, err := PrintHTML(ctx, d.Model, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "1 graphique non imprimé") {
		t.Errorf("graphique non signalé :\n%s", html)
	}
}

func TestPrintHTML(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 1, 1, [][]string{{"Produit", "Prix"}, {"Pomme & co", "1,5"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 5, 2, "=1/0"); err != nil {
		t.Fatal(err)
	}
	html, err := PrintHTML(ctx, d.Model, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<h1 class="print-title">Feuille1</h1>`,
		`<tr><td class="b">Produit</td><td class="b">Prix</td></tr>`,
		`<td>Pomme &amp; co</td><td class="n">1,5</td>`,
		`<td></td><td class="c">#DIV/0!</td>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("%s attendu dans\n%s", want, html)
		}
	}
	if strings.Count(html, "<tr>") != 5 {
		t.Errorf("plage utilisée : %d lignes\n%s", strings.Count(html, "<tr>"), html)
	}
	if _, err := PrintHTML(ctx, d.Model, 99); err == nil {
		t.Error("feuille inconnue acceptée")
	}
}
