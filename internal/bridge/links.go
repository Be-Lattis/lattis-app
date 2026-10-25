package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
)

// SetLink pose le lien hypertexte d'une cellule (adresse vide : le retire) ;
// text, s'il n'est pas vide, remplace aussi le texte affiché. Une seule
// étape d'annulation (dialogue Lien, Ctrl+K ; lien automatique d'une
// adresse tapée).
func (s *DocumentService) SetLink(ctx context.Context, sheet int64, row, col int, target, text string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetLink(ctx, sheet, row, col, target, text) })
}

// LinkPlace donne la destination d'un lien dans le classeur (« #Feuille2!B3 »,
// « #Ventes »), suivi depuis la feuille sheet.
func (s *DocumentService) LinkPlace(ctx context.Context, sheet int64, target string) (canvas.LinkPlace, error) {
	d, err := s.current()
	if err != nil {
		return canvas.LinkPlace{}, err
	}
	return d.LinkPlace(ctx, sheet, target)
}
