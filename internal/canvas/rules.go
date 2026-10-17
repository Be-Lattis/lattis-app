package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Rule est une règle de mise en forme conditionnelle d'une colonne :
// quand la valeur vérifie la condition, la cellule prend le style.
//
//	op : ">", ">=", "<", "<=", "=", "<>", "between", "contains", "empty",
//	     "notempty", "error" (saisie hors type ou erreur de formule)
type Rule struct {
	Op    string `json:"op"`
	Value string `json:"value,omitempty"`
	To    string `json:"to,omitempty"` // borne haute de "between"
	Style Style  `json:"style"`
}

var ruleOps = map[string]bool{">": true, ">=": true, "<": true, "<=": true, "=": true, "<>": true,
	"between": true, "contains": true, "empty": true, "notempty": true, "error": true}

// SetRules remplace les règles de mise en forme conditionnelle de la colonne
// i (liste vide : aucune). La première règle vérifiée l'emporte.
func (m *Model) SetRules(ctx context.Context, table string, i int, rules []Rule) error {
	for _, r := range rules {
		if !ruleOps[r.Op] {
			return fmt.Errorf("règle : condition %q inconnue", r.Op)
		}
	}
	return m.Do(ctx, "Mise en forme conditionnelle", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(tb.Cols) {
			return fmt.Errorf("colonne %d hors du tableau", i)
		}
		c := tb.Cols[i]
		var old sql.NullString
		if err := t.row(`SELECT rules FROM _lattis_columns WHERE tbl = ? AND id = ?`, tb.ID, c.ID).Scan(&old); err != nil {
			return err
		}
		var neu any
		if len(rules) > 0 {
			var b strings.Builder
			enc := json.NewEncoder(&b)
			enc.SetEscapeHTML(false) // garder « > » lisible
			if err := enc.Encode(rules); err != nil {
				return err
			}
			neu = strings.TrimSpace(b.String())
		}
		var prev any
		if old.Valid {
			prev = old.String
		}
		if prev == neu {
			return nil
		}
		q := `UPDATE _lattis_columns SET rules = ? WHERE tbl = ? AND id = ?`
		return t.apply([]stmt{st(q, neu, tb.ID, c.ID)}, []stmt{st(q, prev, tb.ID, c.ID)})
	})
}

// columnRules lit les règles d'une colonne (JSON brut, vide s'il n'y en a pas).
func (t *tx) columnRules(tbl, col int64) (string, error) {
	var s sql.NullString
	err := t.row(`SELECT rules FROM _lattis_columns WHERE tbl = ? AND id = ?`, tbl, col).Scan(&s)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return s.String, err
}
