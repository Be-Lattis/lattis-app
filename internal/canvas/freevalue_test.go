package canvas

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/formula"
)

// Une liste de nombres saisie l'un sous l'autre devient un tableau sans
// en-tête : 10 reste une donnée (audit n° 4).
func TestBirthWithoutHeader(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	for i, v := range []string{"10", "20", "30"} {
		if _, err := d.SetCell(ctx, 1, 3+i, 1, v); err != nil {
			t.Fatal(err)
		}
	}
	l, _ := d.Layout(ctx, 1)
	if len(l.Tables) != 1 {
		t.Fatalf("tableaux : %+v", l.Tables)
	}
	tb := l.Tables[0]
	if tb.Row != 2 || tb.Col != 1 || tb.Rows != 3 || tb.Columns[0].Name != "Colonne 1" || tb.Columns[0].Type != "integer" {
		t.Fatalf("tableau : %+v", tb)
	}
	if _, err := d.SetCell(ctx, 1, 0, 5, "=SOMME("+tb.Name+"[Colonne 1])"); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, 1)
	if l.Cells[0].Value != "60" {
		t.Fatalf("somme : %+v", l.Cells)
	}
	// Un en-tête texte au-dessus de nombres reste un en-tête.
	for i, v := range []string{"Montant", "5", "7"} {
		if _, err := d.SetCell(ctx, 1, 10+i, 1, v); err != nil {
			t.Fatal(err)
		}
	}
	l, _ = d.Layout(ctx, 1)
	if tb := l.Tables[1]; tb.Row != 10 || tb.Rows != 2 || tb.Columns[0].Name != "Montant" {
		t.Fatalf("avec en-tête : %+v", tb)
	}
	// Tout en haut, sans place pour l'en-tête : les cellules restent libres.
	for i, v := range []string{"1", "2"} {
		if _, err := d.SetCell(ctx, 1, i, 8, v); err != nil {
			t.Fatal(err)
		}
	}
	l, _ = d.Layout(ctx, 1)
	if len(l.Tables) != 2 {
		t.Fatalf("liste en ligne 1 : %+v", l.Tables)
	}
	check(t, d, "naissance sans en-tête")
}

func TestTypedHeader(t *testing.T) {
	for _, tt := range []struct {
		rows [][]string
		want bool
	}{
		{[][]string{{"10"}, {"20"}}, false},
		{[][]string{{"16/10/2026"}, {"17/10/2026"}}, false},
		{[][]string{{"VRAI"}, {"FAUX"}}, false},
		{[][]string{{"1"}, {"2,5"}}, false},
		{[][]string{{"Nom"}, {"Alice"}}, true},
		{[][]string{{"Prix"}, {"3"}}, true},
		{[][]string{{"10", "Ville"}, {"20", "Lyon"}}, true},
		// Une première ligne sans texte est une donnée, même au-dessus de
		// valeurs d'autres types (liste mixte : date, heure, booléen, %…).
		{[][]string{{"10"}, {"abc"}}, false},
		{[][]string{{"18/10/2026"}, {"10:30"}, {"VRAI"}, {"15 %"}, {"42"}}, false},
		{[][]string{{"10:30"}, {"abc"}}, false},
		{[][]string{{"15 %"}, {"abc"}}, false},
		{[][]string{{"VRAI"}, {"12"}}, false},
		{[][]string{{"2024", "2025"}, {"3", "4"}}, false},
	} {
		if got := typedHeader(tt.rows); got != tt.want {
			t.Errorf("typedHeader(%v) = %v", tt.rows, got)
		}
	}
}

// En-tête d'un bloc collé : texte au-dessus de valeurs typées, ou, tout en
// texte, vote des colonnes sur la longueur des valeurs.
func TestHasHeader(t *testing.T) {
	for _, tt := range []struct {
		rows [][]string
		want bool
	}{
		{[][]string{{"Prix"}, {"3"}, {"4"}}, true},
		{[][]string{{"Code", "Quand"}, {"12", "01/10/2026"}, {"x7", "hier"}, {"40", "05/09/2026"}}, true},
		{[][]string{{"Code", "Nom"}, {"A", "x"}, {"B", "y"}, {"A", "x"}}, true},
		{[][]string{{"Nom", "Ville"}, {"Dupont;007", "Lyon"}, {"Martin;12", "Rome"}}, true},
		{[][]string{{"Ville", "Pays"}, {"Lyon", "FR"}, {"Rome", "IT"}}, true},
		// Données : longueurs égales à la première, valeurs variées, première
		// valeur répétée, en-têtes en double, deux lignes seulement.
		{[][]string{{"Jean"}, {"Paul"}, {"Anne"}}, false},
		{[][]string{{"Paris"}, {"Lyon"}, {"Nantes"}}, false},
		{[][]string{{"Lyon"}, {"Rome"}, {"Lyon"}}, false},
		{[][]string{{"Nom", "Nom"}, {"a", "b"}, {"c", "d"}}, false},
		{[][]string{{"Produit", "produit"}, {"1", "2"}, {"3", "4"}}, true},
		{[][]string{{"Nom"}, {"Dupont"}}, false},
		{[][]string{{"10:30"}, {"abc"}, {"def"}}, false},
	} {
		if got := hasHeader(tt.rows, len(tt.rows[0])); got != tt.want {
			t.Errorf("hasHeader(%v) = %v", tt.rows, got)
		}
	}
}

