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
	// Bins : histogramme de fréquences, nombre de classes (0 : automatique).
	Bins int `json:"bins,omitempty"`
	// Hidden : lit aussi les lignes masquées par le filtre du tableau (option
	// d'Excel « Afficher les données des lignes masquées ») ; par défaut, un
	// graphique ne montre que les lignes affichées.
	Hidden bool `json:"hidden,omitempty"`
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
		return fmt.Errorf("%w : type %q inconnu", ErrChart, cfg.Kind)
	}
	draft := cfg.Kind == "map" && cfg.MapDraft
	if len(cfg.Series) == 0 && !draft {
		return fmt.Errorf("%w : choisissez au moins une série", ErrChart)
	}
	for _, k := range cfg.SeriesKinds {
		if !chartSeriesKinds[k] {
			return fmt.Errorf("%w : forme de série %q inconnue (bar, line ou area)", ErrChart, k)
		}
	}
	if cfg.MapCoords != "" && cfg.MapCoords != "pair" && cfg.MapCoords != "single" {
		return fmt.Errorf("%w : coordonnées %q inconnues (pair ou single)", ErrChart, cfg.MapCoords)
	}
	if cfg.Kind == "map" && !draft && cfg.MapCoords != "single" && len(cfg.Series) < 2 {
		return fmt.Errorf("%w : une carte demande deux séries, latitude puis longitude (et une valeur facultative)", ErrChart)
	}
	if cfg.MapMode != "" && cfg.MapMode != "points" && cfg.MapMode != "heat" {
		return fmt.Errorf("%w : mode de carte %q inconnu (points ou heat)", ErrChart, cfg.MapMode)
	}
	if cfg.Bins < 0 || cfg.Bins > 200 {
		return fmt.Errorf("%w : nombre de classes entre 1 et 200", ErrChart)
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
		return fmt.Errorf("%w : hors de la feuille", ErrChart)
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
				return chartResult{}, fmt.Errorf("%w : coordonnées en une colonne : choisissez un tableau", ErrChart)
			}
			l, s, err := t.chartFromPivot(cfg, pc)
			return chartResult{labels: l, series: s}, err
		}
		return chartResult{}, fmt.Errorf("%w : tableau « %s » introuvable", ErrChart, cfg.Source)
	}
	if cfg.Kind == "map" {
		return t.mapData(cfg, tb)
	}
	l, s, err := t.chartTable(cfg, tb)
	return chartResult{labels: l, series: s}, err
}

// chartSource : lignes lues par un graphique, celles qu'affiche le filtre du
// tableau (toutes avec Hidden).
func chartSource(cfg ChartConfig, tb Table) string {
	if cfg.Hidden {
		return tb.sql()
	}
	return tb.view()
}

// chartTable lit les données d'un graphique dans un tableau.
func (t *tx) chartTable(cfg ChartConfig, tb Table) ([]string, []ChartSeries, error) {
	exprs, args, _, err := t.columnSQL(tb)
	if err != nil {
		return nil, nil, err
	}
	find := func(name string) (int, error) {
		for i, c := range tb.Cols {
			if strings.EqualFold(c.Name, name) {
				return i, nil
			}
		}
		return 0, fmt.Errorf("%w : colonne « %s » introuvable dans %s", ErrChart, name, tb.Name)
	}
	xi, err := find(cfg.X)
	if err != nil {
		return nil, nil, err
	}
	var vals []string
	series := make([]ChartSeries, len(cfg.Series))
	for k, s := range cfg.Series {
		i, err := find(s)
		if err != nil {
			return nil, nil, err
		}
		series[k].Name = tb.Cols[i].Name
		vals = append(vals, exprs[i])
	}
	xc := tb.Cols[xi]
	x := exprs[xi]
	var q string
	if xc.Formula == "" && (xc.Type == grid.Text || xc.Type == grid.Boolean) && !chartRawRows[cfg.Kind] {
		sums := make([]string, len(vals))
		for k, v := range vals {
			sums[k] = "sum(" + v + ")"
		}
		key := fmt.Sprintf("coalesce(slt_text(%s, %s), '')", x, sqlLiteral(string(xc.Type)))
		if xc.Type == grid.Text {
			key = "coalesce(" + x + ", '')"
		}
		q = `SELECT ` + key + ` AS k, ` + strings.Join(sums, ", ") + ` FROM ` + chartSource(cfg, tb) + ` GROUP BY k ORDER BY min(_pos) LIMIT ?` + fmt.Sprint(len(args)+1)
	} else {
		q = `SELECT ` + x + `, ` + strings.Join(vals, ", ") + ` FROM ` + chartSource(cfg, tb) + ` ORDER BY _pos LIMIT ?` + fmt.Sprint(len(args)+1)
	}
	rs, err := t.query(q, append(args, MaxChartPoints)...)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	var labels []string
	for rs.Next() {
		row := make([]any, len(vals)+1)
		ptrs := make([]any, len(row))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		lv := formula.FromResult(row[0])
		if xc.Formula == "" && xc.Type == grid.Date && lv.Kind == formula.Number {
			lv = formula.DateSerial(lv.Num)
		}
		labels = append(labels, lv.String())
		for k := range vals {
			v := formula.FromResult(row[k+1])
			if v.Kind == formula.Number {
				f := v.Num
				series[k].Values = append(series[k].Values, &f)
			} else {
				series[k].Values = append(series[k].Values, nil)
			}
		}
	}
	return labels, series, rs.Err()
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
func (t *tx) chartFromPivot(cfg ChartConfig, pc PivotConfig) ([]string, []ChartSeries, error) {
	cells, err := t.pivotBody(pc)
	if err != nil {
		return nil, nil, fmt.Errorf("%w : %s", ErrChart, strings.TrimPrefix(err.Error(), ErrPivot.Error()+" : "))
	}
	header := cells[0]
	find := func(name string) (int, error) {
		for i, h := range header {
			if strings.EqualFold(h, name) {
				return i, nil
			}
		}
		return 0, fmt.Errorf("%w : colonne « %s » introuvable dans %s", ErrChart, name, pc.Name)
	}
	xi, err := find(cfg.X)
	if err != nil {
		return nil, nil, err
	}
	series := make([]ChartSeries, len(cfg.Series))
	idx := make([]int, len(cfg.Series))
	for k, s := range cfg.Series {
		if idx[k], err = find(s); err != nil {
			return nil, nil, err
		}
		series[k].Name = header[idx[k]]
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
	return labels, series, nil
}

// PlaceChart pose un graphique au pixel près : cellule d'ancrage, décalage
// dans la cellule (px) et taille (px). Déplacer et redimensionner par
// n'importe quel bord ne fait qu'une étape d'annulation.
func (m *Model) PlaceChart(ctx context.Context, id int64, row, col, offX, offY, width, height int) error {
	if row < 0 || col < 0 {
		return fmt.Errorf("%w : position hors de la feuille", ErrChart)
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
				return fmt.Errorf("%w : graphique %d introuvable", ErrChart, id)
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
