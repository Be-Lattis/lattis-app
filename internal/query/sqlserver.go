package query

import (
	"database/sql"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	mssql "github.com/microsoft/go-mssqldb"
)

// SQL Server : pilote microsoft/go-mssqldb (Go pur ; sans utilisateur,
// authentification Windows intégrée sous Windows). SQL Server n'a pas de
// transaction en lecture seule : la requête est exécutée dans une
// transaction toujours annulée, et seules les requêtes de lecture (SELECT,
// WITH) sont acceptées.

func init() {
	dialects[KindSQLServer] = &dialect{
		label:      "SQL Server",
		open:       openSQLServer,
		authFailed: sqlServerAuthFailed,
		limit:      sqlServerLimit,
		tables: infoSchemaTables(`SELECT TOP 2000 TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE
			FROM INFORMATION_SCHEMA.TABLES
			WHERE TABLE_SCHEMA NOT IN ('sys', 'INFORMATION_SCHEMA')
			ORDER BY CASE WHEN TABLE_SCHEMA = 'dbo' THEN 0 ELSE 1 END, TABLE_SCHEMA, TABLE_NAME`),
		ident: func(s string) string { return "[" + strings.ReplaceAll(s, "]", "]]") + "]" },
		value: sqlServerValue,
	}
}

// sqlServerDSN écrit l'adresse de connexion (sqlserver://…).
func sqlServerDSN(c DBConfig, password string) string {
	u := url.URL{Scheme: "sqlserver", Host: c.Host}
	if c.User != "" {
		u.User = url.UserPassword(c.User, password)
	}
	if c.Instance != "" {
		u.Path = c.Instance
		if c.Port != 0 {
			u.Host = net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
		}
	} else {
		u.Host = net.JoinHostPort(c.Host, strconv.Itoa(c.port(KindSQLServer)))
	}
	q := url.Values{}
	if c.Database != "" {
		q.Set("database", c.Database)
	}
	q.Set("app name", "Lattis")
	q.Set("connection timeout", "10")
	switch c.SSLMode {
	case "disable":
		q.Set("encrypt", "disable")
	case "require":
		q.Set("encrypt", "true")
		q.Set("TrustServerCertificate", "true")
	case "verify-full", "verify-ca":
		q.Set("encrypt", "true")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func openSQLServer(c DBConfig, password string, dial DialFunc) (*sql.DB, error) {
	conn, err := mssql.NewConnector(sqlServerDSN(c, password))
	if err != nil {
		return nil, err
	}
	if dial != nil {
		// Le pilote résout le nom lui-même : Dial reçoit l'adresse IP.
		conn.Dialer = dialer(dial)
	}
	db := sql.OpenDB(conn)
	db.SetMaxOpenConns(1)
	return db, nil
}

// sqlServerAuthFailed reconnaît un refus d'authentification (erreur
// 18456 : Login failed for user).
func sqlServerAuthFailed(err error) bool {
	var me mssql.Error
	if errors.As(err, &me) {
		return me.Number == 18456
	}
	return strings.Contains(err.Error(), "Login failed for user")
}

// sqlServerLimit : SELECT TOP (n) * FROM (…). Une requête WITH ou triée
// (ORDER BY interdit dans une sous-requête sans TOP) n'est pas enveloppée :
// la lecture s'arrête alors après n lignes.
func sqlServerLimit(q string, n int) string {
	if firstKeyword(q) != "SELECT" || strings.Contains(strings.ToUpper(q), "ORDER BY") {
		return q
	}
	return "SELECT TOP (" + strconv.Itoa(n) + ") * FROM (" + q + "\n) AS _lattis_apercu"
}

// sqlServerValue écrit les identifiants uniques (rendus en octets dans
// l'ordre de SQL Server) sous leur forme habituelle.
func sqlServerValue(typeName string, v any) any {
	if b, ok := v.([]byte); ok && strings.EqualFold(typeName, "UNIQUEIDENTIFIER") && len(b) == 16 {
		var u mssql.UniqueIdentifier
		if u.Scan(b) == nil {
			return u.String()
		}
	}
	return v
}
