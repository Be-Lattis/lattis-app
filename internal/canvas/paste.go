package canvas

import (
	"context"
	"strings"

	"github.com/lalamefine/slitable/internal/grid"
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
		if err := t.guard(sheet); err != nil {
			return err
		}
		free, err := t.areaFree(sheet, row-1, col-1, row+h, col+w)
		if err != nil {
			return err
		}
		if free && h >= MinRows {
			// La première ligne n'est un en-tête que si elle en a l'air (voir
			// hasHeader) ; sinon les colonnes s'appellent « Colonne n » et
			// l'en-tête se place au-dessus du bloc collé, qui ne bouge pas.
			first := 0
			headers := make([]string, w)
			if hasHeader(block, w) {
				for c := range w {
					headers[c] = strings.TrimSpace(cell(0, c))
				}
				first = 1
			} else {
				for c := range w {
					headers[c] = "Colonne " + itoa(c+1)
				}
				if row > 0 {
					row--
				}
				ev.Headless = true
			}
			data := make([][]string, h-first)
			for r := range data {
				data[r] = make([]string, w)
				for c := range w {
					data[r][c] = cell(r+first, c)
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
		// Bloc entièrement dans les données d'un tableau : ses lignes sont
		// lues une fois, sans relire les tableaux de la feuille à chaque
		// cellule (sur un grand tableau, plusieurs minutes pour quelques
		// centaines de cellules).
		if done, err := t.pasteInto(sheet, row, col, block, w, &ev); done || err != nil {
			return err
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

// pasteInto colle le bloc dans un tableau qui le contient entièrement
// (données seulement, pas de formule) ; faux s'il ne s'y applique pas.
func (t *tx) pasteInto(sheet int64, row, col int, block [][]string, w int, ev *Event) (bool, error) {
	tables, err := t.tables(sheet)
	if err != nil {
		return false, err
	}
	h := len(block)
	var tb *Table
	for i := range tables {
		x := &tables[i]
		if row > x.Row && row+h-1 <= x.Bottom() && col >= x.Col && col+w-1 <= x.Right() {
			tb = x
		}
	}
	if tb == nil || (tb.Filtered() && row-tb.Row-1+h > tb.Shown) {
		return false, nil
	}
	for _, r := range block {
		for _, v := range r {
			if strings.HasPrefix(strings.TrimSpace(v), "=") {
				return false, nil
			}
		}
	}
	rs, err := t.query(`SELECT id FROM `+tb.view()+` ORDER BY _pos LIMIT ? OFFSET ?`, h, row-tb.Row-1)
	if err != nil {
		return false, err
	}
	var ids []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return false, err
		}
		ids = append(ids, id)
	}
	rs.Close()
	if err := rs.Err(); err != nil || len(ids) != h {
		return false, err
	}
	cur := *tb
	for r, line := range block {
		for c := range w {
			v := ""
			if c < len(line) {
				v = line[c]
			}
			if cur, err = t.setCellAt(cur, ids[r], col+c-cur.Col, v, ev); err != nil {
				return true, err
			}
		}
	}
	return true, nil
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

// hasHeader : comme Excel, la première ligne d'un bloc collé est un en-tête si
// elle n'a que des textes, tous remplis, et qu'au moins une colonne a dessous
// des valeurs d'un autre type (nombres, dates, booléens). Un bloc entièrement
// textuel est pris comme des données.
func hasHeader(block [][]string, w int) bool {
	if len(block) < 2 {
		return false
	}
	for c := range w {
		if c >= len(block[0]) || strings.TrimSpace(block[0][c]) == "" || Infer(block[0][c]) != grid.Text {
			return false
		}
	}
	for c := range w {
		var below []string
		for _, r := range block[1:] {
			if c < len(r) {
				below = append(below, r[c])
			}
		}
		if InferAll(below) != grid.Text {
			return true
		}
	}
	return false
}

// PromoteHeader prend la première ligne de données comme en-têtes (noms des
// colonnes) puis la supprime : « Utiliser la 1re ligne comme en-têtes ».
func (m *Model) PromoteHeader(ctx context.Context, table string) error {
	return m.Do(ctx, "Utiliser la 1re ligne comme en-têtes", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if tb.Rows < 1 || tb.Filtered() {
			return nil
		}
		rid, _, err := t.rowID(tb, 0)
		if err != nil {
			return err
		}
		used := map[string]bool{}
		for i, c := range tb.Cols {
			raw, _, err := t.rawAt(tb.Sheet, tb.Row+1, tb.Col+i)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(raw)
			if name == "" {
				name = c.Name
			}
			for n := 2; used[strings.ToLower(name)]; n++ {
				name = strings.TrimSpace(raw) + " " + itoa(n)
			}
			used[strings.ToLower(name)] = true
			if err := t.rename(tb, c, name); err != nil {
				return err
			}
		}
		return t.deleteRow(tb, rid)
	})
}
