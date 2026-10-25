package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
)

// Graphiques sparkline d'une feuille (canvas/sparkline.go).

// CreateSparkline pose un groupe de graphiques sparkline.
func (s *DocumentService) CreateSparkline(ctx context.Context, sheet int64, cfg canvas.SparklineConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.CreateSparkline(ctx, sheet, cfg)
		return err
	})
}

// SetSparkline change un groupe de graphiques sparkline.
func (s *DocumentService) SetSparkline(ctx context.Context, id int64, cfg canvas.SparklineConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSparkline(ctx, id, cfg) })
}

// DeleteSparkline supprime un groupe de graphiques sparkline.
func (s *DocumentService) DeleteSparkline(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteSparkline(ctx, id) })
}
