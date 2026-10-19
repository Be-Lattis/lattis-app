package transfer

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNeutralize(t *testing.T) {
	for in, want := range map[string]string{
		"=HYPERLINK(\"http://x\")": "'=HYPERLINK(\"http://x\")",
		"+33 6 12":                 "'+33 6 12",
		"-2+3":                     "'-2+3",
		"@SOMME(A1)":               "'@SOMME(A1)",
		"\t=1":                     "'\t=1",
		"Dupont":                   "Dupont",
		"":                         "",
		"a=b":                      "a=b",
	} {
		if got := Neutralize(in); got != want {
			t.Errorf("%q : %q, attendu %q", in, got, want)
		}
	}
}

// Export CSV pour Excel : un texte qui commence comme une formule est
// préfixé d'une apostrophe ; les nombres négatifs et le CSV standard
// restent tels quels (audit 6, Id2).
func TestCSVExportNeutralizesFormulas(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	src := "Nom;Note;Montant\r\n" +
		"Durand;=CMD|' /C calc'!A0;-5\r\n" +
		"Martin;@SOMME(A1);12\r\n"
	if _, err := ImportCSV(ctx, d.Model, write(t, "Clients.csv", []byte(src)), CSVOptions{}); err != nil {
		t.Fatal(err)
	}
	out := exportCSV(t, d, "Clients", ExcelFR)
	for _, want := range []string{"Durand;'=CMD|' /C calc'!A0;-5\r\n", "Martin;'@SOMME(A1);12\r\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("export ExcelFR : %q absent de\n%q", want, out)
		}
	}
	if std := exportCSV(t, d, "Clients", Standard); !strings.Contains(std, "Martin,@SOMME(A1),12") {
		t.Errorf("export standard modifié :\n%q", std)
	}

	// Feuille entière.
	var b bytes.Buffer
	l := layoutOf(t, d)
	if err := ExportSheetCSV(ctx, d.Model, l.Sheets[len(l.Sheets)-1].ID, &b); err != nil {
		t.Fatal(err)
	}
	if s := b.String(); !strings.Contains(s, "Martin;'@SOMME(A1);12") || !strings.Contains(s, ";-5\r\n") {
		t.Errorf("export de la feuille :\n%q", s)
	}
}
