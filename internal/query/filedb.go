package query

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/ncruces/go-sqlite3/driver"
)

// Fichiers de base de données : SQLite (pilote déjà utilisé par le
// classeur, Go pur) et DuckDB (voir duckdb.go). Le fichier est ouvert en
// lecture seule ; la requête SQL de la source y est exécutée telle quelle.

// Genres de fichier de base (Source.Kind).
const (
	KindSQLite = "sqlite"
	KindDuckDB = "duckdb"
)

func init() {
	dialects[KindSQLite] = &dialect{
		label:      "SQLite",
		openFile:   openSQLiteFile,
		authFailed: func(error) bool { return false },
		limit:      limitClause,
		readOnly:   true,
		inferTypes: true,
		tables:     sqliteTables,
		ident:      quoteANSI,
	}
}

// sqliteFileDSN écrit l'adresse d'un fichier SQLite ouvert en lecture
// seule (mode=ro, et query_only par sécurité).
func sqliteFileDSN(path string) string {
	p := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(path)
	return "file:" + p + "?mode=ro&_pragma=query_only(1)"
}

func openSQLiteFile(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("fichier introuvable : %w", err)
	}
	db, err := driver.Open(sqliteFileDSN(path))
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

func sqliteTables(ctx context.Context, db *sql.DB) ([]DBTable, error) {
	rs, err := db.QueryContext(ctx, `SELECT name, type FROM sqlite_master
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite\_%' ESCAPE '\'
		ORDER BY name LIMIT 2000`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []DBTable{}
	for rs.Next() {
		var t DBTable
		var kind string
		if err := rs.Scan(&t.Name, &kind); err != nil {
			return nil, err
		}
		t.View = kind == "view"
		out = append(out, t)
	}
	return out, rs.Err()
}
