package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// choiceSheet : la feuille 1 porte le tableau Feuille1 (Année, Produit,
// Montant) sur deux années ; des formules libres à droite ; la feuille 2
// cite le même tableau.
func choiceSheet(t *testing.T) (*fixture, int64) {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Année", "Produit", "Montant"}, {"2024", "A", "10"}, {"2024", "B", "5"}, {"2025", "A", "7"}, {"2025", "C", "3"}}); err != nil {
		t.Fatal(err)
	}
	if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	for i, f := range []string{`=SOMME(Feuille1[Montant])`, `=SOUS.TOTAL(9;Feuille1[Montant])`, `=ONGLET.CHOIX()`} {
		if _, err := d.SetCell(ctx, 1, i, 6, f); err != nil {
			t.Fatal(err)
		}
	}
	s2, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range []string{`=SOMME(Feuille1[Montant])`, `=SOUS.TOTAL(9;Feuille1[Montant])`, `=ONGLET.CHOIX()`} {
		if _, err := d.SetCell(ctx, s2, i, 0, f); err != nil {
			t.Fatal(err)
		}
	}
	return d, s2
}

// freeValues : valeurs des cellules libres d'une colonne de la feuille.
func freeValues(t *testing.T, d *fixture, sheet int64, col int) string {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range l.Cells {
		if c.Col == col {
			out = append(out, c.Value)
		}
	}
	return strings.Join(out, "|")
}

func rowsOf(t *testing.T, d *fixture) (shown, total int) {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return l.Tables[0].Rows, l.Tables[0].Total
}

