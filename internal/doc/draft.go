package doc

import "context"

// ID donne l'identifiant aléatoire du classeur, créé avec lui et gardé dans
// son fichier.
func (d *Document) ID() string { return d.id }

// Draft donne le brouillon du classeur (archive.Doc.Draft), sans bloquer les
// écritures.
func (d *Document) Draft(ctx context.Context, scrubbed bool, meta []byte) ([]byte, string, error) {
	return d.arc.Draft(ctx, scrubbed, meta)
}
