package transfer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Graphiques sparkline d'une feuille dans un fichier .xlsx : extension x14
// (x14:sparklineGroups dans extLst, dernier enfant de la feuille). Un groupe
// tiré d'un tableau devient une sparkline par ligne de données (colonnes
// sources contiguës seulement, comme le demande le format) ; un groupe de
// cellules libres, une par cellule de son emplacement.

// plainSheet : nom de feuille écrit sans apostrophes dans une référence.
var plainSheet = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_.]*$`)

// maxSparkRows : lignes d'un tableau exportées en sparklines au plus.
const maxSparkRows = 100_000

// sparklinesXML écrit les groupes de la feuille ("" : aucun).
func sparklinesXML(s *xsheet) string {
	if len(s.sparks) == 0 {
		return ""
	}
	sheet := s.name
	if !plainSheet.MatchString(sheet) {
		sheet = quoteSheet(sheet)
	}
	ref := func(r0, c0, r1, c1 int) string {
		a := colName(c0) + strconv.Itoa(r0+1)
		if r0 == r1 && c0 == c1 {
			return a
		}
		return a + ":" + colName(c1) + strconv.Itoa(r1+1)
	}
	var b strings.Builder
	for _, g := range s.sparks {
		if g.Error != "" {
			continue
		}
		var lines []string
		if g.Config.Table != "" {
			var tb *canvas.TableInfo
			for _, xt := range s.tables {
				if xt.info.Name == g.Config.Table {
					tb = &xt.info
				}
			}
			if tb == nil || len(g.Src) == 0 {
				continue
			}
			lo, hi := g.Src[0], g.Src[len(g.Src)-1]
			if hi-lo+1 != len(g.Src) {
				continue // colonnes non contiguës : pas d'équivalent
			}
			for r := tb.Row + 1; r <= tb.Row+min(tb.Total, maxSparkRows); r++ {
				if r >= ExcelRows || g.AtCol >= ExcelCols {
					break
				}
				lines = append(lines, sparkLine(sheet+"!"+ref(r, tb.Col+lo, r, tb.Col+hi), ref(r, g.AtCol, r, g.AtCol)))
			}
		} else {
			for _, c := range g.Cells {
				if c.Row >= ExcelRows || c.Col >= ExcelCols {
					continue
				}
				src := strings.ToUpper(g.Config.Source)
				// Source par ligne ou par colonne : la part de cette cellule.
				lines = append(lines, sparkLine(sheet+"!"+sparkPart(src, len(g.Cells), c, g.Cells[0]), ref(c.Row, c.Col, c.Row, c.Col)))
			}
		}
		if len(lines) == 0 {
			continue
		}
		b.WriteString(sparkGroupXML(g.Config, lines))
	}
	if b.Len() == 0 {
		return ""
	}
	return `<extLst><ext uri="{05C60535-1F16-4fd2-B633-F4F36F0B64E0}" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main">` +
		`<x14:sparklineGroups xmlns:xm="http://schemas.microsoft.com/office/excel/2006/main">` + b.String() + `</x14:sparklineGroups></ext></extLst>`
}

func sparkLine(f, sqref string) string {
	return `<x14:sparkline><xm:f>` + xmlEscape(f) + `</xm:f><xm:sqref>` + sqref + `</xm:sqref></x14:sparkline>`
}

// sparkPart : plage source du k-ième graphique d'un groupe de cellules
// libres (src « B2:M5 ») ; first : la première cellule de l'emplacement.
func sparkPart(src string, n int, c, first canvas.SparkCell) string {
	a, b, ok := strings.Cut(src, ":")
	if !ok {
		return src
	}
	r0, c0 := a1Cell(a)
	r1, c1 := a1Cell(b)
	k := (c.Row - first.Row) + (c.Col - first.Col)
	cell := func(r, col int) string { return colName(col) + strconv.Itoa(r+1) }
	if r1-r0+1 == n && !(n == 1 && r1 > r0) {
		return cell(r0+k, c0) + ":" + cell(r0+k, c1) // une ligne par graphique
	}
	return cell(r0, c0+k) + ":" + cell(r1, c0+k)
}

// a1Cell lit « B12 » (ligne et colonne de 0).
func a1Cell(s string) (int, int) {
	col, i := 0, 0
	for ; i < len(s) && s[i] >= 'A' && s[i] <= 'Z'; i++ {
		col = col*26 + int(s[i]-'A'+1)
	}
	r, _ := strconv.Atoi(s[i:])
	return r - 1, col - 1
}

func sparkGroupXML(c canvas.SparklineConfig, lines []string) string {
	attr := ""
	switch c.Kind {
	case "column":
		attr += ` type="column"`
	case "winloss":
		attr += ` type="stacked"`
	}
	w := c.Weight
	if w == 0 {
		w = 1.5
	}
	attr += fmt.Sprintf(` lineWeight="%s" displayEmptyCellsAs="gap"`, strconv.FormatFloat(w*0.75, 'f', -1, 64))
	for _, f := range []struct {
		on   bool
		name string
	}{{c.Markers, "markers"}, {c.High, "high"}, {c.Low, "low"}, {c.First, "first"}, {c.Last, "last"}, {c.Negative, "negative"}, {c.Axis, "displayXAxis"}} {
		if f.on {
			attr += ` ` + f.name + `="1"`
		}
	}
	for _, a := range []struct {
		v    *float64
		side string
	}{{c.Min, "min"}, {c.Max, "max"}} {
		switch {
		case a.v != nil:
			attr += ` ` + a.side + `AxisType="custom" manual` + strings.ToUpper(a.side[:1]) + a.side[1:] + `="` + strconv.FormatFloat(*a.v, 'g', -1, 64) + `"`
		case c.Same:
			attr += ` ` + a.side + `AxisType="group"`
		}
	}
	col := func(tag, v, def string) string {
		if v == "" {
			v = def
		}
		return `<x14:` + tag + ` rgb="FF` + strings.ToUpper(strings.TrimPrefix(v, "#")) + `"/>`
	}
	return `<x14:sparklineGroup` + attr + `>` +
		col("colorSeries", c.Color, "#376092") + col("colorNegative", c.NegColor, "#D00000") + col("colorAxis", "", "#000000") +
		col("colorMarkers", c.MarkerColor, "#376092") + col("colorFirst", c.FirstColor, "#E07000") + col("colorLast", c.LastColor, "#E07000") +
		col("colorHigh", c.HighColor, "#1E8E3E") + col("colorLow", c.LowColor, "#D00000") +
		`<x14:sparklines>` + strings.Join(lines, "") + `</x14:sparklines></x14:sparklineGroup>`
}
