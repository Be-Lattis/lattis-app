package transfer

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Liens hypertexte : exportés en balises <hyperlink> (relation pour une
// adresse web, emplacement pour une cellule du classeur), relus à l'import.
func TestXLSXLinksRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom", "Site"}, {"Lattis", "lattis"}, {"Autre", "autre"}})
	must(err)
	must(d.RenameSheet(ctx, 1, "Mes données"))
	must(d.SetLink(ctx, 1, 1, 1, "https://lattis.be/?a=1&b=2", ""))
	must(d.SetLink(ctx, 1, 5, 0, "jean@exemple.fr", "Écrire"))
	must(d.SetLink(ctx, 1, 7, 0, "#'Mes données'!B3", "Aller"))
	must(d.SetLink(ctx, 1, 9, 0, "#Feuille1", "Le tableau"))
	p := filepath.Join(t.TempDir(), "liens.xlsx")
	if _, err := ExportXLSXFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	parts := readZip(t, p)
	contains(t, "feuille", parts["xl/worksheets/sheet1.xml"],
		`<hyperlinks><hyperlink ref="B2" r:id="rId2"/>`,
		`<hyperlink ref="A6" r:id="rId3"/>`,
		`<hyperlink ref="A8" location="'Mes données'!B3"/>`,
		`<hyperlink ref="A10" location="'Mes données'!A1:B3"/>`)
	contains(t, "relations", parts["xl/worksheets/_rels/sheet1.xml.rels"],
		`Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://lattis.be/?a=1&amp;b=2" TargetMode="External"`,
		`Target="mailto:jean@exemple.fr" TargetMode="External"`)

	// Import : les liens reviennent (cellules libres et de tableau).
	back := open(t)
	if _, err := ImportXLSX(ctx, back.Model, p); err != nil {
		t.Fatal(err)
	}
	l, err := back.Layout(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var sheet int64
	for _, s := range l.Sheets {
		if s.Name == "Mes données" {
			sheet = s.ID
		}
	}
	l, err = back.Layout(ctx, sheet)
	if err != nil {
		t.Fatal(err)
	}
	got := map[[2]int]string{}
	for _, s := range l.Styles {
		if ln, _ := parseStyle(s.Style)["ln"].(string); ln != "" {
			got[[2]int{s.Row, s.Col}] = ln
		}
	}
	for k, want := range map[[2]int]string{{5, 0}: "mailto:jean@exemple.fr", {7, 0}: "#'Mes données'!B3", {9, 0}: "#'Mes données'!A1:B3"} {
		if got[k] != want {
			t.Errorf("lien importé %v : %q ; attendu %q", k, got[k], want)
		}
	}
	if len(l.Tables) != 1 {
		t.Fatalf("tableaux importés : %+v", l.Tables)
	}
	cs, err := back.TableCellStyles(ctx, l.Tables[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Row != 0 || cs[0].Col != 1 || parseStyle(cs[0].Style)["ln"] != "https://lattis.be/?a=1&b=2" {
		t.Fatalf("lien du tableau importé : %+v", cs)
	}
}

// Un fichier .xlsx dont les liens visent file: ou javascript: : laissés.
func TestXLSXLinksRefused(t *testing.T) {
	sheet := `<sheetData>` +
		row(1, cs("A1", 0), cs("B1", 0), cs("C1", 0)) + `</sheetData><hyperlinks><hyperlink ref="A1" r:id="rId1"/><hyperlink ref="B1" r:id="rId2"/><hyperlink ref="C1" r:id="rId3"/></hyperlinks>`
	rels := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="file:///C:/secret.txt" TargetMode="External"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="javascript:alert(1)" TargetMode="External"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://ok.example" TargetMode="External"/></Relationships>`
	p := xlsxFixtureParts(t, []string{"x"}, [][2]string{{"F", sheet}}, map[string]string{"xl/worksheets/_rels/sheet1.xml.rels": rels})
	x, err := openXLSX(p)
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	if _, _, err := x.plan(context.Background(), map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	links := x.sheetLinks(x.sheets[0])
	if len(links) != 1 || links[0] != (canvas.ImportLink{Row: 0, Col: 2, Target: "https://ok.example"}) {
		t.Fatalf("liens : %+v", links)
	}
}
