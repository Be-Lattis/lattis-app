package canvas

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestTiles(t *testing.T) {
	for _, c := range []struct {
		h, w   int
		dst    Rect
		nr, nc int
	}{
		{1, 1, Rect{0, 0, 4, 2}, 5, 3},  // une cellule remplit la sélection
		{2, 1, Rect{0, 0, 5, 1}, 3, 2},  // motif répété
		{2, 2, Rect{0, 0, 4, 3}, 1, 1},  // pas un multiple : collé une fois
		{3, 3, Rect{2, 2, 2, 2}, 1, 1},  // une seule cellule visée
		{1, 3, Rect{0, 0, 3, 2}, 4, 1},  // une ligne sur quatre lignes
		{2, 2, Rect{0, 0, 1, 1}, 1, 1},  // même taille
		{0, 2, Rect{0, 0, 1, 1}, 1, 1},  // bloc vide
		{1, 1, Rect{3, 3, 3, 3}, 1, 1},  // cellule sur cellule
		{1, 2, Rect{0, 0, 0, 5}, 1, 3},  // sur une ligne
		{2, 1, Rect{0, 0, 1, 0}, 1, 1},  // exactement
		{4, 1, Rect{0, 0, 1, 0}, 1, 1},  // sélection plus petite
		{1, 1, Rect{0, 0, 0, 0}, 1, 1},  // trivial
		{2, 3, Rect{0, 0, 3, 5}, 2, 2},  // 2 × 2 répétitions
		{2, 3, Rect{1, 1, 4, 6}, 2, 2},  // décalé
		{2, 3, Rect{1, 1, 4, 7}, 1, 1},  // largeur non multiple
		{1, 1, Rect{0, 0, 9, 0}, 10, 1}, // colonne
	} {
		nr, nc := tiles(c.h, c.w, c.dst)
		if nr != c.nr || nc != c.nc {
			t.Errorf("tiles(%d, %d, %+v) = %d, %d ; attendu %d, %d", c.h, c.w, c.dst, nr, nc, c.nr, c.nc)
		}
	}
}

