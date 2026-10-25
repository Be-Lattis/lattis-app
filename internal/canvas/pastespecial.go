package canvas

import (
	"context"
	"math"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/work"
)

// Collage spécial (Ctrl+Alt+V), comme la boîte d'un tableur : coller les
// valeurs, les formules, la mise en forme, les valeurs et leur format de
// nombre ou les largeurs de colonnes d'une copie, en les combinant au
// besoin à la destination (Addition, Soustraction, Multiplication,
// Division), sans les cellules vides de la source, ou transposé (lignes et
// colonnes échangées). Une seule étape d'annulation.
//
// La source est une zone de Lattis (PasteSpecial, même sélection que
// Transfer) ou un bloc venu d'ailleurs (PasteBlock : texte tabulé et, copié
// d'un tableur, la mise en forme lue dans sa version HTML). Les règles des
// tableaux sont celles du collage ordinaire : une colonne calculée garde sa
// formule ; une formule de cellule libre entre dans un tableau par sa valeur ;
// la mise en forme posée sur toutes les lignes d'une colonne de tableau
// devient celle de la colonne (lignes à venir comprises), comme ApplyStyle.

// PasteOptions sont les choix du collage spécial.
type PasteOptions struct {
	// What : "all" (ou ""), "values", "formulas", "formats", "valuesNumbers"
	// (valeurs et formats de nombre), "widths" (largeurs de colonnes).
	What string `json:"what"`
	// Op : "" (aucune), "add", "sub", "mul", "div".
	Op string `json:"op"`
	// SkipBlanks : les cellules vides de la source ne remplacent rien.
	SkipBlanks bool `json:"skipBlanks"`
	// Transpose : lignes et colonnes échangées.
	Transpose bool `json:"transpose"`
}

func (o PasteOptions) what() string {
	if o.What == "" {
		return "all"
	}
	return o.What
}

// plain : un collage ordinaire (tout, sans option).
func (o PasteOptions) plain() bool {
	return o.what() == "all" && o.Op == "" && !o.SkipBlanks && !o.Transpose
}

// content : le collage écrit des valeurs ou des formules.
func (o PasteOptions) content() bool {
	switch o.what() {
	case "all", "values", "formulas", "valuesNumbers":
		return true
	}
	return false
}

// pasteCell est une cellule de la source d'un collage spécial.
type pasteCell struct {
	value string // valeur affichée, dans la forme gardée (résultat d'une formule)
	raw   string // saisie d'une cellule libre (formule comprise) ; "" : value
	style Style  // mise en forme (fusion comprise : mr, mc)
}

func (c pasteCell) blank() bool {
	return strings.TrimSpace(c.value) == "" && strings.TrimSpace(c.raw) == ""
}

// notFormat : propriétés de style qui ne sont pas de la mise en forme
// (cellformat.go) ; mr et mc (fusion) sont traités à part.
var notFormat = map[string]bool{"nt": true, "np": true, "dv": true, "mr": true, "mc": true, "hd": true, "cf": true, "ln": true}

// formatKeys : les propriétés de mise en forme connues, retirées de la
// destination quand la source ne les a pas (« format » : format de nombre
// personnalisé, toléré s'il n'est pas encore lu).
var formatKeys = []string{"b", "i", "u", "x", "ff", "fs", "fg", "bg", "al", "va", "wr", "nf", "dp", "ng", "df", "bt", "bb", "bl", "br", "bc", "format"}

// numberKeys : les propriétés du format de nombre (Valeurs et formats de nombre).
var numberKeys = map[string]bool{"nf": true, "dp": true, "ng": true, "df": true, "format": true}

// formatPatch : le correctif qui donne à une cellule la mise en forme s de
// la source (les propriétés absentes de s sont retirées) ; numbers : le
// format de nombre seulement.
func formatPatch(s Style, numbers bool) Style {
	out := Style{}
	for _, k := range formatKeys {
		out[k] = ""
	}
	for k, v := range s {
		if notFormat[k] {
			continue
		}
		if v == nil {
			v = ""
		}
		out[k] = v
	}
	if numbers {
		for k := range out {
			if !numberKeys[k] {
				delete(out, k)
			}
		}
	}
	return out
}

