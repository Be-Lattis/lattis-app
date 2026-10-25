package transfer

import (
	"context"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

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
	// File, Dir remplacent le nom et le dossier tirés de Path (classeur
	// ouvert depuis un serveur : son titre et l'adresse du serveur).
	File, Dir string
	Now       time.Time
}

// PrintSizes sont les tailles de la grille affichée (pixels de la feuille) :
// hauteur et largeur par défaut, puis celles qui en diffèrent ([indice,
// taille], 0 : masquée). Row nul : celles enregistrées dans le classeur.
// Les pages se découpent ainsi sur ce que montre l'écran (largeurs des
// colonnes de tableaux selon leur type, hauteurs ajustées).
type PrintSizes struct {
	Row  int      `json:"row"`
	Col  int      `json:"col"`
	Rows [][2]int `json:"rows"`
	Cols [][2]int `json:"cols"`
}

// PrintPage est une page imprimée : sa bande de lignes et sa bande de
// colonnes (rangs dans PrintPlan.Rows et Cols), son en-tête et son pied de
// page, codes remplacés.
type PrintPage struct {
	Row    int                 `json:"row"`
	Col    int                 `json:"col"`
	Header canvas.HeaderFooter `json:"header"`
	Footer canvas.HeaderFooter `json:"footer"`
}

// PrintPlan est le découpage d'une feuille en pages, pour l'impression et
// l'export PDF (fenêtre d'impression du système), selon sa mise en page
// (onglet Mise en page) : papier et orientation, marges, échelle ou
// ajustement en pages, zone d'impression (sinon la plage utilisée), titres
// répétés, sauts de page, quadrillage et en-têtes, centrage, en-tête et pied
// de page.
//
// Lattis découpe lui-même les pages : chaque page est un bloc de la taille
// du papier, sans marge d'imprimante, avec l'en-tête et le pied de page dans
// ses marges. Ainsi les colonnes répétées, l'ordre des pages et la
// numérotation marchent dans WebView2. Le contenu des pages (valeurs telles
// que la grille les affiche, mise en forme, mise en forme conditionnelle,
// graphiques) est écrit par l'interface (frontend/src/lib/print) avec les
// fonctions de la grille. Sans limite de lignes : le plan ne lit que la mise
// en page de la feuille, pas les lignes des tableaux.
type PrintPlan struct {
	Empty bool `json:"empty"`
	// Papier (mm, orientation comprise) et marges (cm).
	PaperW  float64        `json:"paperW"`
	PaperH  float64        `json:"paperH"`
	Margins canvas.Margins `json:"margins"`
	// Scale : échelle retenue (1 : 100 %).
	Scale float64 `json:"scale"`
	// Rows, Cols : bandes de lignes et de colonnes [première, dernière].
	Rows  [][2]int    `json:"rows"`
	Cols  [][2]int    `json:"cols"`
	Pages []PrintPage `json:"pages"`
	// TitleRows, TitleCols : titres répétés (nil : aucun).
	TitleRows *canvas.Span `json:"titleRows"`
	TitleCols *canvas.Span `json:"titleCols"`
	Area      canvas.Rect  `json:"area"`
	Gridlines bool         `json:"gridlines"`
	Headings  bool         `json:"headings"`
	CenterH   bool         `json:"centerH"`
	CenterV   bool         `json:"centerV"`
	// BlackWhite : en noir et blanc.
	BlackWhite bool `json:"blackWhite"`
	// Tailles retenues (pixels de la feuille) : par défaut, puis celles qui
	// en diffèrent dans la zone et les titres ([indice, taille], 0 : masquée).
	RowPx   int      `json:"rowPx"`
	ColPx   int      `json:"colPx"`
	Heights [][2]int `json:"heights"`
	Widths  [][2]int `json:"widths"`
	// HeadingsW, HeadingsH : largeur des numéros de lignes, hauteur des
	// lettres de colonnes imprimés (pixels).
	HeadingsW int `json:"headingsW"`
	HeadingsH int `json:"headingsH"`
}

