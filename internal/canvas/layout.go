package canvas

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/store"
	"github.com/lalamefine/lattis/internal/work"
)

func tableSQL(id int64) string { return fmt.Sprintf("t_%d", id) }
func colSQL(id int64) string   { return fmt.Sprintf("c_%d", id) }

// Column décrit une colonne de tableau, dans l'ordre d'affichage.
type Column struct {
	ID      int64
	Name    string
	Type    grid.Type
	Fixed   bool
	Formula string // colonne calculée : formule (anglais canonique), sinon vide
}

// Table décrit un tableau posé sur une feuille.
type Table struct {
	ID       int64
	Sheet    int64
	Name     string
	Row, Col int // ancre (cellule d'en-tête en haut à gauche)
	Cols     []Column
	Rows     int   // lignes de données
	Key      int64 // colonne clé (identifiant) pour REFERENCE, 0 si aucune
	// En-têtes sur deux lignes (twolevel.go) : la bande des groupes occupe
	// la ligne Row-1 et fait partie du tableau.
	TwoLevel bool
	// NoFollow : le tableau ne suit pas l'onglet à choix malgré sa colonne
	// de catégorie (« Suivre l'année » décoché, choicecells.go).
	NoFollow bool

	// Filtre : les lignes masquées gardent leur place sur la feuille (Bottom
	// ne change pas), les Shown premières lignes affichent les lignes gardées.
	Filter map[string]ColumnFilter
	Shown  int
	where  string
	// userWhere : condition du seul filtre de l'utilisateur (where y ajoute
	// le choix de l'onglet) ; choice : le tableau suit le choix de l'onglet
	// (il a la colonne de catégorie, voir choice.go), nil sinon.
	userWhere string
	choice    *tableChoice
}

// Top est la première ligne occupée : la bande des groupes, sinon l'en-tête.
func (t Table) Top() int {
	if t.TwoLevel {
		return t.Row - 1
	}
	return t.Row
}

// Bottom est la dernière ligne occupée (l'en-tête si le tableau est vide).
func (t Table) Bottom() int { return t.Row + t.Rows }

// Right est la dernière colonne occupée.
func (t Table) Right() int { return t.Col + len(t.Cols) - 1 }

func (t Table) sql() string { return tableSQL(t.ID) }

// spans indique si les colonnes [lo, hi] recoupent celles du tableau.
func (t Table) spansCols(lo, hi int) bool { return t.Col <= hi && t.Right() >= lo }
func (t Table) spansRows(lo, hi int) bool { return t.Top() <= hi && t.Bottom() >= lo }

