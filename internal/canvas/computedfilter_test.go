package canvas

import (
	"context"
	"strings"
	"testing"
)

// Une colonne calculée se trie et se filtre sur ses valeurs, comme une
// colonne saisie.
func TestComputedSortFilter(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Prix", "Qté"}, {"Lyon", "10", "2"}, {"Nice", "5", "9"}, {"Pau", "30", "1"}}); err != nil {
		t.Fatal(err)
	}
	mustSet(t, d, 1, 0, 3, "CA")
	mustSet(t, d, 1, 1, 3, "=[@Prix]*[@Qté]")
	if err := d.SortTable(ctx, "Feuille1", 3, true); err != nil {
		t.Fatal(err)
	}
	check(t, d, "tri décroissant")
	if got := view(t, d); !strings.HasPrefix(got, lines("[Ville]|[Prix]|[Qté]|[CA]", "Nice|5|9", "Pau|30|1", "Lyon|10|2")) {
		t.Fatalf("tri :\n%s", got)
	}
	if err := d.SetFilter(ctx, "Feuille1", 3, &ColumnFilter{Conds: []FilterCond{{Op: "gt", Value: "25"}}}); err != nil {
		t.Fatal(err)
	}
	ts := tablesOf(t, d)
	if ts[0].Rows != 2 {
		t.Fatalf("filtre par condition : %d ligne(s) affichée(s)", ts[0].Rows)
	}
	vals, err := d.FilterValues(ctx, "Feuille1", 3)
	if err != nil || len(vals) != 3 {
		t.Fatalf("valeurs de la colonne calculée : %+v, %v", vals, err)
	}
	if err := d.SetFilter(ctx, "Feuille1", 3, &ColumnFilter{Values: []string{"20"}}); err != nil {
		t.Fatal(err)
	}
	if ts := tablesOf(t, d); ts[0].Rows != 1 {
		t.Fatalf("filtre par valeur : %d ligne(s) affichée(s)", ts[0].Rows)
	}
	check(t, d, "filtre")
}