// PasteSpecial colle la zone src de la feuille from (copiée dans Lattis) sur
// la feuille to, à partir du coin de dst (une sélection multiple du bloc le
// reçoit répété, comme Transfer). values est le bloc affiché de la source.
func (m *Model) PasteSpecial(ctx context.Context, from int64, src Rect, to int64, dst Rect, opt PasteOptions, values [][]string) (Event, error) {
	var ev Event
	work.Op(ctx, "write", "")
	if src.R0 > src.R1 || src.C0 > src.C1 || dst.R0 < 0 || dst.C0 < 0 {
		return ev, msg.Wrap(ErrTable, "table.areaOutOfSheet", nil)
	}
	if opt.plain() {
		return m.Transfer(ctx, from, src, to, dst, false, values)
	}
	values = m.ui(ctx).valuesIn(values)
	if opt.what() == "widths" {
		return ev, m.pasteWidths(ctx, from, src, to, dst)
	}
	err := m.Do(ctx, "Collage spécial", func(t *tx) error {
		g, err := t.pasteSource(from, src, values)
		if err != nil {
			return err
		}
		return t.pasteGrid(to, dst, g, opt, &ev)
	})
	return ev, err
}

// PasteBlock colle un bloc venu d'ailleurs (presse-papiers) sur la feuille,
// à partir du coin de dst : block est le texte des cellules (dans la langue
// de la session, comme Paste), styles leur mise en forme (JSON de Style,
// "" : aucune ; fusion comprise), lue dans la version HTML d'une copie
// faite dans un tableur. Sans option, c'est un collage ordinaire (Paste :
// îlots, tableaux, en-têtes), mise en forme comprise.
func (m *Model) PasteBlock(ctx context.Context, sheet int64, dst Rect, block [][]string, styles [][]string, opt PasteOptions) (Event, error) {
	var ev Event
	if len(block) == 0 || dst.R0 < 0 || dst.C0 < 0 {
		return ev, nil
	}
	if opt.what() == "widths" {
		return ev, nil // pas de largeurs dans un texte
	}
	g := make([][]pasteCell, len(block))
	for r, line := range block {
		g[r] = make([]pasteCell, len(line))
		for c, v := range line {
			g[r][c] = pasteCell{value: v, raw: v}
			if r < len(styles) && c < len(styles[r]) {
				g[r][c].style = parseStyle(styles[r][c])
			}
		}
	}
	if opt.plain() {
		// Le bloc répété sur la sélection, puis le collage ordinaire, et sa
		// mise en forme dans la même étape d'annulation.
		h, w := len(g), gridWidth(g)
		nr, nc := tiles(h, w, dst)
		area := Rect{dst.R0, dst.C0, dst.R0 + h*nr - 1, dst.C0 + w*nc - 1}
		tiled := make([][]string, area.height())
		for i := range tiled {
			tiled[i] = make([]string, area.width())
			for j := range tiled[i] {
				tiled[i][j] = cellOf(g, i%h, j%w).value
			}
		}
		if !hasStyles(g) {
			return m.Paste(ctx, sheet, dst.R0, dst.C0, tiled)
		}
		return m.paste(ctx, sheet, dst.R0, dst.C0, tiled, func(t *tx) error {
			style := func(r, c int) Style { return cellOf(g, (r-area.R0)%h, (c-area.C0)%w).style }
			if err := t.pasteStyles(sheet, area, style, false); err != nil {
				return err
			}
			return t.pasteLinks(sheet, area, style)
		})
	}
	u := m.ui(ctx)
	for r := range g {
		for c := range g[r] {
			v, err := u.input(g[r][c].value)
			if err != nil {
				return ev, err
			}
			g[r][c].value, g[r][c].raw = v, v
		}
	}
	err := m.Do(ctx, "Collage spécial", func(t *tx) error { return t.pasteGrid(sheet, dst, g, opt, &ev) })
	return ev, err
}

func gridWidth(g [][]pasteCell) int {
	w := 0
	for _, r := range g {
		w = max(w, len(r))
	}
	return w
}

func cellOf(g [][]pasteCell, i, j int) pasteCell {
	if i < len(g) && j < len(g[i]) {
		return g[i][j]
	}
	return pasteCell{}
}

