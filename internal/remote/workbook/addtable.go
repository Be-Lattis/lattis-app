package workbook

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/store"
)

// Tableau ajouté à un classeur collaboratif déjà publié (« Ajouter au
// classeur partagé », docs/collaboration/synchronisation.md §2.10) :
//
//   - sur le poste de l'auteur (AddTable) : lattis.create_table sur la
//     version publiée (le serveur pose déclencheurs et droits), copie des
//     lignes, clé, mise en page du tableau ajoutée à celle du serveur, dans
//     une seule transaction du serveur, elle-même dans une transaction
//     locale (verrou d'écriture tenu : aucune saisie ne se perd entre la
//     lecture des lignes et la correspondance notée) ; refus du serveur :
//     rien ne change en local ;
//   - sur les autres postes (ReadArrivals, InstallArrivals) : un tableau du
//     serveur qu'aucun tableau local ne désigne est créé hors du journal
//     d'annulation, avec ses lignes, d'après la mise en page du serveur,
//     comme à l'ouverture du classeur (Load).

// ErrAddDesign : seuls les propriétaires et concepteurs ajoutent un tableau
// au classeur partagé (lattis.create_table).
var ErrAddDesign = msg.New("workbook.addDesignOnly", nil)

// ErrAddBusy : un autre concepteur a ajouté un tableau au même moment, sous
// le même identifiant de mise en page.
var ErrAddBusy = msg.New("workbook.addBusy", nil)

// Added décrit un tableau ajouté au classeur partagé.
type Added struct {
	Name     string
	RemoteID string
	Rows     int
}

