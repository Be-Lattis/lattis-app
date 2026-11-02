package bridge

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/trace"
)

// Brouillons de session (édition web, version-web.md § 6.3) : le répertoire
// d'une session disparaît avec son moteur. Ce qui n'est pas sur le serveur y
// est gardé après chaque modification : un envoi à la fois ; les modifications
// arrivées pendant un envoi font exactement un envoi de plus, de l'état le
// plus récent, et les états intermédiaires sont sautés. Jamais au bureau ni
// dans la démo publique (UseDrafts n'y est pas appelé).

// DraftEvent est émis quand l'état d'enregistrement sur le serveur change.
const DraftEvent = "draft:status"

// DraftStore garde les brouillons de l'utilisateur.
type DraftStore = workbook.DraftStore

// États et raisons de DraftStatus.
const (
	draftSaving = "saving"
	draftSaved  = "saved"
	draftFailed = "failed"

	draftTooBig      = "tooBig"
	draftUnreachable = "unreachable"
	draftError       = "error"
)

// Délais (surchargés par les essais).
var (
	// draftDebounce : attente après une modification, pour qu'une rafale fasse
	// un seul envoi.
	draftDebounce = 300 * time.Millisecond
	// draftRetry, draftRetryMax : nouveaux essais après un échec, de plus en
	// plus espacés.
	draftRetry    = 2 * time.Second
	draftRetryMax = 2 * time.Minute
	// draftGrace : un classeur collaboratif en ligne dont la file part n'a pas
	// besoin de brouillon : à ne garder que si elle reste en attente.
	draftGrace = 5 * time.Second
	// draftTick : relecture de la file pendant draftGrace.
	draftTick = time.Second
	// draftFlush : attente du dernier envoi à la fermeture du classeur ou à
	// l'arrêt du moteur, avant de l'abandonner (le brouillon précédent reste).
	draftFlush = 20 * time.Second
	// draftUpload : durée au plus d'un envoi.
	draftUpload = 10 * time.Minute
	// draftMax : au-delà, inutile d'essayer : le protocole de PostgreSQL ne
	// passe pas plus d'1 Go par paramètre.
	draftMax = 1000 << 20
	// Chaque enregistrement réécrit l'archive entière dans le WAL du serveur et
	// vers ses réplicas : presque immédiat sous draftFreeSize octets, puis une
	// pause après l'envoi, proportionnelle à la taille (draftPerMB par Mo), au
	// plus draftSpaceMax. Le dernier envoi (onglet fermé, liaison coupée, arrêt)
	// n'attend pas.
	draftFreeSize = 1 << 20
	draftPerMB    = 2 * time.Second
	draftSpaceMax = 60 * time.Second
)

// draftSpacing : pause après l'envoi d'une archive de size octets.
func draftSpacing(size int) time.Duration {
	if size < draftFreeSize {
		return 0
	}
	return min(time.Duration(float64(draftPerMB)*float64(size)/(1<<20)), draftSpaceMax)
}

// DraftStatus : état d'enregistrement sur le serveur du classeur ouvert.
type DraftStatus struct {
	// State : vide (rien à enregistrer), saving, saved ou failed.
	State string `json:"state"`
	// Reason, avec failed : tooBig, unreachable ou error.
	Reason string `json:"reason,omitempty"`
	// Active : cette session garde ses brouillons sur le serveur.
	Active bool `json:"active,omitempty"`
}

// draftState : brouillons de la session (nil : aucun).
type draftState struct {
	mu       sync.Mutex
	store    DraftStore
	filesDir string
	run      *draftRun
	status   DraftStatus
	// adopt : brouillon qu'on vient de restaurer, repris par le classeur
	// suivant (attachDraft).
	adopt string
	// offered : les brouillons ont été proposés une fois (OfferDrafts).
	offered bool
	// missing : serveur sans brouillons (d'avant la migration 0051).
	missing atomic.Bool
}

