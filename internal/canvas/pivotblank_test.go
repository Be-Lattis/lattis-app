package canvas

import (
	"context"
	"fmt"
	"testing"
)

// Les vides viennent toujours en dernier, comme « (Vides) » dans Excel :
// étiquettes des lignes et des colonnes d'un TCD, quel que soit le tri, et
// listes de valeurs des menus ▾ (tableau ou champ de TCD).
func TestPivotBlanksLast(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"Région", "Mois", "Ventes"},
		{"", "jan", "1"},
		{"Nord", "fév", "2"},
		{"Sud", "", "3"},
		{"", "fév", "4"},
		{"Est", "jan", "5"},
	}); err != nil {
		t.Fatal(err)
	}
	base := PivotConfig{Source: "Feuille1", RowFields: []string{"Région"}, Values: []PivotValue{{Field: "Ventes", Agg: "sum"}}}
	sheet, err := d.CreatePivot(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, sheet)
	id := l.Pivots[0].ID
	view := func(cfg PivotConfig) ([]string, []string) {
		t.Helper()
		if err := d.SetPivot(ctx, id, cfg); err != nil {
			t.Fatal(err)
		}
		l, _ := d.Layout(ctx, sheet)
		p := l.Pivots[0]
		if p.Error != "" {
			t.Fatal(p.Error)
		}
		var out []string
		for _, r := range p.Cells[1:] {
			out = append(out, r[0])
		}
		return p.Cells[0][1:], out
	}
	_, rows := view(base)
	// Libellé des vides tel que le TCD l'écrit (avant-dernière ligne).
	empty := rows[len(rows)-2]
	check := func(what string, got, want []string) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("%s : %q, voulu %q", what, got, want)
		}
	}
	check("ordre de la source", rows, []string{"Nord", "Sud", "Est", empty, "Total"})
	for dir, want := range map[string][]string{
		"asc":  {"Est", "Nord", "Sud", empty, "Total"},
		"desc": {"Sud", "Nord", "Est", empty, "Total"},
	} {
		cfg := base
		cfg.Sort = map[string]string{"Région": dir}
		_, rows := view(cfg)
		check("tri "+dir, rows, want)
	}
	// Deux champs en lignes : les vides en dernier dans chaque groupe.
	two := base
	two.RowFields = []string{"Région", "Mois"}
	two.Sort = map[string]string{"Mois": "desc"}
	_, rows = view(two)
	check("deux champs", rows, []string{"Nord", "fév", "Sud", empty, "Est", "jan", empty, "jan", "fév", "Total"})
	two.Sort = nil
	_, rows = view(two)
	check("deux champs sans tri", rows, []string{"Nord", "fév", "Sud", empty, "Est", "jan", empty, "jan", "fév", "Total"})
	// Champ en colonnes : la colonne des vides à droite (avant le total).
	cols := base
	cols.Cols = "Mois"
	for _, dir := range []string{"", "asc", "desc"} {
		cols.Sort = map[string]string{"Mois": dir}
		head, _ := view(cols)
		if len(head) < 4 || head[2] != empty {
			t.Fatalf("colonnes (%q) : %q", dir, head)
		}
	}
	// Menus ▾ : (Vides) en fin de liste.
	vals, err := d.FieldValues(ctx, "Feuille1", "Région")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(vals); n != 4 || vals[n-1].Text != "" || vals[n-1].Count != 2 {
		t.Fatalf("valeurs du champ : %+v", vals)
	}
	vals, _ = d.FilterValues(ctx, "Feuille1", 1)
	if n := len(vals); n != 3 || vals[n-1].Text != "" {
		t.Fatalf("valeurs de la colonne : %+v", vals)
	}
}
