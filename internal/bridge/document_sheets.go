package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/doc"
)

// Onglets : feuille active, ajout, déplacement, renommage, suppression,
// couleur, masquage, onglet à choix, zoom, quadrillage, en-têtes, volets
// figés, mise en page.
// Méthodes de DocumentService (document.go).

// SetActiveSheet indique la feuille affichée : les adresses Cell_A1 sans
// feuille de l'onglet SQL s'y lisent.
func (s *DocumentService) SetActiveSheet(sheet int64) {
	s.mu.Lock()
	s.sheet = sheet
	s.mu.Unlock()
}

func (s *DocumentService) activeSheet() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sheet == 0 {
		return 1
	}
	return s.sheet
}

// AddSheet ajoute un onglet à la fin et renvoie son identifiant.
func (s *DocumentService) AddSheet(ctx context.Context) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.AddSheet(ctx)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	s.fireSheet(api.EventSheetAdd, id)
	return id, err
}

// InsertSheet ajoute un onglet avant l'onglet before et renvoie son identifiant.
func (s *DocumentService) InsertSheet(ctx context.Context, before int64) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.InsertSheet(ctx, before)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	s.fireSheet(api.EventSheetAdd, id)
	return id, err
}

// MoveSheet place un onglet au rang to (0 : premier).
func (s *DocumentService) MoveSheet(ctx context.Context, sheet int64, to int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveSheet(ctx, sheet, to) })
}

// RenameSheet et DeleteSheet agissent sur un onglet.
func (s *DocumentService) RenameSheet(ctx context.Context, sheet int64, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RenameSheetLinked(ctx, sheet, name) })
}

func (s *DocumentService) DeleteSheet(ctx context.Context, sheet int64) (DocState, error) {
	var data map[string]any
	if s.listening(api.EventSheetDelete) {
		data = api.SheetEventData(host{s}, sheet)
	}
	st, err := s.act(ctx, func(d *doc.Document) error { return d.DeleteSheet(ctx, sheet) })
	if err == nil && data != nil {
		s.fireAsync(api.EventSheetDelete, data)
	}
	return st, err
}

// CopySheet copie un onglet (tableaux, cellules, mise en forme, objets) et
// place la copie au rang to ; renvoie l'identifiant de la copie.
func (s *DocumentService) CopySheet(ctx context.Context, sheet int64, to int) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.CopySheet(ctx, sheet, to)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	s.fireSheet(api.EventSheetAdd, id)
	return id, err
}

// SetSheetColor fixe la couleur d'un onglet (« #rrggbb », vide : aucune).
func (s *DocumentService) SetSheetColor(ctx context.Context, sheet int64, color string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetColor(ctx, sheet, color) })
}

// SetSheetsColor colore les onglets d'une sélection de feuilles, en une étape
// d'annulation.
func (s *DocumentService) SetSheetsColor(ctx context.Context, sheets []int64, color string) (DocState, error) {
	gctx := command.WithGroup(ctx, "Couleur d'onglet")
	return s.act(ctx, func(d *doc.Document) error {
		for _, id := range sheets {
			if err := d.SetSheetColor(gctx, id, color); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetSheetsHidden masque ou affiche une sélection de feuilles, en une étape
// d'annulation.
func (s *DocumentService) SetSheetsHidden(ctx context.Context, sheets []int64, hidden bool) (DocState, error) {
	label := "Masquer les feuilles"
	if !hidden {
		label = "Afficher les feuilles"
	}
	gctx := command.WithGroup(ctx, label)
	return s.act(ctx, func(d *doc.Document) error {
		for _, id := range sheets {
			if err := d.SetSheetHidden(gctx, id, hidden); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetSheetHidden masque ou affiche un onglet.
func (s *DocumentService) SetSheetHidden(ctx context.Context, sheet int64, hidden bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetHidden(ctx, sheet, hidden) })
}

// SetFreeze fige les volets de la feuille (0, 0 : les libère).
func (s *DocumentService) SetFreeze(ctx context.Context, sheet int64, rows, cols int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetFreeze(ctx, sheet, rows, cols) })
}

// SheetChoiceValues donne les valeurs proposées par la flèche d'un onglet à
// choix (lignes de chacune ; 0 : valeur ajoutée sans ligne encore).
func (s *DocumentService) SheetChoiceValues(ctx context.Context, sheet int64) ([]canvas.FilterValue, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.SheetChoiceValues(ctx, sheet)
}

// SheetChoiceCandidates donne les colonnes des tableaux de la feuille que
// « Choisir par… » propose (peu de valeurs différentes).
func (s *DocumentService) SheetChoiceCandidates(ctx context.Context, sheet int64) ([]canvas.ChoiceCandidate, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.SheetChoiceCandidates(ctx, sheet)
}

// ChooseSheetBy fait de la feuille un onglet à choix par une colonne d'un de
// ses tableaux (value vide : toutes les valeurs) ; annulable.
func (s *DocumentService) ChooseSheetBy(ctx context.Context, sheet int64, table, column, value string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetChoice(ctx, sheet, table, column, value) })
}

// ChooseSheetByFormula fait de la feuille un onglet à choix par une formule
// sur les colonnes des lignes (« Choisir par › Une formule… ») ; annulable.
func (s *DocumentService) ChooseSheetByFormula(ctx context.Context, sheet int64, name, formula, value string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetChoiceFormula(ctx, sheet, name, formula, value) })
}

