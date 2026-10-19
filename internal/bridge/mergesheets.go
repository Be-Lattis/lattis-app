package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
)

// Regrouper les onglets aux mêmes colonnes
// (docs/conception/regrouper-onglets-semblables.md) : la détection (trait
// sous les onglets semblables, puce, bulle après un import ou à
// l'ouverture), l'aperçu du volet Regrouper et la fusion.

// SimilarSheets donne les groupes d'onglets semblables du classeur (lecture
// des métadonnées seulement).
func (s *DocumentService) SimilarSheets(ctx context.Context) ([]canvas.SheetGroup, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.SimilarSheets(ctx)
}

// MergePreview complète un plan de regroupement et en donne l'aperçu (20
// premières lignes, clé, formules à reprendre, garde-fous).
func (s *DocumentService) MergePreview(ctx context.Context, plan canvas.MergePlan) (canvas.MergePreview, error) {
	d, err := s.current()
	if err != nil {
		return canvas.MergePreview{}, err
	}
	return d.MergePreview(ctx, plan)
}

// MergeSheets regroupe les tableaux du plan en un seul, sur une nouvelle
// feuille (une étape d'annulation).
func (s *DocumentService) MergeSheets(ctx context.Context, plan canvas.MergePlan) (canvas.MergeResult, error) {
	d, err := s.current()
	if err != nil {
		return canvas.MergeResult{}, err
	}
	res, err := d.MergeSheets(ctx, plan)
	if err != nil {
		return res, err
	}
	_, err = s.changed(ctx)
	s.fireSheet(api.EventSheetAdd, res.Sheet)
	return res, err
}

// DismissSimilar : « Ne plus proposer » ce groupe d'onglets semblables
// (gardé dans le classeur, hors annulation).
func (s *DocumentService) DismissSimilar(ctx context.Context, id string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DismissSimilar(ctx, id) })
}
