// Package pgpool ouvre les connexions PostgreSQL d'un Serveur Lattis sous
// l'identité de l'utilisateur : authentification OAUTHBEARER (RFC 7628) avec
// le jeton d'accès de lattisd, que pgx (v5.11+) prend en charge par
// pgconn.Config.OAuthTokenProvider ; un pool d'écriture sur le point rw,
// un pool de lecture sur les points ro quand le serveur en annonce ;
// application_name = Lattis/<version>/<usage>, qui alimente les mesures du
// serveur (spécification de l'application, §4.4 ; du serveur, §12 et §13).
//
// Le fournisseur de jeton est appelé à chaque nouvelle connexion physique :
// une reconnexion après l'expiration du jeton d'accès en obtient un neuf
// (auth.Manager.Token le renouvelle avant son expiration).
package pgpool

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/lalamefine/lattis/internal/msg"
)

// Usages des connexions (étiquette kind de lattis_query_duration_seconds).
const (
	UsageInitialLoad = "initial_load"
	UsagePivot       = "pivot"
	UsageSQL         = "sql"
	UsagePowerQuery  = "power_query"
	UsageDelta       = "delta"
	UsageWrite       = "write"
)

// ReadUsage : usage servi par le pool de lecture (réplicas) quand il
// existe ; écritures, LISTEN et deltas restent sur rw (serveur, §12).
func ReadUsage(usage string) bool {
	switch usage {
	case UsageInitialLoad, UsagePivot, UsageSQL, UsagePowerQuery:
		return true
	}
	return false
}

// AppName écrit l'application_name d'un usage (63 octets au plus, limite
// de PostgreSQL).
func AppName(version, usage string) string {
	if version == "" {
		version = "dev"
	}
	s := "Lattis/" + version + "/" + usage
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}

// TokenFunc donne un jeton d'accès valable.
type TokenFunc func(ctx context.Context) (string, error)

// Target décrit un serveur PostgreSQL annoncé par la découverte.
type Target struct {
	// RW : hôte du parent (hôte ou hôte:port).
	RW string
	// RO : réplicas (vide : pas de pool de lecture).
	RO       []string
	Port     int
	Database string
	// SSLMode : verify-full par défaut (TLS obligatoire, serveur §18).
	SSLMode string
	// User : rôle PostgreSQL de l'utilisateur (revendication pg_role).
	User    string
	Token   TokenFunc
	Version string
	// Device : identifiant stable du poste (config.DeviceID), posé sur
	// chaque connexion du pool rw (SET lattis.device, contrat client §6) :
	// l'auteur d'une écriture est le rôle et le poste, sinon deux postes du
	// même utilisateur passent pour un seul et l'un ne reçoit pas ce que
	// l'autre a écrit (échos). Vide : non posé.
	Device string
}

// DeviceSQL écrit l'instruction qui pose l'identifiant du poste.
func DeviceSQL(device string) string { return "SET lattis.device = " + literal(device) }

// hostsPorts sépare une liste d'hôtes (hôte ou hôte:port, IPv6 entre
// crochets) en listes host= et port= de libpq.
func hostsPorts(eps []string, def int) (string, string, error) {
	if def == 0 {
		def = 5432
	}
	var hs, ps []string
	for _, ep := range eps {
		ep = strings.TrimSpace(ep)
		if ep == "" {
			continue
		}
		host, port := ep, def
		if h, p, err := net.SplitHostPort(ep); err == nil {
			n, err := strconv.Atoi(p)
			if err != nil {
				return "", "", msg.New("pgpool.invalidEndpoint", msg.Args{"endpoint": fmt.Sprintf("%q", ep)})
			}
			host, port = h, n
		}
		if strings.ContainsAny(host, ", '\\") {
			return "", "", msg.New("pgpool.invalidEndpoint", msg.Args{"endpoint": fmt.Sprintf("%q", ep)})
		}
		hs = append(hs, host)
		ps = append(ps, strconv.Itoa(port))
	}
	if len(hs) == 0 {
		return "", "", msg.New("pgpool.noEndpoint", nil)
	}
	return strings.Join(hs, ","), strings.Join(ps, ","), nil
}

