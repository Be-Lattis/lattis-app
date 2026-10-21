package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/script"
)

// Requêtes HTTP et HTTPS des scripts (module http) et des pages HTML.
//
// Sécurité : un classeur ne joint un hôte qu'avec l'accord de
// l'utilisateur. Avant la première requête vers un hôte, une boîte de
// dialogue le lui demande (Autoriser / Refuser, « mémoriser pour ce
// classeur ») ; la réponse vaut pour la session, ou est rangée sur ce
// poste, hors du fichier, pour ce classeur (doc.Document.AllowHTTPHost,
// internal/trust) : un classeur reçu ne s'autorise rien lui-même, et
// l'onglet Script › Accès réseau permet de revoir la liste. Une
// redirection vers un autre hôte est soumise à la même règle.
// Le serveur MCP ne peut pas consulter l'utilisateur (WithoutPrompt) : il
// n'atteint que les hôtes déjà autorisés. Aucune commande ne permet à un
// script d'autoriser un hôte lui-même.
//
// Une requête suit le contexte du script : le bouton Arrêter l'interrompt.
// L'attente du réseau ne compte pas dans la durée maximale du script (elle a
// son propre délai, 15 s par défaut, 120 s au plus).

// Bornes des requêtes.
const (
	httpDefaultTimeout = 15 * time.Second
	httpMaxTimeout     = 120 * time.Second
	httpMaxBody        = 10 << 20 // octets lus au plus dans une réponse
	httpMaxRedirects   = 10
)

// httpTransport fait les requêtes ; les tests le remplacent.
var httpTransport http.RoundTripper = http.DefaultTransport

type noPromptKey struct{}

// WithoutPrompt marque un contexte d'où l'on ne consulte pas l'utilisateur
// (serveur MCP) : une requête HTTP vers un hôte non autorisé y est refusée.
func WithoutPrompt(ctx context.Context) context.Context {
	return context.WithValue(ctx, noPromptKey{}, true)
}

// CanPrompt dit si l'utilisateur peut être consulté depuis ctx.
func CanPrompt(ctx context.Context) bool { return ctx.Value(noPromptKey{}) == nil }

// ErrHostNotAllowed refuse une requête vers un hôte non autorisé.
var ErrHostNotAllowed = msg.New("http.hostNotAllowed", nil)

// HTTPResponse est la réponse d'une requête.
type HTTPResponse struct {
	Status  int               `json:"status"`
	OK      bool              `json:"ok"`  // statut 2xx
	URL     string            `json:"url"` // adresse finale, après redirections
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	// JSON : corps décodé quand la réponse est du JSON (absent sinon).
	JSON any `json:"json,omitempty"`
}

type httpRequest struct {
	method  string
	url     string
	headers map[string]string
	body    []byte
	hasBody bool
	ctype   string // type par défaut du corps
	timeout time.Duration
}

// askHost sérialise les demandes d'autorisation : deux requêtes simultanées
// vers un même hôte ne posent qu'une question.
var askHost = make(chan struct{}, 1)

// allowHost vérifie que le classeur peut joindre host, en consultant
// l'utilisateur la première fois si ctx le permet.
func (c *Call) allowHost(host string) error {
	return c.allowHostFor(host, "Ce classeur veut envoyer des requêtes HTTP à « %s ». Ne l'autorisez que si vous faites confiance au classeur et à ce site : il pourra y envoyer des données du classeur.")
}

// allowHostFor est allowHost avec le message de la demande (%s : l'hôte).
func (c *Call) allowHostFor(host, message string) error {
	return c.allowHostAsk(host, func(h string) Dialog {
		return Dialog{
			Title:   "Accès au réseau",
			Message: fmt.Sprintf(message, h),
			Fields:  []Field{{Name: "remember", Label: "Mémoriser pour ce classeur", Type: "checkbox"}},
			OK:      "Autoriser",
			Cancel:  "Refuser",
		}
	})
}

