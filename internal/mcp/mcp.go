// Package mcp est le serveur MCP (Model Context Protocol) intégré à
// Lattis : un assistant (Claude Desktop, Claude Code…) pilote le classeur
// ouvert avec les mêmes commandes que les scripts Lua (internal/api).
//
// Le serveur n'écoute que sur 127.0.0.1, exige un jeton (en-tête
// Authorization: Bearer) et refuse les requêtes d'une page web (en-tête
// Origin étranger). Il parle JSON-RPC 2.0 en « Streamable HTTP » (POST /mcp,
// réponse JSON) ; les clients qui ne savent lancer qu'un programme en stdio
// passent par « lattis mcp » (Proxy), qui relaie vers l'application
// ouverte.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/api"
)

// Versions du protocole acceptées, la plus récente d'abord.
var versions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// Server répond aux messages MCP pour une application.
type Server struct {
	Host    api.Host
	Name    string
	Version string
	// Watch, s'il est donné, entoure chaque appel d'outil (fil d'activité,
	// accord de l'utilisateur, groupe d'annulation).
	Watch Watcher
	// Connect, s'il est donné, apprend le nom du client à chaque initialize.
	Connect func(client string)

	mu       sync.Mutex
	sessions map[string]string // Mcp-Session-Id → nom du client
}

type sessionKey struct{}

// sessionOut reçoit l'identifiant de session créé par initialize, que
// Handler renvoie dans l'en-tête Mcp-Session-Id.
type sessionOut struct{ id string }

// maxSessions borne les sessions gardées (un client qui se reconnecte en
// ouvre une nouvelle) : au-delà, la liste repart de zéro et un client
// oublié n'est plus nommé (ses appels marchent toujours).
const maxSessions = 64

// session rend le client d'une session connue ("" sinon).
func (s *Server) session(id string) string {
	if id == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}

func (s *Server) newSession(client string) string {
	id := newToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil || len(s.sessions) >= maxSessions {
		s.sessions = map[string]string{}
	}
	s.sessions[id] = client
	return id
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Codes d'erreur JSON-RPC.
const (
	codeParse    = -32700
	codeInvalid  = -32600
	codeMethod   = -32601
	codeParams   = -32602
	codeInternal = -32603
)

// Handle traite un message (ou un lot) et renvoie la réponse, nil pour une
// notification.
func (s *Server) Handle(ctx context.Context, msg []byte) []byte {
	msg = []byte(strings.TrimSpace(string(msg)))
	if len(msg) > 0 && msg[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(msg, &batch); err != nil {
			return encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, err.Error()}})
		}
		var out []json.RawMessage
		for _, m := range batch {
			if r := s.handleOne(ctx, m); r != nil {
				out = append(out, r)
			}
		}
		if len(out) == 0 {
			return nil
		}
		b, _ := json.Marshal(out)
		return b
	}
	return s.handleOne(ctx, msg)
}

func (s *Server) handleOne(ctx context.Context, msg []byte) []byte {
	var req request
	if err := json.Unmarshal(msg, &req); err != nil {
		return encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{codeParse, err.Error()}})
	}
	if req.Method == "" {
		if req.ID == nil {
			return nil // réponse du client à une requête du serveur : aucune n'est émise
		}
		return encode(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{codeInvalid, "méthode manquante"}})
	}
	res, rerr := s.dispatch(ctx, req)
	if req.ID == nil {
		return nil // notification
	}
	out := response{JSONRPC: "2.0", ID: req.ID, Result: res, Error: rerr}
	if rerr == nil && res == nil {
		out.Result = struct{}{}
	}
	return encode(out)
}

func encode(r response) []byte {
	b, err := json.Marshal(r)
	if err != nil {
		b, _ = json.Marshal(response{JSONRPC: "2.0", ID: r.ID, Error: &rpcError{codeInternal, err.Error()}})
	}
	return b
}

