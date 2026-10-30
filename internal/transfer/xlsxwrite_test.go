package transfer

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// exportFixture : deux feuilles, un tableau avec clé et colonne calculée,
// des formules (traduisibles ou non), une cellule nommée, de la mise en
// forme, une fusion et des volets figés.
func exportFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	set := func(sheet int64, r, c int, raw string) {
		t.Helper()
		_, err := d.SetCell(ctx, sheet, r, c, raw)
		must(err)
	}
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix", "Date", "Payé"}, {"Pomme", "1,5", "15/01/2026", "VRAI"}, {"Poire <b>", "2", "16/01/2026", "FAUX"}})
	must(err)
	must(d.RenameSheet(ctx, 1, "Données"))
	must(d.RenameTable(ctx, "Feuille1", "Ventes"))
	must(d.SetKey(ctx, "Ventes", 0))
	set(1, 0, 4, "Total")
	set(1, 1, 4, "=[Prix]*2")
	set(1, 0, 7, "0,2")
	must(d.SetName(ctx, 1, 0, 7, "Taux"))
	set(1, 6, 0, "Somme")
	set(1, 6, 1, "=SOMME(Ventes[Prix])")
	set(1, 7, 1, "=1/0")
	set(1, 8, 1, `=REFERENCE(Ventes[Prix]; "Poire <b>")`)
	set(1, 9, 1, "=Taux*Cell[B7]")
	set(1, 10, 1, `=TEXTE(Cell[B7]; "0,00")`) // TEXTE : format écrit tel quel en .xlsx, valeur en .ods
	must(d.ApplyStyle(ctx, 1, 6, 1, 6, 1, canvas.Style{"b": true, "nf": "€", "bg": "#FFFF00", "al": "right"}))
	must(d.ApplyStyle(ctx, 1, 1, 1, 1, 1, canvas.Style{"i": true, "fg": "#C00000"}))
	set(1, 12, 0, "Fusion")
	_, err = d.MergeCells(ctx, 1, 12, 0, 12, 2)
	must(err)
	must(d.SetColumnWidth(ctx, 1, 0, 140))
	must(d.SetFreeze(ctx, 1, 1, 0))
	id, err := d.AddSheet(ctx)
	must(err)
	must(d.RenameSheet(ctx, id, "Notes"))
	set(id, 0, 0, "=Données!Cell[B7]*2")
	set(id, 1, 0, "=SOMME(Ventes[Total])")
	set(id, 2, 27, "à voir")
	return d
}

// readZip lit les parties d'un fichier zip et vérifie qu'elles sont du XML
// bien formé.
func readZip(t *testing.T, p string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(p)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := map[string]string{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = string(b)
		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".rels") {
			dec := xml.NewDecoder(strings.NewReader(string(b)))
			for {
				if _, err := dec.Token(); err != nil {
					if !errors.Is(err, io.EOF) {
						t.Errorf("%s : XML invalide : %v", f.Name, err)
					}
					break
				}
			}
		}
	}
	return out
}

func contains(t *testing.T, what, s string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Errorf("%s : %s attendu dans\n%s", what, w, s)
		}
	}
}

