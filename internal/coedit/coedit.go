// Package coedit fait travailler plusieurs postes sur un même classeur
// .lattis posé dans un dossier partagé (NAS, partage Windows), sans
// serveur : chaque session écrit son propre journal, en ajout seulement,
// dans le dossier caché « ~$nom.lattis.d » à côté du fichier, et lit ceux
// des autres pour appliquer leurs modifications en direct. Voir
// docs/decisions/0011-coedition-nas.md.
//
// Les opérations sont celles du journal logique des classeurs partagés
// (internal/remote/oplog), relevées sur tous les tableaux (SetAll), et
// s'appliquent par le même chemin que les modifications reçues d'un
// Serveur Lattis (canvas.Model.ApplyRemote). Pendant le travail à
// plusieurs, seules les données des tableaux se modifient.
package coedit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/trace"
)

// Timing règle les délais (les essais les raccourcissent).
type Timing struct {
	// Poll : un tour (envoi de la file, lecture des journaux) ; 500 ms.
	Poll time.Duration
	// Watch : regard d'une session seule sur le dossier ; 2 s.
	Watch time.Duration
	// Beat : battement de la présence ; 2 s.
	Beat time.Duration
	// Stale : battement arrêté depuis Stale, la session est partie ; 30 s.
	Stale time.Duration
	// JoinWait : attente de l'accueil des autres sessions ; 20 s.
	JoinWait time.Duration
	// Compact : intervalle du compactage des journaux ; 10 s.
	Compact time.Duration
}

func (t Timing) withDefaults() Timing {
	def := func(d *time.Duration, v time.Duration) {
		if *d <= 0 {
			*d = v
		}
	}
	def(&t.Poll, 500*time.Millisecond)
	def(&t.Watch, 2*time.Second)
	def(&t.Beat, 2*time.Second)
	def(&t.Stale, 30*time.Second)
	def(&t.JoinWait, 20*time.Second)
	def(&t.Compact, 10*time.Second)
	return t
}

// Options règle une session.
type Options struct {
	Timing Timing
	// Save enregistre le classeur dans son fichier (une autre session
	// demande à rejoindre et celle-ci a des modifications hors journal) ;
	// nil : Document.Save.
	Save func(ctx context.Context) error
	// OnEvent est appelé depuis le fil de la session, hors de tout verrou.
	OnEvent func(Event)
}

// Event décrit ce qu'un tour a changé, pour l'interface.
type Event struct {
	// Data : des données ont été reçues (grille à relire).
	Data bool
	// State : présences, conflits, état du dossier (indicateur).
	State bool
	// Notices : messages pour la barre d'état.
	Notices []string
}

// Peer est une session présente, pour l'indicateur.
type Peer struct {
	Session string    `json:"session"`
	User    string    `json:"user"`
	Machine string    `json:"machine"`
	Since   time.Time `json:"since"`
	Joining bool      `json:"joining,omitempty"`
}

// State décrit le travail à plusieurs pour l'interface.
type State struct {
	// Active : la session relève ses transactions dans son journal.
	Active bool   `json:"active"`
	Peers  []Peer `json:"peers"`
	// Conflicts : conflits ouverts (volet Conflits).
	Conflicts int `json:"conflicts,omitempty"`
	// Offline : dossier injoignable (raison), vide sinon.
	Offline string `json:"offline,omitempty"`
	// Stale : l'état local ne peut plus rejoindre celui des autres
	// (barrière), vide sinon.
	Stale string `json:"stale,omitempty"`
}

// appliedKey : positions appliquées des journaux, dans _lattis_info (JSON),
// écrites avec les données ; l'enregistrement les met dans le manifeste.
const appliedKey = "coedit_applied"

