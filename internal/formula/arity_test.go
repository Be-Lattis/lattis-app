package formula

import (
	"fmt"
	"strings"
	"testing"
)

// fullCalls : un appel de chaque fonction avec tous ses arguments (pour une
// fonction à nombre d'arguments libre, les obligatoires et une répétition).
// TestArity le tronque à chaque nombre d'arguments accepté. « @n » en tête :
// évalué sur la ligne n de Ventes.
var fullCalls = map[string]string{
	"IF":          `=SI(FAUX;1;2)`,
	"IFS":         `=SI.CONDITIONS(VRAI;1;VRAI;2)`,
	"SWITCH":      `=SI.MULTIPLE(2;1;"a";2;"b";"c")`,
	"IFERROR":     `=SIERREUR(1/0;2)`,
	"IFNA":        `=SI.NON.DISP(NA();2)`,
	"AND":         `=ET(VRAI;VRAI)`,
	"OR":          `=OU(FAUX;VRAI)`,
	"XOR":         `=OUX(VRAI;FAUX)`,
	"NOT":         `=NON(FAUX)`,
	"ISERROR":     `=ESTERREUR(1/0)`,
	"ISNA":        `=ESTNA(NA())`,
	"ISBLANK":     `=ESTVIDE(1)`,
	"ISNUMBER":    `=ESTNUM(1)`,
	"ISTEXT":      `=ESTTEXTE("a")`,
	"ISLOGICAL":   `=ESTLOGIQUE(VRAI)`,
	"CHOOSE":      `=CHOISIR(1;"a";"b")`,
	"LET":         `=LET(x;2;y;3;x*y)`,
	"ROUND":       `=ARRONDI(2,675;2)`,
	"ROUNDUP":     `=ARRONDI.SUP(2,12;1)`,
	"ROUNDDOWN":   `=ARRONDI.INF(2,18;1)`,
	"INT":         `=ENT(-2,5)`,
	"TRUNC":       `=TRONQUE(2,57;1)`,
	"ABS":         `=ABS(-2)`,
	"SIGN":        `=SIGNE(-2)`,
	"MOD":         `=MOD(7;3)`,
	"CEILING":     `=PLAFOND(2,3;0,5)`,
	"FLOOR":       `=PLANCHER(2,3;0,5)`,
	"POWER":       `=PUISSANCE(2;3)`,
	"SQRT":        `=RACINE(4)`,
	"EXP":         `=EXP(0)`,
	"LN":          `=LN(1)`,
	"LOG10":       `=LOG10(100)`,
	"PI":          `=PI()`,
	"LOG":         `=LOG(8;2)`,
	"SIN":         `=SIN(0)`,
	"COS":         `=COS(0)`,
	"TAN":         `=TAN(0)`,
	"ASIN":        `=ASIN(0)`,
	"ACOS":        `=ACOS(1)`,
	"ATAN":        `=ATAN(0)`,
	"RADIANS":     `=RADIANS(180)`,
	"DEGREES":     `=DEGRES(0)`,
	"MROUND":      `=ARRONDI.AU.MULTIPLE(7;5)`,
	"QUOTIENT":    `=QUOTIENT(7;2)`,
	"ISEVEN":      `=EST.PAIR(2)`,
	"ISODD":       `=EST.IMPAIR(3)`,
	"RAND":        `=ALEA()`,
	"RANDBETWEEN": `=ALEA.ENTRE.BORNES(1;1)`,
	"CONCAT":      `=CONCAT("a";"b")`,
	"CONCATENATE": `=CONCATENER("a";"b")`,
	"LEN":         `=NBCAR("abc")`,
	"UPPER":       `=MAJUSCULE("a")`,
	"LOWER":       `=MINUSCULE("A")`,
	"PROPER":      `=NOMPROPRE("ab cd")`,
	"TRIM":        `=SUPPRESPACE(" a ")`,
	"LEFT":        `=GAUCHE("abc";2)`,
	"RIGHT":       `=DROITE("abc";2)`,
	"MID":         `=STXT("abc";2;1)`,
	"SUBSTITUTE":  `=SUBSTITUE("aaa";"a";"b";2)`,
	"REPLACE":     `=REMPLACER("abc";2;1;"x")`,
	"FIND":        `=TROUVE("b";"abcb";3)`,
	"SEARCH":      `=CHERCHE("B";"abcb";3)`,
	"TEXT":        `=TEXTE(1,5;"0,00")`,
	"VALUE":       `=CNUM("12")`,
	"REPT":        `=REPT("a";2)`,
	"EXACT":       `=EXACT("a";"a")`,
	"CHAR":        `=CAR(65)`,
	"CODE":        `=CODE("A")`,
	"UNICHAR":     `=UNICAR(65)`,
	"UNICODE":     `=UNICODE("A")`,
	"CLEAN":       `=EPURAGE("a")`,
	"TEXTBEFORE":  `=TEXTE.AVANT("a-b-c";"-";2;0;0;"x")`,
	"TEXTAFTER":   `=TEXTE.APRES("a-b-c";"-";2;0;0;"x")`,
	"NA":          `=NA()`,

	"TODAY":            `=AUJOURDHUI()`,
	"NOW":              `=MAINTENANT()`,
	"SHEETCHOICE":      `=ONGLET.CHOIX()`,
	"DATE":             `=DATE(2026;1;31)`,
	"YEAR":             `=ANNEE(DATE(2026;1;31))`,
	"MONTH":            `=MOIS(DATE(2026;1;31))`,
	"DAY":              `=JOUR(DATE(2026;1;31))`,
	"HOUR":             `=HEURE(0,75)`,
	"MINUTE":           `=MINUTE(0,75)`,
	"SECOND":           `=SECONDE(0,75)`,
	"WEEKDAY":          `=JOURSEM(DATE(2026;1;31);2)`,
	"ISOWEEKNUM":       `=NO.SEMAINE.ISO(DATE(2026;1;31))`,
	"WEEKNUM":          `=NO.SEMAINE(DATE(2026;1;31);2)`,
	"DAYS":             `=JOURS(DATE(2026;2;1);DATE(2026;1;1))`,
	"TIME":             `=TEMPS(9;30;0)`,
	"EOMONTH":          `=FIN.MOIS(DATE(2026;1;15);1)`,
	"EDATE":            `=MOIS.DECALER(DATE(2026;1;15);1)`,
	"DATEDIF":          `=DATEDIF(DATE(2025;1;1);DATE(2026;1;1);"a")`,
	"NETWORKDAYS":      `=NB.JOURS.OUVRES(DATE(2026;5;1);DATE(2026;5;31);Feries[Jour])`,
	"WORKDAY":          `=SERIE.JOUR.OUVRE(DATE(2026;5;1);5;Feries[Jour])`,
	"NETWORKDAYS.INTL": `=NB.JOURS.OUVRES.INTL(DATE(2026;5;1);DATE(2026;5;31);11;Feries[Jour])`,
	"WORKDAY.INTL":     `=SERIE.JOUR.OUVRE.INTL(DATE(2026;5;1);5;"0000011";Feries[Jour])`,
	"DATEVALUE":        `=DATEVAL("15/01/2026")`,
	"TIMEVALUE":        `=TEMPSVAL("18:30")`,
	"YEARFRAC":         `=FRACTION.ANNEE(DATE(2025;1;1);DATE(2026;1;1);1)`,

	"SUM":            `=SOMME(Ventes[Prix];1)`,
	"AVERAGE":        `=MOYENNE(Ventes[Prix];1)`,
	"COUNT":          `=NB(Ventes[Prix];1)`,
	"COUNTA":         `=NBVAL(Ventes[Prix];1)`,
	"COUNTBLANK":     `=NB.VIDE(Ventes[Prix];"")`,
	"MIN":            `=MIN(Ventes[Prix];1)`,
	"MAX":            `=MAX(Ventes[Prix];1)`,
	"MEDIAN":         `=MEDIANE(Ventes[Prix];1)`,
	"LARGE":          `=GRANDE.VALEUR(Ventes[Prix];2)`,
	"SMALL":          `=PETITE.VALEUR(Ventes[Prix];2)`,
	"STDEV":          `=ECARTYPE(Ventes[Prix];1)`,
	"STDEV.S":        `=ECARTYPE.STANDARD(Ventes[Prix];1)`,
	"STDEVP":         `=ECARTYPEP(Ventes[Prix];1)`,
	"STDEV.P":        `=ECARTYPE.PEARSON(Ventes[Prix];1)`,
	"VAR":            `=VAR(Ventes[Prix];1)`,
	"VAR.S":          `=VAR.S(Ventes[Prix];1)`,
	"VAR.P":          `=VAR.P.N(Ventes[Prix];1)`,
	"MODE":           `=MODE(Ventes[Qté];2)`,
	"MODE.SNGL":      `=MODE.SIMPLE(Ventes[Qté];2)`,
	"PERCENTILE":     `=CENTILE(Ventes[Prix];0,5)`,
	"PERCENTILE.INC": `=CENTILE.INCLURE(Ventes[Prix];0,5)`,
	"PERCENTILE.EXC": `=CENTILE.EXCLURE(Ventes[Prix];0,5)`,
	"QUARTILE":       `=QUARTILE(Ventes[Prix];1)`,
	"QUARTILE.INC":   `=QUARTILE.INCLURE(Ventes[Prix];1)`,
	"QUARTILE.EXC":   `=QUARTILE.EXCLURE(Ventes[Prix];2)`,
	"CORREL":         `=COEFFICIENT.CORRELATION(Ventes[Prix];Ventes[Qté])`,
	"SLOPE":          `=PENTE(Ventes[Prix];Ventes[Qté])`,
	"INTERCEPT":      `=ORDONNEE.ORIGINE(Ventes[Prix];Ventes[Qté])`,
	"PRODUCT":        `=PRODUIT(Ventes[Qté];2)`,
	"TEXTJOIN":       `=JOINDRE.TEXTE(",";VRAI;Ventes[Produit];"x")`,
	"SUBTOTAL":       `=SOUS.TOTAL(9;Ventes[Prix];Ventes[Qté])`,
	"SUMIF":          `=SOMME.SI(Ventes[Qté];">1";Ventes[Prix])`,
	"COUNTIF":        `=NB.SI(Ventes[Qté];">1")`,
	"AVERAGEIF":      `=MOYENNE.SI(Ventes[Qté];">1";Ventes[Prix])`,
	"SUMIFS":         `=SOMME.SI.ENS(Ventes[Prix];Ventes[Qté];">1";Ventes[Payé];VRAI)`,
	"COUNTIFS":       `=NB.SI.ENS(Ventes[Qté];">1";Ventes[Payé];VRAI)`,
	"AVERAGEIFS":     `=MOYENNE.SI.ENS(Ventes[Prix];Ventes[Qté];">1";Ventes[Payé];VRAI)`,
	"MAXIFS":         `=MAX.SI.ENS(Ventes[Prix];Ventes[Qté];">1";Ventes[Payé];VRAI)`,
	"MINIFS":         `=MIN.SI.ENS(Ventes[Prix];Ventes[Qté];">1";Ventes[Payé];VRAI)`,
	"SUMPRODUCT":     `=SOMMEPROD(Ventes[Prix];Ventes[Qté])`,

	"PMT":   `=VPM(4%/12;240;200000;0;0)`,
	"PV":    `=VA(4%/12;240;-1200;0;0)`,
	"FV":    `=VC(3%/12;120;-100;-1000;0)`,
	"NPER":  `=NPM(4%/12;-1200;200000;0;0)`,
	"RATE":  `=TAUX(240;-1200;200000;0;0;0,1)`,
	"IPMT":  `=INTPER(4%/12;1;240;200000;0;0)`,
	"PPMT":  `=PRINCPER(4%/12;1;240;200000;0;0)`,
	"NPV":   `=VAN(8%;Ventes[Prix];1)`,
	"IRR":   `=TRI(Ventes[Prix]-50;0,1)`,

	"PREVIOUS":   `@2=PRECEDENT([Prix];0)`,
	"NEXT":       `@2=SUIVANT([Prix];0)`,
	"RUNNINGSUM": `@2=CUMUL([Prix])`,
	"RANK":       `@2=RANG([Prix];Ventes[Prix];1)`,

	"REFERENCE":        `=REFERENCE(Clients;Nom;2)`,
	"REFERENCE.APPROX": `=REFERENCE.APPROCHEE(Baremes;Taux;150)`,
	"EXISTS":           `=EXISTE(Clients;2)`,
	"MATCH":            `=EQUIV(3;Ventes[Qté];0)`,
	"INDEX":            `=INDEX(Ventes[Produit];2)`,
	"VLOOKUP":          `=RECHERCHEV(2;Clients;2;FAUX)`,
	"XLOOKUP":          `=RECHERCHEX(2;Clients[Id];Clients[Nom];"x";0;1)`,
	"HLOOKUP":          `=RECHERCHEH(2;Clients;2;FAUX)`,
	"ROW":              `=LIGNE(Cell[B3])`,
	"COLUMN":           `=COLONNE(Cell[C1])`,
	"ROWS":             `=LIGNES(Ventes)`,
	"COLUMNS":          `=COLONNES(Ventes)`,

	"MUNICIPALITY": `=COMMUNE(48,853;2,3499;"code")`,

	"FILTER":    `=FILTRE(Ventes[Produit];Ventes[Qté]>1;"aucun")`,
	"SORT":      `=TRIER(Ventes[Prix];1;-1;FAUX)`,
	"SORTBY":    `=TRIERPAR(Ventes[Produit];Ventes[Prix];-1;Ventes[Qté];1)`,
	"UNIQUE":    `=UNIQUE(Ventes[Payé];FAUX;VRAI)`,
	"TAKE":      `=PRENDRE(Ventes[Produit];2;1)`,
	"DROP":      `=EXCLURE(Ventes[Produit];1;1)`,
	"GROUPBY":   `=GROUPER.PAR(Ventes[Payé];Ventes[Montant];SOMME)`,
	"QUERY":     `=REQUETE("SELECT 1 AS a")`,
	"SEQUENCE":  `=SEQUENCE(2;2;1;1)`,
	"TEXTSPLIT": `=FRACTIONNER.TEXTE("a,b;c";",";";";VRAI;0;"-")`,
}

