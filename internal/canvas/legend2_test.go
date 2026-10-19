package canvas

import (
	"context"
	"errors"
	"testing"
)

// styleAt donne la mise en forme d'une cellule libre de la feuille 1.
func styleAt(t *testing.T, d *fixture, row, col int) Style {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range l.Styles {
		if s.Row == row && s.Col == col {
			return parseStyle(s.Style)
		}
	}
	return Style{}
}

// Audit 2 I1 : une valeur tapée à droite d'un « Total » gardé à part est la
// valeur de ce total, pas une ligne de données ; au collage cellule par
// cellule aussi.
func TestLegendTotalValue(t *testing.T) {
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	mustSet(t, d, 1, 4, 1, "Total")
	mustSet(t, d, 1, 4, 2, "245")
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×3 B5*=Total C5*=245"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if s := styleAt(t, d, 4, 2); s["b"] != true {
		t.Fatalf("245 n'est pas en total : %v", s)
	}

	d = open(t)
	typeIn(t, d, 0, 0, ventesRows)
	if _, err := d.Paste(context.Background(), 1, 4, 1, [][]string{{"Total", "245"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×3 B5*=Total C5*=245"; got != want {
		t.Fatalf("collage :\n got %s\nwant %s", got, want)
	}
}

// Audit 2 M4 : « Total » ou une formule tapés sous une note sont un total.
func TestLegendTotalUnderNote(t *testing.T) {
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	mustSet(t, d, 1, 4, 0, "Source : INSEE")
	mustSet(t, d, 1, 5, 0, "Total")
	if s := styleAt(t, d, 5, 0); s[outKey] != true || s["b"] != true || s["i"] == true {
		t.Fatalf("« Total » sous la note : %v (%s)", s, layoutOf(t, d))
	}
	mustSet(t, d, 1, 5, 1, "=SOMME(Feuille1[Montant])")
	mustSet(t, d, 1, 6, 1, "=MOYENNE(Feuille1[Montant])")
	if s := styleAt(t, d, 6, 1); s[outKey] == true {
		t.Fatalf("une formule sous un total n'est pas une note : %v", s)
	}
	mustSet(t, d, 1, 5, 2, "x")
	mustSet(t, d, 1, 6, 0, "* hors taxes")
	mustSet(t, d, 1, 7, 0, "=1+1")
	if s := styleAt(t, d, 7, 0); s[outKey] != true || s["b"] != true {
		t.Fatalf("formule sous une note : %v (%s)", s, layoutOf(t, d))
	}
}

// Audit 2 M5 : un nom propre après Total, Somme, Moyenne est une donnée ;
// « Total général », « Total HT », « Sous-total : » restent des totaux.
func TestTotalLabel(t *testing.T) {
	for s, want := range map[string]bool{
		"Total": true, "TOTAL": true, "Sous-total :": true, "Total général": true, "Total HT": true,
		"total des ventes": true, "Moyenne : 12": true,
		"Total Energies": false, "Moyenne Garonne": false, "Totalement": false,
	} {
		if got := totalLabel(s); got != want {
			t.Errorf("totalLabel(%q) = %v", s, got)
		}
	}
	// Tapé sous le tableau : une ligne de données.
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	mustSet(t, d, 1, 4, 0, "Total Energies")
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×4"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	// Une colonne qui a déjà « Total… » parmi ses valeurs : « Total » de plus
	// est une donnée.
	mustSet(t, d, 1, 5, 0, "Total")
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×5"; got != want {
		t.Fatalf("colonne de même forme :\n got %s\nwant %s", got, want)
	}
}

// Audit 2 M6 : sous un tableau filtré, une note ou un total restent permis ;
// une nouvelle ligne est refusée.
func TestLegendUnderFiltered(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	must(t, d.SetFilter(ctx, "Feuille1", 1, &ColumnFilter{Values: []string{"Nord"}}))
	mustSet(t, d, 1, 4, 0, "Source : INSEE")
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×3 A5*=Source : INSEE"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	_, err := d.SetCell(ctx, 1, 4, 1, "Ouest")
	var ef ErrFiltered
	if !errors.As(err, &ef) {
		t.Fatalf("nouvelle ligne sous un tableau filtré : %v", err)
	}
}