// allowHostAsk est allowHost avec la boîte de la demande (ask : pour l'hôte
// normalisé ; la case « remember » mémorise l'accord pour le classeur).
func (c *Call) allowHostAsk(host string, ask func(h string) Dialog) error {
	d, err := c.Doc()
	if err != nil {
		return err
	}
	h := canvas.NormalizeHost(host)
	if h == "" {
		return msg.New("http.noHost", nil)
	}
	check := func() (bool, error) {
		ok, known, err := d.HTTPHostAllowed(c, h)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
		if known {
			return false, msg.Wrap(ErrHostNotAllowed, "http.hostRefusedSession", msg.Args{"host": h})
		}
		if !CanPrompt(c) {
			return false, msg.Wrap(ErrHostNotAllowed, "http.hostNotAllowedWorkbook", msg.Args{"host": h})
		}
		return false, nil
	}
	if ok, err := check(); ok || err != nil {
		return err
	}
	select {
	case askHost <- struct{}{}:
	case <-c.Done():
		return context.Cause(c)
	}
	defer func() { <-askHost }()
	if ok, err := check(); ok || err != nil { // réponse donnée pendant l'attente
		return err
	}
	vals, err := c.dialog(ask(h))
	if err != nil {
		return err
	}
	if vals == nil {
		d.SetSessionHost(h, false)
		return msg.Wrap(ErrHostNotAllowed, "http.hostRefusedByUser", msg.Args{"host": h})
	}
	if remember, _ := vals["remember"].(bool); remember {
		return d.AllowHTTPHost(c, h)
	}
	d.SetSessionHost(h, true)
	return nil
}

// doHTTP envoie une requête et lit sa réponse.
func (c *Call) doHTTP(r httpRequest) (HTTPResponse, error) {
	u, err := url.Parse(strings.TrimSpace(r.url))
	if err != nil {
		return HTTPResponse{}, msg.New("rights.invalidAddress", msg.Args{"address": err})
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return HTTPResponse{}, msg.New("http.schemeExpected", msg.Args{"url": fmt.Sprintf("%q", r.url)})
	}
	if err := c.allowHost(u.Host); err != nil {
		return HTTPResponse{}, err
	}
	timeout := r.timeout
	if timeout <= 0 {
		timeout = httpDefaultTimeout
	}
	timeout = min(timeout, httpMaxTimeout)
	client := &http.Client{
		Transport: httpTransport,
		Timeout:   timeout,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if len(via) >= httpMaxRedirects {
				return msg.New("http.tooManyRedirects", msg.Args{"max": httpMaxRedirects})
			}
			if next.URL.Scheme != "http" && next.URL.Scheme != "https" {
				return msg.New("http.redirectRefused", msg.Args{"url": fmt.Sprintf("%q", next.URL.String())})
			}
			return c.allowHost(next.URL.Host)
		},
	}
	// Version web : pas d'adresse interne du serveur (Guard.DialContext).
	if g, ok := c.Host.(query.Guarded); ok && g.QueryGuard() != nil {
		client.Transport = g.QueryGuard().Transport()
	}
	method := strings.ToUpper(strings.TrimSpace(r.method))
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if r.hasBody {
		body = bytes.NewReader(r.body)
	}
	req, err := http.NewRequestWithContext(c, method, u.String(), body)
	if err != nil {
		return HTTPResponse{}, err
	}
	req.Header.Set("User-Agent", "Lattis")
	if r.hasBody && r.ctype != "" {
		req.Header.Set("Content-Type", r.ctype)
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	// L'attente du réseau ne compte pas dans la durée maximale du script.
	defer script.Pause(c)()
	resp, err := client.Do(req)
	if err != nil {
		if cause := context.Cause(c); cause != nil {
			return HTTPResponse{}, cause
		}
		var ue *url.Error
		if errors.As(err, &ue) {
			if ue.Timeout() {
				return HTTPResponse{}, msg.New("http.timeout", msg.Args{"host": u.Host, "timeout": timeout})
			}
			err = ue.Err
		}
		return HTTPResponse{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, httpMaxBody+1))
	if err != nil {
		if cause := context.Cause(c); cause != nil {
			return HTTPResponse{}, cause
		}
		return HTTPResponse{}, msg.New("http.readError", msg.Args{"err": err})
	}
	if len(data) > httpMaxBody {
		return HTTPResponse{}, msg.New("http.tooLarge", msg.Args{"max": httpMaxBody >> 20})
	}
	out := HTTPResponse{
		Status:  resp.StatusCode,
		OK:      resp.StatusCode >= 200 && resp.StatusCode < 300,
		URL:     resp.Request.URL.String(),
		Headers: map[string]string{},
		Body:    string(data),
	}
	for k, v := range resp.Header {
		out.Headers[strings.ToLower(k)] = strings.Join(v, ", ")
	}
	if isJSON(resp.Header.Get("Content-Type")) && len(bytes.TrimSpace(data)) > 0 {
		var v any
		if json.Unmarshal(data, &v) == nil {
			out.JSON = v
		}
	}
	return out, nil
}

