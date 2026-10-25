package formula

import (
	"fmt"
	"math"
	"strings"
)

// Statistiques ajoutées à l'audit 16 : forme des distributions (KURTOSIS,
// COEFFICIENT.ASYMETRIE…, agrégats dans agg.go), régression (PREVISION,
// TENDANCE, CROISSANCE, DROITEREG, LOGREG), tests (T.TEST, F.TEST, Z.TEST,
// CHISQ.TEST) et MOYENNE.RANG. Comme PENTE, elles parcourent des colonnes
// d'un même tableau (ou des plages de même forme) et ne lisent que leurs
// nombres.

// shapeStats : KURTOSIS, COEFFICIENT.ASYMETRIE(.P), ECART.MOYEN,
// SOMME.CARRES.ECARTS, MOYENNE.GEOMETRIQUE et MOYENNE.HARMONIQUE des valeurs.
func shapeStats(fn string, v []float64) Value {
	n := float64(len(v))
	if n == 0 {
		if fn == "KURT" || fn == "SKEW" || fn == "SKEW.P" {
			return Err(ErrDiv0)
		}
		return Err(ErrNum)
	}
	m := mean(v)
	var s2, s3, s4, abs float64
	for _, x := range v {
		d := x - m
		s2 += d * d
		s3 += d * d * d
		s4 += d * d * d * d
		abs += math.Abs(d)
	}
	switch fn {
	case "DEVSQ":
		return number(s2)
	case "AVEDEV":
		return number(abs / n)
	case "GEOMEAN", "HARMEAN":
		t := 0.0
		for _, x := range v {
			if x <= 0 {
				return Err(ErrNum)
			}
			if fn == "GEOMEAN" {
				t += math.Log(x)
			} else {
				t += 1 / x
			}
		}
		if fn == "GEOMEAN" {
			return number(math.Exp(t / n))
		}
		return number(n / t)
	case "SKEW", "SKEW.P":
		if n < 3 || s2 == 0 {
			return Err(ErrDiv0)
		}
		if fn == "SKEW.P" {
			return number(s3 / n / math.Pow(s2/n, 1.5))
		}
		sd := math.Sqrt(s2 / (n - 1))
		return number(n / ((n - 1) * (n - 2)) * s3 / (sd * sd * sd))
	case "KURT":
		if n < 4 || s2 == 0 {
			return Err(ErrDiv0)
		}
		v := s2 / (n - 1)
		return number(n*(n+1)/((n-1)*(n-2)*(n-3))*s4/(v*v) - 3*(n-1)*(n-1)/((n-2)*(n-3)))
	}
	return Err(ErrName)
}

// pairs : les paires (y, x) d'un parcours de deux colonnes, seulement les
// lignes où les deux valeurs sont des nombres (base de PENTE et des autres).
type pairs struct {
	xs, ys []float64
	err    Value
	numRule
}

func (s *pairs) add(cols []Value) {
	if s.err.IsErr() {
		return
	}
	for _, v := range cols[:2] {
		if v.IsErr() {
			s.err = v
			return
		}
	}
	a, b := s.coerce(cols[0]), s.coerce(cols[1])
	if (a.Kind == Number || a.Kind == Date) && (b.Kind == Number || b.Kind == Date) {
		s.ys, s.xs = append(s.ys, a.Num), append(s.xs, b.Num)
	}
}

// fit : droite des moindres carrés y = m·x + b (b = 0 sans constante) et
// ses sommes.
type fit struct {
	n, m, b          float64
	mx, my, sxx, sxy float64
	ssReg, ssRes     float64
	df               float64
	konst            bool
}

