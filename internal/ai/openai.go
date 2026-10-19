package ai

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/config"
)

// Fournisseurs « compatibles OpenAI » : l'API Chat Completions
// (POST {base}/chat/completions, réponse en flux SSE), que parlent Mistral
// AI, OpenAI, Google Gemini (point d'accès de compatibilité), Mammouth AI et
// les modèles locaux (Ollama, LM Studio…). En net/http pur : pas de SDK.
//
// Comme pour Claude, aucune variable d'environnement n'est lue (ni
// OPENAI_API_KEY ni OPENAI_BASE_URL…) : seules comptent la clé du trousseau,
// la passerelle imposée (ai.endpoint, Credentials.Endpoint) et, pour le
// modèle local, le réglage ai.localUrl. Le proxy du système (HTTPS_PROXY)
// reste suivi, comme pour toute connexion de Lattis.
//
// Particularités prises en compte :
//   - les arguments d'un appel d'outil arrivent en morceaux (texte JSON
//     découpé), rattachés à leur appel par index ; certains fournisseurs
//     (Mistral, Gemini) envoient l'appel entier d'un coup, parfois sans
//     index ou avec le même index pour deux appels parallèles ;
//   - Mistral n'accepte que des identifiants d'appel de 9 caractères
//     alphanumériques : ceux d'un autre fournisseur sont convertis ;
//   - Gemini attache aux appels une signature de réflexion
//     (extra_content.google.thought_signature) qu'il faut lui renvoyer :
//     le message rendu est gardé tel quel (Message.Native) ;
//   - les résultats d'outils sont des messages role=tool, placés juste après
//     l'appel, avant le texte que l'utilisateur aurait ajouté.

// defaultLocalURL : Ollama, sur ce poste.
const defaultLocalURL = "http://localhost:11434/v1"

// LocalURL donne l'adresse du modèle local (réglage ai.localUrl, validé par
// internal/config : http:// seulement vers ce poste). Remplaçable dans les
// tests.
var LocalURL = func() string {
	return config.Current().String("ai.localUrl", defaultLocalURL)
}

// modelsTTL : durée de vie de la liste des modèles lue chez le fournisseur.
const modelsTTL = 5 * time.Minute

// openaiListTimeout borne la lecture de la liste des modèles (et la
// vérification d'une clé).
const openaiListTimeout = 15 * time.Second

// openaiHTTP : pas de délai global, la réponse arrive en flux ; le contexte
// l'interrompt. Les en-têtes de la réponse doivent arriver en quelques
// minutes (un modèle local peut mettre longtemps à se charger) : un proxy
// qui garde la connexion ouverte sans répondre ne bloque pas la réponse
// jusqu'à Échap.
var openaiHTTP = &http.Client{Transport: openaiTransport()}

func openaiTransport() http.RoundTripper {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 3 * time.Minute
	return t
}

// openaiRetryWait : attentes avant chaque nouvelle tentative sur une erreur
// passagère (429, 5xx, coupure avant la réponse) ; au-delà, l'erreur est
// rendue (l'interface propose « Réessayer »).
var openaiRetryWait = []time.Duration{time.Second, 3 * time.Second}

// openaiRetryMax : un Retry-After plus long n'est pas attendu (quota).
const openaiRetryMax = 10 * time.Second

// post envoie une requête, avec de nouvelles tentatives sur une erreur
// passagère.
func (p *openaiProvider) post(ctx context.Context, cred Credentials, path string, body []byte) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		hreq, err := p.newRequest(ctx, cred, http.MethodPost, path, body)
		if err != nil {
			return nil, err
		}
		hreq.Header.Set("Accept", "text/event-stream")
		resp, err := openaiHTTP.Do(hreq)
		// Modèle local injoignable : il n'est pas lancé, l'attente n'y fait rien.
		if attempt >= len(openaiRetryWait) || ctx.Err() != nil || (err != nil && p.local) {
			return resp, err
		}
		wait := openaiRetryWait[attempt]
		if err == nil {
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
				return resp, nil
			}
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				d, perr := time.ParseDuration(strings.TrimSpace(ra) + "s")
				if perr != nil || d > openaiRetryMax {
					return resp, nil
				}
				wait = max(wait, d)
			}
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
			resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

