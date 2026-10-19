package formula

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Les dates sont des numéros de série, comme dans Excel : le nombre de jours
// depuis le 30/12/1899, l'heure en partie décimale. Le 1er janvier 2026 vaut
// 46023. Excel compte un 29 février 1900 qui n'a pas existé : les numéros
// coïncident à partir du 1er mars 1900, ce qui couvre tous les usages.

// timeT abrège time.Time dans la table des fonctions.
type timeT = time.Time

var epoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// maxSerial est le 31/12/9999, dernière date d'Excel.
const maxSerial = 2958465

// TimeSerial donne le numéro de série d'une date et heure (heure murale,
// sans fuseau).
func TimeSerial(t time.Time) float64 {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	days := math.Round(day.Sub(epoch).Hours() / 24)
	secs := t.Hour()*3600 + t.Minute()*60 + t.Second()
	return days + float64(secs)/86400 + float64(t.Nanosecond())/86400e9
}

// SerialTime convertit un numéro de série en date et heure (UTC), à la
// seconde près.
func SerialTime(f float64) time.Time {
	days := math.Floor(f)
	secs := math.Round((f - days) * 86400)
	return epoch.AddDate(0, 0, int(days)).Add(time.Duration(secs) * time.Second)
}

func validSerial(f float64) bool { return f >= 0 && f < maxSerial+1 }

// dateNumber est number() pour un résultat de type date.
func dateNumber(f float64) Value {
	if !validSerial(f) {
		return Err(ErrNum)
	}
	return DateSerial(f)
}

// FormatDate affiche une date en locale française : 15/10/2026, et l'heure
// si elle n'est pas minuit (15/10/2026 09:30, secondes si besoin).
func FormatDate(f float64) string {
	if !validSerial(f) {
		return FormatNumber(f)
	}
	t := SerialTime(f)
	s := t.Format("02/01/2006")
	switch {
	case t.Second() != 0:
		s += t.Format(" 15:04:05")
	case t.Hour() != 0 || t.Minute() != 0:
		s += t.Format(" 15:04")
	}
	return s
}

// ISODate écrit une date comme elle est stockée dans une colonne de type date.
func ISODate(f float64) string { return SerialTime(f).Format("2006-01-02") }

var dateLayouts = []string{
	"2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02 15:04",
	"02/01/2006", "2/1/2006", "02/01/2006 15:04", "2/1/2006 15:04", "02/01/2006 15:04:05",
}

// ParseDate lit une date écrite en texte : ISO (stockage des colonnes date)
// ou jj/mm/aaaa (saisie française).
func ParseDate(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return TimeSerial(t), true
		}
	}
	return 0, false
}

// toSerial lit l'argument d'une fonction de date : une date, un nombre (numéro
// de série) ou un texte de date.
func toSerial(v Value) (float64, Value) {
	switch v.Kind {
	case Date, Number:
		if !validSerial(v.Num) {
			return 0, Err(ErrNum)
		}
		return v.Num, Value{}
	case Empty:
		return 0, Value{}
	case Text:
		if f, ok := ParseDate(v.Str); ok {
			return f, Value{}
		}
		if f, ok := parseNumber(v.Str); ok && validSerial(f) {
			return f, Value{}
		}
		return 0, Err(ErrValue)
	case Error:
		return 0, v
	}
	return 0, Err(ErrValue)
}

func dateArg(v Value) (time.Time, Value) {
	f, e := toSerial(v)
	if e.IsErr() {
		return time.Time{}, e
	}
	return SerialTime(f), Value{}
}

// intArgs convertit des arguments numériques et les tronque.
func intArgs(a []Value) ([]int, Value) {
	out := make([]int, len(a))
	for i, v := range a {
		f, e := ToNumber(v)
		if e.IsErr() {
			return nil, e
		}
		out[i] = int(math.Trunc(f))
	}
	return out, Value{}
}

