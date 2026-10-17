package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ScriptInfo est un script de la bibliothèque du classeur.
type ScriptInfo struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
}

// Scripts liste la bibliothèque de scripts, par nom.
func (m *Model) Scripts(ctx context.Context) ([]ScriptInfo, error) {
	out := []ScriptInfo{}
	err := m.read(ctx, func(t *tx) error {
		rs, err := t.query(`SELECT id, name, source FROM _lattis_scripts ORDER BY name`)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var s ScriptInfo
			if err := rs.Scan(&s.ID, &s.Name, &s.Source); err != nil {
				return err
			}
			out = append(out, s)
		}
		return rs.Err()
	})
	return out, err
}

// SaveScript crée ou modifie un script (id 0 : nouveau) et renvoie son
// identifiant. Enregistrer un script s'annule comme une saisie.
func (m *Model) SaveScript(ctx context.Context, id int64, name, source string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("script : nom vide")
	}
	err := m.Do(ctx, "Enregistrer le script", func(t *tx) error {
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_scripts WHERE name = ? AND id <> ?`, name, id).Scan(&k); err != nil {
			return err
		}
		if k > 0 {
			return fmt.Errorf("script : « %s » existe déjà", name)
		}
		if id == 0 {
			if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_scripts`).Scan(&id); err != nil {
				return err
			}
			return t.apply(
				[]stmt{st(`INSERT INTO _lattis_scripts (id, name, source) VALUES (?, ?, ?)`, id, name, source)},
				[]stmt{st(`DELETE FROM _lattis_scripts WHERE id = ?`, id)},
			)
		}
		var oldName, oldSource string
		err := t.row(`SELECT name, source FROM _lattis_scripts WHERE id = ?`, id).Scan(&oldName, &oldSource)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("script %d introuvable", id)
		}
		if err != nil {
			return err
		}
		if oldName == name && oldSource == source {
			return nil
		}
		q := `UPDATE _lattis_scripts SET name = ?, source = ? WHERE id = ?`
		return t.apply([]stmt{st(q, name, source, id)}, []stmt{st(q, oldName, oldSource, id)})
	})
	return id, err
}

// DeleteScript supprime un script de la bibliothèque.
func (m *Model) DeleteScript(ctx context.Context, id int64) error {
	return m.Do(ctx, "Supprimer le script", func(t *tx) error {
		var name, source string
		if err := t.row(`SELECT name, source FROM _lattis_scripts WHERE id = ?`, id).Scan(&name, &source); err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`DELETE FROM _lattis_scripts WHERE id = ?`, id)},
			[]stmt{st(`INSERT INTO _lattis_scripts (id, name, source) VALUES (?, ?, ?)`, id, name, source)},
		)
	})
}
