package formula

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// Pose de clé pour les tests du paquet, qui n'ont pas le moteur (canvas,
// tx.ensureKey) : même contrôle d'unicité et même index (KeyExpr).

// ErrNotUnique signale une colonne qui ne peut pas servir de clé.
var ErrNotUnique = errors.New("valeurs en double")

// EnsureKey vérifie qu'une colonne est unique et pose l'index UNIQUE de la
// clé, comme le moteur.
func EnsureKey(ctx context.Context, tx *sql.Tx, s *Schema, need KeyNeed) error {
	t, ok := s.Table(need.Table)
	if !ok {
		return fmt.Errorf("tableau %s introuvable", need.Table)
	}
	i, ok := t.col(need.Column)
	if !ok || t.Cols[i].Formula != nil {
		return fmt.Errorf("colonne %s inutilisable comme clé", need.Column)
	}
	c := t.Cols[i]
	expr := KeyExpr(c.SQL, c.Type)
	var total, distinct int
	if err := tx.QueryRowContext(ctx, `SELECT count(`+c.SQL+`), count(DISTINCT `+expr+`) FROM `+t.SQL).Scan(&total, &distinct); err != nil {
		return err
	}
	if total != distinct {
		return fmt.Errorf("%s.%s : %d %w", t.Name, c.Name, total-distinct, ErrNotUnique)
	}
	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS `+t.SQL+`_key ON `+t.SQL+`(`+expr+`)`); err != nil {
		return err
	}
	t.Key = c.Name
	return nil
}

func TestKeyExpr(t *testing.T) {
	for typ, want := range map[grid.Type]string{
		grid.Text:    "c_1 COLLATE NOCASE",
		grid.Date:    "c_1 COLLATE NOCASE",
		grid.Integer: "c_1",
		grid.Decimal: "c_1",
	} {
		if got := KeyExpr("c_1", typ); got != want {
			t.Errorf("KeyExpr(%s) = %q, attendu %q", typ, got, want)
		}
	}
}
