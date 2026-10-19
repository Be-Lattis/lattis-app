package transfer

import (
	"context"
	"testing"
)

// Un import dans un classeur jamais servi ne laisse pas Feuille1 vide en
// tête ; dans un classeur utilisé, les feuilles restent.
func TestImportDropsBlankSheet(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	names := func() []string {
		t.Helper()
		l, err := d.Layout(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, s := range l.Sheets {
			out = append(out, s.Name)
		}
		return out
	}
	if _, err := ImportCSV(ctx, d.Model, write(t, "Ventes.csv", []byte("a;b\n1;2\n")), CSVOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := names(); len(got) != 1 || got[0] != "Ventes" {
		t.Fatalf("feuilles après import : %v", got)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := names(); len(got) != 1 || got[0] != "Feuille1" {
		t.Fatalf("après annulation : %v", got)
	}
	if _, err := d.SetCell(ctx, 1, 0, 0, "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(ctx, d.Model, write(t, "Achats.csv", []byte("a;b\n1;2\n")), CSVOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := names(); len(got) != 2 || got[0] != "Feuille1" {
		t.Fatalf("classeur utilisé : %v", got)
	}
	check(t, d, "import")
}
