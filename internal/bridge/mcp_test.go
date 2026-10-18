package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/mcp"
)

// emitter est un hôte de bureau qui garde les évènements émis ; onEmit
// réagit à un évènement (réponse à une demande d'accord).
type emitter struct {
	NoHost
	mu     sync.Mutex
	events []emitted
	onEmit func(event string, data any)
}

type emitted struct {
	event string
	data  any
}

func (e *emitter) Emit(event string, data any) bool {
	e.mu.Lock()
	e.events = append(e.events, emitted{event, data})
	f := e.onEmit
	e.mu.Unlock()
	if f != nil {
		f(event, data)
	}
	return true
}

func (e *emitter) activities() []MCPActivity {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []MCPActivity
	for _, x := range e.events {
		if a, ok := x.data.(MCPActivity); ok && x.event == MCPActivityEvent {
			out = append(out, a)
		}
	}
	return out
}

// mcpHome redirige le dossier de l'utilisateur (réglages MCP, Claude
// Desktop, Claude Code) vers un dossier temporaire.
func mcpHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return dir
}

func mcpService(t *testing.T) (*DocumentService, *emitter, *mcp.Server) {
	t.Helper()
	mcpHome(t)
	ctx := context.Background()
	s := &DocumentService{}
	e := &emitter{}
	UseHost(e, nil, s)
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.CloseDocument(ctx); s.stopMCP() })
	return s, e, s.mcpServer()
}

// tool appelle un outil comme le ferait le client nommé client.
func tool(t *testing.T, srv *mcp.Server, client, name string, args map[string]any) (string, bool) {
	t.Helper()
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	msg := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":%s}`, params)
	out := srv.Handle(mcp.WithClient(context.Background(), client), []byte(msg))
	var r struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &r); err != nil || len(r.Result.Content) == 0 {
		t.Fatalf("%s : %v", out, err)
	}
	return r.Result.Content[0].Text, r.Result.IsError
}

