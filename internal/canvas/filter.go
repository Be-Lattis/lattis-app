package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// ColumnFilter est le filtre d'une colonne de tableau, comme le filtre
// automatique d'Excel : les lignes dont la valeur (texte affiché, vide pour
// une cellule vide) est dans Values, et/ou seulement les saisies hors type.
//
// Conds sont les filtres par condition (« Filtres textuels », « Filtres
// numériques », dates) : toutes vérifiées, ou l'une d'elles avec Or. Les
// trois critères (valeurs, erreurs, conditions) se cumulent. Un filtre
// enregistré avant les conditions se relit tel quel.
type ColumnFilter struct {
	Values []string     `json:"values,omitempty"`
	Errors bool         `json:"errors,omitempty"`
	Conds  []FilterCond `json:"conds,omitempty"`
	Or     bool         `json:"or,omitempty"`
}

// FilterCond est une condition de filtre, comme le filtre personnalisé
// d'Excel :
//
//	texte    : "eq", "ne" (jokers * et ?), "begins", "ends", "contains", "notcontains"
//	ordre    : "gt", "ge", "lt", "le", "between" (Value et To compris)
//	nombres  : "top", "bottom" (Value : nombre d'éléments), "above", "below" (moyenne)
//
// Les valeurs se saisissent comme dans une cellule (1 234,5 ; 31/12/2026).
type FilterCond struct {
	Op    string `json:"op"`
	Value string `json:"value,omitempty"`
	To    string `json:"to,omitempty"`
}

func (f *ColumnFilter) empty() bool {
	return f == nil || (f.Values == nil && !f.Errors && len(f.Conds) == 0)
}

// ErrFiltered refuse d'ajouter des lignes à un tableau filtré : les lignes
// masquées gardent leur place sur la feuille.
type ErrFiltered struct {
	Table  string
	Column bool // c'est une colonne qu'on ajoutait
	// Choice : seul le choix de l'onglet masque des lignes (« Année :
	// 2025 ») ; « Effacer le filtre » n'y peut rien.
	Choice string
}

func (e ErrFiltered) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e ErrFiltered) Msg() msg.Message {
	id := msg.ID("table.filteredAddRows")
	if e.Choice != "" {
		id = "table.followsChoiceAddRows"
	}
	if e.Column {
		id = id[:len(id)-len("Rows")] + "Column"
	}
	return msg.M(id, msg.Args{"table": e.Table, "choice": e.Choice})
}

// errFiltered : ErrFiltered du tableau, qui dit si c'est le choix de
// l'onglet qui masque ses lignes.
func (t Table) errFiltered(column bool) error {
	e := ErrFiltered{Table: t.Name, Column: column}
	if t.choiceOnly() && t.choice != nil {
		e.Choice = t.choice.Name + " : " + t.choice.Value
	}
	return e
}

// sqlLiteral écrit un texte en littéral SQL.
func sqlLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// loadFilter lit le filtre du tableau et calcule la condition et le nombre de
// lignes affichées. Le choix de l'onglet (choice.go) s'y ajoute : un tableau
// de la feuille qui a la colonne de catégorie n'affiche que la valeur
// choisie, en plus du filtre de l'utilisateur.
func (t *tx) loadFilter(tb *Table) error {
	var raw sql.NullString
	if err := t.row(`SELECT filter FROM _lattis_tables WHERE id = ?`, tb.ID).Scan(&raw); err != nil {
		return err
	}
	tb.Filter, tb.where, tb.userWhere, tb.choice, tb.Shown = nil, "", "", nil, tb.Rows
	var conds []string
	ch, err := t.sheetChoice(tb.Sheet)
	if err != nil {
		return err
	}
	if ch != nil && !tb.NoFollow {
		tc, err := t.followChoice(*tb, ch)
		if err != nil {
			return err
		}
		if tc != nil {
			tb.choice = tc
			if !ch.All {
				conds = append(conds, tc.cond())
			}
		}
	}
	if raw.Valid && raw.String != "" {
		if err := json.Unmarshal([]byte(raw.String), &tb.Filter); err != nil {
			tb.Filter = nil // filtre illisible : ignoré
		} else if user := t.userConds(tb); len(user) > 0 {
			tb.userWhere = strings.Join(user, " AND ")
			conds = append(conds, user...)
		}
	}
	if len(conds) == 0 {
		return nil
	}
	tb.where = strings.Join(conds, " AND ")
	n, err := cached(t, "fcount:"+tb.view(), func() (int, error) {
		var n int
		err := t.row(`SELECT count(*) FROM ` + tb.view()).Scan(&n)
		return n, err
	})
	tb.Shown = n
	return err
}