func hasStyles(g [][]pasteCell) bool {
	for _, r := range g {
		for _, c := range r {
			if len(c.style) > 0 {
				return true
			}
		}
	}
	return false
}

// pasteSource lit la zone src : valeur affichée, saisie des cellules libres,
// mise en forme effective (lignes et colonnes de la feuille, colonne de
// tableau, cellule).
func (t *tx) pasteSource(sheet int64, src Rect, values [][]string) ([][]pasteCell, error) {
	h, w := src.height(), src.width()
	if h*w > MaxPasteCells {
		return nil, ErrPasteTooBig{h * w}
	}
	g := make([][]pasteCell, h)
	for i := range g {
		g[i] = make([]pasteCell, w)
		for j := range g[i] {
			if i < len(values) && j < len(values[i]) {
				g[i][j].value = values[i][j]
			}
		}
	}
	lines, err := t.sheetLines(sheet)
	if err != nil {
		return nil, err
	}
	tables, err := t.tables(sheet)
	if err != nil {
		return nil, err
	}
	in := `sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`
	args := []any{sheet, src.R0, src.R1, src.C0, src.C1}
	for _, q := range []struct {
		sql string
		set func(*pasteCell, string)
	}{
		{`SELECT row, col, raw FROM _lattis_cells WHERE ` + in, func(pc *pasteCell, s string) { pc.raw = s }},
		{`SELECT row, col, style FROM _lattis_cell_styles WHERE ` + in, func(pc *pasteCell, s string) { pc.style = parseStyle(s) }},
	} {
		rs, err := t.query(q.sql, args...)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var r, c int
			var s string
			if err := rs.Scan(&r, &c, &s); err != nil {
				rs.Close()
				return nil, err
			}
			if !inTable(tables, r, c) {
				q.set(&g[r-src.R0][c-src.C0], s)
			}
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return nil, err
		}
	}
	for i := range g {
		for j := range g[i] {
			r, c := src.R0+i, src.C0+j
			if !inTable(tables, r, c) {
				g[i][j].style = over(lines.at(r, c), g[i][j].style)
			}
		}
	}
	for _, tb := range tables {
		if !src.meets(tb) {
			continue
		}
		c0, c1 := max(src.C0, tb.Col), min(src.C1, tb.Right())
		first, last := max(src.R0, tb.Row+1), min(src.R1, tb.Bottom())
		var ids []int64
		if first <= last {
			if ids, err = t.ridsAt(&tb, first, last); err != nil {
				return nil, err
			}
		}
		for c := c0; c <= c1; c++ {
			col := tb.Cols[c-tb.Col]
			if src.has(tb.Row, c) {
				hs, err := t.tableStyle(tb.ID, -1, col.ID)
				if err != nil {
					return nil, err
				}
				g[tb.Row-src.R0][c-src.C0].style = over(lines.at(tb.Row, c), hs)
			}
			if len(ids) == 0 {
				continue
			}
			cs, err := t.tableStyle(tb.ID, 0, col.ID)
			if err != nil {
				return nil, err
			}
			for k, id := range ids {
				cell, err := t.tableStyle(tb.ID, id, col.ID)
				if err != nil {
					return nil, err
				}
				g[first+k-src.R0][c-src.C0].style = over(lines.at(first+k, c), cs, cell)
			}
		}
	}
	return g, nil
}

