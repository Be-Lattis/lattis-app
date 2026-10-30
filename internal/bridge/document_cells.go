package bridge

import (
	"context"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/work"
)

// Cellules : saisie, annuler et rétablir, lignes et colonnes de la feuille,
// collage, effacement, fusion, notes.
// Méthodes de DocumentService (document.go).

// RangeStats résume des cellules d'un tableau pour la barre d'état
// (Moyenne, Nb, Somme), en SQL : lignes first à first+count-1, colonnes cols.
func (s *DocumentService) RangeStats(ctx context.Context, sheet, table int64, first, count int, cols []int) (grid.Stats, error) {
	t, err := s.gridTable(ctx, sheet, table)
	if err != nil {
		return grid.Stats{}, err
	}
	return t.Stats(ctx, first, count, cols)
}

// RuleStats donne les statistiques de colonnes d'un tableau pour la mise
// en forme conditionnelle (10 premiers, moyenne, doublons, barres de
// données, nuances, icônes), calculées en SQL et gardées en cache.
func (s *DocumentService) RuleStats(ctx context.Context, sheet, table int64, cols []int, req grid.RuleStatsReq) (grid.RuleStats, error) {
	t, err := s.gridTable(ctx, sheet, table)
	if err != nil {
		return grid.RuleStats{}, err
	}
	return t.RuleStats(ctx, cols, req)
}

// SetCell traite une saisie validée dans une cellule de la feuille.
func (s *DocumentService) SetCell(ctx context.Context, sheet int64, row, col int, raw string) (canvas.Event, error) {
	return s.SetCellKeyed(ctx, sheet, row, col, raw, "")
}

// SetCellKeyed est SetCell avec la réponse à la question de la clé (refus
// « clé à choisir ») : "index" (numéros de ligne) ou "column" (la colonne
// cherchée devient la clé), dans la même étape d'annulation que la saisie.
//
// La saisie respecte la validation des données de la cellule (refus
// validation.refusedEntry) ; key "dv" : l'utilisateur a accepté la valeur
// après l'alerte (Avertissement : Oui, Informations : OK).
func (s *DocumentService) SetCellKeyed(ctx context.Context, sheet int64, row, col int, raw, key string) (canvas.Event, error) {
	ctx = canvas.WithEntryCheck(ctx, key == "dv")
	if key == "dv" {
		key = ""
	}
	ctx = canvas.WithKeyChoice(ctx, canvas.KeyChoice(key))
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	w := s.watchChange(sheet, row, col, row, col, "edit")
	ev, err := d.SetCell(ctx, sheet, row, col, raw)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	s.fireChange(w)
	return ev, err
}

// Undo et Redo annulent et rétablissent.
func (s *DocumentService) Undo(ctx context.Context) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	wctx, t := s.track(ctx, true)
	work.Op(wctx, "undo", "")
	_, err = d.Bus.Undo(wctx)
	s.untrack(t, &err)
	if err != nil {
		return DocState{}, err
	}
	// Classeur collaboratif : cellules modifiées par un collègue depuis,
	// laissées à sa valeur (§7).
	d.Notify(collab.UndoNoticeIn(s.sessionLang(), d.Oplog, false))
	return s.changed(ctx)
}

func (s *DocumentService) Redo(ctx context.Context) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	wctx, t := s.track(ctx, true)
	work.Op(wctx, "redo", "")
	_, err = d.Bus.Redo(wctx)
	s.untrack(t, &err)
	if err != nil {
		return DocState{}, err
	}
	d.Notify(collab.UndoNoticeIn(s.sessionLang(), d.Oplog, true))
	return s.changed(ctx)
}

// InsertRow, DeleteRow, InsertColumn et DeleteColumn agissent sur un tableau.
// MoveCells déplace les cellules libres d'une plage de (dr, dc).
func (s *DocumentService) MoveCells(ctx context.Context, sheet int64, r0, c0, r1, c1, dr, dc int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveCells(ctx, sheet, r0, c0, r1, c1, dr, dc) })
}

// InsertSheetRows insère n lignes entières avant la ligne at de la feuille.
func (s *DocumentService) InsertSheetRows(ctx context.Context, sheet int64, at, n int) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.InsertSheetRows(ctx, sheet, at, n) })
}

// DeleteSheetRows supprime les lignes entières r0 à r1 de la feuille.
func (s *DocumentService) DeleteSheetRows(ctx context.Context, sheet int64, r0, r1 int) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.DeleteSheetRows(ctx, sheet, r0, r1) })
}

// InsertSheetColumns insère n colonnes entières avant la colonne at de la feuille.
func (s *DocumentService) InsertSheetColumns(ctx context.Context, sheet int64, at, n int) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.InsertSheetColumns(ctx, sheet, at, n) })
}

// DeleteSheetColumns supprime les colonnes entières c0 à c1 de la feuille.
func (s *DocumentService) DeleteSheetColumns(ctx context.Context, sheet int64, c0, c1 int) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.DeleteSheetColumns(ctx, sheet, c0, c1) })
}

