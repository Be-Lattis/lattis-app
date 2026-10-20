package formula

import (
	"strings"
	"testing"
)

// Chaque fonction a un nom allemand et un nom espagnol, et chaque nom, dans
// toutes les langues, ne désigne qu'une fonction : un nom tapé se lit sans
// connaître la langue.
func TestFunctionNames(t *testing.T) {
	known := map[string]bool{}
	owner := map[string]string{}
	for _, f := range funcs {
		known[f.EN] = true
		for _, n := range []string{f.EN, f.FR, namesDE[f.EN], namesES[f.EN]} {
			if n == "" {
				t.Errorf("%s : nom manquant (%q %q %q)", f.EN, f.FR, namesDE[f.EN], namesES[f.EN])
				continue
			}
			if n != strings.ToUpper(n) {
				t.Errorf("%s : %q pas en majuscules", f.EN, n)
			}
			if o, ok := owner[n]; ok && o != f.EN {
				t.Errorf("%q désigne %s et %s", n, o, f.EN)
			}
			owner[n] = f.EN
		}
	}
	for _, m := range []map[string]string{namesDE, namesES} {
		for en := range m {
			if !known[en] {
				t.Errorf("%s : pas au catalogue", en)
			}
		}
	}
}

// Une formule tapée en allemand ou en espagnol est gardée sous la même
// forme que tapée en français (cellules libres) ou en anglais (colonnes
// calculées), et s'y relit à l'identique.
func TestTranslateGermanSpanish(t *testing.T) {
	de, es := ForLanguage("de"), ForLanguage("es")
	for _, c := range []struct{ fr, en, de, es string }{
		{`=SOMME(Ventes[Montant]; 1,5)`, `=SUM(Ventes[Montant], 1.5)`, `=SUMME(Ventes[Montant]; 1,5)`, `=SUMA(Ventes[Montant]; 1,5)`},
		{`=SI(ESTERREUR(1/0); #NOM?; VRAI)`, `=IF(ISERROR(1/0), #NAME?, TRUE)`, `=WENN(ISTFEHLER(1/0); #NAME?; WAHR)`, `=SI(ESERROR(1/0); #¿NOMBRE?; VERDADERO)`},
		{`=SI(FAUX; #VALEUR!; #N/A)&#NOMBRE!&#REF!&#DIV/0!`, `=IF(FALSE, #VALUE!, #N/A)&#NUM!&#REF!&#DIV/0!`, `=WENN(FALSCH; #WERT!; #NV)&#ZAHL!&#BEZUG!&#DIV/0!`, `=SI(FALSO; #¡VALOR!; #N/D)&#¡NUM!&#¡REF!&#¡DIV/0!`},
		{`=RECHERCHEV(Cell[A1]; Clients; 2; FAUX)`, `=VLOOKUP(Cell[A1], Clients, 2, FALSE)`, `=SVERWEIS(Cell[A1]; Clients; 2; FALSCH)`, `=BUSCARV(Cell[A1]; Clients; 2; FALSO)`},
		{`=ANNEE(AUJOURDHUI())+NBCAR(GAUCHE(TEXTE(1; "0"); 1))`, `=YEAR(TODAY())+LEN(LEFT(TEXT(1, "0"), 1))`, `=JAHR(HEUTE())+LÄNGE(LINKS(TEXT(1; "0"); 1))`, `=AÑO(HOY())+LARGO(IZQUIERDA(TEXTO(1; "0"); 1))`},
		{`=SOMME.SI(T[a]; ">1"; T[b])+NB.SI.ENS(T[a]; 1)+ARRONDI(MOYENNE(T[a]); 2)`, `=SUMIF(T[a], ">1", T[b])+COUNTIFS(T[a], 1)+ROUND(AVERAGE(T[a]), 2)`, `=SUMMEWENN(T[a]; ">1"; T[b])+ZÄHLENWENNS(T[a]; 1)+RUNDEN(MITTELWERT(T[a]); 2)`, `=SUMAR.SI(T[a]; ">1"; T[b])+CONTAR.SI.CONJUNTO(T[a]; 1)+REDONDEAR(PROMEDIO(T[a]); 2)`},
		{`=INDEX(T[a]; EQUIV(1; T[b]; 0))&RECHERCHEX(1; T[a]; T[b])`, `=INDEX(T[a], MATCH(1, T[b], 0))&XLOOKUP(1, T[a], T[b])`, `=INDEX(T[a]; VERGLEICH(1; T[b]; 0))&XVERWEIS(1; T[a]; T[b])`, `=INDICE(T[a]; COINCIDIR(1; T[b]; 0))&BUSCARX(1; T[a]; T[b])`},
		{`=REFERENCE(Clients[Ville]; Cell[A1])&PRECEDENT([x])&EXISTE(Clients; 1)`, `=REFERENCE(Clients[Ville], Cell[A1])&PREVIOUS([x])&EXISTS(Clients, 1)`, `=REFERENZ(Clients[Ville]; Cell[A1])&VORHERIGER([x])&EXISTIERT(Clients; 1)`, `=REFERENCIA(Clients[Ville]; Cell[A1])&ANTERIOR([x])&EXISTE(Clients; 1)`},
		{`=GROUPER.PAR(T[a]; T[b]; SOMME)`, `=GROUPBY(T[a], T[b], SUM)`, `=GRUPPIERENNACH(T[a]; T[b]; SUMME)`, `=AGRUPARPOR(T[a]; T[b]; SUMA)`},
		{`=LET(y; 2; o; 3; ET(y>1; OU(o>1; FAUX)))`, `=LET(y, 2, o, 3, AND(y>1, OR(o>1, FALSE)))`, `=LET(y; 2; o; 3; UND(y>1; ODER(o>1; FALSCH)))`, `=LET(y; 2; o; 3; Y(y>1; O(o>1; FALSO)))`},
		{`=SOMME({VRAI;FAUX})`, `=SUM({TRUE;FALSE})`, `=SUMME({WAHR;FALSCH})`, `=SUMA({VERDADERO;FALSO})`},
	} {
		for _, l := range []struct {
			loc  Locale
			want string
		}{{de, c.de}, {es, c.es}} {
			for _, from := range []struct {
				loc Locale
				src string
			}{{FR, c.fr}, {EN, c.en}} {
				got, err := Translate(from.src, from.loc, l.loc)
				if err != nil || got != l.want {
					t.Errorf("%s : %q %v, attendu %q", from.src, got, err, l.want)
					continue
				}
				if back, err := Translate(got, l.loc, from.loc); err != nil || back != from.src {
					t.Errorf("%s relu : %q %v, attendu %q", got, back, err, from.src)
				}
			}
		}
	}
	// Les noms anglais et français restent acceptés en allemand et en
	// espagnol ; WAHR n'est un booléen qu'en allemand.
	if got, _ := Translate(`=SUM(1; 2)+SOMME(VRAI; TRUE)`, de, FR); got != `=SOMME(1; 2)+SOMME(VRAI; VRAI)` {
		t.Errorf("anglais en allemand : %q", got)
	}
	if got, _ := Translate(`=WAHR`, FR, EN); got != `=WAHR` {
		t.Errorf("WAHR en français : %q", got)
	}
	if ErrorText("#VALEUR!", de) != "#WERT!" || ErrorText("#DEVERSEMENT!", es) != "#¡DESBORDAMIENTO!" || ErrorText("#N/A", es) != "#N/D" {
		t.Error("ErrorText")
	}
	if ErrorInput("#WERT!") != "#VALEUR!" || ErrorInput("#ÜBERLAUF!") != "#DEVERSEMENT!" || ErrorInput("#¿NOMBRE?") != "#NOM?" || ErrorInput("#NV") != "#N/A" {
		t.Error("ErrorInput")
	}
}