// Engine est la session de travail à plusieurs d'un classeur ouvert.
type Engine struct {
	d    *doc.Document
	path string
	dir  string
	opt  Options
	t    Timing
	me   archive.Holder
	sid  string

	kick   chan struct{}
	cancel context.CancelFunc
	done   chan struct{}
	// run : un tour, un enregistrement (Before, After, Known) à la fois.
	run sync.Mutex

	// État partagé avec l'interface et le relevé (mu).
	mu        sync.Mutex
	active    bool
	peers     []Presence // sessions vivantes (hors soi)
	offline   string
	stale     string
	conflicts int
	// others : sessions vivantes autres que soi (refus des changements
	// hors données, lu dans les transactions du bus).
	others atomic.Int32
	// localOnly : modifications hors journal depuis le dernier
	// enregistrement (seule : changements hors données).
	localOnly atomic.Bool
	// barrier : une ligne « barrier » est à écrire.
	barrier atomic.Bool
	// inside : enregistrement fait depuis un tour (run déjà tenu).
	inside atomic.Bool

	// Fil de la session (run).
	presence Presence
	welcome  []string
	roster   *roster
	w        writer
	seg      int
	journal  string
	seq      int64
	clock    int64
	applied  map[string]int64
	own      map[string]int64 // segments de cette session : dernière ligne
	rd       *reader
	pending  map[string][]Line
	maxSeq   map[string]int64
	files    map[string]bool // journaux présents à la dernière liste
	cells    map[cellKey]stamp
	lastBeat time.Time
	lastComp time.Time
	lastLock time.Time
	gaps     map[string]int
	manifest manifestCache
	notices  []string
	refused  map[string]bool // sessions dont l'accueil a échoué (une fois)
	coll     *collab.Engine
}

// New prépare la session d'un classeur ouvert depuis son fichier, seul :
// elle regarde le dossier et passe au travail à plusieurs quand une autre
// session demande à rejoindre (Run).
func New(d *doc.Document, opt Options) (*Engine, error) {
	if d.Path() == "" {
		return nil, msg.New("coedit.neverSaved", nil)
	}
	e := newEngine(d, opt)
	e.sid = newSession(e.me)
	e.presence = Presence{V: 1, Session: e.sid, User: e.me.User, Login: e.me.Login, Machine: e.me.Machine, PID: e.me.PID, Since: time.Now().UTC().Truncate(time.Second), State: StateReady}
	e.roster = newRoster(e.sid, e.t.Stale)
	return e, nil
}

// Start démarre la session d'un classeur ouvert après Join (ticket) : elle
// relève aussitôt ses transactions et rattrape les journaux des autres.
func Start(ctx context.Context, d *doc.Document, tk *Ticket, opt Options) (*Engine, error) {
	e := newEngine(d, opt)
	e.sid = tk.p.Session
	e.presence = tk.p
	e.roster = newRoster(e.sid, e.t.Stale)
	e.run.Lock()
	defer e.run.Unlock()
	applied, err := readApplied(ctx, d.DB().Read())
	if err != nil {
		return nil, err
	}
	if err := e.activate(ctx, applied); err != nil {
		return nil, err
	}
	// Rattrapage : tout ce que les journaux ont au-delà du fichier.
	e.tick(ctx)
	return e, nil
}

func newEngine(d *doc.Document, opt Options) *Engine {
	return &Engine{
		d: d, path: d.Path(), dir: DirOf(d.Path()), opt: opt, t: opt.Timing.withDefaults(), me: archive.Me(),
		kick: make(chan struct{}, 1), done: make(chan struct{}),
		applied: map[string]int64{}, own: map[string]int64{}, rd: newReader(), pending: map[string][]Line{},
		maxSeq: map[string]int64{}, files: map[string]bool{}, cells: map[cellKey]stamp{}, gaps: map[string]int{},
		refused: map[string]bool{},
		coll:    collab.New(d.DB(), d.Model, d.Share),
	}
}

// Session donne l'identifiant de la session.
func (e *Engine) Session() string { return e.sid }