// SheetPrintPlan découpe une feuille en pages selon sa mise en page.
func SheetPrintPlan(ctx context.Context, m *canvas.Model, sheet int64, sizes PrintSizes, info PrintInfo) (PrintPlan, error) {
	out := PrintPlan{Scale: 1, Rows: [][2]int{}, Cols: [][2]int{}, Pages: []PrintPage{}, Heights: [][2]int{}, Widths: [][2]int{},
		HeadingsW: headingsColPx, HeadingsH: headingsRowPx}
	sh, err := printSheet(ctx, m, sheet)
	if err != nil {
		return out, err
	}
	if info.Now.IsZero() {
		info.Now = time.Now()
	}
	p := sh.page
	out.PaperW, out.PaperH = p.PaperMM()
	out.Margins = p.PageMargins()
	out.Gridlines, out.Headings, out.CenterH, out.CenterV, out.BlackWhite = p.Gridlines, p.Headings, p.CenterH, p.CenterV, p.BlackWhite
	out.TitleRows, out.TitleCols = p.TitleRows, p.TitleCols
	pg, ok := newPager(&sh, sizes)
	out.RowPx, out.ColPx = pg.rowPx, pg.colPx
	if !ok {
		out.Empty = true
		return out, nil
	}
	out.Scale, out.Rows, out.Cols, out.Area = pg.scale, pg.rows, pg.cols, pg.area
	pages := pg.pages()
	first := max(p.FirstPage, 1)
	for n, at := range pages {
		v := hfVars{page: first + n, pages: len(pages), sheet: sh.name, info: info}
		expand := func(h canvas.HeaderFooter) canvas.HeaderFooter {
			return canvas.HeaderFooter{Left: expandHF(h.Left, v), Center: expandHF(h.Center, v), Right: expandHF(h.Right, v)}
		}
		out.Pages = append(out.Pages, PrintPage{Row: at[0], Col: at[1], Header: expand(p.Header), Footer: expand(p.Footer)})
	}
	// Tailles particulières de la zone et des titres.
	inSpan := func(i, lo, hi int, s *canvas.Span) bool {
		return (i >= lo && i <= hi) || (s != nil && i >= s.From && i <= s.To)
	}
	for _, r := range pg.special(true) {
		if inSpan(r, pg.area.R0, pg.area.R1, p.TitleRows) {
			out.Heights = append(out.Heights, [2]int{r, int(pg.height(r))})
		}
	}
	for _, c := range pg.special(false) {
		if inSpan(c, pg.area.C0, pg.area.C1, p.TitleCols) {
			out.Widths = append(out.Widths, [2]int{c, int(pg.width(c))})
		}
	}
	return out, nil
}

// printSheet lit la mise en page de la feuille à imprimer (cellules libres
// et place des tableaux, sans leurs lignes).
func printSheet(ctx context.Context, m *canvas.Model, sheet int64) (xsheet, error) {
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
		return xsheet{}, msg.New("transfer.sheetNotFound", msg.Args{"sheet": sheet})
	}
	sh, err := collectLayout(ctx, m, info, l, nil, nil)
	if err != nil {
		return sh, err
	}
	for _, xt := range sh.tables {
		t := xt.info
		sh.tableRects = append(sh.tableRects, canvas.Rect{R0: t.Row, C0: t.Col, R1: t.Row + t.Total, C1: t.Col + len(t.Columns) - 1})
	}
	sh.tables = nil
	for _, c := range l.Charts {
		w, h := c.Config.Width, c.Config.Height
		if w <= 0 {
			w = 480
		}
		if h <= 0 {
			h = 288
		}
		sh.charts = append(sh.charts, chartBox{row: c.Row, col: c.Col, x: c.Config.OffX, y: c.Config.OffY, w: w, h: h})
	}
	// Images, formes et zones de texte : imprimées aussi (comme les graphiques).
	for _, o := range l.Objects {
		c := o.Config
		sh.charts = append(sh.charts, chartBox{row: o.Row, col: o.Col, x: c.OffX, y: c.OffY, w: max(c.Width, 1), h: max(c.Height, 1)})
	}
	return sh, nil
}

// chartBox est la place d'un graphique : cellule d'ancrage, décalage et
// taille en pixels (comme la grille).
type chartBox struct{ row, col, x, y, w, h int }

