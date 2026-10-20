package workbook

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/store"
)

// Source désigne le classeur du serveur à charger.
type Source struct {
	Server     string
	ServerName string
	User       string
	WorkbookID string
	// Title : titre du classeur (liste du serveur) ; vide au rechargement,
	// le titre déjà noté est gardé.
	Title string
}

// Load charge un classeur du serveur dans le classeur local db (Fichier ›
// Ouvrir depuis un serveur, puis à chaque nouvelle version) : mise en page,
// tableaux publiés et leurs lignes, scripts, dans une seule transaction
// locale.
//
// fresh : db est un classeur neuf, remplacé en entier. Sinon (rechargement),
// ce que l'utilisateur a ajouté en local depuis le dernier chargement est
// gardé : tableaux privés, feuilles, cellules libres et autres éléments de
// mise en page qui ne viennent pas du serveur (reconnus par différence avec
// la mise en page reçue la dernière fois). Le serveur l'emporte sur une
// même cellule ; un tableau ou une feuille privés dont l'identifiant ou le
// nom est repris par la nouvelle version sont renumérotés ou renommés.
func Load(ctx context.Context, srv Server, db *store.Store, src Source, fresh bool) (*Info, error) {
	var info *Info
	err := srv.Read(ctx, func(r Reader) error {
		caps, err := r.Capabilities(ctx, src.WorkbookID)
		if err != nil {
			return err
		}
		since, err := r.SyncPoint(ctx)
		if err != nil {
			return err
		}
		tables, err := r.Tables(ctx, caps.Schema)
		if err != nil {
			return err
		}
		raw, err := r.Meta(ctx, caps.Schema, layoutKind, layoutKey)
		if err != nil {
			return err
		}
		layout := &Layout{Key: layoutKey, Format: meta.Version, Tables: map[string]*LayoutRows{}}
		if raw != nil {
			if layout, err = decodeLayout(raw); err != nil {
				return err
			}
		}
		if layout.Format > meta.Version {
			return msg.New("workbook.newerFormat", msg.Args{"format": layout.Format, "version": meta.Version})
		}
		scripts, err := r.Scripts(ctx, caps.Schema)
		if err != nil {
			return err
		}
		info = &Info{
			Server: src.Server, ServerName: src.ServerName, WorkbookID: src.WorkbookID, Schema: caps.Schema, Channel: caps.Channel,
			Mode: caps.Mode, Version: caps.Version, Roles: caps.Roles, User: src.User, Synced: time.Now().UTC(),
		}
		// Titre : celui du serveur (my_capabilities), à défaut celui de la
		// liste, à défaut celui déjà noté (serveur d'avant la migration 0007).
		info.Title = caps.Title
		if info.Title == "" {
			info.Title = src.Title
		}
		if old, err := ReadInfo(ctx, db.Read()); err == nil && old != nil && old.WorkbookID == src.WorkbookID && info.Title == "" {
			info.Title = old.Title
		}
		l := &loader{ctx: ctx, r: r, info: info, caps: caps, since: since, tables: tables, layout: layout, scripts: scripts, fresh: fresh}
		return db.Write(ctx, l.run)
	})
	return info, err
}

type loader struct {
	ctx     context.Context
	tx      *sql.Tx
	r       Reader
	info    *Info
	caps    Capabilities
	since   string
	tables  []RemoteTable
	layout  *Layout
	scripts []Script
	fresh   bool

	// Ce que l'utilisateur a ajouté en local (rechargement), et la mise en
	// page reçue la dernière fois (vide pour un classeur neuf).
	local map[string]*LayoutRows
	base  *Layout
	// Tableaux privés gardés : identifiant d'avant → identifiant d'après.
	private map[int64]int64
	// Feuilles ajoutées en local : identifiant d'avant → d'après.
	sheets map[int64]int64
	// Tableaux publiés d'avant (à remplacer).
	oldPub map[int64]bool
	// lattis_id → rid des lignes chargées, par tableau.
	rids map[int64]map[string]int64
}

