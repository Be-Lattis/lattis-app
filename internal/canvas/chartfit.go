package canvas

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// Courbes de tendance et barres d'erreur des graphiques, comme un tableur :
// calculées par le classeur sur les points lus (au plus MaxChartPoints),
// hors du cache des données (le calcul est court).

// Trendline : courbe de tendance d'une série.
type Trendline struct {
	// Type : linear, exp (exponentielle), log (logarithmique), poly
	// (polynomiale, Order de 2 à 6), power (puissance) ou movavg (moyenne
	// mobile sur Period points).
	Type   string `json:"type"`
	Order  int    `json:"order,omitempty"`
	Period int    `json:"period,omitempty"`
	// Forward, Backward : prévision après et avant les points, en unités de
	// X (catégories pour un axe de catégories).
	Forward  float64 `json:"forward,omitempty"`
	Backward float64 `json:"backward,omitempty"`
	// Equation, R2 : affichés sur le graphique.
	Equation bool `json:"equation,omitempty"`
	R2       bool `json:"r2,omitempty"`
}

// ErrorBars : barres d'erreur d'une série.
type ErrorBars struct {
	// Type : fixed (valeur fixe Value), percent (Value % de chaque valeur),
	// stddev (Value écarts types, 1 par défaut, autour de la moyenne de la
	// série), stderr (erreur type) ou custom (montants lus dans les colonnes
	// Plus et Minus ; Minus vide : ceux de Plus).
	Type  string  `json:"type"`
	Value float64 `json:"value,omitempty"`
	Plus  string  `json:"plus,omitempty"`
	Minus string  `json:"minus,omitempty"`
	// Dir : both (défaut), plus ou minus ; NoCap : sans embouts.
	Dir   string `json:"dir,omitempty"`
	NoCap bool   `json:"noCap,omitempty"`
}

// TrendFit : courbe de tendance calculée. Coef : linear et poly, c0 + c1·x
// + c2·x² … ; exp, y = c0·e^(c1·x) ; log, y = c0·ln(x) + c1 ; power,
// y = c0·x^c1 ; movavg, aucun. R2 : coefficient de détermination (sur
// ln y pour exp et power, comme un tableur). Points : la courbe à tracer
// (x, y), prévision comprise ; Index : x est le rang de la catégorie (1,
// 2…), pas une valeur de X.
type TrendFit struct {
	Type   string       `json:"type"`
	Coef   []float64    `json:"coef"`
	R2     float64      `json:"r2"`
	Points [][2]float64 `json:"points"`
	Index  bool         `json:"index,omitempty"`
}

// ErrorValues : barres d'erreur calculées, montant au-dessus (Plus) et
// au-dessous (Minus) de chaque point (nil : pas de barre) ; Center :
// barres centrées sur cette valeur (écart type : la moyenne de la série).
type ErrorValues struct {
	Plus   []*float64 `json:"plus"`
	Minus  []*float64 `json:"minus"`
	Center *float64   `json:"center,omitempty"`
}

var trendTypes = map[string]bool{"linear": true, "exp": true, "log": true, "poly": true, "power": true, "movavg": true}
var errorTypes = map[string]bool{"fixed": true, "percent": true, "stddev": true, "stderr": true, "custom": true}

// chartFitKinds : types qui acceptent courbes de tendance et barres d'erreur.
var chartFitKinds = map[string]bool{"bar": true, "line": true, "area": true, "scatter": true, "combo": true, "bubble": true}

// checkFits vérifie courbes de tendance et barres d'erreur.
func checkFits(cfg ChartConfig) error {
	for _, tl := range cfg.Trendlines {
		if tl == nil {
			continue
		}
		if !trendTypes[tl.Type] {
			return msg.Wrap(ErrChart, "chart.unknownTrend", msg.Args{"type": fmt.Sprintf("%q", tl.Type)})
		}
		if tl.Type == "poly" && (tl.Order < 2 || tl.Order > 6) {
			return msg.Wrap(ErrChart, "chart.trendOrder", nil)
		}
		if tl.Type == "movavg" && (tl.Period < 2 || tl.Period > 255) {
			return msg.Wrap(ErrChart, "chart.trendPeriod", nil)
		}
		if tl.Forward < 0 || tl.Backward < 0 || math.IsNaN(tl.Forward+tl.Backward) || math.IsInf(tl.Forward+tl.Backward, 0) {
			return msg.Wrap(ErrChart, "chart.trendForecast", nil)
		}
	}
	for _, eb := range cfg.ErrorBars {
		if eb == nil {
			continue
		}
		if !errorTypes[eb.Type] {
			return msg.Wrap(ErrChart, "chart.unknownErrorBars", msg.Args{"type": fmt.Sprintf("%q", eb.Type)})
		}
		if eb.Dir != "" && eb.Dir != "both" && eb.Dir != "plus" && eb.Dir != "minus" {
			return msg.Wrap(ErrChart, "chart.unknownErrorDir", msg.Args{"dir": fmt.Sprintf("%q", eb.Dir)})
		}
		if eb.Value < 0 || math.IsNaN(eb.Value) || math.IsInf(eb.Value, 0) {
			return msg.Wrap(ErrChart, "chart.errorValue", nil)
		}
		if eb.Type == "custom" && eb.Plus == "" {
			return msg.Wrap(ErrChart, "chart.errorColumn", nil)
		}
	}
	return nil
}

