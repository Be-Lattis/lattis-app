package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/coedit"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/pgpool"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/trace"
)

// Classeurs partagés (docs/collaboration/publication.md) : Fichier ›
// Publier sur un serveur…, Ouvrir depuis un serveur, Republier ; état du
// classeur partagé ouvert (bandeaux « Nouvelle version disponible »,
// « Hors ligne, lecture seule »). Tout passe par internal/remote/workbook ;
// l'interface est prévenue par Host.Emit.

// ShareEvent est émis quand l'état de partage du classeur ouvert change
// (connexion, nouvelle version publiée) : l'interface relit ShareState et
// la mise en page (tableaux en lecture seule ou non).
const ShareEvent = "share:changed"

// SyncEvent est émis quand la file d'envoi d'un classeur collaboratif
// change ou part vers le serveur (workbook.SyncStatus) : la barre d'état
// dit « Synchronisation… », sans relire la mise en page. Les modifications
// sont faites et affichées en local sans attendre le serveur (écriture
// optimiste, décision 0012).
const SyncEvent = "share:sync"

// shareRetry : délai du premier nouvel essai de connexion au serveur d'un
// classeur partagé hors ligne ; les suivants attendent deux fois plus à
// chaque échec, jusqu'à shareRetryMax fois ce délai (5 s → 2 min).
var shareRetry = 5 * time.Second

// shareRetryMax : plafond des délais entre essais, en multiple de
// shareRetry.
const shareRetryMax = 24

// shareServer donne le serveur d'un classeur partagé (pour les essais : un
// faux serveur ; nil : le vrai, par la session de l'utilisateur).
var shareServer func(ctx context.Context, server string) (workbook.Server, auth.User, string, error)

// shareCacheDir est le dossier des classeurs ouverts depuis un serveur
// (cache local, consultable hors ligne).
var shareCacheDir = func() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Lattis", "partages"), nil
}

// sharePools garde les connexions PostgreSQL de chaque serveur (une
// session : un utilisateur).
var sharePools struct {
	mu sync.Mutex
	m  map[string]*pgpool.Pools
}

func poolsKey(server, role string) string { return server + "\x00" + role }

// closeSharePools ferme les connexions d'un serveur (déconnexion).
func closeSharePools(server string) {
	sharePools.mu.Lock()
	defer sharePools.mu.Unlock()
	for k, p := range sharePools.m {
		if strings.HasPrefix(k, server+"\x00") {
			p.Close()
			delete(sharePools.m, k)
		}
	}
}

// remoteServer ouvre l'API SQL d'un Serveur Lattis sous l'identité de
// l'utilisateur ; *auth.ErrLoginRequired sans session.
func remoteServer(ctx context.Context, raw string) (workbook.Server, auth.User, string, error) {
	if shareServer != nil {
		return shareServer(ctx, raw)
	}
	m, err := sessions()
	if err != nil {
		return nil, auth.User{}, "", err
	}
	server, err := auth.NormalizeURL(raw)
	if err != nil {
		return nil, auth.User{}, "", err
	}
	user, disc, err := m.Session(ctx, server)
	if err != nil {
		return nil, auth.User{}, "", err
	}
	pools := func() (*pgpool.Pools, error) {
		sharePools.mu.Lock()
		defer sharePools.mu.Unlock()
		k := poolsKey(server, user.PGRole)
		if p := sharePools.m[k]; p != nil {
			return p, nil
		}
		p, err := pgpool.Open(ctx, lattisTarget(m, server, user, disc))
		if err != nil {
			return nil, err
		}
		if sharePools.m == nil {
			sharePools.m = map[string]*pgpool.Pools{}
		}
		sharePools.m[k] = p
		return p, nil
	}
	dial := func(ctx context.Context, usage string) (*pgx.Conn, func(), error) {
		p, err := pools()
		if err != nil {
			return nil, nil, err
		}
		c, err := p.Acquire(ctx, usage)
		// Jeton refusé par PostgreSQL : rafraîchi, puis un seul nouvel essai
		// (contrat de lattisd, §3).
		if pgpool.AuthFailed(err) {
			m.Invalidate(server)
			c, err = p.Acquire(ctx, usage)
		}
		if err != nil {
			return nil, nil, err
		}
		return c.Conn(), c.Release, nil
	}
	return workbook.PG(dial), user, disc.Name, nil
}

