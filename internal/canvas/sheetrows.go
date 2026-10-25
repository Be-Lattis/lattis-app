package canvas

import (
	"context"
	"fmt"

	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/work"
)

// Insertion et suppression de lignes ou de colonnes entières de la feuille,
// comme dans Excel : tout ce qui est après se décale (cellules libres,
// styles, noms, tableaux, TCD, graphiques, boutons, hauteurs ou largeurs) ;
// un tableau traversé reçoit des lignes (ou des colonnes) vides, ou perd
// celles qui sont supprimées. Les adresses Cell_A1 des formules suivent ;
// une cellule supprimée devient #REF!.

// axis désigne les lignes ou les colonnes de la feuille.
type axis string

const (
	rowsAxis axis = "row"
	colsAxis axis = "col"
)

// Tables des métadonnées positionnées sur une feuille (colonnes sheet, row, col).
var positioned = []string{"_lattis_cells", "_lattis_choice_cells", "_lattis_cell_styles", "_lattis_range_cells", "_lattis_names", "_lattis_tables", "_lattis_pivots", "_lattis_charts", "_lattis_buttons", "_lattis_objects"}

// Objets ancrés (pas de contenu par cellule) : leur ancre recule au début
// d'une plage supprimée au lieu de disparaître.
var anchored = []string{"_lattis_pivots", "_lattis_charts", "_lattis_buttons", "_lattis_objects"}

func (a axis) sizes() string {
	if a == rowsAxis {
		return "_lattis_row_heights"
	}
	return "_lattis_col_widths"
}

// lineStyles est la table des styles de lignes (ou de colonnes) entières.
func (a axis) lineStyles() string {
	if a == rowsAxis {
		return "_lattis_row_styles"
	}
	return "_lattis_col_styles"
}

// InsertSheetRows insère n lignes vides avant la ligne at de la feuille.
func (m *Model) InsertSheetRows(ctx context.Context, sheet int64, at, n int) error {
	return m.Do(ctx, plural(n, "Insérer une ligne", "Insérer des lignes"), func(t *tx) error {
		work.Op(ctx, "insertRows", "")
		return t.guarded(sheet, func() error { return t.insertLines(sheet, rowsAxis, at, n) })
	})
}

// DeleteSheetRows supprime les lignes r0 à r1 de la feuille.
func (m *Model) DeleteSheetRows(ctx context.Context, sheet int64, r0, r1 int) error {
	return m.Do(ctx, plural(r1-r0+1, "Supprimer une ligne", "Supprimer des lignes"), func(t *tx) error {
		work.Op(ctx, "deleteRows", "")
		return t.guarded(sheet, func() error { return t.deleteLines(sheet, rowsAxis, r0, r1) })
	})
}

// InsertSheetColumns insère n colonnes vides avant la colonne at de la feuille.
func (m *Model) InsertSheetColumns(ctx context.Context, sheet int64, at, n int) error {
	return m.Do(ctx, plural(n, "Insérer une colonne", "Insérer des colonnes"), func(t *tx) error {
		work.Op(ctx, "insertColumn", "")
		return t.guarded(sheet, func() error { return t.insertLines(sheet, colsAxis, at, n) })
	})
}

// DeleteSheetColumns supprime les colonnes c0 à c1 de la feuille.
func (m *Model) DeleteSheetColumns(ctx context.Context, sheet int64, c0, c1 int) error {
	return m.Do(ctx, plural(c1-c0+1, "Supprimer une colonne", "Supprimer des colonnes"), func(t *tx) error {
		work.Op(ctx, "deleteColumns", "")
		return t.guarded(sheet, func() error { return t.deleteLines(sheet, colsAxis, c0, c1) })
	})
}

func plural(n int, one, many string) string {
	if n > 1 {
		return many
	}
	return one
}

