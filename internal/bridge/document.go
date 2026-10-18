package bridge

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ChangedEvent est émis après chaque modification du document : l'interface
// relit la mise en page et vide son cache de lignes.
const ChangedEvent = "doc:changed"

func init() {
	application.RegisterEvent[DocState](ChangedEvent)
}

// DocState décrit le classeur ouvert pour l'interface (titre, menus).
type DocState struct {
	Open  bool   `json:"open"`
	Path  string `json:"path"`
	Title string `json:"title"`
	Dirty bool   `json:"dirty"`
	Undo  string `json:"undo"` // libellé de l'action à annuler
	Redo  string `json:"redo"`
	// Notice : message de la dernière action pour la barre d'état (tableaux
	// fusionnés, cellules rattachées à un tableau), vide sinon.
	Notice string `json:"notice,omitempty"`
}

// OpenResult est le résultat d'une ouverture : soit le classeur est ouvert,
// soit une session précédente interrompue est proposée en récupération.
type OpenResult struct {
	State       DocState        `json:"state"`
	Recoverable *archive.Orphan `json:"recoverable,omitempty"`
	Cancelled   bool            `json:"cancelled"`
}

// DocumentService expose un classeur à l'interface (phase 2).
type DocumentService struct {
	mu     sync.Mutex
	doc    *doc.Document
	tables map[int64]*grid.Table // tableaux chargés pour la lecture par fenêtres
	// memo garde les résultats coûteux du menu ▾ d'une colonne (valeurs du
	// filtre, contrôle de clé, mélange de types), jusqu'à la prochaine
	// modification de leur tableau.
	memo  map[memoKey]memoVal
	sheet int64 // feuille affichée : Cell_A1 de l'onglet SQL s'y lit
	ui    uiState
	mcp   mcpState
	demo  *demoRun // classeur de démonstration demandé au lancement
	pq    pqState  // base de travail des requêtes d'import (query.go)
}

var (
	errNoDoc  = errors.New("aucun classeur ouvert")
	errNoDemo = errors.New("aucune démonstration demandée au lancement (--demo)")
)

func (s *DocumentService) current() (*doc.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return nil, errNoDoc
	}
	return s.doc, nil
}

// ServiceShutdown ferme le classeur. Les modifications non enregistrées
// restent dans le répertoire de travail : elles seront proposées en
// récupération au prochain lancement.
func (s *DocumentService) ServiceShutdown() error {
	s.stopMCP()
	s.closeQuerySession()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return nil
	}
	if dirty, _ := s.doc.Dirty(context.Background()); dirty {
		return nil // garder le répertoire de travail : récupération au prochain lancement
	}
	return s.doc.Close()
}

// Orphans liste les sessions interrompues à proposer au démarrage.
func (s *DocumentService) Orphans() ([]archive.Orphan, error) { return archive.Orphans() }

func (s *DocumentService) replace(d *doc.Document) {
	s.mu.Lock()
	old := s.doc
	s.doc, s.tables, s.memo = d, map[int64]*grid.Table{}, nil
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}
}

// New crée un classeur vide.
func (s *DocumentService) New(ctx context.Context) (DocState, error) {
	d, err := doc.New(ctx)
	if err != nil {
		return DocState{}, err
	}
	s.replace(d)
	return s.State(ctx)
}

// Open ouvre un classeur (path vide : boîte de dialogue).
func (s *DocumentService) Open(ctx context.Context, path string) (OpenResult, error) {
	if path == "" {
		p, err := application.Get().Dialog.OpenFile().
			SetTitle("Ouvrir un classeur").
			AddFilter("Classeur Lattis", "*.lattis").
			PromptForSingleSelection()
		if err != nil {
			return OpenResult{}, err
		}
		if p == "" {
			return OpenResult{Cancelled: true}, nil
		}
		path = p
	}
	d, err := doc.Open(ctx, path)
	var rec *archive.RecoverableError
	if errors.As(err, &rec) {
		return OpenResult{Recoverable: &rec.Orphan}, nil
	}
	if err != nil {
		return OpenResult{}, err
	}
	s.replace(d)
	addRecent(path)
	st, err := s.State(ctx)
	return OpenResult{State: st}, err
}

