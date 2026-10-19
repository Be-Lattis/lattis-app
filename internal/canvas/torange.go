package canvas

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/formula"
)

// Convertir en plage, comme dans Excel.
//
// Le tableau disparaît ; ses cellules deviennent des cellules libres au même
// endroit (l'en-tête en première ligne, puis les lignes de données dans
// l'ordre d'affichage, lignes masquées par un filtre comprises), avec leur
// mise en forme (colonne, en-tête et cellule). Une colonne calculée donne sa
// formule traduite en adresses de cellules ([@Prix]*[@Qté] → Cell[B3]*Cell[C3])
// quand la traduction est simple et redonne la même valeur, sinon ses
// valeurs. Les formules du classeur qui citaient le tableau sont réécrites
// de la même façon quand c'est possible (Ventes[Prix] dans un agrégat →
// Cell[B2:B9]) ; les autres gardent leur texte et donnent #REF!, comme
// toute formule qui cite un tableau absent.
//
// La zone reste marquée « plage » (_lattis_range_cells) : ses cellules ne
// font plus naître de tableau et ne rejoignent pas un tableau voisin ; une
// saisie juste à côté l'agrandit. « Mettre sous forme de tableau »
// (FormatAsTable) en refait un tableau et l'oublie.

// MaxRangeCells borne la taille d'un tableau converti en plage (les cellules
// libres d'une feuille sont toutes chargées à l'affichage).
const MaxRangeCells = 100_000

// rangeTable est la table des positions marquées « plage ».
const rangeTable = "_lattis_range_cells"

// ConvertToRange convertit un tableau en plage de cellules libres, en une
// seule étape d'annulation.
func (m *Model) ConvertToRange(ctx context.Context, name string) error {
	return m.Do(ctx, "Convertir en plage", func(t *tx) error {
		tb, err := t.editTable(name)
		if err != nil {
			return err
		}
		return t.toRange(tb)
	})
}

// rangeCell est une cellule produite par la conversion.
type rangeCell struct {
	row, col int
	raw      string
	style    string
}

func (t *tx) toRange(tb Table) error {
	if n := (tb.Rows + 1) * len(tb.Cols); n > MaxRangeCells {
		return fmt.Errorf("%w : « %s » est trop grand pour être converti en plage (%d cellules, %d au plus)", ErrTable, tb.Name, n, MaxRangeCells)
	}
	var sheetName string
	if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, tb.Sheet).Scan(&sheetName); err != nil {
		return err
	}
	cols := map[string]int{}
	for i, c := range tb.Cols {
		cols[strings.ToLower(c.Name)] = tb.Col + i
	}
	place := formula.TableCells{Table: tb.Name, Row: tb.Row, Rows: tb.Rows, Cols: cols}

	// 1. Ce que le tableau affiche : textes des colonnes stockées (ou saisies
	// invalides), valeurs des colonnes calculées, mise en forme.
	cells, computed, err := t.rangeContent(tb)
	if err != nil {
		return err
	}

	// 2. Formules du classeur qui citent le tableau, traduites en adresses
	// de cellules quand c'est possible ; valeur actuelle des cellules libres.
	outside, err := t.citing(tb, sheetName, place)
	if err != nil {
		return err
	}
	var before map[string]formula.Value
	for _, f := range outside {
		if f.tbl != 0 || f.neu == "" || before != nil {
			continue
		}
		schema, err := t.schema()
		if err != nil {
			return err
		}
		if before, err = t.nameValuesWith(schema, true); err != nil {
			return err
		}
	}

	// 3. Les cellules libres (valeurs), puis les formules : colonnes
	// calculées traduites et formules libres réécrites, gardées seulement si
	// elles redonnent les mêmes valeurs.
	if err := t.putRange(tb, cells); err != nil {
		return err
	}
	var trial []trialFormula
	for i, c := range tb.Cols {
		if c.Formula == "" {
			continue
		}
		raws, ok := columnCells(tb, i, place)
		if !ok {
			continue
		}
		for k, raw := range raws {
			trial = append(trial, trialFormula{group: i, sheet: tb.Sheet, row: tb.Row + 1 + k, col: tb.Col + i, raw: raw, old: valueRaw(computed[i][k]), want: computed[i][k]})
		}
	}
	for k, f := range outside {
		if f.tbl == 0 && f.neu != "" {
			trial = append(trial, trialFormula{group: -1 - k, sheet: f.sheet, row: f.row, col: f.col, raw: f.neu, old: f.text,
				want: before[formula.CellKey(f.sheet, f.row, f.col)]})
		}
	}
	good, err := t.verify(trial)
	if err != nil {
		return err
	}
	var do, undo []stmt
	del := `DELETE FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`
	ins := `INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (?, ?, ?, ?)`
	kept, rewritten, broken := 0, 0, 0
	for _, f := range trial {
		if !good[f.group] {
			continue
		}
		do = append(do, st(del, f.sheet, f.row, f.col), st(ins, f.sheet, f.row, f.col, f.raw))
		undo = append(undo, st(del, f.sheet, f.row, f.col))
		if f.old != "" {
			undo = append(undo, st(ins, f.sheet, f.row, f.col, f.old))
		}
	}
	for g := range good {
		if g >= 0 {
			kept++
		}
	}
	for k, f := range outside {
		switch {
		case f.tbl != 0 && f.neu != "":
			qf := `UPDATE _lattis_columns SET formula = ? WHERE tbl = ? AND id = ?`
			do = append(do, st(qf, f.neu, f.tbl, f.cid))
			undo = append(undo, st(qf, f.text, f.tbl, f.cid))
			rewritten++
		case f.tbl == 0 && good[-1-k]:
			rewritten++
		default:
			broken++
		}
	}
	if len(do) > 0 {
		if err := t.apply(do, undo); err != nil {
			return err
		}
	}

	// 4. Le tableau part (une requête d'import qui le remplissait n'a plus
	// de destination) ; sa zone reste une plage.
	rs, err := t.query(`SELECT id FROM _lattis_queries WHERE target = ?`, tb.ID)
	if err != nil {
		return err
	}
	var queries []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return err
		}
		queries = append(queries, id)
	}
	rs.Close()
	for _, id := range queries {
		q := `UPDATE _lattis_queries SET target = ? WHERE id = ?`
		if err := t.apply([]stmt{st(q, nil, id)}, []stmt{st(q, tb.ID, id)}); err != nil {
			return err
		}
	}
	if err := t.deleteTable(tb); err != nil {
		return err
	}
	if err := t.markRange(tb.Sheet, tb.Row, tb.Col, tb.Bottom(), tb.Right()); err != nil {
		return err
	}

	note := fmt.Sprintf("Tableau « %s » converti en plage", tb.Name)
	if kept > 0 {
		note += fmt.Sprintf(", %s", plural(kept, "colonne calculée gardée en formules", fmt.Sprintf("%d colonnes calculées gardées en formules", kept)))
	}
	if n := countComputed(tb) - kept; n > 0 {
		note += fmt.Sprintf(", %s", plural(n, "colonne calculée remplacée par ses valeurs", fmt.Sprintf("%d colonnes calculées remplacées par leurs valeurs", n)))
	}
	if rewritten > 0 {
		note += fmt.Sprintf(" ; %s", plural(rewritten, "1 formule réécrite en adresses de cellules", fmt.Sprintf("%d formules réécrites en adresses de cellules", rewritten)))
	}
	if broken > 0 {
		note += fmt.Sprintf(" ; %s", plural(broken, "1 formule cite encore le tableau (#REF!)", fmt.Sprintf("%d formules citent encore le tableau (#REF!)", broken)))
	}
	t.notes = append(t.notes, note+" (Ctrl+Z pour annuler)")
	return nil
}

