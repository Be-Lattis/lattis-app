package formula

import (
	"errors"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/msg"
)

// Translate passe d'une langue à l'autre sans rien perdre : relue dans sa
// langue, la formule redonne la forme gardée.
func TestTranslate(t *testing.T) {
	for _, c := range []struct{ fr, en, de string }{
		{`=SOMME(Ventes[Montant]; 1,5)`, `=SUM(Ventes[Montant], 1.5)`, `=SUM(Ventes[Montant]; 1,5)`},
		{`=SI(ESTERREUR(1/0); #NOM?; VRAI)`, `=IF(ISERROR(1/0), #NAME?, TRUE)`, `=IF(ISERROR(1/0); #NAME?; TRUE)`},
		{`=SOMME({1.2;3.4})`, `=SUM({1,2;3,4})`, `=SUM({1.2;3.4})`},
		{`=REFERENCE(Clients[Ville]; Cell[A1])&"; ,"`, `=REFERENCE(Clients[Ville], Cell[A1])&"; ,"`, `=REFERENCE(Clients[Ville]; Cell[A1])&"; ,"`},
		{`=LET(x; 2,5; x*Feuille2!Cell[B3])`, `=LET(x, 2.5, x*Feuille2!Cell[B3])`, `=LET(x; 2,5; x*Feuille2!Cell[B3])`},
	} {
		for _, l := range []struct {
			lang, want string
		}{{"en", c.en}, {"zh", c.en}, {"de", c.de}, {"es", c.de}, {"fr", c.fr}} {
			loc := ForLanguage(l.lang)
			got, err := Translate(c.fr, FR, loc)
			if err != nil || got != l.want {
				t.Errorf("%s %s : %q %v, attendu %q", l.lang, c.fr, got, err, l.want)
				continue
			}
			if back, err := Translate(got, loc, FR); err != nil || back != c.fr {
				t.Errorf("%s %s : relu %q %v", l.lang, got, back, err)
			}
		}
	}
	// Sans « = », la sortie n'en a pas ; illisible : le texte et l'erreur.
	if got, err := Translate("SOMME(1;2)", FR, EN); err != nil || got != "SUM(1, 2)" {
		t.Errorf("sans = : %q %v", got, err)
	}
	var se *SyntaxError
	if got, err := Translate("=SUM(1;2)", EN, FR); !errors.As(err, &se) || got != "=SUM(1;2)" {
		t.Errorf("illisible : %q %v", got, err)
	}
	if ErrorText("#VALEUR!", EN) != "#VALUE!" || ErrorText("#DEVERSEMENT!", EN) != "#SPILL!" || ErrorText("#VALEUR!", FR) != "#VALEUR!" || ErrorText("x", EN) != "x" {
		t.Error("ErrorText")
	}
	if ErrorInput("#NAME?") != "#NOM?" || ErrorInput("#SPILL!") != "#DEVERSEMENT!" || ErrorInput("#N/A") != "#N/A" {
		t.Error("ErrorInput")
	}
}

// Les erreurs de saisie et de conversion se lisent dans chaque langue ; le
// texte français ne change pas.
func TestSyntaxErrorText(t *testing.T) {
	_, err := Parse(`=SUM(1, 2`, EN)
	var se *SyntaxError
	if !errors.As(err, &se) {
		t.Fatal(err)
	}
	if got := se.Text(msg.EN); got != "position 9: missing closing parenthesis “)” for SUM" {
		t.Errorf("anglais : %q", got)
	}
	if got := se.Text(msg.FR); got != se.Error() || !strings.Contains(got, "il manque une parenthèse fermante") {
		t.Errorf("français : %q / %q", got, se.Error())
	}
	_, err = Parse(`=IF(1)`, EN)
	if !errors.As(err, &se) || se.Text(msg.EN) != "position 1: IF expects 2 to 3 arguments" {
		t.Errorf("arité : %v", err)
	}
	ce := convErr("formula.convert.tableNotFound", msg.Args{"table": "T"})
	if ce.Error() != "tableau T introuvable" || ce.Text(msg.EN) != "table T not found" {
		t.Errorf("conversion : %q %q", ce.Error(), ce.Text(msg.EN))
	}
}

// L'aide à la saisie suit la langue : noms, séparateur des signatures,
// exemples relus dans la langue, nombre d'arguments.
func TestCatalog(t *testing.T) {
	find := func(fs []FunctionInfo, name string) FunctionInfo {
		for _, f := range fs {
			if f.Name == name {
				return f
			}
		}
		t.Fatalf("%s absente", name)
		return FunctionInfo{}
	}
	en := find(Catalog(EN, msg.EN), "SUMIF")
	if en.Other != "SOMME.SI" || !strings.Contains(en.Signature, "SUMIF(plage, critère") || strings.Contains(en.Example, ";") || !strings.HasPrefix(en.Example, "=SUMIF(") || en.Arity != "2 to 3 arguments" {
		t.Errorf("anglais : %+v", en)
	}
	fr := find(Functions(FR), "SOMME.SI")
	if !strings.Contains(fr.Signature, "SOMME.SI(plage; critère") || fr.Arity != "de 2 à 3 arguments" {
		t.Errorf("français : %+v", fr)
	}
	de := find(Catalog(ForLanguage("de"), msg.DE), "SUMIF")
	if !strings.Contains(de.Signature, "SUMIF(plage; critère") || !strings.HasPrefix(de.Example, "=SUMIF(") {
		t.Errorf("allemand : %+v", de)
	}
}
