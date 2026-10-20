package workbook

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// RemoteType donne le type du serveur d'un type local (lattis.create_table :
// text, integer, decimal, date, datetime, boolean). Les types locaux sont
// figés à la publication (§9) : ce type ne change plus.
func RemoteType(t grid.Type) string {
	switch t {
	case grid.Integer:
		return "integer"
	case grid.Decimal:
		return "decimal"
	case grid.Boolean:
		return "boolean"
	case grid.Date:
		return "date"
	}
	return "text"
}

// LocalType est l'inverse de RemoteType (datetime, inconnu localement,
// devient du texte ISO 8601).
func LocalType(t string) grid.Type {
	switch t {
	case "integer":
		return grid.Integer
	case "decimal":
		return grid.Decimal
	case "boolean":
		return grid.Boolean
	case "date":
		return grid.Date
	}
	return grid.Text
}

// ToRemote convertit une valeur stockée localement (table STRICT : INTEGER,
// REAL ou TEXT) en valeur PostgreSQL du type du serveur, pour COPY.
func ToRemote(typ string, v any) (any, error) {
	switch x := v.(type) {
	case []byte:
		v = string(x)
	case json.Number:
		// Valeur relue de la file d'envoi (command.Op) : entier exact, sinon réel.
		if n, err := x.Int64(); err == nil {
			v = n
		} else if f, err := x.Float64(); err == nil {
			v = f
		} else {
			v = x.String()
		}
	case bool:
		if typ != "boolean" {
			v = map[bool]int64{true: 1, false: 0}[x]
		}
	}
	if v == nil {
		return nil, nil
	}
	switch typ {
	case "integer":
		switch x := v.(type) {
		case int64:
			return x, nil
		case float64:
			if x == math.Trunc(x) && !math.IsInf(x, 0) {
				return int64(x), nil
			}
		case string:
			if n, err := strconv.ParseInt(x, 10, 64); err == nil {
				return n, nil
			}
		}
	case "decimal":
		switch x := v.(type) {
		case float64:
			return x, nil
		case int64:
			return float64(x), nil
		case string:
			if f, err := strconv.ParseFloat(x, 64); err == nil {
				return f, nil
			}
		}
	case "boolean":
		switch x := v.(type) {
		case int64:
			return x != 0, nil
		case float64:
			return x != 0, nil
		case bool:
			return x, nil
		}
	case "date":
		if s, ok := v.(string); ok {
			if d, err := time.Parse("2006-01-02", s); err == nil {
				return d, nil
			}
		}
	default:
		switch x := v.(type) {
		case string:
			return x, nil
		case int64:
			return strconv.FormatInt(x, 10), nil
		case float64:
			return strconv.FormatFloat(x, 'g', -1, 64), nil
		}
	}
	return nil, msg.New("workbook.valueIncompatible", msg.Args{"value": fmt.Sprint(v), "type": typ})
}

// FromRemote convertit une valeur lue sur le serveur (voir selectExpr) en
// valeur de stockage locale.
func FromRemote(typ string, v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case bool:
		if x {
			return int64(1)
		}
		return int64(0)
	case int32:
		return int64(x)
	case int64:
		if typ == "decimal" {
			return float64(x)
		}
		return x
	case float32:
		return float64(x)
	case float64:
		if typ == "integer" {
			return int64(x)
		}
		return x
	case time.Time:
		if typ == "date" {
			return x.Format("2006-01-02")
		}
		return x.UTC().Format(time.RFC3339)
	case string:
		return x
	}
	return fmt.Sprint(v)
}

// FromJSON convertit une valeur d'une ligne lue en JSON (to_jsonb de
// PostgreSQL : lattis.changes_since, lattis.why_not), décodée avec
// UseNumber, en valeur de stockage locale du type typ du serveur.
func FromJSON(typ string, v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case bool:
		return FromRemote(typ, x)
	case json.Number:
		switch typ {
		case "integer", "boolean":
			if n, err := x.Int64(); err == nil {
				return n
			}
			if f, err := x.Float64(); err == nil {
				return int64(f)
			}
		case "text":
			return x.String()
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case float64:
		return FromRemote(typ, x)
	case string:
		switch typ {
		case "date":
			if len(x) >= 10 {
				return x[:10]
			}
		case "datetime":
			if t, err := time.Parse(time.RFC3339Nano, x); err == nil {
				return t.UTC().Format(time.RFC3339)
			}
		case "integer":
			if n, err := strconv.ParseInt(x, 10, 64); err == nil {
				return n
			}
		case "decimal":
			if f, err := strconv.ParseFloat(x, 64); err == nil {
				return f
			}
		}
		return x
	}
	return fmt.Sprint(v)
}

// SameRemote compare deux valeurs du serveur (ToRemote), pour la condition
// d'une écriture : nombres par valeur, dates par instant.
func SameRemote(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if ta, ok := a.(time.Time); ok {
		tb, ok := b.(time.Time)
		return ok && ta.Equal(tb)
	}
	fa, oka := number(a)
	fb, okb := number(b)
	if oka || okb {
		return oka && okb && fa == fb
	}
	return a == b
}

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}
