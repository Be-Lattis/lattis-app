package grid

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Dates saisies : jj/mm/aaaa et ses formes courtes, comme Excel. Séparateur
// « / », « - » ou « . » ; année sur deux chiffres (00 à 29 : 2000 à 2029,
// 30 à 99 : 1930 à 1999, règle d'Excel) ; sans année, l'année en cours ;
// ISO aaaa-mm-jj ; mois en lettres en français ou en anglais (« 19 oct.
// 2026 », « 19 octobre », « Oct 19, 2026 »).

// Les dates gardées (cellules libres, valeurs des règles) s'écrivent à la
// française : ParseDate lit jour puis mois. La saisie dans une autre langue
// (m/j/aaaa en anglais) passe par DateStyle (datestyle.go).

var (
	dateNumRe  = regexp.MustCompile(`^(\d{1,4})[/.\-](\d{1,2})(?:[/.\-](\d{1,4}))?$`)
	dateWordRe = regexp.MustCompile(`^(\d{1,2})(?:er)?\s+([\p{L}]+)\.?(?:\s+(\d{2,4}))?$`)
	dateUSRe   = regexp.MustCompile(`^([\p{L}]+)\.?\s+(\d{1,2})(?:st|nd|rd|th)?,?(?:\s+(\d{2,4}))?$`)
)

// months : noms et abréviations des mois (français, anglais), sans accents.
var months = map[string]time.Month{}

func init() {
	names := [][]string{
		{"janvier", "janv", "jan", "january"},
		{"fevrier", "fevr", "fev", "february", "feb"},
		{"mars", "mar", "march"},
		{"avril", "avr", "april", "apr"},
		{"mai", "may"},
		{"juin", "june", "jun"},
		{"juillet", "juil", "july", "jul"},
		{"aout", "august", "aug"},
		{"septembre", "sept", "sep", "september"},
		{"octobre", "oct", "october"},
		{"novembre", "nov", "november"},
		{"decembre", "dec", "december"},
	}
	for i, ns := range names {
		for _, n := range ns {
			months[n] = time.Month(i + 1)
		}
	}
}

// monthOf lit un nom de mois (casse et accents ignorés).
func monthOf(s string) (time.Month, bool) {
	s = strings.NewReplacer("é", "e", "è", "e", "û", "u", "ê", "e").Replace(strings.ToLower(s))
	m, ok := months[s]
	return m, ok
}

// fullYear complète une année sur deux chiffres (règle d'Excel).
func fullYear(s string) (int, bool) {
	y, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	switch len(s) {
	case 2:
		if y < 30 {
			return 2000 + y, true
		}
		return 1900 + y, true
	case 4:
		return y, true
	}
	return 0, false
}

// mkDate compose une date ; faux si elle n'existe pas (31/02).
func mkDate(y int, m time.Month, d int) (time.Time, bool) {
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return time.Time{}, false
	}
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if t.Day() != d || t.Month() != m {
		return time.Time{}, false
	}
	return t, true
}

// ParseDate lit une date saisie (voir plus haut), jour puis mois.
func ParseDate(s string) (time.Time, bool) { return parseDate(s, false) }

// ParseDateIn lit une date tapée dans l'écriture st (mois puis jour en
// anglais).
func ParseDateIn(s string, st DateStyle) (time.Time, bool) { return parseDate(s, st.monthFirst()) }

func parseDate(s string, monthFirst bool) (time.Time, bool) {
	s = strings.TrimSpace(s)
	year := func(g string) (int, bool) {
		if g == "" {
			return time.Now().Year(), true
		}
		return fullYear(g)
	}
	if m := dateNumRe.FindStringSubmatch(s); m != nil {
		// aaaa-mm-jj (ou aaaa/mm/jj).
		if len(m[1]) == 4 {
			if len(m[3]) == 0 || len(m[3]) > 2 {
				return time.Time{}, false
			}
			y, _ := strconv.Atoi(m[1])
			mo, _ := strconv.Atoi(m[2])
			d, _ := strconv.Atoi(m[3])
			return mkDate(y, time.Month(mo), d)
		}
		if len(m[1]) > 2 {
			return time.Time{}, false
		}
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		// « 19.10 » sans année ressemble trop à un nombre : refusé.
		if m[3] == "" && strings.Contains(s, ".") {
			return time.Time{}, false
		}
		y, ok := year(m[3])
		if !ok {
			return time.Time{}, false
		}
		if monthFirst {
			a, b = b, a
		}
		return mkDate(y, time.Month(b), a)
	}
	if m := dateWordRe.FindStringSubmatch(s); m != nil {
		mo, ok := monthOf(m[2])
		if !ok {
			return time.Time{}, false
		}
		d, _ := strconv.Atoi(m[1])
		y, ok := year(m[3])
		if !ok {
			return time.Time{}, false
		}
		return mkDate(y, mo, d)
	}
	if m := dateUSRe.FindStringSubmatch(s); m != nil {
		mo, ok := monthOf(m[1])
		if !ok {
			return time.Time{}, false
		}
		d, _ := strconv.Atoi(m[2])
		y, ok := year(m[3])
		if !ok {
			return time.Time{}, false
		}
		return mkDate(y, mo, d)
	}
	return time.Time{}, false
}

// ParseFullDate est ParseDate avec une année écrite (« 19/10/26 »,
// « 19 oct. 2026 ») : un texte comme « 3-4 » reste du texte dans une
// formule.
func ParseFullDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, re := range []*regexp.Regexp{dateNumRe, dateWordRe, dateUSRe} {
		if m := re.FindStringSubmatch(s); m != nil && m[3] == "" && !(re == dateNumRe && len(m[1]) == 4) {
			return time.Time{}, false
		}
	}
	return ParseDate(s)
}
