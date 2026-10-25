package transfer

import (
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Graphiques d'un fichier .xlsx (parties xl/charts, ancrées dans la partie
// de dessin de leur feuille) : un graphique en barres, colonnes, courbes,
// aires, secteurs, anneau, nuage de points ou radar devient un graphique
// de Lattis quand ses séries et ses catégories sont des colonnes d'un même
// tableau importé ; les autres restent signalés (non importés).

type xText struct {
	Rich *struct {
		Ps []struct {
			Rs []struct {
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"p"`
	} `xml:"tx>rich"`
	Cache []string `xml:"tx>strRef>strCache>pt>v"`
}

func (t *xText) text() string {
	if t == nil {
		return ""
	}
	var b strings.Builder
	if t.Rich != nil {
		for i, p := range t.Rich.Ps {
			if i > 0 {
				b.WriteString(" ")
			}
			for _, r := range p.Rs {
				b.WriteString(r.T)
			}
		}
	}
	if b.Len() == 0 && len(t.Cache) > 0 {
		b.WriteString(t.Cache[0])
	}
	return strings.TrimSpace(b.String())
}

type xRef struct {
	Num *struct {
		F string `xml:"f"`
	} `xml:"numRef"`
	Str *struct {
		F string `xml:"f"`
	} `xml:"strRef"`
	Multi *struct {
		F string `xml:"f"`
	} `xml:"multiLvlStrRef"`
}

func (r *xRef) f() string {
	switch {
	case r == nil:
	case r.Num != nil:
		return r.Num.F
	case r.Str != nil:
		return r.Str.F
	case r.Multi != nil:
		return r.Multi.F
	}
	return ""
}

type xSer struct {
	Tx struct {
		Ref *struct {
			F string `xml:"f"`
		} `xml:"strRef"`
		V string `xml:"v"`
	} `xml:"tx"`
	Fill *struct {
		Val string `xml:"val,attr"`
	} `xml:"spPr>solidFill>srgbClr"`
	Line *struct {
		Val string `xml:"val,attr"`
	} `xml:"spPr>ln>solidFill>srgbClr"`
	Cat  *xRef `xml:"cat"`
	Val  *xRef `xml:"val"`
	XVal *xRef `xml:"xVal"`
	YVal *xRef `xml:"yVal"`
}

type xGroup struct {
	BarDir   *xVal  `xml:"barDir"`
	Grouping *xVal  `xml:"grouping"`
	Sers     []xSer `xml:"ser"`
	AxIDs    []xVal `xml:"axId"`
	ShowVal  *xVal  `xml:"dLbls>showVal"`
}

type xAxis struct {
	ID     xVal      `xml:"axId"`
	Delete *xVal     `xml:"delete"`
	Pos    *xVal     `xml:"axPos"`
	Title  *xText    `xml:"title"`
	Max    *xVal     `xml:"scaling>max"`
	Min    *xVal     `xml:"scaling>min"`
	Grid   *struct{} `xml:"majorGridlines"`
}

type xChart struct {
	Title *xText `xml:"chart>title"`
	Plot  struct {
		Bar      []xGroup `xml:"barChart"`
		Bar3D    []xGroup `xml:"bar3DChart"`
		Line     []xGroup `xml:"lineChart"`
		Line3D   []xGroup `xml:"line3DChart"`
		Area     []xGroup `xml:"areaChart"`
		Pie      []xGroup `xml:"pieChart"`
		Pie3D    []xGroup `xml:"pie3DChart"`
		Doughnut []xGroup `xml:"doughnutChart"`
		Scatter  []xGroup `xml:"scatterChart"`
		Radar    []xGroup `xml:"radarChart"`
		CatAx    []xAxis  `xml:"catAx"`
		DateAx   []xAxis  `xml:"dateAx"`
		ValAx    []xAxis  `xml:"valAx"`
	} `xml:"chart>plotArea"`
	Legend *struct {
		Pos *xVal `xml:"legendPos"`
	} `xml:"chart>legend"`
	Blanks *xVal `xml:"chart>dispBlanksAs"`
}

func (v *xVal) s() string {
	if v == nil || v.Val == nil {
		return ""
	}
	return *v.Val
}

// chartAnchor : ancrage d'un cadre de graphique dans la partie de dessin.
type chartAnchor struct {
	From *xdrMarker `xml:"from"`
	To   *xdrMarker `xml:"to"`
	Ext  *xdrExt    `xml:"ext"`
	Pos  *struct {
		X int64 `xml:"x,attr"`
		Y int64 `xml:"y,attr"`
	} `xml:"pos"`
	Frame *struct {
		Chart struct {
			ID string `xml:"id,attr"`
		} `xml:"graphic>graphicData>chart"`
	} `xml:"graphicFrame"`
}

// chartCol : feuille, colonne et lignes d'une plage d'une colonne.
type chartCol struct {
	sheet  string
	col    int
	r0, r1 int
}

func parseChartCol(f, sheet string) (chartCol, bool) {
	f = strings.ReplaceAll(strings.TrimSpace(strings.Trim(f, "()")), "$", "")
	if k := strings.LastIndex(f, "!"); k >= 0 {
		sheet, f = f[:k], f[k+1:]
	}
	r, ok := rangeRect(f)
	if !ok || r.c0 != r.c1 {
		return chartCol{}, false
	}
	return chartCol{sheet: sheetKey(sheet), col: r.c0, r0: r.r0, r1: r.r1}, true
}

// tableCol situe une plage dans une colonne de tableau importé : le tableau
// et l'en-tête de la colonne.
func (x *xlsxFile) tableCol(c chartCol) (*island, string, bool) {
	for _, is := range x.rv.bySheet[c.sheet] {
		if is.table() && c.col >= is.c0 && c.col <= is.c1 && c.r0 >= is.r0 && c.r0 <= is.r0+1 && c.r1 <= is.r1 {
			return is, is.headers[c.col-is.c0], true
		}
	}
	return nil, "", false
}

// chartConfig traduit un graphique ; faux s'il n'a pas d'équivalent.
func (x *xlsxFile) chartConfig(ch *xChart, sheet string) (canvas.ChartConfig, bool) {
	var cfg canvas.ChartConfig
	type group struct {
		kind string
		g    xGroup
	}
	var groups []group
	add := func(kind string, gs []xGroup) {
		for _, g := range gs {
			groups = append(groups, group{kind, g})
		}
	}
	p := &ch.Plot
	add("bar", p.Bar)
	add("bar3d", p.Bar3D)
	add("line", p.Line)
	add("line", p.Line3D)
	add("area", p.Area)
	add("pie", p.Pie)
	add("pie3d", p.Pie3D)
	add("doughnut", p.Doughnut)
	add("scatter", p.Scatter)
	add("radar", p.Radar)
	if len(groups) == 0 {
		return cfg, false
	}
	// Axe secondaire : celui des valeurs placé à droite (ou en haut).
	secAx := map[string]bool{}
	var primary, second *xAxis
	for i := range p.ValAx {
		a := &p.ValAx[i]
		if pos := a.Pos.s(); (pos == "r" || pos == "t") && a.Delete.s() != "1" && len(groups) > 1 {
			secAx[a.ID.s()] = true
			second = a
		} else if primary == nil {
			primary = a
		}
	}
	var table *island
	kinds := map[string]bool{}
	for _, g := range groups {
		kind := g.kind
		if kind == "bar" || kind == "bar3d" {
			if g.g.BarDir.s() == "bar" {
				kind = "column"
			} else if kind == "bar3d" {
				kind = "bar3d"
			}
		}
		sec := false
		for _, id := range g.g.AxIDs {
			sec = sec || secAx[id.s()]
		}
		if gr := g.g.Grouping.s(); gr == "stacked" || gr == "percentStacked" {
			cfg.Stacked = true
		}
		if g.g.ShowVal.s() == "1" {
			cfg.Labels = true
		}
		for _, s := range g.g.Sers {
			val, cat := s.Val, s.Cat
			if g.kind == "scatter" {
				val, cat = s.YVal, s.XVal
			}
			vr, ok := parseChartCol(val.f(), sheet)
			if !ok {
				return cfg, false
			}
			is, head, ok := x.tableCol(vr)
			if !ok || table != nil && is != table {
				return cfg, false
			}
			table = is
			cfg.Series = append(cfg.Series, head)
			if f := cat.f(); f != "" {
				cr, ok := parseChartCol(f, sheet)
				if !ok {
					return cfg, false
				}
				cis, xh, ok := x.tableCol(cr)
				if !ok || cis != is || cfg.X != "" && cfg.X != xh {
					return cfg, false
				}
				cfg.X = xh
			}
			cfg.Secondary = append(cfg.Secondary, sec)
			sk := map[string]string{"bar": "bar", "bar3d": "bar", "column": "bar", "line": "line", "area": "area"}[kind]
			cfg.SeriesKinds = append(cfg.SeriesKinds, sk)
			c := ""
			if s.Fill != nil {
				c = s.Fill.Val
			} else if s.Line != nil {
				c = s.Line.Val
			}
			if h := argb(c); h != "" {
				cfg.Colors = append(cfg.Colors, "#"+strings.ToLower(h))
			} else {
				cfg.Colors = append(cfg.Colors, "")
			}
		}
		kinds[kind] = true
		cfg.Kind = kind
	}
	if table == nil || len(cfg.Series) == 0 {
		return cfg, false
	}
	cfg.Source = table.name
	if len(kinds) > 1 {
		for k := range kinds {
			if k != "bar" && k != "line" && k != "area" {
				return cfg, false
			}
		}
		cfg.Kind = "combo"
	} else {
		cfg.SeriesKinds = nil
	}
	if cfg.X == "" {
		cfg.X = table.headers[0] // sans catégories : la première colonne
	}
	anySec, anyColor := false, false
	for i := range cfg.Series {
		anySec = anySec || cfg.Secondary[i]
		anyColor = anyColor || cfg.Colors[i] != ""
	}
	if !anySec {
		cfg.Secondary = nil
	}
	if !anyColor {
		cfg.Colors = nil
	}
	cfg.Title = ch.Title.text()
	for _, a := range append(append([]xAxis{}, p.CatAx...), p.DateAx...) {
		if a.Delete.s() != "1" && cfg.XTitle == "" {
			cfg.XTitle = a.Title.text()
		}
	}
	if cfg.Kind == "scatter" && len(p.ValAx) > 0 {
		for _, a := range p.ValAx {
			if a.Pos.s() == "b" {
				cfg.XTitle = a.Title.text()
			}
		}
	}
	if primary != nil {
		cfg.YTitle = primary.Title.text()
		if v, err := strconv.ParseFloat(primary.Max.s(), 64); err == nil {
			cfg.YMax = &v
		}
		if v, err := strconv.ParseFloat(primary.Min.s(), 64); err == nil {
			cfg.YMin = &v
		}
		if primary.Grid == nil && cfg.Kind != "pie" && cfg.Kind != "pie3d" && cfg.Kind != "doughnut" {
			cfg.NoGrid = true
		}
	}
	if second != nil {
		cfg.Y2Title = second.Title.text()
	}
	switch {
	case ch.Legend == nil:
		cfg.Legend = "none"
	case ch.Legend.Pos.s() == "t":
		cfg.Legend = "top"
	case ch.Legend.Pos.s() == "r" || ch.Legend.Pos.s() == "l" || ch.Legend.Pos.s() == "tr":
		cfg.Legend = "right"
	}
	if b := ch.Blanks.s(); b == "zero" || b == "span" {
		cfg.Blanks = b
	}
	return cfg, true
}

// readCharts lit les graphiques ancrés dans les feuilles.
func (x *xlsxFile) readCharts(sheets []canvas.ImportSheet) {
	if x.rv == nil {
		return
	}
	for i, sh := range x.sheets {
		if i >= len(sheets) || sh.file == nil {
			continue
		}
		_, byType, err := x.rels(sh.file.Name)
		if err != nil || byType["drawing"] == "" {
			continue
		}
		dp := byType["drawing"]
		f := x.find(dp)
		if f == nil {
			continue
		}
		var d struct {
			Anchors []chartAnchor `xml:",any"`
		}
		if x.decode(f, &d) != nil {
			continue
		}
		targets, _, _ := x.rels(dp)
		widths := map[int]int{}
		heights := map[int]int{}
		if sh.ext != nil {
			for _, c := range sh.ext.cols {
				for k := c.min; k <= min(c.max, c.min+255); k++ {
					if c.width > 0 {
						widths[k] = pxOfWidth(c.width)
					}
				}
			}
			for _, r := range sh.ext.rows {
				if r.custom && r.ht > 0 {
					heights[r.row] = int(r.ht / 0.75)
				}
			}
		}
		colPx := func(c int) int {
			if w, ok := widths[c]; ok {
				return w
			}
			return 64
		}
		rowPx := func(r int) int {
			if h, ok := heights[r]; ok {
				return h
			}
			return 20
		}
		for _, a := range d.Anchors {
			if a.Frame == nil || a.Frame.Chart.ID == "" {
				continue
			}
			cf := x.find(targets[a.Frame.Chart.ID])
			if cf == nil {
				continue
			}
			var ch xChart
			if x.decode(cf, &ch) != nil {
				continue
			}
			cfg, ok := x.chartConfig(&ch, sh.name)
			if !ok {
				continue
			}
			var c canvas.ImportChart
			switch {
			case a.From != nil:
				c.Row, c.Col = int(a.From.Row), int(a.From.Col)
				cfg.OffX, cfg.OffY = int(a.From.ColOff/emuPerPx), int(a.From.RowOff/emuPerPx)
			case a.Pos != nil:
				px, py := int(a.Pos.X/emuPerPx), int(a.Pos.Y/emuPerPx)
				c.Col, cfg.OffX, c.Row, cfg.OffY = px/64, px%64, py/20, py%20
			default:
				continue
			}
			switch {
			case a.Ext != nil && a.Ext.Cx > 0:
				cfg.Width, cfg.Height = int(a.Ext.Cx/emuPerPx), int(a.Ext.Cy/emuPerPx)
			case a.To != nil:
				w := int(a.To.ColOff/emuPerPx) - cfg.OffX
				for k := c.Col; k < int(a.To.Col); k++ {
					w += colPx(k)
				}
				h := int(a.To.RowOff/emuPerPx) - cfg.OffY
				for k := c.Row; k < int(a.To.Row); k++ {
					h += rowPx(k)
				}
				cfg.Width, cfg.Height = w, h
			}
			cfg.Width = max(cfg.Width, canvas.MinChartWidth)
			cfg.Height = max(cfg.Height, canvas.MinChartHeight)
			c.Config = cfg
			sheets[i].Charts = append(sheets[i].Charts, c)
			x.imp.charts++
		}
	}
}
