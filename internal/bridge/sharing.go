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

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/pgpool"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Classeurs partagés (docs/collaboration/publication.md) : Fichier ›
// Partager sur un serveur…, Ouvrir depuis un serveur, Republier ; état du
// classeur partagé ouvert (bandeaux « Nouvelle version disponible »,
// « Hors ligne, lecture seule »). Tout passe par internal/remote/workbook ;
// l'interface est prévenue par Host.Emit.

// ShareEvent est émis quand l'état de partage du classeur ouvert change
// (connexion, nouvelle version publiée) : l'interface relit ShareState et
// la mise en page (tableaux en lecture seule ou non).
const ShareEvent = "share:changed"

// shareRetry : délai entre deux essais de connexion au serveur d'un
// classeur partagé hors ligne.
var shareRetry = 30 * time.Second

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
	m, err := lattisOn()
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
	if s.share.cancel != nil {
		s.share.cancel()
		s.share.cancel = nil
	}
	s.share.sync, s.share.doc = nil, nil
	info := d.Share.Info()
	if info == nil {
		s.share.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.share.cancel = cancel
	if info.Mode == workbook.Collaborative {
		// Classeur collaboratif : envoi de la file, réception des changements
		// des autres, conflits (docs/collaboration/synchronisation.md).
		eng := collab.New(d.DB(), d.Model, d.Share)
		s.share.sync, s.share.doc = eng, d
		s.share.mu.Unlock()
		go s.followCollab(ctx, d, eng, info)
		return
	}
	s.share.mu.Unlock()
	go func() {
		for ctx.Err() == nil {
			srv, _, _, err := remoteServer(ctx, info.Server)
			if err != nil {
				d.Share.SetOnline(false, err.Error())
				s.emitShare(d)
				select {
				case <-ctx.Done():
					return
				case <-time.After(shareRetry):
				}
				continue
			}
			workbook.Watch(ctx, srv, d.Share, shareRetry, func() { s.emitShare(d) })
		}
	}()
}

// followCollab suit un classeur collaboratif tant que ctx court : à
// chaque (re)connexion, la file part et les changements des autres sont
// lus ; sur notification, ceux du tableau notifié.
func (s *DocumentService) followCollab(ctx context.Context, d *doc.Document, eng *collab.Engine, info *workbook.Info) {
	for ctx.Err() == nil {
		srv, _, _, err := remoteServer(ctx, info.Server)
		if err != nil {
			d.Share.SetOnline(false, err.Error())
			s.emitShare(d)
			select {
			case <-ctx.Done():
				return
			case <-time.After(shareRetry):
			}
			continue
		}
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
			}
		})
	}
}

// stopShare arrête le suivi du serveur (fermeture du classeur).
func (s *DocumentService) stopShare() {
	s.share.mu.Lock()
	defer s.share.mu.Unlock()
	if s.share.cancel != nil {
		s.share.cancel()
		s.share.cancel = nil
	}
	s.share.sync, s.share.doc = nil, nil
}

// kickShare demande l'envoi de la file du classeur collaboratif ouvert
// (après une modification locale).
func (s *DocumentService) kickShare() {
	if eng := s.collab(); eng != nil {
		eng.Kick()
	}
}

// syncMarks ajoute à une fenêtre de la grille les cellules en attente du
// serveur ou en conflit (classeur collaboratif).
func (s *DocumentService) syncMarks(ctx context.Context, t *grid.Table, table int64, w *grid.Window) error {
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
		return []collab.Conflict{}, nil
	}
	return eng.Conflicts(ctx)
}

// ShareResolve règle un conflit : mine, « Garder la mienne » (renvoyée,
// conditionnée à la valeur du serveur) ; sinon « Garder la sienne ».
func (s *DocumentService) ShareResolve(ctx context.Context, id int64, mine bool) (DocState, error) {
	eng := s.collab()
	if eng == nil {
		return DocState{}, collab.ErrNotCollaborative
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
	return workbook.Inspect(ctx, d.DB())
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
	return workbook.Inspect(ctx, d.DB())
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
// Partager sur un serveur…). Un classeur déjà enregistré est réenregistré
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

// ShareRepublish publie une nouvelle version du classeur figé ouvert
// (Fichier › Republier) ; tables nil : les tableaux déjà publiés.
func (s *DocumentService) ShareRepublish(ctx context.Context, tables []int64) (workbook.State, error) {
	d, err := s.current()
	if err != nil {
		return workbook.State{}, err
	}
	info := d.Share.Info()
	if info == nil {
		return workbook.State{}, errors.New("ce classeur n'est pas publié : Fichier › Partager sur un serveur…")
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

func samePath(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	return strings.EqualFold(a, b)
}

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
			return "", fmt.Errorf("identifiant de classeur invalide : %q", id)
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
			return OpenResult{}, err
		}
		_, err := s.afterShare(ctx, cur)
		st, serr := s.State(ctx)
		return OpenResult{State: st}, errors.Join(err, serr)
	}
	var d *doc.Document
	fresh := true
	if _, err := os.Stat(path); err == nil {
		if d, err = doc.Open(ctx, path); err == nil {
			fresh = false
		}
	}
	if d == nil {
		if d, err = doc.New(ctx); err != nil {
			return OpenResult{}, err
		}
	}
	// Cache d'un classeur collaboratif : gardé tel quel (sa file d'envoi
	// aussi), mis à jour par les changements depuis son point de reprise à
	// la connexion ; rechargé en entier seulement s'il n'a rien en attente
	// et date d'avant la purge des pierres tombales du serveur.
	if fresh || !keepCollabCache(ctx, d) {
		if _, err := workbook.Load(ctx, srv, d.DB(), src, fresh); err != nil {
			d.Close()
			return OpenResult{}, err
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
	s.replace(d)
	addRecent(path)
	if _, err := s.shareReady(ctx, d); err != nil {
		return OpenResult{}, err
	}
	st, err := s.State(ctx)
	return OpenResult{State: st}, err
}

// collabCacheAge : au-delà, un cache collaboratif sans rien en attente est
// rechargé en entier (le serveur garde les pierres tombales 30 jours).
const collabCacheAge = 25 * 24 * time.Hour

// keepCollabCache dit si le cache local d'un classeur collaboratif se
// rouvre tel quel (synchronisé par ses deltas) plutôt que d'être rechargé.
func keepCollabCache(ctx context.Context, d *doc.Document) bool {
	info := d.Share.Info()
	if info == nil || info.Mode != workbook.Collaborative {
		return false
	}
	var pending int
	if err := d.DB().Read().QueryRowContext(ctx, `SELECT count(*) FROM _lattis_outbox WHERE state = 'pending'`).Scan(&pending); err != nil {
		return false
	}
	return pending > 0 || time.Since(info.Synced) < collabCacheAge
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
		return DocState{}, errors.New("ce classeur n'est pas partagé")
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
