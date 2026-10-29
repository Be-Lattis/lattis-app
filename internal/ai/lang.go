package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// Langue de l'interface et formules de l'assistant. Les commandes de
// internal/api travaillent sur la forme gardée par le classeur (formules en
// français : canvas.StorageForm), quelle que soit la langue du poste. Pour
// que l'assistant écrive les formules comme l'utilisateur les voit dans sa
// grille (SUM(a, b) et 1.2 en anglais), la conversion se fait ici, au bord
// des outils : les formules reçues (textes qui commencent par « = ») passent
// de la langue de la session au français, celles que rendent les outils du
// français à la langue de la session. Une session en français ne convertit
// rien. Une formule illisible dans la langue de la session mais lisible en
// français est acceptée telle quelle (le modèle a écrit en français).

// rawKeys : paramètres dont le texte n'est jamais une formule.
var rawKeys = map[string]bool{"sql": true, "html": true, "css": true, "js": true, "script": true, "title": true}

// formulasIn réécrit en français les formules de v (paramètres d'un outil),
// écrites pour loc ; une formule illisible rend l'erreur d'analyse.
func formulasIn(v any, loc formula.Locale) (any, error) {
	if loc == formula.FR {
		return v, nil
	}
	switch x := v.(type) {
	case string:
		if !strings.HasPrefix(strings.TrimSpace(x), "=") {
			return x, nil
		}
		out, err := formula.Translate(x, loc, formula.FR)
		if err == nil {
			return out, nil
		}
		if _, ferr := formula.Parse(x, formula.FR); ferr == nil {
			return x, nil
		}
		return x, fmt.Errorf("formule %s illisible : %w", x, err)
	case map[string]any:
		for k, e := range x {
			if rawKeys[k] {
				continue
			}
			t, err := formulasIn(e, loc)
			if err != nil {
				return v, err
			}
			x[k] = t
		}
	case []any:
		for i, e := range x {
			t, err := formulasIn(e, loc)
			if err != nil {
				return v, err
			}
			x[i] = t
		}
	}
	return v, nil
}

// formulasOut réécrit pour loc les formules d'un résultat d'outil (JSON) ;
// un texte qui n'est pas du JSON reste tel quel.
func formulasOut(s string, loc formula.Locale) string {
	if loc == formula.FR || !strings.Contains(s, "=") {
		return s
	}
	t := strings.TrimSpace(s)
	if t == "" || (t[0] != '{' && t[0] != '[') {
		return s
	}
	dec := json.NewDecoder(strings.NewReader(t))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil {
		return s
	}
	v = translateOut(v, loc)
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if enc.Encode(v) != nil {
		return s
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func translateOut(v any, loc formula.Locale) any {
	switch x := v.(type) {
	case string:
		return formulaOut(x, loc)
	case map[string]any:
		for k, e := range x {
			x[k] = translateOut(e, loc)
		}
	case []any:
		for i, e := range x {
			x[i] = translateOut(e, loc)
		}
	}
	return v
}

// formulaOut : une formule gardée (français) écrite pour loc ; un autre
// texte, ou une formule illisible, tel quel.
func formulaOut(s string, loc formula.Locale) string {
	if loc == formula.FR || !strings.HasPrefix(strings.TrimSpace(s), "=") {
		return s
	}
	if out, err := formula.Translate(s, formula.FR, loc); err == nil {
		return out
	}
	return s
}

// syntaxHint rappelle l'écriture des formules pour loc.
func syntaxHint(loc formula.Locale) string {
	if loc.Dot {
		return "écriture des formules de cette session : arguments séparés par « , », décimale avec un point (1.2), noms anglais des fonctions (SUM, IF, ROUND), [Colonne] pour la valeur de la ligne"
	}
	return fmt.Sprintf("écriture des formules de cette session : arguments séparés par « ; », décimale avec une virgule (1,2), noms %s des fonctions (%s), [Colonne] pour la valeur de la ligne",
		namesLang(loc), strings.Join([]string{formula.LocalName("SUM", loc), formula.LocalName("IF", loc), formula.LocalName("ROUND", loc)}, ", "))
}

// namesLang : la langue des noms de fonctions de loc.
func namesLang(loc formula.Locale) string {
	switch loc.Names {
	case formula.NamesEN:
		return "anglais"
	case formula.NamesDE:
		return "allemands"
	case formula.NamesES:
		return "espagnols"
	}
	return "français"
}

// langName : le nom d'une langue de l'interface, pour la consigne.
func langName(lang string) string {
	switch strings.ToLower(lang) {
	case "en":
		return "anglais"
	case "de":
		return "allemand"
	case "es":
		return "espagnol"
	case "zh":
		return "chinois"
	}
	return "français"
}