// Les saisies « à la Excel » des cellules libres sont reconnues (audit n° 7).
func TestFreeCellValues(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	year := strconv.Itoa(time.Now().Year())
	inputs := []struct{ raw, stored, formula, value string }{
		{"16/10/2026", "16/10/2026", "=@+1", "17/10/2026"},
		{"16/10", "16/10/" + year, "=JOUR(@)", "16"},
		{"2026-10-16", "16/10/2026", "=MOIS(@)", "10"},
		{"12:30", "12:30", "=@*24", "12,5"},
		{"vrai", "VRAI", "=SI(@;1;2)", "1"},
		{"12 €", "12 €", "=@+1", "13"},
		{"(12)", "-12", "=@*2", "-24"},
		{"1e3", "1000", "=@+1", "1001"},
		{"'007", "'007", "=@&\"!\"", "007!"},
		{"15 %", "15 %", "=@*100", "15"},
	}
	// Une colonne vide entre deux saisies : pas d'îlot, pas de tableau.
	for i, in := range inputs {
		if _, err := d.SetCell(ctx, 1, i*2, 0, in.raw); err != nil {
			t.Fatal(err)
		}
	}
	for i, in := range inputs {
		f := strings.ReplaceAll(in.formula, "@", formula.CellText(formula.CellName(i*2, 0)))
		if _, err := d.SetCell(ctx, 1, i*2, 2, f); err != nil {
			t.Fatal(err)
		}
	}
	l, _ := d.Layout(ctx, 1)
	if len(l.Tables) != 0 {
		t.Fatalf("tableaux : %+v", l.Tables)
	}
	got := map[[2]int]CellInfo{}
	for _, c := range l.Cells {
		got[[2]int{c.Row, c.Col}] = c
	}
	for i, in := range inputs {
		if c := got[[2]int{i * 2, 0}]; c.Raw != in.stored {
			t.Errorf("%q enregistré %q, attendu %q", in.raw, c.Raw, in.stored)
		}
		if c := got[[2]int{i * 2, 2}]; c.Value != in.value {
			t.Errorf("%q : %s vaut %q, attendu %q", in.raw, c.Raw, c.Value, in.value)
		}
	}
}

// Un TCD reprend le format de la colonne source, ou deux décimales pour une
// moyenne (audit n° 15).
func TestPivotFormats(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes", "Prix", "Qté"}, {"FR", "10", "1,5", "1"}, {"IT", "5", "2,25", "2"}, {"FR", "7", "3", "4"}}); err != nil {
		t.Fatal(err)
	}
	// Ventes au format monétaire (colonne entière).
	if err := d.ApplyStyle(ctx, 1, 1, 1, 3, 1, Style{"nf": "€"}); err != nil {
		t.Fatal(err)
	}
	sheet, err := d.CreatePivot(ctx, PivotConfig{Source: "Feuille1", RowFields: []string{"Pays"}, Values: []PivotValue{
		{Field: "Ventes", Agg: "sum"}, {Field: "Qté", Agg: "avg"}, {Field: "Prix", Agg: "max"}, {Field: "Qté", Agg: "sum"}, {Agg: "count"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, sheet)
	if got := fmt.Sprintf("%q", l.Pivots[0].Formats); got != `["" "{\"nf\":\"€\"}" "{\"dp\":2}" "{\"dp\":2}" "" ""]` {
		t.Fatalf("formats : %s", got)
	}
}

// Une colonne calculée décimale est signalée comme telle (audit n° 23).
func TestComputedResult(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Prix", "Qté", "TTC", "Double", "Texte", "Moy"}, {"1,2", "1", "", "", "", ""}, {"2,5", "2", "", "", "", ""}}); err != nil {
		t.Fatal(err)
	}
	for col, f := range map[int]string{2: "=[Prix]*1,2", 3: "=[Qté]*2", 4: `=[Qté]&"x"`, 5: "=MOYENNE(Feuille1[Qté])"} {
		if _, err := d.SetCell(ctx, 1, 1, col, f); err != nil {
			t.Fatal(err)
		}
	}
	l, _ := d.Layout(ctx, 1)
	var got []string
	for _, c := range l.Tables[0].Columns {
		got = append(got, string(c.Result))
	}
	if fmt.Sprintf("%q", got) != `["" "" "decimal" "integer" "" "decimal"]` {
		t.Fatalf("types des résultats : %q", got)
	}
}
