package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/doc"
)

// Protection de la feuille, de la structure et du classeur (Révision).
// Méthodes de DocumentService (document.go).

// ProtectSheet protège une feuille (password vide : sans mot de passe) ;
// hors de l'historique d'annulation, comme Excel.
func (s *DocumentService) ProtectSheet(ctx context.Context, sheet int64, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ProtectSheet(ctx, sheet, password) })
}

// UnprotectSheet ôte la protection d'une feuille.
func (s *DocumentService) UnprotectSheet(ctx context.Context, sheet int64, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UnprotectSheet(ctx, sheet, password) })
}

// ProtectStructure protège la structure d'une feuille (seule la saisie
// dans ses tableaux reste permise) ; hors de l'historique d'annulation.
func (s *DocumentService) ProtectStructure(ctx context.Context, sheet int64, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ProtectStructure(ctx, sheet, password) })
}

// UnprotectStructure ôte la protection de la structure d'une feuille.
func (s *DocumentService) UnprotectStructure(ctx context.Context, sheet int64, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UnprotectStructure(ctx, sheet, password) })
}

// ProtectWorkbook protège la structure du classeur (onglets) ; sheets
// verrouille aussi le contenu de toutes les feuilles.
func (s *DocumentService) ProtectWorkbook(ctx context.Context, password string, sheets, structure bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ProtectWorkbook(ctx, password, sheets, structure) })
}

// UnprotectWorkbook ôte la protection de la structure du classeur.
func (s *DocumentService) UnprotectWorkbook(ctx context.Context, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UnprotectWorkbook(ctx, password) })
}
