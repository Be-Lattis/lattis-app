// Package doc assemble un document Lattis ouvert : l'archive et son
// répertoire de travail (internal/archive), la base migrée (internal/meta),
// le bus de commandes (internal/command) et le modèle des feuilles
// (internal/canvas).
package doc

import (
	"context"
	"errors"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/store"
)

// Document est un classeur ouvert.
type Document struct {
	*canvas.Model
	Bus   *command.Bus
	arc   *archive.Doc
	saved int64 // dernière commande appliquée au moment de l'enregistrement
	// Changements de protection (hors journal) au moment de l'enregistrement.
	savedProt int64
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
	a, err := archive.New(canvas.Register)
	if err != nil {
		return nil, err
	}
	d, err := wrap(ctx, a)
	if err != nil {
		return nil, err
	}
	return d, nil // vide : rien à perdre tant qu'on n'a rien saisi
}

// Open ouvre un classeur. Si une session précédente de ce fichier a été
// interrompue, l'erreur est un *archive.RecoverableError : proposer Recover
// ou archive.Discard.
func Open(ctx context.Context, path string) (*Document, error) {
	a, err := archive.Open(path, canvas.Register)
	if err != nil {
		return nil, err
	}
	return wrap(ctx, a)
}

// Recover rouvre le répertoire de travail d'une session interrompue, avec
// toutes ses modifications validées et son historique d'annulation.
func Recover(ctx context.Context, o archive.Orphan) (*Document, error) {
	a, err := archive.Recover(o, canvas.Register)
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

// Path est le fichier .lattis (vide si jamais enregistré).
func (d *Document) Path() string { return d.arc.Path }

// top donne le numéro de la dernière commande appliquée (0 si aucune).
func (d *Document) top(ctx context.Context) (int64, error) {
	var seq int64
	err := d.arc.DB.Read().QueryRowContext(ctx,
		`SELECT coalesce(max(seq), 0) FROM _lattis_journal WHERE undone = 0`).Scan(&seq)
	return seq, err
}

// Dirty indique si le classeur a changé depuis son dernier enregistrement
// (annuler jusqu'à l'état enregistré le rend de nouveau « propre »).
func (d *Document) Dirty(ctx context.Context) (bool, error) {
	top, err := d.top(ctx)
	return top != d.saved || d.ProtectionChanges() != d.savedProt, err
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
	prot := d.ProtectionChanges()
	// Rien de changé depuis le dernier enregistrement dans ce fichier (le même
	// critère que la question à la fermeture) : le réécrire prendrait des
	// secondes sur un gros classeur et bloquerait les saisies.
	if (path == "" || path == d.arc.Path) && d.arc.Path != "" && top == d.saved && prot == d.savedProt {
		return nil
	}
	if err := d.arc.Save(ctx, path); err != nil {
		return err
	}
	d.saved, d.savedProt = top, prot
	return nil
}

// MarkSaved considère l'état courant comme enregistré, sans écrire de
// fichier (classeur de démonstration : rien à proposer à la fermeture).
func (d *Document) MarkSaved(ctx context.Context) error {
	top, err := d.top(ctx)
	if err == nil {
		d.saved, d.savedProt = top, d.ProtectionChanges()
	}
	return err
}

// Close ferme le classeur (les modifications non enregistrées sont perdues).
func (d *Document) Close() error { return d.arc.Close() }
