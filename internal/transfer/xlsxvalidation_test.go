package transfer

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

func dvJSON(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSpace(b.String())
}

// Lecture d'un <dataValidation> : chaque type, constantes (dates et heures
// en numéros de série), formules, messages et alertes.
func TestValidationFromXlsx(t *testing.T) {
	cases := []struct {
		in   XlsxValidation
		want string
	}{
		{XlsxValidation{Type: "whole", Operator: "between", AllowBlank: true, ShowError: true, Formula1: "1", Formula2: "10"},
			`{"max":"10","min":"1","op":"between","type":"whole"}`},
		{XlsxValidation{Type: "decimal", Operator: "greaterThan", ShowError: true, Formula1: "2.5"},
			`{"blank":false,"min":"2,5","op":">","type":"decimal"}`},
		{XlsxValidation{Type: "date", AllowBlank: true, ShowError: true, Formula1: "46023", Formula2: "46387"},
			`{"max":"31/12/2026","min":"01/01/2026","op":"between","type":"date"}`},
		{XlsxValidation{Type: "time", Operator: "lessThan", AllowBlank: true, ShowError: true, Formula1: "0.5"},
			`{"min":"12:00","op":"<","type":"time"}`},
		{XlsxValidation{Type: "textLength", Operator: "lessThanOrEqual", AllowBlank: true, ShowError: true, Formula1: "$B$1", TopLeft: "C3"},
			`{"at":"C3","min":"=$B$1","op":"<=","type":"length"}`},
		{XlsxValidation{Type: "list", AllowBlank: true, ShowError: true, Formula1: `"Oui,Non"`},
			`{"list":["Oui","Non"],"type":"list"}`},
		{XlsxValidation{Type: "list", AllowBlank: true, ShowError: true, Formula1: "$A$1:$A$5"},
			`{"src":"Cell[A1:A5]","type":"list"}`},
		{XlsxValidation{Type: "custom", AllowBlank: true, ShowError: true, Formula1: "ISNUMBER(B2)", TopLeft: "B2", ErrorStyle: "warning", ErrorTitle: "Nombre", Error: "Un nombre"},
			`{"at":"B2","error":{"msg":"Un nombre","style":"warning","title":"Nombre"},"formula":"=ESTNUM(B2)","type":"custom"}`},
		{XlsxValidation{Type: "whole", Operator: "equal", AllowBlank: true, ShowError: true, ShowInput: true, PromptTitle: "Aide", Prompt: "Cinq", Formula1: "5", ErrorStyle: "information"},
			`{"error":{"msg":"","style":"info","title":""},"input":{"msg":"Cinq","title":"Aide"},"min":"5","op":"=","type":"whole"}`},
		// Sans alerte : seul le message de saisie reste.
		{XlsxValidation{Type: "whole", Formula1: "1", ShowInput: true, Prompt: "Info"},
			`{"input":{"msg":"Info","title":""},"type":"any"}`},
	}
	for _, c := range cases {
		dv, ok := validationFromXlsx(c.in)
		if !ok || dvJSON(dv) != c.want {
			t.Errorf("%+v :\n %s\nattendu\n %s", c.in, dvJSON(dv), c.want)
		}
	}
	if _, ok := validationFromXlsx(XlsxValidation{Type: "whole", Formula1: "1"}); ok {
		t.Error("ni alerte ni message : rien à garder")
	}
}

