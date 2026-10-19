package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/trust"
)

// Une source web d'un classeur reçu ne joint son hôte qu'avec l'accord de
// l'utilisateur, comme un script (audit 6, I4).
func TestQueryWebSourceNeedsConsent(t *testing.T) {
	realNet(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"a": 1}, {"a": 2}]`))
	}))
	defer srv.Close()
	ctx := context.Background()
	h := newHost(t)
	call(t, h, "queries.save", map[string]any{"name": "Web", "source": map[string]any{"kind": "web", "url": srv.URL + "/data"}})

	// Refus : rien n'est envoyé, la question n'est pas reposée.
	for range 2 {
		_, err := Invoke(ctx, h, "queries.refresh", map[string]any{"query": "Web"})
		if !errors.Is(err, ErrHostNotAllowed) {
			t.Fatalf("refus : %v", err)
		}
	}
	if len(h.dialogs) != 1 || !strings.Contains(h.dialogs[0].Message, "Une requête de ce classeur") || hits.Load() != 0 {
		t.Fatalf("%d questions, %d requêtes", len(h.dialogs), hits.Load())
	}

	// Serveur MCP : pas de question, refus.
	h2 := newHost(t)
	call(t, h2, "queries.save", map[string]any{"name": "Web", "source": map[string]any{"kind": "web", "url": srv.URL + "/data"}})
	if _, err := Invoke(WithoutPrompt(ctx), h2, "queries.refresh", map[string]any{"query": "Web"}); !errors.Is(err, ErrHostNotAllowed) || len(h2.dialogs) != 0 {
		t.Fatalf("MCP : %v, %d questions", err, len(h2.dialogs))
	}
	// Accord : la requête est lue.
	h2.answer = map[string]any{}
	if _, err := Invoke(ctx, h2, "queries.refresh", map[string]any{"query": "Web"}); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 || len(h2.dialogs) != 1 {
		t.Fatalf("%d requêtes, %d questions", hits.Load(), len(h2.dialogs))
	}
	// Requête écrite sur ce poste (éditeur) : accordée d'office.
	h3 := newHost(t)
	call(t, h3, "queries.save", map[string]any{"name": "Web", "source": map[string]any{"kind": "web", "url": srv.URL + "/data"}})
	if err := GrantSource(ctx, h3.d, query.Access{Host: strings.TrimPrefix(srv.URL, "http://")}); err != nil {
		t.Fatal(err)
	}
	if _, err := Invoke(WithoutPrompt(ctx), h3, "queries.refresh", map[string]any{"query": "Web"}); err != nil || len(h3.dialogs) != 0 {
		t.Fatalf("source accordée : %v, %d questions", err, len(h3.dialogs))
	}
}

// Un fichier source demande l'accord ; « Mémoriser » le range sur ce poste.
func TestQueryFileSourceNeedsConsent(t *testing.T) {
	defer trust.Use(trust.New("", nil))()
	p := filepath.Join(t.TempDir(), "secret.csv")
	if err := os.WriteFile(p, []byte("Nom;Montant\nDurand;12\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	h := newHost(t)
	call(t, h, "queries.save", map[string]any{"name": "Fichier", "source": map[string]any{"kind": "csv", "path": p}})
	if _, err := Invoke(ctx, h, "queries.refresh", map[string]any{"query": "Fichier"}); !errors.Is(err, query.ErrDenied) {
		t.Fatalf("refus : %v", err)
	}
	if len(h.dialogs) != 1 || !strings.Contains(h.dialogs[0].Message, "secret.csv") {
		t.Fatalf("questions %+v", h.dialogs)
	}

	h2 := newHost(t)
	call(t, h2, "queries.save", map[string]any{"name": "Fichier", "source": map[string]any{"kind": "csv", "path": p}})
	h2.answer = map[string]any{"remember": true}
	if _, err := Invoke(ctx, h2, "queries.refresh", map[string]any{"query": "Fichier"}); err != nil {
		t.Fatal(err)
	}
	if !trust.Default().FileAllowed(h2.d.SecretScope(), p) {
		t.Fatal("accord non mémorisé")
	}
	if _, err := Invoke(WithoutPrompt(ctx), h2, "queries.refresh", map[string]any{"query": "Fichier"}); err != nil || len(h2.dialogs) != 1 {
		t.Fatalf("accord mémorisé : %v, %d questions", err, len(h2.dialogs))
	}
}
