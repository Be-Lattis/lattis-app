package canvas

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/lalamefine/lattis/internal/msg"
)

// Plan de la feuille (Données › Plan), comme dans un tableur : Grouper,
// Dissocier, Afficher et Masquer le détail, Effacer le plan.
//
// Chaque ligne (ou colonne) groupée a un niveau de 1 à MaxOutline
// (_lattis_row_outline, _lattis_col_outline). Un groupe de niveau n est
// une suite de lignes consécutives de niveau n ou plus ; sa ligne de
// synthèse est celle qui le suit (au-dessus de lui si la feuille a la
// synthèse au-dessus, outline_above). Réduire un groupe masque ses lignes :
// le masquage se déduit du plan (Outline.HiddenRows) et ne touche pas au
// masquage choisi par l'utilisateur (hd, cellformat.go).
//
// L'état réduit d'un groupe est noté sur sa ligne extrême, du côté de la
// synthèse (folded, bit n-1 pour le niveau n) : le groupe intérieur
// garde le sien quand on réduit puis rouvre le groupe qui l'entoure. Un
// bit qui ne désigne plus l'extrémité d'un groupe est oublié.
//
// Les lignes suivent les insertions et suppressions (sheetrows.go) ; des
// lignes insérées à l'intérieur d'un groupe y entrent. Les lignes de
// données d'un tableau ne se groupent pas : leur ordre change au tri et
// au filtre (Données › Sous-total donne le plan d'un tableau, subtotal.go).
//
// Classeur partagé : les niveaux sont publiés avec la mise en page ;
// l'état réduit reste propre à chaque poste (layoutOwn), comme la valeur
// d'un onglet à choix. Réduire ou rouvrir n'est donc pas une modification
// de la mise en page : permis à tous, même sur une feuille protégée.

// MaxOutline est le nombre de niveaux de groupes.
const MaxOutline = 8

// ErrOutline signale un groupe impossible.
var ErrOutline = errors.New("plan")

// OutlineLine est une ligne (ou colonne) groupée.
type OutlineLine struct {
	Index  int `json:"index"`
	Level  int `json:"level"`
	Folded int `json:"folded,omitempty"`
}

// Outline est le plan d'une feuille, avec les lignes et colonnes que ses
// groupes réduits masquent.
type Outline struct {
	Rows []OutlineLine `json:"rows"`
	Cols []OutlineLine `json:"cols"`
	// RowsAbove, ColsLeft : synthèse au-dessus des détails (à leur gauche).
	RowsAbove  bool  `json:"rowsAbove,omitempty"`
	ColsLeft   bool  `json:"colsLeft,omitempty"`
	HiddenRows []int `json:"hiddenRows"`
	HiddenCols []int `json:"hiddenCols"`
}

// OutlineGroup est un groupe : lignes From..To de niveau Level.
type OutlineGroup struct {
	Level, From, To int
	Folded          bool
}

func (a axis) outline() string {
	if a == rowsAxis {
		return "_lattis_row_outline"
	}
	return "_lattis_col_outline"
}

// outlineAxis : l'axe d'une commande (lignes ou colonnes).
func outlineAxis(rows bool) axis {
	if rows {
		return rowsAxis
	}
	return colsAxis
}

// marker : la ligne qui porte l'état réduit du groupe.
func (g OutlineGroup) marker(before bool) int {
	if before {
		return g.From
	}
	return g.To
}

// OutlineGroups donne les groupes de lignes groupées (triées ou non), du
// niveau 1 au plus profond ; before : synthèse avant les détails.
func OutlineGroups(lines []OutlineLine, before bool) []OutlineGroup {
	ls := slices.Clone(lines)
	slices.SortFunc(ls, func(a, b OutlineLine) int { return a.Index - b.Index })
	folded := map[int]int{}
	for _, l := range ls {
		folded[l.Index] = l.Folded
	}
	var out []OutlineGroup
	for lv := 1; lv <= MaxOutline; lv++ {
		start, prev := -1, -2
		flush := func() {
			if start >= 0 {
				g := OutlineGroup{Level: lv, From: start, To: prev}
				g.Folded = folded[g.marker(before)]&(1<<(lv-1)) != 0
				out = append(out, g)
			}
			start = -1
		}
		for _, l := range ls {
			if l.Level < lv {
				continue
			}
			if l.Index != prev+1 {
				flush()
			}
			if start < 0 {
				start = l.Index
			}
			prev = l.Index
		}
		flush()
	}
	return out
}

