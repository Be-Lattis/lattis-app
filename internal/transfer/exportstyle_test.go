package transfer

import (
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// TestExportBorders : styles et couleurs de chaque bord (xlsx, ods) ; true
// reste un trait fin de couleur bc.
func TestExportBorders(t *testing.T) {
	x := toXfmt(canvas.Style{"bt": true, "bb": "double #C00000", "bl": "dashed", "br": "thick", "bc": "#00ff00"})
	if x.bt != "thin 00FF00" || x.bb != "double C00000" || x.bl != "dashed 00FF00" || x.br != "thick 00FF00" {
		t.Fatalf("bords : %+v", x)
	}
	xml := xlsxStyles([]xfmt{{}, x})
	for _, want := range []string{`<top style="thin"><color rgb="FF00FF00"/></top>`, `<bottom style="double"><color rgb="FFC00000"/></bottom>`, `<left style="dashed">`, `<right style="thick">`} {
		if !strings.Contains(xml, want) {
			t.Errorf("styles.xml sans %s", want)
		}
	}
	ods := x.odsCellStyle(1)
	for _, want := range []string{`fo:border-bottom="2.25pt double #c00000"`, `fo:border-left="0.75pt dashed #00ff00"`, `fo:border-right="2.5pt solid #00ff00"`} {
		if !strings.Contains(ods, want) {
			t.Errorf("ods sans %s : %s", want, ods)
		}
	}
	if y := toXfmt(canvas.Style{"bt": false, "bc": "#000000"}); y.bt != "" {
		t.Errorf("bord retiré : %+v", y)
	}
}

// TestExportCustomCode : un code personnalisé part en code anglais du
// fichier .xlsx, et au plus près en .ods.
func TestExportCustomCode(t *testing.T) {
	x := toXfmt(canvas.Style{"nc": `# ##0,00 "kg";[Rouge]-# ##0,00 "kg"`, "nf": "€"})
	if got := x.excelNumFmt(); got != `#,##0.00 "kg";[Red]-#,##0.00 "kg"` {
		t.Errorf("code xlsx : %q", got)
	}
	if !strings.Contains(xlsxStyles([]xfmt{{}, x}), `formatCode="#,##0.00 &quot;kg&quot;;[Red]-#,##0.00 &quot;kg&quot;"`) {
		t.Error("numFmt absent")
	}
	for code, want := range map[string]string{
		`# ##0,00 "kg"`: `number:decimal-places="2"`,
		`0,00E+00`:      `number:scientific-number`,
		`0,0 %`:         `number:percentage-style`,
		`jj/mm/aaaa`:    `number:date-style`,
		`@`:             `number:text-style`,
	} {
		if got := toXfmt(canvas.Style{"nc": code}).odsDataStyle("N1"); !strings.Contains(got, want) {
			t.Errorf("ods %q : %s", code, got)
		}
	}
}
