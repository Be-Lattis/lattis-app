package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
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
	MCP                 MCP                          `json:"mcp,omitempty"`
	Features            Features                     `json:"features,omitempty"`
	Limits              Limits                       `json:"limits,omitempty"`
}

// Limits borne ce qui peut tenir l'application ou la mémoire (audit 6,
// M14) : réglables par l'utilisateur (Fichier › Paramètres) comme par la
// DSI.
type Limits struct {
	// UndoDepth : commandes gardées dans l'historique d'annulation (200).
	UndoDepth int `json:"undoDepth,omitempty"`
	// ScriptSeconds : durée maximale d'un script Lua (30).
	ScriptSeconds int `json:"scriptSeconds,omitempty"`
	// SQLReadSeconds, SQLWriteSeconds : durée maximale d'une lecture (30) et
	// d'une écriture (120) de l'onglet SQL.
	SQLReadSeconds  int `json:"sqlReadSeconds,omitempty"`
	SQLWriteSeconds int `json:"sqlWriteSeconds,omitempty"`
}

// LimitRange est la valeur par défaut et les bornes d'un réglage de Limits.
type LimitRange struct{ Default, Min, Max int }

// LimitRanges donne les réglages de Limits par chemin.
var LimitRanges = map[string]LimitRange{
	"limits.undoDepth":       {200, 10, 10000},
	"limits.scriptSeconds":   {30, 1, 3600},
	"limits.sqlReadSeconds":  {30, 1, 3600},
	"limits.sqlWriteSeconds": {120, 1, 3600},
}

// limitRange trouve le réglage de Limits d'un chemin (sans tenir compte de
// la casse).
func limitRange(path string) (LimitRange, bool) {
	for k, r := range LimitRanges {
		if strings.EqualFold(k, path) {
			return r, true
		}
	}
	return LimitRange{}, false
}

