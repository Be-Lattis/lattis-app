package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// DepTarget est un antécédent ou un dépendant d'une cellule : une colonne de
// tableau (Column vide : le tableau entier), des lignes précises d'une
// colonne (Tableau[Col][1], [Col][-1], Tableau[Col][2:4], la ligne trouvée
// par REFERENCE) ou une cellule (nommée ou non).
type DepTarget struct {
	Kind   string `json:"kind"` // column, table, row, cell
	Table  string `json:"table,omitempty"`
	Column string `json:"column,omitempty"`
	Name   string `json:"name,omitempty"`
	Sheet  int64  `json:"sheet"`
	Row    int    `json:"row"` // en-tête de la colonne, première ligne visée (row), ou la cellule
	Col    int    `json:"col"`
	// Rows : nombre de lignes encadrées à partir de Row (kind row, plage
	// Tableau[Col][a:b]) ; 0 ou 1 : une seule cellule.
	Rows int `json:"rows,omitempty"`
	// Note : pourquoi la flèche vise la colonne entière alors que la formule
	// lit une ligne précise (ligne masquée par le filtre ou l'onglet, clé
	// introuvable…), ou que la plage n'est encadrée qu'en partie.
	Note string `json:"note,omitempty"`
}

// Dependencies donne les antécédents (ce que la formule de la cellule lit) et
// les dépendants (les formules qui lisent la cellule ou sa colonne) de la
// cellule (row, col) d'une feuille, comme l'audit de formules d'Excel.
type Dependencies struct {
	Subject    string      `json:"subject"`
	Precedents []DepTarget `json:"precedents"`
	Dependents []DepTarget `json:"dependents"`
}

// depRef est une référence d'une formule. Sur une colonne, les lignes lues
// peuvent être précises : from..to (rangs, 1 = première ligne de données,
// dans l'ordre du tableau comme INDEX) pour Tableau[Col][n] et [Col][a:b],
// rel pour [Col][±k] (à partir de la ligne de la formule), key pour
// Tableau[Col][expression] (rang calculé) et REFERENCE (lookup : clé du
// tableau, ou rang s'il n'a pas de clé). Sans rien de cela : toute la colonne.
type depRef struct {
	table, column, name string
	from, to            int
	rel                 int
	isRel               bool
	key                 formula.Node
	lookup              bool
}

// offsetOf reconnaît le décalage de [Col][-1] / [Col][+2] (signe écrit),
// comme formula.relativeOffset.
func offsetOf(n formula.Node) (int, bool) {
	u, ok := n.(formula.Unary)
	if !ok || (u.Op != "-" && u.Op != "+") {
		return 0, false
	}
	v, ok := u.X.(formula.NumLit)
	if !ok || v.V != math.Trunc(v.V) {
		return 0, false
	}
	if u.Op == "-" {
		return -int(v.V), true
	}
	return int(v.V), true
}

