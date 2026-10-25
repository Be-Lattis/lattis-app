package formula

import (
	"slices"
	"testing"
)

// DECALER, INDIRECT, CELLULE, AGREGAT et fonctions de base de données, sur
// les deux backends. Cellules du fixture : A1=1, A2=2, A3="x", B1..B3 = 10,
// 20, 30 ; critères en E1:F3 (Qté ">2" ou Produit commençant par « Cla ») et
// G1:G2 (Produit « Souris »).
func TestReferenceFunctions(t *testing.T) {
	checkNum(t, []numCase{
		{`=INDIRECT("A1")`, "1", 0},
		{`=INDIRECT("Cell[B2]")`, "20", 0},
		{`=INDIRECT("A"&Cell[A2])`, "2", 0},
		{`=SOMME(INDIRECT("B1:B3"))`, "60", 0},
		{`=SOMME(INDIRECT("Cell[B1:B3]"))*2`, "120", 0},
		{`=INDIRECT("Ventes[Prix][2]")`, "10", 0},
		{`=SOMME(INDIRECT("Ventes[Qté]"))`, "11", 0},
		{`=INDIRECT("Z99")`, "", 0},
		{`=INDIRECT("A1";FAUX)`, "#REF!", 0},
		{`=INDIRECT("1+")`, "#REF!", 0},
		{`=INDIRECT(Ventes[Prix])`, "#REF!", 0},
		{`=INDIRECT("Ventes")`, "#REF!", 0},
		{`@2=INDIRECT("[Prix]")`, "10", 0},
		{`=DECALER(Cell[A1];1;1)`, "20", 0},
		{`=SOMME(DECALER(Cell[A1];0;1;3))`, "60", 0},
		{`=SOMME(DECALER(Cell[A1];0;1;3;1)*2)`, "120", 0},
		{`=LIGNES(DECALER(Cell[A1];0;0;3;2))`, "3", 0},
		{`=DECALER(Cell[A1];-1;0)`, "#REF!", 0},
		{`=DECALER(Cell[A1];0;0;0)`, "#REF!", 0},
		{`=DECALER(Cell[A1];Ventes[Qté];0)`, "#REF!", 0},
		{`=DECALER(INDIRECT("A1");0;1)`, "10", 0},
		{`=DECALER(Ventes[Prix][1];1;0)`, "10", 0},
		{`=SOMME(DECALER(Ventes[Prix];0;1))`, "11", 0},
		{`=SOMME(DECALER(Ventes[Prix][1];0;0;2))`, "35,5", 0},
		{`=DECALER(Ventes[Prix];0;-5)`, "#REF!", 0},
		{`=SOMME(DECALER(Ventes[Prix];1;0))`, "#REF!", 0},
		{`@2=DECALER([Prix];0;1)`, "3", 0},
		{`=CELLULE("adresse";Cell[B3])`, "$B$3", 0},
		{`=CELLULE("address";Feuille2!Cell[C4])`, "Feuille2!$C$4", 0},
		{`=CELLULE("adresse";Ventes[Prix][2])`, "Ventes[Prix][2]", 0},
		{`=CELLULE("ligne";Cell[B3])`, "3", 0},
		{`=CELLULE("col";Cell[C1])`, "3", 0},
		{`=CELLULE("contenu";Cell[B2])`, "20", 0},
		{`=CELLULE("contenu";Cell[B1:B3])`, "10", 0},
		{`=CELLULE("type";Cell[A3])`, "l", 0},
		{`=CELLULE("type";Cell[A1])`, "v", 0},
		{`=CELLULE("type";Cell[Z9])`, "b", 0},
		{`=CELLULE("couleur";Cell[A1])`, "#VALEUR!", 0},
		{`=CELLULE("adresse";DECALER(Cell[A1];2;1))`, "$B$3", 0},
		{`=AGREGAT(9;0;Ventes[Prix])`, "235,49", 1e-12},
		{`=AGREGAT(9;6;Ventes[Prix]/(Ventes[Qté]-1))`, "30,5", 1e-12},
		{`=AGREGAT(9;0;Ventes[Prix]/(Ventes[Qté]-1))`, "#DIV/0!", 0},
		{`=AGREGAT(3;6;Ventes[Prix]/(Ventes[Qté]-1))`, "3", 0},
		{`=AGREGAT(4;5;Ventes[Qté])`, "5", 0},
		{`=AGREGAT(12;0;Ventes[Qté])`, "2,5", 0},
		{`=AGREGAT(14;6;Ventes[Prix]/(Ventes[Qté]-1);1)`, "25,5", 0},
		{`=AGREGAT(16;6;Ventes[Qté];0,5)`, "2,5", 0},
		{`=AGREGAT(14;6;Ventes[Prix])`, "#VALEUR!", 0},
		{`=AGREGAT(20;0;Ventes[Qté])`, "#VALEUR!", 0},
		{`=AGREGAT(9;8;Ventes[Qté])`, "#VALEUR!", 0},
		{`=AGREGAT(9;6;Cell[A1:B3])`, "63", 0},
		{`=BDSOMME(Ventes;"Montant";Cell[E1:F3])`, "81", 1e-12},
		{`=BDNB(Ventes;"Prix";Cell[E1:F3])`, "2", 0},
		{`=BDNBVAL(Ventes;1;Cell[E1:F3])`, "3", 0},
		{`=BDMAX(Ventes;"Montant";Cell[E1:F3])`, "51", 0},
		{`=BDMIN(Ventes;"Montant";Cell[E1:F3])`, "0", 0},
		{`=BDMOYENNE(Ventes;"Montant";Cell[E1:F3])`, "27", 1e-12},
		{`=BDPRODUIT(Ventes;"Qté";Cell[E1:F3])`, "30", 0},
		{`=BDECARTYPE(Ventes;"Qté";Cell[E1:F3])`, "1,527525232", 1e-9},
		{`=BDECARTYPEP(Ventes;"Qté";Cell[E1:F3])`, "1,247219129", 1e-9},
		{`=BDVAR(Ventes;"Qté";Cell[E1:F3])`, "2,333333333", 1e-9},
		{`=BDVARP(Ventes;"Qté";Cell[E1:F3])`, "1,555555556", 1e-9},
		{`=BDLIRE(Ventes;"Prix";Cell[G1:G2])`, "10", 0},
		{`=BDLIRE(Ventes;"Prix";Cell[E1:E2])`, "#NOMBRE!", 0},
		{`=BDLIRE(Ventes;"Prix";Cell[F1:F3])`, "#NOMBRE!", 0},
		{`=BDSOMME(Ventes;"Inconnu";Cell[E1:F3])`, "#VALEUR!", 0},
		{`=BDSOMME(Ventes;"Montant";Cell[A1:B3])`, "#VALEUR!", 0},
		{`=BDSOMME(Inconnu;"Montant";Cell[E1:F3])`, "#VALEUR!", 0},
		{`=BDSOMME(Ventes;"Montant";Cell[E1])`, "#VALEUR!", 0},
		{`@1=BDSOMME(Ventes;"Montant";Cell[E1:F3])-[Montant]`, "30", 1e-12},
		{`=SOMME(TRANSPOSE({1;2;3}))`, "6", 0},
		{`=SOMME(DROITEREG(Mesures[Y];Mesures[X]))`, "-9,956790123", 1e-8},
		{`=SOMME(LOGREG(Mesures[Y];Mesures[X]))`, "2,530736", 1e-5},
		// Anciens noms et synonymes.
		{`=EQUATION.RANG(10;Ventes[Prix])`, "3", 0},
	})
}