// grille : cellules saisies, une ligne par ligne.
func fillCells(t *testing.T, d *fixture, row, col int, rows ...[]string) {
	t.Helper()
	for i, r := range rows {
		for j, v := range r {
			if _, err := d.SetCell(context.Background(), 1, row+i, col+j, v); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func freeCells(t *testing.T, d *fixture) map[string]string {
	t.Helper()
	got := map[string]string{}
	d.read(context.Background(), func(x *tx) error {
		cs, err := x.cells(1)
		for _, c := range cs {
			got[fmt.Sprintf("%d,%d", c.Row, c.Col)] = c.Raw
		}
		return err
	})
	return got
}

func freeStyles(t *testing.T, d *fixture) map[string]string {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range l.Styles {
		got[fmt.Sprintf("%d,%d", s.Row, s.Col)] = s.Style
	}
	return got
}

func undoAll(t *testing.T, d *fixture, n int, want string) {
	t.Helper()
	for range n {
		if _, err := d.bus.Undo(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := dump(t, d); got != want {
		t.Fatalf("annulation :\n%s", diff(want, got))
	}
}

// Copier-coller de cellules libres : formules telles quelles (adresses
// absolues), mise en forme et notes ; la source reste.
func TestTransferCopyFree(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"5", "=Cell[A1]*2"})
	if err := d.ApplyStyle(ctx, 1, 0, 0, 0, 0, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetNote(ctx, 1, 0, 1, "note"); err != nil {
		t.Fatal(err)
	}
	start := dump(t, d)
	if _, err := d.Transfer(ctx, 1, Rect{0, 0, 0, 1}, 1, Rect{4, 3, 4, 3}, false, [][]string{{"5", "10"}}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "copie")
	got := freeCells(t, d)
	for k, v := range map[string]string{"0,0": "5", "0,1": "=Cell[A1]*2", "4,3": "5", "4,4": "=Cell[A1]*2"} {
		if got[k] != v {
			t.Errorf("%s : %q, attendu %q (%v)", k, got[k], v, got)
		}
	}
	st := freeStyles(t, d)
	if !strings.Contains(st["4,3"], `"b":true`) || !strings.Contains(st["4,4"], `"nt":"note"`) {
		t.Errorf("mise en forme non copiée : %v", st)
	}
	undoAll(t, d, 1, start)
}

// Une cellule copiée remplit toute la sélection ; un bloc dont la taille
// divise la sélection y est répété.
func TestTransferTile(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"x"})
	if _, err := d.Transfer(ctx, 1, Rect{0, 0, 0, 0}, 1, Rect{0, 3, 0, 5}, false, nil); err != nil {
		t.Fatal(err)
	}
	got := freeCells(t, d)
	for c := 3; c <= 5; c++ {
		if got[fmt.Sprintf("0,%d", c)] != "x" {
			t.Fatalf("remplissage : %v", got)
		}
	}
	fillCells(t, d, 2, 0, []string{"=1", "=2"})
	if _, err := d.Transfer(ctx, 1, Rect{2, 0, 2, 1}, 1, Rect{4, 0, 4, 3}, false, nil); err != nil {
		t.Fatal(err)
	}
	got = freeCells(t, d)
	if got["4,0"] != "=1" || got["4,1"] != "=2" || got["4,2"] != "=1" || got["4,3"] != "=2" {
		t.Fatalf("motif répété : %v", got)
	}
}

// Un tableau entier copié devient un tableau : types, colonne calculée et
// clé suivent ; le nom est libre.
func TestTransferCopyTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"Produit", "Prix"}, []string{"a", "10"}, []string{"b", "20"})
	fillCells(t, d, 0, 2, []string{"Double"}, []string{"=[Prix]*2"})
	var name string
	d.read(ctx, func(x *tx) error { return x.row(`SELECT name FROM _lattis_tables`).Scan(&name) })
	if err := d.SetKey(ctx, name, 0); err != nil {
		t.Fatal(err)
	}
	start := dump(t, d)
	if _, err := d.Transfer(ctx, 1, Rect{0, 0, 2, 2}, 1, Rect{0, 6, 0, 6}, false, nil); err != nil {
		t.Fatal(err)
	}
	check(t, d, "copie du tableau")
	err := d.read(ctx, func(x *tx) error {
		tables, err := x.tables(1)
		if err != nil {
			return err
		}
		if len(tables) != 2 {
			return fmt.Errorf("tableaux : %+v", tables)
		}
		cp := tables[1]
		if cp.Row != 0 || cp.Col != 6 || cp.Rows != 2 || cp.Name == name {
			return fmt.Errorf("copie : %+v", cp)
		}
		if cp.Cols[1].Type != tables[0].Cols[1].Type || cp.Cols[2].Formula == "" || cp.Key == 0 {
			return fmt.Errorf("colonnes ou clé perdues : %+v", cp)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	undoAll(t, d, 1, start)
}

// Couper-coller (ou glisser) un tableau entier avec des cellules libres :
// tout se déplace ensemble, les adresses suivent, une seule annulation.
func TestTransferMoveMixed(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"Produit", "Prix"}, []string{"a", "10"}, []string{"b", "20"})
	fillCells(t, d, 4, 0, []string{"=Cell[B5]+1", "7"})
	fillCells(t, d, 9, 9, []string{"=Cell[B5]"})
	start := dump(t, d)
	if _, err := d.Transfer(ctx, 1, Rect{0, 0, 4, 1}, 1, Rect{2, 4, 2, 4}, true, nil); err != nil {
		t.Fatal(err)
	}
	check(t, d, "déplacement mixte")
	err := d.read(ctx, func(x *tx) error {
		tables, err := x.tables(1)
		if err != nil {
			return err
		}
		if len(tables) != 1 || tables[0].Row != 2 || tables[0].Col != 4 {
			return fmt.Errorf("tableau : %+v", tables)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got := freeCells(t, d)
	for k, v := range map[string]string{"6,4": "=Cell[F7]+1", "6,5": "7", "9,9": "=Cell[F7]"} {
		if got[k] != v {
			t.Errorf("%s : %q, attendu %q (%v)", k, got[k], v, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("cellules : %v", got)
	}
	undoAll(t, d, 1, start)
}

// Des cellules de tableau déplacées vers une zone libre : leurs valeurs
// partent, la source est vidée.
func TestTransferMovePartial(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"Produit", "Prix"}, []string{"a", "10"}, []string{"b", "20"}, []string{"c", "30"})
	start := dump(t, d)
	if _, err := d.Transfer(ctx, 1, Rect{1, 1, 1, 1}, 1, Rect{1, 5, 1, 5}, true, nil); err != nil {
		t.Fatal(err)
	}
	check(t, d, "déplacement d'une cellule de tableau")
	if got := view(t, d); got != lines("[Produit]|[Prix]", "a|||||'10", "b|20", "c|30") {
		t.Fatalf("feuille\n%s", got)
	}
	undoAll(t, d, 1, start)
}

// Des cellules libres posées dans un tableau remplacent ses valeurs ; juste
// dessous, elles l'étendent.
func TestTransferIntoTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"Produit", "Prix"}, []string{"a", "10"}, []string{"b", "20"})
	fillCells(t, d, 0, 5, []string{"z", "99"})
	start := dump(t, d)
	if _, err := d.Transfer(ctx, 1, Rect{0, 5, 0, 6}, 1, Rect{1, 0, 1, 0}, false, nil); err != nil {
		t.Fatal(err)
	}
	check(t, d, "collage dans un tableau")
	if got := view(t, d); got != lines("[Produit]|[Prix]||||'z|'99", "z|99", "b|20") {
		t.Fatalf("feuille\n%s", got)
	}
	if _, err := d.Transfer(ctx, 1, Rect{0, 5, 0, 6}, 1, Rect{3, 0, 3, 0}, true, nil); err != nil {
		t.Fatal(err)
	}
	check(t, d, "déplacement sous un tableau")
	if got := view(t, d); got != lines("[Produit]|[Prix]", "z|99", "b|20", "z|99") {
		t.Fatalf("feuille\n%s", got)
	}
	undoAll(t, d, 2, start)
}

