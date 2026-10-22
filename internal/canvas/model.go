// Package canvas est le modèle « canevas ↔ tables » (validé au spike 1.2,
// voir docs/decisions/0001-jalon-m1.md).
//
// Une feuille est un canevas libre ; une zone contiguë qui remplit au moins
// trois cases d'un carré de 2 × 2 (voir dense)
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
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
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
	// dataOnly : saisie dans un tableau d'une feuille à structure protégée
	// (guardCells) : la forme des tableaux ne change pas (colonnes vidées,
	// formules des colonnes calculées).
	dataOnly bool
	// Feuilles et tableaux dont l'action a changé la disposition (le
	// rattachement ne regarde qu'eux) ; layoutAll : une instruction ne se
	// lisait pas, toutes les feuilles sont examinées.
	layoutAll    bool
	layoutSheets map[int64]bool
	layoutTables map[int64]bool
	// noLegend : « Ajouter au tableau », une saisie sous le tableau n'est pas
	// lue comme une note ou un total (voir legend.go).
	noLegend bool
	// inComputedFilter : calcul d'un filtre de colonne calculée en cours
	// (computedCond), qui relit les tableaux : pas de récursion.
	inComputedFilter bool
	// keyAsk : saisie d'une formule (SetCell) ; une recherche dans un tableau
	// sans clé pose la question de la clé (voir prepareFormula). Ailleurs
	// (collage, import, actualisation, conversion, règles), les tableaux sans
	// clé gardent leurs numéros de ligne, sans question.
	keyAsk bool
	// born : tableaux créés d'office par l'action ; undoBirth : une saisie
	// ou un collage, dont le premier Ctrl+Z ne défait que ces créations
	// (voir unbirth.go).
	born      []bornTable
	undoBirth bool
	// autoKeyed : colonne devenue clé d'office à la création d'un tableau
	// (autokey.go), pour le message de la saisie ou du collage.
	autoKeyed string
	// syncingFormulas : recalcul des tableaux liés à une formule en cours
	// (spilltable.go).
	syncingFormulas bool
	// pivotTables : tableaux croisés en cours de calcul comme tableau de
	// formules (pivotref.go), contre la récursion.
	pivotTables map[string]bool
	// typed : écriture de la session d'une saisie tapée (SetCell, collage),
	// lue par input dans la forme gardée (session.go) ; nil : la saisie est
	// déjà dans la forme gardée.
	typed *ui
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
	// structSince : dernière commande du journal au moment où la structure
	// d'une feuille a été protégée (voir historyGuard) ; structBase : celle
	// de l'ouverture, pour une feuille déjà protégée.
	structSince map[int64]int64
	structBase  int64

	// Hôtes HTTP autorisés (true) ou refusés (false) pour la session, sans
	// être mémorisés dans le classeur (voir httphosts.go).
	hostMu  sync.Mutex
	session map[string]bool

	// Message de la dernière action pour la barre d'état (TakeNotice).
	noticeMu sync.Mutex
	notice   string

	// Partage sur un Serveur Lattis (nil : classeur local), voir SetSharing.
	sharing Sharing

	// Locale des formules de la session (langue de l'interface), voir
	// SetLocale ; français par défaut.
	locMu sync.Mutex
	loc   formula.Locale
	// Langue de la session (messages de la barre d'état, notes rendues à
	// l'interface), voir SetLanguage ; français par défaut.
	lang msg.Lang
}

// SetLanguage fixe la langue de la session : les notes et messages que le
// modèle rend à l'interface (TakeNotice, aperçus) s'écrivent dans cette
// langue. Les erreurs, elles, se traduisent au pont (*msg.Error).
func (m *Model) SetLanguage(l msg.Lang) {
	m.locMu.Lock()
	m.lang = l
	m.locMu.Unlock()
}

// Language : la langue de la session (français par défaut).
func (m *Model) Language() msg.Lang {
	m.locMu.Lock()
	defer m.locMu.Unlock()
	if m.lang == "" {
		return msg.FR
	}
	return m.lang
}

// say : le message id dans la langue de la session.
func (m *Model) say(id msg.ID, args msg.Args) string {
	return msg.Format(m.Language(), id, args)
}

// say : le message id dans la langue de la session.
func (t *tx) say(id msg.ID, args msg.Args) string { return t.m.say(id, args) }

// SetLocale fixe la locale des formules de la session (formula.ForLanguage
// de la langue de l'interface). Elle règle aujourd'hui la lecture des
// nombres écrits en texte (Schema.Numbers, décision 0013) ; la saisie et
// l'affichage des formules restent en français (docs/i18n.md, lot 5).
func (m *Model) SetLocale(loc formula.Locale) {
	m.locMu.Lock()
	same := m.loc == loc
	m.loc = loc
	m.locMu.Unlock()
	if !same {
		// Résultats en cache calculés avec l'ancienne lecture.
		m.cacheMu.Lock()
		m.cache, m.depCache = nil, nil
		m.cacheMu.Unlock()
	}
}

// Locale : la locale des formules de la session.
func (m *Model) Locale() formula.Locale {
	m.locMu.Lock()
	defer m.locMu.Unlock()
	return m.loc
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
	m := &Model{db: db, bus: bus, now: time.Now(), seed: rand.Uint64(), structSince: map[int64]int64{}}
	if db != nil {
		// Journal gardé (session récupérée) : ses commandes précèdent peut-être
		// la protection de la structure. Sans journal : 0.
		_ = db.Read().QueryRow(`SELECT coalesce(max(seq), 0) FROM _lattis_journal`).Scan(&m.structBase)
	}
	if bus != nil {
		// Une annulation ne doit pas modifier une feuille protégée.
		// (et les cellules par valeur reviennent sur la valeur choisie :
		// choicecells.go).
		bus.Guard = m.undoGuard
		bus.Undone = m.undone
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

// peekOn rend le résultat gardé par cachedOn pour la clé, s'il vaut encore
// (ok faux sinon), sans rien calculer.
func peekOn[T any](t *tx, key string, deps []string) (v T, ok bool) {
	if t.m == nil {
		return v, false
	}
	m := t.m
	ver := m.db.Version(deps...)
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	if e, found := m.depCache[key]; found && e.ver == ver {
		v, ok = e.v.(T)
	}
	return v, ok
}

// peek rend le résultat gardé par cached pour la clé dans la génération de
// la lecture, sans rien calculer.
func peek[T any](t *tx, key string) (v T, ok bool) {
	if !t.ro || t.m == nil {
		return v, false
	}
	m := t.m
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	if m.cache != nil && m.cacheGen == t.gen {
		if e, found := m.cache[key]; found {
			v, ok = e.(T)
		}
	}
	return v, ok
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
		if err == nil {
			err = t.detachBirth()
		}
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
