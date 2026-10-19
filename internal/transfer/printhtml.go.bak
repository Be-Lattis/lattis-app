package transfer

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
)

// MaxPrintRows borne les lignes d'une impression : au-delà, la page dit ce
// qui n'est pas imprimé (filtrez ou exportez en .xlsx).
const MaxPrintRows = 5000

// maxPrintCols borne les colonnes d'une zone d'impression.
const maxPrintCols = 2000

// Tailles par défaut (pixels CSS, 96 par pouce), comme la grille.
const (
	defaultRowPx  = 22
	headingsColPx = 36 // en-têtes de lignes imprimés
	headingsRowPx = 18 // en-têtes de colonnes imprimés
	pxPerMM       = 96 / 25.4
)

// PrintInfo donne ce que citent l'en-tête et le pied de page (&[Fichier],
// &[Chemin], &[Date], &[Heure]).
type PrintInfo struct {
	Path string
	Now  time.Time
}

// PrintHTML rend une feuille en pages HTML, pour l'impression et l'export
// PDF (fenêtre d'impression du système), selon sa mise en page (onglet Mise
// en page) : papier et orientation (@page), marges, échelle ou ajustement en
// pages, zone d'impression (sinon la plage utilisée), titres répétés, sauts
// de page, quadrillage et en-têtes, centrage, en-tête et pied de page.
//
// Lattis découpe lui-même les pages (hauteurs et largeurs de la grille) :
// chaque page est un bloc de la taille du papier, sans marge d'imprimante
// (@page margin: 0), avec l'en-tête et le pied de page dans ses marges.
// Ainsi les colonnes répétées, l'ordre des pages et la numérotation
// marchent dans WebView2, qui ne sait répéter que les lignes d'en-tête d'un
// tableau. Valeurs telles que la grille les affiche, en-têtes de tableaux en
// gras ; sans autre mise en forme ni les graphiques.
func PrintHTML(ctx context.Context, m *canvas.Model, sheet int64, info PrintInfo) (string, error) {
	sh, err := printSheet(ctx, m, sheet, true)
	if err != nil {
		return "", err
	}
	if info.Now.IsZero() {
		info.Now = time.Now()
	}
	// Les graphiques ne sont pas imprimés : la page le dit.
	return renderPrint(sh, info) + chartsNote(sh.rep.Charts), nil
}

