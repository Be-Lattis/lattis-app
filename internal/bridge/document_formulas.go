package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/work"
)

// Formules et requêtes : aide, rechercher et remplacer, recalcul,
// dépendances, requêtes SQL et favoris.
// Méthodes de DocumentService (document.go).

// Query exécute une requête SQL en lecture seule sur le classeur (onglet SQL).
func (s *DocumentService) Query(ctx context.Context, q string) (_ grid.QueryResult, err error) {
	d, err := s.current()
	if err != nil {
		return grid.QueryResult{}, err
	}
	ctx, t := s.track(ctx, true)
	defer s.untrack(t, &err)
	work.Op(ctx, "query", "")
	if q, err = d.CellSQL(ctx, q, s.activeSheet()); err != nil {
		return grid.QueryResult{}, err
	}
	// Les tableaux croisés cités s'interrogent comme des vues.
	if q, err = d.ReadCTE(ctx, q); err != nil {
		return grid.QueryResult{}, err
	}
	qctx, done := s.sqlContext(ctx, 0) // délai de grid.Query
	defer done()
	res, err := grid.Query(qctx, d.DB().Read(), q, QueryLimit)
	res = res.WithoutTechnical() // _pos… : jamais dans l'aperçu
	return res, sqlErr(qctx, err)
}

// QueryToSheet exécute une requête de lecture (onglet SQL) et pose tout son
// résultat dans un nouveau tableau, sur une nouvelle feuille nommée name
// (vide : « Requête1 »…) ; headers : en-têtes affichés. Annulable.
func (s *DocumentService) QueryToSheet(ctx context.Context, q, name string, headers []string) (QuerySheet, error) {
	d, err := s.current()
	if err != nil {
		return QuerySheet{}, err
	}
	if q, err = d.CellSQL(ctx, q, s.activeSheet()); err != nil {
		return QuerySheet{}, err
	}
	if q, err = d.ReadCTE(ctx, q); err != nil {
		return QuerySheet{}, err
	}
	qctx, done := s.sqlContext(ctx, sqlWriteTimeout())
	res, err := d.QueryToSheet(qctx, q, name, headers)
	err = sqlErr(qctx, err)
	done()
	if err != nil {
		return QuerySheet{}, err
	}
	out := QuerySheet{}
	if len(res.Sheets) > 0 {
		out.Sheet = res.Sheets[0]
	}
	if len(res.Tables) > 0 {
		out.Table = res.Tables[0]
	}
	out.State, err = s.changed(ctx)
	return out, err
}

// FormulaHelp renvoie le catalogue de l'aide à la saisie.
func (s *DocumentService) FormulaHelp(ctx context.Context) (FormulaHelp, error) {
	lang := s.sessionLang()
	h := FormulaHelp{Functions: formula.HelpCatalog(formula.ForLanguage(string(lang)), lang), Tables: []canvas.TableNames{}}
	d, err := s.current()
	if err != nil {
		return h, nil
	}
	h.Tables, err = d.AllTables(ctx)
	return h, err
}

// Find cherche un texte dans une feuille (cellules dans l'ordre de lecture),
// limité à la zone in quand elle est donnée (la sélection) ; nil : toute la
// feuille.
func (s *DocumentService) Find(ctx context.Context, sheet int64, text string, in *canvas.Rect) ([]canvas.Match, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.FindIn(ctx, sheet, text, in)
}

// Replace remplace un texte dans la zone in (la sélection, ou la cellule
// trouvée), ou dans toute la feuille si elle est nil, et renvoie le nombre
// de cellules modifiées.
func (s *DocumentService) Replace(ctx context.Context, sheet int64, text, by string, in *canvas.Rect) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	n, err := d.ReplaceIn(ctx, sheet, text, by, in)
	if err == nil && n > 0 {
		s.changed(ctx)
	}
	return n, err
}

// Search cherche avec les options du dialogue Rechercher (casse, cellule
// entière, jokers, valeurs, formules ou notes, par lignes ou par colonnes)
// dans une feuille ; dans tout le classeur, une feuille par appel (Next).
func (s *DocumentService) Search(ctx context.Context, q canvas.SearchQuery) (canvas.SearchResult, error) {
	d, err := s.current()
	if err != nil {
		return canvas.SearchResult{}, err
	}
	return d.Search(ctx, q)
}

// ReplaceAll remplace avec les mêmes options (la feuille, la sélection, une
// cellule ou tout le classeur), en une étape d'annulation ; les cellules
// protégées ou en lecture seule sont laissées et comptées.
func (s *DocumentService) ReplaceAll(ctx context.Context, q canvas.SearchQuery, by string) (canvas.ReplaceResult, error) {
	d, err := s.current()
	if err != nil {
		return canvas.ReplaceResult{}, err
	}
	res, err := d.ReplaceAll(ctx, q, by)
	if err == nil && res.Replaced > 0 {
		s.changed(ctx)
	}
	return res, err
}

// Recalc renouvelle AUJOURDHUI, MAINTENANT et ALEA (F9).
func (s *DocumentService) Recalc(ctx context.Context) (DocState, error) {
	st, err := s.act(ctx, func(d *doc.Document) error {
		d.Recalc()
		return nil
	})
	if err == nil && s.listening(api.EventRecalc) {
		s.fireAsync(api.EventRecalc, map[string]any{})
	}
	return st, err
}

// Dependencies donne les antécédents et les dépendants d'une cellule.
func (s *DocumentService) Dependencies(ctx context.Context, sheet int64, row, col int) (canvas.Dependencies, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Dependencies{}, err
	}
	return d.Dependencies(ctx, sheet, row, col)
}

// Favorites liste les requêtes SQL favorites du classeur.
func (s *DocumentService) Favorites(ctx context.Context) ([]canvas.Favorite, error) {
	d, err := s.current()
	if err != nil {
		return []canvas.Favorite{}, nil
	}
	return d.Favorites(ctx)
}

// AddFavorite ajoute une requête SQL aux favoris (nom vide : sa première
// ligne) et renvoie son identifiant.
func (s *DocumentService) AddFavorite(ctx context.Context, name, sql string) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.AddFavorite(ctx, name, sql)
	if err == nil {
		s.changed(ctx)
	}
	return id, err
}

// RenameFavorite renomme une requête favorite.
func (s *DocumentService) RenameFavorite(ctx context.Context, id int64, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RenameFavorite(ctx, id, name) })
}

// DeleteFavorite retire une requête des favoris.
func (s *DocumentService) DeleteFavorite(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteFavorite(ctx, id) })
}
