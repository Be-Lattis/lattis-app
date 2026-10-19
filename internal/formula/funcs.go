package formula

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

type fnKind int

const (
	scalar    fnKind = iota // arguments évalués, implémentation Go commune aux deux backends
	aggregate               // arguments : colonnes entières, expressions sur colonnes, ou valeurs
	scan                    // parcourt les lignes d'un tableau (voir scan.go)
	special                 // traité par chaque backend (REFERENCE, RECHERCHEV, PRECEDENT…)
	spill                   // rend un tableau de valeurs (voir spill.go)
)

type funcDef struct {
	EN, FR   string
	Min, Max int // Max < 0 : nombre d'arguments libre
	kind     fnKind
	impl     func([]Value) Value
}

func (f funcDef) name(l Locale) string {
	if l == EN {
		return f.EN
	}
	return f.FR
}

func (f funcDef) arity() string {
	switch {
	case f.Max < 0:
		return fmt.Sprintf("au moins %d argument(s)", f.Min)
	case f.Min == f.Max:
		return fmt.Sprintf("%d argument(s)", f.Min)
	}
	return fmt.Sprintf("de %d à %d arguments", f.Min, f.Max)
}

// aggLead donne le nombre d'arguments ordinaires placés avant les valeurs
// agrégées (JOINDRE.TEXTE(délimiteur; ignorer_vides; textes…), VAN(taux; …)).
var aggLead = map[string]int{"TEXTJOIN": 2, "NPV": 1}

// aggOrdered : agrégats dont le résultat dépend de l'ordre des valeurs (ordre
// d'affichage du tableau).
var aggOrdered = map[string]bool{"TEXTJOIN": true, "MODE": true, "MODE.SNGL": true, "NPV": true}