func linearFit(xs, ys []float64, konst bool) (fit, Value) {
	f := fit{n: float64(len(xs)), konst: konst}
	if f.n == 0 {
		return f, Err(ErrDiv0)
	}
	if konst {
		f.mx, f.my = mean(xs), mean(ys)
	}
	var syy float64
	for i := range xs {
		dx, dy := xs[i]-f.mx, ys[i]-f.my
		f.sxx += dx * dx
		f.sxy += dx * dy
		syy += dy * dy
	}
	if f.sxx == 0 {
		return f, Err(ErrDiv0)
	}
	f.m = f.sxy / f.sxx
	f.b = f.my - f.m*f.mx
	for i := range xs {
		r := ys[i] - (f.m*xs[i] + f.b)
		f.ssRes += r * r
	}
	f.ssReg = syy - f.ssRes
	f.df = f.n - 1
	if konst {
		f.df--
	}
	return f, Value{}
}

// regression : PREVISION.LINEAIRE(x; y_connus; x_connus), TENDANCE et
// CROISSANCE(y_connus; x_connus; nouveaux_x; [constante]). CROISSANCE ajuste
// ln(y) (des y positifs).
type regression struct {
	fn     string
	x      Value // la valeur à prévoir
	konst  Value
	growth bool
	pairs
}

func newRegression(fn string) func([]Value) scanner {
	return func(p []Value) scanner {
		r := &regression{fn: fn, x: p[0], growth: fn == "GROWTH"}
		if len(p) > 1 {
			r.konst = p[1]
		}
		return r
	}
}

func (r *regression) result() Value {
	if r.err.IsErr() {
		return r.err
	}
	konst := true
	if !r.konst.IsEmpty() {
		b, e := Truth(r.konst)
		if e.IsErr() {
			return e
		}
		konst = b
	}
	x, e := ToNumber(r.x)
	if e.IsErr() {
		return e
	}
	ys := r.ys
	if r.growth {
		ys = make([]float64, len(r.ys))
		for i, y := range r.ys {
			if y <= 0 {
				return Err(ErrNum)
			}
			ys[i] = math.Log(y)
		}
	}
	f, e := linearFit(r.xs, ys, konst)
	if e.IsErr() {
		return e
	}
	y := f.m*x + f.b
	if r.growth {
		return number(math.Exp(y))
	}
	return number(y)
}

// pairStat : COVARIANCE.STANDARD et .PEARSON, COEFFICIENT.DETERMINATION et
// ERREUR.TYPE.XY.
func pairStat(fn string, xs, ys []float64) Value {
	n := float64(len(xs))
	if n == 0 || fn == "COVARIANCE.S" && n < 2 || fn == "STEYX" && n < 3 {
		return Err(ErrDiv0)
	}
	mx, my := mean(xs), mean(ys)
	var sxy, sxx, syy float64
	for i := range xs {
		dx, dy := xs[i]-mx, ys[i]-my
		sxy += dx * dy
		sxx += dx * dx
		syy += dy * dy
	}
	switch fn {
	case "COVARIANCE.S":
		return number(sxy / (n - 1))
	case "COVARIANCE.P":
		return number(sxy / n)
	case "RSQ":
		if sxx == 0 || syy == 0 {
			return Err(ErrDiv0)
		}
		return number(sxy * sxy / (sxx * syy))
	}
	if sxx == 0 {
		return Err(ErrDiv0)
	}
	return number(math.Sqrt(math.Max(0, syy-sxy*sxy/sxx) / (n - 2)))
}

// linest : DROITEREG et LOGREG(y_connus; x_connus; [constante]; [statistiques]),
// la valeur n° idx (ligne par ligne) de leur tableau de résultats : 1 ligne
// de 2 valeurs (pente, ordonnée), 5 avec les statistiques.
type linest struct {
	log          bool
	konst, stats Value
	idx          int
	pairs
}

func newLinest(log bool) func([]Value) scanner {
	return func(p []Value) scanner {
		l := &linest{log: log, konst: optParam(p, 0), stats: optParam(p, 1)}
		if len(p) > 2 {
			l.idx = int(p[2].Num)
		}
		return l
	}
}

