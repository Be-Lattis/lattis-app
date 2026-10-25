package canvas

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// Données › Sous-total, comme dans un tableur, sans toucher au tableau.
//
// Un tableau Lattis est une table SQL : y insérer des lignes de total en
// mêlerait aux données (tri, filtre, formules, partage). Le résultat va
// donc sur une feuille à côté (« Sous-totaux Ventes ») : les lignes
// affichées du tableau (son filtre et son ordre), coupées à chaque
// changement de la colonne choisie, une ligne =SOUS.TOTAL(9;Cell[C2:C5])
// par groupe et un total général, avec le plan 1 2 3 (outline.go) pour
// réduire aux totaux. Comme dans un tableur, c'est un instantané : on le
// refait (Remplacer les sous-totaux existants) quand les données changent.
// Le total général ne relit que les lignes de détail (une plage par
// groupe), SOUS.TOTAL n'ignorant pas ici les sous-totaux qu'il traverse.

// MaxSubtotalGroups borne le nombre de groupes (une plage par groupe dans
// la formule du total général).
const MaxSubtotalGroups = 1000

// SubtotalOptions : la boîte Sous-total.
type SubtotalOptions struct {
	Table string `json:"table"`
	// By : à chaque changement de cette colonne.
	By string `json:"by"`
	// Func : code de SOUS.TOTAL (1 à 11 : 9 somme, 3 nombre, 1 moyenne…).
	Func int `json:"func"`
	// Columns : colonnes où ajouter un sous-total.
	Columns []string `json:"columns"`
	// Replace : refaire la feuille des sous-totaux du tableau (sinon une
	// nouvelle) ; PageBreaks : saut de page entre les groupes ; Below :
	// synthèse sous les données (sinon au-dessus).
	Replace    bool `json:"replace"`
	PageBreaks bool `json:"pageBreaks"`
	Below      bool `json:"below"`
}

// Subtotal pose les sous-totaux d'un tableau sur une feuille à côté de la
// sienne ; rend la feuille (une étape d'annulation).
func (m *Model) Subtotal(ctx context.Context, o SubtotalOptions) (int64, error) {
	var sheet int64
	err := m.Do(ctx, "Sous-total", func(t *tx) error {
		var err error
		sheet, err = t.subtotal(o)
		return err
	})
	return sheet, err
}

// stRow est une ligne de la feuille des sous-totaux : cellules (raw) et
// styles, niveau dans le plan.
type stRow struct {
	raw   []string
	style []string
	level int
	start bool // première ligne d'un groupe (saut de page avant)
}