// UseDrafts branche les brouillons (moteur d'une session web). filesDir :
// dossier des fichiers de la session, où un classeur restauré retrouve son
// fichier.
func UseDrafts(s *DocumentService, store DraftStore, filesDir string) {
	s.drafts.mu.Lock()
	defer s.drafts.mu.Unlock()
	s.drafts.store, s.drafts.filesDir = store, filesDir
}

// ServerDrafts donne les brouillons du Serveur Lattis de la session.
func ServerDrafts(server string) DraftStore {
	return workbook.PGDrafts(func(ctx context.Context, usage string) (*pgx.Conn, func(), error) {
		dial, err := sessionDialer(ctx, server)
		if err != nil {
			return nil, nil, err
		}
		return dial(ctx, usage)
	})
}

// DraftStatus donne l'état d'enregistrement sur le serveur.
func (s *DocumentService) DraftStatus() DraftStatus {
	s.drafts.mu.Lock()
	defer s.drafts.mu.Unlock()
	st := s.drafts.status
	st.Active = s.drafts.store != nil && !s.drafts.missing.Load() && !publicDemo.Load()
	return st
}

func (s *DocumentService) setDraftStatus(st DraftStatus) {
	s.drafts.mu.Lock()
	same := s.drafts.status == st
	s.drafts.status = st
	s.drafts.mu.Unlock()
	if !same {
		s.sys().Emit(DraftEvent, s.DraftStatus())
	}
}

// draftID : identifiant du brouillon d'un classeur, celui du classeur écrit en
// uuid ; un identifiant d'une autre forme en donne un, stable.
func draftID(id string) string {
	if b, err := hex.DecodeString(id); err == nil && len(b) == 16 {
		if u, err := uuid.FromBytes(b); err == nil {
			return u.String()
		}
	}
	if u, err := uuid.Parse(id); err == nil {
		return u.String()
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(id)).String()
}

// draftMeta : ce que le brouillon garde du fichier du classeur.
type draftMeta struct {
	// Name : nom du fichier dans la session (vide : jamais enregistré).
	Name string `json:"name,omitempty"`
	// Cache : le fichier est le cache d'un classeur du serveur.
	Cache bool `json:"cache,omitempty"`
}

// draftRun enregistre un classeur ouvert.
type draftRun struct {
	s     *DocumentService
	store DraftStore
	d     *doc.Document
	id    string
	// kicks compte les modifications ; sent : valeur au dernier envoi réussi
	// (fil de loop seulement).
	kicks atomic.Int64
	sent  int64
	wake  chan struct{}
	stop  chan struct{}
	hurry chan struct{} // plus de pause : voir FlushDraft
	done  chan struct{}
	ctx   context.Context
	abort context.CancelFunc
	// has : le serveur garde un brouillon de ce classeur écrit par cette session.
	has atomic.Bool
	// checked : la liste du serveur a été lue avant le premier envoi.
	checked bool
	// scrubbed : plus de place pour l'historique d'annulation.
	scrubbed bool
	fails    int
	// heldSince : depuis quand la file d'un classeur collaboratif attend.
	heldSince time.Time
	// fetched : kicks au dernier téléchargement par le navigateur (-1 : aucun).
	fetched atomic.Int64
	// sum : empreinte du dernier contenu envoyé (le même n'est pas renvoyé) ;
	// notBefore : fin de la pause après cet envoi (draftSpacing).
	sum       string
	notBefore time.Time
}

func (r *draftRun) kick() {
	r.kicks.Add(1)
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// attachDraft commence à enregistrer d.
func (s *DocumentService) attachDraft(d *doc.Document) {
	s.drafts.mu.Lock()
	store := s.drafts.store
	adopt := s.drafts.adopt
	s.drafts.adopt = ""
	s.drafts.mu.Unlock()
	if store == nil || d == nil || publicDemo.Load() || s.drafts.missing.Load() {
		s.setDraftStatus(DraftStatus{})
		return
	}
	ctx, abort := context.WithCancel(context.Background())
	r := &draftRun{s: s, store: store, d: d, id: draftID(d.ID()), wake: make(chan struct{}, 1), hurry: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}), ctx: ctx, abort: abort}
	r.fetched.Store(-1)
	status := DraftStatus{}
	if adopt != "" {
		r.id, r.checked = adopt, true
		r.has.Store(true)
		status.State = draftSaved
	}
	s.drafts.mu.Lock()
	s.drafts.run = r
	s.drafts.mu.Unlock()
	s.setDraftStatus(status)
	d.DB().OnWrite(r.kick)
	go r.loop()
	if adopt == "" {
		r.kick() // classeur déjà modifié (récupéré, réparé)
	}
}

