package canvas

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
)

// REFERENCE sur un tableau sans clé : la ligne par son rang (1 = première) ;
// avec une clé, la ligne dont la clé vaut la valeur cherchée. Dans une
// colonne calculée (SQL) comme dans une cellule libre.
func TestReferenceByRankWithoutKey(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"N°", "Client"}, {"100001", "Anna Simon"}, {"100002", "Chloé Faure"}, {"100003", "Théo Simon"}})
	must(t, err)
	_, err = d.Paste(ctx, 1, 0, 4, [][]string{{"Ligne", "ID"}, {"2", "100001"}})
	must(t, err)
	ts := tablesOf(t, d)
	a, b := ts[0].Name, ts[1].Name
	mustSet(t, d, 1, 0, 6, "ParRang")
	mustSet(t, d, 1, 1, 6, `=REFERENCE(`+a+`[Client]; [Ligne])`)
	mustSet(t, d, 1, 0, 7, "ParID")
	mustSet(t, d, 1, 1, 7, `=REFERENCE(`+a+`[Client]; [ID])`)
	mustSet(t, d, 1, 6, 0, `=REFERENCE(`+a+`[Client]; 3)`)
	if k := tablesOf(t, d)[0].Key; k != -1 {
		t.Fatalf("clé posée par la formule : %d", k)
	}
	if c := computedOf(t, d, 2, 2); c != "Chloé Faure" {
		t.Fatalf("par rang : %q", c)
	}
	if c := computedOf(t, d, 2, 3); c != "#REF!" {
		t.Fatalf("rang hors du tableau : %q", c)
	}
	if c := cellOn(t, d, 1, 6, 0).Value; c != "Théo Simon" {
		t.Fatalf("cellule libre, par rang : %q", c)
	}
	must(t, d.SetKey(ctx, a, 0))
	if c := computedOf(t, d, 2, 3); c != "Anna Simon" {
		t.Fatalf("par clé : %q", c)
	}
	_ = b
}

// computedOf lit la première ligne d'une colonne calculée (lecture par fenêtres).
func computedOf(t *testing.T, d *fixture, table int64, col int) string {
	t.Helper()
	exprs, args, _, err := d.ColumnExprs(context.Background(), table)
	if err != nil {
		t.Fatal(err)
	}
	// Toutes les expressions ensemble : elles partagent leurs paramètres.
	got := make([]any, len(exprs))
	ptrs := make([]any, len(exprs))
	for i := range got {
		ptrs[i] = &got[i]
	}
	err = d.read(context.Background(), func(x *tx) error {
		return x.row(`SELECT `+strings.Join(exprs, ", ")+fmt.Sprintf(` FROM t_%d ORDER BY _pos LIMIT 1`, table), args...).Scan(ptrs...)
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(formula.FromResult(got[col]))
}
