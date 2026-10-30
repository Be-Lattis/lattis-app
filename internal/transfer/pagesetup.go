package transfer

import (
	"math"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Mise en page (onglet Mise en page) dans les fichiers .xlsx et .ods :
// écriture et, pour .xlsx, lecture à l'import.

// Codes paperSize d'Excel.
var xlsxPaper = map[string]int{"Letter": 1, "Tabloid": 3, "Legal": 5, "Executive": 7, "A3": 8, "A4": 9, "A5": 11, "B5": 34}

// hfCodes : codes des en-têtes et pieds de page, Lattis ↔ Excel.
var hfCodes = [][2]string{
	{"&[Page]", "&P"}, {"&[Pages]", "&N"}, {"&[Date]", "&D"}, {"&[Heure]", "&T"},
	{"&[Fichier]", "&F"}, {"&[Chemin]", "&Z"}, {"&[Onglet]", "&A"}, {"&[Feuille]", "&A"},
}

// xlsxHFPart écrit une partie d'en-tête au format d'Excel (& seul : &&).
func xlsxHFPart(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "&&") {
			b.WriteString("&&")
			i += 2
			continue
		}
		done := false
		for _, c := range hfCodes {
			if strings.HasPrefix(s[i:], c[0]) {
				b.WriteString(c[1])
				i += len(c[0])
				done = true
				break
			}
		}
		if done {
			continue
		}
		if s[i] == '&' {
			b.WriteString("&&")
		} else {
			b.WriteByte(s[i])
		}
		i++
	}
	return b.String()
}

// xlsxHF écrit un en-tête complet : &L…&C…&R….
func xlsxHF(h canvas.HeaderFooter) string {
	out := ""
	for _, p := range [][2]string{{"&L", h.Left}, {"&C", h.Center}, {"&R", h.Right}} {
		if p[1] != "" {
			out += p[0] + xlsxHFPart(p[1])
		}
	}
	return out
}

// parseXLSXHF lit un en-tête d'Excel ; les codes de mise en forme (police,
// taille, gras…) sont ignorés.
func parseXLSXHF(s string) canvas.HeaderFooter {
	var h canvas.HeaderFooter
	part := &h.Center // sans section : centré
	var b strings.Builder
	flush := func() { *part += b.String(); b.Reset() }
	for i := 0; i < len(s); i++ {
		if s[i] != '&' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch c := s[i]; c {
		case 'L', 'C', 'R':
			flush()
			part = map[byte]*string{'L': &h.Left, 'C': &h.Center, 'R': &h.Right}[c]
		case '&':
			// « && » : une perluète ; gardée double devant « [ » (pas un code).
			if i+1 < len(s) && s[i+1] == '[' {
				b.WriteString("&&")
			} else {
				b.WriteByte('&')
			}
		case 'P':
			b.WriteString("&[Page]")
		case 'N':
			b.WriteString("&[Pages]")
		case 'D':
			b.WriteString("&[Date]")
		case 'T':
			b.WriteString("&[Heure]")
		case 'F':
			b.WriteString("&[Fichier]")
		case 'Z':
			b.WriteString("&[Chemin]")
		case 'A':
			b.WriteString("&[Onglet]")
		case 'K':
			// &Krrggbb (couleur) : ignoré.
			i += min(6, len(s)-i-1)
		case '"':
			// &"Police,Style" : ignoré.
			if j := strings.IndexByte(s[i+1:], '"'); j >= 0 {
				i += j + 1
			}
		default:
			// &12 (taille), &B, &I, &U, &S, &X, &Y, &K… : ignorés.
			for i+1 < len(s) && s[i] >= '0' && s[i] <= '9' && s[i+1] >= '0' && s[i+1] <= '9' {
				i++
			}
		}
	}
	flush()
	return h
}

func inches(cm float64) string { return strconv.FormatFloat(round4(cm/2.54), 'f', -1, 64) }

func round4(v float64) float64 { return float64(int64(v*10000+0.5)) / 10000 }

// xlsxSheetPr : <pageSetUpPr> de sheetPr (ajustement en pages).
func xlsxPageSetUpPr(p canvas.PageSetup) string {
	if p.Fit() {
		return `<pageSetUpPr fitToPage="1"/>`
	}
	return ""
}

