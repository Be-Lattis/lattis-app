package demo

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/doc"
)

// DefaultSeed est la graine utilisée quand aucune n'est donnée.
const DefaultSeed = 1

// Options règle la fabrication d'un classeur de démonstration.
type Options struct {
	Seed uint64 // 0 : celle du plan, sinon DefaultSeed
	// Progress est appelé régulièrement avec le nombre de lignes écrites et le
	// total (peut être appelé depuis la goroutine de l'import).
	Progress func(done, total int)
}

// Result décrit le classeur fabriqué.
type Result struct {
	Sheets   []int64 // onglets créés, dans l'ordre
	Rows     int
	Warnings []string // formules refusées (ne devrait pas arriver)
}

// UnknownPresetError signale un preset inconnu.
type UnknownPresetError struct{ Name string }

func (e UnknownPresetError) Error() string {
	return fmt.Sprintf("preset de démonstration « %s » inconnu ; presets : %s (ou un fichier .yaml, voir lattis --help)", e.Name, strings.Join(Names(), ", "))
}

// Build ajoute au classeur les onglets du preset name (ou du fichier YAML
// name).
func Build(ctx context.Context, m *canvas.Model, name string, opt Options) (Result, error) {
	p, err := Resolve(name)
	if err != nil {
		return Result{}, err
	}
	plan, err := p.Load()
	if err != nil {
		return Result{}, err
	}
	return BuildPlan(ctx, m, plan, opt)
}

// BuildPlan ajoute au classeur les onglets d'un plan : tous les tableaux en
// un seul import, puis les scripts, les requêtes, les tableaux croisés, les
// graphiques et les contrôles, les onglets Document et les pages HTML ;
// enfin les onglets sont rangés dans l'ordre du plan.
func BuildPlan(ctx context.Context, m *canvas.Model, p Plan, opt Options) (Result, error) {
	// Une seule étape d'annulation pour tout le classeur.
	if !command.InGroup(ctx) {
		ctx = command.WithGroup(ctx, "Données de démonstration")
	}
	seed := opt.Seed
	if seed == 0 {
		seed = p.Seed
	}
	if seed == 0 {
		seed = DefaultSeed
	}
	before, err := m.Layout(ctx, 0)
	if err != nil {
		return Result{}, err
	}
	total := p.Rows()
	var done atomic.Int64
	tick := func() {
		if n := done.Add(1); opt.Progress != nil && (n%4096 == 0 || int(n) == total) {
			opt.Progress(int(n), total)
		}
	}
	var sheets []canvas.ImportSheet
	var grids []int // rang dans p.Sheets des feuilles importées
	for k, s := range p.Sheets {
		if s.Query != nil || s.Kind() != "" {
			continue
		}
		grids = append(grids, k)
		is := canvas.ImportSheet{Name: s.Name, Cells: s.Cells}
		for _, t := range s.Tables {
			it := canvas.ImportTable{Name: t.Name, Row: t.Row, Col: t.Col, Count: t.Rows, Key: t.Key}
			for _, c := range t.Cols {
				it.Headers = append(it.Headers, c.Name)
				it.Types = append(it.Types, c.Type)
				it.Formulas = append(it.Formulas, c.Formula)
			}
			t := t
			if t.Select != nil {
				it.Selects, it.SelectKeys = t.Select(seed), true
				base := int(done.Load())
				it.Progress = func(rows int) {
					done.Store(int64(base + rows))
					if opt.Progress != nil {
						opt.Progress(base+rows, total)
					}
				}
				is.Tables = append(is.Tables, it)
				continue
			}
			it.Rows = func(yield func([]any) error) error {
				return Rows(t.Cols, t.Rows, seed, s.Name+"/"+t.Name, func(row []any) error {
					tick()
					return yield(row)
				})
			}
			is.Tables = append(is.Tables, it)
		}
		sheets = append(sheets, is)
	}
	if opt.Progress != nil {
		opt.Progress(0, total)
	}
	ids := make([]int64, len(p.Sheets)) // onglet de chaque SheetSpec
	out := Result{Rows: total}
	if len(sheets) > 0 {
		res, err := m.Import(ctx, "Données de démonstration", sheets)
		if err != nil {
			return Result{}, err
		}
		out.Warnings = res.Warnings
		for i, k := range grids {
			ids[k] = res.Sheets[i]
		}
	}
	if opt.Progress != nil {
		opt.Progress(total, total) // lignes générées en SQL : pas de décompte en route
	}
	for _, s := range p.Scripts {
		if _, err := m.SaveScript(ctx, 0, s.Name, s.Source); err != nil {
			return out, err
		}
	}
	if len(p.Events) > 0 {
		if err := m.SetWorkbookListeners(ctx, p.Events); err != nil {
			return out, err
		}
	}
	for k, s := range p.Sheets {
		if s.Query == nil {
			continue
		}
		id, err := loadQuery(ctx, m, s)
		if err != nil {
			return out, fmt.Errorf("requête « %s » : %w", s.Name, err)
		}
		ids[k] = id
	}
	var extra []int64
	for _, pv := range p.Pivots {
		sheet, err := m.CreatePivot(ctx, pv.Config)
		if err != nil {
			return out, fmt.Errorf("tableau croisé %s : %w", pv.Config.Name, err)
		}
		if pv.Sheet != "" {
			if err := m.RenameSheet(ctx, sheet, pv.Sheet); err != nil {
				return out, err
			}
		}
		extra = append(extra, sheet)
		for _, ch := range pv.Charts {
			if _, err := m.CreateChart(ctx, sheet, ch.Row, ch.Col, ch.Config); err != nil {
				return out, fmt.Errorf("graphique « %s » : %w", ch.Config.Title, err)
			}
		}
	}
	for k, s := range p.Sheets {
		if ids[k] == 0 {
			continue
		}
		if err := placeObjects(ctx, m, ids[k], s); err != nil {
			return out, fmt.Errorf("onglet « %s » : %w", s.Name, err)
		}
	}
	for k, s := range p.Sheets {
		if s.Kind() == "" {
			continue
		}
		id, err := addPage(ctx, m, s)
		if err != nil {
			return out, err
		}
		ids[k] = id
	}
	out.Sheets = append(ids, extra...)
	at := len(before.Sheets)
	for i, id := range out.Sheets {
		if err := m.MoveSheet(ctx, id, at+i); err != nil {
			return out, err
		}
	}
	return out, nil
}

