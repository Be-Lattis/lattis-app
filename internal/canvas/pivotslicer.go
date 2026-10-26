package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/work"
)

// Segments et chronologies d'un tableau croisé (Insérer un segment, comme un
// tableur) : un panneau flottant sur la feuille du TCD, posé comme un
// graphique, qui liste les valeurs d'un champ en boutons (ou, pour une
// chronologie, les périodes d'une date). Sa sélection filtre le TCD comme un
// filtre de rapport, et aussi les autres TCD de même source qu'il désigne
// (Pivots : connexions du segment). Il est rangé dans la configuration de
// son TCD (une mise en page, publiée avec elle) : le déplacer, le
// redimensionner ou y choisir des valeurs est une modification du TCD (une
// étape d'annulation), sans recalcul quand seule sa place change (la clé du
// cache ne garde que sa sélection, voir flat).

// PivotSlicer est un segment (Kind vide) ou une chronologie ("timeline").
type PivotSlicer struct {
	Field string `json:"field"`
	// Values : valeurs gardées (vide : toutes) ; pour une chronologie, des
	// périodes de son niveau (« 2024-03 »).
	Values []string `json:"values,omitempty"`
	Kind   string   `json:"kind,omitempty"`
	// Level : niveau d'une chronologie : years, quarters, months (défaut), days.
	Level string `json:"level,omitempty"`
	// Place sur la feuille, comme un graphique : cellule d'ancrage, décalage
	// et taille en pixels (0 : taille par défaut).
	Row    int `json:"row"`
	Col    int `json:"col"`
	OffX   int `json:"offX,omitempty"`
	OffY   int `json:"offY,omitempty"`
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	// Columns : boutons par ligne (1 par défaut).
	Columns int `json:"columns,omitempty"`
	// Pivots : autres TCD (par leur nom) que le segment filtre aussi.
	Pivots []string `json:"pivots,omitempty"`
}

// PivotSlicerInfo : les éléments d'un segment (valeurs du champ dans toute
// la source, avec leur nombre de lignes), dans l'ordre de PivotConfig.Slicers.
type PivotSlicerInfo struct {
	Items []FilterValue `json:"items"`
	Error string        `json:"error,omitempty"`
}

// timelineLevel : le niveau d'une chronologie.
func (s PivotSlicer) timelineLevel() string {
	if s.Kind != "timeline" {
		return ""
	}
	if indexOf(dateLevels, s.Level) >= 0 {
		return s.Level
	}
	return "months"
}

// slicerFilters : les filtres que posent des segments.
func slicerFilters(list []PivotSlicer) []PivotFilter {
	var out []PivotFilter
	for _, s := range list {
		if len(s.Values) > 0 {
			out = append(out, PivotFilter{Field: s.Field, Values: s.Values, Period: s.timelineLevel()})
		}
	}
	return out
}

// calcSlicers : les segments réduits à ce qui change le calcul (leur
// sélection), pour la clé du cache.
func calcSlicers(list []PivotSlicer) []PivotSlicer {
	var out []PivotSlicer
	for _, s := range list {
		if len(s.Values) > 0 {
			out = append(out, PivotSlicer{Field: s.Field, Values: s.Values, Kind: s.Kind, Level: s.timelineLevel()})
		}
	}
	return out
}

// pivotLinks pose dans cfg.Linked les filtres des segments d'autres TCD
// reliés à celui-ci (même source, son nom dans leurs connexions).
func (t *tx) pivotLinks(cfg *PivotConfig) error {
	cfg.Linked = nil
	if cfg.Name == "" {
		return nil
	}
	all, err := cached(t, "pivotslicers", t.pivotSlicerRows)
	if err != nil {
		return err
	}
	for _, o := range all {
		if strings.EqualFold(o.name, cfg.Name) || !strings.EqualFold(o.source, cfg.Source) {
			continue
		}
		for _, s := range o.slicers {
			if len(s.Values) > 0 && slices.ContainsFunc(s.Pivots, func(n string) bool { return strings.EqualFold(n, cfg.Name) }) {
				cfg.Linked = append(cfg.Linked, PivotFilter{Field: s.Field, Values: s.Values, Period: s.timelineLevel()})
			}
		}
	}
	return nil
}

type slicerRow struct {
	name, source string
	slicers      []PivotSlicer
}

