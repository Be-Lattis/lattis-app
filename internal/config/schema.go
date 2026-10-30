package config

import (
	"errors"
	"github.com/lalamefine/lattis/internal/msg"
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
	UI                  UI                           `json:"ui,omitempty"`
	Updates             Updates                      `json:"updates,omitempty"`
	Files               Files                        `json:"files,omitempty"`
}

// Files règle l'ouverture et l'enregistrement des fichiers.
type Files struct {
	// XLSXSave : enregistrer un fichier .xlsx ouvert dans Lattis : ask
	// (demander au premier enregistrement, par défaut), keep (garder le
	// format .xlsx) ou convert (convertir en .lattis).
	XLSXSave string `json:"xlsxSave,omitempty"`
}

// Updates règle la mise à jour de l'application de bureau
// (docs/conception/mise-a-jour-automatique.md) ; sans objet en édition web.
type Updates struct {
	// Check : chercher les nouvelles versions sur GitHub (activé par défaut) ;
	// faux dans une stratégie : plus aucune requête ni proposition de mise à
	// jour, et le réglage est grisé.
	Check *bool `json:"check,omitempty"`
}

// UpdateCheck dit si l'application de bureau cherche les nouvelles versions
// (updates.check, activé par défaut).
func UpdateCheck(c *Config) bool {
	return c.Bool("updates.check", true)
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
	// HtmlPages : les pages web des classeurs (onglets HTML, CSS et
	// JavaScript, ou site externe) s'affichent et se créent. Coupées par
	// défaut, sans case dans Fichier › Paramètres (choix de Louis,
	// 21/10/2026 : le JavaScript d'une page peut faire sortir des données en
	// faisant naviguer son cadre, ce qu'aucune règle ne bloque) ; activées à
	// la main seulement (config.json, LATTIS_FEATURES__HTMLPAGES, stratégie
	// de l'organisation, verrouillable). Jamais dans la version navigateur ni
	// la démo publique, quel que soit ce réglage.
	HtmlPages *bool `json:"htmlPages,omitempty"`
	// HtmlTabs : ancien réglage des pages web (case des paramètres jusqu'à
	// la 0.3.4-beta), toléré dans les fichiers existants mais sans effet.
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
	// lecture seule ou « quand même », comme avant (décision 0011).
	CoEditing *bool `json:"coEditing,omitempty"`
	// WhatIf : Analyse de scénarios (valeur cible, table de données,
	// scénarios, solveur) dans l'onglet Données. Fonctionnalité à activer.
	WhatIf *bool `json:"whatIf,omitempty"`
	// Sparklines : création de graphiques sparkline (Insertion ›
	// Sparklines) ; ceux d'un classeur restent dessinés. À activer.
	Sparklines *bool `json:"sparklines,omitempty"`
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
			"htmlPages":            false, // à activer à la main (HtmlPages)
			"sharedWorkbooks":      true,  // activé par défaut (choix de Louis, 18/10/2026)
			// Ajout de Serveurs Lattis par l'utilisateur.
			"allowUserLattisServers": true,
			// Travail à plusieurs sur un dossier partagé (décision 0011).
			"coEditing": true,
			// Fonctionnalités à activer (Fichier › Paramètres › Fonctionnalités).
			"whatIf":     false,
			"sparklines": false,
		},
		"ai": Tree{"enabled": false, "provider": "anthropic", "dataSharing": "schema-only",
			"localUrl": "http://localhost:11434/v1"}, // Ollama
		"mcp": Tree{"confirmWrites": false},
		// Thème de couleur par défaut des classeurs (décision 0017). Langue :
		// auto suit le système (docs/i18n.md).
		"ui": Tree{"theme": DefaultTheme, "builtinThemes": true, "language": "auto", "userName": ""},
		// Recherche des nouvelles versions (bureau seulement).
		"updates": Tree{"check": true},
		// Fichier .xlsx ouvert : demander au premier enregistrement.
		"files": Tree{"xlsxSave": "ask"},
		"limits": Tree{
			"undoDepth":       float64(LimitRanges["limits.undoDepth"].Default),
			"scriptSeconds":   float64(LimitRanges["limits.scriptSeconds"].Default),
			"sqlReadSeconds":  float64(LimitRanges["limits.sqlReadSeconds"].Default),
			"sqlWriteSeconds": float64(LimitRanges["limits.sqlWriteSeconds"].Default),
		},
	}
}

