package grid

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/sqltext"
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
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rs, err := db.QueryContext(ctx, q, args...)
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
