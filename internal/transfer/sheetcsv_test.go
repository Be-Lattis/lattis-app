package transfer

import (
	"bytes"
	"context"
	"testing"
)

func TestExportSheetCSV(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 1, 1, [][]string{{"Produit", "Prix"}, {"Pomme; co", "1,5"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 4, 3, "=1/0"); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := ExportSheetCSV(ctx, d.Model, 1, &b); err != nil {
		t.Fatal(err)
	}
	want := "Feff;;;\r\n;Produit;Prix;\r\n;\"Pomme; co\";1,5;\r\n;;;\r\n;;;#DIV/0!\r\n"
	if b.String() != want {
		t.Fatalf("CSV :\n%q\nattendu :\n%q", b.String(), want)
	}
	if err := ExportSheetCSV(ctx, d.Model, 99, &b); err == nil {
		t.Error("feuille inconnue acceptée")
	}
}
