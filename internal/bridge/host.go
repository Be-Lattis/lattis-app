package bridge

import (
	"context"
	"errors"
	"sync"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// UIEvent porte une commande de l'interface venue d'un script ou du serveur
// MCP : sélection, changement de vue, boîte de dialogue, message.
const UIEvent = "ui:command"

func init() {
	application.RegisterEvent[UICommand](UIEvent)
}

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

func emitUI(c UICommand) bool {
	app := application.Get()
	if app == nil {
		return false
	}
	app.Event.Emit(UIEvent, c)
	return true
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
	emitUI(UICommand{Kind: "select", Selection: &sel})
	return nil
}

func (h host) ShowView(_ context.Context, view string) error {
	h.s.ui.mu.Lock()
	h.s.ui.sel.View = view
	h.s.ui.mu.Unlock()
	emitUI(UICommand{Kind: "view", View: view})
	return nil
}

var errNoUI = errors.New("aucune fenêtre pour afficher la boîte de dialogue")

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
	if !emitUI(UICommand{Kind: "dialog", Dialog: &DialogRequest{ID: id, Dialog: d}}) {
		return nil, false, errNoUI
	}
	select {
	case a := <-ch:
		return a.values, a.ok, nil
	case <-ctx.Done():
		emitUI(UICommand{Kind: "dialog-close", ID: id})
		return nil, false, context.Cause(ctx)
	}
}

func (h host) Notify(msg string) { emitUI(UICommand{Kind: "notify", Message: msg}) }

func (h host) Print(line string) {
	if app := application.Get(); app != nil {
		app.Event.Emit(PrintEvent, line)
	}
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
		return errors.New("chemin du classeur manquant")
	}
	res, err := h.s.Open(ctx, path)
	if err != nil {
		return err
	}
	if res.Recoverable != nil {
		return errors.New("une session interrompue de ce classeur attend d'être récupérée : ouvrez-le dans Lattis")
	}
	h.s.changed(ctx)
	return nil
}

func (h host) SaveDoc(ctx context.Context, path string) error {
	d, err := h.s.current()
	if err != nil {
		return err
	}
	if err := d.Save(ctx, path); err != nil {
		return err
	}
	addRecent(d.Path())
	h.s.changed(ctx)
	return nil
}

// SetSelection reçoit la feuille affichée, la sélection de la grille et la
// vue (l'interface l'appelle à chaque changement) : ui.selection la lit.
func (s *DocumentService) SetSelection(sel api.Selection) {
	s.ui.mu.Lock()
	s.ui.sel = sel
	s.ui.mu.Unlock()
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
