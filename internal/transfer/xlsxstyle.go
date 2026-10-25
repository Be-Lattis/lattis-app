package transfer

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/numfmt"
)

// Lecture de la mise en forme d'un classeur .xlsx (xl/styles.xml et le
// thème) : chaque style de cellule (cellXfs, attribut s) devient un style de
// Lattis (canvas.Style) : police, taille, gras, italique, souligné, barré,
// couleur, remplissage, bordures, alignement, renvoi à la ligne et format de
// nombre. Les couleurs du thème (avec leur nuance) et de la palette indexée
// sont résolues en #rrggbb. Ce qui vaut la police par défaut du classeur
// (Calibri 11, noir) n'est pas repris : la cellule garde celle de Lattis.
// Les formats différentiels (dxfs) servent à la mise en forme conditionnelle.

type xColor struct {
	RGB     string  `xml:"rgb,attr"`
	Theme   *int    `xml:"theme,attr"`
	Indexed *int    `xml:"indexed,attr"`
	Tint    float64 `xml:"tint,attr"`
	Auto    string  `xml:"auto,attr"`
}

type xVal struct {
	Val *string `xml:"val,attr"`
}

type xFont struct {
	B      *xVal   `xml:"b"`
	I      *xVal   `xml:"i"`
	U      *xVal   `xml:"u"`
	Strike *xVal   `xml:"strike"`
	Sz     *xVal   `xml:"sz"`
	Name   *xVal   `xml:"name"`
	Color  *xColor `xml:"color"`
}

type xFill struct {
	Pattern *struct {
		Type string  `xml:"patternType,attr"`
		Fg   *xColor `xml:"fgColor"`
		Bg   *xColor `xml:"bgColor"`
	} `xml:"patternFill"`
	Gradient *struct {
		Stops []struct {
			Color xColor `xml:"color"`
		} `xml:"stop"`
	} `xml:"gradientFill"`
}

type xEdge struct {
	Style string  `xml:"style,attr"`
	Color *xColor `xml:"color"`
}

type xBorder struct {
	Left   *xEdge `xml:"left"`
	Right  *xEdge `xml:"right"`
	Top    *xEdge `xml:"top"`
	Bottom *xEdge `xml:"bottom"`
	Start  *xEdge `xml:"start"`
	End    *xEdge `xml:"end"`
}

type xAlign struct {
	H    string `xml:"horizontal,attr"`
	V    string `xml:"vertical,attr"`
	Wrap string `xml:"wrapText,attr"`
}

type xXf struct {
	NumFmt int     `xml:"numFmtId,attr"`
	Font   int     `xml:"fontId,attr"`
	Fill   int     `xml:"fillId,attr"`
	Border int     `xml:"borderId,attr"`
	Align  *xAlign `xml:"alignment"`
}

type xDxf struct {
	Font   *xFont `xml:"font"`
	NumFmt *struct {
		ID   int    `xml:"numFmtId,attr"`
		Code string `xml:"formatCode,attr"`
	} `xml:"numFmt"`
	Fill   *xFill   `xml:"fill"`
	Border *xBorder `xml:"border"`
	Align  *xAlign  `xml:"alignment"`
}

type xStyleSheet struct {
	NumFmts []struct {
		ID   int    `xml:"numFmtId,attr"`
		Code string `xml:"formatCode,attr"`
	} `xml:"numFmts>numFmt"`
	Fonts   []xFont   `xml:"fonts>font"`
	Fills   []xFill   `xml:"fills>fill"`
	Borders []xBorder `xml:"borders>border"`
	Xfs     []xXf     `xml:"cellXfs>xf"`
	Dxfs    []xDxf    `xml:"dxfs>dxf"`
	Indexed []struct {
		RGB string `xml:"rgb,attr"`
	} `xml:"colors>indexedColors>rgbColor"`
}

// bookStyles : la mise en forme lue d'un classeur.
type bookStyles struct {
	xfs  []canvas.Style // par style de cellule (attribut s) ; nil : aucune
	enc  []string       // xfs encodés (comparaison)
	dxfs []canvas.Style
	// theme : couleurs du thème dans l'ordre des index (lt1, dk1, lt2, dk2,
	// accent1 à 6, hlink, folHlink) ; indexed : la palette.
	theme   []string
	indexed []string
	// Police par défaut du classeur (police 0) : ce qui la vaut est omis.
	defName  string
	defSize  float64
	defColor string
}

