package workbook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// listenStub : Server dont seul Listen sert (repli sur LISTEN).
type listenStub struct {
	Server
	called atomic.Bool
}

func (l *listenStub) Listen(ctx context.Context, channel string, ready func(), fn func(string)) error {
	l.called.Store(true)
	ready()
	fn("repli")
	return errors.New("fin du repli")
}

// Relais de lattisd : ready, changements du classeur suivi, accès retiré,
// puis reset (reprise) ; jeton passé en Bearer. Après un reset, le poste se
// réabonne aussitôt ; un flux qui se referme tout de suite, quickResets
// fois de suite, rend l'erreur (coupure).
func TestEventsRelay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer jeton" || r.URL.Query().Get("workbooks") != "wb1" {
			http.Error(w, "refusé", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: ready\ndata: {}\n\n: ping\n\n")
		fmt.Fprint(w, "event: change\ndata: {\"workbook\":\"wb1\",\"payload\":\"t1\",\"lsn\":\"0/1\"}\n\n")
		fmt.Fprint(w, "event: change\ndata: {\"workbook\":\"wb2\",\"payload\":\"autre\"}\n\n")
		fmt.Fprint(w, "event: revoked\ndata: {\"workbook\":\"wb1\"}\n\n")
		fmt.Fprint(w, "event: reset\ndata: {}\n\n")
	}))
	defer srv.Close()
	base := &listenStub{}
	es := WithEvents(base, srv.URL, "wb1", func(context.Context) (string, error) { return "jeton", nil }, nil)
	var got []string
	readies := 0
	err := es.Listen(context.Background(), "wb_x", func() { readies++ }, func(p string) { got = append(got, p) })
	want := strings.TrimSpace(strings.Repeat("t1 *acl ", quickResets+1))
	if !errors.Is(err, errEventsReset) || readies != quickResets+1 || fmt.Sprint(got) != "["+want+"]" || base.called.Load() {
		t.Fatalf("err=%v ready=%d got=%v repli=%v", err, readies, got, base.called.Load())
	}
}

// expired : lattisd ferme le flux à l'expiration du jeton ; le poste se
// réabonne aussitôt avec un jeton neuf, sans rendre d'erreur (le classeur
// ne passe pas hors ligne), et « ready » fait tout relire.
func TestEventsExpiredResubscribes(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if r.Header.Get("Authorization") != fmt.Sprintf("Bearer jeton%d", n) {
			http.Error(w, "refusé", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: ready\ndata: {}\n\nevent: change\ndata: {\"payload\":\"t%d\"}\n\n", n)
		if n == 1 {
			fmt.Fprint(w, "event: expired\ndata: {}\n\n")
			return
		}
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	var tokens atomic.Int32
	es := WithEvents(&listenStub{}, srv.URL, "wb1", func(context.Context) (string, error) {
		return fmt.Sprintf("jeton%d", tokens.Add(1)), nil
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got []string
	readies := 0
	err := es.Listen(ctx, "wb_x", func() { readies++ }, func(p string) {
		got = append(got, p)
		if p == "t2" {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || readies != 2 || fmt.Sprint(got) != "[t1 t2]" || hits.Load() != 2 {
		t.Fatalf("err=%v ready=%d got=%v appels=%d", err, readies, got, hits.Load())
	}
}

// Serveur sans relais (404) : LISTEN, et pour de bon.
func TestEventsFallback(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.NotFound(w, r)
	}))
	defer srv.Close()
	base := &listenStub{}
	es := WithEvents(base, srv.URL, "wb1", func(context.Context) (string, error) { return "jeton", nil }, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for range 2 {
		var got []string
		_ = es.Listen(ctx, "wb_x", func() {}, func(p string) { got = append(got, p) })
		if fmt.Sprint(got) != "[repli]" {
			t.Fatalf("repli : %v", got)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("le relais absent est redemandé : %d appels", hits.Load())
	}
}

// restart : lattisd s'arrête pour une mise à jour ; le poste se réabonne
// aussitôt, attend le Retry-After d'un 503 pendant l'arrêt, sans repli sur
// LISTEN, et reçoit la suite par le nouveau lattisd.
func TestEventsRestart(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch hits.Add(1) {
		case 1:
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: ready\ndata: {}\n\n")
			fmt.Fprint(w, "event: change\ndata: {\"workbook\":\"wb1\",\"payload\":\"avant\"}\n\n")
			fmt.Fprint(w, "event: restart\ndata: {}\n\n")
		case 2:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "arrêt", http.StatusServiceUnavailable)
		default:
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: ready\ndata: {}\n\n")
			fmt.Fprint(w, "event: change\ndata: {\"workbook\":\"wb1\",\"payload\":\"après\"}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}
	}))
	defer srv.Close()
	base := &listenStub{}
	es := WithEvents(base, srv.URL, "wb1", func(context.Context) (string, error) { return "jeton", nil }, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var got []string
	readies := 0
	start := time.Now()
	err := es.Listen(ctx, "wb_x", func() { readies++ }, func(p string) {
		got = append(got, p)
		if p == "après" {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || fmt.Sprint(got) != "[avant après]" || readies != 2 || base.called.Load() || hits.Load() != 3 {
		t.Fatalf("err=%v got=%v ready=%d repli=%v appels=%d", err, got, readies, base.called.Load(), hits.Load())
	}
	if d := time.Since(start); d < time.Second {
		t.Errorf("Retry-After non respecté : %v", d)
	}
}
