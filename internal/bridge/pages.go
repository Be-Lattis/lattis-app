package bridge

import (
	"context"
	"errors"
	"fmt"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/doc"
)

// AddPage ajoute un onglet page HTML avant l'onglet before (0 : à la fin)
// et renvoie son identifiant. Les pages web sont une fonctionnalité à
// activer (Fichier › Paramètres › Fonctionnalités, features.htmlTabs).
func (s *DocumentService) AddPage(ctx context.Context, before int64) (int64, error) {
	if err := s.scripting(); err != nil {
		return 0, err
	}
	if !config.Current().Bool("features.htmlTabs", false) {
		return 0, errors.New("les pages web sont désactivés : activez-les dans Fichier › Paramètres › Fonctionnalités")
	}
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

// AddDoc ajoute un onglet Document avant l'onglet before (0 : à la
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
// feuille (graphiques insérés dans un onglet Document).
func (s *DocumentService) ChartData(ctx context.Context, cfg canvas.ChartConfig) (canvas.ChartInfo, error) {
	d, err := s.current()
	if err != nil {
		return canvas.ChartInfo{}, err
	}
	return d.ChartData(ctx, cfg), nil
}

// PivotData calcule un tableau croisé sans le poser sur une feuille
// (tableaux croisés insérés dans un onglet Document).
func (s *DocumentService) PivotData(ctx context.Context, cfg canvas.PivotConfig) (canvas.PivotInfo, error) {
	d, err := s.current()
	if err != nil {
		return canvas.PivotInfo{}, err
	}
	return d.PivotData(ctx, cfg), nil
}

// RunFunction exécute le script name de la bibliothèque (onglet SCRIPT),
// ou seulement sa fonction entry : bouton d'un onglet Document.
func (s *DocumentService) RunFunction(ctx context.Context, name, entry string) (ScriptResult, error) {
	if err := s.scripting(); err != nil {
		return ScriptResult{}, err
	}
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
	if err := s.scripting(); err != nil {
		return nil, err
	}
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

// DocControl lit un contrôle d'un document et la valeur de sa
// cellule liée.
func (s *DocumentService) DocControl(ctx context.Context, id int64) (api.ControlState, error) {
	return api.ControlOf(host{s}, id)
}

// SetControlValue écrit la valeur saisie dans un contrôle d'un Onglet
// Document dans sa cellule liée, puis déclenche son évènement change.
func (s *DocumentService) SetControlValue(ctx context.Context, id int64, raw string) (ScriptResult, error) {
	return api.SetControlValue(host{s}, id, raw)
}

// MovePivot déplace un tableau croisé entier (couper-coller).
func (s *DocumentService) MovePivot(ctx context.Context, id, sheet int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MovePivot(ctx, id, sheet, row, col) })
}

// PlacePivot pose sur la feuille un tableau croisé de configuration cfg
// (collé depuis un document).
func (s *DocumentService) PlacePivot(ctx context.Context, sheet int64, row, col int, cfg canvas.PivotConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.PlacePivot(ctx, sheet, row, col, cfg)
		return err
	})
}

// CopyPivot colle une copie d'un tableau croisé entier.
func (s *DocumentService) CopyPivot(ctx context.Context, id, sheet int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.CopyPivot(ctx, id, sheet, row, col)
		return err
	})
}
