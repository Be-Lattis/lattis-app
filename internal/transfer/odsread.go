package transfer

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"

	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
)

// Import d'un classeur OpenDocument (.ods) : chaque feuille est récrite en
// SpreadsheetML, en mémoire, puis suit le chemin d'un fichier .xlsx (îlots
// et tableaux, types, mise en forme, fusions, largeurs, hauteurs, lignes et
// colonnes masquées, notes). Valeurs : nombres, pourcentages, montants,
// dates, heures (en texte), booléens, texte ; les formules sont reprises par
// leur valeur. Mise en forme : police, taille, gras, italique, souligné,
// barré, couleurs, remplissage, bordures, alignement, renvoi à la ligne,
// formats de nombre (décimales, milliers, pourcentage, euro, date, heure,
// texte).

// odsStyle est un style de cellule nommé (ses propriétés et celles de son
// parent).
type odsStyle struct {
	parent string
	data   string
	props  canvas.Style
}

type odsReader struct {
	cells   map[string]*odsStyle
	data    map[string]canvas.Style // styles de données (formats)
	kinds   map[string]dateKind
	colW    map[string]int // style de colonne → largeur (px)
	rowH    map[string]int
	index   map[string]int // style de cellule → rang (attribut s)
	st      *bookStyles
	dates   []dateKind
	formula int
	// produced : octets des feuilles réécrites au format .xlsx (maxODSSheetXML).
	produced int64
}

// Bornes de la lecture d'un .ods (fichier reçu : bombe de décompression ou
// de répétition) : texte d'une cellule (limite d'Excel) et feuilles
// réécrites en mémoire, toutes ensemble.
const (
	maxCellText    = 32767
	maxODSSheetXML = 512 << 20
)

// errODSTooLarge : feuilles réécrites au-delà de maxODSSheetXML.
var errODSTooLarge = msg.New("transfer.odsTooLarge", nil)

// odsLen lit une longueur (2.5cm, 1in, 12pt, 30mm, 96px) en pixels.
func odsLen(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	units := map[string]float64{"cm": 96 / 2.54, "mm": 96 / 25.4, "in": 96, "pt": 96.0 / 72, "pc": 16, "px": 1}
	for u, k := range units {
		if strings.HasSuffix(s, u) {
			f, err := strconv.ParseFloat(strings.TrimSuffix(s, u), 64)
			return f * k, err == nil
		}
	}
	return 0, false
}

// odsInEdge lit une bordure (« 0.75pt solid #000000 ») ; "" sans bordure.
func odsInEdge(v string) any {
	f := strings.Fields(v)
	if len(f) == 0 || v == "none" {
		return nil
	}
	w, kind, col := 0.75, "solid", ""
	for _, p := range f {
		switch {
		case strings.HasPrefix(p, "#"):
			col = strings.ToLower(p)
		case p == "solid" || p == "double" || p == "dotted" || p == "dashed":
			kind = p
		case p == "none" || p == "hidden":
			return nil
		default:
			if px, ok := odsLen(p); ok {
				w = px * 0.75
			}
		}
	}
	style := "thin"
	switch {
	case kind == "double":
		style = "double"
	case kind == "dotted" || kind == "dashed":
		style = kind
	case w >= 2.25:
		style = "thick"
	case w >= 1.25:
		style = "medium"
	}
	if col == "" || col == "#000000" {
		if style == "thin" {
			return true
		}
		return style
	}
	return style + " " + col
}