// refsOf liste les références d'une formule ; own est le tableau de la
// formule (pour [@Col] et [Col]).
func refsOf(n formula.Node, own string) []depRef {
	var out []depRef
	tableOf := func(t string) string {
		if t == "" {
			return own
		}
		return t
	}
	var walk func(formula.Node)
	walk = func(n formula.Node) {
		switch n := n.(type) {
		case formula.Ref:
			out = append(out, depRef{table: tableOf(n.Table), column: n.Col})
		case formula.RowRange:
			out = append(out, depRef{table: tableOf(n.Ref.Table), column: n.Ref.Col, from: n.From, to: n.To})
		case formula.Name:
			out = append(out, depRef{name: n.Name})
		case formula.Unary:
			walk(n.X)
		case formula.Binary:
			walk(n.L)
			walk(n.R)
		case formula.Call:
			args := n.Args
			if r, ok := firstRef(args); ok && n.Fn == "ITEM" && len(args) == 2 {
				// Tableau[Col][n], [Col][-1], Tableau[Col][expression].
				x := depRef{table: tableOf(r.Table), column: r.Col}
				if k, ok := offsetOf(args[1]); ok {
					x.rel, x.isRel = k, true
				} else if v, ok := args[1].(formula.NumLit); ok && v.V >= 1 && v.V == math.Trunc(v.V) {
					x.from, x.to = int(v.V), int(v.V)
				} else {
					x.key = args[1]
				}
				out = append(out, x)
				args = args[1:]
			} else if (n.Fn == "REFERENCE" || n.Fn == "REFERENCE.APPROX") && len(args) == 3 {
				t, ok1 := args[0].(formula.Name)
				c, ok2 := args[1].(formula.Name)
				if ok1 && ok2 {
					x := depRef{table: t.Name, column: c.Name}
					if n.Fn == "REFERENCE" {
						x.key, x.lookup = args[2], true
					}
					out = append(out, x)
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

func firstRef(args []formula.Node) (formula.Ref, bool) {
	if len(args) == 0 {
		return formula.Ref{}, false
	}
	r, ok := args[0].(formula.Ref)
	return r, ok
}

// depRow situe une ligne de données d'un tableau : son identifiant et son
// rang (1 = première, dans l'ordre du tableau, filtre compris).
type depRow struct {
	id   int64
	rank int
}

// rankOf donne le rang (1 = première ligne) de la ligne id, 0 si elle n'existe pas.
func (t *tx) rankOf(tb *Table, id int64) (int, error) {
	var n int
	err := t.row(`SELECT n FROM (SELECT id, row_number() OVER (ORDER BY _pos) AS n FROM `+tb.sql()+`) WHERE id = ?`, id).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

// depKey évalue la clé (ou le rang) d'une référence quand c'est possible sans
// calculer la formule : une constante, ou une colonne saisie de la ligne de
// la formule ([IdClient] dans une colonne calculée de ce tableau).
func (t *tx) depKey(n formula.Node, own *Table, cur depRow) (any, bool, error) {
	switch v := n.(type) {
	case formula.NumLit:
		return v.V, true, nil
	case formula.StrLit:
		return v.V, true, nil
	case formula.Unary:
		if x, ok := v.X.(formula.NumLit); ok && (v.Op == "-" || v.Op == "+") {
			if v.Op == "-" {
				return -x.V, true, nil
			}
			return x.V, true, nil
		}
	case formula.Ref:
		if own == nil || cur.id == 0 || !v.Row || (v.Table != "" && !strings.EqualFold(v.Table, own.Name)) {
			return nil, false, nil
		}
		for _, c := range own.Cols {
			if strings.EqualFold(c.Name, v.Col) && c.Formula == "" {
				var x any
				err := t.row(`SELECT `+colSQL(c.ID)+` FROM `+own.sql()+` WHERE id = ?`, cur.id).Scan(&x)
				if err != nil || x == nil {
					return nil, false, nil
				}
				return x, true, nil
			}
		}
	}
	return nil, false, nil
}

// depRanks donne les rangs lus par r dans la colonne de tb (0, 0 : toute la
// colonne), la formule étant dans le tableau own à la ligne cur (own nil ou
// cur.rank 0 : cellule libre, en-tête, ligne inconnue). note dit pourquoi
// une ligne précise n'a pas été trouvée.
func (t *tx) depRanks(r depRef, tb *Table, own *Table, cur depRow) (from, to int, note string, err error) {
	label := tb.Name + "[" + r.column + "]"
	switch {
	case r.from > 0:
		from, to = r.from, r.to
	case r.isRel:
		if own == nil || own.ID != tb.ID || cur.rank == 0 {
			return 0, 0, "", nil
		}
		from = cur.rank + r.rel
		to = from
	case r.key != nil:
		v, ok, err := t.depKey(r.key, own, cur)
		if err != nil || !ok {
			return 0, 0, "", err
		}
		var keyCol *Column
		if r.lookup && tb.Key != 0 {
			for i := range tb.Cols {
				if tb.Cols[i].ID == tb.Key {
					keyCol = &tb.Cols[i]
				}
			}
			if keyCol == nil || keyCol.Formula != "" {
				return 0, 0, "", nil
			}
		}
		if keyCol == nil {
			f, ok := v.(float64)
			if i, isInt := v.(int64); isInt {
				f, ok = float64(i), true
			}
			if !ok || f != math.Trunc(f) {
				return 0, 0, "", nil
			}
			from = int(f)
			to = from
			break
		}
		mode, coll := "n", ""
		if keyCol.Type == grid.Text || keyCol.Type == grid.Date {
			mode, coll = "t", " COLLATE NOCASE"
		}
		var id int64
		err = t.row(`SELECT id FROM `+tb.sql()+` WHERE `+colSQL(keyCol.ID)+` = slt_key(?, '`+mode+`')`+coll+` ORDER BY _pos LIMIT 1`, v).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, t.say("ref.keyNotFound", msg.Args{"label": label, "key": fmt.Sprint(v)}), nil
		}
		if err != nil {
			return 0, 0, "", err
		}
		from, err = t.rankOf(tb, id)
		to = from
		if err != nil || from == 0 {
			return 0, 0, "", err
		}
	default:
		return 0, 0, "", nil
	}
	if from < 1 || from > tb.Rows {
		if r.isRel {
			id := msg.ID("ref.noRowAbove")
			if r.rel > 0 {
				id = "ref.noRowBelow"
			}
			return 0, 0, t.say(id, msg.Args{"label": label, "n": max(r.rel, -r.rel), "row": cur.rank}), nil
		}
		if from == to {
			return 0, 0, t.say("ref.rowMissing", msg.Args{"label": label, "row": from}), nil
		}
		return 0, 0, t.say("ref.rowsMissing", msg.Args{"label": label, "from": from, "to": to}), nil
	}
	return from, min(to, tb.Rows), "", nil
}

// depPlace pose sur la feuille les lignes from..to (rangs) de la colonne ci
// de tb : la cellule (ou les cellules encadrées) affichées. Une ligne masquée
// par le filtre ou l'onglet n'a pas de place : la cible reste la colonne,
// avec une note.
func (t *tx) depPlace(base DepTarget, tb *Table, from, to int) (DepTarget, error) {
	first, last, count := from, to, to-from+1
	if tb.Filtered() {
		err := t.row(`SELECT coalesce(min(n), 0), coalesce(max(n), 0), count(*) FROM (SELECT id, row_number() OVER (ORDER BY _pos) AS n FROM `+tb.view()+`)
			WHERE id IN (SELECT id FROM `+tb.sql()+` ORDER BY _pos LIMIT ? OFFSET ?)`, to-from+1, from-1).Scan(&first, &last, &count)
		if err != nil {
			return base, err
		}
	}
	label := base.Table + "[" + base.Column + "]"
	if count == 0 {
		if from == to {
			base.Note = t.say("ref.rowHidden", msg.Args{"label": label, "row": from})
		} else {
			base.Note = t.say("ref.rowsHidden", msg.Args{"label": label, "from": from, "to": to})
		}
		return base, nil
	}
	x := base
	x.Kind = "row"
	x.Row = tb.Row + first
	if last > first {
		x.Rows = last - first + 1
	}
	// Lignes de la feuille masquées (Masquer les lignes) : sans place non plus.
	var hidden int
	if err := t.row(`SELECT count(*) FROM _lattis_row_styles WHERE sheet = ? AND row BETWEEN ? AND ? AND style LIKE '%"hd"%'`,
		tb.Sheet, x.Row, x.Row+max(x.Rows, 1)-1).Scan(&hidden); err != nil {
		return base, err
	}
	if hidden > 0 && hidden >= max(x.Rows, 1) {
		if from == to {
			base.Note = t.say("ref.rowHiddenSheet", msg.Args{"label": label, "row": from})
		} else {
			base.Note = t.say("ref.rowsHiddenSheet", msg.Args{"label": label, "from": from, "to": to})
		}
		return base, nil
	}
	if count < to-from+1 {
		x.Note = t.say("ref.someRowsHidden", msg.Args{"label": label, "n": to - from + 1 - count, "from": from, "to": to})
	}
	return x, nil
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
		if err := eachRow(rs, func() error {
			var id int64
			err := rs.Scan(&id)
			ids = append(ids, id)
			return err
		}); err != nil {
			return err
		}
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
		if err := eachRow(rs, func() error {
			var n string
			var x named
			if err := rs.Scan(&n, &x.sheet, &x.row, &x.col); err != nil {
				return err
			}
			names[strings.ToLower(n)] = x
			nameAt[[3]int64{x.sheet, int64(x.row), int64(x.col)}] = n
			return nil
		}); err != nil {
			return err
		}
		findTable := func(name string) *Table {
			for i := range tables {
				if strings.EqualFold(tables[i].Name, name) {
					return &tables[i]
				}
			}
			return nil
		}
		findCol := func(tb *Table, name string) int {
			for i, c := range tb.Cols {
				if strings.EqualFold(c.Name, name) {
					return i
				}
			}
			return -1
		}
		colTarget := func(tb *Table, i int) DepTarget {
			return DepTarget{Kind: "column", Table: tb.Name, Column: tb.Cols[i].Name, Sheet: tb.Sheet, Row: tb.Row, Col: tb.Col + i}
		}
		// target situe une référence lue par une formule du tableau own (nil :
		// cellule libre), à la ligne cur.
		target := func(r depRef, own *Table, cur depRow) (DepTarget, bool, error) {
			if r.name != "" {
				if x, ok := names[strings.ToLower(r.name)]; ok {
					return DepTarget{Kind: "cell", Name: r.name, Sheet: x.sheet, Row: x.row, Col: x.col}, true, nil
				}
				if tb := findTable(r.name); tb != nil {
					return DepTarget{Kind: "table", Table: tb.Name, Sheet: tb.Sheet, Row: tb.Row, Col: tb.Col}, true, nil
				}
				return DepTarget{}, false, nil
			}
			tb := findTable(r.table)
			if tb == nil {
				return DepTarget{}, false, nil
			}
			if r.column == "" {
				return DepTarget{Kind: "table", Table: tb.Name, Sheet: tb.Sheet, Row: tb.Row, Col: tb.Col}, true, nil
			}
			i := findCol(tb, r.column)
			if i < 0 {
				return DepTarget{}, false, nil
			}
			x := colTarget(tb, i)
			from, to, note, err := t.depRanks(r, tb, own, cur)
			if err != nil || from == 0 {
				x.Note = note
				return x, true, err
			}
			x, err = t.depPlace(x, tb, from, to)
			return x, true, err
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
		var sub depRow // ligne de données du sujet (rang 0 : en-tête, ligne vide)
		for i := range tables {
			tb := &tables[i]
			if tb.Sheet == sheet && col >= tb.Col && col <= tb.Right() && row >= tb.Row && row <= tb.Bottom() {
				subTable, subCol = tb, &tb.Cols[col-tb.Col]
			}
		}
		if subTable != nil {
			d.Subject = subTable.Name + "[" + subCol.Name + "]"
			if k := row - subTable.Row - 1; k >= 0 && (!subTable.Filtered() || k < subTable.Shown) {
				id, _, err := t.rowID(*subTable, k)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				if err == nil {
					rank, err := t.rankOf(subTable, id)
					if err != nil {
						return err
					}
					sub = depRow{id: id, rank: rank}
				}
			}
			if subCol.Formula != "" {
				if n, err := formula.Parse(subCol.Formula, formula.EN); err == nil {
					for _, r := range refsOf(n, subTable.Name) {
						x, ok, err := target(r, subTable, sub)
						if err != nil {
							return err
						}
						if ok {
							add(&d.Precedents, x)
						}
					}
				}
			}
		} else {
			subName = nameAt[[3]int64{sheet, int64(row), int64(col)}]
			err := t.row(`SELECT raw FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&subRaw)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err // cellule vide : pas de formule, rien à lire
			}
			d.Subject = subName
			if isFormula(subRaw) {
				if n, err := formula.Parse(subRaw, formula.FR); err == nil {
					for _, r := range refsOf(n, "") {
						x, ok, err := target(r, nil, depRow{})
						if err != nil {
							return err
						}
						if ok {
							add(&d.Precedents, x)
						}
					}
				}
			}
		}
		// Est-ce qu'une référence vise le sujet ? Une référence à des lignes
		// précises (Tableau[Col][2], clé constante de REFERENCE) ne vise une
		// cellule de données que si elle en fait partie.
		hits := func(r depRef) (bool, error) {
			if subTable == nil {
				return subName != "" && strings.EqualFold(r.name, subName), nil
			}
			if r.name != "" {
				return strings.EqualFold(r.name, subTable.Name), nil
			}
			if !strings.EqualFold(r.table, subTable.Name) || (r.column != "" && !strings.EqualFold(r.column, subCol.Name)) {
				return false, nil
			}
			if sub.rank == 0 || r.column == "" || r.isRel {
				return true, nil
			}
			from, to, _, err := t.depRanks(r, subTable, nil, depRow{})
			if err != nil || from == 0 {
				return from == 0 && r.from == 0, err // ligne inexistante : ne lit rien
			}
			return sub.rank >= from && sub.rank <= to, nil
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
					ok, err := hits(r)
					if err != nil {
						return err
					}
					if !ok {
						continue
					}
					x := colTarget(tb, k)
					if r.isRel && subTable != nil && tb.ID == subTable.ID && sub.rank > 0 {
						// [Col][-1] : c'est la ligne d'en dessous qui lit le sujet.
						if at := sub.rank - r.rel; at >= 1 && at <= tb.Rows {
							if x, err = t.depPlace(x, tb, at, at); err != nil {
								return err
							}
						}
					}
					add(&d.Dependents, x)
					break
				}
			}
		}
		rs, err = t.query(`SELECT sheet, row, col, raw FROM _lattis_cells WHERE raw LIKE '=%'`)
		if err != nil {
			return err
		}
		var cells []struct {
			sheet    int64
			row, col int
			raw      string
		}
		if err := eachRow(rs, func() error {
			var x struct {
				sheet    int64
				row, col int
				raw      string
			}
			err := rs.Scan(&x.sheet, &x.row, &x.col, &x.raw)
			cells = append(cells, x)
			return err
		}); err != nil {
			return err
		}
		for _, x := range cells {
			if x.sheet == sheet && x.row == row && x.col == col {
				continue
			}
			n, err := formula.Parse(x.raw, formula.FR)
			if err != nil {
				continue
			}
			for _, ref := range refsOf(n, "") {
				ok, err := hits(ref)
				if err != nil {
					return err
				}
				if ok {
					add(&d.Dependents, DepTarget{Kind: "cell", Name: nameAt[[3]int64{x.sheet, int64(x.row), int64(x.col)}], Sheet: x.sheet, Row: x.row, Col: x.col})
					break
				}
			}
		}
		return nil
	})
	return d, err
}