// pasteGrid colle les cellules g sur la feuille, à partir du coin de dst,
// selon opt (voir PasteOptions).
func (t *tx) pasteGrid(sheet int64, dst Rect, g [][]pasteCell, opt PasteOptions, ev *Event) error {
	if opt.Transpose {
		g = transposeCells(g)
	}
	h, w := len(g), gridWidth(g)
	if h == 0 || w == 0 {
		return nil
	}
	nr, nc := tiles(h, w, dst)
	area := Rect{dst.R0, dst.C0, dst.R0 + h*nr - 1, dst.C0 + w*nc - 1}
	if n := area.height() * area.width(); n > MaxPasteCells {
		return ErrPasteTooBig{n}
	}
	at := func(r, c int) pasteCell { return cellOf(g, (r-area.R0)%h, (c-area.C0)%w) }
	what := opt.what()
	if opt.content() {
		if err := t.guardCells(sheet, area, true); err != nil {
			return err
		}
	} else if err := t.guard(sheet); err != nil {
		return err
	}
	// Texte de chaque cellule (saisie d'une cellule libre, texte forcé
	// « '007 » compris ; sa formule seulement avec Tout ou Formules).
	textOf := func(pc pasteCell) string {
		if pc.raw != "" && (!isFormula(pc.raw) || what == "all" || what == "formulas") {
			return pc.raw
		}
		return pc.value
	}
	if opt.content() && opt.Op == "" && !opt.SkipBlanks && area.height() >= MinRows {
		// Zone vide, comme Paste : le bloc fait directement ses tableaux.
		free, err := t.areaClear(sheet, area.R0-1, area.C0-1, area.R1+1, area.C1+1)
		if err == nil && free {
			near, err2 := t.nearRange(sheet, area.R0, area.C0, area.R1, area.C1)
			free, err = !near, err2
		}
		if err != nil {
			return err
		}
		if free {
			block := make([][]string, area.height())
			for i := range block {
				block[i] = make([]string, area.width())
				for j := range block[i] {
					block[i][j] = textOf(at(area.R0+i, area.C0+j))
				}
			}
			t.undoBirth = true
			if err := t.pasteIslands(sheet, area.R0, area.C0, block, area.width(), ev); err != nil {
				return err
			}
			return t.pasteFormats(sheet, area, what, at)
		}
	}
	if opt.content() {
		t.collect = true
		t.pasting = true
		defer func() { t.pasting = false }()
		work.Loop(t.Ctx, "cells", int64(area.height()))
		for r := area.R0; r <= area.R1; r++ {
			for c := area.C0; c <= area.C1; c++ {
				pc := at(r, c)
				if opt.SkipBlanks && pc.blank() {
					continue
				}
				tables, err := t.tables(sheet)
				if err != nil {
					return err
				}
				tb := tableOver(tables, r, c)
				data := tb != nil && r > tb.Row
				if data && tb.Cols[c-tb.Col].Formula != "" {
					continue // une colonne calculée garde sa formule
				}
				text := textOf(pc)
				// Une formule de cellule libre entre dans un tableau par sa
				// valeur, comme au collage ordinaire (Transfer).
				if data && isFormula(text) && pc.value != "" && !isFormula(pc.value) {
					text = pc.value
				}
				if opt.Op != "" {
					cur, _, err := t.rawAt(sheet, r, c)
					if err != nil {
						return err
					}
					var ok bool
					if text, ok = operate(cur, pc.value, opt.Op, !data); !ok {
						continue
					}
				}
				if err := t.input(sheet, r, c, text, ev); err != nil {
					return err
				}
			}
			work.Progress(t.Ctx, int64(r-area.R0+1), int64(area.height()))
		}
	}
	return t.pasteFormats(sheet, area, what, at)
}

// pasteFormats pose la mise en forme de la source selon what (Tout,
// Formats ; Valeurs et formats de nombre : le format de nombre seulement).
func (t *tx) pasteFormats(sheet int64, area Rect, what string, at func(r, c int) pasteCell) error {
	style := func(r, c int) Style { return at(r, c).style }
	switch what {
	case "all":
		if err := t.pasteStyles(sheet, area, style, false); err != nil {
			return err
		}
		return t.pasteLinks(sheet, area, style) // les liens, avec Tout seulement
	case "formats":
		return t.pasteStyles(sheet, area, style, false)
	case "valuesNumbers":
		return t.pasteStyles(sheet, area, style, true)
	}
	return nil
}

// tableOver : le tableau qui contient la cellule (en-tête compris), ou nil.
func tableOver(tables []Table, r, c int) *Table {
	for i := range tables {
		tb := &tables[i]
		if r >= tb.Row && r <= tb.Bottom() && c >= tb.Col && c <= tb.Right() {
			return tb
		}
	}
	return nil
}