func TestXLSXExport(t *testing.T) {
	ctx := context.Background()
	d := exportFixture(t)
	// Une feuille masquée, et une cellule au-delà des limites d'Excel : non
	// exportée, et comptée.
	id, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RenameSheet(ctx, id, "Brouillon: [2026]"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSheetHidden(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, id, ExcelRows+5, 0, "trop bas"); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "export.xlsx")
	rep, err := ExportXLSXFile(ctx, d.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Clipped != 1 || rep.Sheets[2].Clipped != 1 {
		t.Fatalf("cellules coupées : %+v", rep)
	}
	parts := readZip(t, p)
	contains(t, "classeur", parts["xl/workbook.xml"], `name="Données"`, `name="Notes"`, `name="Brouillon_ _2026_" sheetId="3" state="hidden"`, `fullCalcOnLoad="1"`,
		`<definedName name="Taux">'Données'!$H$1</definedName>`)
	contains(t, "types", parts["[Content_Types].xml"], `/xl/tables/table1.xml`)
	contains(t, "liens de la feuille 1", parts["xl/worksheets/_rels/sheet1.xml.rels"], `Target="../tables/table1.xml"`)

	// Le tableau Excel : nom, plage, filtre, colonne calculée.
	contains(t, "tableau", parts["xl/tables/table1.xml"],
		`name="Ventes" displayName="Ventes" ref="A1:E3"`, `<autoFilter ref="A1:E3"/>`,
		`<tableColumn id="1" name="Produit"/>`, `<tableColumn id="4" name="Payé"/>`,
		`<tableColumn id="5" name="Total"><calculatedColumnFormula>Ventes[[#This Row],[Prix]]*2</calculatedColumnFormula></tableColumn>`)

	s1 := parts["xl/worksheets/sheet1.xml"]
	contains(t, "feuille 1", s1,
		`<c r="B2"`, `<v>1.5</v>`, `t="inlineStr"><is><t xml:space="preserve">Produit</t>`,
		`<c r="D3" t="b"><v>0</v></c>`,
		`<f>Ventes[[#This Row],[Prix]]*2</f><v>3</v>`,
		`Poire &lt;b&gt;`,
		`<f>SUM(Ventes[Prix])</f><v>3.5</v>`,
		`<c r="B8" t="e"><f>1/0</f><v>#DIV/0!</v></c>`,
		`<f>INDEX(Ventes[Prix],MATCH(&quot;Poire &lt;b&gt;&quot;,Ventes[Produit],0))</f><v>2</v>`,
		`<f>Taux*B7</f><v>0.7000000000000001</v>`,
		`<c r="B11"><f>TEXT(B7,&quot;0,00&quot;)</f>`,
		`<col min="1" max="1" width="20" customWidth="1"/>`,
		`<pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/>`,
		`<mergeCell ref="A13:C13"/>`,
		`<tableParts count="1"><tablePart r:id="rId1"/></tableParts>`)
	s2 := parts["xl/worksheets/sheet2.xml"]
	contains(t, "feuille 2", s2, `<f>'Données'!B7*2</f><v>7</v>`, `<f>SUM(Ventes[Total])</f><v>7</v>`,
		`<c r="AB3" t="inlineStr"><is><t xml:space="preserve">à voir</t></is></c>`)

	// Mise en forme : le style de B7 (gras, euros, fond jaune, à droite).
	st := parts["xl/styles.xml"]
	contains(t, "styles", st, `formatCode="#,##0.00\ &quot;€&quot;"`, `<fgColor rgb="FFFFFF00"/>`, `<color rgb="FFC00000"/>`, `<xf numFmtId="14"`, `horizontal="right"`) // date courte : format intégré 14
	if !strings.Contains(s1, `<c r="B7" s="`) || !strings.Contains(s1, `<c r="B2" s="`) || !strings.Contains(s1, `<c r="C2" s="`) {
		t.Errorf("cellules mises en forme sans style :\n%s", s1)
	}

	// Le fichier se relit (lecteur .xlsx de Lattis) : le tableau revient
	// avec ses types.
	sheets, closeFn, err := ReadXLSX(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if len(sheets) != 3 || len(sheets[0].Tables) == 0 {
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

func TestODSExport(t *testing.T) {
	ctx := context.Background()
	d := exportFixture(t)
	p := filepath.Join(t.TempDir(), "export.ods")
	if _, err := ExportODSFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(p)
	if err != nil {
		t.Fatal(err)
	}
	if f := zr.File[0]; f.Name != "mimetype" || f.Method != zip.Store {
		t.Errorf("première entrée : %s (méthode %d)", f.Name, f.Method)
	}
	zr.Close()
	parts := readZip(t, p)
	if parts["mimetype"] != "application/vnd.oasis.opendocument.spreadsheet" {
		t.Errorf("mimetype : %q", parts["mimetype"])
	}
	contains(t, "manifeste", parts["META-INF/manifest.xml"], `manifest:full-path="content.xml"`, `manifest:full-path="styles.xml"`)
	c := parts["content.xml"]
	contains(t, "contenu", c,
		`<table:table table:name="Données" table:style-name="ta1">`, `<table:table table:name="Notes"`,
		`table:formula="of:=[.B2]*2" office:value-type="float" office:value="3"`,
		`table:formula="of:=[.B3]*2"`,
		`table:formula="of:=SUM([.$B$2:.$B$3])" office:value-type="float" office:value="3.5"`,
		`table:formula="of:=1/0" office:value-type="string" office:string-value="#DIV/0!"`,
		`table:formula="of:=INDEX([.$B$2:.$B$3];MATCH(&quot;Poire &lt;b&gt;&quot;;[.$A$2:.$A$3];0))"`,
		`table:formula="of:=[.$H$1]*[.B7]"`,
		`table:formula="of:=[$'Données'.B7]*2"`,
		`table:formula="of:=SUM([$'Données'.$E$2:.$E$3])"`,
		`office:value-type="date" office:date-value="2026-01-15T00:00:00"`,
		`office:value-type="boolean" office:boolean-value="true"`,
		`table:number-rows-spanned="1" table:number-columns-spanned="3"`, `<table:covered-table-cell/>`,
		`<table:named-range table:name="Ventes" table:base-cell-address="$'Données'.$A$2" table:cell-range-address="$'Données'.$A$2:.$E$3"/>`,
		`<text:p>à voir</text:p>`)
	if strings.Contains(c, `of:=TEXT`) {
		t.Error("TEXTE ne doit pas être exportée en formule")
	}
	// En-têtes du tableau en gras ; style de B7 et format de date.
	st := parts["styles.xml"]
	contains(t, "styles", st, `fo:font-weight="bold"`, `fo:background-color="#ffff00"`, `<number:currency-symbol`, `<number:date-style`, `fo:color="#c00000"`)
}

func TestTranslateFormulas(t *testing.T) {
	ventes := &tablePlace{name: "Ventes", sheet: "Données", row: 0, col: 0, cols: []string{"Produit", "Prix HT", "Qté"}, key: 0, total: 3}
	sans := &tablePlace{name: "Liste", sheet: "Données", row: 10, col: 0, cols: []string{"Valeur"}, key: -1, total: 2}
	for _, tg := range []target{toXLSX, toODS} {
		b := &book{target: tg, sheets: map[string]string{"données": "Données", "notes": "Notes"},
			tables: map[string]*tablePlace{"ventes": ventes, "liste": sans}, names: map[string]cellPlace{}}
		for _, c := range []struct {
			src       string
			at        at
			xlsx, ods string // "" : non traduite
		}{
			{`=SI([Qté]>2; "beaucoup"; "peu")`, at{"Données", ventes}, `IF(Ventes[[#This Row],[Qté]]>2,"beaucoup","peu")`, `IF([.C` + rowMark + `]>2;"beaucoup";"peu")`},
			{`=SOMME(Ventes[Prix HT])`, at{"Notes", nil}, `SUM(Ventes[Prix HT])`, `SUM([$'Données'.$B$2:.$B$4])`},
			{`=REFERENCE(Liste[Valeur]; 2)`, at{"Notes", nil}, `INDEX(Liste[Valeur],2)`, `INDEX([$'Données'.$A$12:.$A$13];2)`},
			{`=REFERENCE.APPROCHEE(Ventes[Prix HT]; "b")`, at{"Notes", nil}, `_xlfn.XLOOKUP("b",Ventes[Produit],Ventes[Prix HT],NA(),-1)`, ""},
			{`=SI.CONDITIONS(VRAI; 1)`, at{"Notes", nil}, `_xlfn.IFS(TRUE,1)`, `COM.MICROSOFT.IFS(TRUE();1)`},
			{`=RECHERCHEX(1; Ventes[Qté]; Ventes[Produit])`, at{"Notes", nil}, `_xlfn.XLOOKUP(1,Ventes[Qté],Ventes[Produit])`, ""},
			{`=Ventes[Prix HT][2]`, at{"Notes", nil}, `'Données'!$B$3`, `INDEX([$'Données'.$B$2:.$B$4];2)`},
			{`=PRECEDENT([Qté])`, at{"Données", ventes}, "", ""},
			{`=[Qté][-1]`, at{"Données", ventes}, "", ""},
			{`=[Qté][2]`, at{"Données", ventes}, `$C$3`, `INDEX([.$C$2:.$C$4];2)`},
			{`=RANG([Qté])`, at{"Données", ventes}, `RANK(Ventes[[#This Row],[Qté]],Ventes[Qté])`, `RANK([.C` + rowMark + `];[.$C$2:.$C$4])`},
			{`=SOMME(Cell[A1:B2]) & "x,y"`, at{"Notes", nil}, `SUM(A1:B2)&"x,y"`, `SUM([.A1:.B2])&"x,y"`},
			{`=Notes!Cell[C3]`, at{"Données", nil}, `'Notes'!C3`, `[$'Notes'.C3]`},
			{`=COMMUNE("a"; "b")`, at{"Notes", nil}, "", ""},
		} {
			want := c.xlsx
			if tg == toODS {
				want = c.ods
			}
			got, ok := b.translate(c.src, c.at)
			if want == "" {
				if ok {
					t.Errorf("%d %s : traduite en %q", tg, c.src, got)
				}
				continue
			}
			if !ok || got != want {
				t.Errorf("%d %s : %q (%v), attendu %q", tg, c.src, got, ok, want)
			}
		}
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
	used := map[string]bool{}
	for _, c := range [][2]string{{"Ventes", "Ventes"}, {"ventes", "ventes_2"}, {"A1", "_A1"}, {"R2C3", "_R2C3"}, {"2026", "_2026"}, {"Mes ventes", "Mes_ventes"}} {
		if got := uniqueName(excelName(c[0]), used); got != c[1] {
			t.Errorf("nom de tableau %q : %q, attendu %q", c[0], got, c[1])
		}
	}
}
