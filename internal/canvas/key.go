package canvas

import (
	"context"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// KeyCheck dit si la colonne i peut devenir la clé du tableau (identifiant de
// ligne, index UNIQUE) : chaîne vide si oui, sinon la raison.
func (m *Model) KeyCheck(ctx context.Context, table string, i int) (string, error) {
	var why string
	err := m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		why, err = t.keyCheck(tb, i)
		return err
	})
	return why, err
}

func (t *tx) keyCheck(tb Table, i int) (string, error) {
	if i < 0 || i >= len(tb.Cols) {
		return t.say("key.whyOutOfTable", nil), nil
	}
	c := tb.Cols[i]
	if c.Formula != "" {
		return t.say("key.whyComputed", nil), nil
	}
	var total, filled, distinct int
	if err := t.row(`SELECT count(*), count(`+colSQL(c.ID)+`), count(DISTINCT `+keyExpr(c.ID, c.Type)+`) FROM `+tb.sql()).Scan(&total, &filled, &distinct); err != nil {
		return "", err
	}
	// Texte : une case vide vaut la chaîne vide, une valeur comme une autre
	// (une seule permise) ; ailleurs, toutes les cases doivent être remplies.
	if c.Type == grid.Text {
		if empty := total - filled; empty > 1 && distinct == total-empty+1 {
			return t.say("key.whyEmptyCells", msg.Args{"n": empty}), nil
		}
		if distinct < total {
			return t.say("key.whyDuplicates", msg.Args{"n": total - distinct}), nil
		}
		return "", nil
	}
	if filled < total {
		return t.say("key.whyEmpty", msg.Args{"n": total - filled}), nil
	}
	if distinct < filled {
		return t.say("key.whyDuplicates", msg.Args{"n": filled - distinct}), nil
	}
	return "", nil
}

// SetKey fait de la colonne i la clé du tableau : ses valeurs, uniques et
// toutes remplies, identifient les lignes (index UNIQUE, REFERENCE) et
// s'affichent dans la marge à la place des numéros.
func (m *Model) SetKey(ctx context.Context, table string, i int) error {
	return m.Do(ctx, "Définir la clé", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		why, err := t.keyCheck(tb, i)
		if err != nil {
			return err
		}
		if why != "" {
			return ErrKey{tb.Name, tb.Cols[i].Name, why}
		}
		if tb.Key != 0 && tb.Key != tb.Cols[i].ID {
			if err := t.dropKey(tb); err != nil {
				return err
			}
		}
		return t.ensureKey(tb.Name, tb.Cols[i].Name)
	})
}

// ClearKey retire la clé du tableau (les lignes reprennent leurs numéros).
func (m *Model) ClearKey(ctx context.Context, table string) error {
	return m.Do(ctx, "Retirer la clé", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil || tb.Key == 0 {
			return err
		}
		return t.dropKey(tb)
	})
}

func (t *tx) dropKey(tb Table) error {
	for _, c := range tb.Cols {
		if c.ID != tb.Key {
			continue
		}
		return t.apply([]stmt{
			st(`DROP INDEX ` + tb.sql() + `_key`),
			st(`UPDATE _lattis_tables SET key_col = NULL WHERE id = ?`, tb.ID),
		}, []stmt{
			st(`CREATE UNIQUE INDEX ` + tb.sql() + `_key ON ` + tb.sql() + `(` + keyExpr(c.ID, c.Type) + `)`),
			st(`UPDATE _lattis_tables SET key_col = ? WHERE id = ?`, c.ID, tb.ID),
		})
	}
	return nil
}
