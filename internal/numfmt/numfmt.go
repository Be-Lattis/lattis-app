// Package numfmt applique un code de format de nombre comme un tableur :
// « # ##0,00 "kg" », « 0,00E+00 », « # ?/? », « [Rouge]-# ##0 », sections
// positif;négatif;zéro;texte, conditions [>100], dates « jj/mm/aaaa hh:mm »,
// durées [h]:mm, texte entre guillemets, caractères échappés (\x), espaces
// de la largeur d'un caractère (_x) et remplissage (*x, alignement de la
// comptabilité).
//
// Un code s'écrit avec les séparateurs français (virgule décimale, espace
// des milliers, jj/mm/aaaa) ou anglais (point décimal, virgule des milliers,
// dd/mm/yyyy) : les deux sont compris. Le résultat suit la langue donnée
// (Locale). Le même moteur existe côté interface (frontend/src/lib/grid/
// numcode.ts) : les deux suivent les cas de testdata/cases.json.
package numfmt

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Fill marque, dans Result.Text, la place du remplissage (*x) : il est
// suivi du caractère qui remplit. L'affichage pousse ce qui suit à droite
// (comptabilité : symbole à gauche, montant à droite) ; Plain le retire.
const Fill = "\x01"

// Result est un nombre mis en forme.
type Result struct {
	Text  string // texte affiché (Fill + caractère à la place d'un *x)
	Color string // couleur du code ([Rouge] : "#FF0000"), "" sans couleur
	OK    bool   // faux : valeur hors des dates permises (« ##### »)
}

// Plain retire du texte la marque de remplissage (le caractère reste une fois).
func Plain(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, Fill)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		s = s[i+1:]
	}
}

// Locale : séparateurs et noms des mois et des jours d'une langue.
type Locale struct {
	Decimal, Group      string
	Months, MonthsShort [12]string
	Days, DaysShort     [7]string // dimanche d'abord
}

