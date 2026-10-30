package transfer

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// Résultats numériques des formules : écrits comme des nombres à pleine
// précision (point décimal) en .xlsx et .ods, quel que soit l'affichage ;
// le CSV garde l'affichage de la grille. Un texte qui ressemble à un code
// reste du texte.
func TestExportNumberPrecision(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	for _, c := range []struct {
		r, c int
		raw  string
	}{
		{0, 0, "=1/3"},
		{1, 0, "=ATAN2(1; 1)"},
		{2, 0, `="0123"`},
		{3, 0, `="12345678901234567"`},
		{0, 3, "=SEQUENCE(2; 1; 0,1; 0,2)"},
	} {
		if _, err := d.SetCell(ctx, 1, c.r, c.c, c.raw); err != nil {
			t.Fatal(c.raw, err)
		}
	}
	p := filepath.Join(t.TempDir(), "n.xlsx")
	if _, err := ExportXLSXFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	sheet := readZip(t, p)["xl/worksheets/sheet1.xml"]
	contains(t, "xlsx", sheet,
		`<c r="A1"><f>1/3</f><v>0.3333333333333333</v></c>`,
		`<f>ATAN2(1,1)</f><v>0.7853981633974483</v>`,
		`<c r="D1"><v>0.1</v></c>`, `<c r="D2"><v>0.30000000000000004</v></c>`,
		`<c r="A3" t="str"><f>&quot;0123&quot;</f><v>0123</v></c>`,
		`<c r="A4" t="str"><f>&quot;12345678901234567&quot;</f><v>12345678901234567</v></c>`)
	if strings.Contains(sheet, `t="str"><f>ATAN2`) || strings.Contains(sheet, `t="str"><f>1/3`) {
		t.Errorf("résultat numérique écrit en texte :\n%s", sheet)
	}

	o := filepath.Join(t.TempDir(), "n.ods")
	if _, err := ExportODSFile(ctx, d.Model, o); err != nil {
		t.Fatal(err)
	}
	contains(t, "ods", readZip(t, o)["content.xml"],
		`office:value-type="float" office:value="0.3333333333333333"`,
		`office:value-type="float" office:value="0.7853981633974483"`,
		`office:value-type="float" office:value="0.30000000000000004"`)

	var csv bytes.Buffer
	if err := ExportSheetCSV(ctx, d.Model, 1, &csv); err != nil {
		t.Fatal(err)
	}
	contains(t, "csv", csv.String(), "0,333333333333333", "0,785398163397448", "0,3\r\n")
}

// textCell : un nombre écrit comme la grille l'affiche reste un nombre, même
// avec plus de 15 chiffres ; les codes restent du texte.
func TestTextCellNumbers(t *testing.T) {
	for s, want := range map[string]byte{
		"0,785398163397448": 'n', "-0,333333333333333": 'n', "1234,5": 'n', "1 234,5": 'n',
		"01234": 's', "1234567890123456": 's', "00,5": 's',
	} {
		if got := textCell(s).kind; got != want {
			t.Errorf("%q : %c, attendu %c", s, got, want)
		}
	}
}
