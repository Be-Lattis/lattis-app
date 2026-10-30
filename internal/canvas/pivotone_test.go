package canvas

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// TestPivotOnePassSame : le calcul d'un TCD en une passe (pivotone.go) donne
// exactement ce que donne le calcul en plusieurs passes, sur des TCD tirés
// au hasard : plusieurs champs en lignes et en colonnes, sous-totaux,
// totaux masqués, tris et ordres personnalisés, « Afficher les valeurs »,
// filtres, étiquettes gardées, segments, groupes de dates et de nombres,
// champs calculés ; valeurs mêlées (vides, textes dans une colonne de
// nombres, réels à deux décimales, casse différente).
func TestPivotOnePassSame(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	rnd := rand.New(rand.NewSource(30102026))
	regions := []string{"Nord", "Sud", "Est", "nord", "", "Ouest"}
	produits := []string{"A", "B", "C", "D", "", "b"}
	villes := []string{"Lyon", "Paris", "Nice", "Brest", "Lille", "Rome", "Milan"}
	mix := []any{"12", "abc", nil, "3,5", "Zèbre", "zèbre", "7"}
	const n = 600
	if _, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{{
		Name:    "Ventes",
		Headers: []string{"Région", "Produit", "Ville", "Année", "Date", "Montant", "Qté", "Mix", "Remise"},
		Types:   []grid.Type{grid.Text, grid.Text, grid.Text, grid.Integer, grid.Date, grid.Decimal, grid.Integer, grid.Text, grid.Decimal},
		Count:   n,
		Rows: func(yield func([]any) error) error {
			for range n {
				var montant, qte, remise any
				if rnd.Intn(12) > 0 {
					montant = float64(rnd.Intn(200000)-20000) / 100
				}
				if rnd.Intn(10) > 0 {
					qte = int64(rnd.Intn(40) - 5)
				}
				if rnd.Intn(4) > 0 {
					remise = float64(rnd.Intn(30000)) / 10000
				}
				row := []any{regions[rnd.Intn(len(regions))], produits[rnd.Intn(len(produits))], villes[rnd.Intn(len(villes))],
					int64(2022 + rnd.Intn(4)), fmt.Sprintf("%d-%02d-%02d", 2022+rnd.Intn(4), 1+rnd.Intn(12), 1+rnd.Intn(28)),
					montant, qte, mix[rnd.Intn(len(mix))], remise}
				if err := yield(row); err != nil {
					return err
				}
			}
			return nil
		},
	}}}}); err != nil {
		t.Fatal(err)
	}
	start, end := 0.0, 1500.0
	groups := [][]PivotGroup{
		nil,
		{{Field: "Date", By: []string{"years", "months"}}},
		{{Field: "Date", By: []string{"quarters"}}},
		{{Field: "Montant", Start: &start, End: &end, Step: 250}},
	}
	calc := []PivotCalcField{{Name: "Initiale", Formula: "=LEFT([Ville],1)"}, {Name: "Double", Formula: "=[Qté]*2"}}
	fieldsFor := func(g []PivotGroup) []string {
		f := []string{"Région", "Produit", "Ville", "Année", "Initiale", "Mix"}
		if len(g) > 0 && g[0].Field == "Date" {
			if len(g[0].By) == 2 {
				f = append(f, "Années", "Date")
			} else {
				f = append(f, "Date")
			}
		} else if len(g) > 0 {
			f = append(f, "Montant")
		}
		return f
	}
	aggs := []string{"sum", "count", "avg", "min", "max", "sum", "count"}
	valueFields := []string{"Montant", "Qté", "Mix", "Remise", "Double", "", "Région"}
	shows := []string{"", "", "", "pctTotal", "pctCol", "pctRow", "pctParentRow", "pctParentCol", "pctParent", "diff", "pctDiff", "runTotal", "pctRunTotal", "rankAsc", "rankDesc", "index", "pctOf"}
	pick := func(pool []string, k int) []string {
		perm := rnd.Perm(len(pool))
		out := []string{}
		for _, i := range perm[:k] {
			out = append(out, pool[i])
		}
		return out
	}
	one, multi := 0, 0
	for iter := range 300 {
		g := groups[rnd.Intn(len(groups))]
		pool := fieldsFor(g)
		nr, nc := rnd.Intn(4), rnd.Intn(3)
		if nr+nc > len(pool) {
			nc = 0
		}
		fields := pick(pool, nr+nc)
		cfg := PivotConfig{Source: "Ventes", RowFields: fields[:nr], ColFields: fields[nr:], Groups: g, Calc: calc}
		for range 1 + rnd.Intn(3) {
			v := PivotValue{Agg: aggs[rnd.Intn(len(aggs))], Field: valueFields[rnd.Intn(len(valueFields))]}
			if v.Field == "" {
				v.Agg = "count"
			}
			if rnd.Intn(3) == 0 {
				v.Show = shows[rnd.Intn(len(shows))]
				if all := append(append([]string{}, cfg.RowFields...), cfg.ColFields...); len(all) > 0 {
					v.BaseField = all[rnd.Intn(len(all))]
				}
				switch rnd.Intn(3) {
				case 0:
					v.BasePos = "prev"
				case 1:
					v.BaseItem = "Nord"
				}
			}
			cfg.Values = append(cfg.Values, v)
		}
		if rnd.Intn(8) == 0 {
			cfg.Values = append(cfg.Values, PivotValue{Field: "Ville", Agg: "countDistinct"})
		}
		switch rnd.Intn(4) {
		case 0:
			cfg.Subtotals = SubtotalsNone
		case 1:
			cfg.Subtotals = "bottom"
		}
		cfg.NoGrandTotal = rnd.Intn(4) == 0
		if rnd.Intn(4) == 0 && len(fields) > 0 {
			cfg.Sort = map[string]string{fields[rnd.Intn(len(fields))]: []string{"asc", "desc"}[rnd.Intn(2)]}
		}
		if rnd.Intn(8) == 0 && len(fields) > 0 {
			cfg.Orders = map[string]CustomOrder{fields[0]: {Values: []string{"Sud", "C", "Paris", "2024"}}}
		}
		if rnd.Intn(4) == 0 {
			cfg.Filters = []PivotFilter{{Field: "Ville", Values: pick(villes, 1+rnd.Intn(4))}}
		}
		if rnd.Intn(5) == 0 && len(fields) > 0 && fields[0] == "Région" {
			cfg.Items = []PivotFilter{{Field: "Région", Values: []string{"Nord", "Sud", ""}}}
		}
		if rnd.Intn(5) == 0 {
			cfg.Slicers = []PivotSlicer{{Field: "Produit", Values: []string{"A", "B", ""}}}
		}
		if rnd.Intn(6) == 0 {
			cfg.CollapseAll = true
		}
		if rnd.Intn(10) == 0 {
			cfg.Filters = append(cfg.Filters, PivotFilter{Field: "Ville", Values: []string{"Nulle part"}})
		}
		got := map[bool]string{}
		for _, on := range []bool{true, false} {
			pivotOnePass = on
			d.cacheMu.Lock()
			d.cache, d.depCache = nil, nil
			d.cacheMu.Unlock()
			var out string
			d.read(ctx, func(x *tx) error {
				cb, err := x.pivotCubeRaw(cfg)
				if err != nil {
					out = "erreur : " + err.Error()
					return nil
				}
				v, err := x.pivotViewRaw(cfg)
				flat, err2 := x.pivotCellsRaw(cfg)
				out = fmt.Sprintf("cube %d×%d\nlignes %q\ncolonnes %q\nenfants %q\ncellules %s\nvue %q %+v %q %+v %v\nà plat %q %v",
					cb.nr, cb.nc, cb.rows, cb.cols, cb.kids, cubeText(cb), v.cells, v.rows, v.colHead, v.cols, err, flat, err2)
				return nil
			})
			got[on] = out
		}
		pivotOnePass = true
		if got[true] != got[false] {
			t.Fatalf("TCD %d %+v :\nune passe :\n%s\nplusieurs passes :\n%s", iter, cfg, got[true], got[false])
		}
		p, _ := func() (pivotPlan, error) {
			var p pivotPlan
			err := d.read(ctx, func(x *tx) error { var err error; p, err = x.pivotPlan(cfg); return err })
			return p, err
		}()
		if pivotOneFits(p, &pivotCube{nr: len(cfg.RowFields), nc: len(cfg.ColFields), vals: cfg.Values}) {
			one++
		} else {
			multi++
		}
	}
	if one < 150 {
		t.Fatalf("trop peu de TCD calculés en une passe : %d (plusieurs : %d)", one, multi)
	}
	t.Logf("%d TCD en une passe, %d en plusieurs", one, multi)
}

// cubeText écrit les cellules du cube, rangées, avec le type de chaque
// agrégat (un entier reste un entier).
func cubeText(cb *pivotCube) string {
	keys := make([]string, 0, len(cb.cells))
	for k := range cb.cells {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	out := ""
	for _, k := range keys {
		out += strconv.Quote(k) + "="
		for _, v := range cb.cells[k] {
			if f, ok := v.(float64); ok {
				// Un réel au chiffre près de ce qui s'affiche (15 chiffres) :
				// les sommes réelles des niveaux du haut sont des sommes de
				// sommes (au plus un dernier bit d'écart).
				v = strconv.FormatFloat(f, 'g', 15, 64)
			}
			out += fmt.Sprintf("%T(%v) ", v, v)
		}
		out += "; "
	}
	return out
}
