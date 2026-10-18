package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newManager(t *testing.T, f *fakeLattisd, store memStore, index string, c *clock) *Manager {
	if c == nil {
		c = &clock{t: time.Now()}
	}
	m := NewManager(Options{Store: store, IndexPath: index, ClientVersion: "1.2.0", OpenBrowser: browser(t), Now: c.now, LoginTimeout: 10 * time.Second})
	m.sleep = func(context.Context, time.Duration) error { return nil }
	return m
}

func TestNormalizeURL(t *testing.T) {
	for in, want := range map[string]string{
		"lattis.acme.fr":               "https://lattis.acme.fr",
		"HTTPS://Lattis.Acme.fr/":      "https://lattis.acme.fr",
		"https://acme.fr/lattis/?x=1":  "https://acme.fr/lattis",
		"http://127.0.0.1:8090":        "http://127.0.0.1:8090",
		"http://localhost:8090/":       "http://localhost:8090",
		" https://lattis.acme.fr:8443": "https://lattis.acme.fr:8443",
	} {
		if got, err := NormalizeURL(in); err != nil || got != want {
			t.Errorf("NormalizeURL(%q) = %q, %v ; attendu %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "http://lattis.acme.fr", "ftp://x", "https://moi:secret@acme.fr", "https://"} {
		if _, err := NormalizeURL(bad); err == nil {
			t.Errorf("NormalizeURL(%q) accepté", bad)
		}
	}
}

func TestVersionAtLeast(t *testing.T) {
	for _, c := range []struct {
		have, min string
		ok        bool
	}{
		{"1.2.0", "1.2.0", true},
		{"v1.10.0", "1.9.3", true},
		{"1.2.0", "1.3", false},
		{"0.9.9", "1.0.0", false},
		{"1.0.0-rc.1", "1.0.0", false},
		{"1.0.0", "1.0.0-rc.1", true},
		{"1.0.0-rc.2", "1.0.0-rc.1", true},
		{"dev", "9.9.9", true},
		{"", "1.0.0", true},
	} {
		if got := VersionAtLeast(c.have, c.min); got != c.ok {
			t.Errorf("VersionAtLeast(%q, %q) = %v", c.have, c.min, got)
		}
	}
}

func TestDiscover(t *testing.T) {
	f := newFakeLattisd(t)
	d, err := Discover(context.Background(), http.DefaultClient, f.url(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Serveur Acme" || d.Postgres.RW != "pg.acme.test:5432" || len(d.Postgres.RO) != 1 || d.Postgres.RO[0] != "pg-ro.acme.test:6432" || d.Postgres.Database != DefaultDatabase {
		t.Fatalf("découverte = %+v", d)
	}
	if !strings.HasPrefix(d.Logo, "data:image/png;base64,") {
		t.Errorf("logo = %q", d.Logo)
	}
	if len(d.Providers) != 3 || d.Providers[0].ID != "google" || d.Providers[2].Kind != ProviderPassword {
		t.Fatalf("fournisseurs (ordre du serveur) = %+v", d.Providers)
	}
	if !strings.HasPrefix(d.Providers[0].Icon, "data:image/svg+xml;base64,") {
		t.Errorf("icône Google = %q", d.Providers[0].Icon)
	}
	if d.Providers[1].Icon != "" || d.Providers[2].Icon != "" {
		t.Errorf("icône d'un autre hôte gardée : %q", d.Providers[1].Icon)
	}

	f.minClient = "2.0.0"
	_, err = Discover(context.Background(), http.DefaultClient, f.url(), "1.4.2")
	var old *ErrClientTooOld
	if !errors.As(err, &old) || !strings.Contains(err.Error(), "2.0.0") || !strings.Contains(err.Error(), "1.4.2") {
		t.Fatalf("client trop ancien : %v", err)
	}
	if _, err := Discover(context.Background(), http.DefaultClient, f.url(), "dev"); err != nil {
		t.Fatalf("version de développement refusée : %v", err)
	}
}

func TestLoginPKCEAndRestore(t *testing.T) {
	f := newFakeLattisd(t)
	store := memStore{}
	index := filepath.Join(t.TempDir(), "serveurs.json")
	c := &clock{t: time.Now()}
	m := newManager(t, f, store, index, c)
	ctx := context.Background()

	st, err := m.Login(ctx, f.url(), "google")
	if err != nil {
		t.Fatal(err)
	}
	if st.User == nil || st.User.Email != "alice@acme.fr" || st.User.PGRole != "alice@acme.fr" || st.User.DisplayName != "Alice" {
		t.Fatalf("utilisateur = %+v", st.User)
	}
	if f.authorize.Get("provider") != "google" || f.authorize.Get("scope") != "openid lattis" {
		t.Errorf("paramètres d'autorisation = %v", f.authorize)
	}
	acct := "lattis://" + Host(f.url()) + "/alice@acme.fr"
	if store[acct] != "rt-1" {
		t.Fatalf("trousseau = %v", store)
	}
	// Jeton d'accès en mémoire, pas de renouvellement tant qu'il est valable.
	tok1, err := m.Token(ctx, f.url())
	if err != nil || jwtClaims(tok1)["n"] != float64(1) {
		t.Fatalf("Token = %q, %v", tok1, err)
	}
	// Renouvelé un peu avant l'expiration, avec rotation.
	c.add(14*time.Minute + 30*time.Second)
	tok2, err := m.Token(ctx, f.url())
	if err != nil || jwtClaims(tok2)["n"] != float64(2) || store[acct] != "rt-2" {
		t.Fatalf("renouvellement : %q, %v, trousseau %v", tok2, err, store)
	}

	// Nouvelle exécution : la session revient de l'index et du trousseau ;
	// le premier jeton est demandé au serveur (reconnexion).
	m2 := newManager(t, f, store, index, c)
	servers := m2.Servers()
	if len(servers) != 1 || servers[0].User == nil || servers[0].Name != "Serveur Acme" || servers[0].Logo == "" {
		t.Fatalf("serveurs restaurés = %+v", servers)
	}
	tok3, err := m2.Token(ctx, f.url())
	if err != nil || jwtClaims(tok3)["n"] != float64(3) || store[acct] != "rt-3" {
		t.Fatalf("reconnexion : %q, %v", tok3, err)
	}
	user, disc, err := m2.Session(ctx, f.url())
	if err != nil || user.PGRole != "alice@acme.fr" || disc.Postgres.RW != "pg.acme.test:5432" {
		t.Fatalf("Session = %+v, %+v, %v", user, disc, err)
	}

	// Déconnexion : jeton révoqué et oublié.
	if err := m2.SignOut(ctx, f.url()); err != nil {
		t.Fatal(err)
	}
	if _, ok := store[acct]; ok || len(f.revoked) != 1 || f.revoked[0] != "rt-3" {
		t.Fatalf("déconnexion : trousseau %v, révoqués %v", store, f.revoked)
	}
	var lr *ErrLoginRequired
	if _, err := m2.Token(ctx, f.url()); !errors.As(err, &lr) {
		t.Fatalf("Token après déconnexion : %v", err)
	}
	if m3 := newManager(t, f, store, index, c); m3.Servers()[0].User != nil {
		t.Fatal("session restaurée après déconnexion")
	}
}

func TestUserFromTokenResponse(t *testing.T) {
	f := newFakeLattisd(t)
	f.withUser = true
	m := newManager(t, f, memStore{}, "", nil)
	st, err := m.Login(context.Background(), f.url(), "google")
	if err != nil {
		t.Fatal(err)
	}
	if st.User.DisplayName != "Utilisateur alice" || st.User.PGRole != "role_alice" {
		t.Fatalf("utilisateur = %+v", st.User)
	}
}

func TestLoginRefused(t *testing.T) {
	f := newFakeLattisd(t)
	m := newManager(t, f, memStore{}, "", nil)
	for reason, want := range map[string]string{
		"not_invited":         "invitées",
		"denied_tenant":       "organisation autorisée",
		"tenant_not_allowed":  "organisation autorisée",
		"account_disabled":    "désactivé",
		"invitation_required": "invitation",
	} {
		f.refuse["google"] = reason
		_, err := m.Login(context.Background(), f.url(), "google")
		var le *LoginError
		if !errors.As(err, &le) || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "Serveur Acme") {
			t.Errorf("refus %s : %v", reason, err)
		}
	}
	if _, err := m.Login(context.Background(), f.url(), "inconnu"); err == nil {
		t.Fatal("fournisseur inconnu accepté")
	}
}

func TestPasswordLogin(t *testing.T) {
	f := newFakeLattisd(t)
	m := newManager(t, f, memStore{}, "", nil)
	ctx := context.Background()
	if _, err := m.PasswordLogin(ctx, f.url(), "password", "bob@acme.fr", "faux"); err == nil || !strings.Contains(err.Error(), "mot de passe incorrect") {
		t.Fatalf("mauvais mot de passe : %v", err)
	}
	if _, err := m.PasswordLogin(ctx, f.url(), "password", "eve@acme.fr", "x"); err == nil || !strings.Contains(err.Error(), "désactivé") {
		t.Fatalf("compte désactivé : %v", err)
	}
	if _, err := m.PasswordLogin(ctx, f.url(), "google", "bob@acme.fr", "secret"); err == nil {
		t.Fatal("mot de passe accepté pour Google")
	}
	st, err := m.PasswordLogin(ctx, f.url(), "password", " bob@acme.fr ", "secret")
	if err != nil || st.User.Email != "bob@acme.fr" {
		t.Fatalf("connexion : %+v, %v", st, err)
	}
}

func TestDeviceFlow(t *testing.T) {
	f := newFakeLattisd(t)
	m := newManager(t, f, memStore{}, "", nil)
	ctx := context.Background()
	dc, err := m.StartDevice(ctx, f.url(), "google")
	if err != nil || dc.UserCode != "ABCD-EFGH" {
		t.Fatalf("StartDevice = %+v, %v", dc, err)
	}
	st, err := m.WaitDevice(ctx, f.url())
	if err != nil || st.User.Email != "carol@acme.fr" || f.devicePoll != 3 {
		t.Fatalf("WaitDevice = %+v, %v (interrogations : %d)", st, err, f.devicePoll)
	}
	if _, err := m.WaitDevice(ctx, f.url()); err == nil {
		t.Fatal("code consommé deux fois")
	}
}

func TestRefreshRefused(t *testing.T) {
	f := newFakeLattisd(t)
	store := memStore{}
	c := &clock{t: time.Now()}
	m := newManager(t, f, store, "", c)
	ctx := context.Background()
	if _, err := m.Login(ctx, f.url(), "google"); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.refresh = map[string]string{} // révoqué côté serveur
	f.mu.Unlock()
	c.add(time.Hour)
	_, err := m.Token(ctx, f.url())
	var lr *ErrLoginRequired
	if !errors.As(err, &lr) || !strings.Contains(err.Error(), "session expirée") {
		t.Fatalf("Token = %v", err)
	}
	if len(store) != 0 || m.Servers()[0].User != nil {
		t.Fatalf("session gardée : %v", store)
	}
}

func TestCancelLogin(t *testing.T) {
	f := newFakeLattisd(t)
	m := newManager(t, f, memStore{}, "", nil)
	m.opts.OpenBrowser = func(string) error { return nil } // l'utilisateur ne revient jamais
	done := make(chan error, 1)
	go func() {
		_, err := m.Login(context.Background(), f.url(), "google")
		done <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if st, _ := m.State(f.url()); st.Pending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("connexion jamais en attente")
		}
		time.Sleep(10 * time.Millisecond)
	}
	m.Cancel(f.url())
	if err := <-done; !errors.Is(err, ErrCanceled) {
		t.Fatalf("Login annulé : %v", err)
	}
}

func TestLoopbackIgnoresForeignState(t *testing.T) {
	lb, err := startLoopback("bon", "", "Serveur Acme")
	if err != nil {
		t.Fatal(err)
	}
	defer lb.close()
	resp, err := http.Get(lb.redirect + "?state=autre&code=vole")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("état étranger : %s", resp.Status)
	}
	select {
	case r := <-lb.result:
		t.Fatalf("code étranger accepté : %+v", r)
	default:
	}
	if !strings.HasPrefix(lb.redirect, "http://127.0.0.1:") || !strings.HasSuffix(lb.redirect, CallbackPath) {
		t.Fatalf("redirection = %s", lb.redirect)
	}
}

