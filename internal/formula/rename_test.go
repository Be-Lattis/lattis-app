package formula

import "testing"

func TestRenameRefs(t *testing.T) {
	merge := Renames{
		Tables:  map[string]string{"tableau1": "Ventes"},
		Columns: map[string]map[string]string{"tableau1": {"prix": "Prix 2"}},
	}
	rename := Renames{
		Tables:  map[string]string{"ventes": "Achats"},
		Columns: map[string]map[string]string{"ventes": {"qté": "Quantité"}},
		Sheets:  map[string]string{"feuille1": "Mes données"},
		Names:   map[string]string{"taux": "Coef"},
	}
	cases := []struct {
		r        Renames
		loc      Locale
		self     string
		in, want string
	}{
		{merge, FR, "", "=SOMME(Tableau1[Pop])", "=SOMME(Ventes[Pop])"},
		{merge, FR, "", "=Tableau1[Pop][2]*2", "=Ventes[Pop][2]*2"},
		{merge, FR, "", "=tableau1[Prix] + Tableau1[@Prix]", "=Ventes[Prix 2] + Ventes[@Prix 2]"},
		{merge, FR, "", "=Tableau1[Prix][1:3]", "=Ventes[Prix 2][1:3]"},
		// Colonne calculée de Tableau1 : [Prix] est la sienne.
		{merge, EN, "Tableau1", "=[Prix]*2", "=[Prix 2]*2"},
		{merge, EN, "Autre", "=[Prix]*2", "=[Prix]*2"},
		{merge, FR, "", `=REFERENCE(Tableau1[Prix]; "Thé")`, `=REFERENCE(Ventes[Prix 2]; "Thé")`},
		{merge, FR, "", `=REFERENCE(Tableau1; "Prix"; "Thé")`, `=REFERENCE(Ventes; "Prix 2"; "Thé")`},
		{merge, FR, "", `=REFERENCE(Tableau1; Prix; "Thé")`, `=REFERENCE(Ventes; Prix 2; "Thé")`},
		{merge, FR, "", `=EXISTE(Tableau1; "x")`, `=EXISTE(Ventes; "x")`},
		// Source d'un FILTRE : les [Colonne] qui suivent sont les siennes.
		{merge, FR, "", `=FILTRE(Tableau1; [Prix] > 2)`, `=FILTRE(Ventes; [Prix 2] > 2)`},
		{rename, FR, "", "=SOMME(Ventes[Qté]) / Taux", "=SOMME(Achats[Quantité]) / Coef"},
		{rename, FR, "", "=Feuille1!Cell[A8] + 'Feuille1'!Cell[B2:C3]", "='Mes données'!Cell[A8] + 'Mes données'!Cell[B2:C3]"},
		{rename, FR, "", "=Feuille1!$B$3*2", "='Mes données'!$B$3*2"},
		{rename, FR, "", "=Feuille1.Cell_B3", "='Mes données'!Cell[B3]"},
		// Un nom posé par LET n'est pas la cellule nommée.
		{rename, FR, "", "=LET(Taux; 2; Taux*3)", "=LET(Taux; 2; Taux*3)"},
		// Les « ; » d'une constante matricielle ne sont pas des arguments.
		{rename, FR, "", "=LET(a; {1;2}; Taux; 3; SOMME(a)*Taux)", "=LET(a; {1;2}; Taux; 3; SOMME(a)*Taux)"},
		// Le premier argument de FRACTIONNER.TEXTE est une valeur, pas un tableau.
		{rename, FR, "", `=FRACTIONNER.TEXTE(Taux; {",";";"})`, `=FRACTIONNER.TEXTE(Coef; {",";";"})`},
		{rename, FR, "", `=FILTRE(Ventes; Ventes[Qté]>2)`, `=FILTRE(Achats; Achats[Quantité]>2)`},
		{rename, FR, "", `=LIGNES(Ventes)+COLONNES(Ventes[Qté])`, `=LIGNES(Achats)+COLONNES(Achats[Quantité])`},
		// Rien à changer : le texte reste tel quel.
		{rename, FR, "", "=  Autre[Qté] + $B$2", "=  Autre[Qté] + $B$2"},
		{rename, FR, "", `="Ventes[Qté]"`, `="Ventes[Qté]"`},
		{rename, FR, "", "Ventes[Qté]", "Achats[Quantité]"},
	}
	for _, c := range cases {
		got, changed := RenameRefs(c.in, c.loc, c.r, c.self)
		if got != c.want || changed != (c.in != c.want) {
			t.Errorf("%s : %s (changé %v), attendu %s", c.in, got, changed, c.want)
		}
	}
}
