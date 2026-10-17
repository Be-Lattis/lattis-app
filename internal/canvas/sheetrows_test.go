package canvas

import (
	"context"
	"testing"
)

// Supprimer plusieurs colonnes d'un tableau se fait en une reconstruction
// de la table ; l'annulation remet valeurs, saisies invalides, mise en forme
// et clé, à l'identique.
func TestDeleteSeveralColumns(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"Code", "Ville", "Pop", "Note", "Fin"},
		{"A", "Lyon", "520", "x", "1"},
		{"B", "Paris", "2100", "y", "2"},
		{"C", "Nice", "340", "z", "3"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetKey(ctx, "Feuille1", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 2, 2, "beaucoup"); err != nil { // saisie hors type (Pop entier)
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 1, 1, 3, 2, Style{"bg": "#ffeeaa"}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	var styles int
	d.db.Read().QueryRow(`SELECT count(*) FROM _slitable_styles`).Scan(&styles)
	if err := d.DeleteSheetColumns(ctx, 1, 1, 2); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range l.Tables[0].Columns {
		names = append(names, c.Name)
	}
	if got := len(names); got != 3 || names[0] != "Code" || names[1] != "Note" || names[2] != "Fin" {
		t.Fatalf("colonnes restantes : %v", names)
	}
	check(t, d, "après suppression")
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s\nattendu :\n%s", got, before)
	}
	var after int
	d.db.Read().QueryRow(`SELECT count(*) FROM _slitable_styles`).Scan(&after)
	if after != styles {
		t.Fatalf("mise en forme : %d au lieu de %d", after, styles)
	}
	check(t, d, "après annulation")
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "après rétablissement")
}
