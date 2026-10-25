package transfer

import (
	"cmp"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// importCounts : objets repris, et par feuille règles et validations qui ne
// l'ont pas été.
type importCounts struct {
	charts, pivots, notes int
	rules, lists          map[string]int
}

func (c *importCounts) lose(m *map[string]int, sheet string, n int) {
	if n <= 0 {
		return
	}
	if *m == nil {
		*m = map[string]int{}
	}
	(*m)[sheet] += n
}

// visibleSheet garde au moins une feuille visible.
func visibleSheet(sheets []canvas.ImportSheet) {
	for _, s := range sheets {
		if s.Format == nil || !s.Format.Hidden {
			return
		}
	}
	if len(sheets) > 0 {
		sheets[0].Format.Hidden = false
	}
}

// excelTables lit les tableaux d'Excel d'une feuille : nom par plage.
func (x *xlsxFile) excelTables(sh xlsxSheet) map[rect]string {
	out := map[rect]string{}
	if sh.file == nil {
		return out
	}
	byID, _, err := x.rels(sh.file.Name)
	if err != nil {
		return out
	}
	for _, p := range byID {
		if !strings.Contains(strings.ToLower(p), "tables/table") {
			continue
		}
		f := x.find(p)
		if f == nil {
			continue
		}
		var tb struct {
			Name    string `xml:"name,attr"`
			Display string `xml:"displayName,attr"`
			Ref     string `xml:"ref,attr"`
		}
		if x.decode(f, &tb) != nil {
			continue
		}
		if r, ok := rangeRect(tb.Ref); ok {
			out[r] = cmp.Or(tb.Display, tb.Name)
		}
	}
	return out
}

// decorate complète la mise en forme d'une feuille une fois ses tableaux
// connus : fusions, mise en forme conditionnelle (xlsxcf.go), validations
// (xlsxdv.go), notes (xlsxnotes.go) ; rend les avertissements.
func (x *xlsxFile) decorate(sh xlsxSheet, sb *sheetBuild, rv *resolver) []string {
	var warn []string
	if sh.ext != nil {
		sb.merges(sh.ext.merges)
		x.placeRules(sh, sb)
		warn = append(warn, x.placeValidations(sh, sb, rv)...)
		x.placeNotes(sh, sb)
	}
	sb.finish()
	return warn
}