// defaultIndexed est la palette indexée par défaut (64 couleurs).
var defaultIndexed = strings.Fields(`000000 FFFFFF FF0000 00FF00 0000FF FFFF00 FF00FF 00FFFF
000000 FFFFFF FF0000 00FF00 0000FF FFFF00 FF00FF 00FFFF
800000 008000 000080 808000 800080 008080 C0C0C0 808080
9999FF 993366 FFFFCC CCFFFF 660066 FF8080 0066CC CCCCFF
000080 FF00FF FFFF00 00FFFF 800080 800000 008080 0000FF
00CCFF CCFFFF CCFFCC FFFF99 99CCFF FF99CC CC99FF FFCC99
3366FF 33CCCC 99CC00 FFCC00 FF9900 FF6600 666699 969696
003366 339966 003300 333300 993300 993366 333399 333333`)

// defaultTheme : couleurs du thème Office (classeur sans thème).
var defaultTheme = []string{"FFFFFF", "000000", "E7E6E6", "44546A", "4472C4", "ED7D31", "A5A5A5", "FFC000", "5B9BD5", "70AD47", "0563C1", "954F72"}

// readTheme lit les couleurs d'un thème (xl/theme/theme1.xml).
func (x *xlsxFile) readTheme(f *zip.File) []string {
	var raw struct {
		Scheme struct {
			Inner []byte `xml:",innerxml"`
		} `xml:"themeElements>clrScheme"`
	}
	if f == nil || x.decode(f, &raw) != nil {
		return defaultTheme
	}
	got := map[string]string{}
	d := newXMLScanner(strings.NewReader(string(raw.Scheme.Inner)))
	cur := ""
	for d.next() == nil {
		if d.kind != xmlStart {
			continue
		}
		switch n := string(d.name); n {
		case "dk1", "lt1", "dk2", "lt2", "accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "hlink", "folHlink":
			cur = n
		case "srgbClr":
			if cur != "" {
				got[cur] = string(d.attr("val"))
			}
		case "sysClr":
			if cur != "" {
				got[cur] = string(d.attr("lastClr"))
			}
		}
	}
	out := append([]string(nil), defaultTheme...)
	for i, k := range []string{"lt1", "dk1", "lt2", "dk2", "accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "hlink", "folHlink"} {
		if v := got[k]; len(v) == 6 {
			out[i] = strings.ToUpper(v)
		}
	}
	return out
}

// readFormats lit la mise en forme (styles et thème) ; nil si illisible.
func (x *xlsxFile) readFormats(styles, theme *zip.File) *bookStyles {
	var ss xStyleSheet
	if styles == nil || x.decode(styles, &ss) != nil {
		return nil
	}
	st := &bookStyles{theme: x.readTheme(theme), indexed: defaultIndexed}
	if len(ss.Indexed) > 0 {
		st.indexed = nil
		for _, c := range ss.Indexed {
			st.indexed = append(st.indexed, argb(c.RGB))
		}
	}
	st.defName, st.defSize, st.defColor = "Calibri", 11, "000000"
	if len(ss.Fonts) > 0 {
		f := ss.Fonts[0]
		if f.Name != nil && f.Name.Val != nil {
			st.defName = *f.Name.Val
		}
		if f.Sz != nil && f.Sz.Val != nil {
			st.defSize, _ = strconv.ParseFloat(*f.Sz.Val, 64)
		}
		if c := st.color(f.Color); c != "" {
			st.defColor = c
		}
	}
	codes := map[int]string{}
	for _, n := range ss.NumFmts {
		codes[n.ID] = n.Code
	}
	st.xfs = make([]canvas.Style, len(ss.Xfs))
	st.enc = make([]string, len(ss.Xfs))
	for i, xf := range ss.Xfs {
		s := canvas.Style{}
		if xf.Font >= 0 && xf.Font < len(ss.Fonts) {
			st.font(s, &ss.Fonts[xf.Font], true)
		}
		if xf.Fill >= 0 && xf.Fill < len(ss.Fills) {
			st.fill(s, &ss.Fills[xf.Fill], false)
		}
		if xf.Border >= 0 && xf.Border < len(ss.Borders) {
			st.border(s, &ss.Borders[xf.Border])
		}
		align(s, xf.Align)
		code, custom := codes[xf.NumFmt]
		for k, v := range numStyle(xf.NumFmt, code, custom) {
			s[k] = v
		}
		if len(s) > 0 {
			st.xfs[i] = s
			b, _ := json.Marshal(s)
			st.enc[i] = string(b)
		}
	}
	for _, d := range ss.Dxfs {
		s := canvas.Style{}
		if d.Font != nil {
			st.font(s, d.Font, false)
		}
		if d.Fill != nil {
			st.fill(s, d.Fill, true)
		}
		if d.Border != nil {
			st.border(s, d.Border)
		}
		align(s, d.Align)
		if d.NumFmt != nil {
			for k, v := range numStyle(d.NumFmt.ID, d.NumFmt.Code, d.NumFmt.Code != "") {
				s[k] = v
			}
		}
		st.dxfs = append(st.dxfs, s)
	}
	return st
}

