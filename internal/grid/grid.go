// Package grid sert les lignes d'un tableau à la grille, par fenêtres.
//
// La table n'a pas d'ordre propre : chaque ligne porte une clé d'ordre _pos
// (voir internal/order), indexée. Lire la ligne n par LIMIT/OFFSET obligerait
// SQLite à parcourir n lignes ; on relève donc une clé « ancre » toutes les
// AnchorEvery lignes et on part de l'ancre la plus proche, ce qui borne
// l'OFFSET à AnchorEvery-1 quelle que soit la position dans la table.
//
// Prototype de la phase 1.1 : un seul tableau de démonstration, sans
// insertion ni suppression de lignes (les ancres restent donc valables).
package grid

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/slitable/internal/store"
)

// AnchorEvery est l'écart, en lignes, entre deux ancres.
const AnchorEvery = 1000

// MaxWindow borne le nombre de lignes d'une fenêtre.
const MaxWindow = 2000

// Column décrit une colonne affichée.
type Column struct {
	Name string `json:"name"`
	Type Type   `json:"type"`
}

// Info décrit le tableau.
type Info struct {
	Name       string   `json:"name"`
	SQLName    string   `json:"sqlName"` // nom de la table SQL (onglet SQL)
	SQLColumns []string `json:"sqlColumns"`
	Rows       int      `json:"rows"`
	Columns    []Column `json:"columns"`
}

// Window est une fenêtre de lignes consécutives, à partir de la ligne First
// (0 = première ligne dans l'ordre d'affichage). Cells est indexé
// [ligne][colonne] ; IDs donne la clé technique de chaque ligne.
type Window struct {
	First   int           `json:"first"`
	IDs     []int64       `json:"ids"`
	Cells   [][]any       `json:"cells"`
	Invalid []InvalidCell `json:"invalid"` // saisies hors type de la fenêtre
	Micros  int64         `json:"micros"`  // temps passé côté Go, pour les mesures
}

// InvalidCell est une saisie hors type : la cellule est vide, la valeur
// brute est gardée pour l'affichage et la correction.
type InvalidCell struct {
	Row int    `json:"row"` // indice dans la fenêtre
	Col int    `json:"col"` // colonne affichée
	Raw string `json:"raw"`
}

// Table donne accès à un tableau stocké dans une table SQLite.
type Table struct {
	db      *store.Store
	sqlName string
	name    string
	cols    []Column
	sqlCols []string // noms SQL des colonnes, dans l'ordre d'affichage

	mu      sync.RWMutex
	anchors []string
	rows    int

	selectWindow string
	args         []any                    // paramètres des expressions de colonnes (?1…)
	decode       func(col int, v any) any // conversion des valeurs calculées

	// Saisies hors type (tableaux du classeur) : identifiant du tableau et
	// identifiant de chaque colonne affichée.
	tableID int64
	colIDs  map[int64]int
}

// WithExprs remplace les colonnes lues par des expressions SQL (colonnes
// calculées), avec leurs paramètres numérotés ?1…?n, et une conversion des
// valeurs lues pour l'interface.
func (t *Table) WithExprs(exprs []string, args []any, decode func(col int, v any) any) *Table {
	t.selectWindow = fmt.Sprintf(`SELECT id, %s FROM %s WHERE _pos >= ? ORDER BY _pos LIMIT ? OFFSET ?`,
		strings.Join(exprs, ", "), t.sqlName)
	t.args, t.decode = args, decode
	return t
}

// WithInvalid fait lire les saisies hors type de _slitable_invalid avec les
// fenêtres.
func (t *Table) WithInvalid(tableID int64, colIDs []int64) *Table {
	t.tableID = tableID
	t.colIDs = map[int64]int{}
	for i, id := range colIDs {
		t.colIDs[id] = i
	}
	return t
}

// ErrRange signale une fenêtre ou une colonne hors du tableau.
var ErrRange = errors.New("grid: hors du tableau")

// NewTable relie un tableau existant ; sqlCols donne le nom SQL de chaque
// colonne (nil : c_0, c_1…). Load doit être appelé avant les lectures.
func NewTable(db *store.Store, sqlName, name string, cols []Column, sqlCols []string) *Table {
	names := sqlCols
	if names == nil {
		names = make([]string, len(cols))
		for i := range cols {
			names[i] = colName(i)
		}
	}
	return &Table{
		db:      db,
		sqlName: sqlName,
		name:    name,
		cols:    cols,
		sqlCols: names,
		selectWindow: fmt.Sprintf(`SELECT id, %s FROM %s WHERE _pos >= ? ORDER BY _pos LIMIT ? OFFSET ?`,
			strings.Join(names, ", "), sqlName),
	}
}

