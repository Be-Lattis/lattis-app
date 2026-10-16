package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"testing"
)

func TestFilterConds(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 1, 0, [][]string{
		{"Ville", "Montant", "Date"},
		{"Lyon", "120", "01/02/2026"},
		{"Lille", "40", "15/03/2026"},
		{"Paris", "300", "20/01/2026"},
		{"Nice", "75", "05/06/2026"},
		{"Rome 50%", "", "10/10/2026"},
	}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	// Villes affichées avec le filtre f sur la colonne i.
	cities := func(i int, f *ColumnFilter) []string {
		t.Helper()
		if err := d.ClearFilters(ctx, "Feuille1"); err != nil {
			t.Fatal(err)
		}
		if err := d.SetFilter(ctx, "Feuille1", i, f); err != nil {
			t.Fatalf("%+v : %v", f, err)
		}
		var out []string
		d.read(ctx, func(tx *tx) error {
			tbs, err := tx.tables(1)
			if err != nil {
				return err
			}
			tb := tbs[0]
			rs, err := tx.query(`SELECT ` + colSQL(tb.Cols[0].ID) + ` FROM ` + tb.view() + ` ORDER BY _pos`)
			if err != nil {
				return err
			}
			defer rs.Close()
			for rs.Next() {
				var s sql.NullString
				rs.Scan(&s)
				out = append(out, s.String)
			}
			return rs.Err()
		})
		return out
	}
	cond := func(op, v, to string) *ColumnFilter { return &ColumnFilter{Conds: []FilterCond{{Op: op, Value: v, To: to}}} }
	for _, c := range []struct {
		i    int
		f    *ColumnFilter
		want []string
	}{
		{0, cond("contains", "l", ""), []string{"Lyon", "Lille"}},
		{0, cond("notcontains", "l", ""), []string{"Paris", "Nice", "Rome 50%"}},
		{0, cond("begins", "li", ""), []string{"Lille"}},
		{0, cond("ends", "e", ""), []string{"Lille", "Nice"}},
		{0, cond("contains", "50%", ""), []string{"Rome 50%"}},
		{0, cond("eq", "paris", ""), []string{"Paris"}},
		{0, cond("eq", "L*", ""), []string{"Lyon", "Lille"}},
		{0, cond("ne", "paris", ""), []string{"Lyon", "Lille", "Nice", "Rome 50%"}},
		{1, cond("gt", "100", ""), []string{"Lyon", "Paris"}},
		{1, cond("le", "75", ""), []string{"Lille", "Nice"}},
		{1, cond("between", "50", "150"), []string{"Lyon", "Nice"}},
		{1, cond("top", "2", ""), []string{"Lyon", "Paris"}},
		{1, cond("bottom", "1", ""), []string{"Lille"}},
		{1, cond("top", "10", ""), []string{"Lyon", "Lille", "Paris", "Nice"}},
		{1, cond("above", "", ""), []string{"Paris"}},  // moyenne 133,75
		{1, cond("below", "", ""), []string{"Lyon", "Lille", "Nice"}},
		{1, cond("eq", "", ""), []string{"Rome 50%"}},
		{1, cond("ne", "", ""), []string{"Lyon", "Lille", "Paris", "Nice"}},
		{2, cond("lt", "01/03/2026", ""), []string{"Lyon", "Paris"}},
		{2, cond("between", "01/02/2026", "31/03/2026"), []string{"Lyon", "Lille"}},
		{1, &ColumnFilter{Conds: []FilterCond{{Op: "lt", Value: "50"}, {Op: "gt", Value: "200"}}, Or: true}, []string{"Lille", "Paris"}},
		{1, &ColumnFilter{Conds: []FilterCond{{Op: "gt", Value: "50"}, {Op: "lt", Value: "200"}}}, []string{"Lyon", "Nice"}},
	} {
		if got := cities(c.i, c.f); !slices.Equal(got, c.want) {
			t.Errorf("colonne %d, %+v : %q, attendu %q", c.i, *c.f, got, c.want)
		}
	}
	// Valeur invalide ou condition inconnue : refusées.
	for _, f := range []*ColumnFilter{cond("gt", "abc", ""), cond("top", "0", ""), cond("zz", "1", "")} {
		if err := d.SetFilter(ctx, "Feuille1", 1, f); err == nil {
			t.Errorf("%+v accepté", *f)
		}
	}
	if err := d.SetFilter(ctx, "Feuille1", 0, cond("above", "", "")); err == nil {
		t.Error("moyenne d'une colonne de texte acceptée")
	}
	if got := cities(1, nil); len(got) != 5 {
		t.Fatalf("sans filtre : %q", got)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("filtre resté :\n%s", diff(before, got))
	}
}

// Un filtre enregistré avant les conditions se relit tel quel.
func TestColumnFilterJSON(t *testing.T) {
	var f map[string]ColumnFilter
	if err := json.Unmarshal([]byte(`{"3":{"values":["FR"],"errors":true}}`), &f); err != nil {
		t.Fatal(err)
	}
	if g := f["3"]; !slices.Equal(g.Values, []string{"FR"}) || !g.Errors || g.Conds != nil || g.Or {
		t.Fatalf("%+v", g)
	}
	b, _ := json.Marshal(ColumnFilter{Values: []string{"FR"}})
	if string(b) != `{"values":["FR"]}` {
		t.Fatalf("%s", b)
	}
}
