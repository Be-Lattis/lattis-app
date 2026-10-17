package grid

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Statistiques des colonnes pour la mise en forme conditionnelle : 10
// premiers, au-dessus de la moyenne, doublons, barres de données, nuances
// de couleurs, jeux d'icônes. Elles se calculent en SQL sur toute la
// colonne (un million de lignes ne passe jamais dans l'interface) et se
// gardent en cache jusqu'à la prochaine modification du tableau.

// MaxDups borne la liste des valeurs en double renvoyée à l'interface.
const MaxDups = 100_000

// RuleStatsReq dit ce qu'une règle demande en plus des statistiques de base
// (nombre, minimum, maximum, moyenne, écart type).
type RuleStatsReq struct {
	// Rank : seuil des N plus grandes valeurs (N > 0) ou des N plus petites
	// (N < 0) ; Pct : N pour cent des valeurs numériques.
	Rank int  `json:"rank"`
	Pct  bool `json:"pct"`
	// Pctls : centiles voulus (0 à 100), interpolés comme CENTILE.INCLURE.
	Pctls []float64 `json:"pctls"`
	// Dups : valeurs présentes plusieurs fois (texte sans casse).
	Dups bool `json:"dups"`
}

// RuleStats sont les statistiques d'une ou plusieurs colonnes. Les dates
// comptent comme des nombres (numéro de série d'Excel).
type RuleStats struct {
	Count int     `json:"count"` // valeurs non vides
	Nums  int     `json:"nums"`  // valeurs numériques
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
	Std   float64 `json:"std"` // écart type (échantillon)
	// Rank : valeur seuil demandée (nil sans valeur numérique).
	Rank  *float64  `json:"rank"`
	Pctls []float64 `json:"pctls"`
	// Dups : valeurs en double (nombres, ou textes en minuscules ASCII) ;
	// DupsMore : liste tronquée à MaxDups.
	Dups     []any `json:"dups"`
	DupsMore bool  `json:"dupsMore"`
}

// RuleStats calcule les statistiques des colonnes cols (indices
// d'affichage), réunies, sur toutes les lignes affichées du tableau.
func (t *Table) RuleStats(ctx context.Context, cols []int, req RuleStatsReq) (RuleStats, error) {
	kb, _ := json.Marshal(struct {
		C []int
		R RuleStatsReq
	}{cols, req})
	key := string(kb)
	t.mu.RLock()
	if st, ok := t.cf[key]; ok {
		t.mu.RUnlock()
		return st, nil
	}
	t.mu.RUnlock()
	out := RuleStats{Pctls: []float64{}, Dups: []any{}}
	if len(cols) == 0 {
		return out, nil
	}
	// Source : x (valeur brute, pour les doublons) et n (valeur numérique).
	n := len(t.args)
	where := ""
	if n > 0 {
		where = fmt.Sprintf(" WHERE ?%d IS NULL OR 1", n)
	}
	var parts []string
	for _, c := range cols {
		if c < 0 || c >= len(t.exprs) {
			return out, ErrRange
		}
		x := t.exprs[c]
		num := "CASE WHEN typeof(x) IN ('integer', 'real') THEN x END"
		switch t.cols[c].Type {
		case Boolean:
			num = "NULL"
		case Date:
			num = "CASE WHEN typeof(x) = 'text' THEN julianday(x) - 2415018.5 END"
		}
		parts = append(parts, fmt.Sprintf("SELECT x, %s AS n FROM (SELECT %s AS x FROM %s%s)", num, x, t.sqlName, where))
	}
	src := "(" + strings.Join(parts, " UNION ALL ") + ")"
	db := t.db.Read()
	var mn, mx, avg, sq sql.NullFloat64
	err := db.QueryRowContext(ctx, `SELECT count(x), count(n), min(n), max(n), avg(n), avg(n * n) FROM `+src, t.args...).
		Scan(&out.Count, &out.Nums, &mn, &mx, &avg, &sq)
	if err != nil {
		return out, err
	}
	out.Min, out.Max, out.Avg = mn.Float64, mx.Float64, avg.Float64
	if out.Nums > 1 {
		v := (sq.Float64 - avg.Float64*avg.Float64) * float64(out.Nums) / float64(out.Nums-1)
		out.Std = math.Sqrt(math.Max(v, 0))
	}
	// k-ième valeur numérique dans l'ordre croissant (0 : la plus petite).
	nth := func(k int) (float64, error) {
		var v float64
		err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT n FROM %s WHERE n IS NOT NULL ORDER BY n LIMIT 1 OFFSET %d`, src, k), t.args...).Scan(&v)
		return v, err
	}
	if req.Rank != 0 && out.Nums > 0 {
		k := req.Rank
		if k < 0 {
			k = -k
		}
		if req.Pct {
			k = max(1, int(math.Floor(float64(out.Nums)*float64(k)/100)))
		}
		k = min(k, out.Nums)
		pos := k - 1 // N plus petites : la k-ième dans l'ordre croissant
		if req.Rank > 0 {
			pos = out.Nums - k
		}
		v, err := nth(pos)
		if err != nil {
			return out, err
		}
		out.Rank = &v
	}
	for _, p := range req.Pctls {
		if out.Nums == 0 {
			out.Pctls = append(out.Pctls, 0)
			continue
		}
		p = math.Min(math.Max(p, 0), 100)
		pos := p / 100 * float64(out.Nums-1)
		lo := int(math.Floor(pos))
		a, err := nth(lo)
		if err != nil {
			return out, err
		}
		v := a
		if frac := pos - float64(lo); frac > 0 && lo+1 < out.Nums {
			b, err := nth(lo + 1)
			if err != nil {
				return out, err
			}
			v = a + (b-a)*frac
		}
		out.Pctls = append(out.Pctls, v)
	}
	if req.Dups {
		rs, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT k FROM (SELECT CASE WHEN typeof(x) = 'text' THEN lower(x) ELSE x END AS k FROM %s WHERE x IS NOT NULL AND x <> '') GROUP BY k HAVING count(*) > 1 LIMIT %d`, src, MaxDups+1), t.args...)
		if err != nil {
			return out, err
		}
		for rs.Next() {
			var v any
			if err := rs.Scan(&v); err != nil {
				rs.Close()
				return out, err
			}
			if len(out.Dups) == MaxDups {
				out.DupsMore = true
				break
			}
			out.Dups = append(out.Dups, jsonValue(v))
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return out, err
		}
	}
	t.mu.Lock()
	if t.cf == nil {
		t.cf = map[string]RuleStats{}
	}
	t.cf[key] = out
	t.mu.Unlock()
	return out, nil
}
