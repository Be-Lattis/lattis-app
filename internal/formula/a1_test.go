package formula

import (
	"strings"
	"testing"
)

// fakeSheets : Ventes en B2 (en-tête), colonnes Produit, Prix, 3 lignes ;
// Clients en F2, clé Id, 2 lignes ; la cellule nommée TVA1.
type fakeSheets struct{}

func (fakeSheets) Known(name string) bool { return strings.EqualFold(name, "TVA1") }

func (fakeSheets) Tables(sheet string) ([]TablePos, bool) {
	switch strings.ToLower(sheet) {
	case "":
		return []TablePos{
			{Name: "Ventes", Row: 1, Col: 1, Cols: []string{"Produit", "Prix"}, Rows: 3},
			{Name: "Clients", Row: 1, Col: 5, Cols: []string{"Id", "Nom"}, Rows: 2},
		}, true
	case "feuille2", "ma feuille":
		return nil, true
	}
	return nil, false
}

func (fakeSheets) Key(table string, k int) (Node, bool) {
	if table == "Clients" {
		return StrLit{[]string{"", "C001", "C002"}[k]}, true
	}
	return nil, false
}

func TestConvertA1(t *testing.T) {
	for src, want := range map[string]string{
		`=A1*2`:                         `=Cell[A1]*2`,
		`=$a$1+Feuille2!B$3`:            `=Cell[A1]+Feuille2!Cell[B3]`,
		`='Ma feuille'!C4`:              `='Ma feuille'!Cell[C4]`,
		`=C3`:                           `=Ventes[Prix][1]`,
		`=C2`:                           `=Ventes[Prix]`,
		`=G4`:                           `=Clients[Nom][2]`,
		`=TVA1*2`:                       `=TVA1*2`,
		`=SOMME(A10:A12)`:               `=SOMME(Cell[A10:A12])`,
		`=SOMME(A10:B10;C4:C5)`:         `=SOMME(Cell[A10:B10]; Ventes[Prix][2:3])`,
		`=SOMME(C2:C5)`:                 `=SOMME(Ventes[Prix])`,
		`=SOMME(C4:C9)`:                 `=SOMME(Ventes[Prix][2:3])`,
		`=SOMME(Feuille2!A1:A3)`:        `=SOMME(Feuille2!Cell[A1:A3])`,
		`=SOMME(A1:A1)`:                 `=SOMME(Cell[A1])`,
		`=REFERENCE(Clients;Q1;"C001")`: `=REFERENCE(Clients[Q1]; "C001")`,
	} {
		n, err := Parse(src, FR)
		if err != nil {
			t.Errorf("%s : %v", src, err)
			continue
		}
		n, err = ConvertA1(n, fakeSheets{})
		if err != nil {
			t.Errorf("%s : %v", src, err)
			continue
		}
		if got := Format(n, FR); got != want {
			t.Errorf("%s : %s, attendu %s", src, got, want)
		}
	}
	for _, bad := range []string{`=SOMME(B3:C4)`, `=Inconnue!A1`, `=SOMME(A1:CV200)`} {
		n := mustParse(t, bad, FR)
		if _, err := ConvertA1(n, fakeSheets{}); err == nil {
			t.Errorf("%s : conversion acceptée", bad)
		}
	}
	// RECHERCHEV : la plage se situe sur la feuille.
	r := A1Resolver(fakeSheets{})
	if tb, first, ok := r.Resolve("$F$3:G$4"); !ok || tb != "Clients" || first != 0 {
		t.Errorf("Resolve : %s %d %v", tb, first, ok)
	}
	if _, _, ok := r.Resolve("A1:A3"); ok {
		t.Error("plage libre résolue")
	}
}
