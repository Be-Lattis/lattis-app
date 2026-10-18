package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Settings est la configuration typée. Les booléens facultatifs sont des
// pointeurs : nil veut dire « pas configuré ».
type Settings struct {
	Version      int    `json:"version,omitempty"`
	Organization string `json:"organization,omitempty"`
	// ConfigURL : adresse https de la configuration distante.
	ConfigURL string `json:"configUrl,omitempty"`
	Remote    Remote `json:"remote,omitempty"`
	// AllowPolicyOverride : lu seulement dans la stratégie ; autorise
	// LATTIS_ALLOW_POLICY_OVERRIDE=1 à la faire passer sous l'environnement.
	AllowPolicyOverride bool                         `json:"allowPolicyOverride,omitempty"`
	Identity            Identity                     `json:"identity,omitempty"`
	Connections         map[string]ManagedConnection `json:"connections,omitempty"`
	AI                  AI                           `json:"ai,omitempty"`
	Features            Features                     `json:"features,omitempty"`
}

// Remote règle la configuration distante.
type Remote struct {
	// PublicKey : clé publique Ed25519 (PEM ou base64 brute) qui signe la
	// configuration distante. Posée, elle rend la signature obligatoire et
	// permet à la configuration distante de porter une stratégie.
	PublicKey string `json:"publicKey,omitempty"`
	// RefreshMinutes : intervalle de rafraîchissement pendant que Lattis
	// tourne (60 par défaut).
	RefreshMinutes int `json:"refreshMinutes,omitempty"`
}

// Identity déclare les fournisseurs d'identité (OIDC).
type Identity struct {
	Providers map[string]Provider `json:"providers,omitempty"`
}

// Provider est un fournisseur OIDC, client public (PKCE).
type Provider struct {
	Type string `json:"type,omitempty"` // oidc
	// Flavor : entra, okta, google, keycloak, generic.
	Flavor   string   `json:"flavor,omitempty"`
	Issuer   string   `json:"issuer,omitempty"`
	ClientID string   `json:"clientId,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`
	// DesktopClientSecret : Google l'exige même pour une application de
	// bureau et ne le tient pas pour confidentiel.
	DesktopClientSecret string `json:"desktopClientSecret,omitempty"`
	// HostedDomain : domaine Google Workspace attendu (revendication hd).
	HostedDomain string `json:"hostedDomain,omitempty"`
}

// ManagedConnection est une connexion fournie par l'organisation, sans
// secret ; Config a la forme de query.DBConfig.
type ManagedConnection struct {
	Name   string         `json:"name,omitempty"`
	Kind   string         `json:"kind,omitempty"`
	Config map[string]any `json:"config,omitempty"`
	Auth   Auth           `json:"auth,omitempty"`
}

// Auth dit comment s'authentifier, jamais avec quoi.
type Auth struct {
	// Mode : none, password, token, integrated, oidc.
	Mode     string   `json:"mode,omitempty"`
	Provider string   `json:"provider,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`
}

