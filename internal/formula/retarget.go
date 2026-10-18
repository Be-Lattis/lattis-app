package formula

import "strings"

// RetargetTables fait viser à une formule d'autres tableaux : names associe
// au nom (en minuscules) d'un tableau cité le nom qui le remplace. Sert à la
// copie d'une feuille, dont les formules visent les copies de ses tableaux,
// comme dans Excel. Rend la formule et vrai si elle a changé.
func RetargetTables(n Node, names map[string]string) (Node, bool) {
	changed := false
	swap := func(t string) string {
		if to, ok := names[strings.ToLower(t)]; ok && t != "" {
			changed = true
			return to
		}
		return t
	}
	var walk func(Node) Node
	walk = func(n Node) Node {
		switch n := n.(type) {
		case Ref:
			n.Table = swap(n.Table)
			return n
		case RowRange:
			n.Ref.Table = swap(n.Ref.Table)
			return n
		case Unary:
			n.X = walk(n.X)
			return n
		case Binary:
			n.L, n.R = walk(n.L), walk(n.R)
			return n
		case Call:
			args := make([]Node, len(n.Args))
			copy(args, n.Args)
			start := 0
			// Tableau nommé en premier argument (voir Deps).
			named := (n.Fn == "REFERENCE" || n.Fn == "REFERENCE.APPROX") && len(args) == 3 ||
				n.Fn == "EXISTS" && len(args) == 2 || IsSpill(n) && len(args) > 0
			if named {
				switch a := args[0].(type) {
				case Name:
					args[0], start = Name{Name: swap(a.Name)}, 1
				case StrLit:
					if !IsSpill(n) {
						args[0], start = StrLit{V: swap(a.V)}, 1
					}
				}
			}
			for i := start; i < len(args); i++ {
				args[i] = walk(args[i])
			}
			n.Args = args
			return n
		}
		return n
	}
	out := walk(n)
	return out, changed
}
