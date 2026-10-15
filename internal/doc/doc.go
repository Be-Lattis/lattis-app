// Package doc assemble un document Slitable ouvert : l'archive et son
// répertoire de travail (internal/archive), la base migrée (internal/meta),
// le bus de commandes (internal/command) et le modèle des feuilles
// (internal/canvas).
package doc

import (
	"context"
	"errors"

	"github.com/lalamefine/slitable/internal/archive"
	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/command"
	"github.com/lalamefine/slitable/internal/meta"
	"github.com/lalamefine/slitable/internal/store"
)

// Document est un classeur ouvert.
type Document struct {
	*canvas.Model
	Bus   *command.Bus
	arc   *archive.Doc
	saved int64 // dernière commande appliquée au moment de l'enregistrement
}

func wrap(ctx context.Context, a *archive.Doc) (*Document, error) {
	if err := meta.Migrate(ctx, a.DB); err != nil {
		a.Close()
		return nil, err
	}
	bus := command.New(a.DB)
	d := &Document{Model: canvas.New(a.DB, bus), Bus: bus, arc: a}
	top, err := d.top(ctx)
	if err != nil {
		a.Close()
		return nil, err
	}
	d.saved = top
	return d, nil
}

// New crée un classeur vide, jamais enregistré.
func New(ctx context.Context) (*Document, error) {
	a, err := archive.New()
	if err != nil {
		return nil, err
	}
	d, err := wrap(ctx, a)
	if err != nil {
		return nil, err
	}
	d.saved = -1 // jamais enregistré : toujours « modifié »
	return d, nil
}

// Open ouvre un classeur. Si une session précédente de ce fichier a été
// interrompue, l'erreur est un *archive.RecoverableError : proposer Recover
// ou archive.Discard.
func Open(ctx context.Context, path string) (*Document, error) {
	a, err := archive.Open(path)
	if err != nil {
		return nil, err
	}
	return wrap(ctx, a)
}

// Recover rouvre le répertoire de travail d'une session interrompue, avec
// toutes ses modifications validées et son historique d'annulation.
func Recover(ctx context.Context, o archive.Orphan) (*Document, error) {
	a, err := archive.Recover(o)
	if err != nil {
		return nil, err
	}
	d, err := wrap(ctx, a)
	if err != nil {
		return nil, err
	}
	d.saved = -1 // les modifications récupérées ne sont pas enregistrées
	return d, nil
}

// DB donne la base du document.
func (d *Document) DB() *store.Store { return d.arc.DB }

// Path est le fichier .slitable (vide si jamais enregistré).
func (d *Document) Path() string { return d.arc.Path }

// top donne le numéro de la dernière commande appliquée (0 si aucune).
func (d *Document) top(ctx context.Context) (int64, error) {
	var seq int64
	err := d.arc.DB.Read().QueryRowContext(ctx,
		`SELECT coalesce(max(seq), 0) FROM _slitable_journal WHERE undone = 0`).Scan(&seq)
	return seq, err
}

// Dirty indique si le classeur a changé depuis son dernier enregistrement
// (annuler jusqu'à l'état enregistré le rend de nouveau « propre »).
func (d *Document) Dirty(ctx context.Context) (bool, error) {
	top, err := d.top(ctx)
	return top != d.saved, err
}

// Save enregistre dans path (vide : le fichier courant).
func (d *Document) Save(ctx context.Context, path string) error {
	if path == "" && d.arc.Path == "" {
		return errors.New("choisir un fichier pour enregistrer")
	}
	top, err := d.top(ctx)
	if err != nil {
		return err
	}
	if err := d.arc.Save(ctx, path); err != nil {
		return err
	}
	d.saved = top
	return nil
}

// Close ferme le classeur (les modifications non enregistrées sont perdues).
func (d *Document) Close() error { return d.arc.Close() }
