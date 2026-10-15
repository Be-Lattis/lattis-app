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
	Name    string   `json:"name"`
	Rows    int      `json:"rows"`
	Columns []Column `json:"columns"`
}

// Window est une fenêtre de lignes consécutives, à partir de la ligne First
// (0 = première ligne dans l'ordre d'affichage). Cells est indexé
// [ligne][colonne] ; IDs donne la clé technique de chaque ligne.
type Window struct {
	First  int     `json:"first"`
	IDs    []int64 `json:"ids"`
	Cells  [][]any `json:"cells"`
	Micros int64   `json:"micros"` // temps passé côté Go, pour les mesures
}

// Table donne accès à un tableau stocké dans une table SQLite.
type Table struct {
	db      *store.Store
	sqlName string
	name    string
	cols    []Column

	mu      sync.RWMutex
	anchors []string
	rows    int

	selectWindow string
}

// ErrRange signale une fenêtre ou une colonne hors du tableau.
var ErrRange = errors.New("grid: hors du tableau")

// NewTable relie un tableau existant. Load doit être appelé avant les lectures.
func NewTable(db *store.Store, sqlName, name string, cols []Column) *Table {
	names := make([]string, len(cols))
	for i := range cols {
		names[i] = colName(i)
	}
	return &Table{
		db:      db,
		sqlName: sqlName,
		name:    name,
		cols:    cols,
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
	return Info{Name: t.name, Rows: t.rows, Columns: t.cols}
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

	rs, err := t.db.Read().QueryContext(ctx, t.selectWindow, anchor, count, first%AnchorEvery)
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
			row[c] = jsonValue(dest[c+1])
		}
		w.IDs = append(w.IDs, id)
		w.Cells = append(w.Cells, row)
	}
	if err := rs.Err(); err != nil {
		return Window{}, err
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
		res, err := tx.ExecContext(ctx, `UPDATE `+t.sqlName+` SET `+colName(col)+` = ? WHERE id = ?`, v, id)
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
