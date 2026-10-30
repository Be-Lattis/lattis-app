package transfer

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Cellules nommées à l'export .xlsx : un nom défini d'Excel par cellule
// nommée, que les formules citent par son nom (Taux*2). Un nom qu'Excel
// refuserait (adresse, notation L1C1, nom d'un tableau) reste une adresse
// dans les formules et n'est pas écrit.

var (
	excelDefinedName = regexp.MustCompile(`^[\p{L}_\\][\p{L}\p{N}_.]*$`)
	excelR1C1        = regexp.MustCompile(`(?i)^(r\d*c?\d*|c\d*)$`)
	excelA1          = regexp.MustCompile(`(?i)^[a-z]{1,3}\d+$`)
)

// definedName dit si la cellule nommée p s'écrit comme nom défini.
func (b *book) definedName(p cellPlace) bool {
	n := p.name
	if n == "" || !excelDefinedName.MatchString(n) || excelR1C1.MatchString(n) || excelA1.MatchString(n) {
		return false
	}
	low := strings.ToLower(n)
	for _, t := range b.tables {
		if strings.ToLower(t.name) == low {
			return false
		}
	}
	return p.row < ExcelRows && p.col < ExcelCols
}

// definedNamesXML écrit les noms définis des cellules nommées.
func (b *book) definedNamesXML() string {
	var names []cellPlace
	for _, p := range b.names {
		if b.definedName(p) {
			names = append(names, p)
		}
	}
	slices.SortFunc(names, func(x, y cellPlace) int { return strings.Compare(strings.ToLower(x.name), strings.ToLower(y.name)) })
	var out strings.Builder
	for _, p := range names {
		fmt.Fprintf(&out, `<definedName name="%s">%s!$%s$%d</definedName>`, xmlEscape(p.name), xmlEscape(quoteSheet(p.sheet)), colName(p.col), p.row+1)
	}
	return out.String()
}