func countComputed(tb Table) int {
	n := 0
	for _, c := range tb.Cols {
		if c.Formula != "" {
			n++
		}
	}
	return n
}

// rangeContent lit les cellules du tableau telles qu'elles deviendront
// libres ; computed[i] donne, pour une colonne calculée, la valeur de chaque
// ligne.
func (t *tx) rangeContent(tb Table) ([]rangeCell, map[int][]formula.Value, error) {
	var out []rangeCell
	enc := func(s Style) string {
		for k, v := range s {
			if v == nil {
				delete(s, k)
			}
		}
		if len(s) == 0 {
			return ""
		}
		o, _ := encodeStyle(s)
		return o
	}
	colStyles := make([]Style, len(tb.Cols))
	for i, c := range tb.Cols {
		cs, err := t.tableStyle(tb.ID, 0, c.ID)
		if err != nil {
			return nil, nil, err
		}
		hs, err := t.tableStyle(tb.ID, -1, c.ID)
		if err != nil {
			return nil, nil, err
		}
		colStyles[i] = cs
		out = append(out, rangeCell{row: tb.Row, col: tb.Col + i, raw: textRaw(c.Name), style: enc(over(hs))})
	}
	exprs, args, _, err := t.columnSQL(tb)
	if err != nil {
		return nil, nil, err
	}
	sel := []string{"id"}
	for i, c := range tb.Cols {
		if c.Formula != "" {
			sel = append(sel, exprs[i])
			continue
		}
		sel = append(sel, fmt.Sprintf(`coalesce(slt_text(%s, '%s'), (SELECT raw FROM _lattis_invalid i WHERE i.tbl = %d AND i.rid = %s.id AND i.col = %d), '')`,
			tb.sql()+"."+colSQL(c.ID), string(c.Type), tb.ID, tb.sql(), c.ID))
	}
	rs, err := t.query(`SELECT `+strings.Join(sel, ", ")+` FROM `+tb.sql()+` ORDER BY _pos`, args...)
	if err != nil {
		return nil, nil, err
	}
	type line struct {
		rid  int64
		vals []any
	}
	var got []line
	for rs.Next() {
		l := line{vals: make([]any, len(tb.Cols))}
		ptrs := []any{&l.rid}
		for i := range l.vals {
			ptrs = append(ptrs, &l.vals[i])
		}
		if err := rs.Scan(ptrs...); err != nil {
			rs.Close()
			return nil, nil, err
		}
		got = append(got, l)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, nil, err
	}
	computed := map[int][]formula.Value{}
	for k, l := range got {
		for i, c := range tb.Cols {
			var raw string
			if c.Formula != "" {
				v := formula.FromResult(normalize([]any{l.vals[i]})[0])
				computed[i] = append(computed[i], v)
				raw = valueRaw(v)
			} else if s, ok := normalize([]any{l.vals[i]})[0].(string); ok {
				if c.Type == "text" {
					raw = textRaw(s)
				} else {
					raw = freeRaw(s)
				}
			}
			cs, err := t.tableStyle(tb.ID, l.rid, c.ID)
			if err != nil {
				return nil, nil, err
			}
			style := enc(over(colStyles[i], cs))
			if raw == "" && style == "" {
				continue
			}
			out = append(out, rangeCell{row: tb.Row + 1 + k, col: tb.Col + i, raw: raw, style: style})
		}
	}
	return out, computed, nil
}

