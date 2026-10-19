package canvas

import (
	"context"
	"errors"
	"fmt"
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
	// Fusionner la bande avec les données : refusé.
	if _, err := d.MergeCells(ctx, 1, 4, 0, 6, 0); !errors.Is(err, ErrMerge) {
		t.Fatalf("fusion de la bande et des données : %v", err)
	}
}

// singleFixture : tableau Nids sous une ligne de groupes comme celle de
// Louis : Topographie, Orientation et Lierre au-dessus d'en-têtes
// automatiques, des groupes qui répètent l'en-tête (casse, espaces et
// accents près), BOISEMENT au-dessus d'un vrai sous-en-tête.
func singleFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 3, 0, [][]string{
		{"Nom", "a", "b", "c", "ARBRE PORTEUR", "Date decouverte"},
		{"x", "1", "2", "3", "oui", "2024"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameTable(ctx, tablesOf(t, d)[0].Name, "Nids"); err != nil {
		t.Fatal(err)
	}
	setCells(t, d, 3, 1, "", 3, 2, "", 3, 3, "")
	if got := colNames(tablesOf(t, d)[0]); got != "Nom|Colonne 2|Colonne 3|Colonne 4|ARBRE PORTEUR|Date decouverte" {
		t.Fatalf("préparation : %s", got)
	}
	setCells(t, d, 2, 0, "BOISEMENT", 2, 1, "Topographie", 2, 2, "Orientation", 2, 3, "Lierre", 2, 4, " arbre  porteur ", 2, 5, "Date découverte",
		0, 10, "=NBVAL(Nids[Colonne 2])")
	check(t, d, "préparation")
	return d
}

// Activation : un groupe seul au-dessus d'un en-tête automatique devient le
// nom de la colonne, un groupe qui répète l'en-tête disparaît ; ces colonnes
// restent sur un niveau (bande et en-tête fusionnés).
func TestTwoLevelSingleOnActivation(t *testing.T) {
	ctx := context.Background()
	d := singleFixture(t)
	before := dump(t, d)
	d.TakeNotice()
	if err := d.SetTwoLevel(ctx, "Nids", true); err != nil {
		t.Fatal(err)
	}
	check(t, d, "deux lignes")
	want := "BOISEMENT.Nom|Topographie|Orientation|Lierre|ARBRE PORTEUR|Date decouverte"
	if got := colNames(tablesOf(t, d)[0]); got != want {
		t.Fatalf("noms : %s", got)
	}
	if raw, _ := freeAt(t, d, 0, 10); !strings.Contains(raw, "Nids[Topographie]") {
		t.Fatalf("formule : %q", raw)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "1 colonne renommée") || !strings.Contains(n, "5 sur un niveau") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "deux lignes")
}

func TestGroupedName(t *testing.T) {
	for _, c := range []struct {
		g, sub string
		alone  bool
		want   string
	}{
		{"Topographie", "Colonne 17", true, "Topographie"},
		{"Topographie", "Colonne 17", false, "Topographie.Colonne 17"},
		{"Topographie", "Type", true, "Topographie.Type"},
		{"DECOUVREUR", "Découvreur", false, "Découvreur"},
		{"Date découverte", " date  decouverte", true, " date  decouverte"},
		{"", "Note", true, "Note"},
	} {
		if got := groupedName(c.g, c.sub, c.alone); got != c.want {
			t.Errorf("%+v : %q", c, got)
		}
	}
}

func twoLevelOn(t *testing.T) *fixture {
	t.Helper()
	d := twoLevelFixture(t)
	if err := d.SetTwoLevel(context.Background(), "Inv", true); err != nil {
		t.Fatal(err)
	}
	setCells(t, d, 0, 11, "=NBVAL(Inv[BOISEMENT.Statut])", 0, 12, "=NBVAL(Inv[Latitude.L93])")
	d.TakeNotice()
	return d
}

func merged(t *testing.T, d *fixture, r0, c0, r1, c1 int) {
	t.Helper()
	if _, err := d.MergeCells(context.Background(), 1, r0, c0, r1, c1); err != nil {
		t.Fatalf("fusion (%d,%d)-(%d,%d) : %v", r0, c0, r1, c1, err)
	}
	check(t, d, "fusion")
}

