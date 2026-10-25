package transfer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// Formules des règles conditionnelles et des validations : une formule
// d'Excel (anglais, adresses A1 relatives à la cellule en haut à gauche de
// la plage) s'écrit en français pour Lattis, et l'inverse à l'export. Les
// adresses gardent leurs $ (la règle les décale d'une cellule à l'autre),
// que l'analyse des formules retire : elles sont mises de côté le temps de
// la traduction.

var (
	a1Token  = regexp.MustCompile(`(?:(?:'(?:[^']|'')+'|[\p{L}_][\p{L}\w.]*)!)?\$?[A-Za-z]{1,3}\$?[0-9]+(?::\$?[A-Za-z]{1,3}\$?[0-9]+)?`)
	strToken = regexp.MustCompile(`"(?:[^"]|"")*"`)
)

// translateRule traduit une formule (avec ou sans =) d'une langue à l'autre ;
// faux si elle ne se lit pas. Le résultat commence par =.
func translateRule(src string, from, to formula.Locale) (string, bool) {
	src = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(src), "="))
	if src == "" {
		return "", false
	}
	// Adresses mises de côté (hors textes entre guillemets).
	var refs []string
	var b strings.Builder
	last := 0
	strs := strToken.FindAllStringIndex(src, -1)
	inStr := func(i int) bool {
		for _, s := range strs {
			if i >= s[0] && i < s[1] {
				return true
			}
		}
		return false
	}
	for _, m := range a1Token.FindAllStringIndex(src, -1) {
		if inStr(m[0]) || m[0] > 0 && isIdent(src[m[0]-1]) || m[1] < len(src) && (isIdent(src[m[1]]) || src[m[1]] == '(') {
			continue
		}
		b.WriteString(src[last:m[0]])
		fmt.Fprintf(&b, "xlref_%d_", len(refs))
		refs = append(refs, src[m[0]:m[1]])
		last = m[1]
	}
	b.WriteString(src[last:])
	n, err := formula.Parse("="+b.String(), from)
	if err != nil {
		return "", false
	}
	out := formula.Format(n, to)
	for i := len(refs) - 1; i >= 0; i-- {
		ph := fmt.Sprintf("xlref_%d_", i)
		if !strings.Contains(out, ph) {
			return "", false
		}
		out = strings.ReplaceAll(out, ph, refs[i])
	}
	if !strings.HasPrefix(out, "=") {
		out = "=" + out
	}
	return out, true
}

func isIdent(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '$' || c >= 0x80
}
