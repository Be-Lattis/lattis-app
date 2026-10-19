package query

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Accès des sources hors du classeur : hôtes joints (API web, bases de
// données, Serveur Lattis) et fichiers lus. Une requête est écrite dans le
// classeur : un classeur reçu ne doit pas, à l'actualisation, joindre un
// serveur tiers ou importer un fichier du poste sans que l'utilisateur
// l'ait voulu. Env.Allow reçoit chaque accès avant qu'il ait lieu (accord
// par hôte et par fichier, comme les scripts et les pages : voir
// api.AllowSource).

// Access est un accès d'une source : Host (« hôte » ou « hôte:port ») ou
// Path (fichier).
type Access struct {
	Host string
	Path string
}

// ErrDenied refuse un accès.
var ErrDenied = errors.New("accès refusé")

// AllowFunc vérifie un accès avant qu'il ait lieu.
type AllowFunc func(ctx context.Context, a Access) error

// SourceAccess donne l'accès hors du classeur d'une source lue avec la
// connexion conns(nom) ; ok est faux pour une source interne au classeur.
func SourceAccess(src Source, conns func(name string) (Connection, error)) (a Access, ok bool, err error) {
	switch src.Kind {
	case SourceCSV, SourceXLSX, SourceJSON, SourceParquet, SourceSQLite:
		if strings.TrimSpace(src.Path) == "" {
			return Access{}, false, nil // erreur « fichier non choisi » du chargeur
		}
		return Access{Path: src.Path}, true, nil
	case SourceWeb:
		u, err := webURL(src.URL)
		if err != nil {
			return Access{}, false, err
		}
		return Access{Host: u.Host}, true, nil
	case SourcePostgres, SourceMySQL, SourceSQLServer, SourceLattis:
		conn, err := conns(src.Connection)
		if err != nil {
			return Access{}, false, err
		}
		h := ConnectionHost(conn)
		return Access{Host: h}, h != "", nil
	}
	return Access{}, false, nil
}

// ConnectionHost donne l'hôte (et le port) qu'une connexion joint.
func ConnectionHost(c Connection) string {
	switch c.Kind {
	case KindWeb, KindLattis:
		if u, err := url.Parse(strings.TrimSpace(c.Config.URL)); err == nil {
			return u.Host
		}
		return ""
	}
	h := strings.TrimSpace(c.Config.Host)
	if h == "" {
		return ""
	}
	if c.Kind == "" {
		c.Kind = KindPostgres
	}
	return net.JoinHostPort(h, strconv.Itoa(c.Config.port(c.Kind)))
}

// allow vérifie un accès (Env.Allow).
func (r *run) allow(a Access) error {
	if r.env.Allow != nil {
		return r.env.Allow(r.ctx, a)
	}
	return nil
}