// Run suit le dossier jusqu'à Close (à lancer dans un fil).
func (e *Engine) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancel = cancel
	e.mu.Unlock()
	defer close(e.done)
	for {
		wait := e.t.Watch
		if e.isActive() {
			wait = e.t.Poll
		}
		select {
		case <-ctx.Done():
			return
		case <-e.kick:
		case <-time.After(wait):
		}
		e.run.Lock()
		ev := e.tick(ctx)
		e.run.Unlock()
		if ev != nil && e.opt.OnEvent != nil {
			e.opt.OnEvent(*ev)
		}
	}
}

// Sync fait un tour aussitôt (sans Run : essais) et rend son évènement.
func (e *Engine) Sync(ctx context.Context) Event {
	e.run.Lock()
	ev := e.tick(ctx)
	e.run.Unlock()
	if ev == nil {
		return Event{}
	}
	return *ev
}

// Kick demande un tour sans attendre (modification locale à envoyer).
func (e *Engine) Kick() {
	select {
	case e.kick <- struct{}{}:
	default:
	}
}

func (e *Engine) isActive() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active
}

// State décrit la session pour l'interface.
func (e *Engine) State() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := State{Active: e.active, Peers: []Peer{}, Conflicts: e.conflicts, Offline: e.offline, Stale: e.stale}
	for _, p := range e.peers {
		st.Peers = append(st.Peers, Peer{Session: p.Session, User: p.Name(), Machine: p.Machine, Since: p.Since, Joining: p.State == StateJoining})
	}
	return st
}

// Close arrête la session : la file part une dernière fois, la présence
// est retirée ; la dernière session supprime le dossier. Le classeur
// reste ouvert (il se ferme ensuite).
func (e *Engine) Close() {
	e.mu.Lock()
	cancel := e.cancel
	e.mu.Unlock()
	if cancel != nil {
		cancel()
		<-e.done
	}
	e.run.Lock()
	defer e.run.Unlock()
	if !e.isActive() {
		return
	}
	ctx := context.Background()
	if err := e.drain(ctx); err != nil {
		trace.Logf(1, trace.Doc, "travail à plusieurs : dernier envoi : %v", err)
	}
	e.d.Oplog.SetAll(false)
	e.d.Oplog.SetWatch(nil)
	e.d.SetCoEdit(nil)
	removePresence(e.dir, e.sid)
	entries, err := os.ReadDir(e.dir)
	if err == nil && len(e.roster.scan(e.dir, entries, time.Now())) == 0 {
		// Dernière session : ce qui n'a pas été enregistré est perdu, comme
		// pour un classeur fermé sans l'enregistrer.
		os.RemoveAll(e.dir)
	}
	e.mu.Lock()
	e.active = false
	e.mu.Unlock()
}

// --- Un tour ------------------------------------------------------------------

// tick fait un tour (sous run) ; rend l'évènement pour l'interface.
func (e *Engine) tick(ctx context.Context) *Event {
	var ev Event
	now := time.Now()
	entries, err := os.ReadDir(e.dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		e.setOffline(&ev, e.say("coedit.offline", nil))
		return e.flush(&ev)
	}
	if !e.isActive() {
		// Seule : on regarde seulement si quelqu'un demande à rejoindre.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		live := e.roster.scan(e.dir, entries, now)
		if len(live) == 0 {
			return nil
		}
		if !e.welcomeJoiners(ctx, &ev, live) {
			return e.flush(&ev)
		}
		entries, _ = os.ReadDir(e.dir)
	}
	e.setOffline(&ev, "")
	live := e.roster.scan(e.dir, entries, now)
	e.updatePeers(&ev, live)
	e.welcomeJoiners(ctx, &ev, live)
	if err := e.drain(ctx); err != nil {
		e.setOffline(&ev, e.say("coedit.offlineErr", msg.Args{"err": err}))
	}
	if err := e.receive(ctx, &ev, entries); err != nil {
		e.note(&ev, e.say("coedit.receiveFailed", msg.Args{"err": err}))
	}
	if now.Sub(e.lastBeat) >= e.t.Beat {
		e.beat()
	}
	if now.Sub(e.lastLock) >= 5*e.t.Beat {
		e.lastLock = now
		e.d.TryLock() // la session qui le tenait est peut-être partie
	}
	if now.Sub(e.lastComp) >= e.t.Compact {
		e.lastComp = now
		e.compact(live, entries)
	}
	e.countConflicts(&ev)
	return e.flush(&ev)
}