var (
	FR = Locale{Decimal: ",", Group: " ",
		Months:      [12]string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"},
		MonthsShort: [12]string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."},
		Days:        [7]string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"},
		DaysShort:   [7]string{"dim.", "lun.", "mar.", "mer.", "jeu.", "ven.", "sam."}}
	EN = Locale{Decimal: ".", Group: ",",
		Months:      [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
		MonthsShort: [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		Days:        [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
		DaysShort:   [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}}
	DE = Locale{Decimal: ",", Group: ".",
		Months:      [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
		MonthsShort: [12]string{"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.", "Sept.", "Okt.", "Nov.", "Dez."},
		Days:        [7]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"},
		DaysShort:   [7]string{"So.", "Mo.", "Di.", "Mi.", "Do.", "Fr.", "Sa."}}
	ES = Locale{Decimal: ",", Group: ".",
		Months:      [12]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"},
		MonthsShort: [12]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sept", "oct", "nov", "dic"},
		Days:        [7]string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"},
		DaysShort:   [7]string{"dom", "lun", "mar", "mié", "jue", "vie", "sáb"}}
	ZH = Locale{Decimal: ".", Group: ",",
		Months:      [12]string{"一月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"},
		MonthsShort: [12]string{"1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
		Days:        [7]string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"},
		DaysShort:   [7]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}}
)

// ForLanguage donne la locale d'une langue de l'interface (fr par défaut).
func ForLanguage(lang string) Locale {
	switch strings.ToLower(lang) {
	case "en":
		return EN
	case "de":
		return DE
	case "es":
		return ES
	case "zh":
		return ZH
	}
	return FR
}

// --- Lecture du code ----------------------------------------------------------

type kind uint8

const (
	tLit     kind = iota // texte
	tDigit               // 0 # ?
	tDec                 // séparateur décimal
	tPercent             // %
	tExp                 // E+ E- (s : "E+", "e-"…)
	tSlash               // barre de fraction
	tDen                 // dénominateur fixe (s : chiffres)
	tText                // @
	tFill                // *x (s : x)
	tDate                // lettre de date (r : j m a h s, n : nombre)
	tAMPM                // AM/PM (s : "AM/PM" ou "A/P")
	tElapsed             // [h] [mm] [ss] (r : h m s)
	tGeneral             // Standard
)

type tok struct {
	k kind
	s string
	r rune
	n int
}

type cond struct {
	op string
	v  float64
}

func (c *cond) match(x float64) bool {
	switch c.op {
	case "<":
		return x < c.v
	case "<=":
		return x <= c.v
	case ">":
		return x > c.v
	case ">=":
		return x >= c.v
	case "=":
		return x == c.v
	case "<>":
		return x != c.v
	}
	return false
}

type section struct {
	toks    []tok
	color   string
	cond    *cond
	date    bool
	text    bool // contient @
	group   bool // séparateur de milliers
	scale   int  // virgules de mise à l'échelle (milliers)
	percent int
	digits  bool // contient des 0 # ?
	general bool
}

// Format est un code de format lu.
type Format struct {
	secs []section
}

// colors : couleurs des codes, dans les langues de l'interface.
var colors = map[string]string{
	"black": "#000000", "noir": "#000000", "schwarz": "#000000", "negro": "#000000",
	"blue": "#0000FF", "bleu": "#0000FF", "blau": "#0000FF", "azul": "#0000FF",
	"cyan": "#00FFFF", "zyan": "#00FFFF", "cian": "#00FFFF",
	"green": "#00FF00", "vert": "#00FF00", "grün": "#00FF00", "verde": "#00FF00",
	"magenta": "#FF00FF",
	"red":     "#FF0000", "rouge": "#FF0000", "rot": "#FF0000", "rojo": "#FF0000",
	"white": "#FFFFFF", "blanc": "#FFFFFF", "weiß": "#FFFFFF", "weiss": "#FFFFFF", "blanco": "#FFFFFF",
	"yellow": "#FFFF00", "jaune": "#FFFF00", "gelb": "#FFFF00", "amarillo": "#FFFF00",
}

// palette : [CouleurN] (les 16 premières couleurs de la palette classique).
var palette = []string{"#000000", "#FFFFFF", "#FF0000", "#00FF00", "#0000FF", "#FFFF00", "#FF00FF", "#00FFFF",
	"#800000", "#008000", "#000080", "#808000", "#800080", "#008080", "#C0C0C0", "#808080"}

var cache = struct {
	m map[string]*Format
}{m: map[string]*Format{}}

// Parse lit un code (jamais d'erreur : ce qui ne se lit pas est du texte).
func Parse(code string) *Format {
	if f, ok := cache.m[code]; ok {
		return f
	}
	f := &Format{}
	for _, s := range splitSections(code) {
		f.secs = append(f.secs, parseSection(s))
	}
	if len(cache.m) > 500 {
		cache.m = map[string]*Format{}
	}
	cache.m[code] = f
	return f
}

// splitSections coupe un code en sections (« ; » hors guillemets, \x et
// crochets).
func splitSections(code string) []string {
	var out []string
	rs := []rune(code)
	start := 0
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '"':
			for i++; i < len(rs) && rs[i] != '"'; i++ {
			}
		case '\\', '_', '*':
			i++
		case '[':
			for i++; i < len(rs) && rs[i] != ']'; i++ {
			}
		case ';':
			out = append(out, string(rs[start:i]))
			start = i + 1
		}
	}
	return append(out, string(rs[start:]))
}

var fracRe = regexp.MustCompile(`[0#?]\s*/\s*[0-9#?]`)

func isDigitPh(r rune) bool { return r == '0' || r == '#' || r == '?' }

func isSpace(r rune) bool { return r == ' ' || r == '\u00a0' || r == '\u202f' }

// dateLetter ramène une lettre de date (française ou anglaise) à son code.
func dateLetter(r rune) rune {
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

// skeleton rend le code d'une section sans texte entre guillemets, \x, _x,
// *x ni crochets (pour en deviner la nature).
func skeleton(rs []rune) []rune {
	var out []rune
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '"':
			for i++; i < len(rs) && rs[i] != '"'; i++ {
			}
		case '\\', '_', '*':
			i++
		case '[':
			for i++; i < len(rs) && rs[i] != ']'; i++ {
			}
		default:
			out = append(out, rs[i])
		}
	}
	return out
}

var generalWords = []string{"g/standard", "standard", "general", "estándar", "常规"}

// markGeneral remplace le mot Standard (General…) hors guillemets par \x02.
func markGeneral(rs []rune) ([]rune, bool) {
	low := []rune(strings.ToLower(string(rs)))
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '"':
			for i++; i < len(rs) && rs[i] != '"'; i++ {
			}
			continue
		case '\\', '_', '*':
			i++
			continue
		case '[':
			for i++; i < len(rs) && rs[i] != ']'; i++ {
			}
			continue
		}
		for _, w := range generalWords {
			wr := []rune(w)
			if i+len(wr) <= len(low) && string(low[i:i+len(wr)]) == w {
				out := append(append(append([]rune{}, rs[:i]...), '\x02'), rs[i+len(wr):]...)
				return out, true
			}
		}
	}
	return rs, false
}

