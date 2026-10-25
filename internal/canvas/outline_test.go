package canvas

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// outlineText résume le plan des lignes de la feuille 1 : « 2:1 3:2… »
// (ligne:niveau, * : réduit ici) et les lignes masquées.
func outlineText(t *testing.T, d *fixture, rows bool) string {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if l.Outline == nil {
		return "-"
	}
	lines, hidden := l.Outline.Rows, l.Outline.HiddenRows
	if !rows {
		lines, hidden = l.Outline.Cols, l.Outline.HiddenCols
	}
	var b strings.Builder
	for _, x := range lines {
		fmt.Fprintf(&b, "%d:%d", x.Index, x.Level)
		if x.Folded != 0 {
			fmt.Fprintf(&b, "*%d", x.Folded)
		}
		b.WriteByte(' ')
	}
	fmt.Fprintf(&b, "| %v", hidden)
	return b.String()
}

func TestOutlineGroupsPure(t *testing.T) {
	lines := []OutlineLine{{Index: 3, Level: 1}, {Index: 4, Level: 2}, {Index: 5, Level: 2, Folded: 2}, {Index: 6, Level: 1}, {Index: 9, Level: 1, Folded: 1}}
	got := OutlineGroups(lines, false)
	want := []OutlineGroup{{1, 3, 6, false}, {1, 9, 9, true}, {2, 4, 5, true}}
	if !slices.Equal(got, want) {
		t.Fatalf("groupes : %+v", got)
	}
	if h := OutlineHidden(lines, false); !slices.Equal(h, []int{4, 5, 9}) {
		t.Fatalf("masquées : %v", h)
	}
	// Synthèse au-dessus : l'état est noté sur la première ligne.
	if g := OutlineGroups([]OutlineLine{{Index: 2, Level: 1, Folded: 1}, {Index: 3, Level: 1}}, true); len(g) != 1 || !g[0].Folded {
		t.Fatalf("au-dessus : %+v", g)
	}
}

