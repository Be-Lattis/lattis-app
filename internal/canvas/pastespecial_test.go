package canvas

import (
	"context"
	"strings"
	"testing"
)

// cellText : la saisie gardée d'une cellule (cellule libre ou de tableau).
func cellText(t *testing.T, d *fixture, row, col int) string {
	t.Helper()
	var s string
	if err := d.read(context.Background(), func(x *tx) (err error) {
		s, _, err = x.rawAt(1, row, col)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return s
}

func wantCells(t *testing.T, d *fixture, want map[[2]int]string) {
	t.Helper()
	for k, v := range want {
		if got := cellText(t, d, k[0], k[1]); got != v {
			t.Errorf("%v : %q, attendu %q", k, got, v)
		}
	}
}

// Valeurs, formules, formats : chacun ne colle que sa part ; une seule
// annulation.
func TestPasteSpecialParts(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"5", "=Cell[A1]*2"})
	if err := d.ApplyStyle(ctx, 1, 0, 0, 0, 1, Style{"b": true, "bg": "#ffff00"}); err != nil {
		t.Fatal(err)
	}
	fillCells(t, d, 6, 0, []string{"x", "y"})
	start := dump(t, d)
	src, shown := Rect{0, 0, 0, 1}, [][]string{{"5", "10"}}

	if _, err := d.PasteSpecial(ctx, 1, src, 1, Rect{4, 0, 4, 0}, PasteOptions{What: "values"}, shown); err != nil {
		t.Fatal(err)
	}
	wantCells(t, d, map[[2]int]string{{4, 0}: "5", {4, 1}: "10"})
	if st := freeStyles(t, d); st["4,0"] != "" || st["4,1"] != "" {
		t.Errorf("Valeurs : mise en forme collée %v", st)
	}
	undoAll(t, d, 1, start)

	if _, err := d.PasteSpecial(ctx, 1, src, 1, Rect{4, 0, 4, 0}, PasteOptions{What: "formulas"}, shown); err != nil {
		t.Fatal(err)
	}
	wantCells(t, d, map[[2]int]string{{4, 0}: "5", {4, 1}: "=Cell[A1]*2"})
	if st := freeStyles(t, d); st["4,1"] != "" {
		t.Errorf("Formules : mise en forme collée %v", st)
	}
	undoAll(t, d, 1, start)

	if _, err := d.PasteSpecial(ctx, 1, src, 1, Rect{6, 0, 6, 0}, PasteOptions{What: "formats"}, shown); err != nil {
		t.Fatal(err)
	}
	wantCells(t, d, map[[2]int]string{{6, 0}: "x", {6, 1}: "y"})
	if st := freeStyles(t, d); !strings.Contains(st["6,0"], `"b":true`) || !strings.Contains(st["6,1"], `"bg":"#ffff00"`) {
		t.Errorf("Formats : %v", st)
	}
	undoAll(t, d, 1, start)

	// Valeurs et formats de nombre : le format, pas le gras.
	if err := d.ApplyStyle(ctx, 1, 0, 0, 0, 0, Style{"nf": "%"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PasteSpecial(ctx, 1, src, 1, Rect{4, 0, 4, 0}, PasteOptions{What: "valuesNumbers"}, shown); err != nil {
		t.Fatal(err)
	}
	if st := freeStyles(t, d); st["4,0"] != `{"nf":"%"}` {
		t.Errorf("Valeurs et formats de nombre : %q", st["4,0"])
	}
}

// Transposer : la ligne copiée devient une colonne (formules et mise en
// forme comprises avec Tout).
func TestPasteSpecialTranspose(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"a"}, nil, []string{"=1+1"})
	fillCells(t, d, 1, 0, []string{"b"})
	if err := d.ApplyStyle(ctx, 1, 1, 0, 1, 0, Style{"i": true}); err != nil {
		t.Fatal(err)
	}
	start := dump(t, d)
	// Une colonne devient une ligne : saisies, formule et mise en forme.
	if _, err := d.PasteSpecial(ctx, 1, Rect{0, 0, 2, 0}, 1, Rect{6, 3, 6, 3}, PasteOptions{Transpose: true}, [][]string{{"a"}, {"b"}, {"2"}}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "transposé")
	wantCells(t, d, map[[2]int]string{{6, 3}: "a", {6, 4}: "b", {6, 5}: "=1+1", {7, 3}: ""})
	if st := freeStyles(t, d); !strings.Contains(st["6,4"], `"i":true`) {
		t.Errorf("mise en forme non transposée : %v", st)
	}
	undoAll(t, d, 1, start)
	// Une ligne devient une colonne dans une zone vide : un tableau, comme
	// au collage ordinaire (Ctrl+Z défait d'abord sa création).
	fillCells(t, d, 0, 4, []string{"Nom", "x", "y"})
	start = dump(t, d)
	if _, err := d.PasteSpecial(ctx, 1, Rect{0, 4, 0, 6}, 1, Rect{3, 8, 3, 8}, PasteOptions{What: "values", Transpose: true}, [][]string{{"Nom", "x", "y"}}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "transposé en tableau")
	wantCells(t, d, map[[2]int]string{{3, 8}: "Nom", {4, 8}: "x", {5, 8}: "y"})
	undoAll(t, d, 2, start)
}

