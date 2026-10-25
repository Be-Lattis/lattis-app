package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// Graphiques sparkline : un petit graphique dessiné dans une cellule
// (courbe, histogramme ou positif/négatif), par groupe. Deux sources :
//
//   - un tableau : une ligne du tableau, un graphique, tiré de colonnes de
//     la même ligne (Janvier … Décembre) et dessiné dans une colonne du
//     tableau ou juste à sa droite ; il suit les lignes ajoutées, comme une
//     colonne calculée. L'interface lit les valeurs des seules lignes
//     affichées (grand tableau : rien n'est parcouru en entier, sauf pour
//     une échelle commune, LoHi) ;
//   - des cellules libres : une plage source (A1:L4) et un emplacement
//     (M1:M4), un graphique par ligne (ou colonne) de la source ; valeurs
//     lues par le classeur avec la mise en page (Cells).
//
// _lattis_sparklines (id, sheet, config) : configuration JSON, publiée
// avec la mise en page d'un classeur partagé.

// SparklineConfig décrit un groupe de graphiques sparkline.
type SparklineConfig struct {
	// Kind : line (courbe), column (histogramme) ou winloss (positif/négatif).
	Kind string `json:"kind"`
	// Tableau : Table, colonnes sources Cols (dans l'ordre), colonne de
	// dessin At (vide : la colonne juste à droite du tableau).
	Table string   `json:"table,omitempty"`
	Cols  []string `json:"cols,omitempty"`
	At    string   `json:"at,omitempty"`
	// Cellules libres : plage source et emplacement (A1:L4, M1:M4).
	Source   string `json:"source,omitempty"`
	Location string `json:"location,omitempty"`
	// Points mis en valeur : haut, bas, premier, dernier, négatifs ;
	// Markers : tous les points d'une courbe.
	High     bool `json:"high,omitempty"`
	Low      bool `json:"low,omitempty"`
	First    bool `json:"first,omitempty"`
	Last     bool `json:"last,omitempty"`
	Negative bool `json:"negative,omitempty"`
	Markers  bool `json:"markers,omitempty"`
	// Couleurs « #rrggbb » (vides : celles par défaut) : série, points
	// négatifs, marqueurs, points haut, bas, premier et dernier.
	Color       string `json:"color,omitempty"`
	NegColor    string `json:"negColor,omitempty"`
	MarkerColor string `json:"markerColor,omitempty"`
	HighColor   string `json:"highColor,omitempty"`
	LowColor    string `json:"lowColor,omitempty"`
	FirstColor  string `json:"firstColor,omitempty"`
	LastColor   string `json:"lastColor,omitempty"`
	// Axis : axe horizontal à zéro (quand zéro est dans l'échelle) ; Same :
	// même échelle pour tout le groupe ; Min, Max : bornes fixes.
	Axis bool     `json:"axis,omitempty"`
	Same bool     `json:"same,omitempty"`
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
	// Weight : épaisseur de la courbe (px ; 0 : 1,5).
	Weight float64 `json:"weight,omitempty"`
}

// SparkCell : un graphique d'un groupe de cellules libres, sa cellule et
// ses valeurs (nil : vide ou pas un nombre).
type SparkCell struct {
	Row    int        `json:"row"`
	Col    int        `json:"col"`
	Values []*float64 `json:"values"`
}

// SparklineInfo : un groupe posé sur la feuille.
type SparklineInfo struct {
	ID     int64           `json:"id"`
	Config SparklineConfig `json:"config"`
	// Cellules libres : chaque graphique.
	Cells []SparkCell `json:"cells,omitempty"`
	// Tableau : rang des colonnes sources dans le tableau, colonne de la
	// feuille où se dessinent les graphiques.
	Src   []int `json:"src,omitempty"`
	AtCol int   `json:"atCol"`
	// Lo, Hi : échelle commune (Same) sur tout le groupe.
	Lo    *float64 `json:"lo,omitempty"`
	Hi    *float64 `json:"hi,omitempty"`
	Error string   `json:"error,omitempty"`
}

// ErrSparkline signale un graphique sparkline impossible.
var ErrSparkline = errors.New("sparkline")

// Bornes : graphiques d'un groupe de cellules libres, points par graphique.
const (
	MaxSparkCells  = 10000
	MaxSparkPoints = 1000
)

