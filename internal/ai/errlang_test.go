package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
)

// Les erreurs des fournisseurs sont des messages du catalogue : l'interface
// les reçoit dans sa langue (anglais ici), le français restant le texte de
// Error() ; errors.Is(ErrUnreachable) tient toujours.
func TestProviderErrorsTranslated(t *testing.T) {
	defer func(w []time.Duration) { openaiRetryWait = w }(openaiRetryWait)
	openaiRetryWait = nil
	for _, tc := range []struct {
		id     string
		status int
		body   string
		fr, en string
	}{
		{"openai", 401, `{"error":{"message":"bad key"}}`, "clé d'API refusée par OpenAI", "API key rejected by OpenAI: check or replace it (“Connection” button)"},
		{"local", 503, `busy`, "Le modèle local est surchargé", "The local model is overloaded"},
		{"local", 403, `{"error":"no"}`, "accès refusé par le modèle local", "access denied by the local model (key or account permissions) (no)"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			io.WriteString(w, tc.body)
		}))
		_, err := openaiTestProvider(t, tc.id).Turn(context.Background(), Credentials{Key: "k", Endpoint: srv.URL}, Request{Model: "m"}, func(Event) {})
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), tc.fr) || !strings.Contains(msg.Text(msg.EN, err), tc.en) {
			t.Errorf("%s %d : %v / %s", tc.id, tc.status, err, msg.Text(msg.EN, err))
		}
	}
	wrapped := errors.Join(ErrNoKey)
	if got := msg.Text(msg.DE, wrapped); !strings.Contains(got, "„Verbindung“") {
		t.Errorf("ErrNoKey en allemand : %s", got)
	}
	n := Notice(msg.M("ai.stepLimit", msg.Args{"n": 3}))
	if n.Text != "Réponse arrêtée après 3 étapes." || n.Msg.Text(msg.EN) != "Reply stopped after 3 steps." {
		t.Errorf("remarque : %+v", n)
	}
}
