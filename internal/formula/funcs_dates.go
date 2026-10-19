package formula

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Fonctions de dates ajoutées : DATEVAL, TEMPSVAL, FRACTION.ANNEE (les
// formes .INTL des jours ouvrés sont dans dates.go et scan.go).

// frMonthIndex retrouve un mois écrit en toutes lettres ou abrégé (janvier,
// janv., févr…), sans tenir compte des accents ni de la casse.
func frMonthIndex(s string) (time.Month, bool) {
	s = strings.TrimSuffix(foldAccents(strings.ToLower(s)), ".")
	if len(s) < 3 {
		return 0, false
	}
	for i, m := range frMonths {
		if strings.HasPrefix(foldAccents(m), s) {
			return time.Month(i + 1), true
		}
	}
	return 0, false
}

func foldAccents(s string) string {
	return strings.NewReplacer("é", "e", "è", "e", "ê", "e", "û", "u", "ô", "o", "à", "a").Replace(s)
}

// parseLongDate lit « 15 janvier 2026 », « 1er mars 2026 » ou « 15-janv-2026 ».
func parseLongDate(s string) (float64, bool) {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '-' })
	if len(f) != 3 {
		return 0, false
	}
	d, err1 := strconv.Atoi(strings.TrimSuffix(f[0], "er"))
	y, err2 := strconv.Atoi(f[2])
	m, ok := frMonthIndex(f[1])
	if err1 != nil || err2 != nil || !ok || d < 1 || d > 31 || y < 1900 || y > 9999 {
		return 0, false
	}
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if t.Day() != d {
		return 0, false // 31 avril
	}
	return TimeSerial(t), true
}

// fnDateValue : DATEVAL(texte), la date d'un texte (l'heure est ignorée).
// Une valeur déjà date est acceptée telle quelle.
func fnDateValue(a []Value) Value {
	v := a[0]
	switch v.Kind {
	case Error:
		return v
	case Date:
		return DateSerial(math.Floor(v.Num))
	case Text:
		s := strings.TrimSpace(v.Str)
		if f, ok := ParseDate(s); ok {
			return dateNumber(math.Floor(f))
		}
		if f, ok := parseLongDate(s); ok {
			return dateNumber(f)
		}
	}
	return Err(ErrValue)
}

// fnTimeValue : TEMPSVAL(texte), l'heure d'un texte en fraction de journée
// (« 18:00 » donne 0,75 ; une date et heure ne garde que l'heure).
func fnTimeValue(a []Value) Value {
	v := a[0]
	switch v.Kind {
	case Error:
		return v
	case Date:
		return Num(v.Num - math.Floor(v.Num))
	case Text:
	default:
		return Err(ErrValue)
	}
	s := strings.TrimSpace(v.Str)
	if f, ok := ParseDate(s); ok {
		return Num(f - math.Floor(f))
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return Err(ErrValue)
	}
	secs := 0.0
	for i, p := range parts {
		n, err := strconv.ParseFloat(strings.Replace(p, ",", ".", 1), 64)
		if err != nil || n < 0 || i < len(parts)-1 && n != math.Trunc(n) {
			return Err(ErrValue)
		}
		secs += n * []float64{3600, 60, 1}[i]
	}
	day := secs / 86400
	return Num(day - math.Floor(day)) // 25:00 revient à 01:00, comme Excel
}

// fnYearFrac : FRACTION.ANNEE(début; fin; [base]), part d'année entre deux
// dates selon la base de calcul d'Excel (0 : 30/360 américain, 1 : réel/réel,
// 2 : réel/360, 3 : réel/365, 4 : 30/360 européen).
func fnYearFrac(a []Value) Value {
	t1, e := dateArg(a[0])
	if e.IsErr() {
		return e
	}
	t2, e := dateArg(a[1])
	if e.IsErr() {
		return e
	}
	basis := 0
	if len(a) > 2 && !a[2].IsEmpty() {
		f, e := ToNumber(a[2])
		if e.IsErr() {
			return e
		}
		basis = int(math.Trunc(f))
	}
	t1, t2 = t1.Truncate(24*time.Hour), t2.Truncate(24*time.Hour)
	if t2.Before(t1) {
		t1, t2 = t2, t1
	}
	days := math.Round(t2.Sub(t1).Hours() / 24)
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	days360 := func(d1, d2 int) float64 {
		return float64((d2+int(m2)*30+y2*360)-(d1+int(m1)*30+y1*360)) / 360
	}
	switch basis {
	case 0:
		// Règles NASD, dans l'ordre où Excel les applique.
		lastFeb := func(t time.Time) bool { return t.Month() == 2 && t.AddDate(0, 0, 1).Month() == 3 }
		if lastFeb(t1) && lastFeb(t2) {
			d2 = 30
		}
		if lastFeb(t1) {
			d1 = 30
		}
		if d2 == 31 && d1 >= 30 {
			d2 = 30
		}
		if d1 == 31 {
			d1 = 30
		}
		return number(days360(d1, d2))
	case 1:
		if y1 == y2 || y2 == y1+1 && (m1 > m2 || m1 == m2 && d1 >= d2) {
			length := 365.0
			if y1 == y2 && leap(y1) || feb29Between(t1, t2) || m2 == 2 && d2 == 29 {
				length = 366
			}
			return number(days / length)
		}
		span := time.Date(y2+1, 1, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(y1, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24
		return number(days / (math.Round(span) / float64(y2-y1+1)))
	case 2:
		return number(days / 360)
	case 3:
		return number(days / 365)
	case 4:
		return number(days360(min(d1, 30), min(d2, 30)))
	}
	return Err(ErrNum)
}

func leap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

// feb29Between : un 29 février tombe entre deux dates (moins d'un an d'écart).
func feb29Between(t1, t2 time.Time) bool {
	mar1 := func(y int) time.Time { return time.Date(y, 3, 1, 0, 0, 0, 0, time.UTC) }
	y1, y2 := t1.Year(), t2.Year()
	return leap(y1) && t1.Before(mar1(y1)) && !t2.Before(mar1(y1)) ||
		leap(y2) && !t2.Before(mar1(y2)) && t1.Before(mar1(y2))
}
