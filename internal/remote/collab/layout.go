package collab

import (
	"context"
	"fmt"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Mise en page en direct d'un classeur collaboratif publié (cellules libres,
// mises en forme, largeurs, feuilles, graphiques… ; remote/workbook/
// livelayout.go, docs/collaboration/synchronisation.md §2.14, décision
// 0018) : envoyée par les propriétaires et concepteurs LayoutDelay après
// leur dernière modification (et à la fermeture, à la connexion), reçue
// par tous sur « *meta » et à la connexion.

// LayoutDelay : délai entre la dernière modification de la mise en page et
// son envoi (une rafale de saisies ne relit et n'écrit la mise en page
// qu'une fois).
var LayoutDelay = time.Second

// SyncLayout envoie la mise en page modifiée sur le poste (concepteurs)
// et applique celle du serveur ; force : relire le serveur même sans
// modification locale (connexion, « *meta »).
func (e *Engine) SyncLayout(ctx context.Context, force bool) (Report, error) {
	e.run.Lock()
	defer e.run.Unlock()
	return e.syncLayout(ctx, force)
}

func (e *Engine) syncLayout(ctx context.Context, force bool) (Report, error) {
	var rep Report
	info, srv := e.active()
	if info == nil || !e.link.Online() || e.link.Gone() != "" {
		return rep, nil
	}
	res, err := workbook.SyncLayout(ctx, srv, e.db, info, force)
	// Un autre concepteur a enregistré la mise en page entre notre lecture
	// et notre écriture (40001 de lattis.save_meta) : tout est relu et
	// refusionné aussitôt, plutôt qu'au tour suivant après un passage hors
	// ligne.
	for try := 0; try < 2; try++ {
		if code, _ := workbook.Rejected(err); code != "40001" {
			break
		}
		res, err = workbook.SyncLayout(ctx, srv, e.db, info, true)
	}
	if err != nil {
		return rep, err
	}
	theme := e.theme()
	e.mu.Lock()
	e.layoutDone, e.layoutTheme = res.Gen, theme
	e.mu.Unlock()
	e.link.SetLayoutUnsent(false)
	if res.Applied > 0 {
		rep.Layout = true
		e.settleChoices(ctx)
	}
	if res.Conflicts > 0 {
		e.link.AddLayoutConflicts(res.Conflicts)
		rep.notice(LayoutConflictNotice(res.Conflicts))
	}
	if res.Forgot {
		rep.notice(LayoutForgotNotice)
	}
	return rep, nil
}

// LayoutForgotNotice : historique effacé par une mise en page reçue
// (feuille supprimée, colonne calculée ajoutée ou retirée).
const LayoutForgotNotice = "Mise en page modifiée sur le serveur : historique d'annulation effacé"

// LayoutConflictNotice : modifications d'un collègue remplacées par les
// siennes (même élément de la mise en page changé des deux côtés).
func LayoutConflictNotice(n int) string {
	if n == 1 {
		return "Mise en page : 1 modification d'un collègue faite au même moment a été remplacée par la vôtre"
	}
	return fmt.Sprintf("Mise en page : %d modifications d'un collègue faites au même moment ont été remplacées par les vôtres", n)
}

// layoutDue dit si la mise en page a changé sur le poste depuis la
// dernière synchronisation et si LayoutDelay est passé depuis la dernière
// modification (sinon wait : le temps qui reste).
func (e *Engine) layoutDue() (bool, time.Duration) {
	g := e.db.WatchGen()
	theme := e.theme()
	e.mu.Lock()
	defer e.mu.Unlock()
	if g == e.layoutDone && theme == e.layoutTheme {
		return false, 0
	}
	if theme != e.layoutTheme {
		// Thème du classeur (_lattis_info, hors de Store.Watch) : compté
		// comme une écriture de la mise en page.
		e.layoutTheme, e.layoutDone = theme, -1
	}
	if g != e.layoutSeen {
		e.layoutSeen, e.layoutAt = g, time.Now()
	}
	wait := LayoutDelay - time.Since(e.layoutAt)
	return wait <= 0, max(wait, 0)
}

// LayoutDirty : la mise en page a changé sur le poste depuis la dernière
// synchronisation.
func (e *Engine) LayoutDirty() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.db.WatchGen() != e.layoutDone
}

// FlushLayout envoie tout de suite la mise en page modifiée (fermeture du
// classeur) ; hors ligne, ou sans réponse avant ctx, elle reste à envoyer
// (Link.LayoutUnsent : le cache est gardé, elle partira à la reconnexion).
func (e *Engine) FlushLayout(ctx context.Context) {
	if e.LayoutDirty() {
		_, _ = e.SyncLayout(ctx, false)
	}
	e.link.SetLayoutUnsent(e.LayoutDirty() && e.canDesign())
}

// theme lit le thème du classeur ("" : aucun).
func (e *Engine) theme() string {
	var s string
	_ = e.db.Read().QueryRow(`SELECT value FROM _lattis_info WHERE key = ?`, canvas.ThemeKey).Scan(&s)
	return s
}

func (e *Engine) canDesign() bool {
	info := e.link.Info()
	return info != nil && info.CanDesign()
}
