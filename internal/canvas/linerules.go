package canvas

import (
	"context"
	"fmt"
)

// Mise en forme conditionnelle de lignes ou de colonnes entières de la
// feuille, comme dans Excel : les règles s'appliquent à toutes les cellules
// de la ligne ou de la colonne (cellules libres, tableaux, TCD), sous les
// règles propres d'une colonne de tableau.

// LineRules sont les règles d'une ligne ou d'une colonne entière (JSON).
type LineRules struct {
	Index int    `json:"index"`
	Rules string `json:"rules"`
}

func (a axis) lineRules() string {
	if a == rowsAxis {
		return "_lattis_row_rules"
	}
	return "_lattis_col_rules"
}

// SetLineRules remplace les règles des lignes (ou colonnes) i0 à i1 de la
// feuille ; aucune règle : les retire.
func (m *Model) SetLineRules(ctx context.Context, sheet int64, rows bool, i0, i1 int, rules []Rule) error {
	if err := checkRules(rules); err != nil {
		return err
	}
	return m.Do(ctx, "Mise en forme conditionnelle", func(t *tx) error {
		if err := t.guardFormat(sheet); err != nil {
			return err
		}
		return t.setLineRules(sheet, rows, i0, i1, rules)
	})
}

func (t *tx) setLineRules(sheet int64, rows bool, i0, i1 int, rules []Rule) error {
	if i0 > i1 {
		i0, i1 = i1, i0
	}
	if i0 < 0 || i1-i0+1 > MaxStyleLines {
		return fmt.Errorf("mise en forme limitée à %d lignes ou colonnes", MaxStyleLines)
	}
	a := colsAxis
	if rows {
		a = rowsAxis
	}
	rules = append([]Rule{}, rules...)
	for k := range rules {
		rules[k].F = ""
	}
	neu, err := encodeRules(rules)
	if err != nil {
		return err
	}
	if err := t.deleteWhere(a.lineRules(), fmt.Sprintf(`sheet = ? AND %s BETWEEN ? AND ?`, a), sheet, i0, i1); err != nil {
		return err
	}
	if neu == nil {
		return nil
	}
	var do, undo []stmt
	for i := i0; i <= i1; i++ {
		do = append(do, st(fmt.Sprintf(`INSERT INTO %s (sheet, %s, rules) VALUES (?, ?, ?)`, a.lineRules(), a), sheet, i, neu))
		undo = append(undo, st(fmt.Sprintf(`DELETE FROM %s WHERE sheet = ? AND %s = ?`, a.lineRules(), a), sheet, i))
	}
	return t.apply(do, undo)
}

// lineRulesOf lit les règles des lignes et des colonnes d'une feuille.
func (t *tx) lineRulesOf(sheet int64) (rows, cols []LineRules, err error) {
	rows, cols = []LineRules{}, []LineRules{}
	for _, q := range []struct {
		a  axis
		to *[]LineRules
	}{{rowsAxis, &rows}, {colsAxis, &cols}} {
		rs, err := t.query(fmt.Sprintf(`SELECT %s, rules FROM %s WHERE sheet = ? ORDER BY %s`, q.a, q.a.lineRules(), q.a), sheet)
		if err != nil {
			return nil, nil, err
		}
		for rs.Next() {
			var l LineRules
			if err := rs.Scan(&l.Index, &l.Rules); err != nil {
				rs.Close()
				return nil, nil, err
			}
			*q.to = append(*q.to, l)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return nil, nil, err
		}
	}
	return rows, cols, nil
}
