package canvas

import (
	"context"
	"fmt"
	"testing"
)

// Une colonne entière dans une cellule libre (=Bilan[TEST], obtenue en
// pointant l'en-tête) se déverse sous la cellule comme dans Excel, au lieu
// de valoir #VALEUR! ; une cellule remplie sur le chemin donne
// #DEVERSEMENT! sans rien recouvrir ; les cellules recouvertes se lisent ;
// une valeur (Bilan[TEST][2]) et un agrégat (SOMME) ne se déversent pas.
func TestFreeColumnSpills(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"TEST", "année"}, {"1", "2024"}, {"2", "2025"}, {"3", "2026"}})
	must(t, err)
	mustSet(t, d, 1, 1, 4, "=Feuille1[TEST]")
	c := cellOn(t, d, 1, 1, 4)
	if fmt.Sprint(c.Spill) != "[[1] [2] [3]]" || c.Error || c.Value != "1" {
		t.Fatalf("colonne entière : %+v", c)
	}
	// Une cellule recouverte se lit comme une valeur.
	mustSet(t, d, 1, 1, 6, "=SOMME(Cell[E2:E4])")
	if v := cellOn(t, d, 1, 1, 6).Value; v != "6" {
		t.Fatalf("cellules recouvertes : %q", v)
	}
	// Un « 1 » saisi sur le chemin : #DEVERSEMENT!, pas de valeurs dessous.
	mustSet(t, d, 1, 3, 4, "1")
	if c := cellOn(t, d, 1, 1, 4); c.Value != ErrSpillText || !c.Error || c.Spill != nil {
		t.Fatalf("déversement bloqué : %+v", c)
	}
	mustSet(t, d, 1, 3, 4, "")
	for raw, want := range map[string]string{"=Feuille1[TEST][2]": "2", "=SOMME(Feuille1[TEST])": "6", "=Feuille1[Inconnue]": "#REF!"} {
		mustSet(t, d, 1, 1, 4, raw)
		if c := cellOn(t, d, 1, 1, 4); c.Value != want || c.Error != (want[0] == '#') || c.Spill != nil {
			t.Fatalf("%s : %+v", raw, c)
		}
	}
}
