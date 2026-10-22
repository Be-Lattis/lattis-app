package canvas

import (
	"context"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// LayoutFirst (la vue d'abord, docs/audits/vue-d-abord-gros-classeurs-2026-10-22.md) :
// sur un grand tableau, ni suggestion de clé, ni numérotation, ni TCD, ni
// graphique ne sont calculés s'ils ne sont pas en cache (Pending) ; Layout
// les calcule et les garde ; une saisie dans une autre colonne ne refait
// pas la numérotation (elle ne lit que la première colonne).
func TestLayoutFirst(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	n := quickRows + 1
	if _, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{{
		Name: "Gros", Headers: []string{"N", "Pays", "Montant"}, Types: []grid.Type{grid.Integer, grid.Text, grid.Integer}, Count: n,
		Rows: func(yield func([]any) error) error {
			for i := range n {
				if err := yield([]any{int64(i + 1), []string{"FR", "IT", "DE"}[i%3], int64(i % 97)}); err != nil {
					return err
				}
			}
			return nil
		},
	}}}}); err != nil {
		t.Fatal(err)
	}
	var sheet int64
	d.read(ctx, func(t *tx) error { return t.row(`SELECT sheet FROM _lattis_tables WHERE name = 'Gros'`).Scan(&sheet) })
	if _, err := d.CreateChart(ctx, sheet, 0, 6, ChartConfig{Source: "Gros", Kind: "bar", X: "Pays", Series: []string{"Montant"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PlacePivot(ctx, sheet, 0, 12, PivotConfig{Source: "Gros", RowFields: []string{"Pays"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}); err != nil {
		t.Fatal(err)
	}

	first, err := d.LayoutFirst(ctx, sheet)
	if err != nil {
		t.Fatal(err)
	}
	tb := first.Tables[0]
	if !first.Pending || !tb.Pending || tb.KeyHint || tb.AutoNumber != -1 || tb.Rows != n {
		t.Fatalf("LayoutFirst : pending %v, tableau %+v", first.Pending, tb)
	}
	if len(first.Pivots) != 1 || !first.Pivots[0].Pending || len(first.Pivots[0].Cells) != 0 {
		t.Fatalf("TCD pas encore calculé attendu : %+v", first.Pivots)
	}
	if len(first.Charts) != 1 || !first.Charts[0].Pending || len(first.Charts[0].Labels) != 0 {
		t.Fatalf("graphique pas encore calculé attendu : %+v", first.Charts)
	}

	full, err := d.Layout(ctx, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if full.Pending || full.Tables[0].Pending || !full.Tables[0].KeyHint || full.Tables[0].AutoNumber != 0 {
		t.Fatalf("Layout : %+v", full.Tables[0])
	}
	if full.Pivots[0].Pending || len(full.Pivots[0].Cells) == 0 || full.Charts[0].Pending || len(full.Charts[0].Labels) == 0 {
		t.Fatal("Layout : TCD et graphique calculés attendus")
	}

	// En cache : LayoutFirst donne tout.
	again, err := d.LayoutFirst(ctx, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if again.Pending || !again.Tables[0].KeyHint || again.Tables[0].AutoNumber != 0 || len(again.Pivots[0].Cells) == 0 {
		t.Fatalf("LayoutFirst après Layout : %+v", again.Tables[0])
	}

	// Saisie dans la 3e colonne : la clé et la numérotation restent en
	// cache ; le TCD et le graphique (qui lisent Montant) sont à refaire.
	if _, err := d.SetCell(ctx, sheet, 5, 2, "12"); err != nil {
		t.Fatal(err)
	}
	after, err := d.LayoutFirst(ctx, sheet)
	if err != nil {
		t.Fatal(err)
	}
	if a := after.Tables[0]; a.Pending || !a.KeyHint || a.AutoNumber != 0 {
		t.Fatalf("après une saisie hors de la 1re colonne : %+v", a)
	}
	if !after.Pivots[0].Pending || !after.Charts[0].Pending {
		t.Fatal("après une saisie dans leur source : TCD et graphique à recalculer attendus")
	}
}
