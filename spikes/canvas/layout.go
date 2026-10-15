package canvas

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/lalamefine/slitable/internal/grid"
	"github.com/lalamefine/slitable/internal/store"
)

// schema crée les tables de métadonnées. Les tables utilisateur s'appellent
// t_<id> et leurs colonnes c_<id> : un nom affiché peut changer, contenir
// des accents, des espaces ou un mot réservé sans toucher au schéma SQL.
const schema = `
CREATE TABLE IF NOT EXISTS _slitable_sheets (
	id   INTEGER PRIMARY KEY,
	name TEXT NOT NULL UNIQUE
) STRICT;
CREATE TABLE IF NOT EXISTS _slitable_tables (
	id    INTEGER PRIMARY KEY,
	sheet INTEGER NOT NULL REFERENCES _slitable_sheets(id),
	name  TEXT NOT NULL UNIQUE,
	row   INTEGER NOT NULL, -- ancre : ligne d'en-tête
	col   INTEGER NOT NULL  -- ancre : première colonne
) STRICT;
CREATE TABLE IF NOT EXISTS _slitable_columns (
	tbl   INTEGER NOT NULL REFERENCES _slitable_tables(id),
	id    INTEGER NOT NULL,
	name  TEXT NOT NULL,
	type  TEXT NOT NULL,
	fixed INTEGER NOT NULL DEFAULT 0, -- 0 : type inféré, élargi automatiquement
	pos   INTEGER NOT NULL,           -- rang d'affichage, 0..n-1
	PRIMARY KEY (tbl, id)
) STRICT;
CREATE TABLE IF NOT EXISTS _slitable_cells (
	sheet INTEGER NOT NULL REFERENCES _slitable_sheets(id),
	row   INTEGER NOT NULL,
	col   INTEGER NOT NULL,
	raw   TEXT NOT NULL,
	PRIMARY KEY (sheet, row, col)
) STRICT;
CREATE TABLE IF NOT EXISTS _slitable_invalid (
	tbl INTEGER NOT NULL,
	rid INTEGER NOT NULL,
	col INTEGER NOT NULL,
	raw TEXT NOT NULL,
	PRIMARY KEY (tbl, rid, col)
) STRICT;
INSERT OR IGNORE INTO _slitable_sheets (id, name) VALUES (1, 'Feuille 1');
`

// Open ouvre (ou crée) un document.
func Open(path string) (*Doc, error) {
	db, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	err = db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(schema)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Doc{db: db}, nil
}

// Close ferme le document.
func (d *Doc) Close() error { return d.db.Close() }

func tableSQL(id int64) string { return fmt.Sprintf("t_%d", id) }
func colSQL(id int64) string   { return fmt.Sprintf("c_%d", id) }

// Column décrit une colonne de tableau, dans l'ordre d'affichage.
type Column struct {
	ID    int64
	Name  string
	Type  grid.Type
	Fixed bool
}

// Table décrit un tableau posé sur une feuille.
type Table struct {
	ID       int64
	Sheet    int64
	Name     string
	Row, Col int // ancre (cellule d'en-tête en haut à gauche)
	Cols     []Column
	Rows     int // lignes de données
}

// Bottom est la dernière ligne occupée (l'en-tête si le tableau est vide).
func (t Table) Bottom() int { return t.Row + t.Rows }

// Right est la dernière colonne occupée.
func (t Table) Right() int { return t.Col + len(t.Cols) - 1 }

func (t Table) sql() string { return tableSQL(t.ID) }

// spans indique si les colonnes [lo, hi] recoupent celles du tableau.
func (t Table) spansCols(lo, hi int) bool { return t.Col <= hi && t.Right() >= lo }
func (t Table) spansRows(lo, hi int) bool { return t.Row <= hi && t.Bottom() >= lo }

