package transfer

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Écriture d'un classeur OpenDocument (.ods : LibreOffice, OnlyOffice…)
// avec la seule bibliothèque standard, sur le modèle de l'export .xlsx :
// mêmes cellules, formules traduites en OpenFormula quand c'est possible
// (sinon leur valeur), même mise en forme. Les tableaux deviennent des
// plages nommées (leurs données, sous le nom du tableau) avec leur ligne
// d'en-têtes mise en forme. Les styles de cellules sont écrits dans
// styles.xml, après le contenu : ils ne sont tous connus qu'une fois les
// tableaux lus.

// ExportODS écrit tout le classeur au format .ods ; clipped compte les
// cellules au-delà de 1 048 576 lignes × 16 384 colonnes (limites de
// LibreOffice comme d'Excel), qui ne sont pas exportées.
func ExportODS(ctx context.Context, m *canvas.Model, w io.Writer) (clipped int, err error) {
	sheets, fm, err := exportBook(ctx, m, toODS)
	if err != nil {
		return 0, err
	}
	err = writeODS(ctx, w, sheets, fm)
	for _, sh := range sheets {
		clipped += sh.clipped
	}
	return clipped, err
}

// ExportODSFile écrit le classeur dans un fichier (via un fichier temporaire).
func ExportODSFile(ctx context.Context, m *canvas.Model, path string) (clipped int, err error) {
	err = writeAtomic(path, func(w io.Writer) error {
		var e error
		clipped, e = ExportODS(ctx, m, w)
		return e
	})
	return clipped, err
}

const odsNS = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" xmlns:number="urn:oasis:names:tc:opendocument:xmlns:datastyle:1.0" xmlns:of="urn:oasis:names:tc:opendocument:xmlns:of:1.2" office:version="1.3"`

func writeODS(ctx context.Context, w io.Writer, sheets []xsheet, fm *fmts) error {
	zw := zip.NewWriter(w)
	// mimetype : première entrée, non compressée.
	mt, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		return err
	}
	if _, err := io.WriteString(mt, "application/vnd.oasis.opendocument.spreadsheet"); err != nil {
		return err
	}
	file := func(name, content string) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, content)
		return err
	}
	f, err := zw.Create("content.xml")
	if err != nil {
		return err
	}
	if err := odsContent(ctx, f, sheets, fm); err != nil {
		return err
	}
	var st strings.Builder
	fmt.Fprintf(&st, `<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles %s><office:styles><style:style style:name="Default" style:family="table-cell"/>`, odsNS)
	for n, x := range fm.all() {
		if n > 0 {
			st.WriteString(x.odsCellStyle(n))
		}
	}
	st.WriteString(`</office:styles><office:automatic-styles><style:page-layout style:name="pm1"/></office:automatic-styles><office:master-styles><style:master-page style:name="Default" style:page-layout-name="pm1"/></office:master-styles></office:document-styles>`)
	for _, f := range [][2]string{
		{"styles.xml", st.String()},
		{"META-INF/manifest.xml", `<?xml version="1.0" encoding="UTF-8"?>
<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.3"><manifest:file-entry manifest:full-path="/" manifest:version="1.3" manifest:media-type="application/vnd.oasis.opendocument.spreadsheet"/><manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/><manifest:file-entry manifest:full-path="styles.xml" manifest:media-type="text/xml"/></manifest:manifest>`},
	} {
		if err := file(f[0], f[1]); err != nil {
			return err
		}
	}
	return zw.Close()
}

// odsLength écrit une longueur en pixels (96 par pouce).
func odsLength(px int) string {
	return strconv.FormatFloat(math.Round(float64(px)/96*10000)/10000, 'f', -1, 64) + "in"
}

