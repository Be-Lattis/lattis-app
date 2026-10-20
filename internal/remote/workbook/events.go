package workbook

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"github.com/lalamefine/lattis/internal/msg"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Relais des changements de lattisd (GET <émetteur>/api/events, contrat
// client §11 ; audit du mode en ligne, constat 18). Au lieu d'une connexion
// PostgreSQL permanente par poste et par classeur ouvert (LISTEN), le poste
// suit un flux SSE : lattisd écoute PostgreSQL une seule fois pour tous.
// Un serveur sans relais (404) ou qui ne l'a pas encore prêt (503) : le
// poste écoute PostgreSQL comme avant.

// EventsServer est un Server dont Listen suit le relais de lattisd pour un
// classeur (les autres méthodes sont celles de Server).
type EventsServer struct {
	Server
	// URL : adresse du flux (<émetteur>/api/events).
	URL string
	// Workbook : identifiant du classeur suivi.
	Workbook string
	// Token rend un jeton d'accès valide (rafraîchi au besoin) ; Invalidate
	// oublie le jeton refusé.
	Token      func(ctx context.Context) (string, error)
	Invalidate func()
	Client     *http.Client
	// OnPresence reçoit les membres qui suivent le classeur (événement
	// presence du relais ; le poste y est lui-même).
	OnPresence func([]Present)
	// none : le serveur n'a pas de relais (404) : LISTEN pour de bon.
	none *atomic.Bool
}

// Present : un membre qui suit le classeur (relais de lattisd).
type Present struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// WithEvents rend srv avec Listen par le relais de lattisd (voir plus haut).
func WithEvents(srv Server, eventsURL, workbookID string, token func(ctx context.Context) (string, error), invalidate func()) *EventsServer {
	return &EventsServer{Server: srv, URL: eventsURL, Workbook: workbookID, Token: token, Invalidate: invalidate, none: new(atomic.Bool)}
}

// errEventsReset : lattisd a pu perdre des changements (reset) ou le jeton
// du flux a expiré : le suivi reprend comme après une coupure (tout est
// relu à la reconnexion).
var errEventsReset = msg.New("relay.reset", nil)

// errNoRelay : pas de relais sur ce serveur (ou pas prêt) : LISTEN.
var errNoRelay = msg.New("relay.unavailable", nil)

// Listen suit le flux du classeur ; channel n'est utilisé qu'en repli
// (LISTEN sur PostgreSQL).
func (s *EventsServer) Listen(ctx context.Context, channel string, ready func(), fn func(string)) error {
	if s.none.Load() || s.URL == "" {
		return s.Server.Listen(ctx, channel, ready, fn)
	}
	err := s.follow(ctx, ready, fn)
	// restart : ce lattisd s'arrête pour une mise à jour (contrat client
	// §11) ; on se réabonne aussitôt, la demande arrive à un autre lattisd.
	// Pendant l'arrêt, 503 et Retry-After : on attend ce qu'il dit, sans
	// passer à l'écoute PostgreSQL ni signaler de déconnexion.
	for waited := time.Duration(0); errors.Is(err, errRestart); {
		err = s.follow(ctx, ready, fn)
		var busy *retryAfter
		for errors.As(err, &busy) && waited < restartPatience {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(busy.d):
			}
			waited += busy.d
			err = s.follow(ctx, ready, fn)
		}
		if errors.As(err, &busy) {
			err = errNoRelay
		}
	}
	var busy *retryAfter
	if errors.Is(err, errNoRelay) || errors.As(err, &busy) {
		return s.Server.Listen(ctx, channel, ready, fn)
	}
	return err
}

// errRestart : le lattisd du flux s'arrête (événement restart).
var errRestart = errors.New("relais : lattisd redémarre")

// restartPatience borne l'attente d'un autre lattisd après un restart.
const restartPatience = time.Minute

// retryAfter : 503 du relais, à réessayer après d (Retry-After).
type retryAfter struct{ d time.Duration }

func (e *retryAfter) Error() string { return "relais : réessayer dans " + e.d.String() }

// retryDelay lit Retry-After (secondes), borné entre 1 et 30 s.
func retryDelay(h string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(h))
	if err != nil || n < 1 {
		return time.Second
	}
	return min(time.Duration(n)*time.Second, 30*time.Second)
}

func (s *EventsServer) follow(ctx context.Context, ready func(), fn func(string)) error {
	tok, err := s.Token(ctx)
	if err != nil {
		return err
	}
	u, err := url.Parse(s.URL)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("workbooks", s.Workbook)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "text/event-stream")
	hc := s.Client
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		s.none.Store(true)
		return errNoRelay
	case http.StatusServiceUnavailable:
		return &retryAfter{d: retryDelay(resp.Header.Get("Retry-After"))}
	case http.StatusUnauthorized:
		if s.Invalidate != nil {
			s.Invalidate()
		}
		return msg.New("relay.tokenRefused", msg.Args{"status": resp.Status})
	case http.StatusForbidden:
		// Classeur inconnu ou interdit : la relecture des droits le dira
		// (GoneReason) ; l'écoute directe donne le même résultat.
		return errNoRelay
	default:
		return msg.New("relay.error", msg.Args{"status": resp.Status})
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	var kind, data string
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if kind != "" {
				if err := s.event(kind, data, ready, fn); err != nil {
					return err
				}
			}
			kind, data = "", ""
		case strings.HasPrefix(line, ":"):
			// Battement (« : ping »).
		case strings.HasPrefix(line, "event:"):
			kind = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data += strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return msg.New("relay.closed", nil)
}

// event traite un événement du flux.
func (s *EventsServer) event(kind, data string, ready func(), fn func(string)) error {
	var e struct {
		Workbook string    `json:"workbook"`
		Payload  string    `json:"payload"`
		Users    []Present `json:"users"`
	}
	_ = json.Unmarshal([]byte(data), &e)
	switch kind {
	case "ready":
		if ready != nil {
			ready()
		}
	case "change":
		if e.Workbook == "" || e.Workbook == s.Workbook {
			fn(e.Payload)
		}
	case "presence":
		if s.OnPresence != nil && (e.Workbook == "" || e.Workbook == s.Workbook) {
			s.OnPresence(e.Users)
		}
	case "revoked":
		// Accès retiré : la relecture des droits le constate.
		fn(NoticeACL)
	case "reset", "expired":
		return errEventsReset
	case "restart":
		return errRestart
	}
	return nil
}
