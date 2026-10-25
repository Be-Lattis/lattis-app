package canvas

import (
	"context"
	"strings"
	"testing"
)

func sparks(t *testing.T, d *fixture) []SparklineInfo {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return l.Sparklines
}

// Plan d'un groupe de cellules libres : un graphique par ligne (ou colonne)
// de la source ; emplacement en ligne ou colonne, de la bonne taille.
func TestSparkPlan(t *testing.T) {
	at, from, err := sparkPlan(SparklineConfig{Kind: "line", Source: "B2:E4", Location: "F2:F4"})
	if err != nil || len(at) != 3 || at[2] != [2]int{3, 5} || len(from[2]) != 4 || from[2][0] != [2]int{3, 1} {
		t.Fatalf("par ligne : %v %v %v", at, from, err)
	}
	at, from, err = sparkPlan(SparklineConfig{Kind: "line", Source: "B2:D6", Location: "B7:D7"})
	if err != nil || len(at) != 3 || at[0] != [2]int{6, 1} || len(from[0]) != 5 || from[0][4] != [2]int{5, 1} {
		t.Fatalf("par colonne : %v %v %v", at, from, err)
	}
	if _, _, err := sparkPlan(SparklineConfig{Kind: "line", Source: "B2:E4", Location: "F2:F3"}); err == nil || !strings.Contains(err.Error(), "ne correspond pas") {
		t.Fatalf("tailles : %v", err)
	}
	if _, _, err := sparkPlan(SparklineConfig{Kind: "line", Source: "B2:E4", Location: "F2:G4"}); err == nil || !strings.Contains(err.Error(), "ligne ou une colonne") {
		t.Fatalf("emplacement carré : %v", err)
	}
	if _, _, err := sparkPlan(SparklineConfig{Kind: "line", Source: "B2:zz", Location: "F2"}); err == nil {
		t.Fatal("source illisible acceptée")
	}
}