func odsContent(ctx context.Context, w io.Writer, sheets []xsheet, fm *fmts) error {
	bw := bufio.NewWriterSize(w, 64<<10)
	// Styles automatiques : feuilles (affichée, masquée), largeurs de
	// colonnes et hauteurs de lignes.
	widths, heights := map[int]bool{}, map[int]bool{}
	for _, s := range sheets {
		for _, c := range s.widths {
			widths[c.Width] = true
		}
		for _, h := range s.heights {
			heights[h] = true
		}
	}
	fmt.Fprintf(bw, `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content %s><office:automatic-styles>`, odsNS)
	bw.WriteString(`<style:style style:name="ta1" style:family="table" style:master-page-name="Default"><style:table-properties table:display="true" style:writing-mode="lr-tb"/></style:style>`)
	bw.WriteString(`<style:style style:name="ta2" style:family="table" style:master-page-name="Default"><style:table-properties table:display="false" style:writing-mode="lr-tb"/></style:style>`)
	for _, px := range slices.Sorted(maps.Keys(widths)) {
		fmt.Fprintf(bw, `<style:style style:name="co%d" style:family="table-column"><style:table-column-properties fo:break-before="auto" style:column-width="%s"/></style:style>`, px, odsLength(px))
	}
	for _, px := range slices.Sorted(maps.Keys(heights)) {
		fmt.Fprintf(bw, `<style:style style:name="ro%d" style:family="table-row"><style:table-row-properties style:row-height="%s" fo:break-before="auto" style:use-optimal-row-height="false"/></style:style>`, px, odsLength(px))
	}
	bw.WriteString(`</office:automatic-styles><office:body><office:spreadsheet>`)
	for i := range sheets {
		if err := odsSheet(ctx, bw, &sheets[i], fm); err != nil {
			return err
		}
	}
	// Les tableaux : plages nommées sur leurs données.
	var named strings.Builder
	for _, s := range sheets {
		for _, xt := range s.tables {
			p := xt.place
			if p == nil || len(p.cols) == 0 || p.row+1 >= ExcelRows || p.col+len(p.cols)-1 >= ExcelCols {
				continue
			}
			sheet := "$'" + strings.ReplaceAll(p.sheet, "'", "''") + "'"
			first := sheet + ".$" + colName(p.col) + "$" + strconv.Itoa(p.row+2)
			last := "$" + colName(p.col+len(p.cols)-1) + "$" + strconv.Itoa(min(p.lastRow(), ExcelRows-1)+1)
			fmt.Fprintf(&named, `<table:named-range table:name="%s" table:base-cell-address="%s" table:cell-range-address="%s:.%s"/>`, xmlEscape(p.name), xmlEscape(first), xmlEscape(first), last)
		}
	}
	if named.Len() > 0 {
		fmt.Fprintf(bw, `<table:named-expressions>%s</table:named-expressions>`, named.String())
	}
	bw.WriteString(`</office:spreadsheet></office:body></office:document-content>`)
	return bw.Flush()
}

