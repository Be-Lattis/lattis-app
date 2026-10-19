package canvas

import (
	"context"
	"fmt"
	"testing"
)

// « Filtrer l'onglet par cette colonne » (en-tête d'une colonne de tableau)
// et « Filtrer l'onglet par ce champ » (champ d'un tableau croisé) : la
// feuille devient un onglet à choix réglé sur « (Toutes) », comme « Choisir
// par… ».

// TestSheetChoiceFromColumn : une feuille qui n'a qu'un tableau déjà rempli
// devient un onglet à choix par sa colonne Année ; une étape d'annulation.
func TestSheetChoiceFromColumn(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Année", "Produit", "Montant"}, {"2026", "A", "10"}, {"2026", "B", "5"}, {"2027", "A", "7"}}); err != nil {
		t.Fatal(err)
	}
	if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	cands, err := d.SheetChoiceCandidates(ctx, 1)
	if err != nil || fmt.Sprint(cands) != "[{Feuille1 Année 2} {Feuille1 Produit 2} {Feuille1 Montant 3}]" {
		t.Fatalf("colonnes proposées : %v %v", cands, err)
	}
	if err := d.SetSheetChoice(ctx, 1, "Feuille1", "Année", ""); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	if ch := l.Sheets[0].Choice; ch == nil || ch.Name != "Année" || !ch.All {
		t.Fatalf("choix : %+v", ch)
	}
	if tb := l.Tables[0]; tb.Rows != 3 || !tb.Follows {
		t.Fatalf("tableau sur (Toutes) : %+v", tb)
	}
	if vals, _ := d.SheetChoiceValues(ctx, 1); fmt.Sprint(vals) != "[{2026 2} {2027 1}]" {
		t.Fatalf("valeurs : %v", vals)
	}
	if err := d.ChooseSheetValue(ctx, 1, "2027", false); err != nil {
		t.Fatal(err)
	}
	if l, _ := d.Layout(ctx, 1); l.Tables[0].Rows != 1 {
		t.Fatalf("2027 : %d lignes", l.Tables[0].Rows)
	}
	d.bus.Undo(ctx)
	if l, _ := d.Layout(ctx, 1); l.Sheets[0].Choice != nil || l.Tables[0].Rows != 3 {
		t.Fatalf("annulation : %+v", l.Sheets[0].Choice)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}

// TestSheetChoiceFromPivotField : un TCD posé sur une autre feuille que son
// tableau source ; « Filtrer l'onglet par ce champ » (Année) fait de la
// feuille du TCD un onglet à choix : le TCD suit, le tableau source, sur sa
// feuille, n'est pas filtré. Les valeurs de l'onglet viennent de la source.
func TestSheetChoiceFromPivotField(t *testing.T) {
	ctx := context.Background()
	d, s2 := choiceSheet(t)
	if _, err := d.PlacePivot(ctx, s2, 10, 0, PivotConfig{Source: "Feuille1", RowFields: []string{"Produit"}, Cols: "Année", Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}); err != nil {
		t.Fatal(err)
	}
	pivot := func() string {
		l, _ := d.Layout(ctx, s2)
		return fmt.Sprint(l.Pivots[0].Cells)
	}
	all := pivot()
	before := dump(t, d)
	// La source du TCD est proposée par « Choisir par… » de sa feuille.
	cands, err := d.SheetChoiceCandidates(ctx, s2)
	if err != nil || fmt.Sprint(cands) != "[{Feuille1 Année 2} {Feuille1 Produit 3} {Feuille1 Montant 4}]" {
		t.Fatalf("colonnes proposées : %v %v", cands, err)
	}
	if err := d.SetSheetChoice(ctx, s2, "Feuille1", "Année", ""); err != nil {
		t.Fatal(err)
	}
	if got := pivot(); got != all {
		t.Fatalf("TCD sur (Toutes) : %s", got)
	}
	if vals, _ := d.SheetChoiceValues(ctx, s2); fmt.Sprint(vals) != "[{2024 2} {2025 2}]" {
		t.Fatalf("valeurs : %v", vals)
	}
	if err := d.ChooseSheetValue(ctx, s2, "2025", false); err != nil {
		t.Fatal(err)
	}
	if got := pivot(); got != "[[Produit 2025 Total] [A 7 7] [C 3 3] [Total 10 10]]" {
		t.Fatalf("TCD sur 2025 : %s", got)
	}
	// La feuille du tableau source n'a pas de choix : rien n'y est filtré.
	l, _ := d.Layout(ctx, 1)
	if l.Sheets[0].Choice != nil || l.Tables[0].Rows != 4 || l.Tables[0].Follows {
		t.Fatalf("feuille source : %+v %+v", l.Sheets[0].Choice, l.Tables[0])
	}
	// « Nouvelle année… » : le type de la source est vérifié, sa liste
	// (sur une autre feuille) n'est pas touchée.
	if err := d.NewSheetChoiceValue(ctx, s2, "deux mille"); err == nil {
		t.Fatal("valeur hors du type de la colonne source acceptée")
	}
	if err := d.NewSheetChoiceValue(ctx, s2, "2026"); err != nil {
		t.Fatal(err)
	}
	if l, _ := d.Layout(ctx, 1); l.Tables[0].Columns[0].Style != "" {
		t.Fatalf("liste de la source modifiée : %s", l.Tables[0].Columns[0].Style)
	}
	d.bus.Undo(ctx) // nouvelle année
	d.bus.Undo(ctx) // filtrer l'onglet
	if l, _ := d.Layout(ctx, s2); l.Sheets[1].Choice != nil {
		t.Fatalf("annulation : %+v", l.Sheets[1].Choice)
	}
	if got := pivot(); got != all {
		t.Fatalf("TCD après annulation : %s", got)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}

// TestPivotChoiceNoFollowElsewhere : « Suivre l'année » décoché sur le
// tableau source, sur sa propre feuille, n'empêche pas un TCD d'une autre
// feuille de suivre l'onglet de la sienne.
func TestPivotChoiceNoFollowElsewhere(t *testing.T) {
	ctx := context.Background()
	d, s2 := choiceSheet(t)
	if _, err := d.Paste(ctx, 1, 10, 0, [][]string{{"Année", "Poste", "Coût"}, {"2024", "Loyer", "100"}, {"2025", "Loyer", "110"}, {"2025", "Eau", "5"}}); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	dep := l.Tables[1].Name
	if err := d.SetSheetChoice(ctx, 1, "Feuille1", "Année", "2025"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetTableFollow(ctx, dep, false); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PlacePivot(ctx, s2, 10, 0, PivotConfig{Source: dep, RowFields: []string{"Poste"}, Values: []PivotValue{{Field: "Coût", Agg: "sum"}}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSheetChoice(ctx, s2, dep, "Année", "2024"); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, s2)
	if got := fmt.Sprint(l.Pivots[0].Cells); got != "[[Poste Somme de Coût] [Loyer 100] [Total 100]]" {
		t.Fatalf("TCD de l'autre feuille : %s", got)
	}
	if l, _ := d.Layout(ctx, 1); l.Tables[1].Rows != 3 || l.Tables[1].Follows {
		t.Fatalf("source qui ne suit pas : %+v", l.Tables[1])
	}
}