var sparkKinds = map[string]bool{"line": true, "column": true, "winloss": true}

var a1Range = regexp.MustCompile(`^\$?([A-Za-z]{1,3})\$?(\d{1,7})(?::\$?([A-Za-z]{1,3})\$?(\d{1,7}))?$`)

// parseA1 lit une plage « B2:M5 » (ou une cellule) : lignes et colonnes de
// 0, coins rangés.
func parseA1(s string) (formula.CellRange, bool) {
	m := a1Range.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return formula.CellRange{}, false
	}
	r1, c1, ok := formula.CellRef("cell_" + m[1] + m[2])
	if !ok {
		return formula.CellRange{}, false
	}
	r2, c2 := r1, c1
	if m[3] != "" {
		if r2, c2, ok = formula.CellRef("cell_" + m[3] + m[4]); !ok {
			return formula.CellRange{}, false
		}
	}
	return formula.CellRange{R1: min(r1, r2), C1: min(c1, c2), R2: max(r1, r2), C2: max(c1, c2)}, true
}

// a1Text écrit une plage en A1 (une cellule seule sans « : »).
func a1Text(r formula.CellRange) string {
	s := formula.ColumnLetters(r.C1) + strconv.Itoa(r.R1+1)
	if r.R2 != r.R1 || r.C2 != r.C1 {
		s += ":" + formula.ColumnLetters(r.C2) + strconv.Itoa(r.R2+1)
	}
	return s
}

// sparkPlan : graphiques d'un groupe de cellules libres, cellule de dessin
// et cellules sources de chacun. Un graphique par ligne de la source si
// l'emplacement compte autant de cellules que la source de lignes, sinon
// par colonne.
func sparkPlan(cfg SparklineConfig) ([][2]int, [][][2]int, error) {
	src, ok := parseA1(cfg.Source)
	if !ok {
		return nil, nil, msg.Wrap(ErrSparkline, "sparkline.badSource", msg.Args{"range": cfg.Source})
	}
	loc, ok := parseA1(cfg.Location)
	if !ok {
		return nil, nil, msg.Wrap(ErrSparkline, "sparkline.badLocation", msg.Args{"range": cfg.Location})
	}
	lr, lc := loc.Size()
	if lr > 1 && lc > 1 {
		return nil, nil, msg.Wrap(ErrSparkline, "sparkline.locationLine", nil)
	}
	n := lr * lc
	sr, sc := src.Size()
	byRow := sr == n
	if !byRow && sc != n {
		return nil, nil, msg.Wrap(ErrSparkline, "sparkline.sizeMismatch", nil)
	}
	if n == 1 && sr > 1 && sc > 1 {
		return nil, nil, msg.Wrap(ErrSparkline, "sparkline.sizeMismatch", nil)
	}
	if n == 1 {
		byRow = sr == 1
	}
	points := sc
	if !byRow {
		points = sr
	}
	if n > MaxSparkCells || points > MaxSparkPoints {
		return nil, nil, msg.Wrap(ErrSparkline, "sparkline.tooBig", msg.Args{"cells": MaxSparkCells, "points": MaxSparkPoints})
	}
	at := make([][2]int, n)
	from := make([][][2]int, n)
	for k := 0; k < n; k++ {
		if lr > 1 {
			at[k] = [2]int{loc.R1 + k, loc.C1}
		} else {
			at[k] = [2]int{loc.R1, loc.C1 + k}
		}
		cells := make([][2]int, points)
		for i := range cells {
			if byRow {
				cells[i] = [2]int{src.R1 + k, src.C1 + i}
			} else {
				cells[i] = [2]int{src.R1 + i, src.C1 + k}
			}
		}
		from[k] = cells
	}
	return at, from, nil
}