// AddTable ajoute le tableau local tbl au classeur collaboratif publié
// décrit par info. after, s'il est fixé, est appelé dans la transaction
// locale avec la transaction du serveur (xid8 en texte, Writer.Xact) : le
// poste écarte cet écho de ses réceptions. L'ajout ne s'annule pas par
// Ctrl+Z : le tableau se retire du serveur par « Supprimer le tableau ».
func AddTable(ctx context.Context, srv Server, db *store.Store, info *Info, tbl int64, after func(tx *sql.Tx, xid string) error) (Added, error) {
	var out Added
	if info == nil || info.Mode != Collaborative {
		return out, msg.New("workbook.notCollaborative", nil)
	}
	if !info.CanDesign() {
		return out, ErrAddDesign
	}
	check, err := Inspect(ctx, db)
	if err != nil {
		return out, err
	}
	i := slices.IndexFunc(check.Tables, func(t TableCheck) bool { return t.ID == tbl })
	if i < 0 {
		return out, msg.New("workbook.tableNotFound", msg.Args{"table": tbl})
	}
	tc := check.Tables[i]
	if tc.Published {
		return out, msg.New("workbook.tableAlreadyShared", msg.Args{"table": tc.Name})
	}
	if b := check.Blocking([]int64{tbl}); len(b) > 0 {
		return out, msg.New("workbook.addImpossible", msg.Args{"reasons": strings.Join(b, "\n")})
	}
	if !slices.ContainsFunc(tc.Columns, func(c ColumnCheck) bool { return !c.Computed }) {
		return out, msg.New("workbook.onlyComputedColumns", msg.Args{"table": tc.Name})
	}
	err = db.Write(ctx, func(tx *sql.Tx) error {
		var pub int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM _lattis_remote_tables WHERE tbl = ? AND published = 1`, tbl).Scan(&pub); err != nil {
			return err
		}
		if pub > 0 {
			return msg.New("workbook.tableAlreadyShared", msg.Args{"table": tc.Name})
		}
		if err := spreadTx(ctx, tx, []int64{tbl}); err != nil {
			return err
		}
		// Types figés (§9), dans la mise en page publiée comme en local.
		if _, err := tx.ExecContext(ctx, `UPDATE _lattis_columns SET fixed = 1 WHERE tbl = ? AND coalesce(formula, '') = ''`, tbl); err != nil {
			return err
		}
		tables, err := readTables(ctx, tx, []int64{tbl})
		if err != nil {
			return err
		}
		t := tables[0]
		ids0, err := readRowIDs(ctx, tx, []int64{tbl})
		if err != nil {
			return err
		}
		rows, added, err := readNewRows(ctx, tx, t, ids0[tbl])
		if err != nil {
			return err
		}
		var key int64
		if err := tx.QueryRowContext(ctx, `SELECT coalesce(key_col, 0) FROM _lattis_tables WHERE id = ?`, tbl).Scan(&key); err != nil {
			return err
		}
		all := map[int64]string{}
		for k, v := range ids0[tbl] {
			all[k] = v
		}
		for k, v := range added {
			all[k] = v
		}
		local, err := dumpLayout(ctx, tx, meta.Version, map[int64]bool{tbl: true}, rowIDs{tbl: all})
		if err != nil {
			return err
		}
		var tid, xid string
		var lid int64
		var rcols []RemoteColumn
		var base []byte
		err = srv.Write(ctx, func(w Writer) error {
			raw, err := w.Meta(ctx, info.Schema, layoutKind, layoutKey)
			if err != nil {
				return err
			}
			lid, err = freeLayoutID(raw, tbl)
			if err != nil {
				return err
			}
			if tid, rcols, err = w.CreateTable(ctx, info.WorkbookID, info.Schema, t.name, lid, SheetKey(t.sheet), t.specs()); err != nil {
				return err
			}
			// create_table tient le verrou du classeur : la mise en page relue
			// maintenant est la dernière (un ajout concurrent est validé).
			raw2, err := w.Meta(ctx, info.Schema, layoutKind, layoutKey)
			if err != nil {
				return err
			}
			if !bytes.Equal(raw, raw2) {
				if again, err := freeLayoutID(raw2, lid); err != nil {
					return err
				} else if again != lid {
					return ErrAddBusy
				}
			}
			order := make([]RemoteColumn, len(t.cols))
			for k, c := range t.cols {
				j := slices.IndexFunc(rcols, func(rc RemoteColumn) bool { return rc.LocalID == c.id })
				if j < 0 && k < len(rcols) {
					j = k
				}
				if j < 0 {
					return msg.New("workbook.columnMissingOnServer", msg.Args{"column": c.name})
				}
				order[k] = rcols[j]
			}
			if err := w.Fill(ctx, info.Schema, t.name, order, rows); err != nil {
				return err
			}
			if key != 0 {
				if j := slices.IndexFunc(rcols, func(rc RemoteColumn) bool { return rc.LocalID == key }); j >= 0 {
					if err := w.SetKey(ctx, info.WorkbookID, tid, rcols[j].ID); err != nil {
						return err
					}
					rcols[j].Key = true
				}
			}
			if base, err = mergeLayout(raw2, local, tbl, lid, t.sheet); err != nil {
				return err
			}
			if err := w.SaveMeta(ctx, info.WorkbookID, layoutKind, base); err != nil {
				return err
			}
			xid, err = w.Xact(ctx)
			return err
		})
		if err != nil {
			return err
		}
		cols, err := json.Marshal(rcols)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_remote_tables (tbl, published, remote_name, remote_id, columns, since) VALUES (?, 1, ?, ?, ?, NULL)
			ON CONFLICT (tbl) DO UPDATE SET published = 1, remote_name = excluded.remote_name, remote_id = excluded.remote_id, columns = excluded.columns, since = NULL`,
			tbl, t.name, tid, string(cols)); err != nil {
			return err
		}
		if err := insertRowIDMap(ctx, tx, tbl, added, info.User); err != nil {
			return err
		}
		if err := writeBase(ctx, tx, base); err != nil {
			return err
		}
		if err := noteLayoutID(ctx, tx, "table", tbl, lid); err != nil {
			return err
		}
		out = Added{Name: t.name, RemoteID: tid, Rows: len(rows)}
		if after != nil && xid != "" {
			return after(tx, xid)
		}
		return nil
	})
	return out, err
}