// Écriture : les mêmes validations, dans l'autre sens ; une formule écrite
// pour une autre cellule est décalée sur la première cellule de sqref.
func TestValidationToXlsx(t *testing.T) {
	cases := []struct {
		dv   any
		want string
	}{
		{map[string]any{"type": "whole", "op": "between", "min": "1", "max": "10"},
			`<dataValidation type="whole" allowBlank="1" showErrorMessage="1" sqref="B2"><formula1>1</formula1><formula2>10</formula2></dataValidation>`},
		{map[string]any{"type": "decimal", "op": ">", "min": "2,5", "blank": false, "error": map[string]any{"style": "warning", "msg": "Trop <petit>"}},
			`<dataValidation type="decimal" operator="greaterThan" errorStyle="warning" showErrorMessage="1" errorTitle="" error="Trop &lt;petit&gt;" sqref="B2"><formula1>2.5</formula1></dataValidation>`},
		{map[string]any{"type": "date", "op": "between", "min": "01/01/2026", "max": "31/12/2026"},
			`<formula1>46023</formula1><formula2>46387</formula2>`},
		{map[string]any{"type": "time", "op": "<", "min": "12:00"}, `<formula1>0.5</formula1>`},
		{[]any{"Oui", "Non"}, `type="list" allowBlank="1" showErrorMessage="1" sqref="B2"><formula1>&quot;Oui,Non&quot;</formula1>`},
		{map[string]any{"src": "Feuille2!Cell[A1:A5]"}, `<formula1>'Feuille2'!$A$1:$A$5</formula1>`},
		{map[string]any{"type": "custom", "formula": "=ESTNUM(A1)", "at": "A1"}, `type="custom"`},
		{map[string]any{"type": "custom", "formula": "=ESTNUM(A1)", "at": "A1"}, `<formula1>ISNUMBER(B2)</formula1>`},
		{map[string]any{"type": "any", "input": map[string]any{"title": "Aide", "msg": "Un code"}},
			`<dataValidation type="none" allowBlank="1" showInputMessage="1" promptTitle="Aide" prompt="Un code" sqref="B2"></dataValidation>`},
	}
	for _, c := range cases {
		got, ok := validationToXlsx(c.dv, "B2", 1, 1)
		if !ok || !strings.Contains(got, c.want) {
			t.Errorf("%v :\n %s\nattendu\n %s", c.dv, got, c.want)
		}
	}
	// Colonne de tableau : pas de liste tirée d'une colonne dans un fichier.
	if _, ok := validationToXlsx(map[string]any{"src": "Clients[Ville]"}, "B2", 1, 1); ok {
		t.Error("liste tirée d'un tableau écrite")
	}
	// Aller-retour.
	for _, dv := range []map[string]any{
		{"type": "whole", "op": "notbetween", "min": "1", "max": "10"},
		{"type": "length", "op": "<=", "min": "5", "input": map[string]any{"title": "", "msg": "Court"}},
	} {
		x, _ := validationToXlsx(dv, "A1", 0, 0)
		back := XlsxValidation{Type: attr(x, "type"), Operator: attr(x, "operator"), AllowBlank: attr(x, "allowBlank") == "1",
			ShowInput: attr(x, "showInputMessage") == "1", ShowError: attr(x, "showErrorMessage") == "1", Prompt: attr(x, "prompt"),
			Formula1: between(x, "<formula1>", "</formula1>"), Formula2: between(x, "<formula2>", "</formula2>")}
		got, _ := validationFromXlsx(back)
		if r1, _ := canvas.ParseDV(got); r1.Type != dv["type"] || r1.Min != dv["min"] {
			t.Errorf("aller-retour %v : %v (%s)", dv, got, x)
		}
	}
}

func attr(x, name string) string { return between(x, " "+name+`="`, `"`) }

func between(s, a, b string) string {
	i := strings.Index(s, a)
	if i < 0 {
		return ""
	}
	s = s[i+len(a):]
	if j := strings.Index(s, b); j >= 0 {
		return s[:j]
	}
	return ""
}

// L'export .xlsx écrit les validations des cellules libres et des colonnes
// de tableau ([@Colonne] devient l'adresse de la première ligne).
func TestXLSXExportValidations(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Début", "Fin"}, {"01/01/2026", "10/01/2026"}, {"05/01/2026", "06/01/2026"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 1, 1, 2, 1, canvas.Style{"dv": map[string]any{"type": "date", "op": ">=", "min": "=A2", "at": "B2"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 5, 4, 6, 4, canvas.Style{"dv": map[string]any{"type": "whole", "op": "between", "min": "1", "max": "9", "input": map[string]any{"msg": "1 à 9"}}}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "v.xlsx")
	rep, err := ExportXLSXFile(ctx, d.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	parts := readZip(t, p)
	contains(t, "feuille", parts["xl/worksheets/sheet1.xml"],
		`<dataValidations count="2">`,
		`type="date" operator="greaterThanOrEqual" allowBlank="1" showErrorMessage="1" sqref="B2:B3"><formula1>A2</formula1>`,
		`type="whole" allowBlank="1" showInputMessage="1" promptTitle="" prompt="1 à 9" showErrorMessage="1" sqref="E6 E7"><formula1>1</formula1><formula2>9</formula2>`)
	if n := rep.Sheets[0].Lists; n != 0 {
		t.Errorf("validations non exportées : %d", n)
	}
}

// Formule à plusieurs arguments : séparateur « , » sans espace, adresses
// absolues gardées.
func TestValidationToXlsxSeparators(t *testing.T) {
	got, ok := validationToXlsx(map[string]any{"type": "custom", "formula": "=NB.SI($A$2:$A$9;A2)=1", "at": "A2"}, "A2:A9", 1, 0)
	if !ok || !strings.Contains(got, "<formula1>COUNTIF($A$2:$A$9,A2)=1</formula1>") {
		t.Fatalf("%s", got)
	}
}