// Recover rouvre une session interrompue (répertoire de travail dir).
func (s *DocumentService) Recover(ctx context.Context, dir string) (DocState, error) {
	orphans, err := archive.Orphans()
	if err != nil {
		return DocState{}, err
	}
	for _, o := range orphans {
		if o.Dir == dir {
			d, err := doc.Recover(ctx, o)
			if err != nil {
				return DocState{}, err
			}
			s.replace(d)
			return s.State(ctx)
		}
	}
	return DocState{}, errors.New("session introuvable")
}

// Discard abandonne une session interrompue.
func (s *DocumentService) Discard(dir string) error {
	orphans, err := archive.Orphans()
	if err != nil {
		return err
	}
	for _, o := range orphans {
		if o.Dir == dir {
			return archive.Discard(o)
		}
	}
	return nil
}

// Save enregistre ; as (ou un classeur jamais enregistré) demande le fichier.
func (s *DocumentService) Save(ctx context.Context, as bool) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	path := ""
	if as || d.Path() == "" {
		name := "Classeur.lattis"
		if d.Path() != "" {
			name = filepath.Base(d.Path())
		}
		p, err := application.Get().Dialog.SaveFile().
			SetMessage("Enregistrer le classeur").
			SetFilename(name).
			AddFilter("Classeur Lattis", "*.lattis").
			PromptForSingleSelection()
		if err != nil || p == "" {
			return s.State(ctx)
		}
		if !strings.HasSuffix(strings.ToLower(p), ".lattis") {
			p += ".lattis"
		}
		path = p
	}
	if err := saveDoc(ctx, d, path); err != nil {
		return DocState{}, err
	}
	addRecent(d.Path())
	return s.changed(ctx)
}

// State décrit le classeur ouvert.
func (s *DocumentService) State(ctx context.Context) (DocState, error) {
	d, err := s.current()
	if errors.Is(err, errNoDoc) {
		return DocState{}, nil
	}
	st := DocState{Open: true, Path: d.Path(), Title: "Classeur sans titre"}
	if st.Path != "" {
		st.Title = strings.TrimSuffix(filepath.Base(st.Path), filepath.Ext(st.Path))
	}
	if st.Dirty, err = d.Dirty(ctx); err != nil {
		return st, err
	}
	h, err := d.Bus.State(ctx)
	st.Undo, st.Redo = h.Undo, h.Redo
	return st, err
}

// changed invalide les tableaux chargés que les écritures ont pu toucher
// (les autres gardent leurs lignes et leurs totaux en cache) et prévient
// l'interface.
func (s *DocumentService) changed(ctx context.Context) (DocState, error) {
	var content, structure map[string]bool
	all := true
	if d, err := s.current(); err == nil {
		content, structure, all = d.DB().TakeTouched()
	}
	s.mu.Lock()
	for id, t := range s.tables {
		if !t.Refresh(content, structure, all) {
			delete(s.tables, id)
		}
	}
	for k, v := range s.memo {
		if all || content[v.sql] || structure[v.sql] {
			delete(s.memo, k)
		}
	}
	s.mu.Unlock()
	st, err := s.State(ctx)
	if d, derr := s.current(); derr == nil {
		st.Notice = d.TakeNotice()
	}
	if app := application.Get(); app != nil && err == nil {
		app.Event.Emit(ChangedEvent, st)
	}
	return st, err
}

// Layout donne la mise en page d'une feuille (tableaux et cellules libres).
func (s *DocumentService) Layout(ctx context.Context, sheet int64) (canvas.Layout, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Layout{}, err
	}
	return d.Layout(ctx, sheet)
}

