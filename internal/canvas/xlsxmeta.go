package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// Ce qu'un fichier .xlsx ouvert dans Lattis a et que le modèle ne garde pas,
// pour le réécrire tel quel dans le fichier (internal/transfer) : les
// tableaux tirés de plages ordinaires (l'import en fait des tableaux, le
// fichier les reprend en plages, sans tableau Excel) et le style des
// tableaux Excel. En JSON dans _lattis_info (clé « xlsx »), posé à
// l'ouverture hors du journal (l'ouverture n'est pas une étape
// d'annulation) ; par identifiant de tableau.

const xlsxKey = "xlsx"

// XLSXMeta : voir plus haut.
type XLSXMeta struct {
	// Ranges : tableaux à réécrire en plages ordinaires.
	Ranges []int64 `json:"ranges,omitempty"`
	// Styles : style des tableaux Excel (attributs de tableStyleInfo).
	Styles map[int64]string `json:"styles,omitempty"`
}

// XLSXMeta lit ce qu'a noté l'ouverture d'un fichier .xlsx (vide sinon).
func (m *Model) XLSXMeta(ctx context.Context) (XLSXMeta, error) {
	var raw string
	var out XLSXMeta
	err := m.db.Read().QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, xlsxKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	json.Unmarshal([]byte(raw), &out) // illisible : rien de noté
	return out, nil
}

// SetXLSXMeta note ce qu'a l'ouverture d'un fichier .xlsx (hors du journal).
func (m *Model) SetXLSXMeta(ctx context.Context, x XLSXMeta) error {
	b, err := json.Marshal(x)
	if err != nil {
		return err
	}
	return m.db.Write(ctx, func(tx *sql.Tx) error {
		if len(x.Ranges) == 0 && len(x.Styles) == 0 {
			_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key = ?`, xlsxKey)
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, xlsxKey, string(b))
		return err
	})
}
