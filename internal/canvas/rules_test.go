package canvas

import (
	"context"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
)

func TestShiftA1(t *testing.T) {
	for _, c := range []struct {
		src    string
		dr, dc int
		want   string
	}{
		{"=A1>10", 2, 1, "=B3>10"},
		{"=$A1>$B$2", 3, 2, "=$A4>$B$2"},
		{`=ET(A1<>"A1";Taux>B1)`, 1, 0, `=ET(A2<>"A1";Taux>B2)`},
		{"=SOMME(A1:A3)>LOG10(B2)", 1, 0, "=SOMME(A2:A4)>LOG10(B3)"},
		{"=Ventes[Montant]>A1", 0, 0, "=Ventes[Montant]>A1"},
		{"=A1>0", -1, 0, "=#REF!>0"},
		{"=Feuil2!C3=1", 1, 1, "=Feuil2!D4=1"},
	} {
		if got := shiftA1(c.src, c.dr, c.dc); got != c.want {
			t.Errorf("shiftA1(%q, %d, %d) = %q, attendu %q", c.src, c.dr, c.dc, got, c.want)
		}
	}
}

func TestRowRefs(t *testing.T) {
	tb := Table{Row: 0, Col: 1, Cols: []Column{{Name: "Ville"}, {Name: "Pop"}}}
	if got := rowRefs("=$C2>1000", tb); got != "=[@Pop]>1000" {
		t.Fatalf("rowRefs = %q", got)
	}
	if got := rowRefs("=C$2>A2", tb); got != "=C$2>A2" {
		t.Fatalf("rowRefs (ligne absolue, hors tableau) = %q", got)
	}
}

// Règles de toute une feuille en une étape : colonne de tableau (formule
// calculée par ligne), cellules libres (dans leur style), colonnes entières.
func TestSheetRules(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}, {"Paris", "2100"}}); err != nil {
		t.Fatal(err)
	}
	for r, v := range []string{"5", "50", "500"} {
		if _, err := d.SetCell(ctx, 1, r, 5, v); err != nil {
			t.Fatal(err)
		}
	}
	before := dump(t, d)
	red := Style{"bg": "#ffc7ce"}
	ch := RuleChanges{
		Columns: []ColumnRules{{Table: "Feuille1", Col: 1, Rules: []Rule{{ID: "f1", Op: "formula", Value: "=$B2>1000", Style: red}}}},
		Cells:   []RangeRules{{R0: 0, C0: 5, R1: 2, C1: 5, Rules: []Rule{{ID: "g1", Op: "formula", Value: "=F1>=50", Style: red}, {ID: "b1", Op: "bar", Bar: map[string]any{"color": "#638ec6"}}}}},
		Lines:   []LineRuleSet{{Rows: false, I0: 8, I1: 9, Rules: []Rule{{ID: "t1", Op: "top", Value: "3", Style: red}}}},
	}
	if err := d.SetSheetRules(ctx, 1, ch); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := l.Tables[0].Columns[1].Rules; !strings.Contains(got, `"f":"=[Pop]>1000"`) {
		t.Fatalf("formule de colonne non analysée : %s", got)
	}
	// Calcul par ligne, en SQL, après les colonnes (lecture par fenêtres).
	_, args, _, err := d.ColumnExprs(ctx, l.Tables[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	exprs, margs, cols, idx, err := d.RuleExprs(ctx, l.Tables[0].ID, len(args))
	if err != nil || len(exprs) != 1 || cols[0] != 1 || idx[0] != 0 {
		t.Fatalf("RuleExprs : %v %v %v %v", exprs, cols, idx, err)
	}
	var got []bool
	rs, err := d.db.Read().QueryContext(ctx, `SELECT `+exprs[0]+` FROM `+l.Tables[0].SQL+` ORDER BY _pos`, append(args, margs...)...)
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var v any
		rs.Scan(&v)
		got = append(got, Truthy(formula.FromResult(v)))
	}
	rs.Close()
	if len(got) != 2 || got[0] || !got[1] {
		t.Fatalf("formule par ligne : %v", got)
	}
	cf := 0
	for _, s := range l.Styles {
		if strings.Contains(s.Style, `"cf"`) {
			cf++
		}
	}
	if cf != 3 || len(l.ColRules) != 2 {
		t.Fatalf("règles de cellules : %d, de colonnes : %+v", cf, l.ColRules)
	}
	// La formule suit chaque cellule : F1>=50 faux, F2>=50 et F3>=50 vrais.
	marks := map[int]bool{}
	for _, m := range l.RuleMarks {
		if m.Rule == "g1" {
			marks[m.Row] = true
		}
	}
	if len(marks) != 2 || !marks[1] || !marks[2] {
		t.Fatalf("formule des cellules libres : %+v", l.RuleMarks)
	}
	check(t, d, "règles de la feuille")
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation en une étape :\n%s", diff(before, got))
	}
	// Règles refusées.
	for _, bad := range []Rule{{Op: "top", Value: "0"}, {Op: "top", Value: "150", Pct: true}, {Op: "date", Value: "hier"}, {Op: "formula", Value: "=("}, {Op: "scale"}} {
		if err := d.SetSheetRules(ctx, 1, RuleChanges{Cells: []RangeRules{{Rules: []Rule{bad}}}}); err == nil {
			t.Errorf("règle acceptée : %+v", bad)
		}
	}
}

// Les anciennes règles (sans identifiant ni réglages nouveaux) se relisent.
func TestRulesCompat(t *testing.T) {
	rs := parseRuleList(`[{"op":">","value":"1000","style":{"bg":"#ffc7ce","fg":"#9c0006"}},{"op":"empty","style":{}}]`)
	if len(rs) != 2 || rs[0].Op != ">" || rs[0].Style["bg"] != "#ffc7ce" {
		t.Fatalf("anciennes règles : %+v", rs)
	}
	if err := checkRules(rs); err != nil {
		t.Fatal(err)
	}
}
