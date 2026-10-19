package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// openaiTestProvider : un fournisseur neuf (cache des modèles vide).
func openaiTestProvider(t *testing.T, id string) *openaiProvider {
	t.Helper()
	for _, p := range openaiProviders() {
		if p.id == id {
			return p
		}
	}
	t.Fatalf("fournisseur %s inconnu", id)
	return nil
}

// sse écrit des morceaux de flux, puis [DONE].
func sse(w http.ResponseWriter, chunks ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, c := range chunks {
		fmt.Fprintf(w, "data: %s\n\n", c)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	io.WriteString(w, "data: [DONE]\n\n")
}

func TestOpenAICatalog(t *testing.T) {
	for _, id := range []string{"mistral", "openai", "gemini", "mammouth", "local"} {
		p := Lookup(id)
		if p == nil {
			t.Fatalf("%s non enregistré", id)
		}
		info := p.Info()
		if !info.Available || info.DefaultModel == "" || len(info.Models) == 0 || info.Name == "" {
			t.Errorf("%s : %+v", id, info)
		}
		if (info.KeyURL == "") != (id == "local") {
			t.Errorf("%s : page des clés %q", id, info.KeyURL)
		}
		if _, ok := p.(KeyChecker); !ok {
			t.Errorf("%s : pas de CheckKey", id)
		}
	}
	// Anthropic reste en tête du catalogue (ordre des noms).
	if c := Catalog(); c[0].ID != "anthropic" {
		t.Fatalf("%+v", c[0])
	}
}

func TestOpenAITurnText(t *testing.T) {
	var body map[string]any
	var auth, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		sse(w,
			`{"id":"1","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}`,
			`{"id":"1","choices":[{"index":0,"delta":{"content":"Bonjour, "}}]}`,
			`{"id":"1","choices":[{"index":0,"delta":{"content":"voici vos tableaux."}}]}`,
			`{"id":"1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`{"id":"1","choices":[],"usage":{"prompt_tokens":120,"completion_tokens":8}}`,
		)
	}))
	defer srv.Close()

	p := openaiTestProvider(t, "openai")
	req := Request{Model: "gpt-5.4", System: "consigne", Tools: Tools(SchemaOnly),
		Messages: []Message{{Role: User, Content: []Block{{Type: TextBlock, Text: "Bonjour"}}}}}
	var text string
	reply, err := p.Turn(context.Background(), Credentials{Key: "sk-test", Endpoint: srv.URL + "/v1/"}, req, func(e Event) {
		if e.Kind == EventText {
			text += e.Text
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if text != "Bonjour, voici vos tableaux." || reply.Stop != StopEnd {
		t.Fatalf("%q %s", text, reply.Stop)
	}
	if c := reply.Message.Content; len(c) != 1 || c[0].Text != text || reply.Message.Provider != "openai" {
		t.Fatalf("%+v", reply.Message)
	}
	if auth != "Bearer sk-test" || path != "/v1/chat/completions" {
		t.Fatalf("%q %q", auth, path)
	}
	if body["model"] != "gpt-5.4" || body["stream"] != true || body["stream_options"] == nil {
		t.Fatalf("requête : %v", body)
	}
	msgs := body["messages"].([]any)
	if m := msgs[0].(map[string]any); m["role"] != "system" || m["content"] != "consigne" {
		t.Fatalf("consigne : %v", m)
	}
	tools := body["tools"].([]any)
	if len(tools) != len(req.Tools) {
		t.Fatalf("%d outils", len(tools))
	}
	f := tools[0].(map[string]any)["function"].(map[string]any)
	if f["name"] == "" || f["parameters"].(map[string]any)["type"] != "object" {
		t.Fatalf("outil : %v", f)
	}
}

func TestOpenAITurnToolCalls(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		json.Unmarshal(b, &m)
		bodies = append(bodies, m)
		if len(bodies) > 1 {
			sse(w, `{"choices":[{"delta":{"content":"Fait."},"finish_reason":"stop"}]}`)
			return
		}
		// Deux appels parallèles, arguments en morceaux, signature (Gemini).
		sse(w,
			`{"choices":[{"delta":{"content":"Je regarde."}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_A","type":"function","function":{"name":"tables_get","arguments":""}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"tab"}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"le\":\"Ven"}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_B","type":"function","function":{"name":"tables_list","arguments":""},"extra_content":{"google":{"thought_signature":"sig"}}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"tes\"}"}}]}}]}`,
			`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		)
	}))
	defer srv.Close()

	p := openaiTestProvider(t, "gemini")
	cred := Credentials{Key: "AIza-test", Endpoint: srv.URL}
	req := Request{Model: "gemini-3.5-flash", Messages: []Message{{Role: User, Content: []Block{{Type: TextBlock, Text: "Ventes ?"}}}}}
	reply, err := p.Turn(context.Background(), cred, req, func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	c := reply.Message.Content
	if reply.Stop != StopToolUse || len(c) != 3 || c[0].Text != "Je regarde." {
		t.Fatalf("%s %+v", reply.Stop, c)
	}
	if c[1].ID != "call_A" || c[1].Name != "tables_get" || string(c[1].Input) != `{"table":"Ventes"}` {
		t.Fatalf("appel 1 : %+v", c[1])
	}
	if c[2].ID != "call_B" || c[2].Name != "tables_list" || string(c[2].Input) != "{}" {
		t.Fatalf("appel 2 : %+v (%s)", c[2], c[2].Input)
	}

	// Tour suivant : le message rendu revient tel quel (signature comprise),
	// les résultats sont des messages role=tool avant le texte ajouté.
	req.Messages = append(req.Messages, reply.Message, Message{Role: User, Content: []Block{
		{Type: ToolResultBlock, ToolUseID: "call_A", Text: `{"rows":3}`},
		{Type: ToolResultBlock, ToolUseID: "call_B", Text: "boum", IsError: true},
		{Type: TextBlock, Text: "Et ensuite ?"},
	}})
	if _, err := p.Turn(context.Background(), cred, req, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	msgs := bodies[1]["messages"].([]any)
	if len(msgs) != 5 {
		t.Fatalf("%d messages : %v", len(msgs), msgs)
	}
	a := msgs[1].(map[string]any)
	calls := a["tool_calls"].([]any)
	if a["role"] != "assistant" || len(calls) != 2 {
		t.Fatalf("assistant : %v", a)
	}
	c2 := calls[1].(map[string]any)
	if c2["extra_content"] == nil || c2["function"].(map[string]any)["arguments"] != "{}" {
		t.Fatalf("appel renvoyé : %v", c2)
	}
	t1, t2, u := msgs[2].(map[string]any), msgs[3].(map[string]any), msgs[4].(map[string]any)
	if t1["role"] != "tool" || t1["tool_call_id"] != "call_A" || t1["content"] != `{"rows":3}` {
		t.Fatalf("résultat 1 : %v", t1)
	}
	if t2["tool_call_id"] != "call_B" || !strings.HasPrefix(t2["content"].(string), "Erreur") {
		t.Fatalf("résultat 2 : %v", t2)
	}
	if u["role"] != "user" || u["content"] != "Et ensuite ?" {
		t.Fatalf("texte : %v", u)
	}
}

// Appels entiers en un morceau, même index (comme Gemini), sans
// identifiant ; arguments sous forme d'objet (serveur local).
func TestOpenAITurnWholeCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"x1","function":{"name":"a","arguments":"{\"n\":1}"}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"x2","function":{"name":"b","arguments":{"n":2}}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"function":{"name":"c","arguments":"pas du json"}}]}}]}`,
			`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		)
	}))
	defer srv.Close()
	p := openaiTestProvider(t, "local")
	reply, err := p.Turn(context.Background(), Credentials{Endpoint: srv.URL}, Request{Model: "qwen3"}, func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	c := reply.Message.Content
	if reply.Stop != StopToolUse || len(c) != 3 {
		t.Fatalf("%s %+v", reply.Stop, c)
	}
	if c[0].ID != "x1" || c[1].ID != "x2" || string(c[1].Input) != `{"n":2}` || c[2].Name != "c" || len(c[2].ID) != 9 {
		t.Fatalf("%+v", c)
	}
	// Arguments illisibles : passés comme texte JSON, valide.
	if !json.Valid(c[2].Input) || string(c[2].Input) != `"pas du json"` {
		t.Fatalf("%s", c[2].Input)
	}
	if _, err := json.Marshal(reply.Message); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAIStopReasons(t *testing.T) {
	for _, tc := range []struct {
		chunk, stop string
	}{
		{`{"choices":[{"delta":{"content":"tronq"},"finish_reason":"length"}]}`, StopMaxTokens},
		{`{"choices":[{"delta":{},"finish_reason":"content_filter"}]}`, StopRefusal},
		{`{"choices":[{"delta":{"refusal":"non"},"finish_reason":"stop"}]}`, StopRefusal},
		// Mistral : contenu en morceaux (réflexion écartée).
		{`{"choices":[{"delta":{"content":[{"type":"thinking","thinking":[{"type":"text","text":"hmm"}]},{"type":"text","text":"ok"}]},"finish_reason":"stop"}]}`, StopEnd},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sse(w, tc.chunk) }))
		reply, err := openaiTestProvider(t, "mistral").Turn(context.Background(), Credentials{Key: "k", Endpoint: srv.URL}, Request{Model: "m"}, func(Event) {})
		srv.Close()
		if err != nil || reply.Stop != tc.stop {
			t.Fatalf("%s : %s %v", tc.chunk, reply.Stop, err)
		}
		if tc.stop == StopEnd && reply.Message.Content[0].Text != "ok" {
			t.Fatalf("%+v", reply.Message)
		}
	}
}

