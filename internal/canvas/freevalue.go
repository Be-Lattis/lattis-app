package canvas

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Saisies des cellules libres : la même reconnaissance que les colonnes de
// tableau (grid.Parse, Infer), plus les heures et les pourcentages, comme
// Excel. Une apostrophe de tête force le texte (elle n'est pas affichée).

// freeRaw normalise une saisie de cellule libre avant de l'enregistrer, comme
// Excel la convertit : date jj/mm complétée de l'année en cours (elle ne
// change pas l'an prochain), date ISO écrite jj/mm/aaaa, vrai/faux en
// majuscules, (12) et 1e3 écrits comme des nombres. Formules, textes et
// saisies forcées en texte restent tels quels.
func freeRaw(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || isFormula(raw) || strings.HasPrefix(s, "'") {
		return raw
	}
	switch Infer(s) {
	case grid.Boolean:
		return strings.ToUpper(s)
	case grid.Date:
		if d, withTime, ok := grid.ParseDateTime(s); ok {
			return grid.FreeDate(d, withTime)
		}
	case grid.Integer, grid.Decimal:
		if strings.ContainsAny(s, "()eE") && !strings.Contains(s, "€") {
			if v, err := grid.Parse(grid.Decimal, s); err == nil && v != nil {
				return formula.FormatNumber(v.(float64))
			}
		}
	}
	return raw
}

var (
	timeRe    = regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::(\d{2}))?$`)
	percentRe = regexp.MustCompile(`^([+-]?[\d\s\x{a0}\x{202f}]*[.,]?\d+)\s*%$`)
)

// freeTime lit une heure hh:mm ou hh:mm:ss en fraction de jour (numéro de
// série d'Excel).
func freeTime(s string) (float64, bool) {
	m := timeRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	mn, _ := strconv.Atoi(m[2])
	sec := 0
	if m[3] != "" {
		sec, _ = strconv.Atoi(m[3])
	}
	if h > 23 || mn > 59 || sec > 59 {
		return 0, false
	}
	return float64(h*3600+mn*60+sec) / 86400, true
}

// freePercent lit « 15 % » en 0,15.
func freePercent(s string) (float64, bool) {
	m := percentRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	v, err := grid.Parse(grid.Decimal, m[1])
	if err != nil || v == nil {
		return 0, false
	}
	return v.(float64) / 100, true
}

// valueLiteral donne le nœud d'une saisie qui n'est pas une formule : nombre
// (montants en €, notation comptable, pourcentage), date (DATE(a; m; j)),
// heure (fraction de jour), booléen, ou texte (sans l'apostrophe de tête).
func valueLiteral(raw string) formula.Node {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "'") {
		return formula.StrLit{V: strings.TrimPrefix(strings.TrimLeft(raw, " \t"), "'")}
	}
	switch Infer(raw) {
	case grid.Integer, grid.Decimal:
		if v, err := grid.Parse(grid.Decimal, raw); err == nil && v != nil {
			return formula.NumLit{V: v.(float64)}
		}
	case grid.Boolean:
		v, _ := grid.Parse(grid.Boolean, raw)
		return formula.BoolLit{V: v == int64(1)}
	case grid.Date:
		if d, withTime, ok := grid.ParseDateTime(s); ok {
			day := formula.Call{Fn: "DATE", Args: []formula.Node{
				formula.NumLit{V: float64(d.Year())}, formula.NumLit{V: float64(d.Month())}, formula.NumLit{V: float64(d.Day())},
			}}
			if !withTime {
				return day
			}
			// Date et heure : DATE(a; m; j) + la fraction de jour.
			return formula.Binary{Op: "+", L: day, R: formula.NumLit{V: float64(d.Hour()*3600+d.Minute()*60+d.Second()) / 86400}}
		}
	}
	if f, ok := freeTime(s); ok {
		return formula.NumLit{V: f}
	}
	if f, ok := freePercent(s); ok {
		return formula.NumLit{V: f}
	}
	return formula.StrLit{V: raw}
}
