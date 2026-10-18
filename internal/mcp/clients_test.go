package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// home redirige le dossier de l'utilisateur et sa configuration vers un
// dossier temporaire.
func home(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "AppData", "Local"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return dir
}

func TestDesktopConfigPath(t *testing.T) {
	dir := home(t)
	want := map[string]string{
		"linux":   filepath.Join(dir, ".config", "Claude", "claude_desktop_config.json"),
		"darwin":  filepath.Join(dir, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		"windows": filepath.Join(dir, "AppData", "Roaming", "Claude", "claude_desktop_config.json"),
	}[runtime.GOOS]
	if want != "" && DesktopConfigPath() != want {
		t.Fatalf("chemin %q, attendu %q", DesktopConfigPath(), want)
	}
	if DesktopInstalled() {
		t.Fatal("Claude Desktop vu installé sans son dossier")
	}
	os.MkdirAll(DesktopDir(), 0o700)
	if !DesktopInstalled() {
		t.Fatal("dossier de Claude Desktop non vu")
	}
}

func TestConfigureDesktop(t *testing.T) {
	home(t)
	path := DesktopConfigPath()
	exe := filepath.Join(string(filepath.Separator), "opt", "Lattis", "lattis")

	// Fichier absent : créé.
	if err := ConfigureDesktop(path, exe); err != nil {
		t.Fatal(err)
	}
	if !DesktopConfigured(path, exe) {
		t.Fatal("non configuré après écriture")
	}
	if _, err := os.Stat(path + ".bak"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("copie .bak d'un fichier qui n'existait pas")
	}
	if DesktopConfigured(path, exe+"2") {
		t.Fatal("configuré pour un autre exécutable")
	}

	// Fichier existant : le reste est gardé, dans l'ordre, et copié en .bak.
	old := `{"theme":"dark","mcpServers":{"autre":{"command":"x","args":["y"],"env":{"K":"V"}},"lattis":{"command":"vieux"}},"zz":[1,2]}`
	os.Remove(path)
	os.WriteFile(path, []byte(old), 0o600)
	if err := ConfigureDesktop(path, exe); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	s := string(b)
	if !strings.Contains(s, "\n  \"theme\": \"dark\"") {
		t.Fatalf("pas indenté :\n%s", s)
	}
	if i, j, k := strings.Index(s, `"theme"`), strings.Index(s, `"mcpServers"`), strings.Index(s, `"zz"`); !(i < j && j < k) {
		t.Fatalf("ordre des clés perdu :\n%s", s)
	}
	if i, j := strings.Index(s, `"autre"`), strings.Index(s, `"lattis"`); i > j {
		t.Fatalf("ordre des serveurs perdu :\n%s", s)
	}
	var c map[string]any
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	srv := c["mcpServers"].(map[string]any)
	if srv["autre"].(map[string]any)["env"].(map[string]any)["K"] != "V" {
		t.Fatalf("autre serveur abîmé : %v", srv)
	}
	if l := srv["lattis"].(map[string]any); l["command"] != exe || l["args"].([]any)[0] != "mcp" {
		t.Fatalf("entrée lattis : %v", l)
	}
	if bak, _ := os.ReadFile(path + ".bak"); string(bak) != old {
		t.Fatalf(".bak : %s", bak)
	}
	if fi, _ := os.Stat(path); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("droits du fichier changés : %v", fi.Mode())
	}

	// JSON invalide : rien n'est écrit.
	os.WriteFile(path, []byte(`{cassé`), 0o600)
	if err := ConfigureDesktop(path, exe); err == nil {
		t.Fatal("JSON invalide écrasé")
	}
	if b, _ := os.ReadFile(path); string(b) != `{cassé` {
		t.Fatalf("fichier invalide modifié : %s", b)
	}
	// mcpServers à null : remplacé.
	os.WriteFile(path, []byte(`{"mcpServers":null}`), 0o600)
	if err := ConfigureDesktop(path, exe); err != nil || !DesktopConfigured(path, exe) {
		t.Fatalf("mcpServers null : %v", err)
	}
}

func TestFindClaude(t *testing.T) {
	dir := home(t)
	old := lookPath
	t.Cleanup(func() { lookPath = old })
	lookPath = func(string) (string, error) { return "", errors.New("absent") }
	if runtime.GOOS == "windows" {
		t.Skip("emplacements Unix")
	}
	if p := FindClaude(); p != "" && !strings.HasPrefix(p, "/") {
		t.Fatalf("trouvé %q", p)
	}
	// Installeur natif : ~/.local/bin/claude, hors du PATH.
	bin := filepath.Join(dir, ".local", "bin", "claude")
	os.MkdirAll(filepath.Dir(bin), 0o755)
	os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o644)
	if FindClaude() == bin {
		t.Fatal("fichier non exécutable retenu")
	}
	os.Chmod(bin, 0o755)
	if p := FindClaude(); p != bin {
		t.Fatalf("FindClaude = %q, attendu %q", p, bin)
	}
	lookPath = func(string) (string, error) { return "/chemin/claude", nil }
	if p := FindClaude(); p != "/chemin/claude" {
		t.Fatalf("PATH d'abord : %q", p)
	}
}