// xlsxPageXML écrit printOptions, pageMargins, pageSetup, headerFooter,
// rowBreaks et colBreaks (dans cet ordre, celui du schéma).
func xlsxPageXML(p canvas.PageSetup) string {
	if p.IsZero() {
		return ""
	}
	var b strings.Builder
	var opts string
	if p.CenterH {
		opts += ` horizontalCentered="1"`
	}
	if p.CenterV {
		opts += ` verticalCentered="1"`
	}
	if p.Headings {
		opts += ` headings="1"`
	}
	if p.Gridlines {
		opts += ` gridLines="1"`
	}
	if opts != "" {
		fmt.Fprintf(&b, `<printOptions%s/>`, opts)
	}
	m := p.PageMargins()
	fmt.Fprintf(&b, `<pageMargins left="%s" right="%s" top="%s" bottom="%s" header="%s" footer="%s"/>`, inches(m.Left), inches(m.Right), inches(m.Top), inches(m.Bottom), inches(m.Header), inches(m.Footer))
	paper := p.Paper
	if paper == "" {
		paper = "A4"
	}
	fmt.Fprintf(&b, `<pageSetup paperSize="%d"`, xlsxPaper[paper])
	if p.Scale > 0 {
		fmt.Fprintf(&b, ` scale="%d"`, p.Scale)
	}
	if p.FirstPage > 0 {
		fmt.Fprintf(&b, ` firstPageNumber="%d"`, p.FirstPage)
	}
	if p.Fit() {
		fmt.Fprintf(&b, ` fitToWidth="%d" fitToHeight="%d"`, p.FitWide, p.FitTall)
	}
	if p.AcrossFirst {
		b.WriteString(` pageOrder="overThenDown"`)
	}
	if p.Landscape {
		b.WriteString(` orientation="landscape"`)
	} else {
		b.WriteString(` orientation="portrait"`)
	}
	if p.BlackWhite {
		b.WriteString(` blackAndWhite="1"`)
	}
	if p.FirstPage > 0 {
		b.WriteString(` useFirstPageNumber="1"`)
	}
	b.WriteString(`/>`)
	if !p.Header.Empty() || !p.Footer.Empty() {
		b.WriteString(`<headerFooter>`)
		if !p.Header.Empty() {
			fmt.Fprintf(&b, `<oddHeader>%s</oddHeader>`, xmlEscape(xlsxHF(p.Header)))
		}
		if !p.Footer.Empty() {
			fmt.Fprintf(&b, `<oddFooter>%s</oddFooter>`, xmlEscape(xlsxHF(p.Footer)))
		}
		b.WriteString(`</headerFooter>`)
	}
	for _, br := range []struct {
		tag string
		at  []int
		max int
		lim int
	}{{"rowBreaks", p.RowBreaks, ExcelCols - 1, ExcelRows}, {"colBreaks", p.ColBreaks, ExcelRows - 1, ExcelCols}} {
		at := slices.DeleteFunc(slices.Clone(br.at), func(i int) bool { return i >= br.lim })
		if len(at) == 0 {
			continue
		}
		fmt.Fprintf(&b, `<%s count="%d" manualBreakCount="%d">`, br.tag, len(at), len(at))
		for _, i := range at {
			fmt.Fprintf(&b, `<brk id="%d" max="%d" man="1"/>`, i, br.max)
		}
		fmt.Fprintf(&b, `</%s>`, br.tag)
	}
	return b.String()
}

// quoteSheet écrit un nom de feuille dans une formule : 'Feuille 1'.
func quoteSheet(name string) string { return "'" + strings.ReplaceAll(name, "'", "''") + "'" }

// xlsxDefinedNames écrit _xlnm.Print_Area et _xlnm.Print_Titles des feuilles.
func xlsxDefinedNames(sheets []xsheet) string {
	var b strings.Builder
	for i, s := range sheets {
		p, q := s.page, quoteSheet(s.name)
		if a := p.Area; a != nil && a.R1 < ExcelRows && a.C1 < ExcelCols {
			fmt.Fprintf(&b, `<definedName name="_xlnm.Print_Area" localSheetId="%d">%s!$%s$%d:$%s$%d</definedName>`, i, xmlEscape(q), colName(a.C0), a.R0+1, colName(a.C1), a.R1+1)
		}
		var t []string
		if c := p.TitleCols; c != nil && c.To < ExcelCols {
			t = append(t, fmt.Sprintf("%s!$%s:$%s", q, colName(c.From), colName(c.To)))
		}
		if r := p.TitleRows; r != nil && r.To < ExcelRows {
			t = append(t, fmt.Sprintf("%s!$%d:$%d", q, r.From+1, r.To+1))
		}
		if len(t) > 0 {
			fmt.Fprintf(&b, `<definedName name="_xlnm.Print_Titles" localSheetId="%d">%s</definedName>`, i, xmlEscape(strings.Join(t, ",")))
		}
	}
	if len(sheets) > 0 && sheets[0].book != nil {
		b.WriteString(sheets[0].book.definedNamesXML())
	}
	if b.Len() == 0 {
		return ""
	}
	return "<definedNames>" + b.String() + "</definedNames>"
}

