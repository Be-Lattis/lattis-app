package transfer

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Lecture du classeur pour l'export (.xlsx, .ods) et l'impression : chaque
// feuille donne ses cellules (cellules libres, tableaux croisés, résultats
// déversés, cellules mises en forme), ses tableaux, lus ligne à ligne
// pendant l'écriture, et sa mise en page (largeurs, hauteurs, lignes et
// colonnes masquées, fusions). Les tableaux l'emportent sur ce qu'ils
// recouvrent.

// xcell est une cellule à écrire.
type xcell struct {
	kind byte // 'n' nombre, 'd' date (numéro de série), 's' texte, 'b' booléen, 'e' erreur, 0 vide
	num  float64
	str  string
	bold bool   // en-tête (impression)
	f    string // formule traduite (sans =), "" : valeur seule
	fm   int32  // mise en forme (registre fmts), 0 : aucune
}

// blank : rien à écrire (ni valeur, ni formule, ni mise en forme).
func (x xcell) blank() bool { return x.kind == 0 && x.f == "" && x.fm == 0 }

// xtable est un tableau à écrire : ses lignes sont lues pendant l'écriture.
type xtable struct {
	info  canvas.TableInfo
	place *tablePlace // nil à l'impression
	// Mise en forme par colonne : en-tête, cellules sans style propre, et
	// couches (colonne de la feuille, colonne du tableau) pour les autres.
	head, base []int32
	layers     [][2]canvas.Style
	own        map[int]map[int]canvas.Style // rang de ligne → colonne → style propre
	formulas   []string                     // colonne calculée traduite ("" : valeurs)
	rows       map[int]canvas.Style         // styles des lignes de la feuille
}

type xsheet struct {
	name   string
	info   canvas.SheetInfo
	hidden bool
	cells  map[[2]int]xcell
	widths []canvas.ColWidth
	// clipped : cellules au-delà des limites d'Excel, non exportées.
	clipped int
	// tables : tableaux lus pendant l'écriture (export), ligne à ligne, sans
	// garder toute la feuille en mémoire ; vide quand cells les contient déjà.
	tables []*xtable
	rows   func(ctx context.Context, xt *xtable, send func(r int, cells []xcell) error) error
	// Mise en page : hauteurs (pixels), styles et masquage des lignes et
	// colonnes entières, fusions (ligne, colonne, lignes, colonnes).
	heights            map[int]int
	rowFmt, colFmt     map[int]int32
	hiddenR, hiddenC   map[int]bool
	merges             [][4]int
	rowStyle, colStyle map[int]canvas.Style
}

// Limites d'une feuille Excel : la feuille de Lattis va au-delà, l'export coupe.
const (
	ExcelRows = 1_048_576
	ExcelCols = 16_384
)

// exportBook lit tout le classeur pour un export.
func exportBook(ctx context.Context, m *canvas.Model, tg target) ([]xsheet, *fmts, error) {
	first, err := m.Layout(ctx, 0)
	if err != nil {
		return nil, nil, err
	}
	raw := make([]string, len(first.Sheets))
	for i, s := range first.Sheets {
		raw[i] = s.Name
	}
	names := excelSheetNames(raw)
	b := &book{target: tg, sheets: map[string]string{}, tables: map[string]*tablePlace{}, names: map[string]cellPlace{}}
	layouts := make([]canvas.Layout, len(first.Sheets))
	used := map[string]bool{}
	for i, s := range first.Sheets {
		if layouts[i], err = m.Layout(ctx, s.ID); err != nil {
			return nil, nil, err
		}
		b.sheets[strings.ToLower(s.Name)] = names[i]
		for _, tb := range layouts[i].Tables {
			p := &tablePlace{name: uniqueName(excelName(tb.Name), used), sheet: names[i], row: tb.Row, col: tb.Col, key: tb.Key, total: tb.Total}
			for _, c := range tb.Columns {
				p.cols = append(p.cols, c.Name)
			}
			b.tables[strings.ToLower(tb.Name)] = p
		}
		for _, n := range layouts[i].Names {
			b.names[strings.ToLower(n.Name)] = cellPlace{sheet: names[i], row: n.Row, col: n.Col}
		}
	}
	fm := newFmts()
	sheets := make([]xsheet, len(first.Sheets))
	for i, s := range first.Sheets {
		sh, err := collectLayout(ctx, m, s, layouts[i], b, fm)
		if err != nil {
			return nil, nil, err
		}
		sh.name = names[i]
		sh.rows = func(ctx context.Context, xt *xtable, send func(int, []xcell) error) error {
			return tableRows(ctx, m, xt, fm, tg, send)
		}
		sheets[i] = sh
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
	}
	return sheets, fm, nil
}

