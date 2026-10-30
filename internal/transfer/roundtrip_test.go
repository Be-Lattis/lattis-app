package transfer

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Aller-retour d'un fichier .xlsx ouvert dans Lattis puis réenregistré dans
// son format (classeur adossé au .xlsx) : ce qui revient tel quel, ce qui
// est réécrit autrement (même résultat), ce qui est perdu. Le tableau de
// fidélité est écrit dans le journal de l'essai (go test -v -run RoundTrip).

// excelLikeFixture écrit un classeur comme l'enregistre un tableur : un
// tableau Excel avec une colonne calculée en références structurées, des
// formules en adresses A1 (absolues, d'une autre feuille, plages), un nom
// défini, des formats de nombre, des dates, des styles, une fusion, des
// largeurs, des volets figés, une mise en forme conditionnelle, une
// validation, une note, un lien et une plage ordinaire.
func excelLikeFixture(t *testing.T) string {
	t.Helper()
	sst := []string{"Produit", "Qté", "Prix", "Total", "Date", "Pomme", "Poire", "Prune", "Kiwi",
		"Somme", "TVA", "TTC", "Moyenne", "Partiel", "Paramètre", "Recherche", "Erreur", "Texte", "Absolu", "Si", "Arrondi", "Somme.si", "Ifs", "Nb.si",
		"Coef", "Clé", "Valeur", "a", "b", "Fusion", "Site", "Produit vendu"}
	s := func(ref string, i int, st string) string {
		return fmt.Sprintf(`<c r="%s"%s t="s"><v>%d</v></c>`, ref, st, i)
	}
	n := func(ref, v, st string) string { return fmt.Sprintf(`<c r="%s"%s><v>%s</v></c>`, ref, st, v) }
	f := func(ref, fm, v, st string) string {
		return fmt.Sprintf(`<c r="%s"%s><f>%s</f><v>%s</v></c>`, ref, st, xmlEscape(fm), v)
	}
	fs := func(ref, fm, v string) string {
		return fmt.Sprintf(`<c r="%s" t="str"><f>%s</f><v>%s</v></c>`, ref, xmlEscape(fm), v)
	}
	const bold, money, date, pct, fill, border, center = ` s="1"`, ` s="2"`, ` s="3"`, ` s="4"`, ` s="5"`, ` s="6"`, ` s="7"`
	total := `TVentes[[#This Row],[Qté]]*TVentes[[#This Row],[Prix]]`
	data := [][]string{{"5", "3", "1.5", "4.5", "45306"}, {"6", "2", "2", "4", "45307"}, {"7", "10", "0.5", "5", "45308"}, {"8", "5", "3", "15", "45309"}}
	labels := []struct {
		label int
		cell  string
	}{
		{9, f("H1", "SUM(TVentes[Total])", "28.5", bold)},
		{10, n("H2", "0.2", pct)},
		{11, f("H3", "H1*(1+Taux)", "34.2", money)},
		{12, f("H4", "AVERAGE(C2:C5)", "1.75", "")},
		{13, f("H5", "SUM(D2:D3)", "8.5", "")},
		{14, f("H6", "Paramètres!B1*2", "3", "")},
		{15, f("H7", `VLOOKUP("Poire",A2:E5,3,FALSE)`, "2", "")},
		{16, fs("H8", `IFERROR(1/0,"n/a")`, "n/a")},
		{17, fs("H9", `TEXT(H1,"0.00")`, "28.50")},
		{18, f("H10", "$H$1+$B$2", "31.5", "")},
		{19, fs("H11", `IF(H1>10,"haut","bas")`, "haut")},
		{20, f("H12", "ROUND(H3,1)", "34.2", fill)},
		{21, f("H13", `SUMIF(A2:A5,"Pomme",D2:D5)`, "4.5", border)},
		{22, fs("H14", `_xlfn.IFS(H1>100,"a",TRUE,"b")`, "b")},
		{23, f("H15", `COUNTIF(TVentes[Qté],"&gt;2")`, "3", center)},
	}
	var rows strings.Builder
	for r := 1; r <= 16; r++ {
		fmt.Fprintf(&rows, `<row r="%d">`, r)
		if r == 1 {
			for i, c := range []string{"A", "B", "C", "D", "E"} {
				rows.WriteString(s(c+"1", i, bold))
			}
		} else if r <= 5 {
			d := data[r-2]
			i, _ := fmt.Sscan(d[0])
			_ = i
			rows.WriteString(s(fmt.Sprintf("A%d", r), map[string]int{"5": 5, "6": 6, "7": 7, "8": 8}[d[0]], ""))
			rows.WriteString(n(fmt.Sprintf("B%d", r), d[1], ""))
			rows.WriteString(n(fmt.Sprintf("C%d", r), d[2], money))
			rows.WriteString(f(fmt.Sprintf("D%d", r), total, d[3], money))
			rows.WriteString(n(fmt.Sprintf("E%d", r), d[4], date))
		}
		if r <= 15 {
			l := labels[r-1]
			rows.WriteString(s(fmt.Sprintf("G%d", r), l.label, ""))
			rows.WriteString(strings.Replace(l.cell, "&amp;gt;", "&gt;", 1))
		}
		if r == 16 {
			rows.WriteString(s("G16", 29, center))
		}
		rows.WriteString(`</row>`)
	}
	rows.WriteString(`<row r="18">` + s("G18", 30, ` s="8"`) + `</row>`) // style « Lien hypertexte »
	sheet1 := `<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>` +
		`<cols><col min="1" max="1" width="20" customWidth="1"/><col min="7" max="7" width="18" customWidth="1"/></cols>` +
		`<sheetData>` + rows.String() + `</sheetData>` +
		`<mergeCells count="1"><mergeCell ref="G16:H16"/></mergeCells>` +
		`<conditionalFormatting sqref="B2:B5"><cfRule type="cellIs" dxfId="0" priority="1" operator="greaterThan"><formula>4</formula></cfRule></conditionalFormatting>` +
		`<dataValidations count="1"><dataValidation type="list" allowBlank="1" showErrorMessage="1" sqref="A2:A5"><formula1>"Pomme,Poire,Prune,Kiwi"</formula1></dataValidation></dataValidations>` +
		`<hyperlinks><hyperlink ref="G18" r:id="rL1"/></hyperlinks>` +
		`<legacyDrawing r:id="rV1"/><tableParts count="1"><tablePart r:id="rT1"/></tableParts>`
	sheet2 := `<sheetData>` +
		`<row r="1">` + s("A1", 24, "") + n("B1", "1.5", "") + `</row>` +
		`<row r="3">` + s("A3", 25, bold) + s("B3", 26, bold) + `</row>` +
		`<row r="4">` + s("A4", 27, "") + n("B4", "1", "") + `</row>` +
		`<row r="5">` + s("A5", 28, "") + n("B5", "2", "") + `</row>` +
		`<row r="7">` + f("A7", "SUM(B4:B5)", "3", "") + `</row>` +
		`</sheetData>`
	const ns = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	const rel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
	var si strings.Builder
	for _, x := range sst {
		fmt.Fprintf(&si, `<si><t>%s</t></si>`, xmlEscape(x))
	}
	parts := [][2]string{
		{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="vml" ContentType="application/vnd.openxmlformats-officedocument.vmlDrawing"/></Types>`},
		{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="` + rel + `officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<workbook ` + ns + `><sheets><sheet name="Ventes" sheetId="1" r:id="rId1"/><sheet name="Paramètres" sheetId="2" r:id="rId2"/></sheets><definedNames><definedName name="Taux">Ventes!$H$2</definedName></definedNames></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="` + rel + `worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="` + rel + `worksheet" Target="worksheets/sheet2.xml"/><Relationship Id="rS" Type="` + rel + `sharedStrings" Target="sharedStrings.xml"/><Relationship Id="rT" Type="` + rel + `styles" Target="styles.xml"/></Relationships>`},
		{"xl/sharedStrings.xml", `<sst ` + ns + `>` + si.String() + `</sst>`},
		{"xl/styles.xml", `<styleSheet ` + ns + `><numFmts count="1"><numFmt numFmtId="164" formatCode="#,##0.00\ &quot;€&quot;"/></numFmts>` +
			`<fonts count="3"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font><font><u/><sz val="11"/><color rgb="FF0563C1"/><name val="Calibri"/></font></fonts>` +
			`<fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FFFFFF00"/></patternFill></fill></fills>` +
			`<borders count="2"><border><left/><right/><top/><bottom/></border><border><left style="thin"/><right style="thin"/><top style="thin"/><bottom style="thin"/></border></borders>` +
			`<cellXfs count="9"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" applyFont="1"/><xf numFmtId="164" fontId="0" fillId="0" borderId="0" applyNumberFormat="1"/><xf numFmtId="14" fontId="0" fillId="0" borderId="0" applyNumberFormat="1"/><xf numFmtId="10" fontId="0" fillId="0" borderId="0" applyNumberFormat="1"/><xf numFmtId="0" fontId="0" fillId="2" borderId="0" applyFill="1"/><xf numFmtId="0" fontId="0" fillId="0" borderId="1" applyBorder="1"/><xf numFmtId="0" fontId="0" fillId="0" borderId="0" applyAlignment="1"><alignment horizontal="center"/></xf><xf numFmtId="0" fontId="2" fillId="0" borderId="0" applyFont="1"/></cellXfs>` +
			`<dxfs count="1"><dxf><fill><patternFill><bgColor rgb="FFFFC7CE"/></patternFill></fill></dxf></dxfs></styleSheet>`},
		{"xl/worksheets/sheet1.xml", `<worksheet ` + ns + `>` + sheet1 + `</worksheet>`},
		{"xl/worksheets/_rels/sheet1.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rT1" Type="` + rel + `table" Target="../tables/table1.xml"/><Relationship Id="rL1" Type="` + rel + `hyperlink" Target="https://example.com/" TargetMode="External"/><Relationship Id="rC1" Type="` + rel + `comments" Target="../comments1.xml"/><Relationship Id="rV1" Type="` + rel + `vmlDrawing" Target="../drawings/vmlDrawing1.vml"/></Relationships>`},
		{"xl/tables/table1.xml", `<table ` + ns + ` id="1" name="TVentes" displayName="TVentes" ref="A1:E5" totalsRowShown="0"><autoFilter ref="A1:E5"/><tableColumns count="5"><tableColumn id="1" name="Produit"/><tableColumn id="2" name="Qté"/><tableColumn id="3" name="Prix"/><tableColumn id="4" name="Total"><calculatedColumnFormula>` + xmlEscape(total) + `</calculatedColumnFormula></tableColumn><tableColumn id="5" name="Date"/></tableColumns><tableStyleInfo name="TableStyleMedium2" showRowStripes="1"/></table>`},
		{"xl/comments1.xml", `<comments ` + ns + `><authors><author>Louis</author></authors><commentList><comment ref="A1" authorId="0"><text><t>Produit vendu</t></text></comment></commentList></comments>`},
		{"xl/drawings/vmlDrawing1.vml", `<xml xmlns:v="urn:schemas-microsoft-com:vml"></xml>`},
		{"xl/worksheets/sheet2.xml", `<worksheet ` + ns + `>` + sheet2 + `</worksheet>`},
	}
	p := filepath.Join(t.TempDir(), "source.xlsx")
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, part := range parts {
		w, err := z.Create(part[0])
		if err != nil {
			t.Fatal(err)
		}
		body := part[1]
		if !strings.HasSuffix(part[0], ".vml") {
			body = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" + body
		}
		w.Write([]byte(body))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// keepXLSX ouvre un fichier .xlsx comme un classeur adossé au fichier
// (OpenXLSX) et le réenregistre au format .xlsx ; rend le fichier écrit.
func keepXLSX(t *testing.T, src, name string) string {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := OpenXLSX(ctx, d.Model, src); err != nil {
		t.Fatal(err)
	}
	check(t, d, "après ouverture de "+filepath.Base(src))
	out := filepath.Join(t.TempDir(), name)
	if _, err := ExportXLSXFile(ctx, d.Model, out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Fichier d'un tableur, ouvert puis enregistré en gardant le format : ce
// qui doit revenir tel quel revient.
func TestXLSXRoundTripExcelFile(t *testing.T) {
	src := excelLikeFixture(t)
	x1 := keepXLSX(t, src, "x1.xlsx")
	a, b := dumpXLSX(t, src), dumpXLSX(t, x1)
	fd := compareXLSX(a, b)
	t.Logf("Fichier d'un tableur → Lattis → .xlsx :\n%s", fd.report())

	// Tout ce qui doit revenir à l'identique.
	for _, topic := range []string{"valeurs", "formats de nombre", "styles", "fusions", "largeurs de colonnes", "volets figés",
		"mise en forme conditionnelle", "validations", "liens hypertexte", "notes", "noms définis", "tableaux Excel", "feuilles"} {
		if len(fd.lost[topic]) > 0 {
			t.Errorf("%s : perdus %v", topic, fd.lost[topic])
		}
	}
	if l := fd.lost["formules"]; len(l) > 0 {
		t.Errorf("formules perdues : %v", l)
	}
	// Formules gardées telles quelles (à $ et apostrophes près).
	vs := b.sheet("Ventes").cells
	for ref, want := range map[string]string{
		"D2": "TVentes[[#This Row],[Qté]]*TVentes[[#This Row],[Prix]]", "H1": "SUM(TVentes[Total])", "H3": "H1*(1+Taux)",
		"H8": `IFERROR(1/0,"n/a")`, "H9": `TEXT(H1,"0.00")`, "H11": `IF(H1>10,"haut","bas")`, "H12": "ROUND(H3,1)",
		"H14": `_xlfn.IFS(H1>100,"a",TRUE,"b")`, "H15": `COUNTIF(TVentes[Qté],">2")`, "H6": "'Paramètres'!B1*2",
	} {
		if got := vs[ref].f; got != want {
			t.Errorf("%s : formule %q, attendu %q", ref, got, want)
		}
	}
	// La plage ordinaire de Paramètres reste une plage (pas de tableau Excel).
	if p := b.sheet("Paramètres"); len(p.tables) != 0 || p.cells["A7"].f != "SUM($B$4:$B$5)" {
		t.Errorf("Paramètres : tableaux %v, A7 %q", p.tables, p.cells["A7"].f)
	}
	// Point fixe : réenregistré une deuxième fois, le fichier ne change plus.
	x2 := keepXLSX(t, x1, "x2.xlsx")
	fd2 := compareXLSX(b, dumpXLSX(t, x2))
	for topic, l := range fd2.lost {
		t.Errorf("deuxième passage, %s : %v", topic, l)
	}
	for topic, l := range fd2.rewritten {
		t.Errorf("deuxième passage, %s réécrits : %v", topic, l)
	}
}

// Classeur fait dans Lattis, enregistré en .xlsx, rouvert et réenregistré :
// le fichier revient identique (point fixe), graphiques, tableaux croisés
// en valeurs, images et formes compris.
func TestXLSXRoundTripLattisFile(t *testing.T) {
	ctx := context.Background()
	d := exportFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := d.CreateChart(ctx, 1, 20, 0, canvas.ChartConfig{Source: "Ventes", Kind: "bar", X: "Produit", Series: []string{"Prix"}})
	must(err)
	must(d.SetRules(ctx, "Ventes", 1, []canvas.Rule{{ID: "r1", Op: ">", Value: "1", Style: canvas.Style{"bg": "#FF0000"}}}))
	must(d.SetNote(ctx, 1, 6, 0, "À vérifier"))
	must(d.ApplyStyle(ctx, 1, 6, 3, 6, 3, canvas.Style{"dv": []any{"oui", "non"}}))
	must(d.SetLink(ctx, 1, 14, 0, "https://example.com/", "Site"))
	var img bytes.Buffer
	must(png.Encode(&img, image.NewGray(image.Rect(0, 0, 30, 20))))
	_, err = d.CreateImage(ctx, 1, 2, 9, img.Bytes(), canvas.ObjectConfig{Alt: "logo"})
	must(err)
	_, err = d.CreatePivot(ctx, canvas.PivotConfig{Source: "Ventes", Rows: "Produit", Value: "Prix", Agg: "sum"})
	must(err)
	x1 := filepath.Join(t.TempDir(), "lattis.xlsx")
	_, err = ExportXLSXFile(ctx, d.Model, x1)
	must(err)
	x2 := keepXLSX(t, x1, "x2.xlsx")
	a, b := dumpXLSX(t, x1), dumpXLSX(t, x2)
	fd := compareXLSX(a, b)
	t.Logf("Classeur Lattis → .xlsx → Lattis → .xlsx :\n%s", fd.report())
	for topic, l := range fd.lost {
		if topic == "tableaux croisés" {
			continue // exportés en valeurs : rien à relire
		}
		t.Errorf("%s : perdus %v", topic, l)
	}
	for topic, l := range fd.rewritten {
		t.Errorf("%s : réécrits %v", topic, l)
	}
}
