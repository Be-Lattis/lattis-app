// Package auth connecte Lattis à un Serveur Lattis (lattisd) : découverte
// (/.well-known/lattis.json), connexion OAuth 2.0 « code d'autorisation +
// PKCE » par le navigateur du système avec redirection en boucle locale
// (RFC 8252), « device code » en secours (RFC 8628), mot de passe Lattis,
// jetons (accès en mémoire, rafraîchissement dans le trousseau) et
// déconnexion.
//
// lattisd est l'unique émetteur que PostgreSQL connaît (spécification du
// serveur, §7) : le jeton d'accès obtenu ici sert aussi à la connexion
// PostgreSQL en OAUTHBEARER (internal/remote/pgpool).
//
// Les points de la spécification du serveur que celle-ci laisse ouverts
// (forme exacte des documents JSON, paramètres, codes de refus) sont
// rassemblés dans des constantes et décrits dans
// docs/collaboration/connexion.md.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/protocol"
)

// Chemins de lattisd, client et scope par défaut, documents de découverte :
// ceux du paquet protocol, partagé avec le serveur.
const (
	PathDiscovery = protocol.PathDiscovery
	PathAuthorize = protocol.PathAuthorize
	PathToken     = protocol.PathToken
	PathDevice    = protocol.PathDevice
	PathRevoke    = protocol.PathRevoke
	// ClientID : client public de l'application de bureau.
	ClientID = protocol.DesktopClientID
	// Scope demandé quand la découverte n'en donne pas.
	Scope = protocol.DefaultScope
	// ProviderPassword : genre du fournisseur « mot de passe Lattis ».
	ProviderPassword = protocol.ProviderPassword
	// DefaultDatabase : base des classeurs quand la découverte ne la nomme pas.
	DefaultDatabase = protocol.DefaultDatabase
)

type (
	// Discovery est le document /.well-known/lattis.json.
	Discovery = protocol.Discovery
	// Provider est un fournisseur d'identité actif, dans l'ordre du serveur.
	Provider = protocol.Provider
	// Postgres décrit les points de connexion PostgreSQL annoncés.
	Postgres = protocol.Postgres
	// Endpoint est un point de connexion PostgreSQL, « hôte » ou « hôte:port ».
	Endpoint = protocol.Endpoint
)

// Strings donne des points sous forme de textes.
func Strings(eps []Endpoint) []string { return protocol.Strings(eps) }

// ErrClientTooOld refuse une application plus ancienne que la version
// minimale annoncée par le serveur.
type ErrClientTooOld struct {
	Server, Min, Have string
}

func (e *ErrClientTooOld) Error() string {
	return fmt.Sprintf("%s demande Lattis %s ou plus récent (vous avez la version %s) : mettez Lattis à jour pour vous y connecter", e.Server, e.Min, e.Have)
}

// NormalizeURL ramène l'adresse d'un serveur à sa forme canonique
// (https://hôte[:port][/chemin], sans barre finale ni requête). Le schéma
// http n'est admis que pour une adresse de bouclage (essais) ; sans schéma,
// https est supposé.
func NormalizeURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("adresse du serveur vide")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("adresse de serveur invalide : %s", raw)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	switch u.Scheme {
	case "https":
	case "http":
		if !loopbackHost(u.Hostname()) {
			return "", errors.New("un Serveur Lattis s'adresse en https://")
		}
	default:
		return "", errors.New("un Serveur Lattis s'adresse en https://")
	}
	if u.User != nil {
		return "", errors.New("l'adresse du serveur ne doit pas contenir d'identifiants")
	}
	u.Host = strings.ToLower(u.Host)
	u.RawQuery, u.Fragment, u.RawPath = "", "", ""
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}

