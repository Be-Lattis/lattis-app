package canvas

import (
	"context"
	"math"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// TestSteadyStepsSame : la suite régulière de la numérotation automatique
// (autokey.go, lue dans l'ordre des lignes) dit ce que disait la fonction
// de fenêtre d'avant (lag … OVER (ORDER BY _pos)) : écarts, vides, réels
// entiers, dépassement d'un entier.
func TestSteadyStepsSame(t *testing.T) {
	ctx := context.Background()
	cases := [][]any{
		{int64(1), int64(2), int64(3), int64(4)},
		{int64(10), int64(20), int64(30), int64(40)},
		{int64(1), int64(2), int64(4), int64(5)},
		{int64(1), nil, int64(3), int64(4)},
		{int64(5), int64(4), int64(3), int64(2)},
		{int64(1), int64(2), 3.0, int64(4)},
		{int64(math.MaxInt64 - 2), int64(math.MaxInt64 - 1), int64(math.MaxInt64)},
		{int64(math.MinInt64), int64(0), int64(math.MaxInt64)},
		{int64(7), int64(7), int64(7)},
	}
	for i, vals := range cases {
		d := open(t)
		if _, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{{
			Name: "N", Headers: []string{"N", "B"}, Types: []grid.Type{grid.Integer, grid.Text}, Count: len(vals),
			Rows: func(yield func([]any) error) error {
				for _, v := range vals {
					if err := yield([]any{v, "b"}); err != nil {
						return err
					}
				}
				return nil
			},
		}}}}); err != nil {
			t.Fatal(err)
		}
		d.read(ctx, func(x *tx) error {
			tb, err := x.tableByName("N")
			if err != nil {
				t.Fatal(err)
			}
			col := colSQL(tb.Cols[0].ID)
			for _, step := range []int64{1, 10, -1, 0, math.MaxInt64} {
				var off int
				if err := x.row(`SELECT count(*) FROM (SELECT `+col+` - lag(`+col+`) OVER (ORDER BY _pos) AS d, row_number() OVER (ORDER BY _pos) AS n FROM `+tb.sql()+`) WHERE n > 1 AND (d IS NULL OR d <> ?)`, step).Scan(&off); err != nil {
					t.Fatal(err)
				}
				got, err := x.steadySteps(tb, col, step)
				if err != nil {
					t.Fatal(err)
				}
				if got != (off == 0) {
					t.Errorf("cas %d %v, pas %d : %v, la fenêtre dit %v", i, vals, step, got, off == 0)
				}
			}
			return nil
		})
	}
}
