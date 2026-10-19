//go:build mattn

package main

import (
	"database/sql"

	"github.com/mattn/go-sqlite3"
)

const driverName = "mattn"

func init() {
	sql.Register("sqlite3_slt", &sqlite3.SQLiteDriver{
		ConnectHook: func(c *sqlite3.SQLiteConn) error {
			return c.RegisterFunc("slt_div", func(a, b any) any {
				if v, ok := sltDiv(toFloat(a), toFloat(b)); ok {
					return v
				}
				return nil
			}, true)
		},
	})
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case float64:
		return x
	}
	return 0
}

func openDB(path string) (*sql.DB, error) { return sql.Open("sqlite3_slt", "file:"+path) }