// stopDraft arrête l'enregistrement du classeur ouvert ; flush : les dernières
// modifications partent d'abord. Le classeur doit rester ouvert jusqu'au retour.
func (s *DocumentService) stopDraft(flush bool) *draftRun {
	s.drafts.mu.Lock()
	r := s.drafts.run
	s.drafts.run = nil
	s.drafts.mu.Unlock()
	if r == nil {
		return nil
	}
	r.d.DB().OnWrite(nil)
	if !flush {
		r.abort()
	}
	close(r.stop)
	t := time.NewTimer(draftFlush)
	defer t.Stop()
	select {
	case <-r.done:
	case <-t.C:
		trace.Logf(1, trace.Doc, "brouillon : dernier envoi abandonné après %s", draftFlush)
		r.abort()
		<-r.done
	}
	r.abort()
	return r
}

// leaveDraft : le classeur ouvert est quitté (fermé, remplacé, arrêt du
// moteur). Déjà téléchargé tel quel : son brouillon ne sert plus. Sinon les
// dernières modifications partent, et il reste proposé à la prochaine session.
func (s *DocumentService) leaveDraft() {
	s.drafts.mu.Lock()
	r := s.drafts.run
	s.drafts.mu.Unlock()
	if r == nil {
		return
	}
	if r.fetched.Load() == r.kicks.Load() {
		s.DraftForget()
		return
	}
	s.stopDraft(true)
}

// DraftForget retire le brouillon du classeur ouvert (« Ne pas enregistrer ») ;
// plus rien n'est enregistré ensuite.
func (s *DocumentService) DraftForget() {
	r := s.stopDraft(false)
	s.setDraftStatus(DraftStatus{})
	if r == nil || !r.has.Load() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.store.Delete(ctx, r.id); err != nil {
		// Il reste proposé à la prochaine session : on y renonce d'un clic.
		trace.Logf(1, trace.Doc, "brouillon %s non retiré : %v", r.id, err)
	}
}

// kickDraft : le classeur a changé autrement que par une écriture (son fichier).
func (s *DocumentService) kickDraft() {
	s.drafts.mu.Lock()
	r := s.drafts.run
	s.drafts.mu.Unlock()
	if r != nil {
		r.kick()
	}
}

// draftFetched : le navigateur vient de télécharger le classeur tel qu'il est.
func (s *DocumentService) draftFetched() {
	s.drafts.mu.Lock()
	r := s.drafts.run
	s.drafts.mu.Unlock()
	if r != nil {
		r.fetched.Store(r.kicks.Load())
	}
}

// FlushDraft : plus d'onglet connecté (fermé, liaison coupée) : ce qui attend
// part sans attendre la fin de la pause. Ne bloque pas.
func FlushDraft(s *DocumentService) {
	s.drafts.mu.Lock()
	r := s.drafts.run
	s.drafts.mu.Unlock()
	if r == nil {
		return
	}
	select {
	case r.hurry <- struct{}{}:
	default:
	}
}

// last : dernier envoi, sans pause.
func (r *draftRun) last() {
	r.notBefore = time.Time{}
	r.step()
}

// loop : un envoi à la fois ; ce qui arrive pendant se résume à kicks.
func (r *draftRun) loop() {
	defer close(r.done)
	defer recoverPanic("brouillon")
	for {
		select {
		case <-r.wake:
		case <-r.stop:
			r.last()
			return
		}
		// Le point « enregistré » tombe dès la modification, pas à l'envoi.
		if st := r.s.DraftStatus(); st.State == draftSaved && r.kicks.Load() != r.sent {
			r.s.setDraftStatus(DraftStatus{State: draftSaving})
		}
		if !r.pause(draftDebounce) {
			r.last()
			return
		}
		for {
			wait := r.step()
			if wait == 0 {
				break
			}
			if !r.pause(wait) {
				r.last()
				return
			}
		}
		if r.kicks.Load() != r.sent {
			select {
			case r.wake <- struct{}{}:
			default:
			}
		}
	}
}

