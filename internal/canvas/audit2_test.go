package canvas

import (
	"context"
	"testing"
)

// Audit 2 I2 : coller à une ligne d'écart sous un tableau suit le chemin de
// la zone vide (îlots, en-tête, total à part).
func TestPasteOneRowBelowTable(t *testing.T) {
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	block := [][]string{{"Produit", "Prix"}, {"A", "1"}, {"B", "2"}, {"Total", "3"}}
	if _, err := d.Paste(context.Background(), 1, 5, 0, block); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×3 Tableau1@A6:Produit|Prix×2 A9*=Total B9*=3"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}
