package formula

import (
	"strconv"
	"strings"
)

// Nombres écrits dans du texte (décision 0013) : une fonction qui attend des
// nombres (SOMME, MOYENNE, MIN, SOMME.SI…) lit les valeurs d'une colonne de
// texte comme les nombres qu'elles écrivent, à la française ou à
// l'étrangère ; une valeur qui n'écrit pas de nombre compte pour 0.

// TextNumber lit un nombre écrit dans un texte, selon les usages de loc pour
// le seul cas ambigu. Règles :
//
//   - séparateurs de milliers : espace (normale, insécable, fine), apostrophe
//     (« 1'234.5 », Suisse) ; une virgule ou un point répétés par groupes de
//     trois chiffres (« 1,234,567 », « 1.234.567 ») ;
//   - virgule et point ensemble : le dernier est la décimale, l'autre sépare
//     les milliers (« 1,234.56 » et « 1.234,56 » valent 1234,56) ;
//   - un seul séparateur suivi d'autre chose que trois chiffres : décimale
//     (« 3.5 », « 3,5 », « 0.250 ») ;
//   - un seul séparateur suivi d'exactement trois chiffres, après 1 à 3
//     chiffres autres que 0 seul (« 1.234 », « 1,234 ») : ambigu, il suit la
//     langue (loc) : décimale si c'est le séparateur décimal de la langue,
//     milliers sinon. En français, « 1,234 » vaut 1,234 et « 1.234 » 1234 ;
//     en anglais, l'inverse ;
//   - exposant (« 1,5e3 »), signe devant ou derrière (« -12 », « 12- »),
//     parenthèses comptables (« (12) » vaut -12), pourcentage (« 15 % » vaut
//     0,15), symbole ou code monétaire devant ou derrière ($, €, £, CHF, EUR,
//     USD).
func TextNumber(s string, loc Locale) (float64, bool) {
	s = strings.TrimSpace(s)
	neg, paren := false, false
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		neg, paren = true, true
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	signs, pct, money := 0, false, false
	for {
		s = strings.TrimFunc(s, isNumSpace)
		if s == "" {
			return 0, false
		}
		if c := s[0]; c == '-' || c == '+' {
			signs++
			neg = neg != (c == '-')
			s = s[1:]
			continue
		}
		if c := s[len(s)-1]; c == '-' || c == '+' {
			signs++
			neg = neg != (c == '-')
			s = s[:len(s)-1]
			continue
		}
		if !pct && strings.HasSuffix(s, "%") {
			pct = true
			s = s[:len(s)-1]
			continue
		}
		if !money {
			if rest, ok := trimCurrency(s); ok {
				money = true
				s = rest
				continue
			}
		}
		break
	}
	if signs > 1 || signs == 1 && paren {
		return 0, false
	}
	mant, exp := s, ""
	if k := strings.IndexAny(s, "eE"); k >= 0 {
		mant, exp = s[:k], s[k+1:]
		e := strings.TrimPrefix(strings.TrimPrefix(exp, "+"), "-")
		if e == "" || !allDigits(e) {
			return 0, false
		}
	}
	// Espaces et apostrophes : séparateurs de milliers, retirés.
	mant = strings.Map(func(r rune) rune {
		if isNumSpace(r) || r == '\'' || r == '’' {
			return -1
		}
		return r
	}, mant)
	num, ok := mantissa(mant, loc)
	if !ok {
		return 0, false
	}
	if exp != "" {
		num += "e" + exp
	}
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, false
	}
	if pct {
		f /= 100
	}
	if neg {
		f = -f
	}
	return f, true
}

