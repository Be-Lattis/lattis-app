package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Les exemples de la référence (TestExamplesRun) ne sortent pas sur le
// réseau : toute requête reçoit la même réponse JSON. Les tests ci-dessous
// remettent un vrai transport pour parler à un serveur httptest.
func TestMain(m *testing.M) {
	httpTransport = roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"taux": 1.08}`)),
			Request:    r,
		}, nil
	})
	os.Exit(m.Run())
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// realNet fait parler les requêtes au vrai réseau (serveur httptest local).
func realNet(t *testing.T) {
	old := httpTransport
	httpTransport = http.DefaultTransport
	t.Cleanup(func() { httpTransport = old })
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Test", "oui")
		w.Write([]byte(`{"nom": "Lattis", "valeurs": [1, 2, 3], "ok": true}`))
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"method": r.Method, "type": r.Header.Get("Content-Type"),
			"auth": r.Header.Get("Authorization"), "body": string(b),
		})
	})
	mux.HandleFunc("/text", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("introuvable"))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case slowStarted <- struct{}{}:
		default:
		}
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", httpMaxBody+1)))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPGetPostRequest(t *testing.T) {
	realNet(t)
	srv := testServer(t)
	h := newHost(t)
	h.answer = map[string]any{"remember": true}
	run(t, h, `
