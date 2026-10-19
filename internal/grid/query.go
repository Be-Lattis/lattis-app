package grid

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/sqltext"
	"github.com/lalamefine/lattis/internal/store"
)

// QueryResult est le résultat d'une requête de l'onglet SQL.
type QueryResult struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"` // plus de lignes que la limite
	Micros    int64    `json:"micros"`
}

// ErrEmptyQuery signale une requête vide.
var ErrEmptyQuery = errors.New("requête vide")

// ReadTimeout borne une requête de lecture (onglet SQL, db.query des
// scripts) ; ReadTimeoutSetting, s'il est fixé, donne la durée réglée par
// l'utilisateur ou la DSI (limits.sqlReadSeconds).
const ReadTimeout = 30 * time.Second

var ReadTimeoutSetting func() time.Duration

func readTimeout() time.Duration {
	if ReadTimeoutSetting != nil {
		if d := ReadTimeoutSetting(); d > 0 {
			return d
		}
	}
	return ReadTimeout
}

// Query exécute une requête en lecture seule (le pool de lecture est en
// query_only : une écriture échoue) et renvoie au plus limit lignes.
func Query(ctx context.Context, db *sql.DB, q string, limit int) (QueryResult, error) {
	return QueryArgs(ctx, db, q, limit)
}

// QueryArgs est Query avec des paramètres (?). Un changement de structure
// (DROP, ALTER, CREATE…) est refusé avec un message clair, et les erreurs
// de SQLite sont traduites en français.
func QueryArgs(ctx context.Context, db *sql.DB, q string, limit int, args ...any) (QueryResult, error) {
	res, err := queryArgs(ctx, db, q, limit, args...)
	return res, sqltext.French(err)
}

func queryArgs(ctx context.Context, db *sql.DB, q string, limit int, args ...any) (QueryResult, error) {
	if strings.TrimSpace(q) == "" {
		return QueryResult{}, ErrEmptyQuery
	}
	if err := sqltext.CheckStructure(q); err != nil {
		return QueryResult{}, err
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, readTimeout())
	defer cancel()
	// Requête de l'utilisateur : sous l'autorisateur (ni ATTACH, ni tables
	// internes _lattis_*, voir store.UserConn).
	conn, err := store.UserConn(ctx, db)
	if err != nil {
		return QueryResult{}, err
	}
	defer store.ReleaseUserConn(conn)
	rs, err := conn.QueryContext(ctx, q, args...)
	if err != nil {
		return QueryResult{}, err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return QueryResult{}, err
	}
	res := QueryResult{Columns: cols, Rows: [][]any{}}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rs.Next() {
		if len(res.Rows) == limit {
			res.Truncated = true
			break
		}
		if err := rs.Scan(ptrs...); err != nil {
			return QueryResult{}, err
		}
		row := make([]any, len(cols))
		for i, v := range vals {
			row[i] = jsonValue(v)
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rs.Err(); err != nil {
		return QueryResult{}, err
	}
	res.Micros = time.Since(start).Microseconds()
	return res, nil
}

// Technical dit si une colonne de résultat est technique : son nom commence
// par « _ » (_pos…). Les aperçus de l'onglet SQL et les résultats posés sur
// une feuille ne la montrent pas (docs/decisions/0007).
func Technical(name string) bool { return strings.HasPrefix(name, "_") }

// Visible donne les rangs des colonnes non techniques.
func Visible(cols []string) []int {
	out := make([]int, 0, len(cols))
	for i, c := range cols {
		if !Technical(c) {
			out = append(out, i)
		}
	}
	return out
}

// Pick garde de chaque ligne les valeurs aux rangs idx.
func Pick(row []any, idx []int) []any {
	out := make([]any, len(idx))
	for k, i := range idx {
		out[k] = row[i]
	}
	return out
}

// WithoutTechnical retire du résultat les colonnes techniques.
func (r QueryResult) WithoutTechnical() QueryResult {
	idx := Visible(r.Columns)
	if len(idx) == len(r.Columns) {
		return r
	}
	cols := make([]string, len(idx))
	for k, i := range idx {
		cols[k] = r.Columns[i]
	}
	rows := make([][]any, len(r.Rows))
	for j, row := range r.Rows {
		rows[j] = Pick(row, idx)
	}
	r.Columns, r.Rows = cols, rows
	return r
}
