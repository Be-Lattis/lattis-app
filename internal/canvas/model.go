// Package canvas est le modèle « canevas ↔ tables » (validé au spike 1.2,
// voir docs/decisions/0001-jalon-m1.md).
//
// Une feuille est un canevas libre ; une zone contiguë d'au moins deux lignes
// y devient un tableau, stocké dans une vraie table SQLite STRICT. Les
// métadonnées (feuilles, ancres, colonnes, types, cellules libres, saisies
// invalides) vivent dans les tables _slitable_* (internal/meta).
//
// Toute modification passe par le bus de commandes (internal/command) :
// chaque opération élémentaire enregistre ses instructions inverses, et une
// action de l'utilisateur forme une étape d'annulation.
package canvas

import (
	"context"
	"database/sql"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/lalamefine/slitable/internal/command"
	"github.com/lalamefine/slitable/internal/store"
)

type stmt = command.Stmt

var st = command.S

// tx est la transaction d'une action en cours.
type tx struct {
	*command.Tx
	m *Model
}

func (t *tx) apply(do, undo []stmt) error { return t.Apply(do, undo) }

func (t *tx) query(q string, args ...any) (*sql.Rows, error) { return t.Query(q, args...) }

func (t *tx) row(q string, args ...any) *sql.Row { return t.Row(q, args...) }

// Model donne accès aux feuilles et tableaux d'un document.
type Model struct {
	db  *store.Store
	bus *command.Bus

	// Instant et graine des fonctions volatiles (AUJOURDHUI, MAINTENANT,
	// ALEA) : fixés à l'ouverture, renouvelés par Recalc (F9).
	mu   sync.Mutex
	now  time.Time
	seed uint64
}

// New relie le modèle à la base (déjà migrée) et au bus de commandes.
func New(db *store.Store, bus *command.Bus) *Model {
	return &Model{db: db, bus: bus, now: time.Now(), seed: rand.Uint64()}
}

// Recalc renouvelle l'instant et la graine des fonctions volatiles (F9).
func (m *Model) Recalc() {
	m.mu.Lock()
	m.now, m.seed = time.Now(), rand.Uint64()
	m.mu.Unlock()
}

func (m *Model) volatile() (time.Time, uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now, m.seed
}

// Do exécute une action dans une transaction du bus de commandes.
func (m *Model) Do(ctx context.Context, name string, fn func(*tx) error) error {
	_, err := m.bus.Do(ctx, name, func(c *command.Tx) error { return fn(&tx{c, m}) })
	return err
}

// read exécute fn dans une transaction de lecture (rien n'est enregistré).
func (m *Model) read(ctx context.Context, fn func(*tx) error) error {
	sqlTx, err := m.db.Read().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer sqlTx.Rollback()
	return fn(&tx{&command.Tx{Tx: sqlTx, Ctx: ctx}, m})
}
