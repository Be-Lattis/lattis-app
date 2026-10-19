package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
)

// Feuilles à plusieurs tableaux sous un onglet à choix (docs/conception/
// regrouper-onglets-semblables.md §6.5, lot 4).
//
// Cellules par valeur : une cellule libre qui changeait d'un onglet à
// l'autre (« Objectif : 5 000 » en 2024, « 6 000 » en 2025, le titre
// « Janvier 2025 ») garde une valeur par valeur de la catégorie dans
// _lattis_choice_cells, et la cellule libre (_lattis_cells) montre celle de
// la valeur choisie (« * » : « (Toutes) »). Tout ce qui écrit une cellule
// libre continue d'écrire _lattis_cells ; à la fin de chaque action
// (syncChoiceCells), ce qu'on lit aux positions par valeur est rangé sous la
// valeur choisie, dans le journal. Changer de valeur (navigation, hors
// journal) recopie les cellules de la nouvelle valeur ; une annulation aussi
// (undoGuard), car les cellules libres qu'elle remet peuvent être celles
// d'une autre valeur.
//
// Suivre l'année : un tableau suit le choix de l'onglet s'il a la colonne de
// catégorie, sauf réglage contraire gardé avec le tableau
// (_lattis_tables.follow = 0, « Suivre l'année » décoché sur un tableau dont
// la colonne contient plusieurs années, qu'on ne veut pas perdre).

const choiceCellsTable = "_lattis_choice_cells"

// choiceAllKey : la clé des cellules montrées avec « (Toutes) ».
const choiceAllKey = "*"

// choiceKey : la clé des cellules par valeur pour un choix.
func choiceKey(r *choiceRaw) string {
	if r.All {
		return choiceAllKey
	}
	return r.Value
}

// ChoiceKeyText : la clé d'une valeur choisie (texte affiché ; « * » pour
// « (Toutes) »).
func ChoiceKeyText(value string, all bool) string {
	if all {
		return choiceAllKey
	}
	return value
}

