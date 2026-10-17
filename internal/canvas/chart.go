package canvas

import (
	"context"
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
	Kind   string   `json:"kind"` // bar (barres verticales), column (horizontales), line, area, pie, doughnut, scatter
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
}

// ErrChart signale un graphique impossible.
var ErrChart = errors.New("graphique")

// MaxChartPoints borne les points d'un graphique.
const MaxChartPoints = 2000

var chartKinds = map[string]bool{"bar": true, "column": true, "line": true, "area": true, "pie": true, "doughnut": true, "scatter": true}

// CreateChart pose un graphique sur une feuille, coin haut gauche en (row, col).
func (m *Model) CreateChart(ctx context.Context, sheet int64, row, col int, cfg ChartConfig) (int64, error) {
	var id int64
	err := m.Do(ctx, "Insérer un graphique", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		if _, _, err := t.chartData(cfg); err != nil {
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
		if _, _, err := t.chartData(cfg); err != nil {
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
func (t *tx) chartDataRaw(cfg ChartConfig) ([]string, []ChartSeries, error) {
	if !chartKinds[cfg.Kind] {
		return nil, nil, fmt.Errorf("%w : type %q inconnu", ErrChart, cfg.Kind)
	}
	if len(cfg.Series) == 0 {
		return nil, nil, fmt.Errorf("%w : choisissez au moins une série", ErrChart)
	}
	tb, err := t.tableByName(cfg.Source)
	if err != nil {
		if pc, ok, perr := t.pivotByName(cfg.Source); perr == nil && ok {
			return t.chartFromPivot(cfg, pc)
		}
		return nil, nil, fmt.Errorf("%w : tableau « %s » introuvable", ErrChart, cfg.Source)
	}
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
	if xc.Formula == "" && (xc.Type == grid.Text || xc.Type == grid.Boolean) {
		sums := make([]string, len(vals))
		for k, v := range vals {
			sums[k] = "sum(" + v + ")"
		}
		key := fmt.Sprintf("coalesce(slt_text(%s, %s), '')", x, sqlLiteral(string(xc.Type)))
		if xc.Type == grid.Text {
			key = "coalesce(" + x + ", '')"
		}
		q = `SELECT ` + key + ` AS k, ` + strings.Join(sums, ", ") + ` FROM ` + tb.sql() + ` GROUP BY k ORDER BY min(_pos) LIMIT ?` + fmt.Sprint(len(args)+1)
	} else {
		q = `SELECT ` + x + `, ` + strings.Join(vals, ", ") + ` FROM ` + tb.sql() + ` ORDER BY _pos LIMIT ?` + fmt.Sprint(len(args)+1)
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
		labels, series, err := t.chartData(out[i].Config)
		if err != nil {
			out[i].Error = strings.TrimPrefix(err.Error(), ErrChart.Error()+" : ")
			continue
		}
		out[i].Labels, out[i].Series = labels, series
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
	if row < 0 || col < 0 || width < 50 || height < 50 {
		return fmt.Errorf("%w : position ou taille invalide", ErrChart)
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