func (t *tx) insertLines(sheet int64, a axis, at, n int) error {
	if at < 0 || n < 1 || n > 10000 {
		return msg.Wrap(ErrTable, "table.insertOutOfSheet", nil)
	}
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	// Les tableaux traversés s'agrandissent ; les autres se décalent avec le reste.
	for _, tb := range tables {
		if a == rowsAxis && tb.Row < at && at <= tb.Bottom() {
			if err := t.insertTableRows(tb, at-tb.Row-1, n); err != nil {
				return err
			}
		}
		if a == colsAxis && tb.Col < at && at <= tb.Right() {
			if err := t.insertTableCols(tb, at-tb.Col, n); err != nil {
				return err
			}
		}
	}
	for _, tbl := range append(positioned, a.sizes(), a.lineStyles(), a.lineRules()) {
		if err := t.shiftPos(tbl, a, sheet, at, n); err != nil {
			return err
		}
	}
	move := func(i int) (int, bool) {
		if i >= at {
			return i + n, true
		}
		return i, true
	}
	if err := t.shiftPageSetup(sheet, a, move); err != nil {
		return err
	}
	return t.rewriteCellRefs(sheet, along(a, move))
}

func (t *tx) deleteLines(sheet int64, a axis, lo, hi int) error {
	if lo < 0 || hi < lo {
		return msg.Wrap(ErrTable, "table.deleteOutOfSheet", nil)
	}
	n := hi - lo + 1
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	for _, tb := range tables {
		first, last := tb.Top(), tb.Bottom()
		if a == colsAxis {
			first, last = tb.Col, tb.Right()
		}
		switch {
		case hi < first || lo > last:
			continue
		case lo <= first && last <= hi:
			if err := t.deleteTable(tb); err != nil {
				return err
			}
		case a == rowsAxis && lo <= tb.Row:
			id := msg.ID("table.headerRowNotDeletable")
			if tb.TwoLevel {
				id = "table.headerRowsNotDeletable"
			}
			return msg.Wrap(ErrTable, id, msg.Args{"table": tb.Name})
		case a == rowsAxis:
			if err := t.deleteTableRows(tb, lo-tb.Row-1, min(hi, last)-tb.Row-1); err != nil {
				return err
			}
			// Les dernières lignes parties, les lignes vides restées au bas
			// partent aussi (décision 0009, règle du bas).
			if hi >= last {
				if _, err := t.trimBottom(tb.ID); err != nil {
					return err
				}
			}
		default:
			if err := t.deleteTableCols(tb, max(lo, first)-tb.Col, min(hi, last)-tb.Col); err != nil {
				return err
			}
			if first >= lo {
				// Les premières colonnes partent : le tableau commence au début de la plage.
				q := `UPDATE _lattis_tables SET col = ? WHERE id = ?`
				if err := t.apply([]stmt{st(q, lo, tb.ID)}, []stmt{st(q, tb.Col, tb.ID)}); err != nil {
					return err
				}
			}
		}
	}
	for _, tbl := range []string{"_lattis_cells", "_lattis_choice_cells", "_lattis_cell_styles", "_lattis_range_cells", "_lattis_names", a.sizes(), a.lineStyles(), a.lineRules()} {
		if err := t.deleteIn(tbl, a, sheet, lo, hi); err != nil {
			return err
		}
	}
	for _, tbl := range anchored {
		if err := t.clampAnchors(tbl, a, sheet, lo, hi); err != nil {
			return err
		}
	}
	for _, tbl := range append(positioned, a.sizes(), a.lineStyles(), a.lineRules()) {
		if err := t.shiftPos(tbl, a, sheet, hi+1, -n); err != nil {
			return err
		}
	}
	move := func(i int) (int, bool) {
		switch {
		case i > hi:
			return i - n, true
		case i >= lo:
			return 0, false
		}
		return i, true
	}
	if err := t.shiftPageSetup(sheet, a, move); err != nil {
		return err
	}
	return t.rewriteCellRefs(sheet, along(a, move))
}

