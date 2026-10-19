package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Commandes d'Excel au-delà de la simple mise en forme : notes, cellules
// fusionnées, lignes et colonnes masquées, volets figés.
//
// Notes, fusions et masquage sont des propriétés des couches de style
// (styles.go) : ils suivent ainsi l'insertion et la suppression de lignes,
// le déplacement des cellules, le tri des tableaux et l'annulation, comme
// le reste de la mise en forme. Ces propriétés ne font pas partie de la
// mise en forme proprement dite : « Effacer la mise en forme » et le
// pinceau les laissent en place.
//
//	nt      note de la cellule (texte)                   cellule
//	np      note épinglée (toujours affichée)            cellule
//	mr, mc  fusion : lignes et colonnes couvertes        cellule libre en haut à gauche
//	hd      ligne ou colonne masquée                     ligne ou colonne de la feuille
//	dv      validation : liste des valeurs permises      cellule, colonne de tableau
//
// La validation (dv) se pose comme une mise en forme (ApplyStyle) : sur
// toute une colonne de tableau, elle vaut pour les lignes à venir.

// MaxMerge borne le nombre de cellules d'une fusion.
const MaxMerge = 10_000

// ErrMerge signale une fusion impossible.
var ErrMerge = errors.New("fusion")

// SetNote pose la note d'une cellule (texte vide : la retire). Dans un
// tableau, la note suit sa ligne (tri, insertion).
func (m *Model) SetNote(ctx context.Context, sheet int64, row, col int, text string) error {
	text = strings.TrimRight(text, " \t\r\n")
	label := "Note"
	if text == "" {
		label = "Supprimer la note"
	}
	patch := Style{"nt": text}
	if text == "" {
		patch["np"] = nil // une note retirée n'est plus épinglée
	}
	return m.notePatch(ctx, label, sheet, row, col, patch)
}

// PinNote épingle la note d'une cellule (toujours affichée, comme « Afficher
// la note » d'Excel) ou la remet en infobulle.
func (m *Model) PinNote(ctx context.Context, sheet int64, row, col int, pinned bool) error {
	patch := Style{"np": nil}
	label := "Masquer la note"
	if pinned {
		patch["np"], label = true, "Afficher la note"
	}
	return m.notePatch(ctx, label, sheet, row, col, patch)
}

// notePatch pose une propriété de note : dans un tableau elle suit sa ligne.
func (m *Model) notePatch(ctx context.Context, label string, sheet int64, row, col int, patch Style) error {
	return m.Do(ctx, label, func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for i := range tables {
			tb := &tables[i]
			if row < tb.Row || row > tb.Bottom() || col < tb.Col || col > tb.Right() {
				continue
			}
			colID := tb.Cols[col-tb.Col].ID
			if row == tb.Row {
				return t.styleTable(tb.ID, -1, colID, patch, nil)
			}
			ids, err := t.ridsAt(tb, row, row)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				return fmt.Errorf("%w : ligne masquée par le filtre du tableau « %s »", ErrTable, tb.Name)
			}
			return t.styleTable(tb.ID, ids[0], colID, patch, nil)
		}
		return t.styleFree(sheet, row, col, patch, nil)
	})
}

// Merge est une plage de cellules fusionnées (cellule en haut à gauche et
// nombre de lignes et de colonnes couvertes).
type Merge struct {
	Row, Col, Rows, Cols int
}

func (g Merge) bottom() int { return g.Row + g.Rows - 1 }
func (g Merge) right() int  { return g.Col + g.Cols - 1 }

func (g Merge) overlaps(r0, c0, r1, c1 int) bool {
	return g.Row <= r1 && g.bottom() >= r0 && g.Col <= c1 && g.right() >= c0
}

