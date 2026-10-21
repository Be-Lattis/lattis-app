package canvas

import (
	"context"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// ErrComputed signale une saisie de valeur dans une colonne calculée.
type ErrComputed struct{ Column string }

func (e ErrComputed) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e ErrComputed) Msg() msg.Message {
	return msg.M("column.computedInput", msg.Args{"column": e.Column})
}

// ErrFormula signale une formule qui ne peut pas être enregistrée ; Why
// dit pourquoi (dans la langue de la session).
type ErrFormula struct{ Why any }

func (e ErrFormula) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e ErrFormula) Msg() msg.Message { return msg.M("formula.refused", msg.Args{"why": e.Why}) }

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

// noRanges refuse les plages A1 : elles n'existent pas dans Lattis.
type noRanges struct{}

func (noRanges) Resolve(string) (string, int, bool) { return "", 0, false }

// setFormula fait de la colonne i une colonne calculée (src vide : retire la
// formule). La formule est analysée en français et les RECHERCHEV converties
// (voir prepareFormula).
func (t *tx) setFormula(tb Table, i int, src string) error {
	c := tb.Cols[i]
	q := `UPDATE _lattis_columns SET formula = ? WHERE tbl = ? AND id = ?`
	old := any(nil)
	if c.Formula != "" {
		old = c.Formula
	}
	if src == "" {
		return t.apply([]stmt{st(q, nil, tb.ID, c.ID)}, []stmt{st(q, old, tb.ID, c.ID)})
	}
	n, err := t.prepareFormula(tb.Sheet, src)
	if err != nil {
		return err
	}
	en := formula.Format(n, formula.EN)
	return t.apply([]stmt{st(q, en, tb.ID, c.ID)}, []stmt{st(q, old, tb.ID, c.ID)})
}

// prepareFormula analyse une saisie de formule (français) et convertit les
// RECHERCHEV. Une recherche (REFERENCE, REFERENCE.APPROCHEE, EXISTE,
// RECHERCHEV, RECHERCHEX) dans un tableau sans clé ne pose jamais de clé
// d'elle-même (choix de Louis, 19/10/2026). Pendant une saisie (keyAsk), sans
// réponse dans le contexte (WithKeyChoice), elle est refusée par
// ErrKeyChoice ; KeyColumn fait de la colonne cherchée la clé, KeyIndex garde
// les numéros de ligne (et ne propose plus de clé pour ce tableau), dans la
// même étape d'annulation. Hors saisie, les numéros de ligne, sans question.
//
// Les adresses A1 tapées à la Excel (=C14*2, SOMME(C14:C16)) sont réécrites
// selon la feuille sheet : Cell[C14], Tableau[Col][k], Cell[C14:C16]…
func (t *tx) prepareFormula(sheet int64, src string) (formula.Node, error) {
	schema, err := t.schema()
	if err != nil {
		return nil, err
	}
	byRow, err := t.rowTables()
	if err != nil {
		return nil, err
	}
	convert := func() (formula.Node, []formula.KeyNeed, error) {
		n, err := formula.Parse(src, formula.FR)
		if err != nil {
			return nil, nil, ErrFormula{err}
		}
		loc := t.locator(sheet)
		n, needs, err := formula.ConvertWith(n, schema, formula.A1Resolver(loc), src, func(table string) bool {
			return !t.keyAsk || byRow[strings.ToLower(table)]
		})
		if err == nil {
			n, err = formula.ConvertA1(n, loc)
		}
		if loc.err != nil {
			return nil, nil, loc.err
		}
		if err != nil {
			return nil, nil, ErrFormula{err}
		}
		return n, needs, nil
	}
	n, needs, err := convert()
	if err != nil || !t.keyAsk {
		return n, err
	}
	for _, target := range formula.KeyTargets(n) {
		if ft, ok := schema.Table(target); ok && len(ft.Cols) > 0 {
			needs = append(needs, formula.KeyNeed{Table: ft.Name, Column: ft.Cols[0].Name})
		}
	}
	// Tableaux sans clé où la formule cherche, sans choix déjà fait.
	var pending []formula.KeyNeed
	seen := map[string]bool{}
	for _, need := range needs {
		low := strings.ToLower(need.Table)
		if ft, ok := schema.Table(need.Table); ok && ft.Key == "" && !byRow[low] && !seen[low] && !pivotSchemaTable(schema, need.Table) {
			seen[low] = true
			pending = append(pending, need)
		}
	}
	if len(pending) == 0 {
		return n, nil
	}
	switch KeyChoiceFrom(t.Ctx) {
	case KeyColumn:
		for _, need := range pending {
			if err := t.keyFor(need); err != nil {
				return nil, err
			}
		}
		return n, nil
	case KeyIndex:
		for _, need := range pending {
			if err := t.keepRowNumbers(need.Table); err != nil {
				return nil, err
			}
			byRow[strings.ToLower(need.Table)] = true
		}
		n, _, err = convert() // RECHERCHEV : INDEX/EQUIV au lieu de REFERENCE
		return n, err
	}
	return nil, t.keyQuestion(pending[0])
}