// parsePrintNames lit la valeur d'un nom _xlnm.Print_Area ou
// _xlnm.Print_Titles (« 'Feuille 1'!$A$1:$F$40 », « Feuil1!$1:$2,Feuil1!$A:$A »).
func parsePrintNames(p *canvas.PageSetup, name, value string) {
	for _, part := range strings.Split(value, ",") {
		if i := strings.LastIndexByte(part, '!'); i >= 0 {
			part = part[i+1:]
		}
		part = strings.TrimSpace(part)
		switch {
		case strings.EqualFold(name, "_xlnm.Print_Area"):
			if r, err := canvas.ParseA1Range(part); err == nil && p.Area == nil {
				p.Area = &r
			}
		case strings.EqualFold(name, "_xlnm.Print_Titles"):
			if s, err := canvas.ParseLineSpan(part, true); err == nil {
				p.TitleRows = &s
			} else if s, err := canvas.ParseLineSpan(part, false); err == nil {
				p.TitleCols = &s
			}
		}
	}
}

// --- OpenDocument ------------------------------------------------------------

// odsPrintAttr : ce que style:print imprime (quadrillage, en-têtes).
func odsPageLayout(name string, p canvas.PageSetup) string {
	var b strings.Builder
	w, h := p.PaperMM()
	m := p.PageMargins()
	cm := func(v float64) string { return strconv.FormatFloat(round4(v), 'f', -1, 64) + "cm" }
	orient := "portrait"
	if p.Landscape {
		orient = "landscape"
	}
	top, bottom := m.Top, m.Bottom
	if !p.Header.Empty() {
		top = m.Header
	}
	if !p.Footer.Empty() {
		bottom = m.Footer
	}
	print := "objects charts drawings zero-values"
	if p.Gridlines {
		print = "grid " + print
	}
	if p.Headings {
		print = "headers " + print
	}
	order := "ttb"
	if p.AcrossFirst {
		order = "ltr"
	}
	center := "none"
	switch {
	case p.CenterH && p.CenterV:
		center = "both"
	case p.CenterH:
		center = "horizontal"
	case p.CenterV:
		center = "vertical"
	}
	fmt.Fprintf(&b, `<style:page-layout style:name="%s"><style:page-layout-properties fo:page-width="%s" fo:page-height="%s" style:print-orientation="%s" fo:margin-top="%s" fo:margin-bottom="%s" fo:margin-left="%s" fo:margin-right="%s" style:print="%s" style:print-page-order="%s" style:table-centering="%s"`,
		name, cm(w/10), cm(h/10), orient, cm(top), cm(bottom), cm(m.Left), cm(m.Right), print, order, center)
	if p.FirstPage > 0 {
		fmt.Fprintf(&b, ` style:first-page-number="%d"`, p.FirstPage)
	}
	switch {
	case p.Fit():
		if p.FitWide > 0 {
			fmt.Fprintf(&b, ` style:scale-to-X="%d"`, p.FitWide)
		}
		if p.FitTall > 0 {
			fmt.Fprintf(&b, ` style:scale-to-Y="%d"`, p.FitTall)
		}
	case p.Scale > 0:
		fmt.Fprintf(&b, ` style:scale-to="%d%%"`, p.Scale)
	}
	b.WriteString(`/>`)
	if !p.Header.Empty() {
		fmt.Fprintf(&b, `<style:header-style><style:header-footer-properties fo:min-height="%s" fo:margin-bottom="0cm"/></style:header-style>`, cm(max(m.Top-m.Header, 0.1)))
	}
	if !p.Footer.Empty() {
		fmt.Fprintf(&b, `<style:footer-style><style:header-footer-properties fo:min-height="%s" fo:margin-top="0cm"/></style:footer-style>`, cm(max(m.Bottom-m.Footer, 0.1)))
	}
	b.WriteString(`</style:page-layout>`)
	return b.String()
}