local r = http.get("`+srv.URL+`/json")
print(r.status, r.ok, r.json.nom, #r.json.valeurs, r.json.ok, r.headers["x-test"])
local p = http.post("`+srv.URL+`/echo", {client = "Dupont", montant = 120})
print(p.json.method, p.json.type, p.json.body)
local q = http.post("`+srv.URL+`/echo", "bonjour", {headers = {Authorization = "Bearer x"}})
print(q.json.type, q.json.body, q.json.auth)
local s = http.request{method = "put", url = "`+srv.URL+`/echo", json = {1, 2}, timeout = 5}
print(s.json.method, s.json.body)
local n = http.get("`+srv.URL+`/text")
print(n.status, n.ok, n.body, n.json)
`)
	want := []string{
		"200\ttrue\tLattis\t3\ttrue\toui",
		"POST\tapplication/json\t{\"client\":\"Dupont\",\"montant\":120}",
		"text/plain; charset=utf-8\tbonjour\tBearer x",
		"PUT\t[1,2]",
		"404\tfalse\tintrouvable\tnil",
	}
	if got := strings.Join(h.prints, "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("sortie :\n%s", got)
	}
	// Une seule question pour l'hôte, mémorisée dans le classeur.
	if len(h.dialogs) != 1 || !strings.Contains(h.dialogs[0].Message, "127.0.0.1") || h.dialogs[0].OK != "Autoriser" {
		t.Fatalf("boîtes : %+v", h.dialogs)
	}
	hosts, err := h.d.HTTPHosts(context.Background())
	if err != nil || len(hosts) != 1 || !strings.HasPrefix(hosts[0], "127.0.0.1:") {
		t.Fatalf("hôtes mémorisés %v %v", hosts, err)
	}
	// L'accord est rangé sur ce poste, pas dans le classeur.
	if dirty, _ := h.d.Dirty(context.Background()); dirty {
		t.Fatal("hôte mémorisé dans le classeur")
	}
}

func TestHTTPPermission(t *testing.T) {
	realNet(t)
	srv := testServer(t)
	ctx := context.Background()

	// Refus : la requête échoue, et la question n'est pas reposée pendant la session.
	h := newHost(t)
	for range 2 {
		_, err := Invoke(ctx, h, "http.get", map[string]any{"url": srv.URL + "/json"})
		if !errors.Is(err, ErrHostNotAllowed) {
			t.Fatalf("refus : %v", err)
		}
	}
	if len(h.dialogs) != 1 {
		t.Fatalf("%d questions", len(h.dialogs))
	}
	if hosts, _ := h.d.HTTPHosts(ctx); len(hosts) != 0 {
		t.Fatalf("refus mémorisé : %v", hosts)
	}

	// Serveur MCP : pas de question, refus avec un message clair…
	h2 := newHost(t)
	h2.answer = map[string]any{}
	_, err := Invoke(WithoutPrompt(ctx), h2, "http.get", map[string]any{"url": srv.URL + "/json"})
	if !errors.Is(err, ErrHostNotAllowed) || !strings.Contains(err.Error(), "seul l'utilisateur") || len(h2.dialogs) != 0 {
		t.Fatalf("MCP : %v, %d questions", err, len(h2.dialogs))
	}
	// … même dans un script lancé par MCP.
	res, err := RunScriptFrom(WithoutPrompt(ctx), h2, "", `http.get("`+srv.URL+`/json")`, "")
	if err != nil || !strings.Contains(res.Error, "non autorisé") || len(h2.dialogs) != 0 {
		t.Fatalf("script MCP : %+v %v", res, err)
	}
	// Autorisé « pour la session » (sans mémoriser) : MCP passe ensuite.
	if _, err := Invoke(ctx, h2, "http.get", map[string]any{"url": srv.URL + "/json"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Invoke(WithoutPrompt(ctx), h2, "http.get", map[string]any{"url": srv.URL + "/json"}); err != nil {
		t.Fatalf("hôte autorisé : %v", err)
	}
	if hosts, _ := h2.d.HTTPHosts(ctx); len(hosts) != 0 {
		t.Fatalf("autorisation de session mémorisée : %v", hosts)
	}
	// Oublier l'hôte : la question revient.
	if err := h2.d.ForgetHTTPHost(ctx, strings.TrimPrefix(srv.URL, "http://")); err != nil {
		t.Fatal(err)
	}
	if _, err := Invoke(WithoutPrompt(ctx), h2, "http.get", map[string]any{"url": srv.URL + "/json"}); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("hôte oublié : %v", err)
	}

	// Seuls http et https.
	if _, err := Invoke(ctx, h2, "http.get", map[string]any{"url": "file:///etc/passwd"}); err == nil || !strings.Contains(err.Error(), "https://") {
		t.Fatalf("file:// : %v", err)
	}
}

func TestHTTPRedirectAsksForNewHost(t *testing.T) {
	realNet(t)
	srv := testServer(t)
	// 127.0.0.1 redirige vers localhost : un autre hôte, une autre question.
	other := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	redir := httptest.NewServer(http.RedirectHandler(other+"/json", http.StatusFound))
	defer redir.Close()
	h := newHost(t)
	h.answer = map[string]any{}
	res, err := Invoke(context.Background(), h, "http.get", map[string]any{"url": redir.URL})
	if err != nil {
		t.Fatal(err)
	}
	if r := res.(HTTPResponse); r.Status != 200 || !strings.HasPrefix(r.URL, "http://localhost:") {
		t.Fatalf("réponse %+v", r)
	}
	if len(h.dialogs) != 2 || !strings.Contains(h.dialogs[1].Message, "localhost") {
		t.Fatalf("questions %+v", h.dialogs)
	}
}

func TestHTTPLimits(t *testing.T) {
	realNet(t)
	srv := testServer(t)
	h := newHost(t)
	h.answer = map[string]any{}
	ctx := context.Background()

	start := time.Now()
	_, err := Invoke(ctx, h, "http.request", map[string]any{"url": srv.URL + "/slow", "timeout": 0.2})
	if err == nil || !strings.Contains(err.Error(), "pas de réponse") || time.Since(start) > 3*time.Second {
		t.Fatalf("délai : %v en %v", err, time.Since(start))
	}
	_, err = Invoke(ctx, h, "http.get", map[string]any{"url": srv.URL + "/big"})
	if err == nil || !strings.Contains(err.Error(), "trop grande") {
		t.Fatalf("taille : %v", err)
	}
	_, err = Invoke(ctx, h, "http.get", map[string]any{"url": srv.URL + "/json", "options": map[string]any{"timout": 3}})
	if err == nil || !strings.Contains(err.Error(), "timout") {
		t.Fatalf("option inconnue : %v", err)
	}
	_, err = Invoke(ctx, h, "http.request", map[string]any{"url": srv.URL + "/echo", "body": "a", "json": map[string]any{}})
	if err == nil {
		t.Fatal("body et json acceptés ensemble")
	}
}

// slowStarted : une requête est arrivée sur /slow.
var slowStarted = make(chan struct{}, 1)

// Le bouton Arrêter interrompt une requête en cours.
func TestHTTPStop(t *testing.T) {
	realNet(t)
	srv := testServer(t)
	h := newHost(t)
	h.answer = map[string]any{}
	// Arrêt quand la requête est arrivée au serveur (le script tourne),
	// pas après un délai fixe.
	for len(slowStarted) > 0 {
		<-slowStarted
	}
	go func() {
		<-slowStarted
		StopScript()
	}()
	start := time.Now()
	res, err := RunScript(h, "", `http.get("`+srv.URL+`/slow")`, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Error, "arrêté par l'utilisateur") || time.Since(start) > 3*time.Second {
		t.Fatalf("arrêt : %q en %v", res.Error, time.Since(start))
	}
}
