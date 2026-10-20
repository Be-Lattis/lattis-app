package formula

import (
	"regexp"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/msg"
)

// capsWord : un mot en capitales (SUMIF, VAR.P, TRUE) dans un texte traduit.
var capsWord = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:\.[A-Z0-9]+)*\b`)

// TestDocsTranslated : chaque fonction documentée en français l'est dans
// chaque langue, avec autant d'arguments, des noms d'arguments utilisables
// dans une signature (le dernier groupe répété finissant par « 1 »), des
// marques {fn:…} et {=…} valides, aucun nom de fonction, VRAI/FAUX ou code
// d'erreur écrit en dur (ils suivent la langue des noms de la session), et
// un exemple en anglais qui utilise la fonction et se réécrit dans chaque
// locale.
func TestDocsTranslated(t *testing.T) {
	banned := map[string]bool{"TRUE": true, "FALSE": true}
	for _, f := range funcs {
		banned[f.EN] = true
		banned[f.FR] = true
	}
	locales := []Locale{FR, EN, ForLanguage("de"), ForLanguage("es")}
	checkText := func(lang msg.Lang, fn, what, s string) {
		for _, m := range docMark.FindAllStringSubmatch(s, -1) {
			inner := m[1]
			if name, ok := strings.CutPrefix(inner, "fn:"); ok {
				if g, ok := funcIndex[name]; !ok || g.EN != name {
					t.Errorf("%s %s %s : {fn:%s} ne nomme pas une fonction par son nom anglais", lang, fn, what, name)
				}
			} else if _, err := Parse(inner[1:], EN); err != nil {
				t.Errorf("%s %s %s : %s ne s'analyse pas en anglais : %v", lang, fn, what, m[0], err)
			}
		}
		rest := docMark.ReplaceAllString(s, "")
		for _, w := range capsWord.FindAllString(rest, -1) {
			if banned[w] {
				t.Errorf("%s %s %s : %q écrit en dur (utilisez {fn:…} ou {=…}) dans %q", lang, fn, what, w, s)
			}
		}
		if strings.Contains(rest, "#") && regexp.MustCompile(`#[A-Z]`).MatchString(rest) {
			t.Errorf("%s %s %s : code d'erreur écrit en dur (utilisez {=#VALUE!}) dans %q", lang, fn, what, s)
		}
		if strings.TrimSpace(s) != s || s == "" {
			t.Errorf("%s %s %s : texte vide ou mal coupé (%q)", lang, fn, what, s)
		}
	}
	for _, lang := range msg.Langs {
		if lang == msg.FR {
			continue
		}
		set := docSets[lang]
		if set == nil {
			t.Errorf("%s : pas de documentation des fonctions", lang)
			continue
		}
		for _, c := range []string{catLogic, catMath, catText, catDate, catAgg, catRows, catLookup, catSpill, catGeo, catFin} {
			if set.cats[c] == "" {
				t.Errorf("%s : catégorie %q non traduite", lang, c)
			}
		}
		for en, fr := range docs {
			d, ok := set.funcs[en]
			if !ok {
				t.Errorf("%s : %s non documentée", lang, en)
				continue
			}
			if len(d.args) != len(fr.args) {
				t.Errorf("%s %s : %d arguments, %d en français", lang, en, len(d.args), len(fr.args))
				continue
			}
			checkText(lang, en, "description", d.desc)
			if !strings.HasSuffix(d.desc, ".") && !strings.HasSuffix(d.desc, "。") {
				t.Errorf("%s %s : la description ne finit pas par un point (%q)", lang, en, d.desc)
			}
			seen := map[string]bool{}
			for i, a := range d.args {
				if a.name == "" || strings.ContainsAny(a.name, " ;,[]()") || seen[a.name] {
					t.Errorf("%s %s : nom de l'argument %d mal formé ou en double (%q)", lang, en, i, a.name)
				}
				seen[a.name] = true
				checkText(lang, en, a.name, a.desc)
				if i >= len(d.args)-fr.repeat && strings.HasSuffix(fr.args[i].Name, "1") != strings.HasSuffix(a.name, "1") {
					t.Errorf("%s %s : l'argument répété %s doit finir par 1 comme %s", lang, en, a.name, fr.args[i].Name)
				}
			}
			if en == "LET" && !strings.HasSuffix(d.args[0].name, "1") {
				t.Errorf("%s LET : %s doit finir par 1", lang, d.args[0].name)
			}
			if d.example == "" {
				continue
			}
			if !strings.HasPrefix(d.example, "=") || strings.Contains(d.example, "[@") {
				t.Errorf("%s %s : exemple %q sans = ou avec @", lang, en, d.example)
			}
			n, err := Parse(d.example, EN)
			if err != nil {
				t.Errorf("%s %s : exemple %q : %v", lang, en, d.example, err)
				continue
			}
			if !strings.Contains(strings.ToUpper(Format(n, EN)), en+"(") {
				t.Errorf("%s %s : l'exemple %q n'utilise pas %s", lang, en, d.example, en)
			}
			for _, loc := range locales {
				if _, err := Translate(d.example, EN, loc); err != nil {
					t.Errorf("%s %s : l'exemple %q ne se réécrit pas : %v", lang, en, d.example, err)
				}
			}
		}
		for en := range set.funcs {
			if _, ok := docs[en]; !ok {
				t.Errorf("%s : documentation de %s, inconnue en français", lang, en)
			}
		}
	}
}