// checkSparkline vérifie et range une configuration (bornes, couleurs).
func (t *tx) checkSparkline(cfg *SparklineConfig) error {
	if !sparkKinds[cfg.Kind] {
		return msg.Wrap(ErrSparkline, "sparkline.unknownKind", msg.Args{"kind": cfg.Kind})
	}
	for _, c := range []*string{&cfg.Color, &cfg.NegColor, &cfg.MarkerColor, &cfg.HighColor, &cfg.LowColor, &cfg.FirstColor, &cfg.LastColor} {
		if *c != "" && !hexColor.MatchString(*c) {
			return msg.Wrap(ErrSparkline, "sparkline.badColor", msg.Args{"color": *c})
		}
	}
	if cfg.Min != nil && cfg.Max != nil && *cfg.Min >= *cfg.Max {
		return msg.Wrap(ErrSparkline, "sparkline.badBounds", nil)
	}
	if cfg.Weight < 0 || cfg.Weight > 6 {
		return msg.Wrap(ErrSparkline, "sparkline.badWeight", nil)
	}
	if cfg.Table == "" {
		cfg.Cols, cfg.At = nil, ""
		_, _, err := sparkPlan(*cfg)
		if err == nil {
			src, _ := parseA1(cfg.Source)
			loc, _ := parseA1(cfg.Location)
			cfg.Source, cfg.Location = a1Text(src), a1Text(loc)
		}
		return err
	}
	cfg.Source, cfg.Location = "", ""
	tb, err := t.tableByName(cfg.Table)
	if err != nil {
		return msg.Wrap(ErrSparkline, "sparkline.tableNotFound", msg.Args{"table": cfg.Table})
	}
	cfg.Table = tb.Name
	if len(cfg.Cols) == 0 || len(cfg.Cols) > MaxSparkPoints {
		return msg.Wrap(ErrSparkline, "sparkline.chooseColumns", nil)
	}
	for i, c := range cfg.Cols {
		k := colIndex(tb, c)
		if k < 0 {
			return msg.Wrap(ErrSparkline, "sparkline.columnNotFound", msg.Args{"column": c, "table": tb.Name})
		}
		cfg.Cols[i] = tb.Cols[k].Name
	}
	if cfg.At != "" {
		k := colIndex(tb, cfg.At)
		if k < 0 {
			return msg.Wrap(ErrSparkline, "sparkline.columnNotFound", msg.Args{"column": cfg.At, "table": tb.Name})
		}
		cfg.At = tb.Cols[k].Name
	}
	return nil
}

// sparkSheet : feuille d'un groupe neuf (celle de son tableau).
func (t *tx) sparkSheet(sheet int64, cfg SparklineConfig) int64 {
	if cfg.Table != "" {
		if tb, err := t.tableByName(cfg.Table); err == nil {
			return tb.Sheet
		}
	}
	return sheet
}

