package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
)

// Tableaux : types et largeurs des colonnes, tri, filtres, conversion,
// doublons, texte en colonnes, clé, en-têtes, hauteur des lignes.
// Méthodes de DocumentService (document.go).

// ConvertPreview compte les valeurs qui ne se convertiraient pas au type to.
func (s *DocumentService) ConvertPreview(ctx context.Context, table string, i int, to grid.Type) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	return d.ConvertPreview(ctx, table, i, to)
}

// TypeMix donne le type majoritaire d'une colonne passée en texte par
// élargissement, et le nombre de valeurs qui n'en sont pas.
func (s *DocumentService) TypeMix(ctx context.Context, table string, i int) (canvas.Mix, error) {
	return remember(ctx, s, "mix", table, i, func(d *doc.Document) (canvas.Mix, error) { return d.TypeMix(ctx, table, i) })
}

// SetType fixe le type d'une colonne (valeurs non convertibles marquées en erreur).
func (s *DocumentService) SetType(ctx context.Context, table string, i int, to grid.Type) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.SetType(ctx, table, i, to) })
}

// SetColumnWidth fixe la largeur d'une colonne de la feuille.
func (s *DocumentService) SetColumnWidth(ctx context.Context, sheet int64, col, width int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetColumnWidth(ctx, sheet, col, width) })
}

// SetColumnWidths fixe la largeur de plusieurs colonnes en une seule étape
// d'annulation (ajustement automatique des colonnes sélectionnées).
func (s *DocumentService) SetColumnWidths(ctx context.Context, sheet int64, widths []canvas.ColWidth) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetColumnWidths(ctx, sheet, widths) })
}

// SetName nomme une cellule libre (vide : retire le nom).
func (s *DocumentService) SetName(ctx context.Context, sheet int64, row, col int, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetName(ctx, sheet, row, col, name) })
}

// SortTable trie un tableau selon la colonne i (desc : décroissant).
func (s *DocumentService) SortTable(ctx context.Context, table string, i int, desc bool) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.SortTable(ctx, table, i, desc) })
}

// SortTableCustom trie un tableau selon la colonne i dans un ordre
// personnalisé, enregistré pour la colonne.
func (s *DocumentService) SortTableCustom(ctx context.Context, table string, i int, order canvas.CustomOrder) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.SortTableCustom(ctx, table, i, order) })
}

// ColumnOrder rend l'ordre personnalisé enregistré de la colonne i.
func (s *DocumentService) ColumnOrder(ctx context.Context, table string, i int) (ColumnOrderInfo, error) {
	d, err := s.current()
	if err != nil {
		return ColumnOrderInfo{}, err
	}
	o, ok, applied, err := d.ColumnOrder(ctx, table, i)
	if err != nil || !ok {
		return ColumnOrderInfo{}, err
	}
	return ColumnOrderInfo{Order: &o, Applied: applied}, nil
}

// CustomLists rend les listes personnalisées (prédéfinies puis du classeur).
func (s *DocumentService) CustomLists(ctx context.Context) ([]canvas.CustomList, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.CustomLists(ctx)
}

// SetCustomLists remplace les listes personnalisées du classeur.
func (s *DocumentService) SetCustomLists(ctx context.Context, lists []canvas.CustomList) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetCustomLists(ctx, lists) })
}

// SetLinesHidden masque ou affiche des lignes (rows) ou des colonnes [i0, i1].
func (s *DocumentService) SetLinesHidden(ctx context.Context, sheet int64, rows bool, i0, i1 int, hide bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetLinesHidden(ctx, sheet, rows, i0, i1, hide) })
}

// SetFilter remplace le filtre d'une colonne (nil : retire le filtre).
func (s *DocumentService) SetFilter(ctx context.Context, table string, i int, f *canvas.ColumnFilter) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetFilter(ctx, table, i, f) })
}

// ClearFilters retire tous les filtres d'un tableau.
func (s *DocumentService) ClearFilters(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ClearFilters(ctx, table) })
}