func (t *tx) subtotal(o SubtotalOptions) (int64, error) {
	if o.Func < 1 || o.Func > 11 {
		return 0, msg.Wrap(ErrTable, "subtotal.func", nil)
	}
	tb, err := t.tableByName(o.Table)
	if err != nil {
		return 0, err
	}
	col := func(name string) int {
		return slices.IndexFunc(tb.Cols, func(c Column) bool { return strings.EqualFold(c.Name, name) })
	}
	by := col(o.By)
	if by < 0 {
		return 0, msg.Wrap(ErrTable, "table.noSuchColumn", msg.Args{"table": tb.Name, "column": o.By})
	}
	totals := map[int]bool{}
	for _, c := range o.Columns {
		i := col(c)
		if i < 0 {
			return 0, msg.Wrap(ErrTable, "table.noSuchColumn", msg.Args{"table": tb.Name, "column": c})
		}
		if i != by {
			totals[i] = true
		}
	}
	if len(totals) == 0 {
		return 0, msg.Wrap(ErrTable, "subtotal.noColumn", nil)
	}
	if err := t.guardWorkbook(); err != nil {
		return 0, err
	}
	head, lines, err := t.shownCells(tb)
	if err != nil {
		return 0, err
	}
	n := len(tb.Cols)
	if (len(lines)*2+2)*n > MaxRangeCells {
		return 0, msg.Wrap(ErrTable, "table.tooLargeForRange", msg.Args{"table": tb.Name, "n": len(lines) * n, "max": MaxRangeCells})
	}
	// Groupes : à chaque changement de la colonne By.
	type group struct{ from, to int }
	var groups []group
	for i, l := range lines {
		if i == 0 || l.raw[by] != lines[i-1].raw[by] {
			groups = append(groups, group{i, i})
		} else {
			groups[len(groups)-1].to = i
		}
	}
	if len(groups) > MaxSubtotalGroups {
		return 0, msg.Wrap(ErrTable, "subtotal.tooManyGroups", msg.Args{"max": MaxSubtotalGroups})
	}
	bold := func(s string) string {
		st := parseStyle(s)
		st["b"] = true
		out, _ := encodeStyle(st)
		return out
	}
	lang := t.nameLang()
	// Place des lignes : en-tête (ligne 0), puis groupes ; le total général
	// en tête (synthèse au-dessus) ou à la fin.
	rows := []stRow{{raw: head.raw, style: head.style}}
	first := 1
	if !o.Below {
		first = 2
	}
	at := first
	ranges := map[int][]string{} // plages de détail, par colonne de total
	sumRow := func(label string, spans map[int][]string) stRow {
		r := stRow{raw: make([]string, n), style: make([]string, n)}
		for c := range n {
			r.style[c] = bold(head.colStyle[c])
		}
		r.raw[by] = textRaw(label)
		for c := range totals {
			r.raw[c] = fmt.Sprintf("=SOUS.TOTAL(%d;%s)", o.Func, strings.Join(spans[c], ";"))
		}
		return r
	}
	var body []stRow
	for _, g := range groups {
		size := g.to - g.from + 1
		top := at
		if !o.Below {
			top = at + 1
		}
		spans := map[int][]string{}
		for c := range totals {
			s := fmt.Sprintf("Cell[%s%d:%s%d]", ColLetters(c), top+1, ColLetters(c), top+size)
			spans[c] = []string{s}
			ranges[c] = append(ranges[c], s)
		}
		label := msg.Format(lang, "subtotal.total", msg.Args{"value": groupLabel(lines[g.from].raw[by])})
		sub := sumRow(label, spans)
		sub.level = 1
		var det []stRow
		for i := g.from; i <= g.to; i++ {
			l := lines[i]
			det = append(det, stRow{raw: l.raw, style: l.style, level: 2})
		}
		if o.Below {
			det[0].start = true
			body = append(body, det...)
			body = append(body, sub)
		} else {
			sub.start = true
			body = append(body, sub)
			body = append(body, det...)
		}
		at += size + 1
	}
	grand := sumRow(msg.Format(lang, "subtotal.grand", nil), ranges)
	if len(groups) == 0 {
		for c := range totals {
			grand.raw[c] = ""
		}
	}
	if o.Below {
		rows = append(append(rows, body...), grand)
	} else {
		rows = append(append(rows, grand), body...)
	}
	// La feuille : celle des sous-totaux du tableau (refaite), sinon une nouvelle.
	sheet, err := t.subtotalSheet(tb, o.Replace)
	if err != nil {
		return 0, err
	}
	var do []stmt
	var breaks []int
	for r, row := range rows {
		for c, raw := range row.raw {
			if raw != "" {
				do = append(do, st(`INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (?, ?, ?, ?)`, sheet, r, c, raw))
			}
		}
		if o.PageBreaks && row.start && r > first {
			breaks = append(breaks, r)
		}
	}
	if len(do) > 0 {
		if err := t.apply(do, []stmt{st(`DELETE FROM _lattis_cells WHERE sheet = ?`, sheet)}); err != nil {
			return 0, err
		}
	}
	for r, row := range rows {
		for c, s := range row.style {
			if s != "" {
				if err := t.putFreeStyle(sheet, r, c, s); err != nil {
					return 0, err
				}
			}
		}
	}
	if err := t.markRange(sheet, 0, 0, len(rows)-1, n-1); err != nil {
		return 0, err
	}
	// Largeurs des colonnes du tableau.
	for c := range n {
		var w int
		if err := t.row(`SELECT coalesce((SELECT width FROM _lattis_col_widths WHERE sheet = ? AND col = ?), 0)`, tb.Sheet, tb.Col+c).Scan(&w); err != nil {
			return 0, err
		}
		if w > 0 {
			if err := t.setLineSize(`_lattis_col_widths`, `col`, `width`, sheet, c, w); err != nil {
				return 0, err
			}
		}
	}
	// Plan : détails au niveau 2, sous-totaux au niveau 1.
	if err := t.editOutline(sheet, rowsAxis, func(lines map[int]OutlineLine) error {
		for r, row := range rows {
			if row.level > 0 {
				lines[r] = OutlineLine{Index: r, Level: row.level}
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	above := 0
	if !o.Below {
		above = 1
	}
	if err := t.apply([]stmt{st(`UPDATE _lattis_sheets SET outline_above = ? WHERE id = ?`, above, sheet)},
		[]stmt{st(`UPDATE _lattis_sheets SET outline_above = 0 WHERE id = ?`, sheet)}); err != nil {
		return 0, err
	}
	if len(breaks) > 0 {
		p, err := t.pageSetup(sheet)
		if err != nil {
			return 0, err
		}
		p.RowBreaks = breaks
		if err := p.Check(); err != nil {
			return 0, err
		}
		if err := t.setPageSetup(sheet, p); err != nil {
			return 0, err
		}
	}
	return sheet, nil
}

// groupLabel : la valeur d'un groupe telle qu'affichée (sans l'apostrophe
// d'un texte) ; « (vide) » sans valeur.
func groupLabel(raw string) string {
	return strings.TrimPrefix(raw, "'")
}

// shownCells lit les lignes affichées d'un tableau (filtre, ordre) en
// saisies de cellules libres, avec leur mise en forme.
type shownCell struct {
	raw, style, colStyle []string
}

func (t *tx) shownCells(tb Table) (head shownCell, lines []shownCell, err error) {
	n := len(tb.Cols)
	head = shownCell{raw: make([]string, n), style: make([]string, n), colStyle: make([]string, n)}
	colStyles := make([]Style, n)
	enc := func(s Style) string {
		for k, v := range s {
			if v == nil || notFormat[k] {
				delete(s, k)
			}
		}
		if len(s) == 0 {
			return ""
		}
		o, _ := encodeStyle(s)
		return o
	}
	for i, c := range tb.Cols {
		cs, err := t.tableStyle(tb.ID, 0, c.ID)
		if err != nil {
			return head, nil, err
		}
		colStyles[i] = cs
		head.raw[i] = textRaw(c.Name)
		hs := Style{"b": true, "bb": true}
		head.style[i] = enc(hs)
		head.colStyle[i] = enc(over(cs))
	}
	exprs, args, _, err := t.columnSQL(tb)
	if err != nil {
		return head, nil, err
	}
	sel := []string{"id"}
	for i, c := range tb.Cols {
		if c.Formula != "" {
			sel = append(sel, exprs[i])
			continue
		}
		sel = append(sel, fmt.Sprintf(`coalesce(slt_text(%s, '%s'), (SELECT raw FROM _lattis_invalid i WHERE i.tbl = %d AND i.rid = %s.id AND i.col = %d), '')`,
			tb.sql()+"."+colSQL(c.ID), string(c.Type), tb.ID, tb.sql(), c.ID))
	}
	rs, err := t.query(`SELECT `+strings.Join(sel, ", ")+` FROM `+tb.view()+` ORDER BY _pos`, args...)
	if err != nil {
		return head, nil, err
	}
	type line struct {
		rid  int64
		vals []any
	}
	var got []line
	for rs.Next() {
		l := line{vals: make([]any, n)}
		ptrs := []any{&l.rid}
		for i := range l.vals {
			ptrs = append(ptrs, &l.vals[i])
		}
		if err := rs.Scan(ptrs...); err != nil {
			rs.Close()
			return head, nil, err
		}
		got = append(got, l)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return head, nil, err
	}
	for _, l := range got {
		out := shownCell{raw: make([]string, n), style: make([]string, n)}
		for i, c := range tb.Cols {
			if c.Formula != "" {
				out.raw[i] = valueRaw(formula.FromResult(normalize([]any{l.vals[i]})[0]))
			} else if s, ok := normalize([]any{l.vals[i]})[0].(string); ok {
				if c.Type == "text" {
					out.raw[i] = textRaw(s)
				} else {
					out.raw[i] = freeRaw(s)
				}
			}
			cs, err := t.tableStyle(tb.ID, l.rid, c.ID)
			if err != nil {
				return head, nil, err
			}
			out.style[i] = enc(over(colStyles[i], cs))
		}
		lines = append(lines, out)
	}
	return head, lines, nil
}

// subtotalSheet donne la feuille des sous-totaux d'un tableau, placée après
// la sienne : celle du même nom vidée (replace, sans tableau ni objet),
// sinon une nouvelle (« Sous-totaux Ventes (2) »).
func (t *tx) subtotalSheet(tb Table, replace bool) (int64, error) {
	base := msg.Format(t.nameLang(), "subtotal.sheet", msg.Args{"table": tb.Name})
	if r := []rune(base); len(r) > 31 {
		base = string(r[:31])
	}
	if replace {
		var id int64
		err := t.row(`SELECT id FROM _lattis_sheets WHERE name = ? COLLATE NOCASE
			AND NOT EXISTS (SELECT 1 FROM _lattis_pages WHERE sheet = _lattis_sheets.id)
			AND NOT EXISTS (SELECT 1 FROM _lattis_tables WHERE sheet = _lattis_sheets.id)
			AND NOT EXISTS (SELECT 1 FROM _lattis_pivots WHERE sheet = _lattis_sheets.id)
			AND NOT EXISTS (SELECT 1 FROM _lattis_charts WHERE sheet = _lattis_sheets.id)`, base).Scan(&id)
		if err == nil {
			for _, tbl := range []string{`_lattis_cells`, `_lattis_cell_styles`, `_lattis_range_cells`, `_lattis_row_outline`, `_lattis_col_outline`, `_lattis_col_widths`, `_lattis_page_setup`} {
				if err := t.deleteWhere(tbl, `sheet = ?`, id); err != nil {
					return 0, err
				}
			}
			return id, nil
		}
	}
	name := base
	for k := 2; ; k++ {
		taken, err := t.sheetNameTaken(name, 0)
		if err != nil {
			return 0, err
		}
		if !taken {
			break
		}
		name = base + " (" + strconv.Itoa(k) + ")"
	}
	var id int64
	var pos int
	if err := t.row(`SELECT coalesce(max(id), 0) + 1, coalesce(max(pos), -1) + 1 FROM _lattis_sheets`).Scan(&id, &pos); err != nil {
		return 0, err
	}
	if err := t.apply(
		[]stmt{st(`INSERT INTO _lattis_sheets (id, name, pos, gridlines) VALUES (?, ?, ?, 1)`, id, name, pos)},
		[]stmt{st(`DELETE FROM _lattis_sheets WHERE id = ?`, id)},
	); err != nil {
		return 0, err
	}
	ids, err := t.sheetOrder()
	if err != nil {
		return 0, err
	}
	if at := slices.Index(ids, tb.Sheet); at >= 0 {
		if err := t.moveSheet(id, at+1); err != nil {
			return 0, err
		}
	}
	return id, nil
}