// prop lit une propriété de style (balises *-properties).
func (o *odsReader) prop(s canvas.Style, d *xmlScanner) {
	a := func(n string) string { return string(d.attr(n)) }
	switch string(d.name) {
	case "table-cell-properties":
		if c := a("background-color"); len(c) == 7 && c[0] == '#' {
			s["bg"] = strings.ToLower(c)
		}
		if b := a("border"); b != "" {
			for _, k := range []string{"bt", "bb", "bl", "br"} {
				if e := odsInEdge(b); e != nil {
					s[k] = e
				}
			}
		}
		for k, side := range map[string]string{"bt": "border-top", "bb": "border-bottom", "bl": "border-left", "br": "border-right"} {
			if b := a(side); b != "" {
				if e := odsInEdge(b); e != nil {
					s[k] = e
				} else {
					delete(s, k)
				}
			}
		}
		switch a("vertical-align") {
		case "top":
			s["va"] = "top"
		case "bottom":
			s["va"] = "bottom"
		}
		if a("wrap-option") == "wrap" {
			s["wr"] = true
		}
	case "paragraph-properties":
		switch a("text-align") {
		case "start", "left":
			s["al"] = "left"
		case "center":
			s["al"] = "center"
		case "end", "right":
			s["al"] = "right"
		}
	case "text-properties":
		if a("font-weight") == "bold" {
			s["b"] = true
		}
		if a("font-style") == "italic" {
			s["i"] = true
		}
		if u := a("text-underline-style"); u != "" && u != "none" {
			s["u"] = true
		}
		if l := a("text-line-through-style"); l != "" && l != "none" {
			s["x"] = true
		}
		if c := a("color"); len(c) == 7 && c[0] == '#' && strings.ToLower(c) != "#000000" {
			s["fg"] = strings.ToLower(c)
		}
		if sz, ok := odsLen(a("font-size")); ok && sz > 0 && math.Abs(sz*0.75-10) > 0.01 && math.Abs(sz*0.75-11) > 0.01 {
			s["fs"] = math.Round(sz*0.75*10) / 10
		}
		if f := strings.Trim(a("font-family"), `'"`); f != "" {
			s["ff"] = f
		} else if f := a("font-name"); f != "" && f != "Liberation Sans" && f != "Arial" {
			s["ff"] = f
		}
	}
}

// styles lit les styles (styles.xml et styles automatiques de content.xml)
// jusqu'au corps du document ; rend vrai si le corps commence.
func (o *odsReader) styles(d *xmlScanner) (bool, error) {
	var cur *odsStyle
	family, name := "", ""
	var num canvas.Style
	var numKind dateKind
	var pat strings.Builder
	inNum, inText := false, false
	for {
		if err := d.next(); err != nil {
			if err == io.EOF {
				return false, nil
			}
			return false, err
		}
		switch d.kind {
		case xmlStart:
			n := string(d.name)
			switch {
			case n == "body":
				return true, nil
			case n == "style" || n == "default-style":
				family, name = string(d.attr("family")), string(d.attr("name"))
				if n == "default-style" {
					name = "Default"
				}
				cur = &odsStyle{parent: string(d.attr("parent-style-name")), data: string(d.attr("data-style-name")), props: canvas.Style{}}
				if family == "table-cell" {
					o.cells[name] = cur
				}
			case strings.HasSuffix(n, "-properties") && cur != nil:
				switch family {
				case "table-column":
					if w, ok := odsLen(string(d.attr("column-width"))); ok {
						o.colW[name] = int(math.Round(w))
					}
				case "table-row":
					if h, ok := odsLen(string(d.attr("row-height"))); ok && string(d.attr("use-optimal-row-height")) != "true" {
						o.rowH[name] = int(math.Round(h))
					}
				default:
					o.prop(cur.props, d)
				}
			case odsNumStyles[n] && !inNum:
				inNum, name, pat = true, string(d.attr("name")), strings.Builder{}
				num, numKind = canvas.Style{}, notDate
				switch n {
				case "percentage-style":
					num["nf"] = "%"
				case "currency-style":
					num["nf"] = "€"
				case "date-style":
					num["nf"], numKind = "date", dateOnly
				case "time-style":
					num["nf"], numKind = "time", timeOnly
				case "text-style":
					num["nf"] = "text"
				}
			case inNum:
				long := string(d.attr("style")) == "long"
				switch n {
				case "number":
					dp, _ := strconv.Atoi(string(d.attr("decimal-places")))
					if num["nf"] == nil {
						num["nf"] = "0"
						if dp > 0 {
							num["nf"] = "0,00"
						}
					}
					if num["nf"] != "0" && dp != 2 {
						num["dp"] = float64(dp)
					}
					if string(d.attr("grouping")) != "true" && num["nf"] != "%" {
						num["ng"] = true
					}
				case "scientific-number", "fraction":
					num["nc"] = "0,00E+00"
					if n == "fraction" {
						num["nc"] = "# ?/?"
					}
				case "day":
					pat.WriteString(map[bool]string{true: "jj", false: "jj"}[long])
				case "day-of-week":
					pat.WriteString(map[bool]string{true: "jjjj", false: "jjj"}[long])
				case "month":
					switch {
					case string(d.attr("textual")) == "true" && long:
						pat.WriteString("mmmm")
					case string(d.attr("textual")) == "true":
						pat.WriteString("mmm")
					default:
						pat.WriteString("mm")
					}
				case "year":
					pat.WriteString(map[bool]string{true: "aaaa", false: "aa"}[long])
				case "hours":
					pat.WriteString("hh")
					if numKind == dateOnly {
						numKind = dateTime
					}
				case "minutes":
					pat.WriteString("mm")
				case "seconds":
					pat.WriteString("ss")
				case "text":
					inText = true
				}
			}
		case xmlText:
			if inText {
				pat.Write(d.text)
			}
		case xmlEnd:
			n := string(d.name)
			switch {
			case n == "text":
				inText = false
			case inNum && odsNumStyles[n]:
				inNum = false
				if numKind != notDate {
					num["df"] = strings.TrimSpace(pat.String())
				}
				o.data[name] = num
				o.kinds[name] = numKind
			case n == "style" || n == "default-style":
				cur = nil
			}
		}
	}
}