// PreviewChoiceFormula donne l'aperçu d'une catégorie calculée : premières
// valeurs, tableaux qui suivraient, ou l'erreur de la formule.
func (s *DocumentService) PreviewChoiceFormula(ctx context.Context, sheet int64, formula string) (canvas.ChoicePreview, error) {
	d, err := s.current()
	if err != nil {
		return canvas.ChoicePreview{}, err
	}
	return d.PreviewChoiceFormula(ctx, sheet, formula)
}

// ChooseSheetValue règle l'onglet à choix sur une valeur (all : toutes) ;
// de la navigation, hors du journal d'annulation.
func (s *DocumentService) ChooseSheetValue(ctx context.Context, sheet int64, value string, all bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ChooseSheetValue(ctx, sheet, value, all) })
}

// NewSheetChoiceValue ajoute une valeur à l'onglet à choix et l'affiche
// (« Nouvelle année… ») ; annulable.
func (s *DocumentService) NewSheetChoiceValue(ctx context.Context, sheet int64, value string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.NewSheetChoiceValue(ctx, sheet, value) })
}

// ClearSheetChoice retire l'onglet à choix de la feuille ; annulable.
func (s *DocumentService) ClearSheetChoice(ctx context.Context, sheet int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ClearSheetChoice(ctx, sheet) })
}

// SetZoom fixe le zoom de la feuille, en pour cent (10 à 400) ; hors du
// journal d'annulation.
func (s *DocumentService) SetZoom(ctx context.Context, sheet int64, pct int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetZoom(ctx, sheet, pct) })
}

// SetHeadings affiche toujours, ou seulement pendant la saisie d'une
// formule, les lettres de colonnes et numéros de lignes de la feuille.
func (s *DocumentService) SetHeadings(ctx context.Context, sheet int64, show bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetHeadings(ctx, sheet, show) })
}

// SetGridlines affiche ou masque le quadrillage de la feuille.
func (s *DocumentService) SetGridlines(ctx context.Context, sheet int64, show bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetGridlines(ctx, sheet, show) })
}

// SetPageSetup remplace la mise en page d'une feuille (onglet Mise en
// page) ; label nomme l'étape d'annulation (vide : « Mise en page »).
func (s *DocumentService) SetPageSetup(ctx context.Context, sheet int64, p canvas.PageSetup, label string) (DocState, error) {
	if label == "" {
		label = "Mise en page"
	}
	return s.act(ctx, func(d *doc.Document) error { return d.SetPageSetupAs(ctx, sheet, p, label) })
}

// DeleteSheets supprime plusieurs onglets (sélection de plusieurs onglets)
// en une seule étape d'annulation ; une feuille au moins doit rester.
func (s *DocumentService) DeleteSheets(ctx context.Context, sheets []int64) (DocState, error) {
	var events []map[string]any
	if s.listening(api.EventSheetDelete) {
		for _, id := range sheets {
			events = append(events, api.SheetEventData(host{s}, id))
		}
	}
	gctx := command.WithGroup(ctx, "Supprimer les feuilles")
	st, err := s.act(ctx, func(d *doc.Document) error {
		for _, id := range sheets {
			if err := d.DeleteSheet(gctx, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		for _, e := range events {
			s.fireAsync(api.EventSheetDelete, e)
		}
	}
	return st, err
}

// OrderSheets range les onglets dans l'ordre order (toutes les feuilles,
// masquées comprises), en une seule étape d'annulation : glisser plusieurs
// onglets sélectionnés.
func (s *DocumentService) OrderSheets(ctx context.Context, order []int64) (DocState, error) {
	gctx := command.WithGroup(ctx, "Déplacer les feuilles")
	return s.act(ctx, func(d *doc.Document) error {
		for i, id := range order {
			if err := d.MoveSheet(gctx, id, i); err != nil {
				return err
			}
		}
		return nil
	})
}
