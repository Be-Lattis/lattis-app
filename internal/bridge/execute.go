package bridge

import (
	"context"
	"time"
)

// ExecResult est l'issue d'une requête d'écriture de l'onglet SQL.
type ExecResult struct {
	Changed int64    `json:"changed"` // lignes ajoutées, modifiées ou supprimées
	Micros  int64    `json:"micros"`
	State   DocState `json:"state"`
}

// Execute exécute une requête d'écriture (INSERT, UPDATE, DELETE) sur le
// classeur, avec les noms affichés des tableaux. Elle forme une étape
// d'annulation ; les changements de structure sont refusés.
func (s *DocumentService) Execute(ctx context.Context, q string) (ExecResult, error) {
	d, err := s.current()
	if err != nil {
		return ExecResult{}, err
	}
	if q, err = d.CellSQL(ctx, q, s.activeSheet()); err != nil {
		return ExecResult{}, err
	}
	start := time.Now()
	n, err := d.ExecSQL(ctx, q)
	if err != nil {
		return ExecResult{}, err
	}
	res := ExecResult{Changed: n, Micros: time.Since(start).Microseconds()}
	res.State, err = s.changed(ctx)
	return res, err
}
