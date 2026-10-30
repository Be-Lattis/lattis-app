package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
)

// Tableaux croisés dynamiques et graphiques.
// Méthodes de DocumentService (document.go).

// CreatePivot crée un tableau croisé sur une nouvelle feuille ; renvoie la feuille.
func (s *DocumentService) CreatePivot(ctx context.Context, cfg canvas.PivotConfig) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	sheet, err := d.CreatePivot(ctx, cfg)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return sheet, err
}

// SetPivot change la configuration d'un tableau croisé.
func (s *DocumentService) SetPivot(ctx context.Context, id int64, cfg canvas.PivotConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetPivot(ctx, id, cfg) })
}

// SetPivotCalcField ajoute (old vide), modifie ou retire (formula vide) un
// champ calculé d'un tableau croisé (« Champ calculé… ») ; annulable.
func (s *DocumentService) SetPivotCalcField(ctx context.Context, id int64, old, name, formula string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetPivotCalcField(ctx, id, old, name, formula) })
}

// PreviewPivotFormula donne l'aperçu d'un champ calculé sur le tableau
// source (premières valeurs, ou l'erreur de la formule).
func (s *DocumentService) PreviewPivotFormula(ctx context.Context, source, formula string) (canvas.ChoicePreview, error) {
	d, err := s.current()
	if err != nil {
		return canvas.ChoicePreview{}, err
	}
	return d.PreviewPivotFormula(ctx, source, formula)
}

// DeletePivot supprime un tableau croisé.
func (s *DocumentService) DeletePivot(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeletePivot(ctx, id) })
}

// CreateChart pose un graphique sur une feuille.
func (s *DocumentService) CreateChart(ctx context.Context, sheet int64, row, col int, cfg canvas.ChartConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.CreateChart(ctx, sheet, row, col, cfg)
		return err
	})
}

// SetChart change la configuration d'un graphique.
func (s *DocumentService) SetChart(ctx context.Context, id int64, cfg canvas.ChartConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetChart(ctx, id, cfg) })
}

// MoveChart déplace un graphique.
func (s *DocumentService) MoveChart(ctx context.Context, id int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveChart(ctx, id, row, col) })
}

// DeleteChart supprime un graphique.
func (s *DocumentService) DeleteChart(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteChart(ctx, id) })
}

// PivotFieldValues donne les valeurs distinctes d'un champ d'un tableau
// croisé (colonne, colonne calculée, champ calculé, champ groupé).
func (s *DocumentService) PivotFieldValues(ctx context.Context, cfg canvas.PivotConfig, field string) ([]canvas.FilterValue, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.PivotFieldValues(ctx, cfg, field)
}

// PlaceChart déplace et redimensionne un graphique (ancrage libre).
func (s *DocumentService) PlaceChart(ctx context.Context, id int64, row, col, offX, offY, width, height int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.PlaceChart(ctx, id, row, col, offX, offY, width, height) })
}