// shiftPos décale de d la coordonnée a des enregistrements de la feuille
// placés à from ou au-delà, en deux temps (pas de collision de clé).
func (t *tx) shiftPos(table string, a axis, sheet int64, from, d int) error {
	away := fmt.Sprintf(`UPDATE %s SET %s = -(%s + ?) - 1 WHERE sheet = ? AND %s >= ?`, table, a, a, a)
	back := fmt.Sprintf(`UPDATE %s SET %s = -%s - 1 WHERE sheet = ? AND %s < 0`, table, a, a, a)
	return t.apply(
		[]stmt{st(away, d, sheet, from), st(back, sheet)},
		[]stmt{st(away, -d, sheet, from+d), st(back, sheet)},
	)
}

// deleteIn supprime les enregistrements de la feuille dont la coordonnée a
// est dans [lo, hi] ; l'annulation les réinsère tels quels.
func (t *tx) deleteIn(table string, a axis, sheet int64, lo, hi int) error {
	return t.deleteWhere(table, fmt.Sprintf(`sheet = ? AND %s BETWEEN ? AND ?`, a), sheet, lo, hi)
}

// deleteWhere supprime les enregistrements qui vérifient where ;
// l'annulation les réinsère tels quels.
func (t *tx) deleteWhere(table, where string, args ...any) error {
	rs, err := t.query(`SELECT * FROM `+table+` WHERE `+where, args...)
	if err != nil {
		return err
	}
	cols, err := rs.Columns()
	if err != nil {
		rs.Close()
		return err
	}
	var undo []stmt
	ins := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, table, strings.Join(cols, ", "), strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", "))
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			rs.Close()
			return err
		}
		undo = append(undo, st(ins, vals...))
	}
	rs.Close()
	if len(undo) == 0 {
		return nil
	}
	return t.apply([]stmt{st(`DELETE FROM `+table+` WHERE `+where, args...)}, undo)
}

// clampAnchors ramène au début de la plage supprimée l'ancre des objets qui y étaient.
func (t *tx) clampAnchors(table string, a axis, sheet int64, lo, hi int) error {
	rs, err := t.query(fmt.Sprintf(`SELECT id, %s FROM %s WHERE sheet = ? AND %s BETWEEN ? AND ?`, a, table, a), sheet, lo, hi)
	if err != nil {
		return err
	}
	q := fmt.Sprintf(`UPDATE %s SET %s = ? WHERE id = ?`, table, a)
	var do, undo []stmt
	for rs.Next() {
		var id int64
		var at int
		if err := rs.Scan(&id, &at); err != nil {
			rs.Close()
			return err
		}
		do = append(do, st(q, lo, id))
		undo = append(undo, st(q, at, id))
	}
	rs.Close()
	if len(do) == 0 {
		return nil
	}
	return t.apply(do, undo)
}

// insertTableRows insère n lignes vides avant la ligne de données k.
func (t *tx) insertTableRows(tb Table, k, n int) error {
	if tb.Filtered() {
		return tb.errFiltered(false)
	}
	var before, after string
	var err error
	if k > 0 {
		if _, before, err = t.rowID(tb, k-1); err != nil {
			return err
		}
	}
	if k < tb.Rows {
		if _, after, err = t.rowID(tb, k); err != nil {
			return err
		}
	}
	var id int64
	if err := t.row(`SELECT coalesce(max(id), 0) FROM ` + tb.sql()).Scan(&id); err != nil {
		return err
	}
	for range n {
		pos, err := t.rowKey(tb, before, after)
		if err != nil {
			return err
		}
		id++
		if err := t.insertRow(tb, id, pos); err != nil {
			return err
		}
		before = pos
	}
	return nil
}

// insertTableCols insère n colonnes de texte vides avant la colonne i.
func (t *tx) insertTableCols(tb Table, i, n int) error {
	var id int64
	if err := t.row(`SELECT coalesce(max(id), 0) FROM _lattis_columns WHERE tbl = ?`, tb.ID).Scan(&id); err != nil {
		return err
	}
	taken := map[string]bool{}
	for _, c := range tb.Cols {
		taken[strings.ToLower(c.Name)] = true
	}
	k := 1
	for j := range n {
		name := ""
		for ; ; k++ {
			name = fmt.Sprintf("Colonne%d", k)
			if !taken[strings.ToLower(name)] {
				break
			}
		}
		taken[strings.ToLower(name)] = true
		id++
		if err := t.addColumn(tb, id, name, grid.Text, i+j); err != nil {
			return err
		}
	}
	return nil
}

