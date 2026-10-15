//go:build modernc

package main

import (
	"database/sql"
	"database/sql/driver"

	"modernc.org/sqlite"
)

const driverName = "modernc"

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("slt_div", 2, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		if v, ok := sltDiv(toFloat(args[0]), toFloat(args[1])); ok {
			return v, nil
		}
		return nil, nil
	})
}

func toFloat(v driver.Value) float64 {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case float64:
		return x
	}
	return 0
}

func openDB(path string) (*sql.DB, error) { return sql.Open("sqlite", "file:"+path) }
