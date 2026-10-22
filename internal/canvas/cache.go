package canvas

import (
	"encoding/json"
	"fmt"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/work"
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

// pivotFlat calcule un TCD à plat (une colonne par champ des lignes) : la
// vue SQL et les graphiques le lisent ainsi.
func (t *tx) pivotFlat(cfg PivotConfig) ([][]string, error) {
	cfg = cfg.flat()
	// Un TCD ne lit que son tableau source (pas de colonne calculée comme champ).
	if tb, err := t.tableByName(cfg.Source); err == nil && !cfg.byFormula() {
		return cachedOn(t, keyOf("pivot:", cfg), []string{tb.sql()}, func() ([][]string, error) {
			work.Phase(t.Ctx, "pivot")
			return t.pivotCellsRaw(cfg)
		})
	}
	return cached(t, keyOf("pivot:", cfg), func() ([][]string, error) { return t.pivotCellsRaw(cfg) })
}

// pivotCells calcule un TCD tel qu'affiché : en arborescence avec plusieurs
// champs en lignes (rows : une description par ligne sous l'en-tête), à plat
// sinon (rows nil).
func (t *tx) pivotCells(cfg PivotConfig) ([][]string, []PivotRowInfo, error) {
	cfg.Hand = nil // colonnes saisies : ajoutées après le calcul (pivothand.go)
	cfg.Styles = nil
	calc := func() (pivotView, error) {
		work.Phase(t.Ctx, "pivot")
		return t.pivotViewRaw(cfg)
	}
	var v pivotView
	var err error
	if tb, terr := t.tableByName(cfg.Source); terr == nil && !cfg.byFormula() {
		v, err = cachedOn(t, keyOf("pivotview:", cfg), []string{tb.sql()}, calc)
	} else {
		v, err = cached(t, keyOf("pivotview:", cfg), calc)
	}
	return v.cells, v.rows, err
}

// pivotLater dit si le calcul du TCD (pivotCells) parcourrait un grand
// tableau (plus de quickRows lignes) sans être en cache (LayoutFirst).
func (t *tx) pivotLater(cfg PivotConfig) bool {
	cfg.Hand, cfg.Styles = nil, nil
	tb, err := t.tableByName(cfg.Source)
	if err != nil || tb.Rows <= quickRows {
		return false
	}
	key := keyOf("pivotview:", cfg)
	if cfg.byFormula() {
		_, ok := peek[pivotView](t, key)
		return !ok
	}
	_, ok := peekOn[pivotView](t, key, []string{tb.sql()})
	return !ok
}

// chartKey : clé du cache des données d'un graphique (chartData).
func (t *tx) chartKey(cfg ChartConfig) string {
	key := keyOf("chart:", ChartConfig{Source: cfg.Source, Kind: cfg.Kind, X: cfg.X, Series: cfg.Series, MapCoords: cfg.MapCoords, MapDraft: cfg.MapDraft, ColorBy: cfg.ColorBy, Bins: cfg.Bins, Hidden: cfg.Hidden})
	// Le filtre du tableau (rangé hors du tableau) fait partie de la clé.
	if tb, err := t.tableByName(cfg.Source); err == nil && !cfg.Hidden {
		key += "|" + tb.where
	}
	return key
}

// chartLater dit, comme pivotLater, si les données du graphique
// demanderaient de parcourir un grand tableau (sa source, ou celle du TCD
// qu'il lit).
func (t *tx) chartLater(cfg ChartConfig) bool {
	tb, err := t.tableByName(cfg.Source)
	if err != nil {
		pc, ok, perr := t.pivotByName(cfg.Source)
		if perr != nil || !ok {
			return false
		}
		if tb, err = t.tableByName(pc.Source); err != nil {
			return false
		}
	}
	if tb.Rows <= quickRows {
		return false
	}
	key := t.chartKey(cfg)
	if deps, ok := t.chartDeps(cfg.Source); ok {
		_, hit := peekOn[chartResult](t, key, deps)
		return !hit
	}
	_, hit := peek[chartResult](t, key)
	return !hit
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
	labels  []string
	series  []ChartSeries
	invalid int       // carte : coordonnées illisibles ignorées
	guess   *MapGuess // carte en attente : colonnes repérées
	rows    []int64   // carte : ligne (id) de chaque point
	cats    []string  // carte colorée : valeurs distinctes triées
	groups  []int     // carte colorée : rang de la valeur de chaque point
	// Voir ChartInfo.
	total              int
	truncated, sampled bool
	xValues            []*float64
	xDate              bool
	edges              []float64
	boxes              [][]*BoxStats
	formats            []string
	xFormat            string
}

// fill reporte les données dans un graphique.
func (r chartResult) fill(c *ChartInfo) {
	c.Labels, c.Series, c.Invalid, c.Guess = r.labels, r.series, r.invalid, r.guess
	c.Rows, c.Categories, c.Groups = r.rows, r.cats, r.groups
	c.Total, c.Truncated, c.Sampled = r.total, r.truncated, r.sampled
	c.XValues, c.XDate, c.Edges, c.Boxes = r.xValues, r.xDate, r.edges, r.boxes
	c.Formats, c.XFormat = r.formats, r.xFormat
}

func (t *tx) chartData(cfg ChartConfig) (chartResult, error) {
	key := t.chartKey(cfg)
	calc := func() (chartResult, error) {
		work.Phase(t.Ctx, "chart")
		return t.chartDataRaw(cfg)
	}
	var r chartResult
	var err error
	if deps, ok := t.chartDeps(cfg.Source); ok {
		r, err = cachedOn(t, key, deps, calc)
	} else {
		r, err = cached(t, key, calc)
	}
	if err == nil {
		err = t.chartFormats(cfg, &r)
	}
	return r, err
}