// deleteTableRows supprime les lignes de données a à b (corbeille d'annulation).
func (t *tx) deleteTableRows(tb Table, a, b int) error {
	if tb.Filtered() {
		return tb.errFiltered(false)
	}
	return t.trashRows(tb, fmt.Sprintf(`SELECT id FROM %s ORDER BY _pos LIMIT %d OFFSET %d`, tb.sql(), b-a+1, a))
}

// deleteTableCols supprime les colonnes i à j du tableau.
func (t *tx) deleteTableCols(tb Table, i, j int) error {
	idx := make([]int, 0, j-i+1)
	for k := i; k <= j; k++ {
		idx = append(idx, k)
	}
	if err := t.dropColumns(tb, idx); err != nil {
		return err
	}
	return t.trimAfterDrop(tb.ID)
}

// trashRows supprime d'un tableau les lignes dont la requête ids donne les
// identifiants : lignes, saisies invalides et styles passent dans des tables
// de corbeille que l'annulation remet en place.
func (t *tx) trashRows(tb Table, ids string) error {
	trash := fmt.Sprintf("_lattis_trash_rows_%d_%d", tb.ID, uniq())
	return t.ApplyStep(command.Step{
		Do: []stmt{
			st(`CREATE TABLE ` + trash + ` AS SELECT * FROM ` + tb.sql() + ` WHERE id IN (` + ids + `)`),
			st(`CREATE TABLE `+trash+`_inv AS SELECT * FROM _lattis_invalid WHERE tbl = ? AND rid IN (SELECT id FROM `+trash+`)`, tb.ID),
			st(`CREATE TABLE `+trash+`_sty AS SELECT * FROM _lattis_styles WHERE tbl = ? AND rid IN (SELECT id FROM `+trash+`)`, tb.ID),
			st(`DELETE FROM _lattis_invalid WHERE tbl = ? AND rid IN (SELECT id FROM `+trash+`)`, tb.ID),
			st(`DELETE FROM _lattis_styles WHERE tbl = ? AND rid IN (SELECT id FROM `+trash+`)`, tb.ID),
			st(`DELETE FROM ` + tb.sql() + ` WHERE id IN (SELECT id FROM ` + trash + `)`),
		},
		Undo: []stmt{
			st(`INSERT INTO ` + tb.sql() + ` SELECT * FROM ` + trash),
			st(`INSERT INTO _lattis_invalid SELECT * FROM ` + trash + `_inv`),
			st(`INSERT INTO _lattis_styles SELECT * FROM ` + trash + `_sty`),
			st(`DROP TABLE ` + trash),
			st(`DROP TABLE ` + trash + `_inv`),
			st(`DROP TABLE ` + trash + `_sty`),
		},
		Purge: []stmt{
			st(`DROP TABLE IF EXISTS ` + trash),
			st(`DROP TABLE IF EXISTS ` + trash + `_inv`),
			st(`DROP TABLE IF EXISTS ` + trash + `_sty`),
		},
	})
}

// along applique un décalage d'une seule coordonnée (ligne ou colonne).
func along(a axis, move func(int) (int, bool)) func(r, c int) (int, int, bool) {
	return func(r, c int) (int, int, bool) {
		if a == rowsAxis {
			r2, ok := move(r)
			return r2, c, ok
		}
		c2, ok := move(c)
		return r, c2, ok
	}
}

