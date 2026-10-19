package api

import (
	"context"
	"testing"
)

// charts.set ne change que les champs donnés (replace : tout) ; charts.place
// sans taille garde celle du graphique.
func TestChartsSetMergesAndPlaceKeepsSize(t *testing.T) {
	h := newHost(t)
	run(t, h, `
cells.paste{row = 1, col = 1, values = {{"Ville", "Habitants", "Surface"}, {"Paris", 2100000, 105}, {"Lyon", 520000, 48}}}
local tb = sheets.get().tables[1].name
local a = charts.create(1, 8, {source = tb, kind = "bar", x = "Ville", series = {"Habitants", "Surface"}, colors = {"#112233", "#445566"}, secondary = {false, true}, width = 600, height = 300})
charts.set(a, {title = "Villes"})
charts.place(a, 4, 9)
`)
	l, err := h.d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	c := l.Charts[0].Config
	if c.Title != "Villes" || c.Kind != "bar" || len(c.Series) != 2 || len(c.Colors) != 2 || len(c.Secondary) != 2 || c.Width != 600 || c.Height != 300 {
		t.Fatalf("après set et place : %+v", c)
	}
	if l.Charts[0].Row != 3 || l.Charts[0].Col != 8 {
		t.Fatalf("place : %+v", l.Charts[0])
	}
	run(t, h, `
local tb = sheets.get().tables[1].name
charts.set("Villes", {source = tb, kind = "line", x = "Ville", series = {"Surface"}}, true)
print(pcall(charts.place, "Villes", 1, 1, 50, 50))
`)
	l, _ = h.d.Layout(context.Background(), 1)
	if c := l.Charts[0].Config; c.Title != "" || c.Width != 0 || c.Colors != nil || c.Kind != "line" {
		t.Fatalf("remplacement : %+v", c)
	}
	if len(h.prints) != 1 || h.prints[0][:5] != "false" {
		t.Fatalf("taille trop petite acceptée : %q", h.prints)
	}
}
