package transfer

import "slices"

// rect est un rectangle de cellules [r0, r1] × [c0, c1] (bornes comprises).
type rect struct{ r0, c0, r1, c1 int }

func (a rect) contains(r, c int) bool { return r >= a.r0 && r <= a.r1 && c >= a.c0 && c <= a.c1 }

// near indique si deux rectangles se recouvrent ou se touchent, même par un
// coin : il faut au moins une ligne ou une colonne vide entre deux îlots.
func (a rect) near(b rect) bool {
	return a.r0 <= b.r1+1 && b.r0 <= a.r1+1 && a.c0 <= b.c1+1 && b.c0 <= a.c1+1
}

func (a rect) union(b rect) rect {
	return rect{min(a.r0, b.r0), min(a.c0, b.c0), max(a.r1, b.r1), max(a.c1, b.c1)}
}

// islands découpe une feuille en îlots, selon la règle de Lattis : une
// ligne ou une colonne vide termine un tableau. Les cellules occupées sont
// reçues ligne par ligne (cols croissantes), en plages contiguës ; les plages
// qui se touchent forment un îlot, et des îlots dont les rectangles se
// touchent ou se recouvrent sont fusionnés : chaque îlot est un rectangle
// entouré de vide.
type islandBuilder struct {
	runs   []run
	parent []int
	prev   []int // plages de la ligne précédente
	cur    []int
	row    int
}

type run struct{ row, c0, c1 int }

func newIslands() *islandBuilder { return &islandBuilder{row: -2} }

func (b *islandBuilder) find(i int) int {
	for b.parent[i] != i {
		b.parent[i] = b.parent[b.parent[i]]
		i = b.parent[i]
	}
	return i
}

// add enregistre une cellule occupée ; les appels suivent l'ordre du
// fichier (lignes puis colonnes croissantes).
func (b *islandBuilder) add(row, col int) {
	if row != b.row {
		if row == b.row+1 {
			b.prev = b.cur
		} else {
			b.prev = nil
		}
		b.cur = nil
		b.row = row
	}
	if n := len(b.cur); n > 0 {
		last := &b.runs[b.cur[n-1]]
		if col == last.c1+1 {
			last.c1 = col
			b.link(b.cur[n-1])
			return
		}
		if col <= last.c1 {
			return // cellule déjà vue (fichier désordonné)
		}
	}
	b.runs = append(b.runs, run{row, col, col})
	i := len(b.runs) - 1
	b.parent = append(b.parent, i)
	b.cur = append(b.cur, i)
	b.link(i)
}

// link relie une plage à celles de la ligne précédente qui la recouvrent.
func (b *islandBuilder) link(i int) {
	r := b.runs[i]
	for _, j := range b.prev {
		p := b.runs[j]
		if p.c0 <= r.c1 && r.c0 <= p.c1 {
			if a, c := b.find(i), b.find(j); a != c {
				b.parent[a] = c
			}
		}
	}
}

// rects donne les îlots, triés par ligne puis colonne.
func (b *islandBuilder) rects() []rect {
	byRoot := map[int]rect{}
	for i, r := range b.runs {
		root := b.find(i)
		x := rect{r.row, r.c0, r.row, r.c1}
		if old, ok := byRoot[root]; ok {
			x = old.union(x)
		}
		byRoot[root] = x
	}
	out := make([]rect, 0, len(byRoot))
	for _, x := range byRoot {
		out = append(out, x)
	}
	for merged := true; merged; {
		merged = false
		for i := 0; i < len(out); i++ {
			for j := i + 1; j < len(out); j++ {
				if out[i].near(out[j]) {
					out[i] = out[i].union(out[j])
					out = slices.Delete(out, j, j+1)
					merged = true
					j = i
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b rect) int {
		if a.r0 != b.r0 {
			return a.r0 - b.r0
		}
		return a.c0 - b.c0
	})
	return out
}
