package canvas

import (
	"context"
	"math"
	"strings"
	"testing"
)

func approx(a, b, eps float64) bool { return math.Abs(a-b) <= eps*math.Max(1, math.Abs(b)) }

// Moindres carrés : droite, polynôme, exponentielle, logarithme, puissance,
// avec le R² d'un tableur (sur ln y pour exp et power).
func TestFitTrend(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	// y = 2x + 1 exactement : R² = 1.
	f := fitTrend(Trendline{Type: "linear"}, xs, []float64{3, 5, 7, 9, 11}, true)
	if f == nil || !approx(f.Coef[0], 1, 1e-9) || !approx(f.Coef[1], 2, 1e-9) || !approx(f.R2, 1, 1e-12) {
		t.Fatalf("droite : %+v", f)
	}
	// Données bruitées : pente et R² connus (calculés à la main).
	f = fitTrend(Trendline{Type: "linear"}, xs, []float64{2, 4, 5, 4, 5}, true)
	if !approx(f.Coef[1], 0.6, 1e-9) || !approx(f.Coef[0], 2.2, 1e-9) || !approx(f.R2, 0.6, 1e-9) {
		t.Fatalf("droite bruitée : %+v", f)
	}
	// Polynôme de degré 2 sur des dates (X vers 45 000) : bien conditionné.
	dx := []float64{45000, 45001, 45002, 45003, 45004, 45005}
	dy := make([]float64, len(dx))
	for i, x := range dx {
		u := x - 45000
		dy[i] = 3*u*u - 2*u + 7
	}
	f = fitTrend(Trendline{Type: "poly", Order: 2}, dx, dy, false)
	if f == nil || !approx(f.R2, 1, 1e-9) {
		t.Fatalf("polynôme : %+v", f)
	}
	if got := polyEval(f.Coef, 45010); !approx(got, 3*100-20+7, 1e-5) {
		t.Fatalf("polynôme en 45010 : %v", got)
	}
	// Degré 6 : sept points suffisent, six non.
	if fitTrend(Trendline{Type: "poly", Order: 6}, xs, []float64{1, 2, 3, 4, 5}, true) != nil {
		t.Fatal("degré 6 sur 5 points accepté")
	}
	// y = 2·e^(0,5x).
	ey := make([]float64, len(xs))
	for i, x := range xs {
		ey[i] = 2 * math.Exp(0.5*x)
	}
	f = fitTrend(Trendline{Type: "exp"}, xs, ey, true)
	if !approx(f.Coef[0], 2, 1e-9) || !approx(f.Coef[1], 0.5, 1e-9) || !approx(f.R2, 1, 1e-12) {
		t.Fatalf("exponentielle : %+v", f)
	}
	if fitTrend(Trendline{Type: "exp"}, xs, []float64{1, -2, 3, 4, 5}, true) != nil {
		t.Fatal("exponentielle sur une valeur négative")
	}
	// y = 3·ln(x) + 1.
	ly := make([]float64, len(xs))
	for i, x := range xs {
		ly[i] = 3*math.Log(x) + 1
	}
	f = fitTrend(Trendline{Type: "log"}, xs, ly, true)
	if !approx(f.Coef[0], 3, 1e-9) || !approx(f.Coef[1], 1, 1e-9) {
		t.Fatalf("logarithme : %+v", f)
	}
	// y = 4·x^1,5.
	py := make([]float64, len(xs))
	for i, x := range xs {
		py[i] = 4 * math.Pow(x, 1.5)
	}
	f = fitTrend(Trendline{Type: "power"}, xs, py, true)
	if !approx(f.Coef[0], 4, 1e-9) || !approx(f.Coef[1], 1.5, 1e-9) {
		t.Fatalf("puissance : %+v", f)
	}
	// Prévision : la courbe va de 1 − 1 à 5 + 2.
	f = fitTrend(Trendline{Type: "linear", Forward: 2, Backward: 1}, xs, []float64{3, 5, 7, 9, 11}, true)
	if p := f.Points; len(p) != 2 || p[0][0] != 0 || p[1][0] != 7 || !approx(p[1][1], 15, 1e-9) {
		t.Fatalf("prévision : %+v", f.Points)
	}
}

