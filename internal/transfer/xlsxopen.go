package transfer

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// OpenXLSX importe un fichier .xlsx ouvert dans Lattis, qui s'enregistrera
// dans ce même fichier (classeur adossé au .xlsx) : comme ImportXLSX ; en
// plus, les tableaux tirés de plages ordinaires (pas des tableaux d'Excel)
// et le style des tableaux d'Excel sont notés pour être réécrits tels quels
// (canvas.XLSXMeta). Une petite plage dont la première ligne porte des
// nombres reste en cellules libres (xlsxheads.go).
func OpenXLSX(ctx context.Context, m *canvas.Model, path string) (canvas.ImportResult, error) {
	x, err := openXLSX(path)
	if err != nil {
		return canvas.ImportResult{}, err
	}
	defer x.Close()
	x.keep = true
	res, err := x.importBook(ctx, m, path)
	if err != nil || x.rv == nil {
		return res, err
	}
	plain := map[string]bool{}
	styles := map[string]string{}
	for _, sh := range x.sheets {
		var byRect map[rect]string
		for _, is := range x.rv.bySheet[strings.ToLower(sh.name)] {
			switch {
			case !is.table():
			case is.xname == "":
				plain[strings.ToLower(is.name)] = true
			default:
				if byRect == nil {
					byRect = x.tableStyles(sh)
				}
				if s := byRect[is.rect]; s != "" {
					styles[strings.ToLower(is.name)] = s
				}
			}
		}
	}
	meta := canvas.XLSXMeta{Styles: map[int64]string{}}
	for _, sid := range res.Sheets {
		l, err := m.Layout(ctx, sid)
		if err != nil {
			return res, err
		}
		for _, tb := range l.Tables {
			low := strings.ToLower(tb.Name)
			if plain[low] {
				meta.Ranges = append(meta.Ranges, tb.ID)
			}
			if s := styles[low]; s != "" {
				meta.Styles[tb.ID] = s
			}
		}
	}
	return res, m.SetXLSXMeta(ctx, meta)
}

// tableStyles lit le style des tableaux d'Excel d'une feuille (attributs
// de tableStyleInfo, réécrits tels quels), par plage.
func (x *xlsxFile) tableStyles(sh xlsxSheet) map[rect]string {
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
			Ref   string `xml:"ref,attr"`
			Style *struct {
				Name   string `xml:"name,attr"`
				First  string `xml:"showFirstColumn,attr"`
				Last   string `xml:"showLastColumn,attr"`
				Rows   string `xml:"showRowStripes,attr"`
				Colums string `xml:"showColumnStripes,attr"`
			} `xml:"tableStyleInfo"`
		}
		if x.decode(f, &tb) != nil || tb.Style == nil || tb.Style.Name == "" {
			continue
		}
		flag := func(v, def string) string {
			if v == "" {
				return def
			}
			if v == "true" || v == "1" {
				return "1"
			}
			return "0"
		}
		s := tb.Style
		if r, ok := rangeRect(tb.Ref); ok {
			out[r] = fmt.Sprintf(`name="%s" showFirstColumn="%s" showLastColumn="%s" showRowStripes="%s" showColumnStripes="%s"`,
				xmlEscape(s.Name), flag(s.First, "0"), flag(s.Last, "0"), flag(s.Rows, "0"), flag(s.Colums, "0"))
		}
	}
	return out
}

// tableStyleAttrs : attributs de tableStyleInfo notés par tableStyles (lus
// dans le classeur : vérifiés avant d'être écrits dans le fichier).
var tableStyleAttrs = regexp.MustCompile(`^name="[^"<>&]{1,255}"( show(FirstColumn|LastColumn|RowStripes|ColumnStripes)="[01]")*$`)
