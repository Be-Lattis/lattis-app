package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNotes(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}, {"Paris", "2100"}, {"Nice", "340"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	// Cellule libre.
	if err := d.SetNote(ctx, 1, 6, 4, "À vérifier\n"); err != nil {
		t.Fatal(err)
	}
	var s string
	d.db.Read().QueryRow(`SELECT style FROM _slitable_cell_styles WHERE sheet = 1 AND row = 6 AND col = 4`).Scan(&s)
	if s != `{"nt":"À vérifier"}` {
		t.Fatalf("note libre : %q", s)
	}
	// Cellule de tableau (Paris, Pop) : la note suit sa ligne au tri.
	if err := d.SetNote(ctx, 1, 2, 1, "Source : INSEE"); err != nil {
		t.Fatal(err)
	}
	var paris int64
	d.db.Read().QueryRow(`SELECT id FROM t_1 WHERE c_1 = 'Paris'`).Scan(&paris)
	d.db.Read().QueryRow(`SELECT style FROM _slitable_styles WHERE rid = ? AND col = 2`, paris).Scan(&s)
	if s != `{"nt":"Source : INSEE"}` {
		t.Fatalf("note du tableau : %q", s)
	}
	// Une seule ligne de données sélectionnée n'est pas toute la colonne.
	var col string
	d.db.Read().QueryRow(`SELECT coalesce(max(style), '') FROM _slitable_styles WHERE rid = 0`).Scan(&col)
	if col != "" {
		t.Fatalf("la note ne doit pas toucher la colonne : %q", col)
	}
	// « Effacer la mise en forme » ne la retire pas (elle n'est pas dans le correctif).
	if err := d.ApplyStyle(ctx, 1, 6, 4, 6, 4, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetNote(ctx, 1, 6, 4, ""); err != nil {
		t.Fatal(err)
	}
	d.db.Read().QueryRow(`SELECT style FROM _slitable_cell_styles WHERE sheet = 1 AND row = 6 AND col = 4`).Scan(&s)
	if s != `{"b":true}` {
		t.Fatalf("note retirée : %q", s)
	}
	check(t, d, "notes")
	for range 4 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestMergeCells(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		r, c int
		raw  string
	}{{6, 4, "Titre"}, {7, 6, "perdu"}, {9, 4, "dehors"}} {
		if _, err := d.SetCell(ctx, 1, c.r, c.c, c.raw); err != nil {
			t.Fatal(err)
		}
	}
	before := dump(t, d)
	// Dans un tableau : refusé.
	if _, err := d.MergeCells(ctx, 1, 0, 0, 1, 1); !errors.Is(err, ErrMerge) || !strings.Contains(err.Error(), "tableau") {
		t.Fatalf("fusion dans un tableau : %v", err)
	}
	if _, err := d.MergeCells(ctx, 1, 2, 2, 2, 2); !errors.Is(err, ErrMerge) {
		t.Fatalf("une seule cellule : %v", err)
	}
	n, err := d.MergeCells(ctx, 1, 6, 4, 7, 6)
	if err != nil || n != 1 {
		t.Fatalf("fusion : %d valeurs effacées, %v", n, err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Cells) != 2 || l.Cells[0].Raw != "Titre" {
		t.Fatalf("cellules après fusion : %+v", l.Cells)
	}
	var s string
	d.db.Read().QueryRow(`SELECT style FROM _slitable_cell_styles WHERE sheet = 1 AND row = 6 AND col = 4`).Scan(&s)
	if s != `{"al":"center","mc":3,"mr":2}` {
		t.Fatalf("style de la fusion : %q", s)
	}
	// Une fusion qui recouvre la première la remplace.
	if _, err := d.MergeCells(ctx, 1, 7, 5, 8, 8); err != nil {
		t.Fatal(err)
	}
	var ms []Merge
	d.read(ctx, func(t *tx) error { ms, err = t.merges(1); return err })
	if len(ms) != 1 || ms[0] != (Merge{Row: 7, Col: 5, Rows: 2, Cols: 4}) {
		t.Fatalf("fusions : %+v", ms)
	}
	// La fusion suit l'insertion de lignes.
	if err := d.InsertSheetRows(ctx, 1, 0, 2); err != nil {
		t.Fatal(err)
	}
	d.read(ctx, func(t *tx) error { ms, err = t.merges(1); return err })
	if len(ms) != 1 || ms[0].Row != 9 {
		t.Fatalf("fusion après insertion : %+v", ms)
	}
	if err := d.UnmergeCells(ctx, 1, 10, 6, 10, 6); err != nil {
		t.Fatal(err)
	}
	d.read(ctx, func(t *tx) error { ms, err = t.merges(1); return err })
	if len(ms) != 0 {
		t.Fatalf("fusion annulée : %+v", ms)
	}
	check(t, d, "fusions")
	for range 4 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestHiddenLinesAndFreeze(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}, {"Paris", "2100"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyLineStyle(ctx, 1, true, 3, 3, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if err := d.SetLinesHidden(ctx, 1, true, 2, 3, true); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLinesHidden(ctx, 1, false, 1, 1, true); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.RowStyles) != 2 || l.RowStyles[0].Style != `{"hd":true}` || l.RowStyles[1].Style != `{"b":true,"hd":true}` || len(l.ColStyles) != 1 || l.ColStyles[0].Style != `{"hd":true}` {
		t.Fatalf("lignes %+v, colonnes %+v", l.RowStyles, l.ColStyles)
	}
	// Les cellules du tableau ne reçoivent rien : masquer n'est pas une mise en forme.
	var n int
	d.db.Read().QueryRow(`SELECT count(*) FROM _slitable_styles`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d styles de tableau", n)
	}
	// Afficher toute la feuille ne touche que les lignes masquées.
	if err := d.SetLinesHidden(ctx, 1, true, 0, 1_000_000, false); err != nil {
		t.Fatal(err)
	}
	if l, _ = d.Layout(ctx, 1); len(l.RowStyles) != 1 || l.RowStyles[0].Style != `{"b":true}` {
		t.Fatalf("après Afficher : %+v", l.RowStyles)
	}

	// Volets figés.
	if err := d.SetFreeze(ctx, 1, 1, 2); err != nil {
		t.Fatal(err)
	}
	if l, _ = d.Layout(ctx, 1); l.Sheets[0].FreezeRows != 1 || l.Sheets[0].FreezeCols != 2 {
		t.Fatalf("volets : %+v", l.Sheets[0])
	}
	id, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetFreeze(ctx, id, 3, 0); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSheet(ctx, id); err != nil {
		t.Fatal(err)
	}
	d.bus.Undo(ctx) // la feuille revient avec ses volets
	if l, _ = d.Layout(ctx, id); l.Sheets[1].FreezeRows != 3 {
		t.Fatalf("volets de la feuille rendue : %+v", l.Sheets[1])
	}
	check(t, d, "masquage")
	for range 6 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}
