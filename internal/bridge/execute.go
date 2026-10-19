package bridge

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ExecResult est l'issue d'une requête d'écriture de l'onglet SQL.
type ExecResult struct {
	Changed int64    `json:"changed"` // lignes ajoutées, modifiées ou supprimées
	Micros  int64    `json:"micros"`
	State   DocState `json:"state"`
}

// SQLWriteTimeout borne une requête d'écriture de l'onglet SQL : elle tient
// la file d'écriture du classeur et bloque, le temps qu'elle dure, toutes
// les saisies. Au-delà, elle est annulée (rien n'est écrit). Les lectures
// ont leur propre délai (grid.Query).
var SQLWriteTimeout = 2 * time.Minute

var errSQLStopped = errors.New("requête arrêtée par l'utilisateur")

// sqlRuns : requêtes en cours de l'onglet SQL, que StopSQL annule.
type sqlRuns struct {
	mu     sync.Mutex
	next   int
	cancel map[int]context.CancelCauseFunc
}

// sqlContext donne le contexte d'une requête de l'onglet SQL (annulée par
// StopSQL ; timeout > 0 : délai) et la fonction qui la termine.
func (s *DocumentService) sqlContext(ctx context.Context, timeout time.Duration) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(ctx)
	stopTimer := func() bool { return false }
	if timeout > 0 {
		t := time.AfterFunc(timeout, func() {
			cancel(fmt.Errorf("requête arrêtée : durée maximale (%v) dépassée ; aucune ligne n'a été modifiée", timeout))
		})
		stopTimer = t.Stop
	}
	r := &s.sql
	r.mu.Lock()
	r.next++
	id := r.next
	if r.cancel == nil {
		r.cancel = map[int]context.CancelCauseFunc{}
	}
	r.cancel[id] = cancel
	r.mu.Unlock()
	return ctx, func() {
		stopTimer()
		r.mu.Lock()
		delete(r.cancel, id)
		r.mu.Unlock()
		cancel(nil)
	}
}

// sqlErr remplace l'erreur d'une requête annulée par sa raison.
func sqlErr(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) {
		return cause
	}
	return err
}

// StopSQL arrête les requêtes en cours de l'onglet SQL (bouton Arrêter) :
// une écriture arrêtée n'a rien modifié.
func (s *DocumentService) StopSQL() {
	s.sql.mu.Lock()
	defer s.sql.mu.Unlock()
	for _, c := range s.sql.cancel {
		c(errSQLStopped)
	}
}

// Execute exécute une requête d'écriture (INSERT, UPDATE, DELETE) sur le
// classeur, avec les noms affichés des tableaux. Elle forme une étape
// d'annulation ; les changements de structure sont refusés. Elle s'arrête
// avec StopSQL ou au bout de SQLWriteTimeout.
func (s *DocumentService) Execute(ctx context.Context, q string) (ExecResult, error) {
	d, err := s.current()
	if err != nil {
		return ExecResult{}, err
	}
	if q, err = d.CellSQL(ctx, q, s.activeSheet()); err != nil {
		return ExecResult{}, err
	}
	start := time.Now()
	qctx, done := s.sqlContext(ctx, SQLWriteTimeout)
	n, err := d.ExecSQL(qctx, q)
	err = sqlErr(qctx, err)
	done()
	if err != nil {
		return ExecResult{}, err
	}
	res := ExecResult{Changed: n, Micros: time.Since(start).Microseconds()}
	res.State, err = s.changed(ctx)
	return res, err
}
