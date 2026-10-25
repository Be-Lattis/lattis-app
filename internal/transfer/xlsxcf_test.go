package transfer

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
)

const dxfs = `<dxfs count="1"><dxf><font><b/><color rgb="FF9C0006"/></font><fill><patternFill><bgColor rgb="FFFFC7CE"/></patternFill></fill></dxf></dxfs>`

func rulesOf(t *testing.T, js string) []canvas.Rule {
	t.Helper()
	var rs []canvas.Rule
	if js != "" {
		if err := json.Unmarshal([]byte(js), &rs); err != nil {
			t.Fatal(err)
		}
	}
	return rs
}

// Mise en forme conditionnelle, validations (listes) et notes d'un classeur.
func TestXLSXImportRulesListsNotes(t *testing.T) {
	ctx := context.Background()
	sheet := `<sheetData>` +
		row(1, is("A1", "Produit"), is("B1", "Prix"), is("C1", "Statut")) +
		row(2, is("A2", "Pomme"), cn("B2", "1.5"), is("C2", "Oui")) +
		row(3, is("A3", "Poire"), cn("B3", "2"), is("C3", "Non")) +
		row(4, is("A4", "Kiwi"), cn("B4", "3"), is("C4", "Oui")) + row(7, cn("E7", "10"), cn("F7", "20"), cn("G7", "30")) +
		`</sheetData>` +
		`<conditionalFormatting sqref="B2:B4"><cfRule type="cellIs" dxfId="0" priority="2" operator="greaterThan"><formula>1.5</formula></cfRule>` +
		`<cfRule type="colorScale" priority="3"><colorScale><cfvo type="min"/><cfvo type="percentile" val="50"/><cfvo type="max"/><color rgb="FFF8696B"/><color rgb="FFFFEB84"/><color rgb="FF63BE7B"/></colorScale></cfRule>` +
		`<cfRule type="top10" dxfId="0" priority="4" rank="1"/></conditionalFormatting>` +
		`<conditionalFormatting sqref="A2:C4"><cfRule type="expression" dxfId="0" priority="1" stopIfTrue="1"><formula>AND($B2&gt;1,ISNUMBER(SEARCH("o",$A2)))</formula></cfRule></conditionalFormatting>` +
		`<conditionalFormatting sqref="E7:G7"><cfRule type="dataBar" priority="5"><dataBar><cfvo type="min"/><cfvo type="max"/><color rgb="FF638EC6"/></dataBar></cfRule>` +
		`<cfRule type="iconSet" priority="6"><iconSet iconSet="3Arrows" reverse="1"><cfvo type="percent" val="0"/><cfvo type="percent" val="33"/><cfvo type="percent" val="67" gte="0"/></iconSet></cfRule></conditionalFormatting>` +
		`<conditionalFormatting sqref="H:H"><cfRule type="containsText" dxfId="0" priority="7" operator="containsText" text="urgent"><formula>NOT(ISERROR(SEARCH("urgent",H1)))</formula></cfRule></conditionalFormatting>` +
		`<dataValidations count="2"><dataValidation type="list" allowBlank="1" sqref="C2:C4"><formula1>"Oui,Non"</formula1></dataValidation>` +
		`<dataValidation type="list" sqref="G1"><formula1>$E$7:$G$7</formula1></dataValidation></dataValidations>`
	parts := map[string]string{
		"xl/worksheets/_rels/sheet1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="c1" Type="` + relNS + `comments" Target="../comments1.xml"/>` +
			`<Relationship Id="c2" Type="http://schemas.microsoft.com/office/2017/10/relationships/threadedComment" Target="../threadedComments/threadedComment1.xml"/></Relationships>`,
		"xl/comments1.xml": `<comments ` + xmlNS + `><authors><author>Jean</author><author>tc={1}</author></authors><commentList>` +
			`<comment ref="A1" authorId="0"><text><r><rPr><b/></rPr><t>Jean:</t></r><r><t xml:space="preserve">` + "\n" + `À vérifier</t></r></text></comment>` +
			`<comment ref="E7" authorId="1"><text><t>[Threaded comment] …</t></text></comment></commentList></comments>`,
		"xl/threadedComments/threadedComment1.xml": `<ThreadedComments xmlns="http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments">` +
			`<threadedComment ref="E7" personId="{P1}" id="{1}"><text>Objectif ?</text></threadedComment>` +
			`<threadedComment ref="E7" personId="{P2}" id="{2}" parentId="{1}"><text>Oui, 10.</text></threadedComment></ThreadedComments>`,
		"xl/persons/person.xml": `<personList xmlns="http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments"><person displayName="Ana" id="{P1}"/><person displayName="Luc" id="{P2}"/></personList>`,
		"workbookRels":          `<Relationship Id="rP" Type="http://schemas.microsoft.com/office/2017/10/relationships/person" Target="persons/person.xml"/>`,
	}
	path := bookParts(t, [][2]string{{"Données", sheet}}, styleSheet+dxfs, parts)
	d := open(t)
	res, err := ImportXLSX(ctx, d.Model, path)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "après import")
	if w := strings.Join(res.Warnings, "\n"); w != "" {
		t.Errorf("avertissements :\n%s", w)
	}
	l, err := d.Layout(ctx, res.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	cols := l.Tables[0].Columns
	prix := rulesOf(t, cols[1].Rules)
	var ops []string
	for _, r := range prix {
		ops = append(ops, r.Op)
	}
	if got := strings.Join(ops, " "); got != "formula > scale top" {
		t.Fatalf("règles de Prix : %s", got)
	}
	if f := prix[0]; f.Value != `=ET($B2>1; ESTNUM(CHERCHE("o"; $A2)))` || !f.Stop || f.Style["b"] != true || f.Style["bg"] != "#ffc7ce" || f.Style["fg"] != "#9c0006" {
		t.Errorf("règle formule : %+v", f)
	}
	if prix[1].Value != "1.5" || len(prix[2].Scale) != 0 && prix[2].Scale == nil {
		t.Errorf("règles : %+v", prix)
	}
	if s := prix[2].Scale; len(s) != 3 || s[1]["kind"] != "pctl" || s[1]["value"] != 50.0 || s[2]["color"] != "#63be7b" {
		t.Errorf("nuance : %+v", s)
	}
	if prix[3].Value != "1" {
		t.Errorf("10 premiers : %+v", prix[3])
	}
	if r := rulesOf(t, cols[0].Rules); len(r) != 1 || r[0].ID != prix[0].ID {
		t.Errorf("Produit : même règle attendue : %+v", r)
	}
	free := map[[2]int]map[string]any{}
	for _, s := range l.Styles {
		free[[2]int{s.Row, s.Col}] = styleOf(t, s.Style)
	}
	cf, _ := json.Marshal(free[[2]int{6, 5}]["cf"])
	var e2 []canvas.Rule
	json.Unmarshal(cf, &e2)
	if len(e2) != 2 || e2[0].Op != "bar" || e2[0].Bar["color"] != "#638ec6" || e2[1].Op != "icons" || e2[1].Icons["set"] != "3arrows" || e2[1].Icons["reverse"] != true {
		t.Errorf("E2 : %s", cf)
	}
	if pts, _ := e2[1].Icons["points"].([]any); len(pts) != 2 || pts[1].(map[string]any)["gt"] != true {
		t.Errorf("seuils des icônes : %v", e2[1].Icons)
	}
	if len(l.ColRules) != 1 || l.ColRules[0].Index != 7 || rulesOf(t, l.ColRules[0].Rules)[0].Value != "urgent" {
		t.Errorf("colonne H : %+v", l.ColRules)
	}
	if dv := styleOf(t, cols[2].Style)["dv"]; strings.Join(strings.Fields(strings.Trim(strings.ReplaceAll(fmtAny(dv), ",", " "), "[]")), " ") != "Oui Non" {
		t.Errorf("liste de Statut : %v", dv)
	}
	if dv, _ := free[[2]int{0, 6}]["dv"].(map[string]any); dv["src"] != "Cell[E7:G7]" {
		t.Errorf("liste de G1 : %v", free[[2]int{0, 6}])
	}
	if n := styleOf(t, cols[0].HeaderStyle)["nt"]; n != "Jean : À vérifier" {
		t.Errorf("note de A1 : %q", n)
	}
	if n := free[[2]int{6, 4}]["nt"]; n != "Ana : Objectif ?\nLuc : Oui, 10." {
		t.Errorf("note de E1 : %q", n)
	}
}

