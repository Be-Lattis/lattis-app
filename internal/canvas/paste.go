package canvas

import (
	"context"
	"strings"
)

// MaxPasteCells borne un collage cellule par cellule (le collage d'un bloc
// dans une zone libre, qui crée directement un tableau, n'est pas borné).
const MaxPasteCells = 20_000

// ErrPasteTooBig signale un collage trop grand pour être appliqué cellule par
// cellule sur des tableaux existants.
type ErrPasteTooBig struct{ Cells int }

func (e ErrPasteTooBig) Error() string {
	return "collage trop grand sur des tableaux existants (" + itoa(e.Cells) + " cellules) : collez dans une zone vide"
}

func itoa(n int) string {
	var b [20]byte
	i := len(b)
	for {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
		if n == 0 {
			return string(b[i:])
		}
	}
}

// Paste colle un bloc de valeurs (lignes × colonnes, comme le presse-papiers
// d'Excel) à partir de la cellule (row, col). Tout le collage est une seule
// étape d'annulation.
//
// Dans une zone vide (rien dans le bloc ni dans sa bordure), un bloc d'au
// moins MinRows lignes devient directement un tableau : sa première ligne
// donne les en-têtes, les types sont inférés sur toutes les valeurs. Sinon,
// chaque cellule est saisie comme au clavier (extension des tableaux,
// élargissement des types, cellules libres).
func (m *Model) Paste(ctx context.Context, sheet int64, row, col int, block [][]string) (Event, error) {
	var ev Event
	h := len(block)
	w := 0
	for _, r := range block {
		w = max(w, len(r))
	}
	if h == 0 || w == 0 {
		return ev, nil
	}
	cell := func(r, c int) string {
		if c < len(block[r]) {
			return block[r][c]
		}
		return ""
	}
	err := m.Do(ctx, "Coller", func(t *tx) error {
		free, err := t.areaFree(sheet, row-1, col-1, row+h, col+w)
		if err != nil {
			return err
		}
		if free && h >= MinRows {
			headers := make([]string, w)
			for c := range w {
				headers[c] = strings.TrimSpace(cell(0, c))
			}
			data := make([][]string, h-1)
			for r := range data {
				data[r] = make([]string, w)
				for c := range w {
					data[r][c] = cell(r+1, c)
				}
			}
			name, err := t.tableName(sheet, row, col)
			if err != nil {
				return err
			}
			if _, err := t.createTable(sheet, name, row, col, headers, data); err != nil {
				return err
			}
			ev.Created = name
			return nil
		}
		if h*w > MaxPasteCells {
			return ErrPasteTooBig{h * w}
		}
		for r := range h {
			for c := range w {
				if err := t.input(sheet, row+r, col+c, cell(r, c), &ev); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return ev, err
}

// areaFree indique si le rectangle [r0, r1] × [c0, c1] ne touche aucun
// tableau (zone grisée comprise) ni aucune cellule libre.
func (t *tx) areaFree(sheet int64, r0, c0, r1, c1 int) (bool, error) {
	tables, err := t.tables(sheet)
	if err != nil {
		return false, err
	}
	for _, tb := range tables {
		if tb.Row <= r1 && tb.Bottom()+1 >= r0 && tb.Col <= c1 && tb.Right()+1 >= c0 {
			return false, nil
		}
	}
	var n int
	err = t.row(`SELECT count(*) FROM _slitable_cells WHERE sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`,
		sheet, r0, r1, c0, c1).Scan(&n)
	return n == 0, err
}
