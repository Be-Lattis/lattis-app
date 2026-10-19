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

	mu sync.Mutex
	// due : tableaux notifiés à relire ; full : tout relire (connexion,
	// reprise après une coupure).
	due       map[int64]bool
	full      bool
	// acl : droits à relire (notification *acl ; aussi à chaque connexion).
	acl       bool
	listening bool
	timer     *time.Timer
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
	w.mu.Lock()
	w.listening = false
	w.mu.Unlock()
	if w.e.link.Online() || w.e.link.State().Offline != reason {
		w.e.link.SetOnline(false, reason)
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
	if strings.HasPrefix(payload, "*") {
		return // *meta (mise en page, étape 7), *version (classeur figé)
	}
	tbl, err := w.e.TableOf(ctx, payload)
	if err != nil || tbl == 0 {
		return
	}
	w.mu.Lock()
	w.due[tbl] = true
	if w.timer == nil {
		w.timer = time.AfterFunc(Debounce, w.poke)
	}
	w.mu.Unlock()
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
	w.full, w.due, w.acl = false, map[int64]bool{}, false
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	w.mu.Unlock()
	if !listening || ctx.Err() != nil {
		return
	}
	ev := Event{}
	if !w.e.link.Online() {
		w.e.link.SetOnline(true, "")
		ev.State = true
	}
	if full || acl {
		// Droits relus avant l'envoi : une écriture devenue interdite est
		// refusée en local (grille grisée, message) plutôt qu'au serveur.
		if changed, err := w.e.RefreshRights(ctx); err == nil && changed {
			ev.State = true
			ev.Notices = append(ev.Notices, RightsNotice)
		}
	}
	rep, err := w.e.Send(ctx)
	if err == nil {
		var r Report
		switch {
		case full:
			r, err = w.e.Receive(ctx, nil)
		case len(due) > 0:
			r, err = w.e.Receive(ctx, due)
		}
		rep.add(r)
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
	if _, _, changed, cerr := w.e.Counts(ctx); cerr == nil && changed {
		ev.State = true
	}
	ev.Data = rep.Changed()
	ev.Notices = append(ev.Notices, rep.Notices...)
	if ev.Data || ev.State || len(ev.Notices) > 0 {
		w.changed(ev)
	}
}
