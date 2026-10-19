package formula

import (
	"strings"
	"testing"
)

// Anciens tableaux Janvier et Février devenus des parties de Ventes (Mois).
func filteredMonths() map[string]Filtered {
	out := map[string]Filtered{}
	for _, m := range []string{"Janvier", "Février"} {
		v, c := CategoryNodes(Str(m))
		out[strings.ToLower(m)] = Filtered{Table: "Ventes", Category: "Mois", Value: v, Crit: c,
			Columns: map[string]string{"montant": "Montant", "montant €": "Montant", "client": "Client", "qté": "Qté"}}
	}
	return out
}

func TestFilterTables(t *testing.T) {
	cases := []struct{ src, want string }{
		{`=SOMME(Janvier[Montant])`, `=SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier")`},
		{`=MOYENNE(Janvier[Montant €])`, `=MOYENNE.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier")`},
		{`=MAX(Février[Montant])-MIN(Février[Montant])`, `=MAX.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Février")-MIN.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Février")`},
		{`=NBVAL(Janvier[Client])`, `=NB.SI.ENS(Ventes[Client]; "<>"; Ventes[Mois]; "=Janvier")`},
		{`=NB(Janvier[Montant])`, `=NB(FILTRE(Ventes[Montant]; Ventes[Mois]="Janvier"))`},
		{`=SOMME(Janvier[Montant]*Janvier[Qté])`, `=SOMME.SI.ENS(Ventes[Montant]*Ventes[Qté]; Ventes[Mois]; "=Janvier")`},
		{`=SOMME.SI(Janvier[Client]; "A*"; Janvier[Montant])`, `=SOMME.SI.ENS(Ventes[Montant]; Ventes[Client]; "A*"; Ventes[Mois]; "=Janvier")`},
		{`=SOMME.SI(Janvier[Montant]; ">2")`, `=SOMME.SI.ENS(Ventes[Montant]; Ventes[Montant]; ">2"; Ventes[Mois]; "=Janvier")`},
		{`=NB.SI(Janvier[Client]; A1)`, `=NB.SI.ENS(Ventes[Client]; A1; Ventes[Mois]; "=Janvier")`},
		{`=NB.SI.ENS(Février[Client]; "x"; Février[Qté]; ">1")`, `=NB.SI.ENS(Ventes[Client]; "x"; Ventes[Qté]; ">1"; Ventes[Mois]; "=Février")`},
		{`=MAX.SI.ENS(Janvier[Montant]; Janvier[Client]; "x")`, `=MAX.SI.ENS(Ventes[Montant]; Ventes[Client]; "x"; Ventes[Mois]; "=Janvier")`},
		{`=LIGNES(Janvier)`, `=NB.SI.ENS(Ventes[Mois]; "=Janvier")`},
		{`=SOMME(Janvier[Montant]; Février[Montant]; 3)`, `=SOMME(SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier"); SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Février"); 3)`},
		{`=MOYENNE(Janvier[Montant]; Février[Montant])`, `=MOYENNE(FILTRE(Ventes[Montant]; Ventes[Mois]="Janvier"); FILTRE(Ventes[Montant]; Ventes[Mois]="Février"))`},
		{`=MEDIANE(Janvier[Montant])`, `=MEDIANE(FILTRE(Ventes[Montant]; Ventes[Mois]="Janvier"))`},
		{`=JOINDRE.TEXTE("; "; VRAI; Janvier[Client])`, `=JOINDRE.TEXTE("; "; VRAI; FILTRE(Ventes[Client]; Ventes[Mois]="Janvier"))`},
		{`=NBVAL(UNIQUE(Janvier[Client]))`, `=NBVAL(UNIQUE(FILTRE(Ventes[Client]; Ventes[Mois]="Janvier")))`},
		{`=SOMME(FILTRE(Janvier[Montant]; Janvier[Qté]>MOYENNE(Janvier[Qté])))`, `=SOMME(FILTRE(FILTRE(Ventes[Montant]; Ventes[Mois]="Janvier"); Ventes[Qté]>MOYENNE.SI.ENS(Ventes[Qté]; Ventes[Mois]; "=Janvier")))`},
		{`=SI(SOMME(Janvier[Montant])>0; "ok"; "")`, `=SI(SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier")>0; "ok"; "")`},
		// Un critère qui cite un ancien tableau, sur un autre tableau.
		{`=NB.SI(Autre[X]; ">"&MOYENNE(Janvier[Montant]))`, `=NB.SI(Autre[X]; ">"&MOYENNE.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier"))`},
	}
	for _, c := range cases {
		n := mustParse(t, c.src, FR)
		out, cites, why := FilterTables(n, filteredMonths())
		if !cites || why != "" {
			t.Errorf("%s : cite %v, %s", c.src, cites, why)
			continue
		}
		if got := Format(out, FR); got != c.want {
			t.Errorf("%s\n donne   %s\n attendu %s", c.src, got, c.want)
		}
		// La formule rendue se relit telle quelle.
		if back := Format(mustParse(t, Format(out, FR), FR), FR); back != c.want {
			t.Errorf("%s : relue %s", c.src, back)
		}
	}
	// À reprendre : la formule rendue est celle d'origine, avec la raison.
	for _, src := range []string{
		`=Janvier[Montant][2]`,
		`=SOMME(Janvier[Montant][1:3])`,
		`=REFERENCE(Janvier[Montant]; "F-001")`,
		`=EXISTE(Janvier; "F-001")`,
		`=FILTRE(Janvier; Janvier[Qté]>1)`,
		`=NBVAL(Janvier)`,
		`=SOUS.TOTAL(9; Janvier[Montant])`,
		`=EQUIV(3; Janvier[Montant]; 0)`,
		`=GRANDE.VALEUR(Janvier[Montant]; 2)`,
		`=Janvier[Montant]*2`,
		`=SOMME(Janvier[Montant]*Février[Qté])`,
		`=NB.SI(Janvier[Client]; "x")+SOMME(Janvier[Prix])`,
		`=SOMME(Janvier[Montant]; Janvier[Montant]*2)`,
		`=FILTRE(Janvier[Montant]; [Qté]>1)`,
	} {
		n := mustParse(t, src, FR)
		out, cites, why := FilterTables(n, filteredMonths())
		if !cites || why == "" || Format(out, FR) != Format(n, FR) {
			t.Errorf("%s : à reprendre attendu (cite %v, %q, %s)", src, cites, why, Format(out, FR))
		}
	}
	// Ce qui ne cite pas les anciens tableaux ne change pas.
	for _, src := range []string{`=SOMME(Ventes[Montant])`, `=[Prix]*2`, `=Cell[A1]+1`, `=LIGNES(Autre)`} {
		if _, cites, why := FilterTables(mustParse(t, src, FR), filteredMonths()); cites || why != "" {
			t.Errorf("%s : cite %v %s", src, cites, why)
		}
	}
}

