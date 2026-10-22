package bridge

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/trace"
)

// Sortir d'un classeur partagé (bandeau « Hors ligne », Fichier) :
//
//   - « Travailler sur une copie locale » (ShareLocalCopy) : le classeur
//     est enregistré sous un autre nom comme classeur local ordinaire, sans
//     lien avec le serveur, puis ouvert ; le classeur partagé (son cache, ou
//     le fichier publié) reste tel quel ;
//   - « Détacher du serveur » (ShareDetach, propriétaire) : ce classeur
//     devient un classeur local ordinaire ; un classeur ouvert depuis le
//     serveur vit dans le cache (pas un endroit de l'utilisateur) : il est
//     enregistré ailleurs (Enregistrer sous) et son cache est retiré.
//
// Dans les deux cas, le classeur du serveur n'est pas touché, et les
// modifications pas encore envoyées sont gardées dans le classeur local
// (elles ne partent pas). Voir workbook.Detach.

// errNotShared : l'action ne vaut que pour un classeur partagé.
var errNotShared = msg.New("share.notShared", nil)

// ShareLocalCopy enregistre une copie locale du classeur partagé ouvert,
// détachée du serveur (boîte Enregistrer sous), puis l'ouvre.
func (s *DocumentService) ShareLocalCopy(ctx context.Context) (OpenResult, error) {
	d, err := s.current()
	if err != nil {
		return OpenResult{}, err
	}
	if d.Share.Info() == nil {
		return OpenResult{}, errNotShared
	}
	if st := d.Share.State(); st.Gone == workbook.GoneDeleted {
		// Supprimé du serveur (bandeau « Enregistrer une copie locale… ») :
		// le cache n'a plus de raison d'être, ni l'entrée des récents ; les
		// modifications jamais envoyées restent dans la copie.
		r, err := s.detachAs(ctx, d, "Enregistrer une copie locale", docBaseName(d)+".lattis", true, nil)
		if err == nil && !r.Cancelled {
			dropRecent(RecentFile{Server: st.Server, WorkbookID: st.WorkbookID})
		}
		return r, err
	}
	return s.detachAs(ctx, d, "Travailler sur une copie locale", docBaseName(d)+" (copie locale).lattis", false, nil)
}

// ShareDetach fait du classeur partagé ouvert un classeur local ordinaire
// (propriétaire ; l'interface a demandé confirmation). Un classeur ouvert
// depuis le serveur est d'abord enregistré ailleurs (boîte Enregistrer
// sous).
func (s *DocumentService) ShareDetach(ctx context.Context) (OpenResult, error) {
	d, err := s.current()
	if err != nil {
		return OpenResult{}, err
	}
	info := d.Share.Info()
	if info == nil {
		return OpenResult{}, errNotShared
	}
	if !info.CanManage() {
		return OpenResult{}, msg.New("share.detachOwnerOnly", nil)
	}
	if d.ReadOnly() {
		return OpenResult{}, msg.New("share.detachReadOnly", nil)
	}
	if inShareCache(d.Path()) || d.Path() == "" {
		return s.detachAs(ctx, d, "Détacher du serveur : enregistrer le classeur", docBaseName(d)+".lattis", true, nil)
	}
	unsent, err := s.detachTo(ctx, d, "", false, nil)
	if err != nil {
		return OpenResult{}, err
	}
	return s.reopenDetached(ctx, d, detachedNotice(s.sessionLang(), false, unsent, filepath.Base(d.Path())))
}

// ShareRemove retire du serveur le classeur partagé ouvert
// (lattis.drop_workbook, propriétaires ; l'interface a demandé
// confirmation) : données, rôles, pipelines et formulaires disparaissent
// du serveur pour tous ; les partages au Data Lake sont coupés. Les copies
// des postes restent : les membres voient « supprimé du serveur » et
// peuvent garder la leur (shareGone). Celle du propriétaire devient un
// classeur local ordinaire, comme après « Détacher du serveur » ; un
// classeur ouvert depuis le serveur est d'abord enregistré ailleurs
// (boîte Enregistrer sous ; édition web : fichier de la session,
// téléchargé). Annuler la boîte ne retire rien.
func (s *DocumentService) ShareRemove(ctx context.Context) (OpenResult, error) {
	d, err := s.current()
	if err != nil {
		return OpenResult{}, err
	}
	info := d.Share.Info()
	if info == nil {
		return OpenResult{}, errNotShared
	}
	if !info.CanManage() {
		return OpenResult{}, msg.New("share.removeOwnerOnly", nil)
	}
	if d.ReadOnly() {
		return OpenResult{}, msg.New("share.removeReadOnly", nil)
	}
	srv, _, _, err := remoteServer(ctx, info.Server)
	if err != nil {
		return OpenResult{}, err
	}
	// Retiré du serveur une fois le suivi arrêté, juste avant le
	// détachement local : un refus ne change rien au classeur ouvert.
	drop := func() error {
		if err := srv.RemoveWorkbook(ctx, info.WorkbookID); err != nil {
			if workbook.Privilege(err) {
				return msg.New("share.removeOwnerOnly", nil)
			}
			return errors.New(rightsError(err))
		}
		trace.Logf(1, trace.Doc, "classeur partagé « %s » retiré de %s", info.Title, info.Server)
		return nil
	}
	var r OpenResult
	if inShareCache(d.Path()) || d.Path() == "" {
		r, err = s.detachAs(ctx, d, "Retirer du serveur : enregistrer le classeur", docBaseName(d)+".lattis", true, drop)
		if err != nil || r.Cancelled {
			return r, err
		}
	} else {
		unsent, err := s.detachTo(ctx, d, "", false, drop)
		if err != nil {
			return OpenResult{}, err
		}
		if r, err = s.reopenDetached(ctx, d, ""); err != nil {
			return OpenResult{}, err
		}
		r.State.Notice = removedNotice(s.sessionLang(), unsent, filepath.Base(d.Path()), r.State.Notice)
	}
	dropRecent(RecentFile{Server: info.Server, WorkbookID: info.WorkbookID})
	return r, nil
}

