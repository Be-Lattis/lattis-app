package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lalamefine/slitable/internal/formula"
)

// ErrComputed signale une saisie de valeur dans une colonne calculée.
type ErrComputed struct{ Column string }

func (e ErrComputed) Error() string {
	return fmt.Sprintf("« %s » est une colonne calculée : modifiez sa formule (saisie commençant par =) ou effacez-la", e.Column)
}

// ErrFormula signale une formule qui ne peut pas être enregistrée.
type ErrFormula struct{ Msg string }

func (e ErrFormula) Error() string { return "formule : " + e.Msg }

func isFormula(raw string) bool {
	s := strings.TrimSpace(raw)
	return len(s) > 1 && s[0] == '='
}

// displayFormula réécrit une formule stockée (anglais canonique) en français.
func displayFormula(en string) string {
	if en == "" {
		return ""
	}
	n, err := formula.Parse(en, formula.EN)
	if err != nil {
		return en
	}
	return formula.Format(n, formula.FR)
}

// noRanges refuse les plages A1 : elles n'existent pas dans Slitable.
type noRanges struct{}

func (noRanges) Resolve(string) (string, int, bool) { return "", 0, false }

// setFormula fait de la colonne i une colonne calculée (src vide : retire la
// formule). La formule est analysée en français, les RECHERCHEV converties
// en REFERENCE, et les tableaux cherchés reçoivent une clé unique si besoin.
func (t *tx) setFormula(tb Table, i int, src string) error {
	c := tb.Cols[i]
	q := `UPDATE _slitable_columns SET formula = ? WHERE tbl = ? AND id = ?`
	old := any(nil)
	if c.Formula != "" {
		old = c.Formula
	}
	if src == "" {
		return t.apply([]stmt{st(q, nil, tb.ID, c.ID)}, []stmt{st(q, old, tb.ID, c.ID)})
	}
	n, err := t.prepareFormula(src)
	if err != nil {
		return err
	}
	en := formula.Format(n, formula.EN)
	return t.apply([]stmt{st(q, en, tb.ID, c.ID)}, []stmt{st(q, old, tb.ID, c.ID)})
}

// prepareFormula analyse une saisie de formule (français), convertit les
// RECHERCHEV en REFERENCE et donne une clé unique aux tableaux cherchés.
func (t *tx) prepareFormula(src string) (formula.Node, error) {
	n, err := formula.Parse(src, formula.FR)
	if err != nil {
		return nil, ErrFormula{err.Error()}
	}
	schema, err := t.schema()
	if err != nil {
		return nil, err
	}
	n, needs, err := formula.Convert(n, schema, noRanges{}, src)
	if err != nil {
		return nil, ErrFormula{err.Error()}
	}
	for _, target := range referenceTargets(n) {
		if ft, ok := schema.Table(target); ok && ft.Key == "" && len(ft.Cols) > 0 {
			needs = append(needs, formula.KeyNeed{Table: ft.Name, Column: ft.Cols[0].Name})
		}
	}
	for _, need := range needs {
		if err := t.ensureKey(need.Table, need.Column); err != nil {
			return nil, err
		}
	}
	return n, nil
}

// referenceTargets donne les tableaux dont une formule lit la colonne clé
// (REFERENCE, REFERENCE.APPROCHEE, EXISTE).
func referenceTargets(n formula.Node) []string { return formula.KeyTargets(n) }

// ErrKey signale une colonne qui ne peut pas servir de clé.
type ErrKey struct{ Table, Column, Why string }

func (e ErrKey) Error() string {
	return fmt.Sprintf("%s[%s] ne peut pas servir de clé : %s", e.Table, e.Column, e.Why)
}

// ensureKey fait d'une colonne la clé de son tableau : unicité vérifiée sans
// tenir compte de la casse, index UNIQUE posé (les doublons seront ensuite
// refusés à la saisie).
func (t *tx) ensureKey(table, column string) error {
	tb, err := t.tableByName(table)
	if err != nil {
		return err
	}
	idx := -1
	for i, c := range tb.Cols {
		if strings.EqualFold(c.Name, column) {
			idx = i
		}
	}
	if idx < 0 {
		return ErrKey{table, column, "colonne introuvable"}
	}
	c := tb.Cols[idx]
	if tb.Key == c.ID {
		return nil
	}
	if tb.Key != 0 {
		return ErrKey{table, column, "le tableau a déjà une autre clé"}
	}
	if c.Formula != "" {
		return ErrKey{table, column, "c'est une colonne calculée"}
	}
	expr := keyExpr(c.ID, c.Type)
	var total, distinct int
	if err := t.row(`SELECT count(`+colSQL(c.ID)+`), count(DISTINCT `+expr+`) FROM `+tb.sql()).Scan(&total, &distinct); err != nil {
		return err
	}
	if total != distinct {
		return ErrKey{table, column, fmt.Sprintf("%d valeur(s) en double", total-distinct)}
	}
	index := tb.sql() + "_key"
	return t.apply([]stmt{
		st(`CREATE UNIQUE INDEX ` + index + ` ON ` + tb.sql() + `(` + expr + `)`),
		st(`UPDATE _slitable_tables SET key_col = ? WHERE id = ?`, c.ID, tb.ID),
	}, []stmt{
		st(`UPDATE _slitable_tables SET key_col = NULL WHERE id = ?`, tb.ID),
		st(`DROP INDEX ` + index),
	})
}