// shareState suit le classeur partagé ouvert.
type shareState struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	// sync : synchronisation du classeur collaboratif ouvert doc (nil
	// sinon).
	sync *collab.Engine
	doc  *doc.Document
	// retry : nouveaux essais de connexion du classeur suivi (délais
	// croissants ; ShareRetry les fait partir tout de suite).
	retry *workbook.Backoff
}

// collab rend la synchronisation du classeur collaboratif ouvert (nil :
// classeur local ou figé).
func (s *DocumentService) collab() *collab.Engine {
	cur, err := s.current()
	s.share.mu.Lock()
	defer s.share.mu.Unlock()
	if err != nil || cur != s.share.doc {
		return nil
	}
	return s.share.sync
}

// watchShare suit le serveur du classeur d (écoute des republications,
// connexion perdue et retrouvée) ; arrête le suivi du classeur précédent.
func (s *DocumentService) watchShare(d *doc.Document) {
	s.share.mu.Lock()
	// Classeur précédent (ouverture d'un autre) : sa mise en page en
	// attente part d'abord, comme à la fermeture.
	s.flushLayoutLocked()
	if s.share.cancel != nil {
		s.share.cancel()
		s.share.cancel = nil
	}
	unbindCollab(s.share.doc)
	s.share.sync, s.share.doc, s.share.retry = nil, nil, nil
	// Classeur supprimé du serveur : plus rien à suivre (State.Gone).
	info := d.Share.Info()
	if info == nil || d.Share.Gone() == workbook.GoneDeleted {
		s.share.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.share.cancel = cancel
	retry := workbook.NewBackoff(shareRetry, shareRetryMax*shareRetry)
	s.share.retry = retry
	go func() {
		defer recoverPanic("classeur partagé")
		recordOpens(ctx, info)
	}()
	if info.Mode == workbook.Collaborative {
		// Classeur collaboratif : envoi de la file, réception des changements
		// des autres, conflits (docs/collaboration/synchronisation.md).
		eng := collab.New(d.DB(), d.Model, d.Share)
		eng.SetBackoff(retry)
		// File gardée dans le classeur (modifications faites hors ligne) :
		// comptée dès l'ouverture, pour la barre d'état.
		eng.Counts(ctx)
		s.share.sync, s.share.doc = eng, d
		// Structure des tableaux partagés (§6.4) : faite sur le serveur avant
		// la validation locale ; index du serveur (API).
		d.Oplog.SetStructure(eng.Structure)
		d.SetIndexer(eng)
		s.share.mu.Unlock()
		go func() {
			defer recoverPanic("classeur partagé")
			s.followCollab(ctx, d, eng, info, retry)
		}()
		return
	}
	s.share.mu.Unlock()
	go func() {
		defer recoverPanic("classeur partagé")
		for ctx.Err() == nil {
			srv, _, _, err := remoteServer(ctx, info.Server)
			if err != nil {
				s.shareDown(d, info, err)
				if !retry.Wait(ctx) {
					return
				}
				continue
			}
			workbook.WatchWith(ctx, srv, d.Share, retry, func() { s.emitShare(d) })
			if d.Share.Gone() == workbook.GoneDeleted {
				s.shareGone(d, info, nil)
				return
			}
		}
	}()
}

// shareDown note que le serveur du classeur d ne répond pas (session ou
// annuaire injoignables) : l'interface n'est prévenue, et le journal ne
// reçoit une ligne, que si la raison change ; les essais suivants, de plus
// en plus espacés, restent silencieux.
func (s *DocumentService) shareDown(d *doc.Document, info *workbook.Info, err error) {
	reason := err.Error()
	if !d.Share.Online() && d.Share.State().Offline == reason {
		return
	}
	trace.Logf(1, trace.Doc, "serveur %s injoignable (classeur partagé « %s ») : %s", info.Server, info.Title, reason)
	d.Share.SetOnline(false, reason)
	s.emitShare(d)
}

// shareGone : le classeur d a été supprimé du serveur (workbook.GoneDeleted) ;
// le suivi s'est arrêté, sans nouvel essai. La structure de ses tableaux
// ne passe plus par la synchronisation (eng) : tout se fait en local, rien
// ne part plus (Link.Gate).
func (s *DocumentService) shareGone(d *doc.Document, info *workbook.Info, eng *collab.Engine) {
	trace.Logf(1, trace.Doc, "classeur partagé « %s » supprimé de %s : suivi arrêté", info.Title, info.Server)
	s.share.mu.Lock()
	if eng != nil && s.share.sync == eng {
		unbindCollab(d)
		s.share.sync = nil
	}
	s.share.mu.Unlock()
	s.emitShare(d)
}

// ShareRetry essaie tout de suite de joindre le serveur du classeur partagé
// ouvert (bandeau « Réessayer », Fichier › Réessayer la connexion) ; la
// réponse arrive par l'évènement share:changed.
func (s *DocumentService) ShareRetry() {
	s.share.mu.Lock()
	r := s.share.retry
	s.share.mu.Unlock()
	if r != nil {
		r.Wake()
	}
}

// followCollab suit un classeur collaboratif tant que ctx court : à
// chaque (re)connexion, la file part et les changements des autres sont
// lus ; sur notification, ceux du tableau notifié.
func (s *DocumentService) followCollab(ctx context.Context, d *doc.Document, eng *collab.Engine, info *workbook.Info, retry *workbook.Backoff) {
	for ctx.Err() == nil {
		srv, _, _, err := remoteServer(ctx, info.Server)
		if err != nil {
			s.shareDown(d, info, err)
			if !retry.Wait(ctx) {
				return
			}
			continue
		}
		// Changements des autres : par le relais de lattisd (une écoute de
		// PostgreSQL pour tous les postes), sinon par LISTEN.
		srv = withRelay(ctx, srv, info, func(users []workbook.Present) {
			if d.Share.SetPresence(othersPresent(users, info.User)) {
				s.emitShare(d)
			}
		})
		eng.Follow(ctx, srv, shareRetry, func(ev collab.Event) {
			if cur, err := s.current(); err != nil || cur != d {
				return
			}
			if len(ev.Notices) > 0 {
				d.Notify(strings.Join(ev.Notices, " ; "))
			}
			if ev.Data || len(ev.Notices) > 0 {
				s.changed(context.Background())
			}
			if ev.State {
				s.emitShare(d)
			} else if ev.Sync {
				s.emitSync(d)
			}
		})
		if d.Share.Gone() == workbook.GoneDeleted {
			s.shareGone(d, info, eng)
			return
		}
	}
}

// withRelay fait suivre les changements du classeur par le relais de
// lattisd (GET <émetteur>/api/events, workbook.EventsServer) ; repli sur
// LISTEN si le serveur n'en a pas. LATTIS_LISTEN=1 garde LISTEN
// (diagnostic). Les essais (shareServer) gardent leur serveur.
func withRelay(ctx context.Context, srv workbook.Server, info *workbook.Info, presence func([]workbook.Present)) workbook.Server {
	if shareServer != nil || os.Getenv("LATTIS_LISTEN") == "1" {
		return srv
	}
	m, err := sessions()
	if err != nil {
		return srv
	}
	server, err := auth.NormalizeURL(info.Server)
	if err != nil {
		return srv
	}
	_, disc, err := m.Session(ctx, server)
	if err != nil {
		return srv
	}
	base := server
	if disc != nil && disc.Issuer != "" {
		base = disc.Issuer
	}
	es := workbook.WithEvents(srv, strings.TrimSuffix(base, "/")+"/api/events", info.WorkbookID,
		func(ctx context.Context) (string, error) { return m.Token(ctx, server) },
		func() { m.Invalidate(server) })
	es.OnPresence = presence
	return es
}

// othersPresent : noms des membres présents, sans l'utilisateur lui-même
// (son rôle PostgreSQL, souvent son adresse, ou son adresse).
func othersPresent(users []workbook.Present, me string) []string {
	var out []string
	for _, u := range users {
		if strings.EqualFold(u.Email, me) || strings.EqualFold(u.Name, me) {
			continue
		}
		name := u.Name
		if name == "" {
			name = u.Email
		}
		out = append(out, name)
	}
	return out
}

// stopShare arrête le suivi du serveur (fermeture du classeur).
func (s *DocumentService) stopShare() {
	s.share.mu.Lock()
	defer s.share.mu.Unlock()
	s.flushLayoutLocked()
	if s.share.cancel != nil {
		s.share.cancel()
		s.share.cancel = nil
	}
	unbindCollab(s.share.doc)
	s.share.sync, s.share.doc, s.share.retry = nil, nil, nil
}

// flushLayoutLocked envoie ce qui attend encore du classeur suivi, avant
// d'en arrêter le suivi : la file (envoi différé de quelques centaines de
// millisecondes) et la mise en page (LayoutDelay). Hors ligne, rien ne part :
// la mise en page est notée en attente et le cache est gardé
// (keepCollabQueue). s.share.mu tenu.
func (s *DocumentService) flushLayoutLocked() {
	if eng := s.share.sync; eng != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		eng.Send(ctx)
		eng.FlushLayout(ctx)
		cancel()
	}
}

