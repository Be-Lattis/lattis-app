package query

import (
	"database/sql"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// MySQL et MariaDB : pilote go-sql-driver/mysql (Go pur). Lecture seule
// garantie par START TRANSACTION READ ONLY ; la limite de l'aperçu est
// poussée au serveur.

func init() {
	dialects[KindMySQL] = &dialect{
		label:      "MySQL",
		open:       openMySQL,
		authFailed: mysqlAuthFailed,
		limit:      limitClause,
		readOnly:   true,
		tables: infoSchemaTables(`SELECT table_schema, table_name, table_type
			FROM information_schema.tables
			WHERE table_schema NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')
			ORDER BY table_schema = DATABASE() DESC, table_schema, table_name LIMIT 2000`),
		ident: func(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" },
	}
}

// mysqlConfig prépare la configuration du pilote.
func mysqlConfig(c DBConfig, password string) *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = c.User
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, strconv.Itoa(c.port(KindMySQL)))
	cfg.DBName = c.Database
	cfg.ParseTime = true
	cfg.Loc = time.Local
	cfg.Timeout = 10 * time.Second
	cfg.AllowNativePasswords = true
	// Jeu de caractères par l'option du pilote (SET NAMES) : dans Params, il
	// partirait en variable système (« Unknown system variable 'charset' »,
	// MariaDB, issue n°20).
	_ = cfg.Apply(mysql.Charset("utf8mb4", ""))
	switch c.SSLMode {
	case "disable":
		cfg.TLSConfig = "false"
	case "require":
		cfg.TLSConfig = "skip-verify"
	case "verify-full", "verify-ca":
		cfg.TLSConfig = "true"
	default:
		cfg.TLSConfig = "preferred"
	}
	return cfg
}

// mysqlDSN écrit la chaîne de connexion (tests, messages).
func mysqlDSN(c DBConfig, password string) string { return mysqlConfig(c, password).FormatDSN() }

func openMySQL(c DBConfig, password string) (*sql.DB, error) {
	conn, err := mysql.NewConnector(mysqlConfig(c, password))
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(conn)
	db.SetMaxOpenConns(1)
	return db, nil
}

// mysqlAuthFailed reconnaît un refus d'authentification (erreur 1045 :
// Access denied for user).
func mysqlAuthFailed(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1045
	}
	return strings.Contains(err.Error(), "Access denied for user")
}
