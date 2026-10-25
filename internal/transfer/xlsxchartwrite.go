package transfer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Graphiques exportés en graphiques natifs d'Excel (xl/charts/chartN.xml),
// posés dans la partie de dessin de leur feuille : barres, colonnes,
// courbes, aires, secteurs, anneaux, nuages de points, radar et graphiques
// combinés, avec titre, titres des axes, légende, étiquettes, couleurs,
// bornes de l'axe et axe secondaire. Les séries citent les cellules du
// tableau exporté quand le graphique en lit les lignes une à une ; sinon
// (regroupement par catégorie, tableau croisé) leurs valeurs sont écrites
// dans le graphique. Les autres types (carte, cascade, jauge…) restent
// signalés dans le compte rendu.

// chartOut est un graphique à écrire.
type chartOut struct {
	ci  canvas.ChartInfo
	id  int // numéro dans le classeur (chartN.xml)
	xml string
}

// chartGroups : forme Excel de chaque type de graphique de Lattis.
var chartGroups = map[string]string{"bar": "bar", "column": "bar", "line": "line", "area": "area", "pie": "pie", "doughnut": "doughnut",
	"scatter": "scatter", "combo": "bar", "bar3d": "bar", "pie3d": "pie3D", "radar": "radar"}

// chartSeries : données d'une série (références et valeurs en cache).
type chartRefs struct {
	cat     string // plage des catégories ("" : valeurs écrites)
	vals    []string
	names   []string // cellule du nom de chaque série
	literal bool
}

// chartRefsOf cite les cellules du tableau source quand le graphique en
// lit les lignes dans l'ordre.
func chartRefsOf(ci canvas.ChartInfo, b *book) chartRefs {
	var out chartRefs
	tp := b.tables[strings.ToLower(ci.Config.Source)]
	if tp == nil || len(ci.Labels) != tp.total || tp.total == 0 || tp.row+tp.total >= ExcelRows {
		out.literal = true
		return out
	}
	q := quoteSheet(tp.sheet) + "!"
	col := func(name string) (int, bool) {
		j := tp.colIndex(name)
		return tp.col + j, j >= 0
	}
	if ci.Config.X != "" {
		c, ok := col(ci.Config.X)
		if !ok {
			out.literal = true
			return out
		}
		out.cat = fmt.Sprintf("%s$%s$%d:$%s$%d", q, colName(c), tp.row+2, colName(c), tp.row+1+tp.total)
	}
	for _, s := range ci.Config.Series {
		c, ok := col(s)
		if !ok {
			out.literal = true
			return out
		}
		out.vals = append(out.vals, fmt.Sprintf("%s$%s$%d:$%s$%d", q, colName(c), tp.row+2, colName(c), tp.row+1+tp.total))
		out.names = append(out.names, fmt.Sprintf("%s$%s$%d", q, colName(c), tp.row+1))
	}
	return out
}

func richText(s string) string {
	return `<c:tx><c:rich><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>` + xmlEscape(s) + `</a:t></a:r></a:p></c:rich></c:tx><c:overlay val="0"/>`
}

func strData(ref string, vals []string) string {
	var b strings.Builder
	if ref != "" {
		fmt.Fprintf(&b, `<c:strRef><c:f>%s</c:f><c:strCache>`, xmlEscape(ref))
	} else {
		b.WriteString(`<c:strLit>`)
	}
	fmt.Fprintf(&b, `<c:ptCount val="%d"/>`, len(vals))
	for i, v := range vals {
		fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, xmlEscape(v))
	}
	if ref != "" {
		b.WriteString(`</c:strCache></c:strRef>`)
	} else {
		b.WriteString(`</c:strLit>`)
	}
	return b.String()
}

func numData(ref string, vals []*float64) string {
	var b strings.Builder
	if ref != "" {
		fmt.Fprintf(&b, `<c:numRef><c:f>%s</c:f><c:numCache>`, xmlEscape(ref))
	} else {
		b.WriteString(`<c:numLit>`)
	}
	fmt.Fprintf(&b, `<c:formatCode>General</c:formatCode><c:ptCount val="%d"/>`, len(vals))
	for i, v := range vals {
		if v != nil {
			fmt.Fprintf(&b, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, strconv.FormatFloat(*v, 'g', -1, 64))
		}
	}
	if ref != "" {
		b.WriteString(`</c:numCache></c:numRef>`)
	} else {
		b.WriteString(`</c:numLit>`)
	}
	return b.String()
}

