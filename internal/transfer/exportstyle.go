package transfer

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/numfmt"
)

// Mise en forme exportée (.xlsx et .ods) : les couches de style de Lattis
// (canvas.Style) sont superposées puis ramenées à un xfmt, comparable ;
// chaque mise en forme distincte reçoit un numéro (0 : aucune), qui est
// aussi son rang dans les styles du fichier.

// xfmt est la mise en forme d'une cellule exportée.
type xfmt struct {
	// nk : format de nombre ; 0 standard, 'n' nombre, 'p' pourcentage,
	// 'c' euros, 'd' date ou heure (motif df), 't' texte, 'x' code
	// personnalisé (code).
	nk    byte
	code  string
	dp    int8 // décimales, -1 : celles du format
	group bool // séparateur de milliers
	df    string
	b, i  bool
	u, x  bool
	font  string
	size  float64
	fg    string // RRGGBB
	bg    string
	al    string // left, center, right
	va    string // top, center, bottom
	wrap  bool
	// Bords : « style couleur » (thin, medium, thick, double, dotted,
	// dashed ; couleur RRGGBB ou vide : automatique), "" sans bord.
	bt, bb, bl, br string
}

// fmts est le registre des mises en forme d'un export (sûr entre goroutines :
// les tableaux sont lus en parallèle).
type fmts struct {
	mu    sync.Mutex
	list  []xfmt
	idx   map[xfmt]int32
	dated map[[2]int32]int32
	// colors : couleurs du thème du classeur (canvas.ThemeColors), qui
	// résolvent les couleurs prises dans le thème (« theme:accent1:+40 ») ;
	// nil : celles du thème par défaut.
	colors map[string]string
}

func newFmts() *fmts {
	f := &fmts{idx: map[xfmt]int32{}, dated: map[[2]int32]int32{}}
	f.id(xfmt{})
	return f
}

func (f *fmts) id(x xfmt) int32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.idLocked(x)
}

func (f *fmts) idLocked(x xfmt) int32 {
	if n, ok := f.idx[x]; ok {
		return n
	}
	n := int32(len(f.list))
	f.list = append(f.list, x)
	f.idx[x] = n
	return n
}

// all rend les mises en forme enregistrées (après l'écriture des cellules).
func (f *fmts) all() []xfmt {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]xfmt(nil), f.list...)
}

// of superpose des couches de style (la dernière l'emporte).
func (f *fmts) of(layers ...canvas.Style) int32 {
	if f == nil {
		return 0
	}
	st := canvas.Style{}
	for _, l := range layers {
		for k, v := range l {
			st[k] = v
		}
	}
	// Couleurs du thème écrites résolues (#rrggbb), comme à l'écran.
	for _, k := range [...]string{"fg", "bg", "bc"} {
		if s, ok := st[k].(string); ok && canvas.IsThemeRef(s) {
			st[k] = canvas.ResolveColor(s, f.colors)
		}
	}
	for _, k := range [...]string{"bt", "bb", "bl", "br"} {
		if s, ok := st[k].(string); ok {
			if style, c, ok := strings.Cut(s, " "); ok && canvas.IsThemeRef(c) {
				st[k] = style + " " + canvas.ResolveColor(c, f.colors)
			}
		}
	}
	return f.id(toXfmt(st))
}

// date donne la mise en forme n avec un format de date, de date et heure
// ou d'heure selon la valeur (numéro de série), sauf si elle en a déjà un.
func (f *fmts) date(n int32, serial float64) int32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := serialKind(serial)
	if d, ok := f.dated[[2]int32{n, t}]; ok {
		return d
	}
	x := f.list[n]
	if x.nk == 0 {
		x.nk = 'd'
		x.df = [3]string{"jj/mm/aaaa", "jj/mm/aaaa hh:mm", "hh:mm"}[t]
	}
	d := f.idLocked(x)
	f.dated[[2]int32{n, t}] = d
	return d
}

// serialKind : 0 date, 1 date et heure, 2 heure seule.
func serialKind(serial float64) int32 {
	switch {
	case serial > 0 && serial < 1:
		return 2
	case serial != math.Floor(serial):
		return 1
	}
	return 0
}

