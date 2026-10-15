package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/lalamefine/slitable/internal/formula"
	"github.com/lalamefine/slitable/internal/grid"
	"github.com/lalamefine/slitable/internal/order"
)

// MinRows est le nombre de lignes (en-tête compris) à partir duquel une zone
// de cellules libres devient un tableau.
const MinRows = 2

// Event décrit ce qu'une saisie a provoqué, pour l'afficher à l'utilisateur.
type Event struct {
	Created string    `json:"created"` // tableau créé
	Widened string    `json:"widened"` // colonne élargie ("Tableau.Colonne")
	From    grid.Type `json:"from"`    // élargissement
	To      grid.Type `json:"to"`
	Invalid bool      `json:"invalid"` // saisie hors type dans une colonne au type fixé
}

// SetCell traite une saisie validée dans la cellule (row, col) d'une feuille,
// comme le ferait la grille : dans un tableau, dans sa ligne ou sa colonne
// grisée (extension), ou dans une cellule libre (avec détection d'îlot).
func (m *Model) SetCell(ctx context.Context, sheet int64, row, col int, raw string) (Event, error) {
	var ev Event
	err := m.Do(ctx, "Saisie", func(t *tx) error { return t.input(sheet, row, col, raw, &ev) })
	return ev, err
}

// input applique une saisie dans la transaction en cours.
func (t *tx) input(sheet int64, row, col int, raw string, ev *Event) error {
	{
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		empty := strings.TrimSpace(raw) == ""
		for _, tb := range tables {
			inCols := col >= tb.Col && col <= tb.Right()
			inRows := row >= tb.Row && row <= tb.Bottom()
			switch {
			case inCols && row == tb.Row:
				c := tb.Cols[col-tb.Col]
				name := strings.TrimSpace(raw)
				if name == "" {
					name = fmt.Sprintf("Colonne %d", col-tb.Col+1)
				}
				return t.rename(tb, c, name)
			case inCols && inRows:
				return t.setCell(tb, row-tb.Row-1, col-tb.Col, raw, ev)
			case inCols && row == tb.Bottom()+1 && !empty:
				if err := t.appendRows(tb, 1); err != nil {
					return err
				}
				tb.Rows++
				return t.setCell(tb, tb.Rows-1, col-tb.Col, raw, ev)
			case inRows && col == tb.Right()+1 && !empty:
				name := fmt.Sprintf("Colonne %d", len(tb.Cols)+1)
				if row == tb.Row {
					name = strings.TrimSpace(raw)
				}
				tb, err = t.newColumn(tb, len(tb.Cols), name)
				if err != nil || row == tb.Row {
					return err
				}
				return t.setCell(tb, row-tb.Row-1, len(tb.Cols)-1, raw, ev)
			}
		}
		if isFormula(raw) {
			// Formule dans une cellule libre : enregistrée réécrite (RECHERCHEV
			// devient REFERENCE), calculée à l'affichage. Elle ne fait pas
			// naître de tableau.
			n, err := t.prepareFormula(strings.TrimSpace(raw))
			if err != nil {
				return err
			}
			return t.setFreeCell(sheet, row, col, formula.Format(n, formula.FR))
		}
		if err := t.setFreeCell(sheet, row, col, raw); err != nil || empty {
			return err
		}
		name, err := t.birth(sheet, row, col)
		if name != "" {
			ev.Created = name
		}
		return err
	}
}

