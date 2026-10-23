package query

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// plain ramène une valeur lue (tableau du classeur, base de travail) à une
// valeur stockable : int64, float64, string ou nil.
func plain(v any) any {
	switch x := v.(type) {
	case nil, int64, float64, string:
		return v
	case []byte:
		return string(x)
	case bool:
		if x {
			return int64(1)
		}
		return int64(0)
	case int:
		return int64(x)
	case canvas.Invalid:
		return string(x)
	case formula.Value:
		switch x.Kind {
		case formula.Number:
			return x.Num
		case formula.Text:
			return x.Str
		case formula.Bool:
			if x.Bool {
				return "VRAI"
			}
			return "FAUX"
		case formula.Date:
			return excelDate(x.Num)
		case formula.Error:
			return x.Err.String()
		}
		return nil
	case time.Time:
		return x.Format("2006-01-02")
	}
	return nil
}

// excelDate écrit un numéro de série d'Excel en date ISO.
func excelDate(serial float64) string {
	d := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(math.Floor(serial)))
	return d.Format("2006-01-02")
}

// inferText donne le type d'une saisie (français), comme la grille, sauf
// les suites de chiffres à zéro en tête (codes postaux…) qui restent du
// texte.
func inferText(s string) grid.Type {
	t := strings.TrimSpace(s)
	if len(t) > 1 && t[0] == '0' && t[1] >= '0' && t[1] <= '9' && !strings.ContainsAny(t, "/-") {
		return grid.Text
	}
	return canvas.Infer(s)
}

// inferValue donne le type naturel d'une valeur ("" : vide).
func inferValue(v any) grid.Type {
	switch x := v.(type) {
	case int64:
		return grid.Integer
	case float64:
		return grid.Decimal
	case string:
		return inferText(x)
	}
	return ""
}

// castValue convertit une valeur au stockage d'un type : int64 (entier,
// booléen 0/1), float64, string (texte, date ISO) ou nil si elle ne s'y
// prête pas.
func castValue(v any, t grid.Type) any {
	v = plain(v)
	if v == nil {
		return nil
	}
	switch t {
	case grid.Text:
		switch x := v.(type) {
		case int64:
			return strconv.FormatInt(x, 10)
		case float64:
			return formula.FormatNumber(x)
		}
		return v
	case grid.Integer:
		switch x := v.(type) {
		case int64:
			return x
		case float64:
			if x == math.Trunc(x) && math.Abs(x) < 1<<53 {
				return int64(x)
			}
			return nil
		}
	case grid.Boolean:
		switch x := v.(type) {
		case int64:
			if x == 0 || x == 1 {
				return x
			}
			return nil
		case float64:
			if x == 0 || x == 1 {
				return int64(x)
			}
			return nil
		}
	case grid.Decimal:
		switch x := v.(type) {
		case int64:
			return float64(x)
		case float64:
			return x
		}
	case grid.Date:
		if s, ok := v.(string); ok {
			if _, err := time.Parse("2006-01-02", s); err == nil {
				return s
			}
		} else {
			return nil
		}
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	out, err := grid.Parse(t, s)
	if err != nil {
		return nil
	}
	return out
}

// display écrit une valeur comme la grille l'affiche.
func display(v any, t grid.Type) string {
	v = plain(v)
	switch x := v.(type) {
	case nil:
		return ""
	case int64:
		if t == grid.Boolean {
			if x != 0 {
				return "VRAI"
			}
			return "FAUX"
		}
		return strconv.FormatInt(x, 10)
	case float64:
		return formula.FormatNumber(x)
	case string:
		if t == grid.Date {
			return grid.ShowStored(x)
		}
		return x
	}
	return ""
}
