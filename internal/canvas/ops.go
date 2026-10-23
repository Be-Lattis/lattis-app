package canvas

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/order"
)

// --- Opérations élémentaires (chacune enregistre son inverse) ---------------

// setFreeCell pose ou efface (raw vide) une cellule libre.
func (t *tx) setFreeCell(sheet int64, row, col int, raw string) error {
	var old string
	err := t.row(`SELECT raw FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&old)
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if !exists && raw == "" || exists && old == raw {
		return nil
	}
	del := st(`DELETE FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col)
	put := func(v string) stmt {
		return st(`INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (?, ?, ?, ?)`, sheet, row, col, v)
	}
	do := []stmt{del}
	if raw != "" {
		do = append(do, put(raw))
	}
	undo := []stmt{del}
	if exists {
		undo = append(undo, put(old))
	}
	return t.apply(do, undo)
}

// setValue écrit une valeur typée dans une cellule de tableau ; invalid
// (non vide) enregistre une saisie hors type, la cellule valant alors NULL.
func (t *tx) setValue(tb Table, rid int64, c Column, v any, invalid string) error {
	var old any
	if err := t.row(`SELECT `+colSQL(c.ID)+` FROM `+tb.sql()+` WHERE id = ?`, rid).Scan(&old); err != nil {
		return err
	}
	var oldInvalid string
	err := t.row(`SELECT raw FROM _lattis_invalid WHERE tbl = ? AND rid = ? AND col = ?`, tb.ID, rid, c.ID).Scan(&oldInvalid)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if b, ok := old.([]byte); ok {
		old = string(b)
	}
	if old == v && oldInvalid == invalid {
		return nil
	}
	write := func(v any, inv string) []stmt {
		s := []stmt{
			st(`UPDATE `+tb.sql()+` SET `+colSQL(c.ID)+` = ? WHERE id = ?`, v, rid),
			st(`DELETE FROM _lattis_invalid WHERE tbl = ? AND rid = ? AND col = ?`, tb.ID, rid, c.ID),
		}
		if inv != "" {
			s = append(s, st(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) VALUES (?, ?, ?, ?)`, tb.ID, rid, c.ID, inv))
		}
		return s
	}
	return t.apply(write(v, invalid), write(old, oldInvalid))
}

// insertRow ajoute une ligne vide de clé id à la position pos. Sous un
// onglet à choix, elle reçoit la valeur choisie (choice.go) ; dans une
// colonne numérotée, son numéro (autokey.go). Une valeur saisie ensuite
// dans la même action remplace ce numéro.
func (t *tx) insertRow(tb Table, id int64, pos string) error {
	auto, numbered, err := t.autoNumber(tb)
	if err != nil {
		return err
	}
	if !numbered {
		return t.insertRowNumbered(tb, id, pos, nil)
	}
	return t.insertRowNumbered(tb, id, pos, &auto)
}

// insertRowNumbered est insertRow avec le numéro déjà calculé (auto : nil
// hors colonne numérotée), pour plusieurs lignes ajoutées d'un coup.
func (t *tx) insertRowNumbered(tb Table, id int64, pos string, auto *autoNum) error {
	if err := t.apply(
		[]stmt{st(`INSERT INTO `+tb.sql()+` (id, _pos) VALUES (?, ?)`, id, pos)},
		[]stmt{st(`DELETE FROM `+tb.sql()+` WHERE id = ?`, id)},
	); err != nil {
		return err
	}
	if auto != nil {
		if err := t.numberRow(tb, *auto, id, pos); err != nil {
			return err
		}
	}
	return t.imposeChoice(tb, id)
}

// deleteRow supprime une ligne ; l'inverse la réinsère à l'identique (même
// clé, même position, mêmes valeurs et saisies invalides).
func (t *tx) deleteRow(tb Table, rid int64) error {
	names := []string{"id", "_pos"}
	for _, c := range tb.Cols {
		names = append(names, colSQL(c.ID))
	}
	vals := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := t.row(`SELECT `+strings.Join(names, ", ")+` FROM `+tb.sql()+` WHERE id = ?`, rid).Scan(ptrs...); err != nil {
		return err
	}
	undo := []stmt{st(`INSERT INTO `+tb.sql()+` (`+strings.Join(names, ", ")+`) VALUES (`+marks(len(names))+`)`, normalize(vals)...)}
	rs, err := t.query(`SELECT col, raw FROM _lattis_invalid WHERE tbl = ? AND rid = ?`, tb.ID, rid)
	if err != nil {
		return err
	}
	for rs.Next() {
		var col int64
		var raw string
		if err := rs.Scan(&col, &raw); err != nil {
			rs.Close()
			return err
		}
		undo = append(undo, st(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) VALUES (?, ?, ?, ?)`, tb.ID, rid, col, raw))
	}
	rs.Close()
	// Les styles de la ligne partent avec elle (et reviennent à l'annulation).
	rs, err = t.query(`SELECT col, style FROM _lattis_styles WHERE tbl = ? AND rid = ?`, tb.ID, rid)
	if err != nil {
		return err
	}
	for rs.Next() {
		var col int64
		var style string
		if err := rs.Scan(&col, &style); err != nil {
			rs.Close()
			return err
		}
		undo = append(undo, st(`INSERT INTO _lattis_styles (tbl, rid, col, style) VALUES (?, ?, ?, ?)`, tb.ID, rid, col, style))
	}
	rs.Close()
	return t.apply([]stmt{
		st(`DELETE FROM _lattis_invalid WHERE tbl = ? AND rid = ?`, tb.ID, rid),
		st(`DELETE FROM _lattis_styles WHERE tbl = ? AND rid = ?`, tb.ID, rid),
		st(`DELETE FROM `+tb.sql()+` WHERE id = ?`, rid),
	}, undo)
}

