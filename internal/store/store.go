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

	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/work"
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
	// pendingCells : la tâche en cours a écrit des cellules libres.
	pendingCells bool
	// Lecture par la grille (GridVersion, gridver.go) : gridGen avance à
	// chaque écriture qui peut changer la définition d'un tableau lu
	// (colonnes, filtre, choix de l'onglet, suivi, saisies invalides) ;
	// dataGen à chaque écriture de données (tableaux, cellules libres,
	// noms). pendingGrid, pendingData : la tâche en cours en a fait.
	gridGen, dataGen         int64
	pendingGrid, pendingData bool
	// seen : (opération, table) déjà relevés par noteRow dans la tâche en
	// cours (une fois suffit : settle fait encore avancer les versions).
	seen map[rowKey]bool
	// Écritures par colonne (colver.go) : cgen par colonne (« t_3.c_1 »),
	// agen par tableau pour celles qui peuvent toucher toutes ses colonnes ;
	// pendingCols, pendingAll : celles de la tâche en cours. cur : ce
	// qu'écrit la requête en cours ; allSeen : tableaux déjà comptés dans
	// pendingAll par noteRow ; rebuilds : reconstructions de la tâche en
	// cours (goroutine de l'écrivain seulement).
	cgen, agen              map[string]int64
	pendingCols, pendingAll map[string]bool
	cur                     colWrite
	allSeen                 map[string]bool
	rebuilds                map[string]*rebuild
	busy                    atomic.Bool
	// writeConn : connexion SQLite de l'écrivain (autorisateur, AsUser).
	writeConn atomic.Pointer[sqlite3.Conn]
	// unmark : appelé avant la prochaine tâche de l'écrivain (voir Mark).
	unmark atomic.Pointer[func() error]
	// watched : tables dont les écritures sont comptées dans watchGen (voir
	// Watch : mise en page d'un classeur collaboratif).
	watched  atomic.Pointer[map[string]bool]
	watchGen atomic.Int64
	// scans : relevés des requêtes de l'écrivain, par texte (qscan.go).
	scans scanCache
}

var (
	tableName = regexp.MustCompile(`\bt_\d+\b`)
	// Métadonnées qui changent ce qu'on lit d'un tableau : colonnes, types,
	// formules, saisies invalides, tableaux eux-mêmes, et le choix de l'onglet
	// (canvas/choice.go : il filtre les tableaux de la feuille, comme un
	// filtre ; sans lui, une grille gardait le nombre de lignes de l'ancienne
	// valeur et redemandait sans fin des lignes « hors du tableau »).
	// Toute écriture de _lattis_sheets qui nomme la colonne choice compte
	// (UPDATE … SET choice, mais aussi une feuille rétablie avec son choix,
	// INSERT INTO _lattis_sheets (…, choice)) ; les lignes écrites sont en
	// plus relevées par noteRow (gridver.go), quel que soit le texte.
	tableMeta = regexp.MustCompile(`_lattis_(columns|tables|invalid|capabilities)\b|(?s)_lattis_sheets\b.*\bchoice\b`)
	// Structure des tableaux (noms, colonnes, types) : ce que lit tout calcul
	// sur un tableau (voir Version ; les saisies invalides n'en sont pas) ;
	// le choix de l'onglet aussi (TCD et graphiques de la feuille filtrés).
	schemaMeta = regexp.MustCompile(`_lattis_(columns|tables)\b|(?s)_lattis_sheets\b.*\bchoice\b`)
)

// Watch fait compter les écritures des tables nommées (WatchGen) : lignes
// écrites (noteRow) et requêtes qui les nomment (un DELETE sans WHERE ne
// passe pas par noteRow). Un seul ensemble à la fois (le dernier appel).
func (s *Store) Watch(tables ...string) {
	m := map[string]bool{}
	for _, t := range tables {
		m[t] = true
	}
	s.watched.Store(&m)
}

// WatchGen compte les écritures des tables de Watch (une écriture annulée
// compte aussi) : un relevé fait à la génération g reste valable tant
// qu'elle ne change pas.
func (s *Store) WatchGen() int64 { return s.watchGen.Load() }

