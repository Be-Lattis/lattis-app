package transfer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
)

// Export .xlsx des règles de mise en forme conditionnelle, des validations
// (listes) et des notes. Une règle posée à plusieurs endroits (même
// identifiant) devient une règle d'Excel sur toutes ses plages ; sa mise en
// forme va dans les formats différentiels (dxfs) de styles.xml. Les notes
// deviennent des commentaires (avec leur dessin VML, sans lequel Excel ne
// les affiche pas).

// sheetExtras : ce qu'une feuille exporte en plus de ses cellules.
type sheetExtras struct {
	cf    []*cfOut
	dv    []dvOut
	notes []noteOut
}

type cfOut struct {
	rule  canvas.Rule
	refs  []rect
	table *tablePlace // règle d'une colonne de tableau ([@Colonne])
	lines bool        // lignes ou colonnes entières
	xml   string
}

type dvOut struct {
	refs    []rect
	formula string
}

type noteOut struct {
	r, c   int
	text   string
	pinned bool
}

// collectExtras relève règles, validations et notes d'une feuille (export
// .xlsx) ; le compte rendu ne garde que ce qui ne s'exporte pas.
func collectExtras(sh *xsheet, l canvas.Layout, b *book, fm *fmts) *sheetExtras {
	e := &sheetExtras{}
	byID := map[string]*cfOut{}
	addRules := func(js string, ref rect, tp *tablePlace, lines bool) {
		var rs []canvas.Rule
		if json.Unmarshal([]byte(js), &rs) != nil {
			return
		}
		for _, r := range rs {
			id := r.ID
			if id == "" {
				raw, _ := json.Marshal(r)
				id = string(raw)
			}
			o := byID[id]
			if o == nil {
				o = &cfOut{rule: r, table: tp, lines: lines}
				byID[id] = o
				e.cf = append(e.cf, o)
			}
			o.refs = append(o.refs, ref)
		}
	}
	dvs := map[string]*dvOut{}
	var dvOrder []string
	failedDV := 0
	addDV := func(v any, ref rect) {
		f, ok := dvFormula(v, sh.name, b)
		if !ok {
			failedDV++
			return
		}
		o := dvs[f]
		if o == nil {
			o = &dvOut{formula: f}
			dvs[f] = o
			dvOrder = append(dvOrder, f)
		}
		o.refs = append(o.refs, ref)
	}
	note := func(st canvas.Style, r, c int) {
		if t, _ := st["nt"].(string); t != "" && r < ExcelRows && c < ExcelCols {
			e.notes = append(e.notes, noteOut{r: r, c: c, text: t, pinned: st["np"] == true})
		}
	}
	for _, xt := range sh.tables {
		tb := xt.info
		last := tb.Row + max(tb.Total, 1)
		for j, c := range tb.Columns {
			col := tb.Col + j
			ref := rect{r0: tb.Row + 1, c0: col, r1: min(last, ExcelRows-1), c1: col}
			if c.Rules != "" {
				addRules(c.Rules, ref, xt.place, false)
			}
			st := parseStyle(c.Style)
			if st["dv"] != nil {
				addDV(st["dv"], ref)
			}
			note(parseStyle(c.HeaderStyle), tb.Row, col)
		}
		for k, row := range xt.own {
			for j, st := range row {
				note(st, tb.Row+1+k, tb.Col+j)
				if st["dv"] != nil {
					addDV(st["dv"], rect{tb.Row + 1 + k, tb.Col + j, tb.Row + 1 + k, tb.Col + j})
				}
			}
		}
	}
	for _, lr := range l.RowRules {
		addRules(lr.Rules, rect{r0: lr.Index, c0: 0, r1: lr.Index, c1: ExcelCols - 1}, nil, true)
	}
	for _, lr := range l.ColRules {
		addRules(lr.Rules, rect{r0: 0, c0: lr.Index, r1: ExcelRows - 1, c1: lr.Index}, nil, true)
	}
	for _, s := range l.Styles {
		if s.Row >= ExcelRows || s.Col >= ExcelCols {
			continue
		}
		st := parseStyle(s.Style)
		if cf, ok := st["cf"]; ok {
			raw, _ := json.Marshal(cf)
			addRules(string(raw), rect{s.Row, s.Col, s.Row, s.Col}, nil, false)
		}
		if st["dv"] != nil {
			addDV(st["dv"], rect{s.Row, s.Col, s.Row, s.Col})
		}
		note(st, s.Row, s.Col)
	}
	failed := 0
	prio := 0
	var kept []*cfOut
	for _, o := range e.cf {
		o.refs = mergeRects(o.refs)
		prio++
		if o.xml = cfRuleXML(o, prio, fm); o.xml == "" {
			failed++
			continue
		}
		kept = append(kept, o)
	}
	e.cf = kept
	for _, f := range dvOrder {
		o := dvs[f]
		o.refs = mergeRects(o.refs)
		e.dv = append(e.dv, *o)
	}
	sh.rep.Rules, sh.rep.Lists, sh.rep.Notes = failed, failedDV, 0
	return e
}