// CreateSparkline pose un groupe de graphiques sparkline et rend son
// identifiant (une étape d'annulation).
func (m *Model) CreateSparkline(ctx context.Context, sheet int64, cfg SparklineConfig) (int64, error) {
	var id int64
	err := m.Do(ctx, "Insérer des graphiques sparkline", func(t *tx) error {
		if err := t.checkSparkline(&cfg); err != nil {
			return err
		}
		sheet = t.sparkSheet(sheet, cfg)
		if err := t.guard(sheet); err != nil {
			return err
		}
		// Un emplacement déjà pris par un autre groupe : celui-ci le remplace.
		do, undo, err := t.sparkOverlaps(sheet, cfg, 0)
		if err != nil {
			return err
		}
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_sparklines`).Scan(&id); err != nil {
			return err
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		do = append(do, st(`INSERT INTO _lattis_sparklines (id, sheet, config) VALUES (?, ?, ?)`, id, sheet, string(b)))
		undo = append([]stmt{st(`DELETE FROM _lattis_sparklines WHERE id = ?`, id)}, undo...)
		return t.apply(do, undo)
	})
	return id, err
}

// SetSparkline change un groupe (type, source, emplacement, options).
func (m *Model) SetSparkline(ctx context.Context, id int64, cfg SparklineConfig) error {
	return m.Do(ctx, "Modifier les graphiques sparkline", func(t *tx) error {
		if err := t.guardObject(`_lattis_sparklines`, id); err != nil {
			return err
		}
		var sheet int64
		var old string
		if err := t.row(`SELECT sheet, config FROM _lattis_sparklines WHERE id = ?`, id).Scan(&sheet, &old); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return msg.Wrap(ErrSparkline, "sparkline.notFound", msg.Args{"id": id})
			}
			return err
		}
		if err := t.checkSparkline(&cfg); err != nil {
			return err
		}
		do, undo, err := t.sparkOverlaps(sheet, cfg, id)
		if err != nil {
			return err
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		if string(b) == old && len(do) == 0 {
			return nil
		}
		q := `UPDATE _lattis_sparklines SET config = ? WHERE id = ?`
		return t.apply(append(do, st(q, string(b), id)), append([]stmt{st(q, old, id)}, undo...))
	})
}

// DeleteSparkline supprime un groupe.
func (m *Model) DeleteSparkline(ctx context.Context, id int64) error {
	return m.Do(ctx, "Supprimer les graphiques sparkline", func(t *tx) error {
		if err := t.guardObject(`_lattis_sparklines`, id); err != nil {
			return err
		}
		var sheet int64
		var cfg string
		if err := t.row(`SELECT sheet, config FROM _lattis_sparklines WHERE id = ?`, id).Scan(&sheet, &cfg); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return msg.Wrap(ErrSparkline, "sparkline.notFound", msg.Args{"id": id})
			}
			return err
		}
		return t.apply(
			[]stmt{st(`DELETE FROM _lattis_sparklines WHERE id = ?`, id)},
			[]stmt{st(`INSERT INTO _lattis_sparklines (id, sheet, config) VALUES (?, ?, ?)`, id, sheet, cfg)},
		)
	})
}

// Sparklines donne les groupes d'une feuille (sans valeurs : celles des
// cellules libres viennent avec la mise en page).
func (m *Model) Sparklines(ctx context.Context, sheet int64) ([]SparklineInfo, error) {
	var out []SparklineInfo
	err := m.read(ctx, func(t *tx) error {
		var err error
		out, err = t.sparklines(sheet, nil)
		return err
	})
	return out, err
}

type sparkRow struct {
	id  int64
	cfg SparklineConfig
	raw string
}

func (t *tx) sparkRows(sheet int64) ([]sparkRow, error) {
	rs, err := t.query(`SELECT id, config FROM _lattis_sparklines WHERE sheet = ? ORDER BY id`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []sparkRow
	for rs.Next() {
		var r sparkRow
		if err := rs.Scan(&r.id, &r.raw); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(r.raw), &r.cfg) != nil {
			continue
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// sparkOverlaps : un groupe de cellules libres posé là où un autre se
// dessine déjà (cellules communes) le remplace, comme un tableur ; rend la
// suppression des anciens et son inverse. self : groupe modifié (ignoré).
func (t *tx) sparkOverlaps(sheet int64, cfg SparklineConfig, self int64) (do, undo []stmt, err error) {
	if cfg.Table != "" {
		return nil, nil, nil
	}
	at, _, err := sparkPlan(cfg)
	if err != nil {
		return nil, nil, err
	}
	mine := map[[2]int]bool{}
	for _, c := range at {
		mine[c] = true
	}
	rows, err := t.sparkRows(sheet)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range rows {
		if r.id == self || r.cfg.Table != "" {
			continue
		}
		theirs, _, err := sparkPlan(r.cfg)
		if err != nil {
			continue
		}
		for _, c := range theirs {
			if mine[c] {
				do = append(do, st(`DELETE FROM _lattis_sparklines WHERE id = ?`, r.id))
				undo = append(undo, st(`INSERT INTO _lattis_sparklines (id, sheet, config) VALUES (?, ?, ?)`, r.id, sheet, r.raw))
				break
			}
		}
	}
	return do, undo, nil
}

// sparklines : groupes de la feuille ; cells (mise en page) donne les
// valeurs des cellules libres (nil : sans valeurs).
func (t *tx) sparklines(sheet int64, cells []CellInfo) ([]SparklineInfo, error) {
	rows, err := t.sparkRows(sheet)
	if err != nil {
		return nil, err
	}
	var values map[[2]int]*float64
	if cells != nil {
		values = make(map[[2]int]*float64, len(cells))
		for _, c := range cells {
			values[[2]int{c.Row, c.Col}] = freeNumber(c)
		}
		// Source posée sur un tableau (plage choisie à la main) : ses cellules.
		var tables []Table
		for _, r := range rows {
			if r.cfg.Table != "" {
				continue
			}
			src, ok := parseA1(r.cfg.Source)
			if !ok {
				continue
			}
			if tables == nil {
				if tables, err = t.tables(sheet); err != nil {
					return nil, err
				}
			}
			for _, tb := range tables {
				t.tableValues(tb, src, values)
			}
		}
	}
	out := []SparklineInfo{}
	for _, r := range rows {
		s := SparklineInfo{ID: r.id, Config: r.cfg, AtCol: -1}
		if r.cfg.Table != "" {
			t.sparkTable(&s)
		} else {
			at, from, err := sparkPlan(r.cfg)
			if err != nil {
				s.Error = strings.TrimPrefix(err.Error(), ErrSparkline.Error()+" : ")
			} else {
				s.Cells = make([]SparkCell, len(at))
				lo, hi := math.Inf(1), math.Inf(-1)
				for k, c := range at {
					sc := SparkCell{Row: c[0], Col: c[1], Values: make([]*float64, len(from[k]))}
					if values != nil {
						for i, f := range from[k] {
							if v := values[f]; v != nil {
								sc.Values[i] = v
								lo, hi = math.Min(lo, *v), math.Max(hi, *v)
							}
						}
					}
					s.Cells[k] = sc
				}
				if r.cfg.Same && lo <= hi {
					s.Lo, s.Hi = &lo, &hi
				}
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// sparkTable complète un groupe tiré d'un tableau : rang des colonnes,
// colonne de dessin, échelle commune.
func (t *tx) sparkTable(s *SparklineInfo) {
	tb, err := t.tableByName(s.Config.Table)
	if err != nil {
		s.Error = msg.New("sparkline.tableNotFound", msg.Args{"table": s.Config.Table}).Error()
		return
	}
	s.AtCol = tb.Col + len(tb.Cols)
	if s.Config.At != "" {
		k := colIndex(tb, s.Config.At)
		if k < 0 {
			s.Error = msg.New("sparkline.columnNotFound", msg.Args{"column": s.Config.At, "table": tb.Name}).Error()
			return
		}
		s.AtCol = tb.Col + k
	}
	for _, c := range s.Config.Cols {
		k := colIndex(tb, c)
		if k < 0 {
			s.Error = msg.New("sparkline.columnNotFound", msg.Args{"column": c, "table": tb.Name}).Error()
			return
		}
		s.Src = append(s.Src, k)
	}
	if !s.Config.Same {
		return
	}
	exprs, args, _, err := t.columnSQL(tb)
	if err != nil {
		return
	}
	var parts []string
	for _, k := range s.Src {
		v := numericSQL(exprs[k])
		parts = append(parts, "min("+v+")", "max("+v+")")
	}
	row := make([]sql.NullFloat64, len(parts))
	ptrs := make([]any, len(row))
	for i := range row {
		ptrs[i] = &row[i]
	}
	if err := t.rowAll(`SELECT `+strings.Join(parts, ", ")+` FROM `+tb.sql(), args).Scan(ptrs...); err != nil {
		return
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for i, v := range row {
		if !v.Valid {
			continue
		}
		if i%2 == 0 {
			lo = math.Min(lo, v.Float64)
		} else {
			hi = math.Max(hi, v.Float64)
		}
	}
	if lo <= hi {
		s.Lo, s.Hi = &lo, &hi
	}
}

// tableValues ajoute à values les nombres des cellules du tableau tb qui
// tombent dans la plage src (lignes affichées, dans leur ordre).
func (t *tx) tableValues(tb Table, src formula.CellRange, values map[[2]int]*float64) {
	r0, r1 := max(src.R1, tb.Row+1), min(src.R2, tb.Bottom())
	c0, c1 := max(src.C1, tb.Col), min(src.C2, tb.Right())
	if r0 > r1 || c0 > c1 {
		return
	}
	exprs, args, _, err := t.columnSQL(tb)
	if err != nil {
		return
	}
	var sel []string
	for c := c0; c <= c1; c++ {
		sel = append(sel, numericSQL(exprs[c-tb.Col]))
	}
	from := tb.sql()
	if tb.where != "" {
		from = "(SELECT * FROM " + tb.sql() + " WHERE " + tb.where + ") AS " + tb.sql()
	}
	q := fmt.Sprintf(`SELECT %s FROM %s ORDER BY _pos LIMIT %d OFFSET %d`, strings.Join(sel, ", "), from, r1-r0+1, r0-tb.Row-1)
	rs, err := t.query(q, args...)
	if err != nil {
		return
	}
	defer rs.Close()
	for r := r0; rs.Next(); r++ {
		row := make([]sql.NullFloat64, len(sel))
		ptrs := make([]any, len(row))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if rs.Scan(ptrs...) != nil {
			return
		}
		for i, v := range row {
			if v.Valid {
				f := v.Float64
				values[[2]int{r, c0 + i}] = &f
			}
		}
	}
}

// freeNumber : valeur numérique d'une cellule libre (résultat d'une
// formule, ou saisie), nil pour un texte, une erreur ou une cellule vide.
func freeNumber(c CellInfo) *float64 {
	if isFormula(c.Raw) {
		if c.Error {
			return nil
		}
		if f, ok := parseShown(c.Value); ok {
			return &f
		}
		return nil
	}
	if n, ok := valueLiteral(c.Raw).(formula.NumLit); ok {
		f := n.V
		return &f
	}
	return nil
}

// rewriteSparklines suit les lignes et colonnes insérées ou supprimées
// (sheetrows.go) : sources et emplacements des groupes de cellules libres ;
// un groupe dont l'emplacement disparaît part avec lui.
func (t *tx) rewriteSparklines(sheet int64, move func(r, c int) (int, int, bool)) error {
	rows, err := t.sparkRows(sheet)
	if err != nil {
		return err
	}
	var do, undo []stmt
	q := `UPDATE _lattis_sparklines SET config = ? WHERE id = ?`
	for _, r := range rows {
		if r.cfg.Table != "" {
			continue
		}
		cfg := r.cfg
		moved := func(s string) (string, bool) {
			rg, ok := parseA1(s)
			if !ok {
				return s, true
			}
			a := formula.ColumnLetters(rg.C1) + strconv.Itoa(rg.R1+1)
			b := formula.ColumnLetters(rg.C2) + strconv.Itoa(rg.R2+1)
			n, ok := moveRange(a, b, move)
			return a1Text(n), ok
		}
		loc, okL := moved(cfg.Location)
		src, okS := moved(cfg.Source)
		if !okL || !okS {
			do = append(do, st(`DELETE FROM _lattis_sparklines WHERE id = ?`, r.id))
			undo = append(undo, st(`INSERT INTO _lattis_sparklines (id, sheet, config) VALUES (?, ?, ?)`, r.id, sheet, r.raw))
			continue
		}
		if loc == cfg.Location && src == cfg.Source {
			continue
		}
		cfg.Location, cfg.Source = loc, src
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		do = append(do, st(q, string(b), r.id))
		undo = append(undo, st(q, r.raw, r.id))
	}
	if len(do) == 0 {
		return nil
	}
	return t.apply(do, undo)
}

// rewriteSparkRefs : tableau ou colonnes renommés (refs.go).
func (t *tx) rewriteSparkRefs(rc refChange) error {
	cfgs, ids, err := t.configs(`_lattis_sparklines`)
	if err != nil {
		return err
	}
	var ups []textUpdate
	for _, id := range ids {
		var m map[string]any
		if json.Unmarshal([]byte(cfgs[id]), &m) != nil {
			continue
		}
		src, _ := m["table"].(string)
		if src == "" {
			continue
		}
		changed := renameFieldList(m, "cols", rc.Renames, src)
		changed = renameField(m, "at", rc.Renames, src) || changed
		if n := rc.Table(src); n != src {
			m["table"], changed = n, true
		}
		if !changed {
			continue
		}
		if neu, err := encodeJSON(m); err == nil {
			ups = append(ups, textUpdate{[]any{id}, cfgs[id], neu})
		}
	}
	return t.applyUpdates(`UPDATE _lattis_sparklines SET config = ? WHERE id = ?`, ups)
}

// sparkRowsOf donne l'inverse de la suppression des groupes d'une feuille.
func (t *tx) sparkRowsOf(sheet int64) ([]stmt, error) {
	rows, err := t.sparkRows(sheet)
	if err != nil {
		return nil, err
	}
	var undo []stmt
	for _, r := range rows {
		undo = append(undo, st(`INSERT INTO _lattis_sparklines (id, sheet, config) VALUES (?, ?, ?)`, r.id, sheet, r.raw))
	}
	return undo, nil
}
