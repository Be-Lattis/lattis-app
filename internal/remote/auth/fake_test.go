package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// fakeLattisd imite lattisd : découverte, /oauth/authorize (redirection
// immédiate, sans page de fournisseur), /oauth/token (code + PKCE,
// rafraîchissement avec rotation, mot de passe, device code),
// /oauth/device, /oauth/revoke.
type fakeLattisd struct {
	t   *testing.T
	srv *httptest.Server

	mu         sync.Mutex
	minClient  string
	expiresIn  int64
	refuse     map[string]string // fournisseur → reason renvoyée par /oauth/authorize
	codes      map[string]authCode
	refresh    map[string]string // jeton de rafraîchissement valable → email
	used       map[string]bool   // jetons déjà tournés (réutilisation détectée)
	revoked    []string
	devicePoll int
	issued     int
	authorize  url.Values // dernière requête /oauth/authorize
	withUser   bool       // réponse de /oauth/token avec l'objet user
	iss        string     // iss renvoyé au retour ("" : l'émetteur)
	endpoint   string     // token_endpoint annoncé ("" : celui du serveur)
}

type authCode struct {
	challenge, redirect, provider string
}

func newFakeLattisd(t *testing.T) *fakeLattisd {
	f := &fakeLattisd{t: t, expiresIn: 900, refuse: map[string]string{}, codes: map[string]authCode{},
		refresh: map[string]string{}, used: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+PathDiscovery, f.discovery)
	mux.HandleFunc("GET /logo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("\x89PNG-faux"))
	})
	mux.HandleFunc("GET /icons/google.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	})
	mux.HandleFunc("GET "+PathAuthorize, f.authorizeH)
	mux.HandleFunc("POST "+PathToken, f.token)
	mux.HandleFunc("POST "+PathDevice, f.device)
	mux.HandleFunc("POST "+PathRevoke, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		f.mu.Lock()
		f.revoked = append(f.revoked, r.PostForm.Get("token"))
		delete(f.refresh, r.PostForm.Get("token"))
		f.mu.Unlock()
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeLattisd) url() string { return f.srv.URL }

func (f *fakeLattisd) discovery(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tokenEP := f.srv.URL + PathToken
	if f.endpoint != "" {
		tokenEP = f.endpoint
	}
	json.NewEncoder(w).Encode(map[string]any{
		"name":                   "Serveur Acme",
		"logo":                   "/logo.png",
		"version":                "0.1.0",
		"min_client_version":     f.minClient,
		"edition":                "admin",
		"access_mode":            "invitation",
		"issuer":                 f.srv.URL,
		"client_id":              ClientID,
		"scope":                  "openid lattis",
		"authorization_endpoint": f.srv.URL + PathAuthorize,
		"token_endpoint":         tokenEP,
		"revocation_endpoint":    f.srv.URL + PathRevoke,
		"providers": []map[string]any{
			{"id": "google", "kind": "google", "label": "Google", "icon": "/icons/google.svg"},
			{"id": "acme", "kind": "keycloak", "label": "Keycloak (Acme)", "icon": "https://ailleurs.example/i.png"},
			{"id": "password", "kind": "password", "label": "Mot de passe Lattis", "icon": "password"},
		},
		"postgres": map[string]any{"database": "lattis", "sslmode": "verify-full",
			"rw": map[string]any{"host": "pg.acme.test", "port": 5432}, "ro": []map[string]any{{"host": "pg-ro.acme.test", "port": 6432}}},
	})
}

func (f *fakeLattisd) oauthErr(w http.ResponseWriter, status int, code, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "reason": reason})
}

