package grid

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Dates avec heure (demande de Louis, 23/10/2026) : une date et une heure
// séparées d'une espace, dans un ordre ou dans l'autre (« 23/10/2026 14:30 »,
// « 14:30 23/10/2026 »), secondes facultatives, AM/PM (« 10/23/2026 2:30
// PM » en anglais), ISO avec T (« 2026-10-23T14:30:00 »). La date se lit
// comme ParseDate (toutes ses formes) ; une colonne Date garde la date et
// l'heure en ISO, « 2026-10-23 14:30 » (StoreDate).

// clock : une heure hh:mm, hh:mm:ss, avec AM/PM éventuel.
const clock = `\d{1,2}:\d{2}(?::\d{2})?(?:\s*[AaPp]\.?\s?[Mm]\.?)?`

var (
	clockRe         = regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::(\d{2}))?(?:\s*([AaPp])\.?\s?[Mm]\.?)?$`)
	dateThenClockRe = regexp.MustCompile(`^(.+?)\s+(` + clock + `)$`)
	clockThenDateRe = regexp.MustCompile(`^(` + clock + `)\s+(.+)$`)
	isoClockRe      = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}(?::\d{2})?)(?:\.\d+)?Z?$`)
	storedRe        = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})((?: \d{2}:\d{2}(?::\d{2})?)?)$`)
)

// parseClock lit une heure (voir clockRe) : heures, minutes, secondes.
// Avec AM/PM, l'heure va de 1 à 12 (12 AM : minuit, 12 PM : midi).
func parseClock(s string) (h, m, sec int, ok bool) {
	g := clockRe.FindStringSubmatch(strings.TrimSpace(s))
	if g == nil {
		return 0, 0, 0, false
	}
	h, _ = strconv.Atoi(g[1])
	m, _ = strconv.Atoi(g[2])
	if g[3] != "" {
		sec, _ = strconv.Atoi(g[3])
	}
	if m > 59 || sec > 59 {
		return 0, 0, 0, false
	}
	switch strings.ToLower(g[4]) {
	case "":
		if h > 23 {
			return 0, 0, 0, false
		}
	case "a", "p":
		if h < 1 || h > 12 {
			return 0, 0, 0, false
		}
		h %= 12
		if strings.EqualFold(g[4], "p") {
			h += 12
		}
	}
	return h, m, sec, true
}

// splitClock sépare une date et une heure écrites l'une après l'autre,
// dans un ordre ou dans l'autre ; faux sans heure.
func splitClock(s string) (date, clk string, ok bool) {
	if m := isoClockRe.FindStringSubmatch(s); m != nil {
		return m[1], m[2], true
	}
	if m := dateThenClockRe.FindStringSubmatch(s); m != nil {
		return m[1], m[2], true
	}
	if m := clockThenDateRe.FindStringSubmatch(s); m != nil {
		return m[2], m[1], true
	}
	return "", "", false
}

// ParseDateTime lit une date, avec ou sans heure (voir plus haut), jour
// puis mois ; withTime dit si une heure était écrite.
func ParseDateTime(s string) (t time.Time, withTime, ok bool) {
	return parseDateTime(s, false)
}

// ParseDateTimeIn est ParseDateTime dans l'écriture st (mois puis jour en
// anglais).
func ParseDateTimeIn(s string, st DateStyle) (t time.Time, withTime, ok bool) {
	return parseDateTime(s, st.monthFirst())
}

func parseDateTime(s string, monthFirst bool) (time.Time, bool, bool) {
	s = strings.TrimSpace(s)
	if d, ok := parseDate(s, monthFirst); ok {
		return d, false, true
	}
	if !strings.Contains(s, ":") {
		return time.Time{}, false, false
	}
	date, clk, ok := splitClock(s)
	if !ok {
		return time.Time{}, false, false
	}
	d, ok := parseDate(date, monthFirst)
	if !ok {
		return time.Time{}, false, false
	}
	h, m, sec, ok := parseClock(clk)
	if !ok {
		return time.Time{}, false, false
	}
	return d.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second), true, true
}

// StoreDate écrit une date comme la garde une colonne Date : aaaa-mm-jj,
// suivie de l'heure (hh:mm, hh:mm:ss si les secondes ne sont pas nulles)
// quand withTime.
func StoreDate(t time.Time, withTime bool) string {
	switch {
	case !withTime:
		return t.Format("2006-01-02")
	case t.Second() != 0:
		return t.Format("2006-01-02 15:04:05")
	}
	return t.Format("2006-01-02 15:04")
}

// FreeDate écrit une date comme la garde une cellule libre : jj/mm/aaaa,
// suivie de l'heure quand withTime (lue par formula.ParseDate).
func FreeDate(t time.Time, withTime bool) string {
	switch {
	case !withTime:
		return t.Format("02/01/2006")
	case t.Second() != 0:
		return t.Format("02/01/2006 15:04:05")
	}
	return t.Format("02/01/2006 15:04")
}

// ShowStored écrit une valeur d'une colonne Date (StoreDate) à la
// française : « 2026-10-23 14:30 » donne « 23/10/2026 14:30 » ; un autre
// texte ne change pas.
func ShowStored(s string) string {
	m := storedRe.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	return m[3] + "/" + m[2] + "/" + m[1] + m[4]
}

// StoredHasTime : la valeur d'une colonne Date porte une heure.
func StoredHasTime(s string) bool { return len(s) > 10 && storedRe.MatchString(s) }

// IsNull : le texte « null » (casse et espaces ignorés), saisi, collé ou
// importé, vaut une case vide (NULL) dans un tableau (demande de Louis,
// 23/10/2026).
func IsNull(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) == 4 && strings.EqualFold(s, "null")
}
