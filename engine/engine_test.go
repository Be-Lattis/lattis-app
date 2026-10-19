package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/lalamefine/lattis/internal/transport"
)

const bridgePkg = "github.com/lalamefine/lattis/internal/bridge."

// client est un client JSON-RPC de test : il range les réponses par id et
// les évènements par nom.
type client struct {
	t      *testing.T
	ws     *websocket.Conn
	next   int
	resp   chan map[string]json.RawMessage
	events chan transport.Event
}

func dial(t *testing.T, srv *httptest.Server) *client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	ws.SetReadLimit(transport.MaxMessage)
	c := &client{t: t, ws: ws, resp: make(chan map[string]json.RawMessage, 16), events: make(chan transport.Event, 64)}
	go func() {
		for {
			_, b, err := ws.Read(context.Background())
			if err != nil {
				return
			}
			var m map[string]json.RawMessage
			json.Unmarshal(b, &m)
			if string(m["method"]) == `"event"` {
				var p struct {
					Name string          `json:"name"`
					Data json.RawMessage `json:"data"`
				}
				json.Unmarshal(m["params"], &p)
				c.events <- transport.Event{Name: p.Name, Data: p.Data}
				continue
			}
			c.resp <- m
		}
	}()
	t.Cleanup(func() { ws.Close(websocket.StatusNormalClosure, "") })
	return c
}

// call envoie un appel et attend sa réponse (result ou error).
func (c *client) call(method any, params ...any) (json.RawMessage, *transport.Error) {
	c.t.Helper()
	c.next++
	if params == nil {
		params = []any{}
	}
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.next, "method": method, "params": params})
	if err := c.ws.Write(context.Background(), websocket.MessageText, b); err != nil {
		c.t.Fatal(err)
	}
	select {
	case m := <-c.resp:
		if string(m["id"]) != fmt.Sprint(c.next) {
			c.t.Fatalf("réponse à %s, attendue à %d", m["id"], c.next)
		}
		if e, ok := m["error"]; ok {
			var te transport.Error
			json.Unmarshal(e, &te)
			return nil, &te
		}
		return m["result"], nil
	case <-time.After(20 * time.Second):
		c.t.Fatalf("pas de réponse à %v", method)
	}
	return nil, nil
}

func (c *client) mustCall(method any, params ...any) json.RawMessage {
	c.t.Helper()
	r, err := c.call(method, params...)
	if err != nil {
		c.t.Fatalf("%v : %s", method, err.Message)
	}
	return r
}

// event attend l'évènement name.
func (c *client) event(name string) transport.Event {
	c.t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		select {
		case e := <-c.events:
			if e.Name == name {
				return e
			}
		case <-deadline:
			c.t.Fatalf("évènement %s non reçu", name)
		}
	}
}