// linestShape lit les options de DROITEREG : constante (VRAI si omise) et
// statistiques (FAUX si omises).
func linestShape(konst, stats Value) (bool, bool, Value) {
	k, s := true, false
	if !konst.IsEmpty() {
		b, e := Truth(konst)
		if e.IsErr() {
			return false, false, e
		}
		k = b
	}
	if !stats.IsEmpty() {
		b, e := Truth(stats)
		if e.IsErr() {
			return false, false, e
		}
		s = b
	}
	return k, s, Value{}
}

func (l *linest) result() Value {
	m, e := l.matrix()
	if e.IsErr() {
		return e
	}
	if l.idx < 0 || l.idx >= 2*len(m) {
		return Value{}
	}
	return m[l.idx/2][l.idx%2]
}

func (l *linest) matrix() ([][]Value, Value) {
	if l.err.IsErr() {
		return nil, l.err
	}
	konst, stats, e := linestShape(l.konst, l.stats)
	if e.IsErr() {
		return nil, e
	}
	ys := l.ys
	if l.log {
		ys = make([]float64, len(l.ys))
		for i, y := range l.ys {
			if y <= 0 {
				return nil, Err(ErrNum)
			}
			ys[i] = math.Log(y)
		}
	}
	f, e := linearFit(l.xs, ys, konst)
	if e.IsErr() {
		return nil, e
	}
	m, b := number(f.m), number(f.b)
	if l.log {
		m, b = number(math.Exp(f.m)), number(math.Exp(f.b))
	}
	out := [][]Value{{m, b}}
	if !stats {
		return out, Value{}
	}
	div := func(a, b float64) Value {
		if b == 0 {
			return Err(ErrDiv0)
		}
		return number(a / b)
	}
	seY := Err(ErrDiv0)
	seM, seB := Err(ErrDiv0), Err(ErrNA)
	if f.df > 0 {
		s := math.Sqrt(f.ssRes / f.df)
		seY = number(s)
		seM = number(s / math.Sqrt(f.sxx))
		if konst {
			seB = number(s * math.Sqrt(1/f.n+f.mx*f.mx/f.sxx))
		}
	}
	r2 := div(f.ssReg, f.ssReg+f.ssRes)
	fStat := Err(ErrDiv0)
	if f.df > 0 {
		fStat = div(f.ssReg, f.ssRes/f.df)
	}
	out = append(out,
		[]Value{seM, seB},
		[]Value{r2, seY},
		[]Value{fStat, Num(f.df)},
		[]Value{number(f.ssReg), number(f.ssRes)})
	return out, Value{}
}

// linestRows : nombre de lignes du résultat de DROITEREG (fixé par
// l'argument statistiques, qui doit donc être écrit dans la formule).
func linestRows(c Call) (int, ErrCode) {
	if len(c.Args) < 4 || c.Args[3] == nil {
		return 1, 0
	}
	switch a := c.Args[3].(type) {
	case BoolLit:
		if a.V {
			return 5, 0
		}
		return 1, 0
	case NumLit:
		if a.V != 0 {
			return 5, 0
		}
		return 1, 0
	}
	return 0, ErrValue
}

// --- Tests ------------------------------------------------------------------------

// twoSamples : T.TEST(matrice1; matrice2; uni_bilatéral; type) et
// F.TEST(matrice1; matrice2). Chaque colonne garde ses nombres ; T.TEST de
// type 1 (apparié) ne garde que les lignes où les deux en sont.
type twoSamples struct {
	fn         string
	tails, typ Value
	a, b       []float64
	pairs
}

func (s *twoSamples) add(cols []Value) {
	s.pairs.add(cols)
	if s.err.IsErr() {
		return
	}
	if v := s.coerce(cols[0]); v.Kind == Number || v.Kind == Date {
		s.a = append(s.a, v.Num)
	}
	if v := s.coerce(cols[1]); v.Kind == Number || v.Kind == Date {
		s.b = append(s.b, v.Num)
	}
}