func countWhere(t *testing.T, d *fixture, where string) int {
	t.Helper()
	var n int
	if err := d.read(context.Background(), func(t *tx) error { return t.row(`SELECT count(*) FROM t_1 WHERE ` + where).Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestSheetChoiceFilter : le choix de l'onglet filtre les tableaux de la
// feuille qui ont la colonne, les formules libres de la feuille (SOMME,
// SOUS.TOTAL, ONGLET.CHOIX) et ses TCD ; les autres feuilles voient tout.
func TestSheetChoiceFilter(t *testing.T) {
	ctx := context.Background()
	d, s2 := choiceSheet(t)
	before := dump(t, d)
	if got := freeValues(t, d, 1, 6); got != "25|25|#N/A" {
		t.Fatalf("sans choix : %s", got)
	}
	if err := d.SetSheetChoice(ctx, 1, "Feuille1", "Année", "2025"); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	ch := l.Sheets[0].Choice
	if ch == nil || ch.Name != "Année" || ch.Value != "2025" || ch.All || ch.Table != "Feuille1" {
		t.Fatalf("choix : %+v", ch)
	}
	if tb := l.Tables[0]; tb.Rows != 2 || tb.Total != 4 || !tb.Follows || tb.ChoiceCol != 0 || tb.Filter != "" {
		t.Fatalf("tableau : %+v", tb)
	}
	if got := freeValues(t, d, 1, 6); got != "10|10|2025" {
		t.Fatalf("feuille choisie : %s", got)
	}
	if got := freeValues(t, d, s2, 0); got != "25|25|#N/A" {
		t.Fatalf("autre feuille : %s", got)
	}
	// TCD de la feuille : filtre imposé ; ailleurs : toutes les lignes.
	if _, err := d.PlacePivot(ctx, 1, 10, 8, PivotConfig{Source: "Feuille1", RowFields: []string{"Produit"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PlacePivot(ctx, s2, 10, 0, PivotConfig{Source: "Feuille1", RowFields: []string{"Produit"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}); err != nil {
		t.Fatal(err)
	}
	pivot := func(sheet int64) string {
		l, _ := d.Layout(ctx, sheet)
		return fmt.Sprint(l.Pivots[0].Cells)
	}
	if got := pivot(1); got != "[[Produit Somme de Montant] [A 7] [C 3] [Total 10]]" {
		t.Fatalf("TCD de la feuille : %s", got)
	}
	if got := pivot(s2); got != "[[Produit Somme de Montant] [A 17] [B 5] [C 3] [Total 25]]" {
		t.Fatalf("TCD d'une autre feuille : %s", got)
	}
	// Le filtre imposé n'est pas enregistré avec le TCD.
	var cfg string
	d.read(ctx, func(t *tx) error { return t.row(`SELECT config FROM _lattis_pivots WHERE sheet = 1`).Scan(&cfg) })
	if strings.Contains(cfg, "choice") {
		t.Fatalf("filtre imposé enregistré : %s", cfg)
	}
	// Changer de valeur : de la navigation, hors journal.
	n := d.ProtectionChanges()
	if err := d.ChooseSheetValue(ctx, 1, "2024", false); err != nil {
		t.Fatal(err)
	}
	if d.ProtectionChanges() == n {
		t.Fatal("le choix ne rend pas le classeur modifié")
	}
	if got := freeValues(t, d, 1, 6); got != "15|15|2024" {
		t.Fatalf("2024 : %s", got)
	}
	if got := pivot(1); got != "[[Produit Somme de Montant] [A 10] [B 5] [Total 15]]" {
		t.Fatalf("TCD 2024 : %s", got)
	}
	d.bus.Undo(ctx) // le second TCD
	d.bus.Undo(ctx) // le premier
	if l, _ := d.Layout(ctx, 1); l.Sheets[0].Choice == nil || l.Sheets[0].Choice.Value != "2024" {
		t.Fatalf("l'annulation a touché au choix : %+v", l.Sheets[0].Choice)
	}
	// Le filtre de l'utilisateur s'ajoute au choix ; « Effacer le filtre » le laisse.
	if err := d.SetFilter(ctx, "Feuille1", 1, &ColumnFilter{Values: []string{"A"}}); err != nil {
		t.Fatal(err)
	}
	if shown, _ := rowsOf(t, d); shown != 1 {
		t.Fatalf("choix et filtre : %d", shown)
	}
	if got := freeValues(t, d, 1, 6); got != "15|10|2024" {
		t.Fatalf("SOUS.TOTAL avec filtre : %s", got)
	}
	if got := freeValues(t, d, s2, 0); got != "25|17|#N/A" {
		t.Fatalf("autre feuille avec filtre : %s", got)
	}
	// Sous un filtre de l'utilisateur, la règle ErrFiltered vaut toujours.
	if _, err := d.SetCell(ctx, 1, 5, 1, "Z"); !errors.As(err, new(ErrFiltered)) {
		t.Fatalf("saisie sous un tableau filtré : %v", err)
	}
	if err := d.ClearFilters(ctx, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	if shown, total := rowsOf(t, d); shown != 2 || total != 4 {
		t.Fatalf("après Effacer le filtre : %d/%d", shown, total)
	}
	// « (Toutes) » : plus de filtre, ONGLET.CHOIX() vide.
	if err := d.ChooseSheetValue(ctx, 1, "", true); err != nil {
		t.Fatal(err)
	}
	if shown, _ := rowsOf(t, d); shown != 4 {
		t.Fatalf("toutes : %d", shown)
	}
	if got := freeValues(t, d, 1, 6); got != "25|25|" {
		t.Fatalf("toutes : %s", got)
	}
	l, _ = d.Layout(ctx, 1)
	if !l.Tables[0].Follows || !l.Sheets[0].Choice.All {
		t.Fatalf("toutes : %+v %+v", l.Tables[0], l.Sheets[0].Choice)
	}
	// Retirer la catégorie : annulable.
	if err := d.ClearSheetChoice(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if l, _ := d.Layout(ctx, 1); l.Sheets[0].Choice != nil || l.Tables[0].Follows {
		t.Fatalf("choix retiré : %+v", l.Sheets[0].Choice)
	}
	d.bus.Undo(ctx)
	if l, _ := d.Layout(ctx, 1); l.Sheets[0].Choice == nil {
		t.Fatal("choix non rendu par l'annulation")
	}
	check(t, d, "choix")
	for range 3 { // effacer le filtre, filtre, choisir par
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}

// TestSheetChoiceNewRows : une ligne ajoutée sous un tableau qui suit le
// choix, dans ses lignes masquées ou par insertion reçoit la valeur choisie.
func TestSheetChoiceNewRows(t *testing.T) {
	ctx := context.Background()
	d, _ := choiceSheet(t)
	if err := d.SetSheetChoice(ctx, 1, "Feuille1", "Année", "2024"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	// Sous le tableau (sa dernière ligne, masquées comprises : ligne 5).
	if _, err := d.SetCell(ctx, 1, 5, 1, "D"); err != nil {
		t.Fatal(err)
	}
	if n := countWhere(t, d, `c_1 = 2024 AND c_2 = 'D'`); n != 1 {
		t.Fatalf("ligne saisie sous le tableau : %d", n)
	}
	if shown, total := rowsOf(t, d); shown != 3 || total != 5 {
		t.Fatalf("après saisie dessous : %d/%d", shown, total)
	}
	// Dans les lignes masquées (rang 3 des données : ligne 4) : une ligne
	// de plus, à la suite des lignes affichées.
	if _, err := d.SetCell(ctx, 1, 4, 1, "E"); err != nil {
		t.Fatal(err)
	}
	if n := countWhere(t, d, `c_1 = 2024 AND c_2 = 'E'`); n != 1 {
		t.Fatalf("ligne saisie dans les lignes masquées : %d", n)
	}
	if shown, total := rowsOf(t, d); shown != 4 || total != 6 {
		t.Fatalf("après saisie dans les lignes masquées : %d/%d", shown, total)
	}
	// Une case vidée dans les lignes masquées n'ajoute rien.
	if _, err := d.SetCell(ctx, 1, 6, 2, ""); err != nil {
		t.Fatal(err)
	}
	if _, total := rowsOf(t, d); total != 6 {
		t.Fatalf("case vide : %d lignes", total)
	}
	// Insertion en tête des lignes affichées.
	if err := d.InsertRow(ctx, "Feuille1", 0); err != nil {
		t.Fatal(err)
	}
	if n := countWhere(t, d, `c_1 = 2024`); n != 5 {
		t.Fatalf("ligne insérée : %d lignes de 2024", n)
	}
	// Collage dans les lignes masquées.
	if _, err := d.Paste(ctx, 1, 6, 1, [][]string{{"F", "1"}, {"G", "2"}}); err != nil {
		t.Fatal(err)
	}
	if n := countWhere(t, d, `c_1 = 2024 AND c_2 IN ('F', 'G')`); n != 2 {
		t.Fatalf("collage : %d", n)
	}
	if n := countWhere(t, d, `c_1 IS NULL`); n != 0 {
		t.Fatalf("lignes sans année : %d", n)
	}
	check(t, d, "nouvelles lignes")
	for range 4 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}

// TestSheetChoiceNewValue : « Nouvelle année… » ajoute la valeur à la liste
// de validation et règle l'onglet dessus (une étape d'annulation) ; à
// l'ouverture, une valeur sans ligne est remplacée par la dernière, sauf
// juste après « Nouvelle année… ».
func TestSheetChoiceNewValue(t *testing.T) {
	ctx := context.Background()
	d, _ := choiceSheet(t)
	if err := d.SetSheetChoice(ctx, 1, "Feuille1", "Année", "2025"); err != nil {
		t.Fatal(err)
	}
	vals, err := d.SheetChoiceValues(ctx, 1)
	if err != nil || fmt.Sprint(vals) != "[{2024 2} {2025 2}]" {
		t.Fatalf("valeurs : %v %v", vals, err)
	}
	if err := d.NewSheetChoiceValue(ctx, 1, "deux mille"); err == nil {
		t.Fatal("valeur hors du type de la colonne acceptée")
	}
	if err := d.NewSheetChoiceValue(ctx, 1, "2026"); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	if ch := l.Sheets[0].Choice; ch.Value != "2026" || !ch.Fresh || l.Tables[0].Rows != 0 {
		t.Fatalf("nouvelle année : %+v, %d lignes", ch, l.Tables[0].Rows)
	}
	if !strings.Contains(l.Tables[0].Columns[0].Style, `"dv":["2024","2025","2026"]`) {
		t.Fatalf("liste de validation : %s", l.Tables[0].Columns[0].Style)
	}
	if vals, _ = d.SheetChoiceValues(ctx, 1); fmt.Sprint(vals) != "[{2024 2} {2025 2} {2026 0}]" {
		t.Fatalf("valeurs : %v", vals)
	}
	// Juste après « Nouvelle année… », l'ouverture garde l'année vide.
	if err := d.SettleChoices(ctx); err != nil {
		t.Fatal(err)
	}
	if ch, _ := d.SheetChoice(ctx, 1); ch.Value != "2026" {
		t.Fatalf("après Nouvelle année, à l'ouverture : %+v", ch)
	}
	// Une nouvelle ligne de la feuille vide appartient à 2026.
	if _, err := d.SetCell(ctx, 1, 1, 1, "Z"); err != nil {
		t.Fatal(err)
	}
	if n := countWhere(t, d, `c_1 = 2026`); n != 1 {
		t.Fatalf("ligne de 2026 : %d", n)
	}
	d.bus.Undo(ctx)
	d.bus.Undo(ctx)
	l, _ = d.Layout(ctx, 1)
	if ch := l.Sheets[0].Choice; ch.Value != "2025" || ch.Fresh || strings.Contains(l.Tables[0].Columns[0].Style, "dv") {
		t.Fatalf("annulation de Nouvelle année : %+v %s", ch, l.Tables[0].Columns[0].Style)
	}
	// Une valeur choisie qui n'a plus rien à montrer : à l'ouverture, la
	// dernière valeur.
	if err := d.ChooseSheetValue(ctx, 1, "1999", false); err != nil {
		t.Fatal(err)
	}
	if err := d.SettleChoices(ctx); err != nil {
		t.Fatal(err)
	}
	if ch, _ := d.SheetChoice(ctx, 1); ch.Value != "2025" {
		t.Fatalf("valeur disparue, à l'ouverture : %+v", ch)
	}
}

// TestSheetChoiceRename : renommer la colonne de catégorie renomme la
// catégorie (l'onglet suit) ; un autre tableau de la feuille qui a une
// colonne de ce nom suit le choix, un tableau sans elle ne change pas.
func TestSheetChoiceRename(t *testing.T) {
	ctx := context.Background()
	d, _ := choiceSheet(t)
	// Deux autres tableaux : Dépenses (avec Année), Tarifs (sans).
	if _, err := d.Paste(ctx, 1, 10, 0, [][]string{{"Année", "Poste", "Coût"}, {"2024", "Loyer", "100"}, {"2025", "Loyer", "110"}, {"2025", "Eau", "5"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Paste(ctx, 1, 20, 0, [][]string{{"Produit", "Prix"}, {"A", "1"}, {"B", "2"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetSheetChoice(ctx, 1, "Feuille1", "Année", "2025"); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	if len(l.Tables) != 3 {
		t.Fatalf("tableaux : %d", len(l.Tables))
	}
	got := []string{}
	for _, tb := range l.Tables {
		got = append(got, fmt.Sprintf("%d/%d:%v", tb.Rows, tb.Total, tb.Follows))
	}
	if strings.Join(got, " ") != "2/4:true 2/3:true 2/2:false" {
		t.Fatalf("tableaux : %v", got)
	}
	// Renommer la colonne source : la catégorie suit.
	if _, err := d.SetCell(ctx, 1, 0, 0, "Exercice"); err != nil {
		t.Fatal(err)
	}
	if ch, _ := d.SheetChoice(ctx, 1); ch == nil || ch.Name != "Exercice" {
		t.Fatalf("après renommage : %+v", ch)
	}
	l, _ = d.Layout(ctx, 1)
	if !l.Tables[0].Follows || l.Tables[1].Follows {
		t.Fatalf("après renommage, suivent : %v %v", l.Tables[0].Follows, l.Tables[1].Follows)
	}
	// Feuille supprimée puis rendue : le choix revient avec elle.
	if err := d.DeleteSheet(ctx, 1); err != nil {
		t.Fatal(err)
	}
	d.bus.Undo(ctx)
	if ch, _ := d.SheetChoice(ctx, 1); ch == nil || ch.Value != "2025" {
		t.Fatalf("feuille rendue : %+v", ch)
	}
	cands, err := d.SheetChoiceCandidates(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(cands[:3]) != "[{Feuille1 Exercice 2} {Feuille1 Produit 3} {Feuille1 Montant 4}]" {
		t.Fatalf("colonnes proposées : %v", cands)
	}
	check(t, d, "renommage")
}
