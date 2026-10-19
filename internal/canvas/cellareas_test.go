package canvas

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
)

// cellAt donne la cellule libre (row, col) d'une feuille telle qu'affichée.
func cellAt(t *testing.T, d *fixture, sheet int64, row, col int) CellInfo {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range l.Cells {
		if c.Row == row && c.Col == col {
			return c
		}
	}
	t.Fatalf("cellule (%d, %d) introuvable", row, col)
	return CellInfo{}
}

// pivotFixture : une source et un TCD Pays × Année (somme des ventes) posé en
// B2 de la feuille « TCD 1 » :
//
//	B2 Pays  C2 2024  D2 2025  E2 Total
//	B3 FR    C3 11    D3 7     E3 18
//	B4 IT    C4 5     D4       E4 5
//	B5 Total C5 16    D5 7     E5 23
func pivotFixture(t *testing.T) (*fixture, int64) {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Année", "Ventes"}, {"FR", "2024", "10"}, {"IT", "2024", "5"}, {"FR", "2025", "7"}, {"FR", "2024", "1"}}); err != nil {
		t.Fatal(err)
	}
	sheet, err := d.CreatePivot(ctx, PivotConfig{Source: "Feuille1", RowFields: []string{"Pays"}, Cols: "Année", Values: []PivotValue{{Field: "Ventes", Agg: "sum"}}})
	if err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, sheet)
	if got := fmt.Sprint(l.Pivots[0].Cells); got != "[[Pays 2024 2025 Total] [FR 11 7 18] [IT 5  5] [Total 16 7 23]]" {
		t.Fatalf("TCD : %s", got)
	}
	return d, sheet
}

// Une formule qui cite une cellule d'un TCD lit sa valeur affichée (bogue :
// elle lisait une cellule vide).
func TestPivotCellReference(t *testing.T) {
	ctx := context.Background()
	d, sheet := pivotFixture(t)
	// Requête de l'onglet SQL, avant toute formule.
	if q, err := d.CellSQL(ctx, "SELECT Cell[D3], Cell[B5]", sheet); err != nil || q != "SELECT 7, 'Total'" {
		t.Errorf("SQL : %q, %v", q, err)
	}
	formulas := []struct {
		raw, want string
	}{
		{"=Cell[C3]", "11"},
		{"=Cell[C3]*2", "22"}, // un nombre, pas un texte
		{"=Cell[B3]", "FR"},   // étiquette
		{"=Cell[C2]", "2024"}, // en-tête
		{"=Cell[E5]", "23"},   // total général
		{"=Cell[E3]", "18"},   // total de ligne
		{"=ESTVIDE(Cell[D4])", "VRAI"},
		{"=ESTNUM(Cell[C3])", "VRAI"},
		{"=ESTTEXTE(Cell[B3])", "VRAI"},
		{"=SOMME(Cell[C3:C4])", "16"},
		{"=SOMME(Cell[C3:E4])", "46"},
		{"=NB(Cell[B3:E5])", "8"},
		{"=Cell[B5]&\" : \"&Cell[E5]", "Total : 23"},
		{"=C3+1", "12"}, // adresse tapée à la Excel
	}
	for i, f := range formulas {
		if _, err := d.SetCell(ctx, sheet, 10+i, 0, f.raw); err != nil {
			t.Fatalf("%s : %v", f.raw, err)
		}
	}
	for i, f := range formulas {
		if c := cellAt(t, d, sheet, 10+i, 0); c.Value != f.want || c.Error {
			t.Errorf("%s = %q, attendu %q", f.raw, c.Value, f.want)
		}
	}
	// D'une autre feuille.
	if _, err := d.SetCell(ctx, 1, 0, 6, "='TCD 1'!Cell[E5]/2"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 6, "=SOMME('TCD 1'!Cell[C3:D3])"); err != nil {
		t.Fatal(err)
	}
	if c := cellAt(t, d, 1, 0, 6); c.Value != "11,5" {
		t.Errorf("autre feuille : %+v", c)
	}
	if c := cellAt(t, d, 1, 1, 6); c.Value != "18" {
		t.Errorf("plage d'une autre feuille : %+v", c)
	}
	// Requête de l'onglet SQL.
	q, err := d.CellSQL(ctx, "SELECT Cell[C3] + 1, 'TCD 1'!Cell[B4]", sheet)
	if err != nil || q != "SELECT 11 + 1, 'IT'" {
		t.Errorf("SQL : %q, %v", q, err)
	}
	// Cellule nommée posée sur le TCD.
	if err := d.SetName(ctx, sheet, 4, 4, "Grand_total"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 2, 6, "=Grand_total*10"); err != nil {
		t.Fatal(err)
	}
	if c := cellAt(t, d, 1, 2, 6); c.Value != "230" {
		t.Errorf("cellule nommée : %+v", c)
	}

	// La source change : les formules suivent.
	if _, err := d.SetCell(ctx, 1, 2, 2, "50"); err != nil { // IT 2024 : 5 → 50
		t.Fatal(err)
	}
	if c := cellAt(t, d, sheet, 19, 0); c.Value != "61" { // SOMME(Cell[C3:C4])
		t.Errorf("après modification de la source : %+v", c)
	}
	if c := cellAt(t, d, 1, 0, 6); c.Value != "34" {
		t.Errorf("autre feuille après modification : %+v", c)
	}
	// La configuration change : nombre de lignes au lieu de la somme.
	l, _ := d.Layout(ctx, sheet)
	if err := d.SetPivot(ctx, l.Pivots[0].ID, PivotConfig{Source: "Feuille1", RowFields: []string{"Pays"}, Cols: "Année", Values: []PivotValue{{Agg: "count"}}}); err != nil {
		t.Fatal(err)
	}
	if c := cellAt(t, d, sheet, 10, 0); c.Value != "2" {
		t.Errorf("après modification du TCD : %+v", c)
	}
	// Annuler le tout rend les valeurs d'origine.
	for range 2 {
		d.bus.Undo(ctx)
	}
	if c := cellAt(t, d, sheet, 10, 0); c.Value != "11" {
		t.Errorf("après annulation : %+v", c)
	}
}

