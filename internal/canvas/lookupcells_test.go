package canvas

import (
	"fmt"
	"testing"
)

// RECHERCHEV, RECHERCHEH et RECHERCHEX tapées sur des cellules libres, à la
// manière d'un tableur : converties en INDEX / EQUIV(X) sur Cell[…], avec
// les modes approchés (audit des formules du 30/10/2026).
func TestLookupFreeCells(t *testing.T) {
	d := open(t)
	// D1:F3 : seuils 0, 100, 500 ; taux ; libellés.
	for r, row := range [][]string{{"0", "0", "A"}, {"100", "0,05", "B"}, {"500", "0,1", "C"}} {
		for c, v := range row {
			mustSet(t, d, 1, r, 3+c, v)
		}
	}
	for _, c := range []struct{ src, raw, want string }{
		{`=RECHERCHEV(100;D1:F3;3;FAUX)`, `=INDEX(Cell[F1:F3]; EQUIV(100; Cell[D1:D3]; 0))`, "B"},
		{`=RECHERCHEV(250;D1:F3;2)`, `=INDEX(Cell[E1:E3]; EQUIV(250; Cell[D1:D3]; 1))`, "0,05"},
		{`=RECHERCHEH(0;D1:F3;3;FAUX)`, `=INDEX(Cell[D3:F3]; EQUIV(0; Cell[D1:F1]; 0))`, "500"},
		{`=RECHERCHEX(250;D1:D3;F1:F3;"?";1)`, `=SI.NON.DISP(INDEX(Cell[F1:F3]; EQUIVX(250; Cell[D1:D3]; 1)); "?")`, "C"},
		{`=EQUIVX(600;D1:D3;-1)`, `=EQUIVX(600; Cell[D1:D3]; -1)`, "3"},
	} {
		mustSet(t, d, 1, 6, 0, c.src)
		cell := cellOn(t, d, 1, 6, 0)
		if cell.Raw != c.raw || cell.Value != c.want {
			t.Errorf("%s : %q = %q, attendu %q = %q", c.src, cell.Raw, cell.Value, c.raw, c.want)
		}
	}
}

// ASSEMB.V et VERS.COL tapés dans une cellule libre se déversent.
func TestStackSpills(t *testing.T) {
	d := open(t)
	for r, row := range [][]string{{"1", "a"}, {"2", "b"}} {
		for c, v := range row {
			mustSet(t, d, 1, r, c, v)
		}
	}
	for _, c := range []struct{ src, raw, want string }{
		{`=VERS.COL(A1:B2;0;VRAI)`, `=VERS.COL(Cell[A1:B2]; 0; VRAI)`, "[[1] [2] [a] [b]]"},
		{`=ASSEMB.V(A1:B1;{"c".3})`, `=ASSEMB.V(Cell[A1:B1]; {"c".3})`, "[[1 a] [c 3]]"},
		{`=UNIQUE(A1:A2)`, `=UNIQUE(Cell[A1:A2])`, "[[1] [2]]"},
	} {
		mustSet(t, d, 1, 0, 4, c.src)
		cell := cellOn(t, d, 1, 0, 4)
		if got := fmt.Sprint(cell.Spill); cell.Raw != c.raw || got != c.want {
			t.Errorf("%s : %q déverse %s, attendu %q, %s", c.src, cell.Raw, got, c.raw, c.want)
		}
	}
}