// watchedWrite : la requête nomme une table de Watch.
func (s *Store) watchedWrite(q string) {
	w := s.watched.Load()
	if w == nil || !strings.Contains(q, "_lattis_") {
		return
	}
	for t := range *w {
		if strings.Contains(q, t) {
			s.watchGen.Add(1)
			return
		}
	}
}

// noteWrite relève les tableaux qu'une requête de l'écrivain peut modifier.
func (s *Store) noteWrite(q string) {
	defer func() { s.cur = colWrite{} }()
	p := s.scanOf(q)
	if p.read {
		return
	}
	s.trackRebuild(q)
	s.watchedWrite(q)
	names, meta := p.names, p.meta
	if len(names) == 0 && !meta {
		return
	}
	// Une mise à jour qui ne touche pas l'ordre (_pos) ne change que des
	// valeurs : les lignes, leur nombre et leur ordre restent.
	valuesOnly := p.valuesOnly
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if len(names) > 0 {
		s.dataGen++
		s.pendingData = true
	}
	if meta {
		s.touchedAll = true
		s.gridGen++
		s.pendingGrid = true
		if p.schema {
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
	s.noteColumns(names)
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
// change pas, quoi qu'on écrive ailleurs. Avec NoMeta en tête, les
// métadonnées n'y sont pas (voir colver.go).
func (s *Store) Version(tables ...string) string {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	var b strings.Builder
	if len(tables) > 0 && tables[0] == NoMeta {
		tables = tables[1:]
	} else {
		b.WriteString(strconv.FormatInt(s.metaGen, 10))
	}
	for _, n := range tables {
		if t, ok := strings.CutSuffix(n, rowsSuffix); ok {
			b.WriteString("|" + n + ":" + strconv.FormatInt(s.sgen[t], 10))
			continue
		}
		if t, c, ok := strings.Cut(n, "."); ok && isDataTable(t) {
			b.WriteString("|" + n + ":" + s.columnVersion(t, c))
			continue
		}
		b.WriteString("|" + n + ":" + strconv.FormatInt(s.tgen[n], 10))
	}
	return b.String()
}

const rowsSuffix = "#lignes"

// Pending dit si la tâche d'écriture en cours a écrit l'un des tableaux
// nommés (t_N), la structure des tableaux (colonnes, noms, types) ou, avec
// cells, des cellules libres : un tableau lié à une formule n'est recalculé
// que dans ce cas (voir canvas/spilltable.go).
func (s *Store) Pending(cells bool, tables ...string) bool {
	s.tmu.Lock()
	defer s.tmu.Unlock()
	if s.pendingMeta || cells && s.pendingCells {
		return true
	}
	for _, n := range tables {
		if s.pending[n] {
			return true
		}
	}
	return false
}

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
	if s.pendingGrid {
		s.gridGen++
	}
	if s.pendingData {
		s.dataGen++
	}
	s.settleColumns()
	clear(s.pending)
	clear(s.pendingRows)
	clear(s.seen)
	s.pendingMeta, s.pendingGrid, s.pendingData, s.pendingCells = false, false, false, false
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
	// durable : validée avec synchronous=FULL (voir WriteDurable).
	durable bool
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
		// Lignes écrites, quel que soit le texte de la requête
		// (déclencheurs, vues, RETURNING…) : voir noteRow.
		c.UpdateHook(func(op sqlite3.AuthorizerActionCode, schema, table string, _ int64) {
			s.noteRow(op, schema, table)
		})
		return nil
	}, s.preWrite, s.noteWrite)
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
	read.SetMaxOpenConns(readConns())

	s.read, s.write = read, write
	go s.writer()
	return s, nil
}

// Mémoire de SQLite par classeur (audit 6, M9) : chaque connexion a son
// propre cache. 64 Mo pour l'unique connexion d'écriture, 32 Mo pour
// chacune des 4 à 6 connexions de lecture : au plus 256 Mo, au lieu de
// 64 Mo par cœur (plus d'1 Go sur 16 cœurs).
const (
	writeCacheKiB = 65536
	readCacheKiB  = 32768
	minReadConns  = 4
	maxReadConns  = 6
)

// readConns : connexions de lecture, une par cœur entre minReadConns et
// maxReadConns.
func readConns() int { return min(max(minReadConns, runtime.NumCPU()), maxReadConns) }

