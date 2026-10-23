package grid

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DateOrder : ordre du jour, du mois et de l'année d'une date numérique.
type DateOrder uint8

const (
	DMY DateOrder = iota // 19/10/2026
	MDY                  // 10/19/2026
	YMD                  // 2026/10/19
)

// DateStyle : écriture des dates numériques dans une langue de l'interface,
// à la saisie et à l'affichage (docs/i18n.md, § 3) : jj/mm/aaaa en français
// et en espagnol, jj.mm.aaaa en allemand, mm/jj/aaaa en anglais, aaaa/mm/jj
// en chinois. Le classeur garde ses dates à la française (FrenchDates) ou
// en ISO (colonnes Date) : Input et Show passent d'une écriture à l'autre.
type DateStyle struct {
	Order DateOrder
	Sep   string
}

// FrenchDates : l'écriture des dates gardées dans le classeur.
var FrenchDates = DateStyle{DMY, "/"}

// DateStyleFor : l'écriture des dates d'une langue (fr, en, de, es, zh) ;
// une langue inconnue écrit à la française.
func DateStyleFor(lang string) DateStyle {
	switch strings.ToLower(lang) {
	case "en":
		return DateStyle{MDY, "/"}
	case "zh":
		return DateStyle{YMD, "/"}
	case "de":
		return DateStyle{DMY, "."}
	}
	return FrenchDates
}

// monthFirst : une date qui ne commence pas par l'année se lit mois puis
// jour (anglais ; chinois, dont l'ordre est année, mois, jour).
func (s DateStyle) monthFirst() bool { return s.Order != DMY }

var (
	// Date tapée sans l'année en tête : 10/19, 10/19/26, 10/19/2026 (l'heure
	// éventuelle, avant ou après, est séparée par splitClock).
	typedDateRe = regexp.MustCompile(`^(\d{1,2})([/.\-])(\d{1,2})(?:([/.\-])(\d{2}|\d{4}))?$`)
	// Date gardée à la française : 19/10/2026, 19/10/2026 09:30.
	frenchDateRe = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})/(\d{4})((?:\s+\d{1,2}:\d{2}(?::\d{2})?)?)$`)
)

// Input réécrit une date tapée dans l'écriture s en date lue par ParseDate
// (jour puis mois, mêmes séparateurs) ; un autre texte ne change pas, comme
// une date dont le « mois » dépasse 12 (lue alors jour puis mois). Une
// heure écrite avant ou après la date reste à sa place (« 10/23/2026 2:30
// PM » donne « 23/10/2026 2:30 PM », lu par ParseDateTime).
func (s DateStyle) Input(text string) string {
	if !s.monthFirst() {
		return text
	}
	date, pre, post := strings.TrimSpace(text), "", ""
	if d, clk, ok := splitClock(date); ok && !isoClockRe.MatchString(date) {
		if strings.HasPrefix(date, d) {
			post = " " + clk
		} else {
			pre = clk + " "
		}
		date = d
	}
	m := typedDateRe.FindStringSubmatch(date)
	if m == nil {
		return text
	}
	mo, _ := strconv.Atoi(m[1])
	d, _ := strconv.Atoi(m[3])
	if _, ok := mkDate(2000, time.Month(mo), d); !ok {
		return text
	}
	out := m[3] + m[2] + m[1]
	if m[5] != "" {
		out += m[4] + m[5]
	}
	return pre + out + post
}

// Show réécrit une date gardée à la française (jj/mm/aaaa, heure
// éventuelle) dans l'écriture s ; un autre texte ne change pas.
func (s DateStyle) Show(text string) string {
	if s == FrenchDates {
		return text
	}
	m := frenchDateRe.FindStringSubmatch(text)
	if m == nil {
		return text
	}
	return s.join(m[1], m[2], m[3]) + m[4]
}

// Format écrit une date dans l'écriture s (19/10/2026, 10/19/2026…).
func (s DateStyle) Format(t time.Time) string {
	return s.join(t.Format("02"), t.Format("01"), t.Format("2006"))
}

func (s DateStyle) join(d, m, y string) string {
	switch s.Order {
	case MDY:
		return m + s.Sep + d + s.Sep + y
	case YMD:
		return y + s.Sep + m + s.Sep + d
	}
	return d + s.Sep + m + s.Sep + y
}