func quote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// dsn écrit la chaîne de connexion d'une liste de points (essayés dans
// l'ordre : pgx passe au suivant si l'un ne répond pas).
func (t Target) dsn(eps []string, usage string) (string, error) {
	if t.User == "" {
		return "", msg.New("pgpool.unknownRole", nil)
	}
	if t.Token == nil {
		return "", msg.New("pgpool.noToken", nil)
	}
	hosts, ports, err := hostsPorts(eps, t.Port)
	if err != nil {
		return "", err
	}
	db := t.Database
	if db == "" {
		db = "lattis"
	}
	ssl := t.SSLMode
	if ssl == "" {
		ssl = "verify-full"
	}
	return strings.Join([]string{
		"host=" + hosts, "port=" + ports,
		"dbname=" + quote(db),
		"sslmode=" + quote(ssl), "connect_timeout=10",
		"application_name=" + quote(AppName(t.Version, usage)),
	}, " "), nil
}

// ConnConfig prépare la configuration pgx d'une connexion (OAUTHBEARER).
func (t Target) ConnConfig(eps []string, usage string) (*pgx.ConnConfig, error) {
	dsn, err := t.dsn(eps, usage)
	if err != nil {
		return nil, err
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	setToken(cfg, t.User, t.Token)
	return cfg, nil
}

// setToken branche l'utilisateur et le jeton (les points de secours,
// Fallbacks, partagent la configuration). Le rôle contient souvent « @ » :
// il va dans cfg.User, jamais dans une chaîne de connexion (contrat de
// lattisd, §3).
func setToken(cfg *pgx.ConnConfig, user string, tok TokenFunc) {
	cfg.User = user
	cfg.OAuthTokenProvider = tok
}

// OpenDB ouvre une base database/sql (une connexion, ouverte à la
// demande) pour un usage : sur ro quand il y en a et que l'usage est une
// lecture, sur rw sinon. Sert aux requêtes d'import et au navigateur de
// tables.
func (t Target) OpenDB(usage string) (*sql.DB, error) {
	cfg, err := t.ConnConfig(t.endpoints(usage), usage)
	if err != nil {
		return nil, err
	}
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(1)
	return db, nil
}

func (t Target) endpoints(usage string) []string {
	if ReadUsage(usage) && len(t.RO) > 0 {
		return t.RO
	}
	return []string{t.RW}
}

// Pools réunit le pool d'écriture (rw) et, si le serveur annonce des
// réplicas, le pool de lecture (ro).
type Pools struct {
	RW *pgxpool.Pool
	RO *pgxpool.Pool // nil : lectures sur RW
	t  Target
}

// Open crée les pools (connexions ouvertes à la demande).
func Open(ctx context.Context, t Target) (*Pools, error) {
	rw, err := t.pool(ctx, []string{t.RW}, UsageWrite, 8)
	if err != nil {
		return nil, err
	}
	p := &Pools{RW: rw, t: t}
	if len(t.RO) > 0 {
		if p.RO, err = t.pool(ctx, t.RO, UsageInitialLoad, 4); err != nil {
			rw.Close()
			return nil, err
		}
	}
	return p, nil
}

func (t Target) pool(ctx context.Context, eps []string, usage string, max int32) (*pgxpool.Pool, error) {
	dsn, err := t.dsn(eps, usage)
	if err != nil {
		return nil, err
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	setToken(cfg.ConnConfig, t.User, t.Token)
	cfg.MaxConns = max
	cfg.MinConns = 0
	if usage == UsageWrite && t.Device != "" {
		// Pool rw : écritures, deltas et LISTEN.
		q := DeviceSQL(t.Device)
		cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
			_, err := c.Exec(ctx, q)
			return err
		}
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

// Acquire prend une connexion pour un usage (pool de lecture pour les
// lectures quand il existe) et pose son application_name.
func (p *Pools) Acquire(ctx context.Context, usage string) (*pgxpool.Conn, error) {
	pool := p.RW
	if ReadUsage(usage) && p.RO != nil {
		pool = p.RO
	}
	c, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	want := AppName(p.t.Version, usage)
	// application_name est un paramètre signalé par le serveur
	// (ParameterStatus) : pgx connaît sa valeur courante.
	if c.Conn().PgConn().ParameterStatus("application_name") != want {
		if _, err := c.Exec(ctx, "SET application_name = "+literal(want)); err != nil {
			c.Release()
			return nil, err
		}
	}
	return c, nil
}

// literal écrit une chaîne SQL (standard_conforming_strings).
func literal(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// Close ferme les pools.
func (p *Pools) Close() {
	p.RW.Close()
	if p.RO != nil {
		p.RO.Close()
	}
}

// AuthFailed reconnaît un refus d'authentification de PostgreSQL (jeton
// expiré, autre rôle, compte désactivé) : rafraîchir le jeton puis
// réessayer une fois (contrat de lattisd, §3).
func AuthFailed(err error) bool {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code == "28000" || pe.Code == "28P01"
	}
	return err != nil && strings.Contains(err.Error(), "OAuth authentication failed")
}