// style donne la mise en forme du style de cellule i (nil : aucune).
func (st *bookStyles) style(i int) canvas.Style {
	if st == nil || i < 0 || i >= len(st.xfs) {
		return nil
	}
	return st.xfs[i]
}

func (st *bookStyles) dxf(i int) canvas.Style {
	if st == nil || i < 0 || i >= len(st.dxfs) {
		return canvas.Style{}
	}
	out := canvas.Style{}
	for k, v := range st.dxfs[i] {
		out[k] = v
	}
	return out
}

// argb ramène « FFRRGGBB » ou « RRGGBB » à RRGGBB ("" si illisible).
func argb(s string) string {
	if len(s) == 8 {
		s = s[2:]
	}
	if len(s) != 6 {
		return ""
	}
	if _, err := strconv.ParseUint(s, 16, 32); err != nil {
		return ""
	}
	return strings.ToUpper(s)
}

// color résout une couleur (RRGGBB, "" : automatique ou illisible).
func (st *bookStyles) color(c *xColor) string {
	if c == nil || c.Auto == "1" || c.Auto == "true" {
		return ""
	}
	out := ""
	switch {
	case c.RGB != "":
		out = argb(c.RGB)
	case c.Theme != nil:
		if *c.Theme >= 0 && *c.Theme < len(st.theme) {
			out = st.theme[*c.Theme]
		}
	case c.Indexed != nil:
		// 64 et 65 : couleurs du système (texte, fond) : automatique.
		if *c.Indexed >= 0 && *c.Indexed < len(st.indexed) {
			out = st.indexed[*c.Indexed]
		}
	}
	if out != "" && c.Tint != 0 {
		out = tint(out, c.Tint)
	}
	return out
}

// tint éclaircit (tint > 0) ou assombrit une couleur, comme Excel (sur la
// luminosité TSL).
func tint(hex string, t float64) string {
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return hex
	}
	r, g, b := float64(v>>16&0xff)/255, float64(v>>8&0xff)/255, float64(v&0xff)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l := (mx + mn) / 2
	h, s := 0.0, 0.0
	if mx != mn {
		d := mx - mn
		if l > 0.5 {
			s = d / (2 - mx - mn)
		} else {
			s = d / (mx + mn)
		}
		switch mx {
		case r:
			h = (g - b) / d
			if g < b {
				h += 6
			}
		case g:
			h = (b-r)/d + 2
		default:
			h = (r-g)/d + 4
		}
		h /= 6
	}
	if t < 0 {
		l *= 1 + t
	} else {
		l = l*(1-t) + t
	}
	hue := func(p, q, x float64) float64 {
		if x < 0 {
			x++
		}
		if x > 1 {
			x--
		}
		switch {
		case x < 1.0/6:
			return p + (q-p)*6*x
		case x < 0.5:
			return q
		case x < 2.0/3:
			return p + (q-p)*(2.0/3-x)*6
		}
		return p
	}
	if s == 0 {
		r, g, b = l, l, l
	} else {
		q := l * (1 + s)
		if l >= 0.5 {
			q = l + s - l*s
		}
		p := 2*l - q
		r, g, b = hue(p, q, h+1.0/3), hue(p, q, h), hue(p, q, h-1.0/3)
	}
	c := func(f float64) int { return int(math.Round(math.Min(math.Max(f, 0), 1) * 255)) }
	return fmt.Sprintf("%02X%02X%02X", c(r), c(g), c(b))
}