// Les colonnes calculées s'écrivent en anglais.
func TestFilterTablesEnglish(t *testing.T) {
	n := mustParse(t, `=SUM(Janvier[Montant])-[Cible]`, EN)
	out, cites, why := FilterTables(n, filteredMonths())
	if want := `=SUMIFS(Ventes[Montant], Ventes[Mois], "=Janvier")-[Cible]`; !cites || why != "" || Format(out, EN) != want {
		t.Fatalf("%s (%v %s)", Format(out, EN), cites, why)
	}
}

// Le critère de catégorie ne retient que sa valeur (sans tenir compte de la
// casse, comme les .SI.ENS) : jokers échappés ; pas de critère pour un texte
// qui se lirait comme un nombre, un booléen ou une date.
func TestCategoryNodes(t *testing.T) {
	for _, s := range []string{"Janvier", "Lot *", "A?B", "~x", "<>x", "=x", "Client A"} {
		v, c := CategoryNodes(Str(s))
		if v != (StrLit{V: s}) || c == nil {
			t.Errorf("%q : %v %v", s, v, c)
			continue
		}
		cr := parseCriterion(Str(c.(StrLit).V))
		if !cr.match(Str(s)) || !cr.match(Str(strings.ToUpper(s))) || cr.match(Str(s+"x")) || cr.match(Str("x"+s)) || cr.match(Value{}) {
			t.Errorf("%q : le critère %q ne retient pas exactement la valeur", s, c.(StrLit).V)
		}
	}
	for _, s := range []string{"2025", "VRAI", "01/02/2025", "#N/A"} {
		if _, c := CategoryNodes(Str(s)); c != nil {
			t.Errorf("%q : critère %v", s, c)
		}
	}
	if v, c := CategoryNodes(Num(2025)); v != (NumLit{V: 2025}) || c != (NumLit{V: 2025}) {
		t.Errorf("année : %v %v", v, c)
	}
	d, _ := ParseDate("2025-03-01")
	v, c := CategoryNodes(DateSerial(d))
	if Format(v, FR) != "=DATE(2025; 3; 1)" || Format(c, FR) != "=DATE(2025; 3; 1)" {
		t.Errorf("date : %s %s", Format(v, FR), Format(c, FR))
	}
}

// NBVAL d'une colonne et NB.SI.ENS(colonne; "<>") comptent les mêmes cases
// (textes, nombres, booléens, dates, textes numériques ; pas les vides), sur
// les deux backends : la reprise de NBVAL est exacte.
func TestCountAEqualsCountIfsNonBlank(t *testing.T) {
	f := fixture(t)
	for _, col := range []string{"Produit", "Prix", "Qté", "Payé", "Client", "Note", "Date", "Montant", "NomClient"} {
		a := mustParse(t, "=NBVAL(Ventes["+col+"])", FR)
		b := mustParse(t, `=NB.SI.ENS(Ventes[`+col+`]; "<>")`, FR)
		ga, gb := f.evalGo(a, 0).String(), f.evalGo(b, 0).String()
		sa, sb := f.evalSQL(t, a, 0).String(), f.evalSQL(t, b, 0).String()
		if ga != gb || sa != sb || ga != sa {
			t.Errorf("%s : NBVAL %s / %s, NB.SI.ENS %s / %s", col, ga, sa, gb, sb)
		}
	}
	// Une condition de catégorie sur le texte : les deux formes s'accordent.
	for _, p := range []string{"Clavier", "écran"} {
		a := mustParse(t, `=SOMME.SI.ENS(Ventes[Qté]; Ventes[Produit]; "=`+p+`")`, FR)
		b := mustParse(t, `=SOMME(FILTRE(Ventes[Qté]; Ventes[Produit]="`+p+`"))`, FR)
		if ga, gb, sa, sb := f.evalGo(a, 0).String(), f.evalGo(b, 0).String(), f.evalSQL(t, a, 0).String(), f.evalSQL(t, b, 0).String(); ga != gb || sa != sb || ga != sa {
			t.Errorf("%s : %s %s %s %s", p, ga, gb, sa, sb)
		}
	}
}
