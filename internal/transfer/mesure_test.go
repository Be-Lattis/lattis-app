package transfer

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMesureImport importe un vrai fichier et affiche ce qui a été créé :
// LATTIS_IMPORT=chemin.xlsx (ou .csv) go test -run TestMesureImport -v
func TestMesureImport(t *testing.T) {
	path := os.Getenv("LATTIS_IMPORT")
	if path == "" {
		t.Skip("LATTIS_IMPORT non défini")
	}
	ctx := context.Background()
	d := open(t)
	start := time.Now()
	res, err := ImportFile(ctx, d.Model, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("import en %v : tableaux %v", time.Since(start), res.Tables)
	for _, w := range res.Warnings {
		t.Log("avertissement : " + w)
	}
	for _, id := range res.Sheets {
		l, err := d.Layout(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, tb := range l.Tables {
			t.Logf("%s (%d lignes, en %d,%d)", tb.Name, tb.Rows, tb.Row, tb.Col)
			for _, c := range tb.Columns {
				t.Logf("  %s : %s %s", c.Name, c.Type, c.Formula)
			}
			lines := strings.SplitN(exportCSV(t, d, tb.Name, ExcelFR), "\r\n", 4)
			t.Logf("  %s", strings.Join(lines[:min(3, len(lines))], " | "))
		}
		for _, c := range l.Cells {
			t.Logf("cellule %s : %s → %s", cellName(c.Row, c.Col), c.Raw, c.Value)
		}
	}
	check(t, d, "import")
}
