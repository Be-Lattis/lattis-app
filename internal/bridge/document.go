package bridge

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lalamefine/slitable/internal/archive"
	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/doc"
	"github.com/lalamefine/slitable/internal/formula"
	"github.com/lalamefine/slitable/internal/grid"
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
}

var errNoDoc = errors.New("aucun classeur ouvert")

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
	s.doc, s.tables = d, map[int64]*grid.Table{}
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
			AddFilter("Classeur Slitable", "*.slitable").
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
		name := "Classeur.slitable"
		if d.Path() != "" {
			name = filepath.Base(d.Path())
		}
		p, err := application.Get().Dialog.SaveFile().
			SetMessage("Enregistrer le classeur").
			SetFilename(name).
			AddFilter("Classeur Slitable", "*.slitable").
			PromptForSingleSelection()
		if err != nil || p == "" {
			return s.State(ctx)
		}
		if !strings.HasSuffix(strings.ToLower(p), ".slitable") {
			p += ".slitable"
		}
		path = p
	}
	if err := d.Save(ctx, path); err != nil {
		return DocState{}, err
	}
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

// changed invalide les tableaux chargés et prévient l'interface.
func (s *DocumentService) changed(ctx context.Context) (DocState, error) {
	s.mu.Lock()
	s.tables = map[int64]*grid.Table{}
	s.mu.Unlock()
	st, err := s.State(ctx)
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

// Window lit des lignes d'un tableau (lecture par fenêtres).
func (s *DocumentService) Window(ctx context.Context, sheet, table int64, first, count int) (grid.Window, error) {
	d, err := s.current()
	if err != nil {
		return grid.Window{}, err
	}
	s.mu.Lock()
	t := s.tables[table]
	s.mu.Unlock()
	if t == nil {
		l, err := d.Layout(ctx, sheet)
		if err != nil {
			return grid.Window{}, err
		}
		for _, ti := range l.Tables {
			if ti.ID != table {
				continue
			}
			cols := make([]grid.Column, len(ti.Columns))
			sqlCols := make([]string, len(ti.Columns))
			ids := make([]int64, len(ti.Columns))
			for i, c := range ti.Columns {
				cols[i], sqlCols[i], ids[i] = grid.Column{Name: c.Name, Type: c.Type}, c.SQL, c.ID
			}
			exprs, args, computed, err := d.ColumnExprs(ctx, ti.ID)
			if err != nil {
				return grid.Window{}, err
			}
			decode := func(c int, v any) any {
				if c < len(computed) && computed[c] {
					return displayValue(formula.FromResult(v))
				}
				if b, ok := v.([]byte); ok {
					return string(b)
				}
				return v
			}
			t = grid.NewTable(d.DB(), ti.Source, ti.Name, cols, sqlCols).WithInvalid(ti.ID, ids).WithExprs(exprs, args, decode)
			if err := t.Load(ctx); err != nil {
				return grid.Window{}, err
			}
			s.mu.Lock()
			s.tables[table] = t
			s.mu.Unlock()
		}
		if t == nil {
			return grid.Window{}, grid.ErrRange
		}
	}
	return t.Window(ctx, first, count)
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

// Query exécute une requête SQL en lecture seule sur le classeur (onglet SQL).
func (s *DocumentService) Query(ctx context.Context, q string) (grid.QueryResult, error) {
	d, err := s.current()
	if err != nil {
		return grid.QueryResult{}, err
	}
	// Les tableaux croisés cités s'interrogent comme des vues.
	if q, err = d.PivotCTE(ctx, q); err != nil {
		return grid.QueryResult{}, err
	}
	return grid.Query(ctx, d.DB().Read(), q, QueryLimit)
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

// AddSheet ajoute un onglet et renvoie son identifiant.
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

// RenameSheet et DeleteSheet agissent sur un onglet.
func (s *DocumentService) RenameSheet(ctx context.Context, sheet int64, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RenameSheet(ctx, sheet, name) })
}

func (s *DocumentService) DeleteSheet(ctx context.Context, sheet int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteSheet(ctx, sheet) })
}

// ConvertPreview compte les valeurs qui ne se convertiraient pas au type to.
func (s *DocumentService) ConvertPreview(ctx context.Context, table string, i int, to grid.Type) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	return d.ConvertPreview(ctx, table, i, to)
}

// SetType fixe le type d'une colonne (valeurs non convertibles marquées en erreur).
func (s *DocumentService) SetType(ctx context.Context, table string, i int, to grid.Type) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetType(ctx, table, i, to) })
}

// SetColumnWidth fixe la largeur d'une colonne de la feuille.
func (s *DocumentService) SetColumnWidth(ctx context.Context, sheet int64, col, width int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetColumnWidth(ctx, sheet, col, width) })
}

// displayValue convertit une valeur calculée pour l'interface : nombre,
// texte, booléen, ou le libellé d'une erreur (« #DIV/0! »).
func displayValue(v formula.Value) any {
	switch v.Kind {
	case formula.Number:
		return v.Num
	case formula.Text:
		return v.Str
	case formula.Bool:
		return v.Bool
	case formula.Error:
		return v.Err.String()
	}
	return nil
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
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.FilterValues(ctx, table, i)
}

// RenameTable renomme un tableau.
func (s *DocumentService) RenameTable(ctx context.Context, old, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RenameTable(ctx, old, name) })
}

// DeleteTables supprime des tableaux entiers.
func (s *DocumentService) DeleteTables(ctx context.Context, names []string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteTables(ctx, names) })
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
	d, err := s.current()
	if err != nil {
		return "", err
	}
	return d.KeyCheck(ctx, table, i)
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