func (t *tx) tables(sheet int64) ([]Table, error) {
	rs, err := t.query(`SELECT id, name, row, col FROM _slitable_tables WHERE sheet = ? ORDER BY id`, sheet)
	if err != nil {
		return nil, err
	}
	var out []Table
	for rs.Next() {
		tb := Table{Sheet: sheet}
		if err := rs.Scan(&tb.ID, &tb.Name, &tb.Row, &tb.Col); err != nil {
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
	err := t.row(`SELECT sheet, name, row, col FROM _slitable_tables WHERE id = ?`, id).
		Scan(&tb.Sheet, &tb.Name, &tb.Row, &tb.Col)
	if err != nil {
		return tb, err
	}
	return tb, t.fill(&tb)
}

func (t *tx) tableByName(name string) (Table, error) {
	var id int64
	if err := t.row(`SELECT id FROM _slitable_tables WHERE name = ?`, name).Scan(&id); err != nil {
		return Table{}, fmt.Errorf("tableau %q : %w", name, err)
	}
	return t.table(id)
}

// fill charge les colonnes et le nombre de lignes.
func (t *tx) fill(tb *Table) error {
	rs, err := t.query(`SELECT id, name, type, fixed FROM _slitable_columns WHERE tbl = ? ORDER BY pos`, tb.ID)
	if err != nil {
		return err
	}
	defer rs.Close()
	tb.Cols = nil
	for rs.Next() {
		var c Column
		if err := rs.Scan(&c.ID, &c.Name, &c.Type, &c.Fixed); err != nil {
			return err
		}
		tb.Cols = append(tb.Cols, c)
	}
	if err := rs.Err(); err != nil {
		return err
	}
	return t.row(`SELECT count(*) FROM ` + tb.sql()).Scan(&tb.Rows)
}

// rowID donne la clé technique de la ligne de données n (0 = première).
func (t *tx) rowID(tb Table, n int) (id int64, pos string, err error) {
	err = t.row(`SELECT id, _pos FROM `+tb.sql()+` ORDER BY _pos LIMIT 1 OFFSET ?`, n).Scan(&id, &pos)
	return
}

// cell est une cellule libre (hors tableau).
type cell struct {
	Row, Col int
	Raw      string
}

func (t *tx) cells(sheet int64) ([]cell, error) {
	rs, err := t.query(`SELECT row, col, raw FROM _slitable_cells WHERE sheet = ? ORDER BY row, col`, sheet)
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
		if tb.ID != self && tb.spansCols(lo, hi) && tb.Row > oldEdge && (first < 0 || tb.Row < first) {
			first = tb.Row
		}
	}
	for _, c := range cells {
		if c.Col >= lo && c.Col <= hi && c.Row > oldEdge && (first < 0 || c.Row < first) {
			first = c.Row
		}
	}
	if first < 0 || first > newEdge+1 {
		return nil
	}
	delta := newEdge + 2 - first
	// Toute la bande bouge d'abord, puis chaque tableau déplacé dégage sa
	// propre bande : un objet n'est jamais déplacé deux fois.
	var moved []Table
	for _, tb := range tables {
		if tb.ID == self || !tb.spansCols(lo, hi) || tb.Row <= oldEdge {
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
	for _, c := range cells {
		if c.Row >= lo && c.Row <= hi && c.Col > oldEdge && (first < 0 || c.Col < first) {
			first = c.Col
		}
	}
	if first < 0 || first > newEdge+1 {
		return nil
	}
	delta := newEdge + 2 - first
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
	if err := t.moveCells(sheet, func(c cell) bool { return c.Row >= lo && c.Row <= hi && c.Col > oldEdge }, 0, delta); err != nil {
		return err
	}
	for _, tb := range moved {
		if err := t.ensureGapRight(sheet, tb.ID, tb.Right(), tb.Right()+delta, tb.Row, tb.Bottom()); err != nil {
			return err
		}
	}
	return nil
}

func (t *tx) moveTable(tb Table, dRow, dCol int) error {
	return t.apply(
		[]stmt{st(`UPDATE _slitable_tables SET row = row + ?, col = col + ? WHERE id = ?`, dRow, dCol, tb.ID)},
		[]stmt{st(`UPDATE _slitable_tables SET row = row - ?, col = col - ? WHERE id = ?`, dRow, dCol, tb.ID)},
	)
}

// moveCells déplace des cellules libres. Chaque cellule déplacée est une
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
		do = append(do, st(`DELETE FROM _slitable_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, c.Row, c.Col))
		undo = append(undo, st(`DELETE FROM _slitable_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, c.Row+dRow, c.Col+dCol))
	}
	for _, c := range moved {
		do = append(do, st(`INSERT INTO _slitable_cells (sheet, row, col, raw) VALUES (?, ?, ?, ?)`, sheet, c.Row+dRow, c.Col+dCol, c.Raw))
		undo = append(undo, st(`INSERT INTO _slitable_cells (sheet, row, col, raw) VALUES (?, ?, ?, ?)`, sheet, c.Row, c.Col, c.Raw))
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
