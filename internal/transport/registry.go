// Package transport sert les services de internal/bridge hors de Wails :
// appels JSON-RPC 2.0 sur WebSocket (version web, spécification
// collaboration § 12.1) et évènements poussés.
//
// Les méthodes sont celles que Wails lie : méthodes exportées des services,
// sauf ServiceName, ServiceStartup, ServiceShutdown et ServeHTTP. Une
// méthode se désigne par l'identifiant des liaisons générées
// (frontend/bindings, $Call.ByID), calculé comme Wails (FNV-1a 32 bits du
// nom complet), ou par son nom : complet
// (« github.com/lalamefine/lattis/internal/bridge.DocumentService.Open »)
// ou court (« DocumentService.Open »).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"strconv"
)

// internal : méthodes des services que Wails ne lie pas.
var internal = map[string]bool{
	"ServiceName":     true,
	"ServiceStartup":  true,
	"ServiceShutdown": true,
	"ServeHTTP":       true,
}

var (
	ctxType = reflect.TypeFor[context.Context]()
	errType = reflect.TypeFor[error]()
)

// Method est une méthode liée.
type Method struct {
	ID       uint32
	FQN      string // chemin du paquet.Type.Méthode
	Short    string // Type.Méthode
	fn       reflect.Value
	in       []reflect.Type // paramètres, sans le contexte
	needsCtx bool
}

// Registry répertorie les méthodes des services.
type Registry struct {
	byID   map[uint32]*Method
	byName map[string]*Method
}

// ID calcule l'identifiant d'une méthode comme le générateur de liaisons de
// Wails.
func ID(fqn string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(fqn))
	return h.Sum32()
}

// NewRegistry répertorie les méthodes exportées des services (pointeurs
// vers des types nommés).
func NewRegistry(services ...any) (*Registry, error) {
	r := &Registry{byID: map[uint32]*Method{}, byName: map[string]*Method{}}
	for _, svc := range services {
		v := reflect.ValueOf(svc)
		t := v.Type()
		if t.Kind() != reflect.Pointer || t.Elem().Name() == "" {
			return nil, fmt.Errorf("service %T : pointeur vers un type nommé attendu", svc)
		}
		named := t.Elem()
		for i := range t.NumMethod() {
			m := t.Method(i)
			if internal[m.Name] {
				continue
			}
			fn := v.Method(i)
			ft := fn.Type()
			bm := &Method{
				FQN:   named.PkgPath() + "." + named.Name() + "." + m.Name,
				Short: named.Name() + "." + m.Name,
				fn:    fn,
			}
			bm.ID = ID(bm.FQN)
			for j := range ft.NumIn() {
				p := ft.In(j)
				if j == 0 && p.Implements(ctxType) && ctxType.AssignableTo(p) {
					bm.needsCtx = true
					continue
				}
				bm.in = append(bm.in, p)
			}
			if _, dup := r.byID[bm.ID]; dup {
				return nil, fmt.Errorf("identifiant %d en double (%s)", bm.ID, bm.FQN)
			}
			r.byID[bm.ID] = bm
			r.byName[bm.FQN] = bm
			r.byName[bm.Short] = bm
		}
	}
	return r, nil
}

// Lookup trouve une méthode par identifiant (décimal) ou par nom.
func (r *Registry) Lookup(name string) (*Method, bool) {
	if id, err := strconv.ParseUint(name, 10, 32); err == nil {
		m, ok := r.byID[uint32(id)]
		return m, ok
	}
	m, ok := r.byName[name]
	return m, ok
}

// Methods liste les méthodes (pour le diagnostic).
func (r *Registry) Methods() []*Method {
	out := make([]*Method, 0, len(r.byID))
	for _, m := range r.byID {
		out = append(out, m)
	}
	return out
}

// Error est une erreur d'appel, rendue au client dans le champ error de la
// réponse JSON-RPC.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	// Data.kind reprend les genres d'erreur du runtime Wails
	// (ReferenceError, TypeError, RuntimeError).
	Data *ErrorData `json:"data,omitempty"`
	// err : l'erreur rendue par la méthode, que le Hub traduit
	// (Hub.Translate : message de internal/msg dans la langue de la session).
	err error
}

// Unwrap donne l'erreur rendue par la méthode (nil pour une erreur du transport).
func (e *Error) Unwrap() error { return e.err }

// ErrorData précise une erreur.
type ErrorData struct {
	Kind string `json:"kind"`
	// Cause : précisions d'un message traduit (bridge.ErrorCause).
	Cause any `json:"cause,omitempty"`
}

func (e *Error) Error() string { return e.Message }

// Codes JSON-RPC 2.0.
const (
	CodeParse          = -32700
	CodeInvalidRequest = -32600
	CodeNoMethod       = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
	CodeServer         = -32000 // erreur rendue par la méthode
)

func callError(code int, kind, msg string) *Error {
	return &Error{Code: code, Message: msg, Data: &ErrorData{Kind: kind}}
}

// Call appelle la méthode avec des paramètres positionnels (tableau JSON).
// Le résultat suit Wails : nil sans valeur, la valeur seule, ou un tableau
// s'il y en a plusieurs ; une erreur non nulle rendue l'emporte.
func (m *Method) Call(ctx context.Context, params []json.RawMessage) (result any, err error) {
	defer func() {
		if p := recover(); p != nil {
			result, err = nil, callError(CodeInternal, "RuntimeError", fmt.Sprintf("%s : panique : %v", m.FQN, p))
		}
	}()
	if len(params) != len(m.in) {
		return nil, callError(CodeInvalidParams, "TypeError", fmt.Sprintf("%s attend %d paramètre(s), reçu %d", m.Short, len(m.in), len(params)))
	}
	args := make([]reflect.Value, 0, len(m.in)+1)
	if m.needsCtx {
		args = append(args, reflect.ValueOf(ctx))
	}
	for i, raw := range params {
		v := reflect.New(m.in[i])
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, v.Interface()); err != nil {
				return nil, callError(CodeInvalidParams, "TypeError", fmt.Sprintf("%s : paramètre n° %d illisible : %v", m.Short, i+1, err))
			}
		}
		args = append(args, v.Elem())
	}
	var out []any
	var errs []error
	for _, v := range m.fn.Call(args) {
		if v.Type() == errType {
			if !v.IsNil() {
				errs = append(errs, v.Interface().(error))
			}
			continue
		}
		out = append(out, v.Interface())
	}
	if len(errs) > 0 {
		e := errors.Join(errs...)
		var ce *Error
		if errors.As(e, &ce) {
			return nil, ce
		}
		ce = callError(CodeServer, "RuntimeError", e.Error())
		ce.err = e
		return nil, ce
	}
	switch len(out) {
	case 0:
		return nil, nil
	case 1:
		return out[0], nil
	}
	return out, nil
}
