package transfer

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// xlsxFixture écrit un classeur minimal, tel qu'Excel l'enregistre : les
// feuilles sont données par leur contenu <sheetData>.
func xlsxFixture(t *testing.T, sst []string, sheets [][2]string) string {
	t.Helper()
	return xlsxFixtureParts(t, sst, sheets, nil)
}

// xlsxFixtureParts : xlsxFixture avec des parties en plus (graphiques…) ;
// une feuille dont le contenu commence par <sheetData est écrite telle quelle.
func xlsxFixtureParts(t *testing.T, sst []string, sheets [][2]string, parts map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "classeur.xlsx")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	put := func(name, body string) {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" + body)); err != nil {
			t.Fatal(err)
		}
	}
	const ns = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	const rel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
	put("_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="`+rel+`officeDocument" Target="xl/workbook.xml"/></Relationships>`)
	var wbSheets, wbRels strings.Builder
	for i, s := range sheets {
		fmt.Fprintf(&wbSheets, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, s[0], i+1, i+1)
		fmt.Fprintf(&wbRels, `<Relationship Id="rId%d" Type="%sworksheet" Target="worksheets/sheet%d.xml"/>`, i+1, rel, i+1)
		body := s[1]
		if !strings.HasPrefix(body, "<sheetData") {
			body = `<sheetData>` + body + `</sheetData>`
		}
		put(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), `<worksheet `+ns+`>`+body+`</worksheet>`)
	}
	for name, body := range parts {
		put(name, body)
	}
	fmt.Fprintf(&wbRels, `<Relationship Id="rS" Type="%ssharedStrings" Target="sharedStrings.xml"/>`, rel)
	fmt.Fprintf(&wbRels, `<Relationship Id="rT" Type="%sstyles" Target="/xl/styles.xml"/>`, rel)
	put("xl/workbook.xml", `<workbook `+ns+`><sheets>`+wbSheets.String()+`</sheets></workbook>`)
	put("xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+wbRels.String()+`</Relationships>`)
	var si strings.Builder
	for _, s := range sst {
		fmt.Fprintf(&si, `<si><t>%s</t></si>`, s)
	}
	put("xl/sharedStrings.xml", `<sst `+ns+`>`+si.String()+`<si><r><t>Ri</t></r><r><t>che</t></r><rPh><t>x</t></rPh></si></sst>`)
	put("xl/styles.xml", `<styleSheet `+ns+`><numFmts count="1"><numFmt numFmtId="164" formatCode="dd/mm/yyyy\ hh:mm"/></numFmts>`+
		`<cellXfs count="3"><xf numFmtId="0"/><xf numFmtId="14"/><xf numFmtId="164"/></cellXfs></styleSheet>`)
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

// Petits constructeurs de cellules : chaîne partagée, nombre, formule.
func cs(ref string, i int) string { return fmt.Sprintf(`<c r="%s" t="s"><v>%d</v></c>`, ref, i) }
func cn(ref, v string) string     { return fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, v) }
func cf(ref, f, v string) string  { return fmt.Sprintf(`<c r="%s"><f>%s</f><v>%s</v></c>`, ref, f, v) }
func row(n int, cells ...string) string {
	return fmt.Sprintf(`<row r="%d">%s</row>`, n, strings.Join(cells, ""))
}