// mantissa réécrit des chiffres séparés par des virgules et des points en
// nombre lisible par strconv (point décimal), ou false.
func mantissa(s string, loc Locale) (string, bool) {
	for _, r := range s {
		if (r < '0' || r > '9') && r != ',' && r != '.' {
			return "", false
		}
	}
	commas, dots := strings.Count(s, ","), strings.Count(s, ".")
	switch {
	case commas == 0 && dots == 0:
		return s, s != ""
	case commas > 0 && dots > 0:
		last := max(strings.LastIndex(s, ","), strings.LastIndex(s, "."))
		dec, thou := s[last], byte(',')
		if dec == ',' {
			thou = '.'
		}
		if strings.Count(s, string(dec)) != 1 {
			return "", false
		}
		whole, ok := thousands(s[:last], thou)
		return whole + "." + s[last+1:], ok
	}
	sep := byte(',')
	if dots > 0 {
		sep = '.'
	}
	if commas+dots > 1 {
		return thousands(s, sep)
	}
	k := strings.IndexByte(s, sep)
	whole, frac := s[:k], s[k+1:]
	if whole == "" && frac == "" {
		return "", false
	}
	ambiguous := len(frac) == 3 && len(whole) >= 1 && len(whole) <= 3 && whole[0] != '0'
	if ambiguous && rune(sep) != loc.decimal() {
		return whole + frac, true
	}
	return whole + "." + frac, true
}

// thousands retire les séparateurs de milliers sep d'une partie entière :
// 1 à 3 chiffres, puis des groupes de 3.
func thousands(s string, sep byte) (string, bool) {
	parts := strings.Split(s, string(sep))
	for i, p := range parts {
		if p == "" || len(p) > 3 || i > 0 && len(p) != 3 {
			return "", false
		}
	}
	return strings.Join(parts, ""), true
}

var currencies = []string{"$", "€", "£", "CHF", "EUR", "USD"}

// trimCurrency retire un symbole ou un code monétaire en tête ou en fin.
func trimCurrency(s string) (string, bool) {
	up := strings.ToUpper(s)
	for _, c := range currencies {
		if strings.HasPrefix(up, c) {
			return s[len(c):], true
		}
		if strings.HasSuffix(up, c) {
			return s[:len(s)-len(c)], true
		}
	}
	return s, false
}

func isNumSpace(r rune) bool {
	return r == ' ' || r == ' ' || r == ' ' || r == ' ' || r == '\t'
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// numRule dit comment une fonction numérique lit les textes d'une plage :
// convertis selon loc (colonnes de tableau), ou ignorés comme dans Excel
// (cells : plage de cellules libres, dont les saisies numériques sont déjà
// des nombres et dont les textes sont des libellés).
type numRule struct {
	loc   Locale
	cells bool
}

// coerce donne la valeur qu'une fonction numérique lit : un texte non vide
// d'une colonne devient le nombre qu'il écrit, ou 0. Les autres valeurs
// restent telles quelles.
func (r numRule) coerce(v Value) Value {
	if v.Kind != Text || r.cells || strings.TrimFunc(v.Str, isNumSpace) == "" {
		return v
	}
	f, _ := TextNumber(v.Str, r.loc)
	return Num(f)
}

// key ajoute la règle au nom d'une fonction transmis à SQLite (slt_part,
// slt_final, slt_scan…) : « SUM » (français, colonnes), « SUM;e »
// (anglais), « SUM;c » (cellules), « SUM;ec ».
func (r numRule) key(name string) string {
	flags := ""
	if r.loc.Dot {
		flags += "e"
	}
	if r.cells {
		flags += "c"
	}
	if flags == "" {
		return name
	}
	return name + ";" + flags
}

// splitKey sépare le nom et la règle écrits par key.
func splitKey(s string) (string, numRule) {
	name, flags, _ := strings.Cut(s, ";")
	var r numRule
	if strings.Contains(flags, "e") {
		r.loc = EN
	}
	r.cells = strings.Contains(flags, "c")
	return name, r
}

// ruled : scanner qui lit des nombres et reçoit la règle de sa plage.
type ruled interface{ setRule(numRule) }

func (r *numRule) setRule(n numRule) { *r = n }

// newScanner construit le scanner d'une fonction de parcours avec sa règle.
func newScanner(def scanDef, params []Value, r numRule) scanner {
	s := def.make(params)
	if x, ok := s.(ruled); ok {
		x.setRule(r)
	}
	return s
}
