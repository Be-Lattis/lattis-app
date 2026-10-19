package workbook

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// Notifications d'un classeur collaboratif (contrat client de lattisd,
// §6) : NoticeStructure suivi de l'identifiant du tableau (colonnes
// ajoutées, renommées, supprimées ou converties, tableau renommé ou
// supprimé : à recharger en entier) ; NoticeMeta, mise en page ou
// structure (clé).
const (
	NoticeStructure = "*structure:"
	NoticeMeta      = "*meta"
)

// VersionNotice lit une notification de republication (« *version:<n> »,
// lattis.finish_publish). Le canal porte aussi « *meta » et les
// identifiants des tableaux modifiés (classeur collaboratif : voir
// internal/remote/collab) ; un classeur figé ne reçoit que « *version:<n> »
// (depuis la migration 0007 du serveur, la version de travail d'une
// republication ne notifie plus ; un serveur plus ancien le faisait, d'où
// le filtre ici).
func VersionNotice(payload string) (int, bool) {
	s, ok := strings.CutPrefix(payload, "*version:")
	if !ok {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	return v, err == nil
}

// Watch suit un classeur partagé ouvert jusqu'à l'annulation de ctx :
// écoute de son canal (LISTEN), version publiée vérifiée à chaque
// (re)connexion, passage hors ligne quand le serveur ne répond plus et
// nouvel essai toutes les retry. changed est appelé à chaque changement
// d'état (en ligne, hors ligne, nouvelle version).
func Watch(ctx context.Context, srv Server, link *Link, retry time.Duration, changed func()) {
	info := link.Info()
	if info == nil {
		return
	}
	if changed == nil {
		changed = func() {}
	}
	for ctx.Err() == nil {
		err := srv.Listen(ctx, info.Channel, func() {
			link.SetOnline(true, "")
			if caps, err := srv.Capabilities(ctx, info.WorkbookID); err == nil {
				link.SetNewVersion(caps.Version)
			}
			// Rôles (propriétaire ou concepteur : préparer la version
			// suivante) relus à chaque (re)connexion.
			RefreshCapabilities(ctx, srv, link)
			changed()
		}, func(p string) {
			if v, ok := VersionNotice(p); ok && link.SetNewVersion(v) {
				changed()
			}
			if p == NoticeACL {
				if ch, err := RefreshCapabilities(ctx, srv, link); err == nil && ch {
					changed()
				}
			}
		})
		if ctx.Err() != nil {
			return
		}
		reason := "serveur injoignable"
		if err != nil {
			reason = err.Error()
		}
		if link.Online() || link.State().Offline != reason {
			link.SetOnline(false, reason)
			changed()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retry):
		}
	}
}