var odsNumStyles = map[string]bool{"number-style": true, "percentage-style": true, "currency-style": true, "date-style": true, "time-style": true, "boolean-style": true, "text-style": true}

// style donne le rang (attribut s) d'un style de cellule, 0 sans style.
func (o *odsReader) style(name string) int {
	if name == "" {
		return 0
	}
	if i, ok := o.index[name]; ok {
		return i
	}
	s := canvas.Style{}
	kind := notDate
	var chain []*odsStyle
	for n, seen := name, 0; n != "" && seen < 10; seen++ {
		c := o.cells[n]
		if c == nil {
			break
		}
		chain = append(chain, c)
		n = c.parent
	}
	if def := o.cells["Default"]; def != nil {
		chain = append(chain, def)
	}
	data := ""
	for i := len(chain) - 1; i >= 0; i-- {
		for k, v := range chain[i].props {
			s[k] = v
		}
		if chain[i].data != "" {
			data = chain[i].data
		}
	}
	if data != "" {
		for k, v := range o.data[data] {
			s[k] = v
		}
		kind = o.kinds[data]
	}
	i := len(o.st.xfs)
	o.st.xfs = append(o.st.xfs, nil)
	o.st.enc = append(o.st.enc, "")
	o.dates = append(o.dates, kind)
	if len(s) > 0 {
		o.st.xfs[i] = s
		b, _ := json.Marshal(s)
		o.st.enc[i] = string(b)
	}
	o.index[name] = i
	return i
}

// odsInSheet : une feuille en cours d'écriture.
type odsInSheet struct {
	name   string
	cols   strings.Builder
	data   strings.Builder
	merges []string
	notes  map[string]string
	colDef []string // style par défaut des cellules de chaque colonne
}

// odsInTime lit une durée « PT13H45M00S » en « 13:45:00 ».
func odsInTime(v string) string {
	v = strings.TrimPrefix(strings.ToUpper(v), "PT")
	var h, m int
	var s float64
	for _, part := range []struct {
		sep string
		set func(string)
	}{{"H", func(x string) { h, _ = strconv.Atoi(x) }}, {"M", func(x string) { m, _ = strconv.Atoi(x) }}, {"S", func(x string) { s, _ = strconv.ParseFloat(x, 64) }}} {
		if a, b, ok := strings.Cut(v, part.sep); ok {
			part.set(a)
			v = b
		}
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, int(s))
}

