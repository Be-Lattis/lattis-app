package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// ErrSheet signale une opération impossible sur un onglet.
var ErrSheet = errors.New("onglet")

// AddSheet ajoute un onglet à la fin et renvoie son identifiant.
func (m *Model) AddSheet(ctx context.Context) (int64, error) {
	var id int64
	err := m.Do(ctx, "Nouvelle feuille", func(t *tx) (err error) {
		id, err = t.addSheet()
		return err
	})
	return id, err
}

// InsertSheet ajoute un onglet juste avant l'onglet before (Insérer, dans le
// menu d'un onglet) et renvoie son identifiant.
func (m *Model) InsertSheet(ctx context.Context, before int64) (int64, error) {
	var id int64
	err := m.Do(ctx, "Insérer une feuille", func(t *tx) error {
		ids, err := t.sheetOrder()
		if err != nil {
			return err
		}
		at := slices.Index(ids, before)
		if at < 0 {
			return msg.Wrap(ErrSheet, "sheet.notFound", nil)
		}
		if id, err = t.addSheet(); err != nil {
			return err
		}
		return t.moveSheet(id, at)
	})
	return id, err
}

// MoveSheet place l'onglet sheet au rang to (0 : premier), les autres
// onglets gardant leur ordre.
func (m *Model) MoveSheet(ctx context.Context, sheet int64, to int) error {
	return m.Do(ctx, "Déplacer la feuille", func(t *tx) error { return t.moveSheet(sheet, to) })
}

// addSheet ajoute une feuille nommée dans la langue de la session
// (« Feuille2 », « Sheet2 »… ; en français sous StorageForm).
func (t *tx) addSheet() (int64, error) { return t.numberedSheet(t.defaultName("name.sheet"), true) }

// addNamedSheet ajoute un onglet nommé prefix suivi d'un numéro libre, à
// partir de 1.
func (t *tx) addNamedSheet(prefix string) (int64, error) { return t.numberedSheet(prefix, false) }

// numberedSheet ajoute un onglet nommé prefix suivi d'un numéro libre (celui
// de l'onglet pour une feuille, comme Excel ; à partir de 1 sinon).
func (t *tx) numberedSheet(prefix string, sheet bool) (int64, error) {
	if err := t.guardWorkbook(); err != nil {
		return 0, err
	}
	var id int64
	var pos int
	if err := t.row(`SELECT coalesce(max(id), 0) + 1, coalesce(max(pos), -1) + 1 FROM _lattis_sheets`).Scan(&id, &pos); err != nil {
		return 0, err
	}
	name := ""
	start := int(id)
	if !sheet {
		start = 1
	}
	for n := start; ; n++ {
		name = fmt.Sprintf("%s%d", prefix, n)
		taken, err := t.sheetNameTaken(name, 0)
		if err != nil {
			return 0, err
		}
		if !taken {
			break
		}
	}
	return id, t.apply(
		[]stmt{st(`INSERT INTO _lattis_sheets (id, name, pos, gridlines) VALUES (?, ?, ?, 0)`, id, name, pos)},
		[]stmt{st(`DELETE FROM _lattis_sheets WHERE id = ?`, id)},
	)
}

// sheetNameTaken indique si une autre feuille que except porte déjà ce nom,
// sans tenir compte des majuscules (accents compris : « Été » et « été »),
// comme Excel.
func (t *tx) sheetNameTaken(name string, except int64) (bool, error) {
	rs, err := t.query(`SELECT name FROM _lattis_sheets WHERE id <> ?`, except)
	if err != nil {
		return false, err
	}
	defer rs.Close()
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			return false, err
		}
		if strings.EqualFold(n, name) {
			return true, nil
		}
	}
	return false, rs.Err()
}