// tableRows lit un tableau ligne à ligne (en-tête compris) en cellules.
func tableRows(ctx context.Context, m *canvas.Model, xt *xtable, fm *fmts, tg target, send func(r int, cells []xcell) error) error {
	tb := xt.info
	var cols []canvas.ExportColumn
	r, k := tb.Row, -1
	return m.ExportTable(ctx, tb.Name, func(cs []canvas.ExportColumn) error {
		cols = cs
		head := make([]xcell, len(cs))
		for j, c := range cs {
			head[j] = xcell{kind: 's', str: c.Name, bold: true}
			if j < len(xt.head) {
				head[j].fm = xt.head[j]
			}
		}
		return send(r, head)
	}, func(row []any) error {
		r++
		k++
		cells := make([]xcell, len(row))
		for j, v := range row {
			x := tableCell(v, cols[j].Type)
			if j < len(xt.formulas) && xt.formulas[j] != "" {
				x.f = xt.formulas[j]
				if tg == toODS {
					x.f = strings.ReplaceAll(x.f, rowMark, strconv.Itoa(r+1))
				}
			}
			if j < len(xt.base) {
				x.fm = xt.base[j]
				if own, rs := xt.own[k][j], xt.rowStyle(r); own != nil || rs != nil {
					x.fm = fm.of(xt.layers[j][0], rs, xt.layers[j][1], own)
				}
			}
			cells[j] = x
		}
		return send(r, cells)
	})
}

// rowStyle est posé par collectLayout : style de la ligne r de la feuille.
func (xt *xtable) rowStyle(r int) canvas.Style { return xt.rows[r] }

// collectSheet lit tout le contenu d'une feuille, cellule par cellule
// (impression ; l'export lit ses tableaux pendant l'écriture).
func collectSheet(ctx context.Context, m *canvas.Model, s canvas.SheetInfo) (xsheet, error) {
	l, err := m.Layout(ctx, s.ID)
	if err != nil {
		return xsheet{}, err
	}
	sh, err := collectLayout(ctx, m, s, l, nil, nil)
	if err != nil {
		return sh, err
	}
	for _, xt := range sh.tables {
		err := tableRows(ctx, m, xt, nil, 0, func(r int, cells []xcell) error {
			for j, x := range cells {
				sh.put(r, xt.info.Col+j, x)
			}
			return nil
		})
		if err != nil {
			return xsheet{}, err
		}
	}
	sh.tables = nil
	return sh, nil
}

// put pose une cellule, ou la compte hors des limites d'Excel.
func (sh *xsheet) put(r, c int, x xcell) {
	if r >= ExcelRows || c >= ExcelCols {
		if x.kind != 0 {
			sh.clipped++
		}
		return
	}
	if !x.blank() {
		sh.cells[[2]int{r, c}] = x
	}
}

