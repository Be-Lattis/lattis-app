package canvas

import (
	"context"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Les fonctions numériques posées sur une colonne de texte lisent les
// nombres qu'elle écrit ; un texte qui n'en écrit pas compte pour 0
// (décision 0013). Cellules libres (évaluateur Go) et colonne calculée (SQL)
// donnent les mêmes résultats.
func TestNumericFunctionsOnTextColumn(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"Libellé", "Montant", "Groupe"},
		{"a", "12", "A"},
		{"b", "1 234,5", "A"},
		{"c", "12 €", "B"},
		{"d", "15 %", "B"},
		{"e", "abc", "A"},
		{"f", "", "B"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetType(ctx, "Feuille1", 1, grid.Text); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"=SOMME(Feuille1[Montant])":                             "1258,65",
		"=MOYENNE(Feuille1[Montant])":                           "251,73",
		"=NB(Feuille1[Montant])":                                "4",
		"=MIN(Feuille1[Montant])":                               "0",
		"=MAX(Feuille1[Montant])":                               "1234,5",
		"=SOMME.SI(Feuille1[Groupe]; \"B\"; Feuille1[Montant])": "12,15",
	}
	col := 5
	for f := range want {
		if _, err := d.SetCell(ctx, 1, 10, col, f); err != nil {
			t.Fatalf("%s : %v", f, err)
		}
		col++
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range l.Cells {
		got[c.Raw] = c.Value
	}
	for f, w := range want {
		if got[f] != w {
			t.Errorf("cellule libre %s = %q, attendu %q", f, got[f], w)
		}
	}

	// Colonne calculée (compilée en SQL) : même total.
	if _, err := d.SetCell(ctx, 1, 0, 3, "Total"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 3, "=SOMME(Feuille1[Montant])+MOYENNE(Feuille1[Montant])"); err != nil {
		t.Fatal(err)
	}
	exprs, args, computed, err := d.ColumnExprs(ctx, 1)
	if err != nil || !computed[3] {
		t.Fatalf("ColumnExprs = %v, %v", computed, err)
	}
	var v any
	if err := d.db.Read().QueryRow(`SELECT `+exprs[3]+` FROM t_1 ORDER BY _pos LIMIT 1`, args...).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if s := formula.FromResult(v).String(); s != "1510,38" {
		t.Fatalf("colonne calculée = %s, attendu 1510,38", s)
	}
	check(t, d, "fonctions numériques sur une colonne de texte")
}