// readNewRows lit les lignes d'un tableau à envoyer (ordre _pos) ; une
// ligne sans identifiant partagé en reçoit un (added).
func readNewRows(ctx context.Context, tx *sql.Tx, t localTable, ids map[int64]string) ([]NewRow, map[int64]string, error) {
	added := map[int64]string{}
	cols := []string{"id", "_pos"}
	for _, c := range t.cols {
		cols = append(cols, "c_"+strconv.FormatInt(c.id, 10))
	}
	rs, err := tx.QueryContext(ctx, `SELECT `+strings.Join(cols, ", ")+` FROM t_`+strconv.FormatInt(t.id, 10)+` ORDER BY _pos, id`)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	var out []NewRow
	for rs.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(raw))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		rid, _ := asInt(raw[0])
		u, ok := ids[rid]
		if !ok {
			v7, err := uuid.NewV7()
			if err != nil {
				return nil, nil, err
			}
			u = v7.String()
			added[rid] = u
		}
		pos, _ := raw[1].(string)
		vals := make([]any, len(t.cols))
		for i := range t.cols {
			v := raw[2+i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			vals[i] = v
		}
		out = append(out, NewRow{ID: u, Pos: pos, Vals: vals})
	}
	return out, added, rs.Err()
}

// freeLayoutID rend l'identifiant du tableau dans la mise en page du
// serveur : want s'il n'y est pas déjà pris, sinon le suivant du plus grand.
func freeLayoutID(raw []byte, want int64) (int64, error) {
	if raw == nil {
		return want, nil
	}
	l, err := decodeLayout(raw)
	if err != nil {
		return 0, err
	}
	ids := idSet(l.Tables["_lattis_tables"], "id")
	if !ids[want] {
		return want, nil
	}
	next := want
	for id := range ids {
		next = max(next, id)
	}
	return next + 1, nil
}

// layoutTableAt donne la colonne qui désigne le tableau dans une table de
// la mise en page (-1 : aucune).
func layoutTableAt(name string, r *LayoutRows) int {
	if name == "_lattis_tables" {
		return r.index("id")
	}
	return r.index("tbl")
}

// mergeLayout ajoute à la mise en page du serveur raw (nil : aucune) les
// lignes du tableau tbl lues en local (local), sous l'identifiant lid, et
// la feuille sheet si le serveur ne la connaît pas encore (avec sa
// catégorie d'onglet à choix et ses cellules par valeur).
func mergeLayout(raw []byte, local *Layout, tbl, lid, sheet int64) ([]byte, error) {
	l := &Layout{Key: layoutKey, Format: meta.Version, Tables: map[string]*LayoutRows{}}
	if raw != nil {
		var err error
		if l, err = decodeLayout(raw); err != nil {
			return nil, err
		}
	}
	newSheet := !idSet(l.Tables["_lattis_sheets"], "id")[sheet]
	for _, name := range layoutTables {
		lr := local.Tables[name]
		if lr == nil {
			continue
		}
		dst := l.Tables[name]
		if dst == nil {
			dst = &LayoutRows{Columns: slices.Clone(lr.Columns)}
			l.Tables[name] = dst
		}
		if name == "_lattis_sheets" {
			if idSet(dst, "id")[sheet] {
				continue
			}
			at := lr.index("id")
			for _, row := range lr.Rows {
				if id, _ := asInt(row[at]); id == sheet {
					p := projectRow(lr, row, dst.Columns)
					if ci := dst.index("choice"); ci >= 0 {
						p[ci] = choiceText(mapChoice(parseChoice(p[ci]), map[int64]int64{tbl: lid}))
					}
					dst.Rows = append(dst.Rows, p)
				}
			}
			continue
		}
		if name == "_lattis_choice_cells" {
			if at, dat := lr.index("sheet"), dst.index("sheet"); newSheet && at >= 0 && dat >= 0 {
				for _, row := range lr.Rows {
					if id, _ := asInt(row[at]); id == sheet {
						dst.Rows = append(dst.Rows, projectRow(lr, row, dst.Columns))
					}
				}
			}
			continue
		}
		at, dat := layoutTableAt(name, lr), layoutTableAt(name, dst)
		if at < 0 || dat < 0 {
			continue
		}
		dst.Rows = slices.DeleteFunc(dst.Rows, func(row []any) bool {
			id, _ := asInt(row[dat])
			return id == lid
		})
		for _, row := range lr.Rows {
			if id, _ := asInt(row[at]); id != tbl {
				continue
			}
			p := projectRow(lr, row, dst.Columns)
			p[dat] = lid
			dst.Rows = append(dst.Rows, p)
		}
	}
	return encodeLayout(l)
}

// projectRow rend une ligne de r dans l'ordre des colonnes cols (absentes :
// NULL).
func projectRow(r *LayoutRows, row []any, cols []string) []any {
	out := make([]any, len(cols))
	for i, c := range cols {
		if j := r.index(c); j >= 0 {
			out[i] = row[j]
		}
	}
	return out
}

