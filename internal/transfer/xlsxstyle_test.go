package transfer

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

const xmlNS = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
const relNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"

// bookParts écrit un classeur à partir de ses feuilles (contenu de
// <worksheet>, rangé dans l'ordre), de ses styles et d'autres parties. Les
// relations de la feuille i se donnent dans parts["xl/worksheets/_rels/sheet<i>.xml.rels"].
func bookParts(t *testing.T, sheets [][2]string, styles string, parts map[string]string) string {
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
	put("_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="`+relNS+`officeDocument" Target="xl/workbook.xml"/></Relationships>`)
	var wbSheets, wbRels strings.Builder
	for i, s := range sheets {
		name, state, _ := strings.Cut(s[0], "|")
		if state != "" {
			state = ` state="` + state + `"`
		}
		fmt.Fprintf(&wbSheets, `<sheet name="%s" sheetId="%d"%s r:id="rId%d"/>`, name, i+1, state, i+1)
		fmt.Fprintf(&wbRels, `<Relationship Id="rId%d" Type="%sworksheet" Target="worksheets/sheet%d.xml"/>`, i+1, relNS, i+1)
		put(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), `<worksheet `+xmlNS+`>`+s[1]+`</worksheet>`)
	}
	fmt.Fprintf(&wbRels, `<Relationship Id="rT" Type="%sstyles" Target="styles.xml"/>`, relNS)
	if _, ok := parts["xl/theme/theme1.xml"]; ok {
		fmt.Fprintf(&wbRels, `<Relationship Id="rTh" Type="%stheme" Target="theme/theme1.xml"/>`, relNS)
	}
	wb := parts["xl/workbook.xml"]
	if wb == "" {
		wb = `<workbook ` + xmlNS + `><sheets>` + wbSheets.String() + `</sheets></workbook>`
	}
	put("xl/workbook.xml", wb)
	put("xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+wbRels.String()+parts["workbookRels"]+`</Relationships>`)
	put("xl/styles.xml", `<styleSheet `+xmlNS+`>`+styles+`</styleSheet>`)
	for name, body := range parts {
		if name != "xl/workbook.xml" && name != "workbookRels" {
			put(name, body)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func is(ref, s string) string {
	return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, s)
}

// styleOf : propriétés d'un style JSON, pour comparer.
func styleOf(t *testing.T, js string) map[string]any {
	t.Helper()
	m := map[string]any{}
	if js != "" {
		if err := json.Unmarshal([]byte(js), &m); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func wantStyle(t *testing.T, what string, got map[string]any, want map[string]any) {
	t.Helper()
	for k, v := range want {
		if fmt.Sprint(got[k]) != fmt.Sprint(v) {
			t.Errorf("%s : %s = %v, attendu %v (style %v)", what, k, got[k], v, got)
		}
	}
}

const styleSheet = `<numFmts count="2"><numFmt numFmtId="164" formatCode="#,##0.00\ &quot;€&quot;"/><numFmt numFmtId="165" formatCode="0.0&quot; kg&quot;"/></numFmts>` +
	`<fonts count="4"><font><sz val="11"/><color theme="1"/><name val="Calibri"/></font>` +
	`<font><b/><sz val="14"/><color rgb="FFC00000"/><name val="Arial"/></font>` +
	`<font><i/><u/><strike/><sz val="11"/><color theme="4" tint="-0.249977111117893"/><name val="Calibri"/></font>` +
	`<font><sz val="11"/><color indexed="10"/><name val="Calibri"/></font></fonts>` +
	`<fills count="4"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill>` +
	`<fill><patternFill patternType="solid"><fgColor rgb="FFFFFF00"/><bgColor indexed="64"/></patternFill></fill>` +
	`<fill><patternFill patternType="solid"><fgColor theme="5" tint="0.79998168889431442"/></patternFill></fill></fills>` +
	`<borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border>` +
	`<border><left style="thin"><color auto="1"/></left><right style="double"><color rgb="FF0000FF"/></right><top style="medium"/><bottom style="dashed"><color theme="1"/></bottom><diagonal/></border></borders>` +
	`<cellXfs count="8"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>` +
	`<xf numFmtId="0" fontId="1" fillId="2" borderId="1" applyFont="1"><alignment horizontal="center" vertical="top" wrapText="1"/></xf>` +
	`<xf numFmtId="164" fontId="2" fillId="3" borderId="0"/>` +
	`<xf numFmtId="14" fontId="0" fillId="0" borderId="0"/>` +
	`<xf numFmtId="10" fontId="3" fillId="0" borderId="0"/>` +
	`<xf numFmtId="165" fontId="0" fillId="0" borderId="0"/>` +
	`<xf numFmtId="4" fontId="0" fillId="2" borderId="0"/>` +
	`<xf numFmtId="0" fontId="1" fillId="0" borderId="0"/></cellXfs>`

// Styles d'un classeur : police, couleurs (thème avec nuance, palette),
// remplissage, bordures, alignement, formats de nombre, sur des cellules
// libres et sur un tableau (en-têtes, colonnes, cellules qui s'écartent),
// et la feuille : largeurs, hauteurs, masquage, fusion, volets, onglet.
func TestXLSXImportStyles(t *testing.T) {
	ctx := context.Background()
	sheet := `<sheetPr><tabColor rgb="FF00B050"/></sheetPr>` +
		`<sheetViews><sheetView zoomScale="120" workbookViewId="0"><pane ySplit="3" topLeftCell="A4" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>` +
		`<cols><col min="1" max="1" width="20" customWidth="1"/><col min="5" max="5" width="9.140625" hidden="1"/></cols>` +
		`<sheetData>` +
		`<row r="1" ht="30" customHeight="1"><c r="A1" s="1" t="inlineStr"><is><t>Titre</t></is></c><c r="B1" s="1"/><c r="C1" s="1"/></row>` +
		`<row r="3"><c r="A3" s="7" t="inlineStr"><is><t>Produit</t></is></c><c r="B3" s="7" t="inlineStr"><is><t>Prix</t></is></c><c r="C3" s="7" t="inlineStr"><is><t>Date</t></is></c><c r="D3" s="7" t="inlineStr"><is><t>Part</t></is></c></row>` +
		`<row r="4">` + is("A4", "Pomme") + `<c r="B4" s="2"><v>1.5</v></c><c r="C4" s="3"><v>45306</v></c><c r="D4" s="4"><v>0.25</v></c></row>` +
		`<row r="5">` + is("A5", "Poire") + `<c r="B5" s="2"><v>2</v></c><c r="C5" s="3"><v>45307</v></c><c r="D5" s="5"><v>0.5</v></c></row>` +
		`<row r="6" hidden="1">` + is("A6", "Kiwi") + `<c r="B6" s="6"><v>3</v></c><c r="C6" s="3"><v>45308</v></c><c r="D6" s="4"><v>0.75</v></c></row>` +
		`</sheetData><mergeCells count="1"><mergeCell ref="A1:C1"/></mergeCells>`
	path := bookParts(t, [][2]string{{"Ventes", sheet}}, styleSheet, nil)
	d := open(t)
	res, err := ImportXLSX(ctx, d.Model, path)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "après import")
	l, err := d.Layout(ctx, res.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	var info canvas.SheetInfo
	for _, s := range l.Sheets {
		if s.ID == res.Sheets[0] {
			info = s
		}
	}
	if info.Color != "#00b050" || info.FreezeRows != 3 || info.Zoom != 120 {
		t.Errorf("feuille : %+v", info)
	}
	if len(l.Widths) != 1 || l.Widths[0].Col != 0 || l.Widths[0].Width != 140 {
		t.Errorf("largeurs : %+v", l.Widths)
	}
	if len(l.Heights) != 1 || l.Heights[0].Height != 40 {
		t.Errorf("hauteurs : %+v", l.Heights)
	}
	var hidden []string
	for _, s := range l.ColStyles {
		hidden = append(hidden, fmt.Sprintf("c%d:%s", s.Index, s.Style))
	}
	for _, s := range l.RowStyles {
		hidden = append(hidden, fmt.Sprintf("r%d:%s", s.Index, s.Style))
	}
	if got := strings.Join(hidden, " "); got != `c4:{"hd":true} r5:{"hd":true}` {
		t.Errorf("lignes et colonnes : %s", got)
	}
	free := map[[2]int]map[string]any{}
	for _, s := range l.Styles {
		free[[2]int{s.Row, s.Col}] = styleOf(t, s.Style)
	}
	wantStyle(t, "A1", free[[2]int{0, 0}], map[string]any{"b": true, "fs": 14, "ff": "Arial", "fg": "#c00000", "bg": "#ffff00",
		"bl": true, "br": "double #0000ff", "bt": "medium", "bb": "dashed #000000", "al": "center", "va": "top", "wr": true, "mr": 1, "mc": 3})
	wantStyle(t, "B1", free[[2]int{0, 1}], map[string]any{"bg": "#ffff00"})
	if len(l.Tables) != 1 {
		t.Fatalf("tableaux : %d", len(l.Tables))
	}
	tb := l.Tables[0]
	cols := tb.Columns
	wantStyle(t, "en-tête", styleOf(t, cols[0].HeaderStyle), map[string]any{"b": true, "fg": "#c00000", "ff": "Arial"})
	wantStyle(t, "Prix", styleOf(t, cols[1].Style), map[string]any{"nf": "€", "i": true, "u": true, "x": true, "fg": "#2f5597", "bg": "#fbe5d6"})
	wantStyle(t, "Date", styleOf(t, cols[2].Style), map[string]any{"nf": "date"})
	wantStyle(t, "Part", styleOf(t, cols[3].Style), map[string]any{"nf": "%", "fg": "#ff0000"})
	cells, err := d.TableCellStyles(ctx, tb.ID)
	if err != nil {
		t.Fatal(err)
	}
	own := map[[2]int]map[string]any{}
	for _, c := range cells {
		own[[2]int{c.Row, c.Col}] = styleOf(t, c.Style)
	}
	if len(own) != 2 {
		t.Errorf("cellules de tableau : %v", own)
	}
	wantStyle(t, "D5", own[[2]int{1, 3}], map[string]any{"nc": "0,0\" kg\"", "nf": nil, "fg": nil})
	wantStyle(t, "B6", own[[2]int{2, 1}], map[string]any{"nf": "0,00", "bg": "#ffff00", "i": nil})
}

// Un fichier exporté puis réimporté garde sa mise en forme.
func TestXLSXStyleRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := exportFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(d.ApplyStyle(ctx, 1, 0, 0, 0, 0, canvas.Style{"ff": "Georgia", "fs": 13.0, "bb": "double #c00000", "nc": `# ##0,0 "kg"`}))
	must(d.ApplyStyle(ctx, 1, 6, 1, 6, 1, canvas.Style{"u": true, "va": "top", "wr": true}))
	p := filepath.Join(t.TempDir(), "rt.xlsx")
	if _, err := ExportXLSXFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	d2 := open(t)
	res, err := ImportXLSX(ctx, d2.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d2, "après réimport")
	l, err := d2.Layout(ctx, res.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	free := map[[2]int]map[string]any{}
	for _, s := range l.Styles {
		free[[2]int{s.Row, s.Col}] = styleOf(t, s.Style)
	}
	wantStyle(t, "fusion", free[[2]int{12, 0}], map[string]any{"mr": 1, "mc": 3})
	if len(l.Tables) < 2 || l.Tables[1].Row != 6 {
		t.Fatalf("tableaux : %+v", l.Tables)
	}
	// B7 : en-tête du tableau A7:B11 (la somme et les formules dessous).
	wantStyle(t, "B7", styleOf(t, l.Tables[1].Columns[1].HeaderStyle), map[string]any{"b": true, "nf": "€", "bg": "#ffff00", "al": "right", "u": true, "va": "top", "wr": true})
	h := styleOf(t, l.Tables[0].Columns[0].HeaderStyle)
	wantStyle(t, "en-tête", h, map[string]any{"ff": "Georgia", "fs": 13, "bb": "double #c00000", "nc": `# ##0,0 "kg"`})
	if len(l.Widths) == 0 || l.Widths[0].Width != 140 {
		t.Errorf("largeurs : %+v", l.Widths)
	}
	var info canvas.SheetInfo
	for _, s := range l.Sheets {
		if s.ID == res.Sheets[0] {
			info = s
		}
	}
	if info.FreezeRows != 1 {
		t.Errorf("volets : %+v", info)
	}
}