// collectLayout lit une feuille sauf les lignes de ses tableaux : tableaux
// croisés, cellules libres, déversements, mise en forme et mise en page
// (sh.tables : les tableaux). b et fm sont nil à l'impression.
func collectLayout(ctx context.Context, m *canvas.Model, s canvas.SheetInfo, l canvas.Layout, b *book, fm *fmts) (xsheet, error) {
	sh := xsheet{name: s.Name, info: s, hidden: s.Hidden, cells: map[[2]int]xcell{}, widths: l.Widths,
		heights: map[int]int{}, rowFmt: map[int]int32{}, colFmt: map[int]int32{}, hiddenR: map[int]bool{}, hiddenC: map[int]bool{},
		rowStyle: map[int]canvas.Style{}, colStyle: map[int]canvas.Style{}}
	for _, h := range l.Heights {
		sh.heights[h.Row] = h.Height
	}
	for _, ls := range l.RowStyles {
		st := parseStyle(ls.Style)
		sh.rowStyle[ls.Index] = st
		if st["hd"] == true {
			sh.hiddenR[ls.Index] = true
		}
		if n := fm.of(st); n != 0 {
			sh.rowFmt[ls.Index] = n
		}
	}
	for _, ls := range l.ColStyles {
		st := parseStyle(ls.Style)
		sh.colStyle[ls.Index] = st
		if st["hd"] == true {
			sh.hiddenC[ls.Index] = true
		}
		if n := fm.of(st); n != 0 {
			sh.colFmt[ls.Index] = n
		}
	}
	own := map[[2]int]canvas.Style{}
	for _, c := range l.Styles {
		st := parseStyle(c.Style)
		own[[2]int{c.Row, c.Col}] = st
		mr, _ := st["mr"].(float64)
		mc, _ := st["mc"].(float64)
		if mr >= 1 && mc >= 1 && mr*mc > 1 {
			sh.merges = append(sh.merges, [4]int{c.Row, c.Col, int(mr), int(mc)})
		}
	}
	slices.SortFunc(sh.merges, func(a, b [4]int) int {
		if a[0] != b[0] {
			return a[0] - b[0]
		}
		return a[1] - b[1]
	})
	// fmtAt : mise en forme d'une cellule libre (colonne, ligne, cellule).
	fmtAt := func(r, c int, extra canvas.Style) int32 {
		return fm.of(sh.colStyle[c], sh.rowStyle[r], extra, own[[2]int{r, c}])
	}
	put := sh.put
	// Cellules mises en forme sans contenu, tableaux croisés, cellules
	// libres puis tableaux : un tableau l'emporte sur ce qu'il recouvre.
	if fm != nil {
		for k := range own {
			if n := fmtAt(k[0], k[1], nil); n != 0 {
				put(k[0], k[1], xcell{fm: n})
			}
		}
	}
	bold := canvas.Style{"b": true}
	for _, p := range l.Pivots {
		for i, row := range p.Cells {
			for j, v := range row {
				x := textCell(v)
				x.bold = i == 0
				if fm != nil {
					var extra canvas.Style
					if x.bold {
						extra = bold
					}
					x.fm = fmtAt(p.Row+i, p.Col+j, extra)
				}
				put(p.Row+i, p.Col+j, x)
			}
		}
	}
	for _, c := range l.Cells {
		if len(c.Spill) > 0 {
			for i, row := range c.Spill {
				for j, v := range row {
					x := textCell(v)
					x.fm = fmtAt(c.Row+i, c.Col+j, nil)
					put(c.Row+i, c.Col+j, x)
				}
			}
			continue
		}
		var x xcell
		switch {
		case c.Error:
			x = errorCell(c.Value)
		case strings.HasPrefix(c.Raw, "="):
			x = textCell(c.Value)
		default:
			x = textCell(c.Raw)
		}
		if b != nil && strings.HasPrefix(c.Raw, "=") {
			x.f, _ = b.translate(c.Raw, at{sheet: b.sheets[strings.ToLower(s.Name)]})
		}
		x.fm = fmtAt(c.Row, c.Col, nil)
		put(c.Row, c.Col, x)
	}
	for _, tb := range l.Tables {
		xt := &xtable{info: tb, rows: sh.rowStyle}
		if b != nil {
			xt.place = b.tables[strings.ToLower(tb.Name)]
		}
		if fm != nil {
			var cellStyles []canvas.CellStyle
			var err error
			if cellStyles, err = m.TableCellStyles(ctx, tb.ID); err != nil {
				return sh, err
			}
			xt.own = map[int]map[int]canvas.Style{}
			for _, cs := range cellStyles {
				if xt.own[cs.Row] == nil {
					xt.own[cs.Row] = map[int]canvas.Style{}
				}
				xt.own[cs.Row][cs.Col] = parseStyle(cs.Style)
			}
			for j, c := range tb.Columns {
				sc, tc := sh.colStyle[tb.Col+j], parseStyle(c.Style)
				xt.layers = append(xt.layers, [2]canvas.Style{sc, tc})
				xt.base = append(xt.base, fm.of(sc, tc))
				xt.head = append(xt.head, fm.of(sc, sh.rowStyle[tb.Row], bold, parseStyle(c.HeaderStyle)))
			}
		}
		if xt.place != nil {
			for _, c := range tb.Columns {
				f := ""
				if c.Formula != "" {
					f, _ = b.translate(c.Formula, at{sheet: xt.place.sheet, table: xt.place})
				}
				xt.formulas = append(xt.formulas, f)
			}
		}
		sh.tables = append(sh.tables, xt)
	}
	return sh, nil
}

// marks donne les lignes à écrire même sans cellule : hauteur, style ou
// masquage, et lignes couvertes par une fusion.
func (sh *xsheet) marks() []int {
	set := map[int]bool{}
	for r := range sh.heights {
		set[r] = true
	}
	for r := range sh.rowFmt {
		set[r] = true
	}
	for r := range sh.hiddenR {
		set[r] = true
	}
	for _, g := range sh.merges {
		for r := g[0]; r < g[0]+g[2]; r++ {
			set[r] = true
		}
	}
	out := make([]int, 0, len(set))
	for r := range set {
		if r < ExcelRows {
			out = append(out, r)
		}
	}
	slices.Sort(out)
	return out
}

