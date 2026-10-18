package ai

import (
	"os"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/anthropics/anthropic-sdk-go/config"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/doc"
)

type host struct {
	d       *doc.Document
	sel     api.Selection
	changed int
}

func (h *host) Doc() (*doc.Document, error)                     { return h.d, nil }
func (h *host) Changed(context.Context)                         { h.changed++ }
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

// newHost : un classeur avec un tableau Ventes (Client, Montant) de 30 lignes.
func newHost(t *testing.T) *host {
	t.Helper()
	ctx := context.Background()
	d, err := doc.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	h := &host{d: d, sel: api.Selection{Sheet: 1}}
	rows := [][]any{{"Client", "Montant"}}
	for i := range 30 {
		rows = append(rows, []any{fmt.Sprintf("C%d", i%7), float64(i * 10)})
	}
	if _, err := api.Invoke(ctx, h, "cells.paste", map[string]any{"row": 1, "col": 1, "values": rows}); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Invoke(ctx, h, "tables.rename", map[string]any{"table": firstTable(t, h), "name": "Ventes"}); err != nil {
		t.Fatal(err)
	}
	return h
}

func firstTable(t *testing.T, h *host) string {
	t.Helper()
	res, err := api.Invoke(context.Background(), h, "tables.list", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res)
	var l []struct{ Name string }
	json.Unmarshal(b, &l)
	if len(l) == 0 {
		t.Fatal("aucun tableau")
	}
	return l[0].Name
}

func toolNames(ts []Tool) map[string]bool {
	out := map[string]bool{}
	for _, t := range ts {
		out[t.Name] = true
	}
	return out
}

func TestToolsBySharing(t *testing.T) {
	schema := toolNames(Tools(SchemaOnly))
	sample := toolNames(Tools(Sample))
	for _, n := range []string{"tables_list", "columns_insert", "cells_set", "pivots_create", profileTool} {
		if !schema[n] || !sample[n] {
			t.Errorf("%s absent", n)
		}
	}
	// Valeurs des cellules : seulement en mode sample.
	for _, n := range []string{"tables_rows", "db_query", "columns_values", "sheets_get", "cells_get"} {
		if schema[n] {
			t.Errorf("%s proposé sans partage des données", n)
		}
		if !sample[n] {
			t.Errorf("%s absent en mode sample", n)
		}
	}
	// Jamais : suppression de tableaux, fichiers, réseau, scripts.
	for _, n := range []string{"tables_delete", "sheets_delete", "doc_save", "http_get", "scripts_run", "db_execute"} {
		if sample[n] {
			t.Errorf("%s proposé", n)
		}
	}
	for _, c := range structureCommands {
		if api.Lookup(c) == nil {
			t.Errorf("commande inconnue : %s", c)
		}
	}
}

func TestExecSchemaOnly(t *testing.T) {
	h := newHost(t)
	exec := Exec(h, SchemaOnly)
	ctx := context.Background()
	if res, isErr := exec(ctx, "db_query", json.RawMessage(`{"sql":"SELECT * FROM Ventes"}`)); !isErr {
		t.Fatalf("db_query permis : %s", res)
	}
	res, isErr := exec(ctx, profileTool, json.RawMessage(`{"table":"Ventes"}`))
	if isErr {
		t.Fatal(res)
	}
	var p struct {
		Rows    float64
		Columns []struct {
			Name     string
			Distinct float64
			Empty    float64
		}
	}
	if err := json.Unmarshal([]byte(res), &p); err != nil {
		t.Fatal(err, res)
	}
	if p.Rows != 30 || len(p.Columns) != 2 || p.Columns[0].Distinct != 7 || p.Columns[1].Distinct != 30 || p.Columns[0].Empty != 0 {
		t.Fatalf("profil : %s", res)
	}
	if strings.Contains(res, "C3") {
		t.Fatalf("valeur dans le profil : %s", res)
	}
	// Une modification passe par le bus de commandes.
	if res, isErr := exec(ctx, "columns_insert", json.RawMessage(`{"table":"Ventes","name":"TVA"}`)); isErr {
		t.Fatal(res)
	}
	if h.changed == 0 {
		t.Fatal("interface non prévenue")
	}
}

