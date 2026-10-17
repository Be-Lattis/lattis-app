//go:build duckdb && cgo

package query

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

// go test -tags duckdb ./internal/query -run DuckDB
func TestDuckDBFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ventes.duckdb")
	db, err := sql.Open("duckdb", p)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE ventes (id INTEGER, ville VARCHAR, montant DECIMAL(10,2), jour DATE, actif BOOLEAN)`,
		`INSERT INTO ventes VALUES (1, 'Paris', 10.5, DATE '2026-02-01', true), (2, 'Lyon', 20, NULL, false)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	ts, err := FileTables(ctx, KindDuckDB, p)
	if err != nil || fmt.Sprint(ts) != "[{main ventes false}]" {
		t.Fatalf("tables %v %v", ts, err)
	}
	d := newDoc(t)
	pv := previewOf(t, env(d), Source{Kind: SourceDuckDB, Path: p, SQL: TableSQL(KindDuckDB, "main", "ventes")})
	if pv.Error != "" {
		t.Fatal(pv.Error)
	}
	if got := fmt.Sprint(pv.Columns, pv.Rows); got != "[{id integer} {ville text} {montant decimal} {jour date} {actif boolean}] [[1 Paris 10,5 01/02/2026 VRAI] [2 Lyon 20  FAUX]]" {
		t.Fatal(got)
	}
	if pv = previewOf(t, env(d), Source{Kind: SourceDuckDB, Path: p, SQL: "DELETE FROM ventes"}); pv.Error == "" {
		t.Fatal("écriture acceptée")
	}
}
