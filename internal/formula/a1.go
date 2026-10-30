package formula

import (
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// Adresses A1 tapées à la Excel (=C14*2, =SOMME(C14:C16)) : à la validation,
// elles sont réécrites dans la nomenclature de Lattis selon ce qui occupe
// la feuille. Une cellule libre devient Cell[C14] ; une cellule de tableau
// Tableau[Col][k] (k : la clé de la ligne si le tableau en a une, sinon son
// rang), un en-tête Tableau[Col]. Une plage de cellules libres devient
// Cell[C14:C16] ; une plage dans une seule colonne de tableau devient
// Tableau[Col] (toutes les lignes) ou Tableau[Col][1:3] (rangs).

// TablePos situe un tableau sur sa feuille : l'en-tête est en (Row, Col), les
// lignes de données en dessous (rang 1 en Row+1).
type TablePos struct {
	Name     string
	Row, Col int
	Cols     []string
	Rows     int
}

// Locator décrit les feuilles du classeur pour la conversion des adresses.
type Locator interface {
	// Known dit si un mot est un nom de cellule ou de tableau existant (ce
	// n'est alors pas une adresse).
	Known(name string) bool
	// Tables donne les tableaux d'une feuille ("" : celle de la formule) ;
	// faux si la feuille n'existe pas.
	Tables(sheet string) ([]TablePos, bool)
	// Key donne la clé de la ligne de rang k (1 = première) d'un tableau
	// qui en a une ; faux sinon.
	Key(table string, k int) (Node, bool)
}

func (t TablePos) right() int  { return t.Col + len(t.Cols) - 1 }
func (t TablePos) bottom() int { return t.Row + t.Rows }

// tableAt donne le tableau qui recouvre une cellule.
func tableAt(ts []TablePos, row, col int) (TablePos, bool) {
	for _, t := range ts {
		if row >= t.Row && row <= t.bottom() && col >= t.Col && col <= t.right() {
			return t, true
		}
	}
	return TablePos{}, false
}

// a1Range lit « Feuille!A1:B3 » (apostrophes et $ admis).
func splitA1Range(text string) (sheet string, r1, c1, r2, c2 int, ok bool) {
	text = strings.ReplaceAll(text, "$", "")
	if k := strings.LastIndex(text, "!"); k >= 0 {
		sheet = text[:k]
		if strings.HasPrefix(sheet, "'") && strings.HasSuffix(sheet, "'") && len(sheet) >= 2 {
			sheet = strings.ReplaceAll(sheet[1:len(sheet)-1], "''", "'")
		}
		text = text[k+1:]
	}
	a, b, found := strings.Cut(text, ":")
	if !found {
		return "", 0, 0, 0, 0, false
	}
	_, r1, c1, ok1 := A1Address(a)
	_, r2, c2, ok2 := A1Address(b)
	if !ok1 || !ok2 {
		return "", 0, 0, 0, 0, false
	}
	return sheet, min(r1, r2), min(c1, c2), max(r1, r2), max(c1, c2), true
}

// A1Resolver situe les plages A1 des fonctions converties (RECHERCHEV…) à
// l'aide d'un Locator.
func A1Resolver(l Locator) Resolver { return a1Resolver{l} }

type a1Resolver struct{ l Locator }

func (r a1Resolver) Resolve(text string) (string, int, bool) {
	sheet, r1, c1, r2, _, ok := splitA1Range(text)
	if !ok {
		return "", 0, false
	}
	ts, ok := r.l.Tables(sheet)
	if !ok {
		return "", 0, false
	}
	for _, t := range ts {
		if c1 >= t.Col && c1 <= t.right() && r1 <= t.bottom() && r2 >= t.Row {
			return t.Name, c1 - t.Col, true
		}
	}
	return "", 0, false
}

// ConvertA1 réécrit les adresses A1 d'une formule (voir plus haut). Une plage
// qui recoupe un tableau sans tenir dans une de ses colonnes est refusée.
func ConvertA1(n Node, l Locator) (Node, error) {
	var walk func(Node) (Node, error)
	walk = func(n Node) (Node, error) {
		switch m := n.(type) {
		case Name:
			sheet, row, col, ok := A1Address(m.Name)
			if !ok || l.Known(m.Name) {
				return m, nil
			}
			return cellA1(l, sheet, row, col)
		case Range:
			return rangeA1(l, m.Text)
		case Unary:
			x, err := walk(m.X)
			m.X = x
			return m, err
		case Binary:
			a, err := walk(m.L)
			if err != nil {
				return nil, err
			}
			b, err := walk(m.R)
			m.L, m.R = a, b
			return m, err
		case Call:
			// Noms de tableau et de colonne : REFERENCE(Clients; Q1; clé),
			// EXISTE(Clients; clé), FILTRE(Ventes; …).
			names := 0
			switch {
			case m.Fn == "REFERENCE" || m.Fn == "REFERENCE.APPROX":
				names = 2
			case m.Fn == "EXISTS" || NamesSource(m):
				names = 1
			}
			args := make([]Node, len(m.Args))
			for i, a := range m.Args {
				if _, isRange := a.(Range); i < names && !isRange {
					// Une plage A1 en source se convertit : UNIQUE(A1:B20).
					args[i] = a
					continue
				}
				x, err := walk(a)
				if err != nil {
					return nil, err
				}
				args[i] = x
			}
			m.Args = args
			return m, nil
		}
		return n, nil
	}
	return walk(n)
}

func sheetOf(l Locator, sheet string) ([]TablePos, error) {
	ts, ok := l.Tables(sheet)
	if !ok {
		return nil, convErr("formula.convert.sheetNotFound", msg.Args{"sheet": sheet})
	}
	return ts, nil
}

// cellA1 convertit l'adresse d'une cellule.
func cellA1(l Locator, sheet string, row, col int) (Node, error) {
	ts, err := sheetOf(l, sheet)
	if err != nil {
		return nil, err
	}
	t, ok := tableAt(ts, row, col)
	if !ok {
		name := CellName(row, col)
		if sheet != "" {
			name = sheet + "." + name
		}
		return Name{name}, nil
	}
	ref := Ref{Table: t.Name, Col: t.Cols[col-t.Col]}
	if row == t.Row {
		return ref, nil // en-tête : toute la colonne
	}
	k := row - t.Row
	return Call{Fn: "ITEM", Args: []Node{ref, NumLit{float64(k)}}}, nil
}

// rangeA1 convertit une plage.
func rangeA1(l Locator, text string) (Node, error) {
	sheet, r1, c1, r2, c2, ok := splitA1Range(text)
	if !ok {
		return Range{text}, nil
	}
	ts, err := sheetOf(l, sheet)
	if err != nil {
		return nil, err
	}
	for _, t := range ts {
		if r1 > t.bottom() || r2 < t.Row || c1 > t.right() || c2 < t.Col {
			continue
		}
		if c1 != c2 || c1 < t.Col || c1 > t.right() {
			return nil, convErr("formula.convert.rangeOverflow", msg.Args{"range": text, "table": t.Name})
		}
		ref := Ref{Table: t.Name, Col: t.Cols[c1-t.Col]}
		from, to := max(r1-t.Row, 1), min(r2-t.Row, t.Rows)
		if from <= 1 && to >= t.Rows || from > to {
			return ref, nil // toutes les lignes (ou l'en-tête seul)
		}
		return RowRange{Ref: ref, From: from, To: to}, nil
	}
	if r1 == r2 && c1 == c2 {
		return cellA1(l, sheet, r1, c1)
	}
	if (r2-r1+1)*(c2-c1+1) > MaxRangeCells {
		return nil, convErr("formula.convert.rangeTooBig", msg.Args{"range": text, "max": MaxRangeCells})
	}
	return CellRange{Sheet: sheet, R1: r1, C1: c1, R2: r2, C2: c2}, nil
}