func (t *tx) tables(sheet int64) ([]Table, error) {
	rs, err := t.query(`SELECT id, name, row, col, coalesce(key_col, 0), two_level, coalesce(follow, 1) = 0 FROM _lattis_tables WHERE sheet = ? ORDER BY id`, sheet)
	if err != nil {
		return nil, err
	}
	var out []Table
	for rs.Next() {
		tb := Table{Sheet: sheet}
		if err := rs.Scan(&tb.ID, &tb.Name, &tb.Row, &tb.Col, &tb.Key, &tb.TwoLevel, &tb.NoFollow); err != nil {
			rs.Close()
			return nil, err
		}
		out = append(out, tb)
	}
	rs.Close()
	for i := range out {
		if err := t.fill(&out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (t *tx) table(id int64) (Table, error) {
	tb := Table{ID: id}
	err := t.row(`SELECT sheet, name, row, col, coalesce(key_col, 0), two_level, coalesce(follow, 1) = 0 FROM _lattis_tables WHERE id = ?`, id).
		Scan(&tb.Sheet, &tb.Name, &tb.Row, &tb.Col, &tb.Key, &tb.TwoLevel, &tb.NoFollow)
	if err != nil {
		return tb, err
	}
	return tb, t.fill(&tb)
}

func (t *tx) tableByName(name string) (Table, error) {
	var id int64
	if err := t.row(`SELECT id FROM _lattis_tables WHERE name = ?`, name).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Table{}, msg.New("table.notFound", msg.Args{"table": name})
		}
		return Table{}, msg.Wrap(err, "table.error", msg.Args{"table": fmt.Sprintf("%q", name), "err": err})
	}
	return t.table(id)
}

// fill charge les colonnes et le nombre de lignes.
func (t *tx) fill(tb *Table) error {
	rs, err := t.query(`SELECT id, name, type, fixed, coalesce(formula, '') FROM _lattis_columns WHERE tbl = ? ORDER BY pos`, tb.ID)
	if err != nil {
		return err
	}
	defer rs.Close()
	tb.Cols = nil
	for rs.Next() {
		var c Column
		if err := rs.Scan(&c.ID, &c.Name, &c.Type, &c.Fixed, &c.Formula); err != nil {
			return err
		}
		tb.Cols = append(tb.Cols, c)
	}
	if err := rs.Err(); err != nil {
		return err
	}
	// Nombre de lignes : un parcours de tout le tableau (0,2 s sur 10 M de
	// lignes), relu seulement quand des lignes ont pu être ajoutées ou
	// supprimées (ni après une saisie, ni après un tri, ni après un
	// changement de colonne).
	n, err := cachedOn(t, "rows:"+tb.sql(), []string{store.NoMeta, store.CountOf(tb.sql())}, func() (int, error) {
		var n int
		work.Phase(t.Ctx, "count")
		err := t.row(`SELECT count(*) FROM ` + tb.sql()).Scan(&n)
		return n, err
	})
	if err != nil {
		return err
	}
	tb.Rows = n
	return t.loadFilter(tb)
}

// rowID donne la clé technique de la ligne de données n (0 = première) parmi
// les lignes affichées.
func (t *tx) rowID(tb Table, n int) (id int64, pos string, err error) {
	if tb.Filtered() && n >= tb.Shown {
		return 0, "", tb.errFiltered(false)
	}
	err = t.row(`SELECT id, _pos FROM `+tb.view()+` ORDER BY _pos LIMIT 1 OFFSET ?`, n).Scan(&id, &pos)
	return
}

// cell est une cellule libre (hors tableau).
type cell struct {
	Row, Col int
	Raw      string
}

func (t *tx) cells(sheet int64) ([]cell, error) {
	rs, err := t.query(`SELECT row, col, raw FROM _lattis_cells WHERE sheet = ? ORDER BY row, col`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []cell
	for rs.Next() {
		var c cell
		if err := rs.Scan(&c.Row, &c.Col, &c.Raw); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rs.Err()
}

// ensureGapBelow garde au moins une ligne vide sous un objet qui vient de
// grandir de oldEdge à newEdge sur les colonnes [lo, hi] : si un tableau ou
// une cellule libre de cette bande commence trop près, toute la bande sous
// oldEdge descend, et chaque tableau déplacé fait de même avec sa propre
// bande. Jamais de fusion automatique.
func (t *tx) ensureGapBelow(sheet int64, self int64, oldEdge, newEdge, lo, hi int) error {
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	cells, err := t.cells(sheet)
	if err != nil {
		return err
	}
	first := -1
	for _, tb := range tables {
		if tb.ID != self && tb.spansCols(lo, hi) && tb.Top() > oldEdge && (first < 0 || tb.Top() < first) {
			first = tb.Top()
		}
	}
	outs, err := t.outCells(sheet)
	if err != nil {
		return err
	}
	// Une cellule gardée hors du tableau (note, total) reste collée à lui :
	// elle ne demande pas de ligne vide.
	delta := 0
	if first >= 0 {
		delta = newEdge + 2 - first
	}
	for _, c := range cells {
		if c.Col >= lo && c.Col <= hi && c.Row > oldEdge {
			need := newEdge + 2 - c.Row
			if outs[[2]int{c.Row, c.Col}] {
				need--
			}
			delta = max(delta, need)
		}
	}
	if delta <= 0 {
		return nil
	}
	// Toute la bande bouge d'abord, puis chaque tableau déplacé dégage sa
	// propre bande : un objet n'est jamais déplacé deux fois.
	var moved []Table
	for _, tb := range tables {
		if tb.ID == self || !tb.spansCols(lo, hi) || tb.Top() <= oldEdge {
			continue
		}
		if err := t.moveTable(tb, delta, 0); err != nil {
			return err
		}
		moved = append(moved, tb)
	}
	if err := t.moveCells(sheet, func(c cell) bool { return c.Col >= lo && c.Col <= hi && c.Row > oldEdge }, delta, 0); err != nil {
		return err
	}
	for _, tb := range moved {
		if err := t.ensureGapBelow(sheet, tb.ID, tb.Bottom(), tb.Bottom()+delta, tb.Col, tb.Right()); err != nil {
			return err
		}
	}
	return nil
}

// ensureGapRight est le pendant horizontal de ensureGapBelow.
func (t *tx) ensureGapRight(sheet int64, self int64, oldEdge, newEdge, lo, hi int) error {
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	cells, err := t.cells(sheet)
	if err != nil {
		return err
	}
	first := -1
	for _, tb := range tables {
		if tb.ID != self && tb.spansRows(lo, hi) && tb.Col > oldEdge && (first < 0 || tb.Col < first) {
			first = tb.Col
		}
	}
	outs, err := t.outCells(sheet)
	if err != nil {
		return err
	}
	delta := 0
	if first >= 0 {
		delta = newEdge + 2 - first
	}
	for _, c := range cells {
		if c.Row >= lo && c.Row <= hi && c.Col > oldEdge {
			need := newEdge + 2 - c.Col
			if outs[[2]int{c.Row, c.Col}] {
				need--
			}
			delta = max(delta, need)
		}
	}
	if delta <= 0 {
		return nil
	}
	// Toute la bande bouge d'abord, puis chaque tableau déplacé dégage sa
	// propre bande : un objet n'est jamais déplacé deux fois.
	var moved []Table
	for _, tb := range tables {
		if tb.ID == self || !tb.spansRows(lo, hi) || tb.Col <= oldEdge {
			continue
		}
		if err := t.moveTable(tb, 0, delta); err != nil {
			return err
		}
		moved = append(moved, tb)
	}
	band := func(c cell) bool { return c.Row >= lo && c.Row <= hi && c.Col > oldEdge }
	// Une liste libre qui dépasse la bande bouge entière (audit 2 I8).
	whole := wholeIslands(cells, band, func(c cell) bool { return c.Col > oldEdge }, delta, func(r, c int) bool {
		for _, tb := range tables {
			dc := 0
			for _, m := range moved {
				if m.ID == tb.ID {
					dc = delta
				}
			}
			if r >= tb.Top() && r <= tb.Bottom()+1 && c >= tb.Col+dc-1 && c <= tb.Right()+dc+1 {
				return true
			}
		}
		return false
	})
	if err := t.moveCells(sheet, func(c cell) bool { return band(c) || whole[[2]int{c.Row, c.Col}] }, 0, delta); err != nil {
		return err
	}
	for _, tb := range moved {
		if err := t.ensureGapRight(sheet, tb.ID, tb.Right(), tb.Right()+delta, tb.Top(), tb.Bottom()); err != nil {
			return err
		}
	}
	return nil
}

// wholeIslands étend les cellules libres de la bande qui s'écarte de delta
// colonnes aux îlots dont elles font partie (du côté qui bouge : side), et
// aux cellules qu'ils rencontreraient : une liste n'est jamais coupée. Rend
// les cellules en plus ; aucune si l'une arriverait contre un tableau
// (blocked), la bande seule bouge alors, comme avant.
func wholeIslands(cells []cell, band, side func(cell) bool, delta int, blocked func(r, c int) bool) map[[2]int]bool {
	at := make(map[[2]int]cell, len(cells))
	for _, c := range cells {
		at[[2]int{c.Row, c.Col}] = c
	}
	moved := map[[2]int]bool{}
	var stack [][2]int
	for _, c := range cells {
		if band(c) {
			p := [2]int{c.Row, c.Col}
			moved[p] = true
			stack = append(stack, p)
		}
	}
	extra := map[[2]int]bool{}
	add := func(p [2]int) {
		c, ok := at[p]
		if !ok || moved[p] || !side(c) {
			return
		}
		moved[p], extra[p] = true, true
		stack = append(stack, p)
	}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {0, delta}} {
			add([2]int{p[0] + d[0], p[1] + d[1]})
		}
	}
	for p := range extra {
		if blocked(p[0], p[1]+delta) {
			return nil
		}
	}
	return extra
}

func (t *tx) moveTable(tb Table, dRow, dCol int) error {
	return t.apply(
		[]stmt{st(`UPDATE _lattis_tables SET row = row + ?, col = col + ? WHERE id = ?`, dRow, dCol, tb.ID)},
		[]stmt{st(`UPDATE _lattis_tables SET row = row - ?, col = col - ? WHERE id = ?`, dRow, dCol, tb.ID)},
	)
}

// moveCells déplace des cellules libres, avec leurs noms, leur mise en forme
// et les adresses qui les citent. Chaque cellule déplacée est une
// opération explicite (suppression puis insertion) : l'inverse est exact et
// ne dépend pas de l'ordre de mise à jour de la clé primaire.
func (t *tx) moveCells(sheet int64, match func(cell) bool, dRow, dCol int) error {
	cells, err := t.cells(sheet)
	if err != nil {
		return err
	}
	var moved []cell
	for _, c := range cells {
		if match(c) {
			moved = append(moved, c)
		}
	}
	if len(moved) == 0 {
		return nil
	}
	var do, undo []stmt
	for _, c := range moved {
		do = append(do, st(`DELETE FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, c.Row, c.Col))
		undo = append(undo, st(`DELETE FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, c.Row+dRow, c.Col+dCol))
	}
	for _, c := range moved {
		do = append(do, st(`INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (?, ?, ?, ?)`, sheet, c.Row+dRow, c.Col+dCol, c.Raw))
		undo = append(undo, st(`INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (?, ?, ?, ?)`, sheet, c.Row, c.Col, c.Raw))
	}
	if err := t.apply(do, undo); err != nil {
		return err
	}
	if err := t.moveMarks(sheet, match, dRow, dCol); err != nil {
		return err
	}
	// Noms (même d'une cellule vide) et mise en forme suivent, et les
	// formules qui citaient les cellules déplacées aussi.
	var named []cell
	rs, err := t.query(`SELECT row, col FROM _lattis_names WHERE sheet = ?`, sheet)
	if err != nil {
		return err
	}
	for rs.Next() {
		var c cell
		if err := rs.Scan(&c.Row, &c.Col); err != nil {
			rs.Close()
			return err
		}
		if match(c) {
			named = append(named, c)
		}
	}
	rs.Close()
	if err := t.moveNames(sheet, named, dRow, dCol); err != nil {
		return err
	}
	if err := t.moveFreeStyles(sheet, match, dRow, dCol); err != nil {
		return err
	}
	return t.rewriteCellRefs(sheet, func(r, c int) (int, int, bool) {
		if match(cell{Row: r, Col: c}) {
			return r + dRow, c + dCol, true
		}
		return r, c, true
	})
}

// moveFreeStyles déplace la mise en forme des cellules libres retenues par
// match (deux temps : jamais deux styles à la même place).
func (t *tx) moveFreeStyles(sheet int64, match func(cell) bool, dRow, dCol int) error {
	rs, err := t.query(`SELECT row, col, style FROM _lattis_cell_styles WHERE sheet = ?`, sheet)
	if err != nil {
		return err
	}
	type styled struct {
		row, col int
		style    string
	}
	var ss []styled
	for rs.Next() {
		var s styled
		if err := rs.Scan(&s.row, &s.col, &s.style); err != nil {
			rs.Close()
			return err
		}
		if match(cell{Row: s.row, Col: s.col}) {
			ss = append(ss, s)
		}
	}
	rs.Close()
	if len(ss) == 0 {
		return nil
	}
	del := `DELETE FROM _lattis_cell_styles WHERE sheet = ? AND row = ? AND col = ?`
	ins := `INSERT OR REPLACE INTO _lattis_cell_styles (sheet, row, col, style) VALUES (?, ?, ?, ?)`
	var do, undo []stmt
	for _, s := range ss {
		do = append(do, st(del, sheet, s.row, s.col))
		undo = append(undo, st(del, sheet, s.row+dRow, s.col+dCol))
	}
	for _, s := range ss {
		do = append(do, st(ins, sheet, s.row+dRow, s.col+dCol, s.style))
		undo = append(undo, st(ins, sheet, s.row, s.col, s.style))
	}
	return t.apply(do, undo)
}

// island renvoie la zone de cellules libres contiguës (4-connexité) qui
// contient (row, col).
func island(cells []cell, row, col int) []cell {
	at := make(map[[2]int]cell, len(cells))
	for _, c := range cells {
		at[[2]int{c.Row, c.Col}] = c
	}
	seen := map[[2]int]bool{}
	stack := [][2]int{{row, col}}
	var out []cell
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		c, ok := at[p]
		if !ok || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, c)
		stack = append(stack, [2]int{p[0] + 1, p[1]}, [2]int{p[0] - 1, p[1]}, [2]int{p[0], p[1] + 1}, [2]int{p[0], p[1] - 1})
	}
	slices.SortFunc(out, func(a, b cell) int {
		if a.Row != b.Row {
			return a.Row - b.Row
		}
		return a.Col - b.Col
	})
	return out
}