func TestXLSXImport(t *testing.T) {
	ctx := context.Background()
	sst := []string{"Code", "Produit", "Qté", "Prix", "Total", "Date", "Pomme", "Poire", "Prix unitaire", "Total général", "Payé", "Note", "Pays", "France"}
	commandes := row(1, cs("A1", 0), cs("B1", 1), cs("C1", 2), cs("D1", 3), cs("E1", 4), cs("F1", 5), cs("G1", 10), cs("H1", 11)) +
		row(2, cn("A2", "1"), cs("B2", 6), cn("C2", "3"),
			`<c r="D2"><f t="shared" ref="D2:D4" si="1">VLOOKUP(B2,Produits!$A$2:$B$10,2,FALSE)</f><v>1.5</v></c>`,
			`<c r="E2"><f t="shared" ref="E2:E4" si="0">C2*D2</f><v>4.5</v></c>`,
			`<c r="F2" s="1"><v>45306</v></c>`, `<c r="G2" t="b"><v>1</v></c>`, cf("H2", "A2&amp;\"-\"&amp;B2", "1-Pomme")) +
		row(3, cn("A3", "2"), cs("B3", 7), cn("C3", "2"),
			`<c r="D3"><f t="shared" si="1"/><v>2</v></c>`, `<c r="E3"><f t="shared" si="0"/><v>4</v></c>`,
			`<c r="F3" s="1"><v>45307</v></c>`, `<c r="G3" t="b"><v>0</v></c>`, `<c r="H3" t="str"><f>A2&amp;"x"</f><v>1x</v></c>`) +
		row(4, cn("A4", "3"), cs("B4", 6), cn("C4", "10"),
			`<c r="D4"><f t="shared" si="1"/><v>1.5</v></c>`, `<c r="E4"><f t="shared" si="0"/><v>15</v></c>`,
			`<c r="F4" s="2"><v>45308.5</v></c>`, `<c r="G4" t="e"><v>#N/A</v></c>`, `<c r="H4" t="inlineStr"><is><t>libre</t></is></c>`) +
		row(7, cs("A7", 9), cf("B7", "SUM(E2:E4)", "23.5"), cf("C7", "_xlfn.SUM($E$2:$E$5)*2", "47"), cf("D7", "A2*2", "2"), cs("E7", 14))
	produits := row(1, cs("A1", 1), cs("B1", 8), cs("D1", 12)) +
		row(2, cs("A2", 6), cn("B2", "1.5"), cs("D2", 13)) +
		row(3, cs("A3", 7), cn("B3", "2"), cn("D3", "007"))
	path := xlsxFixture(t, sst, [][2]string{{"Commandes", commandes}, {"Produits", produits}})

	d := open(t)
	res, err := ImportXLSX(ctx, d.Model, path)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "après import")
	if got := strings.Join(res.Tables, " "); got != "Commandes Produits Tableau1" {
		t.Errorf("tableaux : %s", got)
	}
	warn := strings.Join(res.Warnings, "\n")
	for _, w := range []string{
		"colonne « Note » : formules différentes",
		"Tableau Commandes : 1 valeur(s) en erreur",
		"cellule D7 : référence à une cellule (A2)",
	} {
		if !strings.Contains(warn, w) {
			t.Errorf("avertissement attendu %q dans :\n%s", w, warn)
		}
	}
	if len(res.Warnings) != 3 {
		t.Errorf("avertissements :\n%s", warn)
	}

	l, err := d.Layout(ctx, res.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	tb := l.Tables[0]
	var cols []string
	for _, c := range tb.Columns {
		s := c.Name + ":" + string(c.Type)
		if c.Formula != "" {
			s += c.Formula
		}
		cols = append(cols, s)
	}
	want := `Code:integer Produit:text Qté:integer Prix:decimal=REFERENCE(Produits[Prix unitaire]; [Produit]) ` +
		`Total:decimal=[Qté]*[Prix] Date:text Payé:boolean Note:text`
	if got := strings.Join(cols, " "); got != want {
		t.Errorf("colonnes :\n%s\nattendu :\n%s", got, want)
	}
	if tb.Rows != 3 || tb.Row != 0 || tb.Col != 0 {
		t.Errorf("tableau : %d lignes en (%d, %d)", tb.Rows, tb.Row, tb.Col)
	}
	out := exportCSV(t, d, "Commandes", ExcelFR)
	wantCSV := "\ufeffCode;Produit;Qté;Prix;Total;Date;Payé;Note\r\n" +
		"1;Pomme;3;1,5;4,5;15/01/2024;VRAI;1-Pomme\r\n" +
		"2;Poire;2;2;4;16/01/2024;FAUX;1x\r\n" +
		"3;Pomme;10;1,5;15;17/01/2024 12:00:00;;libre\r\n"
	if out != wantCSV {
		t.Errorf("export :\n%q\nattendu :\n%q", out, wantCSV)
	}
	cells := map[string]string{}
	for _, c := range l.Cells {
		cells[cellName(c.Row, c.Col)] = c.Raw + " → " + c.Value
	}
	for ref, w := range map[string]string{
		"A7": "Total général → ",
		"B7": "=SOMME(Commandes[Total]) → 23,5",
		"C7": "=SOMME(Commandes[Total])*2 → 47",
		"D7": "2 → ",
		"E7": "Riche → ", // texte enrichi, sans l'indication phonétique
	} {
		if cells[ref] != w {
			t.Errorf("%s : %q, attendu %q", ref, cells[ref], w)
		}
	}

	l2, err := d.Layout(ctx, res.Sheets[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(l2.Tables) != 2 || l2.Tables[1].Name != "Tableau1" || l2.Tables[1].Col != 3 || l2.Tables[1].Columns[0].Type != grid.Text {
		t.Errorf("Produits : %+v", l2.Tables)
	}
	if out := exportCSV(t, d, "Tableau1", Standard); out != "Pays\r\nFrance\r\n7\r\n" {
		t.Errorf("Tableau1 : %q", out)
	}

	// Une seule étape d'annulation pour tout le classeur.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "après annulation")
	if l, _ := d.Layout(ctx, 1); len(l.Sheets) != 1 {
		t.Fatalf("feuilles après annulation : %v", l.Sheets)
	}
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "après rétablissement")
	if out := exportCSV(t, d, "Commandes", ExcelFR); out != wantCSV {
		t.Errorf("après rétablissement :\n%q", out)
	}
}

func TestIslands(t *testing.T) {
	grid := []string{
		"AA.B",
		"AA..",
		"....",
		"C.DD",
		".DDD",
		"....",
		"E...",
		".E..",
	}
	b := newIslands()
	for r, line := range grid {
		for c, ch := range line {
			if ch != '.' {
				b.add(r, c)
			}
		}
	}
	got := fmt.Sprint(b.rects())
	// A et B sont séparés par une colonne vide ; C touche D par un coin, de
	// même que les deux E : chaque paire forme un seul îlot.
	want := "[{0 0 1 1} {0 3 0 3} {3 0 4 3} {6 0 7 1}]"
	if got != want {
		t.Errorf("îlots %s, attendu %s", got, want)
	}
}

func TestSplitFormula(t *testing.T) {
	for _, c := range []struct{ src, key string }{
		{"C2*D2", "\x00|false|{0 false}|{0 false}|{-2 false}|{-2 false}|false\x00*\x00|false|{0 false}|{0 false}|{-1 false}|{-1 false}|false\x00"},
		{"LOG10(A1)", "LOG10(\x00|false|{-1 false}|{-1 false}|{-4 false}|{-4 false}|false\x00)"},
		{`"A1"&TRUE`, `"A1"&TRUE`},
		{"'Feuille 2'!$B:$B", "\x00Feuille 2|true|{0 false}|{0 false}|{1 true}|{1 true}|true\x00"},
	} {
		ps, err := splitFormula(c.src, 1, 4)
		if err != nil {
			t.Errorf("%s : %v", c.src, err)
			continue
		}
		if k := key(ps); k != c.key {
			t.Errorf("%s : %q, attendu %q", c.src, k, c.key)
		}
	}
	for _, src := range []string{"Table1[@Prix]", "SUM(1:1)", "Taux!x", "{1,2}"} {
		if _, err := splitFormula(src, 0, 0); err == nil {
			t.Errorf("%s : erreur attendue", src)
		}
	}
}

func TestFormatKind(t *testing.T) {
	for code, want := range map[string]dateKind{
		"dd/mm/yyyy": dateOnly, "mmm-yy": dateOnly, "h:mm": timeOnly, "[h]:mm:ss": timeOnly,
		"dd/mm/yyyy\\ hh:mm": dateTime, "0.00": notDate, `#,##0 "jours"`: notDate, "[Red]0.0": notDate,
		"General": notDate, "mmmm": dateOnly,
	} {
		if got := formatKind(code); got != want {
			t.Errorf("%q : %d, attendu %d", code, got, want)
		}
	}
}

// Ce que l'import ne reprend pas est signalé : graphiques, tableaux croisés,
// macros, commentaires, mise en forme conditionnelle, validations.
func TestXLSXImportLost(t *testing.T) {
	ctx := context.Background()
	sheet := `<sheetData>` + row(1, cs("A1", 0), cs("B1", 1)) + row(2, cn("A2", "1"), cn("B2", "2")) + row(3, cn("A3", "3"), cn("B3", "4")) + `</sheetData>` +
		`<conditionalFormatting sqref="A2:A3"><cfRule type="cellIs" priority="1" operator="greaterThan"><formula>1</formula></cfRule></conditionalFormatting>` +
		`<dataValidations count="1"><dataValidation type="list" sqref="B2:B3"><formula1>"1,2"</formula1></dataValidation></dataValidations>`
	path := xlsxFixtureParts(t, []string{"A", "B"}, [][2]string{{"Données", sheet}}, map[string]string{
		"xl/charts/chart1.xml":           `<chartSpace/>`,
		"xl/charts/chart2.xml":           `<chartSpace/>`,
		"xl/pivotTables/pivotTable1.xml": `<pivotTableDefinition/>`,
		"xl/vbaProject.bin":              `x`,
		"xl/comments1.xml":               `<comments><commentList><comment ref="A1"/><comment ref="B1"/></commentList></comments>`,
	})
	d := open(t)
	res, err := ImportXLSX(ctx, d.Model, path)
	if err != nil {
		t.Fatal(err)
	}
	warn := strings.Join(res.Warnings, "\n")
	for _, w := range []string{
		"2 graphiques non importés",
		"1 tableau croisé dynamique importé en valeurs",
		"Macros VBA non importées",
		"2 commentaires non importés",
		"Feuille « Données » : 1 règle de mise en forme conditionnelle non importée",
		"Feuille « Données » : 1 validation de données non importée",
	} {
		if !strings.Contains(warn, w) {
			t.Errorf("avertissement attendu %q dans :\n%s", w, warn)
		}
	}
}