// removedNotice : message de la barre d'état après « Retirer du serveur »
// (rest : ce que la réouverture avait à dire).
func removedNotice(l msg.Lang, unsent int, name, rest string) string {
	text := msg.Format(l, "share.removed", msg.Args{"name": name})
	if unsent > 0 {
		text += msg.Format(l, "share.unsentKept", msg.Args{"n": unsent})
	}
	if rest = strings.TrimSpace(rest); rest != "" {
		text += " " + rest
	}
	return text
}

// detachAs demande où enregistrer le classeur détaché (name : nom proposé),
// l'y enregistre puis l'ouvre ; dropCache retire ensuite le cache du
// classeur du serveur (Détacher) ; drop (non nil) le retire d'abord du
// serveur (Retirer du serveur, voir detachTo).
func (s *DocumentService) detachAs(ctx context.Context, d *doc.Document, title, name string, dropCache bool, drop func() error) (OpenResult, error) {
	var unsent int
	var werr error
	p, err := s.sys().SaveFile(FileDialog{
		Title:     title,
		Filename:  name,
		Directory: docFolder(d),
		Filters:   []FileFilter{{"Classeur Lattis", "*.lattis"}},
		Ext:       ".lattis",
	}, func(p string) error {
		if inShareCache(p) {
			werr = msg.New("share.detachChooseFolder", nil)
			return werr
		}
		unsent, werr = s.detachTo(ctx, d, p, dropCache, drop)
		return werr
	})
	if werr != nil {
		return OpenResult{}, werr
	}
	if err != nil {
		return OpenResult{}, err
	}
	if p == "" {
		return OpenResult{Cancelled: true}, nil
	}
	if drop != nil {
		r, err := s.reopenDetached(ctx, d, "")
		if err == nil {
			r.State.Notice = removedNotice(s.sessionLang(), unsent, filepath.Base(p), r.State.Notice)
		}
		return r, err
	}
	return s.reopenDetached(ctx, d, detachedNotice(s.sessionLang(), !dropCache, unsent, filepath.Base(p)))
}

// detachTo détache le classeur d et l'enregistre dans path (vide : son
// fichier). Rend le nombre d'étapes jamais envoyées au serveur, gardées
// dans le classeur détaché. Le cache d'un classeur du serveur est d'abord
// enregistré (sa file d'envoi comprise, qui partira à sa prochaine
// ouverture), sauf dropCache : il est alors retiré. drop (non nil) est
// appelé une fois le suivi arrêté, avant tout changement local (Retirer
// du serveur) : en cas d'échec, le suivi reprend et rien ne change.
func (s *DocumentService) detachTo(ctx context.Context, d *doc.Document, path string, dropCache bool, drop func() error) (int, error) {
	unsent, _, err := workbook.Unsent(ctx, d.DB().Read())
	if err != nil {
		return 0, err
	}
	cache := ""
	if inShareCache(d.Path()) {
		cache = d.Path()
		if !dropCache {
			if dirty, _ := d.Dirty(ctx); dirty {
				if err := saveDoc(ctx, d, ""); err != nil {
					return 0, msg.Wrap(err, "share.detachCacheNotSaved", msg.Args{"err": err})
				}
			}
		}
	}
	// Plus de suivi du serveur : rien ne part pendant le détachement.
	s.stopShare()
	if drop != nil {
		if err := drop(); err != nil {
			s.watchShare(d)
			return 0, err
		}
	}
	if path != "" && !samePath(path, d.Path()) {
		// Enregistré sous le nouveau nom d'abord : un échec ne change rien.
		if err := saveDoc(ctx, d, path); err != nil {
			s.watchShare(d)
			return 0, err
		}
	}
	if err := d.DB().Write(ctx, func(tx *sql.Tx) error { return workbook.Detach(ctx, tx) }); err != nil {
		return 0, err
	}
	if err := d.ShareChanged(ctx); err != nil {
		return 0, err
	}
	if err := saveDoc(ctx, d, ""); err != nil {
		return 0, err
	}
	if dropCache && cache != "" {
		_ = os.Remove(cache)
	}
	return unsent, nil
}

// reopenDetached rouvre le classeur détaché d depuis son fichier : plus de
// suivi du serveur, journal logique sans tableau publié, déclencheurs du
// relevé retirés. L'historique d'annulation de la session est perdu.
func (s *DocumentService) reopenDetached(ctx context.Context, d *doc.Document, notice string) (OpenResult, error) {
	path := d.Path()
	s.replace(nil)
	r, err := s.open(ctx, path, doc.OpenNormal)
	if err != nil {
		return OpenResult{}, msg.Wrap(err, "share.detachReopenFailed", msg.Args{"path": path, "err": err})
	}
	if r.State.Open {
		if r.State.Notice != "" {
			notice += " " + r.State.Notice
		}
		r.State.Notice = notice
	}
	return r, nil
}

// detachedNotice : message de la barre d'état après « Travailler sur une
// copie locale » (asCopy) ou « Détacher du serveur ».
func detachedNotice(l msg.Lang, asCopy bool, unsent int, name string) string {
	id := msg.ID("share.detached")
	if asCopy {
		id = "share.localCopyOpened"
	}
	text := msg.Format(l, id, msg.Args{"name": name})
	if unsent > 0 {
		text += msg.Format(l, "share.unsentKept", msg.Args{"n": unsent})
	}
	return text
}
