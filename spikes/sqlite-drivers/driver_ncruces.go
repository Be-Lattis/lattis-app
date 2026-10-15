//go:build ncruces

package main

import (
	"database/sql"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
)

const driverName = "ncruces"

func openDB(path string) (*sql.DB, error) {
	return driver.Open("file:"+path, func(c *sqlite3.Conn) error {
		return c.CreateFunction("slt_div", 2, sqlite3.DETERMINISTIC, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
			if v, ok := sltDiv(arg[0].Float(), arg[1].Float()); ok {
				ctx.ResultFloat(v)
			}
		})
	})
}
