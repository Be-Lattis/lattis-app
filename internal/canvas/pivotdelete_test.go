package canvas

import (
	"context"
	"testing"
)

// Suppr sur un tableau croisé : pris en entier, il est supprimé ; en partie,
// il reste ; en erreur (source supprimée avec sa feuille), sa cellule suffit.
// Chaque fois en une étape d'annulation.
func TestClearRangeDeletesPivot(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}, {"FR", "7"}}); err != nil {
		t.Fatal(err)
	}
	sheet, err := d.CreatePivot(ctx, PivotConfig{Source: "Feuille1", RowFields: []string{"Pays"}, Values: []PivotValue{{Field: "Ventes", Agg: "sum"}},
		Filters: []PivotFilter{{Field: "Pays"}}})
	must(t, err)
	pivots := func() []PivotInfo {
		t.Helper()
		l, err := d.Layout(ctx, sheet)
		must(t, err)
		return l.Pivots
	}
	p := pivots()[0]
	r := pivotRect(p)
	// Filtre de rapport (2 lignes) + en-tête, FR, IT, total.
	if p.Error != "" || r != (Rect{1, 1, 6, 2}) {
		t.Fatalf("TCD : %+v %+v", p, r)
	}
	// Une partie : rien ne change.
	must(t, d.ClearRange(ctx, sheet, 1, 1, 6, 1, false))
	if len(pivots()) != 1 {
		t.Fatal("une partie du TCD l'a supprimé")
	}
	before := dump(t, d)
	must(t, d.ClearRange(ctx, sheet, 0, 0, 10, 5, false))
	if len(pivots()) != 0 {
		t.Fatal("TCD pris en entier non supprimé")
	}
	undoOnce(t, d, before, "suppression du TCD entier")

	// La feuille de la source supprimée : le TCD est en erreur.
	must(t, d.DeleteSheet(ctx, 1))
	p = pivots()[0]
	if p.Error == "" || pivotRect(p) != (Rect{1, 1, 1, 1}) {
		t.Fatalf("TCD en erreur attendu : %+v", p)
	}
	before = dump(t, d)
	must(t, d.ClearRange(ctx, sheet, 1, 1, 1, 1, false))
	if len(pivots()) != 0 {
		t.Fatal("TCD en erreur non supprimé")
	}
	undoOnce(t, d, before, "suppression du TCD en erreur")
	// Et la commande du volet ou du clic droit.
	must(t, d.DeletePivot(ctx, p.ID))
	if len(pivots()) != 0 {
		t.Fatal("DeletePivot sur un TCD en erreur")
	}
	undoOnce(t, d, before, "DeletePivot du TCD en erreur")
}