func TestPolicy(t *testing.T) {
	f := newFakeLattisd(t)
	other := newFakeLattisd(t)
	m := newManager(t, f, memStore{}, "", nil)
	m.opts.Policy = func() Policy {
		return Policy{Managed: []Managed{{URL: f.url() + "/", Name: "Acme (géré)", Provider: "password"}}}
	}
	ctx := context.Background()
	if _, err := m.Discover(ctx, other.url()); err == nil || !strings.Contains(err.Error(), "votre organisation") {
		t.Fatalf("serveur non géré accepté : %v", err)
	}
	servers := m.Servers()
	if len(servers) != 1 || !servers[0].Managed || servers[0].Name != "Acme (géré)" {
		t.Fatalf("serveurs gérés = %+v", servers)
	}
	st, err := m.Discover(ctx, f.url())
	if err != nil || st.ImposedProvider != "password" || len(st.Providers) != 1 || st.Providers[0].ID != "password" {
		t.Fatalf("fournisseur imposé : %+v, %v", st, err)
	}
	if _, err := m.Login(ctx, f.url(), "google"); err == nil || !strings.Contains(err.Error(), "impose") {
		t.Fatalf("autre fournisseur accepté : %v", err)
	}
	if _, err := m.PasswordLogin(ctx, f.url(), "password", "bob@acme.fr", "secret"); err != nil {
		t.Fatal(err)
	}
}