// transposeCells échange lignes et colonnes (une fusion aussi).
func transposeCells(g [][]pasteCell) [][]pasteCell {
	h, w := len(g), gridWidth(g)
	out := make([][]pasteCell, w)
	for j := range out {
		out[j] = make([]pasteCell, h)
		for i := range h {
			pc := cellOf(g, i, j)
			if pc.style["mr"] != nil || pc.style["mc"] != nil {
				s := over(pc.style)
				s["mr"], s["mc"] = pc.style["mc"], pc.style["mr"]
				pc.style = s
			}
			out[j][i] = pc
		}
	}
	return out
}

// operate combine la valeur de la source (src) à celle de la destination
// (cur, saisie gardée), comme l'opération du collage spécial d'un tableur :
// deux nombres donnent le résultat ; une cellule vide compte pour 0 ; une
// formule de cellule libre (free) devient =(formule)+source. Une source qui
// n'est pas un nombre est collée telle quelle ; une destination de texte ne
// change pas (faux : rien à écrire).
func operate(cur, src, op string, free bool) (string, bool) {
	sym := map[string]string{"add": "+", "sub": "-", "mul": "*", "div": "/"}[op]
	if sym == "" {
		return src, true
	}
	s, sok := pasteNumber(src)
	if strings.TrimSpace(src) == "" {
		s, sok = 0, true
	}
	if !sok {
		return src, true
	}
	if free && isFormula(cur) {
		body := strings.TrimPrefix(strings.TrimSpace(cur), "=")
		return "=(" + body + ")" + sym + pasteNumberText(s), true
	}
	d, dok := pasteNumber(cur)
	if strings.TrimSpace(cur) == "" {
		d, dok = 0, true
	}
	if !dok {
		return "", false
	}
	var v float64
	switch op {
	case "add":
		v = d + s
	case "sub":
		v = d - s
	case "mul":
		v = d * s
	case "div":
		if s == 0 {
			return "#DIV/0!", true
		}
		v = d / s
	}
	return pasteNumberText(v), true
}