// mergeRects réunit les cellules voisines d'une même ligne, puis les
// bandes identiques de lignes successives ; la cellule en haut à gauche
// reste en tête (base des formules relatives).
func mergeRects(rs []rect) []rect {
	slices.SortFunc(rs, func(a, b rect) int {
		if a.r0 != b.r0 {
			return a.r0 - b.r0
		}
		return a.c0 - b.c0
	})
	var out []rect
	for _, r := range rs {
		if n := len(out); n > 0 {
			p := &out[n-1]
			if p.r0 == r.r0 && p.r1 == r.r1 && p.c1+1 == r.c0 {
				p.c1 = r.c1
				continue
			}
		}
		out = append(out, r)
	}
	var fin []rect
	for _, r := range out {
		merged := false
		for i := range fin {
			p := &fin[i]
			if p.c0 == r.c0 && p.c1 == r.c1 && p.r1+1 == r.r0 {
				p.r1 = r.r1
				merged = true
				break
			}
		}
		if !merged {
			fin = append(fin, r)
		}
	}
	return fin
}

// a1 écrit une plage pour une sqref (lignes ou colonnes entières comprises).
func (r rect) a1() string {
	switch {
	case r.r0 == 0 && r.r1 >= ExcelRows-1:
		return colName(r.c0) + ":" + colName(r.c1)
	case r.c0 == 0 && r.c1 >= ExcelCols-1:
		return strconv.Itoa(r.r0+1) + ":" + strconv.Itoa(r.r1+1)
	case r.r0 == r.r1 && r.c0 == r.c1:
		return colName(r.c0) + strconv.Itoa(r.r0+1)
	}
	return colName(r.c0) + strconv.Itoa(r.r0+1) + ":" + colName(r.c1) + strconv.Itoa(r.r1+1)
}

func sqref(rs []rect) string {
	parts := make([]string, len(rs))
	for i, r := range rs {
		parts[i] = r.a1()
	}
	return strings.Join(parts, " ")
}

var colRefRe = regexp.MustCompile(`\[@?([^\[\]]+)\]`)

// ruleFormulaEN traduit la formule d'une règle pour Excel : dans un tableau,
// [@Colonne] devient l'adresse de la colonne sur la première ligne.
func ruleFormulaEN(o *cfOut) (string, bool) {
	src := o.rule.Value
	if o.table != nil {
		tp := o.table
		ok := true
		src = colRefRe.ReplaceAllStringFunc(src, func(m string) string {
			name := colRefRe.FindStringSubmatch(m)[1]
			j := tp.colIndex(name)
			if j < 0 {
				ok = false
				return m
			}
			return "$" + colName(tp.col+j) + strconv.Itoa(tp.row+2)
		})
		if !ok {
			return "", false
		}
	}
	f, ok := translateRule(src, formula.FR, formula.EN)
	return strings.TrimPrefix(f, "="), ok
}