// Cellules libres : valeurs lues avec la mise en page (formules comprises),
// échelle commune ; une étape d'annulation ; lignes insérées suivies ;
// un groupe posé sur un autre le remplace.
func TestSparklineFreeCells(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	for c, v := range []string{"1", "4", "texte", "=2*3"} {
		if _, err := d.SetCell(ctx, 1, 10, c, v); err != nil {
			t.Fatal(err)
		}
	}
	id, err := d.CreateSparkline(ctx, 1, SparklineConfig{Kind: "line", Source: "a11:d11", Location: "E11", High: true, Same: true})
	if err != nil {
		t.Fatal(err)
	}
	s := sparks(t, d)
	if len(s) != 1 || s[0].ID != id || s[0].Config.Source != "A11:D11" || len(s[0].Cells) != 1 {
		t.Fatalf("groupe : %+v", s)
	}
	c := s[0].Cells[0]
	if c.Row != 10 || c.Col != 4 || *c.Values[0] != 1 || c.Values[2] != nil || *c.Values[3] != 6 || *s[0].Hi != 6 {
		t.Fatalf("valeurs : %+v", c)
	}
	// Deux lignes insérées au-dessus : source et emplacement descendent.
	if err := d.InsertSheetRows(ctx, 1, 0, 2); err != nil {
		t.Fatal(err)
	}
	if s := sparks(t, d); s[0].Config.Source != "A13:D13" || s[0].Config.Location != "E13" || *s[0].Cells[0].Values[1] != 4 {
		t.Fatalf("après insertion : %+v", s[0])
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	// Un nouveau groupe sur la même cellule remplace l'ancien.
	if _, err := d.CreateSparkline(ctx, 1, SparklineConfig{Kind: "column", Source: "A11:C11", Location: "E11"}); err != nil {
		t.Fatal(err)
	}
	if s := sparks(t, d); len(s) != 1 || s[0].Config.Kind != "column" {
		t.Fatalf("remplacement : %+v", s)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if s := sparks(t, d); len(s) != 1 || s[0].ID != id {
		t.Fatalf("remplacement annulé : %+v", s)
	}
	cfg := s[0].Config
	cfg.Kind, cfg.Color = "winloss", "#ff0000"
	if err := d.SetSparkline(ctx, id, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Color = "rouge"
	if err := d.SetSparkline(ctx, id, cfg); err == nil {
		t.Fatal("couleur illisible acceptée")
	}
	if err := d.DeleteSparkline(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(sparks(t, d)) != 0 {
		t.Fatal("groupe non supprimé")
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if s := sparks(t, d); len(s) != 1 || s[0].Config.Kind != "winloss" {
		t.Fatalf("suppression annulée : %+v", s)
	}
	// La ligne de l'emplacement supprimée : le groupe part avec elle.
	if err := d.DeleteSheetRows(ctx, 1, 10, 10); err != nil {
		t.Fatal(err)
	}
	if len(sparks(t, d)) != 0 {
		t.Fatal("groupe resté sans emplacement")
	}
}

// Tableau : une ligne, un graphique, dans la colonne voulue ou juste à
// droite ; renommer le tableau et les colonnes suit ; copier la feuille
// copie le groupe sur la copie du tableau ; supprimer la feuille l'emporte.
func TestSparklineTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Jan", "Fév", "Mar"}, {"A", "1", "5", "3"}, {"B", "-2", "4", "8"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateSparkline(ctx, 1, SparklineConfig{Kind: "line", Table: "Feuille1", Cols: []string{"jan", "Fév", "Mar"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateSparkline(ctx, 1, SparklineConfig{Kind: "line", Table: "Feuille1", Cols: []string{"Jan", "Avr"}}); err == nil || !strings.Contains(err.Error(), "Avr") {
		t.Fatalf("colonne inconnue : %v", err)
	}
	s := sparks(t, d)
	if len(s) != 1 || s[0].AtCol != 4 || len(s[0].Src) != 3 || s[0].Src[0] != 1 || s[0].Config.Table != "Feuille1" || s[0].Config.Cols[0] != "Jan" {
		t.Fatalf("groupe du tableau : %+v", s)
	}
	cfg := s[0].Config
	cfg.At, cfg.Same = "produit", true
	if err := d.SetSparkline(ctx, s[0].ID, cfg); err != nil {
		t.Fatal(err)
	}
	if s := sparks(t, d); s[0].AtCol != 0 || *s[0].Lo != -2 || *s[0].Hi != 8 {
		t.Fatalf("colonne de dessin, échelle commune : %+v", s[0])
	}
	// Plage choisie à la main sur les cellules du tableau : ses valeurs.
	if _, err := d.CreateSparkline(ctx, 1, SparklineConfig{Kind: "column", Source: "B2:D3", Location: "F2:F3"}); err != nil {
		t.Fatal(err)
	}
	if s := sparks(t, d); len(s) != 2 || len(s[1].Cells) != 2 || *s[1].Cells[1].Values[0] != -2 || *s[1].Cells[0].Values[2] != 3 {
		t.Fatalf("plage sur un tableau : %+v", s[1].Cells)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 1, "Janvier"); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameTable(ctx, "Feuille1", "Ventes"); err != nil {
		t.Fatal(err)
	}
	if s := sparks(t, d); s[0].Config.Table != "Ventes" || s[0].Config.Cols[0] != "Janvier" || s[0].Error != "" {
		t.Fatalf("renommés : %+v", s[0])
	}
	cp, err := d.CopySheet(ctx, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, cp)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Sparklines) != 1 || l.Sparklines[0].Config.Table == "Ventes" || l.Sparklines[0].Error != "" {
		t.Fatalf("copie : %+v", l.Sparklines)
	}
	if err := d.DeleteSheet(ctx, cp); err != nil {
		t.Fatal(err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if l, err := d.Layout(ctx, cp); err != nil || len(l.Sparklines) != 1 {
		t.Fatalf("feuille rendue : %+v %v", l.Sparklines, err)
	}
}
