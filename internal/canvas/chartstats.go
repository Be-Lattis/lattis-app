package canvas

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Fréquences et boîtes à moustaches : statistiques calculées en SQL sur
// toutes les lignes affichées du tableau (pas seulement les MaxChartPoints
// premières), dans l'ordre que le tri n'influence pas.

// round12 arrondit à 12 chiffres significatifs (bornes sans 0,30000000000000004).
func round12(x float64) float64 {
	f, err := strconv.ParseFloat(strconv.FormatFloat(x, 'g', 12, 64), 64)
	if err != nil {
		return x
	}
	return f
}

// niceStep : pas « rond » (1, 2, 2,5, 5 × 10^k) au moins égal à raw.
func niceStep(raw float64) float64 {
	if !(raw > 0) || math.IsInf(raw, 0) {
		return 1
	}
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if s := m * mag; s >= raw*(1-1e-12) {
			return round12(s)
		}
	}
	return round12(10 * mag)
}

// histogramClasses : classes communes à toutes les séries, n valeurs entre
// lo et hi : bins classes (Sturges par défaut, au plus 50) de largeur ronde.
// Renvoie le début, le pas et les bornes (edges[i] ≤ v < edges[i+1], la
// dernière classe inclut sa borne haute).
func histogramClasses(lo, hi float64, n, bins int) (start, step float64, edges []float64) {
	k := bins
	if k <= 0 {
		k = min(50, max(1, int(math.Ceil(math.Log2(float64(n))))+1))
	}
	if lo == hi {
		lo, hi = lo-0.5, hi+0.5
	}
	step = niceStep((hi - lo) / float64(k))
	start = math.Floor(lo/step+1e-9) * step
	m := max(1, int(math.Ceil((hi-start)/step-1e-9)))
	edges = make([]float64, m+1)
	for i := range edges {
		edges[i] = round12(start + float64(i)*step)
	}
	return start, step, edges
}

// Les paramètres des colonnes calculées sont numérotés (?1…?n) : une requête
// qui ne cite pas le dernier en attend moins. LIMIT ?n+1 (-1 : sans limite)
// les cite tous.
func unlimited(q string, args []any) (string, []any) {
	return q + fmt.Sprintf(" LIMIT ?%d", len(args)+1), append(append([]any{}, args...), -1)
}

func (t *tx) rowAll(q string, args []any) *sql.Row {
	q, a := unlimited(q, args)
	return t.row(q, a...)
}

func (t *tx) queryAll(q string, args []any) (*sql.Rows, error) {
	q, a := unlimited(q, args)
	return t.query(q, a...)
}

// numericWhere : clause qui garde les lignes dont v est un nombre.
func numericWhere(v string) string { return ` WHERE typeof(` + v + `) IN ('integer', 'real')` }

// chartHistogram : effectif de chaque classe pour chaque série (Series),
// bornes des classes (Edges), sur toutes les lignes.
func (t *tx) chartHistogram(cfg ChartConfig, out chartResult, src string, vals []string, args []any) (chartResult, error) {
	out.edges = []float64{}
	union := make([]string, len(vals))
	for k, v := range vals {
		union[k] = `SELECT ` + v + ` AS v FROM ` + src + numericWhere(v)
	}
	var n int
	var lo, hi sql.NullFloat64
	if err := t.rowAll(`SELECT count(*), min(v), max(v) FROM (`+strings.Join(union, " UNION ALL ")+`)`, args).Scan(&n, &lo, &hi); err != nil {
		return chartResult{}, err
	}
	out.total = n
	if n == 0 {
		return out, nil
	}
	start, step, edges := histogramClasses(lo.Float64, hi.Float64, n, cfg.Bins)
	m := len(edges) - 1
	out.edges = edges
	for i := 0; i < m; i++ {
		out.labels = append(out.labels, formula.FormatNumber(edges[i])+" – "+formula.FormatNumber(edges[i+1]))
	}
	p := len(args)
	params := append(append([]any{}, args...), start, step, m-1)
	for k, v := range vals {
		q := fmt.Sprintf(`SELECT min(max(CAST((v - ?%d) / ?%d + 1e-9 AS INTEGER), 0), ?%d) AS c, count(*) FROM (SELECT %s AS v FROM %s%s) GROUP BY c`,
			p+1, p+2, p+3, v, src, numericWhere(v))
		counts := make([]*float64, m)
		for i := range counts {
			z := 0.0
			counts[i] = &z
		}
		rs, err := t.query(q, params...)
		if err != nil {
			return chartResult{}, err
		}
		for rs.Next() {
			var c int
			var nb float64
			if err := rs.Scan(&c, &nb); err != nil {
				rs.Close()
				return chartResult{}, err
			}
			if c >= 0 && c < m {
				*counts[c] = nb
			}
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return chartResult{}, err
		}
		out.series[k].Values = counts
	}
	return out, nil
}

