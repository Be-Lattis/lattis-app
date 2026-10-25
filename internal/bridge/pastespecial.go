package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/canvas"
)

// PasteSpecial colle une zone copiée dans Lattis selon les choix du collage
// spécial (valeurs, formules, formats, opération, transposé… : voir
// canvas.Model.PasteSpecial).
func (s *DocumentService) PasteSpecial(ctx context.Context, from int64, src canvas.Rect, to int64, dst canvas.Rect, opt canvas.PasteOptions, values [][]string) (canvas.Event, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	w := s.watchChange(to, dst.R0, dst.C0, dst.R1, dst.C1, "paste")
	wctx, t := s.track(ctx, true)
	ev, err := d.PasteSpecial(wctx, from, src, to, dst, opt, values)
	s.untrack(t, &err)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	s.fireChange(w)
	return ev, err
}

// PasteBlock colle un bloc venu d'ailleurs, avec la mise en forme lue dans la
// version HTML du presse-papiers (copie faite dans un tableur ; styles : JSON
// de chaque cellule) et les choix du collage spécial (voir
// canvas.Model.PasteBlock).
func (s *DocumentService) PasteBlock(ctx context.Context, sheet int64, dst canvas.Rect, block [][]string, styles [][]string, opt canvas.PasteOptions) (canvas.Event, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	width := 1
	for _, r := range block {
		width = max(width, len(r))
	}
	w := s.watchChange(sheet, dst.R0, dst.C0, max(dst.R1, dst.R0+len(block)-1), max(dst.C1, dst.C0+width-1), "paste")
	wctx, t := s.track(ctx, true)
	ev, err := d.PasteBlock(wctx, sheet, dst, block, styles, opt)
	s.untrack(t, &err)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	s.fireChange(w)
	return ev, err
}
