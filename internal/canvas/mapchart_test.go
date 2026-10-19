package canvas

import (
	"context"
	"math"
	"testing"
)

func TestMapChart(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	rows := [][]string{
		{"Ville", "Position", "Latitude", "Longitude", "Habitants"},
		{"Paris", `48°51'24"N 2°21'03"E`, "48,8566", "2,3522", "2100000"},
		{"Lyon", "45.764, 4.8357", "45.764", "4.8357", "520000"},
		{"Brest", "POINT(-4.4861 48.3904)", "48.3904", "-4.4861", "140000"},
		{"Nulle part", "n'importe quoi", "abc", "2", "1"},
		{"Lune", "95, 0", "95", "0", "1"},
		{"", "", "", "", ""},
	}
	if _, err := d.Paste(ctx, 1, 0, 0, rows); err != nil {
		t.Fatal(err)
	}
	// Carte en attente : rien n'est dessiné, les colonnes sont proposées.
	info := d.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "map", MapDraft: true})
	if info.Error != "" || len(info.Series) != 0 || info.Guess == nil {
		t.Fatalf("en attente : %+v", info)
	}
	if g := *info.Guess; g.Mode != "pair" || g.Lat != "Latitude" || g.Lon != "Longitude" || g.Coord != "Position" || g.Name != "Ville" {
		t.Fatalf("colonnes proposées : %+v", g)
	}
	if _, err := d.CreateChart(ctx, 1, 0, 8, ChartConfig{Source: "Feuille1", Kind: "map", MapDraft: true}); err != nil {
		t.Fatalf("carte en attente refusée : %v", err)
	}
	near := func(p *float64, want float64) bool { return p != nil && math.Abs(*p-want) < 1e-6 }

	// Une colonne : toutes les écritures ; deux illisibles ignorées.
	info = d.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "map", MapCoords: "single", X: "Ville", Series: []string{"Position", "Habitants"}})
	if info.Error != "" || info.Invalid != 2 || len(info.Labels) != 3 || len(info.Series) != 3 || info.Guess == nil || info.Guess.Lat != "Latitude" {
		t.Fatalf("une colonne : %+v", info)
	}
	if info.Labels[2] != "Brest" || !near(info.Series[0].Values[0], 48+51/60.0+24/3600.0) || !near(info.Series[1].Values[2], -4.4861) || !near(info.Series[2].Values[1], 520000) {
		t.Fatalf("une colonne, valeurs : %+v %v %v", info.Labels, *info.Series[0].Values[0], *info.Series[1].Values[2])
	}

	// Deux colonnes : nombres ou texte décimal.
	info = d.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "map", X: "Ville", Series: []string{"Latitude", "Longitude"}})
	if info.Error != "" || info.Invalid != 2 || len(info.Series) != 2 || !near(info.Series[0].Values[0], 48.8566) || !near(info.Series[1].Values[2], -4.4861) {
		t.Fatalf("deux colonnes : %+v", info)
	}

	// Une colonne sans nom de points ; mode inconnu refusé.
	if info = d.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "map", MapCoords: "single", Series: []string{"Position"}}); info.Error != "" || info.Labels[0] != "" {
		t.Fatalf("sans nom : %+v", info)
	}
	if info = d.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "map", MapCoords: "triple", Series: []string{"Position"}}); info.Error == "" {
		t.Fatal("mode inconnu accepté")
	}
	if info = d.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "map", Series: []string{"Position"}}); info.Error == "" {
		t.Fatal("deux colonnes : une seule série acceptée")
	}
}

func TestMapGuess(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	// Rien de sûr : aucun nom ni contenu de coordonnées.
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes", "Coût"}, {"FR", "10", "4"}, {"IT", "5", "2"}}); err != nil {
		t.Fatal(err)
	}
	if g := d.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "map", MapDraft: true}).Guess; g == nil || g.Mode != "" || g.Name != "Pays" {
		t.Fatalf("rien : %+v", g)
	}
	// Une colonne au contenu de coordonnées, sans nom parlant.
	d2 := open(t)
	if _, err := d2.Paste(ctx, 1, 0, 0, [][]string{{"Site", "Où", "Nb"}, {"A", "geo:48.85,2.35", "1"}, {"B", `45°45'N 4°50'E`, "2"}}); err != nil {
		t.Fatal(err)
	}
	if g := d2.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "map", MapDraft: true}).Guess; g == nil || g.Mode != "single" || g.Coord != "Où" || g.Name != "Site" {
		t.Fatalf("une colonne : %+v", g)
	}
	// Deux colonnes repérées par leurs lettres ; « Longueur » n'est pas une longitude.
	d3 := open(t)
	if _, err := d3.Paste(ctx, 1, 0, 0, [][]string{{"Longueur", "Nord", "Est"}, {"3", `48°51'N`, `2°21'E`}, {"4", `45°45'N`, `4°50'O`}}); err != nil {
		t.Fatal(err)
	}
	if g := d3.ChartData(ctx, ChartConfig{Source: "Feuille1", Kind: "map", MapDraft: true}).Guess; g == nil || g.Mode != "pair" || g.Lat != "Nord" || g.Lon != "Est" {
		t.Fatalf("lettres : %+v", g)
	}
}
