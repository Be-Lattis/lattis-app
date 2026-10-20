package workbook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
// puis reset (reprise) ; jeton passé en Bearer.
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
	ready := false
	err := es.Listen(context.Background(), "wb_x", func() { ready = true }, func(p string) { got = append(got, p) })
	if !errors.Is(err, errEventsReset) || !ready || fmt.Sprint(got) != "[t1 *acl]" || base.called.Load() {
		t.Fatalf("err=%v ready=%v got=%v repli=%v", err, ready, got, base.called.Load())
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