// chartFits calcule les courbes de tendance et les barres d'erreur d'un
// graphique sur ses données (r), séries dans l'ordre de cfg.Series.
func (t *tx) chartFits(cfg ChartConfig, r *chartResult) error {
	r.trends, r.errors = nil, nil
	if !chartFitKinds[cfg.Kind] || cfg.Stacked || len(r.series) == 0 {
		return nil
	}
	if slices.ContainsFunc(cfg.Trendlines, func(tl *Trendline) bool { return tl != nil }) {
		xs, index := trendX(cfg, *r)
		r.trends = make([]*TrendFit, len(r.series))
		for k, tl := range cfg.Trendlines {
			if tl == nil || k >= len(r.series) || (cfg.Kind == "bubble" && k%2 == 1) {
				continue
			}
			var px, py []float64
			for i, v := range r.series[k].Values {
				if v != nil && i < len(xs) && xs[i] != nil {
					px, py = append(px, *xs[i]), append(py, *v)
				}
			}
			if f := fitTrend(*tl, px, py, index); f != nil {
				r.trends[k] = f
			}
		}
	}
	if !slices.ContainsFunc(cfg.ErrorBars, func(eb *ErrorBars) bool { return eb != nil }) {
		return nil
	}
	custom, err := t.errorColumns(cfg, *r)
	if err != nil {
		return err
	}
	r.errors = make([]*ErrorValues, len(r.series))
	for k, eb := range cfg.ErrorBars {
		if eb == nil || k >= len(r.series) || (cfg.Kind == "bubble" && k%2 == 1) {
			continue
		}
		var plus, minus []*float64
		if eb.Type == "custom" {
			plus = custom[strings.ToLower(eb.Plus)]
			minus = plus
			if eb.Minus != "" {
				minus = custom[strings.ToLower(eb.Minus)]
			}
			if plus == nil || minus == nil {
				continue // colonnes illisibles (courbe réduite) : pas de barres
			}
		}
		r.errors[k] = errorValues(*eb, r.series[k].Values, plus, minus)
	}
	return nil
}

// errorColumns lit les colonnes des barres d'erreur personnalisées, alignées
// sur les points du graphique (mêmes lignes, ou mêmes catégories
// additionnées) ; rien pour une courbe réduite (d'autres lignes y sont
// gardées).
func (t *tx) errorColumns(cfg ChartConfig, r chartResult) (map[string][]*float64, error) {
	var cols []string
	seen := map[string]bool{}
	for _, eb := range cfg.ErrorBars {
		if eb == nil || eb.Type != "custom" {
			continue
		}
		for _, c := range []string{eb.Plus, eb.Minus} {
			if c != "" && !seen[strings.ToLower(c)] {
				seen[strings.ToLower(c)] = true
				cols = append(cols, c)
			}
		}
	}
	if len(cols) == 0 || r.sampled {
		return nil, nil
	}
	// Même lecture que le graphique : lignes une à une (nuage, bulles) ou
	// regroupées par catégorie (barres, courbes : sans réduction ici).
	kind := "bar"
	if cfg.Kind == "scatter" || cfg.Kind == "bubble" {
		kind = "scatter"
	}
	d := ChartConfig{Source: cfg.Source, Kind: kind, X: cfg.X, Series: cols, Hidden: cfg.Hidden, Filters: cfg.Filters}
	res, err := t.chartData(d)
	if err != nil {
		return nil, err
	}
	out := map[string][]*float64{}
	for k, c := range cols {
		if k < len(res.series) && len(res.series[k].Values) == len(r.labels) {
			vs := make([]*float64, len(res.series[k].Values))
			for i, v := range res.series[k].Values {
				if v != nil {
					a := math.Abs(*v)
					vs[i] = &a
				}
			}
			out[strings.ToLower(c)] = vs
		}
	}
	return out, nil
}

