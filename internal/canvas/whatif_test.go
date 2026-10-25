package canvas

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"
)

// rangeCells pose un bloc de cellules libres (sans faire de tableau).
func rangeCells(t *testing.T, d *fixture, row, col int, rows ...[]string) {
	t.Helper()
	if _, err := d.PasteRange(context.Background(), 1, row, col, rows); err != nil {
		t.Fatal(err)
	}
}

func shown(t *testing.T, d *fixture, sheet int64, row, col int) string {
	t.Helper()
	return freeValue(t, d, sheet, row, col)
}

func num(t *testing.T, s string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil {
		t.Fatalf("%q : pas un nombre", s)
	}
	return f
}

// Valeur cible : linéaire (un pas), non linéaire, sans solution ; rien ne
// change avant « Conserver », qui est une étape d'annulation.
func TestGoalSeek(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	rangeCells(t, d, 0, 0, []string{"4", "=A1*3+2", "=A1*A1", "=A1*A1+1"})
	start := dump(t, d)

	r, err := d.GoalSeek(ctx, 1, "B1", "20", "A1")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Found || r.Raw != "6" || r.Result != "20" || r.Cell != "A1" {
		t.Fatalf("linéaire : %+v", r)
	}
	if got := dump(t, d); got != start {
		t.Fatalf("la recherche a changé le classeur :\n%s", diff(start, got))
	}
	if err := d.SetWhatIfValues(ctx, 1, "goal", []string{r.Cell}, []string{r.Raw}); err != nil {
		t.Fatal(err)
	}
	if v := shown(t, d, 1, 0, 1); v != "20" {
		t.Errorf("conservé : B1 = %q", v)
	}
	undoAll(t, d, 1, start)

	r, err = d.GoalSeek(ctx, 1, "$C$1", "2", "A1")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Found || math.Abs(num(t, r.Raw)-math.Sqrt2) > 1e-9 {
		t.Errorf("non linéaire : %+v", r)
	}

	r, err = d.GoalSeek(ctx, 1, "D1", "0", "A1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Found {
		t.Errorf("sans solution : %+v", r)
	}

	// Erreurs : cible sans formule, cellule à modifier avec formule.
	if _, err := d.GoalSeek(ctx, 1, "A1", "1", "A1"); err == nil {
		t.Error("cible sans formule acceptée")
	}
	if _, err := d.GoalSeek(ctx, 1, "B1", "1", "C1"); err == nil {
		t.Error("cellule à modifier avec formule acceptée")
	}
	if _, err := d.GoalSeek(ctx, 1, "B1", "abc", "A1"); err == nil {
		t.Error("valeur non numérique acceptée")
	}
}

// Valeur cible à travers un tableau : la cellule à modifier est une cellule
// saisie, la cible une formule libre sur la colonne calculée.
func TestGoalSeekTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix", "Qté"}, {"Café", "3", "10"}, {"Thé", "2", "5"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 3, "=[@Prix]*[@Qté]"); err != nil {
		t.Fatal(err)
	}
	// Nom de la colonne calculée : celui que Lattis lui a donné.
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	tb := l.Tables[0]
	total := tb.Columns[3].Name
	if _, err := d.SetCell(ctx, 1, 6, 0, "=SOMME("+tb.Name+"["+total+"])"); err != nil {
		t.Fatal(err)
	}
	if v := shown(t, d, 1, 6, 0); v != "40" {
		t.Fatalf("total : %q", v)
	}
	// Total 100 en changeant le prix du café : (100 - 10) / 10 = 9.
	r, err := d.GoalSeek(ctx, 1, "A7", "100", "B2")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Found || r.Raw != "9" {
		t.Fatalf("tableau : %+v", r)
	}
	// Cible : une cellule de la colonne calculée.
	r, err = d.GoalSeek(ctx, 1, "D3", "30", "C3")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Found || r.Raw != "15" {
		t.Fatalf("colonne calculée : %+v", r)
	}
	if err := d.SetWhatIfValues(ctx, 1, "goal", []string{"C3"}, []string{r.Raw}); err != nil {
		t.Fatal(err)
	}
	if v := shown(t, d, 1, 6, 0); v != "60" {
		t.Errorf("après conserver : %q", v)
	}
	if _, err := d.GoalSeek(ctx, 1, "A7", "1", "D2"); err == nil {
		t.Error("colonne calculée comme cellule à modifier acceptée")
	}
}