// OutlineHidden donne les lignes masquées par les groupes réduits.
func OutlineHidden(lines []OutlineLine, before bool) []int {
	seen := map[int]bool{}
	for _, g := range OutlineGroups(lines, before) {
		if g.Folded {
			for i := g.From; i <= g.To; i++ {
				seen[i] = true
			}
		}
	}
	out := make([]int, 0, len(seen))
	for i := range seen {
		out = append(out, i)
	}
	slices.Sort(out)
	return out
}

// settleFolds oublie les bits d'état qui ne désignent plus l'extrémité
// d'un groupe de leur niveau.
func settleFolds(lines map[int]OutlineLine, before bool) {
	all := make([]OutlineLine, 0, len(lines))
	for _, l := range lines {
		all = append(all, l)
	}
	keep := map[int]int{}
	for _, g := range OutlineGroups(all, before) {
		keep[g.marker(before)] |= 1 << (g.Level - 1)
	}
	for i, l := range lines {
		l.Folded &= keep[i]
		lines[i] = l
	}
}

// outlineLines lit les lignes groupées d'un axe de la feuille.
func (t *tx) outlineLines(sheet int64, a axis) (map[int]OutlineLine, error) {
	rs, err := t.query(fmt.Sprintf(`SELECT %s, level, folded FROM %s WHERE sheet = ?`, a, a.outline()), sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[int]OutlineLine{}
	for rs.Next() {
		var l OutlineLine
		if err := rs.Scan(&l.Index, &l.Level, &l.Folded); err != nil {
			return nil, err
		}
		out[l.Index] = l
	}
	return out, rs.Err()
}

// outlineBefore : la synthèse est avant les détails sur cet axe.
func (t *tx) outlineBefore(sheet int64, a axis) (bool, error) {
	var v int
	if err := t.row(`SELECT coalesce((SELECT outline_above FROM _lattis_sheets WHERE id = ?), 0)`, sheet).Scan(&v); err != nil {
		return false, err
	}
	if a == rowsAxis {
		return v&1 != 0, nil
	}
	return v&2 != 0, nil
}

// writeOutline remplace les lignes groupées de l'axe par neu (comparées à
// old ; une étape d'annulation).
func (t *tx) writeOutline(sheet int64, a axis, old, neu map[int]OutlineLine) error {
	del := fmt.Sprintf(`DELETE FROM %s WHERE sheet = ? AND %s = ?`, a.outline(), a)
	ins := fmt.Sprintf(`INSERT INTO %s (sheet, %s, level, folded) VALUES (?, ?, ?, ?)`, a.outline(), a)
	var do, undo []stmt
	keys := map[int]bool{}
	for i := range old {
		keys[i] = true
	}
	for i := range neu {
		keys[i] = true
	}
	for _, i := range slices.Sorted(func(yield func(int) bool) {
		for k := range keys {
			if !yield(k) {
				return
			}
		}
	}) {
		o, had := old[i]
		n, has := neu[i]
		if has && n.Level <= 0 {
			has = false
		}
		if had == has && (!has || o == n) {
			continue
		}
		do = append(do, st(del, sheet, i))
		undo = append(undo, st(del, sheet, i))
		if has {
			do = append(do, st(ins, sheet, i, n.Level, n.Folded))
		}
		if had {
			undo = append(undo, st(ins, sheet, i, o.Level, o.Folded))
		}
	}
	if len(do) == 0 {
		return nil
	}
	return t.apply(do, undo)
}

// editOutline change le plan d'un axe : edit reçoit une copie des lignes
// groupées ; les bits d'état devenus sans objet sont oubliés.
func (t *tx) editOutline(sheet int64, a axis, edit func(lines map[int]OutlineLine) error) error {
	old, err := t.outlineLines(sheet, a)
	if err != nil {
		return err
	}
	before, err := t.outlineBefore(sheet, a)
	if err != nil {
		return err
	}
	neu := map[int]OutlineLine{}
	for i, l := range old {
		neu[i] = l
	}
	if err := edit(neu); err != nil {
		return err
	}
	for i, l := range neu {
		if l.Level <= 0 {
			delete(neu, i)
		}
	}
	settleFolds(neu, before)
	return t.writeOutline(sheet, a, old, neu)
}

// guardOutlineRows refuse de grouper des lignes qui traversent un tableau.
func (t *tx) guardOutlineRows(sheet int64, i0, i1 int) error {
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	for _, tb := range tables {
		if i0 <= tb.Bottom() && i1 >= tb.Top() {
			return msg.Wrap(ErrOutline, "outline.table", msg.Args{"table": tb.Name})
		}
	}
	return nil
}

// GroupLines groupe (Grouper) les lignes (rows) ou les colonnes [i0, i1] :
// elles descendent d'un niveau dans le plan.
func (m *Model) GroupLines(ctx context.Context, sheet int64, rows bool, i0, i1 int) error {
	if i0 > i1 {
		i0, i1 = i1, i0
	}
	if i0 < 0 || i1-i0+1 > MaxStyleLines {
		return msg.Wrap(ErrOutline, "outline.tooMany", msg.Args{"max": MaxStyleLines})
	}
	a := outlineAxis(rows)
	return m.Do(ctx, "Grouper", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		if rows {
			if err := t.guardOutlineRows(sheet, i0, i1); err != nil {
				return err
			}
		}
		return t.editOutline(sheet, a, func(lines map[int]OutlineLine) error {
			for i := i0; i <= i1; i++ {
				l := lines[i]
				if l.Level >= MaxOutline {
					return msg.Wrap(ErrOutline, "outline.maxLevels", msg.Args{"max": MaxOutline})
				}
				l.Index, l.Level = i, l.Level+1
				lines[i] = l
			}
			return nil
		})
	})
}

