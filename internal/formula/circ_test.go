package formula

import (
	"fmt"
	"strings"
	"testing"
)

func parseOK(t *testing.T, s string, loc Locale) Node {
	t.Helper()
	n, err := Parse(s, loc)
	if err != nil {
		t.Fatalf("%s : %v", s, err)
	}
	return n
}

// Audit n° 3, C1 : la compilation d'une colonne qui agrège sa propre
// colonne s'arrête sur #CIRC!, sous toutes ses formes, au lieu de déborder
// la pile (le sous-compilateur des agrégats précalculés repartait sans les
// colonnes en cours).
func TestCompileSelfAggregate(t *testing.T) {
	circ := fmt.Sprintf("slt_err(%d)", ErrCirc)
	for _, f := range []string{"=SUM(T[X])", "=SUM([X])", "=AVERAGE(T[X])+1", "=COUNTIF(T[X], 1)", "=[Y]*2", "=IFERROR(SUM(T[Y]), 0)"} {
		tb := &Table{Name: "T", SQL: "t_1", Cols: []Column{
			{Name: "X", SQL: "c_1", Formula: parseOK(t, f, EN)},
			{Name: "Y", SQL: "c_2", Formula: parseOK(t, "=[X]+1", EN)},
		}}
		s := &Schema{Tables: []*Table{tb}}
		hoists := 0
		hoist := func(Compiled) Value { hoists++; return Num(0) }
		for i := range tb.Cols {
			if c := CompileColumnFrom(tb, i, s, nil, "t_1", hoist, 0); c.SQL != circ {
				t.Errorf("%s, colonne %s : %s", f, tb.Cols[i].Name, c.SQL)
			}
		}
		// Lue depuis une cellule libre : la même erreur.
		if c := CompileHoisted(parseOK(t, "=SUM(T[X])", EN), s, nil, nil, "", hoist); c.SQL != circ {
			t.Errorf("%s, lu ailleurs : %s", f, c.SQL)
		}
		if hoists > 50 {
			t.Errorf("%s : %d précalculs", f, hoists)
		}
	}
}

// Garde de dernier recours : une formule trop imbriquée donne #CIRC!, sans
// faire déborder la pile.
func TestCompileDepthGuard(t *testing.T) {
	deep := "=" + strings.Repeat("ABS(", 1500) + "1" + strings.Repeat(")", 1500)
	if c := Compile(parseOK(t, deep, EN), &Schema{}, nil, nil, ""); !strings.Contains(c.SQL, fmt.Sprintf("slt_err(%d)", ErrCirc)) {
		t.Fatalf("formule trop imbriquée : %.80s", c.SQL)
	}
	long := "=" + strings.Repeat("1+", 300) + "1"
	if c := Compile(parseOK(t, long, EN), &Schema{}, nil, nil, ""); strings.Contains(c.SQL, "slt_err") {
		t.Fatalf("une longue somme n'est pas circulaire : %.80s", c.SQL)
	}
}

// Une colonne qui lit ses propres lignes voisines est reconnue (même sens),
// pas celle qui lit les deux sens ni sa propre ligne.
func TestSelfOffsets(t *testing.T) {
	for f, want := range map[string][2]int{
		"=[Qty]+[C][-1]":           {1, -1},
		"=[C][-1]+[C][-3]":         {3, -1},
		"=[Qty]+PREVIOUS([C], 0)":  {1, -1},
		"=IF([Qty]>0, [C][+2], 0)": {2, 1},
		"=[C][-1]+[C][+1]":         {0, 0},
		"=[C]+1":                   {0, 0},
		"=[Qty]+[Other][-1]":       {0, 0},
		"=SUM(T[C])+T[C][-1]":      {1, -1},
	} {
		tb := &Table{Name: "T", Cols: []Column{{Name: "Qty"}, {Name: "Other"}, {Name: "C"}}}
		tb.Cols[2].Formula = parseOK(t, f, EN)
		k, dir, ok := selfOffsets(tb.Cols[2].Formula, tb, &tb.Cols[2])
		if !ok {
			k, dir = 0, 0
		}
		if [2]int{k, dir} != want {
			t.Errorf("%s : %d, %d ; attendu %v", f, k, dir, want)
		}
	}
}

// Une adresse de cellule absente des valeurs est une cellule vide, sauf sur
// une feuille inconnue.
func TestEmptyCellKeys(t *testing.T) {
	s := &Schema{Sheets: map[string]bool{"bilan": true}}
	for key, want := range map[string]bool{
		"cell_e15": true, "cell_e15#3": true, "bilan.cell_c3": true,
		"inconnue.cell_c3": false, "taux": false, "cell_": false,
	} {
		if got := s.emptyCell(key); got != want {
			t.Errorf("%s : %v", key, got)
		}
	}
}

// Copie de feuille : les formules visent les copies des tableaux.
func TestRetargetTables(t *testing.T) {
	m := map[string]string{"ventes": "Ventes_2"}
	for in, want := range map[string]string{
		"=SOMME(Ventes[Total])":             "=SOMME(Ventes_2[Total])",
		"=ventes[Prix][2]+Stock[Qté]":       "=Ventes_2[Prix][2]+Stock[Qté]",
		"=REFERENCE(Ventes; Prix; \"Thé\")": "=REFERENCE(Ventes_2[Prix]; \"Thé\")",
		"=NB.SI(Ventes[Produit]; Cell[A1])": "=NB.SI(Ventes_2[Produit]; Cell[A1])",
		"=Ventes_3[Prix]*2":                 "",
	} {
		n, changed := RetargetTables(parseOK(t, in, FR), m)
		got := ""
		if changed {
			got = Format(n, FR)
		}
		if got != want {
			t.Errorf("%s → %q, attendu %q", in, got, want)
		}
	}
}
