package formula

import "testing"

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
