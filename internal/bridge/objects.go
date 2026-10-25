package bridge

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
)

// Images, formes et zones de texte d'une feuille (canvas/objects.go).

// CreateObject pose une forme, une zone de texte ou (cfg.Image) une image
// déjà enregistrée dans le classeur ; rend l'identifiant de l'objet.
func (s *DocumentService) CreateObject(ctx context.Context, sheet int64, row, col int, cfg canvas.ObjectConfig) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.CreateObject(ctx, sheet, row, col, cfg)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return id, err
}

// CreateImage enregistre une image (data : contenu en base64, ou adresse
// data:…;base64,…) et la pose sur la feuille ; cfg donne la taille posée
// et le texte de remplacement (Alt). Rend l'identifiant de l'objet.
func (s *DocumentService) CreateImage(ctx context.Context, sheet int64, row, col int, data string, cfg canvas.ObjectConfig) (int64, error) {
	if i := strings.Index(data, ";base64,"); strings.HasPrefix(data, "data:") && i > 0 {
		data = data[i+len(";base64,"):]
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return 0, msg.Wrap(canvas.ErrObject, "object.imageUnreadable", nil)
	}
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.CreateImage(ctx, sheet, row, col, raw, cfg)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return id, err
}

// SetObject change la présentation d'un objet (couleurs, trait, texte).
func (s *DocumentService) SetObject(ctx context.Context, id int64, cfg canvas.ObjectConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetObject(ctx, id, cfg) })
}

// PlaceObject déplace et redimensionne un objet.
func (s *DocumentService) PlaceObject(ctx context.Context, id int64, row, col, offX, offY, width, height int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.PlaceObject(ctx, id, row, col, offX, offY, width, height) })
}

// PlaceLine place une ligne ou une flèche par sa boîte et son sens.
func (s *DocumentService) PlaceLine(ctx context.Context, id int64, row, col, offX, offY, width, height int, flipH, flipV bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		return d.PlaceLine(ctx, id, row, col, offX, offY, width, height, flipH, flipV)
	})
}

// OrderObject met un objet au premier plan (front) ou à l'arrière-plan.
func (s *DocumentService) OrderObject(ctx context.Context, id int64, front bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.OrderObject(ctx, id, front) })
}

// DeleteObject supprime un objet.
func (s *DocumentService) DeleteObject(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteObject(ctx, id) })
}

// ImageData rend une image du classeur en adresse data: (affichage).
func (s *DocumentService) ImageData(ctx context.Context, hash string) (string, error) {
	d, err := s.current()
	if err != nil {
		return "", err
	}
	data, typ, err := d.ImageData(ctx, hash)
	if err != nil {
		return "", err
	}
	return "data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