// textCell type une valeur affichée (cellule libre, TCD, déversement) comme
// le ferait la saisie : nombre, booléen, date ou texte.
func textCell(s string) xcell {
	if s == "" {
		return xcell{}
	}
	if strings.HasPrefix(s, "#") {
		if _, ok := excelErrors[s]; ok {
			return errorCell(s)
		}
	}
	v := parseText(s)
	switch v.t {
	case grid.Integer:
		return xcell{kind: 'n', num: float64(v.i)}
	case grid.Decimal:
		return xcell{kind: 'n', num: v.f}
	case grid.Boolean:
		return xcell{kind: 'b', num: float64(v.i)}
	case grid.Date:
		if f, ok := formula.ParseDate(v.s); ok {
			return xcell{kind: 'd', num: f}
		}
	}
	if f, ok := formula.ParseDate(s); ok && strings.Contains(s, ":") {
		return xcell{kind: 'd', num: f}
	}
	return xcell{kind: 's', str: s}
}

// excelErrors : codes d'erreur de Lattis (français) et leur code Excel.
var excelErrors = map[string]string{
	"#DIV/0!": "#DIV/0!", "#N/A": "#N/A", "#VALEUR!": "#VALUE!", "#REF!": "#REF!",
	"#NOM?": "#NAME?", "#NOMBRE!": "#NUM!", "#NUL!": "#NULL!",
	"#CIRC!": "#REF!", // Excel n'a pas de code pour une référence circulaire
}

func errorCell(s string) xcell {
	if e, ok := excelErrors[s]; ok {
		return xcell{kind: 'e', str: e}
	}
	return xcell{kind: 's', str: s}
}

// tableCell convertit une valeur lue par canvas.Model.ExportTable.
func tableCell(v any, t grid.Type) xcell {
	switch v := v.(type) {
	case nil:
		return xcell{}
	case int64:
		if t == grid.Boolean {
			return xcell{kind: 'b', num: float64(v)}
		}
		return xcell{kind: 'n', num: float64(v)}
	case float64:
		return xcell{kind: 'n', num: v}
	case string:
		if t == grid.Date {
			if f, ok := formula.ParseDate(v); ok {
				return xcell{kind: 'd', num: f}
			}
		}
		return xcell{kind: 's', str: v}
	case canvas.Invalid:
		return xcell{kind: 's', str: string(v)}
	case formula.Value:
		switch v.Kind {
		case formula.Number:
			return xcell{kind: 'n', num: v.Num}
		case formula.Date:
			return xcell{kind: 'd', num: v.Num}
		case formula.Bool:
			if v.Bool {
				return xcell{kind: 'b', num: 1}
			}
			return xcell{kind: 'b'}
		case formula.Error:
			return errorCell(v.String())
		case formula.Text:
			return xcell{kind: 's', str: v.Str}
		}
		return xcell{}
	}
	return xcell{kind: 's', str: fmt.Sprint(v)}
}