// AI règle l'onglet IA (après la v1).
type AI struct {
	Enabled  *bool  `json:"enabled,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Model    string `json:"model,omitempty"`
	Auth     Auth   `json:"auth,omitempty"`
	// DataSharing : schema-only ou sample.
	DataSharing string `json:"dataSharing,omitempty"`
}

// Features active ou coupe des fonctions.
type Features struct {
	MCP                  Toggle  `json:"mcp,omitempty"`
	LuaHTTP              LuaHTTP `json:"luaHttp,omitempty"`
	AllowUserConnections *bool   `json:"allowUserConnections,omitempty"`
	// WebPages : un Onglet HTML peut afficher une page web externe (l'option
	// « Page web… »). Coupée par défaut ; les classeurs qui en contiennent
	// déjà les affichent quand même.
	WebPages *bool `json:"webPages,omitempty"`
}

// Toggle est un interrupteur.
type Toggle struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// LuaHTTP règle le module Lua http.
type LuaHTTP struct {
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	// Restrict : seuls les domaines listés sont permis.
	Restrict *bool `json:"restrict,omitempty"`
}

// Defaults est la couche des valeurs compilées.
func Defaults() Tree {
	return Tree{
		"version": float64(1),
		"remote":  Tree{"refreshMinutes": float64(60)},
		"features": Tree{
			"mcp":                  Tree{"enabled": false},
			"allowUserConnections": true,
			"webPages":             false,
		},
		"ai": Tree{"enabled": false, "dataSharing": "schema-only"},
	}
}

var topLevel = []string{"version", "organization", "configUrl", "remote", "allowPolicyOverride",
	"identity", "connections", "ai", "features"}

func knownTopLevel(k string) bool {
	for _, t := range topLevel {
		if strings.EqualFold(t, k) {
			return true
		}
	}
	return false
}

// matchPath compare un chemin à un motif (« * » vaut un segment).
func matchPath(pattern, path string) bool {
	ps := strings.Split(strings.ToLower(pattern), ".")
	xs := strings.Split(strings.ToLower(path), ".")
	if len(ps) != len(xs) {
		return false
	}
	for i := range ps {
		if ps[i] != "*" && ps[i] != xs[i] {
			return false
		}
	}
	return true
}

func lastSegment(path string) string {
	if i := strings.LastIndexByte(path, '.'); i >= 0 {
		return path[i+1:]
	}
	return path
}

// isMaskedKey : valeur à masquer à l'affichage.
func isMaskedKey(path string) bool {
	return matchPath("identity.providers.*.desktopClientSecret", path)
}

// isSecretKey : nom de clé qui porte un secret, refusé partout.
func isSecretKey(path string) bool {
	if isMaskedKey(path) {
		return false
	}
	switch strings.ToLower(lastSegment(path)) {
	case "password", "passwd", "secret", "clientsecret", "token", "accesstoken",
		"refreshtoken", "apikey", "api_key":
		return true
	}
	return false
}

// adminOnly : clé lue seulement dans une couche écrite par un
// administrateur (sinon un utilisateur pourrait rediriger toute la
// configuration).
func adminOnly(path string) bool {
	p := strings.ToLower(path)
	return p == "configurl" || p == "remote" || strings.HasPrefix(p, "remote.")
}

type validator func(any) error

// coerce convertit une valeur venue d'un texte ou d'un DWORD (environnement,
// Registre) vers le type attendu ; ok si elle a changé.
func coerce(path string, v any) (any, bool) {
	if !boolPath(path) {
		return v, false
	}
	switch x := v.(type) {
	case float64:
		if x == 0 || x == 1 {
			return x == 1, true
		}
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "1", "yes", "oui":
			return true, true
		case "false", "0", "no", "non":
			return false, true
		}
	}
	return v, false
}

var boolPaths = []string{"allowPolicyOverride", "features.mcp.enabled", "features.allowUserConnections",
	"features.webPages", "features.luaHttp.restrict", "ai.enabled"}

func boolPath(path string) bool {
	for _, p := range boolPaths {
		if matchPath(p, path) {
			return true
		}
	}
	return false
}

var validators = []struct {
	pattern string
	check   validator
}{
	{"version", isNumber},
	{"configUrl", isHTTPSURL},
	{"remote.publicKey", isPublicKey},
	{"remote.refreshMinutes", positive},
	{"allowPolicyOverride", isBool},
	{"features.mcp.enabled", isBool},
	{"features.allowUserConnections", isBool},
	{"features.webPages", isBool},
	{"features.luaHttp.restrict", isBool},
	{"features.luaHttp.allowedDomains", isStringList},
	{"ai.enabled", isBool},
	{"ai.endpoint", isHTTPSURL},
	{"ai.dataSharing", oneOf("schema-only", "sample")},
	{"connections.*.kind", oneOf("postgres", "mysql", "sqlserver", "web")},
	{"connections.*.auth.mode", oneOf("none", "password", "token", "integrated", "oidc")},
	{"ai.auth.mode", oneOf("none", "token", "oidc")},
	{"identity.providers.*.issuer", isHTTPSURL},
	{"identity.providers.*.flavor", oneOf("entra", "okta", "google", "keycloak", "generic")},
	{"identity.providers.*.scopes", isStringList},
}

func validatorFor(path string) validator {
	for _, v := range validators {
		if matchPath(v.pattern, path) {
			return v.check
		}
	}
	return nil
}

func isBool(v any) error {
	if _, ok := v.(bool); !ok {
		return errors.New("booléen attendu (true ou false)")
	}
	return nil
}

func isNumber(v any) error {
	if _, ok := v.(float64); !ok {
		return errors.New("nombre attendu")
	}
	return nil
}

func positive(v any) error {
	if f, ok := v.(float64); !ok || f <= 0 {
		return errors.New("nombre positif attendu")
	}
	return nil
}

func isStringList(v any) error {
	l, ok := v.([]any)
	if !ok {
		return errors.New("liste de textes attendue")
	}
	for _, x := range l {
		if _, ok := x.(string); !ok {
			return errors.New("liste de textes attendue")
		}
	}
	return nil
}

func isHTTPSURL(v any) error {
	s, ok := v.(string)
	if !ok {
		return errors.New("adresse attendue")
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("adresse https:// attendue")
	}
	return nil
}

func isPublicKey(v any) error {
	s, ok := v.(string)
	if !ok {
		return errors.New("clé publique attendue")
	}
	_, err := ParsePublicKey(s)
	return err
}

func oneOf(values ...string) validator {
	return func(v any) error {
		s, ok := v.(string)
		if ok {
			for _, x := range values {
				if strings.EqualFold(s, x) {
					return nil
				}
			}
		}
		return fmt.Errorf("valeur attendue parmi : %s", strings.Join(values, ", "))
	}
}
