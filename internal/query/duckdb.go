//go:build duckdb && cgo

package query

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/duckdb/duckdb-go/v2"
)

// DuckDB : pilote duckdb/duckdb-go, en C (CGO). Il alourdit l'exécutable
// d'environ 70 Mo : il n'est compilé qu'avec -tags duckdb (voir
// duckdb_stub.go et la décision 0003).

func init() {
	dialects[KindDuckDB] = &dialect{
		label:      "DuckDB",
		openFile:   openDuckDB,
		authFailed: func(error) bool { return false },
		limit:      limitClause,
		tables: infoSchemaTables(`SELECT table_schema, table_name, table_type
			FROM information_schema.tables
			WHERE table_schema NOT IN ('information_schema', 'pg_catalog')
			ORDER BY table_schema = 'main' DESC, table_schema, table_name LIMIT 2000`),
		ident: quoteANSI,
	}
}

// DuckDBAvailable : le pilote DuckDB est compilé dans cette version.
const DuckDBAvailable = true

func openDuckDB(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("fichier introuvable : %w", err)
	}
	db, err := sql.Open("duckdb", path+"?access_mode=READ_ONLY")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