// addColumn ajoute une colonne au rang pos.
func (t *tx) addColumn(tb Table, id int64, name string, typ grid.Type, pos int) error {
	return t.apply([]stmt{
		st(`ALTER TABLE ` + tb.sql() + ` ADD COLUMN ` + colSQL(id) + ` ` + sqlType(typ)),
		st(`UPDATE _lattis_columns SET pos = pos + 1 WHERE tbl = ? AND pos >= ?`, tb.ID, pos),
		st(`INSERT INTO _lattis_columns (tbl, id, name, type, pos) VALUES (?, ?, ?, ?, ?)`, tb.ID, id, name, string(typ), pos),
	}, []stmt{
		st(`DELETE FROM _lattis_invalid WHERE tbl = ? AND col = ?`, tb.ID, id),
		st(`DELETE FROM _lattis_columns WHERE tbl = ? AND id = ?`, tb.ID, id),
		st(`UPDATE _lattis_columns SET pos = pos - 1 WHERE tbl = ? AND pos > ?`, tb.ID, pos),
		st(`ALTER TABLE ` + tb.sql() + ` DROP COLUMN ` + colSQL(id)),
	})
}

// dropColumn supprime une colonne. Ses valeurs partent dans une table de
// corbeille, d'où l'annulation les remet : l'inverse reste en quelques
// instructions quelle que soit la taille du tableau.
func (t *tx) dropColumn(tb Table, c Column, pos int) error {
	trash := fmt.Sprintf("_lattis_trash_%d_%d_%d", tb.ID, c.ID, uniq())
	col := colSQL(c.ID)
	rules, err := t.columnRules(tb.ID, c.ID)
	if err != nil {
		return err
	}
	// Supprimer la colonne clé retire d'abord la clé (SQLite refuse de
	// supprimer une colonne indexée) ; l'annulation la rétablit.
	var unkey, rekey []stmt
	if c.ID == tb.Key {
		unkey = []stmt{
			st(`DROP INDEX ` + tb.sql() + `_key`),
			st(`UPDATE _lattis_tables SET key_col = NULL WHERE id = ?`, tb.ID),
		}
		rekey = []stmt{
			st(`CREATE UNIQUE INDEX ` + tb.sql() + `_key ON ` + tb.sql() + `(` + keyExpr(c.ID, c.Type) + `)`),
			st(`UPDATE _lattis_tables SET key_col = ? WHERE id = ?`, c.ID, tb.ID),
		}
	}
	return t.ApplyStep(command.Step{Do: append(unkey, []stmt{
		// Corbeille indexée sur id : l'annulation relit chaque ligne par sa clé
		// (sans index, 1 M de lignes demandaient 1 M de parcours complets).
		st(`CREATE TABLE ` + trash + ` (id INTEGER PRIMARY KEY, v)`),
		// Cellules vides laissées de côté : la colonne rétablie est vide
		// partout ailleurs (une colonne vide ne coûte qu'un parcours).
		st(`INSERT INTO ` + trash + ` (id, v) SELECT id, ` + col + ` FROM ` + tb.sql() + ` WHERE ` + col + ` IS NOT NULL`),
		st(`CREATE TABLE `+trash+`_inv AS SELECT rid, raw FROM _lattis_invalid WHERE tbl = ? AND col = ?`, tb.ID, c.ID),
		st(`DELETE FROM _lattis_invalid WHERE tbl = ? AND col = ?`, tb.ID, c.ID),
		st(`CREATE TABLE `+trash+`_sty AS SELECT rid, style FROM _lattis_styles WHERE tbl = ? AND col = ?`, tb.ID, c.ID),
		st(`DELETE FROM _lattis_styles WHERE tbl = ? AND col = ?`, tb.ID, c.ID),
		st(`ALTER TABLE ` + tb.sql() + ` DROP COLUMN ` + col),
		st(`DELETE FROM _lattis_columns WHERE tbl = ? AND id = ?`, tb.ID, c.ID),
		st(`UPDATE _lattis_columns SET pos = pos - 1 WHERE tbl = ? AND pos > ?`, tb.ID, pos),
	}...), Undo: append([]stmt{
		st(`UPDATE _lattis_columns SET pos = pos + 1 WHERE tbl = ? AND pos >= ?`, tb.ID, pos),
		st(`INSERT INTO _lattis_columns (tbl, id, name, type, fixed, pos, formula, rules) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			tb.ID, c.ID, c.Name, string(c.Type), c.Fixed, pos, nullIfEmpty(c.Formula), nullIfEmpty(rules)),
		st(`ALTER TABLE ` + tb.sql() + ` ADD COLUMN ` + col + ` ` + sqlType(c.Type)),
		st(`UPDATE ` + tb.sql() + ` SET ` + col + ` = ` + trash + `.v FROM ` + trash + ` WHERE ` + trash + `.id = ` + tb.sql() + `.id`),
		st(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) SELECT ?, rid, ?, raw FROM `+trash+`_inv`, tb.ID, c.ID),
		st(`INSERT INTO _lattis_styles (tbl, rid, col, style) SELECT ?, rid, ?, style FROM `+trash+`_sty`, tb.ID, c.ID),
		st(`DROP TABLE ` + trash),
		st(`DROP TABLE ` + trash + `_inv`),
		st(`DROP TABLE ` + trash + `_sty`),
	}, rekey...), Purge: []stmt{
		// La commande sort de l'historique : la corbeille ne servira plus.
		st(`DROP TABLE IF EXISTS ` + trash),
		st(`DROP TABLE IF EXISTS ` + trash + `_inv`),
		st(`DROP TABLE IF EXISTS ` + trash + `_sty`),
	}})
}

