package canvas

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/sqltext"
)

// Requêtes SQL d'écriture annulables (onglet SQL, db.execute en Lua).
//
// On ne rejoue pas le SQL de l'utilisateur pour annuler ou rétablir : le
// temps de la requête, des déclencheurs temporaires notent chaque ligne
// touchée et son état d'avant. Après la requête, l'état d'avant et l'état
// d'après de ces lignes sont copiés dans des tables de corbeille ; annuler
// remet l'état d'avant, rétablir l'état d'après. Le journal ne contient que
// quelques instructions, quelle que soit la taille de la requête, et le
// résultat ne dépend pas de ce que la requête calculait (random(), date...).
//
// La requête peut nommer les tableaux par leur nom affiché : chacun devient
// une vue temporaire ("Commandes"("_id", "Client", ...)) dont les
// déclencheurs INSTEAD OF écrivent dans la table t_<id>. Une ligne ajoutée
// par la vue reçoit sa position (_pos) en fin de tableau.
//
// Restent refusés : les changements de structure (CREATE, ALTER, DROP...),
// les métadonnées _lattis_*, ATTACH et les PRAGMA (autorisateur de
// store.AsUser).

// ErrStructure signale une requête qui change la structure de la base : elle
// se fait depuis la grille.
var ErrStructure = sqltext.ErrStructure

const sqlTrash = "_lattis_sqltrash_"

// ExecSQL exécute une requête d'écriture en une action annulable et renvoie
// le nombre de lignes touchées.
func (m *Model) ExecSQL(ctx context.Context, q string, args ...any) (int64, error) {
	var n int64
	err := m.Do(ctx, "Requête SQL", func(t *tx) error {
		var err error
		n, err = t.execSQL(q, args)
		return err
	})
	return n, sqltext.French(err)
}

// Exec exécute une requête d'écriture pour un script (même commande que ses
// autres écritures).
func (e *Editor) Exec(ctx context.Context, q string, args ...any) (int64, error) {
	var n int64
	err := e.do(ctx, func(t *tx) error {
		var err error
		n, err = t.execSQL(q, args)
		return err
	})
	return n, sqltext.French(err)
}

type captured struct {
	Table
	cols []string // colonnes physiques : id, _pos, c_...
}

// execSQL exécute la requête ; une valeur refusée par le type d'une colonne
// (décimal dans une colonne d'entiers, texte dans une colonne de nombres)
// élargit la colonne, comme une saisie au clavier, puis la requête repart.
func (t *tx) execSQL(q string, args []any) (int64, error) {
	for attempt := 0; ; attempt++ {
		n, err := t.execSQLOnce(q, args)
		if err == nil || attempt >= 8 {
			return n, err
		}
		widened, werr := t.widenFor(err)
		if werr != nil {
			return 0, werr
		}
		if !widened {
			return n, err
		}
	}
}

var strictErr = regexp.MustCompile(`cannot store (\w+) value in \w+ column t_(\d+)\.c_(\d+)`)

// widenFor élargit la colonne nommée par une erreur de type STRICT.
func (t *tx) widenFor(err error) (bool, error) {
	m := strictErr.FindStringSubmatch(err.Error())
	if m == nil {
		return false, nil
	}
	tid, _ := strconv.ParseInt(m[2], 10, 64)
	cid, _ := strconv.ParseInt(m[3], 10, 64)
	tb, terr := t.table(tid)
	if terr != nil {
		return false, nil
	}
	var vt grid.Type
	switch m[1] {
	case "REAL":
		vt = grid.Decimal
	case "INTEGER":
		vt = grid.Integer
	default:
		vt = grid.Text
	}
	for i, c := range tb.Cols {
		if c.ID != cid {
			continue
		}
		to := Join(c.Type, vt)
		if to == c.Type {
			return false, nil
		}
		_, rerr := t.retype(tb, i, to)
		return rerr == nil, rerr
	}
	return false, nil
}

