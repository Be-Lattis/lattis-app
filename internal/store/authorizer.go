package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"github.com/ncruces/go-sqlite3"
)

// Autorisateur des requêtes de l'utilisateur (sqlite3_set_authorizer).
//
// Le SQL écrit par l'utilisateur, ou rangé dans un classeur reçu (onglet
// SQL, db.query et db.execute des scripts, des pages HTML et du serveur
// MCP, requêtes liées), passe par une connexion dont l'autorisateur
// refuse, au moment de la préparation de chaque instruction :
//   - ATTACH et DETACH (créer ou modifier un fichier SQLite n'importe où
//     sur le disque), les transactions et points de sauvegarde ;
//   - tout changement de structure (CREATE, DROP, ALTER, tables
//     virtuelles, ANALYZE, REINDEX), temporaire compris : un déclencheur
//     temporaire resterait sur la connexion et s'exécuterait plus tard au
//     nom du moteur ;
//   - les tables internes _lattis_* (journal, accords, mots de passe des
//     feuilles…), les tables de travail _slt_* et les tables sqlite_*
//     (hormis la lecture de sqlite_schema) ;
//   - en lecture, le schéma temp et toute écriture ; en écriture, le schéma
//     temp sauf les vues aux noms affichés que le moteur y pose ;
//   - les PRAGMA, sauf ceux qui décrivent les tables (table_info…) ;
//   - load_extension et fts3_tokenizer.
//
// Ce que le moteur exécute lui-même (déclencheurs et vues qu'il a créés,
// expressions des colonnes calculées) est vérifié avec le nom du
// déclencheur ou de la vue (inner) et reste permis. Le moteur garde un
// accès complet : l'autorisateur n'est posé que le temps des requêtes de
// l'utilisateur (UserConn, Store.AsUser).

// readPragmas sont les PRAGMA qui ne font que décrire le schéma.
var readPragmas = map[string]bool{
	"table_info": true, "table_xinfo": true, "table_list": true,
	"index_list": true, "index_info": true, "index_xinfo": true,
	"foreign_key_list": true,
}

// deniedFuncs sont les fonctions SQL refusées à l'utilisateur.
var deniedFuncs = map[string]bool{"load_extension": true, "fts3_tokenizer": true}

// userAuthorizer rend l'autorisateur des requêtes de l'utilisateur ; write :
// requête d'écriture (INSERT, UPDATE, DELETE permis sur les tableaux).
func userAuthorizer(write bool) func(sqlite3.AuthorizerActionCode, string, string, string, string) sqlite3.AuthorizerReturnCode {
	return func(action sqlite3.AuthorizerActionCode, name3, name4, schema, inner string) sqlite3.AuthorizerReturnCode {
		switch action {
		case sqlite3.AUTH_SELECT, sqlite3.AUTH_RECURSIVE:
			return sqlite3.AUTH_OK
		case sqlite3.AUTH_FUNCTION:
			if deniedFuncs[strings.ToLower(name4)] {
				return sqlite3.AUTH_DENY
			}
			return sqlite3.AUTH_OK
		case sqlite3.AUTH_PRAGMA:
			if readPragmas[strings.ToLower(name3)] {
				return sqlite3.AUTH_OK
			}
			return sqlite3.AUTH_DENY
		case sqlite3.AUTH_READ:
			if inner != "" {
				return sqlite3.AUTH_OK
			}
			return userTable(name3, schema, write, false)
		case sqlite3.AUTH_INSERT, sqlite3.AUTH_UPDATE, sqlite3.AUTH_DELETE:
			if !write {
				return sqlite3.AUTH_DENY
			}
			if inner != "" {
				return sqlite3.AUTH_OK
			}
			return userTable(name3, schema, write, true)
		}
		// ATTACH, DETACH, CREATE, DROP, ALTER, transactions, points de
		// sauvegarde, ANALYZE, REINDEX, tables virtuelles…
		return sqlite3.AUTH_DENY
	}
}

// userTable dit si l'utilisateur peut lire (ou écrire, modify) la table
// name du schéma schema.
func userTable(name, schema string, write, modify bool) sqlite3.AuthorizerReturnCode {
	n := strings.ToLower(name)
	switch {
	case strings.HasPrefix(n, "_lattis_"), strings.HasPrefix(n, "_slt_"):
		return sqlite3.AUTH_DENY
	case strings.HasPrefix(n, "sqlite_"):
		if !modify && (n == "sqlite_schema" || n == "sqlite_master") && schema != "temp" {
			return sqlite3.AUTH_OK
		}
		return sqlite3.AUTH_DENY
	}
	switch strings.ToLower(schema) {
	case "main", "":
		return sqlite3.AUTH_OK
	case "temp":
		// Écriture : les vues aux noms affichés posées par le moteur
		// (canvas.ExecSQL) ; lecture : aucun objet temporaire.
		if write {
			return sqlite3.AUTH_OK
		}
	}
	return sqlite3.AUTH_DENY
}

type rawConn interface{ Raw() *sqlite3.Conn }

func setUserAuth(c *sql.Conn, write bool) error {
	return c.Raw(func(dc any) error {
		r, ok := dc.(rawConn)
		if !ok {
			return fmt.Errorf("store : connexion SQLite attendue (%T)", dc)
		}
		return r.Raw().SetAuthorizer(userAuthorizer(write))
	})
}

// UserConn réserve une connexion du pool de lecture db pour des requêtes de
// l'utilisateur, sous l'autorisateur de lecture. ReleaseUserConn la rend
// au pool (après avoir fermé ses résultats).
func UserConn(ctx context.Context, db *sql.DB) (*sql.Conn, error) {
	c, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if err := setUserAuth(c, false); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// ReleaseUserConn retire l'autorisateur et rend la connexion au pool. Si
// l'autorisateur ne peut pas être retiré, la connexion est jetée.
func ReleaseUserConn(c *sql.Conn) {
	if c == nil {
		return
	}
	err := c.Raw(func(dc any) error {
		r, ok := dc.(rawConn)
		if !ok {
			return errors.New("connexion inattendue")
		}
		return r.Raw().SetAuthorizer(nil)
	})
	if err != nil {
		// driver.ErrBadConn : database/sql ferme la connexion au lieu de la
		// rendre au pool.
		c.Raw(func(any) error { return driver.ErrBadConn })
	}
	c.Close()
}

// AsUser exécute fn, appelée pendant une écriture (Store.Write), sous
// l'autorisateur d'écriture : les instructions préparées dans fn sur la
// connexion d'écriture sont celles de l'utilisateur.
func (s *Store) AsUser(fn func() error) error {
	c := s.writeConn.Load()
	if c == nil {
		return errors.New("store : connexion d'écriture absente")
	}
	if err := c.SetAuthorizer(userAuthorizer(true)); err != nil {
		return err
	}
	defer c.SetAuthorizer(nil)
	return fn()
}

// AuthDenied dit si err vient d'une instruction refusée par l'autorisateur.
func AuthDenied(err error) bool {
	return err != nil && (errors.Is(err, sqlite3.AUTH) || strings.Contains(err.Error(), "not authorized"))
}