// --- Réception -----------------------------------------------------------------

// Arrivals sont les tableaux du serveur que le poste n'a pas encore, lus
// dans une lecture cohérente.
type Arrivals struct {
	Layout []byte
	// Since : point de reprise de la lecture (celui des tableaux créés).
	Since  string
	Tables []Arrival
}

// Arrival est un tableau du serveur et ses lignes.
type Arrival struct {
	Table RemoteTable
	Rows  []ArrivalRow
}

// ArrivalRow est une ligne d'un tableau reçu (valeurs locales, dans
// l'ordre de Table.Columns).
type ArrivalRow struct {
	ID, Pos, Author string
	Vals            []any
}

// Installed est un tableau créé en local par InstallArrivals.
type Installed struct {
	Table int64
	Name  string
	// lid, sheet : identifiants du tableau et de sa feuille dans la mise en
	// page ; row, col : sa place sur le poste (mise en page de base).
	lid, sheet, row, col int64
}

// ReadArrivals lit, dans la lecture r (Server.Delta), les tableaux du
// schéma qu'aucun tableau local ne désigne (known : identifiants du serveur
// déjà reliés), avec leurs lignes et la mise en page ; nil s'il n'y en a
// pas.
func ReadArrivals(ctx context.Context, r Reader, schema string, known map[string]bool) (*Arrivals, error) {
	sp, err := r.SyncPoint(ctx)
	if err != nil {
		return nil, err
	}
	tables, err := r.Tables(ctx, schema)
	if err != nil {
		return nil, err
	}
	out := &Arrivals{Since: sp}
	for _, t := range tables {
		if known[t.ID] {
			continue
		}
		a := Arrival{Table: t}
		err := r.Rows(ctx, schema, t, func(id, pos, author string, vals []any) error {
			a.Rows = append(a.Rows, ArrivalRow{ID: id, Pos: pos, Author: author, Vals: vals})
			return nil
		})
		if err != nil {
			if Privilege(err) {
				continue // tableau illisible avec ses droits : à la prochaine lecture
			}
			return nil, msg.Wrap(err, "collab.tableError", msg.Args{"table": t.Name, "err": err})
		}
		out.Tables = append(out.Tables, a)
	}
	if len(out.Tables) == 0 {
		return nil, nil
	}
	if out.Layout, err = r.Meta(ctx, schema, layoutKind, layoutKey); err != nil {
		return nil, err
	}
	return out, nil
}

// nextTableSQL : identifiant libre d'un nouveau tableau (comme
// canvas.nextTableID).
const nextTableSQL = `SELECT max(coalesce((SELECT max(id) FROM _lattis_tables), 0), coalesce((SELECT max(tbl) FROM _lattis_remote_tables), 0), coalesce((SELECT max(target) FROM _lattis_queries), 0)) + 1`

