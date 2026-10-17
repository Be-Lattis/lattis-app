//go:build !(duckdb && cgo)

package query

import (
	"database/sql"
	"errors"
)

// Sans -tags duckdb (ou sans CGO), DuckDB n'est pas compilé : la source
// reste connue mais son ouverture explique pourquoi elle échoue.

func init() {
	dialects[KindDuckDB] = &dialect{
		label:      "DuckDB",
		openFile:   func(string) (*sql.DB, error) { return nil, errNoDuckDB },
		authFailed: func(error) bool { return false },
		ident:      quoteANSI,
	}
}

// DuckDBAvailable : le pilote DuckDB est compilé dans cette version.
const DuckDBAvailable = false

var errNoDuckDB = errors.New("DuckDB n'est pas disponible dans cette version de Slitable (pilote en C : compiler avec CGO et -tags duckdb)")