// Moyenne mobile : moyenne des n derniers points, posée sur le dernier.
func TestMovingAverage(t *testing.T) {
	f := fitTrend(Trendline{Type: "movavg", Period: 3}, []float64{1, 2, 3, 4, 5}, []float64{3, 6, 9, 3, 6}, true)
	want := [][2]float64{{3, 6}, {4, 6}, {5, 6}}
	if f == nil || len(f.Points) != 3 {
		t.Fatalf("moyenne mobile : %+v", f)
	}
	for i, p := range want {
		if !approx(f.Points[i][0], p[0], 1e-12) || !approx(f.Points[i][1], p[1], 1e-12) {
			t.Fatalf("point %d : %v, attendu %v", i, f.Points[i], p)
		}
	}
	// Nuage : points pris dans l'ordre de X.
	f = fitTrend(Trendline{Type: "movavg", Period: 2}, []float64{3, 1, 2}, []float64{30, 10, 20}, false)
	if f.Points[0] != [2]float64{2, 15} || f.Points[1] != [2]float64{3, 25} {
		t.Fatalf("ordre de X : %+v", f.Points)
	}
	if fitTrend(Trendline{Type: "movavg", Period: 4}, []float64{1, 2}, []float64{1, 2}, true) != nil {
		t.Fatal("période plus longue que la série")
	}
}

func fp(v float64) *float64 { return &v }

// Barres d'erreur : valeur fixe, pourcentage, écart type (autour de la
// moyenne), erreur type, personnalisées ; sens plus ou moins.
func TestErrorValues(t *testing.T) {
	vs := []*float64{fp(2), fp(4), nil, fp(4), fp(5), fp(5), fp(7), fp(9)}
	got := func(e *ErrorValues, i int) (float64, float64) {
		var p, m float64 = -1, -1
		if e.Plus[i] != nil {
			p = *e.Plus[i]
		}
		if e.Minus[i] != nil {
			m = *e.Minus[i]
		}
		return p, m
	}
	e := errorValues(ErrorBars{Type: "fixed", Value: 1.5}, vs, nil, nil)
	if p, m := got(e, 0); p != 1.5 || m != 1.5 || e.Plus[2] != nil {
		t.Fatalf("fixe : %v %v", p, m)
	}
	e = errorValues(ErrorBars{Type: "percent", Value: 10, Dir: "plus"}, vs, nil, nil)
	if p, m := got(e, 7); !approx(p, 0.9, 1e-12) || m != -1 {
		t.Fatalf("pourcentage : %v %v", p, m)
	}
	// 2, 4, 4, 5, 5, 7, 9 : moyenne 36/7, écart type (n − 1) √(32,857…/6).
	mean := 36.0 / 7
	ss := 0.0
	for _, v := range []float64{2, 4, 4, 5, 5, 7, 9} {
		ss += (v - mean) * (v - mean)
	}
	sd := math.Sqrt(ss / 6)
	e = errorValues(ErrorBars{Type: "stddev", Value: 2}, vs, nil, nil)
	if p, _ := got(e, 1); !approx(p, 2*sd, 1e-12) || e.Center == nil || !approx(*e.Center, mean, 1e-12) {
		t.Fatalf("écart type : %v (centre %v)", p, e.Center)
	}
	e = errorValues(ErrorBars{Type: "stderr", Dir: "minus"}, vs, nil, nil)
	if p, m := got(e, 1); p != -1 || !approx(m, sd/math.Sqrt(7), 1e-12) || e.Center != nil {
		t.Fatalf("erreur type : %v %v", p, m)
	}
	e = errorValues(ErrorBars{Type: "custom"}, vs[:2], []*float64{fp(1), fp(2)}, []*float64{fp(3), nil})
	if p, m := got(e, 0); p != 1 || m != 3 {
		t.Fatalf("personnalisée : %v %v", p, m)
	}
	if p, m := got(e, 1); p != 2 || m != -1 {
		t.Fatalf("personnalisée, moins vide : %v %v", p, m)
	}
}

