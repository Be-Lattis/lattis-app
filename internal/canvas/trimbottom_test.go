package canvas

import (
	"context"
	"strings"
	"testing"
)

// Décision 0009, règle du bas : après une suppression ou un effacement qui
// atteint la dernière ligne d'un tableau, les lignes vides restées au bas
// partent aussi, dans la même étape d'annulation.

// nidsSheet : le cas de Louis (« Suividesnids ») en petit. Onglet à choix sur
// Année = 2025 : 9 lignes remplies, 2 lignes vides (seule l'année, masquée et
// imposée), une ligne « test » ; des lignes de 2024 entre elles et à la fin
// physique du tableau.
func nidsSheet(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	block := [][]string{{"Année", "NID suivi", "Note"}}
	for i := 1; i <= 9; i++ {
		block = append(block, []string{"2025", "N" + itoa(i), "ok"})
		if i == 3 {
			block = append(block, []string{"2024", "V1", "x"})
		}
	}
	block = append(block,
		[]string{"2025", "", ""},
		[]string{"2025", "", ""},
		[]string{"2024", "V2", "y"},
		[]string{"2025", "test", ""},
		[]string{"2024", "V3", "z"},
	)
	if _, err := d.Paste(ctx, 1, 0, 0, block); err != nil {
		t.Fatal(err)
	}
	must(t, promoteIfData(ctx, d, "Feuille1"))
	must(t, d.SetSheetChoice(ctx, 1, "Feuille1", "Année", "2025"))
	if shown, total := rowsOf(t, d); shown != 12 || total != 15 {
		t.Fatalf("départ : %d affichées, %d en tout", shown, total)
	}
	return d
}

const nidsKept = "2025|N1|ok / 2025|N2|ok / 2025|N3|ok / 2024|V1|x / 2025|N4|ok / 2025|N5|ok / 2025|N6|ok / 2025|N7|ok / 2025|N8|ok / 2025|N9|ok / 2024|V2|y / 2024|V3|z"

func TestTrimBottomChoiceDeleteRow(t *testing.T) {
	ctx := context.Background()
	d := nidsSheet(t)
	before := dump(t, d)
	d.TakeNotice()
	must(t, d.DeleteRow(ctx, "Feuille1", 11))
	check(t, d, "ligne supprimée")
	if shown, total := rowsOf(t, d); shown != 9 || total != 12 {
		t.Fatalf("après : %d affichées, %d en tout", shown, total)
	}
	if got := tableRowsOf(t, d, "Feuille1"); got != nidsKept {
		t.Fatalf("lignes :\n got %s\nwant %s", got, nidsKept)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "2 lignes vides retirées en bas du tableau") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "ligne supprimée")
}

func TestTrimBottomChoiceClear(t *testing.T) {
	ctx := context.Background()
	d := nidsSheet(t)
	before := dump(t, d)
	d.TakeNotice()
	// Suppr sur « test » (ligne 12 affichée, colonne NID suivi).
	must(t, d.ClearRange(ctx, 1, 12, 1, 12, 1, false))
	check(t, d, "effacement")
	if got := tableRowsOf(t, d, "Feuille1"); got != nidsKept {
		t.Fatalf("lignes :\n got %s\nwant %s", got, nidsKept)
	}
	n := d.TakeNotice()
	if !strings.Contains(n, "1 ligne vide supprimée") || !strings.Contains(n, "2 lignes vides retirées en bas du tableau") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "effacement")

	// Saisie vide (Retour arrière puis Entrée) sur la dernière ligne : même
	// rangement (la ligne vidée est elle-même au bas).
	if _, err := d.SetCell(ctx, 1, 12, 1, ""); err != nil {
		t.Fatal(err)
	}
	if shown, total := rowsOf(t, d); shown != 9 || total != 12 {
		t.Fatalf("saisie vide : %d affichées, %d en tout", shown, total)
	}
}

