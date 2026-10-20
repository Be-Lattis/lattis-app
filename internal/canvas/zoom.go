package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lalamefine/lattis/internal/msg"
)

// Zoom de la feuille (Affichage › Zoom, Ctrl+molette), comme Excel : un
// réglage d'affichage enregistré avec la feuille, permis sur une feuille
// protégée. Il ne passe pas par le journal (un zoom ne s'annule pas, comme
// dans Excel) mais rend le classeur « modifié », comme un changement de
// protection (voir ProtectionChanges).

// ZoomMin et ZoomMax bornent le zoom, en pour cent.
const (
	ZoomMin = 10
	ZoomMax = 400
)

// SetZoom fixe le zoom de la feuille, en pour cent (10 à 400).
func (m *Model) SetZoom(ctx context.Context, sheet int64, pct int) error {
	if pct < ZoomMin || pct > ZoomMax {
		return fmt.Errorf("zoom : entre %d %% et %d %%", ZoomMin, ZoomMax)
	}
	changed := false
	err := m.db.Write(ctx, func(sqlTx *sql.Tx) error {
		var old int
		if err := sqlTx.QueryRowContext(ctx, `SELECT zoom FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return msg.Wrap(ErrSheet, "sheet.notFound", nil)
			}
			return err
		}
		if old == pct {
			return nil
		}
		changed = true
		_, err := sqlTx.ExecContext(ctx, `UPDATE _lattis_sheets SET zoom = ? WHERE id = ?`, pct, sheet)
		return err
	})
	if err == nil && changed {
		m.protMu.Lock()
		m.protGen++
		m.protMu.Unlock()
	}
	return err
}