// unbindCollab débranche la synchronisation d'un classeur qui n'est plus
// suivi : sa structure partagée redevient refusée en local.
func unbindCollab(d *doc.Document) {
	if d != nil {
		d.Oplog.SetStructure(nil)
		d.SetIndexer(nil)
	}
}

// kickShare demande l'envoi de la file du classeur collaboratif ouvert
// (après une modification locale).
func (s *DocumentService) kickShare() {
	eng := s.collab()
	if eng == nil {
		return
	}
	eng.Kick()
	// Hors ligne, le suivi ne tourne pas : la file est comptée ici pour la
	// barre d'état (« Hors ligne — 3 en attente »).
	if d, err := s.current(); err == nil && !d.Share.Online() {
		if _, _, st, sy, err := eng.Counts(context.Background()); err == nil {
			if st {
				s.emitShare(d)
			} else if sy {
				s.emitSync(d)
			}
		}
	}
}

// syncMarks ajoute à une fenêtre de la grille les cellules en attente du
// serveur ou en conflit (classeur collaboratif).
func (s *DocumentService) syncMarks(ctx context.Context, t *grid.Table, table int64, w *grid.Window) error {
	if err := s.lockedRows(ctx, table, w); err != nil {
		return err
	}
	eng := s.collab()
	if eng == nil || len(w.IDs) == 0 {
		return nil
	}
	marks, err := eng.Marks(ctx, table, w.IDs)
	if err != nil || len(marks) == 0 {
		return err
	}
	for i, rid := range w.IDs {
		for col, kind := range marks[rid] {
			switch {
			case col < 0:
				w.Sync = append(w.Sync, grid.SyncMark{Row: i, Col: -1, Kind: kind})
			case col > 0:
				if c, ok := t.ColIndex(col); ok {
					w.Sync = append(w.Sync, grid.SyncMark{Row: i, Col: c, Kind: kind})
				}
			}
		}
	}
	return nil
}

