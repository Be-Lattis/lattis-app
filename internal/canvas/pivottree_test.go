package canvas

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TCD avec deux champs en lignes : arborescence façon Excel (sous-totaux,
// groupes pliés), enregistrée dans la configuration et annulable ; la vue
// SQL et les graphiques le lisent toujours à plat.
func TestPivotTree(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Service", "Poste", "Salaire"}, {"RH", "Chargé", "30"}, {"IT", "Dev", "50"}, {"RH", "Chef", "60"}, {"IT", "Dev", "40"}, {"IT", "Admin", "35"}}); err != nil {
		t.Fatal(err)
	}
	base := PivotConfig{Source: "Feuille1", RowFields: []string{"Service", "Poste"}, Values: []PivotValue{{Field: "Salaire", Agg: "sum"}, {Field: "Salaire", Agg: "avg"}}}
	sheet, err := d.CreatePivot(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, sheet)
	id := l.Pivots[0].ID
	view := func() (string, string) {
		l, _ := d.Layout(ctx, sheet)
		p := l.Pivots[0]
		if p.Error != "" {
			t.Fatal(p.Error)
		}
		if len(p.Rows) != len(p.Cells)-1 {
			t.Fatalf("%d descriptions pour %d lignes", len(p.Rows), len(p.Cells)-1)
		}
		var rows []string
		for i, r := range p.Rows {
			k := fmt.Sprint(r.Depth)
			switch {
			case r.Total:
				k = "T"
			case r.Subtotal:
				k += "s"
			case r.Collapsed:
				k += "+"
			case r.Group:
				k += "-"
			}
			rows = append(rows, k+":"+strings.Join(p.Cells[i+1], "|"))
		}
		return fmt.Sprint(p.Cells[0]), strings.Join(rows, " ")
	}
	set := func(cfg PivotConfig) {
		t.Helper()
		if err := d.SetPivot(ctx, id, cfg); err != nil {
			t.Fatal(err)
		}
	}
	head, got := view()
	if head != "[Étiquettes de lignes Somme de Salaire Moyenne de Salaire]" {
		t.Fatalf("en-tête : %s", head)
	}
	// Sous-totaux en haut (par défaut) : moyenne du groupe, pas des moyennes.
	if want := "0-:RH|90|45 1:Chargé|30|30 1:Chef|60|60 0-:IT|125|41,6666666666667 1:Dev|90|45 1:Admin|35|35 T:Total|215|43"; got != want {
		t.Fatalf("sous-totaux en haut :\n%s\n%s", got, want)
	}
	l, _ = d.Layout(ctx, sheet)
	if f := l.Pivots[0].Formats; len(f) != 3 || f[0] != "" || f[2] != `{"dp":2}` {
		t.Fatalf("formats : %q", f)
	}
	before := dump(t, d)
	// Plier RH : son sous-total reste sur sa ligne.
	cfg := base
	cfg.Collapsed = [][]string{{"RH"}}
	set(cfg)
	if _, got := view(); got != "0+:RH|90|45 0-:IT|125|41,6666666666667 1:Dev|90|45 1:Admin|35|35 T:Total|215|43" {
		t.Fatalf("RH plié : %s", got)
	}
	// Annuler : de nouveau déplié.
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
	if _, got := view(); !strings.HasPrefix(got, "0-:RH|90|45 1:Chargé") {
		t.Fatalf("après annulation : %s", got)
	}
	// Réduire tout, sauf IT déplié ; sous-totaux en bas (le groupe plié garde
	// le sien sur sa ligne).
	cfg = base
	cfg.CollapseAll, cfg.Expanded, cfg.Subtotals = true, [][]string{{"IT"}}, SubtotalsBottom
	set(cfg)
	if _, got := view(); got != "0+:RH|90|45 0-:IT|| 1:Dev|90|45 1:Admin|35|35 0s:Total IT|125|41,6666666666667 T:Total|215|43" {
		t.Fatalf("réduire tout, en bas : %s", got)
	}
	// Sans sous-totaux, sans total général.
	cfg = base
	cfg.Subtotals, cfg.NoGrandTotal = SubtotalsNone, true
	set(cfg)
	if _, got := view(); got != "0-:RH|| 1:Chargé|30|30 1:Chef|60|60 0-:IT|| 1:Dev|90|45 1:Admin|35|35" {
		t.Fatalf("sans sous-totaux : %s", got)
	}
	// Valeurs gardées d'un champ profond : le sous-total ne compte qu'elles ;
	// pourcentage du total sur les groupes ; trois niveaux.
	cfg = base
	cfg.Values = []PivotValue{{Field: "Salaire", Agg: "sum", Show: "pctTotal"}}
	cfg.Items = []PivotFilter{{Field: "Poste", Values: []string{"Dev", "Chef"}}}
	set(cfg)
	if _, got := view(); got != "0-:IT|60 % 1:Dev|60 % 0-:RH|40 % 1:Chef|40 % T:Total|100 %" {
		t.Fatalf("valeurs gardées, %% du total : %s", got)
	}
	cfg = base
	cfg.RowFields = []string{"Service", "Poste", "Salaire"}
	cfg.Values = []PivotValue{{Agg: "count"}}
	cfg.Collapsed = [][]string{{"IT", "Dev"}}
	set(cfg)
	if _, got := view(); got != "0-:RH|2 1-:Chargé|1 2:30|1 1-:Chef|1 2:60|1 0-:IT|3 1+:Dev|2 1-:Admin|1 2:35|1 T:Total|5" {
		t.Fatalf("trois niveaux : %s", got)
	}
	// Un seul champ en lignes : à plat, sans description.
	cfg = base
	cfg.RowFields = []string{"Service"}
	set(cfg)
	l, _ = d.Layout(ctx, sheet)
	if p := l.Pivots[0]; p.Rows != nil || fmt.Sprint(p.Cells) != "[[Service Somme de Salaire Moyenne de Salaire] [RH 90 45] [IT 125 41,6666666666667] [Total 215 43]]" {
		t.Fatalf("un seul champ : %v %v", p.Cells, p.Rows)
	}
	// Vue SQL et graphique : à plat, sans sous-totaux, plié ou non.
	cfg = base
	cfg.CollapseAll = true
	set(cfg)
	q, err := d.PivotCTE(ctx, `SELECT * FROM "TCD1"`)
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	d.read(ctx, func(tx *tx) error {
		rs, err := tx.query(q)
		if err != nil {
			t.Fatalf("%s : %v", q, err)
		}
		defer rs.Close()
		cols, _ := rs.Columns()
		rows = append(rows, strings.Join(cols, "|"))
		for rs.Next() {
			var a, b string
			var s, m float64
			rs.Scan(&a, &b, &s, &m)
			rows = append(rows, fmt.Sprint(a, " ", b, " ", s))
		}
		return nil
	})
	if got := strings.Join(rows, ", "); got != "Service|Poste|Somme de Salaire|Moyenne de Salaire, RH Chargé 30, IT Dev 90, RH Chef 60, IT Admin 35" {
		t.Fatalf("vue SQL : %s", got)
	}
	c := d.ChartData(ctx, ChartConfig{Source: "TCD1", Kind: "bar", X: "Service", Series: []string{"Somme de Salaire"}})
	if c.Error != "" || fmt.Sprint(c.Labels) != "[RH · Chargé IT · Dev RH · Chef IT · Admin]" || len(c.Series[0].Values) != 4 || *c.Series[0].Values[1] != 90 {
		t.Fatalf("graphique : %+v", c)
	}
	// Document : même arborescence sans feuille.
	p := d.PivotData(ctx, cfg)
	if p.Error != "" || len(p.Rows) != 3 || !p.Rows[0].Collapsed || fmt.Sprint(p.Cells[1]) != "[RH 90 45]" {
		t.Fatalf("PivotData : %+v", p)
	}
}