func parseSection(code string) section {
	sec := section{}
	rs, gen := markGeneral([]rune(code))
	sec.general = gen
	sk := skeleton(rs)
	// Nature : date si des lettres de date viennent avant tout chiffre.
	for _, r := range sk {
		if isDigitPh(r) {
			sec.digits = true
			break
		}
		if dateLetter(r) != 0 {
			sec.date = true
			break
		}
	}
	if !sec.date {
		for _, r := range sk {
			if isDigitPh(r) {
				sec.digits = true
			}
		}
	}
	// Séparateurs anglais : point décimal, ou « #,## ».
	en := false
	if !sec.date {
		s := string(sk)
		en = strings.Contains(s, ".") || strings.Contains(s, "#,#")
	}
	dec := ','
	if en {
		dec = '.'
	}
	seenDec, seenExp := false, false
	// Fraction (# ?/?) : l'espace sépare la partie entière, pas les milliers.
	frac := !sec.date && fracRe.MatchString(string(sk))
	nextPh := func(i int) bool { // un 0 # ? suit-il (sans rien entre) ?
		return i+1 < len(rs) && isDigitPh(rs[i+1])
	}
	prevPh := func() bool {
		if n := len(sec.toks); n > 0 {
			return sec.toks[n-1].k == tDigit
		}
		return false
	}
	lit := func(s string) { sec.toks = append(sec.toks, tok{k: tLit, s: s}) }
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\x02':
			sec.toks = append(sec.toks, tok{k: tGeneral})
		case r == '"':
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				j++
			}
			lit(string(rs[i+1 : min(j, len(rs))]))
			i = j
		case r == '\\':
			if i+1 < len(rs) {
				i++
				lit(string(rs[i]))
			}
		case r == '_':
			if i+1 < len(rs) {
				i++
			}
			lit(" ")
		case r == '*':
			if i+1 < len(rs) {
				i++
				sec.toks = append(sec.toks, tok{k: tFill, s: string(rs[i])})
			}
		case r == '[':
			j := i + 1
			for j < len(rs) && rs[j] != ']' {
				j++
			}
			sec.bracket(string(rs[i+1 : min(j, len(rs))]))
			i = j
		case sec.date:
			if c := dateLetter(r); c != 0 {
				up := strings.ToUpper(string(rs[i:min(i+5, len(rs))]))
				if strings.HasPrefix(up, "AM/PM") {
					sec.toks = append(sec.toks, tok{k: tAMPM, s: "AM/PM"})
					i += 4
					continue
				}
				if strings.HasPrefix(up, "A/P") {
					sec.toks = append(sec.toks, tok{k: tAMPM, s: "A/P"})
					i += 2
					continue
				}
				n := 1
				for i+1 < len(rs) && dateLetter(rs[i+1]) == c {
					i++
					n++
				}
				sec.toks = append(sec.toks, tok{k: tDate, r: c, n: n})
				continue
			}
			if (r == '.' || r == ',') && nextPh(i) && rs[i+1] == '0' {
				// Fractions de seconde : ss,00 ou ss.00.
				j := i + 1
				for j < len(rs) && rs[j] == '0' {
					j++
				}
				sec.toks = append(sec.toks, tok{k: tDec, n: j - i - 1})
				i = j - 1
				continue
			}
			if r == '@' {
				sec.text = true
				sec.toks = append(sec.toks, tok{k: tText})
				continue
			}
			lit(string(r))
		case isDigitPh(r):
			sec.toks = append(sec.toks, tok{k: tDigit, r: r})
		case r == dec && !seenDec && !seenExp:
			seenDec = true
			sec.toks = append(sec.toks, tok{k: tDec})
		case en && r == ',' || !en && isSpace(r):
			switch {
			case prevPh() && nextPh(i) && !seenDec && !frac:
				sec.group = true
			case en && prevPh() && (i+1 == len(rs) || rs[i+1] == ',' || rs[i+1] == '.' || !isDigitPh(rs[i+1])):
				sec.scale++
			default:
				lit(string(r))
			}
		case r == '%':
			sec.percent++
			lit("%")
		case (r == 'E' || r == 'e') && i+1 < len(rs) && (rs[i+1] == '+' || rs[i+1] == '-') && sec.hasDigits():
			sec.toks = append(sec.toks, tok{k: tExp, s: string(rs[i : i+2])})
			seenExp = true
			i++
		case r == '/' && prevPh() && i+1 < len(rs) && (isDigitPh(rs[i+1]) || rs[i+1] >= '1' && rs[i+1] <= '9'):
			sec.toks = append(sec.toks, tok{k: tSlash})
			if rs[i+1] >= '1' && rs[i+1] <= '9' {
				j := i + 1
				for j < len(rs) && rs[j] >= '0' && rs[j] <= '9' {
					j++
				}
				sec.toks = append(sec.toks, tok{k: tDen, s: string(rs[i+1 : j])})
				i = j - 1
			}
		case r == '@':
			sec.text = true
			sec.toks = append(sec.toks, tok{k: tText})
		default:
			lit(string(r))
		}
	}
	return sec
}

