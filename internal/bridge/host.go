package bridge

import (
	"context"
	"errors"
	"sync"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/query"
)

// UIEvent porte une commande de l'interface venue d'un script ou du serveur
// MCP : sélection, changement de vue, boîte de dialogue, message.
const UIEvent = "ui:command"

// UICommand est une commande pour l'interface.
type UICommand struct {
	// Kind : select, view, dialog, dialog-close ou notify.
	Kind      string         `json:"kind"`
	Selection *api.Selection `json:"selection,omitempty"`
	View      string         `json:"view,omitempty"`
	Dialog    *DialogRequest `json:"dialog,omitempty"`
	ID        int            `json:"id,omitempty"` // dialog-close
	Message   string         `json:"message,omitempty"`
}

// DialogRequest est une boîte de dialogue à afficher ; la réponse revient par
// DocumentService.DialogAnswer.
type DialogRequest struct {
	ID int `json:"id"`
	api.Dialog
}

type dialogAnswer struct {
	values map[string]any
	ok     bool
}

// host présente le DocumentService aux commandes (internal/api) sans
// exposer ces méthodes à l'interface.
type host struct{ s *DocumentService }

// ui : état de l'interface vu du Go.
type uiState struct {
	mu      sync.Mutex
	sel     api.Selection
	nextID  int
	dialogs map[int]chan dialogAnswer
}

func (s *DocumentService) emitUI(c UICommand) bool {
	return s.sys().Emit(UIEvent, c)
}

func (h host) Doc() (*doc.Document, error) { return h.s.current() }

func (h host) Changed(ctx context.Context) { h.s.changed(ctx) }

func (h host) Selection() api.Selection {
	h.s.ui.mu.Lock()
	defer h.s.ui.mu.Unlock()
	return h.s.ui.sel
}

func (h host) Select(_ context.Context, sel api.Selection) error {
	h.s.ui.mu.Lock()
	h.s.ui.sel = sel
	h.s.ui.mu.Unlock()
	h.s.emitUI(UICommand{Kind: "select", Selection: &sel})
	return nil
}

func (h host) ShowView(_ context.Context, view string) error {
	h.s.ui.mu.Lock()
	h.s.ui.sel.View = view
	h.s.ui.mu.Unlock()
	h.s.emitUI(UICommand{Kind: "view", View: view})
	return nil
}

var errNoUI = msg.New("host.noWindowDialog", nil)

func (h host) Dialog(ctx context.Context, d api.Dialog) (map[string]any, bool, error) {
	u := &h.s.ui
	u.mu.Lock()
	u.nextID++
	id := u.nextID
	ch := make(chan dialogAnswer, 1)
	if u.dialogs == nil {
		u.dialogs = map[int]chan dialogAnswer{}
	}
	u.dialogs[id] = ch
	u.mu.Unlock()
	defer func() {
		u.mu.Lock()
		delete(u.dialogs, id)
		u.mu.Unlock()
	}()
	if !h.s.emitUI(UICommand{Kind: "dialog", Dialog: &DialogRequest{ID: id, Dialog: d}}) {
		return nil, false, errNoUI
	}
	select {
	case a := <-ch:
		return a.values, a.ok, nil
	case <-ctx.Done():
		h.s.emitUI(UICommand{Kind: "dialog-close", ID: id})
		return nil, false, context.Cause(ctx)
	}
}

// QueryGuard donne la borne des sources de l'hôte de l'application
// (moteur web) ; nil sur le bureau.
func (h host) QueryGuard() *query.Guard {
	if g, ok := h.s.sys().(query.Guarded); ok {
		return g.QueryGuard()
	}
	return nil
}

func (h host) Notify(msg string) { h.s.emitUI(UICommand{Kind: "notify", Message: msg}) }

func (h host) Print(line string) {
	h.s.sys().Emit(PrintEvent, line)
}

func (h host) NewDoc(ctx context.Context) error {
	_, err := h.s.New(ctx)
	if err == nil {
		h.s.changed(ctx)
	}
	return err
}

func (h host) OpenDoc(ctx context.Context, path string) error {
	if path == "" {
		return msg.New("host.missingPath", nil)
	}
	res, err := h.s.Open(ctx, path)
	if err != nil {
		return err
	}
	if res.Recoverable != nil {
		return msg.New("host.sessionPending", nil)
	}
	if res.Damaged != nil {
		return errors.New(res.Damaged.Message + " ; ouvrez-le dans Lattis pour en récupérer les données")
	}
	if res.Locked != nil {
		return errors.New(res.Locked.Message + " : ouvrez-le dans Lattis pour choisir")
	}
	h.s.changed(ctx)
	return nil
}

func (h host) SaveDoc(ctx context.Context, path string) error {
	d, err := h.s.current()
	if err != nil {
		return err
	}
	if err := h.s.storeConversation(ctx, d); err != nil {
		return err
	}
	if err := saveDoc(ctx, d, path); err != nil {
		return err
	}
	addRecentDoc(d)
	h.s.changed(ctx)
	return nil
}

// SetSelection reçoit la feuille affichée, la sélection de la grille et la
// vue (l'interface l'appelle à chaque changement) : ui.selection la lit, et
// les évènements sheet et selection du classeur en partent (events.go).
func (s *DocumentService) SetSelection(sel api.Selection) {
	s.ui.mu.Lock()
	prev := s.ui.sel
	s.ui.sel = sel
	s.ui.mu.Unlock()
	s.ev.mu.Lock()
	known := s.ev.last
	s.ev.last = sel
	s.ev.mu.Unlock()
	// La sélection posée par un script (ui.select) a déjà remplacé ui.sel :
	// rapportée par l'interface, elle ne déclenche rien.
	if prev != sel {
		s.selectionChanged(known, sel)
	}
}

// DialogAnswer donne la réponse à une boîte de dialogue (ok faux : fermée
// sans valider).
func (s *DocumentService) DialogAnswer(id int, values map[string]any, ok bool) {
	s.ui.mu.Lock()
	ch := s.ui.dialogs[id]
	s.ui.mu.Unlock()
	if ch != nil {
		select {
		case ch <- dialogAnswer{values, ok}:
		default:
		}
	}
}

// Call exécute une commande de l'API (internal/api) depuis l'interface.
func (s *DocumentService) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	return api.Invoke(ctx, host{s}, name, args)
}