// Un tableau déposé côte à côte d'un autre fusionne ; posé sur un autre,
// celui-ci s'écarte vers la droite.
func TestTransferTableMergeAndPush(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"A", "B"}, []string{"1", "2"})
	column(t, d, 0, 5, "C", "3")
	start := dump(t, d)
	// C à droite de B : fusion.
	if _, err := d.Transfer(ctx, 1, Rect{0, 5, 1, 5}, 1, Rect{0, 2, 0, 2}, true, nil); err != nil {
		t.Fatal(err)
	}
	check(t, d, "fusion")
	if got := view(t, d); got != lines("[A]|[B]|[C]", "1|2|3") {
		t.Fatalf("feuille\n%s", got)
	}
	undoAll(t, d, 1, start)
	// Une copie de C posée sur A : A et B s'écartent.
	if _, err := d.Transfer(ctx, 1, Rect{0, 5, 1, 5}, 1, Rect{3, 0, 3, 0}, false, nil); err != nil {
		t.Fatal(err)
	}
	check(t, d, "copie posée sur un tableau")
	if got := view(t, d); !strings.HasPrefix(got, "[A]|[B]") {
		t.Fatalf("feuille\n%s", got)
	}
	undoAll(t, d, 1, start)
	if _, err := d.Transfer(ctx, 1, Rect{0, 5, 1, 5}, 1, Rect{1, 1, 1, 1}, false, nil); err != nil {
		t.Fatal(err)
	}
	check(t, d, "copie chevauchant un tableau")
	err := d.read(ctx, func(x *tx) error {
		tables, err := x.tables(1)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			if tb.Name != "Feuille1" && tb.Col == 1 && tb.Row == 1 {
				return nil // la copie reste où elle est posée
			}
		}
		return fmt.Errorf("copie déplacée : %+v", tables)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Des colonnes entières (en-tête et toutes les lignes) copiées font un
// nouveau tableau de ces colonnes.
func TestTransferCopyColumns(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"Produit", "Prix", "Stock"}, []string{"a", "10", "1"}, []string{"b", "20", "2"})
	if _, err := d.Transfer(ctx, 1, Rect{0, 1, 2, 2}, 1, Rect{0, 5, 0, 5}, false, nil); err != nil {
		t.Fatal(err)
	}
	check(t, d, "copie de colonnes")
	if got := view(t, d); got != lines("[Produit]|[Prix]|[Stock]|||[Prix]|[Stock]", "a|10|1|||10|1", "b|20|2|||20|2") {
		t.Fatalf("feuille\n%s", got)
	}
}

