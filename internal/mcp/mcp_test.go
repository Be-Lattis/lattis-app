package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/slitable/internal/api"
	"github.com/lalamefine/slitable/internal/doc"
)

type host struct {
	d   *doc.Document
	sel api.Selection
}

func (h *host) Doc() (*doc.Document, error)                     { return h.d, nil }
func (h *host) Changed(context.Context)                         {}
func (h *host) Selection() api.Selection                        { return h.sel }
func (h *host) Select(_ context.Context, s api.Selection) error { h.sel = s; return nil }
func (h *host) ShowView(context.Context, string) error          { return nil }
func (h *host) Notify(string)                                   {}
func (h *host) Print(string)                                    {}
func (h *host) NewDoc(context.Context) error                    { return nil }
func (h *host) OpenDoc(context.Context, string) error           { return nil }
func (h *host) SaveDoc(context.Context, string) error           { return nil }
func (h *host) Dialog(context.Context, api.Dialog) (map[string]any, bool, error) {
	return nil, false, nil
}

func newServer(t *testing.T) *Server {
	t.Helper()
	d, err := doc.New(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return &Server{Host: &host{d: d, sel: api.Selection{Sheet: 1}}, Version: "test"}
}

func rpc(t *testing.T, s *Server, msg string) map[string]any {
	t.Helper()
	out := s.Handle(context.Background(), []byte(msg))
	var r map[string]any
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("%s : %v", out, err)
	}
	return r
}

func TestProtocol(t *testing.T) {
	s := newServer(t)
	r := rpc(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	if v := r["result"].(map[string]any)["protocolVersion"]; v != "2025-03-26" {
		t.Fatalf("version %v", v)
	}
	if out := s.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); out != nil {
		t.Fatalf("réponse à une notification : %s", out)
	}
	r = rpc(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools := r["result"].(map[string]any)["tools"].([]any)
	names := map[string]map[string]any{}
	for _, x := range tools {
		m := x.(map[string]any)
		names[m["name"].(string)] = m
	}
	for _, want := range []string{"sheets_add", "ui_select", "ui_dialog", "db_query", "buttons_create", "scripts_run", "doc_undo"} {
		if names[want] == nil {
			t.Errorf("outil %s absent", want)
		}
	}
	req := names["sheets_rename"]["inputSchema"].(map[string]any)["required"].([]any)
	if len(req) != 2 {
		t.Fatalf("paramètres requis %v", req)
	}
	r = rpc(t, s, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"sheets_add","arguments":{"name":"Ventes"}}}`)
	res := r["result"].(map[string]any)
	if res["isError"] != false || !strings.Contains(res["content"].([]any)[0].(map[string]any)["text"].(string), "Ventes") {
		t.Fatalf("appel %v", res)
	}
	r = rpc(t, s, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"sheets_rename","arguments":{"sheet":"Nulle","name":"x"}}}`)
	if r["result"].(map[string]any)["isError"] != true {
		t.Fatalf("erreur non signalée : %v", r)
	}
	r = rpc(t, s, `{"jsonrpc":"2.0","id":5,"method":"inconnue"}`)
	if r["error"] == nil {
		t.Fatal("méthode inconnue acceptée")
	}
}

func TestHTTPAndProxy(t *testing.T) {
	s := newServer(t)
	ts := httptest.NewServer(Handler(s, "secret"))
	defer ts.Close()
	post := func(token, origin string) int {
		req, _ := http.NewRequest("POST", ts.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if c := post("faux", ""); c != http.StatusUnauthorized {
		t.Fatalf("mauvais jeton : %d", c)
	}
	if c := post("secret", "https://exemple.com"); c != http.StatusForbidden {
		t.Fatalf("origine étrangère : %d", c)
	}
	if c := post("secret", ""); c != http.StatusOK {
		t.Fatalf("requête valide : %d", c)
	}

	// Relais stdio : réglages pointant vers le serveur de test.
	dir := t.TempDir()
	settingsFile = func() (string, error) { return filepath.Join(dir, "mcp.json"), nil }
	port := ts.Listener.Addr().(interface{ String() string }).String()
	p := port[strings.LastIndexByte(port, ':')+1:]
	var n int
	for _, c := range p {
		n = n*10 + int(c-'0')
	}
	var out bytes.Buffer
	in := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"sheets_list","arguments":{}}}` + "\n"
	if err := SaveSettings(Settings{Enabled: false, Port: n, Token: "secret"}); err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/mcp"
		Handler(s, "secret").ServeHTTP(w, r)
	})
	Proxy(strings.NewReader(in), &out)
	if !strings.Contains(out.String(), "désactivé") {
		t.Fatalf("serveur désactivé : %s", out.String())
	}
	SaveSettings(Settings{Enabled: true, Port: n, Token: "secret"})
	out.Reset()
	Proxy(strings.NewReader(in), &out)
	if !strings.Contains(out.String(), "Feuille1") {
		t.Fatalf("relais : %s", out.String())
	}
}