// colName donne l'identifiant SQL interne de la colonne i. Le nom affiché est
// dans les métadonnées : un renommage ne touche jamais au schéma.
func colName(i int) string { return fmt.Sprintf("c_%d", i) }

// Load relève les ancres et le nombre de lignes.
func (t *Table) Load(ctx context.Context) error {
	rs, err := t.db.Read().QueryContext(ctx, `SELECT _pos FROM `+t.sqlName+` ORDER BY _pos`)
	if err != nil {
		return err
	}
	defer rs.Close()
	var anchors []string
	n := 0
	for rs.Next() {
		if n%AnchorEvery == 0 {
			var p string
			if err := rs.Scan(&p); err != nil {
				return err
			}
			anchors = append(anchors, p)
		}
		n++
	}
	if err := rs.Err(); err != nil {
		return err
	}
	t.mu.Lock()
	t.anchors, t.rows = anchors, n
	t.mu.Unlock()
	return nil
}

// Info décrit le tableau.
func (t *Table) Info() Info {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return Info{Name: t.name, SQLName: t.sqlName, SQLColumns: t.sqlCols, Rows: t.rows, Columns: t.cols}
}

// Window lit count lignes à partir de la ligne first.
func (t *Table) Window(ctx context.Context, first, count int) (Window, error) {
	start := time.Now()
	t.mu.RLock()
	rows := t.rows
	var anchor string
	if first >= 0 && first < rows {
		anchor = t.anchors[first/AnchorEvery]
	}
	t.mu.RUnlock()
	if first < 0 || first >= rows || count <= 0 {
		return Window{}, ErrRange
	}
	count = min(count, MaxWindow, rows-first)

	args := append(append([]any{}, t.args...), anchor, count, first%AnchorEvery)
	rs, err := t.db.Read().QueryContext(ctx, t.selectWindow, args...)
	if err != nil {
		return Window{}, err
	}
	defer rs.Close()

	w := Window{First: first, IDs: make([]int64, 0, count), Cells: make([][]any, 0, count)}
	dest := make([]any, len(t.cols)+1)
	ptrs := make([]any, len(dest))
	for i := range dest {
		ptrs[i] = &dest[i]
	}
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return Window{}, err
		}
		id, _ := dest[0].(int64)
		row := make([]any, len(t.cols))
		for c := range row {
			if t.decode != nil {
				row[c] = t.decode(c, dest[c+1])
				continue
			}
			row[c] = jsonValue(dest[c+1])
		}
		w.IDs = append(w.IDs, id)
		w.Cells = append(w.Cells, row)
	}
	if err := rs.Err(); err != nil {
		return Window{}, err
	}
	if t.colIDs != nil && len(w.IDs) > 0 {
		if err := t.invalid(ctx, &w); err != nil {
			return Window{}, err
		}
	}
	w.Micros = time.Since(start).Microseconds()
	return w, nil
}

// jsonValue évite qu'un texte lu en []byte soit encodé en base64.
func jsonValue(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

// SetCell remplace la valeur d'une cellule à partir d'une saisie et renvoie
// la valeur stockée.
func (t *Table) SetCell(ctx context.Context, id int64, col int, raw string) (any, error) {
	if col < 0 || col >= len(t.cols) {
		return nil, ErrRange
	}
	v, err := Parse(t.cols[col].Type, raw)
	if err != nil {
		return nil, err
	}
	err = t.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE `+t.sqlName+` SET `+t.sqlCols[col]+` = ? WHERE id = ?`, v, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return ErrRange
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (t *Table) invalid(ctx context.Context, w *Window) error {
	index := make(map[int64]int, len(w.IDs))
	lo, hi := w.IDs[0], w.IDs[0]
	for i, id := range w.IDs {
		index[id] = i
		lo, hi = min(lo, id), max(hi, id)
	}
	rs, err := t.db.Read().QueryContext(ctx,
		`SELECT rid, col, raw FROM _slitable_invalid WHERE tbl = ? AND rid BETWEEN ? AND ?`, t.tableID, lo, hi)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		var rid, col int64
		var raw string
		if err := rs.Scan(&rid, &col, &raw); err != nil {
			return err
		}
		r, ok1 := index[rid]
		c, ok2 := t.colIDs[col]
		if ok1 && ok2 {
			w.Invalid = append(w.Invalid, InvalidCell{r, c, raw})
		}
	}
	return rs.Err()
}
