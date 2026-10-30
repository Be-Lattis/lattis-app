package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/doc"
)

// Mise en forme : thème, styles, mise en forme conditionnelle, validation
// des données.
// Méthodes de DocumentService (document.go).

// SetTheme change le thème de couleur du classeur (nil : thème par défaut
// des réglages), annulable.
func (s *DocumentService) SetTheme(ctx context.Context, th *canvas.Theme) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetTheme(ctx, th) })
}

// ApplyStyle met en forme un rectangle de la feuille (patch : propriétés à
// poser, une valeur vide les retire).
func (s *DocumentService) ApplyStyle(ctx context.Context, sheet int64, r0, c0, r1, c1 int, patch map[string]any) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error {
		return d.ApplyStyle(ctx, sheet, r0, c0, r1, c1, patch)
	})
}

// ApplyStyleParts met en forme plusieurs plages de la feuille, en une seule
// étape d'annulation (Contour, Bordure épaisse en encadré).
func (s *DocumentService) ApplyStyleParts(ctx context.Context, sheet int64, parts []StylePart) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error {
		gctx := command.WithGroup(ctx, "Mise en forme")
		for _, p := range parts {
			var err error
			switch p.Line {
			case "rows":
				err = d.ApplyLineStyle(gctx, sheet, true, p.R0, p.R1, p.Patch)
			case "cols":
				err = d.ApplyLineStyle(gctx, sheet, false, p.C0, p.C1, p.Patch)
			default:
				err = d.ApplyStyle(gctx, sheet, p.R0, p.C0, p.R1, p.C1, p.Patch)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// ApplyLineStyle met en forme des lignes (rows) ou des colonnes entières
// [i0, i1] de la feuille, comme Excel.
func (s *DocumentService) ApplyLineStyle(ctx context.Context, sheet int64, rows bool, i0, i1 int, patch map[string]any) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error {
		return d.ApplyLineStyle(ctx, sheet, rows, i0, i1, patch)
	})
}

// SetRules remplace les règles de mise en forme conditionnelle d'une colonne.
func (s *DocumentService) SetRules(ctx context.Context, table string, i int, rules []canvas.Rule) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetRules(ctx, table, i, rules) })
}

// ValidationList lit la liste d'une validation tirée d'une plage (src :
// Tableau[Colonne], Cell[A1:A10], Feuille2!Cell[A1:A10] ou une cellule
// nommée), pour une cellule de la feuille sheet : valeurs distinctes dans
// l'ordre de la source, ou l'erreur qui dit pourquoi elle ne se lit pas.
func (s *DocumentService) ValidationList(ctx context.Context, sheet int64, src string) (canvas.ValidationList, error) {
	d, err := s.current()
	if err != nil {
		return canvas.ValidationList{}, err
	}
	return d.ValidationList(ctx, sheet, src)
}

// ValidationAt donne la validation des données de la cellule (dialogue
// Validation des données), formules et valeurs dans la langue de la
// session ; Type vide : aucune.
func (s *DocumentService) ValidationAt(ctx context.Context, sheet int64, row, col int) (canvas.DVRule, error) {
	d, err := s.current()
	if err != nil {
		return canvas.DVRule{}, err
	}
	return d.ValidationAt(ctx, sheet, row, col)
}

// InvalidCells donne les cellules de la feuille dont la valeur ne respecte
// pas leur validation (Entourer les données non valides).
func (s *DocumentService) InvalidCells(ctx context.Context, sheet int64) ([]canvas.CellPos, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.InvalidCells(ctx, sheet)
}

// SetSheetRules applique des changements de mise en forme conditionnelle à
// une feuille (colonnes de tableaux, lignes ou colonnes entières, cellules
// libres) en une seule étape d'annulation.
func (s *DocumentService) SetSheetRules(ctx context.Context, sheet int64, ch canvas.RuleChanges) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetRules(ctx, sheet, ch) })
}

// SetLineRules fixe la mise en forme conditionnelle de lignes (rows) ou de
// colonnes entières i0 à i1 de la feuille (aucune règle : la retire).
func (s *DocumentService) SetLineRules(ctx context.Context, sheet int64, rows bool, i0, i1 int, rules []canvas.Rule) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetLineRules(ctx, sheet, rows, i0, i1, rules) })
}