func fnDate(a []Value) Value {
	n, e := intArgs(a)
	if e.IsErr() {
		return e
	}
	y := n[0]
	if y >= 0 && y < 1900 {
		y += 1900 // DATE(26;1;1) est en 1926, comme dans Excel
	}
	if y < 0 || y > 9999 {
		return Err(ErrNum)
	}
	t := time.Date(y, time.Month(1), 1, 0, 0, 0, 0, time.UTC).AddDate(0, n[1]-1, n[2]-1)
	return dateNumber(TimeSerial(t))
}

// datePart extrait une composante d'une date.
func datePart(f func(time.Time) int) func([]Value) Value {
	return func(a []Value) Value {
		t, e := dateArg(a[0])
		if e.IsErr() {
			return e
		}
		return Num(float64(f(t)))
	}
}

// fnWeekday : JOURSEM(date; [type]), mêmes numérotations qu'Excel.
func fnWeekday(a []Value) Value {
	t, e := dateArg(a[0])
	if e.IsErr() {
		return e
	}
	mode := 1
	if len(a) > 1 && !a[1].IsEmpty() {
		m, e := ToNumber(a[1])
		if e.IsErr() {
			return e
		}
		mode = int(m)
	}
	wd := int(t.Weekday()) // dimanche = 0
	var first, base int    // premier jour de la semaine (0 = dimanche) et numéro qu'il reçoit
	switch {
	case mode == 1 || mode == 17:
		first, base = 0, 1
	case mode == 2:
		first, base = 1, 1
	case mode == 3:
		first, base = 1, 0
	case mode >= 11 && mode <= 16:
		first, base = (mode-10)%7, 1
	default:
		return Err(ErrNum)
	}
	return Num(float64((wd-first+7)%7 + base))
}

func fnISOWeek(a []Value) Value {
	t, e := dateArg(a[0])
	if e.IsErr() {
		return e
	}
	_, w := t.ISOWeek()
	return Num(float64(w))
}

// addMonths ajoute des mois en restant dans le mois visé (31/01 + 1 mois =
// 28 ou 29/02), comme MOIS.DECALER.
func addMonths(t time.Time, m int) time.Time {
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, m, 0)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(t.Day(), last)-1)
}

func monthsArgs(a []Value) (time.Time, int, Value) {
	t, e := dateArg(a[0])
	if e.IsErr() {
		return t, 0, e
	}
	m, e := ToNumber(a[1])
	if e.IsErr() {
		return t, 0, e
	}
	return t, int(math.Trunc(m)), Value{}
}

func fnEDate(a []Value) Value {
	t, m, e := monthsArgs(a)
	if e.IsErr() {
		return e
	}
	return dateNumber(TimeSerial(addMonths(t, m)))
}

func fnEOMonth(a []Value) Value {
	t, m, e := monthsArgs(a)
	if e.IsErr() {
		return e
	}
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, m+1, -1)
	return dateNumber(TimeSerial(first))
}

// fnDatedif : DATEDIF(début; fin; unité), unités françaises et anglaises.
func fnDatedif(a []Value) Value {
	t1, e := dateArg(a[0])
	if e.IsErr() {
		return e
	}
	t2, e := dateArg(a[1])
	if e.IsErr() {
		return e
	}
	u, e := ToText(a[2])
	if e.IsErr() {
		return e
	}
	t1 = t1.Truncate(24 * time.Hour)
	t2 = t2.Truncate(24 * time.Hour)
	if t2.Before(t1) {
		return Err(ErrNum)
	}
	months := (t2.Year()-t1.Year())*12 + int(t2.Month()-t1.Month())
	if t2.Day() < t1.Day() {
		months--
	}
	switch strings.ToLower(u) {
	case "j", "d":
		return Num(math.Round(t2.Sub(t1).Hours() / 24))
	case "m":
		return Num(float64(months))
	case "a", "y":
		return Num(float64(months / 12))
	case "mj", "md":
		start := addMonths(t1, months)
		return Num(math.Round(t2.Sub(start).Hours() / 24))
	case "am", "ym":
		return Num(float64(months % 12))
	case "aj", "yd":
		start := addMonths(t1, months/12*12)
		return Num(math.Round(t2.Sub(start).Hours() / 24))
	}
	return Err(ErrNum)
}