// Effacer une zone : tableaux entièrement compris supprimés, cellules de
// tableau vidées, cellules libres vidées (mise en forme gardée sans all),
// en une seule étape d'annulation.
func TestClearRange(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"A", "B"}, []string{"1", "2"})
	fillCells(t, d, 0, 4, []string{"C", "D"}, []string{"3", "4"}, []string{"5", "6"})
	fillCells(t, d, 6, 0, []string{"=1"})
	if err := d.ApplyStyle(ctx, 1, 6, 0, 6, 0, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	start := dump(t, d)
	if err := d.ClearRange(ctx, 1, 0, 0, 6, 4, false); err != nil {
		t.Fatal(err)
	}
	check(t, d, "effacer")
	if got := view(t, d); got != lines("||||[C]|[D]", "|||||4", "|||||6") {
		t.Fatalf("feuille\n%s", got)
	}
	if st := freeStyles(t, d); st["6,0"] == "" {
		t.Fatal("mise en forme perdue")
	}
	undoAll(t, d, 1, start)
	if err := d.ClearRange(ctx, 1, 6, 0, 6, 0, true); err != nil {
		t.Fatal(err)
	}
	if st := freeStyles(t, d); st["6,0"] != "" {
		t.Fatal("mise en forme restée")
	}
}

// Glisser des cellules d'un tableau sur le côté (choix de Louis) : elles
// refont un tableau ; le tableau d'origine vidé, jamais personnalisé,
// disparaît ; un tableau nommé perd ses dernières lignes devenues vides.
func TestMoveOutOfTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	// Une liste : en-tête « Colonne 1 » posé au-dessus.
	column(t, d, 2, 1, "10", "20")
	if ts := tablesOf(t, d); len(ts) != 1 || ts[0].Row != 1 {
		t.Fatalf("liste : %+v", ts)
	}
	before := dump(t, d)
	if _, err := d.Transfer(ctx, 1, Rect{2, 1, 3, 1}, 1, Rect{2, 3, 3, 3}, true, [][]string{{"10"}, {"20"}}); err != nil {
		t.Fatal(err)
	}
	ts := tablesOf(t, d)
	if len(ts) != 1 || ts[0].Col != 3 || ts[0].Rows != 2 {
		t.Fatalf("après le glisser : %+v", ts)
	}
	undoOnce(t, d, before, "glisser hors du tableau")

	// Un tableau nommé : il garde ses colonnes, ses lignes vides du bas partent.
	d2 := open(t)
	if _, err := d2.Paste(ctx, 1, 0, 0, [][]string{{"Prix"}, {"1"}, {"2"}, {"3"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d2.Transfer(ctx, 1, Rect{2, 0, 3, 0}, 1, Rect{2, 3, 3, 3}, true, [][]string{{"2"}, {"3"}}); err != nil {
		t.Fatal(err)
	}
	ts = tablesOf(t, d2)
	if len(ts) != 2 || ts[0].Name == "" || ts[0].Rows != 1 {
		t.Fatalf("tableau nommé : %+v", ts)
	}
}

// Effacer les dernières lignes d'un tableau les supprime (choix de Louis) ;
// une ligne vidée au milieu reste.
func TestClearBottomRows(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Prix"}, {"1"}, {"2"}, {"3"}, {"4"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.ClearRange(ctx, 1, 2, 0, 2, 0, false); err != nil {
		t.Fatal(err)
	}
	if ts := tablesOf(t, d); ts[0].Rows != 4 {
		t.Fatalf("ligne du milieu : %+v", ts)
	}
	if err := d.ClearRange(ctx, 1, 3, 0, 4, 0, false); err != nil {
		t.Fatal(err)
	}
	// Lignes 3 et 4 vidées, et la 2 l'était déjà : il reste la 1.
	if ts := tablesOf(t, d); len(ts) != 1 || ts[0].Rows != 1 {
		t.Fatalf("lignes du bas : %+v", ts)
	}
}