// pause attend d ; faux si l'arrêt est demandé.
func (r *draftRun) pause(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-r.hurry:
		r.notBefore = time.Time{}
		return true
	case <-r.stop:
		return false
	}
}

type draftNeed int

const (
	draftSkip draftNeed = iota // rien à garder
	draftDrop                  // tout est sur le serveur : brouillon inutile
	draftHold                  // à revoir dans un instant
	draftSend
)

// need dit ce qu'il faut faire du classeur.
func (r *draftRun) need() draftNeed {
	d := r.d
	if d.ReadOnly() {
		return draftSkip
	}
	queued := false
	if info := d.Share.Info(); info != nil && info.Mode == workbook.Collaborative && d.Share.Gone() != workbook.GoneDeleted {
		db := d.DB().Read()
		pending, conflicts, err := workbook.Unsent(r.ctx, db)
		private, perr := workbook.PrivateTables(r.ctx, db)
		if err == nil && perr == nil {
			if pending+conflicts == 0 && private == 0 && !d.Share.LayoutUnsent() {
				r.heldSince = time.Time{}
				return draftDrop
			}
			queued = pending+conflicts > 0
			if private == 0 && conflicts == 0 && d.Share.Online() {
				if r.heldSince.IsZero() {
					r.heldSince = time.Now()
				}
				if time.Since(r.heldSince) < draftGrace {
					return draftHold
				}
			}
		}
	}
	r.heldSince = time.Time{}
	if dirty, err := d.Dirty(r.ctx); dirty || err != nil || queued || r.has.Load() {
		return draftSend
	}
	return draftSkip
}

// step fait un envoi si des modifications attendent. Rend le temps à attendre
// avant de réessayer (0 : rien à refaire pour l'instant).
func (r *draftRun) step() time.Duration {
	g := r.kicks.Load()
	if r.ctx.Err() != nil {
		return 0
	}
	if g == r.sent {
		// Réveil sans rien de neuf : rien ne change, sauf un état resté en attente.
		if st := r.s.DraftStatus(); st.State == draftSaving && r.has.Load() {
			r.s.setDraftStatus(DraftStatus{State: draftSaved})
		}
		return 0
	}
	switch r.need() {
	case draftSkip:
		r.sent = g
		if !r.has.Load() {
			r.s.setDraftStatus(DraftStatus{})
		}
		return 0
	case draftHold:
		return draftTick
	case draftDrop:
		return r.drop(g)
	}
	if st := r.s.DraftStatus(); st.State != draftFailed {
		r.s.setDraftStatus(DraftStatus{State: draftSaving})
	}
	if wait := time.Until(r.notBefore); wait > 0 {
		return wait
	}
	if err := r.upload(); err != nil {
		return r.failed(err)
	}
	r.sent, r.fails = g, 0
	r.has.Store(true)
	// Une modification déjà arrivée : encore « Enregistrement… », l'envoi
	// suivant vient juste après.
	if r.kicks.Load() == g {
		r.s.setDraftStatus(DraftStatus{State: draftSaved})
	} else {
		r.s.setDraftStatus(DraftStatus{State: draftSaving})
	}
	return 0
}

// drop retire le brouillon d'un classeur dont tout est sur le serveur.
func (r *draftRun) drop(g int64) time.Duration {
	if r.has.Load() {
		if err := r.store.Delete(r.ctx, r.id); err != nil {
			return r.failed(err)
		}
		r.has.Store(false)
	}
	r.sent, r.fails = g, 0
	r.s.setDraftStatus(DraftStatus{})
	return 0
}

