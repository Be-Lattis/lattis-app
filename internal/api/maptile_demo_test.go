package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/query"
)

// demoTileHost : hôte sans réseau sauf le fond de carte (démo publique).
type demoTileHost struct {
	*fakeHost
	g *query.Guard
}

func (h demoTileHost) QueryGuard() *query.Guard { return h.g }

// Démo publique : le fond de carte est l'exception au « sans réseau » ; la
// tuile est servie sans question, toute autre connexion reste refusée.
func TestMapTileDemoAllowed(t *testing.T) {
	old := httpTransport
	t.Cleanup(func() { httpTransport = old })
	httpTransport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() != "tile.openstreetmap.org" {
			t.Errorf("requête %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(bytes.NewReader([]byte("png")))}, nil
	})
	g := &query.Guard{Offline: true, OfflineExcept: []string{"tile.openstreetmap.org"}}
	h := demoTileHost{newHost(t), g}
	h.answer = map[string]any{}
	v, err := Invoke(context.Background(), h, "charts.tile", map[string]any{"z": 3, "x": 5, "y": 2})
	if err != nil || !strings.HasPrefix(v.(string), "data:image/png;base64,") || len(h.dialogs) != 0 {
		t.Fatalf("tuile en démo : %v, %v, questions %+v", v, err, h.dialogs)
	}
	if err := g.Check(context.Background(), query.Access{Host: "example.com:443"}); err == nil {
		t.Fatal("autre hôte permis en démo")
	}
	if !g.Excepted("tile.openstreetmap.org:443") || g.Excepted("") {
		t.Fatal("Excepted")
	}
}
