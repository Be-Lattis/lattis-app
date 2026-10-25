package transfer

import (
	"context"
	"path/filepath"
	"testing"
)

// Plan exporté en .xlsx : niveaux, groupe réduit masqué, ligne de synthèse
// marquée ; l'impression saute les lignes d'un groupe réduit.
func TestXLSXOutline(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	for r, v := range []string{"Titre", "a", "b", "c", "Total"} {
		if _, err := d.SetCell(ctx, 1, r, 5, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.GroupLines(ctx, 1, true, 1, 3); err != nil {
		t.Fatal(err)
	}
	if err := d.FoldGroup(ctx, 1, true, 1, 2, true); err != nil {
		t.Fatal(err)
	}
	if err := d.GroupLines(ctx, 1, false, 2, 3); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "plan.xlsx")
	if _, err := ExportXLSXFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	s1 := readZip(t, p)["xl/worksheets/sheet1.xml"]
	contains(t, "plan", s1,
		`<sheetFormatPr defaultRowHeight="15" outlineLevelRow="1" outlineLevelCol="1"/>`,
		`<row r="2" hidden="1" outlineLevel="1">`,
		`<row r="4" hidden="1" outlineLevel="1">`,
		`<row r="5" collapsed="1">`,
		`<col min="3" max="3" width="14.29" outlineLevel="1" customWidth="1"/>`)
	plan, err := SheetPrintPlan(ctx, d.Model, 1, PrintSizes{}, PrintInfo{})
	if err != nil {
		t.Fatal(err)
	}
	zero := map[int]bool{}
	for _, h := range plan.Heights {
		if h[1] == 0 {
			zero[h[0]] = true
		}
	}
	if !zero[1] || !zero[2] || !zero[3] || zero[4] {
		t.Fatalf("hauteurs imprimées : %v", plan.Heights)
	}
}
