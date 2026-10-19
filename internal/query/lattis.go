package query

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lalamefine/lattis/internal/remote/auth"
)

// Connexion de genre lattis : un Serveur Lattis (classeurs partagés). Elle
// ne porte que l'adresse du serveur et son nom affiché ; l'identité est la
// session ouverte sur ce poste (internal/remote/auth), pas un mot de passe.
// Comme source de requête, elle lit la base du serveur en lecture seule
// (sur un réplica quand il y en a), sous l'identité de l'utilisateur.

// LattisOpen ouvre la base d'un Serveur Lattis pour un usage
// (pgpool.Usage*) ; posée par internal/bridge quand les classeurs partagés
// sont activés. Renvoie *auth.ErrLoginRequired sans session.
var LattisOpen func(ctx context.Context, server, usage string) (*sql.DB, error)

func init() {
	dialects[KindLattis] = &dialect{
		label:      "Serveur Lattis",
		authFailed: pgAuthFailed,
		limit:      limitClause,
		readOnly:   true,
		tables:     lattisTables,
		ident:      PGIdent,
	}
}

// lattisTables liste les tables et vues d'un Serveur Lattis : les données
// publiées (donnees, en tête) et les schémas des classeurs que
// l'utilisateur peut lire, jamais les tables internes du serveur (lattis,
// dont le catalogue montre les noms même quand leur contenu est refusé).
var lattisTables = infoSchemaTables(pgTablesSQL([]string{"lattis"}, "donnees"))

// lattisAccount désigne un serveur (le compte de l'utilisateur est celui de
// la session, lattis://<hôte>/<utilisateur> au trousseau).
func lattisAccount(u string) string {
	if n, err := auth.NormalizeURL(u); err == nil {
		return "lattis://" + auth.Host(n)
	}
	return "lattis://" + u
}

func checkLattis(c DBConfig) error {
	if _, err := auth.NormalizeURL(c.URL); err != nil {
		return fmt.Errorf("Serveur Lattis : %w", err)
	}
	return nil
}

// connectLattis ouvre et teste la base d'un serveur ; *ErrPassword (avec
// Lattis) quand il faut se connecter.
func connectLattis(ctx context.Context, c Connection) (*sql.DB, *dialect, error) {
	if LattisOpen == nil {
		return nil, nil, fmt.Errorf("connexion « %s » : classeurs partagés désactivés (Fichier › Paramètres › Fonctionnalités)", c.Name)
	}
	server, err := auth.NormalizeURL(c.Config.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("connexion « %s » : %w", c.Name, err)
	}
	login := func(err error) error {
		return &ErrPassword{Connection: c.Name, Account: c.Account(), Message: err.Error(), Lattis: server}
	}
	db, err := LattisOpen(ctx, server, "power_query")
	var lr *auth.ErrLoginRequired
	if errors.As(err, &lr) {
		return nil, nil, login(err)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connexion « %s » : %w", c.Name, err)
	}
	d := dialects[KindLattis]
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		if errors.As(err, &lr) || d.authFailed(err) {
			return nil, nil, login(err)
		}
		return nil, nil, fmt.Errorf("connexion « %s » : %w", c.Name, err)
	}
	return db, d, nil
}

// LattisTokens range les jetons de rafraîchissement des Serveurs Lattis
// dans le coffre s (trousseau du système), sans accord par classeur : la
// session appartient à l'utilisateur du poste, pas à un classeur.
func LattisTokens(s *Secrets) auth.TokenStore { return lattisTokens{s.For("")} }

type lattisTokens struct{ s *Secrets }

func (t lattisTokens) Get(account string) (string, bool) { return t.s.Get(account) }
func (t lattisTokens) Set(account, token string) error   { return t.s.Set(account, token, true) }
func (t lattisTokens) Forget(account string) error       { return t.s.Forget(account) }