// boxKey : clé d'un groupe lue en SQL (valeur et type).
func boxKey(v any) string { return fmt.Sprintf("%T|%v", v, v) }

// chartBoxes : une boîte par valeur de X quand X se répète (40 au plus),
// sinon une boîte par série ; quartiles, moyenne, moustaches et valeurs
// extrêmes sur toutes les lignes.
func (t *tx) chartBoxes(out chartResult, src, x string, xType grid.Type, vals []string, args []any) (chartResult, error) {
	key := "coalesce(" + x + ", '')"
	var distinct, rows int
	if err := t.rowAll(`SELECT count(DISTINCT `+key+`), count(*) FROM `+src, args).Scan(&distinct, &rows); err != nil {
		return chartResult{}, err
	}
	out.total = rows
	grouped := distinct < rows && distinct <= 40
	var keys []any
	if grouped {
		rs, err := t.queryAll(`SELECT `+key+` AS g FROM `+src+` GROUP BY g ORDER BY min(_pos)`, args)
		if err != nil {
			return chartResult{}, err
		}
		for rs.Next() {
			var g any
			if err := rs.Scan(&g); err != nil {
				rs.Close()
				return chartResult{}, err
			}
			keys = append(keys, g)
			lv := formula.FromResult(g)
			if xType == grid.Date && lv.Kind == formula.Number {
				lv = formula.DateSerial(lv.Num)
			}
			out.labels = append(out.labels, lv.String())
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return chartResult{}, err
		}
	} else {
		key = "''"
		keys = []any{""}
		for _, s := range out.series {
			out.labels = append(out.labels, s.Name)
		}
	}
	at := map[string]int{}
	for i, g := range keys {
		at[boxKey(g)] = i
	}
	out.boxes = make([][]*BoxStats, len(out.labels))
	for i := range out.boxes {
		out.boxes[i] = make([]*BoxStats, len(vals))
	}
	for k, v := range vals {
		stats, err := t.boxStats(src, key, v, args)
		if err != nil {
			return chartResult{}, err
		}
		for g, b := range stats {
			i, ok := at[g]
			if !ok {
				continue
			}
			if !grouped {
				i = k // une boîte par série, sur sa propre ligne
			}
			out.boxes[i][k] = b
		}
	}
	return out, nil
}