func unmerged(t *testing.T, d *fixture, r0, c0, r1, c1 int) {
	t.Helper()
	if err := d.UnmergeCells(context.Background(), 1, r0, c0, r1, c1); err != nil {
		t.Fatalf("défusion (%d,%d)-(%d,%d) : %v", r0, c0, r1, c1, err)
	}
	check(t, d, "défusion")
}

func wantNames(t *testing.T, d *fixture, when, want string) {
	t.Helper()
	if got := colNames(tablesOf(t, d)[0]); got != want {
		t.Fatalf("%s : %s", when, got)
	}
}

func wantFormula(t *testing.T, d *fixture, col int, ref string) {
	t.Helper()
	if raw, _ := freeAt(t, d, 0, col); !strings.Contains(raw, ref) {
		t.Fatalf("formule : %q, attendu %s", raw, ref)
	}
}

// Fusionner la bande et l'en-tête d'une colonne la met sur un niveau ;
// Défusionner une colonne sur un niveau ne change rien au moteur (le
// groupe se tape dans la bande).
func TestTwoLevelMergeBandAndHeader(t *testing.T) {
	d := twoLevelOn(t)
	before := dump(t, d)
	// Vrai sous-en-tête : il devient le nom, la colonne quitte son groupe.
	merged(t, d, 2, 1, 3, 1)
	wantNames(t, d, "sur un niveau", "BOISEMENT.Nom|Statut|BOISEMENT.Parcelle|Latitude.DD|Latitude.L93|Lieu.Commune|Lieu.Dept|Note")
	wantFormula(t, d, 11, "Inv[Statut]")
	if n := d.TakeNotice(); !strings.Contains(n, "sur un niveau") || !strings.Contains(n, "quitte le groupe") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "sur un niveau")
	// Sous-en-tête automatique : le groupe devient le nom.
	setCells(t, d, 3, 4, "")
	wantNames(t, d, "en-tête effacé", "BOISEMENT.Nom|BOISEMENT.Statut|BOISEMENT.Parcelle|Latitude.DD|Latitude.Colonne 5|Lieu.Commune|Lieu.Dept|Note")
	before = dump(t, d)
	merged(t, d, 2, 4, 3, 4)
	wantNames(t, d, "groupe pour nom", "BOISEMENT.Nom|BOISEMENT.Statut|BOISEMENT.Parcelle|Latitude.DD|Latitude|Lieu.Commune|Lieu.Dept|Note")
	wantFormula(t, d, 12, "Inv[Latitude]")
	undoOnce(t, d, before, "groupe pour nom")
	// Déjà sur un niveau : rien ne change, la barre d'état le dit.
	before = dump(t, d)
	d.TakeNotice()
	merged(t, d, 2, 7, 3, 7)
	if got := dump(t, d); got != before {
		t.Fatalf("colonne déjà sur un niveau :\n%s", diff(before, got))
	}
	if n := d.TakeNotice(); !strings.Contains(n, "déjà sur un niveau") {
		t.Fatalf("message : %q", n)
	}
	unmerged(t, d, 2, 7, 3, 7)
	if got := dump(t, d); got != before {
		t.Fatalf("défusion d'une colonne sur un niveau :\n%s", diff(before, got))
	}
	if n := d.TakeNotice(); !strings.Contains(n, "Groupe.Nom") {
		t.Fatalf("message : %q", n)
	}
	// Son groupe se tape dans la bande (l'interface la sépare d'abord).
	setCells(t, d, 2, 7, "Remarques")
	wantNames(t, d, "groupe tapé", "BOISEMENT.Nom|BOISEMENT.Statut|BOISEMENT.Parcelle|Latitude.DD|Latitude.Colonne 5|Lieu.Commune|Lieu.Dept|Remarques.Note")
	// En-tête effacé sous un groupe seul : la colonne prend le nom du groupe.
	setCells(t, d, 3, 7, "")
	wantNames(t, d, "en-tête effacé", "BOISEMENT.Nom|BOISEMENT.Statut|BOISEMENT.Parcelle|Latitude.DD|Latitude.Colonne 5|Lieu.Commune|Lieu.Dept|Remarques")
	// En-tête qui répète le groupe : sur un niveau aussi.
	setCells(t, d, 2, 7, "Mesure", 3, 7, "mesure")
	wantNames(t, d, "en-tête répété", "BOISEMENT.Nom|BOISEMENT.Statut|BOISEMENT.Parcelle|Latitude.DD|Latitude.Colonne 5|Lieu.Commune|Lieu.Dept|mesure")
}

