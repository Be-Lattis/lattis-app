package canvas

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// TestSortSameOrder : le tri (rangs par INSERT … ORDER BY, sort.go) range
// les lignes comme row_number() OVER (ORDER BY …) d'avant, sur des valeurs
// mêlées (vides, textes de casse différente, nombres, égalités), en ordre
// croissant, décroissant et personnalisé ; l'annulation rend les clés
// d'ordre exactes d'avant.
func TestSortSameOrder(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	rnd := rand.New(rand.NewSource(3010))
	words := []any{"Lyon", "lyon", "Paris", "", nil, "éclair", "Zoo", "zoo", "10", "9"}
	const n = 400
	if _, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{{
		Name: "T", Headers: []string{"Mot", "Nombre", "Réel"}, Types: []grid.Type{grid.Text, grid.Integer, grid.Decimal}, Count: n,
		Rows: func(yield func([]any) error) error {
			for range n {
				var num, real any
				if rnd.Intn(8) > 0 {
					num = int64(rnd.Intn(20) - 5)
				}
				if rnd.Intn(8) > 0 {
					real = float64(rnd.Intn(50)) / 4
				}
				if err := yield([]any{words[rnd.Intn(len(words))], num, real}); err != nil {
					return err
				}
			}
			return nil
		},
	}}}}); err != nil {
		t.Fatal(err)
	}
	// Ordre attendu : la requête d'avant (row_number() OVER (ORDER BY by)).
	expected := func(col int, desc bool) []int64 {
		var out []int64
		d.read(ctx, func(x *tx) error {
			tb, err := x.tableByName("T")
			if err != nil {
				return err
			}
			c := tb.Cols[col]
			e := colSQL(c.ID)
			if c.Type == grid.Text {
				e += " COLLATE NOCASE"
			}
			dir := map[bool]string{false: "ASC", true: "DESC"}[desc]
			rs, err := x.query(`SELECT id FROM (SELECT id, row_number() OVER (ORDER BY ` + colSQL(c.ID) + ` IS NULL, ` + e + ` ` + dir + `, _pos) AS n FROM ` + tb.sql() + `) ORDER BY n`)
			if err != nil {
				return err
			}
			defer rs.Close()
			for rs.Next() {
				var id int64
				rs.Scan(&id)
				out = append(out, id)
			}
			return rs.Err()
		})
		return out
	}
	state := func() ([]int64, map[int64]string) {
		var ids []int64
		pos := map[int64]string{}
		d.read(ctx, func(x *tx) error {
			tb, err := x.tableByName("T")
			if err != nil {
				return err
			}
			rs, err := x.query(`SELECT id, _pos FROM ` + tb.sql() + ` ORDER BY _pos`)
			if err != nil {
				return err
			}
			defer rs.Close()
			for rs.Next() {
				var id int64
				var p string
				rs.Scan(&id, &p)
				ids = append(ids, id)
				pos[id] = p
			}
			return rs.Err()
		})
		return ids, pos
	}
	for step, c := range []struct {
		col  int
		desc bool
	}{{0, false}, {1, false}, {2, true}, {0, true}, {1, true}, {2, false}} {
		want := expected(c.col, c.desc)
		_, before := state()
		if err := d.SortTable(ctx, "T", c.col, c.desc); err != nil {
			t.Fatal(err)
		}
		got, _ := state()
		if !slices.Equal(got, want) {
			t.Fatalf("tri %d (%+v) :\nobtenu  %v\nattendu %v", step, c, got, want)
		}
		// Annuler rend les clés d'avant, puis rétablir l'ordre trié.
		if _, err := d.bus.Undo(ctx); err != nil {
			t.Fatal(err)
		}
		if _, after := state(); fmt.Sprint(after) != fmt.Sprint(before) {
			t.Fatalf("tri %d : l'annulation ne rend pas les clés d'avant", step)
		}
		if _, err := d.bus.Redo(ctx); err != nil {
			t.Fatal(err)
		}
		if got, _ := state(); !slices.Equal(got, want) {
			t.Fatalf("tri %d : rétabli dans un autre ordre", step)
		}
	}
	// Ordre personnalisé : les valeurs de la liste d'abord.
	if err := d.SortTableCustom(ctx, "T", 0, CustomOrder{Values: []string{"Zoo", "Paris", "10"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := state()
	var first []any
	d.read(ctx, func(x *tx) error {
		tb, _ := x.tableByName("T")
		for _, id := range got[:3] {
			var v any
			x.row(`SELECT `+colSQL(tb.Cols[0].ID)+` FROM `+tb.sql()+` WHERE id = ?`, id).Scan(&v)
			first = append(first, v)
		}
		return nil
	})
	for _, v := range first {
		if s, _ := v.(string); !strings.EqualFold(s, "zoo") {
			t.Fatalf("ordre personnalisé : %v en tête", first)
		}
	}
}
