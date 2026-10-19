package canvas

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
)

// rangeFixture : un tableau Feuille1 (Produit, Prix, Qté, Montant calculé)
// en A1:D3, une formule qui le cite en A7, puis converti en plage.
func rangeFixture(t *testing.T) (*fixture, string) {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix", "Qté"}, {"Clavier", "25,5", "2"}, {"007", "10", "3"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 3, "Montant"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 3, "=[@Prix]*[@Qté]"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 6, 0, "=SOMME(Feuille1[Montant])"); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 1, 1, 1, 1, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if err := d.ConvertToRange(ctx, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	check(t, d, "conversion")
	return d, before
}

// cellsOf donne « ligne,col=saisie→valeur » des cellules libres de la feuille.
func cellsOf(t *testing.T, d *fixture, sheet int64) map[string]string {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, c := range l.Cells {
		v := c.Raw
		if c.Value != "" {
			v += "→" + c.Value
		}
		out[fmt.Sprintf("%d,%d", c.Row, c.Col)] = v
	}
	return out
}

func rangesOf(t *testing.T, d *fixture, sheet int64) []Rect {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	return l.Ranges
}

// Convertir en plage : le tableau disparaît, ses cellules deviennent libres
// au même endroit (en-tête, valeurs, formule traduite de la colonne
// calculée, mise en forme), la formule qui le citait vise les cellules ; une
// seule étape d'annulation.
func TestConvertToRange(t *testing.T) {
	ctx := context.Background()
	d, before := rangeFixture(t)
	if ts := tablesOf(t, d); len(ts) != 0 {
		t.Fatalf("tableau restant : %+v", ts)
	}
	got := cellsOf(t, d, 1)
	want := map[string]string{
		"0,0": "Produit", "0,1": "Prix", "0,2": "Qté", "0,3": "Montant",
		"1,0": "Clavier", "1,1": "25,5", "1,2": "2", "1,3": "=Cell[B2]*Cell[C2]→51",
		"2,0": "'007", "2,1": "10", "2,2": "3", "2,3": "=Cell[B3]*Cell[C3]→30",
		"6,0": "=SOMME(Cell[D2:D3])→81",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("cellule %s = %q, attendu %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("cellules : %v", got)
	}
	l, _ := d.Layout(ctx, 1)
	bold := false
	for _, s := range l.Styles {
		if s.Row == 1 && s.Col == 1 && strings.Contains(s.Style, `"b":true`) {
			bold = true
		}
	}
	if !bold {
		t.Errorf("mise en forme perdue : %+v", l.Styles)
	}
	if r := rangesOf(t, d, 1); len(r) != 1 || r[0] != (Rect{0, 0, 2, 3}) {
		t.Errorf("plage : %+v", r)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "converti en plage") || !strings.Contains(n, "1 formule réécrite") {
		t.Errorf("message : %q", n)
	}
	undoOnce(t, d, before, "conversion")
}

// Une colonne calculée sans traduction simple (calcul entre lignes) donne
// ses valeurs ; une formule qui cite le tableau sans traduction simple
// garde son texte et donne #REF!.
func TestConvertToRangeValues(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Mois", "Ventes"}, {"jan", "10"}, {"fév", "20"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 2, "Cumul"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 2, "=CUMUL([@Ventes])"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 5, 0, `=REFERENCE(Feuille1[Ventes]; "jan")`); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if err := d.ConvertToRange(ctx, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	check(t, d, "conversion en valeurs")
	got := cellsOf(t, d, 1)
	if got["1,2"] != "10" || got["2,2"] != "30" {
		t.Errorf("cumul : %v", got)
	}
	if !strings.HasPrefix(got["5,0"], `=REFERENCE(Feuille1[Ventes]; "jan")→#REF!`) {
		t.Errorf("formule qui citait le tableau : %q", got["5,0"])
	}
	if n := d.TakeNotice(); !strings.Contains(n, "remplacée par ses valeurs") || !strings.Contains(n, "#REF!") {
		t.Errorf("message : %q", n)
	}
	undoOnce(t, d, before, "conversion en valeurs")
}

// Écrire dans la plage, juste dessous ou juste à côté ne refait pas de
// tableau : la plage s'agrandit. Plus loin, deux valeurs l'une sous l'autre
// font toujours un tableau.
func TestRangeStaysRange(t *testing.T) {
	ctx := context.Background()
	d, _ := rangeFixture(t)
	for _, s := range []struct {
		r, c int
		raw  string
	}{{1, 0, "Écran"}, {3, 0, "Câble"}, {3, 1, "4"}, {4, 0, "Pile"}, {1, 4, "note"}, {2, 4, "autre"}} {
		if _, err := d.SetCell(ctx, 1, s.r, s.c, s.raw); err != nil {
			t.Fatal(err)
		}
		if ts := tablesOf(t, d); len(ts) != 0 {
			t.Fatalf("saisie %q en %d,%d : tableau créé %+v", s.raw, s.r, s.c, ts)
		}
	}
	check(t, d, "saisies dans la plage")
	marked := map[[2]int]bool{}
	for _, r := range rangesOf(t, d, 1) {
		for row := r.R0; row <= r.R1; row++ {
			for col := r.C0; col <= r.C1; col++ {
				marked[[2]int{row, col}] = true
			}
		}
	}
	for _, p := range [][2]int{{3, 0}, {3, 1}, {4, 0}, {1, 4}, {2, 4}} {
		if !marked[p] {
			t.Errorf("%v pas dans la plage : %v", p, rangesOf(t, d, 1))
		}
	}
	// Collé contre la plage : des cellules libres.
	if _, err := d.Paste(ctx, 1, 5, 0, [][]string{{"Nom", "Prix"}, {"Vis", "1"}}); err != nil {
		t.Fatal(err)
	}
	if ts := tablesOf(t, d); len(ts) != 0 {
		t.Fatalf("collage contre la plage : tableau créé %+v", ts)
	}
	// Loin de la plage, la règle habituelle.
	if _, err := typeBlock(d, 0, 10, [][]string{{"Nom", "Ville"}, {"Dupont", "Lyon"}}); err != nil {
		t.Fatal(err)
	}
	if ts := tablesOf(t, d); len(ts) != 1 || ts[0].Col != 10 {
		t.Fatalf("loin de la plage : %+v", ts)
	}
	check(t, d, "plage et nouveau tableau")
}

// La fusion automatique (cellules libres jointives d'un tableau) ignore les
// cellules d'une plage.
func TestRangeIgnoredByJoin(t *testing.T) {
	ctx := context.Background()
	d, _ := rangeFixture(t)
	if _, err := typeBlock(d, 0, 7, [][]string{{"Code", "Lib"}, {"A", "x"}, {"B", "y"}}); err != nil {
		t.Fatal(err)
	}
	ts := tablesOf(t, d)
	if len(ts) != 1 {
		t.Fatalf("tableau : %+v", ts)
	}
	n := len(cellsOf(t, d, 1))
	// Le tableau posé juste à droite de la plage (colonne E) : rien n'est rattaché.
	if err := d.MoveTable(ctx, ts[0].Name, 0, 4); err != nil {
		t.Fatal(err)
	}
	check(t, d, "tableau contre la plage")
	ts = tablesOf(t, d)
	if len(ts) != 1 || ts[0].Col != 4 || colNames(ts[0]) != "Code|Lib" || ts[0].Rows != 2 {
		t.Fatalf("tableau contre la plage : %+v", ts)
	}
	if got := len(cellsOf(t, d, 1)); got != n {
		t.Fatalf("cellules de la plage rattachées : %d → %d", n, got)
	}
}

// Mettre sous forme de tableau refait un tableau de la plage et l'oublie.
func TestRangeBackToTable(t *testing.T) {
	ctx := context.Background()
	d, _ := rangeFixture(t)
	if _, err := d.SetCell(ctx, 1, 3, 0, "Câble"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	name, err := d.FormatAsTable(ctx, 1, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "plage en tableau")
	ts := tablesOf(t, d)
	if len(ts) != 1 || ts[0].Name != name || ts[0].Row != 0 || ts[0].Col != 0 || ts[0].Rows != 3 || colNames(ts[0]) != "Produit|Prix|Qté|Montant" {
		t.Fatalf("tableau : %+v", ts)
	}
	if r := rangesOf(t, d, 1); len(r) != 0 {
		t.Fatalf("plage pas oubliée : %+v", r)
	}
	if v := view(t, d); !strings.Contains(v, "Clavier|25.5|2|51") || !strings.Contains(v, "007|10|3|30") {
		t.Fatalf("contenu :\n%s", v)
	}
	// La mise en forme de cellule suit.
	l, _ := d.Layout(ctx, 1)
	if !strings.Contains(l.Tables[0].Columns[1].Style+fmt.Sprint(l.Styles), "b") {
		// le style de cellule de tableau n'est pas dans Layout : vérifié par le dump
		if !strings.Contains(dump(t, d), `{"b":true}`) {
			t.Errorf("mise en forme perdue")
		}
	}
	// Redevenu tableau : écrire dessous l'agrandit.
	if _, err := d.SetCell(ctx, 1, 4, 0, "Pile"); err != nil {
		t.Fatal(err)
	}
	if ts := tablesOf(t, d); ts[0].Rows != 4 {
		t.Fatalf("tableau pas agrandi : %+v", ts)
	}
	d.bus.Undo(ctx)
	undoOnce(t, d, before, "mise sous forme de tableau")
	if _, err := d.FormatAsTable(ctx, 1, 9, 9); err == nil {
		t.Fatal("hors plage accepté")
	}
}

// La plage part et revient avec sa feuille (copie, suppression, annulation)
// et suit les lignes insérées.
func TestRangeSheetCopyDelete(t *testing.T) {
	ctx := context.Background()
	d, _ := rangeFixture(t)
	id, err := d.CopySheet(ctx, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if r := rangesOf(t, d, id); len(r) != 1 || r[0] != (Rect{0, 0, 2, 3}) {
		t.Fatalf("copie : %+v", r)
	}
	// Dans la copie aussi, écrire dessous ne fait pas de tableau.
	if _, err := d.SetCell(ctx, id, 3, 0, "Câble"); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, id)
	if len(l.Tables) != 0 {
		t.Fatalf("copie : tableau créé %+v", l.Tables)
	}
	check(t, d, "copie de feuille")
	before := dump(t, d)
	if err := d.DeleteSheet(ctx, id); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.db.Read().QueryRow(`SELECT count(*) FROM _lattis_range_cells WHERE sheet = ?`, id).Scan(&n); err != nil || n != 0 {
		t.Fatalf("plage de la feuille supprimée : %d, %v", n, err)
	}
	undoOnce(t, d, before, "suppression de feuille")
	// Insérer une ligne au-dessus : la plage descend.
	if err := d.InsertSheetRows(ctx, 1, 0, 1); err != nil {
		t.Fatal(err)
	}
	if r := rangesOf(t, d, 1); len(r) != 1 || r[0] != (Rect{1, 0, 3, 3}) {
		t.Fatalf("après insertion : %+v", r)
	}
	// Supprimer (tout effacer) la zone : ce n'est plus une plage.
	if err := d.ClearCells(ctx, 1, 0, 0, 10, 10); err != nil {
		t.Fatal(err)
	}
	if r := rangesOf(t, d, 1); len(r) != 0 {
		t.Fatalf("après Supprimer : %+v", r)
	}
	if _, err := typeBlock(d, 1, 0, [][]string{{"Nom", "Ville"}, {"Dupont", "Lyon"}}); err != nil {
		t.Fatal(err)
	}
	if ts := tablesOf(t, d); len(ts) != 1 {
		t.Fatalf("zone effacée : %+v", ts)
	}
}

func TestRangeRects(t *testing.T) {
	marks := map[[2]int]bool{}
	for r := 0; r <= 2; r++ {
		for c := 0; c <= 3; c++ {
			marks[[2]int{r, c}] = true
		}
	}
	marks[[2]int{3, 0}] = true
	marks[[2]int{4, 0}] = true
	got := rangeRects(marks)
	if len(got) != 2 || got[0] != (Rect{0, 0, 2, 3}) || got[1] != (Rect{3, 0, 4, 0}) {
		t.Fatalf("rangeRects = %+v", got)
	}
}

// Les formules d'ailleurs qui citaient le tableau dans un agrégat visent
// ses cellules : colonne calculée d'un autre tableau, cellule d'une autre
// feuille (adresse qualifiée).
func TestConvertToRangeRewritesElsewhere(t *testing.T) {
	ctx := context.Background()
	d, _ := rangeFixture(t)
	d.bus.Undo(ctx)
	if _, err := typeBlock(d, 0, 8, [][]string{{"Code", "Part"}, {"A", "1"}, {"B", "2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 10, "Total"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 10, "=[@Part]*SOMME(Feuille1[Prix])"); err != nil {
		t.Fatal(err)
	}
	other, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, other, 0, 0, "=MAX(Feuille1[Qté])"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if err := d.ConvertToRange(ctx, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	check(t, d, "réécritures")
	ts := tablesOf(t, d)
	if len(ts) != 1 || ts[0].Columns[2].Formula != "=[Part]*SOMME(Cell[B2:B3])" {
		t.Fatalf("colonne calculée de l'autre tableau : %+v", ts)
	}
	exprs, args, _, err := d.ColumnExprs(ctx, ts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := d.db.Read().Query(`SELECT `+exprs[2]+` FROM `+ts[0].SQL+` ORDER BY _pos`, args...)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var v any
		rows.Scan(&v)
		got = append(got, formula.FromResult(v).String())
	}
	rows.Close()
	if strings.Join(got, ";") != "35,5;71" {
		t.Fatalf("valeurs de l'autre tableau : %v", got)
	}
	if got := cellsOf(t, d, other); got["0,0"] != "=MAX(Feuille1!Cell[C2:C3])→3" {
		t.Fatalf("autre feuille : %v", got)
	}
	undoOnce(t, d, before, "réécritures")
}

// Aller-retour : la colonne calculée redevient calculée et la formule qui
// visait ses cellules vise de nouveau le tableau.
func TestRangeRoundTrip(t *testing.T) {
	ctx := context.Background()
	d, _ := rangeFixture(t)
	if _, err := d.FormatAsTable(ctx, 1, 0, 0); err != nil {
		t.Fatal(err)
	}
	check(t, d, "aller-retour")
	ts := tablesOf(t, d)
	if len(ts) != 1 || ts[0].Columns[3].Formula != "=[Prix]*[Qté]" {
		t.Fatalf("colonne calculée : %+v", ts)
	}
	if got := cellsOf(t, d, 1); got["6,0"] != "=SOMME(Feuille1[Montant])→81" {
		t.Fatalf("formule : %v", got)
	}
}

// Mettre sous forme de tableau une liste d'une colonne restée en cellules
// libres (icône de la cellule) : sa première ligne donne l'en-tête.
func TestFormatFreeListAsTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	for r, v := range []string{"Fruits", "Pomme", "Poire"} {
		if _, err := d.SetCell(ctx, 1, 2+r, 1, v); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(tablesOf(t, d)); n != 0 {
		t.Fatalf("liste : %d tableaux avant", n)
	}
	before := dump(t, d)
	name, err := d.FormatAsTable(ctx, 1, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	ts := tablesOf(t, d)
	if len(ts) != 1 || ts[0].Name != name || colNames(ts[0]) != "Fruits" || ts[0].Rows != 2 || ts[0].Row != 2 {
		t.Fatalf("liste : %+v", ts)
	}
	undoOnce(t, d, before, "liste en tableau")
	if _, err := d.FormatAsTable(ctx, 1, 9, 9); err == nil {
		t.Fatal("cellule vide acceptée")
	}
}
