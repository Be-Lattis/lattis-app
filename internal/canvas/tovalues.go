package canvas

import (
	"context"
	"math"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Convertir en valeurs (demande de Louis) : une colonne calculée devient une
// colonne saisie qui garde ses valeurs actuelles, au type deviné sur ces
// valeurs (ou sur la formule quand elles sont toutes vides) ; une cellule
// libre avec formule garde sa valeur. La colonne garde son nom : les
// formules qui la citent continuent de la lire, mais elle ne suit plus les
// sources de l'ancienne formule. Une seule étape d'annulation.
//
// Dans un tableau publié d'un classeur collaboratif, une colonne calculée
// n'existe que dans la mise en page ; convertie, elle devient une colonne
// du tableau sur le serveur : le relevé de la structure (oplog) y voit une
// colonne ajoutée, envoyée avant ses valeurs (concepteurs seulement ;
// docs/collaboration/synchronisation.md §2.9).

// ColumnsToValues convertit les colonnes calculées cols (rangs dans le
// tableau) en colonnes de valeurs. Une colonne qui n'est pas calculée est
// refusée.
func (m *Model) ColumnsToValues(ctx context.Context, table string, cols []int) error {
	return m.Do(ctx, "Convertir en valeurs", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		for _, i := range cols {
			if i < 0 || i >= len(tb.Cols) {
				return msg.New("table.columnIndexOutOfRange", msg.Args{"n": i})
			}
			if tb.Cols[i].Formula == "" {
				return msg.Wrap(ErrTable, "table.notComputedColumn", msg.Args{"column": tb.Cols[i].Name})
			}
		}
		n, err := t.columnsToValues(tb, cols)
		if err == nil {
			t.valuesNote(tb, cols, n, 0, 0)
		}
		return err
	})
}