// ShareConflicts liste les conflits du classeur collaboratif ouvert
// (volet Conflits).
func (s *DocumentService) ShareConflicts(ctx context.Context) ([]collab.Conflict, error) {
	eng := s.collab()
	if eng == nil {
		if cs, ok, err := s.coeditConflicts(ctx); ok {
			return cs, err
		}
		return []collab.Conflict{}, nil
	}
	return eng.Conflicts(ctx)
}

// ShareResolve règle un conflit : mine, « Garder la mienne » (renvoyée,
// conditionnée à la valeur du serveur) ; sinon « Garder la sienne ».
func (s *DocumentService) ShareResolve(ctx context.Context, id int64, mine bool) (DocState, error) {
	eng := s.collab()
	if eng == nil {
		ce := s.coeditEngine()
		if ce == nil {
			return DocState{}, collab.ErrNotCollaborative
		}
		if err := ce.Resolve(ctx, id, mine); err != nil {
			return DocState{}, err
		}
		if d, err := s.current(); err == nil {
			s.coeditEvent(d, coedit.Event{State: true})
		}
		return s.changed(ctx)
	}
	if err := eng.Resolve(ctx, id, mine); err != nil {
		return DocState{}, err
	}
	if d, err := s.current(); err == nil {
		eng.Counts(ctx)
		s.emitShare(d)
	}
	return s.changed(ctx)
}

// ShareHeld liste les tableaux dont des modifications ne partent pas vers
// le serveur (mises de côté après des échecs répétés ; volet Conflits).
func (s *DocumentService) ShareHeld(ctx context.Context) ([]collab.Held, error) {
	eng := s.collab()
	if eng == nil {
		return []collab.Held{}, nil
	}
	h, err := eng.HeldList(ctx)
	if h == nil {
		h = []collab.Held{}
	}
	return h, err
}

// ShareHeldResolve règle les modifications mises de côté du tableau tbl :
// retry, les renvoyer ; sinon les abandonner (le tableau reprend l'état du
// serveur).
func (s *DocumentService) ShareHeldResolve(ctx context.Context, tbl int64, retry bool) (DocState, error) {
	eng := s.collab()
	if eng == nil {
		return DocState{}, collab.ErrNotCollaborative
	}
	var notices []string
	if retry {
		if err := eng.RetryHeld(ctx, tbl); err != nil {
			return DocState{}, err
		}
		s.kickShare()
	} else {
		rep, err := eng.DropHeld(ctx, tbl)
		if err != nil {
			return DocState{}, err
		}
		notices = rep.Notices
	}
	if d, err := s.current(); err == nil {
		eng.Counts(ctx)
		s.emitShare(d)
	}
	st, err := s.changed(ctx)
	if len(notices) > 0 && st.Notice == "" {
		st.Notice = notices[len(notices)-1]
	}
	return st, err
}

