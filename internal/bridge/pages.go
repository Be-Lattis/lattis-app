package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/doc"
)

// AddPage ajoute un onglet page HTML avant l'onglet before (0 : à la fin)
// et renvoie son identifiant.
func (s *DocumentService) AddPage(ctx context.Context, before int64) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.AddPage(ctx, before)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return id, err
}

// Page donne le contenu d'un onglet page HTML.
func (s *DocumentService) Page(ctx context.Context, sheet int64) (string, error) {
	d, err := s.current()
	if err != nil {
		return "", err
	}
	return d.Page(ctx, sheet)
}

// SavePage remplace le contenu d'un onglet page HTML (annulable).
func (s *DocumentService) SavePage(ctx context.Context, sheet int64, source string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetPage(ctx, sheet, source) })
}

// PageCall exécute une commande demandée par une page HTML (pont
// postMessage du cadre isolé) : les commandes des scripts et du serveur MCP,
// sauf fichiers, annulation et scripts.run (api.PageCall).
func (s *DocumentService) PageCall(ctx context.Context, name string, args map[string]any) (any, error) {
	return api.PageCall(ctx, host{s}, name, args)
}

// HTTPHosts liste les hôtes que les scripts et pages du classeur peuvent
// joindre sans redemander (onglet SCRIPT › Accès réseau).
func (s *DocumentService) HTTPHosts(ctx context.Context) ([]string, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.HTTPHosts(ctx)
}

// AllowHTTPHost autorise un hôte pour le classeur et renvoie la liste.
func (s *DocumentService) AllowHTTPHost(ctx context.Context, h string) ([]string, error) {
	return s.hosts(ctx, func(d *doc.Document) error { return d.AllowHTTPHost(ctx, h) })
}

// ForgetHTTPHost retire un hôte (la prochaine requête redemandera) et
// renvoie la liste.
func (s *DocumentService) ForgetHTTPHost(ctx context.Context, h string) ([]string, error) {
	return s.hosts(ctx, func(d *doc.Document) error { return d.ForgetHTTPHost(ctx, h) })
}

func (s *DocumentService) hosts(ctx context.Context, fn func(*doc.Document) error) ([]string, error) {
	if _, err := s.act(ctx, fn); err != nil {
		return nil, err
	}
	return s.HTTPHosts(ctx)
}