// printSheet lit la feuille à imprimer ; rows : avec les lignes des tableaux.
func printSheet(ctx context.Context, m *canvas.Model, sheet int64, rows bool) (xsheet, error) {
	l, err := m.Layout(ctx, sheet)
	if err != nil {
		return xsheet{}, err
	}
	var info canvas.SheetInfo
	for _, s := range l.Sheets {
		if s.ID == sheet {
			info = s
		}
	}
	if info.ID == 0 {
		return xsheet{}, fmt.Errorf("feuille %d introuvable", sheet)
	}
	sh, err := collectLayout(ctx, m, info, l, nil, nil)
	if err != nil {
		return sh, err
	}
	for _, xt := range sh.tables {
		t := xt.info
		sh.tableRects = append(sh.tableRects, canvas.Rect{R0: t.Row, C0: t.Col, R1: t.Row + t.Total, C1: t.Col + len(t.Columns) - 1})
	}
	if !rows {
		return sh, nil
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

// pager découpe une feuille en pages.
type pager struct {
	sh   *xsheet
	p    canvas.PageSetup
	area canvas.Rect
	// more : lignes de la zone au-delà de MaxPrintRows (non imprimées).
	more int
	// Mise à l'échelle retenue (1 : 100 %), bandes de lignes et de colonnes.
	scale      float64
	rows, cols [][2]int
	widths     map[int]int
}

// usedRect est la plage utilisée : cellules et tableaux (lignes comprises).
func usedRect(sh *xsheet) (canvas.Rect, bool) {
	r := canvas.Rect{R0: -1}
	add := func(r0, c0, r1, c1 int) {
		if r.R0 < 0 {
			r = canvas.Rect{R0: r0, C0: c0, R1: r1, C1: c1}
			return
		}
		r.R0, r.C0, r.R1, r.C1 = min(r.R0, r0), min(r.C0, c0), max(r.R1, r1), max(r.C1, c1)
	}
	for k := range sh.cells {
		add(k[0], k[1], k[0], k[1])
	}
	for _, t := range sh.tableRects {
		add(t.R0, t.C0, t.R1, t.C1)
	}
	return r, r.R0 >= 0
}

func newPager(sh *xsheet) (*pager, bool) {
	pg := &pager{sh: sh, p: sh.page, scale: 1, widths: map[int]int{}}
	for _, w := range sh.widths {
		pg.widths[w.Col] = w.Width
	}
	if a := pg.p.Area; a != nil {
		pg.area = *a
	} else {
		r, ok := usedRect(sh)
		if !ok {
			return pg, false
		}
		pg.area = r
	}
	pg.area.C1 = min(pg.area.C1, pg.area.C0+maxPrintCols-1)
	pg.area.R1 = min(pg.area.R1, ExcelRows-1)
	// Au plus MaxPrintRows lignes visibles.
	n := 0
	for r := pg.area.R0; r <= pg.area.R1; r++ {
		if sh.hiddenR[r] {
			continue
		}
		if n++; n > MaxPrintRows {
			pg.more = pg.area.R1 - r + 1
			pg.area.R1 = r - 1
			break
		}
	}
	pg.paginate()
	return pg, true
}

func (pg *pager) height(r int) float64 {
	if pg.sh.hiddenR[r] {
		return 0
	}
	if h, ok := pg.sh.heights[r]; ok {
		return float64(h)
	}
	return defaultRowPx
}

func (pg *pager) width(c int) float64 {
	if pg.sh.hiddenC[c] {
		return 0
	}
	if w, ok := pg.widths[c]; ok {
		return float64(w)
	}
	return defaultColPx
}

// content donne la place du contenu d'une page (pixels CSS, échelle 1).
func (pg *pager) content() (w, h float64) {
	pw, ph := pg.p.PaperMM()
	m := pg.p.PageMargins()
	w = (pw - (m.Left+m.Right)*10) * pxPerMM
	h = (ph - (m.Top+m.Bottom)*10) * pxPerMM
	if pg.p.Headings {
		w -= headingsColPx
		h -= headingsRowPx
	}
	return max(w, 20), max(h, 20)
}

// titles donne les lignes (ou colonnes) de titre à répéter au-dessus d'une
// bande qui commence à start : celles qui précèdent la bande.
func titles(s *canvas.Span, start int) (int, int, bool) {
	if s == nil || s.From >= start {
		return 0, 0, false
	}
	return s.From, min(s.To, start-1), true
}

// bands découpe lo..hi en pages : une page se termine quand la suivante ne
// tient plus (titres compris) ou avant un saut manuel.
func bands(lo, hi int, size func(int) float64, avail float64, title *canvas.Span, manual []int) [][2]int {
	breaks := map[int]bool{}
	for _, b := range manual {
		breaks[b] = true
	}
	titleSize := func(start int) float64 {
		a, b, ok := titles(title, start)
		if !ok {
			return 0
		}
		s := 0.0
		for i := a; i <= b; i++ {
			s += size(i)
		}
		return s
	}
	var out [][2]int
	start, acc, room := lo, 0.0, avail-titleSize(lo)
	for i := lo; i <= hi; i++ {
		h := size(i)
		if i > start && (breaks[i] || (h > 0 && acc+h > room)) {
			out = append(out, [2]int{start, i - 1})
			start, acc, room = i, 0, avail-titleSize(i)
		}
		acc += h
	}
	return append(out, [2]int{start, hi})
}

// paginate fixe l'échelle et les bandes ; ajustée à un nombre de pages, la
// plus grande échelle (10 à 100 %) qui tient, sauts manuels ignorés (comme
// Excel).
func (pg *pager) paginate() {
	w, h := pg.content()
	a := pg.area
	cut := func(s float64, rows bool) [][2]int {
		if rows {
			var manual []int
			if !pg.p.Fit() {
				manual = pg.p.RowBreaks
			}
			return bands(a.R0, a.R1, pg.height, h/s, pg.p.TitleRows, manual)
		}
		var manual []int
		if !pg.p.Fit() {
			manual = pg.p.ColBreaks
		}
		return bands(a.C0, a.C1, pg.width, w/s, pg.p.TitleCols, manual)
	}
	if !pg.p.Fit() {
		if pg.p.Scale > 0 {
			pg.scale = float64(pg.p.Scale) / 100
		}
		pg.rows, pg.cols = cut(pg.scale, true), cut(pg.scale, false)
		return
	}
	// Plus grande échelle en pour cent dont les pages tiennent dans n.
	best := func(n int, rows bool) int {
		if n <= 0 || len(cut(1, rows)) <= n {
			return 100
		}
		lo, hi := 10, 100 // lo : tient (ou plancher), hi : ne tient pas
		for hi-lo > 1 {
			mid := (lo + hi) / 2
			if len(cut(float64(mid)/100, rows)) <= n {
				lo = mid
			} else {
				hi = mid
			}
		}
		return lo
	}
	pct := min(best(pg.p.FitWide, false), best(pg.p.FitTall, true))
	pg.scale = float64(pct) / 100
	pg.rows, pg.cols = cut(pg.scale, true), cut(pg.scale, false)
}

// pages donne les pages dans l'ordre d'impression (bande de lignes, bande
// de colonnes).
func (pg *pager) pages() [][2]int {
	var out [][2]int
	if pg.p.AcrossFirst {
		for i := range pg.rows {
			for j := range pg.cols {
				out = append(out, [2]int{i, j})
			}
		}
		return out
	}
	for j := range pg.cols {
		for i := range pg.rows {
			out = append(out, [2]int{i, j})
		}
	}
	return out
}

func mm(v float64) string { return strconv.FormatFloat(float64(int(v*100+0.5))/100, 'f', -1, 64) + "mm" }

func renderPrint(sh xsheet, info PrintInfo) string {
	var b strings.Builder
	p := sh.page
	pw, ph := p.PaperMM()
	m := p.PageMargins()
	// Bloc d'une page un peu plus court que le papier : un arrondi ne pousse
	// pas une page blanche.
	fmt.Fprintf(&b, `<style>@page { size: %s %s; margin: 0; } @media print { html, body { margin: 0 !important; padding: 0 !important; } }</style>`, mm(pw), mm(ph))
	pg, ok := newPager(&sh)
	var pages [][2]int
	if ok {
		pages = pg.pages()
	}
	total := max(len(pages), 1)
	first := max(p.FirstPage, 1)
	page := func(n int, body func()) {
		cls := "lp-page"
		if p.BlackWhite {
			cls += " lp-bw"
		}
		fmt.Fprintf(&b, `<section class="%s" style="width:%s;height:%s">`, cls, mm(pw), mm(ph-0.6))
		hf := func(h canvas.HeaderFooter, pos string, at float64) {
			if h.Empty() {
				return
			}
			fmt.Fprintf(&b, `<div class="lp-hf" style="%s:%s;left:%s;right:%s">`, pos, mm(at*10), mm(m.Left*10), mm(m.Right*10))
			for _, part := range [][2]string{{"l", h.Left}, {"c", h.Center}, {"r", h.Right}} {
				fmt.Fprintf(&b, `<span class="%s">%s</span>`, part[0], xmlEscape(expandHF(part[1], hfVars{page: first + n, pages: total, sheet: sh.name, info: info})))
			}
			b.WriteString(`</div>`)
		}
		hf(p.Header, "top", m.Header)
		style := fmt.Sprintf("top:%s;left:%s;width:%s;height:%s", mm(m.Top*10), mm(m.Left*10), mm(pw-(m.Left+m.Right)*10), mm(ph-0.6-(m.Top+m.Bottom)*10))
		if p.CenterH {
			style += ";align-items:center"
		}
		if p.CenterV {
			style += ";justify-content:center"
		}
		fmt.Fprintf(&b, `<div class="lp-body" style="%s">`, style)
		body()
		b.WriteString(`</div>`)
		hf(p.Footer, "bottom", m.Footer)
		b.WriteString(`</section>`)
	}
	if !ok {
		page(0, func() { b.WriteString(`<p class="print-empty">Feuille vide.</p>`) })
		return b.String()
	}
	for n, at := range pages {
		page(n, func() {
			pg.table(&b, pg.rows[at[0]], pg.cols[at[1]])
			if n == len(pages)-1 && pg.more > 0 {
				fmt.Fprintf(&b, `<p class="print-more">%d lignes suivantes non imprimées : filtrez la feuille ou exportez le classeur en .xlsx.</p>`, pg.more)
			}
		})
	}
	return b.String()
}

// table écrit le tableau HTML d'une page : titres répétés, puis la bande.
func (pg *pager) table(b *strings.Builder, rb, cb [2]int) {
	sh, p := pg.sh, pg.p
	var rows, cols []int
	if a, z, ok := titles(p.TitleRows, rb[0]); ok {
		for r := a; r <= z; r++ {
			rows = append(rows, r)
		}
	}
	if a, z, ok := titles(p.TitleCols, cb[0]); ok {
		for c := a; c <= z; c++ {
			cols = append(cols, c)
		}
	}
	for r := rb[0]; r <= rb[1]; r++ {
		rows = append(rows, r)
	}
	for c := cb[0]; c <= cb[1]; c++ {
		cols = append(cols, c)
	}
	rows = filterShown(rows, sh.hiddenR)
	cols = filterShown(cols, sh.hiddenC)
	cls := "print-grid"
	if p.Gridlines {
		cls += " lp-lines"
	}
	style := ""
	if pg.scale != 1 {
		style = fmt.Sprintf(` style="zoom:%s"`, strconv.FormatFloat(pg.scale, 'f', -1, 64))
	}
	width := 0.0
	for _, c := range cols {
		width += pg.width(c)
	}
	if p.Headings {
		width += headingsColPx
	}
	fmt.Fprintf(b, `<table class="%s"%s><colgroup>`, cls, style)
	if p.Headings {
		fmt.Fprintf(b, `<col style="width:%dpx">`, headingsColPx)
	}
	for _, c := range cols {
		fmt.Fprintf(b, `<col style="width:%dpx">`, int(pg.width(c)))
	}
	b.WriteString(`</colgroup><tbody>`)
	if p.Headings {
		fmt.Fprintf(b, `<tr class="lp-ch" style="height:%dpx"><th></th>`, headingsRowPx)
		for _, c := range cols {
			fmt.Fprintf(b, `<th>%s</th>`, canvas.ColLetters(c))
		}
		b.WriteString(`</tr>`)
	}
	// Fusions : la cellule de départ s'étend sur ce qui en est visible ici.
	type span struct{ rows, cols int }
	at := map[[2]int]span{}
	covered := map[[2]int]bool{}
	rowPos, colPos := map[int]int{}, map[int]int{}
	for i, r := range rows {
		rowPos[r] = i
	}
	for j, c := range cols {
		colPos[c] = j
	}
	for _, g := range sh.merges {
		i0, ok0 := rowPos[g[0]]
		j0, ok1 := colPos[g[1]]
		if !ok0 || !ok1 {
			continue
		}
		nr, nc := 0, 0
		for i := i0; i < len(rows) && rows[i] < g[0]+g[2]; i++ {
			nr++
		}
		for j := j0; j < len(cols) && cols[j] < g[1]+g[3]; j++ {
			nc++
		}
		if nr*nc <= 1 {
			continue
		}
		at[[2]int{g[0], g[1]}] = span{nr, nc}
		for i := i0; i < i0+nr; i++ {
			for j := j0; j < j0+nc; j++ {
				if i != i0 || j != j0 {
					covered[[2]int{rows[i], cols[j]}] = true
				}
			}
		}
	}
	for _, r := range rows {
		fmt.Fprintf(b, `<tr style="height:%dpx">`, int(pg.height(r)))
		if p.Headings {
			fmt.Fprintf(b, `<th>%d</th>`, r+1)
		}
		for _, c := range cols {
			k := [2]int{r, c}
			if covered[k] {
				continue
			}
			var cl []string
			if pg.inTable(r, c) {
				cl = append(cl, "t")
			}
			attr := ""
			if s, ok := at[k]; ok {
				if s.rows > 1 {
					attr += fmt.Sprintf(` rowspan="%d"`, s.rows)
				}
				if s.cols > 1 {
					attr += fmt.Sprintf(` colspan="%d"`, s.cols)
				}
				cl = append(cl, "c")
			}
			x, ok := sh.cells[k]
			if ok {
				if x.bold {
					cl = append(cl, "b")
				}
				if x.kind == 'n' || x.kind == 'd' {
					cl = append(cl, "n")
				}
				if x.kind == 'b' || x.kind == 'e' {
					cl = append(cl, "c")
				}
			}
			if len(cl) > 0 {
				attr += ` class="` + strings.Join(cl, " ") + `"`
			}
			if !ok {
				fmt.Fprintf(b, `<td%s></td>`, attr)
				continue
			}
			fmt.Fprintf(b, `<td%s>%s</td>`, attr, xmlEscape(printText(x)))
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
}

func (pg *pager) inTable(r, c int) bool {
	for _, t := range pg.sh.tableRects {
		if r >= t.R0 && r <= t.R1 && c >= t.C0 && c <= t.C1 {
			return true
		}
	}
	return false
}

func filterShown(xs []int, hidden map[int]bool) []int {
	out := xs[:0]
	for _, x := range xs {
		if !hidden[x] {
			out = append(out, x)
		}
	}
	return out
}

type hfVars struct {
	page, pages int
	sheet       string
	info        PrintInfo
}

// expandHF remplace les codes d'un en-tête ou d'un pied de page.
func expandHF(s string, v hfVars) string {
	if !strings.Contains(s, "&") {
		return s
	}
	file, dir := "", ""
	if v.info.Path != "" {
		file = filepath.Base(v.info.Path)
		dir = filepath.Dir(v.info.Path) + string(filepath.Separator)
	}
	return strings.NewReplacer(
		"&&", "&",
		"&[Page]", strconv.Itoa(v.page),
		"&[Pages]", strconv.Itoa(v.pages),
		"&[Date]", v.info.Now.Format("02/01/2006"),
		"&[Heure]", v.info.Now.Format("15:04"),
		"&[Fichier]", file,
		"&[Chemin]", dir,
		"&[Onglet]", v.sheet,
		"&[Feuille]", v.sheet,
	).Replace(s)
}

// PageBreaks sont les sauts de page d'une feuille, pour la grille (traits
// pointillés) : début de chaque page après la première, sauts manuels, et
// zone imprimée.
type PageBreaks struct {
	Rows       []int       `json:"rows"`
	Cols       []int       `json:"cols"`
	ManualRows []int       `json:"manualRows"`
	ManualCols []int       `json:"manualCols"`
	Area       canvas.Rect `json:"area"`
	Pages      int         `json:"pages"`
	Scale      int         `json:"scale"` // échelle retenue, en pour cent
	Empty      bool        `json:"empty"` // rien à imprimer
}

// SheetPageBreaks calcule les sauts de page de la feuille, comme PrintHTML.
func SheetPageBreaks(ctx context.Context, m *canvas.Model, sheet int64) (PageBreaks, error) {
	out := PageBreaks{Rows: []int{}, Cols: []int{}, ManualRows: []int{}, ManualCols: []int{}}
	sh, err := printSheet(ctx, m, sheet, false)
	if err != nil {
		return out, err
	}
	pg, ok := newPager(&sh)
	if !ok {
		out.Empty = true
		return out, nil
	}
	for _, r := range pg.rows[1:] {
		out.Rows = append(out.Rows, r[0])
	}
	for _, c := range pg.cols[1:] {
		out.Cols = append(out.Cols, c[0])
	}
	if !pg.p.Fit() {
		out.ManualRows = append(out.ManualRows, pg.p.RowBreaks...)
		out.ManualCols = append(out.ManualCols, pg.p.ColBreaks...)
	}
	out.Area = pg.area
	out.Pages = len(pg.rows) * len(pg.cols)
	out.Scale = int(pg.scale*100 + 0.5)
	return out, nil
}

// chartsNote dit que les graphiques de la feuille ne sont pas imprimés.
func chartsNote(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return `<p class="print-more">1 graphique non imprimé (l'impression d'une feuille ne reprend que ses cellules).</p>`
	}
	return fmt.Sprintf(`<p class="print-more">%d graphiques non imprimés (l'impression d'une feuille ne reprend que ses cellules).</p>`, n)
}

// printText écrit une cellule comme la grille l'affiche (locale française).
func printText(x xcell) string {
	switch x.kind {
	case 'n':
		return formula.FormatNumber(x.num)
	case 'd':
		return formula.FormatDate(x.num)
	case 'b':
		return formula.Boolean(x.num != 0).String()
	case 'e':
		for fr, en := range excelErrors {
			if en == x.str {
				return fr
			}
		}
	}
	return x.str
}
