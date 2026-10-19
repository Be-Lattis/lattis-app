package formula

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// fnText : TEXTE(valeur; format) avec les formats courants d'Excel.
//
//   - nombres : 0 (chiffre obligatoire), # (chiffre facultatif), virgule ou
//     point décimal, espace ou « #,## » pour les milliers, % ; sections
//     positif;négatif;zéro ; texte entre guillemets ;
//   - dates : jj/dd, jjj, jjjj, mm, mmm, mmmm, aa/yy, aaaa/yyyy, hh, mm
//     (minutes après hh ou avant ss), ss.
//
// Le résultat suit la locale française (virgule décimale, espace pour les
// milliers, noms des jours et des mois en français).
func fnText(a []Value) Value {
	v := a[0]
	f, e := ToText(a[1])
	if e.IsErr() {
		return e
	}
	if v.IsErr() {
		return v
	}
	if v.Kind == Bool {
		return Str(v.String())
	}
	x, e := ToNumber(v)
	if e.IsErr() {
		return v // un texte non numérique est rendu tel quel, comme dans Excel
	}
	sections := splitFormat(f)
	sec := sections[0]
	neg := x < 0
	switch {
	case x < 0 && len(sections) > 1:
		sec, x, neg = sections[1], -x, false
	case x == 0 && len(sections) > 2:
		sec = sections[2]
	}
	if isDateFormat(sec) {
		if !validSerial(x) {
			return Err(ErrValue)
		}
		return Str(formatDateCode(sec, x))
	}
	s := formatNumberCode(sec, math.Abs(x))
	if neg && strings.ContainsAny(s, "123456789") {
		s = "-" + s
	}
	return Str(s)
}

// splitFormat coupe un format en sections (séparées par ; hors guillemets).
func splitFormat(f string) []string {
	var out []string
	var cur strings.Builder
	quoted := false
	rs := []rune(f)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '"':
			quoted = !quoted
		case r == '\\' && !quoted && i+1 < len(rs):
			cur.WriteRune(r)
			i++
			r = rs[i]
		case r == ';' && !quoted:
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	return append(out, cur.String())
}

// fmtTok est un morceau de format : un littéral, ou une suite de lettres de
// date (« jj », « mmmm »…).
type fmtTok struct {
	lit   string
	code  rune // j, m, a, h, s (0 pour un littéral)
	count int
}

func tokenizeFormat(f string) []fmtTok {
	var toks []fmtTok
	rs := []rune(f)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '"':
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				j++
			}
			toks = append(toks, fmtTok{lit: string(rs[i+1 : min(j, len(rs))])})
			i = j
		case r == '\\' && i+1 < len(rs):
			i++
			toks = append(toks, fmtTok{lit: string(rs[i])})
		default:
			c := dateCode(r)
			if c == 0 {
				toks = append(toks, fmtTok{lit: string(r)})
				continue
			}
			n := 1
			for i+1 < len(rs) && dateCode(rs[i+1]) == c {
				i++
				n++
			}
			toks = append(toks, fmtTok{code: c, count: n})
		}
	}
	return toks
}

// dateCode ramène une lettre de format de date à son code (français ou
// anglais).
func dateCode(r rune) rune {
	switch unicode.ToLower(r) {
	case 'j', 'd':
		return 'j'
	case 'm':
		return 'm'
	case 'a', 'y':
		return 'a'
	case 'h':
		return 'h'
	case 's':
		return 's'
	}
	return 0
}

func isDateFormat(f string) bool {
	for _, t := range tokenizeFormat(f) {
		if t.code != 0 {
			return true
		}
	}
	return false
}