// Pourcentages et filtres de rapport : la valeur affichée au-dessus du TCD
// se lit aussi ; « 25 % » vaut 0,25.
func TestPivotCellReferenceFiltersPercent(t *testing.T) {
	ctx := context.Background()
	d, sheet := pivotFixture(t)
	l, _ := d.Layout(ctx, sheet)
	cfg := PivotConfig{Source: "Feuille1", RowFields: []string{"Année"}, Values: []PivotValue{{Field: "Ventes", Agg: "sum", Show: "pctTotal"}},
		Filters: []PivotFilter{{Field: "Pays", Values: []string{"FR"}}}}
	if err := d.SetPivot(ctx, l.Pivots[0].ID, cfg); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, sheet)
	// B2 Pays | C2 FR ; ligne 3 vide ; B4 en-tête, B5 2024 (11/18), B6 2025, B7 Total.
	if got := fmt.Sprint(l.Pivots[0].Cells); !strings.HasPrefix(got, "[[Année Somme de Ventes (% du total)] [2024 61,1 %]") {
		t.Fatalf("TCD : %s", got)
	}
	for i, raw := range []string{"=Cell[B2]", "=Cell[C2]", "=ARRONDI(Cell[C5];4)", "=Cell[C7]", "=ESTVIDE(Cell[B3])"} {
		if _, err := d.SetCell(ctx, sheet, 10+i, 0, raw); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range []string{"Pays", "FR", "0,611", "1", "VRAI"} {
		if c := cellAt(t, d, sheet, 10+i, 0); c.Value != want {
			t.Errorf("ligne %d : %q, attendu %q", 10+i, c.Value, want)
		}
	}
}

// Une colonne calculée du tableau source qui lit le TCD : pas de boucle (le
// TCD ne lit pas les colonnes calculées), la valeur suit.
func TestPivotCellReferenceFromSourceColumn(t *testing.T) {
	ctx := context.Background()
	d, _ := pivotFixture(t)
	if _, err := d.SetCell(ctx, 1, 0, 3, "Part"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 3, "=[@Ventes]/'TCD 1'!Cell[E5]"); err != nil {
		t.Fatal(err)
	}
	exprs, args, _, err := d.ColumnExprs(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := d.db.Read().Query(`SELECT `+exprs[3]+` FROM t_1 ORDER BY _pos`, args...)
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
	if len(got) != 4 || got[0] != formula.FormatNumber(10.0/23) {
		t.Fatalf("Part = %v", got)
	}
}

// Une formule qui cite une cellule recouverte par un déversement (FILTRE…)
// lit la valeur déversée ; l'ancre vaut la première valeur.
func TestSpillCellReference(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}, {"FR", "7"}}); err != nil {
		t.Fatal(err)
	}
	// Formules écrites avant et après le déversement (ordre de calcul).
	if _, err := d.SetCell(ctx, 1, 10, 0, "=Cell[F2]*2"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 4, `=FILTRE(Feuille1;Feuille1[Pays]="FR")`); err != nil {
		t.Fatal(err)
	}
	for i, raw := range []string{"=Cell[E1]", "=SOMME(Cell[F1:F2])", "=Feuille1!Cell[E2]"} {
		if _, err := d.SetCell(ctx, 1, 11+i, 0, raw); err != nil {
			t.Fatal(err)
		}
	}
	// Un second tableau dynamique qui lit le premier.
	if _, err := d.SetCell(ctx, 1, 0, 8, "=FILTRE(Feuille1;Feuille1[Ventes]>=Cell[F2])"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 14, 0, "=Cell[J2]"); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"14", "FR", "17", "FR", "7"} {
		if c := cellAt(t, d, 1, 10+i, 0); c.Value != want || c.Error {
			t.Errorf("ligne %d : %+v, attendu %q", 10+i, c, want)
		}
	}
	// Déversement bloqué : il ne recouvre plus rien.
	if _, err := d.SetCell(ctx, 1, 1, 5, "x"); err != nil {
		t.Fatal(err)
	}
	if c := cellAt(t, d, 1, 12, 0); c.Value != "0" {
		t.Errorf("déversement bloqué : %+v", c)
	}
}

// Deux tableaux dynamiques qui lisent chacun une cellule vide que l'autre
// pourrait recouvrir : pas de fausse référence circulaire.
func TestSpillNoFalseCycle(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}, {"FR", "7"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 4, "=FILTRE(Feuille1;Feuille1[Ventes]>Cell[Z100])"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 29, 12, "=FILTRE(Feuille1;Feuille1[Ventes]>Cell[Z200]+6)"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 40, 0, "=Cell[F1]+Cell[N30]+Cell[F3]"); err != nil {
		t.Fatal(err)
	}
	if c := cellAt(t, d, 1, 40, 0); c.Value != "27" {
		t.Errorf("A41 : %+v", c)
	}
}
