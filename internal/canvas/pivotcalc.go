package canvas

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lalamefine/slitable/internal/formula"
)

// PivotValue est un champ de la zone Valeurs : la colonne agrégée (vide :
// nombre de lignes), la fonction et l'affichage (valeur ou pourcentage).
type PivotValue struct {
	Field string `json:"field,omitempty"`
	Agg   string `json:"agg"`            // sum, count, countDistinct, avg, min, max
	Show  string `json:"show,omitempty"` // "", pctTotal, pctRow, pctCol
}

// PivotFilter est un champ de la zone Filtres : les valeurs gardées.
type PivotFilter struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

// PivotConfig décrit un tableau croisé dynamique, comme le volet « Champs de
// tableau croisé » d'Excel : zones Lignes, Colonnes, Valeurs et Filtres. Les
// champs sont désignés par le nom de leur colonne dans le tableau source.
type PivotConfig struct {
	// Name nomme le TCD (vue SQL, source de graphique) : « TCD1 »…
	Name      string        `json:"name,omitempty"`
	Source    string        `json:"source"`
	RowFields []string      `json:"rowFields,omitempty"`
	Cols      string        `json:"cols,omitempty"`
	Values    []PivotValue  `json:"values,omitempty"`
	Filters   []PivotFilter `json:"filters,omitempty"`
	// NoGrandTotal masque la ligne et la colonne de total général.
	NoGrandTotal bool `json:"noGrandTotal,omitempty"`

	// Ancien format (un champ de lignes, une valeur) : relu puis converti.
	Rows  string `json:"rows,omitempty"`
	Value string `json:"value,omitempty"`
	Agg   string `json:"agg,omitempty"`
}

// normalized convertit l'ancien format.
func (c PivotConfig) normalized() PivotConfig {
	if len(c.RowFields) == 0 && c.Rows != "" {
		c.RowFields = []string{c.Rows}
	}
	if len(c.Values) == 0 && (c.Value != "" || c.Agg != "") {
		c.Values = []PivotValue{{Field: c.Value, Agg: c.Agg}}
	}
	c.Rows, c.Value, c.Agg = "", "", ""
	return c
}

var pivotAggs = map[string]struct{ label string }{
	"sum":           {"Somme"},
	"count":         {"Nombre"},
	"countDistinct": {"Nombre distinct"},
	"avg":           {"Moyenne"},
	"min":           {"Min"},
	"max":           {"Max"},
}

var pivotShows = map[string]string{"": "", "pctTotal": "% du total", "pctRow": "% de la ligne", "pctCol": "% de la colonne"}

type pivotPlan struct {
	tb      Table
	rows    []*Column
	cols    *Column
	vals    []*Column // nil : nombre de lignes
	cfg     PivotConfig
	filters []string
	args    []any
}

func (t *tx) pivotPlan(raw PivotConfig) (pivotPlan, error) {
	cfg := raw.normalized()
	p := pivotPlan{cfg: cfg}
	tb, err := t.tableByName(cfg.Source)
	if err != nil {
		return p, fmt.Errorf("%w : tableau « %s » introuvable", ErrPivot, cfg.Source)
	}
	p.tb = tb
	find := func(name string) (*Column, error) {
		for i := range tb.Cols {
			if strings.EqualFold(tb.Cols[i].Name, name) {
				if tb.Cols[i].Formula != "" {
					return nil, fmt.Errorf("%w : la colonne calculée « %s » ne sert pas encore de champ", ErrPivot, name)
				}
				return &tb.Cols[i], nil
			}
		}
		return nil, fmt.Errorf("%w : colonne « %s » introuvable dans %s", ErrPivot, name, tb.Name)
	}
	for _, f := range cfg.RowFields {
		c, err := find(f)
		if err != nil {
			return p, err
		}
		p.rows = append(p.rows, c)
	}
	if cfg.Cols != "" {
		if p.cols, err = find(cfg.Cols); err != nil {
			return p, err
		}
	}
	for _, v := range cfg.Values {
		if _, ok := pivotAggs[v.Agg]; !ok {
			return p, fmt.Errorf("%w : fonction %q inconnue", ErrPivot, v.Agg)
		}
		if _, ok := pivotShows[v.Show]; !ok {
			return p, fmt.Errorf("%w : affichage %q inconnu", ErrPivot, v.Show)
		}
		var c *Column
		if v.Field != "" {
			if c, err = find(v.Field); err != nil {
				return p, err
			}
		} else if v.Agg != "count" {
			return p, fmt.Errorf("%w : choisissez la colonne des valeurs", ErrPivot)
		}
		p.vals = append(p.vals, c)
	}
	for _, f := range cfg.Filters {
		c, err := find(f.Field)
		if err != nil {
			return p, err
		}
		marks := make([]string, len(f.Values))
		for i, v := range f.Values {
			marks[i] = "?"
			p.args = append(p.args, v)
		}
		if len(marks) == 0 {
			continue // aucune valeur choisie : toutes les lignes
		}
		p.filters = append(p.filters, pivotText(c)+" IN ("+strings.Join(marks, ", ")+")")
	}
	return p, nil
}

