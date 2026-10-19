package query

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// Guard borne les sources quand elles s'exécutent sur un serveur (version
// web, audit 6, I5) : sans lui, une requête pourrait lire les fichiers du
// serveur ou servir de relais vers son réseau interne (SSRF : adresses de
// bouclage, privées, de lien local comme 169.254.169.254). Il se pose sur
// le contexte (WithGuard) : run.allow le consulte avant l'accord de
// l'utilisateur, et les sources web s'y connectent (DialContext).
//
// Les pilotes de bases de données ouvrent leurs connexions eux-mêmes : pour
// eux, l'adresse est vérifiée à la résolution, avant la connexion.
type Guard struct {
	// Dir : seuls les fichiers sous ce répertoire (ceux envoyés par le
	// navigateur) sont lisibles ; vide : aucun fichier.
	Dir string
	// Hosts : hôtes permis même s'ils désignent une adresse interne
	// (liste de l'administrateur, « hôte » ou « hôte:port »).
	Hosts []string
	// resolve remplace la résolution DNS (tests).
	resolve func(ctx context.Context, host string) ([]net.IP, error)
}

// Guarded est implémenté par un hôte d'application qui borne les sources
// (moteur web).
type Guarded interface {
	QueryGuard() *Guard
}

// GuardFrom pose sur ctx le Guard de v s'il en a un.
func GuardFrom(ctx context.Context, v any) context.Context {
	if h, ok := v.(Guarded); ok {
		return WithGuard(ctx, h.QueryGuard())
	}
	return ctx
}

type guardKey struct{}

// WithGuard pose g sur le contexte des sources lues depuis ctx.
func WithGuard(ctx context.Context, g *Guard) context.Context {
	if g == nil {
		return ctx
	}
	return context.WithValue(ctx, guardKey{}, g)
}

// GuardOf donne le Guard posé sur ctx (nil : aucun).
func GuardOf(ctx context.Context) *Guard {
	g, _ := ctx.Value(guardKey{}).(*Guard)
	return g
}

// Check vérifie un accès.
func (g *Guard) Check(ctx context.Context, a Access) error {
	if a.Path != "" {
		return g.CheckPath(a.Path)
	}
	if a.Host == "" {
		return nil
	}
	host, port := a.Host, ""
	if h, p, err := net.SplitHostPort(a.Host); err == nil {
		host, port = h, p
	}
	if g.listed(host, port) {
		return nil
	}
	ips, err := g.lookup(ctx, host)
	if err != nil {
		return fmt.Errorf("%s : %w", host, err)
	}
	return checkIPs(host, ips)
}

// CheckPath vérifie qu'un fichier est sous Dir.
func (g *Guard) CheckPath(p string) error {
	deny := fmt.Errorf("%w : sur ce serveur, une requête ne lit que les fichiers envoyés depuis le navigateur (%s)", ErrDenied, baseName(p))
	if g.Dir == "" || !filepath.IsAbs(p) {
		return deny
	}
	dir, err := filepath.Abs(g.Dir)
	if err != nil {
		return deny
	}
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}
	f := filepath.Clean(p)
	if e, err := filepath.EvalSymlinks(f); err == nil {
		f = e
	}
	rel, err := filepath.Rel(dir, f)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return deny
	}
	return nil
}

func (g *Guard) listed(host, port string) bool {
	host = strings.ToLower(strings.Trim(host, "[]"))
	for _, h := range g.Hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == host || (port != "" && h == net.JoinHostPort(host, port)) {
			return true
		}
	}
	return false
}

func (g *Guard) lookup(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return []net.IP{ip}, nil
	}
	if g.resolve != nil {
		return g.resolve(ctx, host)
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, len(addrs))
	for i, a := range addrs {
		ips[i] = a.IP
	}
	return ips, nil
}

func checkIPs(host string, ips []net.IP) error {
	for _, ip := range ips {
		if InternalIP(ip) {
			return fmt.Errorf("%w : « %s » désigne une adresse interne du serveur (%s)", ErrDenied, host, ip)
		}
	}
	return nil
}

// InternalIP : bouclage, réseau privé, lien local (métadonnées des
// nuages), adresse partagée (CGNAT), non précisée ou multidiffusion.
func InternalIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
		if ip[0] == 0 || (ip[0] == 100 && ip[1]&0xc0 == 64) {
			return true
		}
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

// DialContext ouvre une connexion après avoir vérifié l'adresse jointe :
// une résolution DNS changeante (rebinding) ou une redirection vers une
// adresse interne est refusée au moment même de la connexion.
func (g *Guard) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	d := &net.Dialer{Timeout: 30 * time.Second}
	if g.listed(host, port) {
		return d.DialContext(ctx, network, addr)
	}
	ips, err := g.lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	if err := checkIPs(host, ips); err != nil {
		return nil, err
	}
	err = fmt.Errorf("%s : aucune adresse", host)
	for _, ip := range ips {
		c, derr := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if derr == nil {
			return c, nil
		}
		err = derr
	}
	return nil, err
}

// Transport donne un transport HTTP dont les connexions passent par
// DialContext (et jamais par un mandataire).
func (g *Guard) Transport() *http.Transport {
	return &http.Transport{
		DialContext:         g.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}
}