func isJSON(ctype string) bool {
	mt, _, err := mime.ParseMediaType(ctype)
	if err != nil {
		return false
	}
	return mt == "application/json" || strings.HasSuffix(mt, "+json") || mt == "text/json"
}

// httpOptions lit les options d'une requête : la table du paramètre name
// (http.get, http.post), ou les paramètres eux-mêmes (http.request).
func (c *Call) httpOptions(r *httpRequest, opts map[string]any) error {
	for k, v := range opts {
		if v == nil {
			continue
		}
		switch k {
		case "headers":
			h, ok := v.(map[string]any)
			if !ok {
				return msg.New("http.headersTable", nil)
			}
			r.headers = map[string]string{}
			names := make([]string, 0, len(h))
			for n := range h {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				r.headers[n] = raw(h[n])
			}
		case "timeout":
			f, ok := number(v)
			if !ok || f <= 0 {
				return msg.New("http.timeoutSeconds", nil)
			}
			r.timeout = time.Duration(f * float64(time.Second))
		case "body":
			if err := r.setBody(v); err != nil {
				return err
			}
		case "json":
			b, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("json : %v", err)
			}
			r.body, r.hasBody, r.ctype = b, true, "application/json"
		case "method":
			s, ok := v.(string)
			if !ok {
				return msg.New("http.methodText", nil)
			}
			r.method = s
		default:
			return msg.New("http.unknownOption", msg.Args{"option": fmt.Sprintf("%q", k)})
		}
	}
	return nil
}

// setBody prend un corps : un texte tel quel, une table encodée en JSON.
func (r *httpRequest) setBody(v any) error {
	switch b := v.(type) {
	case string:
		r.body, r.hasBody, r.ctype = []byte(b), true, "text/plain; charset=utf-8"
	case map[string]any, []any:
		j, err := json.Marshal(b)
		if err != nil {
			return fmt.Errorf("body : %v", err)
		}
		r.body, r.hasBody, r.ctype = j, true, "application/json"
	default:
		r.body, r.hasBody, r.ctype = []byte(raw(b)), true, "text/plain; charset=utf-8"
	}
	return nil
}

func (c *Call) optionsParam(name string) (map[string]any, error) {
	if !c.Has(name) {
		return nil, nil
	}
	m, ok := c.Raw(name).(map[string]any)
	if !ok {
		return nil, msg.New("api.optionsTable", msg.Args{"name": name})
	}
	return m, nil
}

const httpReturns = "{status, ok, url, headers, body, json}"

const httpDetails = "La réponse : status (200, 404…), ok (true pour un statut 2xx), url (adresse finale, après les redirections), headers (noms en minuscules), body (texte) et json (le corps décodé quand la réponse est du JSON). Un statut d'erreur (404, 500…) n'est pas une erreur Lua : testez ok ; une panne réseau ou un délai dépassé en est une (pcall pour la rattraper). Avant la première requête vers un hôte, Lattis demande l'autorisation à l'utilisateur (mémorisable pour le classeur ; onglet Script › Accès réseau pour la revoir) ; le serveur MCP n'atteint que les hôtes déjà autorisés. Délai : 15 s par défaut (timeout, 120 s au plus), sans compter dans la durée maximale du script ; réponse de 10 Mo au plus ; 10 redirections au plus ; le bouton Arrêter interrompt la requête."

