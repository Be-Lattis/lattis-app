package auth

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lalamefine/lattis/internal/msg"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// TokenStore garde les jetons de rafraîchissement (trousseau du système,
// voir query.LattisTokens), par compte lattis://<hôte>/<utilisateur>.
type TokenStore interface {
	Get(account string) (string, bool)
	Set(account, token string) error
	Forget(account string) error
}

// Managed est un serveur déclaré par la configuration d'entreprise
// (connections.<id> de genre lattis).
type Managed struct {
	URL  string
	Name string
	// Provider : fournisseur imposé (identifiant annoncé par le serveur).
	Provider string
}

// Policy est la stratégie qui s'applique aux serveurs.
type Policy struct {
	Managed []Managed
	// AllowUserServers : l'utilisateur peut ajouter d'autres serveurs que
	// ceux de l'organisation.
	AllowUserServers bool
}

// Options règle un Manager.
type Options struct {
	HTTP  *http.Client
	Store TokenStore
	// IndexPath : fichier des serveurs et comptes connus, sans secret (vide :
	// en mémoire seulement).
	IndexPath string
	// ClientVersion : version de Lattis, comparée à min_client_version.
	ClientVersion string
	// Policy donne la stratégie courante (nil : aucun serveur géré, ajout
	// libre).
	Policy func() Policy
	// OpenBrowser ouvre l'adresse de connexion (OpenBrowser par défaut).
	OpenBrowser func(string) error
	Now         func() time.Time
	// LoginTimeout borne l'attente du navigateur (5 min par défaut).
	LoginTimeout time.Duration
}

// refreshSkew : le jeton d'accès est renouvelé un peu avant son expiration.
const refreshSkew = 60 * time.Second

// Manager tient les serveurs connus et les sessions ouvertes.
type Manager struct {
	opts Options

	mu      sync.Mutex
	loaded  bool
	servers map[string]*server
	// sleep attend entre deux interrogations du « device code » (tests).
	sleep func(context.Context, time.Duration) error
}

type server struct {
	url  string
	disc *Discovery
	// name, logo : derniers connus (index), pour l'affichage hors ligne.
	name, logo string
	// custom : nom choisi par l'utilisateur (Gérer les serveurs › Renommer),
	// local, jamais envoyé au serveur ; vide : le nom du serveur.
	custom string
	sess   *session
	cancel context.CancelFunc
	device *DeviceCode
	known  bool // ajouté par l'utilisateur ou connecté une fois (index)
}

type session struct {
	mu      sync.Mutex
	user    User
	account string
	access  string
	expiry  time.Time
	refresh string
}

// NewManager crée un gestionnaire.
func NewManager(o Options) *Manager {
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if o.OpenBrowser == nil {
		o.OpenBrowser = OpenBrowser
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.LoginTimeout <= 0 {
		o.LoginTimeout = 5 * time.Minute
	}
	if o.Store == nil {
		o.Store = memStore{}
	}
	return &Manager{opts: o, servers: map[string]*server{}, sleep: sleepCtx}
}

// memStore garde les jetons en mémoire (tests, trousseau absent).
type memStore map[string]string

func (s memStore) Get(a string) (string, bool) { v, ok := s[a]; return v, ok }
func (s memStore) Set(a, v string) error       { s[a] = v; return nil }
func (s memStore) Forget(a string) error       { delete(s, a); return nil }

// ServerState décrit un serveur pour l'interface.
type ServerState struct {
	URL string `json:"url"`
	// Name : nom affiché (Label s'il est choisi, sinon ServerName).
	Name string `json:"name"`
	// Label : nom choisi par l'utilisateur (local, Rename) ; ServerName :
	// celui qu'annonce le serveur.
	Label      string     `json:"label,omitempty"`
	ServerName string     `json:"serverName,omitempty"`
	Logo       string     `json:"logo,omitempty"`
	Version    string     `json:"version,omitempty"`
	Edition    string     `json:"edition,omitempty"`
	AccessMode string     `json:"accessMode,omitempty"`
	Providers  []Provider `json:"providers"`
	// Discovered : la découverte a été lue pendant cette session.
	Discovered bool `json:"discovered"`
	// Managed : déclaré par l'organisation.
	Managed bool `json:"managed"`
	// ImposedProvider : seul fournisseur permis par l'organisation.
	ImposedProvider string `json:"imposedProvider,omitempty"`
	// User : utilisateur connecté (nil : pas de session).
	User *User `json:"user"`
	// Pending : une connexion attend le navigateur ou l'autre appareil.
	Pending bool `json:"pending,omitempty"`
}

// ErrLoginRequired demande une connexion au serveur.
type ErrLoginRequired struct {
	Server string // adresse normalisée
	Name   string
	Cause  error
}

func (e *ErrLoginRequired) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e *ErrLoginRequired) Msg() msg.Message {
	switch {
	case e.Cause != nil && !errors.Is(e.Cause, ErrSessionExpired):
		return msg.M("auth.loginRequiredCause", msg.Args{"server": e.Name, "err": e.Cause})
	case e.Cause != nil:
		return msg.M("auth.sessionExpiredOn", msg.Args{"server": e.Name})
	}
	return msg.M("auth.loginRequired", msg.Args{"server": e.Name})
}