// rowTables : tableaux sans clé dont les lignes se désignent par leur numéro
// (choix fait à une formule, ou « Ne plus proposer » la clé), en minuscules.
func (t *tx) rowTables() (map[string]bool, error) {
	rs, err := t.query(`SELECT name FROM _lattis_tables WHERE key_col IS NULL AND no_key_hint = 1`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[string]bool{}
	for rs.Next() {
		var name string
		if err := rs.Scan(&name); err != nil {
			return nil, err
		}
		out[strings.ToLower(name)] = true
	}
	return out, rs.Err()
}

// keyFor fait de la colonne cherchée la clé de son tableau (mêmes règles que
// « Clé du tableau » : valeurs uniques et toutes remplies).
func (t *tx) keyFor(need formula.KeyNeed) error {
	tb, err := t.tableByName(need.Table)
	if err != nil {
		return err
	}
	i := colIndex(tb, need.Column)
	if i < 0 {
		return ErrKey{need.Table, need.Column, t.say("key.whyNoColumn", nil)}
	}
	why, err := t.keyCheck(tb, i)
	if err != nil {
		return err
	}
	if why != "" {
		return ErrKey{tb.Name, tb.Cols[i].Name, why}
	}
	return t.ensureKey(tb.Name, tb.Cols[i].Name)
}

// keyQuestion : la question à poser pour un tableau sans clé.
func (t *tx) keyQuestion(need formula.KeyNeed) error {
	tb, err := t.tableByName(need.Table)
	if err != nil {
		return err
	}
	e := ErrKeyChoice{Table: tb.Name, Column: need.Column}
	if i := colIndex(tb, need.Column); i >= 0 {
		e.Column = tb.Cols[i].Name
		if e.Why, err = t.keyCheck(tb, i); err != nil {
			return err
		}
	}
	return e
}

func colIndex(tb Table, name string) int {
	for i, c := range tb.Cols {
		if strings.EqualFold(c.Name, name) {
			return i
		}
	}
	return -1
}

// ErrKey signale une colonne qui ne peut pas servir de clé.
type ErrKey struct{ Table, Column, Why string }

func (e ErrKey) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e ErrKey) Msg() msg.Message {
	return msg.M("key.refused", msg.Args{"table": e.Table, "column": e.Column, "why": e.Why})
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
		return ErrKey{table, column, t.say("key.whyNoColumn", nil)}
	}
	c := tb.Cols[idx]
	if tb.Key == c.ID {
		return nil
	}
	if tb.Key != 0 {
		return ErrKey{table, column, t.say("key.whyOtherKey", nil)}
	}
	if c.Formula != "" {
		return ErrKey{table, column, t.say("key.whyComputed", nil)}
	}
	expr := keyExpr(c.ID, c.Type)
	var total, distinct int
	counted := colSQL(c.ID)
	if c.Type == grid.Text {
		counted = "*" // une case vide compte (chaîne vide)
	}
	if err := t.row(`SELECT count(`+counted+`), count(DISTINCT `+expr+`) FROM `+tb.sql()).Scan(&total, &distinct); err != nil {
		return err
	}
	if total != distinct {
		return ErrKey{table, column, t.say("key.whyDuplicates", msg.Args{"n": total - distinct})}
	}
	index := tb.sql() + "_key"
	t.keyed = tb.Name + "." + c.Name
	return t.apply([]stmt{
		st(`CREATE UNIQUE INDEX ` + index + ` ON ` + tb.sql() + `(` + expr + `)`),
		st(`UPDATE _lattis_tables SET key_col = ? WHERE id = ?`, c.ID, tb.ID),
	}, []stmt{
		st(`UPDATE _lattis_tables SET key_col = NULL WHERE id = ?`, tb.ID),
		st(`DROP INDEX ` + index),
	})
}