func (e *Engine) flush(ev *Event) *Event {
	ev.Notices = append(ev.Notices, e.notices...)
	e.notices = nil
	if !ev.Data && !ev.State && len(ev.Notices) == 0 {
		return nil
	}
	return ev
}

func (e *Engine) note(ev *Event, msg string) {
	ev.Notices = append(ev.Notices, msg)
}

func (e *Engine) setOffline(ev *Event, msg string) {
	e.mu.Lock()
	changed := e.offline != msg
	e.offline = msg
	e.mu.Unlock()
	if changed {
		ev.State = true
		if msg != "" {
			ev.Notices = append(ev.Notices, msg)
		}
	}
}

// updatePeers note les sessions vivantes ; arrivées et départs pour la
// barre d'état.
func (e *Engine) updatePeers(ev *Event, live []Presence) {
	e.mu.Lock()
	old := e.peers
	e.peers = live
	e.mu.Unlock()
	e.others.Store(int32(len(live)))
	had := map[string]Presence{}
	for _, p := range old {
		had[p.Session] = p
	}
	now := map[string]bool{}
	for _, p := range live {
		now[p.Session] = true
		if q, ok := had[p.Session]; !ok {
			ev.State = true
			if p.State == StateReady {
				e.note(ev, e.say("coedit.alsoWorking", msg.Args{"who": p.Who()}))
			}
		} else if q.State != p.State {
			ev.State = true
			if p.State == StateReady {
				e.note(ev, e.say("coedit.joined", msg.Args{"who": p.Who()}))
			}
		}
	}
	for _, p := range old {
		if !now[p.Session] {
			ev.State = true
			e.note(ev, e.say("coedit.left", msg.Args{"who": p.Who()}))
		}
	}
}

// welcomeJoiners accueille les sessions qui demandent à rejoindre : les
// modifications hors journal sont d'abord enregistrées, puis la session
// passe au travail à plusieurs. Faux si elle n'a pas pu.
func (e *Engine) welcomeJoiners(ctx context.Context, ev *Event, live []Presence) bool {
	var joiners []Presence
	for _, p := range live {
		if p.State == StateJoining && !slices.Contains(e.welcome, p.Session) && !e.refused[p.Session] {
			joiners = append(joiners, p)
		}
	}
	active := e.isActive()
	if len(joiners) == 0 {
		return active
	}
	dirty, err := e.d.Dirty(ctx)
	if err != nil {
		return active
	}
	if dirty && !active || e.localOnly.Load() {
		save := e.opt.Save
		if save == nil {
			save = func(ctx context.Context) error { return e.d.Save(ctx, "") }
		}
		// Enregistrement depuis le tour (run tenu) : les crochets de
		// l'enregistrement ne le reprennent pas.
		e.inside.Store(true)
		err := save(ctx)
		e.inside.Store(false)
		if err != nil {
			for _, p := range joiners {
				e.refused[p.Session] = true
			}
			e.note(ev, e.say("coedit.joinSaveFailed", msg.Args{"who": joiners[0].Who(), "err": err}))
			return active
		}
		e.localOnly.Store(false)
		e.note(ev, e.say("coedit.joining", msg.Args{"who": joiners[0].Who()}))
	}
	if !active {
		// Premier passage au travail à plusieurs : les journaux restés d'une
		// ancienne séance (sessions plantées) ne concernent pas ce fichier.
		e.clearDir()
		if err := e.activate(ctx, map[string]int64{}); err != nil {
			for _, p := range joiners {
				e.refused[p.Session] = true
			}
			e.note(ev, e.say("coedit.startFailed", msg.Args{"err": err}))
			return false
		}
		ev.State = true
	}
	for _, p := range joiners {
		e.welcome = append(e.welcome, p.Session)
	}
	// Accueils des sessions qui ne demandent plus.
	keep := e.welcome[:0]
	for _, s := range e.welcome {
		for _, p := range live {
			if p.Session == s && p.State == StateJoining {
				keep = append(keep, s)
				break
			}
		}
	}
	e.welcome = keep
	e.beat()
	return true
}

