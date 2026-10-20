package canvas

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// MaxPasteCells borne un collage cellule par cellule (le collage d'un bloc
// dans une zone libre, qui crée directement un tableau, n'est pas borné).
const MaxPasteCells = 20_000

// ErrPasteTooBig signale un collage trop grand pour être appliqué cellule par
// cellule sur des tableaux existants.
type ErrPasteTooBig struct{ Cells int }

func (e ErrPasteTooBig) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e ErrPasteTooBig) Msg() msg.Message {
	return msg.M("paste.tooBig", msg.Args{"n": e.Cells})
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
// Dans une zone vide (rien dans le bloc ni dans sa bordure), le bloc est
// découpé aux lignes et colonnes vides, comme à l'import xlsx (voir
// pasteIslands) : chaque îlot d'au moins MinRows lignes devient directement
// un tableau (sa première ligne donne les en-têtes, les types sont inférés
// sur toutes les valeurs) ; titres, notes et totaux restent libres. Sinon,
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
	// Bloc tapé ou copié dans la langue de la session : formules et valeurs
	// dans la forme gardée (session.go).
	block, err := m.ui(ctx).blockIn(block)
	if err != nil {
		return ev, err
	}
	cell := func(r, c int) string {
		if c < len(block[r]) {
			return block[r][c]
		}
		return ""
	}
	err = m.Do(ctx, "Coller", func(t *tx) error {
		if err := t.guardCells(sheet, Rect{row, col, row + h - 1, col + w - 1}, true); err != nil {
			return err
		}
		t.undoBirth = true
		// Zone vide : rien dans le bloc ni contre lui (la ligne grisée sous un
		// tableau, à une ligne d'écart, ne compte pas : audit 2 I2).
		free, err := t.areaClear(sheet, row-1, col-1, row+h, col+w)
		if err != nil {
			return err
		}
		if free {
			// Collé dans une plage (tableau converti) ou contre elle : des
			// cellules libres qui l'agrandissent, pas un tableau.
			near, err := t.nearRange(sheet, row, col, row+h-1, col+w-1)
			if err != nil {
				return err
			}
			free = !near
		}
		if free && h >= MinRows {
			return t.pasteIslands(sheet, row, col, block, w, &ev)
		}
		if h*w > MaxPasteCells {
			return ErrPasteTooBig{h * w}
		}
		t.collect = true
		// Bloc entièrement dans les données d'un tableau : ses lignes sont
		// lues une fois, sans relire les tableaux de la feuille à chaque
		// cellule (sur un grand tableau, plusieurs minutes pour quelques
		// centaines de cellules).
		if done, err := t.pasteInto(sheet, row, col, block, w, &ev); done || err != nil {
			return err
		}
		t.pasting = true
		defer func() { t.pasting = false }()
		for r := range h {
			for c := range w {
				if err := t.input(sheet, row+r, col+c, cell(r, c), &ev); err != nil {
					return err
				}
			}
		}
		if ev.Created != "" {
			ev.AutoKey = t.autoKeyed
		}
		return nil
	})
	ev.Split = m.splitBirth(ctx, ev, err)
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
	err = t.row(`SELECT count(*) FROM _lattis_cells WHERE sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`,
		sheet, r0, r1, c0, c1).Scan(&n)
	return n == 0, err
}

// areaClear indique si le rectangle [r0, r1] × [c0, c1] ne recouvre aucun
// tableau (sans sa zone grisée) ni aucune cellule libre.
func (t *tx) areaClear(sheet int64, r0, c0, r1, c1 int) (bool, error) {
	tables, err := t.tables(sheet)
	if err != nil {
		return false, err
	}
	for _, tb := range tables {
		if tb.Top() <= r1 && tb.Bottom() >= r0 && tb.Col <= c1 && tb.Right() >= c0 {
			return false, nil
		}
	}
	var n int
	err = t.row(`SELECT count(*) FROM _lattis_cells WHERE sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`,
		sheet, r0, r1, c0, c1).Scan(&n)
	return n == 0, err
}

