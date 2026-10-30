package collab

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// worker fait le travail du suivi (envoi, réception) hors du fil de
// l'écoute, qui doit rester libre pour recevoir les notifications.
type worker struct {
	e       *Engine
	kick    chan struct{}
	changed func(Event)
	retry   time.Duration
	// stop arrête le suivi (classeur supprimé du serveur).
	stop func()

	mu sync.Mutex
	// due : tableaux notifiés à relire ; full : tout relire (connexion,
	// reprise après une coupure).
	due  map[int64]bool
	full bool
	// acl : droits à relire (notification *acl ; aussi à chaque connexion).
	acl       bool
	listening bool
	timer     *time.Timer

	// structure : tableaux dont la structure a changé sur le serveur
	// (*structure:<id>), relus en entier ; meta : structure de tous les
	// tableaux à comparer (*meta : clé, mise en page).
	structure map[int64]bool
	meta      bool
	// layoutWait : un tour est déjà prévu pour envoyer la mise en page.
	layoutWait bool
}

func (w *worker) poke() {
	select {
	case w.kick <- struct{}{}:
	default:
	}
}

// connected : l'écoute est posée (connexion ou reconnexion) ; la file part
// et tout est relu.
func (w *worker) connected() {
	w.mu.Lock()
	w.listening, w.full = true, true
	w.mu.Unlock()
	w.poke()
}

// disconnected : l'écoute est perdue ; le classeur passe hors ligne.
func (w *worker) disconnected(reason string) {
	// Sous w.mu, comme le passage en ligne d'un tour (cycle) : un tour
	// commencé avant la coupure ne remet pas le classeur en ligne après.
	w.mu.Lock()
	w.listening = false
	tell := w.e.link.Online() || w.e.link.State().Offline != reason
	if tell {
		w.e.link.SetOnline(false, reason)
	}
	w.mu.Unlock()
	if tell {
		w.changed(Event{State: true})
	}
}

// notified : un tableau a changé sur le serveur ; sa lecture attend
// Debounce pour regrouper une rafale.
func (w *worker) notified(ctx context.Context, payload string) {
	if payload == workbook.NoticeACL {
		// Droits changés (§8.1) : capacités relues au prochain cycle.
		w.mu.Lock()
		w.acl = true
		w.mu.Unlock()
		w.poke()
		return
	}
	structure := false
	switch {
	case strings.HasPrefix(payload, workbook.NoticeStructure):
		// Structure d'un tableau changée par un concepteur : relu en entier.
		payload, structure = strings.TrimPrefix(payload, workbook.NoticeStructure), true
	case payload == workbook.NoticeMeta:
		// Mise en page ou structure (clé, sur un serveur d'avant la
		// migration 0026 qui ne notifie pas *structure pour elle) : la
		// structure de tous les tableaux est comparée à celle du serveur.
		w.mu.Lock()
		w.meta = true
		if w.timer == nil {
			w.timer = time.AfterFunc(Debounce, w.poke)
		}
		w.mu.Unlock()
		return
	case strings.HasPrefix(payload, "*"):
		return // *version (classeur figé)
	}
	tbl, err := w.e.TableOf(ctx, payload)
	if err != nil || tbl == 0 {
		return
	}
	w.mu.Lock()
	if structure {
		w.structure[tbl] = true
	}
	w.due[tbl] = true
	if w.timer == nil {
		w.timer = time.AfterFunc(Debounce, w.poke)
	}
	w.mu.Unlock()
}

// layoutLater prévoit un tour dans wait (mise en page à envoyer).
func (w *worker) layoutLater(wait time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.layoutWait {
		return
	}
	w.layoutWait = true
	time.AfterFunc(wait, func() {
		w.mu.Lock()
		w.layoutWait = false
		w.mu.Unlock()
		w.poke()
	})
}

func (w *worker) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.kick:
			w.cycle(ctx)
		}
	}
}

