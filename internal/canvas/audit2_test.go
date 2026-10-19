package canvas

import (
	"context"
	"strings"
	"testing"
)

// tableT crée en A1 le tableau T (Nom, Vide, Age ; Léa 1, Tom 2), et en G1 un
// autre tableau U (créé après : l'identifiant de T n'est pas le plus grand).
func tableT(t *testing.T, d *fixture) {
	t.Helper()
	must(t, d.Do(context.Background(), "Saisie", func(x *tx) error {
		if _, err := x.createTable(1, "T", 0, 0, []string{"Nom", "Vide", "Age"}, [][]string{{"Léa", "", "1"}, {"Tom", "", "2"}}); err != nil {
			return err
		}
		_, err := x.createTable(1, "U", 0, 6, []string{"X", "Y"}, [][]string{{"a", "b"}})
		return err
	}))
	mustSet(t, d, 1, 8, 0, "=SOMME(T[Age])")
}

// Audit 2 I3 et I5 : Suppr sur l'en-tête et une partie d'une colonne vide au
// milieu coupe le tableau sans erreur ; le côté gauche (à égalité) garde le
// nom, les formules suivent, et seule la note de la coupe reste.
func TestClearSplitKeepsName(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	tableT(t, d)
	before := dump(t, d)
	d.TakeNotice()
	must(t, d.ClearRange(ctx, 1, 0, 1, 1, 1, false))
	check(t, d, "coupé")
	if got, want := layoutOf(t, d), "U@G1:X|Y×1 T@A1:Nom×2 Tableau2@C1:Age×2 A9==SOMME(Tableau2[Age])"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if v := cellOn(t, d, 1, 8, 0).Value; v != "3" {
		t.Fatalf("formule = %q", v)
	}
	if n := d.TakeNotice(); strings.Contains(n, "converti en plage") || !strings.Contains(n, "coupé en deux") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "coupe")
}

// Audit 2 I4 : Suppr sur un total gardé à part retire la marque « hors
// tableau » (et le gras qu'elle avait donné).
func TestClearRemovesOutMark(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	mustSet(t, d, 1, 4, 0, "Total")
	if s := styleAt(t, d, 4, 0); s[outKey] != true {
		t.Fatalf("total non marqué : %v", s)
	}
	must(t, d.ClearRange(ctx, 1, 4, 0, 4, 1, false))
	if s := styleAt(t, d, 4, 0); len(s) != 0 {
		t.Fatalf("marque restée : %v", s)
	}
	// Retaper là : une nouvelle ligne du tableau, plus un total.
	mustSet(t, d, 1, 4, 0, "Kiwis")
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×4"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

// Audit 2 I5 : une ligne d'en-têtes posée sur un tableau sans en-têtes lui
// laisse son nom.
func TestHeaderOverKeepsName(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "Mesures", 1, 0, []string{"Colonne 1", "Colonne 2"}, [][]string{{"10", "20"}, {"30", "40"}})
		return err
	}))
	typeIn(t, d, 0, 0, [][]string{{"X", "Y", "Z"}})
	if got, want := layoutOf(t, d), "Mesures@A1:X|Y|Z×2"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

// Audit 2 I9 : Suppr sur un bloc de colonnes vides au milieu coupe le
// tableau ; sur des colonnes remplies, elles partent ; une étape chaque fois.
func TestSuppressColumnsBlock(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 0, 0, []string{"A", "B", "C", "D", "E"}, [][]string{{"1", "2", "", "", "5"}, {"6", "7", "", "", "8"}})
		return err
	}))
	before := dump(t, d)
	must(t, d.SuppressColumns(ctx, "T", 2, 3))
	check(t, d, "coupé")
	if got, want := layoutOf(t, d), "T@A1:A|B×2 Tableau1@E1:E×2"; got != want {
		t.Fatalf("coupe :\n got %s\nwant %s", got, want)
	}
	undoOnce(t, d, before, "coupe")
	must(t, d.SuppressColumns(ctx, "T", 0, 1))
	if got, want := layoutOf(t, d), "T@A1:C|D|E×2"; got != want {
		t.Fatalf("suppression :\n got %s\nwant %s", got, want)
	}
	undoOnce(t, d, before, "suppression")
	// Suppr sur l'en-tête et les lignes de deux colonnes vides voisines.
	must(t, d.ClearRange(ctx, 1, 0, 2, 2, 3, false))
	if got, want := layoutOf(t, d), "T@A1:A|B×2 Tableau1@E1:E×2"; got != want {
		t.Fatalf("effacement :\n got %s\nwant %s", got, want)
	}
	undoOnce(t, d, before, "effacement")
	// Colonnes de deux tableaux à la fois : une étape.
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "V", 4, 0, []string{"P", "Q", "R"}, [][]string{{"1", "2", "3"}})
		return err
	}))
	before = dump(t, d)
	must(t, d.DeleteColumnSpans(ctx, []ColumnSpan{{"T", 1, 2}, {"V", 1, 2}}))
	if got, want := layoutOf(t, d), "T@A1:A|D|E×2 V@A5:P×1"; got != want {
		t.Fatalf("colonnes de la feuille :\n got %s\nwant %s", got, want)
	}
	undoOnce(t, d, before, "colonnes de la feuille")
}

// Audit 2 I2 : coller à une ligne d'écart sous un tableau suit le chemin de
// la zone vide (îlots, en-tête, total à part).
func TestPasteOneRowBelowTable(t *testing.T) {
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	block := [][]string{{"Produit", "Prix"}, {"A", "1"}, {"B", "2"}, {"Total", "3"}}
	if _, err := d.Paste(context.Background(), 1, 5, 0, block); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×3 Tableau1@A6:Produit|Prix×2 A9*=Total B9*=3"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}
