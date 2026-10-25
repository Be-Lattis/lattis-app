package numfmt

import (
	"strings"
	"unicode"
)

// frenchColors : nom français des couleurs d'un code ([Red] : [Rouge]).
var frenchColors = map[string]string{
	"black": "Noir", "blue": "Bleu", "cyan": "Cyan", "green": "Vert",
	"magenta": "Magenta", "red": "Rouge", "white": "Blanc", "yellow": "Jaune",
}

// FromExcel écrit en notation française un code lu dans un fichier .xlsx
// (point décimal, virgule des milliers, dd/mm/yyyy, [Red], General) :
// l'inverse de ToExcel.
func FromExcel(code string) string {
	secs := splitSections(code)
	for i, s := range secs {
		secs[i] = frenchSection(s)
	}
	return strings.Join(secs, ";")
}

func frenchSection(code string) string {
	sec := parseSection(code)
	rs, _ := markGeneral([]rune(code))
	sk := string(skeleton(rs))
	frac := !sec.date && fracRe.MatchString(sk)
	var b strings.Builder
	seenDec := false
	ph := func(i int) bool { return i >= 0 && i < len(rs) && isDigitPh(rs[i]) }
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\x02':
			b.WriteString("Standard")
		case r == '"':
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				j++
			}
			b.WriteString(string(rs[i:min(j+1, len(rs))]))
			i = j
		case r == '\\' || r == '_' || r == '*':
			b.WriteRune(r)
			if i+1 < len(rs) {
				i++
				b.WriteRune(rs[i])
			}
		case r == '[':
			j := i + 1
			for j < len(rs) && rs[j] != ']' {
				j++
			}
			in := string(rs[i+1 : min(j, len(rs))])
			low := strings.ToLower(strings.TrimSpace(in))
			switch {
			case frenchColors[low] != "":
				in = frenchColors[low]
			case strings.HasPrefix(low, "color"):
				in = "Couleur" + strings.TrimSpace(low[len("color"):])
			case strings.HasPrefix(low, "<") || strings.HasPrefix(low, ">") || strings.HasPrefix(low, "="):
				in = strings.Replace(in, ".", ",", 1)
			}
			b.WriteString("[" + in + "]")
			i = j
		case sec.date:
			switch c := dateLetter(r); {
			case strings.HasPrefix(strings.ToUpper(string(rs[i:min(i+5, len(rs))])), "AM/PM"):
				b.WriteString("AM/PM")
				i += 4
			case strings.HasPrefix(strings.ToUpper(string(rs[i:min(i+3, len(rs))])), "A/P"):
				b.WriteString("A/P")
				i += 2
			case c == 'j':
				b.WriteRune('j')
			case c == 'a':
				b.WriteRune('a')
			case c != 0:
				b.WriteRune(unicode.ToLower(r))
			case r == '.' && ph(i+1):
				b.WriteRune(',')
			default:
				b.WriteRune(r)
			}
		case r == '.' && !seenDec:
			seenDec = true
			b.WriteRune(',')
		case r == '.':
			b.WriteString(`\.`)
		case r == ',' && ph(i-1) && ph(i+1) && !seenDec:
			b.WriteRune(' ')
		case r == ',':
			b.WriteString(`\,`)
		case isSpace(r) && ph(i-1) && ph(i+1) && !frac:
			b.WriteString(`\ `) // espace littérale (sinon lue comme séparateur des milliers)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