// TestHelpCatalog : le catalogue de l'aide dans une langue garde le genre
// des arguments et le caractère facultatif du français, écrit la signature
// avec les noms traduits et les noms de fonctions de la locale.
func TestHelpCatalog(t *testing.T) {
	find := func(l []FunctionInfo, name string) FunctionInfo {
		for _, f := range l {
			if f.Name == name {
				return f
			}
		}
		t.Fatalf("%s absente du catalogue", name)
		return FunctionInfo{}
	}
	en := HelpCatalog(EN, msg.EN)
	sumif := find(en, "SUMIF")
	if sumif.Signature != "SUMIF(range, criteria, [sum_range])" || sumif.Category != "Aggregates" ||
		!strings.HasPrefix(sumif.Description, "Adds") || sumif.Args[0].Kind != "column" || !sumif.Args[2].Optional {
		t.Errorf("SUMIF en anglais : %+v", sumif)
	}
	if s := find(en, "SUM").Signature; s != "SUM(value1, [value2], …)" {
		t.Errorf("signature de SUM : %q", s)
	}
	if s := find(en, "LET").Signature; s != "LET(name1, value1, [name2, value2], …, calculation)" {
		t.Errorf("signature de LET : %q", s)
	}
	if f := find(en, "FILTER"); f.Args[0].Kind != "table" || f.Example != `=FILTER(Sales, Sales[Qty]>2, "none")` {
		t.Errorf("FILTER en anglais : %+v", f)
	}
	if d := find(en, "IF").Args[2].Desc; !strings.Contains(d, "(FALSE if omitted)") {
		t.Errorf("SI, valeur_si_faux : %q", d)
	}
	// Locale française, textes allemands : noms et exemples en français.
	de := HelpCatalog(FR, msg.DE)
	st := find(de, "SOUS.TOTAL")
	if !strings.Contains(st.Args[0].Desc, "9 SOMME") || !strings.Contains(st.Example, "SOUS.TOTAL(9; ") {
		t.Errorf("SOUS.TOTAL en allemand, locale française : %+v", st)
	}
	if d := find(de, "SI").Args[2].Desc; !strings.Contains(d, "FAUX") {
		t.Errorf("SI en allemand, locale française : %q", d)
	}
	// Français : le catalogue de toujours.
	fr := HelpCatalog(FR, msg.FR)
	if f := find(fr, "SOMME.SI"); f.Signature != "SOMME.SI(plage; critère; [plage_somme])" || f.Category != catAgg {
		t.Errorf("SOMME.SI en français : %+v", f)
	}
}