// schema décrit tous les tableaux du classeur pour le moteur de formules.
func (t *tx) schema() (*formula.Schema, error) {
	rs, err := t.query(`SELECT id FROM _slitable_tables ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rs.Close()
	s := &formula.Schema{}
	if t.m != nil {
		s.Now, s.Seed = t.m.volatile()
	}
	for _, id := range ids {
		tb, err := t.table(id)
		if err != nil {
			return nil, err
		}
		ft := &formula.Table{Name: tb.Name, SQL: tb.sql(), Visible: tb.visibleSQL()}
		for _, c := range tb.Cols {
			fc := formula.Column{Name: c.Name, SQL: colSQL(c.ID), Type: c.Type}
			if c.Formula != "" {
				if fc.Formula, err = formula.Parse(c.Formula, formula.EN); err != nil {
					fc.Formula = formula.ErrLit{V: formula.ErrName}
				}
			}
			if c.ID == tb.Key {
				ft.Key = c.Name
			}
			ft.Cols = append(ft.Cols, fc)
		}
		s.Tables = append(s.Tables, ft)
	}
	return s, nil
}

// evalFree calcule une cellule libre avec formule (moteur SQL : agrégats et
// REFERENCE sur les tableaux du classeur).
func (t *tx) evalFree(raw string) formula.Value {
	n, err := formula.Parse(raw, formula.FR)
	if err != nil {
		return formula.Err(formula.ErrName)
	}
	schema, err := t.schema()
	if err != nil {
		return formula.Err(formula.ErrValue)
	}
	if n, _, err = formula.Convert(n, schema, noRanges{}, raw); err != nil {
		return formula.Err(formula.ErrName)
	}
	names, err := t.nameValues(schema)
	if err != nil {
		return formula.Err(formula.ErrValue)
	}
	c := formula.Compile(n, schema, names, nil, "")
	var v any
	if err := t.row(`SELECT `+c.SQL, c.Args...).Scan(&v); err != nil {
		return formula.Err(formula.ErrValue)
	}
	return formula.FromResult(v)
}

// ColumnExprs donne, pour la lecture par fenêtres d'un tableau, l'expression
// SQL de chaque colonne affichée : la colonne stockée, ou la formule compilée
// d'une colonne calculée. Les agrégats qui ne dépendent pas de la ligne sont
// calculés une fois ici et passés en paramètres (args, numérotés ?1, ?2…).
func (m *Model) ColumnExprs(ctx context.Context, table int64) (exprs []string, args []any, computed []bool, err error) {
	err = m.read(ctx, func(t *tx) error {
		tb, err := t.table(table)
		if err != nil {
			return err
		}
		schema, err := t.schema()
		if err != nil {
			return err
		}
		ft, ok := schema.Table(tb.Name)
		if !ok {
			return errors.New("tableau introuvable")
		}
		names, err := t.nameValues(schema)
		if err != nil {
			return err
		}
		var hoistErr error
		hoist := func(c formula.Compiled) formula.Value {
			var v any
			if err := t.row(`SELECT `+c.SQL, c.Args...).Scan(&v); err != nil {
				hoistErr = err
				return formula.Err(formula.ErrValue)
			}
			return formula.FromResult(v)
		}
		// Une seule compilation pour toutes les colonnes : les paramètres
		// restent numérotés de façon cohérente.
		for _, c := range tb.Cols {
			if c.Formula == "" {
				exprs = append(exprs, tb.sql()+"."+colSQL(c.ID))
				computed = append(computed, false)
				continue
			}
			fc := ft.Cols[len(exprs)]
			cc := formula.CompileHoistedFrom(fc.Formula, schema, names, ft, tb.sql(), hoist, len(args))
			exprs = append(exprs, cc.SQL)
			args = append(args, cc.Args...)
			computed = append(computed, true)
		}
		return hoistErr
	})
	return exprs, args, computed, err
}
