package formula

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Les cellules libres (hors tableau) se lisent par leur adresse :
// Cell[A1] est la cellule de la colonne A, ligne 1, de la feuille de la
// formule (ou du tableau dont la colonne calculée la cite) ; Feuille2!Cell[B3]
// (ou 'Ma feuille'!Cell[B3]) la désigne sur une autre feuille. Les anciennes
// formes Cell_A1 et Feuille2.Cell_B3 restent lues. En interne, l'adresse est
// un nom « Cell_A1 » (ou « Feuille2.Cell_B3 »). La valeur de la cellule
// remplace la référence avant le calcul ; les références circulaires donnent
// #REF!.

// CellText écrit un nom dans une formule : les adresses de cellules dans la
// nomenclature Cell[A1] / Feuille!Cell[A1], les autres noms tels quels.
func CellText(name string) string {
	sheet, addr := "", name
	if k := strings.LastIndex(name, "."); k >= 0 {
		sheet, addr = name[:k], name[k+1:]
	}
	r, c, ok := CellRef(addr)
	if !ok {
		return name
	}
	out := "Cell[" + strings.TrimPrefix(CellName(r, c), "Cell_") + "]"
	if sheet == "" {
		return out
	}
	if !identRe.MatchString(sheet) {
		sheet = "'" + strings.ReplaceAll(sheet, "'", "''") + "'"
	}
	return sheet + "!" + out
}

var identRe = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*$`)

// itemCall traduit Tableau[Col][k] : par la clé si le tableau en a une
// (stable au tri), sinon par le rang de la ligne (1 = première).
// Sans nom de tableau ([Col][k]), c'est le tableau de la formule (cur).
func itemCall(n Call, s *Schema, cur *Table) (Call, bool) {
	if len(n.Args) != 2 {
		return Call{}, false
	}
	ref, ok := n.Args[0].(Ref)
	if !ok {
		return Call{}, false
	}
	if ref.Table == "" {
		if cur == nil {
			return Call{}, false
		}
		ref.Table = cur.Name
	}
	if t, ok := s.Table(ref.Table); ok && t.Key != "" {
		return Call{Fn: "REFERENCE", Args: []Node{Name{t.Name}, StrLit{ref.Col}, n.Args[1]}}, true
	}
	return Call{Fn: "INDEX", Args: []Node{Ref{Table: ref.Table, Col: ref.Col}, n.Args[1]}}, true
}

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
