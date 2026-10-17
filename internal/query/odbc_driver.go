//go:build (windows && (amd64 || 386)) || (odbc && cgo)

package query

// Pilote ODBC : appels système sous Windows, CGO et unixODBC ailleurs
// (-tags odbc).
import _ "github.com/alexbrainman/odbc"

// ODBCAvailable : le pilote ODBC est compilé dans cette version.
const ODBCAvailable = true
