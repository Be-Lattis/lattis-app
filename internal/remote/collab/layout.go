package collab

import (
	"context"
	"database/sql"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
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
	// Retour d'une longue coupure (pas de synchronisation de la mise en page
	// depuis LongAway) : un élément changé des deux côtés garde la version
	// des collègues (question 3 de l'audit du mode en ligne).
	long := e.layoutAway(ctx) > LongAway
	res, err := workbook.SyncLayoutAs(ctx, srv, e.db, info, force, long)
	// Un autre concepteur a enregistré la mise en page entre notre lecture
	// et notre écriture (40001 de lattis.save_meta) : tout est relu et
	// refusionné aussitôt, plutôt qu'au tour suivant après un passage hors
	// ligne.
	for try := 0; try < 2; try++ {
		if code, _ := workbook.Rejected(err); code != "40001" {
			break
		}
		res, err = workbook.SyncLayoutAs(ctx, srv, e.db, info, true, long)
	}
	if err != nil {
		return rep, err
	}
	e.layoutSynced(ctx)
	theme := e.theme()
	e.mu.Lock()
	e.layoutDone, e.layoutTheme = res.Gen, theme
	e.mu.Unlock()
	e.link.SetLayoutUnsent(false)
	if res.Applied > 0 {
		rep.Layout = true
		e.settleChoices(ctx)
	}
	if res.Lost {
		rep.notice(e.say("collab.layoutLost", nil))
	}
	if res.Conflicts > 0 {
		e.link.AddLayoutConflicts(res.Conflicts)
		if res.ServerWon {
			rep.notice(e.say("collab.layoutLongAway", msg.Args{"n": res.Conflicts}))
		} else {
			rep.notice(e.say("collab.layoutConflict", msg.Args{"n": res.Conflicts}))
		}
	}
	if res.Forgot {
		rep.notice(e.say("collab.layoutForgot", nil))
	}
	return rep, nil
}

// LayoutForgotNotice : historique effacé par une mise en page reçue
// (feuille supprimée, colonne calculée ajoutée ou retirée).
const LayoutForgotNotice = "Mise en page modifiée sur le serveur : historique d'annulation effacé"

// LayoutConflictNotice : modifications d'un collègue remplacées par les
// siennes (même élément de la mise en page changé des deux côtés).
func LayoutConflictNotice(n int) string {
	return msg.Format(msg.FR, "collab.layoutConflict", msg.Args{"n": n})
}

// LongAway : au-delà de cette durée sans synchronisation de la mise en
// page (poste resté hors ligne), les changements des collègues l'emportent
// sur ceux du poste pour un même élément.
var LongAway = 24 * time.Hour

// LayoutLongAwayNotice explique pourquoi, au retour d'une longue coupure,
// des changements de mise en page du poste ont été remplacés.
func LayoutLongAwayNotice(n int) string {
	return msg.Format(msg.FR, "collab.layoutLongAway", msg.Args{"n": n})
}

// layoutSyncedKey : date (RFC 3339) de la dernière synchronisation réussie
// de la mise en page (_lattis_info).
const layoutSyncedKey = "layout_synced"

// layoutAway : temps écoulé depuis la dernière synchronisation de la mise
// en page (0 si jamais notée : premier tour après cette version).
func (e *Engine) layoutAway(ctx context.Context) time.Duration {
	var v string
	if e.db.Read().QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, layoutSyncedKey).Scan(&v) != nil {
		return 0
	}
	at, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return 0
	}
	return time.Since(at)
}

// layoutSynced note la date de la synchronisation réussie.
func (e *Engine) layoutSynced(ctx context.Context) {
	_ = e.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, layoutSyncedKey, time.Now().UTC().Format(time.RFC3339))
		return err
	})
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
