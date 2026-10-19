// Package store ouvre la base SQLite d'un document Lattis.
//
// SQLite en WAL accepte plusieurs lecteurs mais un seul écrivain. Le Store
// sépare donc un pool de connexions en lecture (la grille, les requêtes) d'une
// connexion d'écriture unique, possédée par une goroutine qui exécute les
// écritures l'une après l'autre : un script ou une longue transaction ne
// provoque jamais SQLITE_BUSY dans la grille, il passe simplement dans la file.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ncruces/go-sqlite3"
)

// ErrClosed signale une écriture soumise après Close.
var ErrClosed = errors.New("store: base fermée")

// Store regroupe les connexions d'un document.
type Store struct {
	read  *sql.DB
	write *sql.DB
	jobs  chan job
	done  chan struct{}
	// gen compte les écritures : un résultat calculé à la génération g reste
	// valable tant que Generation() vaut g (caches de lecture).
	gen atomic.Int64

	// Tableaux (t_N) modifiés depuis le dernier TakeTouched : contenu
	// (valeurs seulement) ou structure (lignes ajoutées, retirées ou
	// déplacées) ; touchedAll : une écriture qui peut les concerner tous
	// (colonnes, formules…).
	tmu        sync.Mutex
	content    map[string]bool
	structure  map[string]bool
	touchedAll bool
	// Écritures par tableau (t_N) et sur les métadonnées, depuis l'ouverture :
	// la version d'un calcul qui ne lit que quelques tableaux (Version).
	tgen    map[string]int64
	metaGen int64
	// sgen : écritures qui changent les lignes d'un tableau (pas seulement
	// des valeurs) ; voir Version, suffixe RowsOf.
	sgen map[string]int64
	// pending : tableaux écrits par la tâche en cours, dont la version
	// avance encore à sa fin (validée ou annulée) ; busy : une tâche tourne.
	pending     map[string]bool
	pendingRows map[string]bool
	pendingMeta bool
	busy        atomic.Bool
	// writeConn : connexion SQLite de l'écrivain (autorisateur, AsUser).
	writeConn atomic.Pointer[sqlite3.Conn]
	// unmark : appelé avant la prochaine tâche de l'écrivain (voir Mark).
	unmark atomic.Pointer[func() error]
}

var (
	tableName = regexp.MustCompile(`\bt_\d+\b`)
	// Métadonnées qui changent ce qu'on lit d'un tableau : colonnes, types,
	// formules, saisies invalides, tableaux eux-mêmes.
	tableMeta = regexp.MustCompile(`_lattis_(columns|tables|invalid)\b`)
	// Structure des tableaux (noms, colonnes, types) : ce que lit tout calcul
	// sur un tableau (voir Version ; les saisies invalides n'en sont pas).
	schemaMeta = regexp.MustCompile(`_lattis_(columns|tables)\b`)
)

// noteWrite relève les tableaux qu'une requête de l'écrivain peut modifier.
func (s *Store) noteWrite(q string) {
	head := strings.ToUpper(strings.TrimSpace(q))
	if strings.HasPrefix(head, "SELECT") || strings.HasPrefix(head, "PRAGMA") {
		return
	}
	names := tableName.FindAllString(q, -1)
	meta := tableMeta.MatchString(q)
	if len(names) == 0 && !meta {
		return
	}
	// Une mise à jour qui ne touche pas l'ordre (_pos) ne change que des
	// valeurs : les lignes, leur nombre et leur ordre restent.
	valuesOnly := strings.HasPrefix(head, "UPDATE") && !strings.Contains(head, "_POS")
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if meta {
		s.touchedAll = true
		if schemaMeta.MatchString(q) {
			s.metaGen++
			s.pendingMeta = true
		}
	}
	if s.tgen == nil {
		s.tgen = map[string]int64{}
	}
	if s.pending == nil {
		s.pending, s.pendingRows, s.sgen = map[string]bool{}, map[string]bool{}, map[string]int64{}
	}
	for _, n := range names {
		s.tgen[n]++
		s.pending[n] = true
		if !valuesOnly {
			s.sgen[n]++
			s.pendingRows[n] = true
		}
	}
	if s.content == nil {
		s.content, s.structure = map[string]bool{}, map[string]bool{}
	}
	for _, n := range names {
		if valuesOnly {
			s.content[n] = true
		} else {
			s.structure[n] = true
		}
	}
}