// sheetOrder donne les onglets dans l'ordre affiché.
func (t *tx) sheetOrder() ([]int64, error) {
	rs, err := t.query(`SELECT id FROM _lattis_sheets ORDER BY pos, id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var ids []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rs.Err()
}

// moveSheet renumérote les rangs pour placer sheet au rang to ; l'annulation
// rend les rangs d'origine.
func (t *tx) moveSheet(sheet int64, to int) error {
	if err := t.guardTab(sheet); err != nil {
		return err
	}
	ids, err := t.sheetOrder()
	if err != nil {
		return err
	}
	from := slices.Index(ids, sheet)
	if from < 0 {
		return msg.Wrap(ErrSheet, "sheet.notFound", nil)
	}
	to = max(0, min(to, len(ids)-1))
	if from == to {
		return nil
	}
	old := map[int64]int{}
	rs, err := t.query(`SELECT id, pos FROM _lattis_sheets`)
	if err != nil {
		return err
	}
	for rs.Next() {
		var id int64
		var pos int
		if err := rs.Scan(&id, &pos); err != nil {
			rs.Close()
			return err
		}
		old[id] = pos
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	q := `UPDATE _lattis_sheets SET pos = ? WHERE id = ?`
	var do, undo []stmt
	ids = slices.Insert(slices.Delete(ids, from, from+1), to, sheet)
	for i, id := range ids {
		do = append(do, st(q, i, id))
		undo = append(undo, st(q, old[id], id))
	}
	return t.apply(do, undo)
}

// RenameSheet renomme un onglet (scripts, assistant : l'onglet seul).
func (m *Model) RenameSheet(ctx context.Context, sheet int64, name string) error {
	return m.renameSheetAs(ctx, sheet, name, false)
}

// RenameSheetLinked renomme un onglet depuis l'application : son seul
// tableau suit (soletable.go).
func (m *Model) RenameSheetLinked(ctx context.Context, sheet int64, name string) error {
	return m.renameSheetAs(ctx, sheet, name, true)
}

func (m *Model) renameSheetAs(ctx context.Context, sheet int64, name string, linked bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return msg.Wrap(ErrSheet, "sheet.emptyName", nil)
	}
	return m.Do(ctx, "Renommer la feuille", func(t *tx) error {
		if err := t.guardTab(sheet); err != nil {
			return err
		}
		var old string
		if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
			return err
		}
		if old == name {
			return nil
		}
		taken, err := t.sheetNameTaken(name, sheet)
		if err != nil {
			return err
		}
		if taken {
			return msg.Wrap(ErrSheet, "sheet.nameTaken", msg.Args{"name": name})
		}
		if err := t.renameSheet(sheet, old, name); err != nil {
			return err
		}
		if !linked {
			return nil
		}
		// Tableau nommé comme l'onglet : il prend le nouveau nom (soletable.go).
		return t.tableFollowsSheet(sheet, old, name)
	})
}

// renameSheet renomme un onglet ; les adresses Feuille!Cell[A1] suivent.
func (t *tx) renameSheet(sheet int64, old, name string) error {
	if err := t.rewriteRefs(refChange{Renames: formula.Renames{Sheets: lowerMap(old, name)}}); err != nil {
		return err
	}
	q := `UPDATE _lattis_sheets SET name = ? WHERE id = ?`
	return t.apply([]stmt{st(q, name, sheet)}, []stmt{st(q, old, sheet)})
}

// DeleteSheet supprime un onglet et tout ce qu'il contient (tableaux,
// cellules, TCD, graphiques, boutons, mise en forme) ; une seule étape
// d'annulation rend tout. Le dernier onglet visible ne se supprime pas.
func (m *Model) DeleteSheet(ctx context.Context, sheet int64) error {
	return m.Do(ctx, "Supprimer la feuille", func(t *tx) error {
		if err := t.guardTab(sheet); err != nil {
			return err
		}
		var n int
		if err := t.row(`SELECT count(*) FROM _lattis_sheets`).Scan(&n); err != nil {
			return err
		}
		if n <= 1 {
			return msg.Wrap(ErrSheet, "sheet.keepOne", nil)
		}
		var others int
		if err := t.row(`SELECT count(*) FROM _lattis_sheets WHERE hidden = 0 AND id <> ?`, sheet).Scan(&others); err != nil {
			return err
		}
		if others == 0 {
			return msg.Wrap(ErrSheet, "sheet.keepOneVisible", nil)
		}
		return t.deleteSheet(sheet)
	})
}

// deleteSheet retire une feuille et tout ce qu'elle contient (sans les
// vérifications de DeleteSheet), annulable avec le reste de l'action.
func (t *tx) deleteSheet(sheet int64) error {
	var name, color string
	var pos, grid, heads, hidden, frows, fcols, zoom int
	var choice sql.NullString
	{
		// Tout ce que contient la feuille part avec elle, tableaux compris
		// (dans la même étape : une annulation rend tout).
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			if err := t.deleteTable(tb); err != nil {
				return err
			}
		}
		if err := t.row(`SELECT name, pos, gridlines, headings, color, hidden, freeze_rows, freeze_cols, zoom, choice FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&name, &pos, &grid, &heads, &color, &hidden, &frows, &fcols, &zoom, &choice); err != nil {
			return err
		}
		cells, err := t.cells(sheet)
		if err != nil {
			return err
		}
		for _, c := range cells {
			if err := t.setFreeCell(sheet, c.Row, c.Col, ""); err != nil {
				return err
			}
		}
		pivots, err := t.pivotRows(sheet)
		if err != nil {
			return err
		}
		charts, err := t.chartRows(sheet)
		if err != nil {
			return err
		}
		buttons, err := t.buttonRows(sheet)
		if err != nil {
			return err
		}
		objects, err := t.objectRows(sheet)
		if err != nil {
			return err
		}
		pivots = append(append(append(pivots, charts...), buttons...), objects...)
		// Valeurs saisies à droite de ses tableaux croisés (pivothand.go).
		if err := t.forgetSheetPivotsHand(sheet); err != nil {
			return err
		}
		// Mise en forme, largeurs et hauteurs de la feuille.
		do := []stmt{st(`DELETE FROM _lattis_pivots WHERE sheet = ?`, sheet), st(`DELETE FROM _lattis_charts WHERE sheet = ?`, sheet), st(`DELETE FROM _lattis_buttons WHERE sheet = ?`, sheet), st(`DELETE FROM _lattis_objects WHERE sheet = ?`, sheet)}
		// Protections de la feuille (contenu, structure) : supprimées avec
		// elle, rendues par l'annulation.
		for _, tbl := range []string{protTable, structTable} {
			prot, err := t.snapshot(tbl, sheet)
			if err != nil {
				return err
			}
			do = append(do, st(`DELETE FROM `+tbl+` WHERE sheet = ?`, sheet))
			pivots = append(pivots, prot...)
		}
		var choiceArg any // catégorie de l'onglet à choix, rendue avec la feuille
		if choice.Valid {
			choiceArg = choice.String
		}
		for _, tbl := range sheetPositioned {
			rows, err := t.snapshot(tbl, sheet)
			if err != nil {
				return err
			}
			do = append(do, st(`DELETE FROM `+tbl+` WHERE sheet = ?`, sheet))
			pivots = append(pivots, rows...)
		}
		return t.apply(
			append(do, st(`DELETE FROM _lattis_sheets WHERE id = ?`, sheet)),
			append([]stmt{st(`INSERT INTO _lattis_sheets (id, name, pos, gridlines, headings, color, hidden, freeze_rows, freeze_cols, zoom, choice) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, sheet, name, pos, grid, heads, color, hidden, frows, fcols, zoom, choiceArg)}, pivots...),
		)
	}
}

// SetGridlines affiche ou masque le quadrillage de la feuille.
func (m *Model) SetGridlines(ctx context.Context, sheet int64, show bool) error {
	return m.Do(ctx, "Quadrillage", func(t *tx) error {
		var old bool
		if err := t.row(`SELECT gridlines <> 0 FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
			return err
		}
		if old == show {
			return nil
		}
		q := `UPDATE _lattis_sheets SET gridlines = ? WHERE id = ?`
		return t.apply([]stmt{st(q, show, sheet)}, []stmt{st(q, old, sheet)})
	})
}

// SetHeadings affiche toujours (ou seulement pendant la saisie d'une
// formule) les lettres de colonnes et numéros de lignes de la feuille.
func (m *Model) SetHeadings(ctx context.Context, sheet int64, show bool) error {
	return m.Do(ctx, "En-têtes A1", func(t *tx) error {
		var old bool
		if err := t.row(`SELECT headings <> 0 FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
			return err
		}
		if old == show {
			return nil
		}
		q := `UPDATE _lattis_sheets SET headings = ? WHERE id = ?`
		return t.apply([]stmt{st(q, show, sheet)}, []stmt{st(q, old, sheet)})
	})
}

// SetColumnWidth fixe la largeur (en pixels) d'une colonne de la feuille ;
// 0 revient à la largeur par défaut.
func (m *Model) SetColumnWidth(ctx context.Context, sheet int64, col, width int) error {
	return m.SetColumnWidths(ctx, sheet, []ColWidth{{Col: col, Width: width}})
}

// SetColumnWidths fixe la largeur de plusieurs colonnes en une seule étape
// d'annulation (ajustement automatique de toutes les colonnes sélectionnées,
// comme Excel) ; une largeur 0 revient à la largeur par défaut.
func (m *Model) SetColumnWidths(ctx context.Context, sheet int64, widths []ColWidth) error {
	name := "Largeur de colonne"
	if len(widths) > 1 {
		name = "Largeur des colonnes"
	}
	return m.Do(ctx, name, func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		for _, w := range widths {
			if err := t.setLineSize(`_lattis_col_widths`, `col`, `width`, sheet, w.Col, w.Width); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetRowHeight fixe la hauteur (en pixels) d'une ligne de la feuille ; 0
// revient à la hauteur par défaut.
func (m *Model) SetRowHeight(ctx context.Context, sheet int64, row, height int) error {
	return m.SetRowHeights(ctx, sheet, []RowHeight{{Row: row, Height: height}})
}

// SetRowHeights fixe la hauteur de plusieurs lignes en une seule étape
// d'annulation (hauteur automatique des lignes sélectionnées) ; 0 revient à
// la hauteur par défaut.
func (m *Model) SetRowHeights(ctx context.Context, sheet int64, heights []RowHeight) error {
	name := "Hauteur de ligne"
	if len(heights) > 1 {
		name = "Hauteur des lignes"
	}
	return m.Do(ctx, name, func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		for _, h := range heights {
			if err := t.setLineSize(`_lattis_row_heights`, `row`, `height`, sheet, h.Row, h.Height); err != nil {
				return err
			}
		}
		return nil
	})
}

// setLineSize fixe la taille d'une ligne ou d'une colonne (table des
// largeurs ou des hauteurs) ; 0 la retire (taille par défaut). Rien n'est
// écrit si elle ne change pas.
func (t *tx) setLineSize(table, at, size string, sheet int64, i, px int) error {
	var old int
	err := t.row(`SELECT `+size+` FROM `+table+` WHERE sheet = ? AND `+at+` = ?`, sheet, i).Scan(&old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if px < 0 {
		px = 0
	}
	if old == px {
		return nil
	}
	set := func(v int) []stmt {
		s := []stmt{st(`DELETE FROM `+table+` WHERE sheet = ? AND `+at+` = ?`, sheet, i)}
		if v > 0 {
			s = append(s, st(`INSERT INTO `+table+` (sheet, `+at+`, `+size+`) VALUES (?, ?, ?)`, sheet, i, v))
		}
		return s
	}
	return t.apply(set(px), set(old))
}

// sheetPositioned sont les tables de métadonnées repérées par feuille et
// position (sans identifiant propre), supprimées et copiées avec la feuille ;
// le contenu d'un onglet page HTML (_lattis_pages) et les zones « plage »
// (_lattis_range_cells) en font partie, comme la mise en page
// (_lattis_page_setup).
var sheetPositioned = []string{`_lattis_pages`, `_lattis_choice_cells`, `_lattis_range_cells`, `_lattis_cell_styles`, `_lattis_row_styles`, `_lattis_col_styles`, `_lattis_row_rules`, `_lattis_col_rules`, `_lattis_col_widths`, `_lattis_row_heights`, `_lattis_page_setup`}

// snapshot renvoie les insertions qui recréent les lignes d'une feuille dans
// une table de métadonnées (pour l'annulation).
func (t *tx) snapshot(table string, sheet int64) ([]stmt, error) {
	rs, err := t.query(`SELECT * FROM `+table+` WHERE sheet = ?`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	q := `INSERT INTO ` + table + ` (` + strings.Join(cols, ", ") + `) VALUES (?` + strings.Repeat(", ?", len(cols)-1) + `)`
	var out []stmt
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		out = append(out, st(q, vals...))
	}
	return out, rs.Err()
}