// body lit les feuilles du document.
func (o *odsReader) body(d *xmlScanner) ([]*odsInSheet, error) {
	var out []*odsInSheet
	var sh *odsInSheet
	row, col := 0, 0
	var rowBuf strings.Builder
	rowRepeat, rowHasContent := 1, false
	rowAttrs := ""
	// Cellule en cours.
	var cellAttrs map[string]string
	var text, note strings.Builder
	paras, noteParas, depthAnn := 0, 0, 0
	colRepeat := 1
	for {
		if err := d.next(); err != nil {
			if err == io.EOF {
				return out, nil
			}
			return nil, err
		}
		switch d.kind {
		case xmlStart:
			n := string(d.name)
			if depthAnn > 0 {
				depthAnn++
				if n == "p" {
					if noteParas++; noteParas > 1 {
						note.WriteString("\n")
					}
				}
				continue
			}
			switch n {
			case "table":
				sh = &odsInSheet{name: string(d.attr("name")), notes: map[string]string{}}
				out = append(out, sh)
				row = 0
			case "table-column":
				if sh == nil {
					continue
				}
				rep, _ := strconv.Atoi(string(d.attr("number-columns-repeated")))
				rep = max(rep, 1)
				w := o.colW[string(d.attr("style-name"))]
				hidden := string(d.attr("visibility")) == "collapse"
				def := string(d.attr("default-cell-style-name"))
				c0 := len(sh.colDef)
				if rep > 1024 {
					rep = min(rep, max(0, 1024-c0))
				}
				for k := 0; k < rep; k++ {
					sh.colDef = append(sh.colDef, def)
				}
				if rep > 0 && (w > 0 || hidden) {
					fmt.Fprintf(&sh.cols, `<col min="%d" max="%d"`, c0+1, c0+rep)
					if w > 0 {
						fmt.Fprintf(&sh.cols, ` width="%s" customWidth="1"`, strconv.FormatFloat(float64(w)/7, 'f', -1, 64))
					}
					if hidden {
						sh.cols.WriteString(` hidden="1"`)
					}
					sh.cols.WriteString(`/>`)
				}
			case "table-row":
				if sh == nil {
					continue
				}
				col = 0
				rowRepeat, _ = strconv.Atoi(string(d.attr("number-rows-repeated")))
				rowRepeat = max(rowRepeat, 1)
				rowBuf.Reset()
				rowHasContent = false
				rowAttrs = ""
				if h := o.rowH[string(d.attr("style-name"))]; h > 0 {
					rowAttrs += fmt.Sprintf(` ht="%s" customHeight="1"`, strconv.FormatFloat(float64(h)*0.75, 'f', -1, 64))
				}
				if string(d.attr("visibility")) == "collapse" {
					rowAttrs += ` hidden="1"`
				}
			case "table-cell", "covered-table-cell":
				if sh == nil {
					continue
				}
				cellAttrs = map[string]string{"_tag": n}
				for _, a := range d.attrs {
					cellAttrs[string(local(a.name))] = string(a.value)
				}
				colRepeat, _ = strconv.Atoi(cellAttrs["number-columns-repeated"])
				colRepeat = max(colRepeat, 1)
				text.Reset()
				note.Reset()
				paras, noteParas = 0, 0
			case "annotation":
				depthAnn = 1
				if d.self {
					depthAnn = 0
				}
			case "p":
				if cellAttrs != nil {
					if paras++; paras > 1 {
						text.WriteString("\n")
					}
				}
			case "s":
				if cellAttrs != nil {
					// Espaces répétés : pas au-delà du texte d'une cellule
					// (text:c="2000000000" allouerait 2 Go d'un coup).
					k, _ := strconv.Atoi(string(d.attr("c")))
					if n := min(max(k, 1), maxCellText-text.Len()); n > 0 {
						text.WriteString(strings.Repeat(" ", n))
					}
				}
			case "tab":
				if cellAttrs != nil {
					text.WriteString("\t")
				}
			case "line-break":
				if cellAttrs != nil {
					text.WriteString("\n")
				}
			}
		case xmlText:
			switch {
			case depthAnn > 0:
				if noteParas > 0 {
					note.Write(d.text)
				}
			case cellAttrs != nil && paras > 0:
				text.Write(d.text)
			}
		case xmlEnd:
			n := string(d.name)
			if depthAnn > 0 {
				depthAnn--
				continue
			}
			switch n {
			case "table-cell", "covered-table-cell":
				if cellAttrs == nil {
					continue
				}
				o.cell(sh, row, &col, colRepeat, cellAttrs, text.String(), strings.TrimSpace(note.String()), &rowBuf, &rowHasContent, rowRepeat)
				cellAttrs = nil
			case "table-row":
				if sh == nil {
					continue
				}
				if rowHasContent || rowAttrs != "" {
					reps := rowRepeat
					if !rowHasContent {
						reps = min(reps, 1)
					}
					for k := 0; k < min(reps, 100_000); k++ {
						r := strconv.Itoa(row + k + 1)
						n, _ := fmt.Fprintf(&sh.data, `<row r="%s"%s>%s</row>`, r, rowAttrs, strings.ReplaceAll(rowBuf.String(), rowMarkODS, r))
						// Lignes répétées : la feuille réécrite en mémoire reste
						// bornée (un petit fichier ne fait pas des dizaines de Go).
						if o.produced += int64(n); o.produced > maxODSSheetXML {
							return nil, errODSTooLarge
						}
					}
				}
				row += rowRepeat
				if row >= ExcelRows {
					row = ExcelRows - 1
				}
			}
		}
	}
}