// errDraftHuge : brouillon plus gros que le serveur ne peut en recevoir.
var errDraftHuge = errors.New("brouillon trop gros")

// snapshotErr : échec local (disque plein…), pas du serveur.
type snapshotErr struct{ error }

func (e snapshotErr) Unwrap() error { return e.error }

// upload envoie l'état courant du classeur. Refusé faute de place, un second
// essai sans l'historique d'annulation ni les corbeilles.
func (r *draftRun) upload() error {
	ctx, cancel := context.WithTimeout(r.ctx, draftUpload)
	defer cancel()
	if !r.checked {
		// Un brouillon de ce classeur écrit par une autre session (proposé, pas
		// repris ; fichier plus ancien renvoyé par le navigateur) n'est jamais
		// écrasé : celui-ci en fait un autre.
		list, err := r.store.List(ctx)
		if err != nil {
			return err
		}
		for _, o := range list {
			if o.ID == r.id {
				r.id = uuid.NewString()
				break
			}
		}
		r.checked = true
	}
	err := r.send(ctx)
	if workbook.DraftOverflow(err) && !r.scrubbed {
		r.scrubbed = true
		err = r.send(ctx)
	}
	return err
}

func (r *draftRun) send(ctx context.Context) error {
	meta, title := r.describe()
	data, sum, err := r.d.Draft(ctx, r.scrubbed, meta)
	if err != nil {
		return snapshotErr{err}
	}
	// Même contenu que le dernier envoi (Ctrl+S sans changement, écriture sans
	// effet) : rien à réécrire dans le WAL du serveur.
	if sum == r.sum && r.has.Load() {
		return nil
	}
	if len(data) > draftMax {
		return errDraftHuge
	}
	if _, err = r.store.Save(ctx, r.id, title, data); err != nil {
		return err
	}
	r.sum = sum
	r.notBefore = time.Now().Add(draftSpacing(len(data)))
	return nil
}

// describe : fichier et titre du classeur.
func (r *draftRun) describe() (meta []byte, title string) {
	m := draftMeta{}
	if p := r.d.Path(); p != "" {
		m.Name, m.Cache = filepath.Base(p), inShareCache(p)
		title = m.Name
	}
	if r.d.Share != nil && r.d.Share.Info() != nil {
		if t := r.d.Share.State().Title; strings.TrimSpace(t) != "" {
			title = t
		}
	}
	meta, _ = json.Marshal(m)
	return meta, title
}

// failed note un échec et rend l'attente avant le prochain essai.
func (r *draftRun) failed(err error) time.Duration {
	if r.ctx.Err() != nil {
		return 0
	}
	if workbook.DraftsMissing(err) {
		// Serveur d'avant les brouillons : rien à enregistrer, rien à dire ; les
		// avertissements restent ceux d'une session sans brouillon.
		trace.Logf(1, trace.Doc, "brouillons : le serveur ne les connaît pas")
		r.s.drafts.missing.Store(true)
		r.sent = r.kicks.Load()
		r.d.DB().OnWrite(nil)
		r.s.setDraftStatus(DraftStatus{})
		r.s.sys().Emit(DraftEvent, r.s.DraftStatus())
		return 0
	}
	reason := draftUnreachable
	var snap snapshotErr
	switch {
	case workbook.DraftOverflow(err), errors.Is(err, errDraftHuge):
		reason = draftTooBig
	case errors.As(err, &snap):
		reason = draftError
	}
	r.fails++
	trace.Logf(1, trace.Doc, "brouillon non enregistré (%s, essai %d) : %v", reason, r.fails, err)
	r.s.setDraftStatus(DraftStatus{State: draftFailed, Reason: reason})
	wait := draftRetry
	for range r.fails - 1 {
		if wait *= 2; wait >= draftRetryMax {
			return draftRetryMax
		}
	}
	return wait
}

// --- Restauration -------------------------------------------------------------

// draftPrefix désigne un brouillon parmi les sessions à récupérer (Orphan.Dir).
const draftPrefix = "draft:"

func draftOf(dir string) (string, bool) { return strings.CutPrefix(dir, draftPrefix) }

