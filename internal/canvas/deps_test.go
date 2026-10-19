package canvas

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// depsOf résume les antécédents ou les dépendants : « kind ligne,col[+n] note ».
func depsOf(l []DepTarget) string {
	var out []string
	for _, x := range l {
		s := fmt.Sprintf("%s %d,%d", x.Kind, x.Row, x.Col)
		if x.Rows > 1 {
			s += fmt.Sprintf("+%d", x.Rows)
		}
		if x.Note != "" {
			s += " (" + x.Note + ")"
		}
		out = append(out, s)
	}
	return strings.Join(out, " | ")
}

// Audit des formules : une référence à une ligne précise d'une colonne
// (Tableau[Col][n], [Col][a:b], [Col][-1], REFERENCE) vise la cellule de
// cette ligne, pas l'en-tête de la colonne (bogue de Louis : O8 =
// Suividesnids[NID suivi][1] partait de l'en-tête).
func TestDependenciesRows(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	// Tableau Feuille1 : en-tête ligne 0, Nid et Val en colonnes 0 et 1.
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nid", "Val", "Avant"}, {"a", "1", ""}, {"b", "2", ""}, {"c", "3", ""}, {"d", "4", ""}}); err != nil {
		t.Fatal(err)
	}
	if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	set := func(row, col int, raw string) {
		t.Helper()
		if _, err := d.SetCell(ctx, 1, row, col, raw); err != nil {
			t.Fatalf("%s : %v", raw, err)
		}
	}
	set(0, 6, "=Feuille1[Nid][2]")
	set(1, 6, "=SOMME(Feuille1[Val][2:3])")
	set(2, 6, "=Feuille1[Nid][9]")
	set(3, 6, "=Feuille1[Val]")
	set(1, 2, "=[Val][-1]") // colonne calculée Avant
	pre := func(row, col int) string {
		t.Helper()
		dep, err := d.Dependencies(ctx, 1, row, col)
		if err != nil {
			t.Fatal(err)
		}
		return depsOf(dep.Precedents)
	}
	dependents := func(row, col int) string {
		t.Helper()
		dep, err := d.Dependencies(ctx, 1, row, col)
		if err != nil {
			t.Fatal(err)
		}
		return depsOf(dep.Dependents)
	}
	for _, c := range []struct {
		row, col int
		want     string
	}{
		{0, 6, "row 2,0"},   // 2e ligne de données : feuille ligne 2
		{1, 6, "row 2,1+2"}, // lignes 2 à 3 encadrées
		{2, 6, "column 0,0 (Feuille1[Nid] : la ligne 9 n'existe pas)"},
		{3, 6, "column 0,1"}, // toute la colonne
		{3, 2, "row 2,1"},    // [Val][-1] à la 3e ligne : la 2e
		{1, 2, "column 0,1 (Feuille1[Val] : pas de ligne 1 au-dessus de la ligne 1 (valeur vide))"},
		{0, 2, "column 0,1"}, // en-tête : la colonne
	} {
		if got := pre(c.row, c.col); got != c.want {
			t.Errorf("antécédents de %d,%d : %q, attendu %q", c.row, c.col, got, c.want)
		}
	}
	// Dépendants : seule la cellule lue par Feuille1[Nid][2] en dépend.
	if got := dependents(2, 0); got != "cell 0,6" {
		t.Errorf("dépendants de Nid ligne 2 : %q", got)
	}
	if got := dependents(1, 0); got != "" {
		t.Errorf("dépendants de Nid ligne 1 : %q", got)
	}
	// Val ligne 3 : la plage 2:3, la colonne entière, et Avant à la ligne 4.
	if got := dependents(3, 1); got != "row 4,2 | cell 1,6 | cell 3,6" {
		t.Errorf("dépendants de Val ligne 3 : %q", got)
	}
	if got := dependents(0, 1); got != "column 0,2 | cell 1,6 | cell 3,6" {
		t.Errorf("dépendants de l'en-tête Val : %q", got)
	}

	// REFERENCE : la ligne de la clé.
	if err := d.SetKey(ctx, "Feuille1", 0); err != nil {
		t.Fatal(err)
	}
	set(4, 6, `=REFERENCE(Feuille1[Val]; "c")`)
	set(5, 6, `=REFERENCE(Feuille1[Val]; "z")`)
	set(6, 6, `=REFERENCE(Feuille1[Val]; G1)`)
	if got := pre(4, 6); got != "row 3,1" {
		t.Errorf("REFERENCE clé c : %q", got)
	}
	if got := pre(5, 6); got != "column 0,1 (Feuille1[Val] : clé « z » introuvable)" {
		t.Errorf("REFERENCE clé absente : %q", got)
	}
	if got := pre(6, 6); !strings.HasPrefix(got, "column 0,1") {
		t.Errorf("REFERENCE clé calculée : %q", got)
	}

	// Ligne de la feuille masquée : la flèche vise la colonne, avec une note.
	if err := d.SetLinesHidden(ctx, 1, true, 3, 3, true); err != nil {
		t.Fatal(err)
	}
	if got := pre(4, 6); got != "column 0,1 (Feuille1[Val] : la ligne 3 est masquée sur la feuille)" {
		t.Errorf("ligne masquée : %q", got)
	}
	if err := d.SetLinesHidden(ctx, 1, true, 3, 3, false); err != nil {
		t.Fatal(err)
	}

	// Filtre : la 2e ligne (b) masquée, la flèche vise la colonne avec une
	// note ; la 3e (c) est affichée en 2e position.
	if err := d.SetFilter(ctx, "Feuille1", 0, &ColumnFilter{Values: []string{"a", "c", "d"}}); err != nil {
		t.Fatal(err)
	}
	if got := pre(0, 6); got != "column 0,0 (Feuille1[Nid] : la ligne 2 est masquée (filtre ou onglet))" {
		t.Errorf("ligne filtrée : %q", got)
	}
	if got := pre(4, 6); got != "row 2,1" {
		t.Errorf("REFERENCE sous filtre : %q", got)
	}
	if got := pre(1, 6); got != "row 2,1 (Feuille1[Val] : 1 des lignes 2 à 3 sont masquées (filtre ou onglet))" {
		t.Errorf("plage sous filtre : %q", got)
	}
	// [Val][-1] sur la 2e ligne affichée (c, rang 3) : b, masquée.
	if got := pre(2, 2); got != "column 0,1 (Feuille1[Val] : la ligne 2 est masquée (filtre ou onglet))" {
		t.Errorf("relative sous filtre : %q", got)
	}
}

