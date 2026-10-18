package transport

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

// Messages (JSON-RPC 2.0, un message par trame texte) :
//
//	client → moteur  {"jsonrpc":"2.0","id":1,"method":"2316013313","params":[…]}
//	moteur → client  {"jsonrpc":"2.0","id":1,"result":…}
//	                 {"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"…","data":{"kind":"RuntimeError"}}}
//	client → moteur  {"jsonrpc":"2.0","method":"$/cancel","params":{"id":1}}   (annule l'appel 1)
//	moteur → client  {"jsonrpc":"2.0","method":"event","params":{"name":"doc:changed","data":…}}
//
// method est l'identifiant décimal des liaisons générées ou le nom de la
// méthode (voir Registry.Lookup) ; params, un tableau positionnel (absent :
// aucun paramètre).

// Version est la version de JSON-RPC.
const Version = "2.0"

// EventMethod est la méthode des notifications d'évènements poussées.
const EventMethod = "event"

// CancelMethod annule un appel en cours.
const CancelMethod = "$/cancel"

// MaxMessage borne la taille d'un message reçu (collage d'un gros bloc de
// cellules, page HTML…).
const MaxMessage = 64 << 20

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  json.RawMessage `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
	Error   *Error          `json:"error,omitempty"`
}

type errResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   *Error          `json:"error"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// Event est un évènement poussé au client.
type Event struct {
	Name string `json:"name"`
	Data any    `json:"data"`
}

// Hub relie un moteur à ses connexions WebSocket (plusieurs onglets du même
// utilisateur sur la même session) : il sert les appels et diffuse les
// évènements.
type Hub struct {
	reg *Registry
	// Accept règle l'acceptation des connexions (origines permises…) ; nil :
	// même origine seulement.
	Accept *websocket.AcceptOptions
	// OnIdle est appelé quand la dernière connexion se ferme (le moteur
	// peut alors annuler ce qui attend l'interface).
	OnIdle func()

	mu    sync.Mutex
	conns map[*conn]struct{}
}

// NewHub crée un hub pour les méthodes de reg.
func NewHub(reg *Registry) *Hub {
	return &Hub{reg: reg, conns: map[*conn]struct{}{}}
}

type conn struct {
	ws     *websocket.Conn
	wmu    sync.Mutex
	ctx    context.Context
	mu     sync.Mutex
	cancel map[string]context.CancelFunc // appels en cours, par id
}

func (c *conn) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.ws.Write(c.ctx, websocket.MessageText, b)
}

// Connected dit si au moins un client est connecté.
func (h *Hub) Connected() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.conns) > 0
}

// Emit pousse un évènement à tous les clients ; faux si aucun n'est
// connecté.
func (h *Hub) Emit(name string, data any) bool {
	h.mu.Lock()
	cs := make([]*conn, 0, len(h.conns))
	for c := range h.conns {
		cs = append(cs, c)
	}
	h.mu.Unlock()
	msg := notification{JSONRPC: Version, Method: EventMethod, Params: Event{Name: name, Data: data}}
	sent := false
	for _, c := range cs {
		if err := c.send(msg); err == nil {
			sent = true
		}
	}
	return sent
}

// ServeHTTP accepte une connexion WebSocket et la sert jusqu'à sa fermeture.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, h.Accept)
	if err != nil {
		return
	}
	ws.SetReadLimit(MaxMessage)
	h.Serve(r.Context(), ws)
}

// Serve sert une connexion déjà acceptée.
func (h *Hub) Serve(ctx context.Context, ws *websocket.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	c := &conn{ws: ws, ctx: ctx, cancel: map[string]context.CancelFunc{}}
	h.mu.Lock()
	h.conns[c] = struct{}{}
	h.mu.Unlock()
	defer func() {
		cancel() // annule les appels en cours de cette connexion
		h.mu.Lock()
		delete(h.conns, c)
		idle := len(h.conns) == 0
		h.mu.Unlock()
		ws.Close(websocket.StatusNormalClosure, "")
		if idle && h.OnIdle != nil {
			h.OnIdle()
		}
	}()
	for {
		typ, b, err := ws.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			continue
		}
		var req request
		if err := json.Unmarshal(b, &req); err != nil {
			c.send(errResponse{JSONRPC: Version, ID: json.RawMessage("null"), Error: &Error{Code: CodeParse, Message: err.Error()}})
			continue
		}
		h.handle(c, req)
	}
}

func (h *Hub) handle(c *conn, req request) {
	var method string
	if err := json.Unmarshal(req.Method, &method); err != nil {
		// identifiant numérique
		var id uint32
		if err2 := json.Unmarshal(req.Method, &id); err2 != nil {
			h.reply(c, req.ID, nil, callError(CodeInvalidRequest, "TypeError", "méthode illisible"))
			return
		}
		method = jsonNumber(req.Method)
	}
	if method == CancelMethod {
		var p struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(req.Params, &p) == nil {
			c.mu.Lock()
			if f := c.cancel[string(p.ID)]; f != nil {
				f()
			}
			c.mu.Unlock()
		}
		return
	}
	m, ok := h.reg.Lookup(method)
	if !ok {
		h.reply(c, req.ID, nil, callError(CodeNoMethod, "ReferenceError", "méthode inconnue : "+method))
		return
	}
	var params []json.RawMessage
	if len(req.Params) > 0 && string(req.Params) != "null" {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			h.reply(c, req.ID, nil, callError(CodeInvalidParams, "TypeError", "params : tableau attendu"))
			return
		}
	}
	ctx, cancel := context.WithCancel(c.ctx)
	key := string(req.ID)
	if len(req.ID) > 0 {
		c.mu.Lock()
		c.cancel[key] = cancel
		c.mu.Unlock()
	}
	// Chaque appel dans sa goroutine, comme Wails : un appel long (boîte
	// de dialogue d'un script, import) ne bloque pas les autres.
	go func() {
		defer func() {
			cancel()
			if len(req.ID) > 0 {
				c.mu.Lock()
				delete(c.cancel, key)
				c.mu.Unlock()
			}
		}()
		res, err := m.Call(ctx, params)
		if len(req.ID) == 0 {
			return // notification : pas de réponse
		}
		var ce *Error
		if err != nil && !errors.As(err, &ce) {
			ce = callError(CodeServer, "RuntimeError", err.Error())
		}
		h.reply(c, req.ID, res, ce)
	}()
}

func (h *Hub) reply(c *conn, id json.RawMessage, res any, e *Error) {
	if len(id) == 0 {
		return
	}
	var err error
	if e != nil {
		err = c.send(errResponse{JSONRPC: Version, ID: id, Error: e})
	} else {
		err = c.send(response{JSONRPC: Version, ID: id, Result: res})
	}
	if err != nil && c.ctx.Err() == nil {
		// Résultat impossible à encoder (valeur non JSON) : le dire au
		// client plutôt que de le laisser attendre.
		var je *json.UnsupportedValueError
		var jt *json.UnsupportedTypeError
		var jm *json.MarshalerError
		if errors.As(err, &je) || errors.As(err, &jt) || errors.As(err, &jm) {
			c.send(errResponse{JSONRPC: Version, ID: id, Error: callError(CodeInternal, "RuntimeError", "résultat illisible : "+err.Error())})
			return
		}
		slog.Debug("réponse non envoyée", "err", err)
	}
}

func jsonNumber(raw json.RawMessage) string {
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return string(raw)
}