// rowMarkODS tient la place du numéro de ligne dans une ligne répétée.
const rowMarkODS = "\x00"

// cell écrit une cellule (répétée rep fois) dans la ligne.
func (o *odsReader) cell(sh *odsInSheet, row int, col *int, rep int, a map[string]string, text, note string, buf *strings.Builder, has *bool, rowRep int) {
	styleName := a["style-name"]
	c0 := *col
	*col += rep
	if c0 >= ExcelCols {
		return
	}
	if styleName == "" && c0 < len(sh.colDef) {
		styleName = sh.colDef[c0]
	}
	s := o.style(styleName)
	if a["formula"] != "" {
		o.formula++
	}
	var v, t string
	switch a["value-type"] {
	case "float", "percentage", "currency":
		v = a["value"]
	case "date":
		v, t = a["date-value"], "d"
	case "time":
		v, t = odsInTime(a["time-value"]), "str"
	case "boolean":
		v, t = "0", "b"
		if a["boolean-value"] == "true" {
			v = "1"
		}
	default:
		if text != "" {
			v, t = text, "inlineStr"
		}
	}
	if sp, _ := strconv.Atoi(a["number-columns-spanned"]); sp > 1 || a["number-rows-spanned"] != "" && a["number-rows-spanned"] != "1" {
		rs, _ := strconv.Atoi(a["number-rows-spanned"])
		sh.merges = append(sh.merges, fmt.Sprintf(`<mergeCell ref="%s%d:%s%d"/>`, colName(c0), row+1, colName(c0+max(sp, 1)-1), row+max(rs, 1)))
	}
	if note != "" {
		sh.notes[colName(c0)+strconv.Itoa(row+1)] = note
	}
	if v == "" && (s == 0 || o.st.xfs[s] == nil || rep > 256) {
		return
	}
	if v != "" {
		*has = true
	}
	for k := 0; k < min(rep, ExcelCols-c0); k++ {
		ref := colName(c0+k) + rowMarkODS
		attrs := fmt.Sprintf(` r="%s"`, ref)
		if s > 0 {
			attrs += ` s="` + strconv.Itoa(s) + `"`
		}
		switch {
		case v == "":
			fmt.Fprintf(buf, `<c%s/>`, attrs)
		case t == "inlineStr":
			fmt.Fprintf(buf, `<c%s t="inlineStr"><is><t>%s</t></is></c>`, attrs, xmlEscape(v))
		case t != "":
			fmt.Fprintf(buf, `<c%s t="%s"><v>%s</v></c>`, attrs, t, xmlEscape(v))
		default:
			fmt.Fprintf(buf, `<c%s><v>%s</v></c>`, attrs, xmlEscape(v))
		}
	}
}