func TestExecSampleLimits(t *testing.T) {
	h := newHost(t)
	exec := Exec(h, Sample)
	ctx := context.Background()
	res, isErr := exec(ctx, "tables_rows", json.RawMessage(`{"table":"Ventes","count":1000}`))
	if isErr {
		t.Fatal(res)
	}
	var r struct{ Rows [][]any }
	json.Unmarshal([]byte(res), &r)
	if len(r.Rows) != SampleRows {
		t.Fatalf("%d lignes", len(r.Rows))
	}
	res, isErr = exec(ctx, "db_query", json.RawMessage(`{"sql":"SELECT Montant FROM Ventes"}`))
	if isErr {
		t.Fatal(res)
	}
	var q struct {
		Rows      [][]any
		Truncated bool
	}
	json.Unmarshal([]byte(res), &q)
	if len(q.Rows) != SampleRows || !q.Truncated {
		t.Fatalf("db_query : %s", res)
	}
}

// fakeProvider rejoue des réponses écrites d'avance.
type fakeProvider struct {
	replies []Reply
	errs    []error
	seen    []Request
}

func (f *fakeProvider) Info() Info { return Info{ID: "fake", Available: true, DefaultModel: "m"} }

func (f *fakeProvider) Turn(_ context.Context, _ Credentials, req Request, emit func(Event)) (Reply, error) {
	f.seen = append(f.seen, req)
	i := len(f.seen) - 1
	if i < len(f.errs) && f.errs[i] != nil {
		return Reply{}, f.errs[i]
	}
	r := f.replies[i]
	for _, b := range r.Message.Content {
		if b.Type == TextBlock {
			emit(Event{Kind: EventText, Text: b.Text})
		}
	}
	return r, nil
}

func TestSessionToolLoop(t *testing.T) {
	h := newHost(t)
	f := &fakeProvider{replies: []Reply{
		{Stop: StopToolUse, Message: Message{Role: Assistant, Content: []Block{
			{Type: TextBlock, Text: "J'ajoute deux colonnes."},
			{Type: ToolUseBlock, ID: "t1", Name: "columns_insert", Input: json.RawMessage(`{"table":"Ventes","name":"TVA"}`)},
			{Type: ToolUseBlock, ID: "t2", Name: "columns_insert", Input: json.RawMessage(`{"table":"Ventes","name":"TTC"}`)},
		}}},
		{Stop: StopEnd, Message: Message{Role: Assistant, Content: []Block{{Type: TextBlock, Text: "Fait."}}}},
	}}
	var s Session
	var events []Event
	turn := Turn{Provider: f, Model: "m", Tools: Tools(SchemaOnly), Exec: Exec(h, SchemaOnly), Label: Label}
	ctx := command.WithGroup(context.Background(), "Assistant IA")
	if err := s.Send(ctx, turn, "Ajoute TVA et TTC", func(e Event) { events = append(events, e) }); err != nil {
		t.Fatal(err)
	}
	if len(f.seen) != 2 || s.Len() != 4 {
		t.Fatalf("%d tours, %d messages", len(f.seen), s.Len())
	}
	// Second tour : les deux résultats dans un seul message de l'utilisateur.
	last := f.seen[1].Messages[2]
	if last.Role != User || len(last.Content) != 2 || last.Content[0].ToolUseID != "t1" || last.Content[0].IsError {
		t.Fatalf("résultats : %+v", last)
	}
	var tools int
	for _, e := range events {
		if e.Kind == EventTool {
			tools++
			if e.Label != "Insérer une colonne « Ventes »" {
				t.Errorf("libellé : %q", e.Label)
			}
		}
	}
	if tools != 2 {
		t.Fatalf("%d outils", tools)
	}
	// Les deux colonnes forment une seule étape d'annulation.
	if _, err := api.Invoke(context.Background(), h, "doc.undo", nil); err != nil {
		t.Fatal(err)
	}
	info, _ := api.Invoke(context.Background(), h, "tables.get", map[string]any{"table": "Ventes"})
	if b, _ := json.Marshal(info); strings.Contains(string(b), "TVA") {
		t.Fatalf("annulation partielle : %s", b)
	}
}