// Table de données à une et deux variables : valeurs fixes, une étape
// d'annulation, définition gardée pour la relancer.
func TestDataTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	rangeCells(t, d, 0, 0, []string{"1"}, []string{"2"})
	// Une variable en colonne : D2:D4 valeurs, E1 et F1 formules.
	rangeCells(t, d, 0, 3, []string{"", "=A1*10", "=A1+A2"}, []string{"1"}, []string{"2"}, []string{"3"})
	start := dump(t, d)
	n, err := d.DataTable(ctx, 1, Rect{0, 3, 3, 5}, "", "A1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 6 {
		t.Errorf("résultats : %d", n)
	}
	want := map[[2]int]string{{1, 4}: "10", {2, 4}: "20", {3, 4}: "30", {1, 5}: "3", {2, 5}: "4", {3, 5}: "5"}
	for k, v := range want {
		if got := shown(t, d, 1, k[0], k[1]); got != v {
			t.Errorf("%v : %q, attendu %q", k, got, v)
		}
	}
	if def, err := d.DataTableAt(ctx, 1, Rect{0, 3, 3, 5}); err != nil || def == nil || def.Col != "A1" || def.Row != "" {
		t.Errorf("définition : %+v %v", def, err)
	}
	// Les entrées n'ont pas changé.
	if v := shown(t, d, 1, 0, 0); v != "1" {
		t.Errorf("A1 = %q", v)
	}
	undoAll(t, d, 1, start)

	// Une variable en ligne : H1:J1 valeurs, G2 formule.
	rangeCells(t, d, 0, 6, []string{"", "5", "6", "7"}, []string{"=A1*2"})
	if _, err := d.DataTable(ctx, 1, Rect{0, 6, 1, 9}, "A1", ""); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[[2]int]string{{1, 7}: "10", {1, 8}: "12", {1, 9}: "14"} {
		if got := shown(t, d, 1, k[0], k[1]); got != v {
			t.Errorf("en ligne %v : %q, attendu %q", k, got, v)
		}
	}

	// Deux variables : coin L1 = A1*A2, M1:N1 → A1, L2:L3 → A2.
	rangeCells(t, d, 0, 11, []string{"=A1*A2", "2", "3"}, []string{"4"}, []string{"5"})
	if _, err := d.DataTable(ctx, 1, Rect{0, 11, 2, 13}, "A1", "A2"); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[[2]int]string{{1, 12}: "8", {1, 13}: "12", {2, 12}: "10", {2, 13}: "15"} {
		if got := shown(t, d, 1, k[0], k[1]); got != v {
			t.Errorf("deux variables %v : %q, attendu %q", k, got, v)
		}
	}
	// Relancée après un changement d'entrée qui n'est pas une variable : les
	// mêmes ; la cellule d'entrée dans la plage est refusée.
	if _, err := d.DataTable(ctx, 1, Rect{0, 11, 2, 13}, "L1", "A2"); err == nil {
		t.Error("cellule d'entrée dans la plage acceptée")
	}
	if _, err := d.DataTable(ctx, 1, Rect{0, 11, 2, 13}, "", ""); err == nil {
		t.Error("sans cellule d'entrée acceptée")
	}
}