// Fusionner des cellules voisines de la bande : un seul groupe, le premier
// qu'elles portent (« Groupe » sinon) ; Défusionner un groupe le laisse sur
// sa première colonne.
func TestTwoLevelMergeGroups(t *testing.T) {
	d := twoLevelOn(t)
	before := dump(t, d)
	merged(t, d, 2, 4, 2, 7)
	wantNames(t, d, "groupes réunis", "BOISEMENT.Nom|BOISEMENT.Statut|BOISEMENT.Parcelle|Latitude.DD|Latitude.L93|Latitude.Commune|Latitude.Dept|Latitude.Note")
	if got := spansOf(t, d); got != "[[0 2] [3 7]]" {
		t.Fatalf("bande : %s", got)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "« Latitude » sur 4 colonnes") || !strings.Contains(n, "remplace « Lieu »") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "groupes réunis")
	// Défusionner BOISEMENT : il reste sur Nom, les autres sur un niveau.
	unmerged(t, d, 2, 1, 2, 1)
	wantNames(t, d, "groupe défusionné", "BOISEMENT.Nom|Statut|Parcelle|Latitude.DD|Latitude.L93|Lieu.Commune|Lieu.Dept|Note")
	wantFormula(t, d, 11, "Inv[Statut]")
	wantFormula(t, d, 10, "Inv[BOISEMENT.Nom]")
	if n := d.TakeNotice(); !strings.Contains(n, "il reste sur « BOISEMENT.Nom »") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "groupe défusionné")
	// Colonnes sur un niveau seulement : « Groupe », à renommer dans la bande.
	unmerged(t, d, 2, 0, 2, 2)
	merged(t, d, 2, 1, 3, 2)
	wantNames(t, d, "groupe neuf", "BOISEMENT.Nom|Groupe.Statut|Groupe.Parcelle|Latitude.DD|Latitude.L93|Lieu.Commune|Lieu.Dept|Note")
	setCells(t, d, 2, 2, "Suivi")
	wantNames(t, d, "groupe renommé", "BOISEMENT.Nom|Suivi.Statut|Suivi.Parcelle|Latitude.DD|Latitude.L93|Lieu.Commune|Lieu.Dept|Note")
}

