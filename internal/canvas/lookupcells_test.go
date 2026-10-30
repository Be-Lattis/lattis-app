package canvas

import "testing"

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
