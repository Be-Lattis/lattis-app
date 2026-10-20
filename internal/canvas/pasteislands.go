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

// islandPlan : un tableau à créer depuis un îlot d'un bloc collé (lignes
// relatives au bloc).
type islandPlan struct {
	r0, c0, c1, last int // en-tête, colonnes, dernière ligne de données
	headers          []string
	data             [][]string
	headless         bool // en-tête « Colonne n »
	unnamed          bool // … et une 1re ligne qui a du texte (« Pays | 2024 »)
}

// looseCell : une cellule d'un bloc collé qui reste libre (kind : note ou
// total gardés hors du tableau).
type looseCell struct {
	r, c      int
	raw, kind string
}

// pasteIslands colle un bloc dans une zone vide, îlot par îlot (planIslands).
//
// En-tête : la même règle qu'à la saisie (décision de Louis du 21/10/2026,
// « même règle partout » ; textRow) : la 1re ligne d'un îlot est l'en-tête si
// toutes ses cellules remplies sont des textes ; sinon toutes ses lignes sont
// des données, sous un en-tête « Colonne n » posé au-dessus. Sans place libre
// au-dessus (collage en ligne 1, sous la zone grisée d'un tableau…), un
// collage ne perd pas son tableau, contrairement à la saisie : une ligne est
// insérée dans le bloc juste avant l'îlot, l'en-tête « Colonne n » prend la
// place de sa 1re ligne et le reste du bloc descend d'une ligne. S'il n'y a
// pas de place libre sous le bloc pour cela, l'îlot reste en cellules libres,
// comme à la saisie.
func (t *tx) pasteIslands(sheet int64, row, col int, block [][]string, w int, ev *Event) error {
	plans, frees, grown, err := t.planIslands(sheet, row, col, block, w, true)
	if err != nil {
		return err
	}
	if len(grown) > len(block) {
		// La zone vide vérifiée par Paste va jusqu'à la ligne sous le bloc ;
		// les lignes ajoutées et la nouvelle ligne du dessous doivent l'être
		// aussi.
		h := len(block)
		clear, err := t.areaClear(sheet, row+h+1, col-1, row+len(grown), col+w)
		if err != nil {
			return err
		}
		if !clear {
			if plans, frees, _, err = t.planIslands(sheet, row, col, block, w, false); err != nil {
				return err
			}
		}
	}
	for _, p := range plans {
		r := row + p.r0
		name, err := t.tableName(sheet, r, col+p.c0)
		if err != nil {
			return err
		}
		tb, err := t.createTable(sheet, name, r, col+p.c0, p.headers, p.data)
		if err != nil {
			return err
		}
		t.noteBorn(tb)
		if err := t.autoKey(tb.ID); err != nil {
			return err
		}
		if t.autoKeyed != "" && ev.AutoKey == "" {
			ev.AutoKey = t.autoKeyed
		}
		if ev.Created == "" {
			ev.Created = name
		}
		ev.Headless = ev.Headless || p.headless
		ev.Unnamed = ev.Unnamed || p.unnamed
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

// planIslands découpe le bloc en îlots (blockIslands). Dans chaque îlot, les
// titres du haut (titleRows) et les notes ou totaux du bas (footRows)
// restent des cellules libres, ces derniers gardés hors du tableau ; le
// reste devient un tableau s'il a au moins MinRows lignes, sa 1re ligne
// donnant l'en-tête si elle n'a que des textes (textRow). Un îlot sans
// en-tête juste sous un tableau du même bloc, de mêmes colonnes, le
// continue (ligne vide au milieu des données). insert : un îlot sans
// en-tête ni place au-dessus reçoit une ligne insérée avant lui dans le bloc
// (voir pasteIslands) ; rend aussi le bloc, agrandi de ces lignes.
func (t *tx) planIslands(sheet int64, row, col int, block [][]string, w int, insert bool) ([]*islandPlan, []looseCell, [][]string, error) {
	for {
		cell := func(r, c int) string {
			if r >= 0 && r < len(block) && c < len(block[r]) {
				return block[r][c]
			}
			return ""
		}
		var plans []*islandPlan
		var frees []looseCell
		keep := func(i0, i1, c0, c1 int, kind string) {
			for i := i0; i <= i1; i++ {
				for j := c0; j <= c1; j++ {
					if v := cell(i, j); strings.TrimSpace(v) != "" {
						frees = append(frees, looseCell{i, j, v, kind})
					}
				}
			}
		}
		// room : la ligne hr peut porter l'en-tête « Colonne n » des colonnes
		// c0..c1, comme headRoom à la saisie : vide dans le bloc (une colonne
		// de part et d'autre), hors de la zone grisée d'un tableau du bloc ;
		// au-dessus du bloc, libre sur la feuille.
		room := func(hr, c0, c1 int) (bool, error) {
			if hr < 0 {
				return t.headRoom(sheet, row+hr+1, col+c0, col+c1)
			}
			for c := max(c0-1, 0); c <= min(c1+1, w-1); c++ {
				if strings.TrimSpace(cell(hr, c)) != "" {
					return false, nil
				}
			}
			for _, p := range plans {
				if p.last+1 == hr && p.c0 <= c1+1 && p.c1+1 >= c0-1 {
					return false, nil
				}
			}
			return true, nil
		}
		emptyRow := func(r int) bool {
			for c := range w {
				if strings.TrimSpace(cell(r, c)) != "" {
					return false
				}
			}
			return true
		}
		again := false
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
			head := len(body) > 0 && textRow(body[0])
			if titles > 0 && !head {
				// Sans en-tête, l'en-tête irait sur la ligne du titre.
				titles = 0
				body, nfoot, kinds = split(0)
				head = len(body) > 0 && textRow(body[0])
			}
			if len(body) < MinRows {
				keep(z.r0, z.r1, z.c0, z.c1, "")
				continue
			}
			keep(z.r0, z.r0+titles-1, z.c0, z.c1, "")
			for k := range nfoot {
				i := z.r1 - k
				keep(i, i, z.c0, z.c1, kinds[k])
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
			p := &islandPlan{r0: top, c0: z.c0, c1: z.c1, last: last}
			if head {
				p.headers = make([]string, width)
				for j := range p.headers {
					p.headers[j] = strings.TrimSpace(body[0][j])
					if p.headers[j] == "" {
						p.headers[j] = "Colonne " + itoa(j+1) // coin vide
					}
				}
				p.data = body[1:]
			} else {
				ok, err := room(top-1, z.c0, z.c1)
				if err != nil {
					return nil, nil, nil, err
				}
				if !ok {
					// Une ligne insérée avant l'îlot, sur toute la largeur du
					// bloc (seulement entre deux bandes du bloc, pour ne
					// couper aucun îlot voisin) ; puis tout est relu.
					if insert && (top == 0 || emptyRow(top-1)) {
						block = append(block[:top:top], append([][]string{nil}, block[top:]...)...)
						again = true
						break
					}
					// Comme à la saisie : les cellules restent libres.
					keep(top, last, z.c0, z.c1, "")
					continue
				}
				// Les colonnes s'appellent « Colonne n » et l'en-tête se place
				// au-dessus des données, qui ne bougent pas.
				p.headers = autoHeaders(width)
				p.data, p.headless, p.unnamed = body, true, typedHeader(body)
				p.r0--
			}
			plans = append(plans, p)
		}
		if !again {
			return plans, frees, block, nil
		}
	}
}
