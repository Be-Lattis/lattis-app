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

	"github.com/lalamefine/lattis/internal/canvas"
)

// Écriture d'un classeur .xlsx avec la seule bibliothèque standard. Chaque
// feuille du classeur devient une feuille Excel (les feuilles masquées le
// restent), aux mêmes positions : les tableaux deviennent des tableaux
// Excel (nom, ligne d'en-têtes, filtre, colonnes calculées), avec les
// cellules libres, les résultats déversés et les tableaux croisés (en
// valeurs). Les formules sont traduites quand Excel a un équivalent (voir
// exportformula.go), sinon exportées par leur valeur ; la valeur calculée
// accompagne toujours la formule. Mise en forme : formats de nombre et de
// date, police, couleurs, alignement, bordures, largeurs, hauteurs, lignes
// et colonnes masquées, fusions, volets figés, mise en page (impression).
// Ni graphiques, ni mise en forme conditionnelle, ni notes, ni validations :
// le compte rendu (ExportReport) dit ce qui n'est pas exporté tel quel.

// ExportXLSX écrit tout le classeur au format .xlsx et rend le compte rendu,
// dont les cellules hors des limites d'Excel (1 048 576 lignes × 16 384
// colonnes), qui ne sont pas exportées.
func ExportXLSX(ctx context.Context, m *canvas.Model, w io.Writer) (ExportReport, error) {
	sheets, fm, err := exportBook(ctx, m, toXLSX)
	if err != nil {
		return ExportReport{}, err
	}
	if err := writeXLSX(ctx, w, sheets, fm); err != nil {
		return ExportReport{}, err
	}
	return report(ctx, m, sheets), nil
}

// ExportXLSXFile écrit le classeur dans un fichier (via un fichier temporaire).
func ExportXLSXFile(ctx context.Context, m *canvas.Model, path string) (rep ExportReport, err error) {
	err = writeAtomic(path, func(w io.Writer) error {
		var e error
		rep, e = ExportXLSX(ctx, m, w)
		return e
	})
	return rep, err
}

// xlsxTable est un tableau Excel écrit (partie xl/tables/tableN.xml).
type xlsxTable struct {
	id   int
	xt   *xtable
	ref  string
	cols []string
}

func writeXLSX(ctx context.Context, w io.Writer, sheets []xsheet, fm *fmts) error {
	zw := zip.NewWriter(w)
	file := func(name, content string) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, content)
		return err
	}
	active := 0
	for i, s := range sheets {
		if !s.hidden {
			active = i
			break
		}
	}
	// Tableaux Excel : numérotés dans tout le classeur.
	tables := make([][]xlsxTable, len(sheets))
	n := 0
	for i, s := range sheets {
		for _, xt := range s.tables {
			p := xt.place
			if p == nil || len(p.cols) == 0 || p.row+1 >= ExcelRows || p.col+len(p.cols)-1 >= ExcelCols {
				continue
			}
			n++
			ref := fmt.Sprintf("%s%d:%s%d", colName(p.col), p.row+1, colName(p.col+len(p.cols)-1), min(p.lastRow(), ExcelRows-1)+1)
			tables[i] = append(tables[i], xlsxTable{id: n, xt: xt, ref: ref, cols: p.cols})
		}
	}
	var types, wb, rels strings.Builder
	types.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	fmt.Fprintf(&wb, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView activeTab="%d"/></bookViews><sheets>`, active)
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i, s := range sheets {
		n := i + 1
		fmt.Fprintf(&types, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, n)
		state := ""
		if s.hidden {
			state = ` state="hidden"`
		}
		fmt.Fprintf(&wb, `<sheet name="%s" sheetId="%d"%s r:id="rId%d"/>`, xmlEscape(s.name), n, state, n)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, n, n)
		for _, t := range tables[i] {
			fmt.Fprintf(&types, `<Override PartName="/xl/tables/table%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.table+xml"/>`, t.id)
		}
	}
	types.WriteString(`</Types>`)
	// Excel recalcule à l'ouverture : les valeurs écrites ne servent qu'en
	// attendant (et aux lecteurs qui ne calculent pas).
	wb.WriteString(`</sheets>` + xlsxDefinedNames(sheets) + `<calcPr calcId="191029" fullCalcOnLoad="1"/></workbook>`)
	fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`, len(sheets)+1)
	for _, f := range [][2]string{
		{"[Content_Types].xml", types.String()},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", wb.String()},
		{"xl/_rels/workbook.xml.rels", rels.String()},
	} {
		if err := file(f[0], f[1]); err != nil {
			return err
		}
	}
	for i := range sheets {
		f, err := zw.Create(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1))
		if err != nil {
			return err
		}
		if err := writeSheet(ctx, f, &sheets[i], i == active, tables[i], fm); err != nil {
			return err
		}
		if len(tables[i]) == 0 {
			continue
		}
		var r strings.Builder
		r.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
		for k, t := range tables[i] {
			fmt.Fprintf(&r, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/table" Target="../tables/table%d.xml"/>`, k+1, t.id)
			if err := file(fmt.Sprintf("xl/tables/table%d.xml", t.id), tablePart(t)); err != nil {
				return err
			}
		}
		r.WriteString(`</Relationships>`)
		if err := file(fmt.Sprintf("xl/worksheets/_rels/sheet%d.xml.rels", i+1), r.String()); err != nil {
			return err
		}
	}
	// Les styles en dernier : toutes les mises en forme sont alors connues.
	if err := file("xl/styles.xml", xlsxStyles(fm.all())); err != nil {
		return err
	}
	return zw.Close()
}

