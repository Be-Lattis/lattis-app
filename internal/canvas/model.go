// Package canvas est le modèle « canevas ↔ tables » (validé au spike 1.2,
// voir docs/decisions/0001-jalon-m1.md).
//
// Une feuille est un canevas libre ; une zone contiguë d'au moins deux lignes
// y devient un tableau, stocké dans une vraie table SQLite STRICT. Les
// métadonnées (feuilles, ancres, colonnes, types, cellules libres, saisies
// invalides) vivent dans les tables _lattis_* (internal/meta).
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

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/store"
)

type stmt = command.Stmt

var st = command.S

// tx est la transaction d'une action en cours.
type tx struct {
	*command.Tx
	m *Model
	// keyed : colonne devenue clé pendant la transaction ("Tableau.Colonne"),
	// pour l'annoncer quand c'est une formule qui l'a posée.
	keyed string
	// pasting : un collage saisit les cellules ; un en-tête collé déjà pris
	// dans le tableau est renommé (« Produit 2 ») au lieu d'être refusé.
	pasting bool
	// collect : collage ou recopie ; une valeur hors du type figé d'un
	// tableau publié est écartée et listée (Event.Refused) au lieu de
	// refuser toute l'action (§9).
	collect bool
	// ro : transaction de lecture ; gen : génération du Store relevée avant
	// son ouverture (les résultats calculés peuvent être mis en cache).
	ro  bool
	gen int64
	// layout : l'action a changé la disposition d'une feuille (ancres,
	// colonnes, lignes, cellules libres) ; joining : rattachement en cours ;
	// notes : messages pour la barre d'état (voir adjacent.go).
	layout  bool
	joining bool
	notes   []string
	// Feuilles et tableaux dont l'action a changé la disposition (le
	// rattachement ne regarde qu'eux) ; layoutAll : une instruction ne se
	// lisait pas, toutes les feuilles sont examinées.
	layoutAll    bool
	layoutSheets map[int64]bool
	layoutTables map[int64]bool
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

	// Cache des calculs lourds en lecture (TCD, graphiques, formules libres,
	// filtres), valable pour une génération du Store.
	cacheMu  sync.Mutex
	cache    map[string]any
	cacheGen int64
	// depCache : résultats qui ne dépendent que de quelques tableaux (voir
	// cachedOn), gardés tant que ces tableaux ne changent pas.
	depCache map[string]depEntry

	// Changements de protection (hors journal) depuis l'ouverture.
	protMu  sync.Mutex
	protGen int64

	// Hôtes HTTP autorisés (true) ou refusés (false) pour la session, sans
	// être mémorisés dans le classeur (voir httphosts.go).
	hostMu  sync.Mutex
	session map[string]bool

	// Message de la dernière action pour la barre d'état (TakeNotice).
	noticeMu sync.Mutex
	notice   string

	// Partage sur un Serveur Lattis (nil : classeur local), voir SetSharing.
	sharing Sharing
}

// Sharing décide, dans un classeur partagé sur un Serveur Lattis, quels
// tableaux sont publiés et lesquels se modifient (internal/remote/workbook).
type Sharing interface {
	// TableLock refuse la modification d'un tableau publié en lecture
	// seule (classeur figé, hors ligne, droits) ; nil : permise.
	TableLock(tbl int64) error
	// TableShare : "published", "private" (tableau resté local d'un
	// classeur partagé) ou "" (classeur local).
	TableShare(tbl int64) string
}

// SetSharing branche le partage du classeur (à l'ouverture, avant tout
// usage du modèle).
func (m *Model) SetSharing(s Sharing) { m.sharing = s }

// New relie le modèle à la base (déjà migrée) et au bus de commandes (dont
// il garde les annulations : voir protect.go).
func New(db *store.Store, bus *command.Bus) *Model {
	m := &Model{db: db, bus: bus, now: time.Now(), seed: rand.Uint64()}
	if bus != nil {
		// Une annulation ne doit pas modifier une feuille protégée.
		bus.Guard = m.historyGuard
	}
	return m
}

// Recalc renouvelle l'instant et la graine des fonctions volatiles (F9).
func (m *Model) Recalc() {
	m.mu.Lock()
	m.now, m.seed = time.Now(), rand.Uint64()
	m.mu.Unlock()
	m.cacheMu.Lock()
	m.cache = nil
	m.depCache = nil
	m.cacheMu.Unlock()
}

type depEntry struct {
	ver string
	v   any
}

// cachedOn est cached pour un calcul qui ne lit que les tableaux deps (noms
// SQL t_N) et les métadonnées : il reste valable après des écritures
// ailleurs (un TCD ou un graphique ne se recalcule plus quand on modifie un
// autre tableau ou une cellule libre).
func cachedOn[T any](t *tx, key string, deps []string, fn func() (T, error)) (T, error) {
	if t.m == nil {
		return fn()
	}
	m := t.m
	ver := m.db.Version(deps...)
	m.cacheMu.Lock()
	if e, ok := m.depCache[key]; ok && e.ver == ver {
		m.cacheMu.Unlock()
		return e.v.(T), nil
	}
	m.cacheMu.Unlock()
	// Une écriture le lit (version à jour) mais ne garde rien : elle peut
	// être annulée. Une lecture garde son résultat si aucune écriture n'a
	// tourné pendant le calcul.
	quiet := m.db.Quiet()
	v, err := fn()
	if err == nil && t.ro && quiet >= 0 && m.db.Quiet() == quiet && m.db.Version(deps...) == ver {
		m.cacheMu.Lock()
		if m.depCache == nil {
			m.depCache = map[string]depEntry{}
		}
		if len(m.depCache) > 256 {
			clear(m.depCache)
		}
		m.depCache[key] = depEntry{ver, v}
		m.cacheMu.Unlock()
	}
	return v, err
}

// cached rend le résultat de fn pour la clé, calculé une fois par génération
// du Store (lectures seulement : dans une écriture, fn est toujours appelée).
func cached[T any](t *tx, key string, fn func() (T, error)) (T, error) {
	if !t.ro || t.m == nil {
		return fn()
	}
	m := t.m
	m.cacheMu.Lock()
	if m.cache == nil || m.cacheGen != t.gen {
		m.cache, m.cacheGen = map[string]any{}, t.gen
	}
	if v, ok := m.cache[key]; ok {
		m.cacheMu.Unlock()
		return v.(T), nil
	}
	m.cacheMu.Unlock()
	v, err := fn()
	if err == nil {
		m.cacheMu.Lock()
		if m.cache != nil && m.cacheGen == t.gen {
			m.cache[key] = v
		}
		m.cacheMu.Unlock()
	}
	return v, err
}

func (m *Model) volatile() (time.Time, uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now, m.seed
}

// Do exécute une action dans une transaction du bus de commandes.
func (m *Model) Do(ctx context.Context, name string, fn func(*tx) error) error {
	var notes []string
	_, err := m.bus.Do(ctx, name, func(c *command.Tx) error {
		t, err := m.run(c, fn)
		notes = t.notes
		return err
	})
	if err == nil {
		m.note(notes)
	}
	return err
}

// read exécute fn dans une transaction de lecture (rien n'est enregistré).
func (m *Model) read(ctx context.Context, fn func(*tx) error) error {
	gen := m.db.Generation()
	sqlTx, err := m.db.Read().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer sqlTx.Rollback()
	return fn(&tx{Tx: &command.Tx{Tx: sqlTx, Ctx: ctx}, m: m, ro: true, gen: gen})
}