// Mistral : identifiants d'appel de 9 caractères ; ceux d'un autre
// fournisseur sont convertis, appel et résultat restant appariés.
func TestOpenAIMistralIDs(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		if _, ok := body["stream_options"]; ok {
			w.WriteHeader(422)
			io.WriteString(w, `{"detail":[{"msg":"Extra inputs are not permitted"}]}`)
			return
		}
		sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	msgs := []Message{
		{Role: User, Content: []Block{{Type: TextBlock, Text: "a"}}},
		{Role: Assistant, Provider: "anthropic", Content: []Block{{Type: ToolUseBlock, ID: "toolu_01XYZ", Name: "tables_list", Input: json.RawMessage(`{}`)}}},
		{Role: User, Content: []Block{{Type: ToolResultBlock, ToolUseID: "toolu_01XYZ", Text: "[]"}}},
	}
	if _, err := openaiTestProvider(t, "mistral").Turn(context.Background(), Credentials{Key: "k", Endpoint: srv.URL}, Request{Model: "m", Messages: msgs}, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	m := body["messages"].([]any)
	id := m[1].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)["id"].(string)
	if !shortID.MatchString(id) || m[2].(map[string]any)["tool_call_id"] != id {
		t.Fatalf("%v", m)
	}
	if _, ok := m[1].(map[string]any)["extra_content"]; ok {
		t.Fatal("signature envoyée à Mistral")
	}
}