// storeChoiceCells range, dans le journal, ce que montrent les cellules par
// valeur de la feuille sous la clé key.
func (t *tx) storeChoiceCells(sheet int64, key string) error {
	rs, err := t.query(`SELECT p.row, p.col, c.raw, v.raw FROM (SELECT DISTINCT row, col FROM _lattis_choice_cells WHERE sheet = ?1) AS p
		LEFT JOIN _lattis_cells c ON c.sheet = ?1 AND c.row = p.row AND c.col = p.col
		LEFT JOIN _lattis_choice_cells v ON v.sheet = ?1 AND v.row = p.row AND v.col = p.col AND v.value = ?2`, sheet, key)
	if err != nil {
		return err
	}
	var do, undo []stmt
	for rs.Next() {
		var row, col int
		var shown, stored sql.NullString
		if err := rs.Scan(&row, &col, &shown, &stored); err != nil {
			rs.Close()
			return err
		}
		if shown == stored {
			continue
		}
		del := st(`DELETE FROM _lattis_choice_cells WHERE sheet = ? AND row = ? AND col = ? AND value = ?`, sheet, row, col, key)
		put := func(raw string) stmt {
			return st(`INSERT INTO _lattis_choice_cells (sheet, row, col, value, raw) VALUES (?, ?, ?, ?, ?)`, sheet, row, col, key, raw)
		}
		do, undo = append(do, del), append(undo, del)
		if shown.Valid && shown.String != "" {
			do = append(do, put(shown.String))
		}
		if stored.Valid {
			undo = append(undo, put(stored.String))
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil || len(do) == 0 {
		return err
	}
	return t.apply(do, undo)
}

// showChoiceCells montre, dans le journal, les cellules par valeur de la clé
// key (une cellule sans valeur pour cette clé est vide).
func (t *tx) showChoiceCells(sheet int64, key string) error {
	rs, err := t.query(`SELECT p.row, p.col, coalesce(v.raw, '') FROM (SELECT DISTINCT row, col FROM _lattis_choice_cells WHERE sheet = ?1) AS p
		LEFT JOIN _lattis_choice_cells v ON v.sheet = ?1 AND v.row = p.row AND v.col = p.col AND v.value = ?2`, sheet, key)
	if err != nil {
		return err
	}
	var cells []cell
	for rs.Next() {
		var c cell
		if err := rs.Scan(&c.Row, &c.Col, &c.Raw); err != nil {
			rs.Close()
			return err
		}
		cells = append(cells, c)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	for _, c := range cells {
		if err := t.setFreeCell(sheet, c.Row, c.Col, c.Raw); err != nil {
			return err
		}
	}
	return nil
}

// syncChoiceCells : à la fin d'une action, ce qu'elle a écrit aux positions
// par valeur est rangé sous la valeur choisie de leur feuille.
func (t *tx) syncChoiceCells() error {
	if t.ro {
		return nil
	}
	sheets, err := t.choiceCellSheets()
	if err != nil {
		return err
	}
	for _, s := range sheets {
		r, err := t.readChoice(s)
		if err != nil {
			return err
		}
		if r == nil {
			continue // plus d'onglet à choix : les cellules restent telles quelles
		}
		if err := t.storeChoiceCells(s, choiceKey(r)); err != nil {
			return err
		}
	}
	return nil
}

func (t *tx) choiceCellSheets() ([]int64, error) {
	rs, err := t.query(`SELECT DISTINCT sheet FROM _lattis_choice_cells ORDER BY sheet`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []int64
	for rs.Next() {
		var s int64
		if err := rs.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rs.Err()
}

// showChoiceCellsRaw montre les cellules par valeur de la clé key, hors
// journal (changement de valeur, annulation).
func showChoiceCellsRaw(ctx context.Context, tx *sql.Tx, sheet int64, key string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_cells WHERE sheet = ?1 AND (row, col) IN (SELECT row, col FROM _lattis_choice_cells WHERE sheet = ?1)`, sheet); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO _lattis_cells (sheet, row, col, raw) SELECT sheet, row, col, raw FROM _lattis_choice_cells WHERE sheet = ? AND value = ? AND raw <> ''`, sheet, key)
	return err
}

// showAllChoiceCellsRaw remet, hors journal, les cellules par valeur de
// chaque feuille à onglet à choix sur la valeur choisie.
func showAllChoiceCellsRaw(ctx context.Context, tx *sql.Tx) error {
	rs, err := tx.QueryContext(ctx, `SELECT s.id, s.choice FROM _lattis_sheets s WHERE s.choice IS NOT NULL AND s.choice <> ''
		AND EXISTS (SELECT 1 FROM _lattis_choice_cells c WHERE c.sheet = s.id)`)
	if err != nil {
		return err
	}
	type fix struct {
		sheet int64
		key   string
	}
	var fixes []fix
	for rs.Next() {
		var id int64
		var raw string
		if err := rs.Scan(&id, &raw); err != nil {
			rs.Close()
			return err
		}
		var r choiceRaw
		if json.Unmarshal([]byte(raw), &r) == nil {
			fixes = append(fixes, fix{id, choiceKey(&r)})
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	for _, f := range fixes {
		if err := showChoiceCellsRaw(ctx, tx, f.sheet, f.key); err != nil {
			return err
		}
	}
	return nil
}

// ShowChoiceCells remet, hors journal, les cellules par valeur de chaque
// feuille à onglet à choix sur la valeur choisie : après une mise en page
// reçue du serveur (remote/workbook), dont les cellules libres sont celles
// de la valeur choisie par l'auteur.
func ShowChoiceCells(ctx context.Context, tx *sql.Tx) error { return showAllChoiceCellsRaw(ctx, tx) }

// undoGuard : la garde des annulations (protection), puis les cellules par
// valeur remises sur la valeur choisie.
func (m *Model) undoGuard(ctx context.Context, sqlTx *sql.Tx, seq int64) (func() error, error) {
	check, err := m.historyGuard(ctx, sqlTx, seq)
	if err != nil {
		return nil, err
	}
	return func() error {
		if err := showAllChoiceCellsRaw(ctx, sqlTx); err != nil {
			return err
		}
		return check()
	}, nil
}

// ChoiceCell est une cellule par valeur (pour la grille et l'API).
type ChoiceCell struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Value string `json:"value"` // « * » : « (Toutes) »
	Raw   string `json:"raw"`
}

// ChoiceCells donne les cellules par valeur d'une feuille.
func (m *Model) ChoiceCells(ctx context.Context, sheet int64) ([]ChoiceCell, error) {
	out := []ChoiceCell{}
	err := m.read(ctx, func(t *tx) error {
		rs, err := t.query(`SELECT row, col, value, raw FROM _lattis_choice_cells WHERE sheet = ? ORDER BY row, col, value`, sheet)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var c ChoiceCell
			if err := rs.Scan(&c.Row, &c.Col, &c.Value, &c.Raw); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rs.Err()
	})
	return out, err
}

// FollowLabel : « l'année », « le mois », « la ville »… pour « Suivre
// l'année » ; « « Lot » » sinon.
func FollowLabel(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "année":
		return "l'année"
	case "mois":
		return "le mois"
	case "trimestre":
		return "le trimestre"
	case "semaine":
		return "la semaine"
	case "ville":
		return "la ville"
	case "date":
		return "la date"
	case "catégorie":
		return "la catégorie"
	}
	return "« " + name + " »"
}

// SetTableFollow coche (follow) ou décoche « Suivre l'année » sur un
// tableau d'une feuille à onglet à choix ; une étape d'annulation. Le cocher
// ajoute la colonne de catégorie (en dernier, masquée tant qu'une valeur est
// choisie), remplie avec la valeur affichée, et lève un « ne pas suivre »
// gardé. Le décocher retire la colonne si elle n'a qu'une valeur (rien ne se
// perd) ; sinon la colonne reste, et le tableau ne suit plus le choix
// (réglage gardé avec le tableau). Rend le message de la barre d'état.
func (m *Model) SetTableFollow(ctx context.Context, table string, follow bool) (string, error) {
	var notice string
	label := "Suivre l'onglet à choix"
	if !follow {
		label = "Ne plus suivre l'onglet à choix"
	}
	err := m.Do(ctx, label, func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if err := t.guard(tb.Sheet); err != nil {
			return err
		}
		notice, err = t.setTableFollow(tb, follow)
		if notice != "" {
			t.notes = append(t.notes, notice)
		}
		return err
	})
	return notice, err
}

// setFollowFlag écrit _lattis_tables.follow (nil : selon la colonne).
func (t *tx) setFollowFlag(tb Table, v any) error {
	var old sql.NullInt64
	if err := t.row(`SELECT follow FROM _lattis_tables WHERE id = ?`, tb.ID).Scan(&old); err != nil {
		return err
	}
	var prev any
	if old.Valid {
		prev = old.Int64
	}
	if fmt.Sprint(prev) == fmt.Sprint(v) {
		return nil
	}
	q := `UPDATE _lattis_tables SET follow = ? WHERE id = ?`
	return t.apply([]stmt{st(q, v, tb.ID)}, []stmt{st(q, prev, tb.ID)})
}

func (t *tx) setTableFollow(tb Table, follow bool) (string, error) {
	r, err := t.readChoice(tb.Sheet)
	if err != nil {
		return "", err
	}
	ch, err := t.sheetChoice(tb.Sheet)
	if err != nil {
		return "", err
	}
	if r == nil || ch == nil {
		return "", fmt.Errorf("%w : la feuille n'a pas d'onglet à choix (clic droit sur l'onglet › Choisir par…)", ErrSheet)
	}
	what := FollowLabel(ch.Name)
	col, has := choiceColumn(tb, ch.Name)
	if follow {
		if err := t.setFollowFlag(tb, nil); err != nil {
			return "", err
		}
		if has {
			return fmt.Sprintf("« %s » suit %s", tb.Name, what), nil
		}
		v := ""
		if !ch.All {
			v = ch.Value
		}
		if err := t.addChoiceColumn(tb, ch, r, v); err != nil {
			return "", err
		}
		if v == "" {
			return fmt.Sprintf("« %s » suit %s : sa colonne « %s » est à remplir (onglet sur « (Toutes) »)", tb.Name, what, ch.Name), nil
		}
		return fmt.Sprintf("« %s » suit %s (%s)", tb.Name, what, v), nil
	}
	if !has {
		return "", t.setFollowFlag(tb, nil)
	}
	if r.Tbl == tb.ID && r.Col == col.ID {
		return "", fmt.Errorf("%w : « %s » porte la colonne de l'onglet à choix ; retirez plutôt le choix (clic droit sur l'onglet › Ne plus choisir)", ErrTable, tb.Name)
	}
	var n int
	if err := t.row(`SELECT count(DISTINCT ` + textExpr(col) + `) FROM ` + tb.sql() + ` WHERE ` + colSQL(col.ID) + ` IS NOT NULL`).Scan(&n); err != nil {
		return "", err
	}
	if n <= 1 {
		// Une seule valeur (ou aucune) : la colonne ne dit rien de plus.
		i := colIndex(tb, col.Name)
		if len(tb.Cols) == 1 {
			return "", t.setFollowFlag(tb, 0)
		}
		if err := t.dropColumn(tb, col, i); err != nil {
			return "", err
		}
		if err := t.setFollowFlag(tb, nil); err != nil {
			return "", err
		}
		return fmt.Sprintf("« %s » ne suit plus %s : il est le même pour toutes les valeurs (colonne « %s » retirée)", tb.Name, what, col.Name), t.trimAfterDrop(tb.ID)
	}
	if err := t.setFollowFlag(tb, 0); err != nil {
		return "", err
	}
	return fmt.Sprintf("« %s » ne suit plus %s ; sa colonne « %s » est gardée (elle contient %d valeurs différentes)", tb.Name, what, col.Name, n), nil
}

// addChoiceColumn ajoute la colonne de catégorie en dernier, au type de la
// colonne source, avec sa liste déroulante, remplie avec value.
func (t *tx) addChoiceColumn(tb Table, ch *SheetChoice, r *choiceRaw, value string) error {
	var id int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_columns WHERE tbl = ?`, tb.ID).Scan(&id); err != nil {
		return err
	}
	if columnNameTaken(tb, 0, ch.Name) {
		return errColumnTaken(tb, ch.Name)
	}
	typ := ch.Type
	if typ == "" {
		typ = grid.Text
	}
	pos := len(tb.Cols)
	if err := t.addColumn(tb, id, ch.Name, typ, pos); err != nil {
		return err
	}
	q := `UPDATE _lattis_columns SET fixed = ? WHERE tbl = ? AND id = ?`
	if err := t.apply([]stmt{st(q, 1, tb.ID, id)}, []stmt{st(q, 0, tb.ID, id)}); err != nil {
		return err
	}
	if value != "" {
		v, err := grid.Parse(typ, value)
		if err != nil {
			return fmt.Errorf("%w : « %s » ne convient pas à la colonne « %s » (%v)", ErrTable, value, ch.Name, err)
		}
		if err := t.apply([]stmt{st(`UPDATE `+tb.sql()+` SET `+colSQL(id)+` = ?`, v)}, []stmt{st(`UPDATE ` + tb.sql() + ` SET ` + colSQL(id) + ` = NULL`)}); err != nil {
			return err
		}
	}
	if r.Tbl != 0 {
		list, err := t.choiceList(r.Tbl, r.Col)
		if err != nil {
			return err
		}
		if len(list) > 0 {
			vals := make([]any, len(list))
			for i, v := range list {
				vals[i] = v
			}
			if err := t.styleTable(tb.ID, 0, id, Style{"dv": vals}, nil); err != nil {
				return err
			}
		}
	}
	return t.ensureGapRight(tb.Sheet, tb.ID, tb.Right(), tb.Right()+1, tb.Row, tb.Bottom())
}

// followNewTable : un tableau créé sur une feuille réglée sur une valeur
// précise la suit (§6.5) : on était dans l'onglet 2025, ce qu'on tape
// appartient à 2025. Avec « (Toutes) », il ne suit pas.
func (t *tx) followNewTable(id int64) error {
	tb, err := t.table(id)
	if err != nil {
		return err
	}
	r, err := t.readChoice(tb.Sheet)
	if err != nil || r == nil || r.All || r.Value == "" {
		return err
	}
	ch, err := t.sheetChoice(tb.Sheet)
	if err != nil || ch == nil {
		return err
	}
	if _, has := choiceColumn(tb, ch.Name); has || columnNameTaken(tb, 0, ch.Name) {
		return nil
	}
	if _, err := grid.Parse(ch.Type, ch.Value); err != nil {
		return nil
	}
	if err := t.addChoiceColumn(tb, ch, r, ch.Value); err != nil {
		if errors.Is(err, ErrTable) {
			return nil
		}
		return err
	}
	t.notes = append(t.notes, fmt.Sprintf("Ce tableau suit %s (%s). Clic droit › Tableau › Suivre %s pour ne plus le suivre", FollowLabel(ch.Name), ch.Value, FollowLabel(ch.Name)))
	return nil
}
