package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// tableRowsOf lit un tableau : une ligne par ligne, valeurs séparées par
// « | », toutes les lignes (onglet à choix compris).
func tableRowsOf(t *testing.T, d *fixture, name string) string {
	t.Helper()
	var out []string
	var cols []ExportColumn
	must(t, d.ExportTable(context.Background(), name, func(c []ExportColumn) error {
		cols = c
		return nil
	}, func(vals []any) error {
		var r []string
		for i, v := range vals {
			r = append(r, fmt.Sprint(formulaText(v, cols[i].Type)))
		}
		out = append(out, strings.Join(r, "|"))
		return nil
	}))
	return strings.Join(out, " / ")
}

func indiceTable(t *testing.T, d *fixture, headers []string, rows [][]string) {
	t.Helper()
	must(t, d.Do(context.Background(), "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 0, 0, headers, rows)
		return err
	}))
}

// Décision 0009 : Suppr sur des cellules d'un tableau supprime les lignes
// qu'il laisse sans aucune valeur saisie, en une étape (Ctrl+Z les rend) ;
// une ligne où il reste quelque chose, ou vide sans que Suppr y ait rien
// effacé, reste.
func TestClearDropsEmptiedRows(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	indiceTable(t, d, []string{"Nom", "Indice"}, [][]string{
		{"a", "1"}, {"", "05"}, {"", "05"}, {"b", "05"}, {"", ""}, {"c", "3"},
	})
	before := dump(t, d)
	d.TakeNotice()
	// Indice des lignes 2 à 5 (la 5 était déjà vide).
	must(t, d.ClearRange(ctx, 1, 2, 1, 5, 1, false))
	check(t, d, "lignes vidées")
	if got, want := tableRowsOf(t, d, "T"), "a|1 / b| / | / c|3"; got != want {
		t.Fatalf("lignes :\n got %s\nwant %s", got, want)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "2 lignes vides supprimées (Ctrl+Z pour annuler)") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "lignes vidées")

	// Une seule ligne : message au singulier.
	must(t, d.ClearRange(ctx, 1, 2, 1, 2, 1, false))
	if n := d.TakeNotice(); !strings.Contains(n, "1 ligne vide supprimée") {
		t.Fatalf("message : %q", n)
	}
	if got, want := tableRowsOf(t, d, "T"), "a|1 / |5 / b|5 / | / c|3"; got != want {
		t.Fatalf("une ligne :\n got %s\nwant %s", got, want)
	}
}

// Une colonne calculée ne retient pas une ligne ; la clé, si.
func TestClearEmptiedRowsComputedAndKey(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	indiceTable(t, d, []string{"Nom", "Indice", "Calc"}, [][]string{{"a", "1", ""}, {"", "05", ""}, {"c", "3", ""}})
	if _, err := d.SetCell(ctx, 1, 1, 2, `=[@Indice]*2`); err != nil {
		t.Fatal(err)
	}
	if l, _ := d.Layout(ctx, 1); l.Tables[0].Columns[2].Formula == "" {
		t.Fatal("colonne non calculée")
	}
	must(t, d.ClearRange(ctx, 1, 2, 1, 2, 1, false))
	if got := tablesOf(t, d)[0].Total; got != 2 {
		t.Fatalf("colonne calculée : %d lignes", got)
	}

	d2 := open(t)
	indiceTable(t, d2, []string{"Code", "Indice"}, [][]string{{"A1", "1"}, {"A2", "05"}})
	must(t, d2.SetKey(ctx, "T", 0))
	must(t, d2.ClearRange(ctx, 1, 2, 1, 2, 1, false))
	if got, want := tableRowsOf(t, d2, "T"), "A1|1 / A2|"; got != want {
		t.Fatalf("clé :\n got %s\nwant %s", got, want)
	}
}

// Onglet à choix (cas de Louis) : la colonne de catégorie masquée, dont la
// valeur est imposée, ne retient pas la ligne ; sous « (Toutes) », elle est
// visible et compte.
func TestClearEmptiedRowsChoiceSheet(t *testing.T) {
	ctx := context.Background()
	d, _ := choiceSheet(t)
	must(t, d.SetSheetChoice(ctx, 1, "Feuille1", "Année", "2025"))
	before := dump(t, d)
	must(t, d.ClearRange(ctx, 1, 1, 1, 1, 2, false))
	check(t, d, "choix")
	if got, want := tableRowsOf(t, d, "Feuille1"), "2024|A|10 / 2024|B|5 / 2025|C|3"; got != want {
		t.Fatalf("2025 :\n got %s\nwant %s", got, want)
	}
	if shown, total := rowsOf(t, d); shown != 1 || total != 3 {
		t.Fatalf("lignes : %d affichées, %d en tout", shown, total)
	}
	undoOnce(t, d, before, "choix")
	if got, want := tableRowsOf(t, d, "Feuille1"), "2024|A|10 / 2024|B|5 / 2025|A|7 / 2025|C|3"; got != want {
		t.Fatalf("annulation :\n got %s\nwant %s", got, want)
	}
	must(t, d.ChooseSheetValue(ctx, 1, "", true))
	must(t, d.ClearRange(ctx, 1, 1, 1, 1, 2, false))
	if got, want := tableRowsOf(t, d, "Feuille1"), "2024|| / 2024|B|5 / 2025|A|7 / 2025|C|3"; got != want {
		t.Fatalf("(Toutes) :\n got %s\nwant %s", got, want)
	}
}

type lockAll struct{}

func (lockAll) TableLock(int64) error   { return errors.New("lecture seule") }
func (lockAll) TableShare(int64) string { return "published" }

// Garde-fous : un tableau qui ne se modifie pas (publié en lecture seule)
// garde ses lignes ; un tableau garde au moins une ligne ; une feuille
// protégée refuse l'effacement.
func TestClearEmptiedRowsGuards(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	indiceTable(t, d, []string{"Nom", "Indice"}, [][]string{{"", "05"}, {"", "05"}, {"a", "1"}})
	d.SetSharing(lockAll{})
	must(t, d.ClearRange(ctx, 1, 1, 1, 1, 1, false))
	if got := tablesOf(t, d)[0].Total; got != 3 {
		t.Fatalf("tableau verrouillé : %d lignes", got)
	}
	// Toutes les lignes vidées : la première reste (la colonne Indice, vide,
	// quitte le tableau).
	d3 := open(t)
	indiceTable(t, d3, []string{"Nom", "Indice"}, [][]string{{"", "05"}, {"", "05"}})
	must(t, d3.ClearRange(ctx, 1, 1, 1, 2, 1, false))
	if got, want := tableRowsOf(t, d3, "T"), ""; got != want || tablesOf(t, d3)[0].Total != 1 {
		t.Fatalf("dernière ligne :\n got %s\nwant %s", got, want)
	}

	d2 := open(t)
	indiceTable(t, d2, []string{"Nom", "Indice"}, [][]string{{"", "05"}, {"a", "1"}})
	must(t, d2.ProtectSheet(ctx, 1, ""))
	if err := d2.ClearRange(ctx, 1, 1, 1, 1, 1, false); !errors.As(err, &ErrProtected{}) {
		t.Fatalf("feuille protégée : %v", err)
	}
	if got := tablesOf(t, d2)[0].Total; got != 2 {
		t.Fatalf("feuille protégée : %d lignes", got)
	}
}