// Grouper, imbriquer, réduire et rouvrir : chaque action est une étape
// d'annulation ; le groupe intérieur garde son état sous l'extérieur.
func TestOutlineFoldUndo(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	start := dump(t, d)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(d.GroupLines(ctx, 1, true, 2, 6))
	must(d.GroupLines(ctx, 1, true, 3, 4))
	if got := outlineText(t, d, true); got != "2:1 3:2 4:2 5:1 6:1 | []" {
		t.Fatalf("groupés : %s", got)
	}
	steps := []string{dump(t, d)}
	must(d.FoldGroup(ctx, 1, true, 2, 3, true))
	if got := outlineText(t, d, true); got != "2:1 3:2 4:2*2 5:1 6:1 | [3 4]" {
		t.Fatalf("intérieur réduit : %s", got)
	}
	steps = append(steps, dump(t, d))
	must(d.FoldGroup(ctx, 1, true, 1, 2, true))
	if got := outlineText(t, d, true); got != "2:1 3:2 4:2*2 5:1 6:1*1 | [2 3 4 5 6]" {
		t.Fatalf("extérieur réduit : %s", got)
	}
	steps = append(steps, dump(t, d))
	must(d.FoldGroup(ctx, 1, true, 1, 6, false))
	if got := outlineText(t, d, true); got != "2:1 3:2 4:2*2 5:1 6:1 | [3 4]" {
		t.Fatalf("extérieur rouvert (intérieur gardé) : %s", got)
	}
	steps = append(steps, dump(t, d))
	must(d.ShowOutlineLevel(ctx, 1, true, 1))
	if got := outlineText(t, d, true); got != "2:1 3:2 4:2*2 5:1 6:1*1 | [2 3 4 5 6]" {
		t.Fatalf("niveau 1 : %s", got)
	}
	must(d.ShowOutlineLevel(ctx, 1, true, 3))
	if got := outlineText(t, d, true); got != "2:1 3:2 4:2 5:1 6:1 | []" {
		t.Fatalf("niveau 3 : %s", got)
	}
	must(d.UngroupLines(ctx, 1, true, 3, 3))
	if got := outlineText(t, d, true); got != "2:1 3:1 4:2 5:1 6:1 | []" {
		t.Fatalf("dissocié : %s", got)
	}
	// Annuler : Dissocier, niveau 3, niveau 1, puis chaque réduction.
	for range 3 {
		if _, err := d.bus.Undo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for i := len(steps) - 1; i >= 0; i-- {
		if got := dump(t, d); got != steps[i] {
			t.Fatalf("annulation %d : état différent", i)
		}
		if _, err := d.bus.Undo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := dump(t, d); got != start {
		t.Fatal("tout annulé : état différent du départ")
	}
	check(t, d, "plan annulé")
}

// Les groupes suivent les insertions et suppressions ; une ligne insérée
// dans un groupe y entre, une ligne insérée juste après n'y entre pas.
func TestOutlineInsertDelete(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if err := d.GroupLines(ctx, 1, true, 5, 7); err != nil {
		t.Fatal(err)
	}
	if err := d.FoldGroup(ctx, 1, true, 1, 5, true); err != nil {
		t.Fatal(err)
	}
	if err := d.InsertSheetRows(ctx, 1, 1, 2); err != nil {
		t.Fatal(err)
	}
	if got := outlineText(t, d, true); got != "7:1 8:1 9:1*1 | [7 8 9]" {
		t.Fatalf("après insertion au-dessus : %s", got)
	}
	if err := d.InsertSheetRows(ctx, 1, 8, 1); err != nil {
		t.Fatal(err)
	}
	if got := outlineText(t, d, true); got != "7:1 8:1 9:1 10:1*1 | [7 8 9 10]" {
		t.Fatalf("après insertion dans le groupe : %s", got)
	}
	if err := d.InsertSheetRows(ctx, 1, 11, 1); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSheetRows(ctx, 1, 0, 1); err != nil {
		t.Fatal(err)
	}
	if got := outlineText(t, d, true); got != "5:1 6:1 7:1 8:1*1 | [5 6 7 8]" {
		t.Fatalf("après suppression : %s", got)
	}
	// Colonnes : même chose.
	if err := d.GroupLines(ctx, 1, false, 2, 3); err != nil {
		t.Fatal(err)
	}
	if err := d.InsertSheetColumns(ctx, 1, 0, 1); err != nil {
		t.Fatal(err)
	}
	if got := outlineText(t, d, false); got != "3:1 4:1 | []" {
		t.Fatalf("colonnes : %s", got)
	}
	if err := d.ClearOutline(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := outlineText(t, d, true); got != "-" {
		t.Fatalf("plan effacé : %s", got)
	}
	check(t, d, "plan décalé")
}

// Les lignes d'un tableau ne se groupent pas ; ses colonnes si. Huit
// niveaux au plus.
func TestOutlineLimits(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.GroupLines(ctx, 1, true, 1, 2); err == nil || !strings.Contains(err.Error(), "Sous-total") {
		t.Fatalf("lignes de tableau groupées : %v", err)
	}
	if err := d.GroupLines(ctx, 1, false, 0, 1); err != nil {
		t.Fatalf("colonnes de tableau : %v", err)
	}
	for i := range MaxOutline {
		if err := d.GroupLines(ctx, 1, true, 10, 20-i); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.GroupLines(ctx, 1, true, 10, 11); err == nil {
		t.Fatal("neuvième niveau accepté")
	}
	if err := d.UngroupLines(ctx, 1, true, 30, 40); err == nil {
		t.Fatal("dissocier sans groupe accepté")
	}
}

// Sous-total : une feuille à côté, groupes à chaque changement, formules
// SOUS.TOTAL, plan 1 2 3 ; le tableau ne change pas ; Remplacer refait la
// même feuille.
func TestSubtotalSheet(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Région", "Ventes", "Qté"}, {"Nord", "10", "1"}, {"Nord", "5", "2"}, {"Sud", "7", "3"}}); err != nil {
		t.Fatal(err)
	}
	before := d.tableRows(t, "Feuille1")
	sheet, err := d.Subtotal(ctx, SubtotalOptions{Table: "Feuille1", By: "Région", Func: 9, Columns: []string{"Ventes"}, Below: true, PageBreaks: true})
	if err != nil {
		t.Fatal(err)
	}
	if d.tableRows(t, "Feuille1") != before {
		t.Fatal("le tableau a changé")
	}
	l, err := d.Layout(ctx, sheet)
	if err != nil {
		t.Fatal(err)
	}
	cell := func(r, c int) (string, string) {
		for _, x := range l.Cells {
			if x.Row == r && x.Col == c {
				return x.Raw, x.Value
			}
		}
		return "", ""
	}
	if raw, _ := cell(3, 0); raw != "Total Nord" {
		t.Fatalf("libellé : %q", raw)
	}
	if raw, v := cell(3, 1); raw != "=SOUS.TOTAL(9;Cell[B2:B3])" || v != "15" {
		t.Fatalf("sous-total Nord : %q = %q", raw, v)
	}
	if raw, v := cell(6, 1); raw != "=SOUS.TOTAL(9;Cell[B2:B3];Cell[B5:B5])" || v != "22" {
		t.Fatalf("total général : %q = %q", raw, v)
	}
	if raw, _ := cell(6, 0); raw != "Total général" {
		t.Fatalf("libellé général : %q", raw)
	}
	if raw, _ := cell(3, 2); raw != "" {
		t.Fatalf("colonne non totalisée : %q", raw)
	}
	if len(l.Tables) != 0 {
		t.Fatalf("un tableau est né : %+v", l.Tables)
	}
	if l.Outline == nil || fmt.Sprint(l.Outline.Rows) != "[{1 2 0} {2 2 0} {3 1 0} {4 2 0} {5 1 0}]" {
		t.Fatalf("plan : %+v", l.Outline)
	}
	if !slices.Equal(l.PageSetup.RowBreaks, []int{4}) {
		t.Fatalf("sauts de page : %v", l.PageSetup.RowBreaks)
	}
	// Niveau 2 : totaux seuls ; les totaux ne changent pas.
	if err := d.ShowOutlineLevel(ctx, sheet, true, 2); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, sheet)
	if !slices.Equal(l.Outline.HiddenRows, []int{1, 2, 4}) {
		t.Fatalf("niveau 2 : %v", l.Outline.HiddenRows)
	}
	if _, v := cell(6, 1); v != "22" {
		t.Fatalf("total réduit : %s", v)
	}
	// Remplacer : la même feuille, synthèse au-dessus.
	again, err := d.Subtotal(ctx, SubtotalOptions{Table: "Feuille1", By: "Région", Func: 1, Columns: []string{"Qté"}, Replace: true})
	if err != nil || again != sheet {
		t.Fatalf("remplacer : feuille %d (attendu %d) %v", again, sheet, err)
	}
	l, _ = d.Layout(ctx, sheet)
	if raw, v := cell(1, 2); raw != "=SOUS.TOTAL(1;Cell[C4:C5];Cell[C7:C7])" || v != "2" {
		t.Fatalf("général au-dessus : %q = %q", raw, v)
	}
	if raw, v := cell(2, 2); raw != "=SOUS.TOTAL(1;Cell[C4:C5])" || v != "1,5" {
		t.Fatalf("moyenne Nord au-dessus : %q = %q", raw, v)
	}
	if !l.Outline.RowsAbove || len(l.PageSetup.RowBreaks) != 0 {
		t.Fatalf("synthèse au-dessus : %+v %v", l.Outline, l.PageSetup.RowBreaks)
	}
	// Sans remplacer : une nouvelle feuille.
	other, err := d.Subtotal(ctx, SubtotalOptions{Table: "Feuille1", By: "Région", Func: 9, Columns: []string{"Ventes"}, Below: true})
	if err != nil || other == sheet {
		t.Fatalf("nouvelle feuille : %d %v", other, err)
	}
	// Une étape d'annulation : la feuille disparaît.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	ls, _ := d.Layout(ctx, 1)
	if len(ls.Sheets) != 2 {
		t.Fatalf("annulé : %d feuilles", len(ls.Sheets))
	}
	check(t, d, "sous-totaux")
}