// tablePart écrit la définition d'un tableau Excel.
func tablePart(t xlsxTable) string {
	var b strings.Builder
	name := xmlEscape(t.xt.place.name)
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<table xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" id="%d" name="%s" displayName="%s" ref="%s" totalsRowShown="0"><autoFilter ref="%s"/><tableColumns count="%d">`, t.id, name, name, t.ref, t.ref, len(t.cols))
	for j, c := range t.cols {
		fmt.Fprintf(&b, `<tableColumn id="%d" name="%s"`, j+1, xmlEscape(c))
		if j < len(t.xt.formulas) && t.xt.formulas[j] != "" {
			fmt.Fprintf(&b, `><calculatedColumnFormula>%s</calculatedColumnFormula></tableColumn>`, xmlEscape(t.xt.formulas[j]))
		} else {
			b.WriteString(`/>`)
		}
	}
	// Style clair : la mise en forme exportée (en-têtes, couleurs) reste lisible.
	b.WriteString(`</tableColumns><tableStyleInfo name="TableStyleLight1" showFirstColumn="0" showLastColumn="0" showRowStripes="1" showColumnStripes="0"/></table>`)
	return b.String()
}

// pxWidth convertit une largeur en pixels en largeur Excel (caractères :
// environ 7 pixels par caractère).
func pxWidth(px int) string {
	return strconv.FormatFloat(math.Round(float64(px)/7*100)/100, 'f', -1, 64)
}

// defaultColPx : largeur d'une colonne sans largeur choisie.
const defaultColPx = 100

func writeSheet(ctx context.Context, w io.Writer, s *xsheet, active bool, tables []xlsxTable, fm *fmts) error {
	bw := bufio.NewWriterSize(w, 64<<10)
	keys := s.sortedKeys()
	bw.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	pr := xlsxPageSetUpPr(s.page)
	if c := color(s.info.Color); c != "" {
		pr = fmt.Sprintf(`<tabColor rgb="FF%s"/>`, c) + pr
	}
	if pr != "" {
		fmt.Fprintf(bw, `<sheetPr>%s</sheetPr>`, pr)
	}
	maxRow, maxCol := -1, 0
	if len(keys) > 0 {
		maxRow = keys[len(keys)-1][0]
		for _, k := range keys {
			maxCol = max(maxCol, k[1])
		}
	}
	for _, xt := range s.tables {
		tb := xt.info
		maxRow = max(maxRow, min(tb.Row+tb.Total, ExcelRows-1))
		maxCol = max(maxCol, min(tb.Col+len(tb.Columns)-1, ExcelCols-1))
	}
	if maxRow >= 0 {
		fmt.Fprintf(bw, `<dimension ref="A1:%s%d"/>`, colName(maxCol), maxRow+1)
	}
	view := ""
	if active {
		view += ` tabSelected="1"`
	}
	if s.info.NoGrid {
		view += ` showGridLines="0"`
	}
	if z := s.info.Zoom; z > 0 && z != 100 {
		view += fmt.Sprintf(` zoomScale="%d"`, min(max(z, 10), 400))
	}
	fmt.Fprintf(bw, `<sheetViews><sheetView%s workbookViewId="0"`, view)
	if fr, fc := s.info.FreezeRows, s.info.FreezeCols; fr > 0 || fc > 0 {
		pane := "bottomRight"
		switch {
		case fc == 0:
			pane = "bottomLeft"
		case fr == 0:
			pane = "topRight"
		}
		split := ""
		if fc > 0 {
			split += fmt.Sprintf(` xSplit="%d"`, fc)
		}
		if fr > 0 {
			split += fmt.Sprintf(` ySplit="%d"`, fr)
		}
		fmt.Fprintf(bw, `><pane%s topLeftCell="%s%d" activePane="%s" state="frozen"/></sheetView></sheetViews>`, split, colName(fc), fr+1, pane)
	} else {
		bw.WriteString(`/></sheetViews>`)
	}
	writeCols(bw, s)
	bw.WriteString(`<sheetData>`)
	if err := mergeRows(ctx, s, keys, s.marks(), func(r int, cols []int, cells []xcell) error {
		fmt.Fprintf(bw, `<row r="%d"`, r+1)
		if n := s.rowFmt[r]; n != 0 {
			fmt.Fprintf(bw, ` s="%d" customFormat="1"`, n)
		}
		if h, ok := s.heights[r]; ok {
			fmt.Fprintf(bw, ` ht="%s" customHeight="1"`, strconv.FormatFloat(float64(h)*0.75, 'f', -1, 64))
		}
		if s.hiddenR[r] {
			bw.WriteString(` hidden="1"`)
		}
		bw.WriteString(`>`)
		for i, c := range cells {
			writeCell(bw, r, cols[i], c, fm)
		}
		bw.WriteString(`</row>`)
		return nil
	}); err != nil {
		return err
	}
	bw.WriteString(`</sheetData>`)
	var merges []string
	for _, g := range s.merges {
		r1, c1 := g[0]+g[2]-1, g[1]+g[3]-1
		if r1 < ExcelRows && c1 < ExcelCols {
			merges = append(merges, fmt.Sprintf(`<mergeCell ref="%s%d:%s%d"/>`, colName(g[1]), g[0]+1, colName(c1), r1+1))
		}
	}
	if len(merges) > 0 {
		fmt.Fprintf(bw, `<mergeCells count="%d">%s</mergeCells>`, len(merges), strings.Join(merges, ""))
	}
	bw.WriteString(xlsxPageXML(s.page))
	if len(tables) > 0 {
		fmt.Fprintf(bw, `<tableParts count="%d">`, len(tables))
		for k := range tables {
			fmt.Fprintf(bw, `<tablePart r:id="rId%d"/>`, k+1)
		}
		bw.WriteString(`</tableParts>`)
	}
	bw.WriteString(`</worksheet>`)
	return bw.Flush()
}