func newEngine(t *testing.T, o Options) (*Engine, *httptest.Server) {
	t.Helper()
	o.Dir = t.TempDir()
	e, err := New(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(e.Handler())
	t.Cleanup(func() {
		srv.Close()
		e.Close()
	})
	return e, srv
}

func TestEngineCallAndEvent(t *testing.T) {
	_, srv := newEngine(t, Options{})
	c := dial(t, srv)

	// Par identifiant des liaisons générées (nombre ou chaîne), et par nom.
	var p struct{ Web, Scripting bool }
	json.Unmarshal(c.mustCall(transport.ID(bridgePkg+"DocumentService.Platform")), &p)
	if !p.Web || p.Scripting {
		t.Fatalf("Platform = %+v", p)
	}
	var st struct {
		Open  bool   `json:"open"`
		Title string `json:"title"`
	}
	json.Unmarshal(c.mustCall(fmt.Sprint(transport.ID(bridgePkg+"DocumentService.New"))), &st)
	if !st.Open || st.Title == "" {
		t.Fatalf("New = %+v", st)
	}

	// Une modification pousse doc:changed.
	c.mustCall("DocumentService.AddDoc", 0)
	var changed struct {
		Dirty bool `json:"dirty"`
	}
	json.Unmarshal(c.event("doc:changed").Data.(json.RawMessage), &changed)
	if !changed.Dirty {
		t.Fatal("doc:changed : classeur non modifié")
	}

	// Erreurs : méthode inconnue, nombre de paramètres, erreur rendue.
	if _, err := c.call("DocumentService.Nope"); err == nil || err.Code != transport.CodeNoMethod {
		t.Fatalf("méthode inconnue : %+v", err)
	}
	if _, err := c.call("DocumentService.AddDoc"); err == nil || err.Code != transport.CodeInvalidParams {
		t.Fatalf("paramètres manquants : %+v", err)
	}
	if _, err := c.call("DocumentService.RunScript", "x", "print(1)"); err == nil || !strings.Contains(err.Message, "scripts sont désactivés") {
		t.Fatalf("script sans scripting : %+v", err)
	}
}

func TestEngineUploadAndDownload(t *testing.T) {
	_, srv := newEngine(t, Options{})
	c := dial(t, srv)
	c.mustCall("DocumentService.New")

	// Importer (sans chemin) : le moteur demande un fichier au navigateur,
	// qui l'envoie.
	done := make(chan json.RawMessage, 1)
	go func() {
		r, err := c.call("DocumentService.Import", "")
		if err != nil {
			t.Errorf("Import : %s", err.Message)
		}
		done <- r
	}()
	var req OpenFileRequest
	json.Unmarshal(c.event(OpenFileEvent).Data.(json.RawMessage), &req)
	if req.ID == "" || !strings.HasSuffix(req.URL, "/upload/"+req.ID) {
		t.Fatalf("demande d'envoi : %+v", req)
	}
	resp, err := http.Post(srv.URL+req.URL+"?name=ventes.csv", "text/csv", strings.NewReader("produit;quantite\npomme;3\npoire;5\n"))
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("envoi : %v %v", resp, err)
	}
	var imp struct {
		Tables    []string `json:"tables"`
		Cancelled bool     `json:"cancelled"`
	}
	json.Unmarshal(<-done, &imp)
	if imp.Cancelled || len(imp.Tables) != 1 {
		t.Fatalf("Import = %+v", imp)
	}

	// Exporter : le moteur écrit le fichier puis le fait télécharger.
	c.mustCall("DocumentService.ExportCSV", imp.Tables[0])
	var dl Download
	json.Unmarshal(c.event(DownloadEvent).Data.(json.RawMessage), &dl)
	if !strings.HasSuffix(dl.Name, ".csv") {
		t.Fatalf("téléchargement : %+v", dl)
	}
	resp, err = http.Get(srv.URL + dl.URL)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("téléchargement : %v %v", resp, err)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	resp.Body.Close()

	// Annuler une demande d'envoi.
	go func() {
		r, _ := c.call("DocumentService.Import", "")
		done <- r
	}()
	json.Unmarshal(c.event(OpenFileEvent).Data.(json.RawMessage), &req)
	resp, err = http.Post(srv.URL+req.URL+"?cancel=1", "", nil)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("annulation : %v %v", resp, err)
	}
	json.Unmarshal(<-done, &imp)
	if !imp.Cancelled {
		t.Fatalf("Import annulé = %+v", imp)
	}
}

func TestEngineStatic(t *testing.T) {
	files := fstest.MapFS{
		"index.html":        {Data: []byte("<!doctype html>lattis")},
		"_app/version.json": {Data: []byte(`{}`)},
	}
	_, srv := newEngine(t, Options{Static: files})
	for path, want := range map[string]string{"/": "lattis", "/feuille/2": "lattis", "/_app/version.json": "{}"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		buf := make([]byte, 512)
		n, _ := resp.Body.Read(buf)
		b.Write(buf[:n])
		resp.Body.Close()
		if !strings.Contains(b.String(), want) {
			t.Errorf("%s : %q", path, b.String())
		}
	}
}