// TakeTouched donne, et remet à zéro, les tableaux modifiés depuis l'appel
// précédent : dans leurs valeurs seulement (content) ou dans leurs lignes
// (structure) ; all : peut-être tous. Un tableau non touché garde ses
// caches de lecture ; un tableau touché dans ses valeurs garde ses lignes.
func (s *Store) TakeTouched() (content, structure map[string]bool, all bool) {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	content, structure, all = s.content, s.structure, s.touchedAll
	s.content, s.structure, s.touchedAll = nil, nil, false
	return content, structure, all
}

// Version résume les écritures sur les tableaux nommés (t_N) et sur les
// métadonnées : un calcul qui ne lit qu'eux reste valable tant qu'elle ne
// change pas, quoi qu'on écrive ailleurs.
func (s *Store) Version(tables ...string) string {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	var b strings.Builder
	b.WriteString(strconv.FormatInt(s.metaGen, 10))
	for _, n := range tables {
		if t, ok := strings.CutSuffix(n, rowsSuffix); ok {
			b.WriteString("|" + n + ":" + strconv.FormatInt(s.sgen[t], 10))
			continue
		}
		b.WriteString("|" + n + ":" + strconv.FormatInt(s.tgen[n], 10))
	}
	return b.String()
}

const rowsSuffix = "#lignes"

// RowsOf désigne, pour Version, les lignes du tableau t_N seulement
// (ajouts, suppressions, ordre) : un calcul qui n'en lit pas les valeurs,
// comme leur nombre, reste valable après une saisie.
func RowsOf(table string) string { return table + rowsSuffix }

// settle fait encore avancer, à la fin d'une tâche, la version des
// tableaux qu'elle a écrits : un calcul lu pendant la tâche (avant sa
// validation, ou annulée ensuite) ne sert plus.
func (s *Store) settle() {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	for n := range s.pending {
		s.tgen[n]++
	}
	for n := range s.pendingRows {
		s.sgen[n]++
	}
	if s.pendingMeta {
		s.metaGen++
	}
	clear(s.pending)
	clear(s.pendingRows)
	s.pendingMeta = false
}

// Quiet donne la génération si aucune écriture ne tourne (-1 sinon) : un
// calcul de lecture n'est gardé en cache que si Quiet n'a pas changé
// pendant qu'il tournait (il ne peut alors avoir vu qu'un état validé).
func (s *Store) Quiet() int64 {
	if s.busy.Load() {
		return -1
	}
	g := s.gen.Load()
	if s.busy.Load() {
		return -1
	}
	return g
}

// Generation donne le nombre d'écritures passées par le Store.
func (s *Store) Generation() int64 { return s.gen.Load() }

type job struct {
	ctx context.Context
	fn  func(*sql.Tx) error
	raw func(*sql.DB) error // hors transaction (VACUUM, checkpoint)
	res chan error
}

// Open ouvre (ou crée) la base au chemin donné, en mode WAL. extra est
// appelé sur chaque nouvelle connexion (fonctions SQL personnalisées).
func Open(path string, extra ...func(*sqlite3.Conn) error) (*Store, error) {
	s := &Store{jobs: make(chan job), done: make(chan struct{})}
	initWrite := setup(false, extra)
	write, err := openDB("file:"+path, "écriture", func(c *sqlite3.Conn) error {
		if err := initWrite(c); err != nil {
			return err
		}
		// Une seule connexion d'écriture à la fois (SetMaxOpenConns(1)) : la
		// dernière ouverte est celle des écritures (AsUser).
		s.writeConn.Store(c)
		return nil
	}, s.noteWrite)
	if err != nil {
		return nil, err
	}
	write.SetMaxOpenConns(1)
	// journal_mode est persistant : il suffit de le poser une fois, depuis
	// l'écrivain, avant d'ouvrir les lecteurs.
	if _, err := write.Exec("PRAGMA journal_mode=WAL"); err != nil {
		write.Close()
		return nil, err
	}
	read, err := OpenDB("file:"+path, "lecture", setup(true, extra))
	if err != nil {
		write.Close()
		return nil, err
	}
	read.SetMaxOpenConns(max(4, runtime.NumCPU()))

	s.read, s.write = read, write
	go s.writer()
	return s, nil
}