func (l *loader) exec(q string, args ...any) error {
	_, err := l.tx.ExecContext(l.ctx, q, args...)
	return err
}

func (l *loader) run(tx *sql.Tx) error {
	l.tx = tx
	l.private, l.sheets, l.oldPub, l.rids = map[int64]int64{}, map[int64]int64{}, map[int64]bool{}, map[int64]map[string]int64{}
	l.base = &Layout{Tables: map[string]*LayoutRows{}}
	if err := l.readOld(); err != nil {
		return err
	}
	// Tableaux privés mis de côté sous un nom provisoire (leurs saisies
	// invalides sous un identifiant négatif).
	for id := range l.private {
		if err := l.exec(`ALTER TABLE t_` + strconv.FormatInt(id, 10) + ` RENAME TO _lattis_keep_` + strconv.FormatInt(id, 10)); err != nil {
			return err
		}
		if err := l.exec(`UPDATE _lattis_invalid SET tbl = ? WHERE tbl = ?`, -id, id); err != nil {
			return err
		}
	}
	// Tableaux de données remplacés : ceux publiés d'avant et ceux que la
	// nouvelle version apporte (un classeur neuf n'a que les siens).
	drop := maps.Clone(l.oldPub)
	for _, t := range l.layoutTableIDs() {
		drop[t] = true
	}
	for _, t := range l.tables {
		drop[t.LocalID] = true
	}
	if l.fresh {
		ids, err := l.allDataTables()
		if err != nil {
			return err
		}
		for _, id := range ids {
			drop[id] = true
		}
	}
	for id := range drop {
		if _, kept := l.private[id]; kept {
			continue
		}
		if err := l.exec(`DROP TABLE IF EXISTS t_` + strconv.FormatInt(id, 10)); err != nil {
			return err
		}
	}
	// Choix des onglets à choix (valeur choisie propre au poste, voir
	// choice.go), remis après la nouvelle mise en page.
	choices, err := localChoices(l.ctx, tx)
	if err != nil {
		return err
	}
	// Ordre inverse de la liste : les lignes qui citent une feuille ou un
	// tableau partent avant eux (clés étrangères).
	for _, name := range slices.Backward(layoutTables) {
		cols, err := tableColumns(l.ctx, tx, name)
		if err != nil {
			return err
		}
		if len(cols) == 0 {
			continue
		}
		if err := l.exec(`DELETE FROM ` + name); err != nil {
			return err
		}
	}
	for t := range drop {
		if _, kept := l.private[t]; kept {
			continue
		}
		for _, q := range []string{`DELETE FROM _lattis_row_ids WHERE tbl = ?`, `DELETE FROM _lattis_invalid WHERE tbl = ?`, `DELETE FROM _lattis_remote_tables WHERE tbl = ?`} {
			if err := l.exec(q, t); err != nil {
				return err
			}
		}
	}
	// Nouvelle mise en page (les mises en forme par ligne attendent les
	// lignes, pour retrouver leurs rid).
	if err := l.ensureLayout(); err != nil {
		return err
	}
	for _, name := range layoutTables {
		rows := l.layout.Tables[name]
		if rows == nil || l.byRow(rows) {
			continue
		}
		if err := insertRows(l.ctx, tx, name, rows.Columns, rows.Rows, false); err != nil {
			return err
		}
	}
	if err := l.loadTables(); err != nil {
		return err
	}
	for _, name := range layoutTables {
		rows := l.layout.Tables[name]
		if rows == nil || !l.byRow(rows) {
			continue
		}
		if err := insertRows(l.ctx, tx, name, rows.Columns, l.toRids(rows), true); err != nil {
			return err
		}
	}
	if err := l.restoreLocal(); err != nil {
		return err
	}
	if err := applyTheme(l.ctx, tx, l.layout); err != nil {
		return err
	}
	if err := l.mergeChoices(choices); err != nil {
		return err
	}
	if err := l.loadScripts(); err != nil {
		return err
	}
	local := map[string]int64{}
	for _, t := range l.tables {
		local[t.ID] = t.LocalID
	}
	if err := writeCapabilities(l.ctx, tx, l.info, &l.caps, local); err != nil {
		return err
	}
	if err := l.exec(`DELETE FROM _lattis_outbox`); err != nil {
		return err
	}
	if err := command.Forget(l.ctx, tx); err != nil {
		return err
	}
	if err := writeInfo(l.ctx, tx, l.info); err != nil {
		return err
	}
	base, err := json.Marshal(l.layout)
	if err != nil {
		return err
	}
	// Identifiants de la mise en page : ceux du poste, relus du serveur à la
	// première synchronisation (livelayout.go).
	if err := l.exec(`DELETE FROM _lattis_info WHERE key = ?`, idsKey); err != nil {
		return err
	}
	return writeBase(l.ctx, tx, base)
}