// setup configure chaque nouvelle connexion.
func setup(readOnly bool, extra []func(*sqlite3.Conn) error) func(*sqlite3.Conn) error {
	cache := writeCacheKiB
	if readOnly {
		cache = readCacheKiB
	}
	return func(c *sqlite3.Conn) error {
		if err := harden(c); err != nil {
			return err
		}
		pragmas := []string{
			"PRAGMA busy_timeout=5000",
			// En WAL, NORMAL suffit contre l'arrêt brutal du processus ; les
			// commandes de l'utilisateur passent en FULL (WriteDurable).
			"PRAGMA synchronous=NORMAL",
			"PRAGMA foreign_keys=ON",
			// Cache de 64 Mo (32 Mo en lecture) : un tri ou un regroupement sur
			// 1 M de lignes reste en mémoire. Au-delà, les tris et index
			// temporaires débordent sur disque (temp_store=FILE) : en mémoire,
			// trier 10 M de lignes épuise la mémoire de SQLite (« out of
			// memory », limitée à 4 Go en WASM).
			"PRAGMA temp_store=FILE",
			"PRAGMA cache_size=-" + strconv.Itoa(cache),
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

// harden protège une connexion d'une base venue d'un fichier reçu (audit 6,
// M2) : les vues, déclencheurs, index et contraintes du fichier n'appellent
// que des fonctions sans effet de bord (trusted_schema désactivé ; les
// fonctions slt_* sont marquées INNOCUOUS), le mode défensif interdit ce
// qui peut corrompre la base (writable_schema, écriture directe des pages,
// journal désactivé), et cell_size_check repère plus tôt une page altérée.
// Aucune extension ne se charge : le moteur SQLite (WASM) n'en a pas, et
// l'autorisateur refuse load_extension.
func harden(c *sqlite3.Conn) error {
	if _, err := c.Config(sqlite3.DBCONFIG_DEFENSIVE, true); err != nil {
		return err
	}
	if _, err := c.Config(sqlite3.DBCONFIG_TRUSTED_SCHEMA, false); err != nil {
		return err
	}
	return c.Exec("PRAGMA cell_size_check=ON")
}

// Read donne le pool de lecture. Les connexions y sont en query_only.
func (s *Store) Read() *sql.DB { return s.read }

// Write exécute fn dans une transaction, sur la connexion d'écriture, après
// les écritures déjà en file. La transaction est validée si fn ne renvoie pas
// d'erreur, annulée sinon.
func (s *Store) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	return s.submit(job{ctx: ctx, fn: fn, res: make(chan error, 1)})
}

// WriteDurable est Write pour une commande de l'utilisateur (audit 6, M3) :
// la validation est mise sur disque (synchronous=FULL), elle survit à une
// coupure de courant. Les autres écritures (import en plusieurs
// transactions, démonstration, relevés d'un serveur) restent en
// synchronous=NORMAL : elles survivent à l'arrêt brutal du processus, mais
// une coupure de courant peut faire perdre les dernières, jusqu'à la
// précédente écriture durable ou au dernier point de contrôle du WAL.
func (s *Store) WriteDurable(ctx context.Context, fn func(*sql.Tx) error) error {
	return s.submit(job{ctx: ctx, fn: fn, res: make(chan error, 1), durable: true})
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
	if j.durable {
		// Hors transaction : SQLite refuse de changer synchronous dans une
		// transaction. Une seule connexion d'écriture : le réglage la suit.
		if _, err := s.write.ExecContext(j.ctx, "PRAGMA synchronous=FULL"); err != nil {
			return err
		}
		defer s.write.ExecContext(context.Background(), "PRAGMA synchronous=NORMAL")
	}
	tx, err = s.write.BeginTx(j.ctx, nil)
	if err != nil {
		return err
	}
	if err := j.fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	// Écriture sur le disque : longue après une grosse modification.
	work.Phase(j.ctx, "commit")
	return tx.Commit()
}

// ErrEngine signale une erreur interne de SQLite pendant une écriture (par
// exemple la mémoire épuisée) ; l'écriture est annulée.
var ErrEngine = msg.New("store.engine", nil)

// Close arrête la file d'écriture et ferme les connexions.
func (s *Store) Close() error {
	close(s.done)
	return errors.Join(s.read.Close(), s.write.Close())
}