func (s *section) hasDigits() bool {
	for _, t := range s.toks {
		if t.k == tDigit {
			return true
		}
	}
	return false
}

// bracket lit un [..] : couleur, condition, symbole monétaire, durée.
func (s *section) bracket(in string) {
	low := strings.ToLower(strings.TrimSpace(in))
	if c, ok := colors[low]; ok {
		s.color = c
		return
	}
	for _, p := range []string{"color", "couleur", "farbe"} {
		if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(low, p))); strings.HasPrefix(low, p) && err == nil {
			if n >= 1 && n <= len(palette) {
				s.color = palette[n-1]
			}
			return
		}
	}
	if strings.HasPrefix(in, "$") {
		sym, _, _ := strings.Cut(in[1:], "-")
		if sym != "" {
			s.toks = append(s.toks, tok{k: tLit, s: sym})
		}
		return
	}
	for _, op := range []string{"<=", ">=", "<>", "<", ">", "="} {
		if strings.HasPrefix(low, op) {
			v, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(low[len(op):]), ",", ".", 1), 64)
			if err == nil {
				s.cond = &cond{op: op, v: v}
			}
			return
		}
	}
	if low != "" {
		c := dateLetter(rune(low[0]))
		if (c == 'h' || c == 'm' || c == 's') && strings.Trim(low, string(low[0])) == "" {
			s.date = true
			s.toks = append(s.toks, tok{k: tElapsed, r: c, n: len(low)})
		}
	}
}

// IsDate dit si le code met en forme des dates ou des heures (première section).
func (f *Format) IsDate() bool { return len(f.secs) > 0 && f.secs[0].date }

// IsText dit si le code est un format texte (@ seul).
func (f *Format) IsText() bool {
	return len(f.secs) == 1 && f.secs[0].text && !f.secs[0].digits && !f.secs[0].date
}

// --- Mise en forme -------------------------------------------------------------

// Number met en forme un nombre.
func (f *Format) Number(x float64, loc Locale) Result {
	sec, neg := f.pick(x)
	if sec == nil {
		return Result{Text: general(x, loc), OK: true}
	}
	if sec.text && !sec.digits && !sec.date && !sec.general {
		// Section de texte seule (@) : le nombre en standard.
		return Result{Text: general(x, loc), Color: sec.color, OK: true}
	}
	v := x
	if neg {
		v = -x
	}
	var out string
	ok := true
	switch {
	case sec.date:
		out, ok = sec.formatDate(x, loc)
	default:
		out = sec.formatNumber(math.Abs(v), loc)
		if v < 0 && strings.ContainsAny(Plain(out), "123456789") {
			out = "-" + out
		}
	}
	return Result{Text: out, Color: sec.color, OK: ok}
}