// moveRange déplace une plage Cell[a:b] comme Excel : une plage dont les
// coins suivent le même déplacement est déplacée (lignes insérées dedans :
// elle s'agrandit) ; une plage dont des lignes ou colonnes disparaissent se
// réduit à ce qui reste (faux si plus rien) ; une plage dont une partie
// seulement est déplacée (couper-coller) reste en place.
func moveRange(a, b string, move func(r, c int) (int, int, bool)) (formula.CellRange, bool) {
	r1, c1, _ := formula.CellRef("cell_" + a)
	r2, c2, _ := formula.CellRef("cell_" + b)
	r1, r2 = min(r1, r2), max(r1, r2)
	c1, c2 = min(c1, c2), max(c1, c2)
	orig := formula.CellRange{R1: r1, C1: c1, R2: r2, C2: c2}
	tr, tc, ok1 := move(r1, c1)
	br, bc, ok2 := move(r2, c2)
	if ok1 && ok2 {
		// Les deux autres coins doivent suivre (sinon déplacement partiel).
		xr, xc, ok3 := move(r1, c2)
		yr, yc, ok4 := move(r2, c1)
		if ok3 && ok4 && xr == tr && xc == bc && yr == br && yc == tc {
			return formula.CellRange{R1: tr, C1: tc, R2: br, C2: bc}, true
		}
		return orig, true
	}
	// Lignes ou colonnes supprimées : ce qui reste de la plage.
	nr1, nc1, nr2, nc2, found := 0, 0, 0, 0, false
	for r := r1; r <= r2; r++ {
		for c := c1; c <= c2; c++ {
			mr, mc, ok := move(r, c)
			if !ok {
				continue
			}
			if !found {
				nr1, nc1, nr2, nc2, found = mr, mc, mr, mc, true
				continue
			}
			nr1, nc1, nr2, nc2 = min(nr1, mr), min(nc1, mc), max(nr2, mr), max(nc2, mc)
		}
	}
	return formula.CellRange{R1: nr1, C1: nc1, R2: nr2, C2: nc2}, found
}

// rewriteCellRefs met à jour les adresses de cellules (Cell[A1],
// Feuille!Cell[A1], et les anciennes formes) qui désignent la feuille sheet
// après un déplacement : move donne la nouvelle position, ou faux si la
// cellule a disparu (#REF!). Les adresses réécrites suivent la nomenclature
// Cell[A1] / Feuille!Cell[A1].
func (t *tx) rewriteCellRefs(sheet int64, move func(r, c int) (int, int, bool)) error {
	sheetIDs, err := t.sheetIDs()
	if err != nil {
		return err
	}
	rewrite := func(text string, home int64) string {
		return cellRefs(text, func(name, addr, orig string) string {
			on := home
			if name != "" {
				id, ok := sheetIDs[strings.ToLower(name)]
				if !ok {
					return orig
				}
				on = id
			}
			if on != sheet {
				return orig
			}
			if a, b, ok := strings.Cut(addr, ":"); ok {
				rg, ok := moveRange(strings.TrimSpace(a), strings.TrimSpace(b), move)
				if !ok {
					return "#REF!"
				}
				rg.Sheet = name
				return rg.String()
			}
			r, c, _ := formula.CellRef("cell_" + addr)
			r, c, ok := move(r, c)
			if !ok {
				return "#REF!"
			}
			cell := formula.CellName(r, c)
			if name != "" {
				cell = name + "." + cell
			}
			return formula.CellText(cell)
		})
	}
	// Cellules libres (toutes feuilles : les adresses qualifiées).
	type freeCell struct {
		sheet    int64
		row, col int
		raw      string
	}
	var cells []freeCell
	rs, err := t.query(`SELECT sheet, row, col, raw FROM _lattis_cells WHERE raw LIKE '=%' AND raw LIKE '%cell%'`)
	if err != nil {
		return err
	}
	for rs.Next() {
		var c freeCell
		if err := rs.Scan(&c.sheet, &c.row, &c.col, &c.raw); err != nil {
			rs.Close()
			return err
		}
		cells = append(cells, c)
	}
	rs.Close()
	var do, undo []stmt
	qc := `UPDATE _lattis_cells SET raw = ? WHERE sheet = ? AND row = ? AND col = ?`
	for _, c := range cells {
		if neu := rewrite(c.raw, c.sheet); neu != c.raw {
			do = append(do, st(qc, neu, c.sheet, c.row, c.col))
			undo = append(undo, st(qc, c.raw, c.sheet, c.row, c.col))
		}
	}
	// Colonnes calculées : la feuille de leur tableau.
	type colFormula struct {
		tbl, id, sheet int64
		f              string
	}
	var cols []colFormula
	rs, err = t.query(`SELECT c.tbl, c.id, t.sheet, c.formula FROM _lattis_columns c JOIN _lattis_tables t ON t.id = c.tbl
		WHERE c.formula LIKE '%cell%'`)
	if err != nil {
		return err
	}
	for rs.Next() {
		var c colFormula
		if err := rs.Scan(&c.tbl, &c.id, &c.sheet, &c.f); err != nil {
			rs.Close()
			return err
		}
		cols = append(cols, c)
	}
	rs.Close()
	qf := `UPDATE _lattis_columns SET formula = ? WHERE tbl = ? AND id = ?`
	for _, c := range cols {
		if neu := rewrite(c.f, c.sheet); neu != c.f {
			do = append(do, st(qf, neu, c.tbl, c.id))
			undo = append(undo, st(qf, c.f, c.tbl, c.id))
		}
	}
	if len(do) > 0 {
		if err := t.apply(do, undo); err != nil {
			return err
		}
	}
	// Sources des validations tirées d'une plage (feuille de leur couche de style).
	return t.rewriteDVSources(func(src string, home int64) (string, bool) {
		neu := rewrite(src, home)
		return neu, neu != src
	})
}