func TestPKCE(t *testing.T) {
	// Exemple de la RFC 7636, annexe B.
	if got := S256("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"); got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("S256 = %s", got)
	}
	p := NewPKCE()
	if len(p.Verifier) < 43 || p.Challenge != S256(p.Verifier) {
		t.Fatalf("PKCE = %+v", p)
	}
}

func TestContractChecks(t *testing.T) {
	// iss du retour différent de l'émetteur annoncé (RFC 9207) : refusé.
	f := newFakeLattisd(t)
	f.iss = "https://pirate.example"
	m := newManager(t, f, memStore{}, "", nil)
	if _, err := m.Login(context.Background(), f.url(), "google"); err == nil || !strings.Contains(err.Error(), "autre émetteur") {
		t.Fatalf("iss étranger : %v", err)
	}
	// Point OAuth sur un autre hôte : découverte refusée.
	g := newFakeLattisd(t)
	g.endpoint = "https://pirate.example/oauth/token"
	if _, err := Discover(context.Background(), http.DefaultClient, g.url(), "1.0.0"); err == nil || !strings.Contains(err.Error(), "autre hôte") {
		t.Fatalf("point OAuth étranger : %v", err)
	}
	// Points PostgreSQL sous forme de texte (tolérés).
	var p Postgres
	if err := json.Unmarshal([]byte(`{"rw":"pg:5433","ro":["r1",{"host":"r2"}]}`), &p); err != nil || p.RW != "pg:5433" || Strings(p.RO)[0] != "r1" || p.RO[1] != "r2" {
		t.Fatalf("Postgres = %+v, %v", p, err)
	}
}

func TestInvalidate(t *testing.T) {
	f := newFakeLattisd(t)
	m := newManager(t, f, memStore{}, "", nil)
	ctx := context.Background()
	if _, err := m.Login(ctx, f.url(), "google"); err != nil {
		t.Fatal(err)
	}
	tok1, _ := m.Token(ctx, f.url())
	m.Invalidate(f.url())
	tok2, err := m.Token(ctx, f.url())
	if err != nil || tok1 == tok2 {
		t.Fatalf("jeton non renouvelé après refus : %v", err)
	}
}