// userConds : conditions SQL du filtre de l'utilisateur (tb.Filter).
func (t *tx) userConds(tb *Table) []string {
	var conds []string
	for _, c := range tb.Cols {
		f, ok := tb.Filter[strconv.FormatInt(c.ID, 10)]
		if !ok {
			continue
		}
		if f.Errors {
			conds = append(conds, fmt.Sprintf(`id IN (SELECT rid FROM _lattis_invalid WHERE tbl = %d AND col = %d)`, tb.ID, c.ID))
		}
		if f.Values != nil {
			lits := make([]string, len(f.Values))
			for i, v := range f.Values {
				lits[i] = sqlLiteral(v)
			}
			in := func(c Column) (string, error) {
				return fmt.Sprintf(`%s IN (%s)`, textExpr(c), strings.Join(lits, ", ")), nil
			}
			if c.Formula == "" {
				w, _ := in(c)
				conds = append(conds, w)
			} else if w, err := t.computedCond(*tb, c, in); err == nil && w != "" {
				conds = append(conds, w)
			}
		}
		if len(f.Conds) > 0 {
			cond := func(c Column) (string, error) { return condsSQL(*tb, c, f) }
			if c.Formula != "" {
				cond = func(cc Column) (string, error) {
					return t.computedCond(*tb, c, func(cc Column) (string, error) { return condsSQL(*tb, cc, f) })
				}
			}
			if w, err := cond(c); err == nil && w != "" {
				conds = append(conds, w)
			}
		}
	}
	return conds
}

// view est la source SQL des lignes affichées (filtrées) du tableau.
func (t Table) view() string {
	if t.where == "" {
		return t.sql()
	}
	return "(SELECT * FROM " + t.sql() + " WHERE " + t.where + ") AS " + t.sql()
}

// visibleSQL est la sous-requête des lignes affichées (sans alias), vide
// sans filtre.
func (t Table) visibleSQL() string {
	if t.where == "" {
		return ""
	}
	return "(SELECT * FROM " + t.sql() + " WHERE " + t.where + ")"
}

// Filtered indique si un filtre masque des lignes (ou pourrait en masquer) :
// celui de l'utilisateur ou le choix de l'onglet.
func (t Table) Filtered() bool { return t.where != "" }

// userFiltered : le filtre de l'utilisateur masque des lignes. Le choix de
// l'onglet n'en est pas un : on ajoute des lignes à un tableau qui le suit,
// elles reçoivent la valeur choisie.
func (t Table) userFiltered() bool { return t.userWhere != "" }

// choiceOnly : seul le choix de l'onglet masque des lignes.
func (t Table) choiceOnly() bool { return t.where != "" && t.userWhere == "" }

// userVisibleSQL est visibleSQL sans le choix de l'onglet : ce que voit
// SOUS.TOTAL dans une formule d'une autre feuille.
func (t Table) userVisibleSQL() string {
	if t.userWhere == "" {
		return ""
	}
	return "(SELECT * FROM " + t.sql() + " WHERE " + t.userWhere + ")"
}

// SetFilter remplace le filtre de la colonne i (nil : retire le filtre de la
// colonne).
func (m *Model) SetFilter(ctx context.Context, table string, i int, f *ColumnFilter) error {
	return m.Do(ctx, "Filtrer", func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(tb.Cols) {
			return msg.New("table.columnIndexOutOfRange", msg.Args{"n": i})
		}
		c := tb.Cols[i]
		if f != nil && len(f.Conds) > 0 {
			cc := c
			if c.Formula != "" {
				cc.Type = t.computedResult(tb, c)
				if cc.Type == "" {
					cc.Type = grid.Text
				}
			}
			if _, err := condsSQL(tb, cc, *f); err != nil {
				return err
			}
		}
		filter := map[string]ColumnFilter{}
		for k, v := range tb.Filter {
			filter[k] = v
		}
		key := strconv.FormatInt(c.ID, 10)
		if f.empty() {
			delete(filter, key)
		} else {
			filter[key] = *f
		}
		return t.writeFilter(tb, filter)
	})
}

