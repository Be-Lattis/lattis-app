package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// Évènements du classeur (internal/api/events.go) : ouverture,
// enregistrement, fermeture, changement d'onglet, de sélection, saisie…
// Un écouteur est, comme pour un contrôle, une fonction globale d'un script
// de la bibliothèque. La liste est rangée en JSON dans _lattis_info (clé
// « events ») : elle voyage avec le fichier, et s'abonner ou se désabonner
// s'annule comme une saisie.

const eventsKey = "events"

// WorkbookListeners liste les écouteurs des évènements du classeur, dans
// l'ordre où ils ont été abonnés.
func (m *Model) WorkbookListeners(ctx context.Context) ([]Listener, error) {
	var raw string
	err := m.db.Read().QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, eventsKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || err == nil && raw == "" {
		return []Listener{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Listener{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetWorkbookListeners remplace les écouteurs des évènements du classeur.
func (m *Model) SetWorkbookListeners(ctx context.Context, ls []Listener) error {
	clean := make([]Listener, 0, len(ls))
	for _, l := range ls {
		l.Event, l.Script, l.Function = strings.ToLower(strings.TrimSpace(l.Event)), strings.TrimSpace(l.Script), strings.TrimSpace(l.Function)
		clean = append(clean, l)
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return err
	}
	now := string(b)
	if len(clean) == 0 {
		now = ""
	}
	return m.Do(ctx, "Écouteurs du classeur", func(t *tx) error {
		var old string
		err := t.row(`SELECT value FROM _lattis_info WHERE key = ?`, eventsKey).Scan(&old)
		had := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if old == now {
			return nil
		}
		set := func(v string, exists bool) stmt {
			if !exists {
				return st(`DELETE FROM _lattis_info WHERE key = ?`, eventsKey)
			}
			return st(`INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, eventsKey, v)
		}
		return t.apply([]stmt{set(now, now != "")}, []stmt{set(old, had)})
	})
}