func on(v *xVal) bool {
	return v != nil && (v.Val == nil || *v.Val != "0" && *v.Val != "false" && *v.Val != "none")
}

// font : police (base : ce qui vaut la police par défaut est omis ; dans un
// format différentiel, tout ce qui est écrit compte).
func (st *bookStyles) font(s canvas.Style, f *xFont, base bool) {
	if on(f.B) {
		s["b"] = true
	}
	if on(f.I) {
		s["i"] = true
	}
	if on(f.U) {
		s["u"] = true
	}
	if on(f.Strike) {
		s["x"] = true
	}
	if f.Name != nil && f.Name.Val != nil && *f.Name.Val != "" && (!base || *f.Name.Val != st.defName) {
		s["ff"] = *f.Name.Val
	}
	if f.Sz != nil && f.Sz.Val != nil {
		if sz, err := strconv.ParseFloat(*f.Sz.Val, 64); err == nil && sz > 0 && sz < 410 && (!base || sz != st.defSize) {
			s["fs"] = sz
		}
	}
	if c := st.color(f.Color); c != "" && (!base || c != st.defColor) {
		s["fg"] = "#" + strings.ToLower(c)
	}
}

// fill : remplissage uni (un motif prend sa couleur de premier plan, un
// dégradé sa première couleur). Dans un format différentiel, la couleur
// d'un remplissage uni est celle de fond (bgColor).
func (st *bookStyles) fill(s canvas.Style, f *xFill, dxf bool) {
	c := ""
	switch {
	case f.Pattern != nil:
		p := f.Pattern
		if p.Type == "none" || p.Type == "" && !dxf {
			return
		}
		if dxf {
			if c = st.color(p.Bg); c == "" {
				c = st.color(p.Fg)
			}
		} else if c = st.color(p.Fg); c == "" && p.Type != "solid" {
			c = st.color(p.Bg)
		}
	case f.Gradient != nil && len(f.Gradient.Stops) > 0:
		c = st.color(&f.Gradient.Stops[0].Color)
	}
	if c != "" {
		s["bg"] = "#" + strings.ToLower(c)
	}
}

// edgeKinds : style de bord d'Excel vers celui de Lattis.
var edgeKinds = map[string]string{"thin": "thin", "hair": "thin", "medium": "medium", "thick": "thick", "double": "double",
	"dotted": "dotted", "dashed": "dashed", "mediumDashed": "dashed", "dashDot": "dashed", "mediumDashDot": "dashed",
	"dashDotDot": "dotted", "mediumDashDotDot": "dotted", "slantDashDot": "dashed"}

func (st *bookStyles) border(s canvas.Style, b *xBorder) {
	left, right := b.Left, b.Right
	if left == nil {
		left = b.Start
	}
	if right == nil {
		right = b.End
	}
	for _, e := range []struct {
		k string
		e *xEdge
	}{{"bt", b.Top}, {"bb", b.Bottom}, {"bl", left}, {"br", right}} {
		if e.e == nil {
			continue
		}
		kind := edgeKinds[e.e.Style]
		if kind == "" {
			continue
		}
		c := st.color(e.e.Color)
		switch {
		case c == "" && kind == "thin":
			s[e.k] = true
		case c == "":
			s[e.k] = kind
		default:
			s[e.k] = kind + " #" + strings.ToLower(c)
		}
	}
}

func align(s canvas.Style, a *xAlign) {
	if a == nil {
		return
	}
	switch a.H {
	case "left", "right", "center":
		s["al"] = a.H
	case "centerContinuous", "distributed":
		s["al"] = "center"
	case "justify", "fill":
		s["al"] = "left"
	}
	switch a.V {
	case "top":
		s["va"] = "top"
	case "bottom":
		s["va"] = "bottom"
	}
	if a.Wrap == "1" || a.Wrap == "true" {
		s["wr"] = true
	}
}

// builtinCodes : formats intégrés qui n'ont pas d'équivalent simple (code
// anglais, comme dans un fichier).
var builtinCodes = map[int]string{
	11: "0.00E+00", 12: "# ?/?", 13: "# ??/??", 18: "h:mm AM/PM", 19: "h:mm:ss AM/PM",
	37: "#,##0 ;(#,##0)", 38: "#,##0 ;[Red](#,##0)", 39: "#,##0.00;(#,##0.00)", 40: "#,##0.00;[Red](#,##0.00)",
	45: "mm:ss", 46: "[h]:mm:ss", 47: "mm:ss.0", 48: "##0.0E+0",
}

