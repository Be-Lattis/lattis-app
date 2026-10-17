package bridge

import (
	"context"
	"fmt"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/canvas"
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

// AddDoc ajoute un onglet Documentation avant l'onglet before (0 : à la
// fin) et renvoie son identifiant.
func (s *DocumentService) AddDoc(ctx context.Context, before int64) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.AddDoc(ctx, before)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return id, err
}

// ChartData calcule les données d'un graphique sans le poser sur une
// feuille (graphiques insérés dans un onglet Documentation).
func (s *DocumentService) ChartData(ctx context.Context, cfg canvas.ChartConfig) (canvas.ChartInfo, error) {
	d, err := s.current()
	if err != nil {
		return canvas.ChartInfo{}, err
	}
	return d.ChartData(ctx, cfg), nil
}

// RunFunction exécute le script name de la bibliothèque (onglet SCRIPT),
// ou seulement sa fonction entry : bouton d'un onglet Documentation.
func (s *DocumentService) RunFunction(ctx context.Context, name, entry string) (ScriptResult, error) {
	d, err := s.current()
	if err != nil {
		return ScriptResult{}, err
	}
	list, err := d.Scripts(ctx)
	if err != nil {
		return ScriptResult{}, err
	}
	for _, sc := range list {
		if sc.Name == name {
			return api.RunScript(host{s}, sc.Name, sc.Source, entry)
		}
	}
	return ScriptResult{}, fmt.Errorf("script « %s » introuvable (onglet SCRIPT)", name)
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