// boxStats calcule les boîtes d'une série (expression v), par groupe (key).
func (t *tx) boxStats(src, key, v string, args []any) (map[string]*BoxStats, error) {
	s := `s AS (SELECT ` + key + ` AS g, ` + v + ` AS v FROM ` + src + numericWhere(v) + `)`
	// Rang de chaque valeur dans son groupe : les quartiles s'interpolent
	// entre deux rangs voisins (QUARTILE.INCLURE).
	at := func(p string) string {
		return fmt.Sprintf(`max(CASE WHEN i = CAST((n - 1) * %s AS INTEGER) THEN v END), max(CASE WHEN i = CAST((n - 1) * %s AS INTEGER) + 1 THEN v END)`, p, p)
	}
	q := `WITH ` + s + `, r AS (SELECT g, v, row_number() OVER (PARTITION BY g ORDER BY v) - 1 AS i, count(*) OVER (PARTITION BY g) AS n FROM s)
		SELECT g, n, avg(v), ` + at("0.25") + `, ` + at("0.5") + `, ` + at("0.75") + ` FROM r GROUP BY g`
	rs, err := t.queryAll(q, args)
	if err != nil {
		return nil, err
	}
	out := map[string]*BoxStats{}
	raw := map[string]any{}
	for rs.Next() {
		var g any
		var n int
		var mean float64
		var a [6]sql.NullFloat64
		if err := rs.Scan(&g, &n, &mean, &a[0], &a[1], &a[2], &a[3], &a[4], &a[5]); err != nil {
			rs.Close()
			return nil, err
		}
		quant := func(p float64, lo, hi sql.NullFloat64) float64 {
			h := float64(n-1) * p
			b := hi.Float64
			if !hi.Valid {
				b = lo.Float64
			}
			return lo.Float64 + (h-math.Floor(h))*(b-lo.Float64)
		}
		b := &BoxStats{N: n, Mean: mean, Q1: quant(0.25, a[0], a[1]), Median: quant(0.5, a[2], a[3]), Q3: quant(0.75, a[4], a[5]), Outliers: []float64{}}
		out[boxKey(g)] = b
		raw[boxKey(g)] = g
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	// Bornes des moustaches de chaque groupe : 1,5 écart interquartile.
	p := len(args)
	params := append([]any{}, args...)
	var rowsSQL []string
	for g, b := range out {
		iqr := b.Q3 - b.Q1
		params = append(params, raw[g], b.Q1-1.5*iqr, b.Q3+1.5*iqr)
		rowsSQL = append(rowsSQL, fmt.Sprintf("(?%d, ?%d, ?%d)", p+1, p+2, p+3))
		p += 3
	}
	f := `f(g, lo, hi) AS (VALUES ` + strings.Join(rowsSQL, ", ") + `)`
	q = `WITH ` + s + `, ` + f + `
		SELECT s.g, min(s.v) FILTER (WHERE s.v >= f.lo), max(s.v) FILTER (WHERE s.v <= f.hi), count(*) FILTER (WHERE s.v < f.lo OR s.v > f.hi)
		FROM s JOIN f ON s.g = f.g GROUP BY s.g`
	rs, err = t.query(q, params...)
	if err != nil {
		return nil, err
	}
	outliers := false
	for rs.Next() {
		var g any
		var lo, hi sql.NullFloat64
		var nOut int
		if err := rs.Scan(&g, &lo, &hi, &nOut); err != nil {
			rs.Close()
			return nil, err
		}
		if b := out[boxKey(g)]; b != nil {
			b.Min, b.Max, b.Out = lo.Float64, hi.Float64, nOut
			outliers = outliers || nOut > 0
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	if !outliers {
		return out, nil
	}
	// Valeurs extrêmes : les 100 plus basses et les 100 plus hautes de chaque groupe.
	q = `WITH ` + s + `, ` + f + `, o AS (SELECT s.g AS g, s.v AS v FROM s JOIN f ON s.g = f.g WHERE s.v < f.lo OR s.v > f.hi),
		r AS (SELECT g, v, row_number() OVER (PARTITION BY g ORDER BY v) AS a, row_number() OVER (PARTITION BY g ORDER BY v DESC) AS b FROM o)
		SELECT g, v FROM r WHERE a <= 100 OR b <= 100 ORDER BY g, v`
	rs, err = t.query(q, params...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var g any
		var v float64
		if err := rs.Scan(&g, &v); err != nil {
			return nil, err
		}
		if b := out[boxKey(g)]; b != nil {
			b.Outliers = append(b.Outliers, v)
		}
	}
	return out, rs.Err()
}