// merges lit les fusions de la feuille.
func (t *tx) merges(sheet int64) ([]Merge, error) {
	rs, err := t.query(`SELECT row, col, style FROM _lattis_cell_styles WHERE sheet = ? AND style LIKE '%"mr"%'`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []Merge
	for rs.Next() {
		var g Merge
		var s string
		if err := rs.Scan(&g.Row, &g.Col, &s); err != nil {
			return nil, err
		}
		st := parseStyle(s)
		g.Rows, g.Cols = intOf(st["mr"]), intOf(st["mc"])
		if g.Rows < 1 || g.Cols < 1 || g.Rows*g.Cols < 2 {
			continue
		}
		out = append(out, g)
	}
	return out, rs.Err()
}

func intOf(v any) int {
	switch v := v.(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// MergeCells fusionne les cellules libres du rectangle [r0, r1] × [c0, c1]
// et centre leur contenu (« Fusionner et centrer » d'Excel). Comme Excel,
// seule la valeur en haut à gauche est gardée ; renvoie le nombre de
// valeurs effacées. Les fusions recouvertes sont remplacées. Refusé dans un
// tableau ou un tableau croisé : un tableau garde une cellule par valeur ;
// seule la bande des groupes d'en-têtes (avec l'en-tête) se fusionne
// (mergeBand) : la colonne passe sur un niveau, ou les colonnes forment un
// groupe.
func (m *Model) MergeCells(ctx context.Context, sheet int64, r0, c0, r1, c1 int) (int, error) {
	if r0 > r1 {
		r0, r1 = r1, r0
	}
	if c0 > c1 {
		c0, c1 = c1, c0
	}
	if r0 < 0 || c0 < 0 {
		return 0, fmt.Errorf("%w : plage hors de la feuille", ErrMerge)
	}
	if r0 == r1 && c0 == c1 {
		return 0, fmt.Errorf("%w : sélectionnez au moins deux cellules à fusionner", ErrMerge)
	}
	if (r1-r0+1)*(c1-c0+1) > MaxMerge {
		return 0, fmt.Errorf("%w : fusion limitée à %d cellules", ErrMerge, MaxMerge)
	}
	dropped := 0
	err := m.Do(ctx, "Fusionner et centrer", func(t *tx) error {
		if err := t.guardFormat(sheet); err != nil {
			return err
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		// Bande des groupes d'en-têtes : colonne sur un niveau, ou groupe
		// sur plusieurs colonnes (twolevel.go).
		if ok, err := t.mergeBand(tables, r0, c0, r1, c1); ok || err != nil {
			return err
		}
		for _, tb := range tables {
			if tb.Top() <= r1 && tb.Bottom() >= r0 && tb.Col <= c1 && tb.Right() >= c0 {
				return fmt.Errorf("%w : impossible dans le tableau « %s » (un tableau garde une cellule par valeur) ; fusionnez des cellules libres, hors des tableaux", ErrMerge, tb.Name)
			}
		}
		pivots, err := t.pivots(sheet)
		if err != nil {
			return err
		}
		for _, p := range pivots {
			rows, cols := len(p.Cells), 1
			if rows > 0 {
				cols = len(p.Cells[0])
			}
			rows = max(rows, 1)
			if p.Row <= r1 && p.Row+rows-1 >= r0 && p.Col <= c1 && p.Col+cols-1 >= c0 {
				return fmt.Errorf("%w : impossible dans un tableau croisé dynamique", ErrMerge)
			}
		}
		old, err := t.merges(sheet)
		if err != nil {
			return err
		}
		for _, g := range old {
			if g.overlaps(r0, c0, r1, c1) {
				if err := t.styleFree(sheet, g.Row, g.Col, Style{"mr": nil, "mc": nil}, nil); err != nil {
					return err
				}
			}
		}
		// Comme Excel : seule la valeur en haut à gauche reste.
		cells, err := t.cells(sheet)
		if err != nil {
			return err
		}
		for _, c := range cells {
			if c.Row < r0 || c.Row > r1 || c.Col < c0 || c.Col > c1 || (c.Row == r0 && c.Col == c0) || strings.TrimSpace(c.Raw) == "" {
				continue
			}
			var ev Event
			if err := t.input(sheet, c.Row, c.Col, "", &ev); err != nil {
				return err
			}
			dropped++
		}
		return t.styleFree(sheet, r0, c0, Style{"mr": r1 - r0 + 1, "mc": c1 - c0 + 1, "al": "center"}, nil)
	})
	return dropped, err
}

// UnmergeCells annule les fusions qui touchent le rectangle [r0, r1] × [c0, c1].
func (m *Model) UnmergeCells(ctx context.Context, sheet int64, r0, c0, r1, c1 int) error {
	if r0 > r1 {
		r0, r1 = r1, r0
	}
	if c0 > c1 {
		c0, c1 = c1, c0
	}
	return m.Do(ctx, "Annuler la fusion", func(t *tx) error {
		if err := t.guardFormat(sheet); err != nil {
			return err
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		if err := t.unmergeBand(tables, r0, c0, r1, c1); err != nil {
			return err
		}
		old, err := t.merges(sheet)
		if err != nil {
			return err
		}
		for _, g := range old {
			if g.overlaps(r0, c0, r1, c1) {
				if err := t.styleFree(sheet, g.Row, g.Col, Style{"mr": nil, "mc": nil}, nil); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// SetLinesHidden masque (ou affiche) les lignes (rows) ou les colonnes
// [i0, i1] de la feuille. Une ligne masquée garde son contenu, sa hauteur
// et sa place dans les formules : elle n'est simplement plus affichée (les
// lignes d'un tableau qu'elle traverse non plus). Afficher ne touche que les
// lignes masquées de la plage (elle peut être grande : toute la feuille).
func (m *Model) SetLinesHidden(ctx context.Context, sheet int64, rows bool, i0, i1 int, hide bool) error {
	if i0 > i1 {
		i0, i1 = i1, i0
	}
	what := "les colonnes"
	if rows {
		what = "les lignes"
	}
	if i0 < 0 || (hide && i1-i0+1 > MaxStyleLines) {
		return fmt.Errorf("masquage limité à %d lignes ou colonnes", MaxStyleLines)
	}
	label := "Masquer " + what
	if !hide {
		label = "Afficher " + what
	}
	return m.Do(ctx, label, func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		if hide {
			for i := i0; i <= i1; i++ {
				if err := t.styleLine(sheet, rows, i, Style{"hd": true}, nil); err != nil {
					return err
				}
			}
			return nil
		}
		table, col := `_lattis_col_styles`, `col`
		if rows {
			table, col = `_lattis_row_styles`, `row`
		}
		rs, err := t.query(`SELECT `+col+` FROM `+table+` WHERE sheet = ? AND `+col+` BETWEEN ? AND ? AND style LIKE '%"hd"%'`, sheet, i0, i1)
		if err != nil {
			return err
		}
		var lines []int
		for rs.Next() {
			var i int
			if err := rs.Scan(&i); err != nil {
				rs.Close()
				return err
			}
			lines = append(lines, i)
		}
		rs.Close()
		for _, i := range lines {
			if err := t.styleLine(sheet, rows, i, Style{"hd": nil}, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetFreeze fige les volets de la feuille : les rows premières lignes et
// les cols premières colonnes restent affichées pendant le défilement
// (0 et 0 : libère les volets). Un réglage d'affichage, permis sur une
// feuille protégée (comme le quadrillage).
func (m *Model) SetFreeze(ctx context.Context, sheet int64, rows, cols int) error {
	if rows < 0 || cols < 0 || rows > 1000 || cols > 1000 {
		return fmt.Errorf("volets figés : 1 000 lignes ou colonnes au plus")
	}
	label := "Figer les volets"
	if rows == 0 && cols == 0 {
		label = "Libérer les volets"
	}
	return m.Do(ctx, label, func(t *tx) error {
		var r, c int
		if err := t.row(`SELECT freeze_rows, freeze_cols FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&r, &c); err != nil {
			return err
		}
		if r == rows && c == cols {
			return nil
		}
		q := `UPDATE _lattis_sheets SET freeze_rows = ?, freeze_cols = ? WHERE id = ?`
		return t.apply([]stmt{st(q, rows, cols, sheet)}, []stmt{st(q, r, c, sheet)})
	})
}