func sampleVar(v []float64) float64 {
	m, ss := mean(v), 0.0
	for _, x := range v {
		ss += (x - m) * (x - m)
	}
	return ss / float64(len(v)-1)
}

func (s *twoSamples) result() Value {
	if s.err.IsErr() {
		return s.err
	}
	n1, n2 := float64(len(s.a)), float64(len(s.b))
	if s.fn == "F.TEST" {
		if n1 < 2 || n2 < 2 {
			return Err(ErrDiv0)
		}
		v1, v2 := sampleVar(s.a), sampleVar(s.b)
		if v1 == 0 || v2 == 0 {
			return Err(ErrDiv0)
		}
		p := fCDF(v1/v2, n1-1, n2-1)
		return number(2 * math.Min(p, 1-p))
	}
	x, e := distArgs([]Value{s.tails, s.typ}, 2)
	if e.IsErr() {
		return e
	}
	tails, typ := math.Trunc(x[0]), math.Trunc(x[1])
	if tails != 1 && tails != 2 || typ < 1 || typ > 3 {
		return Err(ErrNum)
	}
	var t, df float64
	switch typ {
	case 1:
		n := float64(len(s.xs))
		if n < 2 {
			return Err(ErrDiv0)
		}
		d := make([]float64, len(s.xs))
		for i := range d {
			// pairs garde (y, x) : matrice1 dans ys, matrice2 dans xs.
			d[i] = s.ys[i] - s.xs[i]
		}
		v := sampleVar(d)
		if v == 0 {
			return Err(ErrDiv0)
		}
		t, df = mean(d)/math.Sqrt(v/n), n-1
	case 2, 3:
		if n1 < 2 || n2 < 2 {
			return Err(ErrDiv0)
		}
		v1, v2 := sampleVar(s.a), sampleVar(s.b)
		if typ == 2 {
			df = n1 + n2 - 2
			sp := ((n1-1)*v1 + (n2-1)*v2) / df
			if sp == 0 {
				return Err(ErrDiv0)
			}
			t = (mean(s.a) - mean(s.b)) / math.Sqrt(sp*(1/n1+1/n2))
		} else {
			q1, q2 := v1/n1, v2/n2
			if q1+q2 == 0 {
				return Err(ErrDiv0)
			}
			t = (mean(s.a) - mean(s.b)) / math.Sqrt(q1+q2)
			df = (q1 + q2) * (q1 + q2) / (q1*q1/(n1-1) + q2*q2/(n2-1))
		}
	}
	return number(math.Min(1, tails*tTail(math.Abs(t), df)))
}

// zTest : Z.TEST(matrice; x; [sigma]), probabilité unilatérale que la moyenne
// dépasse celle observée (sigma : écart type de l'échantillon si omis).
type zTest struct {
	x, sigma Value
	vals     []float64
	err      Value
	numRule
}

func (z *zTest) add(cols []Value) {
	switch v := z.coerce(cols[0]); v.Kind {
	case Number, Date:
		z.vals = append(z.vals, v.Num)
	case Error:
		if !z.err.IsErr() {
			z.err = v
		}
	}
}

func (z *zTest) result() Value {
	if z.err.IsErr() {
		return z.err
	}
	x, e := ToNumber(z.x)
	if e.IsErr() {
		return e
	}
	n := float64(len(z.vals))
	if n == 0 {
		return Err(ErrNA)
	}
	var sd float64
	if z.sigma.IsEmpty() {
		if n < 2 {
			return Err(ErrDiv0)
		}
		sd = math.Sqrt(sampleVar(z.vals))
	} else if sd, e = ToNumber(z.sigma); e.IsErr() {
		return e
	}
	if sd == 0 {
		return Err(ErrDiv0)
	}
	return number(1 - normCDF((mean(z.vals)-x)/(sd/math.Sqrt(n))))
}