type openaiProvider struct {
	id   string
	name string
	// label : nom du fournisseur dans les messages d'erreur.
	label string
	// base : adresse publique de l'API (sans /chat/completions).
	base    string
	keyURL  string
	keyHint string
	// models : liste de repli, quand celle du fournisseur n'a pas été lue.
	models       []Model
	defaultModel string
	// local : modèle sur ce poste (ai.localUrl), sans clé.
	local bool
	// streamUsage : demander les jetons consommés en fin de flux
	// (stream_options) ; Mistral refuse les champs qu'il ne connaît pas et
	// donne de toute façon l'usage dans le dernier morceau.
	streamUsage bool
	// shortIDs : identifiants d'appel de 9 caractères alphanumériques
	// (Mistral).
	shortIDs bool
	// signature : appels d'outils sans signature de réflexion (venus d'un
	// autre fournisseur) marqués pour que Gemini ne les refuse pas.
	signature bool

	mu    sync.Mutex
	cache modelCache
}

// modelCache : dernière liste de modèles lue chez le fournisseur.
type modelCache struct {
	key    string // adresse et empreinte de la clé
	at     time.Time
	models []Model
}

func init() {
	for _, p := range openaiProviders() {
		Register(p)
	}
}

// openaiProviders : les fournisseurs compatibles OpenAI. Modèles de repli
// relevés le 18/10/2026 (documentation des fournisseurs) ; la liste
// affichée est lue chez le fournisseur dès qu'une clé est là.
func openaiProviders() []*openaiProvider {
	return []*openaiProvider{
		{
			id: "mistral", name: "Mistral AI", label: "Mistral AI",
			base:   "https://api.mistral.ai/v1",
			keyURL: "https://console.mistral.ai/api-keys",
			models: []Model{
				{ID: "mistral-medium-latest", Name: "Mistral Medium", Note: "rapide et polyvalent"},
				{ID: "mistral-large-latest", Name: "Mistral Large", Note: "le plus capable"},
				{ID: "mistral-small-latest", Name: "Mistral Small", Note: "le plus rapide et le moins cher"},
			},
			defaultModel: "mistral-medium-latest",
			shortIDs:     true,
		},
		{
			id: "openai", name: "OpenAI (GPT)", label: "OpenAI",
			base:    "https://api.openai.com/v1",
			keyURL:  "https://platform.openai.com/api-keys",
			keyHint: "sk-",
			models: []Model{
				{ID: "gpt-5.5", Name: "GPT-5.5", Note: "le plus capable"},
				{ID: "gpt-5.4", Name: "GPT-5.4", Note: "polyvalent"},
				{ID: "gpt-5.4-mini", Name: "GPT-5.4 mini", Note: "rapide et économique"},
			},
			defaultModel: "gpt-5.4",
			streamUsage:  true,
		},
		{
			id: "gemini", name: "Google Gemini", label: "Gemini",
			base:    "https://generativelanguage.googleapis.com/v1beta/openai",
			keyURL:  "https://aistudio.google.com/apikey",
			keyHint: "AIza",
			models: []Model{
				{ID: "gemini-3.5-flash", Name: "Gemini 3.5 Flash", Note: "rapide et polyvalent"},
				{ID: "gemini-3.5-flash-lite", Name: "Gemini 3.5 Flash-Lite", Note: "le plus rapide et le moins cher"},
			},
			defaultModel: "gemini-3.5-flash",
			signature:    true,
		},
		{
			// Agrégateur (modèles de plusieurs éditeurs, un seul abonnement) ;
			// sa liste change souvent : celle lue chez lui fait foi.
			id: "mammouth", name: "Mammouth AI", label: "Mammouth AI",
			base:   "https://api.mammouth.ai/v1",
			keyURL: "https://mammouth.ai/app/account/settings/api",
			models: []Model{
				{ID: "gpt-4.1", Name: "GPT-4.1"},
				{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6"},
			},
			defaultModel: "gpt-4.1",
		},
		{
			id: "local", name: "Modèle local (Ollama, LM Studio…)", label: "le modèle local",
			models: []Model{
				{ID: "qwen3", Name: "Qwen 3", Note: "à installer : ollama pull qwen3"},
			},
			defaultModel: "qwen3",
			local:        true,
			streamUsage:  true,
		},
	}
}

func (p *openaiProvider) Info() Info {
	info := Info{
		ID:           p.id,
		Name:         p.name,
		Available:    true,
		Models:       p.models,
		DefaultModel: p.defaultModel,
		KeyURL:       p.keyURL,
		KeyHint:      p.keyHint,
	}
	p.mu.Lock()
	if len(p.cache.models) > 0 {
		info.Models = p.cache.models
	}
	p.mu.Unlock()
	info.DefaultModel = defaultIn(info.Models, p.models, p.defaultModel)
	info.NeedsKey, info.Local = !p.local, p.local
	return info
}

// defaultIn : le modèle par défaut s'il est dans la liste lue, sinon le
// premier modèle de repli présent, sinon le premier de la liste.
func defaultIn(list, fallback []Model, def string) string {
	has := func(id string) bool {
		return slices.ContainsFunc(list, func(m Model) bool { return m.ID == id })
	}
	if len(list) == 0 || has(def) {
		return def
	}
	for _, m := range fallback {
		if has(m.ID) {
			return m.ID
		}
	}
	return list[0].ID
}

// baseURL : la passerelle imposée, sinon l'adresse du modèle local ou
// l'adresse publique du fournisseur.
func (p *openaiProvider) baseURL(cred Credentials) string {
	b := p.base
	switch {
	case cred.Endpoint != "":
		b = cred.Endpoint
	case p.local:
		b = LocalURL()
		if b == "" {
			b = defaultLocalURL
		}
	}
	return strings.TrimRight(b, "/")
}

// newRequest prépare une requête vers l'API, clé comprise.
func (p *openaiProvider) newRequest(ctx context.Context, cred Credentials, method, path string, body []byte) (*http.Request, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL(cred)+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cred.Key != "" {
		req.Header.Set("Authorization", "Bearer "+cred.Key)
	}
	req.Header.Set("User-Agent", "Lattis")
	return req, nil
}

// --- Format de l'API -------------------------------------------------------

type oaMessage struct {
	Role string `json:"role"`
	// Content : absent pour un message de l'assistant qui n'a que des
	// appels d'outils.
	Content    *string      `json:"content,omitempty"`
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type oaToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function oaFunctionCall `json:"function"`
	// ExtraContent : signature de réflexion de Gemini, à lui renvoyer.
	ExtraContent json.RawMessage `json:"extra_content,omitempty"`
}

type oaFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaTool struct {
	Type     string     `json:"type"`
	Function oaFunction `json:"function"`
}

type oaFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type oaRequest struct {
	Model         string      `json:"model"`
	Messages      []oaMessage `json:"messages"`
	Tools         []oaTool    `json:"tools,omitempty"`
	Stream        bool        `json:"stream"`
	StreamOptions any         `json:"stream_options,omitempty"`
}

// oaChunk est un morceau du flux (ou la réponse entière, pour un serveur
// qui ignore stream : Message à la place de Delta).
type oaChunk struct {
	Choices []struct {
		Delta        oaDelta `json:"delta"`
		Message      oaDelta `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error json.RawMessage `json:"error"`
}

type oaDelta struct {
	// Content : texte, ou liste de morceaux (Mistral, modèles qui
	// raisonnent : seuls les morceaux de type text sont gardés).
	Content   json.RawMessage `json:"content"`
	Refusal   string          `json:"refusal"`
	ToolCalls []struct {
		Index    *int   `json:"index"`
		ID       string `json:"id"`
		Function struct {
			Name string `json:"name"`
			// Arguments : texte JSON (en morceaux), ou objet chez certains
			// serveurs locaux.
			Arguments json.RawMessage `json:"arguments"`
		} `json:"function"`
		ExtraContent json.RawMessage `json:"extra_content"`
	} `json:"tool_calls"`
}

// text : le texte d'un morceau.
func (d oaDelta) text() string {
	if len(d.Content) == 0 || d.Content[0] == 'n' {
		return ""
	}
	var s string
	if json.Unmarshal(d.Content, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	_ = json.Unmarshal(d.Content, &parts)
	var b strings.Builder
	for _, x := range parts {
		if x.Type == "text" {
			b.WriteString(x.Text)
		}
	}
	return b.String()
}

// --- Un tour ----------------------------------------------------------------

func (p *openaiProvider) Turn(ctx context.Context, cred Credentials, req Request, emit func(Event)) (Reply, error) {
	if cred.Key == "" && !p.local {
		return Reply{}, ErrNoKey
	}
	model := req.Model
	if model == "" {
		model = p.Info().DefaultModel
	}
	body := oaRequest{Model: model, Messages: p.messages(req.System, req.Messages), Stream: true}
	for _, t := range req.Tools {
		body.Tools = append(body.Tools, oaTool{Type: "function", Function: oaFunction{
			Name: t.Name, Description: t.Description, Parameters: objectSchema(t.Schema),
		}})
	}
	if p.streamUsage {
		body.StreamOptions = map[string]bool{"include_usage": true}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return Reply{}, err
	}
	resp, err := p.post(ctx, cred, "/chat/completions", b)
	if err != nil {
		return Reply{}, p.netError(ctx, cred, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Reply{}, p.httpError(resp, model)
	}

	var (
		text    strings.Builder
		calls   []*callAcc
		byIndex = map[int]*callAcc{}
		finish  string
		refusal bool
		usage   [2]int
	)
	handle := func(c oaChunk) error {
		if len(c.Error) > 0 && string(c.Error) != "null" {
			return fmt.Errorf("erreur de %s : %s", p.label, errorText(c.Error))
		}
		if c.Usage != nil {
			usage = [2]int{c.Usage.PromptTokens, c.Usage.CompletionTokens}
		}
		for _, ch := range c.Choices {
			d := ch.Delta
			if len(d.Content) == 0 && d.ToolCalls == nil && d.Refusal == "" {
				d = ch.Message
			}
			if t := d.text(); t != "" {
				text.WriteString(t)
				emit(Event{Kind: EventText, Text: t})
			}
			if d.Refusal != "" {
				refusal = true
			}
			for _, tc := range d.ToolCalls {
				var acc *callAcc
				if tc.Index != nil {
					acc = byIndex[*tc.Index]
					// Même index pour deux appels entiers (Gemini) : un
					// nouvel identifiant ouvre un nouvel appel.
					if acc != nil && tc.ID != "" && acc.id != "" && tc.ID != acc.id {
						acc = nil
					}
				} else if n := len(calls); tc.ID == "" && n > 0 && (tc.Function.Name == "" || calls[n-1].name == "") {
					// Sans index ni identifiant : suite du dernier appel,
					// sauf s'il apporte un nouveau nom.
					acc = calls[n-1]
				}
				if acc == nil {
					acc = &callAcc{}
					calls = append(calls, acc)
					if tc.Index != nil {
						byIndex[*tc.Index] = acc
					}
				}
				if tc.ID != "" {
					acc.id = tc.ID
				}
				if acc.name == "" {
					acc.name = tc.Function.Name
				}
				acc.addArgs(tc.Function.Arguments)
				if len(tc.ExtraContent) > 0 && string(tc.ExtraContent) != "null" {
					acc.extra = tc.ExtraContent
				}
			}
			if ch.FinishReason != "" {
				finish = ch.FinishReason
			}
		}
		return nil
	}

	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		// Serveur qui ignore stream : la réponse entière d'un coup.
		var c oaChunk
		if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
			return Reply{}, fmt.Errorf("réponse illisible de %s : %w", p.label, err)
		}
		if err := handle(c); err != nil {
			return Reply{}, err
		}
	} else if err := readSSE(resp.Body, func(data []byte) error {
		var c oaChunk
		if err := json.Unmarshal(data, &c); err != nil {
			return fmt.Errorf("réponse illisible de %s : %w", p.label, err)
		}
		return handle(c)
	}); err != nil {
		if ctx.Err() != nil {
			return Reply{}, ctx.Err()
		}
		return Reply{}, err
	}

	stop := StopEnd
	switch {
	case finish == "error":
		return Reply{}, fmt.Errorf("%s a interrompu sa réponse (erreur du fournisseur) : réessayez", p.label)
	case finish == "length":
		// Appels tronqués : arguments incomplets, rien à exécuter.
		stop, calls = StopMaxTokens, nil
	case finish == "content_filter" || refusal:
		stop = StopRefusal
	case len(calls) > 0:
		stop = StopToolUse
	}

	out := Message{Role: Assistant, Provider: p.id}
	native := oaMessage{Role: Assistant}
	if s := text.String(); s != "" {
		out.Content = append(out.Content, Block{Type: TextBlock, Text: s})
		native.Content = &s
	} else if len(calls) == 0 {
		native.Content = new(string)
	}
	for _, c := range calls {
		if c.id == "" {
			c.id = randomCallID()
		}
		args := c.input()
		out.Content = append(out.Content, Block{Type: ToolUseBlock, ID: c.id, Name: c.name, Input: args})
		native.ToolCalls = append(native.ToolCalls, oaToolCall{
			ID: c.id, Type: "function", ExtraContent: c.extra,
			Function: oaFunctionCall{Name: c.name, Arguments: string(args)},
		})
	}
	out.Native = native
	reply := Reply{Message: out, Stop: stop}
	reply.Usage = Usage{InputTokens: int64(usage[0]), OutputTokens: int64(usage[1])}
	return reply, nil
}

// callAcc : un appel d'outil en cours de réception.
type callAcc struct {
	id, name string
	args     strings.Builder
	extra    json.RawMessage
}

// addArgs ajoute un morceau d'arguments : texte JSON (le cas courant) ou
// objet entier.
func (c *callAcc) addArgs(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var s string
	if raw[0] == '"' && json.Unmarshal(raw, &s) == nil {
		c.args.WriteString(s)
		return
	}
	c.args.Write(raw)
}

// input : les arguments reçus, en JSON valide (« {} » s'il n'y en a pas ;
// un texte invalide est passé comme chaîne JSON, l'outil le refusera et le
// modèle pourra corriger).
func (c *callAcc) input() json.RawMessage {
	s := strings.TrimSpace(c.args.String())
	if s == "" {
		return json.RawMessage("{}")
	}
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	b, _ := json.Marshal(s)
	return b
}

const idChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// randomCallID : identifiant d'un appel que le fournisseur n'a pas nommé
// (9 caractères alphanumériques, accepté partout).
func randomCallID() string {
	b := make([]byte, 9)
	for i := range b {
		b[i] = idChars[rand.IntN(len(idChars))]
	}
	return string(b)
}

var shortID = regexp.MustCompile(`^[a-zA-Z0-9]{9}$`)

// callID : identifiant d'appel tel que le fournisseur l'accepte. Mistral
// veut 9 caractères alphanumériques : un autre identifiant est converti,
// toujours de la même façon (appel et résultat restent appariés).
func (p *openaiProvider) callID(id string) string {
	if !p.shortIDs || shortID.MatchString(id) {
		return id
	}
	h := sha256.Sum256([]byte(id))
	b := make([]byte, 9)
	for i := range b {
		b[i] = idChars[int(h[i])%len(idChars)]
	}
	return string(b)
}

// objectSchema : le schéma JSON des paramètres d'un outil, de type object
// (exigé par l'API ; le schéma partagé n'est pas modifié).
func objectSchema(s map[string]any) map[string]any {
	if s == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if _, ok := s["type"]; ok {
		return s
	}
	out := make(map[string]any, len(s)+1)
	for k, v := range s {
		out[k] = v
	}
	out["type"] = "object"
	return out
}

// geminiSkipSignature : signature factice documentée par Google pour un
// appel d'outil qui n'a pas été produit par Gemini (conversation commencée
// avec un autre fournisseur).
var geminiSkipSignature = json.RawMessage(`{"google":{"thought_signature":"skip_thought_signature_validator"}}`)

// messages traduit la conversation : un message rendu par ce fournisseur
// lui est renvoyé tel quel (signatures comprises), les autres sont
// convertis. Les résultats d'outils deviennent des messages role=tool,
// avant le texte de l'utilisateur qui les accompagne.
func (p *openaiProvider) messages(system string, msgs []Message) []oaMessage {
	out := make([]oaMessage, 0, len(msgs)+1)
	if system != "" {
		out = append(out, oaMessage{Role: "system", Content: &system})
	}
	for _, m := range msgs {
		if n, ok := m.Native.(oaMessage); ok && m.Provider == p.id {
			out = append(out, n)
			continue
		}
		var text []string
		if m.Role == Assistant {
			msg := oaMessage{Role: Assistant}
			for _, b := range m.Content {
				switch b.Type {
				case TextBlock:
					if b.Text != "" {
						text = append(text, b.Text)
					}
				case ToolUseBlock:
					args := string(b.Input)
					if args == "" {
						args = "{}"
					}
					tc := oaToolCall{ID: p.callID(b.ID), Type: "function", Function: oaFunctionCall{Name: b.Name, Arguments: args}}
					if p.signature {
						tc.ExtraContent = geminiSkipSignature
					}
					msg.ToolCalls = append(msg.ToolCalls, tc)
				}
			}
			if len(text) > 0 {
				s := strings.Join(text, "\n\n")
				msg.Content = &s
			}
			if msg.Content != nil || len(msg.ToolCalls) > 0 {
				out = append(out, msg)
			}
			continue
		}
		for _, b := range m.Content {
			switch b.Type {
			case ToolResultBlock:
				s := b.Text
				if b.IsError {
					s = "Erreur : " + s
				}
				out = append(out, oaMessage{Role: "tool", ToolCallID: p.callID(b.ToolUseID), Content: &s})
			case TextBlock:
				if b.Text != "" {
					text = append(text, b.Text)
				}
			}
		}
		if len(text) > 0 {
			s := strings.Join(text, "\n\n")
			out = append(out, oaMessage{Role: User, Content: &s})
		}
	}
	return out
}

// readSSE lit un flux d'évènements (Server-Sent Events) et passe le contenu
// de chaque ligne data: à fn, jusqu'à [DONE] ou la fin du flux.
func readSSE(r io.Reader, fn func(data []byte) error) error {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadBytes('\n')
		line = bytes.TrimRight(line, "\r\n")
		if data, ok := bytes.CutPrefix(line, []byte("data:")); ok {
			data = bytes.TrimSpace(data)
			if string(data) == "[DONE]" {
				return nil
			}
			if len(data) > 0 {
				if ferr := fn(data); ferr != nil {
					return ferr
				}
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("réponse interrompue : %w", err)
		}
	}
}

// --- Modèles et vérification de la clé --------------------------------------

// ListModels lit les modèles proposés par le fournisseur (GET /models),
// gardés quelques minutes ; ceux qui ne servent pas à la conversation avec
// outils sont écartés quand le fournisseur le dit (Mistral) ou que leur nom
// l'indique (plongements, voix, images…).
func (p *openaiProvider) ListModels(ctx context.Context, cred Credentials) ([]Model, error) {
	key := p.cacheKey(cred)
	p.mu.Lock()
	if p.cache.key == key && time.Since(p.cache.at) < modelsTTL && len(p.cache.models) > 0 {
		out := p.cache.models
		p.mu.Unlock()
		return out, nil
	}
	p.mu.Unlock()
	return p.fetchModels(ctx, cred)
}

// CheckKey vérifie la clé (ou, pour le modèle local, que le serveur
// répond) en lisant la liste des modèles.
func (p *openaiProvider) CheckKey(ctx context.Context, cred Credentials) error {
	_, err := p.fetchModels(ctx, cred)
	return err
}

func (p *openaiProvider) cacheKey(cred Credentials) string {
	h := sha256.Sum256([]byte(cred.Key))
	return p.baseURL(cred) + "\x00" + string(h[:8])
}

func (p *openaiProvider) fetchModels(ctx context.Context, cred Credentials) ([]Model, error) {
	if cred.Key == "" && !p.local {
		return nil, ErrNoKey
	}
	ctx, cancel := context.WithTimeout(ctx, openaiListTimeout)
	defer cancel()
	req, err := p.newRequest(ctx, cred, http.MethodGet, "/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := openaiHTTP.Do(req)
	if err != nil {
		return nil, p.netError(ctx, cred, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, p.httpError(resp, "")
	}
	var list struct {
		Data []struct {
			ID           string          `json:"id"`
			Name         string          `json:"name"`
			Deprecation  json.RawMessage `json:"deprecation"`
			Capabilities *struct {
				CompletionChat  *bool `json:"completion_chat"`
				FunctionCalling *bool `json:"function_calling"`
			} `json:"capabilities"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&list); err != nil {
		return nil, fmt.Errorf("liste des modèles illisible chez %s : %w", p.label, err)
	}
	seen := map[string]bool{}
	var out []Model
	for _, d := range list.Data {
		id := strings.TrimPrefix(d.ID, "models/") // Gemini
		if id == "" || seen[id] || !chatModel(id) {
			continue
		}
		if c := d.Capabilities; c != nil && (c.CompletionChat != nil && !*c.CompletionChat || c.FunctionCalling != nil && !*c.FunctionCalling) {
			continue
		}
		if len(d.Deprecation) > 0 && string(d.Deprecation) != "null" {
			continue
		}
		seen[id] = true
		m := Model{ID: id, Name: id}
		if i := slices.IndexFunc(p.models, func(f Model) bool { return f.ID == id }); i >= 0 {
			m = p.models[i]
		}
		out = append(out, m)
	}
	// Les modèles de repli (connus, décrits) d'abord, puis les autres par
	// ordre alphabétique.
	rank := func(id string) int {
		if i := slices.IndexFunc(p.models, func(f Model) bool { return f.ID == id }); i >= 0 {
			return i
		}
		return len(p.models)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := rank(out[i].ID), rank(out[j].ID)
		if ri != rj {
			return ri < rj
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > 0 {
		p.mu.Lock()
		p.cache = modelCache{key: p.cacheKey(cred), at: time.Now(), models: out}
		p.mu.Unlock()
	}
	return out, nil
}

// notChat : morceaux de noms de modèles qui ne font pas de conversation
// avec outils (plongements, modération, voix, images, vidéo, recherche…).
var notChat = []string{"embed", "moderation", "tts", "whisper", "transcri", "dall-e", "gpt-image", "imagen",
	"veo", "audio", "realtime", "ocr", "-search", "aqa", "sora", "babbage", "davinci", "computer-use", "image-generation"}

func chatModel(id string) bool {
	l := strings.ToLower(id)
	for _, s := range notChat {
		if strings.Contains(l, s) {
			return false
		}
	}
	return true
}

// --- Erreurs ---------------------------------------------------------------

// unreachableError : message lisible, reconnu par errors.Is(err,
// ErrUnreachable).
type unreachableError struct {
	msg string
	err error
}

func (e *unreachableError) Error() string        { return e.msg }
func (e *unreachableError) Unwrap() error        { return e.err }
func (e *unreachableError) Is(target error) bool { return target == ErrUnreachable }

// netError : erreur de connexion, lisible.
func (p *openaiProvider) netError(ctx context.Context, cred Credentials, err error) error {
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		if errors.Is(err, context.DeadlineExceeded) {
			return &unreachableError{msg: fmt.Sprintf("%s ne répond pas (délai dépassé)", p.label), err: err}
		}
		return ctx.Err()
	}
	if p.local && cred.Endpoint == "" {
		return &unreachableError{msg: fmt.Sprintf("Le modèle local ne répond pas à %s : lancez Ollama ou LM Studio", p.baseURL(cred)), err: err}
	}
	return &unreachableError{msg: fmt.Sprintf("%s injoignable : %v", p.label, err), err: err}
}

// httpError rend une réponse d'erreur de l'API lisible par l'utilisateur
// (comme anthropicError) ; model : le modèle demandé ("" pour la liste des
// modèles).
func (p *openaiProvider) httpError(resp *http.Response, model string) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	msg := errorText(b)
	low := strings.ToLower(msg)
	detail := ""
	if msg != "" {
		detail = " (" + msg + ")"
	}
	switch code := resp.StatusCode; {
	case code == 401,
		code == 400 && (strings.Contains(low, "api key") || strings.Contains(low, "api_key")): // Gemini : 400 pour une clé invalide
		if p.local {
			return fmt.Errorf("le modèle local demande une clé d'API%s", detail)
		}
		return fmt.Errorf("clé d'API refusée par %s : vérifiez-la (bouton « Compte »)", p.label)
	case code == 403:
		return fmt.Errorf("accès refusé par %s (droits de la clé ou du compte)%s", p.label, detail)
	case code == 402,
		code == 429 && (strings.Contains(low, "quota") || strings.Contains(low, "credit") || strings.Contains(low, "billing") || strings.Contains(low, "balance")):
		return fmt.Errorf("crédits épuisés chez %s : rechargez votre compte ou vérifiez votre forfait%s", p.label, detail)
	case code == 429:
		return fmt.Errorf("limite d'utilisation atteinte chez %s : réessayez dans un moment", p.label)
	case code == 404:
		switch {
		case model == "":
			return fmt.Errorf("adresse de l'API introuvable chez %s : vérifiez l'adresse%s", p.label, detail)
		case p.local:
			return fmt.Errorf("modèle « %s » absent du modèle local : installez-le (ollama pull %s) ou choisissez-en un autre", model, model)
		}
		return fmt.Errorf("modèle « %s » inconnu de %s : choisissez-en un autre", model, p.label)
	case code >= 500:
		return fmt.Errorf("%s est surchargé ou indisponible : réessayez dans un moment", capitalize(p.label))
	}
	return fmt.Errorf("erreur de %s (%d)%s", p.label, resp.StatusCode, detail)
}

// errorText extrait le message d'une erreur de l'API : {"error":{"message"}},
// {"error":"…"}, {"message"}, {"detail"}, ou une liste (Gemini) ; à défaut,
// le début du texte reçu.
func errorText(b []byte) string {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '<' { // page HTML (proxy, passerelle) : illisible
		return ""
	}
	if len(b) > 0 && b[0] == '[' {
		var l []json.RawMessage
		if json.Unmarshal(b, &l) == nil && len(l) > 0 {
			return errorText(l[0])
		}
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		return s
	}
	var e struct {
		Error   json.RawMessage `json:"error"`
		Message any             `json:"message"`
		Detail  any             `json:"detail"`
	}
	if json.Unmarshal(b, &e) == nil {
		if len(e.Error) > 0 && string(e.Error) != "null" {
			return errorText(e.Error)
		}
		for _, v := range []any{e.Message, e.Detail} {
			switch x := v.(type) {
			case string:
				return x
			case nil:
			default:
				j, _ := json.Marshal(x)
				return truncate(string(j))
			}
		}
		return ""
	}
	return truncate(string(b))
}

func truncate(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > 300 {
		return string(r[:300]) + "…"
	}
	return string(r)
}

func capitalize(s string) string {
	r := []rune(s)
	if len(r) > 0 {
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	}
	return string(r)
}

var (
	_ KeyChecker  = (*openaiProvider)(nil)
	_ ModelLister = (*openaiProvider)(nil)
)