// clearDir retire les journaux et présences mortes d'une séance passée.
func (e *Engine) clearDir() {
	entries, err := os.ReadDir(e.dir)
	if err != nil {
		return
	}
	live := map[string]bool{}
	for _, p := range e.roster.live(time.Now()) {
		live[p.Session] = true
	}
	for _, en := range entries {
		n := en.Name()
		switch {
		case strings.HasSuffix(n, journalExt):
			if !live[sessionOf(strings.TrimSuffix(n, journalExt))] {
				os.Remove(filepath.Join(e.dir, n))
			}
		case strings.HasSuffix(n, presenceExt):
			if !live[strings.TrimSuffix(n, presenceExt)] {
				os.Remove(filepath.Join(e.dir, n))
			}
		}
	}
}

// activate fait passer la session au travail à plusieurs (sous run) :
// identifiants partagés des lignes, relevé de tous les tableaux, journal.
func (e *Engine) activate(ctx context.Context, applied map[string]int64) error {
	if e.d.Share.Info() != nil {
		return msg.New("coedit.sharedOnServer", nil)
	}
	if err := assignIDs(ctx, e.d.DB(), applied); err != nil {
		return err
	}
	e.applied = applied
	e.seg = 1
	e.journal = segmentName(e.sid, e.seg)
	e.w = writer{path: filepath.Join(e.dir, e.journal+journalExt)}
	if err := os.MkdirAll(e.dir, 0o755); err != nil {
		return err
	}
	archive.Hide(e.dir)
	if err := e.w.create(); err != nil {
		return err
	}
	e.d.Oplog.SetAll(true)
	e.d.Oplog.SetWatch(e.watch)
	e.d.SetCoEdit(&doc.CoEditHooks{Before: e.beforeSave, Positions: positions, Known: e.known, After: e.afterSave})
	e.presence.State = StateReady
	e.presence.Journal = e.journal
	e.mu.Lock()
	e.active = true
	e.mu.Unlock()
	e.beat()
	return nil
}

// beat réécrit la présence.
func (e *Engine) beat() {
	e.presence.Beat++
	e.presence.Seq = e.seq
	e.presence.Journal = e.journal
	e.presence.Applied = e.appliedAll()
	e.presence.Welcome = slices.Clone(e.welcome)
	if err := writePresence(e.dir, e.presence); err != nil {
		trace.Logf(2, trace.Doc, "travail à plusieurs : présence : %v", err)
		return
	}
	e.lastBeat = time.Now()
}

// appliedAll : positions appliquées, segments de cette session compris.
func (e *Engine) appliedAll() map[string]int64 {
	out := make(map[string]int64, len(e.applied)+len(e.own)+1)
	for k, v := range e.applied {
		out[k] = v
	}
	for k, v := range e.own {
		out[k] = v
	}
	if e.journal != "" {
		out[e.journal] = e.seq
	}
	return out
}

func (e *Engine) mine(journal string) bool { return sessionOf(journal) == e.sid }

// --- Refus des changements hors données ----------------------------------------

// TogetherError refuse, pendant le travail à plusieurs, une action qui
// change autre chose que les données des tableaux.
type TogetherError struct{ Others []string }