// schema décrit tous les tableaux du classeur pour le moteur de formules.
// Une lecture le garde pour la génération du Store (Recalc l'oublie avec
// l'instant et la graine) : la mise en page d'une feuille calcule chaque
// formule libre, et relire tous les tableaux pour chacune rendait le
// passage à une autre feuille très lent (environ 1 ms par formule avec une
// dizaine de tableaux, plus avec davantage). Le moteur de formules ne le
// modifie pas.
func (t *tx) schema() (*formula.Schema, error) {
	s, err := t.schemaBase()
	return t.withPivots(s), err
}

// schemaBase est le schéma en cache, sans les tableaux croisés.
func (t *tx) schemaBase() (*formula.Schema, error) {
	return cached(t, "schema", t.schemaRaw)
}

// withPivots rend une copie du schéma s qui trouve aussi les tableaux
// croisés par leur nom (TCD1[Nombre], pivotref.go), calculés à la demande
// dans cette transaction (le schéma en cache ne garde pas la transaction).
func (t *tx) withPivots(s *formula.Schema) *formula.Schema {
	if s == nil {
		return nil
	}
	c := *s
	c.Extra = t.pivotFormulaTable
	return &c
}

func (t *tx) schemaRaw() (*formula.Schema, error) {
	rs, err := t.query(`SELECT id FROM _lattis_tables ORDER BY id`)
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
	s := &formula.Schema{Sheets: map[string]bool{}}
	if t.m != nil {
		s.Now, s.Seed = t.m.volatile()
		// Nombres écrits en texte : selon la langue de la session (décision 0013).
		s.Numbers = t.m.Locale()
	}
	sheets, err := t.sheetIDs()
	if err != nil {
		return nil, err
	}
	for name := range sheets {
		s.Sheets[name] = true // déjà en minuscules
	}
	for _, id := range ids {
		tb, err := t.table(id)
		if err != nil {
			return nil, err
		}
		// Formules des autres feuilles : SOUS.TOTAL suit le filtre de
		// l'utilisateur, pas le choix de l'onglet (voir schemaFor).
		ft := &formula.Table{Name: tb.Name, SQL: tb.sql(), Visible: tb.userVisibleSQL(), Sheet: tb.Sheet}
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
	// ONGLET.CHOIX() : valeur choisie de chaque onglet à choix.
	for _, id := range sheets {
		ch, err := t.sheetChoice(id)
		if err != nil {
			return nil, err
		}
		if ch != nil {
			if s.Choices == nil {
				s.Choices = map[int64]formula.Value{}
			}
			s.Choices[id] = choiceValue(ch)
		}
	}
	return s, nil
}

// schemaFor est le schéma vu par une formule libre de la feuille sheet : les
// tableaux de la feuille qui suivent son onglet à choix n'y montrent que la
// valeur choisie (SOMME(Ventes[Montant]) posé sous le tableau donne le total
// de l'année affichée, comme dans l'ancien onglet « 2025 »), et SOUS.TOTAL y
// suit aussi le filtre de l'utilisateur. Les autres feuilles voient tout.
func (t *tx) schemaFor(sheet int64) (*formula.Schema, error) {
	s, err := cached(t, fmt.Sprintf("schema:%d", sheet), func() (*formula.Schema, error) {
		base, err := t.schemaBase()
		if err != nil {
			return nil, err
		}
		s := *base
		s.Sheet = sheet
		ch, err := t.sheetChoice(sheet)
		if err != nil || ch == nil || ch.All {
			return &s, err
		}
		tables, err := t.choiceTables(sheet)
		if err != nil {
			return nil, err
		}
		if len(tables) == 0 {
			return &s, nil
		}
		follow := map[string]Table{}
		for _, tb := range tables {
			follow[strings.ToLower(tb.Name)] = tb
		}
		s.Tables = make([]*formula.Table, len(base.Tables))
		for i, ft := range base.Tables {
			tb, ok := follow[strings.ToLower(ft.Name)]
			if !ok {
				s.Tables[i] = ft
				continue
			}
			c := *ft
			c.SQL = "(SELECT * FROM " + tb.sql() + " WHERE " + tb.choice.cond() + ")"
			c.Visible = tb.visibleSQL()
			s.Tables[i] = &c
		}
		return &s, nil
	})
	return t.withPivots(s), err
}

// evalFree calcule une cellule libre avec formule (moteur SQL : agrégats et
// REFERENCE sur les tableaux du classeur).
func (t *tx) evalFreeRaw(sheet int64, raw string) formula.Value {
	n, err := formula.Parse(raw, formula.FR)
	if err != nil {
		return formula.Err(formula.ErrName)
	}
	base, err := t.schema()
	if err != nil {
		return formula.Err(formula.ErrValue)
	}
	// Tableaux de la feuille filtrés par son onglet à choix (schemaFor).
	schema, err := t.schemaFor(sheet)
	if err != nil {
		return formula.Err(formula.ErrValue)
	}
	if n, _, err = formula.Convert(n, schema, noRanges{}, raw); err != nil {
		return formula.Err(formula.ErrName)
	}
	n = formula.QualifyCells(n, sheet)
	names, err := t.nameValues(base)
	if err != nil {
		return formula.Err(formula.ErrValue)
	}
	// Agrégats précalculés : une colonne calculée par SOMME.SI lue dans un
	// agrégat (MOYENNE(T[Montant])) ne refait pas un parcours par ligne.
	hoist, _ := t.hoister()
	c := formula.CompileHoisted(n, schema, names, nil, "", hoist)
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
		exprs, args, computed, err = t.columnSQL(tb)
		return err
	})
	return exprs, args, computed, err
}