// UngroupLines dissocie (Dissocier) les lignes ou colonnes [i0, i1] : les
// groupées remontent d'un niveau.
func (m *Model) UngroupLines(ctx context.Context, sheet int64, rows bool, i0, i1 int) error {
	if i0 > i1 {
		i0, i1 = i1, i0
	}
	a := outlineAxis(rows)
	return m.Do(ctx, "Dissocier", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		return t.editOutline(sheet, a, func(lines map[int]OutlineLine) error {
			n := 0
			for i, l := range lines {
				if i >= i0 && i <= i1 {
					l.Level--
					lines[i] = l
					n++
				}
			}
			if n == 0 {
				return msg.Wrap(ErrOutline, "outline.none", nil)
			}
			return nil
		})
	})
}

// ClearOutline efface le plan de la feuille (lignes et colonnes) : les
// groupes réduits se rouvrent.
func (m *Model) ClearOutline(ctx context.Context, sheet int64) error {
	return m.Do(ctx, "Effacer le plan", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		n := 0
		for _, a := range []axis{rowsAxis, colsAxis} {
			var k int
			if err := t.row(`SELECT count(*) FROM `+a.outline()+` WHERE sheet = ?`, sheet).Scan(&k); err != nil {
				return err
			}
			n += k
			if err := t.deleteWhere(a.outline(), `sheet = ?`, sheet); err != nil {
				return err
			}
		}
		if n == 0 {
			return msg.Wrap(ErrOutline, "outline.none", nil)
		}
		return nil
	})
}

// FoldGroup réduit (fold, Masquer le détail) ou rouvre (Afficher le
// détail) le groupe de niveau level qui contient la ligne line.
func (m *Model) FoldGroup(ctx context.Context, sheet int64, rows bool, level, line int, fold bool) error {
	label := "Afficher le détail"
	if fold {
		label = "Masquer le détail"
	}
	a := outlineAxis(rows)
	return m.Do(ctx, label, func(t *tx) error {
		before, err := t.outlineBefore(sheet, a)
		if err != nil {
			return err
		}
		return t.editOutline(sheet, a, func(lines map[int]OutlineLine) error {
			all := make([]OutlineLine, 0, len(lines))
			for _, l := range lines {
				all = append(all, l)
			}
			for _, g := range OutlineGroups(all, before) {
				if g.Level != level || line < g.From || line > g.To {
					continue
				}
				k := g.marker(before)
				l := lines[k]
				if fold {
					l.Folded |= 1 << (level - 1)
				} else {
					l.Folded &^= 1 << (level - 1)
				}
				lines[k] = l
				return nil
			}
			return msg.Wrap(ErrOutline, "outline.none", nil)
		})
	})
}

