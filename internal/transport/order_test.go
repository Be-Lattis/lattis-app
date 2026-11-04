package transport

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type slow struct {
	mu  sync.Mutex
	log []string
}

func (s *slow) Long(ctx context.Context) {
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
	s.mu.Lock()
	s.log = append(s.log, "long")
	s.mu.Unlock()
}

func (s *slow) Quick(n string) string {
	s.mu.Lock()
	s.log = append(s.log, n)
	s.mu.Unlock()
	return n
}

func (s *slow) Free() string { return "free" }

// Un appel long n'est plus attendu passé Overtake ; un appel libre n'attend
// pas son tour.
func TestHubOvertakeAndFree(t *testing.T) {
	old := Overtake
	Overtake = 200 * time.Millisecond
	t.Cleanup(func() { Overtake = old })
	s := &slow{}
	reg, err := NewRegistry(s)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHub(reg)
	h.Free = func(m *Method) bool { return m.Short == "slow.Free" }
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	ctx := context.Background()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close(websocket.StatusNormalClosure, "") })
	send := func(id int, method string, params ...any) {
		if params == nil {
			params = []any{}
		}
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if err := ws.Write(ctx, websocket.MessageText, b); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	send(1, "slow.Long")
	send(2, "slow.Quick", "a")
	send(3, "slow.Free")
	send(4, "slow.Quick", "b")
	read := func() (string, time.Duration) {
		_, b, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Result string `json:"result"`
		}
		json.Unmarshal(b, &m)
		return m.Result, time.Since(start)
	}
	if r, d := read(); r != "free" || d > 150*time.Millisecond {
		t.Fatalf("appel libre : %q après %v", r, d)
	}
	if r, d := read(); r != "a" || d < 200*time.Millisecond || d > 2*time.Second {
		t.Fatalf("appel derrière un long : %q après %v", r, d)
	}
	if r, _ := read(); r != "b" {
		t.Fatalf("ordre : %q", r)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.Join(s.log, ",") != "a,b" {
		t.Fatalf("exécution : %v", s.log)
	}
}