func (f *fixtureDoc) spillGo(n Node) string {
	cols, rows, e := EvalSpill(n, &Env{Schema: f.schema, Data: f.data, Names: f.names.Values})
	if e.IsErr() {
		return e.String()
	}
	return renderSpill(cols, rows)
}

func (f *fixtureDoc) spillSQL(t *testing.T, n Node) string {
	sp, e := CompileSpill(n, f.schema, f.names.Values)
	if e.IsErr() {
		return e.String()
	}
	rs, err := f.db.Read().Query(sp.SQL, sp.Args...)
	if err != nil {
		return "(erreur SQL " + err.Error() + ")"
	}
	defer rs.Close()
	if sp.Columns == nil { // REQUETE : colonnes connues à l'exécution
		sp.Columns, _ = rs.Columns()
	}
	var rows [][]Value
	for rs.Next() {
		raw := make([]any, len(sp.Columns))
		ptrs := make([]any, len(raw))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		row := make([]Value, len(raw))
		for i, v := range raw {
			row[i] = FromResult(v)
		}
		rows = append(rows, row)
	}
	return renderSpill(sp.Columns, rows)
}

func renderSpill(cols []string, rows [][]Value) string {
	out := []string{strings.Join(cols, "|")}
	for _, r := range rows {
		var cells []string
		for _, v := range r {
			cells = append(cells, v.String())
		}
		out = append(out, strings.Join(cells, "|"))
	}
	return strings.Join(out, " / ")
}

