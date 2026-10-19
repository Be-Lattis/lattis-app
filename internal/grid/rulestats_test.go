package grid

import (
	"context"
	"testing"
)

// Statistiques des règles de mise en forme conditionnelle, en SQL.
func TestRuleStats(t *testing.T) {
	tbl := openDemo(t, 101)
	ctx := context.Background()
	// Colonne 0 : 100000 à 100100.
	st, err := tbl.RuleStats(ctx, []int{0}, RuleStatsReq{Rank: 10, Pctls: []float64{0, 50, 25}})
	if err != nil {
		t.Fatal(err)
	}
	if st.Count != 101 || st.Nums != 101 || st.Min != 100000 || st.Max != 100100 || st.Avg != 100050 {
		t.Fatalf("base : %+v", st)
	}
	if st.Rank == nil || *st.Rank != 100091 {
		t.Fatalf("10 premiers : %v", st.Rank)
	}
	if len(st.Pctls) != 3 || st.Pctls[0] != 100000 || st.Pctls[1] != 100050 || st.Pctls[2] != 100025 {
		t.Fatalf("centiles : %v", st.Pctls)
	}
	if st.Std < 29 || st.Std > 30 {
		t.Fatalf("écart type : %v", st.Std)
	}
	low, err := tbl.RuleStats(ctx, []int{0}, RuleStatsReq{Rank: -10, Pct: true})
	if err != nil || low.Rank == nil || *low.Rank != 100009 {
		t.Fatalf("10 %% les plus bas : %v, %v", low.Rank, err)
	}
	// Doublons : la catégorie revient forcément sur 101 lignes ; le numéro jamais.
	d, err := tbl.RuleStats(ctx, []int{8, 0}, RuleStatsReq{Dups: true})
	if err != nil || len(d.Dups) == 0 {
		t.Fatalf("doublons : %v, %v", d.Dups, err)
	}
	for _, v := range d.Dups {
		if _, ok := v.(string); !ok {
			t.Fatalf("doublon inattendu : %v", v)
		}
	}
	// Dates : numéros de série d'Excel.
	dt, err := tbl.RuleStats(ctx, []int{5}, RuleStatsReq{})
	if err != nil || dt.Nums == 0 || dt.Min < 40000 || dt.Max > 60000 {
		t.Fatalf("dates : %+v, %v", dt, err)
	}
	// Cache : même demande, même résultat.
	again, _ := tbl.RuleStats(ctx, []int{0}, RuleStatsReq{Rank: 10, Pctls: []float64{0, 50, 25}})
	if *again.Rank != *st.Rank {
		t.Fatal("cache")
	}
}