// Limit donne un réglage de Limits de c, ramené dans ses bornes.
func Limit(c *Config, path string) int {
	r, _ := limitRange(path)
	n := r.Default
	if v, ok := c.Lookup(path); ok {
		if f, ok := v.(float64); ok {
			n = int(f)
		}
	}
	return min(max(n, r.Min), r.Max)
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

// AI règle l'onglet IA (opt-in : coupé par défaut).
type AI struct {
	Enabled *bool `json:"enabled,omitempty"`
	// Provider : fournisseur du modèle (anthropic ; d'autres viendront, voir
	// internal/ai).
	Provider string `json:"provider,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Model    string `json:"model,omitempty"`
	Auth     Auth   `json:"auth,omitempty"`
	// DataSharing : schema-only, sample ou full (internal/sharing) ; vaut
	// aussi pour Claude par le serveur MCP.
	DataSharing string `json:"dataSharing,omitempty"`
	// LocalURL : adresse du modèle local (Ollama, LM Studio…), compatible
	// OpenAI ; http:// permis seulement vers ce poste (localhost,
	// 127.0.0.1, ::1).
	LocalURL string `json:"localUrl,omitempty"`
}

// MCP règle le serveur MCP intégré vu de l'utilisateur (l'activation, le
// port et le jeton sont dans mcp.json, voir internal/mcp).
type MCP struct {
	// ConfirmWrites : chaque outil qui modifie le classeur attend l'accord
	// de l'utilisateur dans Lattis (« Demander avant de modifier »).
	ConfirmWrites *bool `json:"confirmWrites,omitempty"`
}

// Features active ou coupe des fonctions.
type Features struct {
	MCP                  Toggle  `json:"mcp,omitempty"`
	LuaHTTP              LuaHTTP `json:"luaHttp,omitempty"`
	AllowUserConnections *bool   `json:"allowUserConnections,omitempty"`
	// HtmlTabs : les pages web (pages HTML, CSS et JavaScript, ou page web
	// externe) sont proposés. Fonctionnalité à activer (choix de Louis,
	// 18/10/2026) ; les classeurs qui en contiennent déjà les affichent et les
	// gardent utilisables quand même.
	HtmlTabs *bool `json:"htmlTabs,omitempty"`
	// SharedWorkbooks : classeurs partagés sur un Serveur Lattis (connexion
	// au serveur, menu Compte). Fonctionnalité à activer.
	SharedWorkbooks *bool `json:"sharedWorkbooks,omitempty"`
	// AllowUserLattisServers : l'utilisateur peut ajouter un Serveur Lattis
	// en plus de ceux de l'organisation (connections.* de genre lattis) ;
	// faux dans une stratégie : verrouillage.
	AllowUserLattisServers *bool `json:"allowUserLattisServers,omitempty"`
	// CoEditing : travail à plusieurs sur un classeur d'un dossier
	// partagé (NAS) ; désactivé, un classeur ouvert ailleurs s'ouvre en
	// lecture seule ou « quand même », comme avant (décision 0009).
	CoEditing *bool `json:"coEditing,omitempty"`
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
			"htmlTabs":             false,
			"sharedWorkbooks":      true, // activé par défaut (choix de Louis, 18/10/2026)
			// Ajout de Serveurs Lattis par l'utilisateur.
			"allowUserLattisServers": true,
			// Travail à plusieurs sur un dossier partagé (décision 0009).
			"coEditing": true,
		},
		"ai": Tree{"enabled": false, "provider": "anthropic", "dataSharing": "schema-only",
			"localUrl": "http://localhost:11434/v1"}, // Ollama
		"mcp": Tree{"confirmWrites": false},
		"limits": Tree{
			"undoDepth":       float64(LimitRanges["limits.undoDepth"].Default),
			"scriptSeconds":   float64(LimitRanges["limits.scriptSeconds"].Default),
			"sqlReadSeconds":  float64(LimitRanges["limits.sqlReadSeconds"].Default),
			"sqlWriteSeconds": float64(LimitRanges["limits.sqlWriteSeconds"].Default),
		},
	}
}

var topLevel = []string{"version", "organization", "configUrl", "remote", "allowPolicyOverride",
	"identity", "connections", "ai", "mcp", "features", "limits"}

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
	if _, ok := limitRange(path); ok {
		// Nombre venu d'un texte (variable LATTIS_*, --set).
		if s, ok := v.(string); ok {
			if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				return f, true
			}
		}
		return v, false
	}
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
	"features.htmlTabs", "features.sharedWorkbooks", "features.allowUserLattisServers", "features.coEditing", "features.luaHttp.restrict", "ai.enabled",
	"ai.keepConversation", "mcp.confirmWrites"}

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
	{"features.htmlTabs", isBool},
	{"features.sharedWorkbooks", isBool},
	{"features.coEditing", isBool},
	{"features.allowUserLattisServers", isBool},
	{"features.luaHttp.restrict", isBool},
	{"features.luaHttp.allowedDomains", isStringList},
	{"ai.enabled", isBool},
	{"ai.endpoint", isHTTPSURL},
	{"ai.localUrl", isLocalOrHTTPSURL},
	{"ai.provider", isIdent},
	{"ai.model", isIdent},
	{"ai.dataSharing", oneOf("schema-only", "sample", "full")},
	{"ai.keepConversation", isBool},
	{"mcp.confirmWrites", isBool},
	{"limits.*", inLimitRange},
	{"connections.*.kind", oneOf("postgres", "mysql", "sqlserver", "web", "lattis")},
	{"connections.*.auth.mode", oneOf("none", "password", "token", "integrated", "oidc")},
	{"ai.auth.mode", oneOf("none", "token", "oidc")},
	{"identity.providers.*.issuer", isHTTPSURL},
	{"identity.providers.*.flavor", oneOf("entra", "okta", "google", "keycloak", "generic")},
	{"identity.providers.*.scopes", isStringList},
}

func validatorFor(path string) validator {
	if r, ok := limitRange(path); ok {
		return between(r.Min, r.Max)
	}
	for _, v := range validators {
		if matchPath(v.pattern, path) {
			return v.check
		}
	}
	return nil
}

// inLimitRange : réglage inconnu de limits (les connus ont leurs bornes,
// voir validatorFor).
func inLimitRange(any) error { return errors.New("réglage inconnu") }

// between : un entier de lo à hi.
func between(lo, hi int) validator {
	return func(v any) error {
		f, ok := v.(float64)
		if !ok || f != float64(int(f)) || int(f) < lo || int(f) > hi {
			return fmt.Errorf("nombre entier de %d à %d attendu", lo, hi)
		}
		return nil
	}
}

func isBool(v any) error {
	if _, ok := v.(bool); !ok {
		return errors.New("booléen attendu (true ou false)")
	}
	return nil
}

// isIdent : identifiant court (fournisseur, modèle) : lettres, chiffres,
// « - », « _ », « . », « : », « / » et « @ ».
func isIdent(v any) error {
	s, ok := v.(string)
	if !ok || s == "" || len(s) > 100 {
		return errors.New("identifiant attendu")
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.:/@", r)) {
			return errors.New("identifiant attendu (lettres, chiffres, - _ . : / @)")
		}
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

// isLocalOrHTTPSURL : adresse https://, ou http:// vers ce poste seulement
// (modèle local : Ollama, LM Studio…) ; rien ne sort alors en clair.
func isLocalOrHTTPSURL(v any) error {
	s, ok := v.(string)
	if !ok {
		return errors.New("adresse attendue")
	}
	u, err := url.Parse(s)
	if err == nil && u.Host != "" {
		switch u.Scheme {
		case "https":
			return nil
		case "http":
			switch u.Hostname() {
			case "localhost", "127.0.0.1", "::1":
				return nil
			}
		}
	}
	return errors.New("adresse https://, ou http://localhost ou http://127.0.0.1, attendue")
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