// openODS lit un classeur .ods et le ramène au modèle d'un classeur .xlsx.
func openODS(path string) (*xlsxFile, int, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, 0, fmt.Errorf("%w (%v)", ErrNotODS, err)
	}
	if err := checkZip(zr); err != nil {
		zr.Close()
		return nil, 0, err
	}
	x := &xlsxFile{zr: zr}
	o := &odsReader{cells: map[string]*odsStyle{}, data: map[string]canvas.Style{}, kinds: map[string]dateKind{}, colW: map[string]int{}, rowH: map[string]int{}, index: map[string]int{},
		st: &bookStyles{xfs: []canvas.Style{nil}, enc: []string{""}, theme: defaultTheme, indexed: defaultIndexed}, dates: []dateKind{notDate}}
	if f := x.find("styles.xml"); f != nil {
		if r, err := f.Open(); err == nil {
			_, err = o.styles(newXMLScanner(r))
			r.Close()
			if err != nil {
				zr.Close()
				return nil, 0, msg.Wrap(ErrNotODS, "transfer.notOdsUnreadable", msg.Args{"err": err})
			}
		}
	}
	f := x.find("content.xml")
	if f == nil {
		zr.Close()
		return nil, 0, ErrNotODS
	}
	r, err := f.Open()
	if err != nil {
		zr.Close()
		return nil, 0, err
	}
	defer r.Close()
	d := newXMLScanner(r)
	inBody, err := o.styles(d)
	var sheets []*odsInSheet
	if err == nil && inBody {
		sheets, err = o.body(d)
	}
	if err != nil {
		zr.Close()
		if err == errODSTooLarge {
			return nil, 0, err
		}
		return nil, 0, msg.Wrap(ErrNotODS, "transfer.notOdsUnreadable", msg.Args{"err": err})
	}
	x.st, x.dates = o.st, o.dates
	for _, sh := range sheets {
		var b strings.Builder
		b.WriteString(`<worksheet>`)
		if sh.cols.Len() > 0 {
			b.WriteString(`<cols>` + sh.cols.String() + `</cols>`)
		}
		b.WriteString(`<sheetData>` + sh.data.String() + `</sheetData>`)
		if len(sh.merges) > 0 {
			b.WriteString(`<mergeCells>` + strings.Join(sh.merges, "") + `</mergeCells>`)
		}
		b.WriteString(`</worksheet>`)
		x.sheets = append(x.sheets, xlsxSheet{name: sh.name, data: []byte(b.String()), ext: &sheetExt{}, notes: sh.notes, page: &canvas.PageSetup{}})
	}
	return x, o.formula, nil
}

// ErrNotODS signale un fichier qui n'est pas un classeur .ods lisible.
var ErrNotODS = msg.New("transfer.notOds", nil)

// ImportODS ajoute au classeur les feuilles d'un classeur OpenDocument .ods.
func ImportODS(ctx context.Context, m *canvas.Model, path string) (canvas.ImportResult, error) {
	x, formulas, err := openODS(path)
	if err != nil {
		return canvas.ImportResult{}, err
	}
	defer x.Close()
	res, err := x.importBook(ctx, m, path)
	if formulas > 0 {
		res.Warnings = append([]string{msg.Format(m.Language(), "import.ods.formulas", msg.Args{"n": formulas})}, res.Warnings...)
	}
	return res, err
}