// pick choisit la section d'un nombre ; neg : la section reçoit la valeur
// absolue sans signe (section des négatifs).
func (f *Format) pick(x float64) (*section, bool) {
	var secs []*section
	for i := range f.secs {
		if len(f.secs) > 1 && i == 3 {
			break // section du texte
		}
		secs = append(secs, &f.secs[i])
	}
	if len(secs) == 0 {
		return nil, false
	}
	conds := false
	for _, s := range secs {
		if s.cond != nil {
			conds = true
		}
	}
	if conds {
		for _, s := range secs {
			if s.cond != nil && s.cond.match(x) {
				return s, false
			}
		}
		for _, s := range secs {
			if s.cond == nil {
				return s, false
			}
		}
		return secs[len(secs)-1], false
	}
	switch {
	case x < 0 && len(secs) >= 2:
		return secs[1], true
	case x == 0 && len(secs) >= 3:
		return secs[2], false
	}
	return secs[0], false
}

// Text met en forme un texte : la section du texte (quatrième, ou @ seul),
// sinon le texte tel quel.
func (f *Format) Text(s string) Result {
	var sec *section
	switch {
	case len(f.secs) >= 4:
		sec = &f.secs[3]
	case f.secs[len(f.secs)-1].text:
		sec = &f.secs[len(f.secs)-1]
	}
	if sec == nil {
		return Result{Text: s, OK: true}
	}
	var b strings.Builder
	for _, t := range sec.toks {
		switch t.k {
		case tText:
			b.WriteString(s)
		case tFill:
			b.WriteString(Fill + t.s)
		case tLit:
			b.WriteString(t.s)
		}
	}
	return Result{Text: b.String(), Color: sec.color, OK: true}
}

// general : affichage standard (10 chiffres significatifs au plus).
func general(x float64, loc Locale) string {
	a := math.Abs(x)
	var s string
	if a != 0 && (a >= 1e11 || a < 1e-9) {
		s = strconv.FormatFloat(x, 'E', 5, 64)
		m, e, _ := strings.Cut(s, "E")
		if strings.Contains(m, ".") {
			m = strings.TrimRight(strings.TrimRight(m, "0"), ".")
		}
		sign := e[:1]
		ev := strings.TrimLeft(e[1:], "0")
		if len(ev) < 2 {
			ev = strings.Repeat("0", 2-len(ev)) + ev
		}
		s = m + "E" + sign + ev
	} else {
		s = strconv.FormatFloat(x, 'g', 10, 64)
		if strings.ContainsAny(s, "eE") {
			v, _ := strconv.ParseFloat(s, 64)
			s = strconv.FormatFloat(v, 'f', -1, 64)
		}
		if strings.Contains(s, ".") {
			s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		}
	}
	return strings.Replace(s, ".", loc.Decimal, 1)
}

// fixed écrit x ≥ 0 avec d décimales, arrondi au plus loin de zéro sur
// ses 15 chiffres significatifs (comme un tableur : 1,005 donne 1,01).
func fixed(x float64, d int) string {
	s := strconv.FormatFloat(x, 'g', 15, 64)
	if strings.ContainsAny(s, "eE") {
		v, _ := strconv.ParseFloat(s, 64)
		s = strconv.FormatFloat(v, 'f', -1, 64)
	}
	ip, fp, _ := strings.Cut(s, ".")
	if len(fp) <= d {
		return joinFixed(ip, fp+strings.Repeat("0", d-len(fp)))
	}
	up := fp[d] >= '5'
	digits := []byte(ip + fp[:d])
	if up {
		i := len(digits) - 1
		for ; i >= 0; i-- {
			if digits[i] == '9' {
				digits[i] = '0'
				continue
			}
			digits[i]++
			break
		}
		if i < 0 {
			digits = append([]byte{'1'}, digits...)
		}
	}
	n := len(digits) - d
	return joinFixed(string(digits[:n]), string(digits[n:]))
}

func joinFixed(ip, fp string) string {
	if ip == "" {
		ip = "0"
	}
	if fp == "" {
		return ip
	}
	return ip + "." + fp
}