// ShowOutlineLevel affiche le plan jusqu'au niveau level (boutons 1 2 3…
// du coin) : les groupes de niveau level et plus sont réduits, les autres
// rouverts.
func (m *Model) ShowOutlineLevel(ctx context.Context, sheet int64, rows bool, level int) error {
	a := outlineAxis(rows)
	return m.Do(ctx, "Afficher le niveau", func(t *tx) error {
		before, err := t.outlineBefore(sheet, a)
		if err != nil {
			return err
		}
		return t.editOutline(sheet, a, func(lines map[int]OutlineLine) error {
			all := make([]OutlineLine, 0, len(lines))
			for i, l := range lines {
				l.Folded = 0
				lines[i] = l
				all = append(all, l)
			}
			for _, g := range OutlineGroups(all, before) {
				if g.Level >= level {
					k := g.marker(before)
					l := lines[k]
					l.Folded |= 1 << (g.Level - 1)
					lines[k] = l
				}
			}
			return nil
		})
	})
}

// outlineOf lit le plan de la feuille (nil sans groupe).
func (t *tx) outlineOf(sheet int64) (*Outline, error) {
	o := &Outline{Rows: []OutlineLine{}, Cols: []OutlineLine{}}
	var above int
	if err := t.row(`SELECT coalesce((SELECT outline_above FROM _lattis_sheets WHERE id = ?), 0)`, sheet).Scan(&above); err != nil {
		return nil, err
	}
	o.RowsAbove, o.ColsLeft = above&1 != 0, above&2 != 0
	for _, a := range []axis{rowsAxis, colsAxis} {
		lines, err := t.outlineLines(sheet, a)
		if err != nil {
			return nil, err
		}
		ls := make([]OutlineLine, 0, len(lines))
		for _, l := range lines {
			ls = append(ls, l)
		}
		slices.SortFunc(ls, func(x, y OutlineLine) int { return x.Index - y.Index })
		if a == rowsAxis {
			o.Rows, o.HiddenRows = ls, OutlineHidden(ls, o.RowsAbove)
		} else {
			o.Cols, o.HiddenCols = ls, OutlineHidden(ls, o.ColsLeft)
		}
	}
	if len(o.Rows) == 0 && len(o.Cols) == 0 {
		return nil, nil
	}
	return o, nil
}

// outlineInserted : des lignes insérées entre deux lignes d'un même groupe
// y entrent (le plus petit des niveaux qui les bordent).
func (t *tx) outlineInserted(sheet int64, a axis, at, n int) error {
	if at <= 0 {
		return nil
	}
	var above, below int
	q := fmt.Sprintf(`SELECT coalesce((SELECT level FROM %[1]s WHERE sheet = ?1 AND %[2]s = ?2), 0), coalesce((SELECT level FROM %[1]s WHERE sheet = ?1 AND %[2]s = ?3), 0)`, a.outline(), a)
	if err := t.row(q, sheet, at-1, at+n).Scan(&above, &below); err != nil {
		return err
	}
	lv := min(above, below)
	if lv <= 0 {
		return nil
	}
	ins := fmt.Sprintf(`INSERT INTO %s (sheet, %s, level, folded) VALUES (?, ?, ?, 0)`, a.outline(), a)
	del := fmt.Sprintf(`DELETE FROM %s WHERE sheet = ? AND %s = ?`, a.outline(), a)
	var do, undo []stmt
	for i := at; i < at+n; i++ {
		do = append(do, st(ins, sheet, i, lv))
		undo = append(undo, st(del, sheet, i))
	}
	return t.apply(do, undo)
}