// ShareCanAddTable dit pourquoi le tableau table ne peut pas être ajouté au
// classeur collaboratif ouvert ("" : il peut l'être ; voir ShareAddTable).
func (s *DocumentService) ShareCanAddTable(ctx context.Context, table string) (string, error) {
	eng := s.collab()
	if eng == nil {
		return collab.ErrNotCollaborative.Error(), nil
	}
	d, err := s.current()
	if err != nil {
		return "", err
	}
	tbl, err := tableIDByName(ctx, d, table)
	if err != nil {
		return "", err
	}
	return eng.CanAdd(tbl), nil
}

// ShareAddTable ajoute le tableau local table au classeur collaboratif
// ouvert (« Ajouter au classeur partagé ») : créé sur le serveur avec ses
// lignes, reçu par les autres membres, synchronisé ensuite comme les
// autres. Propriétaires et concepteurs, en ligne. Ne s'annule pas par
// Ctrl+Z (« Supprimer le tableau » le retire du serveur).
func (s *DocumentService) ShareAddTable(ctx context.Context, table string) (DocState, error) {
	eng := s.collab()
	if eng == nil {
		return DocState{}, collab.ErrNotCollaborative
	}
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	tbl, err := tableIDByName(ctx, d, table)
	if err != nil {
		return DocState{}, err
	}
	name, err := eng.AddTable(ctx, tbl)
	if err != nil {
		return DocState{}, err
	}
	d.Notify(collab.AddedNotice(s.sessionLang(), name))
	s.emitShare(d)
	return s.changed(ctx)
}

// tableIDByName donne l'identifiant local d'un tableau.
func tableIDByName(ctx context.Context, d *doc.Document, table string) (int64, error) {
	var tbl int64
	if err := d.DB().Read().QueryRowContext(ctx, `SELECT id FROM _lattis_tables WHERE name = ?`, table).Scan(&tbl); err != nil {
		return 0, msg.New("table.notFound", msg.Args{"table": table})
	}
	return tbl, nil
}

// ShareEditing note la cellule de feuille en cours de saisie (on faux :
// saisie terminée ou abandonnée) : une modification reçue du serveur pour
// elle attend la validation, qui peut produire un conflit (§6.3).
func (s *DocumentService) ShareEditing(ctx context.Context, sheet int64, row, col int, on bool) error {
	eng := s.collab()
	if eng == nil {
		return nil
	}
	if !on {
		eng.SetEditing(0, 0, 0)
		return nil
	}
	d, err := s.current()
	if err != nil {
		return err
	}
	tbl, rid, c, ok, err := d.TableCell(ctx, sheet, row, col)
	if err != nil || !ok {
		eng.SetEditing(0, 0, 0)
		return err
	}
	eng.SetEditing(tbl, rid, c)
	return nil
}

// emitShare prévient l'interface d'un changement d'état de partage du
// classeur d s'il est toujours ouvert.
func (s *DocumentService) emitShare(d *doc.Document) {
	if cur, err := s.current(); err != nil || cur != d {
		return
	}
	s.mu.Lock()
	s.tables = map[int64]*grid.Table{} // lecture seule ou non : relire
	s.mu.Unlock()
	s.sys().Emit(ShareEvent, d.Share.State())
}

// emitSync prévient l'interface d'un changement de la file d'envoi du
// classeur d s'il est toujours ouvert.
func (s *DocumentService) emitSync(d *doc.Document) {
	if cur, err := s.current(); err != nil || cur != d {
		return
	}
	s.sys().Emit(SyncEvent, d.Share.Sync())
}

// ShareState décrit le partage du classeur ouvert (classeur local :
// Shared faux).
func (s *DocumentService) ShareState() workbook.State {
	d, err := s.current()
	if err != nil {
		return workbook.State{Roles: []string{}}
	}
	return d.Share.State()
}