// cfValueEN écrit une valeur de comparaison : nombre (virgule ou point
// décimal) ou texte entre guillemets.
func cfValueEN(v string) string {
	s := strings.TrimSpace(v)
	n := strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), " ", "")
	if !strings.Contains(n, ".") {
		n = strings.Replace(n, ",", ".", 1)
	}
	if _, err := strconv.ParseFloat(n, 64); err == nil {
		return n
	}
	switch strings.ToUpper(s) {
	case "VRAI", "TRUE":
		return "TRUE"
	case "FAUX", "FALSE":
		return "FALSE"
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

var cfOpsEN = map[string]string{">": "greaterThan", ">=": "greaterThanOrEqual", "<": "lessThan", "<=": "lessThanOrEqual",
	"=": "equal", "<>": "notEqual", "between": "between", "notbetween": "notBetween"}

var cfPeriodsEN = map[string][2]string{
	"today":     {"today", "FLOOR(%[1]s,1)=TODAY()"},
	"yesterday": {"yesterday", "FLOOR(%[1]s,1)=TODAY()-1"},
	"tomorrow":  {"tomorrow", "FLOOR(%[1]s,1)=TODAY()+1"},
	"last7":     {"last7Days", "AND(TODAY()-FLOOR(%[1]s,1)<=6,FLOOR(%[1]s,1)<=TODAY())"},
	"thisweek":  {"thisWeek", "AND(TODAY()-ROUNDDOWN(%[1]s,0)<=WEEKDAY(TODAY())-1,ROUNDDOWN(%[1]s,0)-TODAY()<=7-WEEKDAY(TODAY()))"},
	"lastweek":  {"lastWeek", "AND(TODAY()-ROUNDDOWN(%[1]s,0)>=(WEEKDAY(TODAY())),TODAY()-ROUNDDOWN(%[1]s,0)<(WEEKDAY(TODAY())+7))"},
	"nextweek":  {"nextWeek", "AND(ROUNDDOWN(%[1]s,0)-TODAY()>(7-WEEKDAY(TODAY())),ROUNDDOWN(%[1]s,0)-TODAY()<(15-WEEKDAY(TODAY())))"},
	"thismonth": {"thisMonth", "AND(MONTH(%[1]s)=MONTH(TODAY()),YEAR(%[1]s)=YEAR(TODAY()))"},
	"lastmonth": {"lastMonth", "AND(MONTH(%[1]s)=MONTH(EDATE(TODAY(),0-1)),YEAR(%[1]s)=YEAR(EDATE(TODAY(),0-1)))"},
	"nextmonth": {"nextMonth", "AND(MONTH(%[1]s)=MONTH(EDATE(TODAY(),0+1)),YEAR(%[1]s)=YEAR(EDATE(TODAY(),0+1)))"},
}

func cfvoXML(m map[string]any, first bool) string {
	kind, _ := m["kind"].(string)
	t := map[string]string{"min": "min", "max": "max", "num": "num", "pct": "percent", "pctl": "percentile"}[kind]
	if t == "" {
		t = "min"
		if !first {
			t = "max"
		}
	}
	s := `<cfvo type="` + t + `"`
	if v, ok := m["value"].(float64); ok && (t == "num" || t == "percent" || t == "percentile") {
		s += ` val="` + strconv.FormatFloat(v, 'f', -1, 64) + `"`
	}
	if m["gt"] == true {
		s += ` gte="0"`
	}
	return s + `/>`
}

func argbOf(c any) string {
	s, _ := c.(string)
	if h := color(s); h != "" {
		return "FF" + h
	}
	return "FF638EC6"
}

// cfRuleXML écrit une règle (priorité prio) ; "" si elle n'a pas
// d'équivalent.
func cfRuleXML(o *cfOut, prio int, fm *fmts) string {
	r := o.rule
	if len(o.refs) == 0 {
		return ""
	}
	tl := colName(o.refs[0].c0) + strconv.Itoa(o.refs[0].r0+1) // cellule en haut à gauche
	attrs := fmt.Sprintf(` priority="%d"`, prio)
	if r.Stop {
		attrs += ` stopIfTrue="1"`
	}
	dxf := func() string { return fmt.Sprintf(` dxfId="%d"`, fm.dxf(r.Style)) }
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	f := func(s string) string { return "<formula>" + xmlEscape(s) + "</formula>" }
	switch r.Op {
	case ">", ">=", "<", "<=", "=", "<>", "between", "notbetween":
		body := f(cfValueEN(r.Value))
		if r.Op == "between" || r.Op == "notbetween" {
			body += f(cfValueEN(r.To))
		}
		return fmt.Sprintf(`<cfRule type="cellIs"%s%s operator="%s">%s</cfRule>`, dxf(), attrs, cfOpsEN[r.Op], body)
	case "contains":
		return fmt.Sprintf(`<cfRule type="containsText"%s%s operator="containsText" text="%s">%s</cfRule>`, dxf(), attrs, xmlEscape(r.Value), f("NOT(ISERROR(SEARCH("+q(r.Value)+","+tl+")))"))
	case "notcontains":
		return fmt.Sprintf(`<cfRule type="notContainsText"%s%s operator="notContains" text="%s">%s</cfRule>`, dxf(), attrs, xmlEscape(r.Value), f("ISERROR(SEARCH("+q(r.Value)+","+tl+"))"))
	case "begins":
		return fmt.Sprintf(`<cfRule type="beginsWith"%s%s operator="beginsWith" text="%s">%s</cfRule>`, dxf(), attrs, xmlEscape(r.Value), f("LEFT("+tl+",LEN("+q(r.Value)+"))="+q(r.Value)))
	case "ends":
		return fmt.Sprintf(`<cfRule type="endsWith"%s%s operator="endsWith" text="%s">%s</cfRule>`, dxf(), attrs, xmlEscape(r.Value), f("RIGHT("+tl+",LEN("+q(r.Value)+"))="+q(r.Value)))
	case "empty":
		return fmt.Sprintf(`<cfRule type="containsBlanks"%s%s>%s</cfRule>`, dxf(), attrs, f("LEN(TRIM("+tl+"))=0"))
	case "notempty":
		return fmt.Sprintf(`<cfRule type="notContainsBlanks"%s%s>%s</cfRule>`, dxf(), attrs, f("LEN(TRIM("+tl+"))>0"))
	case "error":
		return fmt.Sprintf(`<cfRule type="containsErrors"%s%s>%s</cfRule>`, dxf(), attrs, f("ISERROR("+tl+")"))
	case "noerror":
		return fmt.Sprintf(`<cfRule type="notContainsErrors"%s%s>%s</cfRule>`, dxf(), attrs, f("NOT(ISERROR("+tl+"))"))
	case "date":
		p, ok := cfPeriodsEN[r.Value]
		if !ok {
			return ""
		}
		return fmt.Sprintf(`<cfRule type="timePeriod"%s%s timePeriod="%s">%s</cfRule>`, dxf(), attrs, p[0], f(fmt.Sprintf(p[1], tl)))
	case "dup":
		return fmt.Sprintf(`<cfRule type="duplicateValues"%s%s/>`, dxf(), attrs)
	case "unique":
		return fmt.Sprintf(`<cfRule type="uniqueValues"%s%s/>`, dxf(), attrs)
	case "top", "bottom":
		n, err := strconv.Atoi(strings.TrimSpace(r.Value))
		if err != nil || n < 1 {
			return ""
		}
		extra := ""
		if r.Pct {
			extra += ` percent="1"`
		}
		if r.Op == "bottom" {
			extra += ` bottom="1"`
		}
		return fmt.Sprintf(`<cfRule type="top10"%s%s rank="%d"%s/>`, dxf(), attrs, n, extra)
	case "above", "below":
		extra := ""
		if r.Op == "below" {
			extra += ` aboveAverage="0"`
		}
		switch v := strings.TrimSpace(r.Value); v {
		case "":
		case "=":
			extra += ` equalAverage="1"`
		default:
			if _, err := strconv.Atoi(v); err == nil {
				extra += ` stdDev="` + v + `"`
			}
		}
		return fmt.Sprintf(`<cfRule type="aboveAverage"%s%s%s/>`, dxf(), attrs, extra)
	case "formula":
		src, ok := ruleFormulaEN(o)
		if !ok {
			return ""
		}
		return fmt.Sprintf(`<cfRule type="expression"%s%s>%s</cfRule>`, dxf(), attrs, f(src))
	case "scale":
		if len(r.Scale) < 2 || len(r.Scale) > 3 {
			return ""
		}
		var cfvos, colors strings.Builder
		for i, p := range r.Scale {
			cfvos.WriteString(cfvoXML(p, i == 0))
			fmt.Fprintf(&colors, `<color rgb="%s"/>`, argbOf(p["color"]))
		}
		return fmt.Sprintf(`<cfRule type="colorScale"%s><colorScale>%s%s</colorScale></cfRule>`, attrs, cfvos.String(), colors.String())
	case "bar":
		if r.Bar == nil {
			return ""
		}
		lo, _ := r.Bar["min"].(map[string]any)
		hi, _ := r.Bar["max"].(map[string]any)
		show := ""
		if r.Bar["only"] == true {
			show = ` showValue="0"`
		}
		return fmt.Sprintf(`<cfRule type="dataBar"%s><dataBar%s>%s%s<color rgb="%s"/></dataBar></cfRule>`, attrs, show, cfvoXML(lo, true), cfvoXML(hi, false), argbOf(r.Bar["color"]))
	case "icons":
		if r.Icons == nil {
			return ""
		}
		set, _ := r.Icons["set"].(string)
		name := ""
		for k, v := range cfIcons {
			if v == set {
				name = k
			}
		}
		pts, _ := r.Icons["points"].([]any)
		if name == "" || len(pts) == 0 {
			return ""
		}
		extra := ""
		if r.Icons["reverse"] == true {
			extra += ` reverse="1"`
		}
		if r.Icons["only"] == true {
			extra += ` showValue="0"`
		}
		var cfvos strings.Builder
		cfvos.WriteString(`<cfvo type="percent" val="0"/>`)
		for _, p := range pts {
			m, _ := p.(map[string]any)
			if m == nil {
				return ""
			}
			cfvos.WriteString(cfvoXML(m, false))
		}
		return fmt.Sprintf(`<cfRule type="iconSet"%s><iconSet iconSet="%s"%s>%s</iconSet></cfRule>`, attrs, name, extra, cfvos.String())
	}
	return ""
}

// dvFormula écrit la source d'une liste de validation : la liste entre
// guillemets, ou la plage (colonne de tableau, cellules) en A1.
func dvFormula(v any, sheet string, b *book) (string, bool) {
	switch x := v.(type) {
	case []any:
		var parts []string
		for _, p := range x {
			s := fmt.Sprint(p)
			if strings.ContainsAny(s, `,"`) {
				return "", false
			}
			parts = append(parts, s)
		}
		f := `"` + strings.Join(parts, ",") + `"`
		return f, len(parts) > 0 && len(f) <= 257
	case map[string]any:
		src, ok := canvas.DVSource(x)
		if !ok || b == nil {
			return "", false
		}
		n, err := formula.Parse(src, formula.FR)
		if err != nil {
			return "", false
		}
		cells := func(sh string, r0, c0, r1, c1 int) string {
			s := ""
			if sh != "" && !strings.EqualFold(sh, sheet) {
				s = quoteSheet(sh) + "!"
			}
			s += "$" + colName(c0) + "$" + strconv.Itoa(r0+1)
			if r1 != r0 || c1 != c0 {
				s += ":$" + colName(c1) + "$" + strconv.Itoa(r1+1)
			}
			return s
		}
		switch r := n.(type) {
		case formula.Ref:
			tp := b.tables[strings.ToLower(r.Table)]
			if tp == nil {
				return "", false
			}
			j := tp.colIndex(r.Col)
			if j < 0 {
				return "", false
			}
			return cells(tp.sheet, tp.row+1, tp.col+j, tp.lastRow(), tp.col+j), true
		case formula.CellRange:
			sh := r.Sheet
			if sh != "" {
				sh = b.sheets[strings.ToLower(sh)]
			}
			return cells(sh, r.R1, r.C1, r.R2, r.C2), true
		case formula.Name:
			if p, ok := b.names[strings.ToLower(r.Name)]; ok {
				return cells(p.sheet, p.row, p.col, p.row, p.col), true
			}
		}
	}
	return "", false
}

// extrasXML écrit les règles et les validations (après les fusions).
func extrasXML(e *sheetExtras) string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	for _, o := range e.cf {
		fmt.Fprintf(&b, `<conditionalFormatting sqref="%s">%s</conditionalFormatting>`, sqref(o.refs), o.xml)
	}
	if len(e.dv) > 0 {
		fmt.Fprintf(&b, `<dataValidations count="%d">`, len(e.dv))
		for _, d := range e.dv {
			fmt.Fprintf(&b, `<dataValidation type="list" allowBlank="1" showInputMessage="1" showErrorMessage="1" sqref="%s"><formula1>%s</formula1></dataValidation>`, sqref(d.refs), xmlEscape(d.formula))
		}
		b.WriteString(`</dataValidations>`)
	}
	return b.String()
}