// trimAfterDrop : après la suppression de colonnes, le tableau se resserre
// (demande de Louis) : les colonnes restées sans nom ni valeur (« Colonne n »
// vides) partent, puis les lignes du bas devenues vides. Une colonne nommée
// reste, même vide ; un tableau filtré, chargé par une requête ou sans
// aucune ligne remplie ne bouge pas.
func (t *tx) trimAfterDrop(id int64) error {
	tb, err := t.table(id)
	if err != nil {
		return err
	}
	if tb.Filtered() {
		return nil
	}
	if fixed, err := t.fixedTables([]Table{tb}); err != nil || fixed[tb.ID] != "" {
		return err
	}
	filled := func(c Column) string {
		return fmt.Sprintf(`(coalesce(%s, '') <> '' OR EXISTS (SELECT 1 FROM _lattis_invalid i WHERE i.tbl = %d AND i.rid = %s.id AND i.col = %d))`, colSQL(c.ID), tb.ID, tb.sql(), c.ID)
	}
	var empty []int
	for i, c := range tb.Cols {
		if c.Formula != "" || !autoColRe.MatchString(c.Name) {
			continue
		}
		var n int
		if err := t.row(`SELECT count(*) FROM ` + tb.sql() + ` WHERE ` + filled(c)).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			empty = append(empty, i)
		}
	}
	if len(empty) > 0 && len(empty) < len(tb.Cols) {
		if err := t.dropColumns(tb, empty); err != nil {
			return err
		}
		if tb, err = t.table(id); err != nil {
			return err
		}
	}
	var conds []string
	for _, c := range tb.Cols {
		if c.Formula == "" {
			conds = append(conds, filled(c))
		}
	}
	if len(conds) == 0 {
		return nil
	}
	// Lignes après la dernière remplie (aucune remplie : rien ne bouge).
	after := ` WHERE _pos > (SELECT max(_pos) FROM ` + tb.sql() + ` WHERE ` + strings.Join(conds, " OR ") + `)`
	var tail int
	if err := t.row(`SELECT count(*) FROM ` + tb.sql() + after).Scan(&tail); err != nil || tail == 0 {
		return err
	}
	return t.trashRows(tb, `SELECT id FROM `+tb.sql()+after)
}

// columnEmpty : aucune ligne du tableau n'a de valeur (ni saisie invalide)
// dans la colonne c.
func (t *tx) columnEmpty(tb Table, c Column) (bool, error) {
	var n int
	q := fmt.Sprintf(`SELECT count(*) FROM %s WHERE coalesce(%s, '') <> '' OR EXISTS (SELECT 1 FROM _lattis_invalid i WHERE i.tbl = %d AND i.rid = %s.id AND i.col = %d)`, tb.sql(), colSQL(c.ID), tb.ID, tb.sql(), c.ID)
	err := t.row(q).Scan(&n)
	return n == 0, err
}