func fmtAny(v any) string {
	b, _ := json.Marshal(v)
	return strings.ReplaceAll(string(b), `"`, "")
}

// Les règles, listes et notes exportées se relisent à l'identique.
func TestXLSXRulesRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := exportFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	red := canvas.Style{"bg": "#ffc7ce", "fg": "#9c0006"}
	must(d.SetRules(ctx, "Ventes", 1, []canvas.Rule{
		{ID: "r1", Op: ">", Value: "1,5", Style: red},
		{ID: "r2", Op: "formula", Value: "=[Prix]>1", Style: canvas.Style{"b": true}, Stop: true},
		{ID: "r3", Op: "scale", Scale: []map[string]any{{"kind": "min", "color": "#f8696b"}, {"kind": "max", "color": "#63be7b"}}},
	}))
	must(d.SetRules(ctx, "Ventes", 0, []canvas.Rule{{ID: "r4", Op: "contains", Value: "Po", Style: red}, {ID: "r5", Op: "dup", Style: red}}))
	must(d.SetLineRules(ctx, 1, false, 9, 9, []canvas.Rule{{ID: "r6", Op: "icons", Icons: map[string]any{"set": "3traffic", "points": []any{map[string]any{"kind": "pct", "value": 33.0}, map[string]any{"kind": "pct", "value": 67.0}}}}}))
	must(d.SetSheetRules(ctx, 1, canvas.RuleChanges{Cells: []canvas.RangeRules{{R0: 20, C0: 0, R1: 20, C1: 2, Rules: []canvas.Rule{
		{ID: "r7", Op: "bar", Bar: map[string]any{"color": "#638ec6", "min": map[string]any{"kind": "min"}, "max": map[string]any{"kind": "max"}}},
		{ID: "r8", Op: "top", Value: "2", Pct: true, Style: red},
		{ID: "r9", Op: "date", Value: "today", Style: red},
		{ID: "r10", Op: "formula", Value: "=A$21>MOYENNE($A$21:$C$21)", Style: red},
	}}}}))
	for c := 0; c <= 2; c++ {
		if _, err := d.SetCell(ctx, 1, 20, c, strconv.Itoa(c+5)); err != nil {
			t.Fatal(err)
		}
	}
	must(d.ApplyStyle(ctx, 1, 1, 3, 2, 3, canvas.Style{"dv": []any{"VRAI", "FAUX"}}))
	must(d.ApplyStyle(ctx, 1, 20, 2, 20, 2, canvas.Style{"dv": map[string]any{"src": "Ventes[Produit]"}}))
	must(d.SetNote(ctx, 1, 0, 0, "Lu : à revoir"))
	must(d.SetNote(ctx, 1, 20, 0, "Note libre"))
	p := filepath.Join(t.TempDir(), "rt.xlsx")
	rep, err := ExportXLSXFile(ctx, d.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	if r := rep.Sheets[0]; r.Rules != 0 || r.Notes != 0 || r.Lists != 0 {
		t.Errorf("compte rendu : %+v", r)
	}
	parts := readZip(t, p)
	if s := parts["xl/worksheets/sheet1.xml"]; !strings.Contains(s, `<conditionalFormatting sqref="B2:B3">`) || !strings.Contains(s, `<legacyDrawing r:id=`) {
		t.Errorf("feuille : %s", s)
	}
	d2 := open(t)
	res, err := ImportXLSX(ctx, d2.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d2, "après réimport")
	for _, w := range res.Warnings {
		if strings.Contains(w, "règle") || strings.Contains(w, "validation") || strings.Contains(w, "commentaire") {
			t.Errorf("avertissement : %s", w)
		}
	}
	l, err := d2.Layout(ctx, res.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	cols := l.Tables[0].Columns
	got := func(js string) string {
		var ops []string
		for _, r := range rulesOf(t, js) {
			v := r.Value
			if r.Op == "formula" {
				v = strings.ReplaceAll(v, " ", "")
			}
			ops = append(ops, r.Op+":"+v)
		}
		return strings.Join(ops, " ")
	}
	if g := got(cols[1].Rules); g != ">:1.5 formula:=$B2>1 scale:" {
		t.Errorf("Prix : %s", g)
	}
	if g := got(cols[0].Rules); g != "contains:Po dup:" {
		t.Errorf("Produit : %s", g)
	}
	if len(l.ColRules) != 1 || got(l.ColRules[0].Rules) != "icons:" {
		t.Errorf("colonne J : %+v", l.ColRules)
	}
	free := map[[2]int]map[string]any{}
	for _, s := range l.Styles {
		free[[2]int{s.Row, s.Col}] = styleOf(t, s.Style)
	}
	cf, _ := json.Marshal(free[[2]int{20, 1}]["cf"])
	if g := got(string(cf)); g != "bar: top:2 date:today formula:=A$21>MOYENNE($A$21:$C$21)" {
		t.Errorf("A22 : %s", g)
	}
	if dv := styleOf(t, cols[3].Style)["dv"]; fmtAny(dv) != "[VRAI,FAUX]" {
		t.Errorf("liste de Payé : %v", dv)
	}
	if dv, _ := free[[2]int{20, 2}]["dv"].(map[string]any); dv["src"] != "Ventes[Produit]" {
		t.Errorf("liste de C21 : %v", free[[2]int{20, 2}])
	}
	if n := styleOf(t, cols[0].HeaderStyle)["nt"]; n != "Lu : à revoir" {
		t.Errorf("note de A1 : %v", n)
	}
	if n := free[[2]int{20, 0}]["nt"]; n != "Note libre" {
		t.Errorf("note de A21 : %v", n)
	}
	_ = formula.FR
}
