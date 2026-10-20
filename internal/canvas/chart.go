package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// ChartConfig décrit un graphique alimenté par un tableau : l'axe des
// catégories (X) et une ou plusieurs séries de valeurs, par nom de colonne.
type ChartConfig struct {
	Source string   `json:"source"`
	Kind   string   `json:"kind"` // voir chartKinds
	X      string   `json:"x"`
	Series []string `json:"series"`
	Title  string   `json:"title,omitempty"`

	// Présentation (lue par l'interface seulement).
	// Décalage (px) dans la cellule d'ancrage : ancrage libre.
	OffX    int      `json:"offX,omitempty"`
	OffY    int      `json:"offY,omitempty"`
	Width   int      `json:"width,omitempty"`  // px, 480 par défaut
	Height  int      `json:"height,omitempty"` // px, 288 par défaut
	Legend  string   `json:"legend,omitempty"` // bottom (défaut), top, right, none
	XTitle  string   `json:"xTitle,omitempty"`
	YTitle  string   `json:"yTitle,omitempty"`
	Labels  bool     `json:"labels,omitempty"` // étiquettes de données
	NoGrid  bool     `json:"noGrid,omitempty"`
	Stacked bool     `json:"stacked,omitempty"`
	YMin    *float64 `json:"yMin,omitempty"`
	YMax    *float64 `json:"yMax,omitempty"`
	Colors  []string `json:"colors,omitempty"` // couleur de chaque série
	// Secondary : séries tracées sur l'axe secondaire (à droite, échelle
	// propre), comme Excel ; Y2Title est le titre de cet axe.
	Secondary []bool `json:"secondary,omitempty"`
	Y2Title   string `json:"y2Title,omitempty"`
	// SeriesKinds : graphique combiné, forme de chaque série (bar, line ou
	// area ; par défaut la première en barres, les suivantes en courbes).
	SeriesKinds []string `json:"seriesKinds,omitempty"`
	// MapMode : carte, « points » (défaut) ou « heat » (densité).
	MapMode string `json:"mapMode,omitempty"`
	// MapCoords : carte, coordonnées en deux colonnes (« pair », défaut :
	// séries latitude, longitude, valeur) ou en une (« single » : séries
	// coordonnées, valeur ; toute écriture standard, voir ParseCoord).
	MapCoords string `json:"mapCoords,omitempty"`
	// MapDraft : carte qui attend le choix de ses coordonnées (carré grisé,
	// rien n'est lu ; ChartInfo.Guess propose des colonnes).
	MapDraft bool `json:"mapDraft,omitempty"`
	// MapView : carte, vue choisie par l'utilisateur (centre et zoom) ;
	// absente, la vue s'ajuste aux points.
	MapView *MapView `json:"mapView,omitempty"`
	// ColorBy : carte de points, colonne dont chaque valeur distincte prend
	// sa couleur (ChartInfo.Categories et Groups) ; vide : une seule couleur.
	ColorBy string `json:"colorBy,omitempty"`
	// MapScale : carte de points, facteur de taille des points (0 ou absent :
	// 1 ; la valeur règle déjà la taille relative).
	MapScale float64 `json:"mapScale,omitempty"`
	// Bins : histogramme de fréquences, nombre de classes (0 : automatique).
	Bins int `json:"bins,omitempty"`
	// Hidden : lit aussi les lignes masquées par le filtre du tableau (option
	// d'Excel « Afficher les données des lignes masquées ») ; par défaut, un
	// graphique ne montre que les lignes affichées.
	Hidden bool `json:"hidden,omitempty"`
	// Filters : graphique tiré d'un tableau, ne lit que les lignes dont la
	// colonne field affiche l'une des valeurs (comme le filtre de rapport
	// d'un TCD) ; posé par Regrouper des onglets sur un graphique qui lisait
	// un ancien onglet (Mois = Janvier).
	Filters []PivotFilter `json:"filters,omitempty"`
	// Blanks : courbes et aires, cellule vide en intervalle ("", comme
	// Excel), à zéro ("zero") ou reliée aux points voisins ("span").
	Blanks string `json:"blanks,omitempty"`
}