// draftOrphans : brouillons du serveur, en sessions à récupérer. Sauf celui du
// classeur ouvert. Serveur injoignable : aucun, ils restent à proposer plus tard.
func (s *DocumentService) draftOrphans() []archive.Orphan {
	s.drafts.mu.Lock()
	store, run := s.drafts.store, s.drafts.run
	s.drafts.offered = true
	s.drafts.mu.Unlock()
	if store == nil || s.drafts.missing.Load() || publicDemo.Load() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	list, err := store.List(ctx)
	if err != nil {
		if workbook.DraftsMissing(err) {
			s.drafts.missing.Store(true)
			s.sys().Emit(DraftEvent, s.DraftStatus())
		} else {
			trace.Logf(1, trace.Doc, "brouillons non lus : %v", err)
		}
		return nil
	}
	var out []archive.Orphan
	for _, o := range list {
		if run != nil && o.ID == run.id {
			continue
		}
		// Regroupé avec une session locale du même classeur (identifiant sans
		// tirets, voir draftID).
		out = append(out, archive.Orphan{Dir: draftPrefix + o.ID, Title: o.Title, Modified: o.UpdatedAt, Workbook: strings.ReplaceAll(o.ID, "-", "")})
	}
	return out
}

// OfferDrafts rend, une seule fois par moteur, les brouillons à proposer alors
// qu'un classeur est déjà ouvert (moteur lancé sur un classeur du serveur,
// --workbook) : ils ne passent pas par l'écran du démarrage.
func (s *DocumentService) OfferDrafts() []archive.Orphan {
	s.drafts.mu.Lock()
	done := s.drafts.offered
	s.drafts.mu.Unlock()
	if done {
		return nil
	}
	return s.draftOrphans()
}

// recoverDraft rouvre un brouillon comme une session interrompue : tableaux,
// mise en page, état de publication, file d'envoi et historique d'annulation.
func (s *DocumentService) recoverDraft(ctx context.Context, id string) (DocState, error) {
	s.drafts.mu.Lock()
	store, files := s.drafts.store, s.drafts.filesDir
	s.drafts.mu.Unlock()
	if store == nil {
		return DocState{}, msg.New("session.notFound", nil)
	}
	data, err := store.Read(ctx, id)
	if errors.Is(err, workbook.ErrDraftGone) {
		return DocState{}, msg.New("session.notFound", nil)
	}
	if err != nil {
		return DocState{}, msg.Wrap(err, "doc.recoverFailed", msg.Args{"err": err})
	}
	man, raw, err := archive.ReadDraft(data)
	if err != nil {
		return DocState{}, msg.Wrap(err, "doc.recoverFailed", msg.Args{"err": err})
	}
	var meta draftMeta
	json.Unmarshal(raw, &meta)
	source := ""
	switch name := filepath.Base(meta.Name); {
	case meta.Cache && man.Remote != nil:
		if source, err = shareCachePath(man.Remote.Server, man.Remote.WorkbookID); err != nil {
			return DocState{}, err
		}
	case meta.Name != "" && name != "." && name != string(filepath.Separator) && name != ".." && files != "":
		source = filepath.Join(files, name)
	}
	if source != "" {
		if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
			return DocState{}, err
		}
	}
	o, err := archive.RestoreDraft(data, source)
	if err != nil {
		return DocState{}, msg.Wrap(err, "doc.recoverFailed", msg.Args{"err": err})
	}
	d, err := doc.Recover(ctx, o)
	if err != nil {
		return DocState{}, err
	}
	s.drafts.mu.Lock()
	s.drafts.adopt = id
	s.drafts.mu.Unlock()
	s.replace(d)
	st, err := s.State(ctx)
	st.Notice = d.TakeOpenNoticeIn(s.sessionLang())
	return st, err
}

// discardDraft retire un brouillon proposé à la récupération.
func (s *DocumentService) discardDraft(id string) error {
	s.drafts.mu.Lock()
	store := s.drafts.store
	s.drafts.mu.Unlock()
	if store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return store.Delete(ctx, id)
}
