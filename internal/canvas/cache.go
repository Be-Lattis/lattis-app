package canvas

import (
	"encoding/json"
	"fmt"

	"github.com/lalamefine/lattis/internal/formula"
)

// Versions en cache des calculs lourds relus à chaque mise en page : une
// formule libre, un tableau dynamique, un TCD ou un graphique sur un million
// de lignes ne se recalcule qu'après une écriture (voir cached).

func keyOf(prefix string, v any) string {
	b, _ := json.Marshal(v)
	return prefix + string(b)
}

func (t *tx) evalFree(sheet int64, raw string) formula.Value {
	v, _ := cached(t, fmt.Sprintf("free:%d:%s", sheet, raw), func() (formula.Value, error) { return t.evalFreeRaw(sheet, raw), nil })
	return v
}

type spillResult struct {
	rows [][]string
	ok   bool
	err  formula.Value
}

func (t *tx) spillFree(sheet int64, raw string) ([][]string, bool, formula.Value) {
	r, _ := cached(t, fmt.Sprintf("spill:%d:%s", sheet, raw), func() (spillResult, error) {
		rows, ok, e := t.spillFreeRaw(sheet, raw)
		return spillResult{rows, ok, e}, nil
	})
	return r.rows, r.ok, r.err
}

func (t *tx) pivotCells(cfg PivotConfig) ([][]string, error) {
	// Un TCD ne lit que son tableau source (pas de colonne calculée comme champ).
	if tb, err := t.tableByName(cfg.Source); err == nil {
		return cachedOn(t, keyOf("pivot:", cfg), []string{tb.sql()}, func() ([][]string, error) { return t.pivotCellsRaw(cfg) })
	}
	return cached(t, keyOf("pivot:", cfg), func() ([][]string, error) { return t.pivotCellsRaw(cfg) })
}

// chartDeps donne les tableaux que lit un graphique, ou faux s'il peut en
// lire d'autres (colonne calculée de la source : formules, REFERENCE…).
func (t *tx) chartDeps(source string) ([]string, bool) {
	if tb, err := t.tableByName(source); err == nil {
		for _, c := range tb.Cols {
			if c.Formula != "" {
				return nil, false
			}
		}
		return []string{tb.sql()}, true
	}
	if pc, ok, err := t.pivotByName(source); err == nil && ok {
		if tb, err := t.tableByName(pc.Source); err == nil {
			return []string{tb.sql()}, true
		}
	}
	return nil, false
}

type chartResult struct {
	labels []string
	series []ChartSeries
}

func (t *tx) chartData(cfg ChartConfig) ([]string, []ChartSeries, error) {
	key := keyOf("chart:", ChartConfig{Source: cfg.Source, Kind: cfg.Kind, X: cfg.X, Series: cfg.Series})
	calc := func() (chartResult, error) {
		l, s, err := t.chartDataRaw(cfg)
		return chartResult{l, s}, err
	}
	var r chartResult
	var err error
	if deps, ok := t.chartDeps(cfg.Source); ok {
		r, err = cachedOn(t, key, deps, calc)
	} else {
		r, err = cached(t, key, calc)
	}
	return r.labels, r.series, err
}