// MapView : vue d'une carte, centre (degrés) et niveau de zoom (fractionnaire,
// celui des tuiles OpenStreetMap : 0 montre le monde entier).
type MapView struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Zoom float64 `json:"zoom"`
}

// ChartSeries est une série calculée (nil : valeur absente).
type ChartSeries struct {
	Name   string     `json:"name"`
	Values []*float64 `json:"values"`
}

// ChartInfo est un graphique posé sur une feuille, avec ses données.
type ChartInfo struct {
	ID     int64         `json:"id"`
	Row    int           `json:"row"`
	Col    int           `json:"col"`
	Config ChartConfig   `json:"config"`
	Labels []string      `json:"labels"`
	Series []ChartSeries `json:"series"`
	Error  string        `json:"error"`
	// Invalid : carte, valeurs de coordonnées illisibles ou hors bornes,
	// ignorées.
	Invalid int `json:"invalid,omitempty"`
	// Guess : carte, colonnes de coordonnées repérées (proposées par le volet).
	Guess *MapGuess `json:"guess,omitempty"`
	// Rows : carte tirée d'un tableau, identifiant (id) de la ligne de chaque
	// point, pour aller à la ligne d'un clic (LocateRow).
	Rows []int64 `json:"rows,omitempty"`
	// Categories : carte colorée (ColorBy), valeurs distinctes triées
	// (nombres, puis texte sans tenir compte de la casse) ; Groups : rang de
	// la valeur de chaque point dans Categories, -1 pour une cellule vide.
	Categories []string `json:"categories,omitempty"`
	Groups     []int    `json:"groups,omitempty"`
	// Total : points de la source (lignes lues une à une, ou catégories une
	// fois regroupées) ; Truncated : seuls les MaxChartPoints premiers sont
	// tracés ; Sampled : courbe réduite à MaxChartPoints points, en gardant
	// le premier, le dernier, le minimum et le maximum de chaque intervalle.
	Total     int  `json:"total,omitempty"`
	Truncated bool `json:"truncated,omitempty"`
	Sampled   bool `json:"sampled,omitempty"`
	// XValues : X lu une ligne à une ligne, valeur numérique de chaque point
	// (nombre, ou numéro de série d'une date ; nil sinon) ; XDate : X est
	// une colonne de dates (axe de temps).
	XValues []*float64 `json:"xValues,omitempty"`
	XDate   bool       `json:"xDate,omitempty"`
	// Edges : fréquences, bornes des classes calculées sur toutes les lignes
	// (Series : effectif de chaque classe).
	Edges []float64 `json:"edges,omitempty"`
	// Boxes : boîtes à moustaches calculées sur toutes les lignes, une ligne
	// par libellé (Labels) et une colonne par série (nil : aucune valeur).
	Boxes [][]*BoxStats `json:"boxes,omitempty"`
	// Formats : format de nombre de chaque série (style JSON nf, dp, df,
	// ng, celui de la colonne source) ; XFormat : celui de X.
	Formats []string `json:"formats,omitempty"`
	XFormat string   `json:"xFormat,omitempty"`
}

// BoxStats : statistiques d'une boîte à moustaches (quartiles interpolés
// comme QUARTILE.INCLURE ; moustaches au plus à 1,5 écart interquartile).
// Outliers liste au plus 100 valeurs extrêmes de chaque côté ; Out les compte toutes.
type BoxStats struct {
	Min      float64   `json:"min"`
	Q1       float64   `json:"q1"`
	Median   float64   `json:"median"`
	Q3       float64   `json:"q3"`
	Max      float64   `json:"max"`
	Mean     float64   `json:"mean"`
	Outliers []float64 `json:"outliers"`
	N        int       `json:"n"`
	Out      int       `json:"out"`
}

// ErrChart signale un graphique impossible.
var ErrChart = errors.New("graphique")

// MaxChartPoints borne les points d'un graphique.
const MaxChartPoints = 2000

// Taille minimale d'un graphique (pixels), à la souris comme par l'API.
const (
	MinChartWidth  = 160
	MinChartHeight = 120
)

