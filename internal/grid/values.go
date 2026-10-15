package grid

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Type est le type logique d'une colonne, tel qu'affiché à l'utilisateur.
type Type string

const (
	Integer Type = "integer"
	Decimal Type = "decimal"
	Text    Type = "text"
	Boolean Type = "boolean"
	Date    Type = "date"
)

// sqlType donne le type de stockage SQLite (table STRICT) d'un type logique.
func (t Type) sqlType() string {
	switch t {
	case Integer, Boolean:
		return "INTEGER"
	case Decimal:
		return "REAL"
	default:
		return "TEXT"
	}
}

// ErrValue signale une saisie qui ne correspond pas au type de la colonne.
var ErrValue = errors.New("valeur incompatible avec le type de la colonne")

// Parse convertit une saisie de l'utilisateur (locale française) en valeur
// stockable. Une saisie vide donne NULL.
//
// Prototype : l'inférence et l'élargissement de type (phase 3) remplaceront
// le refus par un élargissement ou un marquage en erreur.
func Parse(t Type, raw string) (any, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	switch t {
	case Integer:
		n, err := strconv.ParseInt(stripSpaces(s), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w : %q n'est pas un entier", ErrValue, raw)
		}
		return n, nil
	case Decimal:
		f, err := strconv.ParseFloat(strings.ReplaceAll(stripSpaces(s), ",", "."), 64)
		if err != nil {
			return nil, fmt.Errorf("%w : %q n'est pas un décimal", ErrValue, raw)
		}
		return f, nil
	case Boolean:
		switch strings.ToLower(s) {
		case "vrai", "true", "oui", "1":
			return int64(1), nil
		case "faux", "false", "non", "0":
			return int64(0), nil
		}
		return nil, fmt.Errorf("%w : %q n'est pas un booléen", ErrValue, raw)
	case Date:
		for _, layout := range []string{"02/01/2006", "2/1/2006", "2006-01-02"} {
			if d, err := time.Parse(layout, s); err == nil {
				return d.Format("2006-01-02"), nil
			}
		}
		return nil, fmt.Errorf("%w : %q n'est pas une date (jj/mm/aaaa)", ErrValue, raw)
	default:
		return raw, nil
	}
}

// stripSpaces retire les séparateurs de milliers (espace, espace insécable).
func stripSpaces(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == ' ' || r == ' ' {
			return -1
		}
		return r
	}, s)
}
