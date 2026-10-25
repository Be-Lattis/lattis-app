package transfer

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/grid"
)

// Un classeur .ods (ici écrit par l'export de Lattis) s'importe avec ses
// valeurs, leurs types, sa mise en forme, ses fusions et ses largeurs ; les
// formules sont reprises par leur valeur.
func TestODSImport(t *testing.T) {
	ctx := context.Background()
	d := exportFixture(t)
	if err := d.ApplyStyle(ctx, 1, 1, 1, 2, 1, canvas.Style{"nf": "%", "bb": "double #c00000", "ff": "Georgia"}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "rt.ods")
	if _, err := ExportODSFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	d2 := open(t)
	res, err := ImportFile(ctx, d2.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d2, "après import")
	if w := strings.Join(res.Warnings, "\n"); !strings.Contains(w, "formules importées en valeurs") {
		t.Errorf("avertissements : %s", w)
	}
	l, err := d2.Layout(ctx, res.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Tables) < 1 {
		t.Fatal("pas de tableau")
	}
	tb := l.Tables[0]
	var types []string
	for _, c := range tb.Columns {
		types = append(types, c.Name+":"+string(c.Type))
	}
	if got := strings.Join(types, " "); got != "Produit:"+string(grid.Text)+" Prix:"+string(grid.Decimal)+" Date:"+string(grid.Date)+" Payé:"+string(grid.Boolean)+" Total:"+string(grid.Integer) {
		t.Errorf("colonnes : %s", got)
	}
	wantStyle(t, "Prix", styleOf(t, tb.Columns[1].Style), map[string]any{"nf": "%", "bb": "double #c00000", "ff": "Georgia"})
	free := map[[2]int]map[string]any{}
	for _, s := range l.Styles {
		free[[2]int{s.Row, s.Col}] = styleOf(t, s.Style)
	}
	wantStyle(t, "fusion", free[[2]int{12, 0}], map[string]any{"mr": 1, "mc": 3})
	if len(l.Widths) == 0 || l.Widths[0].Col != 0 || l.Widths[0].Width < 135 || l.Widths[0].Width > 145 {
		t.Errorf("largeurs : %+v", l.Widths)
	}
	if len(l.Tables) < 2 || l.Tables[1].Row != 6 {
		t.Fatalf("tableaux : %+v", l.Tables)
	}
	wantStyle(t, "B7", styleOf(t, l.Tables[1].Columns[1].HeaderStyle), map[string]any{"b": true, "nf": "€", "bg": "#ffff00", "al": "right"})
}

// Un .ods écrit à la main : lignes et cellules répétées, style par défaut
// d'une colonne, heure, pourcentage, note, fusion sur deux lignes.
func TestODSImportHand(t *testing.T) {
	ctx := context.Background()
	content := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" xmlns:number="urn:oasis:names:tc:opendocument:xmlns:datastyle:1.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
<office:automatic-styles>
<number:percentage-style style:name="N1"><number:number number:decimal-places="1" number:min-integer-digits="1"/><number:text>%</number:text></number:percentage-style>
<style:style style:name="co1" style:family="table-column"><style:table-column-properties fo:break-before="auto" style:column-width="5cm"/></style:style>
<style:style style:name="ce1" style:family="table-cell" style:parent-style-name="Default"><style:table-cell-properties fo:background-color="#dde8cb"/><style:text-properties fo:font-weight="bold"/></style:style>
<style:style style:name="ce2" style:family="table-cell" style:parent-style-name="Default" style:data-style-name="N1"/>
</office:automatic-styles>
<office:body><office:spreadsheet><table:table table:name="Suivi">
<table:table-column table:style-name="co1" table:default-cell-style-name="ce1"/><table:table-column table:number-columns-repeated="2" table:default-cell-style-name="Default"/><table:table-column table:number-columns-repeated="16381"/>
<table:table-row><table:table-cell office:value-type="string"><text:p>Tâche</text:p></table:table-cell><table:table-cell office:value-type="string"><text:p>Part</text:p></table:table-cell><table:table-cell office:value-type="string"><text:p>Durée</text:p></table:table-cell><table:table-cell table:number-columns-repeated="16381"/></table:table-row>
<table:table-row table:number-rows-repeated="3"><table:table-cell office:value-type="string"><text:p>Revue</text:p><office:annotation><dc:creator>Ana</dc:creator><text:p>À relire</text:p></office:annotation></table:table-cell><table:table-cell table:style-name="ce2" office:value-type="percentage" office:value="0.25"/><table:table-cell office:value-type="time" office:time-value="PT01H30M00S"/><table:table-cell table:number-columns-repeated="16381"/></table:table-row>
<table:table-row table:number-rows-repeated="2"><table:table-cell table:number-columns-repeated="16384"/></table:table-row>
<table:table-row><table:table-cell table:number-columns-spanned="2" table:number-rows-spanned="2" office:value-type="string"><text:p>Bloc</text:p></table:table-cell><table:covered-table-cell/></table:table-row>
<table:table-row table:number-rows-repeated="1048570"><table:table-cell table:number-columns-repeated="16384"/></table:table-row>
</table:table></office:spreadsheet></office:body></office:document-content>`
	p := filepath.Join(t.TempDir(), "main.ods")
	writeZipParts(t, p, map[string]string{"mimetype": "application/vnd.oasis.opendocument.spreadsheet", "content.xml": content})
	d := open(t)
	res, err := ImportODS(ctx, d.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "après import")
	l, err := d.Layout(ctx, res.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Tables) != 1 || l.Tables[0].Rows != 3 {
		t.Fatalf("tableaux : %+v", l.Tables)
	}
	tb := l.Tables[0]
	wantStyle(t, "Tâche", styleOf(t, tb.Columns[0].Style), map[string]any{"b": true, "bg": "#dde8cb"})
	wantStyle(t, "Part", styleOf(t, tb.Columns[1].Style), map[string]any{"nf": "%", "dp": 1})
	if tb.Columns[1].Type != grid.Decimal || tb.Columns[2].Type != grid.Text {
		t.Errorf("types : %+v", tb.Columns)
	}
	cells, err := d.TableCellStyles(ctx, tb.ID)
	if err != nil {
		t.Fatal(err)
	}
	notes := 0
	for _, c := range cells {
		if styleOf(t, c.Style)["nt"] == "À relire" {
			notes++
		}
	}
	if notes != 1 {
		t.Errorf("notes : %v", cells)
	}
	free := map[[2]int]map[string]any{}
	for _, s := range l.Styles {
		free[[2]int{s.Row, s.Col}] = styleOf(t, s.Style)
	}
	wantStyle(t, "Bloc", free[[2]int{6, 0}], map[string]any{"mr": 2, "mc": 2, "b": true})
	if len(l.Widths) != 1 || l.Widths[0].Width != 189 {
		t.Errorf("largeurs : %+v", l.Widths)
	}
}

func writeZipParts(t *testing.T, p string, parts map[string]string) {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for name, body := range parts {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