// Validation de la configuration : types, ordre, période, prévision, sens,
// bulles par paires, graphique boursier de 3 à 5 séries.
func TestChartFitConfig(t *testing.T) {
	base := ChartConfig{Source: "T", Kind: "line", X: "x", Series: []string{"a"}}
	for _, tc := range []struct {
		edit func(*ChartConfig)
		bad  string
	}{
		{func(c *ChartConfig) { c.Trendlines = []*Trendline{{Type: "linear", Equation: true, R2: true}} }, ""},
		{func(c *ChartConfig) { c.Trendlines = []*Trendline{{Type: "spline"}} }, "inconnue"},
		{func(c *ChartConfig) { c.Trendlines = []*Trendline{{Type: "poly", Order: 7}} }, "entre 2 et 6"},
		{func(c *ChartConfig) { c.Trendlines = []*Trendline{{Type: "movavg", Period: 1}} }, "entre 2 et 255"},
		{func(c *ChartConfig) { c.Trendlines = []*Trendline{{Type: "linear", Forward: -1}} }, "prévision"},
		{func(c *ChartConfig) { c.ErrorBars = []*ErrorBars{nil, {Type: "percent", Value: 5, Dir: "plus"}} }, ""},
		{func(c *ChartConfig) { c.ErrorBars = []*ErrorBars{{Type: "range"}} }, "inconnues"},
		{func(c *ChartConfig) { c.ErrorBars = []*ErrorBars{{Type: "fixed", Dir: "up"}} }, "sens"},
		{func(c *ChartConfig) { c.ErrorBars = []*ErrorBars{{Type: "fixed", Value: -1}} }, "positive"},
		{func(c *ChartConfig) { c.ErrorBars = []*ErrorBars{{Type: "custom"}} }, "colonne"},
		{func(c *ChartConfig) { c.Kind, c.Series = "bubble", []string{"a", "b", "c"} }, "paires"},
		{func(c *ChartConfig) { c.Kind, c.Series = "bubble", []string{"a", "b"} }, ""},
		{func(c *ChartConfig) { c.Kind, c.Series = "stock", []string{"a", "b"} }, "boursier"},
		{func(c *ChartConfig) { c.Kind, c.Series = "stock", []string{"a", "b", "c", "d", "e"} }, ""},
	} {
		cfg := base
		tc.edit(&cfg)
		err := checkChart(cfg)
		switch {
		case tc.bad == "" && err != nil:
			t.Errorf("%+v : %v", cfg, err)
		case tc.bad != "" && (err == nil || !strings.Contains(err.Error(), tc.bad)):
			t.Errorf("%+v : %v, attendu « %s »", cfg, err, tc.bad)
		}
	}
}

// Les données d'un graphique portent ses courbes de tendance et ses barres
// d'erreur, personnalisées comprises (colonne du tableau) ; nuage de bulles
// et boursier lisent les lignes une à une.
func TestChartFitsData(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"Mois", "Ventes", "Marge", "Taille"},
		{"Jan", "3", "1", "10"}, {"Fév", "5", "2", "20"}, {"Mar", "7", "1", "30"}, {"Avr", "9", "0.5", "40"},
	}); err != nil {
		t.Fatal(err)
	}
	cfg := ChartConfig{Source: "Feuille1", Kind: "bar", X: "Mois", Series: []string{"Ventes"},
		Trendlines: []*Trendline{{Type: "linear", Forward: 1, Equation: true}},
		ErrorBars:  []*ErrorBars{{Type: "custom", Plus: "Marge"}}}
	id, err := d.CreateChart(ctx, 1, 0, 6, cfg)
	if err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	ch := l.Charts[0]
	if ch.ID != id || len(ch.Trends) != 1 || ch.Trends[0] == nil || !ch.Trends[0].Index {
		t.Fatalf("tendance : %+v", ch.Trends)
	}
	tr := ch.Trends[0]
	if !approx(tr.Coef[1], 2, 1e-9) || !approx(tr.Coef[0], 1, 1e-9) || tr.Points[len(tr.Points)-1][0] != 5 {
		t.Fatalf("droite : %+v", tr)
	}
	if len(ch.Errors) != 1 || *ch.Errors[0].Plus[1] != 2 || *ch.Errors[0].Minus[3] != 0.5 {
		t.Fatalf("barres d'erreur : %+v", ch.Errors)
	}
	// Bulles : X numérique, paires (Y, taille), lignes une à une.
	if _, err := d.CreateChart(ctx, 1, 20, 6, ChartConfig{Source: "Feuille1", Kind: "bubble", X: "Ventes", Series: []string{"Marge", "Taille"}}); err != nil {
		t.Fatal(err)
	}
	// Boursier : refusé sans assez de séries.
	if _, err := d.CreateChart(ctx, 1, 40, 6, ChartConfig{Source: "Feuille1", Kind: "stock", X: "Mois", Series: []string{"Ventes"}}); err == nil {
		t.Fatal("boursier à une série accepté")
	}
	if _, err := d.CreateChart(ctx, 1, 40, 6, ChartConfig{Source: "Feuille1", Kind: "stock", X: "Mois", Series: []string{"Ventes", "Taille", "Marge"}}); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, 1)
	if b := l.Charts[1]; b.Error != "" || len(b.Labels) != 4 || len(b.Series) != 2 {
		t.Fatalf("bulles : %+v", b)
	}
	if s := l.Charts[2]; s.Error != "" || len(s.Labels) != 4 {
		t.Fatalf("boursier : %+v", s)
	}
	// Renommer la colonne des barres personnalisées suit.
	if _, err := d.SetCell(ctx, 1, 0, 2, "Écart"); err != nil {
		t.Fatal(err)
	}
	_, _, got, err := d.ChartPlace(ctx, id)
	if err != nil || got.ErrorBars[0].Plus != "Écart" {
		t.Fatalf("colonne renommée : %+v (%v)", got.ErrorBars[0], err)
	}
}