// ShareCheck fait les vérifications avant publication sur tous les
// tableaux du classeur (noms, types figés, saisies invalides).
func (s *DocumentService) ShareCheck(ctx context.Context) (workbook.Check, error) {
	d, err := s.current()
	if err != nil {
		return workbook.Check{}, err
	}
	return workbook.InspectIn(ctx, d.DB(), s.sessionLang())
}

// ShareAcceptText convertit en texte la colonne i d'un tableau en y
// gardant ses saisies invalides telles quelles (« Accepter comme texte »),
// puis refait les vérifications.
func (s *DocumentService) ShareAcceptText(ctx context.Context, table string, i int) (workbook.Check, error) {
	d, err := s.current()
	if err != nil {
		return workbook.Check{}, err
	}
	if err := d.AcceptAsText(ctx, table, i); err != nil {
		return workbook.Check{}, err
	}
	if _, err := s.changed(ctx); err != nil {
		return workbook.Check{}, err
	}
	return workbook.InspectIn(ctx, d.DB(), s.sessionLang())
}

// ShareRequest est une demande de publication.
type ShareRequest struct {
	Server string `json:"server"`
	Title  string `json:"title"`
	// Mode : frozen (figé) ou collaborative.
	Mode string `json:"mode"`
	// Tables : tableaux publiés ; les autres restent privés.
	Tables []int64 `json:"tables"`
}

// SharePublish publie le classeur ouvert sur un Serveur Lattis (Fichier ›
// Publier sur un serveur…). Un classeur déjà enregistré est réenregistré
// (son manifeste dit le partage).
func (s *DocumentService) SharePublish(ctx context.Context, req ShareRequest) (workbook.State, error) {
	d, err := s.current()
	if err != nil {
		return workbook.State{}, err
	}
	srv, user, name, err := remoteServer(ctx, req.Server)
	if err != nil {
		return workbook.State{}, err
	}
	server, _ := auth.NormalizeURL(req.Server)
	if _, err := workbook.Publish(ctx, srv, d.DB(), workbook.Request{
		Server: server, ServerName: name, User: user.PGRole, Title: req.Title, Mode: req.Mode, Tables: req.Tables,
	}); err != nil {
		return workbook.State{}, err
	}
	return s.afterShare(ctx, d)
}

// ShareCanPublish dit si le compte connecté à server peut y créer un
// classeur (rôle « créateur de classeurs », propriétaire du serveur, ou
// publication ouverte à tous) : sinon, Publier sur un serveur ne propose
// pas de publier.
func (s *DocumentService) ShareCanPublish(ctx context.Context, server string) (bool, error) {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return false, err
	}
	return srv.CanCreateWorkbooks(ctx)
}

// ShareRepublish publie une nouvelle version du classeur figé ouvert
// (Fichier › Republier) ; tables nil : les tableaux déjà publiés.
func (s *DocumentService) ShareRepublish(ctx context.Context, tables []int64) (workbook.State, error) {
	d, err := s.current()
	if err != nil {
		return workbook.State{}, err
	}
	info := d.Share.Info()
	if info == nil {
		return workbook.State{}, msg.New("share.notPublished", nil)
	}
	srv, _, _, err := remoteServer(ctx, info.Server)
	if err != nil {
		return workbook.State{}, err
	}
	if _, err := workbook.Republish(ctx, srv, d.DB(), tables); err != nil {
		return workbook.State{}, err
	}
	return s.afterShare(ctx, d)
}

// afterShare : le partage du classeur d a changé (publication,
// rechargement) : état relu, classeur réenregistré s'il a un fichier,
// interface prévenue, suivi du serveur (re)lancé.
func (s *DocumentService) afterShare(ctx context.Context, d *doc.Document) (workbook.State, error) {
	if err := d.ShareChanged(ctx); err != nil {
		return workbook.State{}, err
	}
	if d.Path() != "" {
		if err := saveDoc(ctx, d, ""); err != nil {
			return d.Share.State(), err
		}
	}
	return s.shareReady(ctx, d)
}

// shareReady : classeur partagé à jour du serveur (en ligne), interface
// prévenue, suivi du serveur (re)lancé.
func (s *DocumentService) shareReady(ctx context.Context, d *doc.Document) (workbook.State, error) {
	d.Share.SetOnline(true, "")
	s.mu.Lock()
	s.tables, s.memo = map[int64]*grid.Table{}, nil
	s.mu.Unlock()
	if _, err := s.changed(ctx); err != nil {
		return d.Share.State(), err
	}
	s.watchShare(d)
	s.sys().Emit(ShareEvent, d.Share.State())
	return d.Share.State(), nil
}