// writeCols écrit les largeurs, styles et masquage des colonnes.
func writeCols(bw *bufio.Writer, s *xsheet) {
	widths := map[int]int{}
	set := map[int]bool{}
	for _, c := range s.widths {
		widths[c.Col] = c.Width
		set[c.Col] = true
	}
	for c := range s.colFmt {
		set[c] = true
	}
	for c := range s.hiddenC {
		set[c] = true
	}
	var cols []int
	for c := range set {
		if c < ExcelCols {
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		return
	}
	slices.Sort(cols)
	bw.WriteString(`<cols>`)
	for _, c := range cols {
		w, ok := widths[c]
		if !ok {
			w = defaultColPx
		}
		fmt.Fprintf(bw, `<col min="%d" max="%d" width="%s"`, c+1, c+1, pxWidth(w))
		if n := s.colFmt[c]; n != 0 {
			fmt.Fprintf(bw, ` style="%d"`, n)
		}
		if s.hiddenC[c] {
			bw.WriteString(` hidden="1"`)
		}
		bw.WriteString(` customWidth="1"/>`)
	}
	bw.WriteString(`</cols>`)
}

// writeCell écrit une cellule de la ligne r, colonne col.
func writeCell(bw *bufio.Writer, r, col int, c xcell, fm *fmts) {
	ref := colName(col) + strconv.Itoa(r+1)
	st := c.fm
	if c.kind == 'd' && fm != nil {
		st = fm.date(st, c.num)
	}
	style := ""
	if st != 0 {
		style = ` s="` + strconv.Itoa(int(st)) + `"`
	}
	f := ""
	if c.f != "" {
		f = "<f>" + xmlEscape(c.f) + "</f>"
	}
	switch c.kind {
	case 'n', 'd':
		fmt.Fprintf(bw, `<c r="%s"%s>%s<v>%s</v></c>`, ref, style, f, strconv.FormatFloat(c.num, 'g', -1, 64))
	case 'b':
		fmt.Fprintf(bw, `<c r="%s"%s t="b">%s<v>%d</v></c>`, ref, style, f, int(c.num))
	case 'e':
		fmt.Fprintf(bw, `<c r="%s"%s t="e">%s<v>%s</v></c>`, ref, style, f, xmlEscape(c.str))
	case 's':
		if f != "" {
			fmt.Fprintf(bw, `<c r="%s"%s t="str">%s<v>%s</v></c>`, ref, style, f, xmlEscape(c.str))
		} else {
			fmt.Fprintf(bw, `<c r="%s"%s t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, style, xmlEscape(c.str))
		}
	default:
		fmt.Fprintf(bw, `<c r="%s"%s>%s</c>`, ref, style, f)
	}
}