// cycle envoie la file, puis lit les changements dus.
func (w *worker) cycle(ctx context.Context) {
	w.mu.Lock()
	full, due, listening, acl := w.full, w.due, w.listening, w.acl
	structure, meta := w.structure, w.meta
	w.full, w.due, w.acl = false, map[int64]bool{}, false
	w.structure, w.meta = map[int64]bool{}, false
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	w.mu.Unlock()
	if !listening || ctx.Err() != nil {
		return
	}
	// Classeur disparu du serveur : rien à envoyer ni à lire ; accès retiré,
	// seule la relecture des droits (connexion, *acl) peut le rendre.
	gone := w.e.link.Gone()
	if gone == workbook.GoneDeleted || gone != "" && !full && !acl {
		return
	}
	ev := Event{}
	w.mu.Lock()
	if !w.listening {
		// Coupé depuis le début du tour (disconnected) : reste hors ligne.
		w.mu.Unlock()
		return
	}
	if !w.e.link.Online() {
		w.e.link.SetOnline(true, "")
		ev.State = true
	}
	w.mu.Unlock()
	var err error
	if full || acl {
		// Droits relus avant l'envoi : une écriture devenue interdite est
		// refusée en local (grille grisée, message) plutôt qu'au serveur.
		// Le titre du classeur aussi (renommé sur le serveur). Relecture en
		// échec (coupure, connexion morte du pool à la reprise) : rien ne
		// part avec des droits d'avant la coupure, le tour est refait comme
		// après une coupure (sinon les droits changés pendant la coupure ne
		// sont relus qu'à la reprise suivante).
		title := w.e.link.State().Title
		var changed bool
		changed, err = w.e.RefreshRights(ctx)
		now := w.e.link.Gone()
		if err == nil && changed && now == "" {
			ev.State = true
			ev.Notices = append(ev.Notices, w.e.say("collab.rightsChanged", nil))
		}
		if w.e.link.State().Title != title || now != gone {
			ev.State = true
		}
		if now != "" {
			// Supprimé du serveur, ou plus d'accès : ni envoi (la file reste
			// dans le classeur), ni lecture, ni nouvel essai ; supprimé, le
			// suivi s'arrête.
			if now == workbook.GoneDeleted {
				w.stop()
			}
			w.changed(ev)
			return
		}
		if gone != "" && err == nil {
			full = true // accès rendu : tout est relu
		}
	}
	var rep Report
	if err == nil {
		// File à envoyer : la barre d'état dit « Synchronisation… » pendant
		// l'envoi (les modifications sont déjà faites et affichées en local).
		if p, _, st, sy, cerr := w.e.Counts(ctx); cerr == nil {
			ev.State = ev.State || st
			if p > 0 && w.e.link.SetSending(true) {
				sy = true
			}
			if sy {
				w.changed(Event{Sync: true})
			}
		}
		rep, err = w.e.Send(ctx)
		if w.e.link.SetSending(false) {
			ev.Sync = true
		}
	}
	if err == nil {
		// Tableaux nés sur ce poste (saisis, collés, insérés…, ici ou hors
		// ligne) : ajoutés au classeur partagé (partage automatique).
		var r Report
		r, err = w.e.AutoShare(ctx)
		rep.add(r)
		if len(r.Shared) > 0 {
			ev.State = true // tableaux partagés : marque « sur ce poste » retirée
		}
	}
	if err == nil {
		// Catégorie d'un onglet à choix posée, changée ou retirée ici par un
		// concepteur ; à la connexion, celles changées hors ligne.
		err = w.e.PublishChoices(ctx, full)
	}
	if err == nil {
		// Scripts changés ici par un concepteur, ou sur le serveur
		// (connexion, « *meta »).
		var r Report
		r, err = w.e.SyncScripts(ctx, full || meta)
		rep.add(r)
	}
	if err == nil {
		var r Report
		switch {
		case full || meta:
			// Connexion : la structure a pu changer pendant la coupure.
			r, err = w.e.ReceiveStructure(ctx, nil, structure)
		case len(structure) > 0:
			r, err = w.e.ReceiveStructure(ctx, due, structure)
		case len(due) > 0:
			r, err = w.e.Receive(ctx, due)
		}
		rep.add(r)
	}
	if err == nil {
		// Mise en page : relue à la connexion et sur « *meta » ; envoyée
		// LayoutDelay après la dernière modification locale.
		force := full || meta
		if due, wait := w.e.layoutDue(); force || due {
			var r Report
			r, err = w.e.SyncLayout(ctx, force)
			rep.add(r)
		} else if wait > 0 {
			w.layoutLater(wait)
		}
	}
	if err != nil && ctx.Err() == nil {
		// Coupure pendant l'envoi ou la lecture : hors ligne, tout sera
		// relu au prochain essai.
		w.mu.Lock()
		w.full = true
		w.mu.Unlock()
		w.e.link.SetOnline(false, errText(err))
		ev.State = true
		time.AfterFunc(w.retry, w.poke)
	}
	if _, _, st, sy, cerr := w.e.Counts(ctx); cerr == nil {
		ev.State = ev.State || st
		ev.Sync = ev.Sync || sy
	}
	ev.Data = rep.Changed()
	ev.Notices = append(ev.Notices, rep.Notices...)
	if ev.Data || ev.State || ev.Sync || len(ev.Notices) > 0 {
		w.changed(ev)
	}
}
