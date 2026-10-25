package transfer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Graphiques sparkline exportés dans l'extension x14 : un par ligne d'un
// tableau, un par cellule d'un groupe de cellules libres.
func TestXLSXSparklines(t *testing.T) {
	ctx := context.Background()
	d := exportFixture(t)
	if _, err := d.CreateSparkline(ctx, 1, canvas.SparklineConfig{Kind: "column", Table: "Ventes", Cols: []string{"Prix", "Date"}, High: true, Same: true}); err != nil {
		t.Fatal(err)
	}
	for c, v := range []string{"1", "5", "3"} {
		if _, err := d.SetCell(ctx, 1, 20, c, v); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.CreateSparkline(ctx, 1, canvas.SparklineConfig{Kind: "line", Source: "A21:C21", Location: "D21", Last: true, Weight: 2}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "spark.xlsx")
	if _, err := ExportXLSXFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	s1 := readZip(t, p)["xl/worksheets/sheet1.xml"]
	contains(t, "sparklines", s1,
		`<x14:sparklineGroup type="column" lineWeight="1.125" displayEmptyCellsAs="gap" high="1" minAxisType="group" maxAxisType="group">`,
		`<xm:f>Données!B2:C2</xm:f><xm:sqref>F2</xm:sqref>`,
		`<xm:f>Données!B3:C3</xm:f><xm:sqref>F3</xm:sqref>`,
		`<x14:sparklineGroup lineWeight="1.5" displayEmptyCellsAs="gap" last="1">`,
		`<xm:f>Données!A21:C21</xm:f><xm:sqref>D21</xm:sqref>`)
	if i, j := strings.Index(s1, "</tableParts>"), strings.Index(s1, "<extLst>"); i < 0 || j < i {
		t.Fatal("extLst pas en dernier")
	}
}
