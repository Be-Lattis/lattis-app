package transfer

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/formula"
	"github.com/lalamefine/slitable/internal/grid"
)

// Écriture d'un classeur .xlsx avec la seule bibliothèque standard. Chaque
// feuille du classeur devient une feuille Excel (les feuilles masquées le
// restent) : les tableaux avec leur ligne d'en-têtes (en gras), les cellules
// libres, les résultats déversés et les tableaux croisés, aux mêmes
// positions, ainsi que les largeurs de colonnes. Version 1 : les formules
// sont exportées par leur valeur calculée (pas de formule Excel), sans la
// mise en forme ni les graphiques.

// xcell est une cellule à écrire.
type xcell struct {
	kind byte // 'n' nombre, 'd' date (numéro de série), 's' texte, 'b' booléen, 'e' erreur
	num  float64
	str  string
	bold bool
}

type xsheet struct {
	name   string
	hidden bool
	cells  map[[2]int]xcell
	widths []canvas.ColWidth
}

// ExportXLSX écrit tout le classeur au format .xlsx.
func ExportXLSX(ctx context.Context, m *canvas.Model, w io.Writer) error {
	first, err := m.Layout(ctx, 0)
	if err != nil {
		return err
	}
	var sheets []xsheet
	for _, s := range first.Sheets {
		sh, err := collectSheet(ctx, m, s)
		if err != nil {
			return err
		}
		sheets = append(sheets, sh)
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return writeXLSX(w, sheets)
}

// collectSheet lit le contenu d'une feuille, cellule par cellule.
func collectSheet(ctx context.Context, m *canvas.Model, s canvas.SheetInfo) (xsheet, error) {
	l, err := m.Layout(ctx, s.ID)
	if err != nil {
		return xsheet{}, err
	}
	sh := xsheet{name: s.Name, hidden: s.Hidden, cells: map[[2]int]xcell{}, widths: l.Widths}
	put := func(r, c int, x xcell) {
		if x.kind != 0 {
			sh.cells[[2]int{r, c}] = x
		}
	}
	// Tableaux croisés, cellules libres puis tableaux : un tableau
	// l'emporte sur ce qu'il recouvre.
	for _, p := range l.Pivots {
		for i, row := range p.Cells {
			for j, v := range row {
				x := textCell(v)
				x.bold = i == 0
				put(p.Row+i, p.Col+j, x)
			}
		}
	}
	for _, c := range l.Cells {
		if len(c.Spill) > 0 {
			for i, row := range c.Spill {
				for j, v := range row {
					put(c.Row+i, c.Col+j, textCell(v))
				}
			}
			continue
		}
		switch {
		case c.Error:
			put(c.Row, c.Col, errorCell(c.Value))
		case strings.HasPrefix(c.Raw, "="):
			put(c.Row, c.Col, textCell(c.Value))
		default:
			put(c.Row, c.Col, textCell(c.Raw))
		}
	}
	for _, tb := range l.Tables {
		var cols []canvas.ExportColumn
		r := tb.Row
		err := m.ExportTable(ctx, tb.Name, func(cs []canvas.ExportColumn) error {
			cols = cs
			for j, c := range cs {
				put(r, tb.Col+j, xcell{kind: 's', str: c.Name, bold: true})
			}
			return nil
		}, func(row []any) error {
			r++
			for j, v := range row {
				put(r, tb.Col+j, tableCell(v, cols[j].Type))
			}
			return nil
		})
		if err != nil {
			return xsheet{}, err
		}
	}
	return sh, nil
}

// ExportXLSXFile écrit le classeur dans un fichier (via un fichier temporaire).
func ExportXLSXFile(ctx context.Context, m *canvas.Model, path string) error {
	return writeAtomic(path, func(w io.Writer) error { return ExportXLSX(ctx, m, w) })
}

// textCell type une valeur affichée (cellule libre, TCD, déversement) comme
// le ferait la saisie : nombre, booléen, date ou texte.
func textCell(s string) xcell {
	if s == "" {
		return xcell{}
	}
	if strings.HasPrefix(s, "#") {
		if _, ok := excelErrors[s]; ok {
			return errorCell(s)
		}
	}
	v := parseText(s)
	switch v.t {
	case grid.Integer:
		return xcell{kind: 'n', num: float64(v.i)}
	case grid.Decimal:
		return xcell{kind: 'n', num: v.f}
	case grid.Boolean:
		return xcell{kind: 'b', num: float64(v.i)}
	case grid.Date:
		if f, ok := formula.ParseDate(v.s); ok {
			return xcell{kind: 'd', num: f}
		}
	}
	if f, ok := formula.ParseDate(s); ok && strings.Contains(s, ":") {
		return xcell{kind: 'd', num: f}
	}
	return xcell{kind: 's', str: s}
}

// excelErrors : codes d'erreur de Slitable (français) et leur code Excel.
var excelErrors = map[string]string{
	"#DIV/0!": "#DIV/0!", "#N/A": "#N/A", "#VALEUR!": "#VALUE!", "#REF!": "#REF!",
	"#NOM?": "#NAME?", "#NOMBRE!": "#NUM!", "#NUL!": "#NULL!",
}

func errorCell(s string) xcell {
	if e, ok := excelErrors[s]; ok {
		return xcell{kind: 'e', str: e}
	}
	return xcell{kind: 's', str: s}
}

// tableCell convertit une valeur lue par canvas.Model.ExportTable.
func tableCell(v any, t grid.Type) xcell {
	switch v := v.(type) {
	case nil:
		return xcell{}
	case int64:
		if t == grid.Boolean {
			return xcell{kind: 'b', num: float64(v)}
		}
		return xcell{kind: 'n', num: float64(v)}
	case float64:
		return xcell{kind: 'n', num: v}
	case string:
		if t == grid.Date {
			if f, ok := formula.ParseDate(v); ok {
				return xcell{kind: 'd', num: f}
			}
		}
		return xcell{kind: 's', str: v}
	case canvas.Invalid:
		return xcell{kind: 's', str: string(v)}
	case formula.Value:
		switch v.Kind {
		case formula.Number:
			return xcell{kind: 'n', num: v.Num}
		case formula.Date:
			return xcell{kind: 'd', num: v.Num}
		case formula.Bool:
			if v.Bool {
				return xcell{kind: 'b', num: 1}
			}
			return xcell{kind: 'b'}
		case formula.Error:
			return errorCell(v.String())
		case formula.Text:
			return xcell{kind: 's', str: v.Str}
		}
		return xcell{}
	}
	return xcell{kind: 's', str: fmt.Sprint(v)}
}

// excelSheetNames rend les noms valides pour Excel : 31 caractères au plus,
// sans []:*?/\, uniques sans tenir compte de la casse.
func excelSheetNames(names []string) []string {
	out := make([]string, len(names))
	seen := map[string]bool{}
	for i, n := range names {
		n = strings.Map(func(r rune) rune {
			if strings.ContainsRune(`[]:*?/\`, r) {
				return '_'
			}
			return r
		}, n)
		n = strings.Trim(n, "'")
		if n == "" {
			n = fmt.Sprintf("Feuille%d", i+1)
		}
		base := truncateRunes(n, 31)
		cand := base
		for k := 2; seen[strings.ToLower(cand)]; k++ {
			suffix := fmt.Sprintf(" (%d)", k)
			cand = truncateRunes(base, 31-utf8.RuneCountInString(suffix)) + suffix
		}
		seen[strings.ToLower(cand)] = true
		out[i] = cand
	}
	return out
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// colName donne la lettre de colonne Excel (0 : A).
func colName(c int) string {
	s := ""
	for c++; c > 0; c = (c - 1) / 26 {
		s = string(rune('A'+(c-1)%26)) + s
	}
	return s
}

func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '&':
			b.WriteString("&amp;")
		case r == '"':
			b.WriteString("&quot;")
		case r == '\t' || r == '\n' || r == '\r' || r >= 0x20 && r != 0xFFFE && r != 0xFFFF:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Styles : 0 normal, 1 date, 2 date et heure, 3 gras.
const xlsxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<numFmts count="1"><numFmt numFmtId="164" formatCode="dd/mm/yyyy\ hh:mm"/></numFmts>
<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts>
<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>
<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="4"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="14" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/><xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/></cellXfs>
<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>
</styleSheet>`

func writeXLSX(w io.Writer, sheets []xsheet) error {
	zw := zip.NewWriter(w)
	file := func(name, content string) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, content)
		return err
	}
	var types, wb, rels strings.Builder
	types.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	wb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>`)
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	names := make([]string, len(sheets))
	for i, s := range sheets {
		names[i] = s.name
	}
	names = excelSheetNames(names)
	active := 0
	for i, s := range sheets {
		if !s.hidden {
			active = i
			break
		}
	}
	for i, s := range sheets {
		n := i + 1
		fmt.Fprintf(&types, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, n)
		state := ""
		if s.hidden {
			state = ` state="hidden"`
		}
		fmt.Fprintf(&wb, `<sheet name="%s" sheetId="%d"%s r:id="rId%d"/>`, xmlEscape(names[i]), n, state, n)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, n, n)
	}
	types.WriteString(`</Types>`)
	wb.WriteString(`</sheets></workbook>`)
	fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`, len(sheets)+1)
	for _, f := range [][2]string{
		{"[Content_Types].xml", types.String()},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", wb.String()},
		{"xl/_rels/workbook.xml.rels", rels.String()},
		{"xl/styles.xml", xlsxStyles},
	} {
		if err := file(f[0], f[1]); err != nil {
			return err
		}
	}
	for i, s := range sheets {
		f, err := zw.Create(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1))
		if err != nil {
			return err
		}
		if err := writeSheet(f, s, i == active); err != nil {
			return err
		}
	}
	return zw.Close()
}

func writeSheet(w io.Writer, s xsheet, active bool) error {
	bw := bufio.NewWriterSize(w, 64<<10)
	keys := make([][2]int, 0, len(s.cells))
	for k := range s.cells {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b [2]int) int {
		if a[0] != b[0] {
			return a[0] - b[0]
		}
		return a[1] - b[1]
	})
	bw.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	if len(keys) > 0 {
		last := keys[len(keys)-1]
		maxCol := 0
		for _, k := range keys {
			maxCol = max(maxCol, k[1])
		}
		fmt.Fprintf(bw, `<dimension ref="A1:%s%d"/>`, colName(maxCol), last[0]+1)
	}
	if active {
		bw.WriteString(`<sheetViews><sheetView tabSelected="1" workbookViewId="0"/></sheetViews>`)
	} else {
		bw.WriteString(`<sheetViews><sheetView workbookViewId="0"/></sheetViews>`)
	}
	if len(s.widths) > 0 {
		ws := slices.Clone(s.widths)
		slices.SortFunc(ws, func(a, b canvas.ColWidth) int { return a.Col - b.Col })
		bw.WriteString(`<cols>`)
		for _, c := range ws {
			// Largeur Excel en caractères : environ 7 pixels par caractère.
			fmt.Fprintf(bw, `<col min="%d" max="%d" width="%s" customWidth="1"/>`, c.Col+1, c.Col+1, strconv.FormatFloat(math.Round(float64(c.Width)/7*100)/100, 'f', -1, 64))
		}
		bw.WriteString(`</cols>`)
	}
	bw.WriteString(`<sheetData>`)
	row := -1
	for _, k := range keys {
		if k[0] != row {
			if row >= 0 {
				bw.WriteString(`</row>`)
			}
			row = k[0]
			fmt.Fprintf(bw, `<row r="%d">`, row+1)
		}
		c := s.cells[k]
		ref := colName(k[1]) + strconv.Itoa(k[0]+1)
		style := ""
		if c.bold {
			style = ` s="3"`
		}
		switch c.kind {
		case 'n':
			fmt.Fprintf(bw, `<c r="%s"%s><v>%s</v></c>`, ref, style, strconv.FormatFloat(c.num, 'g', -1, 64))
		case 'd':
			st := 1
			if c.num != math.Floor(c.num) {
				st = 2
			}
			fmt.Fprintf(bw, `<c r="%s" s="%d"><v>%s</v></c>`, ref, st, strconv.FormatFloat(c.num, 'g', -1, 64))
		case 'b':
			fmt.Fprintf(bw, `<c r="%s"%s t="b"><v>%d</v></c>`, ref, style, int(c.num))
		case 'e':
			fmt.Fprintf(bw, `<c r="%s"%s t="e"><v>%s</v></c>`, ref, style, xmlEscape(c.str))
		default:
			fmt.Fprintf(bw, `<c r="%s"%s t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, style, xmlEscape(c.str))
		}
	}
	if row >= 0 {
		bw.WriteString(`</row>`)
	}
	bw.WriteString(`</sheetData></worksheet>`)
	return bw.Flush()
}
