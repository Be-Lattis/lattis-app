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

// sheetPrefix écrit « Feuille! » ou « 'Ma feuille'! » ("" sans feuille).
func sheetPrefix(sheet string) string {
	if sheet == "" {
		return ""
	}
	if !identRe.MatchString(sheet) {
		sheet = "'" + strings.ReplaceAll(sheet, "'", "''") + "'"
	}
	return sheet + "!"
}

// String écrit la plage dans la nomenclature : Cell[A1:B3], Feuille!Cell[A1:B3].
func (r CellRange) String() string {
	return sheetPrefix(r.Sheet) + "Cell[" + ColumnLetters(r.C1) + strconv.Itoa(r.R1+1) + ":" + ColumnLetters(r.C2) + strconv.Itoa(r.R2+1) + "]"
}

// Size donne le nombre de lignes et de colonnes de la plage.
func (r CellRange) Size() (rows, cols int) { return r.R2 - r.R1 + 1, r.C2 - r.C1 + 1 }

// keys donne la clé (table des noms) de chaque cellule de la plage, ligne
// par ligne : feuille de la formule (Home) ou citée, sinon home (feuille du
// tableau d'une colonne calculée, 0 si inconnue).
func (r CellRange) keys(home int64) []string {
	if r.Home != 0 {
		home = r.Home
	}
	out := make([]string, 0, (r.R2-r.R1+1)*(r.C2-r.C1+1))
	for row := r.R1; row <= r.R2; row++ {
		for col := r.C1; col <= r.C2; col++ {
			switch {
			case r.Sheet != "":
				out = append(out, strings.ToLower(r.Sheet+"."+CellName(row, col)))
			case home != 0:
				out = append(out, CellKey(home, row, col))
			default:
				out = append(out, strings.ToLower(CellName(row, col)))
			}
		}
	}
	return out
}

var a1Re = regexp.MustCompile(`^([A-Za-z]{1,3})([1-9][0-9]{0,6})$`)

// A1Address lit une adresse tapée à la Excel : « C14 », « Feuille2!C14 »
// (le nom de feuille sans apostrophes). Les $ doivent être retirés.
func A1Address(text string) (sheet string, row, col int, ok bool) {
	addr := text
	if k := strings.LastIndex(text, "!"); k >= 0 {
		sheet, addr = text[:k], text[k+1:]
		if sheet == "" {
			return "", 0, 0, false
		}
	}
	if !a1Re.MatchString(addr) {
		return "", 0, 0, false
	}
	row, col, ok = CellRef("Cell_" + addr)
	return sheet, row, col, ok
}

// itemCall traduit Tableau[Col][k] : la k-ième ligne de données dans l'ordre
// d'affichage (1 = première), même si le tableau a une clé (choix de Louis,
// 17/10/2026 : tout par rang ; REFERENCE reste la recherche par la clé).
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
	return Call{Fn: "INDEX", Args: []Node{Ref{Table: ref.Table, Col: ref.Col}, n.Args[1]}}, true
}

// Adresses Cell[…] : 4 lettres et 9 chiffres (feuille sans fin pratique) ;
// les adresses tapées à la Excel restent à 3 lettres et 7 chiffres (a1Re).
var cellRe = regexp.MustCompile(`(?i)^cell_([a-z]{1,4})([1-9][0-9]{0,8})$`)

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
	case CellRange:
		if v.Sheet == "" {
			v.Home = sheet
		}
		return v
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

// External dit si une formule de colonne calculée du tableau own peut
// dépendre d'autre chose que de son propre tableau : un autre tableau, une
// cellule libre ou nommée, une plage de cellules, une fonction volatile
// (ALEA, MAINTENANT…). Faute de pouvoir l'analyser, la réponse est oui.
func External(src, own string) bool {
	n, err := Parse(src, FR)
	if err != nil {
		return true
	}
	var walk func(Node) bool
	walk = func(n Node) bool {
		switch v := n.(type) {
		case Ref:
			return v.Table != "" && !strings.EqualFold(v.Table, own)
		case RowRange:
			return v.Ref.Table != "" && !strings.EqualFold(v.Ref.Table, own)
		case Name, CellRange, Range:
			return true
		case Unary:
			return walk(v.X)
		case Binary:
			return walk(v.L) || walk(v.R)
		case Call:
			switch v.Fn {
			case "RAND", "RANDBETWEEN", "RANDARRAY", "NOW", "TODAY":
				return true
			}
			for _, a := range v.Args {
				if walk(a) {
					return true
				}
			}
		}
		return false
	}
	return walk(n)
}

// relativeOffset reconnaît [Col][-1] / [Col][+1] (Tableau[Col][-2]…) :
// la ligne k lignes plus haut (négatif) ou plus bas, dans l'ordre
// d'affichage, à partir de la ligne de la formule. Le signe écrit distingue
// ce décalage de [Col][2], rang absolu (choix de Louis, 17/10/2026).
func relativeOffset(n Call) (int, bool) {
	if n.Fn != "ITEM" || len(n.Args) != 2 {
		return 0, false
	}
	u, ok := n.Args[1].(Unary)
	if !ok || (u.Op != "-" && u.Op != "+") {
		return 0, false
	}
	v, ok := u.X.(NumLit)
	if !ok || v.V != float64(int(v.V)) {
		return 0, false
	}
	if u.Op == "-" {
		return -int(v.V), true
	}
	return int(v.V), true
}

// relative compile [Col][±k] : la valeur de la colonne k lignes plus haut
// ou plus bas (vide hors du tableau). Seulement dans le tableau de la formule.
func (c *compiler) relative(n Call, k int, sc scope) string {
	r, ok := n.Args[0].(Ref)
	t := sc.table
	if !ok || t == nil || (r.Table != "" && !strings.EqualFold(r.Table, t.Name)) {
		return errSQL(ErrRef)
	}
	i, ok := t.col(r.Col)
	if !ok {
		return errSQL(ErrRef)
	}
	if k == 0 {
		return c.column(t, &t.Cols[i], sc.alias)
	}
	a := c.newAlias()
	val := c.column(t, &t.Cols[i], a)
	op, dir, off := "<", "DESC", -k
	if k > 0 {
		op, dir, off = ">", "ASC", k
	}
	return fmt.Sprintf("(SELECT %s FROM %s AS %s WHERE %s._pos %s %s._pos ORDER BY %s._pos %s LIMIT 1 OFFSET %d)",
		val, t.SQL, a, a, op, sc.alias, a, dir, off-1)
}