// trendX : X de chaque point pour une courbe de tendance. Nuage et bulles :
// la valeur de X (nombre, date ; sinon lue dans le libellé, sinon le rang
// compté de 0, comme le dessin) ; courbe sur un axe de temps : la date ;
// sinon le rang de la catégorie, compté de 1 (index).
func trendX(cfg ChartConfig, r chartResult) ([]*float64, bool) {
	n := len(r.labels)
	xs := make([]*float64, n)
	at := func(i int) *float64 {
		if i < len(r.xValues) && r.xValues[i] != nil {
			return r.xValues[i]
		}
		return nil
	}
	switch {
	case cfg.Kind == "scatter" || cfg.Kind == "bubble":
		for i := range xs {
			v := at(i)
			if v == nil {
				f, ok := parseShown(r.labels[i])
				if !ok {
					f = float64(i)
				}
				v = &f
			}
			xs[i] = v
		}
		return xs, false
	case (cfg.Kind == "line" || cfg.Kind == "area") && r.xDate && n > 1 && !slices.Contains(r.xValues, nil) && len(r.xValues) == n:
		copy(xs, r.xValues)
		return xs, false
	}
	for i := range xs {
		f := float64(i + 1)
		xs[i] = &f
	}
	return xs, true
}

// fitTrend ajuste une courbe de tendance aux points (x, y) ; nil si elle
// est impossible (trop peu de points, valeurs nulles ou négatives pour
// exp, log ou power).
func fitTrend(tl Trendline, xs, ys []float64, index bool) *TrendFit {
	n := len(xs)
	if tl.Type == "movavg" {
		return movingAverage(tl.Period, xs, ys, index)
	}
	need := 2
	if tl.Type == "poly" {
		need = tl.Order + 1
	}
	if n < need {
		return nil
	}
	f := &TrendFit{Type: tl.Type, Index: index}
	lx, ly := xs, ys
	if tl.Type == "log" || tl.Type == "power" {
		if slices.ContainsFunc(xs, func(x float64) bool { return x <= 0 }) {
			return nil
		}
		lx = mapF(xs, math.Log)
	}
	if tl.Type == "exp" || tl.Type == "power" {
		if slices.ContainsFunc(ys, func(y float64) bool { return y <= 0 }) {
			return nil
		}
		ly = mapF(ys, math.Log)
	}
	order := 1
	if tl.Type == "poly" {
		order = tl.Order
	}
	c, ok := polyFit(lx, ly, order)
	if !ok {
		return nil
	}
	f.R2 = rSquared(lx, ly, func(x float64) float64 { return polyEval(c, x) })
	var eval func(x float64) float64
	switch tl.Type {
	case "linear", "poly":
		f.Coef = c
		eval = func(x float64) float64 { return polyEval(c, x) }
	case "exp": // ln y = ln a + b x
		a, b := math.Exp(c[0]), c[1]
		f.Coef = []float64{a, b}
		eval = func(x float64) float64 { return a * math.Exp(b*x) }
	case "log": // y = a ln x + b
		f.Coef = []float64{c[1], c[0]}
		eval = func(x float64) float64 { return c[1]*math.Log(x) + c[0] }
	case "power": // ln y = ln a + b ln x
		a, b := math.Exp(c[0]), c[1]
		f.Coef = []float64{a, b}
		eval = func(x float64) float64 { return a * math.Pow(x, b) }
	}
	lo, hi := slices.Min(xs), slices.Max(xs)
	lo, hi = lo-tl.Backward, hi+tl.Forward
	if (tl.Type == "log" || tl.Type == "power") && lo <= 0 {
		lo = slices.Min(xs) / 1000
	}
	steps := 80
	if tl.Type == "linear" {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		x := lo + (hi-lo)*float64(i)/float64(steps)
		if y := eval(x); !math.IsNaN(y) && !math.IsInf(y, 0) {
			f.Points = append(f.Points, [2]float64{x, y})
		}
	}
	return f
}

func mapF(v []float64, fn func(float64) float64) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = fn(x)
	}
	return out
}

// movingAverage : moyenne des period derniers points (dans l'ordre de X),
// placée sur le dernier, comme un tableur ; ni équation ni prévision.
func movingAverage(period int, xs, ys []float64, index bool) *TrendFit {
	if period < 2 || len(xs) < period {
		return nil
	}
	idx := make([]int, len(xs))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int {
		switch {
		case xs[a] < xs[b]:
			return -1
		case xs[a] > xs[b]:
			return 1
		}
		return 0
	})
	f := &TrendFit{Type: "movavg", Index: index, Points: [][2]float64{}}
	sum := 0.0
	for j, i := range idx {
		sum += ys[i]
		if j >= period {
			sum -= ys[idx[j-period]]
		}
		if j >= period-1 {
			f.Points = append(f.Points, [2]float64{xs[i], sum / float64(period)})
		}
	}
	return f
}