func (s *Server) dispatch(ctx context.Context, req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
			ClientInfo      struct {
				Name string `json:"name"`
			} `json:"clientInfo"`
		}
		_ = json.Unmarshal(req.Params, &p)
		client := p.ClientInfo.Name
		if client == "" {
			client = ClientOf(ctx)
		}
		if out, ok := ctx.Value(sessionKey{}).(*sessionOut); ok {
			out.id = s.newSession(client)
		}
		if s.Connect != nil && client != "" {
			s.Connect(client)
		}
		v := versions[0]
		for _, x := range versions {
			if x == p.ProtocolVersion {
				v = x
			}
		}
		return map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": s.name(), "version": s.Version},
			"instructions":    instructions,
		}, nil
	case "ping":
		return struct{}{}, nil
	case "tools/list":
		return map[string]any{"tools": Tools()}, nil
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{codeParams, err.Error()}
		}
		cmd := commandOf(p.Name)
		if cmd == nil {
			return nil, &rpcError{codeParams, "outil inconnu : " + p.Name}
		}
		// Un assistant ne fait pas répondre l'utilisateur à sa place : une
		// requête HTTP vers un hôte non autorisé est refusée (cmd_http.go).
		run := func(ctx context.Context) (any, error) {
			return api.Invoke(api.WithoutPrompt(ctx), s.Host, cmd.Name, p.Arguments)
		}
		var res any
		var err error
		if s.Watch != nil {
			res, err = s.Watch(ctx, Call{
				Client:  ClientOf(ctx),
				Tool:    p.Name,
				Command: cmd.Name,
				Label:   Label(cmd, p.Arguments),
				Write:   Writes(cmd),
				Target:  Target(p.Arguments),
				Args:    p.Arguments,
			}, run)
		} else {
			res, err = run(ctx)
		}
		if err != nil {
			return toolResult(err.Error(), true), nil
		}
		return toolResult(res, false), nil
	case "resources/list":
		return map[string]any{"resources": []any{}}, nil
	case "prompts/list":
		return map[string]any{"prompts": []any{}}, nil
	}
	if strings.HasPrefix(req.Method, "notifications/") {
		return nil, nil
	}
	return nil, &rpcError{codeMethod, "méthode inconnue : " + req.Method}
}

func (s *Server) name() string {
	if s.Name == "" {
		return "lattis"
	}
	return s.Name
}

const instructions = `Lattis est un tableur relationnel : chaque tableau d'une feuille est une table SQLite. ` +
	`Les feuilles se désignent par leur nom, les tableaux par leur nom, les colonnes par leur nom ; les lignes et colonnes de la feuille se comptent à partir de 1. ` +
	`Commencez par sheets_list, sheets_get et tables_list ; lisez avec db_query (SQL avec les noms affichés) ou tables_rows, écrivez avec cells_set, cells_paste, db_execute et les outils tables_*, columns_*. ` +
	`Chaque outil qui modifie le classeur forme une étape que l'utilisateur peut annuler (Ctrl+Z) ; scripts_run exécute du Lua avec les mêmes fonctions (db.query, sheets.add, ui.select…).`

func toolResult(v any, isErr bool) map[string]any {
	var text string
	switch x := v.(type) {
	case string:
		text = x
	case nil:
		text = "ok"
	default:
		b, err := json.MarshalIndent(v, "", " ")
		if err != nil {
			text, isErr = err.Error(), true
		} else {
			text = string(b)
		}
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}
}

// ToolName est le nom MCP d'une commande ("sheets.add" → "sheets_add").
func ToolName(cmd string) string { return strings.ReplaceAll(cmd, ".", "_") }

func commandOf(tool string) *api.Command {
	for _, c := range api.Commands() {
		if !c.NoMCP && ToolName(c.Name) == tool {
			return c
		}
	}
	return nil
}

// Tool est la description MCP d'un outil.
type Tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

// Tools décrit les commandes exposées en MCP.
func Tools() []Tool {
	var out []Tool
	for _, c := range api.Commands() {
		if c.NoMCP {
			continue
		}
		desc := c.Summary
		if c.Details != "" {
			desc += "\n\n" + c.Details
		}
		if c.Returns != "" {
			desc += "\n\nRenvoie : " + c.Returns
		}
		props := map[string]any{}
		required := []string{}
		for _, p := range c.Params {
			props[p.Name] = schema(p)
			if !p.Optional {
				required = append(required, p.Name)
			}
		}
		t := Tool{
			Name:        ToolName(c.Name),
			Description: desc,
			InputSchema: map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false},
			Annotations: map[string]any{"readOnlyHint": !c.Write && !strings.HasPrefix(c.Name, "doc.") && !strings.HasPrefix(c.Name, "http.") && c.Name != "buttons.run" && c.Name != "scripts.run"},
		}
		if strings.HasPrefix(c.Name, "http.") {
			t.Annotations["openWorldHint"] = true
		}
		if c.Label != "" {
			t.Title = c.Label
		}
		out = append(out, t)
	}
	return out
}

func schema(p api.Param) map[string]any {
	s := map[string]any{"description": p.Desc}
	switch p.Type {
	case api.Ref:
		s["type"] = []string{"string", "integer"}
	case api.Value:
		s["type"] = []string{"string", "number", "boolean"}
	case api.Array:
		s["type"] = "array"
		switch p.Items {
		case "":
		case api.Array:
			s["items"] = map[string]any{"type": "array"}
		default:
			s["items"] = map[string]any{"type": p.Items}
		}
		if p.Items == api.String {
			// Une valeur seule est aussi acceptée (tables_delete…).
			delete(s, "type")
			s["anyOf"] = []any{map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, map[string]any{"type": "string"}}
			delete(s, "items")
		}
	default:
		s["type"] = p.Type
	}
	return s
}

var _ = fmt.Sprint