var pOptions = Param{Name: "options", Type: Object, Optional: true, Desc: "{headers = {nom = valeur}, timeout = secondes}"}

func init() {
	register(
		&Command{
			Name:    "http.get",
			Summary: "Envoie une requête HTTP GET (http:// ou https://) et renvoie la réponse.",
			Details: httpDetails,
			Returns: httpReturns,
			Params:  []Param{{Name: "url", Type: String, Desc: "adresse (http:// ou https://)"}, pOptions},
			Example: `local r = http.get("https://api.example.com/taux?devise=USD")
if r.ok and r.json then
  print("Taux :", r.json.taux)
else
  print("Erreur", r.status, r.body)
end`,
			Run: func(c *Call) (any, error) {
				r := httpRequest{method: http.MethodGet}
				r.url, _ = c.Str("url")
				opts, err := c.optionsParam("options")
				if err != nil {
					return nil, err
				}
				if err := c.httpOptions(&r, opts); err != nil {
					return nil, err
				}
				return c.doHTTP(r)
			},
		},
		&Command{
			Name:    "http.post",
			Summary: "Envoie une requête HTTP POST et renvoie la réponse ; un corps texte part tel quel, une table part en JSON.",
			Details: httpDetails,
			Returns: httpReturns,
			Params: []Param{
				{Name: "url", Type: String, Desc: "adresse (http:// ou https://)"},
				{Name: "body", Type: String, Desc: "corps : texte, ou table encodée en JSON (Content-Type: application/json)", Optional: true},
				pOptions,
			},
			Example: `local r = http.post("https://api.example.com/commandes", {client = "Dupont", montant = 120})
print(r.status, r.ok)`,
			Run: func(c *Call) (any, error) {
				r := httpRequest{method: http.MethodPost}
				r.url, _ = c.Str("url")
				if c.Has("body") {
					if err := r.setBody(c.Raw("body")); err != nil {
						return nil, err
					}
				}
				opts, err := c.optionsParam("options")
				if err != nil {
					return nil, err
				}
				if err := c.httpOptions(&r, opts); err != nil {
					return nil, err
				}
				return c.doHTTP(r)
			},
		},
		&Command{
			Name:    "http.request",
			Summary: "Envoie une requête HTTP quelconque (GET, POST, PUT, PATCH, DELETE…) et renvoie la réponse.",
			Details: "json encode une table en corps JSON (avec l'en-tête Content-Type qui va bien) ; body envoie un texte tel quel. " + httpDetails,
			Returns: httpReturns,
			Params: []Param{
				{Name: "url", Type: String, Desc: "adresse (http:// ou https://)"},
				{Name: "method", Type: String, Desc: "méthode (GET par défaut)", Optional: true},
				{Name: "headers", Type: Object, Desc: "en-têtes : {nom = valeur}", Optional: true},
				{Name: "body", Type: String, Desc: "corps de la requête (texte)", Optional: true},
				{Name: "json", Type: Object, Desc: "corps de la requête : table encodée en JSON", Optional: true},
				{Name: "timeout", Type: Number, Desc: "délai en secondes (15 par défaut, 120 au plus)", Optional: true},
			},
			Example: `local jeton = "abc123"
local r = http.request{
  method = "PUT",
  url = "https://api.example.com/clients/42",
  headers = {Authorization = "Bearer " .. jeton},
  json = {nom = "Dupont", ville = "Lyon"},
  timeout = 30,
}
print(r.status, r.headers["content-type"])`,
			Run: func(c *Call) (any, error) {
				r := httpRequest{}
				r.url, _ = c.Str("url")
				opts := map[string]any{}
				for _, k := range []string{"method", "headers", "body", "json", "timeout"} {
					if c.Has(k) {
						opts[k] = c.Raw(k)
					}
				}
				if c.Has("body") && c.Has("json") {
					return nil, msg.New("http.bodyOrJson", nil)
				}
				if err := c.httpOptions(&r, opts); err != nil {
					return nil, err
				}
				return c.doHTTP(r)
			},
		},
	)
}