// columnNameTaken indique si une autre colonne du tableau que except porte
// déjà ce nom, sans tenir compte des majuscules : Ventes[Produit] serait
// ambigu dans les formules, comme en SQL.
func columnNameTaken(tb Table, except int64, name string) bool {
	for _, c := range tb.Cols {
		if c.ID != except && strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

// freeColumnNameExcept rend name, ou « name 2 », « name 3 »… s'il est pris dans
// le tableau par une autre colonne que except (création automatique :
// collage, conversion).
func freeColumnNameExcept(tb Table, except int64, name string) string {
	cand := name
	for n := 2; columnNameTaken(tb, except, cand); n++ {
		cand = fmt.Sprintf("%s %d", name, n)
	}
	return cand
}

// errColumnTaken : message d'une colonne renommée en doublon.
func errColumnTaken(tb Table, name string) error {
	return msg.New("table.columnExists", msg.Args{"column": name, "table": tb.Name})
}

// uniqueColumnNames complète (« Colonne n ») et rend uniques des en-têtes
// d'un nouveau tableau : le second « Produit » devient « Produit 2 ».
func uniqueColumnNames(headers []string) []string {
	out := make([]string, len(headers))
	seen := map[string]bool{}
	for i, h := range headers {
		h = strings.TrimSpace(h)
		if h == "" {
			h = fmt.Sprintf("Colonne %d", i+1)
		}
		cand := h
		for n := 2; seen[strings.ToLower(cand)]; n++ {
			cand = fmt.Sprintf("%s %d", h, n)
		}
		seen[strings.ToLower(cand)] = true
		out[i] = cand
	}
	return out
}

// rename change le nom affiché d'une colonne.
func (t *tx) rename(tb Table, c Column, name string) error {
	if name == c.Name {
		return nil
	}
	// Ce qui cite la colonne suit (formules, TCD, graphiques, requêtes).
	var rc refChange
	rc.column(tb.Name, c.Name, name)
	if err := t.rewriteRefs(rc); err != nil {
		return err
	}
	q := `UPDATE _lattis_columns SET name = ? WHERE tbl = ? AND id = ?`
	return t.apply([]stmt{st(q, name, tb.ID, c.ID)}, []stmt{st(q, c.Name, tb.ID, c.ID)})
}

// rebuildStmts reconstruit la table avec la colonne colID au type to (SQLite
// ne sait pas changer le type d'une colonne) : nouvelle table, copie avec
// conversion, suppression de l'ancienne, renommage, index. Les colonnes
// gardent leur ordre physique (cols), pour que l'annulation redonne
// exactement le même schéma.
func rebuildStmts(tb Table, physical []Column, colID int64, from, to grid.Type) []stmt {
	name, tmp := tb.sql(), tb.sql()+"_rebuild"
	defs := []string{"id INTEGER PRIMARY KEY", "_pos TEXT NOT NULL"}
	cols := []string{"id", "_pos"}
	sel := []string{"id", "_pos"}
	for _, c := range physical {
		typ, expr := c.Type, colSQL(c.ID)
		if c.ID == colID {
			typ, expr = to, convertExpr(colSQL(c.ID), from, to)
		}
		defs = append(defs, colSQL(c.ID)+" "+sqlType(typ))
		cols = append(cols, colSQL(c.ID))
		sel = append(sel, expr)
	}
	out := []stmt{
		st(`CREATE TABLE ` + tmp + ` (` + strings.Join(defs, ", ") + `) STRICT`),
		st(`INSERT INTO ` + tmp + ` (` + strings.Join(cols, ", ") + `) SELECT ` + strings.Join(sel, ", ") + ` FROM ` + name),
		st(`DROP TABLE ` + name),
		st(`ALTER TABLE ` + tmp + ` RENAME TO ` + name),
		st(`CREATE INDEX ` + name + `_pos ON ` + name + `(_pos)`),
	}
	// La clé du tableau (REFERENCE) garde son index UNIQUE.
	for _, c := range physical {
		if c.ID == tb.Key {
			typ := c.Type
			if c.ID == colID {
				typ = to
			}
			out = append(out, st(`CREATE UNIQUE INDEX `+name+`_key ON `+name+`(`+keyExpr(c.ID, typ)+`)`))
		}
	}
	return append(out, st(`UPDATE _lattis_columns SET type = ? WHERE tbl = ? AND id = ?`, string(to), tb.ID, colID))
}

// keyExpr est l'expression de l'index de la clé (formula.KeyExpr).
func keyExpr(id int64, t grid.Type) string { return formula.KeyExpr(colSQL(id), t) }

// retype change le type de stockage d'une colonne (reconstruction).
func (t *tx) retype(tb Table, i int, to grid.Type) (Table, error) {
	c := tb.Cols[i]
	physical, err := t.physical(tb)
	if err != nil {
		return tb, err
	}
	if err := t.apply(rebuildStmts(tb, physical, c.ID, c.Type, to), rebuildStmts(tb, physical, c.ID, to, c.Type)); err != nil {
		return tb, err
	}
	tb.Cols = append([]Column(nil), tb.Cols...)
	tb.Cols[i].Type = to
	return tb, nil
}

// createTable crée un tableau à partir de saisies : en-têtes et lignes de
// données, types inférés colonne par colonne.
func (t *tx) createTable(sheet int64, name string, row, col int, headers []string, data [][]string) (Table, error) {
	var id int64
	if err := t.row(nextTableID).Scan(&id); err != nil {
		return Table{}, err
	}
	tb := Table{ID: id, Sheet: sheet, Name: name, Row: row, Col: col, Rows: len(data)}
	defs := []string{"id INTEGER PRIMARY KEY", "_pos TEXT NOT NULL"}
	names := []string{"id", "_pos"}
	do := []stmt{st(`INSERT INTO _lattis_tables (id, sheet, name, row, col) VALUES (?, ?, ?, ?, ?)`, id, sheet, name, row, col)}
	for i, h := range uniqueColumnNames(headers) {
		raws := make([]string, len(data))
		for r := range data {
			raws[r] = data[r][i]
		}
		c := Column{ID: int64(i + 1), Name: h, Type: InferAll(raws)}
		tb.Cols = append(tb.Cols, c)
		defs = append(defs, colSQL(c.ID)+" "+sqlType(c.Type))
		names = append(names, colSQL(c.ID))
		do = append(do, st(`INSERT INTO _lattis_columns (tbl, id, name, type, pos) VALUES (?, ?, ?, ?, ?)`, id, c.ID, c.Name, string(c.Type), i))
	}
	do = append([]stmt{
		st(`CREATE TABLE ` + tb.sql() + ` (` + strings.Join(defs, ", ") + `) STRICT`),
		st(`CREATE INDEX ` + tb.sql() + `_pos ON ` + tb.sql() + `(_pos)`),
	}, do...)
	keys := order.Spread(len(data))
	for r, raws := range data {
		args := []any{int64(r + 1), keys[r]}
		for i, raw := range raws {
			// Apostrophe de tête ('001) : elle a fait choisir le texte, elle n'est pas gardée.
			forced := tb.Cols[i].Type == grid.Text && len(raw) > 1 && raw[0] == '\''
			if forced {
				raw = raw[1:]
			}
			v, err := grid.Parse(tb.Cols[i].Type, raw)
			if err != nil {
				return Table{}, err
			}
			if forced {
				v = raw // « 'null » : le texte null
			}
			args = append(args, v)
		}
		do = append(do, st(`INSERT INTO `+tb.sql()+` (`+strings.Join(names, ", ")+`) VALUES (`+marks(len(names))+`)`, args...))
	}
	undo := []stmt{
		st(`DELETE FROM _lattis_invalid WHERE tbl = ?`, id),
		st(`DELETE FROM _lattis_columns WHERE tbl = ?`, id),
		st(`DELETE FROM _lattis_tables WHERE id = ?`, id),
		st(`DROP TABLE ` + tb.sql()),
	}
	return tb, t.apply(do, undo)
}

// TableSchema donne les instructions qui créent la table de données d'un
// tableau (comme createTable : STRICT, index de l'ordre, index unique de la
// clé key si elle n'est pas 0). Sert au chargement d'un classeur partagé
// depuis un Serveur Lattis, qui reçoit les métadonnées toutes faites.
func TableSchema(id int64, cols []Column, key int64) []string {
	tb := Table{ID: id, Cols: cols}
	defs := []string{"id INTEGER PRIMARY KEY", "_pos TEXT NOT NULL"}
	for _, c := range cols {
		defs = append(defs, colSQL(c.ID)+" "+sqlType(c.Type))
	}
	out := []string{
		`CREATE TABLE ` + tb.sql() + ` (` + strings.Join(defs, ", ") + `) STRICT`,
		`CREATE INDEX ` + tb.sql() + `_pos ON ` + tb.sql() + `(_pos)`,
	}
	for _, c := range cols {
		if key != 0 && c.ID == key {
			out = append(out, `CREATE UNIQUE INDEX `+tb.sql()+`_key ON `+tb.sql()+`(`+keyExpr(c.ID, c.Type)+`)`)
		}
	}
	return out
}

// ColumnDef donne la définition SQL de la colonne id d'un tableau (ALTER
// TABLE … ADD COLUMN : colonne calculée reçue d'un classeur collaboratif).
func ColumnDef(id int64, t grid.Type) string { return colSQL(id) + " " + sqlType(t) }

// nextTableID donne l'identifiant d'un nouveau tableau. Celui d'un tableau
// publié sur un Serveur Lattis ne resert jamais, même supprimé : son
// identifiant le désigne dans la file d'envoi et sur le serveur, et
// l'annulation peut le rétablir. Celui qu'une requête d'import désigne
// encore comme son tableau chargé (tableau supprimé avec sa feuille, ou
// seul) non plus : un nouveau tableau, un résultat copié par exemple, serait
// pris pour le sien et remplacé par elle à l'actualisation (F5).
const nextTableID = `SELECT max(coalesce((SELECT max(id) FROM _lattis_tables), 0), coalesce((SELECT max(tbl) FROM _lattis_remote_tables), 0), coalesce((SELECT max(target) FROM _lattis_queries), 0)) + 1`

func marks(n int) string { return strings.TrimSuffix(strings.Repeat("?, ", n), ", ") }

func normalize(vals []any) []any {
	for i, v := range vals {
		if b, ok := v.([]byte); ok {
			vals[i] = string(b)
		}
	}
	return vals
}

// physical donne les colonnes dans l'ordre du schéma SQL (l'ordre d'ajout),
// qui diffère de l'ordre d'affichage après une insertion de colonne.
func (t *tx) physical(tb Table) ([]Column, error) {
	byName := map[string]Column{}
	for _, c := range tb.Cols {
		byName[colSQL(c.ID)] = c
	}
	rs, err := t.query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, tb.sql())
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []Column
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			return nil, err
		}
		if c, ok := byName[n]; ok {
			out = append(out, c)
		}
	}
	return out, rs.Err()
}