// ClearFilters retire tous les filtres du tableau.
func (m *Model) ClearFilters(ctx context.Context, table string) error {
	return m.Do(ctx, "Effacer le filtre", func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		return t.writeFilter(tb, nil)
	})
}

func (t *tx) writeFilter(tb Table, filter map[string]ColumnFilter) error {
	var old sql.NullString
	if err := t.row(`SELECT filter FROM _lattis_tables WHERE id = ?`, tb.ID).Scan(&old); err != nil {
		return err
	}
	var neu, prev any
	if len(filter) > 0 {
		b, err := json.Marshal(filter)
		if err != nil {
			return err
		}
		neu = string(b)
	}
	if old.Valid {
		prev = old.String
	}
	if neu == prev {
		return nil
	}
	q := `UPDATE _lattis_tables SET filter = ? WHERE id = ?`
	return t.apply([]stmt{st(q, neu, tb.ID)}, []stmt{st(q, prev, tb.ID)})
}

// FilterValue est une valeur distincte d'une colonne, avec son nombre de lignes.
type FilterValue struct {
	Text  string `json:"text"` // vide : cellules vides
	Count int    `json:"count"`
}

// MaxFilterValues borne la liste des valeurs proposées par le filtre.
const MaxFilterValues = 1000

// FilterValues donne les valeurs distinctes de la colonne i (toutes les
// lignes, filtre compris), vides d'abord puis dans l'ordre de la colonne.
func (m *Model) FilterValues(ctx context.Context, table string, i int) ([]FilterValue, error) {
	out := []FilterValue{}
	err := m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(tb.Cols) {
			return msg.New("table.columnIndexOutOfRange", msg.Args{"n": i})
		}
		c := tb.Cols[i]
		src := tb.sql()
		if c.Formula != "" {
			// Colonne calculée : ses valeurs, lues dans la vue calculée.
			if src, c, err = t.computedView(tb, c); err != nil {
				return err
			}
		}
		col := colSQL(c.ID)
		rs, err := t.query(`SELECT `+textExpr(c)+` AS v, count(*) FROM `+src+
			` GROUP BY v ORDER BY min(`+col+`) IS NOT NULL, min(`+col+`) LIMIT ?`, MaxFilterValues)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var v FilterValue
			if err := rs.Scan(&v.Text, &v.Count); err != nil {
				return err
			}
			out = append(out, v)
		}
		return rs.Err()
	})
	// Les vides (texte "") viennent en dernier, une seule fois, comme
	// « (Vides) » dans Excel.
	slices.SortStableFunc(out, func(a, b FilterValue) int {
		switch {
		case a.Text == "" && b.Text != "":
			return 1
		case b.Text == "" && a.Text != "":
			return -1
		}
		return 0
	})
	return out, err
}

// FieldValues est FilterValues avec la colonne désignée par son nom.
func (m *Model) FieldValues(ctx context.Context, table, field string) ([]FilterValue, error) {
	i := -1
	err := m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		for k, c := range tb.Cols {
			if strings.EqualFold(c.Name, field) {
				i = k
			}
		}
		return nil
	})
	if err != nil || i < 0 {
		return []FilterValue{}, err
	}
	return m.FilterValues(ctx, table, i)
}

// textExpr est le texte affiché d'une colonne pour comparer des valeurs : la
// valeur elle-même pour un texte (rapide), slt_text sinon ; "" si vide.
func textExpr(c Column) string {
	if c.Type == grid.Text {
		return "coalesce(" + colSQL(c.ID) + ", '')"
	}
	return fmt.Sprintf("coalesce(slt_text(%s, %s), '')", colSQL(c.ID), sqlLiteral(string(c.Type)))
}
