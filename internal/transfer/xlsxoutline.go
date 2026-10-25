package transfer

import (
	"bufio"
	"fmt"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Plan d'une feuille dans un fichier .xlsx (canvas/outline.go) : niveau
// de chaque ligne et colonne groupée (outlineLevel), ligne de synthèse
// d'un groupe réduit (collapsed), lignes masquées (hidden, posé avec le
// masquage), synthèse au-dessus ou à gauche (outlinePr).

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// outlineCollapsed donne les lignes de synthèse des groupes réduits.
func outlineCollapsed(lines []canvas.OutlineLine, before bool) map[int]bool {
	out := map[int]bool{}
	for _, g := range canvas.OutlineGroups(lines, before) {
		if !g.Folded {
			continue
		}
		if before {
			if g.From > 0 {
				out[g.From-1] = true
			}
		} else {
			out[g.To+1] = true
		}
	}
	return out
}

// outlineAttrs donne le niveau des lignes (rows) ou des colonnes et leurs
// lignes de synthèse réduites.
func outlineAttrs(o *canvas.Outline, rows bool) (map[int]int, map[int]bool) {
	if o == nil {
		return nil, nil
	}
	lines, before := o.Rows, o.RowsAbove
	if !rows {
		lines, before = o.Cols, o.ColsLeft
	}
	levels := map[int]int{}
	for _, l := range lines {
		levels[l.Index] = l.Level
	}
	return levels, outlineCollapsed(lines, before)
}

// writeOutlineFormat écrit sheetFormatPr avec le plus grand niveau des
// lignes et des colonnes (rien sans plan).
func writeOutlineFormat(bw *bufio.Writer, o *canvas.Outline) {
	if o == nil {
		return
	}
	deep := func(ls []canvas.OutlineLine) int {
		n := 0
		for _, l := range ls {
			n = max(n, l.Level)
		}
		return n
	}
	fmt.Fprintf(bw, `<sheetFormatPr defaultRowHeight="15" outlineLevelRow="%d" outlineLevelCol="%d"/>`, deep(o.Rows), deep(o.Cols))
}
