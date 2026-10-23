package workbook

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ConvertRemote fait d'une valeur du serveur (ToRemote) d'un type à un
// autre ce qu'en fait le changement de type d'une colonne
// (lattis.alter_column_type, expression lattis.convert_expr ; heures en
// UTC, AlterColumnType). Le faux serveur des essais s'en sert ; la
// synchronisation aussi, pour la valeur d'avant (condition) d'une case
// modifiée dans l'étape qui a changé le type de sa colonne.
func ConvertRemote(v any, from, to string) (any, error) {
	if v == nil {
		return nil, nil
	}
	if to == "text" {
		switch x := v.(type) {
		case bool:
			return strconv.FormatBool(x), nil
		case time.Time:
			if from == "date" {
				return x.Format("2006-01-02"), nil
			}
			return x.UTC().Format("2006-01-02 15:04:05+00"), nil
		}
		return ToRemote("text", FromRemote(from, v))
	}
	if from == "text" {
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" {
			return nil, nil
		}
		switch to {
		case "decimal", "integer":
			f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
			if err != nil {
				return nil, fmt.Errorf("syntaxe en entrée invalide pour le type numeric : « %s »", s)
			}
			if to == "integer" {
				return int64(math.Round(f)), nil
			}
			return f, nil
		case "boolean":
			switch strings.ToLower(s) {
			case "t", "true", "y", "yes", "on", "1":
				return true, nil
			case "f", "false", "n", "no", "off", "0":
				return false, nil
			}
			return nil, fmt.Errorf("syntaxe en entrée invalide pour le type boolean : « %s »", s)
		}
		return ToRemote(to, s)
	}
	switch {
	case from == "decimal" && to == "integer":
		f, _ := v.(float64)
		return int64(math.Round(f)), nil
	case from == "boolean" && (to == "integer" || to == "decimal"):
		n := int64(0)
		if x, _ := v.(bool); x {
			n = 1
		}
		return ToRemote(to, n)
	case (from == "integer" || from == "decimal") && to == "boolean":
		f, _ := ToRemote("decimal", FromRemote(from, v))
		x, _ := f.(float64)
		return x != 0, nil
	}
	return ToRemote(to, FromRemote(from, v))
}