func mapKeys[K comparable, V any](m map[K]V) func(func(K) bool) {
	return func(yield func(K) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

func odsSheet(ctx context.Context, bw *bufio.Writer, s *xsheet, fm *fmts) error {
	keys := s.sortedKeys()
	ta := "ta1"
	if s.hidden {
		ta = "ta2"
	}
	fmt.Fprintf(bw, `<table:table table:name="%s" table:style-name="%s">`, xmlEscape(s.name), ta)
	// Colonnes : jusqu'à la dernière utilisée ou mise en forme.
	maxCol := -1
	for _, k := range keys {
		maxCol = max(maxCol, k[1])
	}
	for _, xt := range s.tables {
		maxCol = max(maxCol, xt.info.Col+len(xt.info.Columns)-1)
	}
	widths := map[int]int{}
	for _, c := range s.widths {
		widths[c.Col] = c.Width
		maxCol = max(maxCol, c.Col)
	}
	for c := range s.colFmt {
		maxCol = max(maxCol, c)
	}
	for c := range s.hiddenC {
		maxCol = max(maxCol, c)
	}
	maxCol = min(maxCol, ExcelCols-1)
	colAttrs := func(c int) string {
		a := ""
		if w, ok := widths[c]; ok {
			a += fmt.Sprintf(` table:style-name="co%d"`, w)
		}
		if s.hiddenC[c] {
			a += ` table:visibility="collapse"`
		}
		def := "Default"
		if n := s.colFmt[c]; n != 0 {
			def = fmt.Sprintf("ce%d", n)
		}
		return a + ` table:default-cell-style-name="` + def + `"`
	}
	if maxCol < 0 {
		bw.WriteString(`<table:table-column table:default-cell-style-name="Default"/>`)
	}
	for c := 0; c <= maxCol; {
		a := colAttrs(c)
		n := 1
		for c+n <= maxCol && colAttrs(c+n) == a {
			n++
		}
		rep := ""
		if n > 1 {
			rep = fmt.Sprintf(` table:number-columns-repeated="%d"`, n)
		}
		fmt.Fprintf(bw, `<table:table-column%s%s/>`, a, rep)
		c += n
	}
	// Fusions : la cellule en haut à gauche porte l'étendue, les autres
	// sont couvertes.
	spans := map[[2]int][2]int{}
	covered := map[int]map[int]bool{}
	for _, g := range s.merges {
		spans[[2]int{g[0], g[1]}] = [2]int{g[2], g[3]}
		for r := g[0]; r < g[0]+g[2]; r++ {
			for c := g[1]; c < g[1]+g[3]; c++ {
				if r == g[0] && c == g[1] {
					continue
				}
				if covered[r] == nil {
					covered[r] = map[int]bool{}
				}
				covered[r][c] = true
			}
		}
	}
	next := 0 // prochaine ligne à écrire
	wrote := false
	err := mergeRows(ctx, s, keys, s.marks(), func(r int, cols []int, cells []xcell) error {
		if r > next {
			fmt.Fprintf(bw, `<table:table-row table:number-rows-repeated="%d"><table:table-cell/></table:table-row>`, r-next)
		}
		next = r + 1
		wrote = true
		bw.WriteString(`<table:table-row`)
		if h, ok := s.heights[r]; ok {
			fmt.Fprintf(bw, ` table:style-name="ro%d"`, h)
		}
		if s.hiddenR[r] {
			bw.WriteString(` table:visibility="collapse"`)
		}
		if n := s.rowFmt[r]; n != 0 {
			fmt.Fprintf(bw, ` table:default-cell-style-name="ce%d"`, n)
		}
		bw.WriteString(`>`)
		// Colonnes de la ligne : cellules, et fusions (début ou couverture).
		at := map[int]int{}
		pos := slices.Clone(cols)
		for i, c := range cols {
			at[c] = i
		}
		for c := range covered[r] {
			if _, ok := at[c]; !ok {
				pos = append(pos, c)
			}
		}
		for k := range spans {
			if _, ok := at[k[1]]; k[0] == r && !ok && !covered[r][k[1]] {
				pos = append(pos, k[1])
			}
		}
		slices.Sort(pos)
		pos = slices.Compact(pos)
		col := 0
		for _, c := range pos {
			if c > col {
				rep := ""
				if c-col > 1 {
					rep = fmt.Sprintf(` table:number-columns-repeated="%d"`, c-col)
				}
				fmt.Fprintf(bw, `<table:table-cell%s/>`, rep)
			}
			col = c + 1
			if covered[r][c] {
				bw.WriteString(`<table:covered-table-cell/>`)
				continue
			}
			var x xcell
			if i, ok := at[c]; ok {
				x = cells[i]
			}
			sp, isSpan := spans[[2]int{r, c}]
			odsCell(bw, x, fm, sp, isSpan, s.colFmt[c] != 0 || s.rowFmt[r] != 0)
		}
		if len(pos) == 0 {
			bw.WriteString(`<table:table-cell/>`)
		}
		bw.WriteString(`</table:table-row>`)
		return nil
	})
	if err != nil {
		return err
	}
	if !wrote {
		bw.WriteString(`<table:table-row><table:table-cell/></table:table-row>`)
	}
	bw.WriteString(`</table:table>`)
	return nil
}

// odsCell écrit une cellule ; sp : lignes et colonnes fusionnées (si span),
// styled : la ligne ou la colonne a un style (une cellule sans mise en forme
// doit alors le dire).
func odsCell(bw *bufio.Writer, x xcell, fm *fmts, sp [2]int, span, styled bool) {
	bw.WriteString(`<table:table-cell`)
	st := x.fm
	if x.kind == 'd' {
		st = fm.date(st, x.num)
	}
	if st != 0 {
		fmt.Fprintf(bw, ` table:style-name="ce%d"`, st)
	} else if styled {
		bw.WriteString(` table:style-name="Default"`)
	}
	if span {
		fmt.Fprintf(bw, ` table:number-rows-spanned="%d" table:number-columns-spanned="%d"`, sp[0], sp[1])
	}
	if x.f != "" {
		fmt.Fprintf(bw, ` table:formula="of:=%s"`, xmlEscape(x.f))
	}
	text := ""
	switch x.kind {
	case 'n':
		fmt.Fprintf(bw, ` office:value-type="float" office:value="%s"`, strconv.FormatFloat(x.num, 'g', -1, 64))
		text = printText(x)
	case 'd':
		t := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(time.Duration(math.Round(x.num*86400)) * time.Second)
		if serialKind(x.num) == 2 {
			fmt.Fprintf(bw, ` office:value-type="time" office:time-value="PT%02dH%02dM%02dS"`, t.Hour(), t.Minute(), t.Second())
		} else {
			fmt.Fprintf(bw, ` office:value-type="date" office:date-value="%s"`, t.Format("2006-01-02T15:04:05"))
		}
		text = printText(x)
	case 'b':
		fmt.Fprintf(bw, ` office:value-type="boolean" office:boolean-value="%t"`, x.num != 0)
		text = printText(x)
	case 'e':
		text = printText(x)
		fmt.Fprintf(bw, ` office:value-type="string" office:string-value="%s"`, xmlEscape(text))
	case 's':
		bw.WriteString(` office:value-type="string"`)
		text = x.str
	}
	if text == "" {
		bw.WriteString(`/>`)
		return
	}
	bw.WriteString(`>`)
	bw.WriteString(odsText(text))
	bw.WriteString(`</table:table-cell>`)
}

// odsText écrit un texte en paragraphes (un par ligne), espaces répétées et
// tabulations comprises.
func odsText(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		b.WriteString(`<text:p>`)
		rs := []rune(line)
		for i := 0; i < len(rs); i++ {
			switch {
			case rs[i] == '\t':
				b.WriteString(`<text:tab/>`)
			case rs[i] == ' ':
				// espaces : la première telle quelle (sauf en début de ligne),
				// les suivantes comptées
				j := i
				for j < len(rs) && rs[j] == ' ' {
					j++
				}
				n := j - i
				if i > 0 {
					b.WriteByte(' ')
					n--
				}
				if n > 0 {
					fmt.Fprintf(&b, `<text:s text:c="%d"/>`, n)
				}
				i = j - 1
			default:
				b.WriteString(xmlEscape(string(rs[i])))
			}
		}
		b.WriteString(`</text:p>`)
	}
	return b.String()
}
