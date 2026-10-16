package canvas

import (
	"encoding/json"
	"fmt"

	"github.com/lalamefine/slitable/internal/formula"
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
	return cached(t, keyOf("pivot:", cfg), func() ([][]string, error) { return t.pivotCellsRaw(cfg) })
}

type chartResult struct {
	labels []string
	series []ChartSeries
}

func (t *tx) chartData(cfg ChartConfig) ([]string, []ChartSeries, error) {
	r, err := cached(t, keyOf("chart:", ChartConfig{Source: cfg.Source, Kind: cfg.Kind, X: cfg.X, Series: cfg.Series}), func() (chartResult, error) {
		l, s, err := t.chartDataRaw(cfg)
		return chartResult{l, s}, err
	})
	return r.labels, r.series, err
}
