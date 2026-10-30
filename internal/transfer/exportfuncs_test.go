package transfer

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
)

// Chaque fonction du catalogue est traduite à l'export ou déclarée sans
// équivalent : une fonction ajoutée ne peut pas être oubliée.
func TestExportCatalogue(t *testing.T) {
	known := map[string]bool{}
	for _, f := range formula.Functions(formula.FR) {
		en := f.Other
		known[en] = true
		_, mapped := exportFuncs[en]
		switch {
		case mapped && notExported[en]:
			t.Errorf("%s (%s) : traduite et déclarée sans équivalent", f.Name, en)
		case !mapped && !exportSpecial[en] && !notExported[en]:
			t.Errorf("%s (%s) : ni traduite à l'export (exportFuncs) ni déclarée sans équivalent (notExported)", f.Name, en)
		}
	}
	for en := range notExported {
		if !known[en] {
			t.Errorf("notExported : %s n'est pas au catalogue", en)
		}
	}
}

// Formules des fonctions récentes : enregistrées en .xlsx puis rouvertes,
// elles reviennent avec le même texte et la même valeur.
func TestExportNewFunctionsRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Qté", "Nom"}, {"1", "abc"}, {"3", "bcd"}, {"5", "x1"}, {"7", "y22"}, {"9", "zz"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameTable(ctx, "Feuille1", "Ventes"); err != nil {
		t.Fatal(err)
	}
	formulas := []struct{ src, xlsx string }{
		{`=EQUIVX(5; Ventes[Qté])`, `_xlfn.XMATCH(5,Ventes[Qté])`},
		{`=EQUIVX(6; Ventes[Qté]; 1)`, `_xlfn.XMATCH(6,Ventes[Qté],1)`},
		{`=RECHERCHEX(6; Ventes[Qté]; Ventes[Nom]; "?"; -1)`, `_xlfn.IFNA(INDEX(Ventes[Nom],_xlfn.XMATCH(6,Ventes[Qté],-1)),"?")`},
		{`=REGEXTEST(Ventes[Nom][3]; "^x\d")`, `_xlfn.REGEXTEST($B$4,"^x\d")`},
		{`=REGEXEXTRACT(Ventes[Nom][4]; "\d+")`, `_xlfn.REGEXEXTRACT($B$5,"\d+")`},
		{`=REGEXREPLACE(Ventes[Nom][1]; "b"; "B")`, `_xlfn.REGEXREPLACE($B$2,"b","B")`},
		{`=SOMME(ASSEMB.V(PRENDRE(Ventes[Qté]; 2); PRENDRE(Ventes[Qté]; -2)))`, `SUM(_xlfn.VSTACK(_xlfn.TAKE(Ventes[Qté],2),_xlfn.TAKE(Ventes[Qté],-2)))`},
		{`=NBVAL(ASSEMB.H(Ventes[Qté]; Ventes[Nom]))`, `COUNTA(_xlfn.HSTACK(Ventes[Qté],Ventes[Nom]))`},
		{`=SOMME(CHOISIRCOLS(Ventes; 1))`, `SUM(_xlfn.CHOOSECOLS(Ventes,1))`},
		{`=NBVAL(CHOISIRLIGNES(Ventes; 1; 3))`, `COUNTA(_xlfn.CHOOSEROWS(Ventes,1,3))`},
		{`=SOMME(VERS.COL(Ventes[Qté]))`, `SUM(_xlfn.TOCOL(Ventes[Qté]))`},
		{`=SOMME(PRENDRE(Ventes; 2; 1))`, `SUM(_xlfn.TAKE(Ventes,2,1))`},
		{`=NBVAL(EXCLURE(Ventes; 2; 1))`, `COUNTA(_xlfn.DROP(Ventes,2,1))`},
		{`=SOMME(TRIER(Ventes[Qté]; 1; -1))`, `SUM(_xlfn._xlws.SORT(Ventes[Qté],1,-1))`},
		{`=SOMME(TRIERPAR(Ventes[Qté]; Ventes[Nom]; -1))`, `SUM(_xlfn.SORTBY(Ventes[Qté],Ventes[Nom],-1))`},
		{`=SOMME(FILTRE(Ventes[Qté]; Ventes[Qté]>4))`, `SUM(_xlfn._xlws.FILTER(Ventes[Qté],Ventes[Qté]>4))`},
		{`=SOMME(UNIQUE(Ventes[Qté]))`, `SUM(_xlfn.UNIQUE(Ventes[Qté]))`},
		{`=SOMME(SEQUENCE(4))`, `SUM(_xlfn.SEQUENCE(4))`},
		{`=NBVAL(FRACTIONNER.TEXTE("a,b,c"; ","))`, `COUNTA(_xlfn.TEXTSPLIT("a,b,c",","))`},
		{`=PLAFOND.MATH(2,3)`, `_xlfn.CEILING.MATH(2.3)`},
		{`=PLANCHER.MATH(-2,3; 1; 1)`, `_xlfn.FLOOR.MATH(-2.3,1,1)`},
		{`=PAIR(3)`, `EVEN(3)`},
		{`=IMPAIR(4)`, `ODD(4)`},
		{`=FACT(5)`, `FACT(5)`},
		{`=COMBIN(5; 2)`, `COMBIN(5,2)`},
		{`=PERMUTATION(5; 2)`, `PERMUT(5,2)`},
		{`=ATAN2(1; 1)`, `ATAN2(1,1)`},
		{`=ESTERR(1/0)`, `ISERR(1/0)`},
		{`=JOURS360(DATE(2026; 1; 31); DATE(2026; 3; 1))`, `DAYS360(DATE(2026,1,31),DATE(2026,3,1))`},
		{`=ECARTYPE.STANDARD(Ventes[Qté])`, `_xlfn.STDEV.S(Ventes[Qté])`},
	}
	for i, f := range formulas {
		if _, err := d.SetCell(ctx, 1, i, 7, f.src); err != nil {
			t.Fatal(f.src, err)
		}
	}
	before, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "fonctions.xlsx")
	if _, err := ExportXLSXFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	cells := dumpXLSX(t, p).sheet("Feuille1").cells
	for i, f := range formulas {
		if f.xlsx == "" {
			continue
		}
		ref := "H" + strconv.Itoa(i+1)
		if got := cells[ref].f; got != f.xlsx {
			t.Errorf("%s %s : écrite %q, attendu %q", ref, f.src, got, f.xlsx)
		}
	}
	d2 := open(t)
	res, err := ImportXLSX(ctx, d2.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) > 0 {
		t.Errorf("avertissements : %v", res.Warnings)
	}
	after, err := d2.Layout(ctx, res.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	type cv struct{ raw, value string }
	got := map[[2]int]cv{}
	for _, c := range after.Cells {
		got[[2]int{c.Row, c.Col}] = cv{c.Raw, c.Value}
	}
	n := 0
	for _, c := range before.Cells {
		if c.Col != 7 {
			continue
		}
		n++
		if c.Value == "" || c.Value[0] == '#' {
			t.Errorf("%s : valeur %q avant l'export", c.Raw, c.Value)
		}
		if g := got[[2]int{c.Row, c.Col}]; g.raw != c.Raw || g.value != c.Value {
			t.Errorf("%s = %q : relue %s = %q", c.Raw, c.Value, g.raw, g.value)
		}
	}
	if n != len(formulas) {
		t.Errorf("%d formules, attendu %d", n, len(formulas))
	}
}