// chartRects ajoute à la plage utilisée les cellules sous les graphiques
// (imprimés avec la feuille).
func (pg *pager) chartRects(add func(r0, c0, r1, c1 int)) {
	for _, b := range pg.sh.charts {
		// Dernière case sous le bord (bornée : cases masquées sans fin).
		end := func(i, off, size int, px func(int) float64) int {
			at := float64(off + size)
			for n := 0; n < 10_000; n, i = n+1, i+1 {
				if at -= px(i); at <= 0 {
					break
				}
			}
			return i
		}
		r0, c0 := b.row, b.col
		add(r0, c0, end(r0, b.y, b.h, pg.height), end(c0, b.x, b.w, pg.width))
	}
}

// pager découpe une feuille en pages.
type pager struct {
	sh   *xsheet
	p    canvas.PageSetup
	area canvas.Rect
	// Mise à l'échelle retenue (1 : 100 %), bandes de lignes et de colonnes.
	scale      float64
	rows, cols [][2]int
	// Tailles : par défaut, et particulières (masquées comprises : 0).
	rowPx, colPx    int
	heights, widths map[int]int
}

// usedRect est la plage utilisée : cellules, tableaux (lignes comprises) et
// graphiques.
func (pg *pager) usedRect() (canvas.Rect, bool) {
	sh := pg.sh
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
	pg.chartRects(add)
	return r, r.R0 >= 0
}

func newPager(sh *xsheet, sizes PrintSizes) (*pager, bool) {
	pg := &pager{sh: sh, p: sh.page, scale: 1, rowPx: defaultRowPx, colPx: defaultColPx, heights: map[int]int{}, widths: map[int]int{}}
	if sizes.Row > 0 && sizes.Col > 0 {
		// Tailles de la grille affichée.
		pg.rowPx, pg.colPx = sizes.Row, sizes.Col
		for _, s := range sizes.Rows {
			pg.heights[s[0]] = max(s[1], 0)
		}
		for _, s := range sizes.Cols {
			pg.widths[s[0]] = max(s[1], 0)
		}
	} else {
		for r, h := range sh.heights {
			pg.heights[r] = h
		}
		for _, w := range sh.widths {
			pg.widths[w.Col] = w.Width
		}
	}
	for r := range sh.hiddenR {
		pg.heights[r] = 0
	}
	for c := range sh.hiddenC {
		pg.widths[c] = 0
	}
	if a := pg.p.Area; a != nil {
		pg.area = *a
	} else {
		r, ok := pg.usedRect()
		if !ok {
			return pg, false
		}
		pg.area = r
	}
	pg.area.C1 = min(pg.area.C1, pg.area.C0+maxPrintCols-1)
	pg.area.R1 = min(pg.area.R1, ExcelRows-1)
	pg.paginate()
	return pg, true
}

func (pg *pager) height(r int) float64 {
	if h, ok := pg.heights[r]; ok {
		return float64(h)
	}
	return float64(pg.rowPx)
}

func (pg *pager) width(c int) float64 {
	if w, ok := pg.widths[c]; ok {
		return float64(w)
	}
	return float64(pg.colPx)
}

// special donne, triés, les lignes (ou colonnes) de taille particulière.
func (pg *pager) special(rows bool) []int {
	m := pg.widths
	if rows {
		m = pg.heights
	}
	out := make([]int, 0, len(m))
	for i := range m {
		out = append(out, i)
	}
	slices.Sort(out)
	return out
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
// un tableur).
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
	if v.info.File != "" {
		file, dir = v.info.File, v.info.Dir
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

// SheetPageBreaks calcule les sauts de page de la feuille, comme
// SheetPrintPlan (mêmes tailles : celles de la grille quand elle les donne).
func SheetPageBreaks(ctx context.Context, m *canvas.Model, sheet int64, sizes PrintSizes) (PageBreaks, error) {
	out := PageBreaks{Rows: []int{}, Cols: []int{}, ManualRows: []int{}, ManualCols: []int{}}
	sh, err := printSheet(ctx, m, sheet)
	if err != nil {
		return out, err
	}
	pg, ok := newPager(&sh, sizes)
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

// printText écrit une cellule comme la grille l'affiche (locale française) :
// exports .ods et .csv.
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