// chartKinds : types de graphiques. bar (barres verticales), column
// (horizontales), line, area, pie, doughnut, scatter ; combo (barres et
// courbes, SeriesKinds) ; bar3d, pie3d (perspective) ; map (carte
// OpenStreetMap : séries latitude, longitude et valeur facultative) ;
// waterfall (cascade), treemap (compartimentage), radar, funnel (entonnoir),
// boxplot (boîte à moustaches), histogram (fréquences), gauge (jauge).
var chartKinds = map[string]bool{
	"bar": true, "column": true, "line": true, "area": true, "pie": true, "doughnut": true, "scatter": true,
	"combo": true, "bar3d": true, "pie3d": true, "map": true,
	"waterfall": true, "treemap": true, "radar": true, "funnel": true, "boxplot": true, "histogram": true, "gauge": true,
}

// chartRawRows : types qui lisent les lignes du tableau une à une, sans
// regrouper par catégorie (coordonnées, distributions).
var chartRawRows = map[string]bool{"map": true, "histogram": true, "boxplot": true}

// chartSeriesKinds : formes possibles d'une série d'un graphique combiné.
var chartSeriesKinds = map[string]bool{"": true, "bar": true, "line": true, "area": true}

// checkChart vérifie ce qui ne dépend pas des données.
func checkChart(cfg ChartConfig) error {
	if !chartKinds[cfg.Kind] {
		return msg.Wrap(ErrChart, "chart.unknownKind", msg.Args{"kind": fmt.Sprintf("%q", cfg.Kind)})
	}
	draft := cfg.Kind == "map" && cfg.MapDraft
	if len(cfg.Series) == 0 && !draft {
		return msg.Wrap(ErrChart, "chart.chooseSeries", nil)
	}
	for _, k := range cfg.SeriesKinds {
		if !chartSeriesKinds[k] {
			return msg.Wrap(ErrChart, "chart.unknownSeriesKind", msg.Args{"kind": fmt.Sprintf("%q", k)})
		}
	}
	if cfg.MapCoords != "" && cfg.MapCoords != "pair" && cfg.MapCoords != "single" {
		return msg.Wrap(ErrChart, "chart.unknownMapCoords", msg.Args{"coords": fmt.Sprintf("%q", cfg.MapCoords)})
	}
	if cfg.Kind == "map" && !draft && cfg.MapCoords != "single" && len(cfg.Series) < 2 {
		return msg.Wrap(ErrChart, "chart.mapNeedsTwoSeries", nil)
	}
	if cfg.MapMode != "" && cfg.MapMode != "points" && cfg.MapMode != "heat" {
		return msg.Wrap(ErrChart, "chart.unknownMapMode", msg.Args{"mode": fmt.Sprintf("%q", cfg.MapMode)})
	}
	if cfg.Blanks != "" && cfg.Blanks != "gap" && cfg.Blanks != "zero" && cfg.Blanks != "span" {
		return fmt.Errorf("%w : cellules vides %q inconnues (gap, zero ou span)", ErrChart, cfg.Blanks)
	}
	if cfg.Bins < 0 || cfg.Bins > 200 {
		return msg.Wrap(ErrChart, "chart.binsRange", nil)
	}
	return nil
}

// CreateChart pose un graphique sur une feuille, coin haut gauche en (row, col).
func (m *Model) CreateChart(ctx context.Context, sheet int64, row, col int, cfg ChartConfig) (int64, error) {
	var id int64
	err := m.Do(ctx, "Insérer un graphique", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		if _, err := t.chartData(cfg); err != nil {
			return err
		}
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_charts`).Scan(&id); err != nil {
			return err
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`INSERT INTO _lattis_charts (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, string(b))},
			[]stmt{st(`DELETE FROM _lattis_charts WHERE id = ?`, id)},
		)
	})
	return id, err
}