// notAllNull : condition SQL « au moins une de ces colonnes n'est pas vide ».
func notAllNull(cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = c + ` IS NOT NULL`
	}
	return `(` + strings.Join(parts, ` OR `) + `)`
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// dropColumns supprime plusieurs colonnes d'un coup (positions d'affichage
// idx, croissantes). SQLite réécrit toute la table à chaque DROP COLUMN :
// avec plusieurs colonnes, une seule reconstruction de la table les retire
// toutes. Leurs valeurs partent dans une corbeille indexée sur id, avec
// leurs saisies invalides et leur mise en forme ; l'annulation reconstruit
// la table d'origine (même schéma) en y rejoignant la corbeille.
func (t *tx) dropColumns(tb Table, idx []int) error {
	if len(idx) == 1 {
		return t.dropColumn(tb, tb.Cols[idx[0]], idx[0])
	}
	physical, err := t.physical(tb)
	if err != nil {
		return err
	}
	gone := map[int64]Column{}
	var ids []string
	for _, i := range idx {
		gone[tb.Cols[i].ID] = tb.Cols[i]
		ids = append(ids, strconv.FormatInt(tb.Cols[i].ID, 10))
	}
	trash := fmt.Sprintf("_lattis_trash_%d_cols_%d", tb.ID, uniq())
	name, tmp := tb.sql(), tb.sql()+"_rebuild"
	in := strings.Join(ids, ", ")

	// Corbeille : id puis une colonne par colonne supprimée.
	tdefs := []string{"id INTEGER PRIMARY KEY"}
	tcols, tsel := []string{"id"}, []string{"id"}
	for _, c := range physical {
		if _, ok := gone[c.ID]; ok {
			tdefs = append(tdefs, colSQL(c.ID))
			tcols = append(tcols, colSQL(c.ID))
			tsel = append(tsel, colSQL(c.ID))
		}
	}
	// Schéma reconstruit : sans (do) ou avec (undo) les colonnes supprimées.
	build := func(keep bool) []stmt {
		defs := []string{"id INTEGER PRIMARY KEY", "_pos TEXT NOT NULL"}
		cols := []string{"id", "_pos"}
		sel := []string{name + ".id", name + "._pos"}
		for _, c := range physical {
			_, dropped := gone[c.ID]
			if dropped && !keep {
				continue
			}
			defs = append(defs, colSQL(c.ID)+" "+sqlType(c.Type))
			cols = append(cols, colSQL(c.ID))
			if dropped {
				sel = append(sel, trash+"."+colSQL(c.ID))
			} else {
				sel = append(sel, name+"."+colSQL(c.ID))
			}
		}
		from := name
		if keep {
			from = name + " LEFT JOIN " + trash + " ON " + trash + ".id = " + name + ".id"
		}
		out := []stmt{
			st(`CREATE TABLE ` + tmp + ` (` + strings.Join(defs, ", ") + `) STRICT`),
			st(`INSERT INTO ` + tmp + ` (` + strings.Join(cols, ", ") + `) SELECT ` + strings.Join(sel, ", ") + ` FROM ` + from),
			st(`DROP TABLE ` + name),
			st(`ALTER TABLE ` + tmp + ` RENAME TO ` + name),
			st(`CREATE INDEX ` + name + `_pos ON ` + name + `(_pos)`),
		}
		for _, c := range physical {
			if _, dropped := gone[c.ID]; c.ID == tb.Key && (keep || !dropped) {
				out = append(out, st(`CREATE UNIQUE INDEX `+name+`_key ON `+name+`(`+keyExpr(c.ID, c.Type)+`)`))
			}
		}
		return out
	}

	do := []stmt{
		st(`CREATE TABLE ` + trash + ` (` + strings.Join(tdefs, ", ") + `)`),
		// Lignes vides dans toutes les colonnes supprimées laissées de côté :
		// l'annulation les rejoint en LEFT JOIN (NULL).
		st(`INSERT INTO ` + trash + ` (` + strings.Join(tcols, ", ") + `) SELECT ` + strings.Join(tsel, ", ") + ` FROM ` + name + ` WHERE ` + notAllNull(tsel[1:])),
		st(`CREATE TABLE `+trash+`_inv AS SELECT rid, col, raw FROM _lattis_invalid WHERE tbl = ? AND col IN (`+in+`)`, tb.ID),
		st(`DELETE FROM _lattis_invalid WHERE tbl = ? AND col IN (`+in+`)`, tb.ID),
		st(`CREATE TABLE `+trash+`_sty AS SELECT rid, col, style FROM _lattis_styles WHERE tbl = ? AND col IN (`+in+`)`, tb.ID),
		st(`DELETE FROM _lattis_styles WHERE tbl = ? AND col IN (`+in+`)`, tb.ID),
	}
	if _, ok := gone[tb.Key]; ok && tb.Key != 0 {
		do = append(do, st(`UPDATE _lattis_tables SET key_col = NULL WHERE id = ?`, tb.ID))
	}
	do = append(do, build(false)...)
	// Colonnes retirées des métadonnées, de la dernière à la première.
	var undo []stmt
	for k := len(idx) - 1; k >= 0; k-- {
		c := tb.Cols[idx[k]]
		do = append(do,
			st(`DELETE FROM _lattis_columns WHERE tbl = ? AND id = ?`, tb.ID, c.ID),
			st(`UPDATE _lattis_columns SET pos = pos - 1 WHERE tbl = ? AND pos > ?`, tb.ID, idx[k]))
	}
	for _, i := range idx {
		c := tb.Cols[i]
		rules, err := t.columnRules(tb.ID, c.ID)
		if err != nil {
			return err
		}
		undo = append(undo,
			st(`UPDATE _lattis_columns SET pos = pos + 1 WHERE tbl = ? AND pos >= ?`, tb.ID, i),
			st(`INSERT INTO _lattis_columns (tbl, id, name, type, fixed, pos, formula, rules) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				tb.ID, c.ID, c.Name, string(c.Type), c.Fixed, i, nullIfEmpty(c.Formula), nullIfEmpty(rules)))
	}
	undo = append(undo, build(true)...)
	if _, ok := gone[tb.Key]; ok && tb.Key != 0 {
		undo = append(undo, st(`UPDATE _lattis_tables SET key_col = ? WHERE id = ?`, tb.Key, tb.ID))
	}
	undo = append(undo,
		st(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) SELECT ?, rid, col, raw FROM `+trash+`_inv`, tb.ID),
		st(`INSERT INTO _lattis_styles (tbl, rid, col, style) SELECT ?, rid, col, style FROM `+trash+`_sty`, tb.ID),
		st(`DROP TABLE `+trash),
		st(`DROP TABLE `+trash+`_inv`),
		st(`DROP TABLE `+trash+`_sty`),
	)
	return t.ApplyStep(command.Step{Do: do, Undo: undo, Purge: []stmt{
		st(`DROP TABLE IF EXISTS ` + trash),
		st(`DROP TABLE IF EXISTS ` + trash + `_inv`),
		st(`DROP TABLE IF EXISTS ` + trash + `_sty`),
	}})
}
