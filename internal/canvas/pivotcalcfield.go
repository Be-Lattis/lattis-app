package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lalamefine/lattis/internal/msg"
	"slices"
	"strings"
)

// Champs calculés des tableaux croisés (« Champ calculé… », PivotCalcField) :
// une formule sur les colonnes de chaque ligne du tableau source, qui sert
// de champ comme une colonne (regroupement en lignes ou en colonnes,
// filtre, valeur). Voir choiceformula.go pour le calcul.

// prepareCalc vérifie les champs calculés d'une configuration et convertit
// leurs formules saisies (Text, en français) en anglais canonique
// (Formula) ; Text est retiré (il n'est rendu qu'à la lecture).
func (t *tx) prepareCalc(cfg *PivotConfig) error {
	if len(cfg.Calc) == 0 {
		return nil
	}
	tb, err := t.tableByName(cfg.Source)
	if err != nil {
		return msg.Wrap(ErrPivot, "pivot.tableNotFound", msg.Args{"table": cfg.Source})
	}
	calc := make([]PivotCalcField, len(cfg.Calc))
	seen := map[string]bool{}
	for i, f := range cfg.Calc {
		f.Name = strings.TrimSpace(f.Name)
		if f.Name == "" {
			return msg.Wrap(ErrPivot, "pivot.nameField", nil)
		}
		if seen[Fold(f.Name)] {
			return msg.Wrap(ErrPivot, "pivot.duplicateField", msg.Args{"field": f.Name})
		}
		seen[Fold(f.Name)] = true
		if colIndex(tb, f.Name) >= 0 {
			return msg.Wrap(ErrPivot, "pivot.fieldIsColumn", msg.Args{"field": f.Name, "table": tb.Name})
		}
		if strings.TrimSpace(f.Text) != "" {
			en, cols, err := t.prepareRowFormula(tb.Sheet, f.Text)
			if err != nil {
				return msg.Wrap(ErrPivot, "pivot.fieldError", msg.Args{"field": f.Name, "err": formulaError(err)})
			}
			if miss := missingColumns(tb, cols); len(miss) > 0 {
				return msg.Wrap(ErrPivot, "pivot.fieldMissingColumns", msg.Args{"field": f.Name, "columns": quoteList(miss), "table": tb.Name})
			}
			f.Formula = en
		}
		if f.Formula == "" {
			return msg.Wrap(ErrPivot, "pivot.fieldNoFormula", msg.Args{"field": f.Name})
		}
		f.Text = ""
		calc[i] = f
	}
	cfg.Calc = calc
	return nil
}

// calcText rend la formule de chaque champ calculé en français (Text).
func calcText(cfg *PivotConfig) {
	if len(cfg.Calc) == 0 {
		return
	}
	calc := slices.Clone(cfg.Calc)
	for i := range calc {
		calc[i].Text = displayFormula(calc[i].Formula)
	}
	cfg.Calc = calc
}

// PreviewPivotFormula calcule l'aperçu d'un champ calculé sur le tableau
// source (src en français) : premières valeurs et leur nombre ; Error :
// formule refusée.
func (m *Model) PreviewPivotFormula(ctx context.Context, source, src string) (ChoicePreview, error) {
	out := ChoicePreview{Values: []FilterValue{}, Tables: []string{}, Others: []string{}}
	err := m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(source)
		if err != nil {
			out.Error = err.Error()
			return nil
		}
		en, cols, err := t.prepareRowFormula(tb.Sheet, src)
		if err != nil {
			out.Error = formulaError(err)
			return nil
		}
		if miss := missingColumns(tb, cols); len(miss) > 0 {
			out.Error = fmt.Sprintf("%s n'a pas les colonnes %s", tb.Name, quoteList(miss))
			return nil
		}
		expr, ok, err := t.rowLabelSQL(tb, en)
		if err != nil || !ok {
			return err
		}
		out.Tables = append(out.Tables, tb.Name)
		rs, err := t.query(`SELECT `+expr+` AS v, count(*) FROM `+tb.sql()+` GROUP BY v HAVING v <> '' LIMIT ?`, MaxFilterValues)
		if err != nil {
			return err
		}
		defer rs.Close()
		var all []FilterValue
		for rs.Next() {
			var v FilterValue
			if err := rs.Scan(&v.Text, &v.Count); err != nil {
				return err
			}
			all = append(all, v)
		}
		if err := rs.Err(); err != nil {
			return err
		}
		sortChoiceValues(all)
		out.Count = len(all)
		out.Values = append(out.Values, all[:min(len(all), choicePreviewValues)]...)
		return nil
	})
	return out, err
}