var topLevel = []string{"version", "organization", "configUrl", "remote", "allowPolicyOverride",
	"identity", "connections", "ai", "mcp", "features", "limits", "ui", "updates", "files"}

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
	"features.htmlPages", "features.htmlTabs", "features.sharedWorkbooks", "features.allowUserLattisServers", "features.coEditing", "features.whatIf", "features.sparklines", "features.luaHttp.restrict", "ai.enabled",
	"ai.keepConversation", "mcp.confirmWrites", "ui.builtinThemes", "updates.check"}

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
	{"features.htmlPages", isBool},
	{"features.htmlTabs", isBool}, // ancien réglage, sans effet
	{"features.sharedWorkbooks", isBool},
	{"features.coEditing", isBool},
	{"features.whatIf", isBool},
	{"features.sparklines", isBool},
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
	{"ui.theme", isIdent},
	{"ui.language", oneOf("auto", "fr", "en", "de", "es", "zh")},
	{"ui.builtinThemes", isBool},
	{"ui.userName", isDisplayName},
	{"updates.check", isBool},
	{"files.xlsxSave", oneOf("ask", "keep", "convert")},
	{"ui.themes", isThemes},
	{"ui.themes.*", isTheme},
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
func inLimitRange(any) error { return msg.New("config.unknownSetting", nil) }

// between : un entier de lo à hi.
func between(lo, hi int) validator {
	return func(v any) error {
		f, ok := v.(float64)
		if !ok || f != float64(int(f)) || int(f) < lo || int(f) > hi {
			return msg.New("config.integerRange", msg.Args{"min": lo, "max": hi})
		}
		return nil
	}
}

func isBool(v any) error {
	if _, ok := v.(bool); !ok {
		return msg.New("config.booleanExpected", nil)
	}
	return nil
}

// isIdent : identifiant court (fournisseur, modèle) : lettres, chiffres,
// « - », « _ », « . », « : », « / » et « @ ».
func isIdent(v any) error {
	s, ok := v.(string)
	if !ok || s == "" || len(s) > 100 {
		return msg.New("config.idExpected", nil)
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.:/@", r)) {
			return msg.New("config.idChars", nil)
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
		return msg.New("config.positiveNumber", nil)
	}
	return nil
}

func isStringList(v any) error {
	l, ok := v.([]any)
	if !ok {
		return msg.New("config.textList", nil)
	}
	for _, x := range l {
		if _, ok := x.(string); !ok {
			return msg.New("config.textList", nil)
		}
	}
	return nil
}

func isHTTPSURL(v any) error {
	s, ok := v.(string)
	if !ok {
		return msg.New("config.addressExpected", nil)
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return msg.New("config.httpsExpected", nil)
	}
	return nil
}

// isDisplayName : nom affiché (auteur des commentaires et des versions) :
// court, sans caractère de contrôle ; vide : le nom de l'utilisateur du
// système.
func isDisplayName(v any) error {
	s, ok := v.(string)
	if !ok || len([]rune(s)) > 80 {
		return msg.New("config.nameExpected", nil)
	}
	for _, r := range s {
		if r < ' ' || r == 0x7f {
			return msg.New("config.nameExpected", nil)
		}
	}
	return nil
}

// isLocalOrHTTPSURL : adresse https://, ou http:// vers ce poste seulement
// (modèle local : Ollama, LM Studio…) ; rien ne sort alors en clair.
func isLocalOrHTTPSURL(v any) error {
	s, ok := v.(string)
	if !ok {
		return msg.New("config.addressExpected", nil)
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
	return msg.New("config.httpsOrLocalExpected", nil)
}

func isPublicKey(v any) error {
	s, ok := v.(string)
	if !ok {
		return msg.New("config.publicKeyExpected", nil)
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
		return msg.New("config.valueAmong", msg.Args{"values": strings.Join(values, ", ")})
	}
}