// parseStyle lit un style JSON (vide ou illisible : aucun).
func parseStyle(s string) canvas.Style {
	st := canvas.Style{}
	if s != "" {
		_ = json.Unmarshal([]byte(s), &st)
	}
	return st
}

var colorRe = regexp.MustCompile(`^#?([0-9a-fA-F]{6})$`)

func color(v any) string {
	s, _ := v.(string)
	if m := colorRe.FindStringSubmatch(s); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}

func toXfmt(st canvas.Style) xfmt {
	flag := func(k string) bool { b, _ := st[k].(bool); return b }
	str := func(k string) string { s, _ := st[k].(string); return s }
	x := xfmt{dp: -1, b: flag("b"), i: flag("i"), u: flag("u"), x: flag("x"), wrap: flag("wr"),
		font: str("ff"), fg: color(st["fg"]), bg: color(st["bg"])}
	x.bt, x.bb, x.bl, x.br = edge(st["bt"], st["bc"]), edge(st["bb"], st["bc"]), edge(st["bl"], st["bc"]), edge(st["br"], st["bc"])
	if f, ok := st["fs"].(float64); ok && f > 0 && f < 410 {
		x.size = f
	}
	switch str("al") {
	case "left", "center", "right":
		x.al = str("al")
	}
	switch str("va") {
	case "top", "bottom":
		x.va = str("va")
	case "middle":
		x.va = "center"
	}
	if f, ok := st["dp"].(float64); ok && f >= 0 && f <= 15 {
		x.dp = int8(f)
	}
	x.group = !flag("ng")
	switch str("nf") {
	case "0":
		x.nk = 'n'
		if x.dp < 0 {
			x.dp = 0
		}
	case "0,00":
		x.nk = 'n'
		if x.dp < 0 {
			x.dp = 2
		}
	case "%":
		x.nk, x.group = 'p', false
		if x.dp < 0 {
			x.dp = 2
		}
	case "€":
		x.nk = 'c'
		if x.dp < 0 {
			x.dp = 2
		}
	case "date":
		x.nk, x.dp, x.df = 'd', -1, str("df")
		if x.df == "" {
			x.df = "jj/mm/aaaa"
		}
	case "time":
		x.nk, x.dp, x.df = 'd', -1, str("df")
		if x.df == "" {
			x.df = "hh:mm"
		}
	case "text":
		x.nk, x.dp = 't', -1
	default:
		if x.dp >= 0 {
			x.nk = 'n'
		}
	}
	if c := strings.TrimSpace(str("nc")); c != "" {
		x.nk, x.code, x.dp, x.df, x.group = 'x', c, -1, "", false
	}
	x.group = x.group && (x.nk == 'n' || x.nk == 'c')
	if x.nk != 'n' && x.nk != 'p' && x.nk != 'c' {
		x.dp = 0 // sans objet : la mise en forme vide reste xfmt{}
	}
	return x
}

var edgeStyles = map[string]bool{"thin": true, "medium": true, "thick": true, "double": true, "dotted": true, "dashed": true}

// edge lit un bord : true (trait fin, couleur bc), ou « style couleur ».
func edge(v, bc any) string {
	switch v := v.(type) {
	case bool:
		if v {
			return strings.TrimSpace("thin " + color(bc))
		}
	case string:
		style, c, _ := strings.Cut(strings.TrimSpace(v), " ")
		if !edgeStyles[style] {
			style = "thin"
		}
		if c = color(c); c == "" {
			c = color(bc)
		}
		return strings.TrimSpace(style + " " + c)
	}
	return ""
}

// edgeParts sépare le style et la couleur (RRGGBB, "" : automatique) d'un bord.
func edgeParts(e string) (style, c string) {
	style, c, _ = strings.Cut(e, " ")
	return style, c
}

// --- Excel ------------------------------------------------------------------

// numCode donne le code de format Excel d'un nombre.
func numCode(dp int8, group bool) string {
	s := "0"
	if group {
		s = "#,##0"
	}
	if dp > 0 {
		s += "." + strings.Repeat("0", int(dp))
	}
	return s
}