func fnHourPart(div, mod float64) func([]Value) Value {
	return func(a []Value) Value {
		f, e := toSerial(a[0])
		if e.IsErr() {
			return e
		}
		secs := math.Round((f - math.Floor(f)) * 86400)
		return Num(math.Mod(math.Floor(secs/div), mod))
	}
}

// --- Jours ouvrés -------------------------------------------------------------

// workdays regroupe les jours fériés et les jours de week-end (indice 0 :
// lundi) de NB.JOURS.OUVRES et SERIE.JOUR.OUVRE.
type workdays struct {
	holidays map[int]bool
	off      [7]bool
}

func newWorkdaySet() workdays {
	return workdays{holidays: map[int]bool{}, off: [7]bool{5: true, 6: true}}
}

// weekendCodes : jours de week-end des codes numériques des formes .INTL.
var weekendCodes = map[int][]int{
	1: {5, 6}, 2: {6, 0}, 3: {0, 1}, 4: {1, 2}, 5: {2, 3}, 6: {3, 4}, 7: {4, 5},
	11: {6}, 12: {0}, 13: {1}, 14: {2}, 15: {3}, 16: {4}, 17: {5},
}

// weekend lit le week-end des formes .INTL : un code (1 samedi et dimanche,
// 11 dimanche seul…) ou sept chiffres 0/1 du lundi au dimanche ("0000011").
func (w *workdays) weekend(v Value) Value {
	var off [7]bool
	switch {
	case v.IsEmpty():
		return Value{}
	case v.IsErr():
		return v
	case v.Kind == Text && len(v.Str) == 7 && strings.Trim(v.Str, "01") == "":
		if v.Str == "1111111" {
			return Err(ErrValue) // aucun jour ouvré
		}
		for i, c := range v.Str {
			off[i] = c == '1'
		}
	default:
		f, e := ToNumber(v)
		if e.IsErr() {
			return e
		}
		days, ok := weekendCodes[int(f)]
		if !ok || f != math.Trunc(f) {
			return Err(ErrNum)
		}
		for _, d := range days {
			off[d] = true
		}
	}
	w.off = off
	return Value{}
}

func (w workdays) add(v Value) Value {
	if v.IsEmpty() {
		return Value{}
	}
	f, e := toSerial(v)
	if e.IsErr() {
		return e
	}
	w.holidays[int(f)] = true
	return Value{}
}

func (w workdays) open(day int) bool {
	wd := (day + 5) % 7 // le jour 0 (30/12/1899) est un samedi ; 0 = lundi
	return !w.off[wd] && !w.holidays[day]
}

// networkdays compte les jours ouvrés entre deux dates incluses.
func (w workdays) networkdays(a, b Value) Value {
	x, e := toSerial(a)
	if e.IsErr() {
		return e
	}
	y, e := toSerial(b)
	if e.IsErr() {
		return e
	}
	from, to, sign := int(x), int(y), 1.0
	if from > to {
		from, to, sign = to, from, -1
	}
	n := 0
	for d := from; d <= to; d++ {
		if w.open(d) {
			n++
		}
	}
	return Num(sign * float64(n))
}

// workday avance de n jours ouvrés.
func (w workdays) workday(a, b Value) Value {
	x, e := toSerial(a)
	if e.IsErr() {
		return e
	}
	nf, e := ToNumber(b)
	if e.IsErr() {
		return e
	}
	d, n := int(x), int(math.Trunc(nf))
	step := 1
	if n < 0 {
		step, n = -1, -n
	}
	for n > 0 {
		d += step
		if !validSerial(float64(d)) {
			return Err(ErrNum)
		}
		if w.open(d) {
			n--
		}
	}
	return DateSerial(float64(d))
}

// --- Noms des jours et des mois (fonction TEXTE) -----------------------------

var (
	frDays        = []string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"}
	frDaysShort   = []string{"dim.", "lun.", "mar.", "mer.", "jeu.", "ven.", "sam."}
	frMonths      = []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	frMonthsShort = []string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."}
)

func pad(n, w int) string { return fmt.Sprintf("%0*d", w, n) }