func formatDateCode(f string, x float64) string {
	t := SerialTime(x)
	toks := tokenizeFormat(f)
	var b strings.Builder
	for i, tk := range toks {
		if tk.code == 0 {
			b.WriteString(tk.lit)
			continue
		}
		switch tk.code {
		case 'j':
			switch {
			case tk.count >= 4:
				b.WriteString(frDays[t.Weekday()])
			case tk.count == 3:
				b.WriteString(frDaysShort[t.Weekday()])
			default:
				b.WriteString(pad(t.Day(), tk.count))
			}
		case 'm':
			if tk.count <= 2 && minuteContext(toks, i) {
				b.WriteString(pad(t.Minute(), tk.count))
				continue
			}
			switch {
			case tk.count >= 5:
				b.WriteString(strings.ToUpper(frMonths[t.Month()-1][:1]))
			case tk.count == 4:
				b.WriteString(frMonths[t.Month()-1])
			case tk.count == 3:
				b.WriteString(frMonthsShort[t.Month()-1])
			default:
				b.WriteString(pad(int(t.Month()), tk.count))
			}
		case 'a':
			if tk.count <= 2 {
				b.WriteString(pad(t.Year()%100, 2))
			} else {
				b.WriteString(pad(t.Year(), 4))
			}
		case 'h':
			b.WriteString(pad(t.Hour(), min(tk.count, 2)))
		case 's':
			b.WriteString(pad(t.Second(), min(tk.count, 2)))
		}
	}
	return b.String()
}

// minuteContext : « mm » désigne les minutes juste après des heures ou juste
// avant des secondes.
func minuteContext(toks []fmtTok, i int) bool {
	for j := i - 1; j >= 0; j-- {
		if toks[j].code != 0 {
			if toks[j].code == 'h' {
				return true
			}
			break
		}
	}
	for j := i + 1; j < len(toks); j++ {
		if toks[j].code != 0 {
			return toks[j].code == 's'
		}
	}
	return false
}

// formatNumberCode applique un format numérique à un nombre positif.
func formatNumberCode(f string, x float64) string {
	// Bloc numérique : du premier au dernier caractère 0 # ? (avec les
	// séparateurs entre eux) ; le reste est littéral.
	rs := []rune(f)
	quoted := false
	first, last := -1, -1
	percent := 0
	for i := 0; i < len(rs); i++ {
		switch r := rs[i]; {
		case r == '"':
			quoted = !quoted
		case quoted:
		case r == '\\':
			i++
		case r == '0' || r == '#' || r == '?':
			if first < 0 {
				first = i
			}
			last = i
		case r == '%':
			percent++
		}
	}
	x *= math.Pow(100, float64(percent))
	if first < 0 {
		return literal(f)
	}
	block := string(rs[first : last+1])
	enStyle := strings.Contains(block, ".") || strings.Contains(block, "#,##")
	dec := ","
	if enStyle {
		dec = "."
	}
	intPart, fracPart, _ := strings.Cut(block, dec)
	grouping := strings.ContainsAny(intPart, "  ") || enStyle && strings.Contains(intPart, ",")
	minInt := strings.Count(intPart, "0")
	minFrac := strings.Count(fracPart, "0")
	maxFrac := minFrac + strings.Count(fracPart, "#") + strings.Count(fracPart, "?")

	// Arrondi au plus loin de zéro, comme ARRONDI.
	r := fnRound([]Value{Num(x), Num(float64(maxFrac))})
	s := strconv.FormatFloat(r.Num, 'f', maxFrac, 64)
	ip, fp, _ := strings.Cut(s, ".")
	for len(fp) > minFrac && strings.HasSuffix(fp, "0") {
		fp = fp[:len(fp)-1]
	}
	ip = strings.TrimLeft(ip, "0")
	for len(ip) < minInt {
		ip = "0" + ip
	}
	if grouping {
		ip = groupThousands(ip)
	}
	num := ip
	if fp != "" {
		num += ","
		num += fp
	}
	return literal(string(rs[:first])) + num + literal(string(rs[last+1:]))
}

func groupThousands(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// literal rend la partie littérale d'un format (guillemets et \ retirés).
func literal(f string) string {
	var b strings.Builder
	for _, t := range tokenizeFormatLiteral(f) {
		b.WriteString(t)
	}
	return b.String()
}

func tokenizeFormatLiteral(f string) []string {
	var out []string
	rs := []rune(f)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '"':
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				j++
			}
			out = append(out, string(rs[i+1:min(j, len(rs))]))
			i = j
		case '\\':
			if i+1 < len(rs) {
				i++
				out = append(out, string(rs[i]))
			}
		default:
			out = append(out, string(rs[i]))
		}
	}
	return out
}
