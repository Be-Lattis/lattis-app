package formula

import (
	"strings"
	"testing"
)

func TestCellRef(t *testing.T) {
	for _, c := range []struct {
		name     string
		row, col int
	}{{"Cell_A1", 0, 0}, {"cell_b3", 2, 1}, {"CELL_Z10", 9, 25}, {"Cell_AA1", 0, 26}} {
		r, k, ok := CellRef(c.name)
		if !ok || r != c.row || k != c.col {
			t.Errorf("%s : %d,%d,%v", c.name, r, k, ok)
		}
		if CellName(c.row, c.col) != "Cell_"+c.name[5:] && !ok {
			t.Errorf("CellName %s", c.name)
		}
	}
	for _, bad := range []string{"Cell_A0", "Cell_1", "Cells_A1", "Cell_A1x"} {
		if _, _, ok := CellRef(bad); ok {
			t.Errorf("%s accepté", bad)
		}
	}
	if CellName(9, 27) != "Cell_AB10" {
		t.Error(CellName(9, 27))
	}
	n, err := Parse("=Cell_A1*2+SOMME(Cell_B2;Feuille2.Cell_C3)", FR)
	if err != nil {
		t.Fatal(err)
	}
	names, _ := Deps(QualifyCells(n, 7))
	if len(names) != 3 || names[0] != "cell_a1#7" || names[1] != "cell_b2#7" || names[2] != "feuille2.cell_c3" {
		t.Errorf("dépendances : %v", names)
	}
}

func TestNomenclature(t *testing.T) {
	for src, want := range map[string]string{
		`=Cell[A1]*2`:                   `=Cell[A1]*2`,
		`=cell_b3+Feuille2.Cell_C4`:     `=Cell[B3]+Feuille2!Cell[C4]`,
		`=Feuille2!Cell[b3]`:            `=Feuille2!Cell[B3]`,
		`='Ma feuille'!Cell[C4]`:        `='Ma feuille'!Cell[C4]`,
		`=Ventes[Prix][2]+Ventes[Prix]`: `=Ventes[Prix][2]+Ventes[Prix]`,
		`=Clients[Ville]["C001"]`:       `=Clients[Ville]["C001"]`,
		`=[@Prix]*[Qté]`:                `=[Prix]*[Qté]`,
		`=Ventes[Prix][Cell[A1]+1]`:     `=Ventes[Prix][Cell[A1]+1]`,
	} {
		n, err := Parse(src, FR)
		if err != nil {
			t.Errorf("%s : %v", src, err)
			continue
		}
		if got := Format(n, FR); got != want {
			t.Errorf("%s : %s, attendu %s", src, got, want)
		}
	}
	if _, err := Parse(`=Cell[1A]`, FR); err == nil {
		t.Error("adresse invalide acceptée")
	}
}

func TestRanges(t *testing.T) {
	for src, want := range map[string]string{
		`=SOMME(Cell[c16:C14])`:            `=SOMME(Cell[C14:C16])`,
		`=SOMME(Feuille2!Cell[A1:B3])`:     `=SOMME(Feuille2!Cell[A1:B3])`,
		`=SOMME('Ma feuille'!Cell[B3:A1])`: `=SOMME('Ma feuille'!Cell[A1:B3])`,
		`=Cell[A1:A1]`:                     `=Cell[A1]`,
		`=SOMME(Ventes[Prix][3:1])`:        `=SOMME(Ventes[Prix][1:3])`,
		`=SOMME([Prix][1:2])`:              `=SOMME([Prix][1:2])`,
	} {
		n, err := Parse(src, FR)
		if err != nil {
			t.Errorf("%s : %v", src, err)
			continue
		}
		if got := Format(n, FR); got != want {
			t.Errorf("%s : %s, attendu %s", src, got, want)
		}
	}
	n := mustParse(t, "=SOMME(Cell[A1:B2])+NB(Feuille2!Cell[C1:C2])", FR)
	names, _ := Deps(QualifyCells(n, 7))
	want := []string{"cell_a1#7", "cell_b1#7", "cell_a2#7", "cell_b2#7", "feuille2.cell_c1", "feuille2.cell_c2"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("dépendances : %v", names)
	}
	for _, bad := range []string{`=SOMME(Cell[A1:A20000])`, `=SOMME(Ventes[Prix][0:2])`, `=SOMME(Cell[A1:1B])`} {
		if _, err := Parse(bad, FR); err == nil {
			t.Errorf("%s accepté", bad)
		}
	}
}

func TestSyntaxMessages(t *testing.T) {
	for src, want := range map[string]string{
		`=1+`:            "position 3 : il manque une valeur après « + »",
		`=SOMME(1;`:      "position 9 : il manque une valeur après « ; »",
		`=SOMME(1;2`:     "position 10 : il manque une parenthèse fermante « ) » à SOMME",
		`=(1+2`:          "position 1 : il manque une parenthèse fermante « ) »",
		`=1+2)`:          "position 4 : parenthèse fermante « ) » en trop",
		`=1 2`:           "position 3 : il manque un opérateur avant « 2 »",
		`=*2`:            "position 1 : il manque une valeur avant « * »",
		`=`:              "position 1 : la formule est vide",
		`=SOMME(1 2)`:    "position 9 : il manque « ; » ou « ) » avant « 2 »",
		`=$C$14+C$2`:     "",
		`=Feuille2!$B$3`: "",
	} {
		_, err := Parse(src, FR)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != want {
			t.Errorf("%s : %q, attendu %q", src, got, want)
		}
	}
	if n := mustParse(t, "=$C$14+Feuille2!B$3", FR); Format(n, FR) != "=C14+Feuille2!B3" {
		t.Error(Format(n, FR))
	}
}

func TestExternal(t *testing.T) {
	for src, want := range map[string]bool{
		"=ARRONDI([Quantité]*[Prix unitaire]; 2)": false,
		"=SOMME(Ventes[Prix])":                    false,
		"=[Prix]*Autres[Taux]":                    true,
		"=REFERENCE(Clients[Ville]; [Client])":    true,
		"=[Prix]*Cell[A1]":                        true,
		"=ALEA()":                                 true,
		"=(((":                                    true,
	} {
		if got := External(src, "Ventes"); got != want {
			t.Errorf("%s : %v", src, got)
		}
	}
}