// columnSQL : voir ColumnExprs.
func (t *tx) columnSQL(tb Table) (exprs []string, args []any, computed []bool, err error) {
	err = func() error {
		schema, err := t.schema()
		if err != nil {
			return err
		}
		ft, ok := schema.Table(tb.Name)
		if !ok {
			return msg.New("table.notFoundPlain", nil)
		}
		names, err := t.nameValues(schema)
		if err != nil {
			return err
		}
		hoist, hoistErr := t.hoister()
		// Une seule compilation pour toutes les colonnes : les paramètres
		// restent numérotés de façon cohérente.
		for _, c := range tb.Cols {
			if c.Formula == "" {
				exprs = append(exprs, tb.sql()+"."+colSQL(c.ID))
				computed = append(computed, false)
				continue
			}
			cc := formula.CompileColumnFrom(ft, len(exprs), schema, names, tb.sql(), hoist, len(args))
			exprs = append(exprs, cc.SQL)
			args = append(args, cc.Args...)
			computed = append(computed, true)
		}
		return *hoistErr
	}()
	return exprs, args, computed, err
}

// hoister calcule à part, une fois, les agrégats qui ne dépendent pas de la
// ligne (dont les totaux par valeur des familles .SI) ; l'erreur éventuelle
// est rangée dans *err.
func (t *tx) hoister() (func(formula.Compiled) formula.Value, *error) {
	var hoistErr error
	return func(c formula.Compiled) formula.Value {
		var v any
		if err := t.row(`SELECT `+c.SQL, c.Args...).Scan(&v); err != nil {
			hoistErr = err
			return formula.Err(formula.ErrValue)
		}
		return formula.FromResult(v)
	}, &hoistErr
}
