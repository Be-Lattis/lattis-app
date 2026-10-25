package transfer

import (
	"encoding/json"
	"maps"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Mise en forme d'une feuille importée, assemblée au fil de la lecture :
// cellules libres, et pour chaque tableau ses en-têtes, ses colonnes et les
// cellules qui s'écartent de leur colonne. Une colonne de tableau reçoit le
// style le plus fréquent de ses lignes (une seule entrée pour 100 000
// lignes uniformes) ; seules les cellules d'un autre style sont écrites une
// à une. Notes, validations, fusions et règles conditionnelles passent par
// patch, qui trouve la bonne couche (cellule libre, en-tête ou cellule de
// tableau).

// xfRun : à partir de la ligne de données row, les cellules ont le style xf.
type xfRun struct{ row, xf int }

type colFmt struct {
	counts map[int]int
	runs   []xfRun
}

func (cf *colFmt) add(row, xf int) {
	if cf.counts == nil {
		cf.counts = map[int]int{}
	}
	cf.counts[xf]++
	if n := len(cf.runs); n == 0 || cf.runs[n-1].xf != xf {
		cf.runs = append(cf.runs, xfRun{row, xf})
	}
}

type tableBuild struct {
	is    *island
	tf    *canvas.TableFormat
	head  []int // style de l'en-tête (-1 : aucun)
	cols  []colFmt
	cells map[[2]int]int // (ligne de données, colonne) → rang dans tf.Cells
}

type sheetBuild struct {
	x      *xlsxFile
	f      *canvas.ImportFormat
	free   map[[2]int]int // cellule libre → rang dans f.Cells
	tables []*tableBuild
	colEnc map[int]string // style des colonnes de la feuille (encodé)
	rowSt  map[int]bool   // lignes qui ont un style
}

// maxFreeStyles borne les cellules libres mises en forme d'une feuille.
const maxFreeStyles = 200_000

func newSheetBuild(x *xlsxFile, f *canvas.ImportFormat) *sheetBuild {
	sb := &sheetBuild{x: x, f: f, free: map[[2]int]int{}, colEnc: map[int]string{}, rowSt: map[int]bool{}}
	for _, l := range f.ColStyles {
		sb.colEnc[l.Index] = encStyle(l.Style)
	}
	for _, l := range f.RowStyles {
		sb.rowSt[l.Index] = true
	}
	return sb
}

func encStyle(s canvas.Style) string {
	if len(s) == 0 {
		return ""
	}
	b, _ := json.Marshal(s)
	return string(b)
}

func (sb *sheetBuild) addTable(is *island) *tableBuild {
	w := is.c1 - is.c0 + 1
	tb := &tableBuild{is: is, tf: &canvas.TableFormat{Head: make([]canvas.Style, w), Cols: make([]canvas.Style, w)},
		head: make([]int, w), cols: make([]colFmt, w), cells: map[[2]int]int{}}
	for i := range tb.head {
		tb.head[i] = -1
	}
	sb.tables = append(sb.tables, tb)
	return tb
}

func (sb *sheetBuild) tableAt(r, c int) *tableBuild {
	for _, tb := range sb.tables {
		if tb.is.contains(r, c) {
			return tb
		}
	}
	return nil
}

// cellStyle : style d'une cellule libre (xf), sauf s'il vaut déjà celui de
// sa colonne.
func (sb *sheetBuild) freeXF(r, c, xf int) {
	st := sb.x.st.style(xf)
	if st == nil || len(sb.f.Cells) >= maxFreeStyles {
		return
	}
	if !sb.rowSt[r] && sb.colEnc[c] == sb.x.st.enc[xf] {
		return
	}
	sb.patch(r, c, st)
}

// patch ajoute des propriétés à une cellule (libre, en-tête ou cellule de
// tableau).
func (sb *sheetBuild) patch(r, c int, p canvas.Style) {
	if len(p) == 0 {
		return
	}
	if tb := sb.tableAt(r, c); tb != nil {
		j := c - tb.is.c0
		if r == tb.is.r0 {
			if tb.tf.Head[j] == nil {
				tb.tf.Head[j] = canvas.Style{}
			}
			maps.Copy(tb.tf.Head[j], p)
			return
		}
		k := [2]int{r - tb.is.r0 - 1, j}
		i, ok := tb.cells[k]
		if !ok {
			i = len(tb.tf.Cells)
			tb.cells[k] = i
			tb.tf.Cells = append(tb.tf.Cells, canvas.ImportStyle{Row: k[0], Col: j, Style: canvas.Style{}})
		}
		maps.Copy(tb.tf.Cells[i].Style, p)
		return
	}
	k := [2]int{r, c}
	i, ok := sb.free[k]
	if !ok {
		i = len(sb.f.Cells)
		sb.free[k] = i
		sb.f.Cells = append(sb.f.Cells, canvas.ImportStyle{Row: r, Col: c, Style: canvas.Style{}})
	}
	maps.Copy(sb.f.Cells[i].Style, p)
}

// patchColumn ajoute des propriétés à toute une colonne de tableau.
func (sb *sheetBuild) patchColumn(tb *tableBuild, j int, p canvas.Style) {
	if tb.tf.Cols[j] == nil {
		tb.tf.Cols[j] = canvas.Style{}
	}
	maps.Copy(tb.tf.Cols[j], p)
}

// cell note le style d'une cellule lue (valeur ou non).
func (sb *sheetBuild) cell(tb *tableBuild, r, c, xf int) {
	if tb == nil {
		sb.freeXF(r, c, xf)
		return
	}
	j := c - tb.is.c0
	if r == tb.is.r0 {
		tb.head[j] = xf
		return
	}
	tb.cols[j].add(r-tb.is.r0-1, xf)
}

// finish calcule les styles des tableaux : en-têtes, colonnes (style le plus
// fréquent), cellules qui s'en écartent. Les propriétés déjà posées par
// patch (notes, liens…) restent.
func (sb *sheetBuild) finish() {
	st := sb.x.st
	if st == nil {
		return
	}
	for _, tb := range sb.tables {
		rows := tb.is.r1 - tb.is.r0
		patched := tb.tf.Cells
		tb.tf.Cells, tb.cells = nil, map[[2]int]int{}
		for j := range tb.cols {
			if s := st.style(tb.head[j]); s != nil {
				h := maps.Clone(s)
				maps.Copy(h, tb.tf.Head[j])
				tb.tf.Head[j] = h
			}
			cf := &tb.cols[j]
			mode, best := 0, -1
			for xf, n := range cf.counts {
				if n > best || n == best && xf < mode {
					mode, best = xf, n
				}
			}
			base := st.style(mode)
			if base != nil {
				c := maps.Clone(base)
				maps.Copy(c, tb.tf.Cols[j]) // validation de la colonne (patchColumn)
				tb.tf.Cols[j] = c
			}
			diffs := map[int]canvas.Style{}
			for k, run := range cf.runs {
				if run.xf == mode {
					continue
				}
				d, ok := diffs[run.xf]
				if !ok {
					d = styleDiff(st.style(run.xf), base)
					diffs[run.xf] = d
				}
				if len(d) == 0 {
					continue
				}
				end := rows
				if k+1 < len(cf.runs) {
					end = cf.runs[k+1].row
				}
				for r := run.row; r < end; r++ {
					tb.cells[[2]int{r, j}] = len(tb.tf.Cells)
					tb.tf.Cells = append(tb.tf.Cells, canvas.ImportStyle{Row: r, Col: j, Style: maps.Clone(d)})
				}
			}
		}
		for _, p := range patched {
			if i, ok := tb.cells[[2]int{p.Row, p.Col}]; ok {
				maps.Copy(tb.tf.Cells[i].Style, p.Style)
			} else {
				tb.cells[[2]int{p.Row, p.Col}] = len(tb.tf.Cells)
				tb.tf.Cells = append(tb.tf.Cells, p)
			}
		}
		tb.cols = nil
	}
}

// styleDiff : ce qui fait passer de base à s (propriétés à null pour celles
// que s n'a pas).
func styleDiff(s, base canvas.Style) canvas.Style {
	out := canvas.Style{}
	for k, v := range s {
		if bv, ok := base[k]; !ok || encStyle(canvas.Style{k: bv}) != encStyle(canvas.Style{k: v}) {
			out[k] = v
		}
	}
	for k := range base {
		if _, ok := s[k]; !ok {
			out[k] = nil
		}
	}
	return out
}

// merges pose les fusions sur les cellules libres (une fusion qui touche un
// tableau est passée).
func (sb *sheetBuild) merges(ms []rect) {
	for _, m := range ms {
		rows, cols := m.r1-m.r0+1, m.c1-m.c0+1
		if rows*cols <= 1 || rows*cols > canvas.MaxMerge {
			continue
		}
		touch := false
		for _, tb := range sb.tables {
			if !(m.r1 < tb.is.r0 || m.r0 > tb.is.r1 || m.c1 < tb.is.c0 || m.c0 > tb.is.c1) {
				touch = true
			}
		}
		if !touch {
			sb.patch(m.r0, m.c0, canvas.Style{"mr": float64(rows), "mc": float64(cols)})
		}
	}
}

// sheetKey : nom de feuille comparable (minuscules, sans guillemets).
func sheetKey(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		s = strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}
	return strings.ToLower(s)
}
