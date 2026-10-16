package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lalamefine/slitable/internal/formula"
)

// PivotConfig décrit un tableau croisé dynamique : un tableau source, le
// champ des lignes, celui des colonnes (facultatif) et la valeur agrégée.
// Les champs sont désignés par le nom de leur colonne.
type PivotConfig struct {
	Source string `json:"source"`
	Rows   string `json:"rows"`
	Cols   string `json:"cols,omitempty"`
	Value  string `json:"value,omitempty"` // vide : nombre de lignes
	Agg    string `json:"agg"`             // sum, count, avg, min, max
}

// PivotInfo est un tableau croisé posé sur une feuille, avec son résultat.
type PivotInfo struct {
	ID     int64       `json:"id"`
	Row    int         `json:"row"`
	Col    int         `json:"col"`
	Config PivotConfig `json:"config"`
	// Cells : en-tête (1 ligne), groupes, puis la ligne de total.
	Cells [][]string `json:"cells"`
	Error string     `json:"error"`
}

// ErrPivot signale un tableau croisé impossible.
var ErrPivot = errors.New("tableau croisé")

// Bornes d'un tableau croisé : au-delà, le résultat n'est plus lisible.
const (
	MaxPivotRows = 10000
	MaxPivotCols = 50
)

var pivotAggs = map[string]struct{ sql, label string }{
	"sum":   {"sum", "Somme"},
	"count": {"count", "Nombre"},
	"avg":   {"avg", "Moyenne"},
	"min":   {"min", "Min"},
	"max":   {"max", "Max"},
}

// CreatePivot crée un tableau croisé sur une nouvelle feuille, comme Excel,
// et renvoie l'identifiant de la feuille.
func (m *Model) CreatePivot(ctx context.Context, cfg PivotConfig) (int64, error) {
	if _, ok := pivotAggs[cfg.Agg]; !ok {
		return 0, fmt.Errorf("%w : fonction %q inconnue", ErrPivot, cfg.Agg)
	}
	var sheet int64
	err := m.Do(ctx, "Tableau croisé dynamique", func(t *tx) error {
		if _, err := t.pivotPlan(cfg); err != nil {
			return err
		}
		var pos int
		if err := t.row(`SELECT coalesce(max(id), 0) + 1, coalesce(max(pos), -1) + 1 FROM _slitable_sheets`).Scan(&sheet, &pos); err != nil {
			return err
		}
		name := ""
		for n := 1; ; n++ {
			name = fmt.Sprintf("TCD %d", n)
			var k int
			if err := t.row(`SELECT count(*) FROM _slitable_sheets WHERE name = ?`, name).Scan(&k); err != nil {
				return err
			}
			if k == 0 {
				break
			}
		}
		var id int64
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _slitable_pivots`).Scan(&id); err != nil {
			return err
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return t.apply([]stmt{
			st(`INSERT INTO _slitable_sheets (id, name, pos) VALUES (?, ?, ?)`, sheet, name, pos),
			st(`INSERT INTO _slitable_pivots (id, sheet, row, col, config) VALUES (?, ?, 1, 1, ?)`, id, sheet, string(b)),
		}, []stmt{
			st(`DELETE FROM _slitable_pivots WHERE id = ?`, id),
			st(`DELETE FROM _slitable_sheets WHERE id = ?`, sheet),
		})
	})
	return sheet, err
}

// SetPivot change la configuration d'un tableau croisé.
func (m *Model) SetPivot(ctx context.Context, id int64, cfg PivotConfig) error {
	if _, ok := pivotAggs[cfg.Agg]; !ok {
		return fmt.Errorf("%w : fonction %q inconnue", ErrPivot, cfg.Agg)
	}
	return m.Do(ctx, "Modifier le tableau croisé", func(t *tx) error {
		if _, err := t.pivotPlan(cfg); err != nil {
			return err
		}
		var old string
		if err := t.row(`SELECT config FROM _slitable_pivots WHERE id = ?`, id).Scan(&old); err != nil {
			return err
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		q := `UPDATE _slitable_pivots SET config = ? WHERE id = ?`
		return t.apply([]stmt{st(q, string(b), id)}, []stmt{st(q, old, id)})
	})
}

// DeletePivot supprime un tableau croisé (sa feuille reste).
func (m *Model) DeletePivot(ctx context.Context, id int64) error {
	return m.Do(ctx, "Supprimer le tableau croisé", func(t *tx) error {
		var sheet int64
		var row, col int
		var cfg string
		if err := t.row(`SELECT sheet, row, col, config FROM _slitable_pivots WHERE id = ?`, id).Scan(&sheet, &row, &col, &cfg); err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`DELETE FROM _slitable_pivots WHERE id = ?`, id)},
			[]stmt{st(`INSERT INTO _slitable_pivots (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg)},
		)
	})
}

type pivotPlan struct {
	tb             Table
	rows, cols, vc *Column
	agg            string
}

