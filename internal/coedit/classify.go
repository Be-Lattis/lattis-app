package coedit

import (
	"regexp"
	"strings"
)

// Instructions d'une étape du bus qui ne changent que les lignes des
// tableaux (ce que décrit le journal logique) : écritures dans t_<id>,
// effacement des saisies invalides et des mises en forme des lignes
// supprimées, tables de travail du tri (_lattis_trash_*, _lattis_rank_*),
// index d'ordre des lignes. Tout le reste (cellules libres, mise en forme,
// structure, feuilles, objets…) n'est pas partagé pendant le travail à
// plusieurs.
var dataStatements = []*regexp.Regexp{
	regexp.MustCompile(`^(INSERT(\s+OR\s+\w+)?\s+INTO|REPLACE\s+INTO|UPDATE(\s+OR\s+\w+)?|DELETE\s+FROM)\s+(main\.)?t_\d+\b`),
	regexp.MustCompile(`^(INSERT(\s+OR\s+\w+)?\s+INTO|UPDATE|DELETE\s+FROM)\s+(main\.)?_lattis_invalid\b`),
	regexp.MustCompile(`^DELETE\s+FROM\s+(main\.)?_lattis_styles\s+WHERE\s+tbl\s*=\s*\S+\s+AND\s+rid\s*=`),
	regexp.MustCompile(`^(CREATE\s+(TEMP\s+)?TABLE(\s+IF\s+NOT\s+EXISTS)?|DROP\s+TABLE(\s+IF\s+EXISTS)?|INSERT(\s+OR\s+\w+)?\s+INTO|UPDATE|DELETE\s+FROM)\s+(main\.|temp\.)?_lattis_(trash|rank)_\w+`),
	regexp.MustCompile(`^(CREATE\s+(UNIQUE\s+)?INDEX(\s+IF\s+NOT\s+EXISTS)?|DROP\s+INDEX(\s+IF\s+EXISTS)?)\s+(main\.)?t_\d+_pos\b`),
}

// dataOnly dit si l'instruction q ne change que des lignes de tableaux.
func dataOnly(q string) bool {
	q = strings.TrimSpace(q)
	head := strings.ToUpper(q[:min(len(q), 12)])
	if strings.HasPrefix(head, "SELECT") || strings.HasPrefix(head, "WITH") || strings.HasPrefix(head, "PRAGMA") {
		return true // lecture (ou réglage de la connexion)
	}
	q = caseFold(q)
	for _, re := range dataStatements {
		if re.MatchString(q) {
			return true
		}
	}
	return false
}

// caseFold met les mots-clés en majuscules sans toucher aux noms
// (t_<id>, _lattis_*), qui sont en minuscules.
func caseFold(q string) string {
	fields := strings.Fields(q)
	for i, f := range fields {
		if i > 8 {
			break
		}
		switch u := strings.ToUpper(f); u {
		case "INSERT", "INTO", "OR", "REPLACE", "UPDATE", "DELETE", "FROM", "CREATE", "DROP", "TABLE", "TEMP", "IF", "NOT", "EXISTS", "INDEX", "UNIQUE", "WHERE", "AND", "IGNORE", "ABORT", "FAIL", "ROLLBACK":
			fields[i] = u
		}
	}
	return strings.Join(fields, " ")
}
