package canvas

import (
	"context"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// DepTarget est un antécédent ou un dépendant d'une cellule : une colonne de
// tableau (Column vide : le tableau entier) ou une cellule (nommée ou non).
type DepTarget struct {
	Kind   string `json:"kind"` // column, table, cell
	Table  string `json:"table,omitempty"`
	Column string `json:"column,omitempty"`
	Name   string `json:"name,omitempty"`
	Sheet  int64  `json:"sheet"`
	Row    int    `json:"row"` // en-tête de la colonne, ou la cellule
	Col    int    `json:"col"`
}

// Dependencies donne les antécédents (ce que la formule de la cellule lit) et
// les dépendants (les formules qui lisent la cellule ou sa colonne) de la
// cellule (row, col) d'une feuille, comme l'audit de formules d'Excel.
type Dependencies struct {
	Subject    string      `json:"subject"`
	Precedents []DepTarget `json:"precedents"`
	Dependents []DepTarget `json:"dependents"`
}

type depRef struct{ table, column, name string }

// refsOf liste les références d'une formule ; own est le tableau de la
// formule (pour [@Col] et [Col]).
func refsOf(n formula.Node, own string) []depRef {
	var out []depRef
	var walk func(formula.Node)
	walk = func(n formula.Node) {
		switch n := n.(type) {
		case formula.Ref:
			t := n.Table
			if t == "" {
				t = own
			}
			out = append(out, depRef{table: t, column: n.Col})
		case formula.Name:
			out = append(out, depRef{name: n.Name})
		case formula.Unary:
			walk(n.X)
		case formula.Binary:
			walk(n.L)
			walk(n.R)
		case formula.Call:
			args := n.Args
			if (n.Fn == "REFERENCE" || n.Fn == "REFERENCE.APPROX") && len(args) == 3 {
				t, ok1 := args[0].(formula.Name)
				c, ok2 := args[1].(formula.Name)
				if ok1 && ok2 {
					out = append(out, depRef{table: t.Name, column: c.Name})
					args = args[2:]
				}
			} else if formula.IsSpill(n) && len(args) > 0 {
				if t, ok := args[0].(formula.Name); ok {
					out = append(out, depRef{table: t.Name})
					args = args[1:]
				}
			}
			for _, a := range args {
				walk(a)
			}
		}
	}
	walk(n)
	return out
}

// Dependencies calcule l'audit de la cellule (row, col).
func (m *Model) Dependencies(ctx context.Context, sheet int64, row, col int) (Dependencies, error) {
	var d Dependencies
	d.Precedents, d.Dependents = []DepTarget{}, []DepTarget{}
	err := m.read(ctx, func(t *tx) error {
		// Tous les tableaux et cellules nommées du classeur.
		rs, err := t.query(`SELECT id FROM _lattis_tables`)
		if err != nil {
			return err
		}
		var ids []int64
		for rs.Next() {
			var id int64
			rs.Scan(&id)
			ids = append(ids, id)
		}
		rs.Close()
		var tables []Table
		for _, id := range ids {
			tb, err := t.table(id)
			if err != nil {
				return err
			}
			tables = append(tables, tb)
		}
		type named struct {
			sheet    int64
			row, col int
		}
		names := map[string]named{}
		nameAt := map[[3]int64]string{}
		rs, err = t.query(`SELECT name, sheet, row, col FROM _lattis_names`)
		if err != nil {
			return err
		}
		for rs.Next() {
			var n string
			var x named
			rs.Scan(&n, &x.sheet, &x.row, &x.col)
			names[strings.ToLower(n)] = x
			nameAt[[3]int64{x.sheet, int64(x.row), int64(x.col)}] = n
		}
		rs.Close()
		findTable := func(name string) *Table {
			for i := range tables {
				if strings.EqualFold(tables[i].Name, name) {
					return &tables[i]
				}
			}
			return nil
		}
		target := func(r depRef) (DepTarget, bool) {
			if r.name != "" {
				if x, ok := names[strings.ToLower(r.name)]; ok {
					return DepTarget{Kind: "cell", Name: r.name, Sheet: x.sheet, Row: x.row, Col: x.col}, true
				}
				if tb := findTable(r.name); tb != nil {
					return DepTarget{Kind: "table", Table: tb.Name, Sheet: tb.Sheet, Row: tb.Row, Col: tb.Col}, true
				}
				return DepTarget{}, false
			}
			tb := findTable(r.table)
			if tb == nil {
				return DepTarget{}, false
			}
			if r.column == "" {
				return DepTarget{Kind: "table", Table: tb.Name, Sheet: tb.Sheet, Row: tb.Row, Col: tb.Col}, true
			}
			for i, c := range tb.Cols {
				if strings.EqualFold(c.Name, r.column) {
					return DepTarget{Kind: "column", Table: tb.Name, Column: c.Name, Sheet: tb.Sheet, Row: tb.Row, Col: tb.Col + i}, true
				}
			}
			return DepTarget{}, false
		}
		add := func(list *[]DepTarget, x DepTarget) {
			for _, y := range *list {
				if y == x {
					return
				}
			}
			*list = append(*list, x)
		}

		// Le sujet : une colonne de tableau, ou une cellule libre.
		var subTable *Table
		var subCol *Column
		var subName, subRaw string
		for i := range tables {
			tb := &tables[i]
			if tb.Sheet == sheet && col >= tb.Col && col <= tb.Right() && row >= tb.Row && row <= tb.Bottom() {
				subTable, subCol = tb, &tb.Cols[col-tb.Col]
			}
		}
		if subTable != nil {
			d.Subject = subTable.Name + "[" + subCol.Name + "]"
			if subCol.Formula != "" {
				if n, err := formula.Parse(subCol.Formula, formula.EN); err == nil {
					for _, r := range refsOf(n, subTable.Name) {
						if x, ok := target(r); ok {
							add(&d.Precedents, x)
						}
					}
				}
			}
		} else {
			subName = nameAt[[3]int64{sheet, int64(row), int64(col)}]
			t.row(`SELECT raw FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&subRaw)
			d.Subject = subName
			if isFormula(subRaw) {
				if n, err := formula.Parse(subRaw, formula.FR); err == nil {
					for _, r := range refsOf(n, "") {
						if x, ok := target(r); ok {
							add(&d.Precedents, x)
						}
					}
				}
			}
		}
		// Est-ce qu'une référence vise le sujet ?
		hits := func(r depRef) bool {
			if subTable != nil {
				if r.name != "" {
					return strings.EqualFold(r.name, subTable.Name)
				}
				return strings.EqualFold(r.table, subTable.Name) && (r.column == "" || strings.EqualFold(r.column, subCol.Name))
			}
			return subName != "" && strings.EqualFold(r.name, subName)
		}
		// Dépendants : colonnes calculées, puis cellules libres avec formule.
		for i := range tables {
			tb := &tables[i]
			for k, c := range tb.Cols {
				if c.Formula == "" || (subTable != nil && tb.ID == subTable.ID && c.ID == subCol.ID) {
					continue
				}
				n, err := formula.Parse(c.Formula, formula.EN)
				if err != nil {
					continue
				}
				for _, r := range refsOf(n, tb.Name) {
					if hits(r) {
						add(&d.Dependents, DepTarget{Kind: "column", Table: tb.Name, Column: c.Name, Sheet: tb.Sheet, Row: tb.Row, Col: tb.Col + k})
						break
					}
				}
			}
		}
		rs, err = t.query(`SELECT sheet, row, col, raw FROM _lattis_cells WHERE raw LIKE '=%'`)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var sh int64
			var r, c int
			var raw string
			rs.Scan(&sh, &r, &c, &raw)
			if sh == sheet && r == row && c == col {
				continue
			}
			n, err := formula.Parse(raw, formula.FR)
			if err != nil {
				continue
			}
			for _, ref := range refsOf(n, "") {
				if hits(ref) {
					add(&d.Dependents, DepTarget{Kind: "cell", Name: nameAt[[3]int64{sh, int64(r), int64(c)}], Sheet: sh, Row: r, Col: c})
					break
				}
			}
		}
		return rs.Err()
	})
	return d, err
}