var (
	numRe   = regexp.MustCompile(`^(#,##)?0(\.0+)?$`)
	pctRe   = regexp.MustCompile(`^0(\.0+)?%$`)
	euroRe  = regexp.MustCompile(`^(#,##)?0(\.0+)?(\\ | |\\\x{a0}|\x{a0})?("€"|€|\[\$€[^\]]*\])$`)
	plainDf = regexp.MustCompile(`^(yyyy|yy|mmmm|mmm|mm|dddd|ddd|dd|hh|ss|[/\-:., ]|\\[/\-:., ])+$`)
)

// numStyle traduit un format de nombre (identifiant intégré, ou code d'un
// format personnalisé) : en format simple quand il en a un (nf, dp, ng, df),
// sinon en code personnalisé (nc).
func numStyle(id int, code string, custom bool) canvas.Style {
	if !custom {
		switch id {
		case 0:
			return nil
		case 1:
			return canvas.Style{"nf": "0", "ng": true}
		case 2:
			return canvas.Style{"nf": "0,00", "ng": true}
		case 3:
			return canvas.Style{"nf": "0"}
		case 4:
			return canvas.Style{"nf": "0,00"}
		case 9:
			return canvas.Style{"nf": "%", "dp": 0.0}
		case 10:
			return canvas.Style{"nf": "%"}
		case 14:
			return canvas.Style{"nf": "date"}
		case 15:
			return canvas.Style{"nf": "date", "df": "jj-mmm-aa"}
		case 16:
			return canvas.Style{"nf": "date", "df": "jj-mmm"}
		case 17:
			return canvas.Style{"nf": "date", "df": "mmm-aa"}
		case 20:
			return canvas.Style{"nf": "time"}
		case 21:
			return canvas.Style{"nf": "time", "df": "hh:mm:ss"}
		case 22:
			return canvas.Style{"nf": "date", "df": "jj/mm/aaaa hh:mm"}
		case 49:
			return canvas.Style{"nf": "text"}
		}
		if c, ok := builtinCodes[id]; ok {
			return canvas.Style{"nc": numfmt.FromExcel(c)}
		}
		if builtinKind(id) != notDate {
			return canvas.Style{"nf": "date"}
		}
		return nil
	}
	c := strings.TrimSpace(code)
	switch strings.ToLower(c) {
	case "", "general":
		return nil
	case "@":
		return canvas.Style{"nf": "text"}
	}
	if m := numRe.FindStringSubmatch(c); m != nil {
		return simpleNum(m[1] != "", len(m[2])-min(len(m[2]), 1))
	}
	if m := pctRe.FindStringSubmatch(c); m != nil {
		s := canvas.Style{"nf": "%"}
		if dp := len(m[1]) - min(len(m[1]), 1); dp != 2 {
			s["dp"] = float64(dp)
		}
		return s
	}
	if m := euroRe.FindStringSubmatch(c); m != nil {
		s := canvas.Style{"nf": "€"}
		if dp := len(m[2]) - min(len(m[2]), 1); dp != 2 {
			s["dp"] = float64(dp)
		}
		if m[1] == "" {
			s["ng"] = true
		}
		return s
	}
	if k := formatKind(c); k != notDate && plainDf.MatchString(strings.ToLower(c)) {
		df := strings.NewReplacer("yyyy", "aaaa", "yy", "aa", "dddd", "jjjj", "ddd", "jjj", "dd", "jj", `\`, "").Replace(strings.ToLower(c))
		if k == timeOnly {
			return canvas.Style{"nf": "time", "df": df}
		}
		return canvas.Style{"nf": "date", "df": df}
	}
	return canvas.Style{"nc": numfmt.FromExcel(c)}
}

func simpleNum(group bool, dp int) canvas.Style {
	s := canvas.Style{"nf": "0"}
	if dp > 0 {
		s["nf"] = "0,00"
		if dp != 2 {
			s["dp"] = float64(dp)
		}
	}
	if !group {
		s["ng"] = true
	}
	return s
}
