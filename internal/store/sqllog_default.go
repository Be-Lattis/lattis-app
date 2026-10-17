//go:build !production

package store

// defaultSQLLog : hors version livrée (go run, wails dev), le journal SQL
// est actif par défaut.
const defaultSQLLog = true