// InstallArrivals crée en local, dans tx, hors du journal d'annulation,
// les tableaux reçus : mise en page du serveur (feuille, place, colonnes,
// calculées comprises, clé, mises en forme), lignes et identifiants
// partagés, correspondance avec le serveur (point de reprise a.Since). Un
// tableau déjà relié entre-temps est écarté ; un identifiant ou un nom pris
// en local en reçoit un autre (« Ventes_2 ») ; un tableau qui recouvrirait
// ce qui est déjà sur la feuille va dessous.
func InstallArrivals(ctx context.Context, tx *sql.Tx, a *Arrivals) ([]Installed, error) {
	if a == nil {
		return nil, nil
	}
	l := &Layout{Key: layoutKey, Format: meta.Version, Tables: map[string]*LayoutRows{}}
	if a.Layout != nil {
		var err error
		if l, err = decodeLayout(a.Layout); err != nil {
			return nil, err
		}
	}
	baseSheets := map[int64]bool{}
	if raw, err := readBase(ctx, tx); err != nil {
		return nil, err
	} else if raw != nil {
		if b, err := decodeLayout(raw); err == nil {
			baseSheets = idSet(b.Tables["_lattis_sheets"], "id")
		}
	}
	var out []Installed
	for _, ar := range a.Tables {
		var known int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM _lattis_remote_tables WHERE remote_id = ? AND published = 1`, ar.Table.ID).Scan(&known); err != nil {
			return nil, err
		}
		if known > 0 {
			continue
		}
		in, err := installOne(ctx, tx, l, baseSheets, a.Since, ar)
		if err != nil {
			return nil, msg.Wrap(err, "workbook.receivedTableError", msg.Args{"table": ar.Table.Name, "err": err})
		}
		out = append(out, in)
	}
	if len(out) > 0 && a.Layout != nil {
		if err := arrivedBase(ctx, tx, l, a.Layout, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// arrivedBase ajoute à la mise en page de base les lignes des tableaux
// reçus (tableau à sa place sur le poste, colonnes, mises en forme) et leur
// feuille si elle est nouvelle, telles que le serveur les décrit ; le reste
// de la mise en page du serveur (cellules de la feuille reçue…) arrive par
// la mise en page en direct (SyncLayout), qui le compare à la base. Sans
// base, la mise en page du serveur.
func arrivedBase(ctx context.Context, tx *sql.Tx, l *Layout, raw []byte, got []Installed) error {
	baseRaw, err := readBase(ctx, tx)
	if err != nil {
		return err
	}
	if baseRaw == nil {
		return writeBase(ctx, tx, raw)
	}
	b, err := decodeLayout(baseRaw)
	if err != nil {
		return err
	}
	for _, in := range got {
		for _, name := range []string{"_lattis_tables", "_lattis_columns", "_lattis_styles"} {
			src, rows := layoutRowsOf(l, name, in.lid)
			if src == nil {
				continue
			}
			dst := b.Tables[name]
			if dst == nil {
				dst = &LayoutRows{Columns: slices.Clone(src.Columns), Rows: [][]any{}}
				b.Tables[name] = dst
			}
			at := layoutTableAt(name, dst)
			if at < 0 {
				continue
			}
			dst.Rows = slices.DeleteFunc(dst.Rows, func(row []any) bool {
				id, _ := asInt(row[at])
				return id == in.lid
			})
			for _, row := range rows {
				p := projectRow(src, row, dst.Columns)
				if name == "_lattis_tables" {
					if i := dst.index("row"); i >= 0 {
						p[i] = in.row
					}
					if i := dst.index("col"); i >= 0 {
						p[i] = in.col
					}
				}
				dst.Rows = append(dst.Rows, p)
			}
		}
		ss := l.Tables["_lattis_sheets"]
		if ss == nil || idSet(b.Tables["_lattis_sheets"], "id")[in.sheet] {
			continue
		}
		if at := rowOf(ss, in.sheet); at >= 0 {
			dst := b.Tables["_lattis_sheets"]
			if dst == nil {
				dst = &LayoutRows{Columns: slices.Clone(ss.Columns), Rows: [][]any{}}
				b.Tables["_lattis_sheets"] = dst
			}
			dst.Rows = append(dst.Rows, projectRow(ss, ss.Rows[at], dst.Columns))
		}
	}
	out, err := encodeLayout(b)
	if err != nil {
		return err
	}
	return writeBase(ctx, tx, out)
}

func count(ctx context.Context, tx *sql.Tx, q string, args ...any) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

// layoutRowsOf rend les lignes d'une table de la mise en page qui
// désignent le tableau lid.
func layoutRowsOf(l *Layout, name string, lid int64) (*LayoutRows, [][]any) {
	r := l.Tables[name]
	if r == nil {
		return nil, nil
	}
	at := layoutTableAt(name, r)
	if at < 0 {
		return r, nil
	}
	var out [][]any
	for _, row := range r.Rows {
		if id, ok := asInt(row[at]); ok && id == lid && lid != 0 {
			out = append(out, slices.Clone(row))
		}
	}
	return r, out
}

func installOne(ctx context.Context, tx *sql.Tx, l *Layout, baseSheets map[int64]bool, since string, ar Arrival) (Installed, error) {
	rt := ar.Table
	lid := rt.LocalID
	// Identifiant local : celui de la mise en page s'il est libre.
	id := lid
	if id > 0 {
		n, err := count(ctx, tx, `SELECT (SELECT count(*) FROM _lattis_tables WHERE id = ?1) + (SELECT count(*) FROM _lattis_remote_tables WHERE tbl = ?1)
			+ (SELECT count(*) FROM _lattis_queries WHERE target = ?1) + (SELECT count(*) FROM main.sqlite_master WHERE type = 'table' AND name = ?2)`, id, "t_"+strconv.FormatInt(id, 10))
		if err != nil {
			return Installed{}, err
		}
		if n > 0 {
			id = 0
		}
	}
	if id <= 0 {
		if err := tx.QueryRowContext(ctx, nextTableSQL).Scan(&id); err != nil {
			return Installed{}, err
		}
	}
	tr, trows := layoutRowsOf(l, "_lattis_tables", lid)
	var trow []any
	if len(trows) > 0 {
		trow = trows[0]
	}
	get := func(r *LayoutRows, row []any, col string) any {
		if r == nil || row == nil {
			return nil
		}
		if i := r.index(col); i >= 0 {
			return row[i]
		}
		return nil
	}
	// Feuille.
	sheet, err := arrivalSheet(ctx, tx, l, baseSheets, get(tr, trow, "sheet"))
	if err != nil {
		return Installed{}, err
	}
	lsheet, ok := asInt(get(tr, trow, "sheet"))
	if !ok || rowOf(l.Tables["_lattis_sheets"], lsheet) < 0 {
		lsheet = sheet
	} else if lsheet != sheet {
		// Feuille créée sous un autre identifiant (pris ici).
		if err := noteLayoutID(ctx, tx, "sheet", sheet, lsheet); err != nil {
			return Installed{}, err
		}
	}
	// Nom libre (un tableau local du même nom garde le sien).
	name := rt.Name
	for n := 2; ; n++ {
		k, err := count(ctx, tx, `SELECT count(*) FROM _lattis_tables WHERE name = ? COLLATE NOCASE`, name)
		if err != nil {
			return Installed{}, err
		}
		if k == 0 {
			break
		}
		name = fmt.Sprintf("%s_%d", rt.Name, n)
	}
	// Colonnes : celles de la mise en page (calculées comprises), sinon
	// celles du serveur.
	cr, crows := layoutRowsOf(l, "_lattis_columns", lid)
	if len(crows) == 0 {
		cr = &LayoutRows{Columns: []string{"tbl", "id", "name", "type", "fixed", "pos"}}
		for i, c := range rt.Columns {
			crows = append(crows, []any{lid, int64(i + 1), c.Name, string(LocalType(c.Type)), int64(1), int64(i)})
			rt.Columns[i].LocalID = int64(i + 1)
		}
	}
	tblAt := cr.index("tbl")
	var cols []canvas.Column
	for _, row := range crows {
		row[tblAt] = id
		cid, _ := asInt(get(cr, row, "id"))
		cname, _ := get(cr, row, "name").(string)
		ctyp, _ := get(cr, row, "type").(string)
		cols = append(cols, canvas.Column{ID: cid, Name: cname, Type: grid.Type(ctyp)})
	}
	// Colonne du serveur → colonne locale (identifiant local, sinon nom).
	local := make([]int64, len(rt.Columns))
	for i, c := range rt.Columns {
		if slices.ContainsFunc(cols, func(x canvas.Column) bool { return x.ID == c.LocalID && c.LocalID != 0 }) {
			local[i] = c.LocalID
			continue
		}
		for _, x := range cols {
			if strings.EqualFold(x.Name, c.Name) {
				local[i] = x.ID
			}
		}
	}
	var key int64
	if k, ok := asInt(get(tr, trow, "key_col")); ok && slices.ContainsFunc(cols, func(x canvas.Column) bool { return x.ID == k }) {
		key = k
	} else if rt.Key != "" {
		for i, c := range rt.Columns {
			if c.ID == rt.Key {
				key = local[i]
			}
		}
	}
	// Place : celle de l'auteur, sauf si elle recouvre ce qui est déjà sur
	// la feuille (tableaux, cellules) : dessous.
	r0, _ := asInt(get(tr, trow, "row"))
	c0, _ := asInt(get(tr, trow, "col"))
	if r0, err = freeAnchor(ctx, tx, sheet, r0, c0, int64(len(ar.Rows)), int64(len(cols))); err != nil {
		return Installed{}, err
	}
	if trow == nil {
		tr = &LayoutRows{Columns: []string{"id", "sheet", "name", "row", "col", "key_col"}}
		trow = make([]any, 6)
	}
	set := func(col string, v any) {
		if i := tr.index(col); i >= 0 {
			trow[i] = v
		}
	}
	set("id", id)
	set("sheet", sheet)
	set("name", name)
	set("row", r0)
	set("col", c0)
	if key != 0 {
		set("key_col", key)
	} else {
		set("key_col", nil)
	}
	if err := insertRows(ctx, tx, "_lattis_tables", tr.Columns, [][]any{trow}, false); err != nil {
		return Installed{}, err
	}
	if err := insertRows(ctx, tx, "_lattis_columns", cr.Columns, crows, false); err != nil {
		return Installed{}, err
	}
	for _, q := range canvas.TableSchema(id, cols, key) {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return Installed{}, err
		}
	}
	// Lignes.
	tname := "t_" + strconv.FormatInt(id, 10)
	names := []string{"id", "_pos"}
	var keep []int
	for i, c := range local {
		if c != 0 {
			names = append(names, "c_"+strconv.FormatInt(c, 10))
			keep = append(keep, i)
		}
	}
	ins, err := tx.PrepareContext(ctx, `INSERT INTO `+tname+` (`+strings.Join(names, ", ")+`) VALUES (`+strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", ")+`)`)
	if err != nil {
		return Installed{}, err
	}
	defer ins.Close()
	idIns, err := tx.PrepareContext(ctx, `INSERT INTO _lattis_row_ids (tbl, rid, lattis_id, lattis_author) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return Installed{}, err
	}
	defer idIns.Close()
	rids := map[string]int64{}
	for k, r := range ar.Rows {
		rid := int64(k + 1)
		args := []any{rid, r.Pos}
		for _, i := range keep {
			var v any
			if i < len(r.Vals) {
				v = r.Vals[i]
			}
			args = append(args, v)
		}
		if _, err := ins.ExecContext(ctx, args...); err != nil {
			return Installed{}, err
		}
		var au any
		if r.Author != "" {
			au = r.Author
		}
		if _, err := idIns.ExecContext(ctx, id, rid, r.ID, au); err != nil {
			return Installed{}, err
		}
		rids[r.ID] = rid
	}
	// Mises en forme du tableau (colonnes ; lignes par leur identifiant
	// partagé).
	if sr, srows := layoutRowsOf(l, "_lattis_styles", lid); len(srows) > 0 {
		tAt, rAt := sr.index("tbl"), sr.index("rid")
		var keepRows [][]any
		for _, row := range srows {
			row[tAt] = id
			if rAt >= 0 {
				if s, ok := row[rAt].(string); ok {
					rid, ok := rids[s]
					if !ok {
						continue
					}
					row[rAt] = rid
				}
			}
			keepRows = append(keepRows, row)
		}
		if err := insertRows(ctx, tx, "_lattis_styles", sr.Columns, keepRows, true); err != nil {
			return Installed{}, err
		}
	}
	mapped := slices.Clone(rt.Columns)
	for i := range mapped {
		mapped[i].LocalID = local[i]
		mapped[i].Key = rt.Key != "" && mapped[i].ID == rt.Key
	}
	rc, err := json.Marshal(mapped)
	if err != nil {
		return Installed{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_remote_tables (tbl, published, remote_name, remote_id, columns, since) VALUES (?, 1, ?, ?, ?, ?)
		ON CONFLICT (tbl) DO UPDATE SET published = 1, remote_name = excluded.remote_name, remote_id = excluded.remote_id, columns = excluded.columns, since = excluded.since`,
		id, rt.Name, rt.ID, string(rc), since); err != nil {
		return Installed{}, err
	}
	if lid != 0 {
		if err := noteLayoutID(ctx, tx, "table", id, lid); err != nil {
			return Installed{}, err
		}
	}
	return Installed{Table: id, Name: name, lid: lid, sheet: lsheet, row: r0, col: c0}, nil
}

// arrivalSheet rend la feuille locale d'un tableau reçu : la feuille du
// serveur qui porte cet identifiant, sinon une feuille créée d'après la
// mise en page (feuille ajoutée avec le tableau), sinon la première.
func arrivalSheet(ctx context.Context, tx *sql.Tx, l *Layout, baseSheets map[int64]bool, v any) (int64, error) {
	s, ok := asInt(v)
	sr := l.Tables["_lattis_sheets"]
	var srow []any
	if ok && sr != nil {
		at := sr.index("id")
		for _, row := range sr.Rows {
			if id, _ := asInt(row[at]); id == s {
				srow = slices.Clone(row)
			}
		}
	}
	if ok {
		var name string
		err := tx.QueryRowContext(ctx, `SELECT name FROM _lattis_sheets WHERE id = ?`, s).Scan(&name)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		if err == nil {
			theirs := ""
			if srow != nil {
				theirs, _ = srow[sr.index("name")].(string)
			}
			if baseSheets[s] || strings.EqualFold(theirs, name) {
				return s, nil
			}
		}
	}
	if srow != nil {
		id := s
		if n, err := count(ctx, tx, `SELECT count(*) FROM _lattis_sheets WHERE id = ?`, id); err != nil {
			return 0, err
		} else if n > 0 {
			if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(id), 0) + 1 FROM _lattis_sheets`).Scan(&id); err != nil {
				return 0, err
			}
		}
		base, _ := srow[sr.index("name")].(string)
		name := base
		for n := 2; ; n++ {
			k, err := count(ctx, tx, `SELECT count(*) FROM _lattis_sheets WHERE name = ? COLLATE NOCASE`, name)
			if err != nil {
				return 0, err
			}
			if k == 0 {
				break
			}
			name = fmt.Sprintf("%s %d", base, n)
		}
		var pos int64
		if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(pos), -1) + 1 FROM _lattis_sheets`).Scan(&pos); err != nil {
			return 0, err
		}
		srow[sr.index("id")] = id
		srow[sr.index("name")] = name
		if i := sr.index("pos"); i >= 0 {
			srow[i] = pos
		}
		if err := insertRows(ctx, tx, "_lattis_sheets", sr.Columns, [][]any{srow}, false); err != nil {
			return 0, err
		}
		return id, nil
	}
	var first int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM _lattis_sheets ORDER BY pos, id LIMIT 1`).Scan(&first)
	return first, err
}