// TRANSPOSE et DROITEREG se déversent ; les deux backends rendent le même
// tableau.
func TestTransposeLinestSpill(t *testing.T) {
	f := fixture(t)
	for src, want := range map[string]string{
		`=TRANSPOSE(Cell[A1:B2])`:                "Valeur1|Valeur2 / 1|2 / 10|20",
		`=TRANSPOSE({1.2.3})`:                    "Valeur / 1 / 2 / 3",
		`=TRANSPOSE(Ventes[Prix])`:               "#VALEUR!",
		`=DROITEREG(Cell[B1:B3];Cell[A1:A2])`:    "Valeur1|Valeur2 / #VALEUR!|#VALEUR!",
		`=DROITEREG(Mesures[Y];Mesures[X])`:      "Valeur1|Valeur2 / 0,709104938271605|-10,6658950617284",
		`=DROITEREG(Mesures[Y];Mesures[X];FAUX)`: "Valeur1|Valeur2 / 0,386394295625361|0",
		`=DROITEREG(Mesures[Y];Mesures[X];VRAI;VRAI)`: "Valeur1|Valeur2 / 0,709104938271605|-10,6658950617284 / 0,192672189121853|6,20691595722328 / " +
			"0,81867768627086|3,10196240820114 / 13,5451230921387|3 / 130,333487654321|28,866512345679",
		`=DROITEREG(Mesures[Y];Mesures[X];VRAI;Cell[A1])`: "#VALEUR!",
	} {
		n := mustParse(t, src, FR)
		goRes, sqlRes := f.spillGo(n), f.spillSQL(t, n)
		if goRes != want || sqlRes != want {
			t.Errorf("%s : Go %q, SQL %q, attendu %q", src, goRes, sqlRes, want)
		}
	}
}