// SetChart change la configuration d'un graphique.
func (m *Model) SetChart(ctx context.Context, id int64, cfg ChartConfig) error {
	return m.Do(ctx, "Modifier le graphique", func(t *tx) error {
		if err := t.guardObject(`_lattis_charts`, id); err != nil {
			return err
		}
		if _, err := t.chartData(cfg); err != nil {
			return err
		}
		var old string
		if err := t.row(`SELECT config FROM _lattis_charts WHERE id = ?`, id).Scan(&old); err != nil {
			return err
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		q := `UPDATE _lattis_charts SET config = ? WHERE id = ?`
		return t.apply([]stmt{st(q, string(b), id)}, []stmt{st(q, old, id)})
	})
}

// MoveChart déplace un graphique.
func (m *Model) MoveChart(ctx context.Context, id int64, row, col int) error {
	if row < 0 || col < 0 {
		return msg.Wrap(ErrChart, "chart.outOfSheet", nil)
	}
	return m.Do(ctx, "Déplacer le graphique", func(t *tx) error {
		if err := t.guardObject(`_lattis_charts`, id); err != nil {
			return err
		}
		var r, c int
		if err := t.row(`SELECT row, col FROM _lattis_charts WHERE id = ?`, id).Scan(&r, &c); err != nil {
			return err
		}
		if r == row && c == col {
			return nil
		}
		q := `UPDATE _lattis_charts SET row = ?, col = ? WHERE id = ?`
		return t.apply([]stmt{st(q, row, col, id)}, []stmt{st(q, r, c, id)})
	})
}

// DeleteChart supprime un graphique.
func (m *Model) DeleteChart(ctx context.Context, id int64) error {
	return m.Do(ctx, "Supprimer le graphique", func(t *tx) error {
		if err := t.guardObject(`_lattis_charts`, id); err != nil {
			return err
		}
		var sheet int64
		var row, col int
		var cfg string
		if err := t.row(`SELECT sheet, row, col, config FROM _lattis_charts WHERE id = ?`, id).Scan(&sheet, &row, &col, &cfg); err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`DELETE FROM _lattis_charts WHERE id = ?`, id)},
			[]stmt{st(`INSERT INTO _lattis_charts (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg)},
		)
	})
}

// chartData lit les données d'un graphique. Un axe X de texte (ou de
// booléens) regroupe les lignes par catégorie et additionne les séries,
// comme un graphique tiré d'un tableau croisé ; un axe numérique ou de dates
// prend les lignes dans l'ordre du tableau.
func (t *tx) chartDataRaw(cfg ChartConfig) (chartResult, error) {
	if err := checkChart(cfg); err != nil {
		return chartResult{}, err
	}
	tb, err := t.tableByName(cfg.Source)
	if err != nil {
		if pc, ok, perr := t.pivotByName(cfg.Source); perr == nil && ok {
			if cfg.Kind == "map" && cfg.MapDraft {
				return chartResult{labels: []string{}, series: []ChartSeries{}, guess: &MapGuess{}}, nil
			}
			if cfg.Kind == "map" && cfg.MapCoords == "single" {
				return chartResult{}, msg.Wrap(ErrChart, "chart.singleColumnNeedsTable", nil)
			}
			return t.chartFromPivot(cfg, pc)
		}
		return chartResult{}, msg.Wrap(ErrChart, "chart.tableNotFound", msg.Args{"table": cfg.Source})
	}
	if cfg.Kind == "map" {
		return t.mapData(cfg, tb)
	}
	return t.chartTable(cfg, tb)
}

// chartSource : lignes lues par un graphique, celles qu'affiche le filtre du
// tableau (toutes avec Hidden), parmi celles que retiennent ses propres
// filtres (Filters ; une colonne introuvable n'en retient aucune).
func chartSource(cfg ChartConfig, tb Table) string {
	var conds []string
	where := tb.where
	if tb.choice != nil {
		// Un filtre du graphique sur la colonne de l'onglet à choix l'emporte
		// sur le choix de l'onglet (Janvier reste Janvier quand l'onglet
		// montre Mars).
		for _, f := range cfg.Filters {
			if strings.EqualFold(f.Field, tb.choice.Col.Name) && len(f.Values) > 0 {
				where = tb.userWhere
			}
		}
	}
	if !cfg.Hidden && where != "" {
		conds = append(conds, "("+where+")")
	}
	for _, f := range cfg.Filters {
		if len(f.Values) == 0 {
			continue
		}
		k := colIndex(tb, f.Field)
		if k < 0 {
			conds = append(conds, "0")
			continue
		}
		lits := make([]string, len(f.Values))
		for i, v := range f.Values {
			lits[i] = sqlLiteral(v)
		}
		conds = append(conds, textExpr(tb.Cols[k])+" IN ("+strings.Join(lits, ", ")+")")
	}
	if len(conds) == 0 {
		return tb.sql()
	}
	return "(SELECT * FROM " + tb.sql() + " WHERE " + strings.Join(conds, " AND ") + ") AS " + tb.sql()
}

// numericSQL : la valeur v si c'est un nombre, NULL sinon (un texte dans
// une colonne de nombres ne compte ni pour 0 ni pour une valeur).
func numericSQL(v string) string {
	return "CASE WHEN typeof(" + v + ") IN ('integer', 'real') THEN " + v + " END"
}

// chartPoint : valeur d'une série (nil : absente ou pas un nombre).
func chartPoint(r any) *float64 {
	if v := formula.FromResult(r); v.Kind == formula.Number {
		f := v.Num
		return &f
	}
	return nil
}

// chartTable lit les données d'un graphique dans un tableau, sur les
// lignes affichées par son filtre (chartSource).
func (t *tx) chartTable(cfg ChartConfig, tb Table) (chartResult, error) {
	exprs, args, _, err := t.columnSQL(tb)
	if err != nil {
		return chartResult{}, err
	}
	find := func(name string) (int, error) {
		for i, c := range tb.Cols {
			if strings.EqualFold(c.Name, name) {
				return i, nil
			}
		}
		return 0, msg.Wrap(ErrChart, "chart.columnNotFound", msg.Args{"column": name, "table": tb.Name})
	}
	xi, err := find(cfg.X)
	if err != nil {
		return chartResult{}, err
	}
	out := chartResult{labels: []string{}, series: make([]ChartSeries, len(cfg.Series))}
	var vals []string
	for k, s := range cfg.Series {
		i, err := find(s)
		if err != nil {
			return chartResult{}, err
		}
		out.series[k] = ChartSeries{Name: tb.Cols[i].Name, Values: []*float64{}}
		vals = append(vals, exprs[i])
	}
	xc := tb.Cols[xi]
	// Type de X : celui de la colonne, ou du résultat d'une colonne calculée.
	src := chartSource(cfg, tb)
	x := exprs[xi]
	xType := xc.Type
	if xc.Formula != "" {
		if xType = t.computedResult(tb, xc); xType == "" {
			// Résultat non numérique deviné sur la formule : texte si les
			// valeurs le sont (regroupées comme une colonne de texte).
			var kind sql.NullString
			if err := t.rowAll(`SELECT typeof(`+x+`) FROM `+src+` WHERE `+x+` IS NOT NULL`, args).Scan(&kind); err == nil && kind.String == "text" {
				xType = grid.Text
			}
		}
	}
	// Paramètres : ceux des colonnes calculées (?1…?n), puis les nôtres.
	next := func() string { args = append(args, nil); return "?" + fmt.Sprint(len(args)) }
	switch cfg.Kind {
	case "histogram":
		return t.chartHistogram(cfg, out, src, vals, args)
	case "boxplot":
		return t.chartBoxes(out, src, x, xType, vals, args)
	}
	cols := make([]string, len(vals))
	for k, v := range vals {
		cols[k] = fmt.Sprintf("%s AS v%d", v, k)
	}
	// Un nuage de points prend les lignes une à une, même avec un X de texte (comme Excel).
	grouped := (xType == grid.Text || xType == grid.Boolean) && !chartRawRows[cfg.Kind] && cfg.Kind != "scatter"
	var q string
	if grouped {
		// Une catégorie par valeur de X, séries additionnées.
		key := fmt.Sprintf("coalesce(slt_text(%s, %s), '')", x, sqlLiteral(string(xType)))
		if xType == grid.Text {
			key = "coalesce(" + x + ", '')"
		}
		inner := `SELECT ` + key + ` AS k, ` + strings.Join(cols, ", ") + `, _pos FROM ` + src
		sums := make([]string, len(vals))
		for k := range vals {
			sums[k] = "sum(" + numericSQL(fmt.Sprintf("v%d", k)) + ")"
		}
		lim := next()
		q = `SELECT k, ` + strings.Join(sums, ", ") + ` FROM (` + inner + `) GROUP BY k ORDER BY min(_pos) LIMIT ` + lim
		args[len(args)-1] = MaxChartPoints + 1
		if err := t.rowAll(`SELECT count(DISTINCT k) FROM (`+inner+`)`, args[:len(args)-1]).Scan(&out.total); err != nil {
			return chartResult{}, err
		}
	} else {
		out.total = tb.Shown
		if cfg.Hidden {
			out.total = tb.Rows
		}
		inner := `SELECT ` + x + ` AS x0, ` + strings.Join(cols, ", ") + `, _pos FROM ` + src
		vs := make([]string, len(vals))
		for k := range vals {
			vs[k] = fmt.Sprintf("v%d", k)
		}
		sel := `x0, ` + strings.Join(vs, ", ")
		if (cfg.Kind == "line" || cfg.Kind == "area") && out.total > MaxChartPoints {
			// Courbe trop longue : réduite par intervalles, en gardant le
			// premier et le dernier point de chacun et les extrêmes de
			// chaque série (les pics restent visibles).
			buckets := max(1, MaxChartPoints/(2+2*len(vals)))
			keep := []string{`SELECT min(i) FROM b GROUP BY g`, `SELECT max(i) FROM b GROUP BY g`}
			for _, v := range vs {
				for _, f := range []string{"min", "max"} {
					keep = append(keep, fmt.Sprintf(`SELECT i FROM (SELECT i, %s(%s) FROM b WHERE typeof(%s) IN ('integer', 'real') GROUP BY g)`, f, v, v))
				}
			}
			q = `WITH r AS (SELECT row_number() OVER (ORDER BY _pos) - 1 AS i, ` + sel + ` FROM (` + inner + `)), ` +
				fmt.Sprintf(`b AS (SELECT i, i * %d / %d AS g, `, buckets, out.total) + strings.Join(vs, ", ") + ` FROM r) ` +
				`SELECT ` + sel + ` FROM r WHERE i IN (` + strings.Join(keep, " UNION ") + `) ORDER BY i LIMIT ` + next()
			out.sampled = true
		} else {
			q = `SELECT ` + sel + ` FROM (` + inner + `) ORDER BY _pos LIMIT ` + next()
		}
		args[len(args)-1] = MaxChartPoints + 1
		out.xDate = xType == grid.Date
		if xType == grid.Integer || xType == grid.Decimal || xType == grid.Date {
			out.xValues = []*float64{}
		}
	}
	rs, err := t.query(q, args...)
	if err != nil {
		return chartResult{}, err
	}
	defer rs.Close()
	for rs.Next() {
		if len(out.labels) == MaxChartPoints {
			out.truncated = true
			break
		}
		row := make([]any, len(vals)+1)
		ptrs := make([]any, len(row))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return chartResult{}, err
		}
		lv := formula.FromResult(row[0])
		if xType == grid.Date && lv.Kind == formula.Number {
			lv = formula.DateSerial(lv.Num)
		}
		out.labels = append(out.labels, lv.String())
		if out.xValues != nil {
			xv := chartPoint(row[0])
			// Une colonne de dates range ses valeurs en texte ISO : numéro de série.
			if f, ok := formula.ParseDate(lv.Str); xv == nil && xType == grid.Date && lv.Kind == formula.Text && ok {
				xv = &f
			}
			out.xValues = append(out.xValues, xv)
		}
		for k := range vals {
			out.series[k].Values = append(out.series[k].Values, chartPoint(row[k+1]))
		}
	}
	return out, rs.Err()
}

// chartFormats reporte le format des colonnes d'un tableau source (séries
// et X) ; lu hors du cache des données : changer le format d'une colonne
// ne touche pas au tableau.
func (t *tx) chartFormats(cfg ChartConfig, r *chartResult) error {
	if cfg.Kind == "map" {
		return nil
	}
	tb, err := t.tableByName(cfg.Source)
	if err != nil {
		return nil // TCD : formats déjà lus avec les données
	}
	col := func(name string) (Column, bool) {
		for _, c := range tb.Cols {
			if strings.EqualFold(c.Name, name) {
				return c, true
			}
		}
		return Column{}, false
	}
	r.formats = make([]string, len(cfg.Series))
	for k, s := range cfg.Series {
		if c, ok := col(s); ok {
			if r.formats[k], err = t.columnFormat(tb, c); err != nil {
				return err
			}
		}
	}
	r.xFormat = ""
	if c, ok := col(cfg.X); ok {
		r.xFormat, err = t.columnFormat(tb, c)
	}
	return err
}

// columnFormat : format de nombre d'une colonne (nf, dp, df, ng de son
// style), en JSON ; une colonne de dates sans format s'affiche en date.
func (t *tx) columnFormat(tb Table, c Column) (string, error) {
	var raw string
	err := t.row(`SELECT style FROM _lattis_styles WHERE tbl = ? AND rid = 0 AND col = ?`, tb.ID, c.ID).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	f := map[string]any{}
	var st map[string]any
	if raw != "" && json.Unmarshal([]byte(raw), &st) == nil {
		for _, k := range []string{"nf", "dp", "df", "ng"} {
			if x, ok := st[k]; ok && x != nil {
				f[k] = x
			}
		}
	}
	if len(f) == 0 && c.Formula == "" && c.Type == grid.Date {
		f["nf"] = "date"
	}
	if len(f) == 0 {
		return "", nil
	}
	b, err := json.Marshal(f)
	return string(b), err
}

// charts donne les graphiques d'une feuille, avec leurs données.
func (t *tx) charts(sheet int64) ([]ChartInfo, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_charts WHERE sheet = ? ORDER BY id`, sheet)
	if err != nil {
		return nil, err
	}
	var out []ChartInfo
	for rs.Next() {
		var c ChartInfo
		var cfg string
		if err := rs.Scan(&c.ID, &c.Row, &c.Col, &cfg); err != nil {
			rs.Close()
			return nil, err
		}
		if err := json.Unmarshal([]byte(cfg), &c.Config); err != nil {
			c.Error = "configuration illisible"
		}
		out = append(out, c)
	}
	rs.Close()
	for i := range out {
		if out[i].Error != "" {
			continue
		}
		r, err := t.chartData(out[i].Config)
		if err != nil {
			out[i].Error = strings.TrimPrefix(err.Error(), ErrChart.Error()+" : ")
			continue
		}
		r.fill(&out[i])
	}
	return out, nil
}

// chartRows donne l'inverse de la suppression des graphiques d'une feuille.
func (t *tx) chartRows(sheet int64) ([]stmt, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_charts WHERE sheet = ?`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var undo []stmt
	for rs.Next() {
		var id int64
		var row, col int
		var cfg string
		if err := rs.Scan(&id, &row, &col, &cfg); err != nil {
			return nil, err
		}
		undo = append(undo, st(`INSERT INTO _lattis_charts (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg))
	}
	return undo, rs.Err()
}

