package bridge

import (
	"context"
	"sync/atomic"

	"github.com/lalamefine/lattis/internal/demo"
)

// DemoProgress est l'avancement de la fabrication du classeur de démonstration.
type DemoProgress struct {
	Preset string `json:"preset"`
	Done   int    `json:"done"` // lignes écrites
	Total  int    `json:"total"`
	// Cellules écrites (estimées d'après les lignes) et total.
	Cells      int `json:"cells"`
	TotalCells int `json:"totalCells"`
}

// DemoResult est le classeur de démonstration ouvert.
type DemoResult struct {
	State DocState `json:"state"`
	Sheet int64    `json:"sheet"` // premier onglet
}

// demoRun est l'état d'une démonstration : demandée au lancement (--demo),
// ou classeur d'exemple ouvert depuis l'accueil.
type demoRun struct {
	preset      string
	seed        uint64
	done, total atomic.Int64
	cells       int // cellules de données du preset
}

// SetDemo demande, au lancement, un classeur de démonstration (main.go,
// option --demo) ; seed 0 : graine par défaut.
func (s *DocumentService) SetDemo(preset string, seed uint64) {
	s.demo = &demoRun{preset: preset, seed: seed}
	if p, err := demo.Resolve(preset); err == nil {
		if plan, err := p.Load(); err == nil {
			s.demo.cells = plan.Cells()
		}
	}
}

// DemoRequested donne le preset demandé au lancement (--demo), ou "".
func (s *DocumentService) DemoRequested() string {
	if s.demo == nil {
		return ""
	}
	return s.demo.preset
}

// DemoProgress donne l'avancement de OpenDemo ou de OpenSample.
func (s *DocumentService) DemoProgress() DemoProgress {
	r := s.building.Load()
	if r == nil {
		r = s.demo
	}
	if r == nil {
		return DemoProgress{}
	}
	p := DemoProgress{Preset: r.preset, Done: int(r.done.Load()), Total: int(r.total.Load()), TotalCells: r.cells}
	if p.Total > 0 {
		p.Cells = int(int64(p.Done) * int64(p.TotalCells) / int64(p.Total))
	}
	return p
}

// OpenDemo fabrique et ouvre le classeur de démonstration demandé au
// lancement. Il n'est jamais enregistré : Fichier › Enregistrer sous pour
// le garder.
func (s *DocumentService) OpenDemo(ctx context.Context) (DemoResult, error) {
	if s.demo == nil {
		return DemoResult{}, errNoDemo
	}
	return s.openPreset(ctx, s.demo)
}

// SamplePreset est le classeur d'exemple de l'accueil : la vitrine (onglet
// « À découvrir », ventes, TCD, SQL, scripts).
const SamplePreset = demo.DefaultPreset

// OpenSample ouvre le classeur d'exemple depuis l'accueil (audit 6, I8), à
// la place du classeur ouvert (l'interface l'a fermé avant). Comme une
// démonstration, il n'est jamais enregistré de lui-même et ne laisse rien
// à récupérer : le fermer sans l'avoir modifié ne demande rien.
func (s *DocumentService) OpenSample(ctx context.Context) (DemoResult, error) {
	r := &demoRun{preset: SamplePreset}
	if p, err := demo.Resolve(SamplePreset); err == nil {
		if plan, err := p.Load(); err == nil {
			r.cells = plan.Cells()
		}
	}
	return s.openPreset(ctx, r)
}

// openPreset fabrique le classeur de démonstration r et l'ouvre.
func (s *DocumentService) openPreset(ctx context.Context, r *demoRun) (DemoResult, error) {
	s.building.Store(r)
	defer s.building.CompareAndSwap(r, nil)
	d, res, err := demo.New(ctx, r.preset, demo.Options{Seed: r.seed, Progress: func(done, total int) {
		r.done.Store(int64(done))
		r.total.Store(int64(total))
	}})
	if err != nil {
		return DemoResult{}, err
	}
	s.replace(d)
	st, err := s.State(ctx)
	return DemoResult{State: st, Sheet: res.Sheets[0]}, err
}
