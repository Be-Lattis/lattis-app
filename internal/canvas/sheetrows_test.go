package canvas

import (
	"context"
	"fmt"
	"strings"
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
	d.db.Read().QueryRow(`SELECT count(*) FROM _lattis_styles`).Scan(&styles)
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
	d.db.Read().QueryRow(`SELECT count(*) FROM _lattis_styles`).Scan(&after)
	if after != styles {
		t.Fatalf("mise en forme : %d au lieu de %d", after, styles)
	}
	check(t, d, "après annulation")
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "après rétablissement")
}

// [Col][-1] et [Col][+1] : la ligne précédente ou suivante du tableau
// (vide au bord), [Col][2] reste le rang absolu.
func TestRelativeRows(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Mois", "Ventes"}, {"jan", "10"}, {"fév", "15"}, {"mars", "12"}}); err != nil {
		t.Fatal(err)
	}
	// Colonnes dans l'ordre : une saisie à deux colonnes du tableau ne l'étend pas.
	for k, f := range []string{"=[Ventes]-[Ventes][-1]", "=[Ventes][+1]", "=[Ventes][2]"} {
		col := k + 2
		if _, err := d.SetCell(ctx, 1, 0, col, "c"+fmt.Sprint(col)); err != nil {
			t.Fatal(err)
		}
		if _, err := d.SetCell(ctx, 1, 1, col, f); err != nil {
			t.Fatal(err)
		}
	}
	exprs, args, _, err := d.ColumnExprs(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := d.db.Read().QueryContext(ctx, "SELECT "+strings.Join(exprs[2:], ", ")+" FROM t_1 ORDER BY _pos", args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var got []string
	for rs.Next() {
		var a, b, c any
		if err := rs.Scan(&a, &b, &c); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprint(a, " ", b, " ", c))
	}
	if g := strings.Join(got, " / "); g != "10 15 15 / 5 12 15 / -3 <nil> 15" { // au bord : vide, 0 dans un calcul (comme Excel)
		t.Fatalf("valeurs : %s", g)
	}
}

// Index de ligne calculé, référence imbriquée comprise : Tableau[Col][1+Tableau[Col2][1]].
func TestNestedRowIndex(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Mois", "Ventes"}, {"jan", "10"}, {"fév", "15"}, {"mars", "12"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 8, 0, "=Feuille1[Mois][1+Feuille1[Ventes][1]-9]"); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range l.Cells {
		if c.Row == 8 && c.Col == 0 {
			if c.Value != "fév" {
				t.Fatalf("valeur : %q (erreur %v)", c.Value, c.Error)
			}
			return
		}
	}
	t.Fatal("cellule introuvable")
}
