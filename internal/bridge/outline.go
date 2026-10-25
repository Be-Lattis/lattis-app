package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
)

// Plan de la feuille et sous-totaux (canvas/outline.go, canvas/subtotal.go).

// GroupLines groupe les lignes (rows) ou les colonnes [i0, i1].
func (s *DocumentService) GroupLines(ctx context.Context, sheet int64, rows bool, i0, i1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.GroupLines(ctx, sheet, rows, i0, i1) })
}

// UngroupLines dissocie les lignes ou les colonnes [i0, i1].
func (s *DocumentService) UngroupLines(ctx context.Context, sheet int64, rows bool, i0, i1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UngroupLines(ctx, sheet, rows, i0, i1) })
}

// ClearOutline efface le plan de la feuille.
func (s *DocumentService) ClearOutline(ctx context.Context, sheet int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ClearOutline(ctx, sheet) })
}

// FoldGroup réduit (fold) ou rouvre le groupe de niveau level qui contient
// la ligne (ou colonne) line.
func (s *DocumentService) FoldGroup(ctx context.Context, sheet int64, rows bool, level, line int, fold bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.FoldGroup(ctx, sheet, rows, level, line, fold) })
}

// ShowOutlineLevel affiche le plan jusqu'au niveau level (boutons 1 2 3).
func (s *DocumentService) ShowOutlineLevel(ctx context.Context, sheet int64, rows bool, level int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ShowOutlineLevel(ctx, sheet, rows, level) })
}

// Subtotal pose les sous-totaux d'un tableau sur une feuille à côté ; rend
// cette feuille.
func (s *DocumentService) Subtotal(ctx context.Context, o canvas.SubtotalOptions) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.Subtotal(ctx, o)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return id, err
}
