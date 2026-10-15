// Package canvas est le prototype du modèle « canevas ↔ tables » (spike 1.2).
//
// Une feuille est un canevas libre ; une zone contiguë d'au moins deux lignes
// y devient un tableau, stocké dans une vraie table SQLite STRICT. Les
// métadonnées (feuilles, ancres, colonnes, types, cellules libres, valeurs
// invalides) vivent dans des tables _slitable_*.
//
// Toute modification passe par Doc.Do : chaque opération élémentaire exécute
// ses instructions SQL et enregistre en même temps les instructions inverses.
// Une action de l'utilisateur forme une commande (une étape d'annulation),
// rejouable à l'identique pour le rétablissement.
package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lalamefine/slitable/internal/store"
)

// stmt est une instruction SQL avec ses paramètres.
type stmt struct {
	q    string
	args []any
}

func st(q string, args ...any) stmt { return stmt{q, args} }

// step est une opération élémentaire : instructions à exécuter et inverses.
type step struct {
	do, undo []stmt
}

// Command regroupe les opérations d'une action de l'utilisateur.
type Command struct {
	Name  string
	steps []step
}

// Doc est un document ouvert, avec ses piles d'annulation et de rétablissement.
type Doc struct {
	db   *store.Store
	undo []*Command
	redo []*Command
}

// ErrNothing signale une pile d'annulation ou de rétablissement vide.
var ErrNothing = errors.New("canvas: rien à annuler ou rétablir")

// tx est la transaction d'une commande en cours de construction.
type tx struct {
	*sql.Tx
	ctx context.Context
	cmd *Command
}

// apply exécute do et enregistre l'opération avec son inverse.
func (t *tx) apply(do, undo []stmt) error {
	if err := run(t.ctx, t.Tx, do); err != nil {
		return err
	}
	t.cmd.steps = append(t.cmd.steps, step{do, undo})
	return nil
}

func (t *tx) query(q string, args ...any) (*sql.Rows, error) {
	return t.QueryContext(t.ctx, q, args...)
}

func (t *tx) row(q string, args ...any) *sql.Row { return t.QueryRowContext(t.ctx, q, args...) }

func run(ctx context.Context, tx *sql.Tx, stmts []stmt) error {
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s.q, s.args...); err != nil {
			return fmt.Errorf("%s : %w", s.q, err)
		}
	}
	return nil
}

// Do exécute une action dans une transaction. Si fn échoue, rien n'est
// appliqué ; sinon l'action rejoint la pile d'annulation et vide celle de
// rétablissement. Une action qui ne change rien n'est pas empilée.
func (d *Doc) Do(ctx context.Context, name string, fn func(*tx) error) error {
	cmd := &Command{Name: name}
	err := d.db.Write(ctx, func(sqlTx *sql.Tx) error {
		return fn(&tx{Tx: sqlTx, ctx: ctx, cmd: cmd})
	})
	if err != nil {
		return err
	}
	if len(cmd.steps) > 0 {
		d.undo = append(d.undo, cmd)
		d.redo = nil
	}
	return nil
}

// Undo annule la dernière action.
func (d *Doc) Undo(ctx context.Context) (string, error) {
	if len(d.undo) == 0 {
		return "", ErrNothing
	}
	cmd := d.undo[len(d.undo)-1]
	err := d.db.Write(ctx, func(t *sql.Tx) error {
		for i := len(cmd.steps) - 1; i >= 0; i-- {
			if err := run(ctx, t, cmd.steps[i].undo); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	d.undo = d.undo[:len(d.undo)-1]
	d.redo = append(d.redo, cmd)
	return cmd.Name, nil
}

// Redo rétablit la dernière action annulée.
func (d *Doc) Redo(ctx context.Context) (string, error) {
	if len(d.redo) == 0 {
		return "", ErrNothing
	}
	cmd := d.redo[len(d.redo)-1]
	err := d.db.Write(ctx, func(t *sql.Tx) error {
		for _, s := range cmd.steps {
			if err := run(ctx, t, s.do); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	d.redo = d.redo[:len(d.redo)-1]
	d.undo = append(d.undo, cmd)
	return cmd.Name, nil
}

// CanUndo et CanRedo indiquent l'état des piles.
func (d *Doc) CanUndo() bool { return len(d.undo) > 0 }
func (d *Doc) CanRedo() bool { return len(d.redo) > 0 }
