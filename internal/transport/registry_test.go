package transport

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type svc struct{}

func (*svc) Add(a, b int) int                                  { return a + b }
func (*svc) Fail(ctx context.Context) error                    { return errors.New("raté") }
func (*svc) Pair() (string, int)                               { return "a", 1 }
func (*svc) ServiceShutdown() error                            { return nil }
func (*svc) Ctx(ctx context.Context, s string) (string, error) { return s + "!", ctx.Err() }

func TestIDMatchesWails(t *testing.T) {
	// Identifiant de frontend/bindings/…/bridge/gridservice.ts (BenchRequested).
	if got := ID("github.com/lalamefine/lattis/internal/bridge.GridService.BenchRequested"); got != 2316013313 {
		t.Fatalf("ID = %d", got)
	}
}

func TestRegistryCall(t *testing.T) {
	r, err := NewRegistry(&svc{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Lookup("svc.ServiceShutdown"); ok {
		t.Fatal("ServiceShutdown lié")
	}
	add, ok := r.Lookup("svc.Add")
	if !ok {
		t.Fatal("svc.Add introuvable")
	}
	if m, ok := r.Lookup(jsonNumber(json.RawMessage(`` + itoa(add.ID)))); !ok || m != add {
		t.Fatal("recherche par identifiant")
	}
	ctx := context.Background()
	res, err := add.Call(ctx, []json.RawMessage{json.RawMessage("2"), json.RawMessage("3")})
	if err != nil || res != 5 {
		t.Fatalf("Add = %v, %v", res, err)
	}
	if _, err := add.Call(ctx, []json.RawMessage{json.RawMessage(`"x"`), json.RawMessage("3")}); err == nil {
		t.Fatal("paramètre illisible accepté")
	}
	m, _ := r.Lookup("svc.Fail")
	var ce *Error
	if _, err := m.Call(ctx, nil); !errors.As(err, &ce) || ce.Message != "raté" || ce.Code != CodeServer {
		t.Fatalf("Fail : %v", err)
	}
	m, _ = r.Lookup("svc.Pair")
	if res, _ := m.Call(ctx, nil); len(res.([]any)) != 2 {
		t.Fatalf("Pair = %v", res)
	}
	m, _ = r.Lookup("svc.Ctx")
	if res, err := m.Call(ctx, []json.RawMessage{json.RawMessage(`"a"`)}); err != nil || res != "a!" {
		t.Fatalf("Ctx = %v, %v", res, err)
	}
}

func itoa(n uint32) string {
	b, _ := json.Marshal(n)
	return string(b)
}