// textRaw écrit un texte en saisie de cellule libre qui l'affiche tel quel :
// « 007 », « 12:30 » ou « =x » prennent une apostrophe (sinon un nombre, une
// heure, une formule).
func textRaw(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	if lit, ok := valueLiteral(s).(formula.StrLit); ok && lit.V == s && !isFormula(s) {
		return s
	}
	return "'" + s
}

// valueRaw écrit une valeur calculée en saisie de cellule libre.
func valueRaw(v formula.Value) string {
	switch v.Kind {
	case formula.Text:
		return textRaw(v.Str)
	case formula.Empty:
		return ""
	}
	return v.String()
}

// putRange pose les cellules converties (zone du tableau : aucune cellule
// libre ne s'y trouve, sauf reliquat qui est remplacé).
func (t *tx) putRange(tb Table, cells []rangeCell) error {
	in := `sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`
	args := []any{tb.Sheet, tb.Row, tb.Bottom(), tb.Col, tb.Right()}
	if err := t.deleteWhere(`_lattis_cells`, in, args...); err != nil {
		return err
	}
	var do []stmt
	const chunk = 200
	for i := 0; i < len(cells); i += chunk {
		var vals []string
		var a []any
		for _, c := range cells[i:min(i+chunk, len(cells))] {
			if c.raw == "" {
				continue
			}
			vals = append(vals, "(?, ?, ?, ?)")
			a = append(a, tb.Sheet, c.row, c.col, c.raw)
		}
		if len(vals) > 0 {
			do = append(do, st(`INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES `+strings.Join(vals, ", "), a...))
		}
	}
	if len(do) > 0 {
		if err := t.apply(do, []stmt{st(`DELETE FROM _lattis_cells WHERE `+in, args...)}); err != nil {
			return err
		}
	}
	for _, c := range cells {
		if c.style == "" {
			continue
		}
		if err := t.putFreeStyle(tb.Sheet, c.row, c.col, c.style); err != nil {
			return err
		}
	}
	return nil
}

// columnCells traduit la formule de la colonne calculée i en une formule de
// cellule libre par ligne ; faux si elle ne se traduit pas simplement.
func columnCells(tb Table, i int, place formula.TableCells) ([]string, bool) {
	n, err := formula.Parse(tb.Cols[i].Formula, formula.EN)
	if err != nil || tb.Rows == 0 {
		return nil, false
	}
	raws := make([]string, tb.Rows)
	place.Self = true
	for k := range raws {
		place.At = tb.Row + 1 + k
		x, ok := formula.ToCells(n, place)
		if !ok {
			return nil, false
		}
		raws[k] = formula.Format(x, formula.FR)
	}
	return raws, true
}

// trialFormula est une formule de cellule libre à essayer : elle n'est
// gardée que si elle donne want. Les formules d'un même groupe (une colonne
// calculée) sont gardées ou écartées ensemble.
type trialFormula struct {
	group    int
	sheet    int64
	row, col int
	raw      string // formule à essayer
	old      string // saisie actuelle de la cellule
	want     formula.Value
}

// verify essaie les formules (dans un point de sauvegarde : rien ne reste)
// et rend les groupes qui redonnent leurs valeurs ; un groupe écarté peut en
// fausser un autre, d'où les essais répétés.
func (t *tx) verify(trial []trialFormula) (map[int]bool, error) {
	good := map[int]bool{}
	for _, f := range trial {
		good[f.group] = true
	}
	for len(good) > 0 {
		bad, err := t.tryFormulas(trial, good)
		if err != nil {
			return nil, err
		}
		if len(bad) == 0 {
			break
		}
		for g := range bad {
			delete(good, g)
		}
	}
	return good, nil
}

func (t *tx) tryFormulas(trial []trialFormula, good map[int]bool) (map[int]bool, error) {
	if _, err := t.Tx.Tx.ExecContext(t.Ctx, `SAVEPOINT lattis_torange`); err != nil {
		return nil, err
	}
	defer func() {
		t.Tx.Tx.ExecContext(t.Ctx, `ROLLBACK TO lattis_torange`)
		t.Tx.Tx.ExecContext(t.Ctx, `RELEASE lattis_torange`)
	}()
	ins := `INSERT OR REPLACE INTO _lattis_cells (sheet, row, col, raw) VALUES (?, ?, ?, ?)`
	for _, f := range trial {
		if !good[f.group] {
			continue
		}
		if _, err := t.Tx.Tx.ExecContext(t.Ctx, ins, f.sheet, f.row, f.col, f.raw); err != nil {
			return nil, err
		}
	}
	schema, err := t.schema()
	if err != nil {
		return nil, err
	}
	values, err := t.nameValuesWith(schema, true)
	if err != nil {
		return nil, err
	}
	bad := map[int]bool{}
	for _, f := range trial {
		if good[f.group] && !sameValue(values[formula.CellKey(f.sheet, f.row, f.col)], f.want) {
			bad[f.group] = true
		}
	}
	return bad, nil
}