// Dépendances : une cible écrite en littéraux est une dépendance ordinaire ;
// sinon la formule est volatile et calculée après les autres.
func TestDynamicDeps(t *testing.T) {
	deps := func(src string) []string {
		names, _ := Deps(mustParse(t, src, FR))
		return names
	}
	if d := deps(`=DECALER(Cell[A1];1;2)`); !slices.Contains(d, "cell_c2") {
		t.Errorf("DECALER : %v", d)
	}
	if d := deps(`=SOMME(INDIRECT("Feuille2!B1:B2"))`); !slices.Contains(d, "feuille2.cell_b2") {
		t.Errorf("INDIRECT : %v", d)
	}
	if _, tables := Deps(mustParse(t, `=SOMME(INDIRECT("Ventes[Prix]"))`, FR)); !slices.Contains(tables, "ventes") {
		t.Errorf("INDIRECT d'une colonne : %v", tables)
	}
	for src, want := range map[string]bool{
		`=INDIRECT("B3")`:                  false,
		`=DECALER(Cell[A1];1;0)`:           false,
		`=INDIRECT(Cell[A1])`:              true,
		`=DECALER(Cell[A1];Cell[B1];0)`:    true,
		`=1+SOMME(INDIRECT("A"&Cell[B1]))`: true,
	} {
		if got := IsVolatile(mustParse(t, src, FR)); got != want {
			t.Errorf("IsVolatile(%s) = %v", src, got)
		}
	}
	// A1 lit C1 par INDIRECT(B1) : calculée après C1 (et D1), quel que soit
	// l'ordre des noms.
	ns := &Names{Formulas: map[string]Node{}}
	for k, src := range map[string]string{
		"cell_a1": `=INDIRECT(Cell[B1])`,
		"cell_b1": `="C1"`,
		"cell_c1": `=Cell[D1]*2`,
		"cell_d1": `=21`,
		"cell_e1": `=Cell[A1]+1`,
	} {
		ns.Formulas[k] = mustParse(t, src, FR)
	}
	ns.Recalc(&Env{Schema: &Schema{}})
	if a, e := ns.Values["cell_a1"], ns.Values["cell_e1"]; a.String() != "42" || e.String() != "43" {
		t.Errorf("ordre des formules volatiles : A1 %v, E1 %v", a, e)
	}
	if got := ns.Affected(nil, []string{"cell_d1"}); !slices.Contains(got, "cell_a1") || !slices.Contains(got, "cell_e1") {
		t.Errorf("Affected : %v", got)
	}
}

// QualifyCells rattache la référence calculée à la feuille de la formule.
func TestDynamicHome(t *testing.T) {
	n := QualifyCells(mustParse(t, `=INDIRECT("B1")+DECALER(Cell[A1];1;0)`, FR), 7)
	env := &Env{Schema: &Schema{}, Names: map[string]Value{"cell_b1#7": Num(5), "cell_a2#7": Num(3)}}
	if v := Eval(n, env); v.String() != "8" {
		t.Errorf("feuille de la formule : %v", v)
	}
}
