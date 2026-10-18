package canvas

import "strings"

// blockRect est une zone d'un bloc collé (bornes comprises).
type blockRect struct{ r0, r1, c0, c1 int }

// blockIslands découpe une zone d'un bloc aux lignes puis aux colonnes
// entièrement vides, jusqu'à ne plus pouvoir couper (règle de Lattis : une
// ligne ou une colonne vide termine un tableau). Les îlots viennent de haut
// en bas, puis de gauche à droite.
func blockIslands(cell func(r, c int) string, z blockRect) []blockRect {
	bands := func(lo, hi int, filled func(int) bool) [][2]int {
		var out [][2]int
		start := -1
		for i := lo; i <= hi+1; i++ {
			if i <= hi && filled(i) {
				if start < 0 {
					start = i
				}
				continue
			}
			if start >= 0 {
				out = append(out, [2]int{start, i - 1})
				start = -1
			}
		}
		return out
	}
	full := func(s string) bool { return strings.TrimSpace(s) != "" }
	var out []blockRect
	for _, rb := range bands(z.r0, z.r1, func(r int) bool {
		for c := z.c0; c <= z.c1; c++ {
			if full(cell(r, c)) {
				return true
			}
		}
		return false
	}) {
		for _, cb := range bands(z.c0, z.c1, func(c int) bool {
			for r := rb[0]; r <= rb[1]; r++ {
				if full(cell(r, c)) {
					return true
				}
			}
			return false
		}) {
			sub := blockRect{rb[0], rb[1], cb[0], cb[1]}
			if sub == z {
				out = append(out, sub)
				continue
			}
			out = append(out, blockIslands(cell, sub)...)
		}
	}
	return out
}

// pasteIslands colle un bloc dans une zone vide, îlot par îlot. Dans chaque
// îlot, les titres du haut (titleRows) et les notes ou totaux du bas
// (footRows) restent des cellules libres, ces derniers gardés hors du
// tableau ; le reste devient un tableau s'il a au moins MinRows lignes. Un
// îlot sans en-tête juste sous un tableau du même bloc, de mêmes colonnes,
// le continue (ligne vide au milieu des données).
func (t *tx) pasteIslands(sheet int64, row, col int, block [][]string, w int, ev *Event) error {
	cell := func(r, c int) string {
		if r < len(block) && c < len(block[r]) {
			return block[r][c]
		}
		return ""
	}
	type plan struct {
		r0, c0, c1, last int // en-tête (bloc), colonnes, dernière ligne de données
		headers          []string
		data             [][]string
		headless         bool
	}
	type loose struct {
		r, c      int
		raw, kind string
	}
	var plans []*plan
	var frees []loose
	for _, z := range blockIslands(cell, blockRect{0, len(block) - 1, 0, w - 1}) {
		width := z.c1 - z.c0 + 1
		g := make([][]string, z.r1-z.r0+1)
		for i := range g {
			g[i] = make([]string, width)
			for j := range g[i] {
				g[i][j] = cell(z.r0+i, z.c0+j)
			}
		}
		split := func(titles int) (body [][]string, nfoot int, kinds []string) {
			nfoot, kinds = footRows(g[titles:])
			return g[titles : len(g)-nfoot], nfoot, kinds
		}
		titles := titleRows(g)
		body, nfoot, kinds := split(titles)
		head := hasHeader(body, width)
		if titles > 0 && !head {
			// Sans en-tête, l'en-tête irait sur la ligne du titre.
			titles = 0
			body, nfoot, kinds = split(0)
			head = hasHeader(body, width)
		}
		if len(body) < MinRows {
			for i := range g {
				for j, v := range g[i] {
					if strings.TrimSpace(v) != "" {
						frees = append(frees, loose{z.r0 + i, z.c0 + j, v, ""})
					}
				}
			}
			continue
		}
		for i := range titles {
			for j, v := range g[i] {
				if strings.TrimSpace(v) != "" {
					frees = append(frees, loose{z.r0 + i, z.c0 + j, v, ""})
				}
			}
		}
		for k := range nfoot {
			i := len(g) - 1 - k
			for j, v := range g[i] {
				if strings.TrimSpace(v) != "" {
					frees = append(frees, loose{z.r0 + i, z.c0 + j, v, kinds[k]})
				}
			}
		}
		top := z.r0 + titles
		last := top + len(body) - 1
		if !head && len(plans) > 0 {
			if p := plans[len(plans)-1]; p.c0 == z.c0 && p.c1 == z.c1 && p.last+2 == top {
				p.data = append(p.data, body...)
				p.last = last
				continue
			}
		}
		p := &plan{r0: top, c0: z.c0, c1: z.c1, last: last}
		if head {
			p.headers = make([]string, width)
			for j, v := range body[0] {
				p.headers[j] = strings.TrimSpace(v)
			}
			p.data = body[1:]
		} else {
			// Les colonnes s'appellent « Colonne n » et l'en-tête se place
			// au-dessus des données, qui ne bougent pas.
			p.headers = make([]string, width)
			for j := range p.headers {
				p.headers[j] = "Colonne " + itoa(j+1)
			}
			p.data, p.headless = body, true
			p.r0--
		}
		plans = append(plans, p)
	}
	for _, p := range plans {
		r := max(row+p.r0, 0)
		name, err := t.tableName(sheet, r, col+p.c0)
		if err != nil {
			return err
		}
		if _, err := t.createTable(sheet, name, r, col+p.c0, p.headers, p.data); err != nil {
			return err
		}
		if ev.Created == "" {
			ev.Created = name
		}
		ev.Headless = ev.Headless || p.headless
	}
	for _, f := range frees {
		if err := t.putFree(sheet, row+f.r, col+f.c, f.raw); err != nil {
			return err
		}
		if f.kind != "" {
			if err := t.markOut(sheet, row+f.r, col+f.c, f.kind); err != nil {
				return err
			}
		}
	}
	return nil
}