// FilterValues donne les valeurs distinctes d'une colonne pour le filtre.
func (s *DocumentService) FilterValues(ctx context.Context, table string, i int) ([]canvas.FilterValue, error) {
	return remember(ctx, s, "values", table, i, func(d *doc.Document) ([]canvas.FilterValue, error) { return d.FilterValues(ctx, table, i) })
}

// RenameTable renomme un tableau.
func (s *DocumentService) RenameTable(ctx context.Context, old, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RenameTable(ctx, old, name) })
}

// DeleteTables supprime des tableaux entiers.
func (s *DocumentService) DeleteTables(ctx context.Context, names []string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteTables(ctx, names) })
}

// ConvertToRange convertit un tableau en plage de cellules libres.
func (s *DocumentService) ConvertToRange(ctx context.Context, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ConvertToRange(ctx, name) })
}

// ColumnsToValues convertit des colonnes calculées en colonnes de valeurs.
func (s *DocumentService) ColumnsToValues(ctx context.Context, table string, cols []int) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.ColumnsToValues(ctx, table, cols) })
}

// RangeToValues convertit en valeurs les colonnes calculées et les cellules
// libres avec formule de la plage.
func (s *DocumentService) RangeToValues(ctx context.Context, sheet int64, r0, c0, r1, c1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RangeToValues(ctx, sheet, r0, c0, r1, c1) })
}

// FormatAsTable refait un tableau de la plage qui contient la cellule.
func (s *DocumentService) FormatAsTable(ctx context.Context, sheet int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.FormatAsTable(ctx, sheet, row, col)
		return err
	})
}

// ConvertSpill remplace le résultat déversé de la formule de la cellule
// par un tableau : lié (recalculé quand ses sources changent) ou de
// valeurs, en une seule étape d'annulation.
func (s *DocumentService) ConvertSpill(ctx context.Context, sheet int64, row, col int, linked bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.ConvertSpill(ctx, sheet, row, col, linked)
		return err
	})
}

// MoveTable déplace un tableau (fusion s'il est posé côte à côte d'un autre).
func (s *DocumentService) MoveTable(ctx context.Context, name string, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveTable(ctx, name, row, col) })
}

// ExtractColumns sort des colonnes d'un tableau vers un nouveau tableau.
func (s *DocumentService) ExtractColumns(ctx context.Context, name string, i0, i1, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ExtractColumns(ctx, name, i0, i1, row, col) })
}

// RemoveDuplicates supprime les lignes en double d'un tableau ; renvoie leur nombre.
func (s *DocumentService) RemoveDuplicates(ctx context.Context, table string, cols []int) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	wctx, t := s.track(ctx, true)
	n, err := d.RemoveDuplicates(wctx, table, cols)
	s.untrack(t, &err)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return n, err
}

// TextToColumnsPreview donne l'aperçu de « Convertir » (texte en colonnes)
// sur les lignes r0 à r1 de la colonne col.
func (s *DocumentService) TextToColumnsPreview(ctx context.Context, sheet int64, r0, r1, col int, o canvas.TextColumnsOptions) (canvas.TextColumnsPreview, error) {
	d, err := s.current()
	if err != nil {
		return canvas.TextColumnsPreview{}, err
	}
	return d.TextToColumnsPreview(ctx, sheet, r0, r1, col, o)
}

// TextToColumns convertit le texte des lignes r0 à r1 de la colonne col en
// colonnes ; renvoie le nombre de colonnes importées.
func (s *DocumentService) TextToColumns(ctx context.Context, sheet int64, r0, r1, col int, o canvas.TextColumnsOptions) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	n, err := d.TextToColumns(ctx, sheet, r0, r1, col, o)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return n, err
}

// KeyCheck dit si une colonne peut devenir la clé du tableau (vide : oui).
func (s *DocumentService) KeyCheck(ctx context.Context, table string, i int) (string, error) {
	return remember(ctx, s, "key", table, i, func(d *doc.Document) (string, error) { return d.KeyCheck(ctx, table, i) })
}

// SetKey fait d'une colonne la clé du tableau.
func (s *DocumentService) SetKey(ctx context.Context, table string, i int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetKey(ctx, table, i) })
}

