package canvas

import (
	"context"
	"strings"
)

// Hôtes que les scripts et les pages HTML du classeur peuvent joindre en
// HTTP (module http des scripts, internal/api/cmd_http.go). Avant la
// première requête vers un hôte, l'utilisateur est consulté. La réponse
// vaut pour la session (ici) ou, avec « mémoriser », est rangée sur ce
// poste, hors du classeur (internal/trust, doc.Document.AllowHTTPHost) :
// un classeur reçu ne peut pas s'accorder lui-même l'accès au réseau.
//
// Les versions précédentes rangeaient les hôtes dans le classeur
// (_lattis_http_hosts) : la table n'est plus lue que pour reprendre, une
// fois, les accords d'un classeur de ce poste (LegacyHTTPHosts).

// SessionHost donne la réponse de l'utilisateur pour la session ; known
// est faux s'il n'a encore rien répondu pour cet hôte.
func (m *Model) SessionHost(host string) (allowed, known bool) {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()
	allowed, known = m.session[NormalizeHost(host)]
	return allowed, known
}

// SetSessionHost retient la réponse de l'utilisateur pour la session (le
// classeur ouvert), sans la mémoriser.
func (m *Model) SetSessionHost(host string, allowed bool) {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()
	if m.session == nil {
		m.session = map[string]bool{}
	}
	m.session[NormalizeHost(host)] = allowed
}

// ForgetSessionHost oublie la réponse donnée pour la session.
func (m *Model) ForgetSessionHost(host string) {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()
	delete(m.session, NormalizeHost(host))
}

// LegacyHTTPHosts lit les hôtes enregistrés dans le classeur par une
// version précédente (vide si la table n'existe pas).
func (m *Model) LegacyHTTPHosts(ctx context.Context) ([]string, error) {
	var n int
	if err := m.db.Read().QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = '_lattis_http_hosts'`).Scan(&n); err != nil || n == 0 {
		return nil, err
	}
	rs, err := m.db.Read().QueryContext(ctx, `SELECT host FROM _lattis_http_hosts ORDER BY host`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var h string
		if err := rs.Scan(&h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rs.Err()
}

// NormalizeHost écrit un hôte comme il est rangé : en minuscules, sans
// espace autour.
func NormalizeHost(host string) string { return strings.ToLower(strings.TrimSpace(host)) }