// setup configure chaque nouvelle connexion.
func setup(readOnly bool, extra []func(*sqlite3.Conn) error) func(*sqlite3.Conn) error {
	return func(c *sqlite3.Conn) error {
		pragmas := []string{
			"PRAGMA busy_timeout=5000",
			"PRAGMA synchronous=NORMAL",
			"PRAGMA foreign_keys=ON",
			// Cache de 64 Mo : un tri ou un regroupement sur 1 M de lignes reste
			// en mémoire. Au-delà, les tris et index temporaires débordent sur
			// disque (temp_store=FILE) : en mémoire, trier 10 M de lignes épuise
			// la mémoire de SQLite (« out of memory », limitée à 4 Go en WASM).
			"PRAGMA temp_store=FILE",
			"PRAGMA cache_size=-65536",
		}
		if readOnly {
			pragmas = append(pragmas, "PRAGMA query_only=ON")
		}
		for _, p := range pragmas {
			if err := c.Exec(p); err != nil {
				return err
			}
		}
		for _, f := range extra {
			if err := f(c); err != nil {
				return err
			}
		}
		return nil
	}
}

// Read donne le pool de lecture. Les connexions y sont en query_only.
func (s *Store) Read() *sql.DB { return s.read }

// Write exécute fn dans une transaction, sur la connexion d'écriture, après
// les écritures déjà en file. La transaction est validée si fn ne renvoie pas
// d'erreur, annulée sinon.
func (s *Store) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	return s.submit(job{ctx: ctx, fn: fn, res: make(chan error, 1)})
}

// Exec exécute fn sur la connexion d'écriture, hors transaction, à son tour
// dans la file : pour les instructions qui refusent une transaction (VACUUM
// INTO, wal_checkpoint).
func (s *Store) Exec(ctx context.Context, fn func(*sql.DB) error) error {
	return s.submit(job{ctx: ctx, raw: fn, res: make(chan error, 1)})
}

func (s *Store) submit(j job) error {
	select {
	case s.jobs <- j:
	case <-s.done:
		return ErrClosed
	case <-j.ctx.Done():
		return j.ctx.Err()
	}
	return <-j.res
}

// Mark exécute mark dans la file d'écriture, si aucune tâche n'y est passée
// depuis la génération since (since < 0 : sans condition), et appelle
// unmark juste avant la tâche suivante, quelle qu'elle soit : une marque
// posée par mark (« rien de modifié ») ne survit pas à une écriture. Si
// unmark échoue, la tâche suivante est refusée avec son erreur (et unmark
// réessayé à la suivante). ok : la marque a été posée.
func (s *Store) Mark(ctx context.Context, since int64, mark func() error, unmark func() error) (ok bool, err error) {
	err = s.submit(job{ctx: ctx, res: make(chan error, 1), raw: func(*sql.DB) error {
		if since >= 0 && s.gen.Load() != since {
			return nil
		}
		if err := mark(); err != nil {
			return err
		}
		s.unmark.Store(&unmark)
		ok = true
		return nil
	}})
	return ok, err
}

func (s *Store) writer() {
	for {
		select {
		case j := <-s.jobs:
			if f := s.unmark.Load(); f != nil {
				if err := (*f)(); err != nil {
					j.res <- err
					continue
				}
				s.unmark.CompareAndSwap(f, nil)
			}
			s.busy.Store(true)
			err := s.run(j)
			s.settle()
			s.gen.Add(1)
			s.busy.Store(false)
			j.res <- err
		case <-s.done:
			return
		}
	}
}

func (s *Store) run(j job) (err error) {
	if err := j.ctx.Err(); err != nil {
		return err
	}
	// Une panique de SQLite (mémoire épuisée…) devient l'erreur de cette
	// écriture : l'application l'affiche au lieu de s'arrêter.
	var tx *sql.Tx
	defer func() {
		if r := recover(); r != nil {
			if tx != nil {
				func() {
					defer func() { recover() }()
					tx.Rollback()
				}()
			}
			err = fmt.Errorf("%w : %v", ErrEngine, r)
		}
	}()
	if j.raw != nil {
		return j.raw(s.write)
	}
	tx, err = s.write.BeginTx(j.ctx, nil)
	if err != nil {
		return err
	}
	if err := j.fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ErrEngine signale une erreur interne de SQLite pendant une écriture (par
// exemple la mémoire épuisée) ; l'écriture est annulée.
var ErrEngine = errors.New("opération impossible, le classeur n'a pas été modifié")

// Close arrête la file d'écriture et ferme les connexions.
func (s *Store) Close() error {
	close(s.done)
	return errors.Join(s.read.Close(), s.write.Close())
}