// Erreur passagère (503, 429 avec un Retry-After court) : nouvelle
// tentative ; un Retry-After long (quota) est rendu tout de suite (M6).
func TestOpenAIRetry(t *testing.T) {
	defer func(w []time.Duration) { openaiRetryWait = w }(openaiRetryWait)
	openaiRetryWait = []time.Duration{time.Millisecond, time.Millisecond}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			w.WriteHeader(503)
		case 2:
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
		default:
			sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`)
		}
	}))
	defer srv.Close()
	r, err := openaiTestProvider(t, "openai").Turn(context.Background(), Credentials{Key: "k", Endpoint: srv.URL}, Request{Model: "m"}, func(Event) {})
	if err != nil || calls != 3 || len(r.Message.Content) == 0 {
		t.Fatalf("%d appels, %v", calls, err)
	}
	calls = 0
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
		io.WriteString(w, `{"message":"Requests rate limit exceeded"}`)
	}))
	defer srv2.Close()
	if _, err := openaiTestProvider(t, "mistral").Turn(context.Background(), Credentials{Key: "k", Endpoint: srv2.URL}, Request{Model: "m"}, func(Event) {}); err == nil || calls != 1 {
		t.Fatalf("%d appels, %v", calls, err)
	}
}

func TestOpenAIErrors(t *testing.T) {
	defer func(w []time.Duration) { openaiRetryWait = w }(openaiRetryWait)
	openaiRetryWait = nil
	for _, tc := range []struct {
		id     string
		status int
		body   string
		want   string
	}{
		{"openai", 401, `{"error":{"message":"Incorrect API key provided","type":"invalid_request_error"}}`, "clé d'API refusée par OpenAI"},
		{"gemini", 400, `[{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT"}}]`, "clé d'API refusée par Gemini"},
		{"mammouth", 402, `{"error":"Insufficient credits"}`, "crédits épuisés chez Mammouth AI"},
		{"openai", 429, `{"error":{"message":"You exceeded your current quota","code":"insufficient_quota"}}`, "crédits épuisés chez OpenAI"},
		{"mistral", 429, `{"message":"Requests rate limit exceeded"}`, "limite d'utilisation atteinte chez Mistral AI"},
		{"mistral", 404, `{"message":"Invalid model: zzz"}`, "modèle « zzz » inconnu de Mistral AI"},
		{"local", 404, `{"error":{"message":"model \"zzz\" not found, try pulling it first"}}`, "ollama pull zzz"},
		{"openai", 503, `<html>Service Unavailable</html>`, "OpenAI est surchargé"},
		{"mistral", 418, `{"message":"théière"}`, "erreur de Mistral AI (418) (théière)"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			io.WriteString(w, tc.body)
		}))
		_, err := openaiTestProvider(t, tc.id).Turn(context.Background(), Credentials{Key: "k", Endpoint: srv.URL}, Request{Model: "zzz"}, func(Event) {})
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s %d : %v", tc.id, tc.status, err)
		}
	}

	// Erreur au milieu du flux.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"delta":{"content":"dé"}}]}`, `{"error":{"message":"overloaded"}}`)
	}))
	_, err := openaiTestProvider(t, "openai").Turn(context.Background(), Credentials{Key: "k", Endpoint: srv.URL}, Request{Model: "m"}, func(Event) {})
	srv.Close()
	if err == nil || !strings.Contains(err.Error(), "overloaded") {
		t.Fatalf("%v", err)
	}

	// Pas de clé : rien n'est envoyé.
	if _, err := openaiTestProvider(t, "openai").Turn(context.Background(), Credentials{}, Request{}, func(Event) {}); !errors.Is(err, ErrNoKey) {
		t.Fatalf("%v", err)
	}

	// Modèle local arrêté : message clair, adresse du réglage.
	srv = httptest.NewServer(http.NotFoundHandler())
	addr := srv.URL + "/v1"
	srv.Close()
	old := LocalURL
	LocalURL = func() string { return addr }
	defer func() { LocalURL = old }()
	_, err = openaiTestProvider(t, "local").Turn(context.Background(), Credentials{}, Request{Model: "qwen3"}, func(Event) {})
	if err == nil || err.Error() != "Le modèle local ne répond pas à "+addr+" : lancez Ollama ou LM Studio" || !errors.Is(err, ErrUnreachable) {
		t.Fatalf("%v", err)
	}
}

