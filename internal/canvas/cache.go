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

// pivotDeps : les colonnes de la source que lit le calcul d'un TCD
// (cachecols.go), ou faux s'il peut lire d'autres tableaux (champ calculé,
// onglet à choix par une formule, colonne calculée de la source : formules,
// recherches…).
func (t *tx) pivotDeps(cfg PivotConfig) ([]string, bool) {
	tb, err := t.tableByName(cfg.Source)
	if err != nil || cfg.byFormula() {
		return nil, false
	}
	for _, c := range tb.Cols {
		if c.Formula != "" && pivotUses(cfg, c.Name) {
			return nil, false
		}
	}
	return colDeps(tb, cfg, false), true
}

// pivotUses : le TCD cite le champ name (zones, filtres, groupes, segments).
func pivotUses(cfg PivotConfig, name string) bool {
	names := append(append([]string{}, cfg.RowFields...), cfg.ColFields...)
	names = append(names, cfg.Cols)
	for _, v := range cfg.Values {
		names = append(names, v.Field)
	}
	for _, list := range [][]PivotFilter{cfg.Filters, cfg.Items, cfg.Linked} {
		for _, f := range list {
			names = append(names, f.Field)
		}
	}
	for _, g := range cfg.Groups {
		names = append(names, g.Field)
	}
	for _, s := range cfg.Slicers {
		names = append(names, s.Field)
	}
	return foldIndex(names, name) >= 0
}

// pivotCube calcule les agrégats d'un TCD (pivotcube.go), gardés en cache
// tant que sa source ne change pas.
func (t *tx) pivotCube(cfg PivotConfig) (*pivotCube, error) {
	cfg = cfg.normalized().cubed()
	key := keyOf("pivotcube:", cfg)
	run := func() (*pivotCube, error) {
		work.Phase(t.Ctx, "pivot")
		return t.pivotCubeRaw(cfg)
	}
	if deps, ok := t.pivotDeps(cfg); ok {
		return cachedOn(t, key, deps, run)
	}
	return cached(t, key, run)
}

// pivotFlat calcule un TCD à plat (une colonne par champ des lignes) : la
// vue SQL et les graphiques le lisent ainsi.
func (t *tx) pivotFlat(cfg PivotConfig) ([][]string, error) {
	cfg = cfg.flat()
	if deps, ok := t.pivotDeps(cfg); ok {
		return cachedOn(t, keyOf("pivot:", cfg), deps, func() ([][]string, error) { return t.pivotCellsRaw(cfg) })
	}
	return cached(t, keyOf("pivot:", cfg), func() ([][]string, error) { return t.pivotCellsRaw(cfg) })
}

// pivotCells calcule un TCD tel qu'affiché : en arborescence avec plusieurs
// champs en lignes (rows : une description par ligne sous l'en-tête), à plat
// sinon (rows nil).
func (t *tx) pivotCells(cfg PivotConfig) ([][]string, []PivotRowInfo, error) {
	v, err := t.pivotViewOf(cfg)
	return v.cells, v.rows, err
}

// pivotViewOf est pivotCells avec les en-têtes imbriqués des colonnes.
func (t *tx) pivotViewOf(cfg PivotConfig) (pivotView, error) {
	cfg.Hand = nil // colonnes saisies : ajoutées après le calcul (pivothand.go)
	cfg.Styles = nil
	cfg.Slicers = calcSlicers(cfg.Slicers) // leur place ne change pas le calcul
	calc := func() (pivotView, error) {
		work.Phase(t.Ctx, "pivot")
		return t.pivotViewRaw(cfg)
	}
	if deps, ok := t.pivotDeps(cfg); ok {
		return cachedOn(t, keyOf("pivotview:", cfg), deps, calc)
	}
	return cached(t, keyOf("pivotview:", cfg), calc)
}

// pivotLater dit si le calcul du TCD (pivotCells) parcourrait un grand
// tableau (plus de quickRows lignes) sans être en cache (LayoutFirst).
func (t *tx) pivotLater(cfg PivotConfig) bool {
	cfg.Hand, cfg.Styles = nil, nil
	cfg.Slicers = calcSlicers(cfg.Slicers)
	tb, err := t.tableByName(cfg.Source)
	if err != nil || tb.Rows <= quickRows {
		return false
	}
	key := keyOf("pivotview:", cfg)
	if deps, ok := t.pivotDeps(cfg); ok {
		_, ok := peekOn[pivotView](t, key, deps)
		return !ok
	}
	_, ok := peek[pivotView](t, key)
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
	if deps, ok := t.chartDeps(cfg); ok {
		_, hit := peekOn[chartResult](t, key, deps)
		return !hit
	}
	_, hit := peek[chartResult](t, key)
	return !hit
}

// chartDeps donne ce que lit un graphique : les colonnes de son tableau
// source nommées par ses axes et son filtre (cachecols.go), tout le tableau
// d'une carte à repérer ou sans axe, ce que lit le TCD source ; ou faux
// s'il peut lire d'autres tableaux (colonne calculée de la source :
// formules, REFERENCE…).
func (t *tx) chartDeps(cfg ChartConfig) ([]string, bool) {
	if tb, err := t.tableByName(cfg.Source); err == nil {
		for _, c := range tb.Cols {
			if c.Formula != "" {
				return nil, false
			}
		}
		if cfg.MapDraft || cfg.X == "" || len(cfg.Series) == 0 {
			return []string{tb.sql()}, true
		}
		return colDeps(tb, ChartConfig{Kind: cfg.Kind, X: cfg.X, Series: cfg.Series, MapCoords: cfg.MapCoords, ColorBy: cfg.ColorBy, Bins: cfg.Bins}, !cfg.Hidden), true
	}
	if pc, ok, err := t.pivotByName(cfg.Source); err == nil && ok {
		if deps, ok := t.pivotDeps(pc); ok {
			return deps, true
		}
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
	trends             []*TrendFit
	errors             []*ErrorValues
}

// fill reporte les données dans un graphique.
func (r chartResult) fill(c *ChartInfo) {
	c.Labels, c.Series, c.Invalid, c.Guess = r.labels, r.series, r.invalid, r.guess
	c.Rows, c.Categories, c.Groups = r.rows, r.cats, r.groups
	c.Total, c.Truncated, c.Sampled = r.total, r.truncated, r.sampled
	c.XValues, c.XDate, c.Edges, c.Boxes = r.xValues, r.xDate, r.edges, r.boxes
	c.Formats, c.XFormat = r.formats, r.xFormat
	c.Trends, c.Errors = r.trends, r.errors
}

func (t *tx) chartData(cfg ChartConfig) (chartResult, error) {
	key := t.chartKey(cfg)
	calc := func() (chartResult, error) {
		work.Phase(t.Ctx, "chart")
		return t.chartDataRaw(cfg)
	}
	var r chartResult
	var err error
	if deps, ok := t.chartDeps(cfg); ok {
		r, err = cachedOn(t, key, deps, calc)
	} else {
		r, err = cached(t, key, calc)
	}
	if err == nil {
		err = t.chartFormats(cfg, &r)
	}
	if err == nil {
		err = t.chartFits(cfg, &r)
	}
	return r, err
}