// odsMasterPage écrit la page maître d'une feuille (en-tête, pied de page).
func odsMasterPage(name, layout string, p canvas.PageSetup) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<style:master-page style:name="%s" style:page-layout-name="%s">`, name, layout)
	for _, hf := range []struct {
		tag string
		h   canvas.HeaderFooter
	}{{"header", p.Header}, {"footer", p.Footer}} {
		if hf.h.Empty() {
			fmt.Fprintf(&b, `<style:%s style:display="false"/>`, hf.tag)
			continue
		}
		fmt.Fprintf(&b, `<style:%s>`, hf.tag)
		for _, r := range [][2]string{{"left", hf.h.Left}, {"center", hf.h.Center}, {"right", hf.h.Right}} {
			fmt.Fprintf(&b, `<style:region-%s><text:p>%s</text:p></style:region-%s>`, r[0], odsHFText(r[1]), r[0])
		}
		fmt.Fprintf(&b, `</style:%s>`, hf.tag)
	}
	b.WriteString(`</style:master-page>`)
	return b.String()
}

// odsHFText écrit une partie d'en-tête en texte ODF (champs compris).
func odsHFText(s string) string {
	fields := [][2]string{
		{"&[Page]", `<text:page-number>1</text:page-number>`},
		{"&[Pages]", `<text:page-count>1</text:page-count>`},
		{"&[Date]", `<text:date/>`},
		{"&[Heure]", `<text:time/>`},
		{"&[Fichier]", `<text:file-name text:display="name-and-extension"/>`},
		{"&[Chemin]", `<text:file-name text:display="path"/>`},
		{"&[Onglet]", `<text:sheet-name/>`},
		{"&[Feuille]", `<text:sheet-name/>`},
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "&&") {
			b.WriteString("&amp;")
			i += 2
			continue
		}
		done := false
		for _, f := range fields {
			if strings.HasPrefix(s[i:], f[0]) {
				b.WriteString(f[1])
				i += len(f[0])
				done = true
				break
			}
		}
		if done {
			continue
		}
		// Un caractère (UTF-8 compris) à la fois, échappé.
		j := i + 1
		for j < len(s) && s[j]&0xC0 == 0x80 {
			j++
		}
		b.WriteString(xmlEscape(s[i:j]))
		i = j
	}
	return b.String()
}

// pageReader lit la mise en page d'une feuille .xlsx, balise par balise.
type pageReader struct {
	fit, rowBreaks bool
}

func (pr *pageReader) tag(p *canvas.PageSetup, d *xmlScanner) {
	num := func(name string) (float64, bool) {
		v := d.attr(name)
		if v == nil {
			return 0, false
		}
		f, err := strconv.ParseFloat(string(v), 64)
		return f, err == nil
	}
	on := func(name string) bool {
		v := string(d.attr(name))
		return v == "1" || v == "true"
	}
	switch string(d.name) {
	case "pageSetUpPr":
		pr.fit = on("fitToPage")
	case "printOptions":
		p.CenterH, p.CenterV, p.Headings, p.Gridlines = on("horizontalCentered"), on("verticalCentered"), on("headings"), on("gridLines")
	case "pageMargins":
		m := canvas.MarginsNormal
		normal := true
		for _, f := range []struct {
			name string
			dst  *float64
		}{{"left", &m.Left}, {"right", &m.Right}, {"top", &m.Top}, {"bottom", &m.Bottom}, {"header", &m.Header}, {"footer", &m.Footer}} {
			if v, ok := num(f.name); ok {
				cm := round4(v * 2.54)
				if math.Abs(cm-*f.dst) > 0.001 {
					normal = false
				}
				*f.dst = cm
			}
		}
		if !normal {
			p.Margins = &m
		}
	case "pageSetup":
		if v, ok := num("paperSize"); ok {
			for name, code := range xlsxPaper {
				if float64(code) == v {
					p.Paper = name
				}
			}
		}
		if v, ok := num("scale"); ok && v >= 10 && v <= 400 {
			p.Scale = int(v)
		}
		if pr.fit {
			p.FitWide, p.FitTall = 1, 1
			if v, ok := num("fitToWidth"); ok {
				p.FitWide = int(v)
			}
			if v, ok := num("fitToHeight"); ok {
				p.FitTall = int(v)
			}
			if p.FitWide == 0 && p.FitTall == 0 {
				p.FitWide = 1
			}
		}
		p.Landscape = string(d.attr("orientation")) == "landscape"
		p.AcrossFirst = string(d.attr("pageOrder")) == "overThenDown"
		p.BlackWhite = on("blackAndWhite")
		if v, ok := num("firstPageNumber"); ok && on("useFirstPageNumber") && v >= 1 {
			p.FirstPage = int(v)
		}
	case "rowBreaks":
		pr.rowBreaks = true
	case "colBreaks":
		pr.rowBreaks = false
	case "brk":
		if v, ok := num("id"); ok && v > 0 && string(d.attr("man")) != "0" {
			if pr.rowBreaks {
				p.RowBreaks = append(p.RowBreaks, int(v))
			} else {
				p.ColBreaks = append(p.ColBreaks, int(v))
			}
		}
	}
}