// Catalogue des fonctions (voir docs/fonctions.md). Le nom
// anglais est le nom canonique, stocké dans l'AST.
var funcs = []funcDef{
	// Logique
	{"IF", "SI", 2, 3, scalar, fnIf},
	{"IFS", "SI.CONDITIONS", 2, -1, scalar, fnIfs},
	{"SWITCH", "SI.MULTIPLE", 3, -1, scalar, fnSwitch},
	{"IFERROR", "SIERREUR", 2, 2, scalar, func(a []Value) Value {
		if a[0].IsErr() {
			return a[1]
		}
		return a[0]
	}},
	{"IFNA", "SI.NON.DISP", 2, 2, scalar, func(a []Value) Value {
		if a[0].IsErr() && a[0].Err == ErrNA {
			return a[1]
		}
		return a[0]
	}},
	{"AND", "ET", 1, -1, scalar, func(a []Value) Value { return logic(a, true) }},
	{"OR", "OU", 1, -1, scalar, func(a []Value) Value { return logic(a, false) }},
	{"XOR", "OUX", 1, -1, scalar, fnXor},
	{"NOT", "NON", 1, 1, scalar, func(a []Value) Value {
		b, e := Truth(a[0])
		if e.IsErr() {
			return e
		}
		return Boolean(!b)
	}},
	{"ISERROR", "ESTERREUR", 1, 1, scalar, func(a []Value) Value { return Boolean(a[0].IsErr()) }},
	{"ISNA", "ESTNA", 1, 1, scalar, func(a []Value) Value { return Boolean(a[0].IsErr() && a[0].Err == ErrNA) }},
	{"ISBLANK", "ESTVIDE", 1, 1, scalar, func(a []Value) Value { return Boolean(a[0].IsEmpty()) }},
	{"ISNUMBER", "ESTNUM", 1, 1, scalar, func(a []Value) Value { return Boolean(a[0].Kind == Number || a[0].Kind == Date) }},
	{"ISTEXT", "ESTTEXTE", 1, 1, scalar, func(a []Value) Value { return Boolean(a[0].Kind == Text) }},
	{"ISLOGICAL", "ESTLOGIQUE", 1, 1, scalar, func(a []Value) Value { return Boolean(a[0].Kind == Bool) }},
	{"CHOOSE", "CHOISIR", 2, -1, scalar, fnChoose},
	{"LET", "LET", 3, -1, special, nil},

	// Mathématiques
	{"ROUND", "ARRONDI", 2, 2, scalar, fnRound},
	{"ROUNDUP", "ARRONDI.SUP", 2, 2, scalar, roundWith(func(v float64) float64 { return math.Ceil(v) })},
	{"ROUNDDOWN", "ARRONDI.INF", 2, 2, scalar, roundWith(math.Floor)},
	{"INT", "ENT", 1, 1, scalar, math1(math.Floor)},
	{"TRUNC", "TRONQUE", 1, 2, scalar, fnTrunc},
	{"ABS", "ABS", 1, 1, scalar, math1(math.Abs)},
	{"SIGN", "SIGNE", 1, 1, scalar, math1(func(x float64) float64 {
		switch {
		case x > 0:
			return 1
		case x < 0:
			return -1
		}
		return 0
	})},
	{"MOD", "MOD", 2, 2, scalar, fnMod},
	{"CEILING", "PLAFOND", 1, 2, scalar, fnCeiling},
	{"FLOOR", "PLANCHER", 1, 2, scalar, fnFloor},
	{"POWER", "PUISSANCE", 2, 2, scalar, func(a []Value) Value { return Arith('^', a[0], a[1]) }},
	{"SQRT", "RACINE", 1, 1, scalar, math1(math.Sqrt)},
	{"EXP", "EXP", 1, 1, scalar, math1(math.Exp)},
	{"LN", "LN", 1, 1, scalar, math1(func(x float64) float64 {
		if x <= 0 {
			return math.NaN()
		}
		return math.Log(x)
	})},
	{"LOG10", "LOG10", 1, 1, scalar, math1(func(x float64) float64 {
		if x <= 0 {
			return math.NaN()
		}
		return math.Log10(x)
	})},
	{"PI", "PI", 0, 0, scalar, func([]Value) Value { return Num(math.Pi) }},
	{"LOG", "LOG", 1, 2, scalar, fnLog},
	{"SIN", "SIN", 1, 1, scalar, math1(math.Sin)},
	{"COS", "COS", 1, 1, scalar, math1(math.Cos)},
	{"TAN", "TAN", 1, 1, scalar, math1(math.Tan)},
	{"ASIN", "ASIN", 1, 1, scalar, math1(math.Asin)},
	{"ACOS", "ACOS", 1, 1, scalar, math1(math.Acos)},
	{"ATAN", "ATAN", 1, 1, scalar, math1(math.Atan)},
	{"RADIANS", "RADIANS", 1, 1, scalar, math1(func(x float64) float64 { return x * math.Pi / 180 })},
	{"DEGREES", "DEGRES", 1, 1, scalar, math1(func(x float64) float64 { return x * 180 / math.Pi })},
	{"MROUND", "ARRONDI.AU.MULTIPLE", 2, 2, scalar, fnMRound},
	{"QUOTIENT", "QUOTIENT", 2, 2, scalar, fnQuotient},
	{"ISEVEN", "EST.PAIR", 1, 1, scalar, parity(true)},
	{"ISODD", "EST.IMPAIR", 1, 1, scalar, parity(false)},
	{"RAND", "ALEA", 0, 0, special, nil},
	{"RANDBETWEEN", "ALEA.ENTRE.BORNES", 2, 2, special, fnRandBetween},

	// Texte
	{"CONCAT", "CONCAT", 1, -1, scalar, fnConcat},
	{"CONCATENATE", "CONCATENER", 1, -1, scalar, fnConcat},
	{"LEN", "NBCAR", 1, 1, scalar, func(a []Value) Value {
		s, e := ToText(a[0])
		if e.IsErr() {
			return e
		}
		return Num(float64(utf8.RuneCountInString(s)))
	}},
	{"UPPER", "MAJUSCULE", 1, 1, scalar, func(a []Value) Value { return textFn(a[0], strings.ToUpper) }},
	{"LOWER", "MINUSCULE", 1, 1, scalar, func(a []Value) Value { return textFn(a[0], strings.ToLower) }},
	{"PROPER", "NOMPROPRE", 1, 1, scalar, fnProper},
	{"TRIM", "SUPPRESPACE", 1, 1, scalar, fnTrim},
	{"LEFT", "GAUCHE", 1, 2, scalar, fnLeft},
	{"RIGHT", "DROITE", 1, 2, scalar, fnRight},
	{"MID", "STXT", 3, 3, scalar, fnMid},
	{"SUBSTITUTE", "SUBSTITUE", 3, 4, scalar, fnSubstitute},
	{"REPLACE", "REMPLACER", 4, 4, scalar, fnReplace},
	{"FIND", "TROUVE", 2, 3, scalar, find(true)},
	{"SEARCH", "CHERCHE", 2, 3, scalar, find(false)},
	{"TEXT", "TEXTE", 2, 2, scalar, fnText},
	{"VALUE", "CNUM", 1, 1, scalar, fnValue},
	{"REPT", "REPT", 2, 2, scalar, fnRept},
	{"EXACT", "EXACT", 2, 2, scalar, fnExact},
	{"CHAR", "CAR", 1, 1, scalar, fnChar},
	{"CODE", "CODE", 1, 1, scalar, fnCode},
	{"UNICHAR", "UNICAR", 1, 1, scalar, fnUnichar},
	{"UNICODE", "UNICODE", 1, 1, scalar, fnUnicode},
	{"CLEAN", "EPURAGE", 1, 1, scalar, fnClean},
	{"TEXTBEFORE", "TEXTE.AVANT", 2, 6, scalar, textAround(false)},
	{"TEXTAFTER", "TEXTE.APRES", 2, 6, scalar, textAround(true)},
	{"NA", "NA", 0, 0, scalar, func([]Value) Value { return Err(ErrNA) }},

	// Dates
	{"TODAY", "AUJOURDHUI", 0, 0, special, nil},
	{"NOW", "MAINTENANT", 0, 0, special, nil},
	{"DATE", "DATE", 3, 3, scalar, fnDate},
	{"YEAR", "ANNEE", 1, 1, scalar, datePart(func(t timeT) int { return t.Year() })},
	{"MONTH", "MOIS", 1, 1, scalar, datePart(func(t timeT) int { return int(t.Month()) })},
	{"DAY", "JOUR", 1, 1, scalar, datePart(func(t timeT) int { return t.Day() })},
	{"HOUR", "HEURE", 1, 1, scalar, fnHourPart(3600, 24)},
	{"MINUTE", "MINUTE", 1, 1, scalar, fnHourPart(60, 60)},
	{"SECOND", "SECONDE", 1, 1, scalar, fnHourPart(1, 60)},
	{"WEEKDAY", "JOURSEM", 1, 2, scalar, fnWeekday},
	{"ISOWEEKNUM", "NO.SEMAINE.ISO", 1, 1, scalar, fnISOWeek},
	{"WEEKNUM", "NO.SEMAINE", 1, 2, scalar, fnWeeknum},
	{"DAYS", "JOURS", 2, 2, scalar, fnDays},
	{"TIME", "TEMPS", 3, 3, scalar, fnTime},
	{"EOMONTH", "FIN.MOIS", 2, 2, scalar, fnEOMonth},
	{"EDATE", "MOIS.DECALER", 2, 2, scalar, fnEDate},
	{"DATEDIF", "DATEDIF", 3, 3, scalar, fnDatedif},
	{"NETWORKDAYS", "NB.JOURS.OUVRES", 2, 3, scan, nil},
	{"WORKDAY", "SERIE.JOUR.OUVRE", 2, 3, scan, nil},
	{"NETWORKDAYS.INTL", "NB.JOURS.OUVRES.INTL", 2, 4, scan, nil},
	{"WORKDAY.INTL", "SERIE.JOUR.OUVRE.INTL", 2, 4, scan, nil},
	{"DATEVALUE", "DATEVAL", 1, 1, scalar, fnDateValue},
	{"TIMEVALUE", "TEMPSVAL", 1, 1, scalar, fnTimeValue},
	{"YEARFRAC", "FRACTION.ANNEE", 2, 3, scalar, fnYearFrac},

	// Agrégats
	{"SUM", "SOMME", 1, -1, aggregate, nil},
	{"AVERAGE", "MOYENNE", 1, -1, aggregate, nil},
	{"COUNT", "NB", 1, -1, aggregate, nil},
	{"COUNTA", "NBVAL", 1, -1, aggregate, nil},
	{"COUNTBLANK", "NB.VIDE", 1, -1, aggregate, nil},
	{"MIN", "MIN", 1, -1, aggregate, nil},
	{"MAX", "MAX", 1, -1, aggregate, nil},
	{"MEDIAN", "MEDIANE", 1, -1, aggregate, nil},
	{"LARGE", "GRANDE.VALEUR", 2, 2, scan, nil},
	{"SMALL", "PETITE.VALEUR", 2, 2, scan, nil},
	{"STDEV", "ECARTYPE", 1, -1, aggregate, nil},
	{"STDEV.S", "ECARTYPE.STANDARD", 1, -1, aggregate, nil},
	{"STDEVP", "ECARTYPEP", 1, -1, aggregate, nil},
	{"STDEV.P", "ECARTYPE.PEARSON", 1, -1, aggregate, nil},
	{"VAR", "VAR", 1, -1, aggregate, nil},
	{"VAR.S", "VAR.S", 1, -1, aggregate, nil},
	// VARP s'appelle VAR.P en français, le nom anglais de VAR.P.N : seule la
	// forme récente est au catalogue, et VAR.P y mène dans les deux langues.
	{"VAR.P", "VAR.P.N", 1, -1, aggregate, nil},
	{"MODE", "MODE", 1, -1, aggregate, nil},
	{"MODE.SNGL", "MODE.SIMPLE", 1, -1, aggregate, nil},
	{"PERCENTILE", "CENTILE", 2, 2, scan, nil},
	{"PERCENTILE.INC", "CENTILE.INCLURE", 2, 2, scan, nil},
	{"PERCENTILE.EXC", "CENTILE.EXCLURE", 2, 2, scan, nil},
	{"QUARTILE", "QUARTILE", 2, 2, scan, nil},
	{"QUARTILE.INC", "QUARTILE.INCLURE", 2, 2, scan, nil},
	{"QUARTILE.EXC", "QUARTILE.EXCLURE", 2, 2, scan, nil},
	{"CORREL", "COEFFICIENT.CORRELATION", 2, 2, scan, nil},
	{"SLOPE", "PENTE", 2, 2, scan, nil},
	{"INTERCEPT", "ORDONNEE.ORIGINE", 2, 2, scan, nil},
	{"PRODUCT", "PRODUIT", 1, -1, aggregate, nil},
	{"TEXTJOIN", "JOINDRE.TEXTE", 3, -1, aggregate, nil},
	{"SUBTOTAL", "SOUS.TOTAL", 2, -1, special, nil},
	{"SUMIF", "SOMME.SI", 2, 3, scan, nil},
	{"COUNTIF", "NB.SI", 2, 2, scan, nil},
	{"AVERAGEIF", "MOYENNE.SI", 2, 3, scan, nil},
	{"SUMIFS", "SOMME.SI.ENS", 3, -1, scan, nil},
	{"COUNTIFS", "NB.SI.ENS", 2, -1, scan, nil},
	{"AVERAGEIFS", "MOYENNE.SI.ENS", 3, -1, scan, nil},
	{"MAXIFS", "MAX.SI.ENS", 3, -1, scan, nil},
	{"MINIFS", "MIN.SI.ENS", 3, -1, scan, nil},
	{"SUMPRODUCT", "SOMMEPROD", 1, -1, scan, nil},

	// Finances (voir funcs_finance.go)
	{"PMT", "VPM", 3, 5, scalar, fnPmt},
	{"PV", "VA", 3, 5, scalar, fnPv},
	{"FV", "VC", 3, 5, scalar, fnFv},
	{"NPER", "NPM", 3, 5, scalar, fnNper},
	{"RATE", "TAUX", 3, 6, scalar, fnRate},
	{"IPMT", "INTPER", 4, 6, scalar, fnIpmtPpmt(false)},
	{"PPMT", "PRINCPER", 4, 6, scalar, fnIpmtPpmt(true)},
	{"NPV", "VAN", 2, -1, aggregate, nil},
	{"IRR", "TRI", 1, 2, scan, nil},

	// Calculs entre lignes, dans l'ordre du tableau
	{"PREVIOUS", "PRECEDENT", 1, 2, special, nil},
	{"NEXT", "SUIVANT", 1, 2, special, nil},
	{"RUNNINGSUM", "CUMUL", 1, 1, special, nil},
	{"RANK", "RANG", 1, 3, scan, nil},

	// Recherche
	{"REFERENCE", "REFERENCE", 2, 3, special, nil},
	{"REFERENCE.APPROX", "REFERENCE.APPROCHEE", 2, 3, special, nil},
	{"EXISTS", "EXISTE", 2, 2, special, nil},
	{"MATCH", "EQUIV", 2, 3, scan, nil},
	{"INDEX", "INDEX", 2, 2, scan, nil},
	{"VLOOKUP", "RECHERCHEV", 3, 4, special, nil},
	{"XLOOKUP", "RECHERCHEX", 3, 6, special, nil},
	{"HLOOKUP", "RECHERCHEH", 3, 4, special, nil},
	{"ROW", "LIGNE", 0, 1, special, nil},
	{"COLUMN", "COLONNE", 0, 1, special, nil},
	{"ROWS", "LIGNES", 1, 1, special, nil},
	{"COLUMNS", "COLONNES", 1, 1, special, nil},

	// Géographie (contours des communes embarqués, voir funcs_geo.go)
	{"MUNICIPALITY", "COMMUNE", 2, 3, scalar, fnMunicipality},

	// Tableaux dynamiques
	{"FILTER", "FILTRE", 2, 3, spill, nil},
	{"SORT", "TRIER", 1, 4, spill, nil},
	{"SORTBY", "TRIERPAR", 2, -1, spill, nil},
	{"UNIQUE", "UNIQUE", 1, 3, spill, nil},
	{"TAKE", "PRENDRE", 2, 3, spill, nil},
	{"DROP", "EXCLURE", 2, 3, spill, nil},
	{"GROUPBY", "GROUPER.PAR", 3, 3, spill, nil},
	{"QUERY", "REQUETE", 1, 1, spill, nil},
	{"SEQUENCE", "SEQUENCE", 1, 4, spill, nil},
	{"TEXTSPLIT", "FRACTIONNER.TEXTE", 2, 6, spill, nil},
}