func TestSessionErrorRollsBack(t *testing.T) {
	f := &fakeProvider{errs: []error{errors.New("panne")}, replies: []Reply{{}, {Stop: StopEnd, Message: Message{Role: Assistant, Content: []Block{{Type: TextBlock, Text: "ok"}}}}}}
	var s Session
	turn := Turn{Provider: f, Exec: func(context.Context, string, json.RawMessage) (string, bool) { return "", false }}
	if err := s.Send(context.Background(), turn, "bonjour", func(Event) {}); err == nil || s.Len() != 0 {
		t.Fatalf("err %v, %d messages", err, s.Len())
	}
	if err := s.Send(context.Background(), turn, "bonjour", func(Event) {}); err != nil || s.Len() != 2 {
		t.Fatalf("err %v, %d messages", err, s.Len())
	}
}

func TestSessionCancelKeepsHistoryValid(t *testing.T) {
	f := &fakeProvider{replies: []Reply{
		{Stop: StopToolUse, Message: Message{Role: Assistant, Content: []Block{{Type: ToolUseBlock, ID: "t1", Name: "x"}}}},
		{Stop: StopEnd, Message: Message{Role: Assistant, Content: []Block{{Type: TextBlock, Text: "ok"}}}},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	var s Session
	turn := Turn{Provider: f, Exec: func(context.Context, string, json.RawMessage) (string, bool) {
		cancel()
		return "fait", false
	}}
	if err := s.Send(ctx, turn, "un", func(Event) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
	// user, assistant (appel), user (résultat) ; le message suivant rejoint
	// le dernier message de l'utilisateur.
	if err := s.Send(context.Background(), turn, "deux", func(Event) {}); err != nil {
		t.Fatal(err)
	}
	m := f.seen[1].Messages
	if len(m) != 3 || m[2].Role != User || len(m[2].Content) != 2 || m[2].Content[1].Text != "deux" {
		t.Fatalf("conversation : %+v", m)
	}
}

func TestAnthropicTurn(t *testing.T) {
	var body map[string]any
	var beta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		beta = r.Header.Get("anthropic-beta")
		if r.Header.Get("X-Api-Key") != "sk-test" && r.Header.Get("Authorization") != "Bearer jeton-compte" {
			w.WriteHeader(401)
			io.WriteString(w, `{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range []string{
			`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":1}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Je regarde "}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"vos tableaux."}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"tables_list","input":{}}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{}"}}`,
			`{"type":"content_block_stop","index":1}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":20}}`,
			`{"type":"message_stop"}`,
		} {
			var e struct{ Type string }
			json.Unmarshal([]byte(ev), &e)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, ev)
		}
	}))
	defer srv.Close()

	p := Lookup("anthropic")
	req := Request{Model: "claude-opus-5-5", System: "consigne", Tools: Tools(SchemaOnly),
		Messages: []Message{{Role: User, Content: []Block{{Type: TextBlock, Text: "Bonjour"}}}}}
	var text string
	// Passerelle (Endpoint posé) : pas de repli côté serveur.
	reply, err := p.Turn(context.Background(), Credentials{Key: "sk-test", Endpoint: srv.URL}, req, func(e Event) { text += e.Text })
	if err != nil {
		t.Fatal(err)
	}
	if text != "Je regarde vos tableaux." || reply.Stop != StopToolUse {
		t.Fatalf("%q %s", text, reply.Stop)
	}
	c := reply.Message.Content
	if len(c) != 2 || c[1].Type != ToolUseBlock || c[1].Name != "tables_list" || c[1].ID != "toolu_1" {
		t.Fatalf("contenu : %+v", c)
	}
	if body["model"] != "claude-opus-5-5" || body["stream"] != true || body["fallbacks"] != nil || strings.Contains(beta, "fallback") {
		t.Fatalf("requête : %v / %q", body, beta)
	}
	tools, _ := body["tools"].([]any)
	if len(tools) != len(req.Tools) {
		t.Fatalf("%d outils envoyés", len(tools))
	}
	// Le message rendu est renvoyé tel quel au tour suivant.
	req.Messages = append(req.Messages, reply.Message, Message{Role: User, Content: []Block{{Type: ToolResultBlock, ToolUseID: "toolu_1", Text: "[]"}}})
	if _, err := p.Turn(context.Background(), Credentials{Key: "sk-test", Endpoint: srv.URL}, req, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("%d messages", len(msgs))
	}
	if b, _ := json.Marshal(msgs[1]); !strings.Contains(string(b), `"toolu_1"`) || !strings.Contains(string(b), "vos tableaux") {
		t.Fatalf("message rejoué : %s", b)
	}
	// Clé refusée : message lisible.
	if _, err := p.Turn(context.Background(), Credentials{Key: "sk-faux", Endpoint: srv.URL}, req, func(Event) {}); err == nil || !strings.Contains(err.Error(), "clé d'API refusée") {
		t.Fatalf("err %v", err)
	}
	// Sans clé ni compte connecté.
	dir := t.TempDir()
	t.Setenv("ANTHROPIC_CONFIG_DIR", dir)
	t.Setenv("ANTHROPIC_PROFILE", "")
	os.Unsetenv("ANTHROPIC_PROFILE")
	if _, err := p.Turn(context.Background(), Credentials{Endpoint: srv.URL}, req, func(Event) {}); !errors.Is(err, ErrNoKey) {
		t.Fatalf("err %v", err)
	}
	if acc := p.(AccountProvider).Account(); acc.Connected {
		t.Fatalf("compte inventé : %+v", acc)
	}
	// Compte connecté (profil rangé par « ant auth login ») : jeton porteur.
	writeProfile(t, dir, "jeton-compte")
	if acc := p.(AccountProvider).Account(); !acc.Connected || acc.Name != "louis@exemple.fr" {
		t.Fatalf("compte : %+v", acc)
	}
	if _, err := p.Turn(context.Background(), Credentials{Endpoint: srv.URL}, req, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	// Une clé enregistrée reste prioritaire.
	if _, err := p.Turn(context.Background(), Credentials{Key: "sk-test", Endpoint: srv.URL}, req, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	// Jeton refusé : message propre au compte.
	writeProfile(t, dir, "jeton-expire")
	if _, err := p.Turn(context.Background(), Credentials{Endpoint: srv.URL}, req, func(Event) {}); err == nil || !strings.Contains(err.Error(), "reconnectez-vous") {
		t.Fatalf("err %v", err)
	}
}

// writeProfile range un profil Anthropic comme le fait « ant auth login ».
func writeProfile(t *testing.T, dir, token string) {
	t.Helper()
	cfg := &config.Config{AuthenticationInfo: config.NewUserOAuthAuthentication("")}
	if err := config.SaveProfile(dir, "default", cfg); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteCredentials(config.ProfileCredentialsPath(dir, "default"), config.Credentials{AccessToken: token, AccountEmail: "louis@exemple.fr"}); err != nil {
		t.Fatal(err)
	}
}

func TestKeysMemory(t *testing.T) {
	k := NewKeys(false)
	if _, ok := k.Get("anthropic"); ok {
		t.Fatal("clé inventée")
	}
	if err := k.Set("anthropic", "  sk-ant-x \n"); err != nil {
		t.Fatal(err)
	}
	if v, ok := k.Get("anthropic"); !ok || v != "sk-ant-x" {
		t.Fatalf("%q", v)
	}
	k.Delete("anthropic")
	if _, ok := k.Get("anthropic"); ok {
		t.Fatal("clé gardée")
	}
}

func TestCatalog(t *testing.T) {
	c := Catalog()
	if len(c) == 0 || c[0].ID != "anthropic" || !c[0].Available || c[0].DefaultModel == "" {
		t.Fatalf("%+v", c)
	}
	// Seuls les fournisseurs branchés sont proposés.
	for _, p := range c {
		if !p.Available {
			t.Errorf("%s proposé sans être branché", p.ID)
		}
	}
}