// freeAnchor rend la ligne d'en-tête d'un tableau de rows lignes et cols
// colonnes posé en (row, col) sur la feuille : row si la place (et une
// case autour) est libre, sinon deux lignes sous ce qui occupe la feuille.
func freeAnchor(ctx context.Context, tx *sql.Tx, sheet, row, col, rows, cols int64) (int64, error) {
	type rect struct{ r0, c0, r1, c1 int64 }
	var used []rect
	rs, err := tx.QueryContext(ctx, `SELECT id, row, col, (SELECT count(*) FROM _lattis_columns k WHERE k.tbl = t.id) FROM _lattis_tables t WHERE sheet = ?`, sheet)
	if err != nil {
		return 0, err
	}
	type tb struct{ id, r, c, n int64 }
	var tbs []tb
	for rs.Next() {
		var x tb
		if err := rs.Scan(&x.id, &x.r, &x.c, &x.n); err != nil {
			rs.Close()
			return 0, err
		}
		tbs = append(tbs, x)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return 0, err
	}
	for _, x := range tbs {
		var n int64
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM t_`+strconv.FormatInt(x.id, 10)).Scan(&n); err != nil {
			n = 0
		}
		used = append(used, rect{x.r, x.c, x.r + n, x.c + max(x.n, 1) - 1})
	}
	rs, err = tx.QueryContext(ctx, `SELECT row, col FROM _lattis_cells WHERE sheet = ?`, sheet)
	if err != nil {
		return 0, err
	}
	for rs.Next() {
		var r, c int64
		if err := rs.Scan(&r, &c); err != nil {
			rs.Close()
			return 0, err
		}
		used = append(used, rect{r, c, r, c})
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return 0, err
	}
	want := rect{row - 1, col - 1, row + rows + 1, col + max(cols, 1)}
	overlap := false
	bottom := int64(-1)
	for _, u := range used {
		if u.r0 <= want.r1 && want.r0 <= u.r1 && u.c0 <= want.c1 && want.c0 <= u.c1 {
			overlap = true
		}
		bottom = max(bottom, u.r1)
	}
	if !overlap {
		return row, nil
	}
	return bottom + 2, nil
}