// refused : fonctions d'Excel reconnues pour être refusées à la saisie avec
// un message (voir Convert), et donc absentes de l'aide à la saisie.
var refused = map[string]bool{"HLOOKUP": true}

var funcIndex = func() map[string]funcDef {
	m := map[string]funcDef{}
	for _, f := range funcs {
		m[f.EN] = f
		m[f.FR] = f
	}
	return m
}()

// LocalName donne le nom d'une fonction (anglais ou français) dans une
// langue ; le nom tel quel pour une fonction inconnue.
func LocalName(name string, l Locale) string {
	if f, ok := lookupFunc(name); ok {
		return f.name(l)
	}
	return strings.ToUpper(name)
}

func lookupFunc(name string) (funcDef, bool) {
	f, ok := funcIndex[strings.ToUpper(name)]
	return f, ok
}

// --- Logique ------------------------------------------------------------------

func fnIf(a []Value) Value {
	b, e := Truth(a[0])
	if e.IsErr() {
		return e
	}
	if b {
		return a[1]
	}
	if len(a) > 2 {
		return a[2]
	}
	return Boolean(false)
}

// fnIfs : SI.CONDITIONS(cond1; valeur1; cond2; valeur2…), #N/A si aucune
// condition n'est vraie.
func fnIfs(a []Value) Value {
	if len(a)%2 != 0 {
		return Err(ErrValue)
	}
	for i := 0; i < len(a); i += 2 {
		b, e := Truth(a[i])
		if e.IsErr() {
			return e
		}
		if b {
			return a[i+1]
		}
	}
	return Err(ErrNA)
}