// REFERENCE dans une colonne calculée : la clé est la valeur d'une colonne
// de la ligne de la formule ([Client]), la flèche vise la ligne trouvée.
func TestDependenciesReferenceRow(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Id", "Ville"}, {"10", "Paris"}, {"20", "Lyon"}, {"30", "Nice"}}); err != nil {
		t.Fatal(err)
	}
	if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetKey(ctx, "Feuille1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Paste(ctx, 1, 0, 4, [][]string{{"Client", "Ville"}, {"30", ""}, {"10", ""}}); err != nil {
		t.Fatal(err)
	}
	all, _ := d.AllTables(ctx)
	if len(all) != 2 {
		t.Fatalf("tableaux : %+v", all)
	}
	second := all[1].Name
	if err := promoteIfData(ctx, d, second); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 5, "=REFERENCE(Feuille1[Ville]; [Client])"); err != nil {
		t.Fatal(err)
	}
	dep, err := d.Dependencies(ctx, 1, 1, 5) // Client 30 : Nice, 3e ligne
	if err != nil {
		t.Fatal(err)
	}
	if got := depsOf(dep.Precedents); !strings.Contains(got, "row 3,1") {
		t.Fatalf("antécédents : %q", got)
	}
	dep, _ = d.Dependencies(ctx, 1, 2, 5) // Client 10 : Paris, 1re ligne
	if got := depsOf(dep.Precedents); !strings.Contains(got, "row 1,1") {
		t.Fatalf("antécédents : %q", got)
	}
	dep, _ = d.Dependencies(ctx, 1, 0, 5) // en-tête : la colonne
	if got := depsOf(dep.Precedents); !strings.Contains(got, "column 0,1") {
		t.Fatalf("antécédents de l'en-tête : %q", got)
	}
}

// En-têtes sur deux lignes : la 1re ligne de données est sous l'en-tête
// (pas sous la bande des groupes).
func TestDependenciesRowsTwoLevel(t *testing.T) {
	ctx := context.Background()
	d := twoLevelFixture(t)
	if err := d.SetTwoLevel(ctx, "Inv", true); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 10, "=Inv[BOISEMENT.Statut][1]"); err != nil {
		t.Fatal(err)
	}
	dep, err := d.Dependencies(ctx, 1, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := depsOf(dep.Precedents); got != "row 4,1" {
		t.Fatalf("antécédents : %q", got)
	}
}
