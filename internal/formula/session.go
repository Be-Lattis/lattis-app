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

// errNamesEN : codes d'erreur hors français, comme Excel en anglais.
var errNamesEN = map[ErrCode]string{
	ErrDiv0:  "#DIV/0!",
	ErrValue: "#VALUE!",
	ErrNA:    "#N/A",
	ErrRef:   "#REF!",
	ErrName:  "#NAME?",
	ErrNum:   "#NUM!",
	ErrCirc:  "#CIRC!",
}

// In : le code d'erreur dans la langue des noms de loc (#VALEUR! en
// français, #VALUE! ailleurs). Les deux formes sont acceptées à la saisie.
func (e ErrCode) In(loc Locale) string {
	if loc.french() {
		return errNames[e]
	}
	return errNamesEN[e]
}

// Codes d'erreur propres au moteur des cellules libres (tableaux
// dynamiques), affichés sans être des ErrCode.
const (
	spillFR = "#DEVERSEMENT!"
	spillEN = "#SPILL!"
)

// ErrorText réécrit un code d'erreur affiché en français (Value.String) dans
// la langue de loc ; un autre texte ne change pas.
func ErrorText(s string, loc Locale) string {
	if loc.french() || !strings.HasPrefix(s, "#") {
		return s
	}
	if s == spillFR {
		return spillEN
	}
	for code, name := range errNames {
		if name == s {
			return errNamesEN[code]
		}
	}
	return s
}

// ErrorInput réécrit un code d'erreur affiché dans une autre langue
// (#VALUE!, #SPILL!) en français ; un autre texte ne change pas.
func ErrorInput(s string) string {
	if !strings.HasPrefix(s, "#") {
		return s
	}
	if s == spillEN {
		return spillFR
	}
	for code, name := range errNamesEN {
		if name == s {
			return errNames[code]
		}
	}
	return s
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
	other := EN
	if !loc.french() {
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