// polyFit : moindres carrés d'un polynôme de degré order, coefficients
// c0 + c1·x + … ; X centré réduit pendant le calcul (dates, degré 6 : le
// système reste bien conditionné), coefficients ramenés à X ensuite.
func polyFit(xs, ys []float64, order int) ([]float64, bool) {
	n := len(xs)
	if n <= order {
		return nil, false
	}
	m := 0.0
	for _, x := range xs {
		m += x
	}
	m /= float64(n)
	s := 0.0
	for _, x := range xs {
		s += (x - m) * (x - m)
	}
	s = math.Sqrt(s / float64(n))
	if s == 0 {
		return nil, false // tous les X égaux
	}
	k := order + 1
	// Équations normales (A·a = b) sur u = (x − m) / s.
	a := make([][]float64, k)
	for i := range a {
		a[i] = make([]float64, k+1)
	}
	for p, x := range xs {
		u := (x - m) / s
		pw := make([]float64, 2*k)
		pw[0] = 1
		for j := 1; j < len(pw); j++ {
			pw[j] = pw[j-1] * u
		}
		for i := 0; i < k; i++ {
			for j := 0; j < k; j++ {
				a[i][j] += pw[i+j]
			}
			a[i][k] += pw[i] * ys[p]
		}
	}
	coef, ok := solve(a)
	if !ok {
		return nil, false
	}
	// p(x) = Σ c_i ((x − m)/s)^i = Σ_j x^j Σ_i c_i s^−i C(i, j) (−m)^(i−j).
	out := make([]float64, k)
	for i, c := range coef {
		ci := c / math.Pow(s, float64(i))
		for j := 0; j <= i; j++ {
			out[j] += ci * binom(i, j) * math.Pow(-m, float64(i-j))
		}
	}
	return out, true
}

// solve résout un système linéaire (matrice augmentée), pivot partiel.
func solve(a [][]float64) ([]float64, bool) {
	k := len(a)
	for c := 0; c < k; c++ {
		p := c
		for r := c + 1; r < k; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		if math.Abs(a[p][c]) < 1e-12 {
			return nil, false
		}
		a[c], a[p] = a[p], a[c]
		for r := 0; r < k; r++ {
			if r == c {
				continue
			}
			f := a[r][c] / a[c][c]
			for j := c; j <= k; j++ {
				a[r][j] -= f * a[c][j]
			}
		}
	}
	out := make([]float64, k)
	for i := range out {
		out[i] = a[i][k] / a[i][i]
	}
	return out, true
}

func binom(n, k int) float64 {
	r := 1.0
	for i := 1; i <= k; i++ {
		r = r * float64(n-k+i) / float64(i)
	}
	return r
}

func polyEval(c []float64, x float64) float64 {
	y := 0.0
	for i := len(c) - 1; i >= 0; i-- {
		y = y*x + c[i]
	}
	return y
}

// rSquared : 1 − SCres / SCtot ; 1 si toutes les valeurs sont égales et
// ajustées exactement.
func rSquared(xs, ys []float64, f func(float64) float64) float64 {
	m := 0.0
	for _, y := range ys {
		m += y
	}
	m /= float64(len(ys))
	var res, tot float64
	for i, x := range xs {
		d := ys[i] - f(x)
		res += d * d
		tot += (ys[i] - m) * (ys[i] - m)
	}
	if tot == 0 {
		if res < 1e-18 {
			return 1
		}
		return 0
	}
	return 1 - res/tot
}

// errorValues : montants des barres d'erreur d'une série (values) ;
// plus et minus : colonnes d'une barre personnalisée.
func errorValues(eb ErrorBars, values, plus, minus []*float64) *ErrorValues {
	out := &ErrorValues{Plus: make([]*float64, len(values)), Minus: make([]*float64, len(values))}
	var amount float64
	switch eb.Type {
	case "stddev", "stderr":
		var vs []float64
		for _, v := range values {
			if v != nil {
				vs = append(vs, *v)
			}
		}
		if len(vs) < 2 {
			return out
		}
		mean := 0.0
		for _, v := range vs {
			mean += v
		}
		mean /= float64(len(vs))
		ss := 0.0
		for _, v := range vs {
			ss += (v - mean) * (v - mean)
		}
		sd := math.Sqrt(ss / float64(len(vs)-1))
		if eb.Type == "stddev" {
			mult := eb.Value
			if mult == 0 {
				mult = 1
			}
			amount = mult * sd
			out.Center = &mean
		} else {
			amount = sd / math.Sqrt(float64(len(vs)))
		}
	case "fixed":
		amount = eb.Value
	}
	for i, v := range values {
		if v == nil {
			continue
		}
		var p, q *float64
		switch eb.Type {
		case "percent":
			a := math.Abs(*v) * eb.Value / 100
			p, q = &a, &a
		case "custom":
			if i < len(plus) {
				p = plus[i]
			}
			if i < len(minus) {
				q = minus[i]
			}
		default:
			a := amount
			p, q = &a, &a
		}
		if eb.Dir != "minus" {
			out.Plus[i] = p
		}
		if eb.Dir != "plus" {
			out.Minus[i] = q
		}
	}
	return out
}