// TestArity vérifie, pour chaque fonction du catalogue, que le nombre
// d'arguments déclaré (Min, Max) est celui que l'analyse accepte, et que les
// deux backends acceptent chaque nombre d'arguments documenté : appelée avec
// ses n premiers arguments (Min ≤ n ≤ tous), la fonction ne donne ni #VALEUR!
// ni #NOM? (un argument documenté mais refusé), et le même résultat en Go et
// en SQL.
func TestArity(t *testing.T) {
	f := fixture(t)
	for _, fn := range funcs {
		src, ok := fullCalls[fn.EN]
		if !ok {
			t.Errorf("%s : pas d'appel complet dans fullCalls", fn.EN)
			continue
		}
		row := 0
		if strings.HasPrefix(src, "@") {
			row = int(src[1] - '0')
			src = src[2:]
		}
		full := mustParse(t, src, FR).(Call)
		if full.Fn != fn.EN {
			t.Errorf("%s : l'appel complet %s appelle %s", fn.EN, src, full.Fn)
			continue
		}
		if fn.Max >= 0 && len(full.Args) != fn.Max {
			t.Errorf("%s : l'appel complet a %d arguments, Max=%d", fn.EN, len(full.Args), fn.Max)
		}
		// L'analyse refuse un argument de moins et un de plus.
		with := func(n int) string {
			args := make([]string, n)
			for i := range args {
				args[i] = "1"
			}
			return "=" + fn.FR + "(" + strings.Join(args, ";") + ")"
		}
		if fn.Min > 0 {
			if _, err := Parse(with(fn.Min-1), FR); err == nil || !strings.Contains(err.Error(), "attend") {
				t.Errorf("%s : %d argument(s) acceptés à l'analyse (Min=%d)", fn.EN, fn.Min-1, fn.Min)
			}
		}
		if fn.Max >= 0 {
			if _, err := Parse(with(fn.Max+1), FR); err == nil || !strings.Contains(err.Error(), "attend") {
				t.Errorf("%s : %d arguments acceptés à l'analyse (Max=%d)", fn.EN, fn.Max+1, fn.Max)
			}
		}
		if convertedAtInput[fn.EN] {
			continue // RECHERCHEV… : réécrites à la saisie (voir Convert), jamais évaluées
		}
		calls := []Call{}
		if legacyForm[fn.EN] {
			// Ancienne forme complète, puis la forme documentée.
			calls = append(calls, full, mustParse(t, extraCalls[fn.EN], FR).(Call))
		} else {
			for n := fn.Min; n <= len(full.Args); n++ {
				if skipCount(fn, n) {
					continue
				}
				calls = append(calls, Call{Fn: full.Fn, Args: full.Args[:n]})
			}
		}
		for _, c := range calls {
			form := Format(c, FR)
			if _, err := Parse(form, FR); err != nil {
				t.Errorf("%s : %s refusé à l'analyse : %v", fn.EN, form, err)
				continue
			}
			var goRes, sqlRes string
			if fn.kind == spill {
				goRes, sqlRes = f.spillGo(c), f.spillSQL(t, c)
			} else {
				goRes, sqlRes = f.evalGo(c, row).String(), f.evalSQL(t, c, row).String()
			}
			if testing.Verbose() {
				fmt.Printf("%-18s %-60s Go=%-30q SQL=%q\n", fn.EN, form, goRes, sqlRes)
			}
			if fn.EN == "QUERY" {
				goRes = sqlRes // SQL libre : moteur SQL seulement
			}
			if goRes != sqlRes {
				t.Errorf("%s : Go donne %q, SQL %q", form, goRes, sqlRes)
			}
			refused := strings.Contains(sqlRes, "#VALEUR!") || strings.Contains(sqlRes, "#NOM?") || strings.Contains(sqlRes, "erreur SQL")
			if refused != unsupported[form] {
				t.Errorf("%s : %q (argument documenté mais refusé, ou refus attendu disparu : voir unsupported)", form, sqlRes)
			}
		}
	}
}

