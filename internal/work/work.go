// Package work suit les opérations longues pour la barre d'état : l'action
// en cours (« Suppression de la colonne Prix »), son étape (« réécriture du
// tableau ») et, quand on la connaît sans rien relire, l'avancement (n sur
// total). Le suivi voyage dans le contexte : le pont (internal/bridge) le
// pose autour d'un appel de l'interface, les couches du dessous (canvas,
// command, store) y écrivent sans savoir s'il y en a un.
//
// Rien n'est émis pour une opération courte : la première émission attend
// Delay, les suivantes sont espacées d'au moins Every. Une étape ne coûte
// qu'une recherche dans le contexte et quelques comparaisons : aucune
// requête n'est faite pour la décrire.
package work

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Step est l'état d'une opération, tel que l'interface l'affiche. Op et
// Phase sont des clés (traduites par l'interface : shell.work.*), Name ce
// que l'action vise (colonne, tableau). End : l'opération est finie.
type Step struct {
	ID     int64  `json:"id"`
	Op     string `json:"op,omitempty"`
	Name   string `json:"name,omitempty"`
	Phase  string `json:"phase,omitempty"`
	Done   int64  `json:"done,omitempty"`
	Total  int64  `json:"total,omitempty"`
	Cancel bool   `json:"cancel,omitempty"`
	End    bool   `json:"end,omitempty"`
}

// Delay : une opération plus courte n'est jamais montrée. Every : écart
// minimal entre deux émissions d'une même opération.
var (
	Delay = 300 * time.Millisecond
	Every = 150 * time.Millisecond
)

// ErrCancelled est la cause d'une opération interrompue par l'utilisateur.
var ErrCancelled = errors.New("opération annulée")

var nextID atomic.Int64

// Tracker suit une opération.
type Tracker struct {
	emit   func(Step)
	cancel context.CancelCauseFunc

	mu      sync.Mutex
	cur     Step
	shown   bool      // première émission faite (après Delay)
	ended   bool      // End appelé : plus rien n'est émis
	dirty   bool      // changement pas encore émis
	last    time.Time // dernière émission
	timer   *time.Timer
	hold    bool        // étape de Loop : les instructions ne la changent pas
	final   bool        // Final passé : Cancel ne peut plus l'interrompre
	stopped atomic.Bool // annulée par l'utilisateur

	emitMu sync.Mutex // émissions dans l'ordre, la fin en dernier
}

type key struct{}

// From donne le suivi porté par ctx (nil s'il n'y en a pas).
func From(ctx context.Context) *Tracker {
	if ctx == nil {
		return nil
	}
	t, _ := ctx.Value(key{}).(*Tracker)
	return t
}

// Start commence le suivi d'une opération ; emit reçoit ses états (depuis
// une autre goroutine). cancellable : Cancel peut l'interrompre, son
// contexte est alors annulé (cause ErrCancelled). Dans une opération déjà
// suivie, rien ne commence : ctx est rendu tel quel, avec un suivi nil
// (End et Cancel sur nil ne font rien), et l'opération intérieure écrit
// dans celle du dessus.
func Start(ctx context.Context, emit func(Step), cancellable bool) (context.Context, *Tracker) {
	if From(ctx) != nil || emit == nil {
		return ctx, nil
	}
	t := &Tracker{emit: emit}
	t.cur.ID = nextID.Add(1)
	if cancellable {
		ctx, t.cancel = context.WithCancelCause(ctx)
		t.cur.Cancel = true
	}
	t.timer = time.AfterFunc(Delay, t.flush)
	return context.WithValue(ctx, key{}, t), t
}

// ID identifie l'opération (pour Cancel) ; 0 sur nil.
func (t *Tracker) ID() int64 {
	if t == nil {
		return 0
	}
	return t.cur.ID
}

// Cancel interrompt l'opération si elle peut l'être ; vrai si c'est le cas.
func (t *Tracker) Cancel() bool {
	if t == nil || t.cancel == nil {
		return false
	}
	t.mu.Lock()
	if t.final {
		t.mu.Unlock()
		return false
	}
	t.stopped.Store(true)
	t.mu.Unlock()
	t.cancel(ErrCancelled)
	return true
}

// Final marque le point de non-retour d'une opération qu'on pouvait
// interrompre (publication : la transaction du serveur va être validée) :
// Cancel ne l'interrompt plus et « Annuler » disparaît. Faux si elle a déjà
// été annulée (ce qui suit ne doit pas être fait) ; vrai sans suivi.
func Final(ctx context.Context) bool {
	t := From(ctx)
	if t == nil || t.cancel == nil {
		return true
	}
	ok := true
	t.change(func(s *Step) bool {
		if t.stopped.Load() {
			ok = false
			return false
		}
		t.final = true
		if !s.Cancel {
			return false
		}
		s.Cancel = false
		return true
	})
	return ok
}

// Cancelled : l'opération a été interrompue par Cancel.
func (t *Tracker) Cancelled() bool { return t != nil && t.stopped.Load() }