// gridTable donne le tableau prêt à lire par fenêtres (gardé en cache).
func (s *DocumentService) gridTable(ctx context.Context, sheet, table int64) (*grid.Table, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	t := s.tables[table]
	s.mu.Unlock()
	if t == nil {
		l, err := d.Layout(ctx, sheet)
		if err != nil {
			return nil, err
		}
		for _, ti := range l.Tables {
			if ti.ID != table {
				continue
			}
			if t, err = api.LoadGrid(ctx, d, ti); err != nil {
				return nil, err
			}
			s.mu.Lock()
			s.tables[table] = t
			s.mu.Unlock()
		}
		if t == nil {
			return nil, grid.ErrRange
		}
	}
	return t, nil
}

// Window lit des lignes d'un tableau (lecture par fenêtres).
func (s *DocumentService) Window(ctx context.Context, sheet, table int64, first, count int) (grid.Window, error) {
	t, err := s.gridTable(ctx, sheet, table)
	if err != nil {
		return grid.Window{}, err
	}
	return t.Window(ctx, first, count)
}

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
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	ev, err := d.SetCell(ctx, sheet, row, col, raw)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	return ev, err
}

// Undo et Redo annulent et rétablissent.
func (s *DocumentService) Undo(ctx context.Context) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	if _, err := d.Bus.Undo(ctx); err != nil {
		return DocState{}, err
	}
	return s.changed(ctx)
}

func (s *DocumentService) Redo(ctx context.Context) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	if _, err := d.Bus.Redo(ctx); err != nil {
		return DocState{}, err
	}
	return s.changed(ctx)
}

// InsertRow, DeleteRow, InsertColumn et DeleteColumn agissent sur un tableau.
// MoveCells déplace les cellules libres d'une plage de (dr, dc).
func (s *DocumentService) MoveCells(ctx context.Context, sheet int64, r0, c0, r1, c1, dr, dc int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveCells(ctx, sheet, r0, c0, r1, c1, dr, dc) })
}

// InsertSheetRows insère n lignes entières avant la ligne at de la feuille.
func (s *DocumentService) InsertSheetRows(ctx context.Context, sheet int64, at, n int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.InsertSheetRows(ctx, sheet, at, n) })
}

// DeleteSheetRows supprime les lignes entières r0 à r1 de la feuille.
func (s *DocumentService) DeleteSheetRows(ctx context.Context, sheet int64, r0, r1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteSheetRows(ctx, sheet, r0, r1) })
}

// InsertSheetColumns insère n colonnes entières avant la colonne at de la feuille.
func (s *DocumentService) InsertSheetColumns(ctx context.Context, sheet int64, at, n int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.InsertSheetColumns(ctx, sheet, at, n) })
}

// DeleteSheetColumns supprime les colonnes entières c0 à c1 de la feuille.
func (s *DocumentService) DeleteSheetColumns(ctx context.Context, sheet int64, c0, c1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteSheetColumns(ctx, sheet, c0, c1) })
}

func (s *DocumentService) InsertRow(ctx context.Context, table string, n int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.InsertRow(ctx, table, n) })
}

func (s *DocumentService) DeleteRow(ctx context.Context, table string, n int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteRow(ctx, table, n) })
}

func (s *DocumentService) InsertColumn(ctx context.Context, table string, i int, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.InsertColumn(ctx, table, i, name) })
}

func (s *DocumentService) DeleteColumn(ctx context.Context, table string, i int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteColumn(ctx, table, i) })
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

// Query exécute une requête SQL en lecture seule sur le classeur (onglet SQL).
func (s *DocumentService) Query(ctx context.Context, q string) (grid.QueryResult, error) {
	d, err := s.current()
	if err != nil {
		return grid.QueryResult{}, err
	}
	if q, err = d.CellSQL(ctx, q, s.activeSheet()); err != nil {
		return grid.QueryResult{}, err
	}
	// Les tableaux croisés cités s'interrogent comme des vues.
	if q, err = d.PivotCTE(ctx, q); err != nil {
		return grid.QueryResult{}, err
	}
	return grid.Query(ctx, d.DB().Read(), q, QueryLimit)
}

