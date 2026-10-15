package canvas

import (
	"strings"

	"github.com/lalamefine/slitable/internal/grid"
)

// Infer donne le type le plus étroit qui accepte la saisie, dans le treillis
// booléen → entier → décimal → texte et date → texte. Une saisie vide n'a pas
// de type (""), elle ne contraint pas la colonne.
func Infer(raw string) grid.Type {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	switch strings.ToLower(s) {
	case "vrai", "faux":
		return grid.Boolean
	}
	for _, t := range []grid.Type{grid.Integer, grid.Decimal, grid.Date} {
		if _, err := grid.Parse(t, s); err == nil {
			return t
		}
	}
	return grid.Text
}

// rank situe un type numérique dans la chaîne booléen < entier < décimal.
func rank(t grid.Type) int {
	switch t {
	case grid.Boolean:
		return 1
	case grid.Integer:
		return 2
	case grid.Decimal:
		return 3
	}
	return 0
}

// Join donne le plus petit type qui accepte les valeurs des deux types.
func Join(a, b grid.Type) grid.Type {
	switch {
	case a == "":
		return b
	case b == "", a == b:
		return a
	case rank(a) > 0 && rank(b) > 0:
		if rank(a) > rank(b) {
			return a
		}
		return b
	default:
		return grid.Text
	}
}

// InferAll donne le type d'une colonne à partir de ses saisies. Une colonne
// sans aucune valeur est en texte.
func InferAll(raws []string) grid.Type {
	var t grid.Type
	for _, r := range raws {
		t = Join(t, Infer(r))
	}
	if t == "" {
		return grid.Text
	}
	return t
}

// castExpr convertit la colonne col du type from vers le type to dans un
// SELECT de reconstruction. Les conversions vers un type plus étroit ne
// servent qu'à l'annulation, quand toutes les valeurs sont convertibles.
func castExpr(col string, from, to grid.Type) string {
	switch {
	case from == to:
		return col
	case to == grid.Text && from == grid.Boolean:
		return "CASE " + col + " WHEN 1 THEN 'VRAI' WHEN 0 THEN 'FAUX' END"
	case to == grid.Boolean && from == grid.Text:
		return "CASE " + col + " WHEN 'VRAI' THEN 1 WHEN 'FAUX' THEN 0 END"
	case to == grid.Text:
		return "CAST(" + col + " AS TEXT)"
	case to == grid.Integer, to == grid.Boolean:
		return "CAST(" + col + " AS INTEGER)"
	case to == grid.Decimal:
		return "CAST(" + col + " AS REAL)"
	default: // date ← texte : même stockage
		return col
	}
}

// sqlType donne le type de stockage d'un type logique dans une table STRICT.
func sqlType(t grid.Type) string {
	switch t {
	case grid.Integer, grid.Boolean:
		return "INTEGER"
	case grid.Decimal:
		return "REAL"
	default:
		return "TEXT"
	}
}