func loopbackHost(h string) bool {
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// Host donne l'hôte (et le port) d'une adresse de serveur normalisée.
func Host(server string) string {
	u, err := url.Parse(server)
	if err != nil {
		return server
	}
	return u.Host
}

// maxDiscovery borne la taille du document de découverte, maxImage celle
// d'un logo ou d'une icône.
const (
	maxDiscovery = 1 << 20
	maxImage     = 256 << 10
)

// Discover lit la découverte d'un serveur (adresse normalisée), vérifie la
// version minimale du client (clientVersion ; "dev" ou vide : pas de
// contrôle) et ramène logo et icônes à des URI data:.
func Discover(ctx context.Context, hc *http.Client, server, clientVersion string) (*Discovery, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server+PathDiscovery, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("serveur %s injoignable : %w", Host(server), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s ne répond pas comme un Serveur Lattis (%s sur %s)", Host(server), resp.Status, PathDiscovery)
	}
	var d Discovery
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxDiscovery)).Decode(&d); err != nil {
		return nil, fmt.Errorf("%s ne répond pas comme un Serveur Lattis (découverte illisible : %v)", Host(server), err)
	}
	if strings.TrimSpace(d.Name) == "" {
		d.Name = Host(server)
	}
	if d.Postgres.Database == "" {
		d.Postgres.Database = DefaultDatabase
	}
	if d.ClientID == "" {
		d.ClientID = ClientID
	}
	if d.Scope == "" {
		d.Scope = Scope
	}
	for _, ep := range []struct {
		v    *string
		path string
	}{{&d.AuthorizationEndpoint, PathAuthorize}, {&d.TokenEndpoint, PathToken}, {&d.RevocationEndpoint, PathRevoke}, {&d.DeviceEndpoint, PathDevice}} {
		if *ep.v == "" {
			*ep.v = server + ep.path
		} else if !sameOrigin(server, *ep.v) {
			// Le navigateur et les jetons ne vont qu'au serveur choisi.
			return nil, fmt.Errorf("%s annonce un point OAuth sur un autre hôte (%s) : refusé", Host(server), *ep.v)
		}
	}
	if d.MinClientVersion != "" && !VersionAtLeast(clientVersion, d.MinClientVersion) {
		return &d, &ErrClientTooOld{Server: d.Name, Min: d.MinClientVersion, Have: clientVersion}
	}
	d.Logo = fetchImage(ctx, hc, server, d.Logo)
	for i := range d.Providers {
		d.Providers[i].Icon = fetchImage(ctx, hc, server, d.Providers[i].Icon)
		if d.Providers[i].Label == "" {
			d.Providers[i].Label = d.Providers[i].ID
		}
	}
	return &d, nil
}

// fetchImage ramène une image (adresse relative au serveur, absolue sur le
// même serveur, ou URI data:) à une URI data: ; "" si elle est absente,
// trop grosse, d'un autre hôte ou n'est pas une image.
func fetchImage(ctx context.Context, hc *http.Client, server, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if !strings.HasPrefix(ref, "/") && !strings.Contains(ref, "://") && !strings.HasPrefix(ref, "data:") {
		return "" // nom d'icône (le genre du fournisseur), pas une image
	}
	if strings.HasPrefix(ref, "data:") {
		if len(ref) <= maxImage*4/3+64 && imageType(strings.TrimPrefix(strings.SplitN(ref, ";", 2)[0], "data:")) {
			return ref
		}
		return ""
	}
	base, err := url.Parse(server + "/")
	if err != nil {
		return ""
	}
	u, err := base.Parse(ref)
	if err != nil || !strings.EqualFold(u.Host, base.Host) || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ""
	}
	resp, err := hc.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if resp.StatusCode != http.StatusOK || !imageType(ct) {
		return ""
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxImage+1))
	if err != nil || len(b) > maxImage {
		return ""
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(b)
}

// sameOrigin : même schéma et même hôte (port compris) que le serveur.
func sameOrigin(server, raw string) bool {
	a, err1 := url.Parse(server)
	b, err2 := url.Parse(raw)
	return err1 == nil && err2 == nil && strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func imageType(ct string) bool {
	switch strings.ToLower(ct) {
	case "image/png", "image/svg+xml", "image/jpeg", "image/gif", "image/webp", "image/x-icon", "image/vnd.microsoft.icon":
		return true
	}
	return false
}

// VersionAtLeast dit si have ≥ min (versions x.y.z, préfixe v et suffixe
// -rc.1 tolérés ; une pré-version précède la version finale). Une version
// de développement ("dev", vide ou illisible) passe toujours.
func VersionAtLeast(have, min string) bool {
	h, hpre, ok := parseVersion(have)
	if !ok {
		return true
	}
	m, mpre, ok := parseVersion(min)
	if !ok {
		return true
	}
	for i := range 3 {
		if h[i] != m[i] {
			return h[i] > m[i]
		}
	}
	switch {
	case hpre == "" || hpre == mpre:
		return true
	case mpre == "":
		return false
	}
	return hpre >= mpre
}

func parseVersion(v string) ([3]int, string, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	pre := ""
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		if v[i] == '-' {
			pre = v[i+1:]
			if j := strings.IndexByte(pre, '+'); j >= 0 {
				pre = pre[:j]
			}
		}
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, "", false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, "", false
		}
		out[i] = n
	}
	return out, pre, true
}