// Modèle local : adresse du réglage ai.localUrl, sans clé ni en-tête
// Authorization.
func TestOpenAILocal(t *testing.T) {
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/models":
			io.WriteString(w, `{"object":"list","data":[{"id":"llama3.2:latest"},{"id":"qwen3:8b"},{"id":"nomic-embed-text:latest"}]}`)
		case "/v1/chat/completions":
			// Serveur qui ignore stream : réponse entière.
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"salut"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1}}`)
		}
	}))
	defer srv.Close()
	old := LocalURL
	LocalURL = func() string { return srv.URL + "/v1/" }
	defer func() { LocalURL = old }()

	p := openaiTestProvider(t, "local")
	if err := p.CheckKey(context.Background(), Credentials{}); err != nil {
		t.Fatal(err)
	}
	info := p.Info()
	if len(info.Models) != 2 || info.DefaultModel != "llama3.2:latest" {
		t.Fatalf("%+v", info)
	}
	reply, err := p.Turn(context.Background(), Credentials{}, Request{}, func(Event) {})
	if err != nil || reply.Message.Content[0].Text != "salut" {
		t.Fatalf("%+v %v", reply, err)
	}
	for _, a := range auth {
		if a != "" {
			t.Fatalf("en-tête Authorization envoyé : %q", a)
		}
	}
}

func TestOpenAIModels(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "Bearer bonne" {
			w.WriteHeader(401)
			io.WriteString(w, `{"message":"Unauthorized"}`)
			return
		}
		io.WriteString(w, `{"object":"list","data":[
			{"id":"mistral-large-latest","capabilities":{"completion_chat":true,"function_calling":true}},
			{"id":"zz-custom","capabilities":{"completion_chat":true,"function_calling":true}},
			{"id":"mistral-medium-latest","capabilities":{"completion_chat":true,"function_calling":true}},
			{"id":"mistral-embed","capabilities":{"completion_chat":false,"function_calling":false}},
			{"id":"codestral-mamba","capabilities":{"completion_chat":true,"function_calling":false}},
			{"id":"open-mistral-7b","capabilities":{"completion_chat":true,"function_calling":true},"deprecation":"2025-03-30T12:00:00Z"},
			{"id":"mistral-medium-latest","capabilities":{"completion_chat":true,"function_calling":true}}
		]}`)
	}))
	defer srv.Close()

	p := openaiTestProvider(t, "mistral")
	if err := p.CheckKey(context.Background(), Credentials{Key: "mauvaise", Endpoint: srv.URL}); err == nil || !strings.Contains(err.Error(), "clé d'API refusée par Mistral AI") || errors.Is(err, ErrUnreachable) {
		t.Fatalf("%v", err)
	}
	cred := Credentials{Key: "bonne", Endpoint: srv.URL}
	ms, err := p.ListModels(context.Background(), cred)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range ms {
		ids = append(ids, m.ID)
	}
	if strings.Join(ids, ",") != "mistral-medium-latest,mistral-large-latest,zz-custom" || ms[0].Name != "Mistral Medium" {
		t.Fatalf("%v", ms)
	}
	// Liste gardée quelques minutes, et rendue par Info.
	n := hits.Load()
	if _, err := p.ListModels(context.Background(), cred); err != nil || hits.Load() != n {
		t.Fatalf("liste relue (%v)", err)
	}
	if info := p.Info(); len(info.Models) != 3 || info.DefaultModel != "mistral-medium-latest" {
		t.Fatalf("%+v", info)
	}

	// Gemini : identifiants préfixés de models/, modèles d'images écartés.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"object":"list","data":[{"id":"models/gemini-9-pro"},{"id":"models/gemini-embedding-001"},{"id":"models/imagen-4.0-generate-001"}]}`)
	}))
	defer srv2.Close()
	g := openaiTestProvider(t, "gemini")
	gm, err := g.ListModels(context.Background(), Credentials{Key: "k", Endpoint: srv2.URL})
	if err != nil || len(gm) != 1 || gm[0].ID != "gemini-9-pro" || g.Info().DefaultModel != "gemini-9-pro" {
		t.Fatalf("%v %v", gm, err)
	}
}

// La boucle d'outils commune fonctionne avec l'adaptateur.
func TestOpenAISession(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			sse(w, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"abcdefghi","function":{"name":"tables_list","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
			return
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"tool_call_id":"abcdefghi"`) {
			w.WriteHeader(400)
			return
		}
		sse(w, `{"choices":[{"delta":{"content":"Un tableau : Ventes."},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	h := newHost(t)
	var s Session
	var text string
	err := s.Send(context.Background(), Turn{
		Provider: openaiTestProvider(t, "mistral"), Cred: Credentials{Key: "k", Endpoint: srv.URL},
		Model: "mistral-medium-latest", Tools: Tools(SchemaOnly), Exec: Exec(h, SchemaOnly),
	}, "Quels tableaux ?", func(e Event) { text += e.Text })
	if err != nil || text != "Un tableau : Ventes." || s.Len() != 4 {
		t.Fatalf("%v %q %d", err, text, s.Len())
	}
}
