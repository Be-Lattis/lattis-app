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
		tables:     pgTables,
		ident:      PGIdent,
	}
}

// pgTables liste les tables et vues d'une base PostgreSQL.
var pgTables = infoSchemaTables(pgTablesSQL(nil, "public"))

// pgTablesSQL écrit la requête du navigateur des tables : tables
// (ordinaires, partitionnées, étrangères), vues et vues matérialisées hors
// schémas système (pg_catalog, information_schema, pg_toast, pg_temp_*… :
// PostgreSQL réserve le préfixe pg_ à ses schémas) et hors exclude, celles
// que l'utilisateur peut lire seulement (USAGE sur le schéma et SELECT sur
// la table) : le catalogue montre les noms de tout ce qui existe, même
// illisible. Le schéma first vient en tête. Les noms passés sont des
// constantes du programme.
func pgTablesSQL(exclude []string, first string) string {
	skip := []string{"'information_schema'"}
	for _, s := range exclude {
		skip = append(skip, "'"+strings.ReplaceAll(s, "'", "''")+"'")
	}
	return `SELECT n.nspname, c.relname,
		CASE WHEN c.relkind IN ('v', 'm') THEN 'VIEW' ELSE 'BASE TABLE' END
	FROM pg_catalog.pg_class c
	JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
	WHERE c.relkind IN ('r', 'p', 'f', 'v', 'm')
		AND left(n.nspname, 3) <> 'pg_'
		AND n.nspname NOT IN (` + strings.Join(skip, ", ") + `)
		AND has_schema_privilege(n.oid, 'USAGE')
		AND has_table_privilege(c.oid, 'SELECT')
	ORDER BY n.nspname = '` + strings.ReplaceAll(first, "'", "''") + `' DESC, n.nspname, c.relname LIMIT 2000`
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