// chartFromPivot lit les données d'un graphique dans un tableau croisé : X
// est une colonne de lignes, les séries des colonnes de mesures (en-têtes).
func (t *tx) chartFromPivot(cfg ChartConfig, pc PivotConfig) (chartResult, error) {
	cells, err := t.pivotBody(pc)
	if err != nil {
		return chartResult{}, fmt.Errorf("%w : %s", ErrChart, strings.TrimPrefix(err.Error(), ErrPivot.Error()+" : "))
	}
	header := cells[0]
	find := func(name string) (int, error) {
		for i, h := range header {
			if strings.EqualFold(h, name) {
				return i, nil
			}
		}
		return 0, msg.Wrap(ErrChart, "chart.columnNotFound", msg.Args{"column": name, "table": pc.Name})
	}
	xi, err := find(cfg.X)
	if err != nil {
		return chartResult{}, err
	}
	series := make([]ChartSeries, len(cfg.Series))
	idx := make([]int, len(cfg.Series))
	for k, s := range cfg.Series {
		if idx[k], err = find(s); err != nil {
			return chartResult{}, err
		}
		series[k].Name = header[idx[k]]
	}
	out := chartResult{total: len(cells) - 1, truncated: len(cells)-1 > MaxChartPoints}
	// Format de chaque mesure : celui du TCD (colonne source, ou deux décimales).
	if fs, err := t.pivotFormats(pc.normalized().flat(), cells); err == nil && fs != nil {
		out.formats = make([]string, len(idx))
		for k, i := range idx {
			if i < len(fs) {
				out.formats[k] = fs[i]
			}
		}
	}
	var labels []string
	for _, r := range cells[1:] {
		if len(labels) >= MaxChartPoints {
			break
		}
		// X sur un champ de lignes : avec les champs suivants (« FR · Lyon »).
		if nk := len(pc.normalized().RowFields); xi < nk {
			labels = append(labels, strings.Join(r[xi:nk], " · "))
		} else {
			labels = append(labels, r[xi])
		}
		for k, i := range idx {
			if f, ok := parseShown(r[i]); ok {
				series[k].Values = append(series[k].Values, &f)
			} else {
				series[k].Values = append(series[k].Values, nil)
			}
		}
	}
	out.labels, out.series = labels, series
	return out, nil
}