func (s *section) formatNumber(x float64, loc Locale) string {
	x *= math.Pow(100, float64(s.percent))
	x /= math.Pow(1000, float64(s.scale))
	// Parties : entière, décimale, exposant, fraction.
	var intPh, fracPh, expPh []int
	decAt, expAt, slashAt := -1, -1, -1
	for i, t := range s.toks {
		switch t.k {
		case tDigit:
			switch {
			case expAt >= 0:
				expPh = append(expPh, i)
			case decAt >= 0:
				fracPh = append(fracPh, i)
			default:
				intPh = append(intPh, i)
			}
		case tDec:
			if decAt < 0 && expAt < 0 {
				decAt = i
			}
		case tExp:
			expAt = i
		case tSlash:
			slashAt = i
		}
	}
	out := make([]string, len(s.toks))
	if s.general {
		for i, t := range s.toks {
			if t.k == tGeneral {
				out[i] = general(x, loc)
			}
		}
		if !s.digits {
			return s.join(out, loc)
		}
	}
	if slashAt >= 0 && decAt < 0 && expAt < 0 {
		return s.formatFraction(x, loc, slashAt)
	}
	if expAt >= 0 {
		k := max(1, len(intPh))
		hash := false
		for _, i := range intPh {
			if s.toks[i].r == '#' {
				hash = true
			}
		}
		step := 1
		if hash && k > 1 {
			step = k
		}
		exp := 0
		if x != 0 {
			exp = int(math.Floor(math.Log10(x)))
			if step > 1 {
				exp = int(math.Floor(float64(exp)/float64(step))) * step
			} else {
				exp -= k - 1
			}
			m, _ := strconv.ParseFloat(fixed(x/math.Pow(10, float64(exp)), len(fracPh)), 64)
			if m >= math.Pow(10, float64(k)) {
				exp += step
			}
			x /= math.Pow(10, float64(exp))
		}
		sign := ""
		if exp < 0 {
			sign = "-"
		} else if strings.HasSuffix(s.toks[expAt].s, "+") {
			sign = "+"
		}
		out[expAt] = s.toks[expAt].s[:1] + sign
		s.fillInt(out, expPh, strconv.Itoa(abs(exp)), false, loc)
	}
	ip, fp, _ := strings.Cut(fixed(x, len(fracPh)), ".")
	if ip == "0" {
		ip = ""
	}
	s.fillInt(out, intPh, ip, s.group, loc)
	// Décimales : les # de la fin s'effacent, les ? deviennent des espaces.
	keep := len(fp)
	for keep > 0 && fp[keep-1] == '0' && s.toks[fracPh[keep-1]].r != '0' {
		keep--
	}
	for j, i := range fracPh {
		switch {
		case j < keep:
			out[i] = fp[j : j+1]
		case s.toks[i].r == '?':
			out[i] = " "
		}
	}
	if keep == 0 && len(fracPh) > 0 && out[fracPh[len(fracPh)-1]] == "" {
		out[decAt] = noDec // « 0,## » : 3 et non « 3, »
	}
	return s.join(out, loc)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// fillInt répartit les chiffres d'une partie entière sur ses 0 # ? (de
// droite à gauche ; le surplus va au premier).
func (s *section) fillInt(out []string, ph []int, digits string, group bool, loc Locale) {
	if len(ph) == 0 {
		return
	}
	if group {
		// Chiffres complétés par les 0 et ? obligatoires, puis groupés.
		var b []rune
		d := []rune(digits)
		for j := len(ph) - 1; j >= 0; j-- {
			k := len(d) - (len(ph) - j)
			switch {
			case k >= 0:
				b = append([]rune{d[k]}, b...)
			case s.toks[ph[j]].r == '0':
				b = append([]rune{'0'}, b...)
			case s.toks[ph[j]].r == '?':
				b = append([]rune{' '}, b...)
			}
		}
		if extra := len(d) - len(ph); extra > 0 {
			b = append(d[:extra:extra], b...)
		}
		var g strings.Builder
		n := 0
		for _, r := range b {
			if r >= '0' && r <= '9' {
				n++
			}
		}
		for _, r := range b {
			if r >= '0' && r <= '9' {
				g.WriteRune(r)
				n--
				if n > 0 && n%3 == 0 {
					g.WriteString(loc.Group)
				}
			} else {
				g.WriteRune(r)
			}
		}
		out[ph[0]] = g.String()
		return
	}
	d := digits
	for j := len(ph) - 1; j >= 0; j-- {
		i := ph[j]
		switch {
		case j == 0 && len(d) > 0:
			out[i] = d
		case len(d) > 0:
			out[i] = d[len(d)-1:]
			d = d[:len(d)-1]
		case s.toks[i].r == '0':
			out[i] = "0"
		case s.toks[i].r == '?':
			out[i] = " "
		}
	}
}

// noDec retire le séparateur décimal (aucune décimale affichée).
const noDec = ""

// join assemble le texte : littéraux, chiffres déjà placés (out).
func (s *section) join(out []string, loc Locale) string {
	var b strings.Builder
	for i, t := range s.toks {
		switch t.k {
		case tLit:
			b.WriteString(t.s)
		case tDec:
			if out[i] != noDec {
				b.WriteString(loc.Decimal)
			}
		case tFill:
			b.WriteString(Fill + t.s)
		case tDen:
			if out[i] != "" {
				b.WriteString(out[i])
			} else {
				b.WriteString(t.s)
			}
		default:
			b.WriteString(out[i])
		}
	}
	return b.String()
}

// formatFraction : # ?/?, ?/4, 0 00/100…
func (s *section) formatFraction(x float64, loc Locale, slash int) string {
	// Numérateur : les 0 # ? juste avant la barre ; partie entière : ceux
	// d'avant.
	numStart := slash
	for numStart > 0 && s.toks[numStart-1].k == tDigit {
		numStart--
	}
	var intPh, numPh, denPh []int
	fixedDen := 0
	for i, t := range s.toks {
		switch {
		case t.k == tDigit && i < numStart:
			intPh = append(intPh, i)
		case t.k == tDigit && i < slash:
			numPh = append(numPh, i)
		case t.k == tDigit:
			denPh = append(denPh, i)
		case t.k == tDen:
			fixedDen, _ = strconv.Atoi(t.s)
		}
	}
	whole := 0.0
	frac := x
	if len(intPh) > 0 {
		whole = math.Floor(x)
		frac = x - whole
	}
	var num, den int
	if fixedDen > 0 {
		den = fixedDen
		num = int(math.Round(frac * float64(den)))
	} else {
		num, den = approx(frac, int(math.Pow(10, float64(max(1, len(denPh)))))-1)
	}
	if len(intPh) > 0 && num == den {
		whole++
		num = 0
	}
	out := make([]string, len(s.toks))
	wholeDigits := ""
	if whole > 0 {
		wholeDigits = strconv.FormatFloat(whole, 'f', 0, 64)
	}
	blank := len(intPh) > 0 && num == 0
	if blank && wholeDigits == "" {
		wholeDigits = "0"
	}
	s.fillInt(out, intPh, wholeDigits, s.group, loc)
	if blank {
		// Nombre entier : la fraction laisse place à des espaces.
		for i := numStart; i < len(s.toks); i++ {
			switch t := s.toks[i]; t.k {
			case tDigit, tSlash:
				out[i] = " "
			case tDen:
				out[i] = strings.Repeat(" ", len(t.s))
			}
		}
		return s.join(out, loc)
	}
	s.fillInt(out, numPh, strconv.Itoa(num), false, loc)
	out[slash] = "/"
	// Dénominateur aligné à gauche.
	ds := strconv.Itoa(den)
	for j, i := range denPh {
		switch {
		case j < len(ds) && j == len(denPh)-1:
			out[i] = ds[j:]
		case j < len(ds):
			out[i] = ds[j : j+1]
		case s.toks[i].r == '0':
			out[i] = "0"
		case s.toks[i].r == '?':
			out[i] = " "
		}
	}
	return s.join(out, loc)
}

// approx : meilleure fraction num/den de x (0 ≤ x < 1 ou plus) avec un
// dénominateur au plus maxDen (fractions continues).
func approx(x float64, maxDen int) (int, int) {
	if maxDen < 1 {
		maxDen = 1
	}
	whole := math.Floor(x)
	f := x - whole
	// Meilleure approximation de f par recherche de Stern-Brocot.
	ln, ld, rn, rd := 0, 1, 1, 1
	bn, bd := 0, 1
	if f > 0.5 {
		bn, bd = 1, 1
	}
	best := math.Abs(f - float64(bn)/float64(bd))
	for {
		mn, md := ln+rn, ld+rd
		if md > maxDen {
			break
		}
		v := float64(mn) / float64(md)
		if d := math.Abs(f - v); d < best-1e-12 {
			best, bn, bd = d, mn, md
		}
		if f < v {
			rn, rd = mn, md
		} else if f > v {
			ln, ld = mn, md
		} else {
			break
		}
	}
	return bn + int(whole)*bd, bd
}

// --- Dates ---------------------------------------------------------------------

var epoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

func (s *section) formatDate(x float64, loc Locale) (string, bool) {
	if x < 0 || x >= 2958466 {
		return "", false
	}
	// Précision : la plus fine affichée (secondes ou fractions).
	fd := 0
	for _, t := range s.toks {
		if t.k == tDec {
			fd = max(fd, t.n)
		}
	}
	unit := math.Pow(10, float64(fd))
	ms := math.Round(x * 86400 * unit)
	days := math.Floor(ms / (86400 * unit))
	rest := ms - days*86400*unit
	tm := epoch.AddDate(0, 0, int(days)).Add(time.Duration(rest/unit*1e9) * time.Nanosecond)
	sub := int(math.Mod(rest, unit))
	ampm := false
	for _, t := range s.toks {
		if t.k == tAMPM {
			ampm = true
		}
	}
	var b strings.Builder
	for i, t := range s.toks {
		switch t.k {
		case tLit:
			b.WriteString(t.s)
		case tFill:
			b.WriteString(Fill + t.s)
		case tText:
		case tDec:
			b.WriteString(loc.Decimal)
			b.WriteString(pad(sub, t.n))
		case tAMPM:
			pm := tm.Hour() >= 12
			switch {
			case t.s == "A/P" && pm:
				b.WriteString("P")
			case t.s == "A/P":
				b.WriteString("A")
			case pm:
				b.WriteString("PM")
			default:
				b.WriteString("AM")
			}
		case tElapsed:
			total := ms / unit
			switch t.r {
			case 'h':
				b.WriteString(pad(int(math.Floor(total/3600)), t.n))
			case 'm':
				b.WriteString(pad(int(math.Floor(total/60)), t.n))
			default:
				b.WriteString(pad(int(math.Floor(total)), t.n))
			}
		case tDate:
			switch t.r {
			case 'j':
				switch {
				case t.n >= 4:
					b.WriteString(loc.Days[tm.Weekday()])
				case t.n == 3:
					b.WriteString(loc.DaysShort[tm.Weekday()])
				default:
					b.WriteString(pad(tm.Day(), t.n))
				}
			case 'm':
				if t.n <= 2 && minuteAt(s.toks, i) {
					b.WriteString(pad(tm.Minute(), t.n))
					continue
				}
				switch {
				case t.n >= 5:
					r, _ := utf8.DecodeRuneInString(loc.Months[tm.Month()-1])
					b.WriteString(strings.ToUpper(string(r)))
				case t.n == 4:
					b.WriteString(loc.Months[tm.Month()-1])
				case t.n == 3:
					b.WriteString(loc.MonthsShort[tm.Month()-1])
				default:
					b.WriteString(pad(int(tm.Month()), t.n))
				}
			case 'a':
				if t.n <= 2 {
					b.WriteString(pad(tm.Year()%100, 2))
				} else {
					b.WriteString(pad(tm.Year(), 4))
				}
			case 'h':
				h := tm.Hour()
				if ampm {
					h %= 12
					if h == 0 {
						h = 12
					}
				}
				b.WriteString(pad(h, min(t.n, 2)))
			case 's':
				b.WriteString(pad(tm.Second(), min(t.n, 2)))
			}
		}
	}
	return b.String(), true
}

// minuteAt : « m » ou « mm » désigne les minutes juste après des heures ou
// juste avant des secondes.
func minuteAt(toks []tok, i int) bool {
	for j := i - 1; j >= 0; j-- {
		if toks[j].k == tDate || toks[j].k == tElapsed {
			if toks[j].r == 'h' {
				return true
			}
			break
		}
	}
	for j := i + 1; j < len(toks); j++ {
		if toks[j].k == tDate || toks[j].k == tElapsed {
			return toks[j].r == 's'
		}
	}
	return false
}

func pad(n, w int) string {
	s := strconv.Itoa(n)
	if len(s) < w {
		s = strings.Repeat("0", w-len(s)) + s
	}
	return s
}
