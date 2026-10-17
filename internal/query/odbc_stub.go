//go:build !((windows && (amd64 || 386)) || (odbc && cgo))

package query

// ODBCAvailable : le pilote ODBC est compilé dans cette version (ici non,
// voir odbc_driver.go).
const ODBCAvailable = false