// byRow : table dont des lignes désignent une ligne de tableau (tbl, rid),
// publiées par lattis_id.
func (l *loader) byRow(r *LayoutRows) bool { return r.index("tbl") >= 0 && r.index("rid") >= 0 }

func (l *loader) layoutTableIDs() []int64 {
	var out []int64
	if r := l.layout.Tables["_lattis_tables"]; r != nil {
		at := r.index("id")
		for _, row := range r.Rows {
			if id, ok := asInt(row[at]); ok {
				out = append(out, id)
			}
		}
	}
	return out
}

func (l *loader) allDataTables() ([]int64, error) {
	rs, err := l.tx.QueryContext(l.ctx, `SELECT name FROM main.sqlite_master WHERE type = 'table' AND name GLOB 't_[0-9]*'`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []int64
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			return nil, err
		}
		if id, err := strconv.ParseInt(strings.TrimPrefix(n, "t_"), 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return out, rs.Err()
}

// readOld relève, avant de tout remplacer, les tableaux publiés d'avant et
// ce que l'utilisateur a ajouté en local depuis le dernier chargement.
func (l *loader) readOld() error {
	if l.fresh {
		return nil
	}
	rs, err := l.tx.QueryContext(l.ctx, `SELECT tbl FROM _lattis_remote_tables WHERE published = 1`)
	if err != nil {
		return err
	}
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return err
		}
		l.oldPub[id] = true
	}
	rs.Close()
	ids, err := readRowIDs(l.ctx, l.tx, slices.Collect(maps.Keys(l.oldPub)))
	if err != nil {
		return err
	}
	cur, err := dumpLayout(l.ctx, l.tx, meta.Version, nil, ids)
	if err != nil {
		return err
	}
	baseRaw, err := readBase(l.ctx, l.tx)
	if err != nil {
		return err
	}
	base := &Layout{Tables: map[string]*LayoutRows{}}
	if baseRaw != nil {
		if base, err = decodeLayout(baseRaw); err != nil {
			return err
		}
	}
	l.base = base
	baseTables := idSet(base.Tables["_lattis_tables"], "id")
	baseSheets := idSet(base.Tables["_lattis_sheets"], "id")
	l.local = map[string]*LayoutRows{}
	for name, rows := range cur.Tables {
		seen := map[string]bool{}
		if b := base.Tables[name]; b != nil {
			for _, row := range b.project(rows.Columns) {
				seen[rowKey(row)] = true
			}
		}
		lr := &LayoutRows{Columns: rows.Columns}
		tblAt := rows.index("tbl")
		if name == "_lattis_tables" {
			tblAt = rows.index("id")
		}
		for k, row := range rows.project(rows.Columns) {
			if seen[rowKey(row)] {
				continue
			}
			if tblAt >= 0 {
				// Ce qui touche un tableau venu du serveur est à lui.
				tbl, _ := asInt(row[tblAt])
				if l.oldPub[tbl] || baseTables[tbl] {
					continue
				}
			}
			if name == "_lattis_sheets" {
				id, _ := asInt(row[rows.index("id")])
				if baseSheets[id] {
					continue // feuille du serveur réglée en local : le serveur l'emporte
				}
			}
			lr.Rows = append(lr.Rows, rows.Rows[k])
		}
		if len(lr.Rows) > 0 {
			l.local[name] = lr
		}
	}
	if r := l.local["_lattis_tables"]; r != nil {
		at := r.index("id")
		for _, row := range r.Rows {
			if id, ok := asInt(row[at]); ok {
				l.private[id] = id
			}
		}
	}
	// Feuilles citées par ce qui est gardé : ajoutées en local, ou du serveur
	// (disparues de la nouvelle version : elles reviennent en local).
	newSheets := idSet(l.layout.Tables["_lattis_sheets"], "id")
	need := map[int64]bool{}
	for _, rows := range l.local {
		if at := rows.index("sheet"); at >= 0 {
			for _, row := range rows.Rows {
				if id, ok := asInt(row[at]); ok {
					need[id] = true
				}
			}
		}
	}
	if r := l.local["_lattis_sheets"]; r != nil {
		at := r.index("id")
		for _, row := range r.Rows {
			id, _ := asInt(row[at])
			need[id] = true
		}
	}
	for id := range need {
		if newSheets[id] && baseSheets[id] {
			continue // feuille du serveur toujours là
		}
		if !idSet(l.local["_lattis_sheets"], "id")[id] {
			// Feuille du serveur disparue : reprise de la mise en page reçue.
			if b := base.Tables["_lattis_sheets"]; b != nil {
				for _, row := range b.project(cur.Tables["_lattis_sheets"].Columns) {
					if v, _ := asInt(row[0]); v == id {
						lr := l.local["_lattis_sheets"]
						if lr == nil {
							lr = &LayoutRows{Columns: cur.Tables["_lattis_sheets"].Columns}
							l.local["_lattis_sheets"] = lr
						}
						lr.Rows = append(lr.Rows, row)
					}
				}
			}
		}
		l.sheets[id] = id
	}
	return nil
}