// End termine le suivi : si l'opération a été montrée, l'interface reçoit
// sa fin. Le contexte de l'opération est libéré.
func (t *Tracker) End() {
	if t == nil {
		return
	}
	t.mu.Lock()
	if t.ended {
		t.mu.Unlock()
		return
	}
	t.ended = true
	t.timer.Stop()
	shown := t.shown
	end := t.cur
	t.mu.Unlock()
	if shown {
		end.End = true
		t.emitMu.Lock()
		t.emit(end)
		t.emitMu.Unlock()
	}
	if t.cancel != nil {
		t.cancel(nil)
	}
}

// flush émet l'état courant (minuterie : première émission, ou changement
// gardé par Every).
func (t *Tracker) flush() {
	t.emitMu.Lock()
	defer t.emitMu.Unlock()
	t.mu.Lock()
	if t.ended {
		t.mu.Unlock()
		return
	}
	t.shown, t.dirty, t.last = true, false, time.Now()
	s := t.cur
	t.mu.Unlock()
	t.emit(s)
}

// change applique f à l'état courant et l'émet, au plus tôt Every après la
// précédente émission ; avant Delay, rien (la minuterie s'en charge).
func (t *Tracker) change(f func(*Step) bool) {
	t.mu.Lock()
	if t.ended || !f(&t.cur) {
		t.mu.Unlock()
		return
	}
	if !t.shown || t.dirty {
		t.mu.Unlock()
		return
	}
	if wait := Every - time.Since(t.last); wait > 0 {
		t.dirty = true
		t.timer.Reset(wait)
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()
	t.flush()
}

// Op nomme l'action (clé, et ce qu'elle vise). La première nommée garde
// son nom : une action faite à l'intérieur d'une autre n'en est qu'une
// étape.
func Op(ctx context.Context, op, name string) {
	if t := From(ctx); t != nil {
		t.change(func(s *Step) bool {
			if s.Op != "" {
				return false
			}
			s.Op, s.Name = op, name
			return true
		})
	}
}

// Phase indique l'étape en cours (clé) ; l'avancement repart de zéro.
func Phase(ctx context.Context, phase string) {
	if t := From(ctx); t != nil {
		t.phase(phase, false, 0)
	}
}

// Loop indique une étape faite par une boucle de total tours (lignes d'un
// collage…) : Progress en donne l'avancement, et les instructions SQL de
// chaque tour ne changent pas l'étape (Statement) jusqu'à la suivante.
func Loop(ctx context.Context, phase string, total int64) {
	if t := From(ctx); t != nil {
		t.phase(phase, true, total)
	}
}

func (t *Tracker) phase(phase string, hold bool, total int64) {
	t.change(func(s *Step) bool {
		t.hold = hold
		if s.Phase == phase && s.Total == total && s.Done == 0 {
			return false
		}
		s.Phase, s.Done, s.Total = phase, 0, total
		return true
	})
}

// Progress donne l'avancement de l'étape : done sur total.
func Progress(ctx context.Context, done, total int64) {
	if t := From(ctx); t != nil {
		t.change(func(s *Step) bool {
			if s.Done == done && s.Total == total {
				return false
			}
			s.Done, s.Total = done, total
			return true
		})
	}
}

// Statement indique l'étape d'après une instruction SQL qui va être jouée :
// les instructions lourdes des opérations sur les tableaux (corbeille,
// reconstruction, index, mise à jour des lignes…) se reconnaissent à leur
// début. Les autres ne changent pas l'étape.
func Statement(ctx context.Context, q string) {
	t := From(ctx)
	if t == nil {
		return
	}
	t.mu.Lock()
	hold := t.hold
	t.mu.Unlock()
	if hold {
		return
	}
	if p := Classify(q); p != "" {
		t.phase(p, false, 0)
	}
}

// Classify donne l'étape d'une instruction SQL ("" : rien à dire).
// Tableaux de données : t_<id> ; corbeilles : _lattis_trash_… ;
// reconstructions : t_<id>_rebuild ; rangs d'un tri : _lattis_rank_….
func Classify(q string) string {
	q = strings.TrimLeft(q, " \t\r\n")
	switch {
	case strings.HasPrefix(q, "INSERT INTO _lattis_trash"), strings.HasPrefix(q, "CREATE TABLE _lattis_trash") && strings.Contains(q, " AS SELECT"):
		return "keep"
	case strings.HasPrefix(q, "INSERT INTO _lattis_rank"):
		return "rank"
	case strings.HasPrefix(q, "INSERT INTO t_") && strings.Contains(q, "_rebuild"),
		strings.HasPrefix(q, "ALTER TABLE t_") && strings.Contains(q, " DROP COLUMN "):
		return "rewrite"
	case strings.HasPrefix(q, "ALTER TABLE t_"):
		return "schema"
	case strings.HasPrefix(q, "CREATE INDEX"), strings.HasPrefix(q, "CREATE UNIQUE INDEX"):
		return "index"
	case strings.HasPrefix(q, "UPDATE t_") && strings.Contains(q, " SET _pos "):
		return "order"
	case strings.HasPrefix(q, "UPDATE t_"):
		return "update"
	case strings.HasPrefix(q, "DELETE FROM t_"):
		return "delete"
	case strings.HasPrefix(q, "INSERT INTO t_"):
		return "insert"
	case strings.HasPrefix(q, "DROP TABLE"):
		return "drop"
	}
	return ""
}