// excelNumFmt donne le code de format Excel ("" : standard).
func (x xfmt) excelNumFmt() string {
	switch x.nk {
	case 'n':
		return numCode(x.dp, x.group)
	case 'p':
		return numCode(x.dp, false) + "%"
	case 'c':
		return numCode(x.dp, x.group) + `\ "€"`
	case 't':
		return "@"
	case 'd':
		return excelDate(x.df)
	case 'x':
		return numfmt.ToExcel(x.code)
	}
	return ""
}

var dateTok = regexp.MustCompile(`aaaa|aa|mmmm|mmm|mm|jjjj|jjj|jj|hh|ss|[^amjhs]+|.`)

// excelDate traduit un motif de date de Lattis (jj/mm/aaaa) en code Excel,
// en français quand il nomme les mois ou les jours.
func excelDate(p string) string {
	var b strings.Builder
	named := false
	for _, t := range dateTok.FindAllString(p, -1) {
		switch t {
		case "aaaa":
			b.WriteString("yyyy")
		case "aa":
			b.WriteString("yy")
		case "mmmm", "mmm":
			named = true
			b.WriteString(t)
		case "mm", "hh", "ss":
			b.WriteString(t)
		case "jjjj":
			named = true
			b.WriteString("dddd")
		case "jjj":
			named = true
			b.WriteString("ddd")
		case "jj":
			b.WriteString("dd")
		default:
			for _, r := range t {
				if strings.ContainsRune("/-:., ", r) {
					if r == ' ' {
						b.WriteString(`\ `)
					} else {
						b.WriteRune(r)
					}
				} else {
					b.WriteString(`\`)
					b.WriteRune(r)
				}
			}
		}
	}
	if named {
		return "[$-40C]" + b.String()
	}
	return b.String()
}

func (x xfmt) fontKey() [7]string {
	return [7]string{strconv.FormatBool(x.b), strconv.FormatBool(x.i), strconv.FormatBool(x.u), strconv.FormatBool(x.x), x.font, strconv.FormatFloat(x.size, 'f', -1, 64), x.fg}
}

// xlsxStyles écrit xl/styles.xml : une entrée cellXfs par mise en forme, au
// même rang que dans le registre.
func xlsxStyles(list []xfmt) string {
	var numFmts, fonts, fills, borders, xfs strings.Builder
	numIDs := map[string]int{}
	fontIDs := map[[7]string]int{}
	fillIDs := map[string]int{}
	borderIDs := map[[4]string]int{}
	fonts.WriteString(`<font><sz val="11"/><name val="Calibri"/></font>`)
	fontIDs[xfmt{}.fontKey()] = 0
	fills.WriteString(`<fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill>`)
	borders.WriteString(`<border><left/><right/><top/><bottom/><diagonal/></border>`)
	borderIDs[[4]string{}] = 0
	nFonts, nFills, nBorders := 1, 2, 1
	for _, x := range list {
		num := 0
		if code := x.excelNumFmt(); code != "" {
			if id, ok := numIDs[code]; ok {
				num = id
			} else {
				num = 164 + len(numIDs)
				numIDs[code] = num
				fmt.Fprintf(&numFmts, `<numFmt numFmtId="%d" formatCode="%s"/>`, num, xmlEscape(code))
			}
		}
		fk := x.fontKey()
		font, ok := fontIDs[fk]
		if !ok {
			font = nFonts
			nFonts++
			fontIDs[fk] = font
			fonts.WriteString(`<font>`)
			if x.b {
				fonts.WriteString(`<b/>`)
			}
			if x.i {
				fonts.WriteString(`<i/>`)
			}
			if x.x {
				fonts.WriteString(`<strike/>`)
			}
			if x.u {
				fonts.WriteString(`<u/>`)
			}
			size := 11.0
			if x.size > 0 {
				size = x.size
			}
			fmt.Fprintf(&fonts, `<sz val="%s"/>`, strconv.FormatFloat(size, 'f', -1, 64))
			if x.fg != "" {
				fmt.Fprintf(&fonts, `<color rgb="FF%s"/>`, x.fg)
			}
			name := "Calibri"
			if x.font != "" {
				name = x.font
			}
			fmt.Fprintf(&fonts, `<name val="%s"/></font>`, xmlEscape(name))
		}
		fill := 0
		if x.bg != "" {
			if id, ok := fillIDs[x.bg]; ok {
				fill = id
			} else {
				fill = nFills
				nFills++
				fillIDs[x.bg] = fill
				fmt.Fprintf(&fills, `<fill><patternFill patternType="solid"><fgColor rgb="FF%s"/><bgColor indexed="64"/></patternFill></fill>`, x.bg)
			}
		}
		bk := [4]string{x.bl, x.br, x.bt, x.bb}
		border, ok := borderIDs[bk]
		if !ok {
			border = nBorders
			nBorders++
			borderIDs[bk] = border
			side := func(tag string, e string) {
				if e == "" {
					fmt.Fprintf(&borders, `<%s/>`, tag)
					return
				}
				style, bc := edgeParts(e)
				c := `<color auto="1"/>`
				if bc != "" {
					c = `<color rgb="FF` + bc + `"/>`
				}
				fmt.Fprintf(&borders, `<%s style="%s">%s</%s>`, tag, style, c, tag)
			}
			borders.WriteString(`<border>`)
			side("left", x.bl)
			side("right", x.br)
			side("top", x.bt)
			side("bottom", x.bb)
			borders.WriteString(`<diagonal/></border>`)
		}
		fmt.Fprintf(&xfs, `<xf numFmtId="%d" fontId="%d" fillId="%d" borderId="%d" xfId="0"`, num, font, fill, border)
		if num != 0 {
			xfs.WriteString(` applyNumberFormat="1"`)
		}
		if font != 0 {
			xfs.WriteString(` applyFont="1"`)
		}
		if fill != 0 {
			xfs.WriteString(` applyFill="1"`)
		}
		if border != 0 {
			xfs.WriteString(` applyBorder="1"`)
		}
		if x.al != "" || x.va != "" || x.wrap {
			xfs.WriteString(` applyAlignment="1"><alignment`)
			if x.al != "" {
				fmt.Fprintf(&xfs, ` horizontal="%s"`, x.al)
			}
			if x.va != "" {
				fmt.Fprintf(&xfs, ` vertical="%s"`, x.va)
			}
			if x.wrap {
				xfs.WriteString(` wrapText="1"`)
			}
			xfs.WriteString(`/></xf>`)
		} else {
			xfs.WriteString(`/>`)
		}
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	if len(numIDs) > 0 {
		fmt.Fprintf(&b, `<numFmts count="%d">%s</numFmts>`, len(numIDs), numFmts.String())
	}
	fmt.Fprintf(&b, `<fonts count="%d">%s</fonts><fills count="%d">%s</fills><borders count="%d">%s</borders>`, nFonts, fonts.String(), nFills, fills.String(), nBorders, borders.String())
	b.WriteString(`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>`)
	fmt.Fprintf(&b, `<cellXfs count="%d">%s</cellXfs>`, len(list), xfs.String())
	b.WriteString(`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`)
	return b.String()
}

// --- OpenDocument -----------------------------------------------------------

// odsNumber écrit l'élément number:number d'un format.
func odsNumber(dp int8, group bool) string {
	d := max(dp, 0)
	return fmt.Sprintf(`<number:number number:decimal-places="%d" number:min-decimal-places="%d" number:min-integer-digits="1" number:grouping="%t"/>`, d, d, group)
}

// odsDataStyle écrit le style de données (format de nombre) de la mise en
// forme, nommé name ; "" sans format.
func (x xfmt) odsDataStyle(name string) string {
	switch x.nk {
	case 'n':
		return fmt.Sprintf(`<number:number-style style:name="%s">%s</number:number-style>`, name, odsNumber(x.dp, x.group))
	case 'p':
		return fmt.Sprintf(`<number:percentage-style style:name="%s">%s<number:text>%%</number:text></number:percentage-style>`, name, odsNumber(x.dp, false))
	case 'c':
		return fmt.Sprintf(`<number:currency-style style:name="%s" number:language="fr" number:country="FR">%s<number:text> </number:text><number:currency-symbol number:language="fr" number:country="FR">€</number:currency-symbol></number:currency-style>`, name, odsNumber(x.dp, x.group))
	case 't':
		return fmt.Sprintf(`<number:text-style style:name="%s"><number:text-content/></number:text-style>`, name)
	case 'd':
		return odsDate(name, x.df)
	case 'x':
		return odsCode(name, x.code)
	}
	return ""
}

// odsCode traduit un code personnalisé (sa première section) en style de
// données OpenDocument, au plus près : date ou heure, scientifique,
// pourcentage, nombre (décimales, milliers) ; "" pour un code de texte.
func odsCode(name, code string) string {
	f := numfmt.Parse(code)
	if f.IsText() {
		return fmt.Sprintf(`<number:text-style style:name="%s"><number:text-content/></number:text-style>`, name)
	}
	first, _, _ := strings.Cut(numfmt.ToExcel(code), ";")
	if f.IsDate() {
		pat := strings.NewReplacer("yyyy", "aaaa", "yy", "aa", "dddd", "jjjj", "ddd", "jjj", "dd", "jj", `\`, "", `"`, "").Replace(strings.ToLower(first))
		return odsDate(name, pat)
	}
	core := regexp.MustCompile(`"[^"]*"|\\.|\[[^\]]*\]|[_*].`).ReplaceAllString(first, "")
	_, frac, _ := strings.Cut(core, ".")
	dp := int8(0)
	for _, r := range frac {
		if r == '0' || r == '#' || r == '?' {
			dp++
		} else {
			break
		}
	}
	group := strings.Contains(core, "#,#") || strings.Contains(core, "0,0")
	switch {
	case strings.ContainsAny(core, "Ee") && strings.ContainsAny(core, "+-"):
		return fmt.Sprintf(`<number:number-style style:name="%s"><number:scientific-number number:decimal-places="%d" number:min-integer-digits="1" number:min-exponent-digits="2"/></number:number-style>`, name, dp)
	case strings.Contains(core, "%"):
		return fmt.Sprintf(`<number:percentage-style style:name="%s">%s<number:text>%%</number:text></number:percentage-style>`, name, odsNumber(dp, false))
	case strings.ContainsAny(core, "0#?"):
		return fmt.Sprintf(`<number:number-style style:name="%s">%s</number:number-style>`, name, odsNumber(dp, group))
	}
	return ""
}

// odsDate traduit un motif de date de Lattis en style de date OpenDocument.
func odsDate(name, p string) string {
	var b strings.Builder
	toks := dateTok.FindAllString(p, -1)
	field := func(t string) bool {
		switch t {
		case "aaaa", "aa", "mmmm", "mmm", "mm", "jjjj", "jjj", "jj", "hh", "ss":
			return true
		}
		return false
	}
	near := func(k, dir int) string {
		for j := k + dir; j >= 0 && j < len(toks); j += dir {
			if field(toks[j]) {
				return toks[j]
			}
		}
		return ""
	}
	for k, t := range toks {
		switch t {
		case "aaaa":
			b.WriteString(`<number:year number:style="long"/>`)
		case "aa":
			b.WriteString(`<number:year/>`)
		case "mmmm":
			b.WriteString(`<number:month number:style="long" number:textual="true"/>`)
		case "mmm":
			b.WriteString(`<number:month number:textual="true"/>`)
		case "mm":
			if near(k, -1) == "hh" || near(k, 1) == "ss" {
				b.WriteString(`<number:minutes number:style="long"/>`)
			} else {
				b.WriteString(`<number:month number:style="long"/>`)
			}
		case "jjjj":
			b.WriteString(`<number:day-of-week number:style="long"/>`)
		case "jjj":
			b.WriteString(`<number:day-of-week/>`)
		case "jj":
			b.WriteString(`<number:day number:style="long"/>`)
		case "hh":
			b.WriteString(`<number:hours number:style="long"/>`)
		case "ss":
			b.WriteString(`<number:seconds number:style="long"/>`)
		default:
			fmt.Fprintf(&b, `<number:text>%s</number:text>`, xmlEscape(t))
		}
	}
	return fmt.Sprintf(`<number:date-style style:name="%s" number:language="fr" number:country="FR">%s</number:date-style>`, name, b.String())
}

// odsLine écrit un bord OpenDocument (épaisseur, trait, couleur).
func odsLine(e string) string {
	style, c := edgeParts(e)
	if c == "" {
		c = "000000"
	}
	w, kind := "0.75pt", "solid"
	switch style {
	case "medium":
		w = "1.5pt"
	case "thick":
		w = "2.5pt"
	case "double":
		w, kind = "2.25pt", "double"
	case "dotted", "dashed":
		kind = style
	}
	return w + " " + kind + " #" + strings.ToLower(c)
}

// odsCellStyle écrit le style de cellule « ce<n> » (et son style de données).
func (x xfmt) odsCellStyle(n int) string {
	var b strings.Builder
	data := x.odsDataStyle(fmt.Sprintf("N%d", n))
	b.WriteString(data)
	fmt.Fprintf(&b, `<style:style style:name="ce%d" style:family="table-cell" style:parent-style-name="Default"`, n)
	if data != "" {
		fmt.Fprintf(&b, ` style:data-style-name="N%d"`, n)
	}
	b.WriteString(`>`)
	var cell strings.Builder
	if x.bg != "" {
		fmt.Fprintf(&cell, ` fo:background-color="#%s"`, strings.ToLower(x.bg))
	}
	if x.wrap {
		cell.WriteString(` fo:wrap-option="wrap"`)
	}
	switch x.va {
	case "top":
		cell.WriteString(` style:vertical-align="top"`)
	case "center":
		cell.WriteString(` style:vertical-align="middle"`)
	case "bottom":
		cell.WriteString(` style:vertical-align="bottom"`)
	}
	for _, s := range []struct {
		edge string
		side string
	}{{x.bt, "top"}, {x.bb, "bottom"}, {x.bl, "left"}, {x.br, "right"}} {
		if s.edge != "" {
			fmt.Fprintf(&cell, ` fo:border-%s="%s"`, s.side, odsLine(s.edge))
		}
	}
	if x.al != "" {
		cell.WriteString(` style:text-align-source="fix"`)
	}
	if cell.Len() > 0 {
		fmt.Fprintf(&b, `<style:table-cell-properties%s/>`, cell.String())
	}
	if x.al != "" {
		al := map[string]string{"left": "start", "center": "center", "right": "end"}[x.al]
		fmt.Fprintf(&b, `<style:paragraph-properties fo:text-align="%s"/>`, al)
	}
	var text strings.Builder
	if x.b {
		text.WriteString(` fo:font-weight="bold" style:font-weight-asian="bold" style:font-weight-complex="bold"`)
	}
	if x.i {
		text.WriteString(` fo:font-style="italic" style:font-style-asian="italic" style:font-style-complex="italic"`)
	}
	if x.u {
		text.WriteString(` style:text-underline-style="solid" style:text-underline-width="auto" style:text-underline-color="font-color"`)
	}
	if x.x {
		text.WriteString(` style:text-line-through-style="solid"`)
	}
	if x.fg != "" {
		fmt.Fprintf(&text, ` fo:color="#%s"`, strings.ToLower(x.fg))
	}
	if x.size > 0 {
		fmt.Fprintf(&text, ` fo:font-size="%spt"`, strconv.FormatFloat(x.size, 'f', -1, 64))
	}
	if x.font != "" {
		fmt.Fprintf(&text, ` fo:font-family="%s"`, xmlEscape(x.font))
	}
	if text.Len() > 0 {
		fmt.Fprintf(&b, `<style:text-properties%s/>`, text.String())
	}
	b.WriteString(`</style:style>`)
	return b.String()
}