// RangeToValues convertit en valeurs ce qui est calculé dans la plage : les
// colonnes calculées des tableaux qu'elle touche (en entier) et les
// cellules libres avec formule. Les formules qui déversent un tableau
// dynamique (FILTRE, TRIER…) sont gardées.
func (m *Model) RangeToValues(ctx context.Context, sheet int64, r0, c0, r1, c1 int) error {
	return m.Do(ctx, "Convertir en valeurs", func(t *tx) error {
		// Cellules libres d'abord : les colonnes calculées converties ensuite
		// lisent les mêmes valeurs.
		cells, kept, err := t.freeToValues(sheet, r0, c0, r1, c1)
		if err != nil {
			return err
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		var one Table
		var oneCols []int
		cols, tabs := 0, 0
		for _, tb := range tables {
			if !tb.spansCols(c0, c1) || !tb.spansRows(r0, r1) {
				continue
			}
			var idx []int
			for i, c := range tb.Cols {
				if c.Formula != "" && tb.Col+i >= c0 && tb.Col+i <= c1 {
					idx = append(idx, i)
				}
			}
			if len(idx) == 0 {
				continue
			}
			if tb, err = t.editTable(tb.Name); err != nil {
				return err
			}
			if _, err := t.columnsToValues(tb, idx); err != nil {
				return err
			}
			one, oneCols = tb, idx
			cols += len(idx)
			tabs++
		}
		if cols == 0 && cells == 0 {
			if kept > 0 {
				return msg.New("toValues.spillKept", msg.Args{"n": kept})
			}
			return msg.New("toValues.nothing", nil)
		}
		if tabs == 1 && cells == 0 {
			t.valuesNote(one, oneCols, len(oneCols), 0, kept)
			return nil
		}
		t.valuesNote(Table{}, nil, cols, cells, kept)
		return nil
	})
}

// valuesNote : message de la barre d'état.
func (t *tx) valuesNote(tb Table, idx []int, cols, cells, kept int) {
	var parts []string
	switch {
	case cols == 1 && len(idx) == 1:
		parts = append(parts, t.say("toValues.column", msg.Args{"column": tb.Cols[idx[0]].Name}))
	case cols > 0:
		parts = append(parts, t.say("toValues.columns", msg.Args{"n": cols}))
	}
	if cells > 0 {
		id := msg.ID("toValues.cells")
		if len(parts) == 0 {
			id = "toValues.cellsAlone"
		}
		parts = append(parts, t.say(id, msg.Args{"n": cells}))
	}
	note := strings.Join(parts, ", ")
	if kept > 0 {
		note += t.say("toValues.spilledKept", msg.Args{"n": kept})
	}
	t.notes = append(t.notes, note+t.say("toValues.undo", nil))
}

// columnsToValues fait des colonnes calculées idx de tb des colonnes de
// valeurs ; renvoie leur nombre.
func (t *tx) columnsToValues(tb Table, idx []int) (int, error) {
	idx = slices.Clone(idx)
	slices.Sort(idx)
	idx = slices.Compact(idx)
	if len(idx) == 0 {
		return 0, nil
	}
	// 1. Les valeurs de toutes les lignes, lues avant tout changement (une
	// colonne convertie peut en citer une autre), et le type deviné sur
	// chaque formule.
	exprs, args, _, err := t.columnSQL(tb)
	if err != nil {
		return 0, err
	}
	rs, err := t.query(`SELECT id, `+strings.Join(exprs, ", ")+` FROM `+tb.sql()+` ORDER BY _pos`, args...)
	if err != nil {
		return 0, err
	}
	var rids []int64
	vals := make([][]formula.Value, len(idx))
	for rs.Next() {
		var rid int64
		raw := make([]any, len(exprs))
		ptrs := []any{&rid}
		for k := range raw {
			ptrs = append(ptrs, &raw[k])
		}
		if err := rs.Scan(ptrs...); err != nil {
			rs.Close()
			return 0, err
		}
		rids = append(rids, rid)
		for k, i := range idx {
			vals[k] = append(vals[k], formula.FromResult(raw[i]))
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return 0, err
	}
	hints := make([]grid.Type, len(idx))
	for k, i := range idx {
		hints[k] = t.computedResult(tb, tb.Cols[i])
	}
	// 2. Chaque colonne : formule retirée, type des valeurs, valeurs posées
	// (le tableau relu : setType reconstruit la table avec les types du
	// moment).
	for k, i := range idx {
		if tb, err = t.table(tb.ID); err != nil {
			return 0, err
		}
		c := tb.Cols[i]
		to := valuesType(vals[k], hints[k])
		if err := t.setFormula(tb, i, ""); err != nil {
			return 0, err
		}
		if to != c.Type {
			if err := t.setType(tb, i, to); err != nil {
				return 0, err
			}
			// setType fixe le type : la colonne garde le réglage d'avant.
			fix := `UPDATE _lattis_columns SET fixed = ? WHERE tbl = ? AND id = ?`
			if err := t.apply([]stmt{st(fix, c.Fixed, tb.ID, c.ID)}, []stmt{st(fix, 1, tb.ID, c.ID)}); err != nil {
				return 0, err
			}
		}
		nc := Column{ID: c.ID, Name: c.Name, Type: to, Fixed: c.Fixed}
		for r, rid := range rids {
			v, invalid := valueCell(vals[k][r], to)
			if err := t.setValue(tb, rid, nc, v, invalid); err != nil {
				return 0, err
			}
		}
	}
	return len(idx), nil
}

// valuesType devine le type d'une colonne de valeurs calculées : entier,
// décimal, date ou booléen quand toutes les valeurs (hors vides et
// erreurs) le sont, texte sinon ; hint (type deviné sur la formule) quand
// il n'y a aucune valeur.
func valuesType(vals []formula.Value, hint grid.Type) grid.Type {
	var num, dec, date, boolean, n int
	for _, v := range vals {
		switch v.Kind {
		case formula.Empty, formula.Error:
			continue
		case formula.Number:
			num++
			if v.Num != math.Trunc(v.Num) || math.Abs(v.Num) >= 1<<53 {
				dec++
			}
		case formula.Date:
			if v.Num == math.Trunc(v.Num) {
				date++
			}
		case formula.Bool:
			boolean++
		}
		n++
	}
	switch {
	case n == 0:
		if hint != "" {
			return hint
		}
		return grid.Text
	case num == n && (dec > 0 || hint == grid.Decimal):
		return grid.Decimal
	case num == n:
		return grid.Integer
	case date == n:
		return grid.Date
	case boolean == n:
		return grid.Boolean
	}
	return grid.Text
}

// valueCell écrit une valeur calculée dans une colonne de type to ; une
// erreur (#DIV/0!…) hors d'une colonne texte reste visible comme saisie en
// erreur.
func valueCell(v formula.Value, to grid.Type) (any, string) {
	switch v.Kind {
	case formula.Empty:
		return nil, ""
	case formula.Error:
		if to == grid.Text {
			return v.String(), ""
		}
		return nil, v.String()
	}
	switch to {
	case grid.Integer:
		return int64(v.Num), ""
	case grid.Decimal:
		return v.Num, ""
	case grid.Date:
		return formula.ISODate(v.Num), ""
	case grid.Boolean:
		if v.Bool {
			return int64(1), ""
		}
		return int64(0), ""
	}
	return v.String(), ""
}

// freeToValues remplace les formules des cellules libres de la plage par
// leur valeur ; renvoie le nombre de cellules converties et celui des
// formules déversées gardées.
func (t *tx) freeToValues(sheet int64, r0, c0, r1, c1 int) (int, int, error) {
	rs, err := t.query(`SELECT row, col, raw FROM _lattis_cells WHERE sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ? ORDER BY row, col`, sheet, r0, r1, c0, c1)
	if err != nil {
		return 0, 0, err
	}
	var found []cell
	for rs.Next() {
		var c cell
		if err := rs.Scan(&c.Row, &c.Col, &c.Raw); err != nil {
			rs.Close()
			return 0, 0, err
		}
		if isFormula(c.Raw) {
			found = append(found, c)
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return 0, 0, err
	}
	if len(found) == 0 {
		return 0, 0, nil
	}
	if err := t.guard(sheet); err != nil {
		return 0, 0, err
	}
	// Toutes les valeurs d'abord : une cellule convertie peut en citer une
	// autre de la plage.
	type put struct {
		row, col int
		raw      string
	}
	var puts []put
	kept := 0
	for _, c := range found {
		if _, spill, _ := t.spillFreeRaw(sheet, c.Raw); spill {
			kept++
			continue
		}
		puts = append(puts, put{c.Row, c.Col, valueRaw(t.evalFreeRaw(sheet, c.Raw))})
	}
	for _, p := range puts {
		if err := t.setFreeCell(sheet, p.row, p.col, p.raw); err != nil {
			return 0, 0, err
		}
	}
	return len(puts), kept, nil
}