// ClearKey retire la clé d'un tableau.
func (s *DocumentService) ClearKey(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ClearKey(ctx, table) })
}

// SetHandKey change la clé de jointure des colonnes saisies à côté d'un
// tableau chargé (column : colonne chargée ; "#" : rang des lignes). Les
// valeurs mises de côté sont oubliées (annulable).
func (s *DocumentService) SetHandKey(ctx context.Context, table, column string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.SetHandKey(ctx, table, column)
		return err
	})
}

// ForgetHandKept oublie les valeurs saisies mises de côté (lignes disparues
// du résultat) d'un tableau chargé.
func (s *DocumentService) ForgetHandKept(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.ForgetHandKept(ctx, table)
		return err
	})
}

// DismissKeyHint ne propose plus la première colonne du tableau comme clé.
func (s *DocumentService) DismissKeyHint(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DismissKeyHint(ctx, table) })
}

// DuplicatesPreview compte les lignes en double selon des colonnes.
func (s *DocumentService) DuplicatesPreview(ctx context.Context, table string, cols []int) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	return d.DuplicatesPreview(ctx, table, cols)
}

// FieldValues donne les valeurs distinctes d'une colonne désignée par son nom.
func (s *DocumentService) FieldValues(ctx context.Context, table, field string) ([]canvas.FilterValue, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.FieldValues(ctx, table, field)
}

// PromoteHeader prend la première ligne d'un tableau comme en-têtes.
func (s *DocumentService) PromoteHeader(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.PromoteHeader(ctx, table) })
}

// HeaderToTitle sort l'en-tête d'un tableau (c'est un titre) : la 1re ligne
// de données devient l'en-tête.
func (s *DocumentService) HeaderToTitle(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.HeaderToTitle(ctx, table) })
}

// TitleToHeader : « Utiliser comme en-têtes » ; la ligne juste au-dessus du
// tableau donne les noms des colonnes, l'ancien en-tête devient la 1re ligne.
func (s *DocumentService) TitleToHeader(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.TitleToHeader(ctx, table) })
}

// SetTwoLevel affiche (ou non) les en-têtes d'un tableau sur deux lignes
// (groupe « Groupe.Sous » dans une bande au-dessus de l'en-tête).
func (s *DocumentService) SetTwoLevel(ctx context.Context, table string, on bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetTwoLevel(ctx, table, on) })
}

// RowToNote sort la dernière ligne d'un tableau : elle reste dessous, hors
// du tableau (note ou total).
func (s *DocumentService) RowToNote(ctx context.Context, table string, n int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RowToNote(ctx, table, n) })
}

// JoinTable ajoute au tableau voisin une cellule libre gardée hors du
// tableau (note, total, remarque).
func (s *DocumentService) JoinTable(ctx context.Context, sheet int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.JoinTable(ctx, sheet, row, col) })
}

// SetRowHeight fixe la hauteur d'une ligne (0 : hauteur par défaut).
func (s *DocumentService) SetRowHeight(ctx context.Context, sheet int64, row, height int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetRowHeight(ctx, sheet, row, height) })
}

// SetRowHeights fixe la hauteur de plusieurs lignes en une seule étape
// d'annulation (0 : hauteur par défaut).
func (s *DocumentService) SetRowHeights(ctx context.Context, sheet int64, heights []canvas.RowHeight) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetRowHeights(ctx, sheet, heights) })
}

// SuppressColumns : Suppr sur les colonnes entières i à j d'un tableau, en
// une étape (vides au milieu : le tableau se coupe en deux).
func (s *DocumentService) SuppressColumns(ctx context.Context, table string, i, j int) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.SuppressColumns(ctx, table, i, j) })
}

// DeleteColumnSpans supprime des colonnes de plusieurs tableaux (colonnes
// entières de la feuille qui les traversent), en une étape.
func (s *DocumentService) DeleteColumnSpans(ctx context.Context, spans []canvas.ColumnSpan) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.DeleteColumnSpans(ctx, spans) })
}
