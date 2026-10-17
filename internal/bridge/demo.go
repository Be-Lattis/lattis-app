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

// demoRun est l'état de la démonstration demandée au lancement (--demo).
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
	if p, ok := demo.Lookup(preset); ok {
		s.demo.cells = p.Plan().Cells()
	}
}

// DemoRequested donne le preset demandé au lancement (--demo), ou "".
func (s *DocumentService) DemoRequested() string {
	if s.demo == nil {
		return ""
	}
	return s.demo.preset
}

// DemoProgress donne l'avancement de OpenDemo.
func (s *DocumentService) DemoProgress() DemoProgress {
	if s.demo == nil {
		return DemoProgress{}
	}
	p := DemoProgress{Preset: s.demo.preset, Done: int(s.demo.done.Load()), Total: int(s.demo.total.Load()), TotalCells: s.demo.cells}
	if p.Total > 0 {
		p.Cells = int(int64(p.Done) * int64(p.TotalCells) / int64(p.Total))
	}
	return p
}

// OpenDemo fabrique et ouvre le classeur de démonstration demandé au
// lancement. Il n'est jamais enregistré : Fichier › Enregistrer sous pour
// le garder.
func (s *DocumentService) OpenDemo(ctx context.Context) (DemoResult, error) {
	r := s.demo
	if r == nil {
		return DemoResult{}, errNoDemo
	}
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
