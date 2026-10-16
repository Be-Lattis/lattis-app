package transfer

import (
	"context"
	"fmt"
	"strings"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/formula"
)

// MaxPrintRows borne les lignes d'une impression : au-delà, la page dit ce
// qui n'est pas imprimé (filtrez ou exportez en .xlsx).
const MaxPrintRows = 5000

// PrintHTML rend la plage utilisée d'une feuille en tableau HTML, pour
// l'impression et l'export PDF (fenêtre d'impression du système) : valeurs
// telles que la grille les affiche, en-têtes de tableaux en gras, largeurs de
// colonnes. Version 1 : sans la mise en forme ni les graphiques.
func PrintHTML(ctx context.Context, m *canvas.Model, sheet int64) (string, error) {
	l, err := m.Layout(ctx, sheet)
	if err != nil {
		return "", err
	}
	var info canvas.SheetInfo
	for _, s := range l.Sheets {
		if s.ID == sheet {
			info = s
		}
	}
	if info.ID == 0 {
		return "", fmt.Errorf("feuille %d introuvable", sheet)
	}
	sh, err := collectSheet(ctx, m, info)
	if err != nil {
		return "", err
	}
	return renderPrint(sh), nil
}

func renderPrint(sh xsheet) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<h1 class="print-title">%s</h1>`, xmlEscape(sh.name))
	if len(sh.cells) == 0 {
		b.WriteString(`<p class="print-empty">Feuille vide.</p>`)
		return b.String()
	}
	r0, c0, r1, c1 := -1, -1, -1, -1
	for k := range sh.cells {
		if r0 < 0 || k[0] < r0 {
			r0 = k[0]
		}
		if k[0] > r1 {
			r1 = k[0]
		}
		if c0 < 0 || k[1] < c0 {
			c0 = k[1]
		}
		if k[1] > c1 {
			c1 = k[1]
		}
	}
	last := min(r1, r0+MaxPrintRows-1)
	widths := map[int]int{}
	for _, w := range sh.widths {
		widths[w.Col] = w.Width
	}
	b.WriteString(`<table class="print-grid"><colgroup>`)
	for c := c0; c <= c1; c++ {
		w := widths[c]
		if w == 0 {
			w = 100
		}
		fmt.Fprintf(&b, `<col style="width:%dpx">`, w)
	}
	b.WriteString(`</colgroup><tbody>`)
	for r := r0; r <= last; r++ {
		b.WriteString(`<tr>`)
		for c := c0; c <= c1; c++ {
			x, ok := sh.cells[[2]int{r, c}]
			if !ok {
				b.WriteString(`<td></td>`)
				continue
			}
			var cls []string
			if x.bold {
				cls = append(cls, "b")
			}
			if x.kind == 'n' || x.kind == 'd' {
				cls = append(cls, "n")
			}
			if x.kind == 'b' || x.kind == 'e' {
				cls = append(cls, "c")
			}
			attr := ""
			if len(cls) > 0 {
				attr = ` class="` + strings.Join(cls, " ") + `"`
			}
			fmt.Fprintf(&b, `<td%s>%s</td>`, attr, xmlEscape(printText(x)))
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	if last < r1 {
		fmt.Fprintf(&b, `<p class="print-more">%d lignes suivantes non imprimées : filtrez la feuille ou exportez le classeur en .xlsx.</p>`, r1-last)
	}
	return b.String()
}

// printText écrit une cellule comme la grille l'affiche (locale française).
func printText(x xcell) string {
	switch x.kind {
	case 'n':
		return formula.FormatNumber(x.num)
	case 'd':
		return formula.FormatDate(x.num)
	case 'b':
		return formula.Boolean(x.num != 0).String()
	case 'e':
		for fr, en := range excelErrors {
			if en == x.str {
				return fr
			}
		}
	}
	return x.str
}