func (t *tx) pivotPlan(cfg PivotConfig) (pivotPlan, error) {
	var p pivotPlan
	tb, err := t.tableByName(cfg.Source)
	if err != nil {
		return p, fmt.Errorf("%w : tableau « %s » introuvable", ErrPivot, cfg.Source)
	}
	p.tb, p.agg = tb, cfg.Agg
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
	if p.rows, err = find(cfg.Rows); err != nil {
		return p, err
	}
	if cfg.Cols != "" {
		if p.cols, err = find(cfg.Cols); err != nil {
			return p, err
		}
	}
	if cfg.Value != "" {
		if p.vc, err = find(cfg.Value); err != nil {
			return p, err
		}
	} else if cfg.Agg != "count" {
		return p, fmt.Errorf("%w : choisissez la colonne des valeurs", ErrPivot)
	}
	return p, nil
}

// pivotCells calcule le tableau croisé : deux passes, les valeurs distinctes
// du champ des colonnes puis un agrégat par colonne (CASE WHEN).
func (t *tx) pivotCells(cfg PivotConfig) ([][]string, error) {
	p, err := t.pivotPlan(cfg)
	if err != nil {
		return nil, err
	}
	agg := pivotAggs[p.agg]
	text := func(c *Column) string {
		return fmt.Sprintf("coalesce(slt_text(%s, %s), '')", colSQL(c.ID), sqlLiteral(string(c.Type)))
	}
	val := "1"
	label := "Nombre"
	if p.vc != nil {
		val = colSQL(p.vc.ID)
		label = agg.label + " de " + p.vc.Name
	}
	measure := func(cond string) string {
		v := val
		if cond != "" {
			v = "CASE WHEN " + cond + " THEN " + val + " END"
		}
		return agg.sql + "(" + v + ")"
	}
	header := []string{p.rows.Name}
	var exprs []string
	var args []any
	if p.cols != nil {
		rs, err := t.query(`SELECT `+text(p.cols)+` AS k FROM `+p.tb.sql()+` GROUP BY k ORDER BY min(`+colSQL(p.cols.ID)+`) IS NULL, min(`+colSQL(p.cols.ID)+`) LIMIT ?`, MaxPivotCols+1)
		if err != nil {
			return nil, err
		}
		var keys []string
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
		for _, k := range keys {
			exprs = append(exprs, measure(text(p.cols)+" = ?"))
			args = append(args, k)
			if k == "" {
				k = "(vide)"
			}
			header = append(header, k)
		}
	}
	exprs = append(exprs, measure(""))
	if p.cols != nil {
		header = append(header, "Total")
	} else {
		header = append(header, label)
	}
	out := [][]string{header}
	read := func(q string, args []any, withKey bool) error {
		rs, err := t.query(q, args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			n := len(exprs)
			if withKey {
				n++
			}
			vals := make([]any, n)
			ptrs := make([]any, n)
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				return err
			}
			row := []string{"Total"}
			start := 0
			if withKey {
				row[0] = formula.FromResult(vals[0]).String()
				if row[0] == "" {
					row[0] = "(vide)"
				}
				start = 1
			}
			for _, v := range vals[start:] {
				row = append(row, formula.FromResult(v).String())
			}
			out = append(out, row)
		}
		return rs.Err()
	}
	rowsArgs := append(append([]any{}, args...), MaxPivotRows+1)
	if err := read(`SELECT `+text(p.rows)+` AS k, `+strings.Join(exprs, ", ")+` FROM `+p.tb.sql()+
		` GROUP BY k ORDER BY min(`+colSQL(p.rows.ID)+`) IS NULL, min(`+colSQL(p.rows.ID)+`) LIMIT ?`, rowsArgs, true); err != nil {
		return nil, err
	}
	if len(out) > MaxPivotRows+1 {
		return nil, fmt.Errorf("%w : plus de %d valeurs dans « %s »", ErrPivot, MaxPivotRows, p.rows.Name)
	}
	if err := read(`SELECT `+strings.Join(exprs, ", ")+` FROM `+p.tb.sql(), args, false); err != nil {
		return nil, err
	}
	return out, nil
}

// pivots donne les tableaux croisés d'une feuille, calculés.
func (t *tx) pivots(sheet int64) ([]PivotInfo, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _slitable_pivots WHERE sheet = ? ORDER BY id`, sheet)
	if err != nil {
		return nil, err
	}
	var out []PivotInfo
	for rs.Next() {
		var p PivotInfo
		var cfg string
		if err := rs.Scan(&p.ID, &p.Row, &p.Col, &cfg); err != nil {
			rs.Close()
			return nil, err
		}
		if err := json.Unmarshal([]byte(cfg), &p.Config); err != nil {
			p.Error = "configuration illisible"
		}
		out = append(out, p)
	}
	rs.Close()
	for i := range out {
		if out[i].Error != "" {
			continue
		}
		cells, err := t.pivotCells(out[i].Config)
		if err != nil {
			out[i].Error = strings.TrimPrefix(err.Error(), ErrPivot.Error()+" : ")
			continue
		}
		out[i].Cells = cells
	}
	return out, nil
}

// pivotIDs liste les tableaux croisés d'une feuille (suppression de feuille).
func (t *tx) pivotRows(sheet int64) ([]stmt, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _slitable_pivots WHERE sheet = ?`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var undo []stmt
	for rs.Next() {
		var id int64
		var row, col int
		var cfg sql.NullString
		if err := rs.Scan(&id, &row, &col, &cfg); err != nil {
			return nil, err
		}
		undo = append(undo, st(`INSERT INTO _slitable_pivots (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg.String))
	}
	return undo, rs.Err()
}
