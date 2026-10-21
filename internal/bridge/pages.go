package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
)

// htmlPages dit si les pages web des classeurs sont permises : réglage
// features.htmlPages (coupé par défaut, sans case dans les paramètres :
// configuration à la main ou stratégie de l'organisation), et jamais dans
// la version web ni la démo publique, où rien n'empêche le JavaScript
// d'une page de faire sortir des données en faisant naviguer son cadre.
// Coupées, les onglets existants restent dans le classeur mais
// l'interface ne les affiche pas.
func (s *DocumentService) htmlPages() bool {
	p := s.sys().Platform()
	return !p.Web && !p.Demo && !publicDemo.Load() && api.HTMLPagesConfigured()
}

// AddPage ajoute un onglet page HTML avant l'onglet before (0 : à la fin)
// et renvoie son identifiant ; refusé quand les pages web sont coupées
// (htmlPages).
func (s *DocumentService) AddPage(ctx context.Context, before int64) (int64, error) {
	if !s.htmlPages() {
		return 0, api.ErrPagesDisabled
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

// LocateRow situe une ligne de tableau sur sa feuille, d'après son
// identifiant (point de carte cliqué : ChartInfo.Rows).
func (s *DocumentService) LocateRow(ctx context.Context, table string, id int64) (canvas.RowSpot, error) {
	d, err := s.current()
	if err != nil {
		return canvas.RowSpot{}, err
	}
	return d.LocateRow(ctx, table, id)
}

// ReferenceRow : la ligne visée par une cellule de colonne de référence
// (validation tirée de la clé d'un tableau) qui vaut value, pour « Basculer
// sur ». Table vide : src n'est pas la colonne clé d'un tableau.
func (s *DocumentService) ReferenceRow(ctx context.Context, src, value string) (canvas.KeyRow, error) {
	d, err := s.current()
	if err != nil {
		return canvas.KeyRow{}, err
	}
	return d.ReferenceRow(ctx, src, value)
}

// Citations : colonnes de référence qui citent la valeur value de la clé du
// tableau table (nombre de lignes, première ligne).
func (s *DocumentService) Citations(ctx context.Context, table, value string) ([]canvas.Citation, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.Citations(ctx, table, value)
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

// RunFunction exécute le script name de la bibliothèque (onglet Script),
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
			return api.RunLibraryScript(host{s}, sc.Name, sc.Source, entry)
		}
	}
	return ScriptResult{}, msg.New("pages.scriptNotFound", msg.Args{"script": name})
}

// Page donne le contenu d'un onglet page HTML.
func (s *DocumentService) Page(ctx context.Context, sheet int64) (string, error) {
	d, err := s.current()
	if err != nil {
		return "", err
	}
	return d.Page(ctx, sheet)
}

// PageAllowed dit si cette version d'une page web (son HTML) peut
// s'afficher sur ce poste : créée ou modifiée ici, ou déjà autorisée.
// Sinon (classeur reçu), l'interface demande l'accord de l'utilisateur au
// lieu d'exécuter la page.
func (s *DocumentService) PageAllowed(ctx context.Context, source string) bool {
	return doc.PageAllowed(source)
}

// AllowPage autorise sur ce poste cette version de la page web de
// l'onglet sheet (réponse « Autoriser »).
func (s *DocumentService) AllowPage(ctx context.Context, sheet int64, source string) error {
	d, err := s.current()
	if err != nil {
		return err
	}
	return d.AllowPage(ctx, sheet, source)
}

// SavePage remplace le contenu d'un onglet page HTML ou Document
// (annulable) ; une page web, seulement quand elles sont permises.
func (s *DocumentService) SavePage(ctx context.Context, sheet int64, source string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		if k, err := d.PageKind(ctx, sheet); err != nil {
			return err
		} else if k == canvas.KindHTML && !s.htmlPages() {
			return api.ErrPagesDisabled
		}
		return d.SetPage(ctx, sheet, source)
	})
}

// PageCall exécute une commande demandée par une page HTML (pont
// postMessage du cadre isolé) : les commandes de la liste autorisée aux
// pages (api.PageCall). Pages web coupées (htmlPages, toujours dans la
// version web et la démo publique) : tout est refusé.
func (s *DocumentService) PageCall(ctx context.Context, name string, args map[string]any) (any, error) {
	if !s.htmlPages() {
		return nil, api.ErrPagesDisabled
	}
	if err := s.scripting(); err != nil {
		return nil, err
	}
	return api.PageCall(ctx, host{s}, name, args)
}

// HTTPHosts liste les hôtes que les scripts et pages du classeur peuvent
// joindre sans redemander (onglet Script › Accès réseau).
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

// CreatePivotAt crée un tableau croisé, son coin haut gauche en (row, col)
// de la feuille (Insertion › Tableau croisé, puis un clic sur la cellule).
func (s *DocumentService) CreatePivotAt(ctx context.Context, sheet int64, row, col int, cfg canvas.PivotConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.CreatePivotAt(ctx, sheet, row, col, cfg)
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