func (t *tx) execSQLOnce(q string, args []any) (int64, error) {
	if strings.TrimSpace(q) == "" {
		return 0, errors.New("requête vide")
	}
	if err := sqltext.CheckStructure(q); err != nil {
		return 0, err
	}
	if err := t.dropOrphanTrash(); err != nil {
		return 0, err
	}
	tables, err := t.captureTables()
	if err != nil {
		return 0, err
	}
	setup, teardown, err := t.captureSQL(tables)
	if err != nil {
		return 0, err
	}
	var before, after int64
	if err := t.row(`PRAGMA main.schema_version`).Scan(&before); err != nil {
		return 0, err
	}
	if err := t.exec(setup); err != nil {
		return 0, err
	}
	// La requête de l'utilisateur passe sous l'autorisateur (ni ATTACH, ni
	// structure, ni tables internes : store.AsUser) ; les déclencheurs et
	// vues posés ci-dessus restent ceux du moteur.
	err = t.m.db.AsUser(func() error {
		_, err := t.ExecContext(t.Ctx, q, args...)
		return err
	})
	if err != nil {
		// La requête n'a rien écrit (instruction atomique) : on défait la capture.
		t.exec(teardown)
		for _, tb := range tables {
			t.exec([]string{fmt.Sprintf(`DROP TABLE IF EXISTS temp._slt_ids_%d`, tb.ID), fmt.Sprintf(`DROP TABLE IF EXISTS temp._slt_old_%d`, tb.ID)})
		}
		return 0, err
	}
	if err := t.row(`PRAGMA main.schema_version`).Scan(&after); err != nil {
		return 0, err
	}
	if after != before {
		return 0, ErrStructure
	}
	for _, tb := range tables {
		if err := t.placeNewRows(tb.Table); err != nil {
			return 0, err
		}
	}
	if err := t.exec(teardown); err != nil {
		return 0, err
	}
	// Feuille protégée : une requête qui touche l'un de ses tableaux est
	// refusée (la transaction est défaite).
	for _, tb := range tables {
		var touched int
		if err := t.row(fmt.Sprintf(`SELECT count(*) FROM temp._slt_ids_%d`, tb.ID)).Scan(&touched); err != nil {
			return 0, err
		}
		if touched == 0 {
			continue
		}
		if err := t.guard(tb.Sheet); err != nil {
			for _, tb := range tables {
				t.exec([]string{fmt.Sprintf(`DROP TABLE IF EXISTS temp._slt_ids_%d`, tb.ID), fmt.Sprintf(`DROP TABLE IF EXISTS temp._slt_old_%d`, tb.ID)})
			}
			return 0, err
		}
	}

	var total int64
	for _, tb := range tables {
		n, err := t.recordChanges(tb)
		if err != nil {
			return 0, err
		}
		total += n
	}
	var drops []string
	for _, tb := range tables {
		drops = append(drops, fmt.Sprintf(`DROP TABLE temp._slt_ids_%d`, tb.ID), fmt.Sprintf(`DROP TABLE temp._slt_old_%d`, tb.ID))
	}
	return total, t.exec(drops)
}

func (t *tx) exec(stmts []string) error {
	for _, s := range stmts {
		if _, err := t.ExecContext(t.Ctx, s); err != nil {
			return fmt.Errorf("%s : %w", s, err)
		}
	}
	return nil
}