// Opérations : combinées aux nombres de la destination ; une formule de
// cellule libre devient =(formule)+source ; un texte ne change pas.
func TestPasteSpecialOperations(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"2", "0,5", "4"})
	fillCells(t, d, 4, 0, []string{"10", "=1+1", "texte"})
	start := dump(t, d)
	src, shown := Rect{0, 0, 0, 2}, [][]string{{"2", "0,5", "4"}}
	for _, c := range []struct {
		op   string
		want map[[2]int]string
	}{
		{"add", map[[2]int]string{{4, 0}: "12", {4, 1}: "=1+1+0,5", {4, 2}: "texte"}},
		{"sub", map[[2]int]string{{4, 0}: "8", {4, 1}: "=1+1-0,5"}},
		{"mul", map[[2]int]string{{4, 0}: "20", {4, 1}: "=(1+1)*0,5"}},
		{"div", map[[2]int]string{{4, 0}: "5", {4, 1}: "=(1+1)/0,5"}},
	} {
		if _, err := d.PasteSpecial(ctx, 1, src, 1, Rect{4, 0, 4, 0}, PasteOptions{What: "values", Op: c.op}, shown); err != nil {
			t.Fatal(c.op, err)
		}
		wantCells(t, d, c.want)
		undoAll(t, d, 1, start)
	}
	// Division par une cellule vide : #DIV/0!, sauf « Blancs non compris ».
	fillCells(t, d, 8, 0, []string{"6", "", "9"})
	start = dump(t, d)
	blank := [][]string{{"3", "", "3"}}
	fillCells(t, d, 10, 0, []string{"3", "", "3"})
	start = dump(t, d)
	if _, err := d.PasteSpecial(ctx, 1, Rect{10, 0, 10, 2}, 1, Rect{8, 0, 8, 0}, PasteOptions{What: "values", Op: "div"}, blank); err != nil {
		t.Fatal(err)
	}
	wantCells(t, d, map[[2]int]string{{8, 0}: "2", {8, 1}: "#DIV/0!", {8, 2}: "3"})
	undoAll(t, d, 1, start)
	if _, err := d.PasteSpecial(ctx, 1, Rect{10, 0, 10, 2}, 1, Rect{8, 0, 8, 0}, PasteOptions{What: "values", Op: "add", SkipBlanks: true}, blank); err != nil {
		t.Fatal(err)
	}
	wantCells(t, d, map[[2]int]string{{8, 0}: "9", {8, 1}: "", {8, 2}: "12"})
}