// Sans onglet à choix : la dernière ligne supprimée (Supprimer la ligne du
// tableau, ou ligne entière de la feuille) ou effacée emporte les lignes
// vides au-dessus d'elle.
func TestTrimBottomPlain(t *testing.T) {
	ctx := context.Background()
	rows := [][]string{{"a", "1"}, {"b", "2"}, {"c", "3"}, {"", ""}, {"", ""}, {"x", ""}}
	const want = "a|1 / b|2 / c|3"
	for _, how := range []string{"ligne", "feuille", "suppr", "coupe"} {
		d := open(t)
		indiceTable(t, d, []string{"Nom", "Indice"}, rows)
		before := dump(t, d)
		switch how {
		case "ligne":
			must(t, d.DeleteRow(ctx, "T", 5))
		case "feuille":
			must(t, d.DeleteSheetRows(ctx, 1, 6, 6))
		case "suppr":
			must(t, d.ClearRange(ctx, 1, 6, 0, 6, 1, false))
		case "coupe":
			// Couper « x » et le coller hors du tableau.
			if _, err := d.Transfer(ctx, 1, Rect{6, 0, 6, 0}, 1, Rect{0, 6, 0, 6}, true, [][]string{{"x"}}); err != nil {
				t.Fatal(err)
			}
		}
		check(t, d, how)
		if got := tableRowsOf(t, d, "T"); got != want {
			t.Fatalf("%s :\n got %s\nwant %s", how, got, want)
		}
		undoOnce(t, d, before, how)
	}

	// Une ligne supprimée au milieu : les lignes vides du bas restent (elles
	// ne sont pas touchées).
	d := open(t)
	indiceTable(t, d, []string{"Nom", "Indice"}, [][]string{{"a", "1"}, {"b", "2"}, {"", ""}})
	must(t, d.DeleteRow(ctx, "T", 0))
	if got, want := tableRowsOf(t, d, "T"), "b|2 / |"; got != want {
		t.Fatalf("milieu :\n got %s\nwant %s", got, want)
	}

	// Tout vide sous la dernière : le tableau garde une ligne.
	d2 := open(t)
	indiceTable(t, d2, []string{"Nom", "Indice"}, [][]string{{"", ""}, {"", ""}, {"x", "1"}})
	must(t, d2.DeleteRow(ctx, "T", 2))
	if got := tablesOf(t, d2)[0].Total; got != 1 {
		t.Fatalf("dernière ligne : %d lignes", got)
	}
}

// La colonne numérotée d'office (clé d'entiers) ne retient pas une ligne.
func TestTrimBottomAutoKey(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	indiceTable(t, d, []string{"ID", "Nom"}, [][]string{{"1", "a"}, {"2", "b"}, {"3", ""}, {"4", ""}, {"5", "x"}})
	must(t, d.SetKey(ctx, "T", 0))
	before := dump(t, d)
	must(t, d.DeleteRow(ctx, "T", 4))
	if got, want := tableRowsOf(t, d, "T"), "1|a / 2|b"; got != want {
		t.Fatalf("clé :\n got %s\nwant %s", got, want)
	}
	undoOnce(t, d, before, "clé")
}

// Une ligne insérée vide exprès reste : rien ne part à l'insertion, ni
// quand une suppression ou un effacement ne touche pas la dernière ligne.
func TestTrimBottomKeepsInsertedRow(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	indiceTable(t, d, []string{"Nom", "Indice"}, [][]string{{"a", "1"}, {"b", "2"}, {"c", "3"}})
	must(t, d.InsertRow(ctx, "T", 3)) // à la fin
	must(t, d.InsertRow(ctx, "T", 1)) // au milieu
	if got, want := tableRowsOf(t, d, "T"), "a|1 / | / b|2 / c|3 / |"; got != want {
		t.Fatalf("insertion :\n got %s\nwant %s", got, want)
	}
	must(t, d.ClearRange(ctx, 1, 4, 1, 4, 1, false)) // Indice de c
	must(t, d.DeleteRow(ctx, "T", 0))
	if got, want := tableRowsOf(t, d, "T"), "| / b|2 / c| / |"; got != want {
		t.Fatalf("ligne insérée :\n got %s\nwant %s", got, want)
	}
}
