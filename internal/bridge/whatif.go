package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
)

// Analyse de scénarios (Données › Analyse de scénarios) : valeur cible,
// table de données, scénarios et solveur (voir canvas/whatif.go). Les
// recherches ne changent rien ; Conserver, Afficher, la table de données et
// la synthèse sont chacun une étape d'annulation.

// GoalSeek cherche la valeur de changing qui donne goal dans target.
func (s *DocumentService) GoalSeek(ctx context.Context, sheet int64, target, goal, changing string) (canvas.GoalSeekResult, error) {
	d, err := s.current()
	if err != nil {
		return canvas.GoalSeekResult{}, err
	}
	wctx, t := s.track(ctx, true)
	r, err := d.GoalSeek(wctx, sheet, target, goal, changing)
	s.untrack(t, &err)
	return r, err
}

// Solve résout un problème du solveur.
func (s *DocumentService) Solve(ctx context.Context, sheet int64, p canvas.SolverParams) (canvas.SolverResult, error) {
	d, err := s.current()
	if err != nil {
		return canvas.SolverResult{}, err
	}
	wctx, t := s.track(ctx, true)
	r, err := d.Solve(wctx, sheet, p)
	s.untrack(t, &err)
	return r, err
}

// SetWhatIfValues pose les valeurs trouvées (Conserver) ; kind : "goal" ou
// "solver".
func (s *DocumentService) SetWhatIfValues(ctx context.Context, sheet int64, kind string, cells, raws []string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetWhatIfValues(ctx, sheet, kind, cells, raws) })
}

// DataTable calcule une table de données sur la plage (valeurs fixes).
func (s *DocumentService) DataTable(ctx context.Context, sheet int64, area canvas.Rect, rowInput, colInput string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		wctx, t := s.track(ctx, true)
		_, err := d.DataTable(wctx, sheet, area, rowInput, colInput)
		s.untrack(t, &err)
		return err
	})
}

// DataTableAt rend la table de données déjà posée sur la plage (nil :
// aucune), pour la recalculer.
func (s *DocumentService) DataTableAt(ctx context.Context, sheet int64, area canvas.Rect) (*canvas.DataTableDef, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.DataTableAt(ctx, sheet, area)
}

// Scenarios rend les scénarios de la feuille.
func (s *DocumentService) Scenarios(ctx context.Context, sheet int64) ([]canvas.Scenario, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.Scenarios(ctx, sheet)
}

// WhatIfCells vérifie des cellules variables et rend leurs valeurs actuelles.
func (s *DocumentService) WhatIfCells(ctx context.Context, sheet int64, refs string) ([]canvas.WhatIfCell, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.WhatIfCells(ctx, sheet, refs)
}

// SaveScenario ajoute (old vide) ou modifie un scénario.
func (s *DocumentService) SaveScenario(ctx context.Context, sheet int64, old string, sc canvas.Scenario) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SaveScenario(ctx, sheet, old, sc) })
}

// DeleteScenario supprime un scénario.
func (s *DocumentService) DeleteScenario(ctx context.Context, sheet int64, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteScenario(ctx, sheet, name) })
}

// ShowScenario pose les valeurs d'un scénario.
func (s *DocumentService) ShowScenario(ctx context.Context, sheet int64, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ShowScenario(ctx, sheet, name) })
}

// ScenarioSummary crée l'onglet de synthèse des scénarios et rend son
// identifiant.
func (s *DocumentService) ScenarioSummary(ctx context.Context, sheet int64, results string) (int64, error) {
	var id int64
	_, err := s.act(ctx, func(d *doc.Document) (err error) {
		id, err = d.ScenarioSummary(ctx, sheet, results)
		return err
	})
	return id, err
}
