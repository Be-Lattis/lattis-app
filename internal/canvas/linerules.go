package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
		return "_slitable_row_rules"
	}
	return "_slitable_col_rules"
}

// SetLineRules remplace les règles des lignes (ou colonnes) i0 à i1 de la
// feuille ; aucune règle : les retire.
func (m *Model) SetLineRules(ctx context.Context, sheet int64, rows bool, i0, i1 int, rules []Rule) error {
	for _, r := range rules {
		if !ruleOps[r.Op] {
			return fmt.Errorf("règle : condition %q inconnue", r.Op)
		}
	}
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
	var neu any
	if len(rules) > 0 {
		var b strings.Builder
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(rules); err != nil {
			return err
		}
		neu = strings.TrimSpace(b.String())
	}
	return m.Do(ctx, "Mise en forme conditionnelle", func(t *tx) error {
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
	})
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