// chiTest : CHISQ.TEST(plage_réelle; plage_attendue) sur une colonne ou une
// plage (degrés de liberté : lignes − 1 pour une seule colonne, (lignes − 1)
// × (colonnes − 1) pour une plage).
type chiTest struct {
	rows, cols int
	pairs
}

func (c *chiTest) result() Value {
	if c.err.IsErr() {
		return c.err
	}
	if len(c.xs) == 0 {
		return Err(ErrNA)
	}
	chi := 0.0
	for i := range c.xs {
		if c.xs[i] == 0 {
			return Err(ErrDiv0)
		}
		d := c.ys[i] - c.xs[i]
		chi += d * d / c.xs[i]
	}
	df := float64(len(c.xs) - 1)
	if c.rows > 1 && c.cols > 1 {
		df = float64((c.rows - 1) * (c.cols - 1))
	}
	if df < 1 {
		return Err(ErrNA)
	}
	return number(gammaQ(df/2, chi/2))
}

// rankAvg : MOYENNE.RANG(valeur; référence; [ordre]), le rang moyen des
// valeurs égales.
type rankAvg struct {
	rank
	eq int
}

func (r *rankAvg) add(cols []Value) {
	r.rank.add(cols)
	if v := cols[0]; (v.Kind == Number || v.Kind == Date) && r.v.Kind != Empty && v.Num == r.v.Num {
		r.eq++
	}
}

func (r *rankAvg) result() Value {
	if v := r.rank.result(); v.IsErr() {
		return v
	}
	return number(float64(r.n) + float64(r.eq+1)/2)
}

// pairStatScan : COVARIANCE, COEFFICIENT.DETERMINATION, ERREUR.TYPE.XY.
type pairStatScan struct {
	fn string
	pairs
}

func (s *pairStatScan) result() Value {
	if s.err.IsErr() {
		return s.err
	}
	return pairStat(s.fn, s.xs, s.ys)
}

func pairStatOf(fn string) func([]Value) scanner {
	return func([]Value) scanner { return &pairStatScan{fn: fn} }
}

func linestRoles(n int) ([]role, bool) { return roles(column, column, param, param, param)[:n], n >= 2 }

// linestCall : le parcours qui donne la valeur n° idx de DROITEREG.
func linestCall(c Call, idx int) scanCall {
	if len(c.Args) < 2 || IsSpill(c.Args[0]) || IsSpill(c.Args[1]) {
		return scanCall{err: ErrValue}
	}
	arg := func(i int) Node {
		if i < len(c.Args) {
			return c.Args[i]
		}
		return nil
	}
	return scanCall{def: scans[c.Fn], name: c.Fn, cols: []Node{c.Args[0], c.Args[1]},
		params: []Node{arg(2), arg(3), NumLit{V: float64(idx)}}}
}

// linest évalue DROITEREG et LOGREG : un tableau de 1 ou 5 lignes sur 2
// colonnes.
func (env *Env) linest(c Call) (rel, Value) {
	rows, e := linestRows(c)
	if e != 0 {
		return rel{}, Err(e)
	}
	out := rel{cols: seqHeaders(2)}
	for r := range rows {
		line := make([]Value, 2)
		for j := range line {
			line[j] = env.scan(linestCall(c, 2*r+j))
		}
		out.values = append(out.values, line)
	}
	return out, Value{}
}

func (c *compiler) linest(call Call, sc scope) (sqlRel, ErrCode) {
	rows, e := linestRows(call)
	if e != 0 {
		return sqlRel{}, e
	}
	var parts []string
	for r := range rows {
		parts = append(parts, fmt.Sprintf("SELECT %s AS v1, %s AS v2, %d AS _n",
			c.scan(linestCall(call, 2*r), sc), c.scan(linestCall(call, 2*r+1), sc), r+1))
	}
	return sqlRel{cols: seqHeaders(2), from: strings.Join(parts, " UNION ALL ")}, 0
}