func sheetNames(t *testing.T, s *DocumentService) string {
	t.Helper()
	v, err := s.Call(context.Background(), "sheets.list", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// Les écritures d'une série forment une seule étape « Claude Desktop (MCP) »,
// que MCPUndoSeries annule ; chaque appel est montré (mcp:activity).
func TestMCPSeriesUndo(t *testing.T) {
	s, e, srv := mcpService(t)
	ctx := context.Background()
	for _, n := range []string{"Ventes", "Achats"} {
		if txt, isErr := tool(t, srv, "claude-ai", "sheets_add", map[string]any{"name": n}); isErr {
			t.Fatal(txt)
		}
	}
	tool(t, srv, "claude-ai", "sheets_list", map[string]any{})
	if _, isErr := tool(t, srv, "claude-ai", "sheets_rename", map[string]any{"sheet": "Nulle", "name": "x"}); !isErr {
		t.Fatal("erreur non rendue")
	}
	st, _ := s.State(ctx)
	if st.Undo != "Claude Desktop (MCP)" {
		t.Fatalf("étape d'annulation %q", st.Undo)
	}
	acts := e.activities()
	if len(acts) != 4 {
		t.Fatalf("activités : %+v", acts)
	}
	a := acts[0]
	if a.Series != 1 || a.Client != "Claude Desktop" || a.Tool != "sheets_add" || !a.Write || a.Error || a.Target != "Ventes" || !strings.Contains(a.Label, "« Ventes »") || a.At == "" {
		t.Fatalf("première activité : %+v", a)
	}
	if acts[2].Write || acts[2].Series != 1 {
		t.Fatalf("lecture : %+v", acts[2])
	}
	if !acts[3].Error || !strings.Contains(acts[3].Detail, "Nulle") {
		t.Fatalf("erreur : %+v", acts[3])
	}
	if c, _ := s.MCPClients(); c.Connected != "Claude Desktop" {
		t.Fatalf("client connecté %q", c.Connected)
	}
	if list := s.MCPSeries(ctx); len(list) != 1 || !list[0].Undo || list[0].Writes != 1 {
		t.Fatalf("séries : %+v", list)
	}
	if _, err := s.MCPUndoSeries(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if n := sheetNames(t, s); strings.Contains(n, "Ventes") || strings.Contains(n, "Achats") {
		t.Fatalf("feuilles après annulation : %s", n)
	}
	if _, err := s.MCPUndoSeries(ctx, 1); err == nil || !strings.Contains(err.Error(), "déjà") {
		t.Fatalf("deuxième annulation : %v", err)
	}
	// La série annulée est close : l'appel suivant en ouvre une autre.
	tool(t, srv, "claude-ai", "sheets_add", map[string]any{"name": "Stocks"})
	if acts := e.activities(); acts[len(acts)-1].Series != 2 {
		t.Fatalf("série après annulation : %+v", acts[len(acts)-1])
	}
}

// Une action de l'utilisateur après la série bloque son annulation d'un
// clic ; une pause de plus de deux minutes, ou un autre client, ouvre une
// nouvelle série.
func TestMCPSeriesBlockedAndGap(t *testing.T) {
	s, e, srv := mcpService(t)
	ctx := context.Background()
	tool(t, srv, "claude-code", "sheets_add", map[string]any{"name": "Ventes"})
	if _, err := s.Call(ctx, "sheets.add", map[string]any{"name": "Moi"}); err != nil {
		t.Fatal(err)
	}
	list := s.MCPSeries(ctx)
	if len(list) != 1 || list[0].Undo || !strings.Contains(list[0].Reason, "D’autres modifications") {
		t.Fatalf("séries : %+v", list)
	}
	if _, err := s.MCPUndoSeries(ctx, 1); err == nil {
		t.Fatal("annulation acceptée malgré une action suivante")
	}
	// Reprise de la série après l'action de l'utilisateur : nouvelle étape
	// du même nom, toujours suivie.
	tool(t, srv, "claude-code", "sheets_add", map[string]any{"name": "Achats"})
	s.mcp.mu.Lock()
	ser := s.mcp.w.series[1]
	n := len(ser.seqs)
	ser.last = time.Now().Add(-3 * time.Minute)
	s.mcp.mu.Unlock()
	if n != 2 {
		t.Fatalf("étapes de la série : %d", n)
	}
	tool(t, srv, "claude-code", "sheets_list", map[string]any{})
	tool(t, srv, "claude-ai", "sheets_list", map[string]any{})
	acts := e.activities()
	if acts[2].Series != 2 || acts[3].Series != 3 || acts[3].Client != "Claude Desktop" {
		t.Fatalf("séries : %+v", acts)
	}
	if _, err := s.MCPUndoSeries(ctx, 2); err == nil || !strings.Contains(err.Error(), "Aucune modification") {
		t.Fatalf("série sans écriture : %v", err)
	}
}

// mcp.confirmWrites : chaque écriture attend MCPAnswer ; refus et délai
// dépassé rendent une erreur d'outil, la lecture passe sans demande.
func TestMCPConfirmWrites(t *testing.T) {
	s, e, srv := mcpService(t)
	old, oldT := mcpConfirmWrites, mcpConfirmTimeout
	t.Cleanup(func() { mcpConfirmWrites, mcpConfirmTimeout = old, oldT })
	mcpConfirmWrites = func() bool { return true }
	answer := true
	var asked []MCPConfirm
	e.onEmit = func(event string, data any) {
		if event == MCPConfirmEvent {
			q := data.(MCPConfirm)
			asked = append(asked, q)
			if answer {
				go s.MCPAnswer(q.ID, true)
			} else {
				go s.MCPAnswer(q.ID, false)
			}
		}
	}
	if txt, isErr := tool(t, srv, "claude-ai", "sheets_add", map[string]any{"name": "Oui"}); isErr {
		t.Fatal(txt)
	}
	answer = false
	txt, isErr := tool(t, srv, "claude-ai", "sheets_add", map[string]any{"name": "Non"})
	if !isErr || txt != "refusé par l'utilisateur dans Lattis" {
		t.Fatalf("refus : %q", txt)
	}
	tool(t, srv, "claude-ai", "sheets_list", map[string]any{})
	if len(asked) != 2 || asked[0].Client != "Claude Desktop" || !strings.Contains(asked[0].Label, "« Oui »") {
		t.Fatalf("demandes : %+v", asked)
	}
	if n := sheetNames(t, s); !strings.Contains(n, "Oui") || strings.Contains(n, "Non") {
		t.Fatalf("feuilles : %s", n)
	}
	// Pas de réponse : refus après le délai, et la demande est close.
	e.onEmit = nil
	mcpConfirmTimeout = 20 * time.Millisecond
	if txt, isErr := tool(t, srv, "claude-ai", "sheets_add", map[string]any{"name": "Tard"}); !isErr || !strings.Contains(txt, "n'a pas répondu") {
		t.Fatalf("délai : %q", txt)
	}
	e.mu.Lock()
	last := e.events[len(e.events)-2]
	e.mu.Unlock()
	if last.event != MCPConfirmDoneEvent {
		t.Fatalf("demande non close : %+v", last)
	}
	s.MCPAnswer(999, true) // demande inconnue : sans effet
	if c, _ := s.MCPClients(); !c.ConfirmWrites {
		t.Fatal("réglage non rendu par MCPClients")
	}
}

func TestMCPClientsRefusedInWeb(t *testing.T) {
	s := &DocumentService{}
	UseHost(webLike{}, nil, s)
	if _, err := s.MCPClients(); !errors.Is(err, errNoMCPWeb) {
		t.Fatalf("MCPClients en web : %v", err)
	}
	if _, err := s.MCPConfigureDesktop(); !errors.Is(err, errNoMCPWeb) {
		t.Fatalf("MCPConfigureDesktop en web : %v", err)
	}
	if _, err := s.MCPConfigureCode(context.Background()); !errors.Is(err, errNoMCPWeb) {
		t.Fatalf("MCPConfigureCode en web : %v", err)
	}
}

func TestMCPConfigureDesktop(t *testing.T) {
	s, e, _ := mcpService(t)
	oldExe := mcpExecutable
	t.Cleanup(func() { mcpExecutable = oldExe })
	mcpExecutable = func() string { return "/Applications/Lattis.app/Contents/MacOS/Lattis" }
	c, err := s.MCPClients()
	if err != nil || c.DesktopInstalled || c.DesktopConfigured {
		t.Fatalf("avant : %+v, %v", c, err)
	}
	os.MkdirAll(mcp.DesktopDir(), 0o700)
	os.WriteFile(c.DesktopConfig, []byte(`{"mcpServers":{"autre":{"command":"x"}}}`), 0o600)
	if c, err = s.MCPConfigureDesktop(); err != nil {
		t.Fatal(err)
	}
	if !c.DesktopInstalled || !c.DesktopConfigured {
		t.Fatalf("après : %+v", c)
	}
	if st, _ := s.MCPState(); !st.Enabled || !st.Running {
		t.Fatalf("serveur MCP non démarré : %+v", st)
	}
	// De bout en bout : le serveur démarré reçoit un appel HTTP de Claude
	// Code (session ouverte par initialize) et le montre.
	st, _ := s.MCPState()
	post := func(body, sid string) *http.Response {
		req, _ := http.NewRequest("POST", st.URL, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+st.Token)
		req.Header.Set("Content-Type", "application/json")
		if sid != "" {
			req.Header.Set("Mcp-Session-Id", sid)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	sid := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"claude-code"}}}`, "").Header.Get("Mcp-Session-Id")
	post(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"sheets_add","arguments":{"name":"Distante"}}}`, sid)
	if acts := e.activities(); len(acts) != 1 || acts[0].Client != "Claude Code" || acts[0].Error {
		t.Fatalf("activité HTTP : %+v", acts)
	}
	if c, _ := s.MCPClients(); c.Connected != "Claude Code" {
		t.Fatalf("connecté : %q", c.Connected)
	}
	b, _ := os.ReadFile(c.DesktopConfig)
	if !strings.Contains(string(b), `"autre"`) || !strings.Contains(string(b), `MacOS/Lattis"`) {
		t.Fatalf("configuration : %s", b)
	}
}

// Claude Code : un faux « claude » dans ~/.local/bin (hors du PATH) note
// ses arguments et range l'entrée comme le vrai dans ~/.claude.json.
func TestMCPConfigureCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("faux claude en script shell")
	}
	s, _, _ := mcpService(t)
	home, _ := os.UserHomeDir()
	t.Setenv("PATH", "/usr/bin:/bin")
	if c, _ := s.MCPClients(); c.CodeFound && !strings.HasPrefix(mcp.FindClaude(), "/usr") {
		t.Fatalf("claude trouvé : %s", mcp.FindClaude())
	}
	if _, err := s.MCPConfigureCode(context.Background()); err != nil && !strings.Contains(err.Error(), "introuvable") && mcp.FindClaude() == "" {
		t.Fatalf("sans claude : %v", err)
	}
	log := filepath.Join(home, "args.log")
	script := `#!/bin/sh
echo "$@" >> "` + log + `"
if [ "$2" = "add" ]; then
  printf '{"mcpServers":{"lattis":{"type":"http","url":"%s","headers":{"Authorization":"%s"}}}}' "$8" "${10#Authorization: }" > "` + filepath.Join(home, ".claude.json") + `"
fi
`
	bin := filepath.Join(home, ".local", "bin", "claude")
	os.MkdirAll(filepath.Dir(bin), 0o755)
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	c, err := s.MCPConfigureCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !c.CodeFound || !c.CodeConfigured {
		t.Fatalf("après : %+v", c)
	}
	b, _ := os.ReadFile(log)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	st, _ := s.MCPState()
	if len(lines) != 2 || lines[0] != "mcp remove --scope user lattis" || lines[1] != "mcp add --transport http --scope user lattis "+st.URL+" --header Authorization: Bearer "+st.Token {
		t.Fatalf("appels de claude : %q", lines)
	}
	if !strings.Contains(c.CodeCommand, st.URL) || !strings.Contains(c.CodeCommand, "--scope user") {
		t.Fatalf("commande à copier : %s", c.CodeCommand)
	}
}
