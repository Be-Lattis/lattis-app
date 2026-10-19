package canvas

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// sheetLocator décrit les feuilles du classeur au moteur de formules, pour
// convertir les adresses A1 tapées à la Excel (formula.ConvertA1) et situer
// les plages des RECHERCHEV. Les rangs des lignes de tableau suivent l'ordre
// d'affichage sans filtre.
type sheetLocator struct {
	t      *tx
	home   int64
	sheets map[string]int64
	tables map[int64][]Table
	err    error
}

func (t *tx) locator(home int64) *sheetLocator {
	return &sheetLocator{t: t, home: home, tables: map[int64][]Table{}}
}

func (l *sheetLocator) Known(name string) bool {
	var n int
	err := l.t.row(`SELECT (SELECT count(*) FROM _lattis_names WHERE name = ? COLLATE NOCASE)
		+ (SELECT count(*) FROM _lattis_tables WHERE name = ? COLLATE NOCASE)`, name, name).Scan(&n)
	if err != nil && l.err == nil {
		l.err = err
	}
	return n > 0
}

func (l *sheetLocator) sheetTables(sheet string) ([]Table, bool) {
	id := l.home
	if sheet != "" {
		if l.sheets == nil {
			s, err := l.t.sheetIDs()
			if err != nil {
				l.err = err
				return nil, false
			}
			l.sheets = s
		}
		var ok bool
		if id, ok = l.sheets[strings.ToLower(sheet)]; !ok {
			return nil, false
		}
	}
	if ts, ok := l.tables[id]; ok {
		return ts, true
	}
	ts, err := l.t.tables(id)
	if err != nil {
		l.err = err
		return nil, false
	}
	l.tables[id] = ts
	return ts, true
}

func (l *sheetLocator) Tables(sheet string) ([]formula.TablePos, bool) {
	ts, ok := l.sheetTables(sheet)
	if !ok {
		return nil, false
	}
	out := make([]formula.TablePos, len(ts))
	for i, tb := range ts {
		cols := make([]string, len(tb.Cols))
		for k, c := range tb.Cols {
			cols[k] = c.Name
		}
		out[i] = formula.TablePos{Name: tb.Name, Row: tb.Row, Col: tb.Col, Cols: cols, Rows: tb.Rows}
	}
	return out, true
}

func (l *sheetLocator) Key(table string, k int) (formula.Node, bool) {
	tb, err := l.t.tableByName(table)
	if err != nil || tb.Key == 0 {
		return nil, false
	}
	for _, c := range tb.Cols {
		if c.ID != tb.Key {
			continue
		}
		var v any
		err := l.t.row(`SELECT `+colSQL(c.ID)+` FROM `+tb.sql()+` ORDER BY _pos LIMIT 1 OFFSET ?`, k-1).Scan(&v)
		if errors.Is(err, sql.ErrNoRows) || v == nil {
			return nil, false
		}
		if err != nil {
			l.err = err
			return nil, false
		}
		switch x := v.(type) {
		case int64:
			return formula.NumLit{V: float64(x)}, true
		case float64:
			return formula.NumLit{V: x}, true
		case []byte:
			return formula.StrLit{V: string(x)}, true
		case string:
			if c.Type == grid.Date {
				return formula.StrLit{V: text(x, grid.Date)}, true
			}
			return formula.StrLit{V: x}, true
		}
	}
	return nil, false
}