// QuerySheet décrit un résultat SQL posé sur une nouvelle feuille.
type QuerySheet struct {
	Sheet int64    `json:"sheet"`
	Table string   `json:"table"`
	State DocState `json:"state"`
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
	if q, err = d.PivotCTE(ctx, q); err != nil {
		return QuerySheet{}, err
	}
	res, err := d.QueryToSheet(ctx, q, name, headers)
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

// Paste colle un bloc (presse-papiers d'Excel) à partir d'une cellule.
func (s *DocumentService) Paste(ctx context.Context, sheet int64, row, col int, block [][]string) (canvas.Event, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	ev, err := d.Paste(ctx, sheet, row, col, block)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	return ev, err
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
	return id, err
}

// MoveSheet place un onglet au rang to (0 : premier).
func (s *DocumentService) MoveSheet(ctx context.Context, sheet int64, to int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveSheet(ctx, sheet, to) })
}

// RenameSheet et DeleteSheet agissent sur un onglet.
func (s *DocumentService) RenameSheet(ctx context.Context, sheet int64, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RenameSheet(ctx, sheet, name) })
}

func (s *DocumentService) DeleteSheet(ctx context.Context, sheet int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteSheet(ctx, sheet) })
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
	return id, err
}

// SetSheetColor fixe la couleur d'un onglet (« #rrggbb », vide : aucune).
func (s *DocumentService) SetSheetColor(ctx context.Context, sheet int64, color string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetColor(ctx, sheet, color) })
}

// SetSheetHidden masque ou affiche un onglet.
func (s *DocumentService) SetSheetHidden(ctx context.Context, sheet int64, hidden bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetHidden(ctx, sheet, hidden) })
}

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
	return s.act(ctx, func(d *doc.Document) error { return d.SetType(ctx, table, i, to) })
}

// SetColumnWidth fixe la largeur d'une colonne de la feuille.
func (s *DocumentService) SetColumnWidth(ctx context.Context, sheet int64, col, width int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetColumnWidth(ctx, sheet, col, width) })
}

// FormulaHelp sert l'aide à la saisie des formules : fonctions (en
// français) et tableaux du classeur avec leurs colonnes.
type FormulaHelp struct {
	Functions []formula.FunctionInfo `json:"functions"`
	Tables    []canvas.TableNames    `json:"tables"`
}

// FormulaHelp renvoie le catalogue de l'aide à la saisie.
func (s *DocumentService) FormulaHelp(ctx context.Context) (FormulaHelp, error) {
	h := FormulaHelp{Functions: formula.Functions(formula.FR), Tables: []canvas.TableNames{}}
	d, err := s.current()
	if err != nil {
		return h, nil
	}
	h.Tables, err = d.AllTables(ctx)
	return h, err
}

// SetName nomme une cellule libre (vide : retire le nom).
func (s *DocumentService) SetName(ctx context.Context, sheet int64, row, col int, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetName(ctx, sheet, row, col, name) })
}

// SortTable trie un tableau selon la colonne i (desc : décroissant).
func (s *DocumentService) SortTable(ctx context.Context, table string, i int, desc bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SortTable(ctx, table, i, desc) })
}

// ApplyStyle met en forme un rectangle de la feuille (patch : propriétés à
// poser, une valeur vide les retire).
func (s *DocumentService) ApplyStyle(ctx context.Context, sheet int64, r0, c0, r1, c1 int, patch map[string]any) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ApplyStyle(ctx, sheet, r0, c0, r1, c1, patch) })
}

// ApplyLineStyle met en forme des lignes (rows) ou des colonnes entières
// [i0, i1] de la feuille, comme Excel.
func (s *DocumentService) ApplyLineStyle(ctx context.Context, sheet int64, rows bool, i0, i1 int, patch map[string]any) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ApplyLineStyle(ctx, sheet, rows, i0, i1, patch) })
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

// SetLinesHidden masque ou affiche des lignes (rows) ou des colonnes [i0, i1].
func (s *DocumentService) SetLinesHidden(ctx context.Context, sheet int64, rows bool, i0, i1 int, hide bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetLinesHidden(ctx, sheet, rows, i0, i1, hide) })
}

// SetFreeze fige les volets de la feuille (0, 0 : les libère).
func (s *DocumentService) SetFreeze(ctx context.Context, sheet int64, rows, cols int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetFreeze(ctx, sheet, rows, cols) })
}

