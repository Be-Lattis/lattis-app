package query

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
)

// PostgreSQL : pilote pgx (Go pur). Lecture seule garantie par une
// transaction READ ONLY ; la limite de l'aperçu est poussée au serveur.

func init() {
	dialects[KindPostgres] = &dialect{
		label:      "PostgreSQL",
		open:       openPG,
		authFailed: pgAuthFailed,
		limit:      limitClause,
		readOnly:   true,
		tables: infoSchemaTables(`SELECT table_schema, table_name, table_type
			FROM information_schema.tables
			WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
			ORDER BY table_schema = 'public' DESC, table_schema, table_name LIMIT 2000`),
		ident: PGIdent,
	}
}

// dsnValue écrit une valeur du format clé='valeur' de libpq.
func dsnValue(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// pgDSN écrit la chaîne de connexion libpq. Un mot de passe vide laisse
// pgx chercher PGPASSWORD et le fichier .pgpass.
func pgDSN(c DBConfig, password string) string {
	parts := []string{
		"host=" + dsnValue(c.Host),
		"port=" + strconv.Itoa(c.port(KindPostgres)),
		"dbname=" + dsnValue(c.Database),
		"connect_timeout=10",
		"application_name=Lattis",
	}
	if c.User != "" {
		parts = append(parts, "user="+dsnValue(c.User))
	}
	if c.SSLMode != "" {
		parts = append(parts, "sslmode="+dsnValue(c.SSLMode))
	}
	if password != "" {
		parts = append(parts, "password="+dsnValue(password))
	}
	return strings.Join(parts, " ")
}

// openPG ouvre une connexion (sans la tester).
func openPG(c DBConfig, password string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(pgDSN(c, password))
	if err != nil {
		return nil, err
	}
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(1)
	return db, nil
}

// pgAuthFailed reconnaît un refus d'authentification.
func pgAuthFailed(err error) bool {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code == "28P01" || pe.Code == "28000"
	}
	return strings.Contains(err.Error(), "password authentication failed") || strings.Contains(err.Error(), "no password")
}

// PGIdent écrit un identifiant PostgreSQL entre guillemets si besoin.
func PGIdent(s string) string {
	simple := s != ""
	for i, r := range s {
		if !(r == '_' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			simple = false
		}
	}
	if simple {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
