package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/auth"
)

// Session d'un moteur lancé par lattisd (cmd/lattis-engine en mode serveur) :
// pas de connexion par le navigateur ni de trousseau, les jetons d'accès de
// l'utilisateur viennent de lattisd (FixedSession.Fetch), pour son seul
// serveur. Au bureau, les sessions sont celles de auth.Manager.

// sessionSource donne les sessions des Serveurs Lattis.
type sessionSource interface {
	Session(ctx context.Context, server string) (auth.User, *auth.Discovery, error)
	Token(ctx context.Context, server string) (string, error)
	Invalidate(server string)
}

// fixed : session imposée par lattisd (nil au bureau).
var fixed *FixedSession

// UseFixedSession fait passer toutes les connexions aux Serveurs Lattis par
// s (moteur d'une session web) ; nil revient aux sessions du bureau.
func UseFixedSession(s *FixedSession) { fixed = s }

// sessions rend la source des sessions : celle de lattisd, ou auth.Manager.
func sessions() (sessionSource, error) {
	if fixed != nil {
		return fixed, nil
	}
	return lattisOn()
}

// FixedSession est la session d'un utilisateur sur un serveur, dont les
// jetons sont fournis par lattisd.
type FixedSession struct {
	// Server : adresse du serveur (émetteur), normalisée.
	Server string
	// Fetch rend un jeton d'accès, l'utilisateur (rôle PostgreSQL, et son
	// identifiant, e-mail et nom quand lattisd les donne) et la durée de
	// validité du jeton.
	Fetch func(ctx context.Context) (token string, user auth.User, ttl time.Duration, err error)
	// Client lit la découverte (http.DefaultClient si nil).
	Client *http.Client

	mu     sync.Mutex
	token  string
	user   auth.User
	expiry time.Time
	disc   *auth.Discovery
}

// ErrOtherServer : un moteur de session ne se connecte qu'à son serveur.
var ErrOtherServer = msg.New("session.otherServer", nil)

func (f *FixedSession) check(server string) error {
	u, err := auth.NormalizeURL(server)
	if err != nil {
		return err
	}
	if u != f.Server {
		return ErrOtherServer
	}
	return nil
}

// Session rend l'utilisateur (son rôle PostgreSQL) et la découverte du
// serveur, lue une fois.
func (f *FixedSession) Session(ctx context.Context, server string) (auth.User, *auth.Discovery, error) {
	if err := f.check(server); err != nil {
		return auth.User{}, nil, err
	}
	if _, err := f.Token(ctx, server); err != nil {
		return auth.User{}, nil, err
	}
	f.mu.Lock()
	disc, user := f.disc, f.user
	f.mu.Unlock()
	if disc == nil {
		hc := f.Client
		if hc == nil {
			hc = http.DefaultClient
		}
		d, err := auth.Discover(ctx, hc, f.Server, "")
		if err != nil {
			return auth.User{}, nil, err
		}
		f.mu.Lock()
		f.disc, disc = d, d
		f.mu.Unlock()
	}
	return user, disc, nil
}

// tokenSkew : un jeton est redemandé une minute avant son expiration.
const tokenSkew = time.Minute

// Token rend un jeton d'accès valable, redemandé à lattisd avant son
// expiration.
func (f *FixedSession) Token(ctx context.Context, server string) (string, error) {
	if err := f.check(server); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token != "" && time.Until(f.expiry) > tokenSkew {
		return f.token, nil
	}
	tok, user, ttl, err := f.Fetch(ctx)
	if err != nil {
		return "", msg.Wrap(err, "session.tokenRefused", msg.Args{"err": err})
	}
	f.token, f.user, f.expiry = tok, user, time.Now().Add(ttl)
	return tok, nil
}

// Invalidate oublie le jeton (refusé par PostgreSQL) : le suivant est
// redemandé.
func (f *FixedSession) Invalidate(string) {
	f.mu.Lock()
	f.token = ""
	f.mu.Unlock()
}

// state décrit le serveur de la session pour l'interface (« Ouvrir depuis
// un serveur »).
func (f *FixedSession) state() LattisState {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := auth.ServerState{URL: f.Server, Name: f.Server, Discovered: f.disc != nil, Providers: []auth.Provider{}}
	if f.disc != nil && f.disc.Name != "" {
		st.Name = f.disc.Name
	}
	st.ServerName = st.Name
	u := f.user
	st.User = &u
	return LattisState{Enabled: true, Servers: []auth.ServerState{st}}
}

// TokenURL fait lire les jetons de la session à lattisd : GET url, avec le
// secret du moteur en Authorization: Bearer ; réponse
// {"access_token", "pg_role", "expires_in"}, et depuis le serveur 2852cec
// "user_id", "email", "display_name" (libellé du compte ; facultatifs).
func TokenURL(url, secret string, hc *http.Client) func(ctx context.Context) (string, auth.User, time.Duration, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return func(ctx context.Context) (string, auth.User, time.Duration, error) {
		var none auth.User
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", none, 0, err
		}
		if secret != "" {
			req.Header.Set("Authorization", "Bearer "+secret)
		}
		resp, err := hc.Do(req)
		if err != nil {
			return "", none, 0, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return "", none, 0, err
		}
		if resp.StatusCode != http.StatusOK {
			return "", none, 0, fmt.Errorf("%s : %s", url, resp.Status)
		}
		var t struct {
			AccessToken string `json:"access_token"`
			PGRole      string `json:"pg_role"`
			ExpiresIn   int64  `json:"expires_in"`
			UserID      string `json:"user_id"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
		}
		if err := json.Unmarshal(body, &t); err != nil {
			return "", none, 0, msg.Wrap(err, "session.unreadableResponse", msg.Args{"url": url, "err": err})
		}
		if t.AccessToken == "" || t.PGRole == "" {
			return "", none, 0, msg.New("session.missingTokenOrRole", msg.Args{"url": url})
		}
		ttl := time.Duration(t.ExpiresIn) * time.Second
		if ttl <= 0 {
			ttl = 5 * time.Minute
		}
		return t.AccessToken, auth.User{ID: t.UserID, Email: t.Email, DisplayName: t.DisplayName, PGRole: t.PGRole}, ttl, nil
	}
}