func (e TogetherError) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e TogetherError) Msg() msg.Message {
	return msg.M("coedit.together", msg.Args{"who": strings.Join(e.Others, ", "), "n": len(e.Others)})
}

// lang : la langue de la session du classeur.
func (e *Engine) lang() msg.Lang {
	if e.d == nil || e.d.Model == nil {
		return msg.FR
	}
	return e.d.Language()
}

// say : le message id dans la langue de la session du classeur.
func (e *Engine) say(id msg.ID, args msg.Args) string { return msg.Format(e.lang(), id, args) }

// watch voit chaque instruction des étapes du bus (oplog.SetWatch).
func (e *Engine) watch(q string) error {
	if dataOnly(q) {
		return nil
	}
	if e.others.Load() > 0 {
		e.mu.Lock()
		var names []string
		for _, p := range e.peers {
			names = append(names, p.Name())
		}
		e.mu.Unlock()
		return TogetherError{Others: names}
	}
	// Seule : permis, mais hors journal (enregistré avant qu'un autre
	// poste rejoigne ; barrière pour une session déconnectée).
	e.localOnly.Store(true)
	e.barrier.Store(true)
	return nil
}

// --- Enregistrement ---------------------------------------------------------------

// beforeSave envoie la file avant l'enregistrement : le fichier ne
// contient rien qui ne soit dans un journal.
func (e *Engine) beforeSave(ctx context.Context) error {
	if !e.inside.Load() {
		e.run.Lock()
		defer e.run.Unlock()
	}
	if err := e.drain(ctx); err != nil {
		trace.Logf(1, trace.Doc, "travail à plusieurs : envoi avant l'enregistrement : %v", err)
	}
	return nil
}

// positions lit, dans la file d'écriture, les positions des journaux
// contenues dans la base (archive.Doc.Journal).
func positions(ctx context.Context, db *sql.DB) (map[string]int64, error) {
	m, err := readApplied(ctx, db)
	if err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]int64{}
	}
	return m, nil
}

func readApplied(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}) (map[string]int64, error) {
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, appliedKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]int64{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	if json.Unmarshal([]byte(raw), &out) != nil {
		return map[string]int64{}, nil
	}
	return out, nil
}

func writeApplied(ctx context.Context, tx *sql.Tx, m map[string]int64) error {
	raw, _ := json.Marshal(m)
	_, err := tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, appliedKey, string(raw))
	return err
}

// known : le fichier modifié par ailleurs (un collègue l'a enregistré) ne
// contient que des lignes déjà appliquées ici (archive.Doc.Known).
func (e *Engine) known(m archive.Manifest) bool {
	e.mu.Lock()
	stale := e.stale
	e.mu.Unlock()
	if stale != "" || m.Journal == nil {
		return false
	}
	if !e.inside.Load() {
		e.run.Lock()
		defer e.run.Unlock()
	}
	covered := func() bool {
		have := e.appliedAll()
		for j, pos := range m.Journal {
			if have[j] < pos {
				return false
			}
		}
		return true
	}
	if covered() || e.inside.Load() {
		return covered()
	}
	// Peut-être pas encore lu : un tour de plus.
	if ev := e.tick(context.Background()); ev != nil && e.opt.OnEvent != nil {
		go e.opt.OnEvent(*ev)
	}
	return covered()
}

// afterSave : le fichier contient tout le segment en cours, un nouveau
// segment commence (l'ancien disparaîtra au compactage).
func (e *Engine) afterSave(ctx context.Context, path string, same bool) {
	if !e.inside.Load() {
		e.run.Lock()
		defer e.run.Unlock()
	}
	if !same {
		return
	}
	e.localOnly.Store(false)
	e.manifest = manifestCache{}
	m, err := archive.ReadManifest(path)
	if err != nil || e.seq == 0 || m.Journal[e.journal] < e.seq {
		return
	}
	next := writer{path: filepath.Join(e.dir, segmentName(e.sid, e.seg+1)+journalExt)}
	if err := next.create(); err != nil {
		return
	}
	e.own[e.journal] = e.seq
	e.seg++
	e.journal = segmentName(e.sid, e.seg)
	e.w = next
	e.seq = 0
	e.beat()
}