func pivotText(c *Column) string {
	return fmt.Sprintf("coalesce(slt_text(%s, %s), '')", colSQL(c.ID), sqlLiteral(string(c.Type)))
}

func pivotLabel(v PivotValue) string {
	label := pivotAggs[v.Agg].label
	if v.Field != "" {
		label += " de " + v.Field
	}
	if s := pivotShows[v.Show]; s != "" {
		label += " (" + s + ")"
	}
	return label
}

func pivotAgg(agg, v string) string {
	switch agg {
	case "countDistinct":
		return "count(DISTINCT " + v + ")"
	}
	return agg + "(" + v + ")"
}

// pivotCells calcule le tableau croisé en deux passes : les valeurs distinctes
// du champ des colonnes, puis un agrégat par colonne et par valeur
// (CASE WHEN), groupé par les champs des lignes.
func (t *tx) pivotCells(raw PivotConfig) ([][]string, error) {
	p, err := t.pivotPlan(raw)
	if err != nil {
		return nil, err
	}
	cfg := p.cfg
	where := ""
	if len(p.filters) > 0 {
		where = " WHERE " + strings.Join(p.filters, " AND ")
	}
	// Valeurs distinctes du champ des colonnes.
	var keys []string
	if p.cols != nil {
		rs, err := t.query(`SELECT `+pivotText(p.cols)+` AS k FROM `+p.tb.sql()+where+` GROUP BY k ORDER BY min(`+colSQL(p.cols.ID)+`) IS NULL, min(`+colSQL(p.cols.ID)+`) LIMIT ?`, append(append([]any{}, p.args...), MaxPivotCols+1)...)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var k string
			if err := rs.Scan(&k); err != nil {
				rs.Close()
				return nil, err
			}
			keys = append(keys, k)
		}
		rs.Close()
		if len(keys) > MaxPivotCols {
			return nil, fmt.Errorf("%w : plus de %d valeurs dans « %s »", ErrPivot, MaxPivotCols, p.cols.Name)
		}
	}
	// Colonnes de mesure : (clé de colonne, valeur), puis les totaux.
	type measure struct {
		v     int     // rang dans Values
		key   *string // nil : toutes les colonnes (total)
		label string
	}
	var ms []measure
	vals := cfg.Values
	for _, k := range keys {
		for vi := range vals {
			name := k
			if name == "" {
				name = "(vide)"
			}
			if len(vals) > 1 {
				name += " · " + pivotLabel(vals[vi])
			}
			ms = append(ms, measure{vi, &k, name})
		}
	}
	if p.cols == nil || !cfg.NoGrandTotal {
		for vi := range vals {
			name := pivotLabel(vals[vi])
			if p.cols != nil {
				name = "Total"
				if len(vals) > 1 {
					name += " · " + pivotLabel(vals[vi])
				}
			}
			ms = append(ms, measure{vi, nil, name})
		}
	}
	var exprs []string
	var margs []any
	for _, m := range ms {
		v := "1"
		if c := p.vals[m.v]; c != nil {
			v = colSQL(c.ID)
		}
		if m.key != nil {
			exprs = append(exprs, pivotAgg(vals[m.v].Agg, "CASE WHEN "+pivotText(p.cols)+" = ? THEN "+v+" END"))
			margs = append(margs, *m.key)
		} else {
			exprs = append(exprs, pivotAgg(vals[m.v].Agg, v))
		}
	}
	header := make([]string, 0, len(p.rows)+len(ms))
	for _, c := range p.rows {
		header = append(header, c.Name)
	}
	if len(p.rows) == 0 {
		header = append(header, "")
	}
	for _, m := range ms {
		header = append(header, m.label)
	}
	nk := len(p.rows)
	// Lignes : groupes, puis total général.
	type line struct {
		keys []string
		nums []any
	}
	var lines []line
	scan := func(q string, args []any, withKeys bool) error {
		rs, err := t.query(q, args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			n := len(exprs)
			if withKeys {
				n += nk
			}
			row := make([]any, n)
			ptrs := make([]any, n)
			for i := range row {
				ptrs[i] = &row[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				return err
			}
			l := line{}
			if withKeys {
				for _, k := range row[:nk] {
					s := formula.FromResult(k).String()
					if s == "" {
						s = "(vide)"
					}
					l.keys = append(l.keys, s)
				}
				l.nums = row[nk:]
			} else {
				l.keys = []string{"Total"}
				l.nums = row
			}
			lines = append(lines, l)
		}
		return rs.Err()
	}
	if nk > 0 {
		sel := make([]string, nk)
		group := make([]string, nk)
		order := make([]string, 0, 2*nk)
		for i, c := range p.rows {
			sel[i] = pivotText(c) + " AS k" + strconv.Itoa(i)
			group[i] = "k" + strconv.Itoa(i)
			order = append(order, "min("+colSQL(c.ID)+") IS NULL", "min("+colSQL(c.ID)+")")
		}
		cols := append(sel, exprs...)
		args := append(append(append([]any{}, margs...), p.args...), MaxPivotRows+1)
		if err := scan(`SELECT `+strings.Join(cols, ", ")+` FROM `+p.tb.sql()+where+` GROUP BY `+strings.Join(group, ", ")+` ORDER BY `+strings.Join(order, ", ")+` LIMIT ?`, args, true); err != nil {
			return nil, err
		}
		if len(lines) > MaxPivotRows {
			return nil, fmt.Errorf("%w : plus de %d lignes", ErrPivot, MaxPivotRows)
		}
	}
	var total *line
	if len(exprs) > 0 && (nk == 0 || !cfg.NoGrandTotal || hasPct(vals)) {
		if err := scan(`SELECT `+strings.Join(exprs, ", ")+` FROM `+p.tb.sql()+where, append(append([]any{}, margs...), p.args...), false); err != nil {
			return nil, err
		}
		total = &lines[len(lines)-1]
		if nk > 0 && cfg.NoGrandTotal {
			lines = lines[:len(lines)-1]
		}
	}
	// Mise en forme, avec les pourcentages.
	num := func(v any) (float64, bool) {
		f := formula.FromResult(v)
		return f.Num, f.Kind == formula.Number
	}
	// Colonne du total de ligne pour une valeur donnée.
	rowTotal := map[int]int{}
	for i, m := range ms {
		if m.key == nil {
			rowTotal[m.v] = i
		}
	}
	out := [][]string{header}
	for _, l := range lines {
		row := append([]string{}, l.keys...)
		if nk > 1 && len(l.keys) == 1 {
			for len(row) < nk {
				row = append(row, "")
			}
		}
		for i, v := range l.nums {
			m := ms[i]
			show := vals[m.v].Show
			x, ok := num(v)
			if show == "" || !ok {
				row = append(row, formula.FromResult(v).String())
				continue
			}
			var base float64
			switch show {
			case "pctTotal":
				if total != nil {
					if j, ok := rowTotal[m.v]; ok {
						base, _ = num(total.nums[j])
					} else {
						base, _ = num(total.nums[i])
					}
				}
			case "pctCol":
				if total != nil {
					base, _ = num(total.nums[i])
				}
			case "pctRow":
				if j, ok := rowTotal[m.v]; ok {
					base, _ = num(l.nums[j])
				} else {
					base = x
				}
			}
			if base == 0 {
				row = append(row, "")
				continue
			}
			row = append(row, percent(x/base))
		}
		out = append(out, row)
	}
	return out, nil
}

func hasPct(vals []PivotValue) bool {
	for _, v := range vals {
		if v.Show != "" {
			return true
		}
	}
	return false
}

// percent écrit une part à la française : 0.125 → « 12,5 % ».
func percent(f float64) string {
	s := strconv.FormatFloat(f*100, 'f', 1, 64)
	s = strings.TrimSuffix(strings.TrimSuffix(s, "0"), ".")
	return strings.ReplaceAll(s, ".", ",") + " %"
}