// Blancs non compris : une cellule vide de la source laisse la destination.
func TestPasteSpecialSkipBlanks(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"1", "", "3"})
	fillCells(t, d, 4, 0, []string{"a", "b", "c"})
	start := dump(t, d)
	shown := [][]string{{"1", "", "3"}}
	if _, err := d.PasteSpecial(ctx, 1, Rect{0, 0, 0, 2}, 1, Rect{4, 0, 4, 0}, PasteOptions{SkipBlanks: true}, shown); err != nil {
		t.Fatal(err)
	}
	wantCells(t, d, map[[2]int]string{{4, 0}: "1", {4, 1}: "b", {4, 2}: "3"})
	undoAll(t, d, 1, start)
	if _, err := d.PasteSpecial(ctx, 1, Rect{0, 0, 0, 2}, 1, Rect{4, 0, 4, 0}, PasteOptions{What: "values"}, shown); err != nil {
		t.Fatal(err)
	}
	wantCells(t, d, map[[2]int]string{{4, 1}: ""})
}

// Dans un tableau : les valeurs remplacent celles des cellules, une formule
// de cellule libre y entre par sa valeur, une colonne calculée garde sa
// formule ; la mise en forme collée sur toutes les lignes d'une colonne
// devient celle de la colonne.
func TestPasteSpecialTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"Produit", "Prix"}, []string{"a", "10"}, []string{"b", "20"})
	fillCells(t, d, 0, 2, []string{"Double"}, []string{"=[Prix]*2"})
	fillCells(t, d, 10, 0, []string{"100", "=Cell[A11]+1", "7"})
	fillCells(t, d, 11, 0, []string{"200", "=Cell[A12]+1", "8"})
	if err := d.ApplyStyle(ctx, 1, 10, 0, 11, 2, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	var tb Table
	read := func() {
		if err := d.read(ctx, func(x *tx) error {
			ts, err := x.tables(1)
			if err == nil {
				tb = ts[0]
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	read()
	start := dump(t, d)
	src, shown := Rect{10, 0, 11, 2}, [][]string{{"100", "101", "7"}, {"200", "201", "8"}}
	// Sur les colonnes Prix (1) et Double (2) depuis la colonne 1 : la valeur
	// de la formule entre dans Prix, Double garde sa formule.
	if _, err := d.PasteSpecial(ctx, 1, Rect{10, 1, 11, 2}, 1, Rect{1, 1, 1, 1}, PasteOptions{What: "formulas"}, [][]string{{"101", "7"}, {"201", "8"}}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "formules dans un tableau")
	read()
	wantCells(t, d, map[[2]int]string{{1, 1}: "101", {2, 1}: "201"})
	if tb.Cols[2].Formula == "" {
		t.Errorf("la colonne calculée a perdu sa formule : %+v", tb.Cols[2])
	}
	undoAll(t, d, 1, start)

	// Formats sur toute la colonne Produit : style de la colonne.
	if _, err := d.PasteSpecial(ctx, 1, src, 1, Rect{1, 0, 1, 0}, PasteOptions{What: "formats"}, shown); err != nil {
		t.Fatal(err)
	}
	read()
	var col, cell Style
	if err := d.read(ctx, func(x *tx) (err error) {
		if col, err = x.tableStyle(tb.ID, 0, tb.Cols[0].ID); err != nil {
			return err
		}
		ids, err := x.ridsAt(&tb, 1, 1)
		if err != nil {
			return err
		}
		cell, err = x.tableStyle(tb.ID, ids[0], tb.Cols[0].ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if col["b"] != true || len(cell) != 0 {
		t.Errorf("Formats sur la colonne : colonne %v, cellule %v", col, cell)
	}
	wantCells(t, d, map[[2]int]string{{1, 0}: "a", {2, 1}: "20"})
	undoAll(t, d, 1, start)

	// Valeurs transposées dans le tableau, puis Ctrl+Z.
	if _, err := d.PasteSpecial(ctx, 1, Rect{10, 0, 11, 0}, 1, Rect{1, 0, 1, 0}, PasteOptions{What: "values", Transpose: true}, [][]string{{"100"}, {"200"}}); err != nil {
		t.Fatal(err)
	}
	wantCells(t, d, map[[2]int]string{{1, 0}: "100", {1, 1}: "200"})
	undoAll(t, d, 1, start)
}

// Largeurs de colonnes : celles de la source, rien d'autre.
func TestPasteSpecialWidths(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 0, 0, []string{"a", "b"})
	if err := d.SetColumnWidths(ctx, 1, []ColWidth{{Col: 0, Width: 150}, {Col: 1, Width: 40}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PasteSpecial(ctx, 1, Rect{0, 0, 0, 1}, 1, Rect{0, 4, 0, 4}, PasteOptions{What: "widths"}, [][]string{{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]int{}
	for _, w := range l.Widths {
		got[w.Col] = w.Width
	}
	if got[4] != 150 || got[5] != 40 {
		t.Errorf("largeurs : %v", got)
	}
	wantCells(t, d, map[[2]int]string{{0, 4}: ""})
}

// Collage d'un tableur (HTML) : le bloc devient un tableau comme au collage
// ordinaire, et garde sa mise en forme (en-tête, colonne) et ses fusions,
// en une seule étape d'annulation.
func TestPasteBlockStyled(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	start := dump(t, d)
	block := [][]string{{"Ventes 2026", ""}, {"", ""}, {"Ville", "Montant"}, {"Paris", "12,5"}, {"Lyon", "8"}}
	head := `{"b":true,"bg":"#ffff00"}`
	money := `{"nf":"0,00","al":"right"}`
	styles := [][]string{{`{"b":true,"mr":1,"mc":2}`, ""}, {"", ""}, {head, head}, {"", money}, {"", money}}
	if _, err := d.PasteBlock(ctx, 1, Rect{0, 0, 0, 0}, block, styles, PasteOptions{}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "collage d'un tableur")
	var tb Table
	var hs, cs Style
	if err := d.read(ctx, func(x *tx) error {
		ts, err := x.tables(1)
		if err != nil || len(ts) != 1 {
			t.Fatalf("tableaux : %+v %v", ts, err)
		}
		tb = ts[0]
		if hs, err = x.tableStyle(tb.ID, -1, tb.Cols[0].ID); err != nil {
			return err
		}
		cs, err = x.tableStyle(tb.ID, 0, tb.Cols[1].ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if tb.Row != 2 || tb.Cols[1].Name != "Montant" || hs["bg"] != "#ffff00" || cs["nf"] != "0,00" {
		t.Errorf("tableau %+v, en-tête %v, colonne %v", tb, hs, cs)
	}
	if st := freeStyles(t, d); !strings.Contains(st["0,0"], `"mc":2`) || !strings.Contains(st["0,0"], `"b":true`) {
		t.Errorf("titre fusionné : %v", st)
	}
	// Ctrl+Z défait d'abord la création du tableau (comme un collage), puis le collage.
	undoAll(t, d, 2, start)
}

// Bloc venu d'ailleurs avec options : Valeurs (sans mise en forme) et
// Transposer.
func TestPasteBlockSpecial(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	fillCells(t, d, 5, 5, []string{"x"})
	start := dump(t, d)
	block := [][]string{{"1", "2", "3"}}
	styles := [][]string{{`{"b":true}`, "", ""}}
	if _, err := d.PasteBlock(ctx, 1, Rect{0, 0, 0, 0}, block, styles, PasteOptions{What: "values", Transpose: true}); err != nil {
		t.Fatal(err)
	}
	// Des nombres seuls dans une zone vide : un tableau sous « Colonne 1 »,
	// comme au collage ordinaire.
	wantCells(t, d, map[[2]int]string{{0, 0}: "Colonne 1", {1, 0}: "1", {2, 0}: "2", {3, 0}: "3", {0, 1}: ""})
	if st := freeStyles(t, d); st["1,0"] != "" {
		t.Errorf("Valeurs : mise en forme collée %v", st)
	}
	undoAll(t, d, 2, start)
	if _, err := d.PasteBlock(ctx, 1, Rect{5, 5, 5, 5}, block, styles, PasteOptions{What: "formats"}); err != nil {
		t.Fatal(err)
	}
	wantCells(t, d, map[[2]int]string{{5, 5}: "x"})
	if st := freeStyles(t, d); st["5,5"] != `{"b":true}` {
		t.Errorf("Formats : %v", st)
	}
}