// Fusions refusées : bande et données, au-dessus de la bande, en-têtes
// entre eux, deux tableaux.
func TestTwoLevelMergeRefused(t *testing.T) {
	ctx := context.Background()
	d := twoLevelOn(t)
	if _, err := d.Paste(ctx, 1, 3, 9, [][]string{{"X", "Y"}, {"1", "2"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetTwoLevel(ctx, tablesOf(t, d)[1].Name, true); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	for _, r := range [][4]int{{2, 0, 4, 0}, {1, 0, 2, 1}, {3, 0, 3, 1}, {2, 7, 2, 9}, {2, 7, 2, 8}} {
		_, err := d.MergeCells(ctx, 1, r[0], r[1], r[2], r[3])
		if !errors.Is(err, ErrMerge) {
			t.Fatalf("fusion %v : %v", r, err)
		}
	}
	if got := dump(t, d); got != before {
		t.Fatalf("fusions refusées :\n%s", diff(before, got))
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

// mergedGroupsFixture : tableau Nids (en-tête ligne 3, six colonnes) sous une
// ligne de groupes préparée par prep, puis groupes d'en-têtes activés.
func mergedGroupsFixture(t *testing.T, prep func(d *fixture)) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 3, 0, [][]string{
		{"Nid", "Ref", "Nom", "Statut", "Parcelle", "Note"},
		{"1", "2", "a", "b", "c", "z"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameTable(ctx, tablesOf(t, d)[0].Name, "Nids"); err != nil {
		t.Fatal(err)
	}
	prep(d)
	if err := d.SetTwoLevel(ctx, "Nids", true); err != nil {
		t.Fatal(err)
	}
	check(t, d, "deux lignes")
	return d
}

func mergeFree(t *testing.T, d *fixture, r0, c0, r1, c1 int) {
	t.Helper()
	if _, err := d.MergeCells(context.Background(), 1, r0, c0, r1, c1); err != nil {
		t.Fatal(err)
	}
}

// spansOf : les cellules de la bande (groupSpan), de gauche à droite.
func spansOf(t *testing.T, d *fixture) string {
	t.Helper()
	var out [][2]int
	if err := d.read(context.Background(), func(tx *tx) error {
		ts, err := tx.tables(1)
		if err != nil {
			return err
		}
		for i := 0; i < len(ts[0].Cols); {
			lo, hi := groupSpan(ts[0], i)
			out = append(out, [2]int{lo, hi})
			i = hi + 1
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(out)
}

// Une cellule fusionnée donne son texte à toutes les colonnes qu'elle
// couvre : la bande la garde fusionnée.
func TestTwoLevelKeepsMerges(t *testing.T) {
	d := mergedGroupsFixture(t, func(d *fixture) {
		setCells(t, d, 2, 0, "N° DE NID", 2, 2, "BOISEMENT")
		mergeFree(t, d, 2, 0, 2, 1)
		mergeFree(t, d, 2, 2, 2, 4)
	})
	want := "N° DE NID.Nid|N° DE NID.Ref|BOISEMENT.Nom|BOISEMENT.Statut|BOISEMENT.Parcelle|Note"
	if got := colNames(tablesOf(t, d)[0]); got != want {
		t.Fatalf("groupes : %s", got)
	}
	if got := spansOf(t, d); got != "[[0 1] [2 4] [5 5]]" {
		t.Fatalf("bande : %s", got)
	}
}

// Fusion dont la première cellule est vide mais qui couvre un texte : le
// texte devient le groupe de toute la fusion. Une fusion vide ne prend pas
// le groupe d'à côté, et une cellule blanche n'interrompt pas un groupe non
// fusionné.
func TestTwoLevelMergeEmptyFirstCell(t *testing.T) {
	d := mergedGroupsFixture(t, func(d *fixture) {
		mergeFree(t, d, 2, 0, 2, 1)
		if err := d.Do(context.Background(), "texte caché", func(tx *tx) error {
			return tx.setFreeCell(1, 2, 1, "N° DE NID")
		}); err != nil {
			t.Fatal(err)
		}
		mergeFree(t, d, 2, 2, 2, 3)
		setCells(t, d, 2, 4, "Lieu", 2, 5, " ")
	})
	want := "N° DE NID.Nid|N° DE NID.Ref|Nom|Statut|Lieu.Parcelle|Lieu.Note"
	if got := colNames(tablesOf(t, d)[0]); got != want {
		t.Fatalf("groupes : %s", got)
	}
	if got := spansOf(t, d); got != "[[0 1] [2 2] [3 3] [4 5]]" {
		t.Fatalf("bande : %s", got)
	}
}

// Fusion sur deux lignes (le titre descend jusqu'à la bande) : son texte
// devient le groupe et quitte la ligne du dessus.
func TestTwoLevelTallMerge(t *testing.T) {
	d := mergedGroupsFixture(t, func(d *fixture) {
		setCells(t, d, 1, 0, "N° DE NID", 2, 2, "BOISEMENT")
		mergeFree(t, d, 1, 0, 2, 1)
		mergeFree(t, d, 2, 2, 2, 4)
	})
	want := "N° DE NID.Nid|N° DE NID.Ref|BOISEMENT.Nom|BOISEMENT.Statut|BOISEMENT.Parcelle|Note"
	if got := colNames(tablesOf(t, d)[0]); got != want {
		t.Fatalf("groupes : %s", got)
	}
	if raw, ok := freeAt(t, d, 1, 0); ok {
		t.Fatalf("texte resté au-dessus : %q", raw)
	}
	if got := spansOf(t, d); got != "[[0 1] [2 4] [5 5]]" {
		t.Fatalf("bande : %s", got)
	}
}
