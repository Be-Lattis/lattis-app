package canvas

import (
	"context"
	"strings"
	"testing"

	"github.com/lalamefine/slitable/internal/formula"
)

// TestRangesAndA1 : adresses A1 tapées à la Excel converties à la saisie,
// plages Cell[A1:C1] et Tableau[Col][1:2] calculées, et réécrites quand des
// colonnes sont insérées ou supprimées.
func TestRangesAndA1(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	set := func(row, col int, raw string) {
		t.Helper()
		if _, err := d.SetCell(ctx, 1, row, col, raw); err != nil {
			t.Fatalf("%s : %v", raw, err)
		}
	}
	cells := func() (raw, val map[string]string) {
		t.Helper()
		l, err := d.Layout(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		raw, val = map[string]string{}, map[string]string{}
		for _, c := range l.Cells {
			a := strings.TrimPrefix(formula.CellName(c.Row, c.Col), "Cell_")
			raw[a], val[a] = c.Raw, c.Value
		}
		return raw, val
	}
	// A1:C1 = 1, 2, 3 (une ligne : pas de tableau) ; tableau T en A5 : x = 1, 2, 3.
	set(0, 0, "1")
	set(0, 1, "2")
	set(0, 2, "3")
	set(4, 0, "x")
	set(5, 0, "1")
	set(6, 0, "2")
	set(7, 0, "3")
	var table string
	d.read(ctx, func(x *tx) error { return x.row(`SELECT name FROM _slitable_tables`).Scan(&table) })
	set(0, 4, "=SOMME(A1:C1)")
	set(1, 4, "=SOMME(A6:$A$7)")
	set(2, 4, "=A6*10+B1")
	set(3, 4, "=SOMME(A5:A8)")
	set(4, 4, "=SOMME(Cell[A1:B1])*2")
	raw, val := cells()
	for a, want := range map[string]string{
		"E1": "=SOMME(Cell[A1:C1])",
		"E2": "=SOMME(" + table + "[x][1:2])",
		"E3": "=" + table + "[x][1]*10+Cell[B1]",
		"E4": "=SOMME(" + table + "[x])",
	} {
		if raw[a] != want {
			t.Errorf("%s : %q, attendu %q", a, raw[a], want)
		}
	}
	for a, want := range map[string]string{"E1": "6", "E2": "3", "E3": "12", "E4": "6", "E5": "6"} {
		if val[a] != want {
			t.Errorf("%s vaut %q, attendu %q", a, val[a], want)
		}
	}
	// Une plage qui déborde d'un tableau est refusée, le message le dit.
	if _, err := d.SetCell(ctx, 1, 9, 4, "=SOMME(A7:B8)"); err == nil || !strings.Contains(err.Error(), "déborde") {
		t.Errorf("plage à cheval : %v", err)
	}
	start := dump(t, d)

	// Une colonne insérée avant B : la plage s'élargit et suit.
	if err := d.InsertSheetColumns(ctx, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	raw, val = cells()
	if raw["F1"] != "=SOMME(Cell[A1:D1])" || val["F1"] != "6" || raw["F5"] != "=SOMME(Cell[A1:C1])*2" {
		t.Errorf("après insertion : F1 %q = %q, F5 %q", raw["F1"], val["F1"], raw["F5"])
	}
	// B et C supprimées (la colonne vide et l'ancienne B) : la plage se réduit.
	if err := d.DeleteSheetColumns(ctx, 1, 1, 2); err != nil {
		t.Fatal(err)
	}
	raw, val = cells()
	if raw["D1"] != "=SOMME(Cell[A1:B1])" || val["D1"] != "4" {
		t.Errorf("après suppression : D1 %q = %q", raw["D1"], val["D1"])
	}
	// Toute la plage disparaît : #REF!.
	if err := d.DeleteSheetColumns(ctx, 1, 0, 1); err != nil {
		t.Fatal(err)
	}
	if raw, _ = cells(); raw["B1"] != "=SOMME(#REF!)" {
		t.Errorf("plage supprimée : %q", raw["B1"])
	}
	check(t, d, "plages")
	for range 3 {
		if _, err := d.bus.Undo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if dump(t, d) != start {
		t.Error("annulations")
	}
}

// TestMoveRange : couper-coller d'une plage entière, ou d'une partie.
func TestMoveRange(t *testing.T) {
	move := func(r0, c0, r1, c1, dr, dc int) func(r, c int) (int, int, bool) {
		return func(r, c int) (int, int, bool) {
			if r >= r0 && r <= r1 && c >= c0 && c <= c1 {
				return r + dr, c + dc, true
			}
			return r, c, true
		}
	}
	if rg, ok := moveRange("A1", "B2", move(0, 0, 5, 5, 3, 1)); !ok || rg.String() != "Cell[B4:C5]" {
		t.Errorf("plage entière : %s", rg)
	}
	if rg, ok := moveRange("A1", "B2", move(0, 0, 0, 5, 3, 1)); !ok || rg.String() != "Cell[A1:B2]" {
		t.Errorf("plage en partie : %s", rg)
	}
	del := func(r, c int) (int, int, bool) {
		if r >= 1 && r <= 2 {
			return 0, 0, false
		}
		if r > 2 {
			return r - 2, c, true
		}
		return r, c, true
	}
	if rg, ok := moveRange("A2", "A5", del); !ok || rg.String() != "Cell[A2:A3]" {
		t.Errorf("lignes supprimées : %s", rg)
	}
	if _, ok := moveRange("A2", "A3", del); ok {
		t.Error("plage supprimée gardée")
	}
}