// setCell écrit une saisie dans la ligne de données n, colonne i. Une saisie
// hors type élargit une colonne au type inféré, ou est marquée invalide dans
// une colonne au type fixé. Une colonne encore vide prend le type de sa
// première valeur.
func (t *tx) setCell(tb Table, n, i int, raw string, ev *Event) error {
	rid, _, err := t.rowID(tb, n)
	if err != nil {
		return err
	}
	c := tb.Cols[i]
	if src := strings.TrimSpace(raw); strings.HasPrefix(src, "=") && len(src) > 1 {
		return t.setFormula(tb, i, src)
	}
	if c.Formula != "" {
		if strings.TrimSpace(raw) == "" {
			return t.setFormula(tb, i, "") // effacer une cellule calculée retire la formule
		}
		return ErrComputed{c.Name}
	}
	if strings.TrimSpace(raw) == "" {
		return t.setValue(tb, rid, c, nil, "")
	}
	v, perr := grid.Parse(c.Type, raw)
	if c.Fixed {
		if perr != nil {
			ev.Invalid = true
			return t.setValue(tb, rid, c, nil, raw)
		}
		return t.setValue(tb, rid, c, v, "")
	}
	var others int
	if err := t.row(`SELECT count(`+colSQL(c.ID)+`) FROM `+tb.sql()+` WHERE id <> ?`, rid).Scan(&others); err != nil {
		return err
	}
	to := c.Type
	switch {
	case others == 0:
		to = Infer(raw) // colonne vide : elle prend le type de sa première valeur
	case perr != nil:
		to = Join(c.Type, Infer(raw))
		ev.Widened, ev.From, ev.To = tb.Name+"."+c.Name, c.Type, to
	}
	if to != c.Type {
		if tb, err = t.retype(tb, i, to); err != nil {
			return err
		}
	}
	if v, err = grid.Parse(to, raw); err != nil {
		return err
	}
	return t.setValue(tb, rid, tb.Cols[i], v, "")
}

// appendRows ajoute n lignes vides en fin de tableau et décale ce qui est dessous.
func (t *tx) appendRows(tb Table, n int) error {
	var last string
	var maxID int64
	if err := t.row(`SELECT coalesce(max(_pos), ''), coalesce(max(id), 0) FROM `+tb.sql()).Scan(&last, &maxID); err != nil {
		return err
	}
	for k := range n {
		pos, err := order.Between(last, "")
		if err != nil {
			return err
		}
		if err := t.insertRow(tb, maxID+int64(k)+1, pos); err != nil {
			return err
		}
		last = pos
	}
	return t.ensureGapBelow(tb.Sheet, tb.ID, tb.Bottom(), tb.Bottom()+n, tb.Col, tb.Right())
}

