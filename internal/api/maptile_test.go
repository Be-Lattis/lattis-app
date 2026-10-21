package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestChartsNewKindsFromLua(t *testing.T) {
	h := newHost(t)
	run(t, h, `
cells.paste{row = 1, col = 1, values = {{"Ville", "Lat", "Lon", "Habitants"}, {"Paris", 48.8566, 2.3522, 2100000}, {"Lyon", 45.764, 4.8357, 520000}, {"Paris", 48.86, 2.35, 10}}}
local tb = sheets.get().tables[1].name
local a = charts.create(1, 8, {source = tb, kind = "combo", x = "Ville", series = {"Habitants", "Lat"}, seriesKinds = {"bar", "line"}, secondary = {false, true}})
local b = charts.create(20, 8, {source = tb, kind = "map", x = "Ville", series = {"Lat", "Lon", "Habitants"}, mapMode = "heat"})
for _, k in ipairs({"bar3d", "pie3d", "waterfall", "treemap", "radar", "funnel", "boxplot", "histogram", "gauge"}) do
  charts.set(a, {source = tb, kind = k, x = "Ville", series = {"Habitants"}})
end
print(#sheets.get().charts)
print(pcall(charts.create, 40, 8, {source = tb, kind = "map", x = "Ville", series = {"Lat"}}))
`)
	if len(h.prints) != 2 || h.prints[0] != "2" || !strings.Contains(h.prints[1], "false") || !strings.Contains(h.prints[1], "latitude") {
		t.Fatalf("sortie %q", h.prints)
	}
	l, err := h.d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	// La carte lit les lignes une à une : Paris deux fois, sans additionner les coordonnées.
	var m bool
	for _, c := range l.Charts {
		if c.Config.Kind == "map" {
			m = true
			if c.Error != "" || len(c.Labels) != 3 || *c.Series[0].Values[2] != 48.86 || c.Config.MapMode != "heat" {
				t.Fatalf("carte : %+v", c)
			}
		}
	}
	if !m {
		t.Fatal("carte absente")
	}
}

// resetTileCache vide le cache des tuiles, partagé par tout le processus
// (sinon un essai relancé, -count=2, trouve sa tuile déjà là).
func resetTileCache(t *testing.T) {
	reset := func() {
		tileMu.Lock()
		tileCache, tileOrder = map[string]string{}, nil
		tileMu.Unlock()
	}
	reset()
	t.Cleanup(reset)
}

func TestMapTilePermissionAndCache(t *testing.T) {
	old := httpTransport
	t.Cleanup(func() { httpTransport = old })
	resetTileCache(t)
	fetched := 0
	httpTransport = roundTrip(func(r *http.Request) (*http.Response, error) {
		fetched++
		if r.Host != "tile.openstreetmap.org" || r.URL.Path != "/3/4/2.png" || !strings.Contains(r.Header.Get("User-Agent"), "Lattis") {
			t.Errorf("requête %s %s", r.URL, r.Header.Get("User-Agent"))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(strings.NewReader("\x89PNG\x00\xff")), Request: r}, nil
	})
	ctx := context.Background()
	args := map[string]any{"z": 3, "x": 4, "y": 2}

	// Refus : pas de requête, l'interface garde un fond neutre.
	h := newHost(t)
	if _, err := Invoke(ctx, h, "charts.tile", args); !errors.Is(err, ErrHostNotAllowed) || fetched != 0 {
		t.Fatalf("refus : %v, %d requêtes", err, fetched)
	}
	if len(h.dialogs) != 1 || !strings.Contains(h.dialogs[0].Message, "tile.openstreetmap.org") {
		t.Fatalf("questions %+v", h.dialogs)
	}

	// Accord : data URL, puis le cache (mais l'autorisation reste vérifiée).
	h2 := newHost(t)
	h2.answer = map[string]any{}
	for range 2 {
		res, err := Invoke(ctx, h2, "charts.tile", args)
		if err != nil {
			t.Fatal(err)
		}
		if s := res.(string); s != "data:image/png;base64,iVBORwD/" {
			t.Fatalf("tuile %q", s)
		}
	}
	if fetched != 1 {
		t.Fatalf("%d requêtes", fetched)
	}
	if _, err := Invoke(ctx, h, "charts.tile", args); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("le cache contourne le refus : %v", err)
	}
	if _, err := Invoke(ctx, h2, "charts.tile", map[string]any{"z": 2, "x": 4, "y": 0}); err == nil {
		t.Fatal("tuile hors de la carte acceptée")
	}
	// Pas dans les scripts.
	res, err := RunScript(h2, "Test", `charts.tile(1, 0, 0)`, "")
	if err != nil || res.Error == "" {
		t.Fatalf("charts.tile dans un script : %+v %v", res, err)
	}
}