// ShareWorkbooks liste les classeurs accessibles sur un serveur (Fichier ›
// Ouvrir depuis un serveur) ; brouillons écartés.
func (s *DocumentService) ShareWorkbooks(ctx context.Context, server string) ([]workbook.Listing, error) {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return nil, err
	}
	list, err := srv.Workbooks(ctx)
	if err != nil {
		return nil, err
	}
	out := []workbook.Listing{}
	for _, l := range list {
		if l.State != "draft" {
			out = append(out, l)
		}
	}
	return out, nil
}

// samePath : même fichier (archive.SamePath : liens résolus, casse ignorée
// seulement là où le système l'ignore).
func samePath(a, b string) bool { return archive.SamePath(a, b) }

// shareCachePath donne le fichier local (cache) d'un classeur du serveur.
func shareCachePath(server, id string) (string, error) {
	dir, err := shareCacheDir()
	if err != nil {
		return "", err
	}
	host := strings.Map(func(r rune) rune {
		if r == ':' || r == '/' || r == '\\' || r == '[' || r == ']' {
			return '_'
		}
		return r
	}, auth.Host(server))
	for _, r := range id {
		if !(r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return "", msg.New("share.invalidWorkbookId", msg.Args{"id": fmt.Sprintf("%q", id)})
		}
	}
	return filepath.Join(dir, host, id+".lattis"), nil
}

