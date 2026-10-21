package transfer

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
	html, err := PrintHTML(ctx, d.Model, 1, PrintInfo{})
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
	html, err := PrintHTML(ctx, d.Model, 1, PrintInfo{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`@page { size: 210mm 297mm; margin: 0; }`,
		`<tr style="height:22px"><td class="t b">Produit</td><td class="t b">Prix</td></tr>`,
		`<td class="t">Pomme &amp; co</td><td class="t n">1,5</td>`,
		`<td></td><td class="c">#DIV/0!</td>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("%s attendu dans\n%s", want, html)
		}
	}
	if strings.Count(html, "<tr ") != 5 || strings.Count(html, `<section class="lp-page"`) != 1 {
		t.Errorf("plage utilisée : %d lignes\n%s", strings.Count(html, "<tr "), html)
	}
	if strings.Contains(html, "lp-hf") || strings.Contains(html, "print-title") {
		t.Errorf("en-tête ou titre sans mise en page\n%s", html)
	}
	if _, err := PrintHTML(ctx, d.Model, 99, PrintInfo{}); err == nil {
		t.Error("feuille inconnue acceptée")
	}
	empty, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	html, _ = PrintHTML(ctx, d.Model, empty, PrintInfo{})
	if !strings.Contains(html, "Feuille vide.") {
		t.Errorf("feuille vide :\n%s", html)
	}
}

// sections coupe le HTML imprimé en pages.
func sections(html string) []string {
	parts := strings.Split(html, `<section class="lp-page`)
	return parts[1:]
}

func TestPrintPageSetup(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	// 30 lignes : un en-tête et 29 lignes de données, trois colonnes.
	rows := [][]string{{"Produit", "Prix", "Stock"}}
	for i := 1; i < 30; i++ {
		rows = append(rows, []string{fmt.Sprintf("P%d", i), fmt.Sprint(i), fmt.Sprint(i * 2)})
	}
	if _, err := d.Paste(ctx, 1, 0, 0, rows); err != nil {
		t.Fatal(err)
	}
	set := func(p canvas.PageSetup) {
		t.Helper()
		if err := d.SetPageSetup(ctx, 1, p); err != nil {
			t.Fatal(err)
		}
	}
	// Chemin du système : le nom du fichier se lit de même sous Windows et Linux.
	info := PrintInfo{Path: filepath.Join("docs", "Stock.lattis"), Now: time.Date(2026, 10, 19, 9, 5, 0, 0, time.UTC)}
	print := func() string {
		t.Helper()
		html, err := PrintHTML(ctx, d.Model, 1, info)
		if err != nil {
			t.Fatal(err)
		}
		return html
	}

	// Zone A1:B30, titre ligne 1, sauts avant les lignes 11 et 21 : trois
	// pages, la ligne de titre en tête de chacune.
	set(canvas.PageSetup{Area: &canvas.Rect{R0: 0, C0: 0, R1: 29, C1: 1}, TitleRows: &canvas.Span{From: 0, To: 0}, RowBreaks: []int{10, 20},
		Header: canvas.HeaderFooter{Left: "&[Fichier]", Center: "&[Onglet]", Right: "&[Date] &[Heure]"}, Footer: canvas.HeaderFooter{Center: "Page &[Page] sur &[Pages] && fin"}})
	html := print()
	pages := sections(html)
	if len(pages) != 3 {
		t.Fatalf("%d pages\n%s", len(pages), html)
	}
	for i, pg := range pages {
		if !strings.Contains(pg, `<td class="t b">Produit</td>`) {
			t.Errorf("page %d sans la ligne de titre", i+1)
		}
		if strings.Contains(pg, "Stock") && !strings.Contains(pg, "Stock.lattis") {
			t.Errorf("page %d : colonne hors de la zone", i+1)
		}
		if !strings.Contains(pg, fmt.Sprintf(`<span class="c">Page %d sur 3 &amp; fin</span>`, i+1)) {
			t.Errorf("pied de la page %d :\n%s", i+1, pg)
		}
	}
	for _, want := range []string{`<span class="l">Stock.lattis</span>`, `<span class="c">Feuille1</span>`, `<span class="r">19/10/2026 09:05</span>`} {
		if !strings.Contains(pages[0], want) {
			t.Errorf("en-tête : %s attendu", want)
		}
	}
	if !strings.Contains(pages[1], ">P10<") || strings.Contains(pages[1], ">P9<") || strings.Contains(pages[1], ">P20<") {
		t.Errorf("page 2 : lignes 11 à 20 attendues\n%s", pages[1])
	}

	// Paysage A3, échelle 50 %, quadrillage et en-têtes, centrage,
	// numérotation à partir de 5.
	set(canvas.PageSetup{Paper: "A3", Landscape: true, Scale: 50, Gridlines: true, Headings: true, CenterH: true, CenterV: true, FirstPage: 5, Footer: canvas.HeaderFooter{Right: "&[Page]/&[Pages]"}})
	html = print()
	for _, want := range []string{`size: 420mm 297mm`, `style="zoom:0.5"`, `class="print-grid lp-lines"`, `<th>A</th><th>B</th><th>C</th>`, `<th>1</th>`, `align-items:center;justify-content:center`, `<span class="r">5/1</span>`} {
		if !strings.Contains(html, want) {
			t.Errorf("%s attendu", want)
		}
	}

	// Ajuster à une page en hauteur : une seule page, réduite.
	set(canvas.PageSetup{FitTall: 1, Margins: &canvas.Margins{Top: 10, Bottom: 10, Left: 2, Right: 2, Header: 1, Footer: 1}})
	html = print()
	if n := len(sections(html)); n != 1 || !strings.Contains(html, "zoom:0.") {
		t.Errorf("ajusté à une page : %d pages\n%s", n, html)
	}
	br, err := SheetPageBreaks(ctx, d.Model, 1)
	if err != nil || br.Pages != 1 || br.Scale >= 100 || br.Scale < 10 || len(br.Rows) != 0 {
		t.Errorf("sauts, ajusté : %+v %v", br, err)
	}

	// Colonnes : saut avant B, colonne A répétée ; ordre « à droite, puis
	// vers le bas » avec un saut de ligne.
	set(canvas.PageSetup{TitleCols: &canvas.Span{From: 0, To: 0}, ColBreaks: []int{1, 2}, RowBreaks: []int{15}, AcrossFirst: true})
	pages = sections(print())
	if len(pages) != 6 {
		t.Fatalf("%d pages au lieu de 6", len(pages))
	}
	if !strings.Contains(pages[1], ">Produit<") || !strings.Contains(pages[1], ">Prix<") || strings.Contains(pages[1], ">Stock<") {
		t.Errorf("page 2 : colonnes A et B attendues\n%s", pages[1])
	}
	if !strings.Contains(pages[3], ">P15<") || strings.Contains(pages[3], ">Prix<") {
		t.Errorf("page 4 (ordre à droite puis vers le bas) :\n%s", pages[3])
	}
	br, err = SheetPageBreaks(ctx, d.Model, 1)
	if err != nil || !reflect.DeepEqual(br.Rows, []int{15}) || !reflect.DeepEqual(br.Cols, []int{1, 2}) || !reflect.DeepEqual(br.ManualCols, []int{1, 2}) || br.Pages != 6 {
		t.Errorf("sauts : %+v %v", br, err)
	}

	// Sans saut manuel, la hauteur de la page coupe : A4 portrait, marges
	// normales, lignes de 22 px → 41 lignes par page (titres compris).
	rows = rows[:1]
	for i := 1; i < 100; i++ {
		rows = append(rows, []string{fmt.Sprintf("Q%d", i), "1", "2"})
	}
	if _, err := d.Paste(ctx, 1, 0, 0, rows); err != nil {
		t.Fatal(err)
	}
	set(canvas.PageSetup{TitleRows: &canvas.Span{From: 0, To: 0}})
	br, _ = SheetPageBreaks(ctx, d.Model, 1)
	if len(br.Rows) != 2 || br.Rows[0] < 30 || br.Rows[0] > 45 {
		t.Errorf("sauts automatiques : %+v", br)
	}
	for _, pg := range sections(print()) {
		if !strings.Contains(pg, ">Produit<") {
			t.Error("titre non répété")
		}
	}
}

func TestExpandHF(t *testing.T) {
	v := hfVars{page: 2, pages: 7, sheet: "Ventes", info: PrintInfo{Path: "/tmp/a/b.lattis", Now: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)}}
	if got := expandHF("&[Feuille] – &[Page]/&[Pages] – &[Chemin]&[Fichier] – R&&D", v); got != "Ventes – 2/7 – /tmp/a/b.lattis – R&D" && got != `Ventes – 2/7 – \tmp\a\b.lattis – R&D` {
		t.Errorf("codes : %q", got)
	}
}
