package formula

import (
	"sort"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// Formules dans la langue de la session (docs/i18n.md, § 5) : l'utilisateur
// tape et lit ses formules dans sa langue (noms des fonctions, séparateur
// d'arguments, décimale, VRAI/FAUX, codes d'erreur), le classeur les garde
// dans une forme qui n'en dépend pas : français pour les cellules libres,
// les règles, les validations, les noms et les champs calculés, anglais
// (EN) pour les colonnes calculées. Translate passe de l'une à l'autre.

// errLang : codes d'erreur hors français, comme Excel dans chaque langue.
// #CIRC! (référence circulaire, propre à Lattis) ne change pas. Les codes
// de toutes les langues sont acceptés à la saisie (errAliases).
var errLang = map[NameLang]map[ErrCode]string{
	NamesEN: {
		ErrDiv0:  "#DIV/0!",
		ErrValue: "#VALUE!",
		ErrNA:    "#N/A",
		ErrRef:   "#REF!",
		ErrName:  "#NAME?",
		ErrNum:   "#NUM!",
		ErrCirc:  "#CIRC!",
	},
	NamesDE: {
		ErrDiv0:  "#DIV/0!",
		ErrValue: "#WERT!",
		ErrNA:    "#NV",
		ErrRef:   "#BEZUG!",
		ErrName:  "#NAME?",
		ErrNum:   "#ZAHL!",
		ErrCirc:  "#CIRC!",
	},
	NamesES: {
		ErrDiv0:  "#¡DIV/0!",
		ErrValue: "#¡VALOR!",
		ErrNA:    "#N/D",
		ErrRef:   "#¡REF!",
		ErrName:  "#¿NOMBRE?",
		ErrNum:   "#¡NUM!",
		ErrCirc:  "#CIRC!",
	},
}

// In : le code d'erreur dans la langue des noms de loc (#VALEUR! en
// français, #VALUE! en anglais, #WERT! en allemand, #¡VALOR! en espagnol).
// Toutes les formes sont acceptées à la saisie.
func (e ErrCode) In(loc Locale) string {
	if names, ok := errLang[loc.Names]; ok {
		return names[e]
	}
	return errNames[e]
}

// spillNames : le code du débordement d'un tableau dynamique (moteur des
// cellules libres), affiché sans être un ErrCode, dans chaque langue.
var spillNames = map[NameLang]string{
	NamesFR: "#DEVERSEMENT!",
	NamesEN: "#SPILL!",
	NamesDE: "#ÜBERLAUF!",
	NamesES: "#¡DESBORDAMIENTO!",
}

// ErrorText réécrit un code d'erreur affiché en français (Value.String) dans
// la langue de loc ; un autre texte ne change pas.
func ErrorText(s string, loc Locale) string {
	if loc.french() || !strings.HasPrefix(s, "#") {
		return s
	}
	if s == spillNames[NamesFR] {
		return spillNames[loc.Names]
	}
	for code, name := range errNames {
		if name == s {
			return code.In(loc)
		}
	}
	return s
}

// ErrorInput réécrit un code d'erreur affiché dans une autre langue
// (#VALUE!, #WERT!, #¡VALOR!, #SPILL!…) en français ; un autre texte ne
// change pas.
func ErrorInput(s string) string {
	if !strings.HasPrefix(s, "#") {
		return s
	}
	for _, spill := range spillNames {
		if s == spill {
			return spillNames[NamesFR]
		}
	}
	if code, ok := errAliases[s]; ok {
		return errNames[code]
	}
	return s
}

// boolNames : FAUX et VRAI dans chaque langue des noms, comme Excel.
var boolNames = map[NameLang][2]string{
	NamesFR: {"FAUX", "VRAI"},
	NamesEN: {"FALSE", "TRUE"},
	NamesDE: {"FALSCH", "WAHR"},
	NamesES: {"FALSO", "VERDADERO"},
}

// BoolText : VRAI ou FAUX dans la langue des noms de loc (TRUE, WAHR,
// VERDADERO…).
func BoolText(b bool, loc Locale) string {
	if b {
		return boolNames[loc.Names][1]
	}
	return boolNames[loc.Names][0]
}

// BoolWord lit un booléen écrit dans la langue de loc, ou en français ou en
// anglais (acceptés partout, comme avant) ; ok faux pour un autre mot.
// WAHR ne se lit qu'en allemand et VERDADERO qu'en espagnol : ailleurs, ce
// sont des noms comme les autres.
func BoolWord(word string, loc Locale) (v, ok bool) {
	for _, l := range []NameLang{loc.Names, NamesFR, NamesEN} {
		w := boolNames[l]
		switch {
		case strings.EqualFold(word, w[1]):
			return true, true
		case strings.EqualFold(word, w[0]):
			return false, true
		}
	}
	return false, false
}

// Translate réécrit la formule src, écrite dans la locale from, dans la
// locale to : noms des fonctions, séparateurs, décimale, VRAI/FAUX et codes
// d'erreur. Le « = » de tête est gardé s'il y était. Locales égales : src
// tel quel ; formule illisible dans from : src et l'erreur d'analyse.
func Translate(src string, from, to Locale) (string, error) {
	if from == to {
		return src, nil
	}
	n, err := Parse(src, from)
	if err != nil {
		return src, err
	}
	out := Format(n, to)
	if !strings.HasPrefix(strings.TrimSpace(src), "=") {
		out = strings.TrimPrefix(out, "=")
	}
	return out, nil
}

// Catalog liste les fonctions pour l'aide à la saisie dans la locale loc
// (noms, signatures avec son séparateur, exemples réécrits) et la langue
// lang (nombre d'arguments) ; les descriptions restent en français.
func Catalog(loc Locale, lang msg.Lang) []FunctionInfo {
	seen := map[string]bool{}
	var out []FunctionInfo
	// L'autre nom montré par l'aide : l'anglais, ou le français pour
	// l'anglais (et le chinois, qui prend les noms anglais).
	other := EN
	if loc.Names == NamesEN {
		other = FR
	}
	sep := string(loc.sep())
	for _, f := range funcIndex {
		if seen[f.EN] || refused[f.EN] {
			continue
		}
		seen[f.EN] = true
		d := docs[f.EN]
		as := make([]ArgDoc, 0, len(d.args))
		for _, a := range d.args {
			a.Kind = argKind(a)
			as = append(as, a)
		}
		example := d.example
		if s, err := Translate(example, FR, loc); err == nil {
			example = s
		}
		out = append(out, FunctionInfo{
			Name: f.name(loc), Other: f.name(other), Arity: f.arityMsg().Text(lang),
			Category: d.cat, Description: d.desc, Signature: d.signatureIn(f.name(loc), sep),
			Args: as, Repeat: d.repeat, Example: example,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