// --- Conflits ----------------------------------------------------------------------

// Conflicts liste les conflits ouverts (volet Conflits).
func (e *Engine) Conflicts(ctx context.Context) ([]collab.Conflict, error) {
	return e.coll.Conflicts(ctx)
}

// Resolve règle un conflit : mine, « Garder la mienne » (réécrite, elle
// repart dans le journal) ; sinon « Garder la sienne ».
func (e *Engine) Resolve(ctx context.Context, id int64, mine bool) error {
	if !mine {
		// La valeur du collègue est déjà dans la cellule : le conflit est
		// oublié.
		return e.coll.Resolve(ctx, id, false)
	}
	var tbl, col int64
	var row, local, server string
	err := e.d.DB().Read().QueryRowContext(ctx, `SELECT tbl, row_id, col, coalesce(local, 'null'), coalesce(server, 'null') FROM _lattis_conflicts WHERE id = ?`, id).Scan(&tbl, &row, &col, &local, &server)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("conflit %d introuvable (déjà réglé ?)", id)
	}
	if err != nil {
		return err
	}
	mineV, theirs := decodeJSON(local), decodeJSON(server)
	ops, err := command.EncodeOps([]command.Op{{Kind: command.OpSet, Table: tbl, Row: row, Col: col, Old: theirs, New: mineV}})
	if err != nil {
		return err
	}
	// Ma valeur revient dans la cellule (hors du journal d'annulation) et
	// part dans le journal : sa nouvelle horloge l'emporte partout.
	rows := []canvas.RemoteTable{{Table: tbl, Rows: []canvas.RemoteRow{{ID: row, Values: map[int64]any{col: mineV}, NoCreate: true}}}}
	_, err = e.d.ApplyRemote(ctx, rows, nil, func(tx *sql.Tx) error {
		var oid int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO _lattis_outbox (seq, kind, batch, ops) VALUES (NULL, 'do', 0, ?) RETURNING id`, ops).Scan(&oid); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE _lattis_outbox SET batch = id WHERE id = ?`, oid); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_conflicts WHERE id = ?`, id)
		return err
	})
	if err != nil {
		return err
	}
	e.d.MarkChanged()
	e.Kick()
	return nil
}

// decodeJSON relit une valeur de cellule gardée en JSON.
func decodeJSON(s string) any {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil {
		return nil
	}
	return localValue(v)
}

func (e *Engine) countConflicts(ev *Event) {
	var n int
	if e.d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_conflicts`).Scan(&n) != nil {
		return
	}
	e.mu.Lock()
	changed := n != e.conflicts
	e.conflicts = n
	e.mu.Unlock()
	if changed {
		ev.State = true
	}
}

// --- Identifiants partagés -----------------------------------------------------

// assignIDs donne aux lignes qui n'en ont pas un identifiant partagé
// déterministe, n<tableau>.<clé locale> : tous les postes partent du même
// fichier, donc des mêmes clés, et obtiennent les mêmes identifiants sans se
// parler. Hors du journal d'annulation. La file d'envoi est vidée : ce
// qu'elle contenait est dans le fichier.
func assignIDs(ctx context.Context, db interface {
	Write(ctx context.Context, fn func(*sql.Tx) error) error
}, applied map[string]int64) error {
	return db.Write(ctx, func(tx *sql.Tx) error {
		rs, err := tx.QueryContext(ctx, `SELECT t.id FROM _lattis_tables t JOIN sqlite_master m ON m.type = 'table' AND m.name = 't_' || t.id`)
		if err != nil {
			return err
		}
		var ids []int64
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return err
			}
			ids = append(ids, id)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT OR IGNORE INTO _lattis_row_ids (tbl, rid, lattis_id) SELECT %d, id, 'n%d.' || id FROM t_%d AS t WHERE NOT EXISTS (SELECT 1 FROM _lattis_row_ids AS r WHERE r.tbl = %d AND r.rid = t.id)`, id, id, id, id)); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_outbox`); err != nil {
			return err
		}
		return writeApplied(ctx, tx, applied)
	})
}