// settle écarte les tableaux de la feuille qui se chevauchent ou se touchent
// (un tableau « rentre » s'il garde au moins une cellule libre de chaque
// côté) : l'autre tableau part vers la droite jusqu'à rentrer, et pousse à
// son tour ceux qu'il rencontre. keep (0 : aucun) ne bouge jamais ; entre
// deux autres tableaux, c'est celui de droite (puis du bas) qui cède.
func (t *tx) settle(sheet, keep int64) error {
	tables, err := t.tables(sheet)
	if err != nil || len(tables) < 2 {
		return err
	}
	cells, err := t.cells(sheet)
	if err != nil {
		return err
	}
	orig := make([]int, len(tables))
	for i, tb := range tables {
		orig[i] = tb.Col
	}
	// cède : b s'écarte de a.
	yields := func(a, b Table) bool {
		switch {
		case b.ID == keep:
			return false
		case a.ID == keep:
			return true
		case a.Col != b.Col:
			return a.Col < b.Col
		case a.Row != b.Row:
			return a.Row < b.Row
		}
		return a.ID < b.ID
	}
	blocked := func(b Table) bool {
		for _, c := range cells {
			if c.Row >= b.Top()-1 && c.Row <= b.Bottom()+1 && c.Col >= b.Col-1 && c.Col <= b.Right()+1 {
				return true
			}
		}
		return false
	}
	for changed, n := true, 0; changed; n++ {
		if n > 10*len(tables) {
			return msg.Wrap(ErrTable, "table.spreadFailed", nil)
		}
		changed = false
		for i := range tables {
			for j := range tables {
				a, b := tables[i], tables[j]
				if i == j || !touches(a.Top(), a.Bottom(), a.Col, a.Right(), b) || !yields(a, b) {
					continue
				}
				b.Col = a.Right() + 2
				for blocked(b) {
					b.Col++
				}
				tables[j], changed = b, true
			}
		}
	}
	for i, tb := range tables {
		if d := tb.Col - orig[i]; d != 0 {
			if err := t.moveTable(tb, 0, d); err != nil {
				return err
			}
		}
	}
	return nil
}