func (e *ErrLoginRequired) Unwrap() error { return e.Cause }

// ErrCanceled : connexion abandonnée par l'utilisateur.
var ErrCanceled = msg.New("auth.canceled", nil)

func (m *Manager) policy() Policy {
	if m.opts.Policy == nil {
		return Policy{AllowUserServers: true}
	}
	return m.opts.Policy()
}

func (m *Manager) managed(p Policy, u string) (Managed, bool) {
	for _, x := range p.Managed {
		if n, err := NormalizeURL(x.URL); err == nil && n == u {
			x.URL = n
			return x, true
		}
	}
	return Managed{}, false
}

// Allowed vérifie qu'un serveur (adresse normalisée) est permis par la
// stratégie.
func (m *Manager) Allowed(u string) error {
	p := m.policy()
	if _, ok := m.managed(p, u); ok || p.AllowUserServers {
		return nil
	}
	return msg.New("auth.orgServersOnly", msg.Args{"host": Host(u)})
}

// get donne (ou crée) l'entrée d'un serveur ; mu tenu.
func (m *Manager) get(u string) *server {
	m.load()
	s := m.servers[u]
	if s == nil {
		s = &server{url: u}
		m.servers[u] = s
	}
	return s
}

// label : nom affiché, celui choisi par l'utilisateur d'abord.
func (s *server) label() string {
	if s.custom != "" {
		return s.custom
	}
	return s.own()
}

// own : nom annoncé par le serveur (découverte, index), sinon son hôte.
func (s *server) own() string {
	switch {
	case s.disc != nil:
		return s.disc.Name
	case s.name != "":
		return s.name
	}
	return Host(s.url)
}

func (m *Manager) state(p Policy, s *server) ServerState {
	st := ServerState{URL: s.url, Name: s.label(), Label: s.custom, ServerName: s.own(), Logo: s.logo, Providers: []Provider{}, Pending: s.cancel != nil}
	if mg, ok := m.managed(p, s.url); ok {
		st.Managed = true
		st.ImposedProvider = mg.Provider
		if s.disc == nil && s.name == "" && mg.Name != "" {
			st.ServerName = mg.Name
			if s.custom == "" {
				st.Name = mg.Name
			}
		}
	}
	if d := s.disc; d != nil {
		st.Discovered = true
		st.Logo, st.Version, st.Edition, st.AccessMode = d.Logo, d.Version, d.Edition, d.AccessMode
		for _, pr := range d.Providers {
			if st.ImposedProvider == "" || pr.ID == st.ImposedProvider {
				st.Providers = append(st.Providers, pr)
			}
		}
	}
	if s.sess != nil {
		u := s.sess.user
		st.User = &u
	}
	return st
}

