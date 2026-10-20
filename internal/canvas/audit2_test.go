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

// Audit 2 I6 : un tableau croisé à coin vide et des dates en en-tête,
// collés. Tapés, la règle de la saisie (décision 0005, 20/10/2026) : une
// 1re ligne de textes (coin vide compris) est l'en-tête, comme collée ; des
// dates dans la 1re ligne en font une donnée (en ligne 1, sans place pour
// l'en-tête au-dessus : cellules libres).
func TestPasteHeaderLikeTyping(t *testing.T) {
	for _, tc := range []struct {
		name  string
		block [][]string
		want  string
		typed string
	}{
		{"coin vide", [][]string{{"", "Jan", "Fév"}, {"Nord", "1", "2"}, {"Sud", "3", "4"}}, "Feuille1@A1:Colonne 1|Jan|Fév×2", "Feuille1@A1:Colonne 1|Jan|Fév×2"},
		{"dates en en-tête", [][]string{{"Pays", "01/01/2024", "01/02/2024"}, {"France", "1", "2"}, {"Italie", "3", "4"}}, "Feuille1@A1:Pays|01/01/2024|01/02/2024×2",
			"A1=Pays B1=01/01/2024 C1=01/02/2024 A2=France B2=1 C2=2 A3=Italie B3=3 C3=4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := open(t)
			if _, err := d.Paste(context.Background(), 1, 0, 0, tc.block); err != nil {
				t.Fatal(err)
			}
			if got := layoutOf(t, d); got != tc.want {
				t.Fatalf("collé :\n got %s\nwant %s", got, tc.want)
			}
			d = open(t)
			typeIn(t, d, 0, 0, tc.block)
			if got := layoutOf(t, d); got != tc.typed {
				t.Fatalf("tapé :\n got %s\nwant %s", got, tc.typed)
			}
		})
	}
}

// Audit 2 I8 : une liste libre plus longue que le tableau qu'elle touche sur
// le côté n'est pas coupée : le tableau (liste à gauche) ou la liste (à
// droite) s'écarte d'une colonne, en une étape.
func TestLongListBesideTable(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name      string
		list, tbl int // colonnes de la liste et du tableau
		typeAt    int // colonne de la saisie (ligne 1)
		want      string
	}{
		{"liste à gauche", 0, 2, 1, "T@C1:Code|H1|H2×2 A1=Liste A2=x A3=y A4=z A5=w A6=v"},
		{"liste à droite", 3, 0, 2, "T@A1:H1|H2|Code×2 E1=Liste E2=x E3=y E4=z E5=w E6=v"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := open(t)
			must(t, d.Do(ctx, "Saisie", func(x *tx) error {
				for i, v := range []string{"Liste", "x", "y", "z", "w", "v"} {
					if err := x.setFreeCell(1, i, tc.list, v); err != nil {
						return err
					}
				}
				_, err := x.createTable(1, "T", 0, tc.tbl, []string{"H1", "H2"}, [][]string{{"1", "2"}, {"3", "4"}})
				return err
			}))
			before := dump(t, d)
			mustSet(t, d, 1, 0, tc.typeAt, "Code")
			check(t, d, tc.name)
			if got := layoutOf(t, d); got != tc.want {
				t.Fatalf("\n got %s\nwant %s", got, tc.want)
			}
			undoOnce(t, d, before, tc.name)
		})
	}
}

// Audit 2 I8 : une liste plus longue déplacée contre le côté droit d'un
// tableau s'en écarte d'une colonne, entière, et le message le dit.
func TestLongListMovedBesideTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		for i, v := range []string{"Liste", "x", "y", "z"} {
			if err := x.setFreeCell(1, i, 4, v); err != nil {
				return err
			}
		}
		_, err := x.createTable(1, "T", 0, 0, []string{"H1", "H2"}, [][]string{{"1", "2"}})
		return err
	}))
	d.TakeNotice()
	must(t, d.MoveCells(ctx, 1, 0, 4, 3, 4, 0, -2))
	if got, want := layoutOf(t, d), "T@A1:H1|H2×1 D1=Liste D2=x D3=y D4=z"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "reste entière") {
		t.Fatalf("message : %q", n)
	}
}

// Audit 2 M1 : une case juste sous un tableau et juste à droite d'un autre
// va au tableau du dessus (nouvelle ligne), quel que soit l'ordre des
// tableaux, comme l'aperçu du cadre.
func TestCornerCellGoesBelow(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		if _, err := x.createTable(1, "B", 3, 0, []string{"P", "Q"}, [][]string{{"1", "2"}, {"3", "4"}}); err != nil {
			return err
		}
		_, err := x.createTable(1, "A", 0, 2, []string{"X", "Y"}, [][]string{{"5", "6"}, {"7", "8"}})
		return err
	}))
	mustSet(t, d, 1, 3, 2, "9")
	for _, ti := range tablesOf(t, d) {
		if ti.Name == "A" && ti.Total != 3 || ti.Name == "B" && len(ti.Columns) != 2 {
			t.Fatalf("%s", layoutOf(t, d))
		}
	}
}

// Audit 2 M3 : une valeur tapée par erreur à droite ou dessous, puis effacée
// par une saisie vide, ne laisse ni colonne ni ligne vide (comme Suppr).
func TestEmptyInputShrinks(t *testing.T) {
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	mustSet(t, d, 1, 1, 3, "12")
	mustSet(t, d, 1, 1, 3, "")
	mustSet(t, d, 1, 4, 0, "Kiwis")
	mustSet(t, d, 1, 4, 0, "")
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×3"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	// Une colonne nommée reste, même vide.
	mustSet(t, d, 1, 0, 3, "Prix")
	mustSet(t, d, 1, 1, 3, "5")
	mustSet(t, d, 1, 1, 3, "")
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant|Prix×3"; got != want {
		t.Fatalf("colonne nommée :\n got %s\nwant %s", got, want)
	}
}

// Audit 2 M7 : des nombres tapés qui font un tableau sans en-têtes le
// disent (Event.Headless), comme au collage.
func TestTypedHeadless(t *testing.T) {
	d := open(t)
	typeIn(t, d, 2, 0, [][]string{{"10", "20"}})
	ev, err := d.SetCell(context.Background(), 1, 3, 0, "30")
	if err != nil || ev.Created == "" || !ev.Headless {
		t.Fatalf("%+v %v (%s)", ev, err, layoutOf(t, d))
	}
	if got, want := layoutOf(t, d), "Feuille1@A2:Colonne 1|Colonne 2×2"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

// Audit 2 M8 : une formule tapée à droite d'une ligne fait une colonne
// calculée, et le message le dit.
func TestFormulaRightNotice(t *testing.T) {
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	d.TakeNotice()
	mustSet(t, d, 1, 1, 3, "=[Montant]*2")
	if n := d.TakeNotice(); !strings.Contains(n, "Colonne calculée « Colonne 4 »") {
		t.Fatalf("message : %q", n)
	}
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