// fnSwitch : SI.MULTIPLE(expr; val1; résultat1; …; [défaut]).
func fnSwitch(a []Value) Value {
	x := a[0]
	if x.IsErr() {
		return x
	}
	rest := a[1:]
	for len(rest) >= 2 {
		if !rest[0].IsErr() && cmp(x, rest[0]) == 0 && order(x.Kind) == order(rest[0].Kind) {
			return rest[1]
		}
		rest = rest[2:]
	}
	if len(rest) == 1 {
		return rest[0]
	}
	return Err(ErrNA)
}

func logic(a []Value, and bool) Value {
	for _, v := range a {
		b, e := Truth(v)
		if e.IsErr() {
			return e
		}
		if b != and {
			return Boolean(!and)
		}
	}
	return Boolean(and)
}

func fnXor(a []Value) Value {
	n := 0
	for _, v := range a {
		b, e := Truth(v)
		if e.IsErr() {
			return e
		}
		if b {
			n++
		}
	}
	return Boolean(n%2 == 1)
}

// --- Mathématiques -----------------------------------------------------------

func math1(f func(float64) float64) func([]Value) Value {
	return func(a []Value) Value {
		x, e := ToNumber(a[0])
		if e.IsErr() {
			return e
		}
		return number(f(x))
	}
}