// SetZoom fixe le zoom de la feuille, en pour cent (10 à 400) ; hors du
// journal d'annulation.
func (s *DocumentService) SetZoom(ctx context.Context, sheet int64, pct int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetZoom(ctx, sheet, pct) })
}

// Find cherche un texte dans une feuille (cellules dans l'ordre de lecture).
func (s *DocumentService) Find(ctx context.Context, sheet int64, text string) ([]canvas.Match, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.Find(ctx, sheet, text)
}

// Replace remplace un texte dans toute la feuille et renvoie le nombre de
// cellules modifiées.
func (s *DocumentService) Replace(ctx context.Context, sheet int64, text, by string) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	n, err := d.Replace(ctx, sheet, text, by)
	if err == nil && n > 0 {
		s.changed(ctx)
	}
	return n, err
}

// SetRules remplace les règles de mise en forme conditionnelle d'une colonne.
func (s *DocumentService) SetRules(ctx context.Context, table string, i int, rules []canvas.Rule) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetRules(ctx, table, i, rules) })
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

// FormatAsTable refait un tableau de la plage qui contient la cellule.
func (s *DocumentService) FormatAsTable(ctx context.Context, sheet int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.FormatAsTable(ctx, sheet, row, col)
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

// Recalc renouvelle AUJOURDHUI, MAINTENANT et ALEA (F9).
func (s *DocumentService) Recalc(ctx context.Context) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		d.Recalc()
		return nil
	})
}

// RemoveDuplicates supprime les lignes en double d'un tableau ; renvoie leur nombre.
func (s *DocumentService) RemoveDuplicates(ctx context.Context, table string, cols []int) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	n, err := d.RemoveDuplicates(ctx, table, cols)
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

// CreatePivot crée un tableau croisé sur une nouvelle feuille ; renvoie la feuille.
func (s *DocumentService) CreatePivot(ctx context.Context, cfg canvas.PivotConfig) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	sheet, err := d.CreatePivot(ctx, cfg)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return sheet, err
}

// SetPivot change la configuration d'un tableau croisé.
func (s *DocumentService) SetPivot(ctx context.Context, id int64, cfg canvas.PivotConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetPivot(ctx, id, cfg) })
}

// DeletePivot supprime un tableau croisé.
func (s *DocumentService) DeletePivot(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeletePivot(ctx, id) })
}

// CreateChart pose un graphique sur une feuille.
func (s *DocumentService) CreateChart(ctx context.Context, sheet int64, row, col int, cfg canvas.ChartConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.CreateChart(ctx, sheet, row, col, cfg)
		return err
	})
}

// SetChart change la configuration d'un graphique.
func (s *DocumentService) SetChart(ctx context.Context, id int64, cfg canvas.ChartConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetChart(ctx, id, cfg) })
}

// MoveChart déplace un graphique.
func (s *DocumentService) MoveChart(ctx context.Context, id int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveChart(ctx, id, row, col) })
}

// DeleteChart supprime un graphique.
func (s *DocumentService) DeleteChart(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteChart(ctx, id) })
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

// PlaceChart déplace et redimensionne un graphique (ancrage libre).
func (s *DocumentService) PlaceChart(ctx context.Context, id int64, row, col, offX, offY, width, height int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.PlaceChart(ctx, id, row, col, offX, offY, width, height) })
}

// Dependencies donne les antécédents et les dépendants d'une cellule.
func (s *DocumentService) Dependencies(ctx context.Context, sheet int64, row, col int) (canvas.Dependencies, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Dependencies{}, err
	}
	return d.Dependencies(ctx, sheet, row, col)
}

// SetHeadings affiche toujours, ou seulement pendant la saisie d'une
// formule, les lettres de colonnes et numéros de lignes de la feuille.
func (s *DocumentService) SetHeadings(ctx context.Context, sheet int64, show bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetHeadings(ctx, sheet, show) })
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

// SetGridlines affiche ou masque le quadrillage de la feuille.
func (s *DocumentService) SetGridlines(ctx context.Context, sheet int64, show bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetGridlines(ctx, sheet, show) })
}