// chartXML écrit un graphique ; "" si son type n'a pas d'équivalent.
func chartXML(ci canvas.ChartInfo, b *book) string {
	cfg := ci.Config
	group, ok := chartGroups[cfg.Kind]
	if !ok || len(ci.Series) == 0 || ci.Error != "" {
		return ""
	}
	refs := chartRefsOf(ci, b)
	labels := `<c:dLbls><c:showLegendKey val="0"/><c:showVal val="1"/><c:showCatName val="0"/><c:showSerName val="0"/><c:showPercent val="0"/><c:showBubbleSize val="0"/></c:dLbls>`
	if !cfg.Labels {
		labels = ""
	}
	kindOf := func(i int) string {
		if cfg.Kind != "combo" {
			return group
		}
		k := ""
		if i < len(cfg.SeriesKinds) {
			k = cfg.SeriesKinds[i]
		}
		switch {
		case k == "":
			if i == 0 {
				return "bar"
			}
			return "line"
		}
		return k
	}
	secondary := func(i int) bool { return i < len(cfg.Secondary) && cfg.Secondary[i] }
	// Groupes : (forme, axe secondaire) dans l'ordre d'apparition.
	type grp struct {
		kind string
		sec  bool
		sers []int
	}
	var groups []*grp
	for i := range ci.Series {
		k, s := kindOf(i), secondary(i) && group != "pie" && group != "pie3D" && group != "doughnut"
		var g *grp
		for _, x := range groups {
			if x.kind == k && x.sec == s {
				g = x
			}
		}
		if g == nil {
			g = &grp{kind: k, sec: s}
			groups = append(groups, g)
		}
		g.sers = append(g.sers, i)
	}
	hasSec := false
	for _, g := range groups {
		hasSec = hasSec || g.sec
	}
	var w strings.Builder
	w.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><c:roundedCorners val="0"/><c:chart>`)
	if cfg.Title != "" {
		w.WriteString(`<c:title>` + richText(cfg.Title) + `</c:title><c:autoTitleDeleted val="0"/>`)
	} else {
		w.WriteString(`<c:autoTitleDeleted val="1"/>`)
	}
	if cfg.Kind == "pie3d" {
		w.WriteString(`<c:view3D><c:rotX val="30"/><c:rotY val="0"/><c:rAngAx val="0"/></c:view3D>`)
	}
	w.WriteString(`<c:plotArea><c:layout/>`)
	stacked := cfg.Stacked
	for gi, g := range groups {
		ax1, ax2 := 1, 2
		if g.sec {
			ax1, ax2 = 3, 4
		}
		tag := map[string]string{"bar": "barChart", "line": "lineChart", "area": "areaChart", "pie": "pieChart", "pie3D": "pie3DChart",
			"doughnut": "doughnutChart", "scatter": "scatterChart", "radar": "radarChart"}[g.kind]
		fmt.Fprintf(&w, `<c:%s>`, tag)
		switch g.kind {
		case "bar":
			dir := "col"
			if cfg.Kind == "column" {
				dir = "bar"
			}
			grouping := "clustered"
			if stacked {
				grouping = "stacked"
			}
			fmt.Fprintf(&w, `<c:barDir val="%s"/><c:grouping val="%s"/><c:varyColors val="0"/>`, dir, grouping)
		case "line", "area":
			grouping := "standard"
			if stacked {
				grouping = "stacked"
			}
			fmt.Fprintf(&w, `<c:grouping val="%s"/><c:varyColors val="0"/>`, grouping)
		case "pie", "pie3D", "doughnut":
			w.WriteString(`<c:varyColors val="1"/>`)
		case "scatter":
			w.WriteString(`<c:scatterStyle val="lineMarker"/><c:varyColors val="0"/>`)
		case "radar":
			w.WriteString(`<c:radarStyle val="marker"/><c:varyColors val="0"/>`)
		}
		for _, i := range g.sers {
			s := ci.Series[i]
			fmt.Fprintf(&w, `<c:ser><c:idx val="%d"/><c:order val="%d"/>`, i, i)
			name := ""
			if !refs.literal && i < len(refs.names) {
				name = refs.names[i]
			}
			w.WriteString(`<c:tx>` + strData(name, []string{s.Name}) + `</c:tx>`)
			if i < len(cfg.Colors) && color(cfg.Colors[i]) != "" && g.kind != "pie" && g.kind != "pie3D" && g.kind != "doughnut" {
				c := color(cfg.Colors[i])
				if g.kind == "line" || g.kind == "scatter" || g.kind == "radar" {
					fmt.Fprintf(&w, `<c:spPr><a:ln w="28575"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:ln></c:spPr>`, c)
				} else {
					fmt.Fprintf(&w, `<c:spPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill></c:spPr>`, c)
				}
			}
			if g.kind == "bar" {
				w.WriteString(`<c:invertIfNegative val="0"/>`)
			}
			if g.kind == "line" && cfg.Kind != "combo" {
				w.WriteString(`<c:marker><c:symbol val="none"/></c:marker>`)
			}
			cat, val := "", ""
			if !refs.literal {
				cat = refs.cat
				if i < len(refs.vals) {
					val = refs.vals[i]
				}
			}
			if g.kind == "scatter" {
				if cfg.X != "" {
					xs := ci.XValues
					if len(xs) != len(ci.Labels) {
						xs = make([]*float64, len(ci.Labels))
						for k, l := range ci.Labels {
							if f, err := strconv.ParseFloat(strings.ReplaceAll(l, ",", "."), 64); err == nil {
								xs[k] = &f
							}
						}
					}
					w.WriteString(`<c:xVal>` + numData(cat, xs) + `</c:xVal>`)
				}
				w.WriteString(`<c:yVal>` + numData(val, s.Values) + `</c:yVal><c:smooth val="0"/>`)
			} else {
				if cfg.X != "" || len(ci.Labels) > 0 {
					w.WriteString(`<c:cat>` + strData(cat, ci.Labels) + `</c:cat>`)
				}
				w.WriteString(`<c:val>` + numData(val, s.Values) + `</c:val>`)
				if g.kind == "line" {
					w.WriteString(`<c:smooth val="0"/>`)
				}
			}
			w.WriteString(`</c:ser>`)
		}
		w.WriteString(labels)
		switch g.kind {
		case "bar":
			w.WriteString(`<c:gapWidth val="150"/>`)
			if stacked {
				w.WriteString(`<c:overlap val="100"/>`)
			}
		case "line":
			w.WriteString(`<c:marker val="1"/>`)
		case "pie", "pie3D":
			if g.kind == "pie" {
				w.WriteString(`<c:firstSliceAng val="0"/>`)
			}
		case "doughnut":
			w.WriteString(`<c:firstSliceAng val="0"/><c:holeSize val="50"/>`)
		}
		if g.kind != "pie" && g.kind != "pie3D" && g.kind != "doughnut" {
			fmt.Fprintf(&w, `<c:axId val="%d"/><c:axId val="%d"/>`, ax1, ax2)
		}
		fmt.Fprintf(&w, `</c:%s>`, tag)
		_ = gi
	}
	if group != "pie" && group != "pie3D" && group != "doughnut" {
		title := func(s string) string {
			if s == "" {
				return ""
			}
			return `<c:title>` + richText(s) + `</c:title>`
		}
		grid := `<c:majorGridlines/>`
		if cfg.NoGrid {
			grid = ""
		}
		scaling := `<c:scaling><c:orientation val="minMax"/>`
		if cfg.YMax != nil {
			scaling += `<c:max val="` + strconv.FormatFloat(*cfg.YMax, 'g', -1, 64) + `"/>`
		}
		if cfg.YMin != nil {
			scaling += `<c:min val="` + strconv.FormatFloat(*cfg.YMin, 'g', -1, 64) + `"/>`
		}
		scaling += `</c:scaling>`
		catPos, valPos := "b", "l"
		if cfg.Kind == "column" {
			catPos, valPos = "l", "b"
		}
		if group == "scatter" {
			fmt.Fprintf(&w, `<c:valAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/>%s<c:numFmt formatCode="General" sourceLinked="1"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:crossBetween val="midCat"/></c:valAx>`, title(cfg.XTitle))
		} else {
			fmt.Fprintf(&w, `<c:catAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="%s"/>%s<c:numFmt formatCode="General" sourceLinked="1"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblAlgn val="ctr"/><c:lblOffset val="100"/></c:catAx>`, catPos, title(cfg.XTitle))
		}
		fmt.Fprintf(&w, `<c:valAx><c:axId val="2"/>%s<c:delete val="0"/><c:axPos val="%s"/>%s%s<c:numFmt formatCode="General" sourceLinked="1"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>`, scaling, valPos, grid, title(cfg.YTitle))
		if hasSec {
			w.WriteString(`<c:catAx><c:axId val="3"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="1"/><c:axPos val="b"/><c:numFmt formatCode="General" sourceLinked="1"/><c:tickLblPos val="nextTo"/><c:crossAx val="4"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblAlgn val="ctr"/><c:lblOffset val="100"/></c:catAx>`)
			fmt.Fprintf(&w, `<c:valAx><c:axId val="4"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="r"/>%s<c:numFmt formatCode="General" sourceLinked="1"/><c:tickLblPos val="nextTo"/><c:crossAx val="3"/><c:crosses val="max"/><c:crossBetween val="between"/></c:valAx>`, title(cfg.Y2Title))
		}
	}
	w.WriteString(`</c:plotArea>`)
	if pos := map[string]string{"": "b", "bottom": "b", "top": "t", "right": "r"}[cfg.Legend]; pos != "" {
		fmt.Fprintf(&w, `<c:legend><c:legendPos val="%s"/><c:overlay val="0"/></c:legend>`, pos)
	}
	blanks := map[string]string{"": "gap", "gap": "gap", "zero": "zero", "span": "span"}[cfg.Blanks]
	fmt.Fprintf(&w, `<c:plotVisOnly val="1"/><c:dispBlanksAs val="%s"/></c:chart></c:chartSpace>`, blanks)
	return w.String()
}

// collectCharts prépare les graphiques d'une feuille ; le compte rendu ne
// garde que ceux qui ne s'exportent pas.
func (e *sheetExtras) collectCharts(sh *xsheet, l canvas.Layout, b *book) {
	failed := 0
	for _, ci := range l.Charts {
		if ci.Row >= ExcelRows || ci.Col >= ExcelCols {
			failed++
			continue
		}
		x := chartXML(ci, b)
		if x == "" {
			failed++
			continue
		}
		e.charts = append(e.charts, chartOut{ci: ci, xml: x})
	}
	sh.rep.Charts = failed
}

// chartFrames écrit l'ancrage des graphiques de la feuille dans sa partie de
// dessin ; n et rid : objets et relations déjà écrits.
func chartFrames(b, rels *strings.Builder, s *xsheet, n, rid int) (int, int) {
	if s.extra == nil {
		return n, rid
	}
	for _, c := range s.extra.charts {
		cfg := c.ci.Config
		w, h := cfg.Width, cfg.Height
		if w <= 0 {
			w = 480
		}
		if h <= 0 {
			h = 288
		}
		n++
		rid++
		id := fmt.Sprintf("rId%d", rid)
		fmt.Fprintf(rels, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart%d.xml"/>`, id, c.id)
		fmt.Fprintf(b, `<xdr:oneCellAnchor><xdr:from><xdr:col>%d</xdr:col><xdr:colOff>%d</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>%d</xdr:rowOff></xdr:from><xdr:ext cx="%d" cy="%d"/>`,
			c.ci.Col, int64(cfg.OffX)*emuPerPx, c.ci.Row, int64(cfg.OffY)*emuPerPx, int64(w)*emuPerPx, int64(h)*emuPerPx)
		fmt.Fprintf(b, `<xdr:graphicFrame macro=""><xdr:nvGraphicFramePr><xdr:cNvPr id="%d" name="Graphique %d"/><xdr:cNvGraphicFramePr/></xdr:nvGraphicFramePr><xdr:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/></xdr:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="%s"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:oneCellAnchor>`,
			n+1, n, id)
	}
	return n, rid
}
