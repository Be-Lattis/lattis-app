package transfer

import (
	"math"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Mise en forme propre à une feuille .xlsx, lue au premier passage (les
// balises autour de <sheetData>) : largeurs, styles et masquage des
// colonnes, hauteurs, styles et masquage des lignes, fusions, volets figés,
// zoom, couleur de l'onglet, mise en forme conditionnelle et validations.

type xCol struct {
	min, max int // à partir de 0
	width    float64
	custom   bool
	style    int
	hidden   bool
}

type xRow struct {
	row    int
	ht     float64
	custom bool
	hidden bool
	style  int // -1 : aucun
}

// xCFRule est une règle de mise en forme conditionnelle telle qu'écrite.
type xCFRule struct {
	attrs    map[string]string
	formulas []string
	kind     string // colorScale, dataBar, iconSet ("" : aucun)
	cfvos    []map[string]string
	colors   []xColor
}

type xCF struct {
	sqref string
	rules []*xCFRule
}

// xDV est une validation des données telle qu'écrite.
type xDV struct {
	attrs  map[string]string
	f1, f2 string
	sqref  string
}

type sheetExt struct {
	done     bool
	defWidth float64
	cols     []xCol
	rows     []xRow
	merges   []rect
	freezeR  int
	freezeC  int
	zoom     int
	tab      *xColor
	cfs      []*xCF
	dvs      []*xDV
	comments string // partie des commentaires (relation de la feuille)
	threaded string
	pivots   []string
}

// fmtReader suit les balises de mise en forme pendant un passage.
type fmtReader struct {
	e     *sheetExt
	ext   int // profondeur dans <extLst>
	cf    *xCF
	rule  *xCFRule
	dv    *xDV
	field string // texte capturé : formula, formula1, formula2, f, sqref
	buf   strings.Builder
}

func attrs(d *xmlScanner) map[string]string {
	m := make(map[string]string, len(d.attrs))
	for _, a := range d.attrs {
		m[string(local(a.name))] = string(a.value)
	}
	return m
}

func colorAttrs(d *xmlScanner) xColor {
	var c xColor
	c.RGB = string(d.attr("rgb"))
	if v, err := strconv.Atoi(string(d.attr("theme"))); err == nil {
		c.Theme = &v
	}
	if v, err := strconv.Atoi(string(d.attr("indexed"))); err == nil {
		c.Indexed = &v
	}
	c.Tint, _ = strconv.ParseFloat(string(d.attr("tint")), 64)
	c.Auto = string(d.attr("auto"))
	return c
}

func numAttr(d *xmlScanner, name string) (float64, bool) {
	v := d.attr(name)
	if v == nil {
		return 0, false
	}
	f, err := strconv.ParseFloat(string(v), 64)
	return f, err == nil
}

func flagAttr(d *xmlScanner, name string) bool {
	v := string(d.attr(name))
	return v == "1" || v == "true"
}

// start lit une balise ouvrante ; vrai si elle est prise (le passage n'a
// pas à la regarder).
func (r *fmtReader) start(d *xmlScanner, row int) bool {
	name := string(d.name)
	if r.ext > 0 {
		r.ext++ // la fin (même d'une balise vide) la redescend
		// Validations de l'extension x14 (listes tirées d'une autre feuille).
		switch name {
		case "dataValidation":
			r.dv = &xDV{attrs: attrs(d)}
		case "f", "sqref":
			if r.dv != nil {
				r.field = name
				r.buf.Reset()
			}
		}
		return true
	}
	e := r.e
	switch name {
	case "extLst":
		if !d.self {
			r.ext = 1
		}
		return true
	case "sheetFormatPr":
		e.defWidth, _ = numAttr(d, "defaultColWidth")
	case "sheetView":
		if z, ok := numAttr(d, "zoomScale"); ok {
			e.zoom = int(z)
		}
	case "pane":
		if s := string(d.attr("state")); s == "frozen" || s == "frozenSplit" {
			x, _ := numAttr(d, "xSplit")
			y, _ := numAttr(d, "ySplit")
			e.freezeC, e.freezeR = int(x), int(y)
		}
	case "tabColor":
		c := colorAttrs(d)
		e.tab = &c
	case "col":
		lo, _ := numAttr(d, "min")
		hi, _ := numAttr(d, "max")
		c := xCol{min: int(lo) - 1, max: int(hi) - 1, custom: flagAttr(d, "customWidth"), hidden: flagAttr(d, "hidden")}
		c.width, _ = numAttr(d, "width")
		s, _ := numAttr(d, "style")
		c.style = int(s)
		if c.min >= 0 && c.max >= c.min {
			e.cols = append(e.cols, c)
		}
		return true
	case "row":
		x := xRow{row: row, custom: flagAttr(d, "customHeight"), hidden: flagAttr(d, "hidden"), style: -1}
		x.ht, _ = numAttr(d, "ht")
		if flagAttr(d, "customFormat") {
			if s, ok := numAttr(d, "s"); ok {
				x.style = int(s)
			}
		}
		if x.custom && x.ht > 0 || x.hidden || x.style > 0 {
			e.rows = append(e.rows, x)
		}
		return false
	case "mergeCell":
		if rc, ok := rangeRect(string(d.attr("ref"))); ok {
			e.merges = append(e.merges, rc)
		}
		return true
	case "conditionalFormatting":
		r.cf = &xCF{sqref: string(d.attr("sqref"))}
		e.cfs = append(e.cfs, r.cf)
		return true
	case "cfRule":
		if r.cf != nil {
			r.rule = &xCFRule{attrs: attrs(d)}
			r.cf.rules = append(r.cf.rules, r.rule)
		}
		return true
	case "formula":
		if r.rule != nil {
			r.field = name
			r.buf.Reset()
		}
		return true
	case "colorScale", "dataBar", "iconSet":
		if r.rule != nil {
			r.rule.kind = name
			if name == "iconSet" || name == "dataBar" {
				for k, v := range attrs(d) {
					r.rule.attrs[name+"."+k] = v
				}
			}
		}
		return true
	case "cfvo":
		if r.rule != nil {
			r.rule.cfvos = append(r.rule.cfvos, attrs(d))
		}
		return true
	case "color":
		if r.rule != nil {
			r.rule.colors = append(r.rule.colors, colorAttrs(d))
			return true
		}
	case "dataValidation":
		r.dv = &xDV{attrs: attrs(d), sqref: string(d.attr("sqref"))}
		e.dvs = append(e.dvs, r.dv)
		return true
	case "formula1", "formula2":
		if r.dv != nil {
			r.field = name
			r.buf.Reset()
		}
		return true
	}
	return false
}

func (r *fmtReader) text(t []byte) bool {
	if r.field != "" {
		r.buf.Write(t)
		return true
	}
	return r.ext > 0 || r.rule != nil || r.dv != nil
}

func (r *fmtReader) end(name string) bool {
	if r.ext > 0 {
		r.ext--
		switch name {
		case "f":
			if r.dv != nil && r.field == "f" {
				r.dv.f1, r.field = r.buf.String(), ""
			}
		case "sqref":
			if r.dv != nil && r.field == "sqref" {
				r.dv.sqref, r.field = r.buf.String(), ""
			}
		case "dataValidation":
			if r.dv != nil {
				r.e.dvs = append(r.e.dvs, r.dv)
				r.dv = nil
			}
		}
		return true
	}
	switch name {
	case "formula":
		if r.rule != nil && r.field != "" {
			r.rule.formulas = append(r.rule.formulas, r.buf.String())
			r.field = ""
		}
		return true
	case "formula1":
		if r.dv != nil {
			r.dv.f1, r.field = r.buf.String(), ""
		}
		return true
	case "formula2":
		if r.dv != nil {
			r.dv.f2, r.field = r.buf.String(), ""
		}
		return true
	case "cfRule":
		r.rule = nil
		return true
	case "conditionalFormatting":
		r.cf, r.rule = nil, nil
		return true
	case "dataValidation":
		r.dv = nil
		return true
	case "colorScale", "dataBar", "iconSet", "cfvo", "color", "col", "mergeCell", "extLst":
		return r.rule != nil || name != "color"
	}
	return false
}

// rangeRect lit « A1:C3 » ou « B2 » (une cellule).
func rangeRect(s string) (rect, bool) {
	r0, c0, r1, c1, ok := parseRange(strings.ReplaceAll(s, "$", ""))
	return rect{r0: min(r0, r1), c0: min(c0, c1), r1: max(r0, r1), c1: max(c0, c1)}, ok
}

// pxOfWidth : largeur Excel (caractères) en pixels (export : px / 7).
func pxOfWidth(w float64) int { return int(math.Round(w * 7)) }

// sheetFormat assemble la mise en forme de la feuille (hors cellules) :
// cols sont bornées à maxCol (colonnes « jusqu'au bout » d'Excel).
func (x *xlsxFile) sheetFormat(sh xlsxSheet, maxCol int) *canvas.ImportFormat {
	e := sh.ext
	f := &canvas.ImportFormat{FreezeRows: e.freezeR, FreezeCols: e.freezeC, Zoom: e.zoom, Hidden: sh.hidden}
	if e.tab != nil && x.st != nil {
		if c := x.st.color(e.tab); c != "" {
			f.Color = "#" + strings.ToLower(c)
		}
	}
	def := e.defWidth
	if def == 0 {
		def = 9.140625
	}
	for _, c := range e.cols {
		hi := c.max
		if hi-c.min >= 256 {
			hi = min(hi, max(maxCol, c.min)) // « jusqu'au bout » : la partie utilisée
		}
		for i := c.min; i <= hi; i++ {
			if c.width > 0 && (c.custom || math.Abs(c.width-def) > 0.5) {
				f.Widths = append(f.Widths, canvas.ColWidth{Col: i, Width: pxOfWidth(c.width)})
			}
			st := canvas.Style{}
			for k, v := range x.st.style(c.style) {
				st[k] = v
			}
			if c.hidden {
				st["hd"] = true
			}
			if len(st) > 0 {
				f.ColStyles = append(f.ColStyles, canvas.ImportLine{Index: i, Style: st})
			}
		}
	}
	for _, r := range e.rows {
		if r.custom && r.ht > 0 {
			f.Heights = append(f.Heights, canvas.RowHeight{Row: r.row, Height: int(math.Round(r.ht / 0.75))})
		}
		st := canvas.Style{}
		if r.style > 0 {
			for k, v := range x.st.style(r.style) {
				st[k] = v
			}
		}
		if r.hidden {
			st["hd"] = true
		}
		if len(st) > 0 {
			f.RowStyles = append(f.RowStyles, canvas.ImportLine{Index: r.row, Style: st})
		}
	}
	return f
}
