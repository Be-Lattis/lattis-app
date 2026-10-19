package transfer

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// pageFixture : une feuille « Ventes 'T1' » avec une mise en page complète.
func pageFixture(t *testing.T) (*fixture, canvas.PageSetup) {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Pomme", "1"}, {"Poire", "2"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameSheet(ctx, 1, "Ventes 'T1'"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetColumnWidth(ctx, 1, 2, 140); err != nil {
		t.Fatal(err)
	}
	m := canvas.Margins{Top: 2.5, Bottom: 2, Left: 1, Right: 1, Header: 1, Footer: 0.8}
	p := canvas.PageSetup{Paper: "Letter", Landscape: true, Margins: &m, FitWide: 1, Area: &canvas.Rect{R0: 0, C0: 0, R1: 39, C1: 5},
		TitleRows: &canvas.Span{From: 0, To: 1}, TitleCols: &canvas.Span{From: 0, To: 0}, RowBreaks: []int{20}, ColBreaks: []int{2},
		Gridlines: true, Headings: true, CenterH: true, AcrossFirst: true, FirstPage: 3,
		Header: canvas.HeaderFooter{Center: "&[Onglet] & co"}, Footer: canvas.HeaderFooter{Left: "&[Fichier]", Right: "Page &[Page] sur &[Pages]"}}
	if err := d.SetPageSetup(ctx, 1, p); err != nil {
		t.Fatal(err)
	}
	got, _ := d.PageSetup(ctx, 1)
	return d, got
}

func TestXLSXPageSetup(t *testing.T) {
	ctx := context.Background()
	d, want := pageFixture(t)
	p := filepath.Join(t.TempDir(), "page.xlsx")
	if _, err := ExportXLSXFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	parts := readZip(t, p)
	contains(t, "feuille", parts["xl/worksheets/sheet1.xml"],
		`<sheetPr><pageSetUpPr fitToPage="1"/></sheetPr>`,
		`<printOptions horizontalCentered="1" headings="1" gridLines="1"/><pageMargins left="0.3937" right="0.3937" top="0.9843" bottom="0.7874" header="0.3937" footer="0.315"/>`,
		`<pageSetup paperSize="1" firstPageNumber="3" fitToWidth="1" fitToHeight="0" pageOrder="overThenDown" orientation="landscape" useFirstPageNumber="1"/>`,
		`<headerFooter><oddHeader>&amp;C&amp;A &amp;&amp; co</oddHeader><oddFooter>&amp;L&amp;F&amp;RPage &amp;P sur &amp;N</oddFooter></headerFooter>`,
		`<rowBreaks count="1" manualBreakCount="1"><brk id="20" max="16383" man="1"/></rowBreaks><colBreaks count="1" manualBreakCount="1"><brk id="2" max="1048575" man="1"/></colBreaks>`)
	if i, j := strings.Index(parts["xl/worksheets/sheet1.xml"], "<pageSetup"), strings.Index(parts["xl/worksheets/sheet1.xml"], "<tableParts"); i < 0 || j < i {
		t.Error("pageSetup doit précéder tableParts")
	}
	contains(t, "classeur", parts["xl/workbook.xml"],
		`</sheets><definedNames><definedName name="_xlnm.Print_Area" localSheetId="0">'Ventes ''T1'!$A$1:$F$40</definedName><definedName name="_xlnm.Print_Titles" localSheetId="0">'Ventes ''T1'!$A:$A,'Ventes ''T1'!$1:$2</definedName></definedNames><calcPr`)

	// Relu à l'import : la même mise en page.
	d2 := open(t)
	res, err := ImportXLSX(ctx, d2.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d2.PageSetup(ctx, res.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	// Les marges passent par les pouces : arrondies.
	gm, wm := *got.Margins, *want.Margins
	got.Margins, want.Margins = nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("relue :\n%+v\nau lieu de\n%+v", got, want)
	}
	for _, v := range [][2]float64{{gm.Top, wm.Top}, {gm.Footer, wm.Footer}, {gm.Left, wm.Left}} {
		if v[0]-v[1] > 0.001 || v[1]-v[0] > 0.001 {
			t.Errorf("marges %+v au lieu de %+v", gm, wm)
		}
	}
}

func TestParseXLSXHF(t *testing.T) {
	h := parseXLSXHF(`&L&"Arial,Gras"&12Rapport &KFF0000&B2026&C&P / &N&R&D &T && &Z&F`)
	if h.Left != "Rapport 2026" || h.Center != "&[Page] / &[Pages]" || h.Right != "&[Date] &[Heure] & &[Chemin]&[Fichier]" {
		t.Errorf("en-tête lu : %+v", h)
	}
	if got := xlsxHF(canvas.HeaderFooter{Left: "R&D", Right: "&[Feuille]"}); got != "&LR&&D&R&A" {
		t.Errorf("en-tête écrit : %q", got)
	}
}

func TestODSPageSetup(t *testing.T) {
	ctx := context.Background()
	d, _ := pageFixture(t)
	p := filepath.Join(t.TempDir(), "page.ods")
	if _, err := ExportODSFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	parts := readZip(t, p)
	contains(t, "styles", parts["styles.xml"],
		`<style:page-layout style:name="pm2"><style:page-layout-properties fo:page-width="27.94cm" fo:page-height="21.59cm" style:print-orientation="landscape" fo:margin-top="1cm" fo:margin-bottom="0.8cm" fo:margin-left="1cm" fo:margin-right="1cm" style:print="headers grid objects charts drawings zero-values" style:print-page-order="ltr" style:table-centering="horizontal" style:first-page-number="3" style:scale-to-X="1"/>`,
		`<style:header-style><style:header-footer-properties fo:min-height="1.5cm"`,
		`<style:master-page style:name="Page1" style:page-layout-name="pm2"><style:header><style:region-left><text:p></text:p></style:region-left><style:region-center><text:p><text:sheet-name/> &amp; co</text:p></style:region-center>`,
		`<style:region-right><text:p>Page <text:page-number>1</text:page-number> sur <text:page-count>1</text:page-count></text:p></style:region-right>`)
	contains(t, "contenu", parts["content.xml"],
		`<style:style style:name="tp0" style:family="table" style:master-page-name="Page1">`,
		`table:style-name="tp0" table:print-ranges="'Ventes ''T1'.A1:'Ventes ''T1'.F40"`,
		`<table:table-header-columns><table:table-column table:default-cell-style-name="Default"/></table:table-header-columns><table:table-column table:default-cell-style-name="Default"/><table:table-column table:style-name="cob140"`,
		`<table:table-header-rows><table:table-row>`,
		`</table:table-row></table:table-header-rows>`,
		`<table:table-row table:style-name="rob">`)
}