func (f *fakeLattisd) authorizeH(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.authorize = q
	redirect := q.Get("redirect_uri")
	ru, err := url.Parse(redirect)
	if q.Get("client_id") != ClientID || q.Get("scope") != "openid lattis" || q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" ||
		err != nil || ru.Scheme != "http" || ru.Hostname() != "127.0.0.1" || ru.Path != CallbackPath || q.Get("code_challenge") == "" {
		http.Error(w, "requête d'autorisation invalide", http.StatusBadRequest)
		return
	}
	back := url.Values{"state": {q.Get("state")}}
	if reason, ok := f.refuse[q.Get("provider")]; ok {
		back.Set("error", "access_denied")
		back.Set("reason", reason)
	} else {
		code := fmt.Sprintf("code-%d", len(f.codes)+1)
		f.codes[code] = authCode{challenge: q.Get("code_challenge"), redirect: redirect, provider: q.Get("provider")}
		back.Set("code", code)
	}
	back.Set("iss", f.srv.URL)
	if f.iss != "" {
		back.Set("iss", f.iss)
	}
	http.Redirect(w, r, redirect+"?"+back.Encode(), http.StatusFound)
}

// jwt écrit un faux jeton d'accès (signature factice : le client ne la
// vérifie pas).
func jwt(claims map[string]any) string {
	b, _ := json.Marshal(claims)
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"EdDSA"}`)) + "." + enc(b) + ".c2ln"
}

// issue émet des jetons ; mu tenu.
func (f *fakeLattisd) issue(w http.ResponseWriter, email string) {
	f.issued++
	rt := fmt.Sprintf("rt-%d", f.issued)
	f.refresh[rt] = email
	name := strings.Split(email, "@")[0]
	resp := map[string]any{
		"access_token":  jwt(map[string]any{"sub": "u-" + name, "email": email, "name": strings.ToUpper(name[:1]) + name[1:], "n": f.issued}),
		"pg_role":       email,
		"token_type":    "Bearer",
		"expires_in":    f.expiresIn,
		"refresh_token": rt,
	}
	if f.withUser {
		resp["user"] = map[string]string{"id": "u-" + name, "email": email, "display_name": "Utilisateur " + name, "pg_role": "role_" + name}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (f *fakeLattisd) token(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	p := r.PostForm
	f.mu.Lock()
	defer f.mu.Unlock()
	if p.Get("client_id") != ClientID {
		f.oauthErr(w, 401, "invalid_client", "")
		return
	}
	switch p.Get("grant_type") {
	case "authorization_code":
		c, ok := f.codes[p.Get("code")]
		delete(f.codes, p.Get("code"))
		if !ok || c.redirect != p.Get("redirect_uri") || S256(p.Get("code_verifier")) != c.challenge {
			f.oauthErr(w, 400, "invalid_grant", "")
			return
		}
		f.issue(w, "alice@acme.fr")
	case "refresh_token":
		rt := p.Get("refresh_token")
		email, ok := f.refresh[rt]
		if !ok {
			f.oauthErr(w, 400, "invalid_grant", "")
			return
		}
		delete(f.refresh, rt) // rotation
		f.used[rt] = true
		f.issue(w, email)
	case "password":
		switch {
		case p.Get("username") == "bob@acme.fr" && p.Get("password") == "secret":
			f.issue(w, "bob@acme.fr")
		case p.Get("username") == "eve@acme.fr":
			f.oauthErr(w, 400, "access_denied", "account_disabled")
		default:
			f.oauthErr(w, 400, "invalid_grant", "")
		}
	case "urn:ietf:params:oauth:grant-type:device_code":
		if p.Get("device_code") != "dev-1" {
			f.oauthErr(w, 400, "invalid_grant", "")
			return
		}
		f.devicePoll++
		switch f.devicePoll {
		case 1:
			f.oauthErr(w, 400, "authorization_pending", "")
		case 2:
			f.oauthErr(w, 400, "slow_down", "")
		default:
			f.issue(w, "carol@acme.fr")
		}
	default:
		f.oauthErr(w, 400, "unsupported_grant_type", "")
	}
}

func (f *fakeLattisd) device(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"device_code": "dev-1", "user_code": "ABCD-EFGH",
		"verification_uri": f.srv.URL + "/device", "expires_in": 600, "interval": 5,
	})
}

// browser imite le navigateur du système : suit l'adresse de connexion
// jusqu'à la redirection locale.
func browser(t *testing.T) func(string) error {
	return func(u string) error {
		go func() {
			resp, err := http.Get(u)
			if err != nil {
				t.Errorf("navigateur : %v", err)
				return
			}
			resp.Body.Close()
		}()
		return nil
	}
}