// newColumn ajoute une colonne au rang pos et décale ce qui est à droite.
func (t *tx) newColumn(tb Table, pos int, name string) (Table, error) {
	var id int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _slitable_columns WHERE tbl = ?`, tb.ID).Scan(&id); err != nil {
		return tb, err
	}
	if err := t.addColumn(tb, id, name, grid.Text, pos); err != nil {
		return tb, err
	}
	if err := t.ensureGapRight(tb.Sheet, tb.ID, tb.Right(), tb.Right()+1, tb.Row, tb.Bottom()); err != nil {
		return tb, err
	}
	return t.table(tb.ID)
}

// birth crée un tableau si la cellule saisie appartient à une zone de
// cellules libres d'au moins MinRows lignes. La première ligne de la zone
// donne les en-têtes.
func (t *tx) birth(sheet int64, row, col int) (string, error) {
	cells, err := t.cells(sheet)
	if err != nil {
		return "", err
	}
	isl := island(cells, row, col)
	r0, r1, c0, c1 := isl[0].Row, isl[0].Row, isl[0].Col, isl[0].Col
	for _, c := range isl {
		r1 = max(r1, c.Row)
		c0, c1 = min(c0, c.Col), max(c1, c.Col)
	}
	if r1-r0+1 < MinRows {
		return "", nil
	}
	grid2 := make([][]string, r1-r0+1)
	for i := range grid2 {
		grid2[i] = make([]string, c1-c0+1)
	}
	for _, c := range cells {
		if c.Row >= r0 && c.Row <= r1 && c.Col >= c0 && c.Col <= c1 {
			grid2[c.Row-r0][c.Col-c0] = c.Raw
			if err := t.setFreeCell(sheet, c.Row, c.Col, ""); err != nil {
				return "", err
			}
		}
	}
	name, err := t.tableName(sheet, r0, c0)
	if err != nil {
		return "", err
	}
	_, err = t.createTable(sheet, name, r0, c0, grid2[0], grid2[1:])
	return name, err
}

// tableName : un tableau placé en haut à gauche de la feuille prend le nom de
// la feuille, les autres « TableauN ». Comme dans Excel, un nom de tableau
// est un identifiant (sans espace), pour s'écrire tel quel dans une formule.

// identifier rend un nom utilisable dans une formule (« Feuille 1 » →
// « Feuille1 ») ; vide s'il ne reste rien.
func identifier(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case unicode.IsLetter(r) || r == '_':
			b.WriteRune(r)
		case unicode.IsDigit(r) && i > 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (t *tx) tableName(sheet int64, row, col int) (string, error) {
	var sheetName string
	if err := t.row(`SELECT name FROM _slitable_sheets WHERE id = ?`, sheet).Scan(&sheetName); err != nil {
		return "", err
	}
	free := func(n string) (bool, error) {
		var k int
		err := t.row(`SELECT count(*) FROM _slitable_tables WHERE name = ?`, n).Scan(&k)
		return k == 0, err
	}
	if row == 0 && col == 0 {
		if name := identifier(sheetName); name != "" {
			if ok, err := free(name); ok || err != nil {
				return name, err
			}
		}
	}
	for n := 1; ; n++ {
		name := fmt.Sprintf("Tableau%d", n)
		if ok, err := free(name); ok || err != nil {
			return name, err
		}
	}
}

// --- Actions sur un tableau désigné par son nom ------------------------------

// InsertRow insère une ligne vide avant la ligne de données n (n = nombre de
// lignes pour ajouter à la fin).
func (m *Model) InsertRow(ctx context.Context, table string, n int) error {
	return m.Do(ctx, "Insérer une ligne", func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		if n < 0 || n > tb.Rows {
			return errors.New("canvas: ligne hors du tableau")
		}
		var before, after string
		if n > 0 {
			if _, before, err = t.rowID(tb, n-1); err != nil {
				return err
			}
		}
		if n < tb.Rows {
			if _, after, err = t.rowID(tb, n); err != nil {
				return err
			}
		}
		pos, err := order.Between(before, after)
		if err != nil {
			return err
		}
		var id int64
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM ` + tb.sql()).Scan(&id); err != nil {
			return err
		}
		if err := t.insertRow(tb, id, pos); err != nil {
			return err
		}
		return t.ensureGapBelow(tb.Sheet, tb.ID, tb.Bottom(), tb.Bottom()+1, tb.Col, tb.Right())
	})
}

// DeleteRow supprime la ligne de données n. Le tableau rétrécit ; ce qui est
// dessous ne remonte pas.
func (m *Model) DeleteRow(ctx context.Context, table string, n int) error {
	return m.Do(ctx, "Supprimer une ligne", func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		rid, _, err := t.rowID(tb, n)
		if err != nil {
			return err
		}
		return t.deleteRow(tb, rid)
	})
}

// InsertColumn insère une colonne avant la colonne i.
func (m *Model) InsertColumn(ctx context.Context, table string, i int, name string) error {
	return m.Do(ctx, "Insérer une colonne", func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		if i < 0 || i > len(tb.Cols) {
			return errors.New("canvas: colonne hors du tableau")
		}
		_, err = t.newColumn(tb, i, name)
		return err
	})
}

// DeleteColumn supprime la colonne i (un tableau garde au moins une colonne).
func (m *Model) DeleteColumn(ctx context.Context, table string, i int) error {
	return m.Do(ctx, "Supprimer une colonne", func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(tb.Cols) || len(tb.Cols) == 1 {
			return errors.New("canvas: colonne impossible à supprimer")
		}
		return t.dropColumn(tb, tb.Cols[i], i)
	})
}

// FixType fixe le type de la colonne i : il ne sera plus élargi, les saisies
// hors type seront marquées invalides.
func (m *Model) FixType(ctx context.Context, table string, i int) error {
	return m.Do(ctx, "Fixer le type", func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		q := `UPDATE _slitable_columns SET fixed = ? WHERE tbl = ? AND id = ?`
		return t.apply([]stmt{st(q, 1, tb.ID, tb.Cols[i].ID)}, []stmt{st(q, tb.Cols[i].Fixed, tb.ID, tb.Cols[i].ID)})
	})
}
