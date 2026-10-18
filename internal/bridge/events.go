package bridge

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lalamefine/lattis/internal/api"
)

// Évènements du classeur (internal/api/events.go) déclenchés par les
// actions de l'utilisateur : ce fichier les repère et les fait traiter, un
// à la fois et dans l'ordre, par une file à part. Un évènement ne retient
// donc jamais l'interface : seuls before_save et before_close attendent
// leurs écouteurs, pour pouvoir annuler l'action.

// selectionPause : la sélection doit rester immobile ce temps-là avant
// l'évènement selection (pas un évènement par case survolée en glissant).
const selectionPause = 250 * time.Millisecond

// sheetSettle : après un changement d'onglet, l'interface rapporte la
// sélection de la nouvelle feuille ; ce n'est pas un évènement selection.
const sheetSettle = 400 * time.Millisecond

type eventLoop struct {
	once  sync.Once
	queue chan func()

	mu       sync.Mutex
	last     api.Selection // dernière sélection rapportée par l'interface
	selTimer *time.Timer
	sheetAt  time.Time
	opened   bool // open déjà déclenché pour ce classeur

	// closeOK : before_close a laissé fermer la fenêtre.
	closeOK atomic.Bool
	// closing : les écouteurs before_close de la fermeture de la fenêtre
	// tournent.
	closing atomic.Bool
}

func (s *DocumentService) evQueue() chan func() {
	s.ev.once.Do(func() {
		s.ev.queue = make(chan func(), 256)
		go func() {
			for f := range s.ev.queue {
				f()
			}
		}()
	})
	return s.ev.queue
}

// resetEvents prépare les évènements d'un classeur qui s'ouvre.
func (s *DocumentService) resetEvents() {
	s.ev.mu.Lock()
	s.ev.last = api.Selection{}
	if s.ev.selTimer != nil {
		s.ev.selTimer.Stop()
	}
	s.ev.opened = false
	s.ev.mu.Unlock()
	s.ev.closeOK.Store(false)
	api.ResetEvents()
}

func (s *DocumentService) listening(event string) bool {
	if s.scripting() != nil {
		return false // sans scripting, aucun écouteur Lua ne tourne
	}
	return api.Listening(host{s}, event)
}

// fireAsync fait traiter un évènement par la file, sans l'attendre ; un
// évènement de trop (file pleine) est perdu.
func (s *DocumentService) fireAsync(event string, data map[string]any) {
	select {
	case s.evQueue() <- func() {
		r, err := api.FireEvent(host{s}, event, data)
		if eventDone != nil {
			eventDone(event, r, err)
		}
	}:
	default:
	}
}

// eventDone reçoit l'issue de chaque évènement traité sans attente (tests).
var eventDone func(event string, r api.EventResult, err error)

// fireSync fait traiter un évènement par la file et attend ses écouteurs.
func (s *DocumentService) fireSync(event string, data map[string]any) api.EventResult {
	done := make(chan api.EventResult, 1)
	s.evQueue() <- func() {
		r, err := api.FireEvent(host{s}, event, data)
		if err != nil && r.Error == "" {
			r.Error = err.Error()
		}
		done <- r
	}
	return <-done
}

// --- Cellules modifiées ---------------------------------------------------------

// changeWatch prépare l'évènement change d'une plage (à partir de 0) ; old
// est la cellule avant la modification, quand la plage n'en a qu'une.
type changeWatch struct {
	sheet          int64
	r0, c0, r1, c1 int
	source         string
	old            *api.CellValue
	data           map[string]any // déjà décrit (ligne de tableau supprimée)
}

// watchChange relève la plage avant une modification ; nil si personne
// n'écoute change (rien n'est lu).
func (s *DocumentService) watchChange(sheet int64, r0, c0, r1, c1 int, source string) *changeWatch {
	if !s.listening(api.EventChange) {
		return nil
	}
	w := &changeWatch{sheet: sheet, r0: min(r0, r1), c0: min(c0, c1), r1: max(r0, r1), c1: max(c0, c1), source: source}
	if w.r0 == w.r1 && w.c0 == w.c1 {
		if v, err := api.CellAt(host{s}, sheet, w.r0, w.c0); err == nil {
			w.old = &v
		}
	}
	return w
}