// ShareOpen ouvre un classeur du serveur : chargé en entier dans son cache
// local (un fichier .lattis du dossier cache de l'utilisateur, rouvert et
// actualisé s'il existe), qui reste consultable hors ligne.
func (s *DocumentService) ShareOpen(ctx context.Context, server, id, title string) (OpenResult, error) {
	srv, user, name, err := remoteServer(ctx, server)
	if err != nil {
		return OpenResult{}, err
	}
	server, _ = auth.NormalizeURL(server)
	path, err := shareCachePath(server, id)
	if err != nil {
		return OpenResult{}, err
	}
	src := workbook.Source{Server: server, ServerName: name, User: user.PGRole, WorkbookID: id, Title: title}
	// Déjà ouvert : rechargé sur place.
	if cur, err := s.current(); err == nil && cur.Path() != "" && samePath(cur.Path(), path) {
		if keepCollabCache(ctx, cur) {
			st, err := s.State(ctx) // collaboratif : déjà suivi, à jour par ses deltas
			return OpenResult{State: st}, err
		}
		if _, err := workbook.Load(ctx, srv, cur.DB(), src, false); err != nil {
			gone := workbook.GoneReason(err)
			if gone == "" {
				return OpenResult{}, err
			}
			if cur.Share.SetGone(gone) {
				s.emitShare(cur)
			}
			st, err := s.State(ctx)
			return OpenResult{State: st}, err
		}
		_, err := s.afterShare(ctx, cur)
		st, serr := s.State(ctx)
		return OpenResult{State: st}, errors.Join(err, serr)
	}
	var d *doc.Document
	fresh := true
	aside := ""
	if _, err := os.Stat(path); err == nil {
		d, err = doc.Open(ctx, path)
		var rec *archive.RecoverableError
		var bad *archive.OpenError
		switch {
		case err == nil:
			fresh = false
		case errors.As(err, &rec):
			// Session interrompue (plantage) sur ce cache : elle garde les
			// modifications pas encore envoyées ; proposée en récupération
			// plutôt que rechargée du serveur par-dessus.
			return OpenResult{Recoverable: &rec.Orphan}, nil
		case errors.As(err, &bad) && bad.Salvage:
			// Cache abîmé : mis de côté (sa file pourra être récupérée par
			// Fichier › Ouvrir), le classeur est rechargé du serveur.
			aside = strings.TrimSuffix(path, filepath.Ext(path)) + " (abîmé " + time.Now().Format("2006-01-02 15h04") + ")" + filepath.Ext(path)
			if rerr := os.Rename(path, aside); rerr != nil {
				return OpenResult{}, err
			}
		default:
			// Verrouillé, illisible : le cache n'est jamais écrasé.
			return OpenResult{}, err
		}
	}
	if d == nil {
		if d, err = doc.New(ctx); err != nil {
			return OpenResult{}, err
		}
	}
	// Cache d'un classeur collaboratif : gardé tel quel (sa file d'envoi
	// aussi), mis à jour par les changements depuis son point de reprise à
	// la connexion (un tableau dont le point de reprise n'est plus couvert
	// par les pierres tombales du serveur est relu en entier).
	gone := ""
	if fresh || !keepCollabCache(ctx, d) {
		if _, err := workbook.Load(ctx, srv, d.DB(), src, fresh); err != nil {
			// Classeur supprimé du serveur, ou plus d'accès : son cache s'ouvre
			// tel quel, avec le bandeau qui le dit (pas « ne répond pas ») ;
			// sans cache, une erreur qui le dit.
			if gone = workbook.GoneReason(err); gone == "" || fresh {
				d.Close()
				if gone != "" {
					return OpenResult{}, goneError(gone, name, server)
				}
				return OpenResult{}, err
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		d.Close()
		return OpenResult{}, err
	}
	if err := d.ShareChanged(ctx); err != nil {
		d.Close()
		return OpenResult{}, err
	}
	if err := saveDoc(ctx, d, path); err != nil {
		d.Close()
		return OpenResult{}, err
	}
	// Avant replace : un classeur supprimé n'est plus suivi (watchShare).
	d.Share.SetGone(gone)
	s.replace(d)
	addRecentDoc(d)
	if _, err := s.shareReady(ctx, d); err != nil {
		return OpenResult{}, err
	}
	st, err := s.State(ctx)
	st.Notice = d.TakeOpenNoticeIn(s.sessionLang())
	if aside != "" {
		st.Notice = s.say("share.cacheDamaged", msg.Args{"title": st.Title, "path": aside})
	}
	return OpenResult{State: st}, err
}

// goneError : le classeur demandé n'est plus sur le serveur (supprimé) ou
// plus pour l'utilisateur, et n'a pas de cache à ouvrir. Mêmes mots que le
// bandeau (frontend/src/lib/remote/share.ts, goneText).
func goneError(gone, name, server string) error {
	where := strings.TrimSpace(name)
	if where == "" {
		where = auth.Host(server)
	}
	if gone == workbook.GoneRevoked {
		return msg.New("share.accessLost", msg.Args{"server": where})
	}
	return msg.New("share.deleted", msg.Args{"server": where})
}

// keepCollabCache dit si le cache local d'un classeur collaboratif se
// rouvre tel quel (synchronisé par ses deltas) plutôt que d'être rechargé.
// Toujours, file d'envoi comprise : un point de reprise trop ancien (des
// pierres tombales purgées depuis) fait recharger ses tableaux à la
// réception (lattis.since_covered, collab.Engine.Receive).
func keepCollabCache(ctx context.Context, d *doc.Document) bool {
	info := d.Share.Info()
	return info != nil && info.Mode == workbook.Collaborative
}

// ShareReload recharge la version publiée du classeur partagé ouvert
// (bandeau « Nouvelle version disponible ») : les ajouts locaux (tableaux
// privés, cellules libres, feuilles) sont gardés.
func (s *DocumentService) ShareReload(ctx context.Context) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	info := d.Share.Info()
	if info == nil {
		return DocState{}, msg.New("share.notSharedPlain", nil)
	}
	srv, user, name, err := remoteServer(ctx, info.Server)
	if err != nil {
		return DocState{}, err
	}
	src := workbook.Source{Server: info.Server, ServerName: name, User: user.PGRole, WorkbookID: info.WorkbookID}
	if _, err := workbook.Load(ctx, srv, d.DB(), src, false); err != nil {
		return DocState{}, err
	}
	if _, err := s.afterShare(ctx, d); err != nil {
		return DocState{}, err
	}
	return s.State(ctx)
}

// recordOpenEvery : un classeur du serveur resté ouvert est noté ouvert au
// plus une fois par heure (« dernier lecteur », contrat de lattisd §5).
var recordOpenEvery = time.Hour

// recordOpens note l'ouverture du classeur partagé sur son serveur, dès
// qu'il est joignable, puis toutes les recordOpenEvery tant que ctx court.
func recordOpens(ctx context.Context, info *workbook.Info) {
	retry := workbook.NewBackoff(shareRetry, shareRetryMax*shareRetry)
	for {
		wait := retry.Next()
		if srv, _, _, err := remoteServer(ctx, info.Server); err == nil {
			if err := srv.RecordOpen(ctx, info.WorkbookID); err == nil {
				wait = recordOpenEvery
				retry.Reset()
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// SetShareCacheDir range les copies locales des classeurs du serveur dans
// dir (moteur d'une session web : son répertoire de session).
func SetShareCacheDir(dir string) {
	shareCacheDir = func() (string, error) { return dir, nil }
}