// hasHeader : comme Excel, la première ligne d'un bloc collé est un en-tête si
// elle n'a que des textes, tous remplis, et qu'une colonne
// a dessous des valeurs d'un autre type (nombres, dates, booléens, heures,
// pourcentages), même mêlées de textes. Quand tout est du texte, chaque
// colonne vote, comme la détection d'en-tête des CSV : des valeurs de même
// longueur dessous (codes, villes de 4 lettres…) et un premier texte d'une
// autre longueur votent pour, la même longueur contre ; un premier texte
// répété plus bas, ou deux premiers textes égaux, sont des données. Un bloc
// de deux lignes tout en texte reste des données.
func hasHeader(block [][]string, w int) bool {
	if len(block) < 2 {
		return false
	}
	seen, dup, text := map[string]bool{}, false, false
	// Coin vide d'un tableau croisé copié d'Excel ( | Jan | Fév, puis Nord |
	// 1 | 2) : la 1re colonne, des textes dessous, n'a pas de nom (audit 2 I6).
	corner := w > 1 && len(block[0]) > 0 && strings.TrimSpace(block[0][0]) == "" && textValues(block[1:], 0)
	for c := range w {
		if c >= len(block[0]) {
			return false
		}
		h := strings.TrimSpace(block[0][c])
		if c == 0 && corner {
			continue
		}
		// Une année ou une date (Pays | 2024 | 2025, Pays | 01/01/2024) nomme
		// une colonne comme un texte, pourvu que la ligne ait aussi un vrai
		// texte.
		if h == "" || (Infer(h) != grid.Text || isTimeOrPercent(h)) && !yearLike(h) && Infer(h) != grid.Date {
			return false
		}
		text = text || !yearLike(h) && Infer(h) != grid.Date
		dup = dup || seen[strings.ToLower(h)]
		seen[strings.ToLower(h)] = true
	}
	if !text {
		return false
	}
	votes := 0
	for c := range w {
		h := strings.TrimSpace(block[0][c])
		if h == "" {
			continue // coin vide
		}
		var below []string
		for _, r := range block[1:] {
			if c < len(r) && strings.TrimSpace(r[c]) != "" {
				below = append(below, strings.TrimSpace(r[c]))
			}
		}
		if len(below) == 0 {
			continue
		}
		typed := false
		for _, v := range below {
			if strings.EqualFold(v, h) {
				return false
			}
			typed = typed || Infer(v) != grid.Text || isTimeOrPercent(v)
		}
		if InferAll(below) != grid.Text {
			return true
		}
		if typed {
			votes++
			continue
		}
		if len(below) < 2 {
			continue
		}
		n := utf8.RuneCountInString(below[0])
		same := true
		for _, v := range below[1:] {
			same = same && utf8.RuneCountInString(v) == n
		}
		switch {
		case !same:
		case utf8.RuneCountInString(h) != n:
			votes++
		default:
			votes--
		}
	}
	return votes > 0 && !dup
}

// textValues : les valeurs de la colonne c des lignes sont des textes (au
// moins une).
func textValues(rows [][]string, c int) bool {
	n := 0
	for _, r := range rows {
		if c >= len(r) || strings.TrimSpace(r[c]) == "" {
			continue
		}
		if v := strings.TrimSpace(r[c]); Infer(v) != grid.Text || isTimeOrPercent(v) {
			return false
		}
		n++
	}
	return n > 0
}

// yearLike : un entier entre 1900 et 2100, en-tête de colonne courant.
func yearLike(s string) bool {
	if len(s) != 4 || Infer(s) != grid.Integer {
		return false
	}
	return s >= "1900" && s <= "2100"
}

// promote : la première ligne de données donne les noms des colonnes, puis
// est supprimée (PromoteHeader, unnamed.go ; HeaderToTitle).
func (t *tx) promote(tb Table) error {
	rid, _, err := t.rowID(tb, 0)
	if err != nil {
		return err
	}
	final := make([]string, len(tb.Cols))
	for i, c := range tb.Cols {
		raw, _, err := t.rawAt(tb.Sheet, tb.Row+1, tb.Col+i)
		if err != nil {
			return err
		}
		if final[i] = strings.TrimSpace(raw); final[i] == "" {
			final[i] = c.Name
		}
	}
	if err := t.renameAll(tb, uniqueColumnNames(final)); err != nil {
		return err
	}
	return t.deleteRow(tb, rid)
}