// fireChange déclenche change après une modification réussie.
func (s *DocumentService) fireChange(w *changeWatch) {
	if w == nil {
		return
	}
	data := w.data
	if data == nil {
		data = api.RangeEventData(host{s}, w.sheet, w.r0, w.c0, w.r1, w.c1)
		if w.old != nil {
			if v, err := api.CellAt(host{s}, w.sheet, w.r0, w.c0); err == nil {
				data["value"], data["old"] = v.Value, w.old.Value
			}
		}
	}
	data["source"] = w.source
	s.fireAsync(api.EventChange, data)
}

// watchTableRow prépare change pour la ligne de données n d'un tableau.
func (s *DocumentService) watchTableRow(table string, n int, source string) *changeWatch {
	if !s.listening(api.EventChange) {
		return nil
	}
	return &changeWatch{source: source, data: api.TableRowEventData(host{s}, table, n)}
}

// fireTableRow déclenche change pour une ligne de tableau ; la ligne
// ajoutée est décrite après coup, la ligne supprimée avant.
func (s *DocumentService) fireTableRow(w *changeWatch, table string, n int) {
	if w == nil {
		return
	}
	if w.data == nil {
		w.data = api.TableRowEventData(host{s}, table, n)
	}
	if w.data == nil {
		return
	}
	s.fireChange(w)
}

// --- Feuilles ---------------------------------------------------------------------

func (s *DocumentService) fireSheet(event string, sheet int64) {
	if s.listening(event) {
		s.fireAsync(event, api.SheetEventData(host{s}, sheet))
	}
}

// --- Sélection et onglet ------------------------------------------------------------

// selectionChanged compare la sélection rapportée par l'interface à la
// précédente : onglet changé (sheet), ou plage changée (selection, après
// une pause). Une sélection posée par un script (ui.select) est déjà connue
// quand l'interface la rapporte : elle ne déclenche rien.
func (s *DocumentService) selectionChanged(prev, sel api.Selection) {
	d, err := s.current()
	if err != nil || prev.Sheet == 0 {
		return
	}
	s.ev.mu.Lock()
	defer s.ev.mu.Unlock()
	if sel.Sheet != prev.Sheet {
		if s.ev.selTimer != nil {
			s.ev.selTimer.Stop()
		}
		s.ev.sheetAt = time.Now()
		if s.listening(api.EventSheet) {
			data := api.SheetEventData(host{s}, sel.Sheet)
			old := api.SheetEventData(host{s}, prev.Sheet)
			data["previous"], data["previous_id"] = old["sheet"], prev.Sheet
			s.fireAsync(api.EventSheet, data)
		}
		return
	}
	if sel.R0 == prev.R0 && sel.C0 == prev.C0 && sel.R1 == prev.R1 && sel.C1 == prev.C1 {
		return // la vue a changé (grille, SQL, script), pas la sélection
	}
	if sel.View != "" && sel.View != "grid" || time.Since(s.ev.sheetAt) < sheetSettle {
		return
	}
	if s.ev.selTimer != nil {
		s.ev.selTimer.Stop()
	}
	s.ev.selTimer = time.AfterFunc(selectionPause, func() {
		if cur, err := s.current(); err != nil || cur != d || !s.listening(api.EventSelection) {
			return
		}
		s.fireAsync(api.EventSelection, api.RangeEventData(host{s}, sel.Sheet, sel.R0, sel.C0, sel.R1, sel.C1))
	})
}

// --- Ouverture et fermeture -----------------------------------------------------------

// WorkbookReady est appelé par l'interface quand un classeur ouvert est
// affiché : ses écouteurs open s'exécutent (une fois par classeur).
func (s *DocumentService) WorkbookReady(ctx context.Context) {
	d, err := s.current()
	if err != nil {
		return
	}
	s.ev.mu.Lock()
	done := s.ev.opened
	s.ev.opened = true
	s.ev.mu.Unlock()
	if done || !s.listening(api.EventOpen) {
		return
	}
	st, _ := s.State(ctx)
	s.fireAsync(api.EventOpen, map[string]any{"path": d.Path(), "title": st.Title})
}

// BeforeClose exécute les écouteurs before_close ; faux si l'un d'eux
// annule la fermeture (il a renvoyé false).
func (s *DocumentService) BeforeClose(ctx context.Context) bool {
	d, err := s.current()
	if err != nil || !s.listening(api.EventBeforeClose) {
		return true
	}
	dirty, _ := d.Dirty(ctx)
	r := s.fireSync(api.EventBeforeClose, map[string]any{"path": d.Path(), "dirty": dirty})
	if r.Cancelled {
		host{s}.Notify("Fermeture annulée par un script (évènement before_close)")
	}
	return !r.Cancelled
}
