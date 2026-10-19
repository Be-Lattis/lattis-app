package formula

import "strings"

// LIGNE, COLONNE, LIGNES et COLONNES lisent la forme d'une référence, pas
// ses valeurs : Cell[B3], Cell[A1:B3], Tableau[Col], Tableau[Col][1:3], un
// tableau entier ou une constante matricielle. Le résultat ne dépend des
// données que pour le nombre de lignes d'une colonne ou d'un tableau entier,
// que chaque moteur compte (count).

// position calcule LIGNE, COLONNE, LIGNES ou COLONNES. Si count n'est pas
// nil, le résultat est le nombre de lignes de ce tableau.
func position(fn string, n Call, s *Schema, current *Table) (v Value, count *Table) {
	if len(n.Args) == 0 || n.Args[0] == nil {
		// LIGNE() et COLONNE() sans référence : la position de la formule
		// n'est pas connue du moteur.
		return Err(ErrValue), nil
	}
	rows := fn == "ROWS"
	switch a := n.Args[0].(type) {
	case CellRange:
		r, c := a.Size()
		switch fn {
		case "ROW":
			return Num(float64(a.R1 + 1)), nil
		case "COLUMN":
			return Num(float64(a.C1 + 1)), nil
		case "ROWS":
			return Num(float64(r)), nil
		}
		return Num(float64(c)), nil
	case Name:
		addr := a.Name
		if k := strings.LastIndex(addr, "#"); k > 0 {
			addr = addr[:k]
		}
		if k := strings.LastIndex(addr, "."); k >= 0 {
			addr = addr[k+1:]
		}
		if r, c, ok := CellRef(addr); ok {
			switch fn {
			case "ROW":
				return Num(float64(r + 1)), nil
			case "COLUMN":
				return Num(float64(c + 1)), nil
			}
			return Num(1), nil
		}
		t, ok := s.Table(a.Name)
		if !ok {
			return Err(ErrValue), nil
		}
		switch fn {
		case "ROWS":
			return Value{}, t
		case "COLUMNS":
			return Num(float64(len(t.Cols))), nil
		}
		return Num(1), nil // première ligne de données, première colonne
	case Ref:
		t := current
		if a.Table != "" {
			t, _ = s.Table(a.Table)
		}
		if t == nil {
			return Err(ErrValue), nil
		}
		i, ok := t.col(a.Col)
		if !ok {
			return Err(ErrRef), nil
		}
		switch {
		case a.Row && fn == "ROW":
			return Err(ErrValue), nil // [Col] : la valeur de la ligne, dont le rang n'est pas lu
		case a.Row && fn == "ROWS":
			return Num(1), nil
		case fn == "ROWS":
			return Value{}, t
		case fn == "COLUMN":
			return Num(float64(i + 1)), nil // rang de la colonne dans le tableau
		}
		return Num(1), nil
	case RowRange:
		switch {
		case rows:
			return Num(float64(a.To - a.From + 1)), nil
		case fn == "ROW":
			return Num(float64(a.From)), nil
		case fn == "COLUMN":
			return position(fn, Call{Fn: fn, Args: []Node{a.Ref}}, s, current)
		}
		return Num(1), nil
	case ArrayLit:
		switch fn {
		case "ROWS":
			return Num(float64(len(a.Rows))), nil
		case "COLUMNS":
			return Num(float64(len(a.Rows[0]))), nil
		}
		return Num(1), nil
	}
	return Err(ErrValue), nil
}