// pivotSlicerRows : les TCD du classeur qui ont des segments reliés à
// d'autres TCD.
func (t *tx) pivotSlicerRows() ([]slicerRow, error) {
	rs, err := t.query(`SELECT id, config FROM _lattis_pivots WHERE config LIKE '%"pivots"%'`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []slicerRow
	for rs.Next() {
		var id int64
		var raw string
		if err := rs.Scan(&id, &raw); err != nil {
			return nil, err
		}
		var c PivotConfig
		if json.Unmarshal([]byte(raw), &c) != nil || len(c.Slicers) == 0 {
			continue
		}
		if c.Name == "" {
			c.Name = fmt.Sprintf("TCD%d", id)
		}
		out = append(out, slicerRow{c.Name, c.Source, c.Slicers})
	}
	return out, rs.Err()
}

// pivotFieldValues : les valeurs distinctes d'un champ du TCD (colonne,
// champ calculé, groupe, ou période d'une date au niveau period), dans
// toute la source (sans filtre), avec leur nombre de lignes : la liste des
// filtres du volet, les éléments d'un segment.
func (t *tx) pivotFieldValues(cfg PivotConfig, field, period string) ([]FilterValue, error) {
	cfg = cfg.normalized()
	cfg.Filters, cfg.Items, cfg.Slicers, cfg.Linked, cfg.Choice = nil, nil, nil, nil, nil
	q := PivotConfig{Source: cfg.Source, Lang: cfg.Lang, Calc: cfg.Calc, Groups: cfg.Groups, RowFields: []string{field}}
	key := keyOf("pivotvalues:"+period+":", q)
	run := func() ([]FilterValue, error) {
		work.Phase(t.Ctx, "pivot")
		return t.pivotFieldValuesRaw(q, field, period)
	}
	if deps, ok := t.pivotDeps(q); ok {
		return cachedOn(t, key, deps, run)
	}
	return cached(t, key, run)
}

func (t *tx) pivotFieldValuesRaw(cfg PivotConfig, field, period string) ([]FilterValue, error) {
	p, err := t.pivotPlan(cfg)
	if err != nil {
		return nil, err
	}
	c := p.rows[0]
	v, order := p.text(c), "min("+colSQL(c.ID)+")"
	if period != "" {
		// Une chronologie lit la date d'origine, pas un niveau groupé.
		if len(p.cfg.Groups) > 0 {
			q := cfg
			q.Groups = nil
			if p, err = t.pivotPlan(q); err != nil {
				return nil, err
			}
			c = p.rows[0]
		}
		v = "coalesce(" + periodSQL(period, pivotDateSQL(p.exprOf(c))) + ", '')"
		order = "v"
	}
	out := []FilterValue{}
	rs, err := t.query(`SELECT `+v+` AS v, count(*) FROM `+p.from()+` GROUP BY v ORDER BY v = '', `+order+` LIMIT ?`, MaxFilterValues)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var x FilterValue
		var raw any
		if err := rs.Scan(&raw, &x.Count); err != nil {
			return nil, err
		}
		x.Text = pivotKeyText(raw)
		if period != "" && x.Text == "" {
			continue // pas une date : hors de la chronologie
		}
		out = append(out, x)
	}
	return out, rs.Err()
}

// pivotSlicersAttach donne à un TCD calculé les éléments de ses segments.
func (t *tx) pivotSlicersAttach(p *PivotInfo) {
	if len(p.Config.Slicers) == 0 {
		return
	}
	p.Slicers = make([]PivotSlicerInfo, len(p.Config.Slicers))
	for i, s := range p.Config.Slicers {
		items, err := t.pivotFieldValues(p.Config, s.Field, s.timelineLevel())
		if err != nil {
			p.Slicers[i] = PivotSlicerInfo{Items: []FilterValue{}, Error: strings.TrimPrefix(err.Error(), ErrPivot.Error()+" : ")}
			continue
		}
		p.Slicers[i] = PivotSlicerInfo{Items: items}
	}
}

// PivotFieldValues donne les valeurs distinctes d'un champ d'un tableau
// croisé de configuration cfg (colonne, colonne calculée, champ calculé,
// champ groupé), avec leur nombre de lignes : liste du filtre d'un champ
// dans le volet et dans le ▾ de son étiquette.
func (m *Model) PivotFieldValues(ctx context.Context, cfg PivotConfig, field string) ([]FilterValue, error) {
	var out []FilterValue
	err := m.read(ctx, func(t *tx) error {
		if err := t.prepareCalc(&cfg); err != nil {
			return err
		}
		var err error
		out, err = t.pivotFieldValues(cfg, field, "")
		return err
	})
	if out == nil {
		out = []FilterValue{}
	}
	return out, err
}