// roundWith arrondit à d décimales avec une fonction d'arrondi appliquée à la
// valeur absolue (signe conservé), après correction de la représentation
// binaire : 15 chiffres significatifs, comme Excel.
func roundWith(f func(float64) float64) func([]Value) Value {
	return func(a []Value) Value {
		x, e := ToNumber(a[0])
		if e.IsErr() {
			return e
		}
		d, e := ToNumber(a[1])
		if e.IsErr() {
			return e
		}
		p := math.Pow(10, math.Trunc(d))
		v, _ := parseNumber(FormatNumber(x * p))
		return number(math.Copysign(f(math.Abs(v)), v) / p)
	}
}

// fnRound arrondit « au plus loin de zéro » comme Excel (2,675 arrondi à 2
// décimales donne 2,68).
var fnRound = roundWith(func(v float64) float64 { return math.Floor(v + 0.5) })

func fnTrunc(a []Value) Value {
	if len(a) < 2 || a[1].IsEmpty() {
		a = []Value{a[0], Num(0)}
	}
	return roundWith(math.Floor)(a)
}

// fnMod : le reste a le signe du diviseur, comme dans Excel.
func fnMod(a []Value) Value {
	x, e := ToNumber(a[0])
	if e.IsErr() {
		return e
	}
	d, e := ToNumber(a[1])
	if e.IsErr() {
		return e
	}
	if d == 0 {
		return Err(ErrDiv0)
	}
	return number(x - d*math.Floor(x/d))
}

// fnRandBetween reçoit en 3e argument le tirage aléatoire (voir randAt).
func fnRandBetween(a []Value) Value {
	lo, e := ToNumber(a[0])
	if e.IsErr() {
		return e
	}
	hi, e := ToNumber(a[1])
	if e.IsErr() {
		return e
	}
	lo, hi = math.Ceil(lo), math.Floor(hi)
	if lo > hi {
		return Err(ErrNum)
	}
	return Num(lo + math.Floor(a[2].Num*(hi-lo+1)))
}

// randAt est un tirage pseudo-aléatoire stable : il ne dépend que de la
// graine du document (changée par F9), de la ligne et du rang de l'appel dans
// la formule. Les valeurs ne bougent donc pas quand on fait défiler la grille.
func randAt(seed uint64, row int64, k int) float64 {
	x := seed ^ uint64(row)*0x9E3779B97F4A7C15 ^ uint64(k+1)*0xC2B2AE3D27D4EB4F
	x ^= x >> 30
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 27
	x *= 0x94D049BB133111EB
	x ^= x >> 31
	return float64(x>>11) / (1 << 53)
}

func textFn(v Value, f func(string) string) Value {
	s, e := ToText(v)
	if e.IsErr() {
		return e
	}
	return Str(f(s))
}