// excelSheetNames rend les noms valides pour Excel : 31 caractères au plus,
// sans []:*?/\, uniques sans tenir compte de la casse.
func excelSheetNames(names []string) []string {
	out := make([]string, len(names))
	seen := map[string]bool{}
	for i, n := range names {
		n = strings.Map(func(r rune) rune {
			if strings.ContainsRune(`[]:*?/\`, r) {
				return '_'
			}
			return r
		}, n)
		n = strings.Trim(n, "'")
		if n == "" {
			n = fmt.Sprintf("Feuille%d", i+1)
		}
		base := truncateRunes(n, 31)
		cand := base
		for k := 2; seen[strings.ToLower(cand)]; k++ {
			suffix := fmt.Sprintf(" (%d)", k)
			cand = truncateRunes(base, 31-len([]rune(suffix))) + suffix
		}
		seen[strings.ToLower(cand)] = true
		out[i] = cand
	}
	return out
}

// cellLike : noms qu'Excel lirait comme une adresse (A1, R1C1, R, C).
var cellLike = regexp.MustCompile(`(?i)^([a-z]{1,3}[0-9]+|r[0-9]*c?[0-9]*|c[0-9]*)$`)

// excelName rend un nom de tableau valide pour Excel et LibreOffice :
// lettres, chiffres, _ et ., commençant par une lettre ou _, et qui ne se
// lit pas comme une adresse.
func excelName(n string) string {
	s := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, n)
	if s == "" {
		s = "Tableau"
	}
	if r := []rune(s)[0]; !unicode.IsLetter(r) && r != '_' || cellLike.MatchString(s) {
		s = "_" + s
	}
	return truncateRunes(s, 250)
}

// uniqueName rend un nom pas encore pris (sans tenir compte de la casse).
func uniqueName(n string, used map[string]bool) string {
	cand := n
	for k := 2; used[strings.ToLower(cand)]; k++ {
		cand = n + "_" + strconv.Itoa(k)
	}
	used[strings.ToLower(cand)] = true
	return cand
}

func truncateRunes(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// colName donne la lettre de colonne Excel (0 : A).
func colName(c int) string {
	s := ""
	for c++; c > 0; c = (c - 1) / 26 {
		s = string(rune('A'+(c-1)%26)) + s
	}
	return s
}

func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '&':
			b.WriteString("&amp;")
		case r == '"':
			b.WriteString("&quot;")
		case r == '\t' || r == '\n' || r == '\r' || r >= 0x20 && r != 0xFFFE && r != 0xFFFF:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// tableLine est une ligne d'un tableau lue pendant l'écriture.
type tableLine struct {
	r     int
	cells []xcell
}

// mergeRows rend les lignes de la feuille dans l'ordre : les cellules posées
// (keys, triées) et les lignes des tableaux, lues en parallèle au fil de
// l'écriture ; un tableau l'emporte sur ce qu'il recouvre. Les lignes de
// marks sont rendues même vides.
func mergeRows(ctx context.Context, s *xsheet, keys [][2]int, marks []int, emit func(r int, cols []int, cells []xcell) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type source struct {
		xt   *xtable
		ch   chan tableLine
		head *tableLine
		err  error
	}
	srcs := make([]*source, len(s.tables))
	for i, xt := range s.tables {
		src := &source{xt: xt, ch: make(chan tableLine, 512)}
		srcs[i] = src
		go func() {
			defer close(src.ch)
			src.err = s.rows(ctx, xt, func(r int, cells []xcell) error {
				select {
				case src.ch <- tableLine{r, cells}:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}()
	}
	next := func(src *source) {
		if l, ok := <-src.ch; ok {
			src.head = &l
		} else {
			src.head = nil
		}
	}
	for _, src := range srcs {
		next(src)
	}
	k, mk := 0, 0
	line := map[int]xcell{}
	var emitErr error
	for emitErr == nil {
		r := -1
		if k < len(keys) {
			r = keys[k][0]
		}
		if mk < len(marks) && (r < 0 || marks[mk] < r) {
			r = marks[mk]
		}
		for _, src := range srcs {
			if src.head != nil && (r < 0 || src.head.r < r) {
				r = src.head.r
			}
		}
		if r < 0 {
			break
		}
		marked := false
		for ; mk < len(marks) && marks[mk] == r; mk++ {
			marked = true
		}
		clear(line)
		for ; k < len(keys) && keys[k][0] == r; k++ {
			line[keys[k][1]] = s.cells[keys[k]]
		}
		for _, src := range srcs {
			for src.head != nil && src.head.r == r {
				for j, x := range src.head.cells {
					c := src.xt.info.Col + j
					if r >= ExcelRows || c >= ExcelCols {
						if x.kind != 0 {
							s.clipped++
						}
						continue
					}
					if !x.blank() {
						line[c] = x
					} else {
						delete(line, c)
					}
				}
				next(src)
			}
		}
		if r >= ExcelRows || len(line) == 0 && !marked {
			continue
		}
		cols := make([]int, 0, len(line))
		for c := range line {
			cols = append(cols, c)
		}
		slices.Sort(cols)
		cells := make([]xcell, len(cols))
		for i, c := range cols {
			cells[i] = line[c]
		}
		emitErr = emit(r, cols, cells)
	}
	if emitErr != nil {
		cancel()
		for _, src := range srcs {
			for range src.ch {
			}
		}
		return emitErr
	}
	for _, src := range srcs {
		if src.err != nil {
			return src.err
		}
	}
	return ctx.Err()
}

// sortedKeys donne les cellules posées dans l'ordre (ligne, colonne).
func (sh *xsheet) sortedKeys() [][2]int {
	keys := make([][2]int, 0, len(sh.cells))
	for k := range sh.cells {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b [2]int) int {
		if a[0] != b[0] {
			return a[0] - b[0]
		}
		return a[1] - b[1]
	})
	return keys
}
