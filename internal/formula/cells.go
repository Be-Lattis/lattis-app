package formula

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Les cellules libres (hors tableau) se lisent par leur adresse :
// Cell_A1 est la cellule de la colonne A, ligne 1, de la feuille de la
// formule (ou du tableau dont la colonne calculée la cite) ; Feuille2.Cell_B3
// la désigne sur une autre feuille. La valeur de la cellule remplace la
// référence avant le calcul ; les références circulaires donnent #REF!.

var cellRe = regexp.MustCompile(`(?i)^cell_([a-z]{1,3})([1-9][0-9]{0,6})$`)

// CellRef lit une adresse « Cell_A1 » : ligne et colonne de la feuille
// (0, 0 pour A1).
func CellRef(name string) (row, col int, ok bool) {
	m := cellRe.FindStringSubmatch(name)
	if m == nil {
		return 0, 0, false
	}
	for _, ch := range strings.ToUpper(m[1]) {
		col = col*26 + int(ch-'A'+1)
	}
	r, _ := strconv.Atoi(m[2])
	return r - 1, col - 1, true
}

// ColumnLetters donne les lettres d'une colonne (0 → A, 26 → AA).
func ColumnLetters(col int) string {
	s := ""
	for col++; col > 0; col = (col - 1) / 26 {
		s = string(rune('A'+(col-1)%26)) + s
	}
	return s
}

// CellName donne l'adresse d'une cellule : « Cell_A1 ».
func CellName(row, col int) string {
	return "Cell_" + ColumnLetters(col) + strconv.Itoa(row+1)
}

// CellKey est la clé interne (table des noms) d'une cellule d'une feuille.
func CellKey(sheet int64, row, col int) string {
	return fmt.Sprintf("%s#%d", strings.ToLower(CellName(row, col)), sheet)
}

// QualifyCells rattache à la feuille sheet les adresses Cell_A1 d'une
// formule (les adresses déjà qualifiées par une feuille sont gardées).
func QualifyCells(n Node, sheet int64) Node {
	switch v := n.(type) {
	case Name:
		if r, c, ok := CellRef(v.Name); ok {
			return Name{Name: CellKey(sheet, r, c)}
		}
	case Unary:
		v.X = QualifyCells(v.X, sheet)
		return v
	case Binary:
		v.L = QualifyCells(v.L, sheet)
		v.R = QualifyCells(v.R, sheet)
		return v
	case Call:
		args := make([]Node, len(v.Args))
		for i, a := range v.Args {
			args[i] = QualifyCells(a, sheet)
		}
		v.Args = args
		return v
	}
	return n
}