// New crée un classeur neuf (jamais enregistré) rempli avec le preset name
// (ou le fichier YAML name). L'onglet vide d'un classeur neuf est retiré, et
// le classeur est considéré comme « propre » : le fermer sans enregistrer ne
// demande rien et ne laisse rien à récupérer. Ses scripts sont autorisés sur
// ce poste, comme s'ils y avaient été écrits : ils ne demandent pas l'accord.
func New(ctx context.Context, name string, opt Options) (*doc.Document, Result, error) {
	p, err := Resolve(name)
	if err != nil {
		return nil, Result{}, err
	}
	plan, err := p.Load()
	if err != nil {
		return nil, Result{}, err
	}
	d, err := doc.New(ctx)
	if err != nil {
		return nil, Result{}, err
	}
	ctx = command.WithGroup(ctx, "Données de démonstration") // avec la feuille vide retirée
	l, err := d.Layout(ctx, 0)
	if err != nil {
		d.Close()
		return nil, Result{}, err
	}
	res, err := BuildPlan(ctx, d.Model, plan, opt)
	if err == nil {
		// L'import retire déjà la feuille vide d'un classeur neuf ; sans
		// import (requêtes seules), elle est encore là.
		var now canvas.Layout
		now, err = d.Layout(ctx, 0)
		for _, s := range l.Sheets {
			if err != nil || !slices.ContainsFunc(now.Sheets, func(x canvas.SheetInfo) bool { return x.ID == s.ID }) {
				continue
			}
			err = d.DeleteSheet(ctx, s.ID)
		}
	}
	if err == nil && len(plan.Scripts) > 0 {
		err = d.AllowAllScripts(ctx)
	}
	if err == nil && plan.hasMap() {
		// Les fonds de carte (OpenStreetMap) s'affichent sans demander
		// l'accès au réseau, pour cette session seulement (rien n'est
		// mémorisé sur le poste ni dans le classeur).
		d.Model.SetSessionHost(MapTileHost, true)
	}
	if err == nil {
		err = d.MarkSaved(ctx)
	}
	if err != nil {
		d.Close()
		return nil, Result{}, err
	}
	return d, res, nil
}

// MapTileHost est l'hôte des fonds de carte (internal/api, cmd_maptile.go).
const MapTileHost = "tile.openstreetmap.org"

// hasMap dit si le plan a une carte (sur une feuille ou dans un document).
func (p Plan) hasMap() bool {
	for _, s := range p.Sheets {
		for _, c := range s.Charts {
			if c.Config.Kind == "map" {
				return true
			}
		}
		for _, b := range s.Doc {
			if b.Chart != nil && b.Chart.Kind == "map" {
				return true
			}
		}
	}
	for _, pv := range p.Pivots {
		for _, c := range pv.Charts {
			if c.Config.Kind == "map" {
				return true
			}
		}
	}
	return false
}
