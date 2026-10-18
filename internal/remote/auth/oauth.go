package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PKCE (RFC 7636) : vérificateur aléatoire et défi S256.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE tire un vérificateur de 43 caractères (32 octets aléatoires).
func NewPKCE() PKCE {
	v := randomString(32)
	return PKCE{Verifier: v, Challenge: S256(v)}
}

// S256 calcule le défi d'un vérificateur.
func S256(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand ne manque jamais sur les plateformes visées
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// User est l'utilisateur connecté, tel que lattisd le décrit.
type User struct {
	ID          string `json:"id,omitempty"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	// PGRole : rôle PostgreSQL de l'utilisateur (revendication pg_role du
	// jeton), nom d'utilisateur de la connexion PostgreSQL.
	PGRole string `json:"pgRole,omitempty"`
}

// Name donne le nom à afficher.
func (u User) Name() string {
	for _, s := range []string{u.DisplayName, u.Email, u.PGRole, u.ID} {
		if s != "" {
			return s
		}
	}
	return "?"
}

// key désigne l'utilisateur dans le compte du trousseau.
func (u User) key() string {
	for _, s := range []string{u.Email, u.PGRole, u.ID} {
		if s != "" {
			return s
		}
	}
	return "utilisateur"
}

// Tokens est une réponse de /oauth/token.
type Tokens struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	Scope        string `json:"scope"`
	// PGRole : rôle PostgreSQL de l'utilisateur (contrat de lattisd).
	PGRole string `json:"pg_role"`
	// User : extension facultative de lattisd (sinon lue dans les
	// revendications du jeton d'accès, puis du jeton d'identité).
	User *tokenUser `json:"user,omitempty"`
	// Expiry : calculée à la réception.
	Expiry time.Time `json:"-"`
}

type tokenUser struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	PGRole      string `json:"pg_role"`
}

// user tire l'utilisateur de la réponse.
func (t *Tokens) user() User {
	u := User{PGRole: t.PGRole}
	if t.User != nil {
		u = User{ID: t.User.ID, Email: t.User.Email, DisplayName: t.User.DisplayName, PGRole: t.User.PGRole}
		if u.PGRole == "" {
			u.PGRole = t.PGRole
		}
	}
	for _, jwt := range []string{t.AccessToken, t.IDToken} {
		c := jwtClaims(jwt)
		if c == nil {
			continue
		}
		fill := func(dst *string, keys ...string) {
			for _, k := range keys {
				if *dst != "" {
					return
				}
				if s, ok := c[k].(string); ok {
					*dst = s
				}
			}
		}
		fill(&u.ID, "sub")
		fill(&u.Email, "email")
		fill(&u.DisplayName, "name", "preferred_username")
		fill(&u.PGRole, "pg_role")
	}
	return u
}

// jwtClaims décode les revendications d'un JWT sans vérifier sa signature :
// elles ne servent qu'à l'affichage et au nom du rôle PostgreSQL (le
// serveur, lui, vérifie le jeton). nil si ce n'est pas un JWT.
func jwtClaims(tok string) map[string]any {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil
	}
	var c map[string]any
	if json.Unmarshal(b, &c) != nil {
		return nil
	}
	return c
}

// Codes de refus de lattisd (paramètre reason d'une erreur access_denied,
// ou code d'erreur lui-même) ; les alias suivent les étiquettes des mesures
// du serveur (lattis_auth_logins_total{result}).
const (
	ReasonTenant     = "denied_tenant"
	ReasonInvitation = "not_invited"
	ReasonDisabled   = "account_disabled"
)

var reasonAliases = map[string]string{
	"denied_tenant": ReasonTenant, "tenant_not_allowed": ReasonTenant, "tenant_denied": ReasonTenant,
	"not_invited": ReasonInvitation, "invitation_required": ReasonInvitation,
	"account_disabled": ReasonDisabled, "user_disabled": ReasonDisabled, "disabled": ReasonDisabled,
}

// LoginError est un refus de connexion, avec un message clair.
type LoginError struct {
	// Code : erreur OAuth (access_denied, invalid_grant…).
	Code string
	// Reason : ReasonTenant, ReasonInvitation, ReasonDisabled ou "".
	Reason      string
	Description string
	// Server : nom affiché du serveur.
	Server string
}

func (e *LoginError) Error() string {
	srv := e.Server
	if srv == "" {
		srv = "le serveur"
	}
	switch e.Reason {
	case ReasonTenant:
		return fmt.Sprintf("Connexion refusée : votre compte n'appartient pas à une organisation autorisée sur %s. Utilisez un autre compte ou demandez à l'administrateur du serveur d'autoriser la vôtre.", srv)
	case ReasonInvitation:
		return fmt.Sprintf("Connexion refusée : %s n'accepte que les personnes invitées. Demandez une invitation à son administrateur, puis reconnectez-vous avec l'adresse invitée.", srv)
	case ReasonDisabled:
		return fmt.Sprintf("Connexion refusée : votre compte est désactivé sur %s. Contactez son administrateur.", srv)
	}
	switch e.Code {
	case "invalid_grant":
		if e.Description != "" {
			return "Connexion refusée : " + e.Description
		}
		return "Adresse ou mot de passe incorrect."
	case "access_denied":
		if e.Description != "" {
			return "Connexion refusée : " + e.Description
		}
		return "Connexion refusée par " + srv + "."
	case "expired_token":
		return "Le code de connexion a expiré : recommencez."
	}
	msg := "Connexion impossible (" + e.Code + ")"
	if e.Description != "" {
		msg += " : " + e.Description
	}
	return msg
}

// newLoginError lit un refus (paramètres de redirection ou corps JSON).
func newLoginError(code, desc, reason, server string) *LoginError {
	r := reasonAliases[strings.ToLower(reason)]
	if r == "" {
		r = reasonAliases[strings.ToLower(code)]
	}
	return &LoginError{Code: code, Reason: r, Description: desc, Server: server}
}

// ErrSessionExpired : le jeton de rafraîchissement est refusé (révoqué,
// expiré, compte désactivé) : il faut se reconnecter.
var ErrSessionExpired = errors.New("session expirée : reconnectez-vous")

type oauthError struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
	Reason      string `json:"reason"`
}

// client parle aux points OAuth d'un serveur.
type client struct {
	hc     *http.Client
	server string // adresse normalisée
	name   string // nom affiché (messages)
	now    func() time.Time
	// Lus dans la découverte (valeurs par défaut sinon).
	clientID, scope, issuer      string
	authURL, tokenURL, revokeURL string
	deviceURL                    string
}

// post envoie un formulaire et décode la réponse JSON dans out ; une
// réponse d'erreur OAuth devient *LoginError.
func (c *client) post(ctx context.Context, endpoint string, form url.Values, out any) error {
	path := endpoint
	if u, err := url.Parse(endpoint); err == nil {
		path = u.Path
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("%s injoignable : %w", c.label(), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		var oe oauthError
		if json.Unmarshal(body, &oe) == nil && oe.Error != "" {
			return newLoginError(oe.Error, oe.Description, oe.Reason, c.name)
		}
		return fmt.Errorf("%s : réponse inattendue de %s (%s)", c.label(), path, resp.Status)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s : réponse illisible de %s : %w", c.label(), path, err)
	}
	return nil
}

func (c *client) label() string {
	if c.name != "" {
		return c.name
	}
	return Host(c.server)
}

// token appelle /oauth/token.
func (c *client) token(ctx context.Context, form url.Values) (*Tokens, error) {
	form.Set("client_id", c.clientID)
	var t Tokens
	if err := c.post(ctx, c.tokenURL, form, &t); err != nil {
		return nil, err
	}
	if t.AccessToken == "" {
		return nil, fmt.Errorf("%s : jeton d'accès absent de la réponse", c.label())
	}
	if !strings.EqualFold(t.TokenType, "bearer") && t.TokenType != "" {
		return nil, fmt.Errorf("%s : type de jeton %q non pris en charge", c.label(), t.TokenType)
	}
	ttl := time.Duration(t.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 15 * time.Minute // durée par défaut des jetons de lattisd (§7.1)
	}
	t.Expiry = c.now().Add(ttl)
	return &t, nil
}

// authorizeURL écrit l'adresse de /oauth/authorize.
func (c *client) authorizeURL(provider, redirect, state string, p PKCE) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", redirect)
	q.Set("scope", c.scope)
	q.Set("state", state)
	q.Set("code_challenge", p.Challenge)
	q.Set("code_challenge_method", "S256")
	if provider != "" {
		q.Set("provider", provider)
	}
	sep := "?"
	if strings.Contains(c.authURL, "?") {
		sep = "&"
	}
	return c.authURL + sep + q.Encode()
}

// exchange échange un code d'autorisation.
func (c *client) exchange(ctx context.Context, code, redirect, verifier string) (*Tokens, error) {
	return c.token(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirect},
		"code_verifier": {verifier},
	})
}

// password connecte un compte à mot de passe Lattis (grant password,
// réservé au fournisseur password du client de bureau).
func (c *client) password(ctx context.Context, provider, email, password string) (*Tokens, error) {
	f := url.Values{
		"grant_type": {"password"},
		"username":   {email},
		"password":   {password},
		"scope":      {c.scope},
	}
	if provider != "" {
		f.Set("provider", provider)
	}
	return c.token(ctx, f)
}

// refresh renouvelle les jetons (le serveur fait tourner le jeton de
// rafraîchissement : le nouveau remplace l'ancien).
func (c *client) refresh(ctx context.Context, refreshToken string) (*Tokens, error) {
	t, err := c.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}})
	var le *LoginError
	if errors.As(err, &le) && (le.Code == "invalid_grant" || le.Reason != "") {
		if le.Reason != "" {
			return nil, le
		}
		return nil, ErrSessionExpired
	}
	return t, err
}

// revoke révoque un jeton (RFC 7009) ; une erreur est sans conséquence
// pour la déconnexion locale.
func (c *client) revoke(ctx context.Context, token, hint string) error {
	return c.post(ctx, c.revokeURL, url.Values{"token": {token}, "token_type_hint": {hint}, "client_id": {c.clientID}}, nil)
}

// DeviceCode est une demande de connexion depuis un autre appareil
// (RFC 8628).
type DeviceCode struct {
	UserCode                string    `json:"userCode"`
	VerificationURI         string    `json:"verificationUri"`
	VerificationURIComplete string    `json:"verificationUriComplete,omitempty"`
	ExpiresAt               time.Time `json:"expiresAt"`

	deviceCode string
	interval   time.Duration
}

type deviceResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

// device demande un code (POST /oauth/device).
func (c *client) device(ctx context.Context, provider string) (*DeviceCode, error) {
	f := url.Values{"client_id": {c.clientID}, "scope": {c.scope}}
	if provider != "" {
		f.Set("provider", provider)
	}
	var r deviceResponse
	if err := c.post(ctx, c.deviceURL, f, &r); err != nil {
		return nil, err
	}
	if r.DeviceCode == "" || r.UserCode == "" || r.VerificationURI == "" {
		return nil, fmt.Errorf("%s : réponse incomplète de %s", c.label(), PathDevice)
	}
	if r.ExpiresIn <= 0 {
		r.ExpiresIn = 600
	}
	if r.Interval <= 0 {
		r.Interval = 5
	}
	return &DeviceCode{
		UserCode: r.UserCode, VerificationURI: r.VerificationURI, VerificationURIComplete: r.VerificationURIComplete,
		ExpiresAt:  c.now().Add(time.Duration(r.ExpiresIn) * time.Second),
		deviceCode: r.DeviceCode, interval: time.Duration(r.Interval) * time.Second,
	}, nil
}

// pollDevice attend que l'utilisateur ait validé le code.
func (c *client) pollDevice(ctx context.Context, dc *DeviceCode, sleep func(context.Context, time.Duration) error) (*Tokens, error) {
	interval := dc.interval
	for {
		if err := sleep(ctx, interval); err != nil {
			return nil, err
		}
		if c.now().After(dc.ExpiresAt) {
			return nil, &LoginError{Code: "expired_token", Server: c.name}
		}
		t, err := c.token(ctx, url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "device_code": {dc.deviceCode}})
		var le *LoginError
		if errors.As(err, &le) {
			switch le.Code {
			case "authorization_pending":
				continue
			case "slow_down":
				interval += 5 * time.Second
				continue
			}
		}
		return t, err
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