// Servers liste les serveurs connus : ceux de l'organisation d'abord, puis
// ceux de l'utilisateur (sauf si la stratégie les interdit), par nom.
func (m *Manager) Servers() []ServerState {
	p := m.policy()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.load()
	for _, mg := range p.Managed {
		if u, err := NormalizeURL(mg.URL); err == nil {
			m.get(u)
		}
	}
	out := []ServerState{}
	for _, s := range m.servers {
		st := m.state(p, s)
		if !st.Managed && (!p.AllowUserServers || !s.known && s.sess == nil && s.disc == nil) {
			continue
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Managed != out[j].Managed {
			return out[i].Managed
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// State donne l'état d'un serveur.
func (m *Manager) State(raw string) (ServerState, error) {
	u, err := NormalizeURL(raw)
	if err != nil {
		return ServerState{}, err
	}
	p := m.policy()
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state(p, m.get(u)), nil
}

// Discover lit (ou relit) la découverte d'un serveur et le retient.
func (m *Manager) Discover(ctx context.Context, raw string) (ServerState, error) {
	u, err := NormalizeURL(raw)
	if err != nil {
		return ServerState{}, err
	}
	if err := m.Allowed(u); err != nil {
		return ServerState{}, err
	}
	d, err := Discover(ctx, m.opts.HTTP, u, m.opts.ClientVersion)
	if err != nil {
		return ServerState{}, err
	}
	p := m.policy()
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.get(u)
	s.disc, s.name, s.logo = d, d.Name, d.Logo
	if !s.known {
		s.known = true
		m.save()
	}
	return m.state(p, s), nil
}

// prepare vérifie la stratégie et le fournisseur, et lit la découverte si
// besoin.
func (m *Manager) prepare(ctx context.Context, raw, provider string, kind string) (*server, *client, error) {
	u, err := NormalizeURL(raw)
	if err != nil {
		return nil, nil, err
	}
	if err := m.Allowed(u); err != nil {
		return nil, nil, err
	}
	m.mu.Lock()
	s := m.get(u)
	disc := s.disc
	m.mu.Unlock()
	if disc == nil {
		if _, err := m.Discover(ctx, u); err != nil {
			return nil, nil, err
		}
		m.mu.Lock()
		disc = s.disc
		m.mu.Unlock()
	}
	var pr *Provider
	for i := range disc.Providers {
		if disc.Providers[i].ID == provider {
			pr = &disc.Providers[i]
		}
	}
	if pr == nil {
		return nil, nil, msg.New("auth.providerNotOffered", msg.Args{"server": s.label(), "provider": provider})
	}
	if mg, ok := m.managed(m.policy(), u); ok && mg.Provider != "" && mg.Provider != provider {
		return nil, nil, msg.New("auth.providerImposed", msg.Args{"server": s.label(), "provider": mg.Provider})
	}
	if kind == ProviderPassword && pr.Kind != ProviderPassword {
		return nil, nil, msg.New("auth.noPassword", msg.Args{"provider": pr.Label})
	}
	return s, m.client(s), nil
}

func (m *Manager) client(s *server) *client {
	c := &client{hc: m.opts.HTTP, server: s.url, name: s.label(), now: m.opts.Now,
		clientID: ClientID, scope: Scope,
		authURL: s.url + PathAuthorize, tokenURL: s.url + PathToken, revokeURL: s.url + PathRevoke, deviceURL: s.url + PathDevice}
	if d := s.disc; d != nil {
		c.clientID, c.scope, c.issuer = d.ClientID, d.Scope, d.Issuer
		c.authURL, c.tokenURL, c.revokeURL, c.deviceURL = d.AuthorizationEndpoint, d.TokenEndpoint, d.RevocationEndpoint, d.DeviceEndpoint
	}
	return c
}

// begin note une connexion en cours (annulable) ; une connexion précédente
// est annulée.
func (m *Manager) begin(ctx context.Context, s *server, timeout time.Duration) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	m.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = cancel
	m.mu.Unlock()
	return ctx, func() {
		cancel()
		m.mu.Lock()
		// Une connexion plus récente a pu prendre la place.
		s.cancel = nil
		m.mu.Unlock()
	}
}

// Cancel abandonne la connexion en cours à un serveur.
func (m *Manager) Cancel(raw string) {
	u, err := NormalizeURL(raw)
	if err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.servers[u]; s != nil && s.cancel != nil {
		s.cancel()
	}
}

func canceled(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return ErrCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return msg.New("auth.timeout", nil)
	}
	return err
}

// Login connecte l'utilisateur par le navigateur du système (code
// d'autorisation + PKCE, redirection en boucle locale). Bloque jusqu'au
// retour du navigateur, à l'annulation (Cancel) ou au délai.
func (m *Manager) Login(ctx context.Context, raw, provider string) (ServerState, error) {
	s, c, err := m.prepare(ctx, raw, provider, "")
	if err != nil {
		return ServerState{}, err
	}
	ctx, done := m.begin(ctx, s, m.opts.LoginTimeout)
	defer done()
	state := randomString(16)
	p := NewPKCE()
	lb, err := startLoopback(state, c.issuer, c.name)
	if err != nil {
		return ServerState{}, msg.New("auth.loopbackFailed", msg.Args{"err": err})
	}
	defer lb.close()
	if err := m.opts.OpenBrowser(c.authorizeURL(provider, lb.redirect, state, p)); err != nil {
		return ServerState{}, msg.New("auth.otherDevice", msg.Args{"err": err})
	}
	code, err := lb.wait(ctx)
	if err != nil {
		return ServerState{}, canceled(err)
	}
	t, err := c.exchange(ctx, code, lb.redirect, p.Verifier)
	if err != nil {
		return ServerState{}, canceled(err)
	}
	return m.establish(s, t)
}

// StartDevice demande un code de connexion depuis un autre appareil.
func (m *Manager) StartDevice(ctx context.Context, raw, provider string) (*DeviceCode, error) {
	s, c, err := m.prepare(ctx, raw, provider, "")
	if err != nil {
		return nil, err
	}
	dc, err := c.device(ctx, provider)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	s.device = dc
	m.mu.Unlock()
	return dc, nil
}

// WaitDevice attend la validation du code demandé par StartDevice.
func (m *Manager) WaitDevice(ctx context.Context, raw string) (ServerState, error) {
	u, err := NormalizeURL(raw)
	if err != nil {
		return ServerState{}, err
	}
	m.mu.Lock()
	s := m.get(u)
	dc := s.device
	m.mu.Unlock()
	if dc == nil {
		return ServerState{}, msg.New("auth.noPendingCode", nil)
	}
	ctx, done := m.begin(ctx, s, time.Until(dc.ExpiresAt)+time.Minute)
	defer done()
	t, err := m.client(s).pollDevice(ctx, dc, m.sleep)
	m.mu.Lock()
	s.device = nil
	m.mu.Unlock()
	if err != nil {
		return ServerState{}, canceled(err)
	}
	return m.establish(s, t)
}

// account désigne le compte du trousseau.
func account(server string, u User) string {
	return "lattis://" + Host(server) + "/" + u.key()
}

// establish ouvre la session à partir des jetons reçus.
func (m *Manager) establish(s *server, t *Tokens) (ServerState, error) {
	u := t.user()
	sess := &session{user: u, account: account(s.url, u), access: t.AccessToken, expiry: t.Expiry, refresh: t.RefreshToken}
	var storeErr error
	if t.RefreshToken != "" {
		storeErr = m.opts.Store.Set(sess.account, t.RefreshToken)
	}
	p := m.policy()
	m.mu.Lock()
	if old := s.sess; old != nil && old.account != sess.account {
		m.opts.Store.Forget(old.account)
	}
	s.sess, s.known = sess, true
	m.save()
	st := m.state(p, s)
	m.mu.Unlock()
	if storeErr != nil {
		return st, msg.Wrap(storeErr, "auth.keyringRefused", msg.Args{"err": storeErr})
	}
	return st, nil
}

// SignOut ferme la session d'un serveur : jeton révoqué (au mieux),
// oublié du trousseau.
func (m *Manager) SignOut(ctx context.Context, raw string) error {
	u, err := NormalizeURL(raw)
	if err != nil {
		return err
	}
	m.mu.Lock()
	s := m.get(u)
	sess := s.sess
	s.sess = nil
	c := m.client(s)
	m.save()
	m.mu.Unlock()
	if sess == nil {
		return nil
	}
	sess.mu.Lock()
	rt := sess.refresh
	sess.mu.Unlock()
	if rt == "" {
		rt, _ = m.opts.Store.Get(sess.account)
	}
	if rt != "" {
		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		c.revoke(rctx, rt, "refresh_token")
		cancel()
	}
	return m.opts.Store.Forget(sess.account)
}

// Forget retire un serveur de la liste de l'utilisateur (Gérer les
// serveurs) : connexion en cours abandonnée, session fermée comme par
// SignOut, entrée ôtée de l'index. Rien n'est supprimé sur le serveur. Un
// serveur déclaré par l'organisation ne se retire pas.
func (m *Manager) Forget(ctx context.Context, raw string) error {
	u, err := NormalizeURL(raw)
	if err != nil {
		return err
	}
	if _, ok := m.managed(m.policy(), u); ok {
		return msg.New("auth.orgServerLocked", msg.Args{"host": Host(u)})
	}
	m.Cancel(u)
	if err := m.SignOut(ctx, u); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.servers, u)
	m.save()
	return nil
}

// MaxLabel : longueur maximale (en caractères) du nom choisi pour un
// serveur.
const MaxLabel = 60

// Rename donne à un serveur le nom choisi par l'utilisateur (Gérer les
// serveurs › Renommer), gardé dans l'index et jamais envoyé au serveur :
// une nouvelle lecture de la découverte ne le remplace pas. Un nom vide,
// ou celui du serveur, revient au nom du serveur.
func (m *Manager) Rename(raw, label string) (ServerState, error) {
	u, err := NormalizeURL(raw)
	if err != nil {
		return ServerState{}, err
	}
	label = strings.Join(strings.Fields(label), " ")
	if n := utf8.RuneCountInString(label); n > MaxLabel {
		return ServerState{}, msg.New("auth.labelTooLong", msg.Args{"max": MaxLabel})
	}
	p := m.policy()
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.get(u)
	if label == m.state(p, s).ServerName {
		label = ""
	}
	s.custom = label
	m.save()
	return m.state(p, s), nil
}

// Label donne le nom choisi par l'utilisateur pour un serveur ("" : aucun,
// ou adresse invalide).
func (m *Manager) Label(raw string) string {
	u, err := NormalizeURL(raw)
	if err != nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.load()
	if s := m.servers[u]; s != nil {
		return s.custom
	}
	return ""
}

// Session donne l'utilisateur connecté et la découverte d'un serveur
// (relue si besoin) ; *ErrLoginRequired sans session.
func (m *Manager) Session(ctx context.Context, raw string) (User, *Discovery, error) {
	u, err := NormalizeURL(raw)
	if err != nil {
		return User{}, nil, err
	}
	if err := m.Allowed(u); err != nil {
		return User{}, nil, err
	}
	m.mu.Lock()
	s := m.get(u)
	sess, disc, name := s.sess, s.disc, s.label()
	m.mu.Unlock()
	if sess == nil {
		return User{}, nil, &ErrLoginRequired{Server: u, Name: name}
	}
	if disc == nil {
		if _, err := m.Discover(ctx, u); err != nil {
			return User{}, nil, err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return sess.user, s.disc, nil
}

// Token donne un jeton d'accès valable pour un serveur, renouvelé avant
// son expiration (et donc à chaque reconnexion qui en a besoin).
// *ErrLoginRequired sans session ou si le serveur refuse le
// renouvellement.
func (m *Manager) Token(ctx context.Context, raw string) (string, error) {
	u, err := NormalizeURL(raw)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	s := m.get(u)
	sess, name := s.sess, s.label()
	c := m.client(s)
	m.mu.Unlock()
	if sess == nil {
		return "", &ErrLoginRequired{Server: u, Name: name}
	}
	sess.mu.Lock()
	tok, user, err := m.renew(ctx, c, sess)
	sess.mu.Unlock()
	var lr *ErrLoginRequired
	switch {
	case errors.As(err, &lr):
		lr.Server, lr.Name = u, name
		m.drop(s, sess)
	case user != nil:
		m.mu.Lock()
		if user.key() == sess.user.key() {
			sess.user = *user
		}
		m.mu.Unlock()
	}
	return tok, err
}

// renew rend le jeton d'accès de la session, renouvelé si besoin ;
// sess.mu tenu. user : utilisateur relu au renouvellement.
func (m *Manager) renew(ctx context.Context, c *client, sess *session) (string, *User, error) {
	if sess.access != "" && m.opts.Now().Add(refreshSkew).Before(sess.expiry) {
		return sess.access, nil, nil
	}
	rt := sess.refresh
	if rt == "" {
		rt, _ = m.opts.Store.Get(sess.account)
	}
	if rt == "" {
		return "", nil, &ErrLoginRequired{Cause: ErrSessionExpired}
	}
	t, err := c.refresh(ctx, rt)
	if err != nil {
		var le *LoginError
		if errors.Is(err, ErrSessionExpired) || errors.As(err, &le) {
			m.opts.Store.Forget(sess.account)
			return "", nil, &ErrLoginRequired{Cause: err}
		}
		return "", nil, err // réseau : la session reste, on réessaiera
	}
	sess.access, sess.expiry = t.AccessToken, t.Expiry
	if t.RefreshToken != "" && t.RefreshToken != rt {
		sess.refresh = t.RefreshToken
		m.opts.Store.Set(sess.account, t.RefreshToken)
	} else {
		sess.refresh = rt
	}
	nu := t.user()
	return sess.access, &nu, nil
}

// Invalidate oublie le jeton d'accès d'un serveur (refusé par PostgreSQL) :
// le prochain Token le renouvelle.
func (m *Manager) Invalidate(raw string) {
	u, err := NormalizeURL(raw)
	if err != nil {
		return
	}
	m.mu.Lock()
	sess := m.get(u).sess
	m.mu.Unlock()
	if sess == nil {
		return
	}
	sess.mu.Lock()
	sess.access = ""
	sess.mu.Unlock()
}

// drop ferme une session refusée par le serveur.
func (m *Manager) drop(s *server, sess *session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.sess == sess {
		s.sess = nil
		m.save()
	}
}

// Index des serveurs connus (sans secret) : nom et logo pour l'affichage
// hors ligne, utilisateur connecté (son jeton est au trousseau).
type indexFile struct {
	Servers []indexEntry `json:"servers"`
}

type indexEntry struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
	// Label : nom choisi par l'utilisateur (Rename).
	Label string `json:"label,omitempty"`
	Logo  string `json:"logo,omitempty"`
	User  *User  `json:"user,omitempty"`
}

// load lit l'index (une fois) ; mu tenu. Un compte dont le trousseau a
// perdu le jeton est oublié.
func (m *Manager) load() {
	if m.loaded {
		return
	}
	m.loaded = true
	if m.opts.IndexPath == "" {
		return
	}
	b, err := os.ReadFile(m.opts.IndexPath)
	if err != nil {
		return
	}
	var f indexFile
	if json.Unmarshal(b, &f) != nil {
		return
	}
	for _, e := range f.Servers {
		u, err := NormalizeURL(e.URL)
		if err != nil {
			continue
		}
		s := &server{url: u, name: e.Name, logo: e.Logo, custom: e.Label, known: true}
		if e.User != nil {
			a := account(u, *e.User)
			if _, ok := m.opts.Store.Get(a); ok {
				s.sess = &session{user: *e.User, account: a}
			}
		}
		m.servers[u] = s
	}
}

// save écrit l'index ; mu tenu. Une erreur d'écriture n'empêche rien.
func (m *Manager) save() {
	if m.opts.IndexPath == "" {
		return
	}
	f := indexFile{Servers: []indexEntry{}}
	for _, s := range m.servers {
		if !s.known && s.sess == nil && s.custom == "" {
			continue
		}
		e := indexEntry{URL: s.url, Name: s.name, Label: s.custom, Logo: s.logo}
		if s.sess != nil {
			u := s.sess.user
			e.User = &u
		}
		f.Servers = append(f.Servers, e)
	}
	sort.Slice(f.Servers, func(i, j int) bool { return f.Servers[i].URL < f.Servers[j].URL })
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(m.opts.IndexPath), 0o700) != nil {
		return
	}
	os.WriteFile(m.opts.IndexPath, b, 0o600)
}