// VERS.LIGNE ne se calcule qu'en se déversant (exportée en valeurs) :
// traduction seule. En .ods, les fonctions récentes gardent le nom que
// LibreOffice lit ; les tableaux dynamiques, recherches et expressions
// régulières récentes ne sont pas traduits (valeur gardée).
func TestExportNewFunctionsNames(t *testing.T) {
	for _, tg := range []target{toXLSX, toODS} {
		b := &book{target: tg, sheets: map[string]string{"notes": "Notes"}, tables: map[string]*tablePlace{}, names: map[string]cellPlace{}}
		for _, c := range []struct{ src, xlsx, ods string }{
			{`=VERS.LIGNE(Cell[A1:B2]; 1; VRAI)`, `_xlfn.TOROW(A1:B2,1,TRUE)`, ``},
			{`=PLAFOND.MATH(2,3)`, `_xlfn.CEILING.MATH(2.3)`, `COM.MICROSOFT.CEILING.MATH(2.3)`},
			{`=PLANCHER.MATH(2,3; 2)`, `_xlfn.FLOOR.MATH(2.3,2)`, `COM.MICROSOFT.FLOOR.MATH(2.3;2)`},
			{`=ESTERR(1/0)`, `ISERR(1/0)`, `ISERR(1/0)`},
			{`=PERMUTATION(5; 2)`, `PERMUT(5,2)`, `PERMUT(5;2)`},
			{`=REGEXTEST("a"; "a")`, `_xlfn.REGEXTEST("a","a")`, ``},
			{`=SOMME(SEQUENCE(3))`, `SUM(_xlfn.SEQUENCE(3))`, ``},
		} {
			want := c.xlsx
			if tg == toODS {
				want = c.ods
			}
			got, ok := b.translate(c.src, at{sheet: "Notes"})
			if want == "" {
				if ok {
					t.Errorf("%d %s : traduite en %q", tg, c.src, got)
				}
				continue
			}
			if !ok || got != want {
				t.Errorf("%d %s : %q (%v), attendu %q", tg, c.src, got, ok, want)
			}
		}
	}
}