// --- Envoi ---------------------------------------------------------------------------

// drain envoie la file locale dans le journal (une ligne par transaction),
// la met sur disque, puis marque les entrées comme envoyées.
func (e *Engine) drain(ctx context.Context) error {
	if !e.isActive() {
		return nil
	}
	db := e.d.DB()
	rs, err := db.Read().QueryContext(ctx, `SELECT id, kind, batch, ops FROM _lattis_outbox WHERE state = 'pending' ORDER BY id`)
	if err != nil {
		return err
	}
	type batch struct {
		id   int64
		ids  []int64
		kind string
		ops  []command.Op
	}
	var batches []*batch
	for rs.Next() {
		var id, b int64
		var kind, data string
		if err := rs.Scan(&id, &kind, &b, &data); err != nil {
			rs.Close()
			return err
		}
		ops, err := command.DecodeOps(data)
		if err != nil {
			rs.Close()
			return fmt.Errorf("file d'envoi, entrée %d : %w", id, err)
		}
		if n := len(batches); n == 0 || batches[n-1].id != b {
			batches = append(batches, &batch{id: b, kind: kind})
		}
		cur := batches[len(batches)-1]
		cur.ids = append(cur.ids, id)
		for _, op := range ops {
			if op.Kind != command.OpMeta {
				cur.ops = append(cur.ops, op)
			}
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	barrier := e.barrier.Load()
	if len(batches) == 0 && !barrier {
		return nil
	}
	deps := map[string]int64{}
	for k, v := range e.applied {
		if v > 0 {
			deps[k] = v
		}
	}
	at := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	seq, clock := e.seq, e.clock
	var buf []byte
	var lines []Line
	add := func(kind string, ops []command.Op) error {
		seq++
		clock++
		l := Line{V: LineVersion, Seq: seq, Clock: clock, At: at, By: e.presence.Name(), Base: e.d.BaseSum(), Deps: deps, Kind: kind}
		if len(ops) > 0 {
			data, err := command.EncodeOps(ops)
			if err != nil {
				return err
			}
			l.Ops = json.RawMessage(data)
		}
		b, err := json.Marshal(l)
		if err != nil {
			return err
		}
		buf = append(append(buf, b...), '\n')
		lines = append(lines, l)
		return nil
	}
	if barrier {
		if err := add(kindBarrier, nil); err != nil {
			return err
		}
	}
	var sent []int64
	for _, b := range batches {
		sent = append(sent, b.ids...)
		if len(b.ops) == 0 {
			continue
		}
		if err := add(b.kind, b.ops); err != nil {
			return err
		}
	}
	if len(buf) > 0 {
		if err := e.w.append(buf); err != nil {
			return err
		}
	}
	if barrier {
		e.barrier.Store(false)
	}
	e.seq, e.clock = seq, clock
	for _, l := range lines {
		e.stampLocal(l)
	}
	applied := e.appliedAll()
	err = db.Write(ctx, func(tx *sql.Tx) error {
		if len(sent) > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE _lattis_outbox SET state = 'sent' WHERE id IN (SELECT value FROM json_each(?))`, idList(sent)); err != nil {
				return err
			}
		}
		// Plus utiles à l'annulation : commande sortie du journal.
		if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_outbox WHERE state <> 'pending' AND (seq IS NULL OR seq NOT IN (SELECT seq FROM _lattis_journal))`); err != nil {
			return err
		}
		return writeApplied(ctx, tx, applied)
	})
	return err
}

func idList(ids []int64) string {
	b, _ := json.Marshal(ids)
	return string(b)
}
