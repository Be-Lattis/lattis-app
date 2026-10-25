package numfmt

import (
	"strings"
	"unicode"
)

// englishColors : nom anglais des couleurs (codes d'un fichier .xlsx).
var englishColors = map[string]string{
	"#000000": "Black", "#0000FF": "Blue", "#00FFFF": "Cyan", "#00FF00": "Green",
	"#FF00FF": "Magenta", "#FF0000": "Red", "#FFFFFF": "White", "#FFFF00": "Yellow",
}

// ToExcel écrit un code avec les séparateurs et les lettres anglais d'un
// fichier .xlsx (point décimal, virgule des milliers, dd/mm/yyyy, [Red],
// General), comme il s'enregistre quelle que soit la langue.
func ToExcel(code string) string {
	secs := splitSections(code)
	for i, s := range secs {
		secs[i] = excelSection(s)
	}
	return strings.Join(secs, ";")
}

func excelSection(code string) string {
	sec := parseSection(code)
	rs, _ := markGeneral([]rune(code))
	sk := string(skeleton(rs))
	en := !sec.date && (strings.Contains(sk, ".") || strings.Contains(sk, "#,#"))
	frac := !sec.date && fracRe.MatchString(sk)
	var b strings.Builder
	seenDec := false
	ph := func(i int) bool { return i >= 0 && i < len(rs) && isDigitPh(rs[i]) }
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\x02':
			b.WriteString("General")
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
			if c, ok := colors[low]; ok {
				in = englishColors[c]
			} else {
				for _, p := range []string{"couleur", "farbe"} {
					if strings.HasPrefix(low, p) {
						in = "Color" + strings.TrimSpace(low[len(p):])
					}
				}
				if strings.HasPrefix(low, "<") || strings.HasPrefix(low, ">") || strings.HasPrefix(low, "=") {
					in = strings.Replace(in, ",", ".", 1)
				}
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
				b.WriteRune('d')
			case c == 'a':
				b.WriteRune('y')
			case c != 0:
				b.WriteRune(unicode.ToLower(r))
			case r == ',' && ph(i+1):
				b.WriteRune('.')
			default:
				b.WriteString(excelLiteral(r))
			}
		case en:
			if r == '.' {
				seenDec = true
			}
			b.WriteRune(r)
		case r == ',' && !seenDec:
			seenDec = true
			b.WriteRune('.')
		case r == ',':
			b.WriteString(`\,`)
		case isSpace(r) && ph(i-1) && ph(i+1) && !seenDec && !frac:
			b.WriteRune(',')
		case r == '.':
			b.WriteString(`\.`)
		default:
			b.WriteString(excelLiteral(r))
		}
	}
	return b.String()
}

// excelLiteral écrit un caractère littéral (échappé s'il n'est pas de ceux
// qu'un code admet tels quels).
func excelLiteral(r rune) string {
	switch {
	case isSpace(r):
		return " "
	case strings.ContainsRune("$-+/():!^&'~{}<>=0123456789#?%@,.Ee€£¥", r):
		return string(r)
	}
	return `\` + string(r)
}