// pasteNumber lit un nombre gardé (« 3,5 », « 1 234 », « 12 € »).
func pasteNumber(s string) (float64, bool) {
	v, err := grid.Parse(grid.Decimal, s)
	if err != nil || v == nil {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

// pasteNumberText écrit un nombre dans la forme gardée (décimale « , »),
// sans les restes de l'arithmétique binaire (0,1 + 0,2 donne 0,3).
func pasteNumberText(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "#NOMBRE!"
	}
	return formula.FormatNumber(f)
}

// pasteStyles pose sur la zone la mise en forme de la source (at), comme
// ApplyStyle : cellule libre, en-tête et cellules d'un tableau, et toute la
// colonne quand le collage en couvre toutes les lignes avec une même mise en
// forme (lignes à venir comprises). numbers : le format de nombre seulement.
// Les tableaux croisés gardent la leur. Les fusions de la source sont
// recréées sur les cellules libres (hors tableaux).
func (t *tx) pasteStyles(sheet int64, area Rect, at func(r, c int) Style, numbers bool) error {
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	lines, err := t.sheetLines(sheet)
	if err != nil {
		return err
	}
	places, err := t.stylePivotPlaces(sheet)
	if err != nil {
		return err
	}
	patch := func(r, c int) Style { return formatPatch(at(r, c), numbers) }
	for c := area.C0; c <= area.C1; c++ {
		var tb *Table
		for i := range tables {
			if c >= tables[i].Col && c <= tables[i].Right() && area.R1 >= tables[i].Row && area.R0 <= tables[i].Bottom() {
				tb = &tables[i]
			}
		}
		for r := area.R0; r <= area.R1; r++ {
			if tb != nil && r >= tb.Row && r <= tb.Bottom() || pivotPlaceAt(places, r, c) >= 0 {
				continue
			}
			if err := t.styleFree(sheet, r, c, patch(r, c), lines.at(r, c)); err != nil {
				return err
			}
		}
		if tb == nil {
			continue
		}
		colID := tb.Cols[c-tb.Col].ID
		if area.has(tb.Row, c) {
			if err := t.styleTable(tb.ID, -1, colID, patch(tb.Row, c), lines.at(tb.Row, c)); err != nil {
				return err
			}
		}
		first, last := max(area.R0, tb.Row+1), min(area.R1, tb.Bottom())
		if first > last {
			continue
		}
		if first == tb.Row+1 && last == tb.Bottom() && sameStyles(patch, first, last, c) {
			if err := t.styleTableColumn(tb, colID, c, patch(first, c), lines); err != nil {
				return err
			}
			continue
		}
		ids, err := t.ridsAt(tb, first, last)
		if err != nil {
			return err
		}
		colStyle, err := t.tableStyle(tb.ID, 0, colID)
		if err != nil {
			return err
		}
		for i, id := range ids {
			if err := t.styleTable(tb.ID, id, colID, patch(first+i, c), over(lines.at(first+i, c), colStyle)); err != nil {
				return err
			}
		}
	}
	if numbers {
		return nil
	}
	return t.pasteMerges(sheet, area, at, tables)
}

// sameStyles : toutes les lignes [first, last] de la colonne c reçoivent la
// même mise en forme.
func sameStyles(patch func(r, c int) Style, first, last, c int) bool {
	ref, _ := encodeStyle(patch(first, c))
	for r := first + 1; r <= last; r++ {
		if s, _ := encodeStyle(patch(r, c)); s != ref {
			return false
		}
	}
	return true
}

// pasteMerges recrée les fusions de la source dont la zone, dans la
// destination, ne touche ni tableau ni tableau croisé ; les fusions qu'elles
// recouvrent sont remplacées.
func (t *tx) pasteMerges(sheet int64, area Rect, at func(r, c int) Style, tables []Table) error {
	var want []Merge
	for r := area.R0; r <= area.R1; r++ {
		for c := area.C0; c <= area.C1; c++ {
			s := at(r, c)
			g := Merge{Row: r, Col: c, Rows: intOf(s["mr"]), Cols: intOf(s["mc"])}
			if g.Rows < 1 || g.Cols < 1 || g.Rows*g.Cols < 2 {
				continue
			}
			z := Rect{g.Row, g.Col, g.bottom(), g.right()}
			if slices.ContainsFunc(tables, z.meets) {
				continue
			}
			want = append(want, g)
		}
	}
	if len(want) == 0 {
		return nil
	}
	pivots, err := t.pivots(sheet)
	if err != nil {
		return err
	}
	old, err := t.merges(sheet)
	if err != nil {
		return err
	}
	for _, g := range want {
		hit := false
		for _, p := range pivots {
			if r := pivotRect(p); g.overlaps(r.R0, r.C0, r.R1, r.C1) {
				hit = true
			}
		}
		if hit {
			continue
		}
		for _, o := range old {
			if o.overlaps(g.Row, g.Col, g.bottom(), g.right()) && (o.Row != g.Row || o.Col != g.Col) {
				if err := t.styleFree(sheet, o.Row, o.Col, Style{"mr": nil, "mc": nil}, nil); err != nil {
					return err
				}
			}
		}
		if err := t.styleFree(sheet, g.Row, g.Col, Style{"mr": g.Rows, "mc": g.Cols}, nil); err != nil {
			return err
		}
	}
	return nil
}

// pasteWidths donne aux colonnes de la destination la largeur des colonnes
// de la source (répétées sur une sélection plus large, comme le bloc).
func (m *Model) pasteWidths(ctx context.Context, from int64, src Rect, to int64, dst Rect) error {
	return m.Do(ctx, "Collage spécial", func(t *tx) error {
		if err := t.guard(to); err != nil {
			return err
		}
		widths := map[int]int{}
		rs, err := t.query(`SELECT col, width FROM _lattis_col_widths WHERE sheet = ? AND col BETWEEN ? AND ?`, from, src.C0, src.C1)
		if err != nil {
			return err
		}
		for rs.Next() {
			var c, px int
			if err := rs.Scan(&c, &px); err != nil {
				rs.Close()
				return err
			}
			widths[c] = px
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
		w := src.width()
		_, nc := tiles(src.height(), w, Rect{dst.R0, dst.C0, dst.R0 + src.height() - 1, dst.C1})
		for j := range w * nc {
			if err := t.setLineSize(`_lattis_col_widths`, `col`, `width`, to, dst.C0+j, widths[src.C0+j%w]); err != nil {
				return err
			}
		}
		return nil
	})
}
