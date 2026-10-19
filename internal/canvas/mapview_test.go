package canvas

import (
	"context"
	"testing"
)

// La vue d'une carte (zoom et déplacement) s'enregistre dans sa configuration,
// et s'annule comme tout réglage.
func TestMapViewSaved(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Latitude", "Longitude"}, {"Paris", "48.8566", "2.3522"}, {"Lyon", "45.764", "4.8357"}}); err != nil {
		t.Fatal(err)
	}
	cfg := ChartConfig{Source: "Feuille1", Kind: "map", X: "Ville", Series: []string{"Latitude", "Longitude"}}
	id, err := d.CreateChart(ctx, 1, 0, 5, cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MapView = &MapView{Lat: 47.1, Lon: 3.5, Zoom: 7.25}
	if err := d.SetChart(ctx, id, cfg); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	if v := l.Charts[0].Config.MapView; v == nil || *v != (MapView{Lat: 47.1, Lon: 3.5, Zoom: 7.25}) {
		t.Fatalf("vue enregistrée : %+v", v)
	}
	if len(l.Charts[0].Series) != 2 || *l.Charts[0].Series[0].Values[0] != 48.8566 {
		t.Fatalf("données de la carte : %+v", l.Charts[0].Series)
	}
	d.bus.Undo(ctx)
	l, _ = d.Layout(ctx, 1)
	if v := l.Charts[0].Config.MapView; v != nil {
		t.Fatalf("après annulation, vue %+v (attendu : ajustée aux points)", v)
	}
}