// commentsXML écrit les commentaires d'une feuille et leur dessin VML
// (n : rang de la feuille, pour des identifiants uniques).
func commentsXML(e *sheetExtras, n int) (string, string) {
	if e == nil || len(e.notes) == 0 {
		return "", ""
	}
	var c, v strings.Builder
	c.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><authors><author></author></authors><commentList>`)
	fmt.Fprintf(&v, `<xml xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:x="urn:schemas-microsoft-com:office:excel"><o:shapelayout v:ext="edit"><o:idmap v:ext="edit" data="%d"/></o:shapelayout><v:shapetype id="_x0000_t202" coordsize="21600,21600" o:spt="202" path="m,l,21600r21600,l21600,xe"><v:stroke joinstyle="miter"/><v:path gradientshapeok="t" o:connecttype="rect"/></v:shapetype>`, n)
	for k, nt := range e.notes {
		ref := colName(nt.c) + strconv.Itoa(nt.r+1)
		fmt.Fprintf(&c, `<comment ref="%s" authorId="0"><text><t xml:space="preserve">%s</t></text></comment>`, ref, xmlEscape(nt.text))
		vis, visible := "hidden", ""
		if nt.pinned {
			vis, visible = "visible", "<x:Visible/>"
		}
		fmt.Fprintf(&v, `<v:shape id="_x0000_s%d" type="#_x0000_t202" style="position:absolute;margin-left:59.25pt;margin-top:1.5pt;width:144pt;height:59.25pt;z-index:%d;visibility:%s" fillcolor="#ffffe1" o:insetmode="auto"><v:fill color2="#ffffe1"/><v:shadow on="t" color="black" obscured="t"/><v:path o:connecttype="none"/><v:textbox style="mso-direction-alt:auto"><div style="text-align:left"></div></v:textbox><x:ClientData ObjectType="Note"><x:MoveWithCells/><x:SizeWithCells/><x:Anchor>%d, 15, %d, 2, %d, 15, %d, 16</x:Anchor><x:AutoFill>False</x:AutoFill><x:Row>%d</x:Row><x:Column>%d</x:Column>%s</x:ClientData></v:shape>`,
			n*1024+k+1, k+1, vis, nt.c+1, nt.r, nt.c+3, nt.r+3, nt.r, nt.c, visible)
	}
	c.WriteString(`</commentList></comments>`)
	v.WriteString(`</xml>`)
	return c.String(), v.String()
}

// --- Formats différentiels (dxfs) ------------------------------------------

// dxf enregistre la mise en forme d'une règle ; rend son rang.
func (f *fmts) dxf(st canvas.Style) int {
	s := canvas.Style{}
	for k, v := range st {
		s[k] = v
	}
	for _, k := range [...]string{"fg", "bg", "bc"} {
		if c, ok := s[k].(string); ok && canvas.IsThemeRef(c) {
			s[k] = canvas.ResolveColor(c, f.colors)
		}
	}
	x := toXfmt(s)
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, d := range f.dxfs {
		if d == x {
			return i
		}
	}
	f.dxfs = append(f.dxfs, x)
	return len(f.dxfs) - 1
}

// dxfsXML écrit les formats différentiels (après cellStyles).
func dxfsXML(list []xfmt) string {
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<dxfs count="%d">`, len(list))
	for i, x := range list {
		b.WriteString(`<dxf>`)
		if x.b || x.i || x.u || x.x || x.fg != "" {
			b.WriteString(`<font>`)
			if x.b {
				b.WriteString(`<b/>`)
			}
			if x.i {
				b.WriteString(`<i/>`)
			}
			if x.x {
				b.WriteString(`<strike/>`)
			}
			if x.u {
				b.WriteString(`<u/>`)
			}
			if x.fg != "" {
				fmt.Fprintf(&b, `<color rgb="FF%s"/>`, x.fg)
			}
			b.WriteString(`</font>`)
		}
		if code := x.excelNumFmt(); code != "" {
			fmt.Fprintf(&b, `<numFmt numFmtId="%d" formatCode="%s"/>`, 1000+i, xmlEscape(code))
		}
		if x.bg != "" {
			fmt.Fprintf(&b, `<fill><patternFill patternType="solid"><fgColor rgb="FF%s"/><bgColor rgb="FF%s"/></patternFill></fill>`, x.bg, x.bg)
		}
		if x.bl != "" || x.br != "" || x.bt != "" || x.bb != "" {
			b.WriteString(`<border>`)
			for _, s := range []struct{ tag, e string }{{"left", x.bl}, {"right", x.br}, {"top", x.bt}, {"bottom", x.bb}} {
				if s.e == "" {
					continue
				}
				style, c := edgeParts(s.e)
				col := `<color auto="1"/>`
				if c != "" {
					col = `<color rgb="FF` + c + `"/>`
				}
				fmt.Fprintf(&b, `<%s style="%s">%s</%s>`, s.tag, style, col, s.tag)
			}
			b.WriteString(`</border>`)
		}
		b.WriteString(`</dxf>`)
	}
	b.WriteString(`</dxfs>`)
	return b.String()
}
