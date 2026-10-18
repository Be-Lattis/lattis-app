package canvas

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestMapColorByAndLocate(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	rows := [][]string{
		{"Ville", "Lat", "Lon", "Région", "Rang"},
		{"Paris", "48.8566", "2.3522", "Île-de-France", "1"},
		{"Lyon", "45.764", "4.8357", "auvergne", "2"},
		{"Brest", "48.3904", "-4.4861", "Bretagne", "10"},
		{"Rennes", "48.1173", "-1.6778", "Bretagne", "2"},
		{"Nulle part", "abc", "2", "Zzz", "3"},
		{"Vide", "47", "1", "", ""},
	}
	if _, err := d.Paste(ctx, 1, 2, 1, rows); err != nil {
		t.Fatal(err)
	}
	base := ChartConfig{Source: "Feuille1", Kind: "map", X: "Ville", Series: []string{"Lat", "Lon"}}
	info := d.ChartData(ctx, base)
	if info.Error != "" || len(info.Rows) != 5 || info.Categories != nil || info.Groups != nil {
		t.Fatalf("sans couleur : %+v", info)
	}
	cfg := base
	cfg.ColorBy = "Région"
	info = d.ChartData(ctx, cfg)
	// Valeurs triées sans tenir compte de la casse ; la ligne illisible
	// (Zzz) n'a pas de point, sa valeur n'apparaît pas ; vide : -1.
	if info.Error != "" || strings.Join(info.Categories, "|") != "auvergne|Bretagne|Île-de-France" {
		t.Fatalf("catégories : %+v", info)
	}
	if got := fmt.Sprint(info.Groups); got != "[2 0 1 1 -1]" {
		t.Fatalf("groupes : %s", got)
	}
	// Nombres triés par valeur, avant le texte.
	nums := cfg
	nums.ColorBy = "Rang"
	if r := d.ChartData(ctx, nums); strings.Join(r.Categories, "|") != "1|2|10" || fmt.Sprint(r.Groups) != "[0 1 2 1 -1]" {
		t.Fatalf("nombres : %+v %v", r.Categories, r.Groups)
	}
	bad := cfg
	bad.ColorBy = "Inconnue"
	if r := d.ChartData(ctx, bad); r.Error == "" {
		t.Fatal("colonne de couleur inconnue acceptée")
	}

	// Aller à la ligne d'un point : Brest (3e point), même après un tri et un filtre.
	brest := info.Rows[2]
	spot, err := d.LocateRow(ctx, "Feuille1", brest)
	if err != nil || spot.Sheet != 1 || spot.Row != 2+3 || spot.Col != 1 || spot.Cols != 5 || spot.Hidden {
		t.Fatalf("Brest : %+v %v", spot, err)
	}
	if err := d.SortTable(ctx, "Feuille1", 0, false); err != nil {
		t.Fatal(err)
	}
	// Brest, Lyon, Nulle part, Paris, Rennes, Vide.
	if spot, err = d.LocateRow(ctx, "Feuille1", brest); err != nil || spot.Row != 2+1 {
		t.Fatalf("Brest trié : %+v %v", spot, err)
	}
	if err := d.SetFilter(ctx, "Feuille1", 3, &ColumnFilter{Values: []string{"Bretagne"}}); err != nil {
		t.Fatal(err)
	}
	rennes := info.Rows[3]
	if spot, err = d.LocateRow(ctx, "Feuille1", rennes); err != nil || spot.Row != 2+2 || spot.Hidden {
		t.Fatalf("Rennes filtré : %+v %v", spot, err)
	}
	if spot, err = d.LocateRow(ctx, "Feuille1", info.Rows[0]); err != nil || !spot.Hidden {
		t.Fatalf("Paris masqué : %+v %v", spot, err)
	}
	if _, err = d.LocateRow(ctx, "Feuille1", 9999); err == nil {
		t.Fatal("ligne inconnue trouvée")
	}
}
