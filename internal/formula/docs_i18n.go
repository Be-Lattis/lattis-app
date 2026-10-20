package formula

import (
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// Documentation des fonctions dans les autres langues que le français
// (docs/i18n.md, § 5) : docs_en.go, docs_de.go, docs_es.go et docs_zh.go
// traduisent docs.go fonction par fonction, par nom anglais. Le français
// reste la référence : catégorie, arguments facultatifs, groupe répété et
// genre des arguments (argKind) viennent de docs.go ; une langue ne donne
// que les textes. Une fonction absente d'une langue garde son texte
// français (TestDocsTranslated vérifie qu'il n'en manque aucune).
//
// Les textes traduits ne nomment pas les fonctions en dur : {fn:SUMIF}
// s'écrit dans la langue des noms de la session (SOMME.SI, SUMIF…), et
// {=…} est un bout de formule écrit en anglais ({=FALSE}, {=#VALUE!},
// {=RANK([Column])}) réécrit pour la locale comme les exemples : FALSE ou
// FAUX, « ; » ou « , ». Les exemples traduits sont écrits en anglais
// (SUMIF(a, b), décimale « . ») avec des noms de colonnes de la langue ;
// HelpCatalog les réécrit pour la locale.

// docText : la documentation d'une fonction dans une langue.
type docText struct {
	desc    string
	args    []argTr // autant que dans docs.go, dans le même ordre
	example string  // formule en anglais ; vide : l'exemple français
}

// argTr : nom et description d'un argument dans une langue. Le nom d'un
// argument répété finit par « 1 », comme en français.
type argTr struct{ name, desc string }

// docSet : la documentation d'une langue.
type docSet struct {
	cats  map[string]string // catégorie française → nom dans la langue
	funcs map[string]docText
}

// ta : un argument traduit ; tdx : une fonction traduite, avec son exemple.
func ta(name, desc string) argTr { return argTr{name, desc} }
func tdx(desc, example string, args ...argTr) docText {
	return docText{desc: desc, args: args, example: example}
}

// docSets : la documentation de chaque langue autre que le français.
var docSets = map[msg.Lang]*docSet{msg.EN: &docsEN, msg.DE: &docsDE, msg.ES: &docsES, msg.ZH: &docsZH}

// HelpCatalog : Catalog avec les descriptions, les noms et descriptions des
// arguments, les catégories et les exemples dans la langue lang (le
// français à défaut).
func HelpCatalog(loc Locale, lang msg.Lang) []FunctionInfo {
	out := Catalog(loc, lang)
	localizeDocs(out, loc, lang)
	return out
}

// localizeDocs traduit la documentation de out (rendu par Catalog pour loc)
// dans la langue lang ; rien en français ou dans une langue sans
// documentation.
func localizeDocs(out []FunctionInfo, loc Locale, lang msg.Lang) {
	set := docSets[lang]
	if set == nil {
		return
	}
	byName := map[string]string{}
	for _, f := range funcs {
		byName[f.name(loc)] = f.EN
	}
	sep := string(loc.sep())
	for i := range out {
		info := &out[i]
		en := byName[info.Name]
		fr, ok := docs[en]
		if !ok {
			continue
		}
		if c := set.cats[fr.cat]; c != "" {
			info.Category = c
		}
		t, ok := set.funcs[en]
		if !ok || len(t.args) != len(fr.args) {
			continue
		}
		info.Description = expandDoc(t.desc, loc)
		d := funcDoc{args: make([]ArgDoc, len(fr.args)), repeat: fr.repeat}
		for j, a := range t.args {
			d.args[j] = ArgDoc{Name: a.name, Desc: expandDoc(a.desc, loc), Optional: fr.args[j].Optional}
			if j < len(info.Args) {
				info.Args[j].Name, info.Args[j].Desc = d.args[j].Name, d.args[j].Desc
			}
		}
		if en == "LET" {
			n, v, c := d.args[0].Name, d.args[1].Name, d.args[2].Name
			info.Signature = info.Name + "(" + strings.Join([]string{n, v, "[" + numbered(n, 2) + sep + " " + numbered(v, 2) + "]", "…", c}, sep+" ") + ")"
		} else {
			info.Signature = d.signatureIn(info.Name, sep)
		}
		if t.example != "" {
			if s, err := Translate(t.example, EN, loc); err == nil {
				info.Example = s
			}
		}
	}
}

// docMark : {fn:NOM} et {=formule} dans un texte traduit.
var docMark = regexp.MustCompile(`\{(fn:[A-Z0-9.]+|=[^{}]+)\}`)

// expandDoc écrit les noms de fonctions ({fn:SUMIF}) et les bouts de formule
// ({=FALSE}) d'un texte traduit dans la locale loc.
func expandDoc(s string, loc Locale) string {
	if !strings.Contains(s, "{") {
		return s
	}
	return docMark.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[1 : len(m)-1]
		if name, ok := strings.CutPrefix(inner, "fn:"); ok {
			return LocalName(name, loc)
		}
		expr := inner[1:]
		if out, err := Translate(expr, EN, loc); err == nil {
			return out
		}
		return expr
	})
}