// SetRowHeight fixe la hauteur d'une ligne (0 : hauteur par défaut).
func (s *DocumentService) SetRowHeight(ctx context.Context, sheet int64, row, height int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetRowHeight(ctx, sheet, row, height) })
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

// ProtectSheet protège une feuille (password vide : sans mot de passe) ;
// hors de l'historique d'annulation, comme Excel.
func (s *DocumentService) ProtectSheet(ctx context.Context, sheet int64, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ProtectSheet(ctx, sheet, password) })
}

// UnprotectSheet ôte la protection d'une feuille.
func (s *DocumentService) UnprotectSheet(ctx context.Context, sheet int64, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UnprotectSheet(ctx, sheet, password) })
}

// ProtectWorkbook protège la structure du classeur (onglets) ; sheets
// verrouille aussi le contenu de toutes les feuilles.
func (s *DocumentService) ProtectWorkbook(ctx context.Context, password string, sheets bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ProtectWorkbook(ctx, password, sheets) })
}

// UnprotectWorkbook ôte la protection de la structure du classeur.
func (s *DocumentService) UnprotectWorkbook(ctx context.Context, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UnprotectWorkbook(ctx, password) })
}

// ClearCells vide entièrement les cellules libres d'une plage (valeurs,
// mise en forme, fusions, notes, validations).
func (s *DocumentService) ClearCells(ctx context.Context, sheet int64, r0, c0, r1, c1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ClearCells(ctx, sheet, r0, c0, r1, c1) })
}

// ClearRange efface une zone en une seule étape d'annulation (tableaux
// entièrement compris supprimés ; all : mise en forme des cellules libres
// comprise, comme Supprimer).
func (s *DocumentService) ClearRange(ctx context.Context, sheet int64, r0, c0, r1, c1 int, all bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ClearRange(ctx, sheet, r0, c0, r1, c1, all) })
}

// Transfer copie ou déplace (move) une zone d'une feuille vers une autre
// zone, d'une feuille à l'autre : copier-coller et couper-coller internes,
// glisser de la bordure de la sélection (voir canvas.Model.Transfer).
func (s *DocumentService) Transfer(ctx context.Context, from int64, src canvas.Rect, to int64, dst canvas.Rect, move bool, values [][]string) (canvas.Event, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	ev, err := d.Transfer(ctx, from, src, to, dst, move, values)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	return ev, err
}

// PinNote épingle la note d'une cellule (toujours affichée) ou la remet en infobulle.
func (s *DocumentService) PinNote(ctx context.Context, sheet int64, row, col int, pinned bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.PinNote(ctx, sheet, row, col, pinned) })
}

type memoKey struct {
	kind, table string
	col         int
}

type memoVal struct {
	sql string // nom SQL du tableau (t_N), pour l'invalidation
	v   any
}

// remember calcule fn une fois par tableau, colonne et genre de résultat,
// jusqu'à la prochaine modification de ce tableau (voir changed).
func remember[T any](ctx context.Context, s *DocumentService, kind, table string, i int, fn func(*doc.Document) (T, error)) (T, error) {
	var zero T
	d, err := s.current()
	if err != nil {
		return zero, err
	}
	k := memoKey{kind, table, i}
	s.mu.Lock()
	m, ok := s.memo[k]
	s.mu.Unlock()
	if ok {
		if v, ok := m.v.(T); ok {
			return v, nil
		}
	}
	v, err := fn(d)
	if err != nil {
		return v, err
	}
	var id int64
	if err := d.DB().Read().QueryRowContext(ctx, `SELECT id FROM _lattis_tables WHERE name = ?`, table).Scan(&id); err != nil {
		return v, nil // tableau introuvable : résultat non gardé
	}
	s.mu.Lock()
	if s.doc == d {
		if s.memo == nil {
			s.memo = map[memoKey]memoVal{}
		}
		s.memo[k] = memoVal{sql: fmt.Sprintf("t_%d", id), v: v}
	}
	s.mu.Unlock()
	return v, nil
}