// PlaceChart pose un graphique au pixel près : cellule d'ancrage, décalage
// dans la cellule (px) et taille (px). Déplacer et redimensionner par
// n'importe quel bord ne fait qu'une étape d'annulation.
func (m *Model) PlaceChart(ctx context.Context, id int64, row, col, offX, offY, width, height int) error {
	if row < 0 || col < 0 {
		return msg.Wrap(ErrChart, "chart.positionOutOfSheet", nil)
	}
	if width < MinChartWidth || height < MinChartHeight {
		return fmt.Errorf("%w : taille minimale %d × %d pixels", ErrChart, MinChartWidth, MinChartHeight)
	}
	return m.Do(ctx, "Déplacer le graphique", func(t *tx) error {
		if err := t.guardObject(`_lattis_charts`, id); err != nil {
			return err
		}
		var r, c int
		var old string
		if err := t.row(`SELECT row, col, config FROM _lattis_charts WHERE id = ?`, id).Scan(&r, &c, &old); err != nil {
			return err
		}
		var cfg ChartConfig
		if err := json.Unmarshal([]byte(old), &cfg); err != nil {
			return err
		}
		cfg.OffX, cfg.OffY, cfg.Width, cfg.Height = offX, offY, width, height
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		q := `UPDATE _lattis_charts SET row = ?, col = ?, config = ? WHERE id = ?`
		return t.apply([]stmt{st(q, row, col, string(b), id)}, []stmt{st(q, r, c, old, id)})
	})
}

// ChartPlace donne la cellule d'ancrage et la configuration d'un graphique.
func (m *Model) ChartPlace(ctx context.Context, id int64) (row, col int, cfg ChartConfig, err error) {
	err = m.read(ctx, func(t *tx) error {
		var raw string
		if err := t.row(`SELECT row, col, config FROM _lattis_charts WHERE id = ?`, id).Scan(&row, &col, &raw); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return msg.Wrap(ErrChart, "chart.notFound", msg.Args{"id": id})
			}
			return err
		}
		return json.Unmarshal([]byte(raw), &cfg)
	})
	return
}

// ChartData calcule les données d'un graphique qui n'est posé sur aucune
// feuille (onglet Document) ; une erreur est rendue dans Error.
func (m *Model) ChartData(ctx context.Context, cfg ChartConfig) ChartInfo {
	out := ChartInfo{Config: cfg, Labels: []string{}, Series: []ChartSeries{}}
	err := m.read(ctx, func(t *tx) error {
		r, err := t.chartData(cfg)
		if err == nil {
			r.fill(&out)
		}
		return err
	})
	if err != nil {
		out.Error = strings.TrimPrefix(err.Error(), ErrChart.Error()+" : ")
	}
	return out
}
