package canvas

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
)

// Filtres par condition (« Filtres textuels », « Filtres numériques », dates),
// compilés en SQL sur la colonne du tableau.

// condsSQL est la condition SQL des filtres par condition d'une colonne
// (vide sans condition).
func condsSQL(tb Table, c Column, f ColumnFilter) (string, error) {
	var parts []string
	for _, fc := range f.Conds {
		w, err := condSQL(tb, c, fc)
		if err != nil {
			return "", err
		}
		parts = append(parts, "("+w+")")
	}
	if len(parts) == 0 {
		return "", nil
	}
	sep := " AND "
	if f.Or {
		sep = " OR "
	}
	return "(" + strings.Join(parts, sep) + ")", nil
}

// ordered indique un type comparé par sa valeur (nombres, dates, booléens).
func ordered(t grid.Type) bool {
	return t == grid.Integer || t == grid.Decimal || t == grid.Date || t == grid.Boolean
}

// condValue convertit la valeur d'une condition au type de la colonne (saisie
// comme dans une cellule) et l'écrit en littéral SQL.
func condValue(c Column, s string) (string, error) {
	v, err := grid.Parse(c.Type, s)
	if err != nil {
		return "", fmt.Errorf("filtre de « %s » : %w", c.Name, err)
	}
	switch x := v.(type) {
	case nil:
		return "", fmt.Errorf("filtre de « %s » : valeur manquante", c.Name)
	case int64:
		return strconv.FormatInt(x, 10), nil
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case string:
		return sqlLiteral(x), nil
	}
	return sqlLiteral(fmt.Sprint(v)), nil
}

// likePattern écrit un motif LIKE (échappement par \) où * et ? sont les
// jokers d'Excel.
func likePattern(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '%', '_', '\\':
			b.WriteRune('\\')
			b.WriteRune(r)
		case '*':
			b.WriteRune('%')
		case '?':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// condSQL est la condition SQL d'un filtre par condition sur la colonne c.
func condSQL(tb Table, c Column, fc FilterCond) (string, error) {
	col := colSQL(c.ID)
	txt := textExpr(c)
	like := func(pat string, not bool) string {
		op := " LIKE "
		if not {
			op = " NOT LIKE "
		}
		return txt + op + sqlLiteral(pat) + ` ESCAPE '\'`
	}
	// Comparaison d'ordre : sur la valeur pour un nombre ou une date, sur le
	// texte (sans casse) sinon ; les vides n'y répondent jamais.
	cmp := func(op, s string) (string, error) {
		if ordered(c.Type) {
			v, err := condValue(c, s)
			if err != nil {
				return "", err
			}
			return col + " " + op + " " + v, nil
		}
		if strings.TrimSpace(s) == "" {
			return "", fmt.Errorf("filtre de « %s » : valeur manquante", c.Name)
		}
		return fmt.Sprintf("%s <> '' AND lower(%s) %s lower(%s)", txt, txt, op, sqlLiteral(s)), nil
	}
	switch fc.Op {
	case "contains", "notcontains":
		return like("%"+likePattern(fc.Value)+"%", fc.Op == "notcontains"), nil
	case "begins":
		return like(likePattern(fc.Value)+"%", false), nil
	case "ends":
		return like("%"+likePattern(fc.Value), false), nil
	case "eq", "ne":
		ne := fc.Op == "ne"
		if strings.TrimSpace(fc.Value) == "" {
			if ne {
				return txt + " <> ''", nil
			}
			return txt + " = ''", nil
		}
		if ordered(c.Type) {
			if v, err := condValue(c, fc.Value); err == nil {
				if ne {
					return col + " IS NULL OR " + col + " <> " + v, nil
				}
				return col + " = " + v, nil
			}
		}
		if strings.ContainsAny(fc.Value, "*?") {
			return like(likePattern(fc.Value), ne), nil
		}
		op := " = "
		if ne {
			op = " <> "
		}
		return "lower(" + txt + ")" + op + "lower(" + sqlLiteral(fc.Value) + ")", nil
	case "gt":
		return cmp(">", fc.Value)
	case "ge":
		return cmp(">=", fc.Value)
	case "lt":
		return cmp("<", fc.Value)
	case "le":
		return cmp("<=", fc.Value)
	case "between":
		a, err := cmp(">=", fc.Value)
		if err != nil {
			return "", err
		}
		b, err := cmp("<=", fc.To)
		if err != nil {
			return "", err
		}
		return a + " AND " + b, nil
	case "top", "bottom":
		if !ordered(c.Type) {
			return "", fmt.Errorf("« %s » n'est pas une colonne de nombres ou de dates", c.Name)
		}
		n, err := strconv.Atoi(strings.TrimSpace(fc.Value))
		if err != nil || n < 1 {
			return "", fmt.Errorf("filtre de « %s » : nombre d'éléments invalide (%q)", c.Name, fc.Value)
		}
		op, order, edge := ">=", "DESC", "min"
		if fc.Op == "bottom" {
			op, order, edge = "<=", "ASC", "max"
		}
		// Seuil : le n-ième plus grand (ou petit), ex aequo compris comme
		// Excel ; toutes les lignes s'il y en a moins de n.
		src := tb.sql()
		return fmt.Sprintf("%s %s coalesce((SELECT %s FROM %s WHERE %s IS NOT NULL ORDER BY %s %s LIMIT 1 OFFSET %d), (SELECT %s(%s) FROM %s))",
			col, op, col, src, col, col, order, n-1, edge, col, src), nil
	case "above", "below":
		if c.Type != grid.Integer && c.Type != grid.Decimal {
			return "", fmt.Errorf("« %s » n'est pas une colonne de nombres", c.Name)
		}
		op := ">"
		if fc.Op == "below" {
			op = "<"
		}
		return fmt.Sprintf("%s %s (SELECT avg(%s) FROM %s)", col, op, col, tb.sql()), nil
	}
	return "", fmt.Errorf("filtre : condition %q inconnue", fc.Op)
}
