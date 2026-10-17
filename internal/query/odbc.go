package query

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// ODBC générique : pilote alexbrainman/odbc. Sous Windows il appelle le
// gestionnaire de pilotes ODBC par des appels système (pas de CGO) ;
// ailleurs il demande CGO et unixODBC : il n'est compilé qu'avec
// -tags odbc (voir odbc_driver.go). La chaîne de connexion est celle du
// pilote (DSN=Ventes, ou Driver={…};Server=…) ; le mot de passe n'y est
// jamais enregistré, il est ajouté (PWD=) à l'ouverture. Rien ne garantit la
// lecture seule côté pilote : la requête est exécutée dans une transaction
// toujours annulée, et seules les requêtes de lecture sont acceptées.

func init() {
	dialects[KindODBC] = &dialect{
		label:      "ODBC",
		open:       openODBC,
		authFailed: odbcAuthFailed,
		tables:     odbcTables,
		ident:      quoteANSI,
	}
}

// errNoODBC : pilote ODBC absent de cette version.
var errNoODBC = errors.New("ODBC n'est pas disponible dans cette version de Slitable (sous Linux et macOS, compiler avec -tags odbc et unixODBC)")

// odbcValue écrit une valeur d'une chaîne de connexion ODBC, entre
// accolades si besoin.
func odbcValue(s string) string {
	if strings.ContainsAny(s, ";{}=") || strings.TrimSpace(s) != s {
		return "{" + strings.ReplaceAll(s, "}", "}}") + "}"
	}
	return s
}

// odbcAttrs découpe une chaîne de connexion en paires clé=valeur (les
// valeurs entre accolades peuvent contenir des points-virgules).
func odbcAttrs(s string) [][2]string {
	var out [][2]string
	for len(s) > 0 {
		s = strings.TrimLeft(s, "; \t")
		if s == "" {
			break
		}
		eq := strings.IndexByte(s, '=')
		if eq < 0 {
			out = append(out, [2]string{strings.TrimSpace(s), ""})
			break
		}
		key := strings.TrimSpace(s[:eq])
		s = s[eq+1:]
		var val string
		if t := strings.TrimLeft(s, " "); strings.HasPrefix(t, "{") {
			t = t[1:]
			var b strings.Builder
			for len(t) > 0 {
				if t[0] == '}' {
					if len(t) > 1 && t[1] == '}' {
						b.WriteByte('}')
						t = t[2:]
						continue
					}
					t = t[1:]
					break
				}
				b.WriteByte(t[0])
				t = t[1:]
			}
			val = b.String()
			if i := strings.IndexByte(t, ';'); i >= 0 {
				s = t[i+1:]
			} else {
				s = ""
			}
		} else if i := strings.IndexByte(s, ';'); i >= 0 {
			val, s = strings.TrimSpace(s[:i]), s[i+1:]
		} else {
			val, s = strings.TrimSpace(s), ""
		}
		out = append(out, [2]string{key, val})
	}
	return out
}

// odbcHasPassword : la chaîne de connexion contient un mot de passe.
func odbcHasPassword(s string) bool {
	for _, a := range odbcAttrs(s) {
		if k := strings.ToUpper(a[0]); k == "PWD" || k == "PASSWORD" {
			return true
		}
	}
	return false
}

// odbcConnString complète la chaîne de connexion avec l'utilisateur et le
// mot de passe (s'ils ne sont pas vides).
func odbcConnString(c DBConfig, password string) string {
	s := strings.TrimRight(strings.TrimSpace(c.ODBC), ";")
	if c.User != "" {
		s += ";UID=" + odbcValue(c.User)
	}
	if password != "" {
		s += ";PWD=" + odbcValue(password)
	}
	return s
}

func openODBC(c DBConfig, password string) (*sql.DB, error) {
	if !ODBCAvailable {
		return nil, errNoODBC
	}
	db, err := sql.Open("odbc", odbcConnString(c, password))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// odbcAuthFailed reconnaît un refus d'authentification (SQLSTATE 28000).
func odbcAuthFailed(err error) bool {
	s := err.Error()
	return strings.Contains(s, "28000") || strings.Contains(s, "Login failed") || strings.Contains(s, "password authentication failed") || strings.Contains(s, "Access denied")
}

// odbcTables essaie INFORMATION_SCHEMA (la plupart des SGBD) ; sans lui,
// pas de navigateur (requête SQL seulement).
func odbcTables(ctx context.Context, db *sql.DB) ([]DBTable, error) {
	ts, err := infoSchemaTables(`SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM INFORMATION_SCHEMA.TABLES`)(ctx, db)
	if err != nil {
		return []DBTable{}, nil
	}
	out := ts[:0]
	for _, t := range ts {
		switch strings.ToLower(t.Schema) {
		case "information_schema", "pg_catalog", "sys", "mysql", "performance_schema":
			continue
		}
		out = append(out, t)
	}
	sortTables(out, "")
	return out, nil
}