// SetPivotCalcField ajoute (old vide), modifie ou retire (src vide) un champ
// calculé d'un tableau croisé ; une étape d'annulation. Renommé, il garde sa
// place dans les zones ; retiré, il en sort.
func (m *Model) SetPivotCalcField(ctx context.Context, id int64, old, name, src string) error {
	label := "Champ calculé"
	if strings.TrimSpace(src) == "" {
		label = "Supprimer le champ calculé"
	}
	return m.Do(ctx, label, func(t *tx) error {
		if err := t.guardObject(`_lattis_pivots`, id); err != nil {
			return err
		}
		var raw string
		if err := t.row(`SELECT config FROM _lattis_pivots WHERE id = ?`, id).Scan(&raw); err != nil {
			return err
		}
		var cfg PivotConfig
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return fmt.Errorf("%w : configuration illisible", ErrPivot)
		}
		cfg = cfg.normalized()
		calc := slices.Clone(cfg.Calc)
		at := slices.IndexFunc(calc, func(f PivotCalcField) bool { return old != "" && strings.EqualFold(f.Name, old) })
		if old != "" && at < 0 {
			return msg.Wrap(ErrPivot, "pivot.fieldNotFound", msg.Args{"field": old})
		}
		switch {
		case strings.TrimSpace(src) == "":
			if at >= 0 {
				calc = slices.Delete(calc, at, at+1)
				cfg = renamePivotField(cfg, old, "")
			}
		case at >= 0:
			calc[at] = PivotCalcField{Name: name, Text: src}
			if !strings.EqualFold(old, strings.TrimSpace(name)) {
				cfg = renamePivotField(cfg, old, strings.TrimSpace(name))
			}
		default:
			calc = append(calc, PivotCalcField{Name: name, Text: src})
		}
		cfg.Calc = calc
		if err := t.prepareCalc(&cfg); err != nil {
			return err
		}
		if _, err := t.pivotPlan(cfg); err != nil {
			return err
		}
		b, err := json.Marshal(cfg.stored())
		if err != nil {
			return err
		}
		q := `UPDATE _lattis_pivots SET config = ? WHERE id = ?`
		return t.apply([]stmt{st(q, string(b), id)}, []stmt{st(q, raw, id)})
	})
}

// renamePivotField renomme un champ dans les zones d'un TCD (neu vide : le
// retire des zones).
func renamePivotField(cfg PivotConfig, old, neu string) PivotConfig {
	same := func(s string) bool { return strings.EqualFold(s, old) }
	var rows []string
	for _, f := range cfg.RowFields {
		switch {
		case !same(f):
			rows = append(rows, f)
		case neu != "":
			rows = append(rows, neu)
		}
	}
	cfg.RowFields = rows
	if same(cfg.Cols) {
		cfg.Cols = neu
	}
	var vals []PivotValue
	for _, v := range cfg.Values {
		if same(v.Field) {
			if neu == "" {
				continue
			}
			v.Field = neu
		}
		vals = append(vals, v)
	}
	cfg.Values = vals
	filters := func(list []PivotFilter) []PivotFilter {
		var out []PivotFilter
		for _, f := range list {
			if same(f.Field) && f.Formula == "" {
				if neu == "" {
					continue
				}
				f.Field = neu
			}
			out = append(out, f)
		}
		return out
	}
	cfg.Filters, cfg.Items = filters(cfg.Filters), filters(cfg.Items)
	if k, ok := foldKey(cfg.Sort, old); ok {
		sort := map[string]string{}
		for kk, v := range cfg.Sort {
			if kk != k {
				sort[kk] = v
			} else if neu != "" {
				sort[neu] = v
			}
		}
		cfg.Sort = sort
	}
	if k, ok := foldKey(cfg.Orders, old); ok {
		orders := map[string]CustomOrder{}
		for kk, v := range cfg.Orders {
			if kk != k {
				orders[kk] = v
			} else if neu != "" {
				orders[neu] = v
			}
		}
		cfg.Orders = orders
	}
	return cfg
}

// byFormula : le TCD lit une formule par ligne (champ calculé, onglet à
// choix par une formule), qui peut citer d'autres tableaux que sa source.
func (c PivotConfig) byFormula() bool {
	return len(c.Calc) > 0 || c.Choice != nil && c.Choice.Formula != ""
}