// captureTables liste tous les tableaux avec leurs colonnes physiques.
func (t *tx) captureTables() ([]captured, error) {
	rs, err := t.query(`SELECT id FROM _lattis_tables ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rs.Close()
	var out []captured
	for _, id := range ids {
		tb, err := t.table(id)
		if err != nil {
			return nil, err
		}
		phys, err := t.physical(tb)
		if err != nil {
			return nil, err
		}
		cols := []string{"id", "_pos"}
		for _, c := range phys {
			cols = append(cols, colSQL(c.ID))
		}
		out = append(out, captured{tb, cols})
	}
	return out, nil
}

// captureSQL prépare les objets temporaires : tables de capture et
// déclencheurs par tableau, vues aux noms affichés, garde des métadonnées.
func (t *tx) captureSQL(tables []captured) (setup, teardown []string, err error) {
	taken := map[string]bool{} // noms d'objets de la base principale
	rs, err := t.query(`SELECT name FROM main.sqlite_schema`)
	if err != nil {
		return nil, nil, err
	}
	var meta []string
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			rs.Close()
			return nil, nil, err
		}
		taken[strings.ToLower(n)] = true
		if strings.HasPrefix(n, "_lattis_") {
			meta = append(meta, n)
		}
	}
	rs.Close()

	for _, name := range meta {
		var typ string
		if err := t.row(`SELECT type FROM main.sqlite_schema WHERE name = ?`, name).Scan(&typ); err != nil {
			return nil, nil, err
		}
		if typ != "table" {
			continue
		}
		for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
			trg := fmt.Sprintf("_slt_guard_%s_%s", name, strings.ToLower(op))
			setup = append(setup, fmt.Sprintf(`CREATE TEMP TRIGGER %s BEFORE %s ON main.%s BEGIN SELECT RAISE(ABORT, 'les tables internes de Lattis ne se modifient pas en SQL'); END`, trg, op, name))
			teardown = append(teardown, `DROP TRIGGER temp.`+trg)
		}
	}

	views := map[string]bool{}
	for _, tb := range tables {
		id, name := tb.ID, tb.sql()
		old := make([]string, len(tb.cols))
		for i, c := range tb.cols {
			old[i] = "old." + c
		}
		setup = append(setup,
			fmt.Sprintf(`CREATE TEMP TABLE _slt_ids_%d (id INTEGER PRIMARY KEY, existed INTEGER NOT NULL)`, id),
			fmt.Sprintf(`CREATE TEMP TABLE _slt_old_%d (%s, PRIMARY KEY (id))`, id, strings.Join(tb.cols, ", ")),
			fmt.Sprintf(`CREATE TEMP TRIGGER _slt_i_%d AFTER INSERT ON main.%s BEGIN
				INSERT OR IGNORE INTO _slt_ids_%d VALUES (new.id, 0); END`, id, name, id),
			fmt.Sprintf(`CREATE TEMP TRIGGER _slt_u_%d AFTER UPDATE ON main.%s BEGIN
				INSERT OR IGNORE INTO _slt_ids_%d VALUES (old.id, 1);
				INSERT OR IGNORE INTO _slt_ids_%d VALUES (new.id, 0);
				INSERT OR IGNORE INTO _slt_old_%d SELECT %s WHERE (SELECT existed FROM _slt_ids_%d WHERE id = old.id) = 1; END`,
				id, name, id, id, id, strings.Join(old, ", "), id),
			fmt.Sprintf(`CREATE TEMP TRIGGER _slt_d_%d AFTER DELETE ON main.%s BEGIN
				INSERT OR IGNORE INTO _slt_ids_%d VALUES (old.id, 1);
				INSERT OR IGNORE INTO _slt_old_%d SELECT %s WHERE (SELECT existed FROM _slt_ids_%d WHERE id = old.id) = 1; END`,
				id, name, id, id, strings.Join(old, ", "), id),
		)
		teardown = append(teardown,
			fmt.Sprintf(`DROP TRIGGER temp._slt_i_%d`, id),
			fmt.Sprintf(`DROP TRIGGER temp._slt_u_%d`, id),
			fmt.Sprintf(`DROP TRIGGER temp._slt_d_%d`, id),
		)

		// Vue au nom affiché, sauf si le nom est déjà celui d'un objet de la
		// base (elle le masquerait) ou d'un autre tableau.
		key := strings.ToLower(tb.Name)
		if taken[key] || views[key] || strings.HasPrefix(key, "_slt_") {
			continue
		}
		views[key] = true
		view := quoteIdent(tb.Name)
		// Une colonne calculée se lit par sa formule (WHERE "Montant HT" > 100
		// voit ses valeurs) et ne s'écrit pas : la vue la rend, les
		// déclencheurs l'ignorent.
		var calc []string
		for _, c := range tb.Cols {
			if c.Formula != "" {
				exprs, args, computed, err := t.columnSQL(tb.Table)
				if err != nil {
					return nil, nil, err
				}
				calc = make([]string, len(exprs))
				for i := range exprs {
					if computed[i] {
						if calc[i], err = inlineSQL(exprs[i], args); err != nil {
							return nil, nil, err
						}
					}
				}
				break
			}
		}
		names := []string{quoteIdent("_id")}
		read := []string{"id"}
		used := map[string]bool{"_id": true}
		set := []string{`id = new."_id"`}
		insCols, insVals := []string{"id", "_pos"}, []string{`new."_id"`, "''"}
		for i, c := range tb.Cols {
			n := c.Name
			for k := 2; used[strings.ToLower(n)]; k++ {
				n = fmt.Sprintf("%s (%d)", c.Name, k)
			}
			used[strings.ToLower(n)] = true
			names = append(names, quoteIdent(n))
			if calc != nil && calc[i] != "" {
				read = append(read, calc[i])
				continue
			}
			read = append(read, colSQL(c.ID))
			set = append(set, colSQL(c.ID)+" = new."+quoteIdent(n))
			insCols = append(insCols, colSQL(c.ID))
			insVals = append(insVals, "new."+quoteIdent(n))
		}
		setup = append(setup,
			fmt.Sprintf(`CREATE TEMP VIEW %s (%s) AS SELECT %s FROM main.%s AS %s`, view, strings.Join(names, ", "), strings.Join(read, ", "), name, name),
			fmt.Sprintf(`CREATE TEMP TRIGGER _slt_vi_%d INSTEAD OF INSERT ON %s BEGIN
				INSERT INTO %s (%s) VALUES (%s); END`, id, view, name, strings.Join(insCols, ", "), strings.Join(insVals, ", ")),
			fmt.Sprintf(`CREATE TEMP TRIGGER _slt_vu_%d INSTEAD OF UPDATE ON %s BEGIN
				UPDATE %s SET %s WHERE id = old."_id"; END`, id, view, name, strings.Join(set, ", ")),
			fmt.Sprintf(`CREATE TEMP TRIGGER _slt_vd_%d INSTEAD OF DELETE ON %s BEGIN
				DELETE FROM %s WHERE id = old."_id"; END`, id, view, name),
		)
		// Supprimer la vue supprime ses déclencheurs.
		teardown = append(teardown, `DROP VIEW temp.`+view)
	}
	return setup, teardown, nil
}

// placeNewRows donne une position en fin de tableau aux lignes ajoutées sans
// position (par une vue), dans l'ordre de leur clé.
func (t *tx) placeNewRows(tb Table) error {
	rs, err := t.query(`SELECT id FROM ` + tb.sql() + ` WHERE _pos = '' ORDER BY id`)
	if err != nil {
		return err
	}
	var ids []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return err
		}
		ids = append(ids, id)
	}
	rs.Close()
	if len(ids) == 0 {
		return nil
	}
	var last string
	if err := t.row(`SELECT coalesce(max(_pos), '') FROM ` + tb.sql() + ` WHERE _pos <> ''`).Scan(&last); err != nil {
		return err
	}
	// Ajouts en fin : rowKey incrémente la dernière clé, qui reste courte.
	for _, id := range ids {
		key, err := t.rowKey(tb, last, "")
		if err != nil {
			return err
		}
		if _, err := t.ExecContext(t.Ctx, `UPDATE `+tb.sql()+` SET _pos = ? WHERE id = ?`, key, id); err != nil {
			return err
		}
		last = key
	}
	return nil
}

// recordChanges enregistre l'étape d'annulation d'un tableau touché par la
// requête, nettoie ses saisies invalides et décale ce qui est dessous s'il a
// grandi. Renvoie le nombre de lignes touchées.
func (t *tx) recordChanges(tb captured) (int64, error) {
	var touched, inserted, deleted int64
	ids := fmt.Sprintf("temp._slt_ids_%d", tb.ID)
	err := t.row(`SELECT count(*),
		coalesce(sum(existed = 0 AND id IN (SELECT id FROM `+tb.sql()+`)), 0),
		coalesce(sum(existed = 1 AND id NOT IN (SELECT id FROM `+tb.sql()+`)), 0)
		FROM `+ids).Scan(&touched, &inserted, &deleted)
	if err != nil || touched == 0 {
		return 0, err
	}
	trash := fmt.Sprintf("%s%d_%d", sqlTrash, tb.ID, uniq())
	cols := strings.Join(tb.cols, ", ")
	err = t.exec([]string{
		`CREATE TABLE ` + trash + `_ids AS SELECT id FROM ` + ids,
		`CREATE TABLE ` + trash + `_old AS SELECT ` + cols + ` FROM temp._slt_old_` + fmt.Sprint(tb.ID),
		`CREATE TABLE ` + trash + `_new AS SELECT ` + cols + ` FROM ` + tb.sql() + ` WHERE id IN (SELECT id FROM ` + ids + `)`,
	})
	if err != nil {
		return 0, err
	}
	restore := func(from string) []stmt {
		return []stmt{
			st(`DELETE FROM ` + tb.sql() + ` WHERE id IN (SELECT id FROM ` + trash + `_ids)`),
			st(`INSERT INTO ` + tb.sql() + ` (` + cols + `) SELECT ` + cols + ` FROM ` + trash + from),
		}
	}
	// Do remet l'état d'après, qui est déjà celui de la table : sans effet
	// maintenant, c'est ce que rejouera Rétablir.
	err = t.ApplyStep(command.Step{Do: restore("_new"), Undo: restore("_old"), Purge: []stmt{
		st(`DROP TABLE IF EXISTS ` + trash + `_ids`),
		st(`DROP TABLE IF EXISTS ` + trash + `_old`),
		st(`DROP TABLE IF EXISTS ` + trash + `_new`),
	}})
	if err != nil {
		return 0, err
	}
	if err := t.cleanInvalid(tb.Table, trash+"_ids"); err != nil {
		return 0, err
	}
	if grow := int(inserted - deleted); grow > 0 {
		// tb.Rows est le nombre de lignes d'avant la requête.
		if err := t.ensureGapBelow(tb.Sheet, tb.ID, tb.Bottom(), tb.Bottom()+grow, tb.Col, tb.Right()); err != nil {
			return 0, err
		}
	}
	return touched, nil
}

// cleanInvalid retire les saisies invalides devenues sans objet dans les
// lignes touchées : ligne supprimée, ou cellule qui a reçu une valeur.
func (t *tx) cleanInvalid(tb Table, ids string) error {
	rs, err := t.query(`SELECT rid, col, raw FROM _lattis_invalid WHERE tbl = ? AND rid IN (SELECT id FROM `+ids+`)`, tb.ID)
	if err != nil {
		return err
	}
	type inv struct {
		rid, col int64
		raw      string
	}
	var all []inv
	for rs.Next() {
		var i inv
		if err := rs.Scan(&i.rid, &i.col, &i.raw); err != nil {
			rs.Close()
			return err
		}
		all = append(all, i)
	}
	rs.Close()
	for _, i := range all {
		var keep int
		err := t.row(`SELECT count(*) FROM `+tb.sql()+` WHERE id = ? AND `+colSQL(i.col)+` IS NULL`, i.rid).Scan(&keep)
		if err != nil {
			// Colonne disparue de la table : la saisie n'a plus de cellule.
			keep = 0
		}
		if keep == 1 {
			continue
		}
		q := `DELETE FROM _lattis_invalid WHERE tbl = ? AND rid = ? AND col = ?`
		if err := t.apply(
			[]stmt{st(q, tb.ID, i.rid, i.col)},
			[]stmt{st(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) VALUES (?, ?, ?, ?)`, tb.ID, i.rid, i.col, i.raw)},
		); err != nil {
			return err
		}
	}
	return nil
}

// dropOrphanTrash supprime les corbeilles de requêtes dont la commande a
// quitté le journal sans passer par Purge (branche de rétablissement
// abandonnée par une nouvelle action).
func (t *tx) dropOrphanTrash() error {
	rs, err := t.query(`SELECT name FROM main.sqlite_schema WHERE type = 'table' AND name LIKE ? ESCAPE '\'`, strings.ReplaceAll(sqlTrash, "_", `\_`)+"%")
	if err != nil {
		return err
	}
	var names []string
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			rs.Close()
			return err
		}
		names = append(names, n)
	}
	rs.Close()
	var drops []string
	for _, n := range names {
		var used int
		if err := t.row(`SELECT count(*) FROM _lattis_journal WHERE instr(steps, ?) > 0`, n).Scan(&used); err != nil {
			return err
		}
		if used == 0 {
			drops = append(drops, `DROP TABLE `+n)
		}
	}
	return t.exec(drops)
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
