package bridge

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"

	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/pgpool"
)

// Serveur Lattis (classeurs partagés, docs/collaboration/connexion.md) :
// découverte, page de connexion, menu Compte. Tout passe par
// internal/remote/auth ; rien ici ne dépend de la fenêtre (le navigateur du
// système est ouvert par auth.OpenBrowser). Fonctionnalité à activer
// (Fichier › Paramètres › Fonctionnalités, features.sharedWorkbooks).

// lattisManager tient les serveurs et les sessions de l'application.
var lattisManager = sync.OnceValue(func() *auth.Manager {
	index := ""
	if dir, err := os.UserConfigDir(); err == nil {
		index = filepath.Join(dir, "Lattis", "serveurs-lattis.json")
	}
	return auth.NewManager(auth.Options{
		Store:         query.LattisTokens(query.DefaultSecrets),
		IndexPath:     index,
		ClientVersion: Version,
		Policy:        lattisPolicy,
	})
})

func init() {
	query.LattisOpen = lattisOpen
}

// lattisPolicy lit la stratégie courante (serveurs gérés, verrouillage).
func lattisPolicy() auth.Policy {
	c := config.Current()
	p := auth.Policy{AllowUserServers: config.AllowUserLattisServers(c)}
	for _, s := range config.LattisServers(c) {
		p.Managed = append(p.Managed, auth.Managed{URL: s.URL, Name: s.Name, Provider: s.Provider})
	}
	return p
}

var errSharedOff = msg.New("share.disabled", nil)

func lattisOn() (*auth.Manager, error) {
	if !config.SharedWorkbooks(config.Current()) {
		return nil, errSharedOff
	}
	return lattisManager(), nil
}

// lattisOpen ouvre la base d'un serveur sous l'identité de l'utilisateur
// (query.LattisOpen).
func lattisOpen(ctx context.Context, server, usage string) (*sql.DB, error) {
	m, err := sessions()
	if err != nil {
		return nil, err
	}
	user, disc, err := m.Session(ctx, server)
	if err != nil {
		return nil, err
	}
	// Jeton obtenu tout de suite : une session expirée se signale avant
	// toute connexion PostgreSQL.
	if _, err := m.Token(ctx, server); err != nil {
		return nil, err
	}
	t := lattisTarget(m, server, user, disc)
	db, err := t.OpenDB(usage)
	if err != nil {
		return nil, err
	}
	// Jeton refusé par PostgreSQL : rafraîchi, puis un seul nouvel essai
	// (contrat de lattisd, §3).
	if err := db.PingContext(ctx); pgpool.AuthFailed(err) {
		db.Close()
		m.Invalidate(server)
		return t.OpenDB(usage)
	}
	return db, nil
}

// lattisTarget décrit la base du serveur pour pgpool (OAUTHBEARER).
func lattisTarget(m sessionSource, server string, user auth.User, disc *auth.Discovery) pgpool.Target {
	pg := disc.Postgres
	return pgpool.Target{
		RW: string(pg.RW), RO: auth.Strings(pg.RO), Port: pg.Port, Database: pg.Database, SSLMode: pg.SSLMode,
		User:    user.PGRole,
		Token:   func(ctx context.Context) (string, error) { return m.Token(ctx, server) },
		Version: Version,
		Device:  config.DeviceID(),
	}
}

// LattisState est l'état des Serveurs Lattis pour l'interface.
type LattisState struct {
	// Enabled : classeurs partagés activés (sinon tout est masqué).
	Enabled bool `json:"enabled"`
	// AllowUserServers : l'utilisateur peut ajouter un serveur.
	AllowUserServers bool               `json:"allowUserServers"`
	Servers          []auth.ServerState `json:"servers"`
}

// LattisState donne les serveurs connus (ceux de l'organisation, ceux où
// l'utilisateur s'est connecté) et l'utilisateur connecté à chacun.
func (s *DocumentService) LattisState() LattisState {
	if fixed != nil {
		return fixed.state()
	}
	m, err := lattisOn()
	if err != nil {
		return LattisState{Servers: []auth.ServerState{}}
	}
	return LattisState{Enabled: true, AllowUserServers: config.AllowUserLattisServers(config.Current()), Servers: m.Servers()}
}

// LattisDiscover lit la découverte d'un serveur (/.well-known/lattis.json) :
// nom affiché, logo, fournisseurs ; refus explicite si cette version de
// Lattis est trop ancienne ou si l'organisation ne permet pas ce serveur.
func (s *DocumentService) LattisDiscover(ctx context.Context, url string) (auth.ServerState, error) {
	m, err := lattisOn()
	if err != nil {
		return auth.ServerState{}, err
	}
	return m.Discover(ctx, url)
}

// LattisLogin connecte l'utilisateur par le navigateur du système avec le
// fournisseur provider ; attend son retour (LattisCancelLogin l'abandonne).
func (s *DocumentService) LattisLogin(ctx context.Context, url, provider string) (auth.ServerState, error) {
	m, err := lattisOn()
	if err != nil {
		return auth.ServerState{}, err
	}
	return m.Login(ctx, url, provider)
}

// LattisDeviceStart demande un code pour se connecter depuis un autre
// appareil (secours quand le navigateur de ce poste ne peut pas servir).
func (s *DocumentService) LattisDeviceStart(ctx context.Context, url, provider string) (auth.DeviceCode, error) {
	m, err := lattisOn()
	if err != nil {
		return auth.DeviceCode{}, err
	}
	dc, err := m.StartDevice(ctx, url, provider)
	if err != nil {
		return auth.DeviceCode{}, err
	}
	return *dc, nil
}

// LattisDeviceWait attend que le code de LattisDeviceStart soit validé.
func (s *DocumentService) LattisDeviceWait(ctx context.Context, url string) (auth.ServerState, error) {
	m, err := lattisOn()
	if err != nil {
		return auth.ServerState{}, err
	}
	return m.WaitDevice(ctx, url)
}

// LattisCancelLogin abandonne la connexion en cours à un serveur.
func (s *DocumentService) LattisCancelLogin(url string) {
	lattisManager().Cancel(url)
}

// LattisSignOut ferme la session d'un serveur (jeton révoqué et oublié du
// trousseau) et rend l'état à jour.
func (s *DocumentService) LattisSignOut(ctx context.Context, url string) (LattisState, error) {
	if n, err := auth.NormalizeURL(url); err == nil {
		closeSharePools(n)
	}
	if err := lattisManager().SignOut(ctx, url); err != nil {
		return s.LattisState(), err
	}
	return s.LattisState(), nil
}

// LattisForget retire un serveur de la liste (Gérer les serveurs) : session
// fermée, connexions du serveur fermées, entrée oubliée. Rien n'est supprimé
// sur le serveur ; un serveur de l'organisation reste.
func (s *DocumentService) LattisForget(ctx context.Context, url string) (LattisState, error) {
	m, err := lattisOn()
	if err != nil {
		return s.LattisState(), err
	}
	if n, err := auth.NormalizeURL(url); err == nil {
		closeSharePools(n)
	}
	if err := m.Forget(ctx, url); err != nil {
		return s.LattisState(), err
	}
	return s.LattisState(), nil
}

// checkLattisConnection vérifie qu'une connexion de genre lattis vise un
// serveur permis (fonction active, stratégie de l'organisation).
func checkLattisConnection(c query.Connection) error {
	if c.Kind != query.KindLattis {
		return nil
	}
	m, err := lattisOn()
	if err != nil {
		return err
	}
	u, err := auth.NormalizeURL(c.Config.URL)
	if err != nil {
		return err
	}
	return m.Allowed(u)
}
