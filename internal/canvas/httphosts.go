package canvas

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Hôtes que les scripts et les pages HTML du classeur peuvent joindre en
// HTTP (module http des scripts, internal/api/cmd_http.go). Avant la
// première requête vers un hôte, l'utilisateur est consulté ; « mémoriser
// pour ce classeur » range l'hôte dans _lattis_http_hosts. Comme la
// protection, ce réglage ne passe pas par le journal (une autorisation ne
// s'annule pas) mais rend le classeur « modifié » (ProtectionChanges).

// HTTPHosts liste les hôtes mémorisés, par ordre alphabétique.
func (m *Model) HTTPHosts(ctx context.Context) ([]string, error) {
	rs, err := m.db.Read().QueryContext(ctx, `SELECT host FROM _lattis_http_hosts ORDER BY host`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []string{}
	for rs.Next() {
		var h string
		if err := rs.Scan(&h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rs.Err()
}

// HTTPHostAllowed dit si l'hôte est autorisé : mémorisé dans le classeur ou
// accordé pour la session. known est faux si l'utilisateur n'a encore rien
// répondu pour cet hôte.
func (m *Model) HTTPHostAllowed(ctx context.Context, host string) (allowed, known bool, err error) {
	host = NormalizeHost(host)
	var k int
	if err := m.db.Read().QueryRowContext(ctx, `SELECT count(*) FROM _lattis_http_hosts WHERE host = ?`, host).Scan(&k); err != nil {
		return false, false, err
	}
	if k > 0 {
		return true, true, nil
	}
	m.hostMu.Lock()
	defer m.hostMu.Unlock()
	allowed, known = m.session[host]
	return allowed, known, nil
}

// SetSessionHost retient la réponse de l'utilisateur pour la session (le
// classeur ouvert), sans la mémoriser dans le fichier.
func (m *Model) SetSessionHost(host string, allowed bool) {
	m.hostMu.Lock()
	defer m.hostMu.Unlock()
	if m.session == nil {
		m.session = map[string]bool{}
	}
	m.session[NormalizeHost(host)] = allowed
}

// AllowHTTPHost mémorise l'hôte dans le classeur.
func (m *Model) AllowHTTPHost(ctx context.Context, host string) error {
	host = NormalizeHost(host)
	if host == "" || strings.ContainsAny(host, "/ \t") {
		return errors.New("hôte invalide : nom de domaine (et port éventuel) attendu")
	}
	m.SetSessionHost(host, true)
	return m.writeHosts(ctx, `INSERT OR IGNORE INTO _lattis_http_hosts (host) VALUES (?)`, host)
}

// ForgetHTTPHost retire l'hôte du classeur et oublie la réponse donnée pour
// la session : la prochaine requête redemandera.
func (m *Model) ForgetHTTPHost(ctx context.Context, host string) error {
	host = NormalizeHost(host)
	m.hostMu.Lock()
	delete(m.session, host)
	m.hostMu.Unlock()
	return m.writeHosts(ctx, `DELETE FROM _lattis_http_hosts WHERE host = ?`, host)
}

func (m *Model) writeHosts(ctx context.Context, q, host string) error {
	changed := false
	err := m.db.Write(ctx, func(t *sql.Tx) error {
		r, err := t.ExecContext(ctx, q, host)
		if err != nil {
			return err
		}
		n, _ := r.RowsAffected()
		changed = n > 0
		return nil
	})
	if err == nil && changed {
		m.protMu.Lock()
		m.protGen++
		m.protMu.Unlock()
	}
	return err
}

// NormalizeHost écrit un hôte comme il est rangé : en minuscules, sans
// espace autour.
func NormalizeHost(host string) string { return strings.ToLower(strings.TrimSpace(host)) }
