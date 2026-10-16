package canvas

import (
	"context"
	"testing"
)

func TestMergeInherited(t *testing.T) {
	cases := []struct {
		old        string
		patch, inh Style
		want       string
	}{
		{``, Style{"b": true}, nil, `{"b":true}`},
		{`{"b":true}`, Style{"b": false}, nil, ``},
		// Une couche au-dessous met en gras : le retrait est explicite.
		{`{"b":true}`, Style{"b": false}, Style{"b": true}, `{"b":null}`},
		{``, Style{"bg": nil}, Style{"bg": "#ff0000"}, `{"bg":null}`},
		{`{"bg":null}`, Style{"bg": "#00ff00"}, Style{"bg": "#ff0000"}, `{"bg":"#00ff00"}`},
		// Un retrait au-dessous ne compte pas.
		{``, Style{"b": false}, Style{"b": nil}, ``},
	}
	for _, c := range cases {
		got, err := merge(c.old, c.patch, c.inh)
		if err != nil || got != c.want {
			t.Errorf("merge(%q, %v, %v) = %q, %v ; attendu %q", c.old, c.patch, c.inh, got, err, c.want)
		}
	}
	if got, _ := strip(`{"b":null,"i":true}`, Style{"b": true}); got != `{"i":true}` {
		t.Errorf("strip = %q", got)
	}
}

func TestLineStyles(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}, {"Paris", "2100"}, {"Nice", "340"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 2, 5, 2, 5, Style{"b": true, "fg": "#ff0000"}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 6, 3, 6, 3, Style{"i": true}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	free := func(r, c int) string {
		var s string
		d.db.Read().QueryRow(`SELECT style FROM _slitable_cell_styles WHERE sheet = 1 AND row = ? AND col = ?`, r, c).Scan(&s)
		return s
	}
	tableStyle := func(rid, col int64) string {
		var s string
		d.db.Read().QueryRow(`SELECT style FROM _slitable_styles WHERE rid = ? AND col = ?`, rid, col).Scan(&s)
		return s
	}

	// Ligne 2 (Paris) en gras : style de ligne, la cellule libre de la ligne
	// le reprend, les cellules du tableau le reçoivent.
	if err := d.ApplyLineStyle(ctx, 1, true, 2, 2, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	if got := free(2, 5); got != `{"fg":"#ff0000"}` {
		t.Fatalf("cellule libre de la ligne : %q", got)
	}
	var paris int64
	d.db.Read().QueryRow(`SELECT id FROM t_1 WHERE c_1 = 'Paris'`).Scan(&paris)
	if got := tableStyle(paris, 1); got != `{"b":true}` {
		t.Fatalf("cellule du tableau : %q", got)
	}
	// Une cellule de la ligne sans gras : retrait explicite.
	if err := d.ApplyStyle(ctx, 1, 2, 5, 2, 5, Style{"b": false}); err != nil {
		t.Fatal(err)
	}
	if got := free(2, 5); got != `{"b":null,"fg":"#ff0000"}` {
		t.Fatalf("retrait dans une ligne en gras : %q", got)
	}

	// Colonne 1 (Pop) remplie : style de colonne de la feuille, et la colonne
	// du tableau (en-tête compris) le reçoit.
	if err := d.ApplyLineStyle(ctx, 1, false, 1, 1, Style{"bg": "#eeeeee"}); err != nil {
		t.Fatal(err)
	}
	if got := tableStyle(0, 2); got != `{"bg":"#eeeeee"}` {
		t.Fatalf("colonne du tableau : %q", got)
	}
	if got := tableStyle(-1, 2); got != `{"bg":"#eeeeee"}` {
		t.Fatalf("en-tête : %q", got)
	}

	// Ligne 6 en gras puis colonne 3 sans gras : au croisement, la colonne
	// l'emporte (comme Excel), la ligne garde le gras ailleurs.
	if err := d.ApplyLineStyle(ctx, 1, true, 6, 6, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyLineStyle(ctx, 1, false, 3, 3, Style{"b": false}); err != nil {
		t.Fatal(err)
	}
	if got := free(6, 3); got != `{"b":null,"i":true}` {
		t.Fatalf("croisement : %q", got)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.RowStyles) != 2 || l.RowStyles[0].Index != 2 || l.RowStyles[0].Style != `{"b":true}` || len(l.ColStyles) != 1 || l.ColStyles[0].Style != `{"bg":"#eeeeee"}` {
		t.Fatalf("styles de lignes %+v, de colonnes %+v", l.RowStyles, l.ColStyles)
	}
	check(t, d, "styles de lignes")
	for range 5 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestDeleteSheetStyles(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	id, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyLineStyle(ctx, id, true, 0, 1, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyLineStyle(ctx, id, false, 2, 2, Style{"bg": "#eeeeee"}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, id, 4, 4, 4, 4, Style{"i": true}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetColumnWidth(ctx, id, 1, 150); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if err := d.DeleteSheet(ctx, id); err != nil {
		t.Fatal(err)
	}
	var n int
	d.db.Read().QueryRow(`SELECT (SELECT count(*) FROM _slitable_row_styles) + (SELECT count(*) FROM _slitable_col_styles) + (SELECT count(*) FROM _slitable_cell_styles) + (SELECT count(*) FROM _slitable_col_widths)`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d styles restent après la suppression de la feuille", n)
	}
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}
