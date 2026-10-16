package transfer

import (
	"archive/zip"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestXLSXExport(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix", "Date", "Payé"}, {"Pomme", "1,5", "15/01/2026", "VRAI"}, {"Poire <b>", "2", "16/01/2026", "FAUX"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 4, "Total"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 4, "=[Prix]*2"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 6, 0, "Somme"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 6, 1, "=SOMME(Feuille1[Prix])"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 7, 1, "=1/0"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetColumnWidth(ctx, 1, 0, 140); err != nil {
		t.Fatal(err)
	}
	id, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RenameSheet(ctx, id, "Notes: [2026]"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, id, 2, 27, "à voir"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSheetHidden(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	// Au-delà des limites d'Excel : la cellule n'est pas exportée, et comptée.
	if _, err := d.SetCell(ctx, id, ExcelRows+5, 0, "trop bas"); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "export.xlsx")
	clipped, err := ExportXLSXFile(ctx, d.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	if clipped != 1 {
		t.Fatalf("cellules coupées : %d", clipped)
	}

	// Le fichier contient les parties attendues.
	zr, err := zip.OpenReader(p)
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		for _, f := range zr.File {
			if f.Name == name {
				r, _ := f.Open()
				b, _ := io.ReadAll(r)
				r.Close()
				return string(b)
			}
		}
		t.Fatalf("partie %s absente", name)
		return ""
	}
	wb := read("xl/workbook.xml")
	for _, want := range []string{`name="Feuille1"`, `name="Notes_ _2026_" sheetId="2" state="hidden"`} {
		if !strings.Contains(wb, want) {
			t.Errorf("classeur : %s attendu dans %s", want, wb)
		}
	}
	s1 := read("xl/worksheets/sheet1.xml")
	for _, want := range []string{
		`<c r="A1" s="3" t="inlineStr"><is><t xml:space="preserve">Produit</t></is></c>`,
		`<c r="B2"><v>1.5</v></c>`,
		`<c r="C2" s="1"><v>46037</v></c>`,
		`<c r="D3" t="b"><v>0</v></c>`,
		`<c r="E2"><v>3</v></c>`,
		`Poire &lt;b&gt;`,
		`<c r="B7"><v>3.5</v></c>`,
		`<c r="B8" t="e"><v>#DIV/0!</v></c>`,
		`<col min="1" max="1" width="20" customWidth="1"/>`,
	} {
		if !strings.Contains(s1, want) {
			t.Errorf("feuille 1 : %s attendu dans\n%s", want, s1)
		}
	}
	if s2 := read("xl/worksheets/sheet2.xml"); !strings.Contains(s2, `<c r="AB3" t="inlineStr"><is><t xml:space="preserve">à voir</t></is></c>`) {
		t.Errorf("feuille 2 :\n%s", s2)
	}
	zr.Close()

	// Le fichier se relit : le tableau revient avec ses types.
	sheets, closeFn, err := ReadXLSX(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if len(sheets) != 2 || len(sheets[0].Tables) == 0 {
		t.Fatalf("relecture : %+v", sheets)
	}
	tb := sheets[0].Tables[0]
	var cols []string
	for i, h := range tb.Headers {
		cols = append(cols, h+":"+string(tb.Types[i]))
	}
	if got := strings.Join(cols, " "); got != "Produit:text Prix:decimal Date:date Payé:boolean Total:integer" {
		t.Errorf("colonnes relues : %s", got)
	}
}

func TestExcelSheetNames(t *testing.T) {
	got := excelSheetNames([]string{"Ventes", "ventes", "a/b", strings.Repeat("x", 40), ""})
	want := []string{"Ventes", "ventes (2)", "a_b", strings.Repeat("x", 31), "Feuille5"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d : %q, attendu %q", i, got[i], want[i])
		}
	}
	if colName(0) != "A" || colName(25) != "Z" || colName(26) != "AA" || colName(701) != "ZZ" || colName(702) != "AAA" {
		t.Error("lettres de colonnes")
	}
}
