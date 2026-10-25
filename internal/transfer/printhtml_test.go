package transfer

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
)

func TestPrintPlan(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 1, 1, [][]string{{"Produit", "Prix"}, {"Pomme & co", "1,5"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 5, 2, "=1/0"); err != nil {
		t.Fatal(err)
	}
	plan, err := SheetPrintPlan(ctx, d.Model, 1, PrintSizes{}, PrintInfo{})
	if err != nil {
		t.Fatal(err)
	}
	// Plage utilisée B2:C6, une page A4 portrait à 100 %.
	if plan.Empty || plan.PaperW != 210 || plan.PaperH != 297 || plan.Scale != 1 || len(plan.Pages) != 1 {
		t.Fatalf("plan : %+v", plan)
	}
	if want := (canvas.Rect{R0: 1, C0: 1, R1: 5, C1: 2}); plan.Area != want {
		t.Errorf("zone %+v, %+v attendue", plan.Area, want)
	}
	if !reflect.DeepEqual(plan.Rows, [][2]int{{1, 5}}) || !reflect.DeepEqual(plan.Cols, [][2]int{{1, 2}}) {
		t.Errorf("bandes : %v %v", plan.Rows, plan.Cols)
	}
	if plan.RowPx != 22 || plan.ColPx != 100 || plan.Margins != canvas.MarginsNormal {
		t.Errorf("tailles ou marges : %+v", plan)
	}
	if _, err := SheetPrintPlan(ctx, d.Model, 99, PrintSizes{}, PrintInfo{}); err == nil {
		t.Error("feuille inconnue acceptée")
	}
	empty, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	plan, _ = SheetPrintPlan(ctx, d.Model, empty, PrintSizes{}, PrintInfo{})
	if !plan.Empty || len(plan.Pages) != 0 {
		t.Errorf("feuille vide : %+v", plan)
	}
}

// Tailles de la grille : les pages se découpent sur ce que montre l'écran.
func TestPrintPlanSizes(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	for r := 0; r < 4; r++ {
		if _, err := d.SetCell(ctx, 1, r, 0, "x"); err != nil {
			t.Fatal(err)
		}
	}
	// Ligne 2 masquée (0), ligne 3 haute de 960 px : une page par ligne haute.
	sizes := PrintSizes{Row: 20, Col: 88, Rows: [][2]int{{1, 0}, {2, 960}}, Cols: [][2]int{{0, 150}}}
	plan, err := SheetPrintPlan(ctx, d.Model, 1, sizes, PrintInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.RowPx != 20 || plan.ColPx != 88 || !reflect.DeepEqual(plan.Widths, [][2]int{{0, 150}}) || !reflect.DeepEqual(plan.Heights, [][2]int{{1, 0}, {2, 960}}) {
		t.Errorf("tailles : %+v", plan)
	}
	if !reflect.DeepEqual(plan.Rows, [][2]int{{0, 1}, {2, 2}, {3, 3}}) {
		t.Errorf("bandes : %v", plan.Rows)
	}
	br, _ := SheetPageBreaks(ctx, d.Model, 1, sizes)
	if !reflect.DeepEqual(br.Rows, []int{2, 3}) {
		t.Errorf("sauts de la grille : %+v", br)
	}
}

// Un graphique s'imprime : il agrandit la plage utilisée.
func TestPrintPlanCharts(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Pomme", "1,5"}, {"Poire", "2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateChart(ctx, 1, 5, 0, canvas.ChartConfig{Source: "Feuille1", Kind: "bar", X: "Produit", Series: []string{"Prix"}}); err != nil {
		t.Fatal(err)
	}
	plan, err := SheetPrintPlan(ctx, d.Model, 1, PrintSizes{}, PrintInfo{})
	if err != nil {
		t.Fatal(err)
	}
	// 480 × 288 px depuis A6 : colonnes de 100 px, lignes de 22 px.
	if plan.Area.R1 != 5+13 || plan.Area.C1 != 4 {
		t.Errorf("zone avec le graphique : %+v", plan.Area)
	}
}

// Aucune limite de lignes : 100 000 lignes se découpent sans être lues.
func TestPrintPlanLarge(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	rows := [][]string{{"N", "Libellé"}}
	for i := 1; i <= 100_000; i++ {
		rows = append(rows, []string{fmt.Sprint(i), "ligne"})
	}
	if _, err := d.Paste(ctx, 1, 0, 0, rows); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	plan, err := SheetPrintPlan(ctx, d.Model, 1, PrintSizes{}, PrintInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Area.R1 != 100_000 || plan.Rows[len(plan.Rows)-1][1] != 100_000 {
		t.Fatalf("fin non imprimée : zone %+v, dernière bande %v", plan.Area, plan.Rows[len(plan.Rows)-1])
	}
	// 44 lignes de 22 px par page A4 (marges normales).
	if n := len(plan.Pages); n < 2200 || n > 2350 || plan.Pages[n-1].Row != n-1 {
		t.Errorf("%d pages", n)
	}
	if d := time.Since(t0); d > 5*time.Second {
		t.Errorf("plan en %v", d)
	}
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
	plan := func() PrintPlan {
		t.Helper()
		p, err := SheetPrintPlan(ctx, d.Model, 1, PrintSizes{}, info)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}

	// Zone A1:B30, titre ligne 1, sauts avant les lignes 11 et 21 : trois
	// pages, la ligne de titre en tête de chacune.
	set(canvas.PageSetup{Area: &canvas.Rect{R0: 0, C0: 0, R1: 29, C1: 1}, TitleRows: &canvas.Span{From: 0, To: 0}, RowBreaks: []int{10, 20},
		Header: canvas.HeaderFooter{Left: "&[Fichier]", Center: "&[Onglet]", Right: "&[Date] &[Heure]"}, Footer: canvas.HeaderFooter{Center: "Page &[Page] sur &[Pages] && fin"}})
	p := plan()
	if len(p.Pages) != 3 || !reflect.DeepEqual(p.Rows, [][2]int{{0, 9}, {10, 19}, {20, 29}}) || !reflect.DeepEqual(p.Cols, [][2]int{{0, 1}}) {
		t.Fatalf("pages : %+v", p)
	}
	if p.TitleRows == nil || p.TitleRows.From != 0 {
		t.Errorf("titres : %+v", p.TitleRows)
	}
	for i, pg := range p.Pages {
		if want := fmt.Sprintf("Page %d sur 3 & fin", i+1); pg.Footer.Center != want {
			t.Errorf("pied de la page %d : %q", i+1, pg.Footer.Center)
		}
	}
	if h := p.Pages[0].Header; h.Left != "Stock.lattis" || h.Center != "Feuille1" || h.Right != "19/10/2026 09:05" {
		t.Errorf("en-tête : %+v", h)
	}

	// Paysage A3, échelle 50 %, quadrillage et en-têtes, centrage,
	// numérotation à partir de 5.
	set(canvas.PageSetup{Paper: "A3", Landscape: true, Scale: 50, Gridlines: true, Headings: true, CenterH: true, CenterV: true, FirstPage: 5, Footer: canvas.HeaderFooter{Right: "&[Page]/&[Pages]"}})
	p = plan()
	if p.PaperW != 420 || p.PaperH != 297 || p.Scale != 0.5 || !p.Gridlines || !p.Headings || !p.CenterH || !p.CenterV || p.Pages[0].Footer.Right != "5/1" {
		t.Errorf("mise en page : %+v", p)
	}

	// Ajuster à une page en hauteur : une seule page, réduite.
	set(canvas.PageSetup{FitTall: 1, Margins: &canvas.Margins{Top: 10, Bottom: 10, Left: 2, Right: 2, Header: 1, Footer: 1}})
	p = plan()
	if len(p.Pages) != 1 || p.Scale >= 1 || p.Scale < 0.1 {
		t.Errorf("ajusté à une page : %d pages à %v", len(p.Pages), p.Scale)
	}
	br, err := SheetPageBreaks(ctx, d.Model, 1, PrintSizes{})
	if err != nil || br.Pages != 1 || br.Scale >= 100 || br.Scale < 10 || len(br.Rows) != 0 {
		t.Errorf("sauts, ajusté : %+v %v", br, err)
	}

	// Colonnes : saut avant B, colonne A répétée ; ordre « à droite, puis
	// vers le bas » avec un saut de ligne.
	set(canvas.PageSetup{TitleCols: &canvas.Span{From: 0, To: 0}, ColBreaks: []int{1, 2}, RowBreaks: []int{15}, AcrossFirst: true})
	p = plan()
	if len(p.Pages) != 6 {
		t.Fatalf("%d pages au lieu de 6", len(p.Pages))
	}
	order := [][2]int{}
	for _, pg := range p.Pages {
		order = append(order, [2]int{pg.Row, pg.Col})
	}
	if !reflect.DeepEqual(order, [][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}}) {
		t.Errorf("ordre « à droite puis vers le bas » : %v", order)
	}
	br, err = SheetPageBreaks(ctx, d.Model, 1, PrintSizes{})
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
	br, _ = SheetPageBreaks(ctx, d.Model, 1, PrintSizes{})
	if len(br.Rows) != 2 || br.Rows[0] < 30 || br.Rows[0] > 45 {
		t.Errorf("sauts automatiques : %+v", br)
	}
}

func TestExpandHF(t *testing.T) {
	v := hfVars{page: 2, pages: 7, sheet: "Ventes", info: PrintInfo{Path: "/tmp/a/b.lattis", Now: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)}}
	if got := expandHF("&[Feuille] – &[Page]/&[Pages] – &[Chemin]&[Fichier] – R&&D", v); got != "Ventes – 2/7 – /tmp/a/b.lattis – R&D" && got != `Ventes – 2/7 – \tmp\a\b.lattis – R&D` {
		t.Errorf("codes : %q", got)
	}
}