func (s *DocumentService) InsertRow(ctx context.Context, table string, n int) (DocState, error) {
	w := s.watchTableRow("", 0, "insert_row")
	st, err := s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.InsertRow(ctx, table, n) })
	if err == nil {
		s.fireTableRow(w, table, n)
	}
	return st, err
}

func (s *DocumentService) DeleteRow(ctx context.Context, table string, n int) (DocState, error) {
	w := s.watchTableRow(table, n, "delete_row")
	st, err := s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.DeleteRow(ctx, table, n) })
	if err == nil {
		s.fireTableRow(w, table, n)
	}
	return st, err
}

func (s *DocumentService) InsertColumn(ctx context.Context, table string, i int, name string) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.InsertColumn(ctx, table, i, name) })
}

func (s *DocumentService) DeleteColumn(ctx context.Context, table string, i int) (DocState, error) {
	return s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.DeleteColumn(ctx, table, i) })
}

func (s *DocumentService) act(ctx context.Context, fn func(*doc.Document) error) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	if err := fn(d); err != nil {
		return DocState{}, err
	}
	return s.changed(ctx)
}

// Paste colle un bloc (presse-papiers d'Excel) à partir d'une cellule.
func (s *DocumentService) Paste(ctx context.Context, sheet int64, row, col int, block [][]string) (canvas.Event, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	width := 1
	for _, r := range block {
		width = max(width, len(r))
	}
	w := s.watchChange(sheet, row, col, row+max(len(block), 1)-1, col+width-1, "paste")
	wctx, t := s.track(ctx, true)
	ev, err := d.Paste(wctx, sheet, row, col, block)
	s.untrack(t, &err)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	s.fireChange(w)
	return ev, err
}

// SetNote pose la note d'une cellule (texte vide : la retire).
func (s *DocumentService) SetNote(ctx context.Context, sheet int64, row, col int, text string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetNote(ctx, sheet, row, col, text) })
}

// MergeCells fusionne et centre des cellules libres ; renvoie le nombre de
// valeurs effacées (seule celle en haut à gauche est gardée).
func (s *DocumentService) MergeCells(ctx context.Context, sheet int64, r0, c0, r1, c1 int) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	n, err := d.MergeCells(ctx, sheet, r0, c0, r1, c1)
	if err == nil {
		s.changed(ctx)
	}
	return n, err
}

// UnmergeCells annule les fusions qui touchent un rectangle.
func (s *DocumentService) UnmergeCells(ctx context.Context, sheet int64, r0, c0, r1, c1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UnmergeCells(ctx, sheet, r0, c0, r1, c1) })
}

// ClearCells vide entièrement les cellules libres d'une plage (valeurs,
// mise en forme, fusions, notes, validations).
func (s *DocumentService) ClearCells(ctx context.Context, sheet int64, r0, c0, r1, c1 int) (DocState, error) {
	w := s.watchChange(sheet, r0, c0, r1, c1, "clear")
	st, err := s.act(ctx, func(d *doc.Document) error { return d.ClearCells(ctx, sheet, r0, c0, r1, c1) })
	if err == nil {
		s.fireChange(w)
	}
	return st, err
}

// ClearRange efface une zone en une seule étape d'annulation (tableaux
// entièrement compris supprimés ; all : mise en forme des cellules libres
// comprise, comme Supprimer).
func (s *DocumentService) ClearRange(ctx context.Context, sheet int64, r0, c0, r1, c1 int, all bool) (DocState, error) {
	w := s.watchChange(sheet, r0, c0, r1, c1, "clear")
	st, err := s.actw(ctx, true, func(ctx context.Context, d *doc.Document) error { return d.ClearRange(ctx, sheet, r0, c0, r1, c1, all) })
	if err == nil {
		s.fireChange(w)
	}
	return st, err
}

// Transfer copie ou déplace (move) une zone d'une feuille vers une autre
// zone, d'une feuille à l'autre : copier-coller et couper-coller internes,
// glisser de la bordure de la sélection (voir canvas.Model.Transfer).
func (s *DocumentService) Transfer(ctx context.Context, from int64, src canvas.Rect, to int64, dst canvas.Rect, move bool, values [][]string) (canvas.Event, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	source := "copy"
	if move {
		source = "move"
	}
	w := s.watchChange(to, dst.R0, dst.C0, dst.R1, dst.C1, source)
	wctx, t := s.track(ctx, true)
	ev, err := d.Transfer(wctx, from, src, to, dst, move, values)
	s.untrack(t, &err)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	s.fireChange(w)
	return ev, err
}

// PinNote épingle la note d'une cellule (toujours affichée) ou la remet en infobulle.
func (s *DocumentService) PinNote(ctx context.Context, sheet int64, row, col int, pinned bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.PinNote(ctx, sheet, row, col, pinned) })
}