// convertedAtInput : fonctions d'Excel réécrites à la saisie (RECHERCHEV en
// REFERENCE…) ou refusées (RECHERCHEH) : jamais évaluées telles quelles.
var convertedAtInput = map[string]bool{"VLOOKUP": true, "XLOOKUP": true, "HLOOKUP": true}

// extraCalls : la forme documentée des fonctions qui acceptent encore une
// ancienne forme plus longue (legacyForm).
var extraCalls = map[string]string{
	"REFERENCE":        `=REFERENCE(Clients[Nom];2)`,
	"REFERENCE.APPROX": `=REFERENCE.APPROCHEE(Baremes[Taux];150)`,
}

// unsupported : appels dont #VALEUR! est documenté (argument « non pris en
// charge »).
var unsupported = map[string]bool{
	"=LIGNE()":                        true, // Lattis ne lit pas la position de la formule
	"=COLONNE()":                      true,
	"=PRENDRE(Ventes[Produit]; 2; 1)": true, // colonnes : non pris en charge
	"=EXCLURE(Ventes[Produit]; 1; 1)": true,
}

// skipCount : nombres d'arguments qui coupent un groupe répété en son milieu
// (SOMME.SI.ENS(plage; plage_critères1; critère1; plage_critères2)) ou
// laissent LET sans calcul : refusés à juste titre.
func skipCount(fn funcDef, n int) bool {
	switch fn.EN {
	case "LET":
		return n < 5 // LET(x; 2; y) : y inconnu ; la forme courte est essayée ailleurs
	case "SWITCH", "SORTBY", "CHOOSE":
		return false // défaut final, ordre facultatif, valeurs simples
	}
	return docs[fn.EN].repeat == 2 && (n-fn.Min)%2 != 0
}