// Scénarios : ajout, modification, affichage (une étape d'annulation),
// suppression et synthèse sur un nouvel onglet.
func TestScenarios(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	rangeCells(t, d, 0, 0, []string{"4", "", "1", "", "=A1*3+C1"})
	cur, err := d.WhatIfCells(ctx, 1, "A1;C1")
	if err != nil || len(cur) != 2 || cur[0].Value != "4" || cur[1].Value != "1" {
		t.Fatalf("valeurs actuelles : %+v %v", cur, err)
	}
	if _, err := d.WhatIfCells(ctx, 1, "E1"); err == nil {
		t.Error("cellule avec formule acceptée")
	}
	if err := d.SaveScenario(ctx, 1, "", Scenario{Name: "Haut", Cells: []string{"A1", "C1"}, Values: []string{"10", "2,5"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveScenario(ctx, 1, "", Scenario{Name: "Bas", Cells: []string{"A1"}, Values: []string{"1"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveScenario(ctx, 1, "", Scenario{Name: "haut", Cells: []string{"A1"}, Values: []string{"1"}}); err == nil {
		t.Error("nom en double accepté")
	}
	if err := d.SaveScenario(ctx, 1, "Bas", Scenario{Name: "Bas", Cells: []string{"A1"}, Values: []string{"2"}}); err != nil {
		t.Fatal(err)
	}
	list, err := d.Scenarios(ctx, 1)
	if err != nil || len(list) != 2 || list[0].Values[1] != "2,5" || list[1].Values[0] != "2" {
		t.Fatalf("liste : %+v %v", list, err)
	}
	start := dump(t, d)
	if err := d.ShowScenario(ctx, 1, "Haut"); err != nil {
		t.Fatal(err)
	}
	if v := shown(t, d, 1, 0, 4); v != "32,5" {
		t.Errorf("Haut : E1 = %q", v)
	}
	undoAll(t, d, 1, start)

	id, err := d.ScenarioSummary(ctx, 1, "E1")
	if err != nil {
		t.Fatal(err)
	}
	if id == 1 {
		t.Fatal("pas de nouvel onglet")
	}
	// Ligne 2 : en-têtes ; 4 et 5 : A1 et C1 ; 7 : E1.
	want := map[[2]int]string{
		{2, 1}: "Valeurs actuelles", {2, 2}: "Haut", {2, 3}: "Bas",
		{4, 0}: "A1", {4, 1}: "4", {4, 2}: "10", {4, 3}: "2",
		{5, 0}: "C1", {5, 1}: "1", {5, 2}: "2,5", {5, 3}: "1",
		{7, 0}: "E1", {7, 1}: "13", {7, 2}: "32,5", {7, 3}: "7",
	}
	for k, v := range want {
		if got := shown(t, d, id, k[0], k[1]); got != v {
			t.Errorf("synthèse %v : %q, attendu %q", k, got, v)
		}
	}
	// La feuille d'origine n'a pas changé.
	if v := shown(t, d, 1, 0, 0); v != "4" {
		t.Errorf("A1 après la synthèse : %q", v)
	}
	if err := d.DeleteScenario(ctx, 1, "Bas"); err != nil {
		t.Fatal(err)
	}
	if list, _ := d.Scenarios(ctx, 1); len(list) != 1 {
		t.Errorf("après suppression : %+v", list)
	}
}

// Solveur : optimum linéaire connu, entiers, irréalisable, non linéaire.
func TestSolver(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	// x = A1, y = B1 ; objectif D1 = 3x + 5y ; A3 = x, B3 = 2y, C3 = 3x + 2y.
	rangeCells(t, d, 0, 0, []string{"0", "0", "", "=3*A1+5*B1"})
	rangeCells(t, d, 2, 0, []string{"=A1", "=2*B1", "=3*A1+2*B1"})
	start := dump(t, d)
	p := SolverParams{Objective: "D1", Goal: "max", Variables: "A1:B1", NonNegative: true,
		Constraints: []SolverConstraint{{Ref: "A3", Op: "<=", RHS: "4"}, {Ref: "B3", Op: "<=", RHS: "12"}, {Ref: "C3", Op: "<=", RHS: "18"}}}
	r, err := d.Solve(ctx, 1, p)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "optimal" || !r.Linear || r.Raws[0] != "2" || r.Raws[1] != "6" || r.Objective != "36" {
		t.Fatalf("linéaire : %+v", r)
	}
	if got := dump(t, d); got != start {
		t.Fatalf("le solveur a changé le classeur :\n%s", diff(start, got))
	}
	if err := d.SetWhatIfValues(ctx, 1, "solver", r.Cells, r.Raws); err != nil {
		t.Fatal(err)
	}
	if v := shown(t, d, 1, 0, 3); v != "36" {
		t.Errorf("conservé : D1 = %q", v)
	}
	undoAll(t, d, 1, start)

	// Entiers : max 5x + 4y, 6x + 4y <= 24, x + 2y <= 6 → (4, 0), 20.
	rangeCells(t, d, 4, 0, []string{"=5*A1+4*B1", "=6*A1+4*B1", "=A1+2*B1"})
	p = SolverParams{Objective: "A5", Goal: "max", Variables: "A1;B1", NonNegative: true,
		Constraints: []SolverConstraint{{Ref: "B5", Op: "<=", RHS: "24"}, {Ref: "C5", Op: "<=", RHS: "6"}, {Ref: "A1:B1", Op: "int"}}}
	if r, err = d.Solve(ctx, 1, p); err != nil {
		t.Fatal(err)
	}
	if r.Status != "optimal" || r.Raws[0] != "4" || r.Raws[1] != "0" || r.Objective != "20" {
		t.Errorf("entiers : %+v", r)
	}
	// Valeur de : 3x + 5y = 10 avec x = 0 (binaire et borne).
	p = SolverParams{Objective: "D1", Goal: "value", Value: "10", Variables: "A1:B1", NonNegative: true,
		Constraints: []SolverConstraint{{Ref: "A1", Op: "=", RHS: "0"}}}
	if r, err = d.Solve(ctx, 1, p); err != nil {
		t.Fatal(err)
	}
	if r.Status != "optimal" || r.Raws[1] != "2" {
		t.Errorf("valeur de : %+v", r)
	}
	// Irréalisable : x >= 5 et 3x + 2y <= 6 (y >= 0).
	p = SolverParams{Objective: "D1", Goal: "min", Variables: "A1:B1", NonNegative: true,
		Constraints: []SolverConstraint{{Ref: "A1", Op: ">=", RHS: "5"}, {Ref: "C3", Op: "<=", RHS: "6"}}}
	if r, err = d.Solve(ctx, 1, p); err != nil {
		t.Fatal(err)
	}
	if r.Status != "infeasible" {
		t.Errorf("irréalisable : %+v", r)
	}
	// Non linéaire : min (x - 3)² + (y + 1)² + 1, y >= 0 → (3, 0), 2.
	rangeCells(t, d, 6, 0, []string{"=(A1-3)^2+(B1+1)^2+1"})
	p = SolverParams{Objective: "A7", Goal: "min", Variables: "A1:B1", NonNegative: true}
	if r, err = d.Solve(ctx, 1, p); err != nil {
		t.Fatal(err)
	}
	if r.Status != "approx" || r.Linear || math.Abs(num(t, r.Raws[0])-3) > 1e-3 || math.Abs(num(t, r.Raws[1])) > 1e-3 {
		t.Errorf("non linéaire : %+v", r)
	}
}