// sameValue compare deux résultats comme la grille les affiche.
func sameValue(a, b formula.Value) bool {
	if a.Kind == formula.Empty && b.Kind == formula.Text && b.Str == "" || b.Kind == formula.Empty && a.Kind == formula.Text && a.Str == "" {
		return true
	}
	return a.String() == b.String()
}

// citingFormula est une formule du classeur qui cite le tableau converti.
type citingFormula struct {
	sheet    int64 // cellule libre : sa feuille ; colonne calculée : 0
	row, col int
	tbl, cid int64 // colonne calculée d'un autre tableau
	text     string
	neu      string // réécrite, "" si impossible
}

// citing relève les formules qui citent le tableau et prépare leur
// réécriture en adresses de cellules.
func (t *tx) citing(tb Table, sheetName string, place formula.TableCells) ([]citingFormula, error) {
	low := strings.ToLower(tb.Name)
	var out []citingFormula
	rs, err := t.query(`SELECT sheet, row, col, raw FROM _lattis_cells WHERE raw LIKE '=%'`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var f citingFormula
		if err := rs.Scan(&f.sheet, &f.row, &f.col, &f.text); err != nil {
			rs.Close()
			return nil, err
		}
		if strings.Contains(strings.ToLower(f.text), low) {
			out = append(out, f)
		}
	}
	rs.Close()
	rs, err = t.query(`SELECT c.tbl, c.id, t.sheet, c.formula FROM _lattis_columns c JOIN _lattis_tables t ON t.id = c.tbl
		WHERE c.formula IS NOT NULL AND c.tbl <> ?`, tb.ID)
	if err != nil {
		return nil, err
	}
	var cols []citingFormula
	var homes []int64
	for rs.Next() {
		var f citingFormula
		var home int64
		if err := rs.Scan(&f.tbl, &f.cid, &home, &f.text); err != nil {
			rs.Close()
			return nil, err
		}
		if strings.Contains(strings.ToLower(f.text), low) {
			cols = append(cols, f)
			homes = append(homes, home)
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	var kept []citingFormula
	for _, f := range out {
		n, err := formula.Parse(f.text, formula.FR)
		if err != nil || !formula.Mentions(n, place) {
			continue
		}
		p := place
		if f.sheet != tb.Sheet {
			p.Sheet = sheetName
		}
		if x, ok := formula.ToCells(n, p); ok {
			f.neu = formula.Format(x, formula.FR)
		}
		kept = append(kept, f)
	}
	for k, f := range cols {
		n, err := formula.Parse(f.text, formula.EN)
		if err != nil || !formula.Mentions(n, place) {
			continue
		}
		p := place
		if homes[k] != tb.Sheet {
			p.Sheet = sheetName
		}
		if x, ok := formula.ToCells(n, p); ok {
			f.neu = formula.Format(x, formula.EN)
		}
		kept = append(kept, f)
	}
	return kept, nil
}

// --- Zones « plage » ---------------------------------------------------------

// markRange marque « plage » les positions du rectangle qui ne le sont pas.
func (t *tx) markRange(sheet int64, r0, c0, r1, c1 int) error {
	marks, err := t.rangeMarks(sheet)
	if err != nil {
		return err
	}
	var ps [][2]int
	for r := r0; r <= r1; r++ {
		for c := c0; c <= c1; c++ {
			if !marks[[2]int{r, c}] {
				ps = append(ps, [2]int{r, c})
			}
		}
	}
	return t.mark(sheet, ps)
}

// mark marque « plage » des positions (non marquées).
func (t *tx) mark(sheet int64, ps [][2]int) error {
	if len(ps) == 0 {
		return nil
	}
	var do, undo []stmt
	const chunk = 300
	for i := 0; i < len(ps); i += chunk {
		var vals []string
		var a []any
		for _, p := range ps[i:min(i+chunk, len(ps))] {
			vals = append(vals, "(?, ?, ?)")
			a = append(a, sheet, p[0], p[1])
		}
		do = append(do, st(`INSERT INTO `+rangeTable+` (sheet, row, col) VALUES `+strings.Join(vals, ", "), a...))
	}
	for _, p := range ps {
		undo = append(undo, st(`DELETE FROM `+rangeTable+` WHERE sheet = ? AND row = ? AND col = ?`, sheet, p[0], p[1]))
	}
	return t.apply(do, undo)
}

// rangeMarks donne les positions marquées « plage » de la feuille.
func (t *tx) rangeMarks(sheet int64) (map[[2]int]bool, error) {
	rs, err := t.query(`SELECT row, col FROM `+rangeTable+` WHERE sheet = ?`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[[2]int]bool{}
	for rs.Next() {
		var r, c int
		if err := rs.Scan(&r, &c); err != nil {
			return nil, err
		}
		out[[2]int{r, c}] = true
	}
	return out, rs.Err()
}

// inRange : la position est marquée « plage ».
func (t *tx) inRange(sheet int64, row, col int) (bool, error) {
	var k int
	err := t.row(`SELECT count(*) FROM `+rangeTable+` WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&k)
	return k > 0, err
}

// nearRange : une position marquée « plage » est dans le rectangle ou le
// borde.
func (t *tx) nearRange(sheet int64, r0, c0, r1, c1 int) (bool, error) {
	var k int
	err := t.row(`SELECT count(*) FROM `+rangeTable+` WHERE sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`,
		sheet, r0-1, r1+1, c0-1, c1+1).Scan(&k)
	return k > 0, err
}

// keepRange : un îlot de cellules libres qui contient ou borde une position
// marquée « plage » reste une plage (pas de tableau) et la zone s'étend à
// tout l'îlot. Vrai si c'est le cas.
func (t *tx) keepRange(sheet int64, isl []cell) (bool, error) {
	marks, err := t.rangeMarks(sheet)
	if err != nil || len(marks) == 0 {
		return false, err
	}
	near := false
	for _, c := range isl {
		for _, d := range [][2]int{{0, 0}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			if marks[[2]int{c.Row + d[0], c.Col + d[1]}] {
				near = true
			}
		}
	}
	if !near {
		return false, nil
	}
	var ps [][2]int
	for _, c := range isl {
		if p := [2]int{c.Row, c.Col}; !marks[p] {
			ps = append(ps, p)
		}
	}
	return true, t.mark(sheet, ps)
}

// rangeRects regroupe les positions marquées en rectangles (pour
// l'affichage) : suites horizontales, puis suites identiques superposées.
func rangeRects(marks map[[2]int]bool) []Rect {
	ps := make([][2]int, 0, len(marks))
	for p := range marks {
		ps = append(ps, p)
	}
	slices.SortFunc(ps, func(a, b [2]int) int {
		if a[0] != b[0] {
			return a[0] - b[0]
		}
		return a[1] - b[1]
	})
	var runs []Rect
	for _, p := range ps {
		if n := len(runs); n > 0 && runs[n-1].R0 == p[0] && runs[n-1].C1 == p[1]-1 {
			runs[n-1].C1 = p[1]
			continue
		}
		runs = append(runs, Rect{p[0], p[1], p[0], p[1]})
	}
	var out []Rect
	open := map[[2]int]int{} // (c0, c1) → rectangle encore extensible
	for _, r := range runs {
		k := [2]int{r.C0, r.C1}
		if i, ok := open[k]; ok && out[i].R1 == r.R0-1 {
			out[i].R1 = r.R0
			continue
		}
		open[k] = len(out)
		out = append(out, r)
	}
	return out
}

// --- Mettre sous forme de tableau --------------------------------------------

// ErrNotRange signale « Mettre sous forme de tableau » hors d'une plage.
var ErrNotRange = errors.New("cette cellule n'est pas dans une plage convertie")

// FormatAsTable refait un tableau de la plage qui contient la cellule (row,
// col) : la zone n'est plus marquée « plage » ; sa première ligne donne les
// en-têtes, les lignes suivantes les données (une formule donne sa valeur).
// Renvoie le nom du tableau.
func (m *Model) FormatAsTable(ctx context.Context, sheet int64, row, col int) (string, error) {
	var name string
	err := m.Do(ctx, "Mettre sous forme de tableau", func(t *tx) (err error) {
		if err := t.guard(sheet); err != nil {
			return err
		}
		// Hors d'une plage : l'îlot de cellules libres qui contient la cellule
		// (une liste d'une colonne, un bloc resté libre) devient d'abord une plage.
		if in, err := t.inRange(sheet, row, col); err != nil {
			return err
		} else if !in {
			cells, err := t.cells(sheet)
			if err != nil {
				return err
			}
			isl := island(cells, row, col)
			if len(isl) < 2 {
				return fmt.Errorf("%w : il faut au moins deux cellules remplies, l'une sous l'autre ou côte à côte", ErrTable)
			}
			r0, c0, r1, c1 := isl[0].Row, isl[0].Col, isl[0].Row, isl[0].Col
			for _, c := range isl {
				r0, c0, r1, c1 = min(r0, c.Row), min(c0, c.Col), max(r1, c.Row), max(c1, c.Col)
			}
			if err := t.markRange(sheet, r0, c0, r1, c1); err != nil {
				return err
			}
		}
		if name, err = t.formatAsTable(sheet, row, col); err != nil {
			return err
		}
		// Première colonne ID, ou 1, 2, 3… : la clé (autokey.go).
		tb, err := t.tableByName(name)
		if err != nil {
			return err
		}
		return t.autoKey(tb.ID)
	})
	return name, err
}

func (t *tx) formatAsTable(sheet int64, row, col int) (string, error) {
	marks, err := t.rangeMarks(sheet)
	if err != nil {
		return "", err
	}
	if !marks[[2]int{row, col}] {
		return "", fmt.Errorf("%w : %w", ErrTable, ErrNotRange)
	}
	cells, err := t.cells(sheet)
	if err != nil {
		return "", err
	}
	at := map[[2]int]cell{}
	for _, c := range cells {
		at[[2]int{c.Row, c.Col}] = c
	}
	// La plage : positions marquées et cellules remplies reliées à la cellule.
	seen := map[[2]int]bool{}
	stack := [][2]int{{row, col}}
	var zone [][2]int
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[p] {
			continue
		}
		if _, filled := at[p]; !filled && !marks[p] {
			continue
		}
		seen[p] = true
		zone = append(zone, p)
		stack = append(stack, [2]int{p[0] + 1, p[1]}, [2]int{p[0] - 1, p[1]}, [2]int{p[0], p[1] + 1}, [2]int{p[0], p[1] - 1})
	}
	var undo, do []stmt
	for _, p := range zone {
		if marks[p] {
			do = append(do, st(`DELETE FROM `+rangeTable+` WHERE sheet = ? AND row = ? AND col = ?`, sheet, p[0], p[1]))
			undo = append(undo, st(`INSERT INTO `+rangeTable+` (sheet, row, col) VALUES (?, ?, ?)`, sheet, p[0], p[1]))
		}
	}
	if err := t.apply(do, undo); err != nil {
		return "", err
	}
	// Rectangle des cellules remplies.
	r0, c0, r1, c1, found := 0, 0, 0, 0, false
	for _, p := range zone {
		if _, filled := at[p]; !filled {
			continue
		}
		if !found {
			r0, c0, r1, c1, found = p[0], p[1], p[0], p[1], true
			continue
		}
		r0, c0, r1, c1 = min(r0, p[0]), min(c0, p[1]), max(r1, p[0]), max(c1, p[1])
	}
	if !found {
		t.notes = append(t.notes, "Plage vide : elle n'est plus marquée comme plage")
		return "", nil
	}
	tables, err := t.tables(sheet)
	if err != nil {
		return "", err
	}
	for _, tb := range tables {
		if tb.Row <= r1 && tb.Bottom() >= r0 && tb.Col <= c1 && tb.Right() >= c0 {
			return "", fmt.Errorf("%w : la plage recouvre le tableau « %s »", ErrTable, tb.Name)
		}
	}
	text := func(r, c int) string {
		x, ok := at[[2]int{r, c}]
		if !ok {
			return ""
		}
		if isFormula(x.Raw) {
			return valueRaw(t.evalFreeRaw(sheet, x.Raw))
		}
		return x.Raw
	}
	headers := make([]string, c1-c0+1)
	taken := map[string]bool{}
	for i := range headers {
		h := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text(r0, c0+i)), "'"))
		if h == "" {
			h = fmt.Sprintf("Colonne %d", i+1)
		}
		base := h
		for n := 2; taken[strings.ToLower(h)]; n++ {
			h = fmt.Sprintf("%s %d", base, n)
		}
		taken[strings.ToLower(h)] = true
		headers[i] = h
	}
	// Colonnes dont chaque ligne a la même formule, à la ligne près
	// (Cell[B2]*Cell[C2], Cell[B3]*Cell[C3]…) : de nouveau calculées.
	var sheetName string
	if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&sheetName); err != nil {
		return "", err
	}
	area := Rect{r0, c0, r1, c1}
	computed := map[int]string{}
	for i := range headers {
		if f, ok := rowFormula(area, headers, sheetName, func(r int) string { return at[[2]int{r, c0 + i}].Raw }); ok {
			computed[i] = f
		}
	}
	data := make([][]string, r1-r0)
	for k := range data {
		data[k] = make([]string, len(headers))
		for i := range headers {
			data[k][i] = text(r0+1+k, c0+i)
		}
	}
	// Mise en forme des cellules : elle passe au tableau.
	type styled struct {
		r, c  int
		style string
	}
	var styles []styled
	rs, err := t.query(`SELECT row, col, style FROM _lattis_cell_styles WHERE sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`, sheet, r0, r1, c0, c1)
	if err != nil {
		return "", err
	}
	for rs.Next() {
		var s styled
		if err := rs.Scan(&s.r, &s.c, &s.style); err != nil {
			rs.Close()
			return "", err
		}
		styles = append(styles, s)
	}
	rs.Close()
	in := `sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`
	for _, tbl := range []string{"_lattis_cells", "_lattis_cell_styles"} {
		if err := t.deleteWhere(tbl, in, sheet, r0, r1, c0, c1); err != nil {
			return "", err
		}
	}
	name, err := t.tableName(sheet, r0, c0)
	if err != nil {
		return "", err
	}
	tb, err := t.createTable(sheet, name, r0, c0, headers, data)
	if err != nil {
		return "", err
	}
	for i, f := range computed {
		if err := t.setFormula(tb, i, f); err != nil {
			return "", err
		}
	}
	// Les formules qui visaient les cellules de la plage visent le tableau.
	if err := t.cellsToTable(sheet, sheetName, area, name, headers); err != nil {
		return "", err
	}
	for _, s := range styles {
		rid := int64(s.r - r0) // 0 → en-tête (-1), k → ligne k
		if rid == 0 {
			rid = -1
		}
		if err := t.putTableStyle(tb.ID, rid, tb.Cols[s.c-c0].ID, s.style); err != nil {
			return "", err
		}
	}
	t.notes = append(t.notes, fmt.Sprintf("Plage mise sous forme de tableau « %s » (Ctrl+Z pour annuler)", name))
	// Sous un onglet à choix réglé sur une valeur, le tableau la suit (§6.5).
	return name, t.followNewTable(tb.ID)
}

// moveMarks déplace les positions « plage » retenues par match, avec les
// cellules libres qu'un tableau qui grandit écarte (voir moveCells).
func (t *tx) moveMarks(sheet int64, match func(cell) bool, dRow, dCol int) error {
	marks, err := t.rangeMarks(sheet)
	if err != nil || len(marks) == 0 {
		return err
	}
	var moved [][2]int
	for p := range marks {
		if match(cell{Row: p[0], Col: p[1]}) {
			moved = append(moved, p)
		}
	}
	if len(moved) == 0 {
		return nil
	}
	del := `DELETE FROM ` + rangeTable + ` WHERE sheet = ? AND row = ? AND col = ?`
	ins := `INSERT OR IGNORE INTO ` + rangeTable + ` (sheet, row, col) VALUES (?, ?, ?)`
	var do, undo []stmt
	for _, p := range moved {
		do = append(do, st(del, sheet, p[0], p[1]))
		undo = append(undo, st(del, sheet, p[0]+dRow, p[1]+dCol))
	}
	for _, p := range moved {
		do = append(do, st(ins, sheet, p[0]+dRow, p[1]+dCol))
		undo = append(undo, st(ins, sheet, p[0], p[1]))
	}
	// Une position d'arrivée déjà marquée (et non déplacée) le reste.
	for _, p := range moved {
		q := [2]int{p[0] + dRow, p[1] + dCol}
		if marks[q] && !match(cell{Row: q[0], Col: q[1]}) {
			undo = append(undo, st(ins, sheet, q[0], q[1]))
		}
	}
	return t.apply(do, undo)
}

// tableRef traduit une adresse de cellules de la zone d'un tableau (en-tête
// en zone.R0, données dessous) en référence structurée : Tableau[Col] (toute
// la colonne), Tableau[Col][1:3], Tableau[Col][2] ; faux hors des données
// ou sur plusieurs colonnes.
func tableRef(zone Rect, table string, headers []string, addr string) (string, bool) {
	a, b, isRange := strings.Cut(addr, ":")
	if !isRange {
		b = a
	}
	ra, ca, ok1 := formula.CellRef("Cell_" + strings.TrimSpace(a))
	rb, cb, ok2 := formula.CellRef("Cell_" + strings.TrimSpace(b))
	if !ok1 || !ok2 {
		return "", false
	}
	ra, rb = min(ra, rb), max(ra, rb)
	if ca != cb || ca < zone.C0 || ca > zone.C1 || ra <= zone.R0 || rb > zone.R1 {
		return "", false
	}
	col := headers[ca-zone.C0]
	if strings.ContainsAny(col, "[]") {
		return "", false
	}
	ref := table + "[" + col + "]"
	from, to := ra-zone.R0, rb-zone.R0
	switch {
	case isRange && from == 1 && to == zone.R1-zone.R0:
		return ref, true
	case from == to:
		return fmt.Sprintf("%s[%d]", ref, from), true
	}
	return fmt.Sprintf("%s[%d:%d]", ref, from, to), true
}

// rowFormula : si chaque ligne de données d'une colonne de la zone a la même
// formule à la ligne près, la formule de colonne calculée correspondante
// ([@Prix]*[@Qté]) ; les cellules de la même ligne deviennent [@Col].
func rowFormula(zone Rect, headers []string, sheetName string, raw func(r int) string) (string, bool) {
	if zone.R1 <= zone.R0 {
		return "", false
	}
	var out string
	for r := zone.R0 + 1; r <= zone.R1; r++ {
		src := raw(r)
		if !isFormula(src) {
			return "", false
		}
		ok := true
		f := cellRefs(src, func(sh, addr, orig string) string {
			if sh != "" && !strings.EqualFold(sh, sheetName) {
				return orig
			}
			if strings.Contains(addr, ":") {
				if rr, c1, _ := formula.CellRef("Cell_" + strings.Split(addr, ":")[0]); rr >= zone.R0 && rr <= zone.R1 && c1 >= zone.C0 && c1 <= zone.C1 {
					ok = false
				}
				return orig
			}
			rr, cc, valid := formula.CellRef("Cell_" + addr)
			if !valid || cc < zone.C0 || cc > zone.C1 || rr < zone.R0 || rr > zone.R1 {
				return orig
			}
			if rr != r || strings.ContainsAny(headers[cc-zone.C0], "[]") {
				ok = false
				return orig
			}
			return "[@" + headers[cc-zone.C0] + "]"
		})
		if !ok || out != "" && f != out {
			return "", false
		}
		out = f
	}
	return out, true
}

// cellsToTable réécrit les formules du classeur qui visaient les données de
// la zone (feuille sheet), devenue le tableau table : Cell[D2:D9] →
// Tableau[Montant], Cell[B3] → Tableau[Prix][2].
func (t *tx) cellsToTable(sheet int64, sheetName string, zone Rect, table string, headers []string) error {
	rewrite := func(text string, home int64) string {
		return cellRefs(text, func(sh, addr, orig string) string {
			if sh != "" && !strings.EqualFold(sh, sheetName) || sh == "" && home != sheet {
				return orig
			}
			if ref, ok := tableRef(zone, table, headers, addr); ok {
				return ref
			}
			return orig
		})
	}
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
	if len(do) == 0 {
		return nil
	}
	return t.apply(do, undo)
}

// PasteRange écrit un bloc en cellules libres, sans faire de tableau : la
// zone est marquée « plage » puis le bloc collé, en une seule étape
// d'annulation.
func (m *Model) PasteRange(ctx context.Context, sheet int64, row, col int, block [][]string) (Event, error) {
	h, w := len(block), 0
	for _, r := range block {
		w = max(w, len(r))
	}
	if h == 0 || w == 0 {
		return Event{}, nil
	}
	if !command.HasGroup(ctx) {
		ctx = command.WithGroup(ctx, "Coller en plage")
	}
	err := m.Do(ctx, "Coller en plage", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			if tb.Row <= row+h-1 && tb.Bottom() >= row && tb.Col <= col+w-1 && tb.Right() >= col {
				return fmt.Errorf("%w : la zone recouvre le tableau « %s »", ErrTable, tb.Name)
			}
		}
		return t.markRange(sheet, row, col, row+h-1, col+w-1)
	})
	if err != nil {
		return Event{}, err
	}
	return m.Paste(ctx, sheet, row, col, block)
}

// splitAt coupe le tableau tb en deux autour de ses colonnes i à j, vides :
// à gauche et à droite deux tableaux, les colonnes redeviennent des colonnes
// vides de la feuille. Les formules qui citaient tb citent le tableau de
// leur colonne (passage par une plage) ; le plus large des deux (à égalité,
// celui de gauche) garde le nom de tb. Un côté dont l'en-tête n'est fait que
// de « Colonne n » (un tableau absorbé) prend sa 1re ligne pour en-tête.
func (t *tx) splitAt(tb Table, i, j int) error {
	notes := len(t.notes)
	if err := t.toRange(tb); err != nil {
		return err
	}
	in := `sheet = ? AND col BETWEEN ? AND ? AND row BETWEEN ? AND ?`
	for _, tbl := range []string{"_lattis_cells", rangeTable} {
		if err := t.deleteWhere(tbl, in, tb.Sheet, tb.Col+i, tb.Col+j, tb.Row, tb.Bottom()); err != nil {
			return err
		}
	}
	for _, side := range [][2]int{{0, i - 1}, {j + 1, len(tb.Cols) - 1}} {
		auto := true
		for k := side[0]; k <= side[1]; k++ {
			auto = auto && autoColRe.MatchString(tb.Cols[k].Name)
		}
		if auto && tb.Rows > 0 {
			if err := t.deleteWhere(`_lattis_cells`, `sheet = ? AND row = ? AND col BETWEEN ? AND ?`, tb.Sheet, tb.Row, tb.Col+side[0], tb.Col+side[1]); err != nil {
				return err
			}
		}
	}
	var names []string
	for _, c := range []int{tb.Col, tb.Col + j + 1} {
		name, err := t.formatAsTable(tb.Sheet, tb.Row, c)
		if err != nil {
			return err
		}
		names = append(names, name)
	}
	// Seule la note de la coupe reste (pas celles de la plage).
	t.notes = t.notes[:notes]
	keep := 0
	if len(tb.Cols)-1-j > i {
		keep = 1
	}
	if err := t.keepName(names, keep, tb.Name); err != nil {
		return err
	}
	what := "à la colonne vide"
	if j > i {
		what = "aux colonnes vides"
	}
	t.notes = append(t.notes, fmt.Sprintf("Tableau « %s » coupé en deux %s : « %s » et « %s » (Ctrl+Z pour annuler)", tb.Name, what, names[0], names[1]))
	return nil
}

// keepName donne le nom old au tableau names[keep], qui remplace un tableau
// disparu (coupe, ligne d'en-tête posée) ; un autre de names qui l'aurait
// pris d'office est renommé. names est mis à jour.
func (t *tx) keepName(names []string, keep int, old string) error {
	if names[keep] == old {
		return nil
	}
	retitle := func(from, to string) error {
		var id int64
		if err := t.row(`SELECT id FROM _lattis_tables WHERE name = ?`, from).Scan(&id); err != nil {
			return err
		}
		if err := t.rewriteRefs(tableChange(from, to)); err != nil {
			return err
		}
		q := `UPDATE _lattis_tables SET name = ? WHERE id = ?`
		return t.apply([]stmt{st(q, to, id)}, []stmt{st(q, from, id)})
	}
	for k, n := range names {
		if k == keep || !strings.EqualFold(n, old) {
			continue
		}
		fresh, err := t.freshName()
		if err != nil {
			return err
		}
		if err := retitle(n, fresh); err != nil {
			return err
		}
		names[k] = fresh
	}
	var taken int
	if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE name = ? COLLATE NOCASE`, old).Scan(&taken); err != nil || taken > 0 {
		return err
	}
	if err := retitle(names[keep], old); err != nil {
		return err
	}
	names[keep] = old
	return nil
}

// freshName : premier nom « TableauN » libre.
func (t *tx) freshName() (string, error) {
	for n := 1; ; n++ {
		name := fmt.Sprintf("Tableau%d", n)
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE name = ?`, name).Scan(&k); err != nil || k == 0 {
			return name, err
		}
	}
}
