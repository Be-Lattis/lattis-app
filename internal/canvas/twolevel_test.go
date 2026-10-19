package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func setCells(t *testing.T, d *fixture, cells ...any) {
	t.Helper()
	for k := 0; k+2 < len(cells); k += 3 {
		if _, err := d.SetCell(context.Background(), 1, cells[k].(int), cells[k+1].(int), cells[k+2].(string)); err != nil {
			t.Fatalf("saisie %v : %v", cells[k:k+3], err)
		}
	}
}

func freeAt(t *testing.T, d *fixture, row, col int) (string, bool) {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range l.Cells {
		if c.Row == row && c.Col == col {
			return c.Raw, true
		}
	}
	return "", false
}

// twoLevelFixture : tableau Inv (en-tête ligne 3) sous une ligne de groupes
// (BOISEMENT fusionné sur 3 colonnes, Latitude non fusionné suivi d'une
// cellule vide, Lieu fusionné sur 2, rien au-dessus de Note), un titre en
// ligne 0 et une formule qui cite une colonne.
func twoLevelFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 3, 0, [][]string{
		{"Nom", "Statut", "Parcelle", "DD", "L93", "Commune", "Dept", "Note"},
		{"a", "b", "c", "1", "2", "x", "y", "z"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameTable(ctx, tablesOf(t, d)[0].Name, "Inv"); err != nil {
		t.Fatal(err)
	}
	setCells(t, d, 0, 0, "Inventaire", 2, 0, "BOISEMENT", 2, 3, "Latitude", 2, 5, "Lieu", 0, 10, "=NBVAL(Inv[Nom])")
	if _, err := d.MergeCells(ctx, 1, 2, 0, 2, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := d.MergeCells(ctx, 1, 2, 5, 2, 6); err != nil {
		t.Fatal(err)
	}
	if ts := tablesOf(t, d); len(ts) != 1 || ts[0].Row != 3 {
		t.Fatalf("préparation : %+v", ts)
	}
	check(t, d, "préparation")
	return d
}

func TestTwoLevelOnTakesGroupsAbove(t *testing.T) {
	ctx := context.Background()
	d := twoLevelFixture(t)
	before := dump(t, d)
	if err := d.SetTwoLevel(ctx, "Inv", true); err != nil {
		t.Fatal(err)
	}
	check(t, d, "deux lignes")
	ts := tablesOf(t, d)
	want := "BOISEMENT.Nom|BOISEMENT.Statut|BOISEMENT.Parcelle|Latitude.DD|Latitude.L93|Lieu.Commune|Lieu.Dept|Note"
	if len(ts) != 1 || !ts[0].TwoLevel || ts[0].Row != 3 || colNames(ts[0]) != want {
		t.Fatalf("deux lignes : %+v", ts)
	}
	// Les cellules de la ligne et leurs fusions sont parties, le titre reste.
	for _, c := range []int{0, 3, 5} {
		if raw, ok := freeAt(t, d, 2, c); ok {
			t.Fatalf("cellule (2,%d) restée libre : %q", c, raw)
		}
	}
	if raw, ok := freeAt(t, d, 0, 0); !ok || raw != "Inventaire" {
		t.Fatalf("titre : %q %v", raw, ok)
	}
	if err := d.read(ctx, func(tx *tx) error {
		ms, err := tx.merges(1)
		if err == nil && len(ms) != 0 {
			t.Fatalf("fusions restées : %+v", ms)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// La formule suit le renommage.
	if raw, _ := freeAt(t, d, 0, 10); !strings.Contains(raw, "Inv[BOISEMENT.Nom]") {
		t.Fatalf("formule : %q", raw)
	}
	if got := view(t, d); !strings.Contains(got, "{BOISEMENT}|{BOISEMENT}|{BOISEMENT}|{Latitude}|{Latitude}|{Lieu}|{Lieu}|{}") {
		t.Fatalf("bande :\n%s", got)
	}
	// Une seule étape d'annulation.
	undoOnce(t, d, before, "deux lignes")
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	// Désactivé : les noms complets restent, sur une ligne.
	if err := d.SetTwoLevel(ctx, "Inv", false); err != nil {
		t.Fatal(err)
	}
	if ts := tablesOf(t, d); ts[0].TwoLevel || colNames(ts[0]) != want {
		t.Fatalf("une ligne : %+v", ts)
	}
	if got := view(t, d); strings.Contains(got, "{") {
		t.Fatalf("bande restée :\n%s", got)
	}
	check(t, d, "une ligne")
	// Le SQL lit les colonnes sous leur nom complet.
	if n, err := d.ExecSQL(ctx, `UPDATE Inv SET "BOISEMENT.Nom" = "Latitude.DD" || 'z' WHERE "Lieu.Commune" = 'x'`); err != nil || n != 1 {
		t.Fatalf("SQL : %d %v", n, err)
	}
	if got := view(t, d); !strings.Contains(got, "1z|b|c|1|2|x") {
		t.Fatalf("SQL :\n%s", got)
	}
}

func TestTwoLevelBandEditing(t *testing.T) {
	ctx := context.Background()
	d := twoLevelFixture(t)
	if err := d.SetTwoLevel(ctx, "Inv", true); err != nil {
		t.Fatal(err)
	}
	d.TakeNotice()
	// Saisie dans une colonne sans groupe : son groupe.
	setCells(t, d, 2, 7, "Remarques")
	// Saisie sur un groupe fusionné : tout le groupe est renommé, la formule suit.
	setCells(t, d, 2, 1, "BOIS")
	want := "BOIS.Nom|BOIS.Statut|BOIS.Parcelle|Latitude.DD|Latitude.L93|Lieu.Commune|Lieu.Dept|Remarques.Note"
	if ts := tablesOf(t, d); colNames(ts[0]) != want {
		t.Fatalf("groupes saisis : %s", colNames(ts[0]))
	}
	if raw, _ := freeAt(t, d, 0, 10); !strings.Contains(raw, "Inv[BOIS.Nom]") {
		t.Fatalf("formule : %q", raw)
	}
	// L'en-tête se saisit sans le groupe, qui reste.
	setCells(t, d, 3, 7, "DD")
	// Effacer un groupe : les noms redeviennent le sous-en-tête…
	setCells(t, d, 2, 0, "", 2, 3, "")
	want = "Nom|Statut|Parcelle|DD|L93|Lieu.Commune|Lieu.Dept|Remarques.DD"
	if ts := tablesOf(t, d); colNames(ts[0]) != want {
		t.Fatalf("groupes effacés : %s", colNames(ts[0]))
	}
	if raw, _ := freeAt(t, d, 0, 10); !strings.Contains(raw, "Inv[Nom]") {
		t.Fatalf("formule : %q", raw)
	}
	// … sauf s'il est pris : le nom complet reste et la barre d'état le dit.
	d.TakeNotice()
	setCells(t, d, 2, 7, "")
	if ts := tablesOf(t, d); !strings.HasSuffix(colNames(ts[0]), "|Remarques.DD") {
		t.Fatalf("doublon : %s", colNames(ts[0]))
	}
	if n := d.TakeNotice(); !strings.Contains(n, "garde son nom complet") {
		t.Fatalf("message : %q", n)
	}
	// Un point dans un groupe est refusé.
	if _, err := d.SetCell(ctx, 1, 2, 5, "H.T."); err == nil {
		t.Fatal("groupe avec un point accepté")
	}
	check(t, d, "saisies dans la bande")
}

func TestTwoLevelBandProtected(t *testing.T) {
	ctx := context.Background()
	d := twoLevelFixture(t)
	if err := d.SetTwoLevel(ctx, "Inv", true); err != nil {
		t.Fatal(err)
	}
	// Un titre se pose au-dessus de la bande et reste libre.
	setCells(t, d, 1, 4, "Relevé")
	if raw, ok := freeAt(t, d, 1, 4); !ok || raw != "Relevé" {
		t.Fatalf("titre au-dessus de la bande : %q %v", raw, ok)
	}
	// Une cellule déplacée dans la bande devient le groupe de sa colonne.
	if err := d.MoveCells(ctx, 1, 1, 4, 1, 4, 1, 0); err != nil {
		t.Fatal(err)
	}
	check(t, d, "cellule déplacée dans la bande")
	if _, ok := freeAt(t, d, 2, 4); ok {
		t.Fatal("cellule libre dans la bande")
	}
	if ts := tablesOf(t, d); ts[0].Columns[4].Name != "Relevé.L93" {
		t.Fatalf("groupe déplacé : %s", colNames(ts[0]))
	}
	// La bande ne se supprime pas seule ; une ligne insérée au-dessus la
	// décale avec le tableau.
	if err := d.DeleteSheetRows(ctx, 1, 2, 2); !errors.Is(err, ErrTable) {
		t.Fatalf("suppression de la bande : %v", err)
	}
	if err := d.InsertSheetRows(ctx, 1, 1, 2); err != nil {
		t.Fatal(err)
	}
	if ts := tablesOf(t, d); ts[0].Row != 5 {
		t.Fatalf("insertion : %+v", ts[0])
	}
	check(t, d, "insertion au-dessus")
	// Fusionner dans la bande : refusé.
	if _, err := d.MergeCells(ctx, 1, 4, 0, 4, 1); !errors.Is(err, ErrMerge) {
		t.Fatalf("fusion dans la bande : %v", err)
	}
}

// Un tableau en première ligne descend d'une ligne pour faire place à la
// bande ; un tableau empilé dessous s'écarte quand sa bande toucherait le
// tableau du dessus.
func TestTwoLevelPlacement(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"A", "B"}, {"1", "2"}, {"3", "4"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Paste(ctx, 1, 4, 0, [][]string{{"C", "D"}, {"5", "6"}}); err != nil {
		t.Fatal(err)
	}
	ts := tablesOf(t, d)
	if len(ts) != 2 || ts[0].Row != 0 || ts[1].Row != 4 {
		t.Fatalf("préparation : %+v", ts)
	}
	before := dump(t, d)
	if err := d.SetTwoLevel(ctx, ts[0].Name, true); err != nil {
		t.Fatal(err)
	}
	check(t, d, "tableau en ligne 0")
	got := tablesOf(t, d)
	if got[0].Row != 1 || got[1].Row != 5 {
		t.Fatalf("tableau en ligne 0 : %+v", got)
	}
	undoOnce(t, d, before, "tableau en ligne 0")
	// Dessous : sa bande toucherait le premier, il descend.
	if err := d.SetTwoLevel(ctx, ts[1].Name, true); err != nil {
		t.Fatal(err)
	}
	check(t, d, "tableau empilé")
	if got := tablesOf(t, d); got[1].Row != 5 {
		t.Fatalf("tableau empilé : %+v", got)
	}
	// Bande vide : la saisie y pose un groupe.
	setCells(t, d, 4, 1, "Mesures")
	if got := tablesOf(t, d); colNames(got[1]) != "C|Mesures.D" {
		t.Fatalf("groupe saisi : %s", colNames(got[1]))
	}
	// Déplacé en ligne 0, il garde sa bande sur la feuille.
	if err := d.MoveTable(ctx, ts[1].Name, 0, 5); err != nil {
		t.Fatal(err)
	}
	if got := tablesOf(t, d); got[1].Row != 1 {
		t.Fatalf("déplacé en haut : %+v", got[1])
	}
	check(t, d, "déplacé en haut")
}

func TestSplitGroup(t *testing.T) {
	for _, c := range []struct{ in, g, s string }{
		{"BOISEMENT.Nom", "BOISEMENT", "Nom"},
		{"Prix H.T.", "Prix H", "T."},
		{"Nom", "", "Nom"},
		{".x", "", ".x"},
		{"x.", "", "x."},
	} {
		if g, s := splitGroup(c.in); g != c.g || s != c.s {
			t.Errorf("%q : %q %q", c.in, g, s)
		}
	}
}