func TestCodeConfigured(t *testing.T) {
	dir := home(t)
	url := URL(47390)
	if CodeConfigured(url, "jeton") {
		t.Fatal("configuré sans fichier")
	}
	args := CodeArgs(url, "jeton")
	if strings.Join(args, " ") != "mcp add --transport http --scope user lattis "+url+" --header Authorization: Bearer jeton" {
		t.Fatalf("arguments %v", args)
	}
	if c := CodeCommandLine(url, "jeton"); c != `claude mcp add --transport http --scope user lattis `+url+` --header "Authorization: Bearer jeton"` {
		t.Fatalf("commande %q", c)
	}
	// Ce que « claude mcp add --scope user » range dans ~/.claude.json.
	conf := `{"numStartups":3,"projects":{},"mcpServers":{"lattis":{"type":"http","url":"` + url + `","headers":{"Authorization":"Bearer jeton"}}}}`
	os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(conf), 0o600)
	if !CodeConfigured(url, "jeton") {
		t.Fatal("configuration non lue")
	}
	if CodeConfigured(url, "autre") || CodeConfigured(URL(1), "jeton") {
		t.Fatal("jeton ou port changés non vus")
	}
	alt := filepath.Join(dir, "alt")
	os.MkdirAll(alt, 0o700)
	t.Setenv("CLAUDE_CONFIG_DIR", alt)
	if CodeConfigured(url, "jeton") {
		t.Fatal("CLAUDE_CONFIG_DIR ignoré")
	}
}

// Le client est nommé par clientInfo (initialize, session HTTP) ou par le
// relais stdio ; Watch voit chaque appel avec son libellé.
func TestWatchAndClient(t *testing.T) {
	s := newServer(t)
	var calls []Call
	var connected []string
	s.Connect = func(c string) { connected = append(connected, c) }
	s.Watch = func(ctx context.Context, c Call, run func(context.Context) (any, error)) (any, error) {
		calls = append(calls, c)
		if c.Tool == "sheets_delete" {
			return nil, errors.New("refusé par l'utilisateur dans Lattis")
		}
		return run(ctx)
	}
	ts := httptest.NewServer(Handler(s, "secret"))
	defer ts.Close()
	post := func(body, session, client string) (*http.Response, map[string]any) {
		req, _ := http.NewRequest("POST", ts.URL, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer secret")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		if client != "" {
			req.Header.Set(ClientHeader, client)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var r map[string]any
		json.NewDecoder(resp.Body).Decode(&r)
		return resp, r
	}
	resp, _ := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"claude-code","version":"2"}}}`, "", "")
	sid := resp.Header.Get("Mcp-Session-Id")
	if sid == "" || len(connected) != 1 || connected[0] != "claude-code" {
		t.Fatalf("session %q, connectés %v", sid, connected)
	}
	_, r := post(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"sheets_add","arguments":{"name":"Ventes"}}}`, sid, "")
	if r["result"].(map[string]any)["isError"] != false {
		t.Fatalf("appel : %v", r)
	}
	_, r = post(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"sheets_delete","arguments":{"sheet":"Ventes"}}}`, "", "claude-ai")
	res := r["result"].(map[string]any)
	if res["isError"] != true || !strings.Contains(res["content"].([]any)[0].(map[string]any)["text"].(string), "refusé par l'utilisateur") {
		t.Fatalf("refus : %v", r)
	}
	_, _ = post(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"sheets_list","arguments":{}}}`, "", "")
	if len(calls) != 3 {
		t.Fatalf("appels vus : %+v", calls)
	}
	if c := calls[0]; c.Client != "claude-code" || !c.Write || c.Target != "Ventes" || c.Label != "Ajouter une feuille « Ventes »" && !strings.Contains(c.Label, "« Ventes »") {
		t.Fatalf("premier appel : %+v", c)
	}
	if c := calls[1]; c.Client != "claude-ai" || !c.Write {
		t.Fatalf("deuxième appel : %+v", c)
	}
	if c := calls[2]; c.Client != "" || c.Write || c.Target != "" {
		t.Fatalf("troisième appel : %+v", c)
	}
	if ClientLabel("claude-ai") != "Claude Desktop" || ClientLabel("claude-code") != "Claude Code" || ClientLabel("") != "Client MCP" || ClientLabel("Cursor") != "Cursor" {
		t.Fatal("noms affichés des clients")
	}
}

// Le relais stdio transmet le nom du client de initialize à chaque message.
func TestProxyClientHeader(t *testing.T) {
	var seen []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get(ClientHeader))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer ts.Close()
	dir := t.TempDir()
	old := settingsFile
	t.Cleanup(func() { settingsFile = old })
	settingsFile = func() (string, error) { return filepath.Join(dir, "mcp.json"), nil }
	port := ts.Listener.Addr().String()
	var n int
	for _, c := range port[strings.LastIndexByte(port, ':')+1:] {
		n = n*10 + int(c-'0')
	}
	SaveSettings(Settings{Enabled: true, Port: n, Token: "t"})
	in := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"claude-ai"}}}` + "\n"
	var out bytes.Buffer
	if err := Proxy(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	in = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"claude-ai"}}}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"ping"}` + "\n"
	seen = nil
	Proxy(strings.NewReader(in), &out)
	if len(seen) != 2 || seen[0] != "claude-ai" || seen[1] != "claude-ai" {
		t.Fatalf("en-têtes vus : %v", seen)
	}
}