func idSet(r *LayoutRows, col string) map[int64]bool {
	out := map[int64]bool{}
	if r == nil {
		return out
	}
	at := r.index(col)
	if at < 0 {
		return out
	}
	for _, row := range r.Rows {
		if id, ok := asInt(row[at]); ok {
			out[id] = true
		}
	}
	return out
}

// ensureLayout complète une mise en page qui ne décrit pas tous les
// tableaux du serveur (publiés sans elle) : une feuille, et chaque tableau
// sous le précédent.
func (l *loader) ensureLayout() error {
	have := idSet(l.layout.Tables["_lattis_tables"], "id")
	var missing []RemoteTable
	used := map[int64]bool{}
	for i := range l.tables {
		t := &l.tables[i]
		if t.LocalID == 0 || used[t.LocalID] {
			t.LocalID = 0
		}
		if t.LocalID != 0 {
			used[t.LocalID] = true
		}
	}
	next := int64(1)
	for _, id := range slices.Sorted(maps.Keys(have)) {
		next = max(next, id+1)
	}
	for id := range used {
		next = max(next, id+1)
	}
	for i := range l.tables {
		t := &l.tables[i]
		if t.LocalID == 0 {
			t.LocalID = next
			next++
		}
		if !have[t.LocalID] {
			missing = append(missing, *t)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sheets := l.layout.Tables["_lattis_sheets"]
	if sheets == nil || len(sheets.Rows) == 0 {
		sheets = &LayoutRows{Columns: []string{"id", "name", "pos"}, Rows: [][]any{{int64(1), "Feuille1", int64(0)}}}
		l.layout.Tables["_lattis_sheets"] = sheets
	}
	sheet, _ := asInt(sheets.Rows[0][sheets.index("id")])
	tbls := l.layout.Tables["_lattis_tables"]
	if tbls == nil {
		tbls = &LayoutRows{Columns: []string{"id", "sheet", "name", "row", "col"}}
		l.layout.Tables["_lattis_tables"] = tbls
	}
	cols := l.layout.Tables["_lattis_columns"]
	if cols == nil {
		cols = &LayoutRows{Columns: []string{"tbl", "id", "name", "type", "fixed", "pos"}}
		l.layout.Tables["_lattis_columns"] = cols
	}
	row := int64(0)
	for _, t := range missing {
		trow := make([]any, len(tbls.Columns))
		set := func(r *LayoutRows, vals []any, k string, v any) {
			if i := r.index(k); i >= 0 {
				vals[i] = v
			}
		}
		set(tbls, trow, "id", t.LocalID)
		set(tbls, trow, "sheet", sheet)
		set(tbls, trow, "name", t.Name)
		set(tbls, trow, "row", row)
		set(tbls, trow, "col", int64(0))
		set(tbls, trow, "no_key_hint", int64(0))
		tbls.Rows = append(tbls.Rows, trow)
		for i, c := range t.Columns {
			crow := make([]any, len(cols.Columns))
			set(cols, crow, "tbl", t.LocalID)
			set(cols, crow, "id", int64(i+1))
			set(cols, crow, "name", c.Name)
			set(cols, crow, "type", string(LocalType(c.Type)))
			set(cols, crow, "fixed", int64(1))
			set(cols, crow, "pos", int64(i))
			cols.Rows = append(cols.Rows, crow)
			t.Columns[i].LocalID = int64(i + 1)
		}
		for i := range l.tables {
			if l.tables[i].ID == t.ID {
				l.tables[i].Columns = t.Columns
			}
		}
		row += 3 // en-tête, une ligne, une ligne vide (le tableau grandit vers le bas)
	}
	return nil
}

// layoutColumns donne les colonnes d'un tableau de la mise en page, et sa
// clé.
func (l *loader) layoutColumns(tbl int64) ([]canvas.Column, int64) {
	var out []canvas.Column
	if r := l.layout.Tables["_lattis_columns"]; r != nil {
		tAt, idAt, nAt, tyAt := r.index("tbl"), r.index("id"), r.index("name"), r.index("type")
		for _, row := range r.Rows {
			if t, _ := asInt(row[tAt]); t != tbl {
				continue
			}
			id, _ := asInt(row[idAt])
			name, _ := row[nAt].(string)
			typ, _ := row[tyAt].(string)
			out = append(out, canvas.Column{ID: id, Name: name, Type: grid.Type(typ)})
		}
	}
	var key int64
	if r := l.layout.Tables["_lattis_tables"]; r != nil {
		if kAt := r.index("key_col"); kAt >= 0 {
			for _, row := range r.Rows {
				if id, _ := asInt(row[r.index("id")]); id == tbl {
					key, _ = asInt(row[kAt])
				}
			}
		}
	}
	return out, key
}

// loadTables crée les tableaux publiés et y copie leurs lignes.
func (l *loader) loadTables() error {
	for _, t := range l.tables {
		cols, key := l.layoutColumns(t.LocalID)
		byID := map[int64]bool{}
		for _, c := range cols {
			byID[c.ID] = true
		}
		// Colonne du serveur → colonne locale (par identifiant local, à
		// défaut par nom).
		local := make([]int64, len(t.Columns))
		for i, rc := range t.Columns {
			if byID[rc.LocalID] {
				local[i] = rc.LocalID
				continue
			}
			for _, c := range cols {
				if strings.EqualFold(c.Name, rc.Name) {
					local[i] = c.ID
				}
			}
		}
		for _, q := range canvas.TableSchema(t.LocalID, cols, key) {
			if err := l.exec(q); err != nil {
				return msg.Wrap(err, "collab.tableError", msg.Args{"table": t.Name, "err": err})
			}
		}
		name := "t_" + strconv.FormatInt(t.LocalID, 10)
		names := []string{"id", "_pos"}
		var keep []int
		for i, id := range local {
			if id != 0 {
				names = append(names, "c_"+strconv.FormatInt(id, 10))
				keep = append(keep, i)
			}
		}
		ins, err := l.tx.PrepareContext(l.ctx, `INSERT INTO `+name+` (`+strings.Join(names, ", ")+`) VALUES (`+strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", ")+`)`)
		if err != nil {
			return err
		}
		idIns, err := l.tx.PrepareContext(l.ctx, `INSERT INTO _lattis_row_ids (tbl, rid, lattis_id, lattis_author) VALUES (?, ?, ?, ?)`)
		if err != nil {
			ins.Close()
			return err
		}
		rids := map[string]int64{}
		var rid int64
		err = l.r.Rows(l.ctx, l.info.Schema, t, func(id, pos, author string, vals []any) error {
			rid++
			args := []any{rid, pos}
			for _, i := range keep {
				args = append(args, vals[i])
			}
			if _, err := ins.ExecContext(l.ctx, args...); err != nil {
				return err
			}
			var a any
			if author != "" {
				a = author
			}
			if _, err := idIns.ExecContext(l.ctx, t.LocalID, rid, id, a); err != nil {
				return err
			}
			rids[id] = rid
			return nil
		})
		ins.Close()
		idIns.Close()
		if err != nil {
			return msg.Wrap(err, "collab.tableError", msg.Args{"table": t.Name, "err": err})
		}
		l.rids[t.LocalID] = rids
		for i := range t.Columns {
			t.Columns[i].Key = t.Key != "" && t.Columns[i].ID == t.Key
		}
		rc, err := json.Marshal(t.Columns)
		if err != nil {
			return err
		}
		if err := l.exec(`INSERT INTO _lattis_remote_tables (tbl, published, remote_name, remote_id, columns, since) VALUES (?, 1, ?, ?, ?, ?)
			ON CONFLICT (tbl) DO UPDATE SET published = 1, remote_name = excluded.remote_name, remote_id = excluded.remote_id, columns = excluded.columns, since = excluded.since`,
			t.LocalID, t.Name, t.ID, string(rc), l.since); err != nil {
			return err
		}
	}
	return nil
}

// toRids remet les rid locaux à la place des lattis_id d'une table de mise
// en forme par ligne (lignes inconnues écartées).
func (l *loader) toRids(r *LayoutRows) [][]any {
	tAt, rAt := r.index("tbl"), r.index("rid")
	var out [][]any
	for _, row := range r.Rows {
		tbl, _ := asInt(row[tAt])
		if s, ok := row[rAt].(string); ok {
			rid, ok := l.rids[tbl][s]
			if !ok {
				continue
			}
			row = slices.Clone(row)
			row[rAt] = rid
		}
		out = append(out, row)
	}
	return out
}

// restoreLocal remet ce que l'utilisateur avait ajouté en local, renuméroté
// si la nouvelle version a pris ses identifiants ou ses noms.
func (l *loader) restoreLocal() error {
	if len(l.local) == 0 && len(l.private) == 0 {
		return nil
	}
	// Feuilles.
	takenSheets := idSet(l.layout.Tables["_lattis_sheets"], "id")
	sheetNames, sheetPos := map[string]bool{}, int64(0)
	if r := l.layout.Tables["_lattis_sheets"]; r != nil {
		for _, row := range r.Rows {
			if n, ok := row[r.index("name")].(string); ok {
				sheetNames[strings.ToLower(n)] = true
			}
			if p, ok := asInt(row[r.index("pos")]); ok {
				sheetPos = max(sheetPos, p+1)
			}
		}
	}
	nextSheet := int64(1)
	for id := range takenSheets {
		nextSheet = max(nextSheet, id+1)
	}
	for id := range l.sheets {
		nextSheet = max(nextSheet, id+1)
	}
	for _, id := range slices.Sorted(maps.Keys(l.sheets)) {
		if takenSheets[id] {
			if !idSet(l.local["_lattis_sheets"], "id")[id] {
				continue // feuille du serveur, toujours là
			}
			l.sheets[id] = nextSheet
			nextSheet++
		}
	}
	// Tableaux privés.
	takenTables := map[int64]bool{}
	for _, t := range l.tables {
		takenTables[t.LocalID] = true
	}
	for id := range idSet(l.layout.Tables["_lattis_tables"], "id") {
		takenTables[id] = true
	}
	tableNames := map[string]bool{}
	if r := l.layout.Tables["_lattis_tables"]; r != nil {
		for _, row := range r.Rows {
			if n, ok := row[r.index("name")].(string); ok {
				tableNames[n] = true
			}
		}
	}
	nextTable := int64(1)
	for id := range takenTables {
		nextTable = max(nextTable, id+1)
	}
	for id := range l.private {
		nextTable = max(nextTable, id+1)
	}
	var maxRemote int64
	if err := l.tx.QueryRowContext(l.ctx, `SELECT coalesce(max(tbl), 0) FROM _lattis_remote_tables`).Scan(&maxRemote); err != nil {
		return err
	}
	nextTable = max(nextTable, maxRemote+1)
	for _, id := range slices.Sorted(maps.Keys(l.private)) {
		if takenTables[id] {
			l.private[id] = nextTable
			nextTable++
		}
	}
	for _, name := range layoutTables {
		rows := l.local[name]
		if rows == nil {
			continue
		}
		sAt, tAt := rows.index("sheet"), rows.index("tbl")
		idAt := -1
		if name == "_lattis_tables" || name == "_lattis_sheets" {
			idAt = rows.index("id")
		}
		var out [][]any
		for _, row := range rows.Rows {
			row = slices.Clone(row)
			if sAt >= 0 {
				if id, ok := asInt(row[sAt]); ok {
					if to, ok := l.sheets[id]; ok {
						row[sAt] = to
					}
				}
			}
			if tAt >= 0 {
				if id, ok := asInt(row[tAt]); ok {
					to, ok := l.private[id]
					if !ok {
						continue
					}
					row[tAt] = to
				}
			}
			switch name {
			case "_lattis_sheets":
				id, _ := asInt(row[idAt])
				row[idAt] = l.sheets[id]
				nAt := rows.index("name")
				row[nAt] = unique(row[nAt].(string), sheetNames, true)
				if pAt := rows.index("pos"); pAt >= 0 {
					row[pAt] = sheetPos
					sheetPos++
				}
			case "_lattis_tables":
				id, _ := asInt(row[idAt])
				row[idAt] = l.private[id]
				nAt := rows.index("name")
				row[nAt] = unique(row[nAt].(string), tableNames, false)
			}
			out = append(out, row)
		}
		if err := insertRows(l.ctx, l.tx, name, rows.Columns, out, true); err != nil {
			return err
		}
	}
	for from, to := range l.private {
		if err := l.exec(`ALTER TABLE _lattis_keep_` + strconv.FormatInt(from, 10) + ` RENAME TO t_` + strconv.FormatInt(to, 10)); err != nil {
			return err
		}
		if err := l.renameIndexes(from, to); err != nil {
			return err
		}
		if err := l.exec(`UPDATE _lattis_invalid SET tbl = ? WHERE tbl = ?`, to, -from); err != nil {
			return err
		}
	}
	return nil
}

// unique rend un nom libre (« Ventes », « Ventes 2 »…) et le note pris.
func unique(name string, taken map[string]bool, fold bool) string {
	k := func(s string) string {
		if fold {
			return strings.ToLower(s)
		}
		return s
	}
	cand := name
	for n := 2; taken[k(cand)]; n++ {
		cand = fmt.Sprintf("%s %d", name, n)
	}
	taken[k(cand)] = true
	return cand
}

// renameIndexes renomme les index d'un tableau renuméroté (t_3_pos →
// t_9_pos) : un tableau qui reprendrait l'ancien identifiant les
// recréerait.
func (l *loader) renameIndexes(from, to int64) error {
	table := "t_" + strconv.FormatInt(to, 10)
	oldPrefix, newPrefix := "t_"+strconv.FormatInt(from, 10)+"_", table+"_"
	rs, err := l.tx.QueryContext(l.ctx, `SELECT name, sql FROM main.sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL`, table)
	if err != nil {
		return err
	}
	type idx struct{ name, sql string }
	var all []idx
	for rs.Next() {
		var i idx
		if err := rs.Scan(&i.name, &i.sql); err != nil {
			rs.Close()
			return err
		}
		all = append(all, i)
	}
	rs.Close()
	for _, i := range all {
		if from == to || !strings.HasPrefix(i.name, oldPrefix) {
			continue
		}
		name := newPrefix + strings.TrimPrefix(i.name, oldPrefix)
		if err := l.exec(`DROP INDEX ` + i.name); err != nil {
			return err
		}
		if err := l.exec(strings.Replace(i.sql, i.name, name, 1)); err != nil {
			return err
		}
	}
	return nil
}

// loadScripts remet les scripts publiés (dernière version de chacun). Ils
// ne s'exécutent qu'avec l'accord de l'utilisateur de ce poste (empreinte
// du source, internal/trust).
func (l *loader) loadScripts() error {
	for _, s := range l.scripts {
		if err := l.exec(`INSERT INTO _lattis_scripts (name, source) VALUES (?, ?) ON CONFLICT (name) DO UPDATE SET source = excluded.source`, s.Name, s.Source); err != nil {
			return err
		}
	}
	return nil
}

// localChoices lit la colonne choice de chaque feuille (nil : NULL) ;
// rien pour un format plus ancien.
func localChoices(ctx context.Context, tx *sql.Tx) (map[int64]any, error) {
	cols, err := tableColumns(ctx, tx, "_lattis_sheets")
	if err != nil || !slices.Contains(cols, "choice") {
		return nil, err
	}
	rs, err := tx.QueryContext(ctx, `SELECT id, choice FROM _lattis_sheets`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[int64]any{}
	for rs.Next() {
		var id int64
		var c sql.NullString
		if err := rs.Scan(&id, &c); err != nil {
			return nil, err
		}
		out[id] = nil
		if c.Valid {
			out[id] = c.String
		}
	}
	return out, rs.Err()
}

// mergeChoices remet le choix des onglets à choix d'avant le chargement
// (choices, par feuille) : tel quel sur une feuille du poste, et sur une
// feuille du serveur dont la catégorie n'a pas changé depuis la dernière
// mise en page reçue (ou que la mise en page ne décrit pas : publiée avant
// que les catégories le soient) ; sinon la catégorie du serveur, avec la
// valeur choisie du poste si c'est la même (mergeChoice). Puis les
// cellules par valeur montrent celles de la valeur choisie de chacun.
func (l *loader) mergeChoices(choices map[int64]any) error {
	if l.fresh || choices == nil {
		return canvas.ShowChoiceCells(l.ctx, l.tx)
	}
	server := l.layout.Tables["_lattis_sheets"]
	has := server != nil && server.index("choice") >= 0
	local := idSet(l.local["_lattis_sheets"], "id")
	for id, c := range choices {
		to := id
		if t, ok := l.sheets[id]; ok {
			to = t
		}
		v := c
		q := `UPDATE _lattis_sheets SET choice = ? WHERE id = ?`
		switch {
		case local[id]:
			// Feuille du poste : son choix tel quel.
		case !has:
			// Mise en page sans catégories : le choix du poste.
			if c == nil {
				continue
			}
			q += ` AND choice IS NULL`
		default:
			sd, bd := layoutChoice(l.layout, rowOf(server, id)), layoutChoice(l.base, rowOf(l.base.Tables["_lattis_sheets"], id))
			if defText(sd) != defText(bd) {
				v = choiceText(mergeChoice(parseChoice(c), sd))
			}
		}
		if err := l.exec(q, v, to); err != nil {
			return err
		}
	}
	return canvas.ShowChoiceCells(l.ctx, l.tx)
}

// rowOf donne le rang de la ligne d'identifiant id (-1 : aucune).
func rowOf(r *LayoutRows, id int64) int {
	if r == nil || r.index("id") < 0 {
		return -1
	}
	at := r.index("id")
	return slices.IndexFunc(r.Rows, func(row []any) bool {
		v, _ := asInt(row[at])
		return v == id
	})
}
