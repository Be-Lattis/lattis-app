package bridge

import (
	"context"

	"github.com/lalamefine/slitable/internal/api"
	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/doc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// PrintEvent porte une ligne de sortie d'un script en cours (onglet SCRIPT).
const PrintEvent = "script:print"

func init() {
	application.RegisterEvent[string](PrintEvent)
}

// ScriptResult est l'issue d'un script pour l'interface.
type ScriptResult = api.ScriptResult

// RunScript exécute un script Lua sur le classeur ouvert. Tout ce qu'il
// modifie (écritures, feuilles, tableaux…) forme une seule étape
// d'annulation, nommée d'après le script.
func (s *DocumentService) RunScript(ctx context.Context, name, src string) (ScriptResult, error) {
	return api.RunScript(host{s}, name, src, "")
}

// StopScript arrête le script en cours (bouton Arrêter).
func (s *DocumentService) StopScript() { api.StopScript() }

// RunButton exécute l'action d'un bouton (clic dans la grille).
func (s *DocumentService) RunButton(ctx context.Context, id int64) (ScriptResult, error) {
	return api.RunButton(host{s}, id)
}

// CreateButton pose un bouton d'action sur une feuille.
func (s *DocumentService) CreateButton(ctx context.Context, sheet int64, row, col int, cfg canvas.ButtonConfig) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.CreateButton(ctx, sheet, row, col, cfg)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return id, err
}

// SetButton change le libellé, le script ou la fonction d'un bouton.
func (s *DocumentService) SetButton(ctx context.Context, id int64, cfg canvas.ButtonConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetButton(ctx, id, cfg) })
}

// PlaceButton déplace et redimensionne un bouton.
func (s *DocumentService) PlaceButton(ctx context.Context, id int64, row, col, offX, offY, width, height int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.PlaceButton(ctx, id, row, col, offX, offY, width, height) })
}

// DeleteButton supprime un bouton.
func (s *DocumentService) DeleteButton(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteButton(ctx, id) })
}

// Scripts liste la bibliothèque de scripts du classeur.
func (s *DocumentService) Scripts(ctx context.Context) ([]canvas.ScriptInfo, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.Scripts(ctx)
}

// SaveScript enregistre un script (id 0 : nouveau) et renvoie son identifiant.
func (s *DocumentService) SaveScript(ctx context.Context, id int64, name, source string) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err = d.SaveScript(ctx, id, name, source)
	if err == nil {
		s.changed(ctx)
	}
	return id, err
}

// DeleteScript supprime un script.
func (s *DocumentService) DeleteScript(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteScript(ctx, id) })
}
