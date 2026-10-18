package demo

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
)

// DefaultSeed est la graine utilisée quand aucune n'est donnée.
const DefaultSeed = 1

// Options règle la fabrication d'un classeur de démonstration.
type Options struct {
	Seed uint64 // 0 : DefaultSeed
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
	return fmt.Sprintf("preset de démonstration « %s » inconnu ; presets : %s", e.Name, strings.Join(Names(), ", "))
}

// Build ajoute au classeur les onglets du preset name.
func Build(ctx context.Context, m *canvas.Model, name string, opt Options) (Result, error) {
	p, ok := Lookup(name)
	if !ok {
		return Result{}, UnknownPresetError{name}
	}
	return BuildPlan(ctx, m, p.Plan(), opt)
}

// BuildPlan ajoute au classeur les onglets d'un plan : tous les tableaux en
// un seul import (une étape d'annulation), puis les tableaux croisés et les
// graphiques.
func BuildPlan(ctx context.Context, m *canvas.Model, p Plan, opt Options) (Result, error) {
	seed := opt.Seed
	if seed == 0 {
		seed = DefaultSeed
	}
	total := p.Rows()
	var done atomic.Int64
	tick := func() {
		if n := done.Add(1); opt.Progress != nil && (n%4096 == 0 || int(n) == total) {
			opt.Progress(int(n), total)
		}
	}
	var sheets []canvas.ImportSheet
	for _, s := range p.Sheets {
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
	res, err := m.Import(ctx, "Données de démonstration", sheets)
	if err != nil {
		return Result{}, err
	}
	if opt.Progress != nil {
		opt.Progress(total, total) // lignes générées en SQL : pas de décompte en route
	}
	out := Result{Sheets: res.Sheets, Rows: total, Warnings: res.Warnings}
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
		out.Sheets = append(out.Sheets, sheet)
		for _, ch := range pv.Charts {
			if _, err := m.CreateChart(ctx, sheet, ch.Row, ch.Col, ch.Config); err != nil {
				return out, fmt.Errorf("graphique « %s » : %w", ch.Config.Title, err)
			}
		}
	}
	return out, nil
}

// New crée un classeur neuf (jamais enregistré) rempli avec le preset name.
// L'onglet vide d'un classeur neuf est retiré, et le classeur est considéré
// comme « propre » : le fermer sans enregistrer ne demande rien et ne laisse
// rien à récupérer.
func New(ctx context.Context, name string, opt Options) (*doc.Document, Result, error) {
	if _, ok := Lookup(name); !ok {
		return nil, Result{}, UnknownPresetError{name}
	}
	d, err := doc.New(ctx)
	if err != nil {
		return nil, Result{}, err
	}
	l, err := d.Layout(ctx, 0)
	if err != nil {
		d.Close()
		return nil, Result{}, err
	}
	res, err := Build(ctx, d.Model, name, opt)
	if err == nil {
		for _, s := range l.Sheets {
			if err = d.DeleteSheet(ctx, s.ID); err != nil {
				break
			}
		}
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
