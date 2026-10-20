package bridge

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/workbook"
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
		r, err := s.detachAs(ctx, d, "Enregistrer une copie locale", docBaseName(d)+".lattis", true)
		if err == nil && !r.Cancelled {
			dropRecent(RecentFile{Server: st.Server, WorkbookID: st.WorkbookID})
		}
		return r, err
	}
	return s.detachAs(ctx, d, "Travailler sur une copie locale", docBaseName(d)+" (copie locale).lattis", false)
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
		return s.detachAs(ctx, d, "Détacher du serveur : enregistrer le classeur", docBaseName(d)+".lattis", true)
	}
	unsent, err := s.detachTo(ctx, d, "", false)
	if err != nil {
		return OpenResult{}, err
	}
	return s.reopenDetached(ctx, d, detachedNotice(false, unsent, filepath.Base(d.Path())))
}

// detachAs demande où enregistrer le classeur détaché (name : nom proposé),
// l'y enregistre puis l'ouvre ; dropCache retire ensuite le cache du
// classeur du serveur (Détacher).
func (s *DocumentService) detachAs(ctx context.Context, d *doc.Document, title, name string, dropCache bool) (OpenResult, error) {
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
		unsent, werr = s.detachTo(ctx, d, p, dropCache)
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
	return s.reopenDetached(ctx, d, detachedNotice(!dropCache, unsent, filepath.Base(p)))
}

// detachTo détache le classeur d et l'enregistre dans path (vide : son
// fichier). Rend le nombre d'étapes jamais envoyées au serveur, gardées
// dans le classeur détaché. Le cache d'un classeur du serveur est d'abord
// enregistré (sa file d'envoi comprise, qui partira à sa prochaine
// ouverture), sauf dropCache : il est alors retiré.
func (s *DocumentService) detachTo(ctx context.Context, d *doc.Document, path string, dropCache bool) (int, error) {
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
func detachedNotice(asCopy bool, unsent int, name string) string {
	msg := fmt.Sprintf("Classeur détaché du serveur : « %s » est maintenant un classeur local, il ne se synchronise plus.", name)
	if asCopy {
		msg = fmt.Sprintf("Copie locale « %s » ouverte : un classeur local, sans lien avec le serveur ; le classeur partagé n'a pas changé.", name)
	}
	switch {
	case unsent == 1:
		msg += " 1 modification n'avait pas été envoyée au serveur : elle est dans ce classeur, pas sur le serveur."
	case unsent > 1:
		msg += fmt.Sprintf(" %d modifications n'avaient pas été envoyées au serveur : elles sont dans ce classeur, pas sur le serveur.", unsent)
	}
	return msg
}
