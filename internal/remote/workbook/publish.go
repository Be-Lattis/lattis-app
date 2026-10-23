package workbook

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/order"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/store"
	"github.com/lalamefine/lattis/internal/work"
)

// Request est une demande de publication (Fichier › Partager sur un
// serveur…).
type Request struct {
	// Server : adresse normalisée du serveur ; ServerName : son nom affiché.
	Server     string
	ServerName string
	// User : rôle PostgreSQL de l'utilisateur connecté.
	User  string
	Title string
	Mode  string
	// Tables : tableaux à publier ; les autres restent privés (locaux).
	Tables []int64
}

// localTable est un tableau à publier, lu dans le classeur.
type localTable struct {
	id    int64
	name  string
	sheet int64
	cols  []localColumn
}

type localColumn struct {
	id     int64
	name   string
	typ    grid.Type
	remote string
}

func (t localTable) specs() []ColumnSpec {
	out := make([]ColumnSpec, len(t.cols))
	for i, c := range t.cols {
		out[i] = ColumnSpec{Name: c.name, Type: c.remote, LocalID: c.id}
	}
	return out
}

// published est le résultat d'une publication, à écrire dans le classeur.
type published struct {
	info    *Info
	tables  []localTable
	remote  map[int64]string         // tableau local → identifiant du serveur
	columns map[int64][]RemoteColumn // tableau local → colonnes du serveur
	newIDs  map[int64][]newRowID     // identifiants partagés posés par la publication (ordre des rid)
	base    []byte
}

// Publish publie le classeur db sur le serveur : création du classeur,
// tableaux choisis, copie des lignes, mise en page et scripts, dans une
// seule transaction du serveur. Le classeur devient un classeur partagé
// dont l'utilisateur est propriétaire.
func Publish(ctx context.Context, srv Server, db *store.Store, req Request) (*Info, error) {
	if old, err := ReadInfo(ctx, db.Read()); err != nil {
		return nil, err
	} else if old != nil {
		return nil, msg.New("publish.alreadyPublished", msg.Args{"server": old.Server})
	}
	if req.Mode != Frozen && req.Mode != Collaborative {
		return nil, msg.New("publish.unknownMode", msg.Args{"mode": fmt.Sprintf("%q", req.Mode)})
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, msg.New("publish.missingTitle", nil)
	}
	info := &Info{Server: req.Server, ServerName: req.ServerName, Title: strings.TrimSpace(req.Title), Mode: req.Mode, User: req.User}
	return publish(ctx, srv, db, info, req.Tables, false)
}

// Republish publie une nouvelle version d'un classeur figé (Fichier ›
// Republier) : les tableaux choisis (nil : ceux déjà publiés) et la mise en
// page actuelle remplacent d'un coup la version publiée.
func Republish(ctx context.Context, srv Server, db *store.Store, tables []int64) (*Info, error) {
	info, err := ReadInfo(ctx, db.Read())
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, msg.New("share.notPublished", nil)
	}
	if info.Mode != Frozen {
		return nil, msg.New("publish.republishFrozenOnly", nil)
	}
	if !info.CanDesign() {
		return nil, msg.New("publish.republishOwnersDesigners", nil)
	}
	if tables == nil {
		rs, err := db.Read().QueryContext(ctx, `SELECT tbl FROM _lattis_remote_tables WHERE published = 1 AND tbl IN (SELECT id FROM _lattis_tables) ORDER BY tbl`)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, err
			}
			tables = append(tables, id)
		}
		rs.Close()
	}
	return publish(ctx, srv, db, info, tables, true)
}

func publish(ctx context.Context, srv Server, db *store.Store, info *Info, ids []int64, again bool) (*Info, error) {
	if len(ids) == 0 {
		return nil, msg.New("publish.noTable", nil)
	}
	// Étapes dites dans la barre d'état et dans la boîte de publication
	// (internal/work) : un gros tableau prend des minutes.
	op := "publish"
	if again {
		op = "republish"
	}
	work.Op(ctx, op, info.Title)
	work.Loop(ctx, "check", 0)
	check, err := Inspect(ctx, db)
	if err != nil {
		return nil, err
	}
	if b := check.Blocking(ids); len(b) > 0 {
		return nil, msg.New("publish.impossible", msg.Args{"reasons": strings.Join(b, "\n")})
	}
	var total int64
	for _, t := range check.Tables {
		if slices.Contains(ids, t.ID) {
			total += int64(t.Rows)
		}
	}
	work.Loop(ctx, "order", 0)
	if err := spread(ctx, db, ids); err != nil {
		return nil, err
	}
	work.Loop(ctx, "prepare", 0)
	// Une lecture cohérente du classeur pendant toute la publication.
	rtx, err := db.Read().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer rtx.Rollback()
	tables, err := readTables(ctx, rtx, ids)
	if err != nil {
		return nil, err
	}
	out := &published{info: info, tables: tables, remote: map[int64]string{}, columns: map[int64][]RemoteColumn{}, newIDs: map[int64][]newRowID{}}
	err = srv.Publish(ctx, func(p Publisher) error {
		var schema string
		if again {
			if schema, err = p.Republish(ctx, info.WorkbookID); err != nil {
				return err
			}
		} else {
			id, err := p.CreateWorkbook(ctx, info.Title, info.Mode)
			if err != nil {
				if Privilege(err) {
					return ErrCannotCreate
				}
				return err
			}
			info.WorkbookID = id
			caps, err := p.Capabilities(ctx, id)
			if err != nil {
				return err
			}
			schema = caps.Schema
		}
		// Lignes envoyées, tous tableaux confondus (barre de progression).
		var sent int64
		for _, t := range tables {
			tid, rcols, err := p.CreateTable(ctx, info.WorkbookID, schema, t.name, t.id, SheetKey(t.sheet), t.specs())
			if err != nil {
				return msg.Wrap(err, "collab.tableError", msg.Args{"table": t.name, "err": err})
			}
			out.remote[t.id], out.columns[t.id] = tid, rcols
			work.Loop(ctx, "send", total)
			work.Progress(ctx, sent, total)
			src := &copySource{ctx: ctx, q: rtx, t: t, sent: &sent, total: total}
			names := []string{"lattis_id", "lattis_pos"}
			for _, c := range t.cols {
				names = append(names, c.name)
			}
			if _, err := p.Copy(ctx, schema, t.name, names, src); err != nil {
				return msg.Wrap(err, "collab.tableError", msg.Args{"table": t.name, "err": err})
			}
			// Dans l'ordre des rid : recherche (mise en page) et écriture
			// (record) en ordre.
			slices.SortFunc(src.added, func(a, b newRowID) int { return cmp.Compare(a.rid, b.rid) })
			out.newIDs[t.id] = src.added
		}
		work.Loop(ctx, "layout", 0)
		pub := map[int64]bool{}
		for _, t := range tables {
			pub[t.id] = true
		}
		layout, err := dumpLayout(ctx, rtx, meta.Version, pub, &publishIDs{q: rtx, tables: pub, added: out.newIDs})
		if err != nil {
			return err
		}
		if out.base, err = json.Marshal(layout); err != nil {
			return err
		}
		if err := p.SaveMeta(ctx, info.WorkbookID, layoutKind, out.base); err != nil {
			return msg.Wrap(err, "publish.layoutError", msg.Args{"err": err})
		}
		scripts, err := localScripts(ctx, rtx)
		if err != nil {
			return err
		}
		for _, s := range scripts {
			if _, err := p.PublishScript(ctx, info.WorkbookID, s[0], s[1]); err != nil {
				return msg.Wrap(err, "publish.scriptError", msg.Args{"script": s[0], "err": err})
			}
		}
		work.Loop(ctx, "finish", 0)
		if info.Version, err = p.FinishPublish(ctx, info.WorkbookID); err != nil {
			return err
		}
		caps, err := p.Capabilities(ctx, info.WorkbookID)
		if err != nil {
			return err
		}
		info.Schema, info.Channel, info.Roles = caps.Schema, caps.Channel, caps.Roles
		if caps.Mode != "" {
			info.Mode = caps.Mode
		}
		// Point de non-retour : la transaction du serveur va être validée
		// et la publication notée dans le classeur ; « Annuler » n'est plus
		// proposé. Une annulation arrivée avant défait tout, comme une
		// erreur : la transaction du serveur est abandonnée, rien n'est
		// noté, le classeur reste local.
		if !work.Final(ctx) {
			return context.Cause(ctx)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	rtx.Rollback()
	info.Synced = time.Now().UTC()
	// Publié : la note dans le classeur ne s'interrompt plus.
	ctx = context.WithoutCancel(ctx)
	if err := db.Write(ctx, func(tx *sql.Tx) error { return out.record(ctx, tx) }); err != nil {
		return nil, msg.Wrap(err, "publish.notRecorded", msg.Args{"version": info.Version, "err": err})
	}
	return info, nil
}

// spread répartit les clés d'ordre (_pos) des tableaux à publier : des
// clés courtes et régulières, dans l'ordre actuel (§5.2 : après des
// insertions répétées au même endroit, order.Between allonge les clés).
func spread(ctx context.Context, db *store.Store, ids []int64) error {
	return db.Write(ctx, func(tx *sql.Tx) error { return spreadTx(ctx, tx, ids) })
}

func spreadTx(ctx context.Context, tx *sql.Tx, ids []int64) error {
	{
		for _, id := range ids {
			name := "t_" + strconv.FormatInt(id, 10)
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM `+name).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				continue
			}
			if ok, err := spreadAlready(ctx, tx, name, n); err != nil {
				return err
			} else if ok {
				continue
			}
			// Seules les lignes dont la clé change sont réécrites : un tableau
			// déjà réparti (import, publication précédente) n'est que relu.
			q := `WITH r AS (SELECT id, row_number() OVER (ORDER BY _pos, id) AS n FROM ` + name + `),
				k AS (SELECT id, ` + order.SpreadSQL(n, "n") + ` AS pos FROM r)
				UPDATE ` + name + ` SET _pos = k.pos FROM k WHERE k.id = ` + name + `.id AND ` + name + `._pos IS NOT k.pos`
			if _, err := tx.ExecContext(ctx, q); err != nil {
				return err
			}
		}
	}
	return nil
}

// spreadAlready dit si les n lignes du tableau name ont déjà les clés que
// leur donnerait spreadTx (tableau importé, déjà publié) : une lecture de
// l'index de _pos, sans rien réécrire (à un million de lignes, 7 s → 1 s).
func spreadAlready(ctx context.Context, tx *sql.Tx, name string, n int) (bool, error) {
	rs, err := tx.QueryContext(ctx, `SELECT _pos FROM `+name+` ORDER BY _pos, id`)
	if err != nil {
		return false, err
	}
	defer rs.Close()
	key := order.SpreadKeys(n)
	i := 0
	var pos sql.RawBytes
	for rs.Next() {
		if i >= n {
			return false, nil
		}
		if err := rs.Scan(&pos); err != nil {
			return false, err
		}
		if !bytes.Equal(pos, key(i)) {
			return false, nil
		}
		i++
	}
	return i == n, rs.Err()
}

// SheetKey est l'identifiant d'une feuille pour le serveur
// (lattis_meta_tables.sheet_id, set_level('sheet', …)) : son identifiant
// local dans la mise en page publiée, en décimal, comme table_local_id.
// Une feuille venue du serveur garde cet identifiant au chargement (seules
// les feuilles ajoutées en local sont renumérotées) : la clé vaut pour tous
// les postes.
func SheetKey(sheet int64) string {
	if sheet <= 0 {
		return ""
	}
	return strconv.FormatInt(sheet, 10)
}

func readTables(ctx context.Context, q txOrDB, ids []int64) ([]localTable, error) {
	var out []localTable
	for _, id := range ids {
		t := localTable{id: id}
		if err := q.QueryRowContext(ctx, `SELECT name, coalesce(sheet, 0) FROM _lattis_tables WHERE id = ?`, id).Scan(&t.name, &t.sheet); err != nil {
			return nil, msg.Wrap(err, "table.error", msg.Args{"table": id, "err": err})
		}
		rs, err := q.QueryContext(ctx, `SELECT id, name, type FROM _lattis_columns WHERE tbl = ? AND coalesce(formula, '') = '' ORDER BY pos`, id)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var c localColumn
			var typ string
			if err := rs.Scan(&c.id, &c.name, &typ); err != nil {
				rs.Close()
				return nil, err
			}
			c.typ = grid.Type(typ)
			c.remote = RemoteType(c.typ)
			t.cols = append(t.cols, c)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return nil, err
		}
		// Colonne Date avec des heures : datetime (values.go).
		for i, c := range t.cols {
			if c.typ == grid.Date {
				r, err := DateRemote(ctx, q, id, c.id)
				if err != nil {
					return nil, err
				}
				t.cols[i].remote = r
			}
		}
		out = append(out, t)
	}
	return out, nil
}

func readRowIDs(ctx context.Context, q txOrDB, ids []int64) (rowIDs, error) {
	out := rowIDs{}
	for _, id := range ids {
		m := map[int64]string{}
		rs, err := q.QueryContext(ctx, `SELECT rid, lattis_id FROM _lattis_row_ids WHERE tbl = ?`, id)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var rid int64
			var u string
			if err := rs.Scan(&rid, &u); err != nil {
				rs.Close()
				return nil, err
			}
			m[rid] = u
		}
		rs.Close()
		out[id] = m
	}
	return out, nil
}

func localScripts(ctx context.Context, q txOrDB) ([][2]string, error) {
	rs, err := q.QueryContext(ctx, `SELECT name, source FROM _lattis_scripts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out [][2]string
	for rs.Next() {
		var s [2]string
		if err := rs.Scan(&s[0], &s[1]); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rs.Err()
}

// copySource lit les lignes d'un tableau pour COPY : lattis_id (celui du
// poste, lu par jointure, ou posé ici pour une ligne qui n'en a pas),
// lattis_pos, puis les colonnes saisies converties au type du serveur. Les
// identifiants posés sont gardés dans added (16 octets par ligne : à
// 10 millions de lignes, une table d'association de chaînes prenait
// plusieurs Go). sent : lignes envoyées, tous tableaux confondus.
type copySource struct {
	ctx   context.Context
	q     txOrDB
	t     localTable
	sent  *int64
	total int64
	added []newRowID
	rs    *sql.Rows
	raw   []any
	ptrs  []any
	vals  []any
	err   error
}

// newRowID est un identifiant partagé posé par la publication.
type newRowID struct {
	rid int64
	u   uuid.UUID
}

// progressEvery : avancement donné toutes les progressEvery lignes.
const progressEvery = 4096

func (c *copySource) Next() bool {
	if c.err != nil {
		return false
	}
	if c.rs == nil {
		tbl := strconv.FormatInt(c.t.id, 10)
		cols := []string{"t.id", "t._pos", "r.lattis_id"}
		for _, col := range c.t.cols {
			cols = append(cols, "t.c_"+strconv.FormatInt(col.id, 10))
		}
		c.rs, c.err = c.q.QueryContext(c.ctx, `SELECT `+strings.Join(cols, ", ")+` FROM t_`+tbl+` t
			LEFT JOIN _lattis_row_ids r ON r.tbl = `+tbl+` AND r.rid = t.id ORDER BY t._pos, t.id`)
		if c.err != nil {
			return false
		}
		c.raw = make([]any, 3+len(c.t.cols))
		c.ptrs = make([]any, len(c.raw))
		for i := range c.raw {
			c.ptrs[i] = &c.raw[i]
		}
	}
	if !c.rs.Next() {
		c.err = c.rs.Err()
		c.rs.Close()
		work.Progress(c.ctx, *c.sent, c.total)
		return false
	}
	clear(c.raw)
	if c.err = c.rs.Scan(c.ptrs...); c.err != nil {
		c.rs.Close()
		return false
	}
	rid, _ := asInt(c.raw[0])
	var u string
	switch x := c.raw[2].(type) {
	case string:
		u = x
	case []byte:
		u = string(x)
	default:
		v7, err := uuid.NewV7()
		if err != nil {
			c.err = err
			c.rs.Close()
			return false
		}
		u = v7.String()
		c.added = append(c.added, newRowID{rid, v7})
	}
	pos, _ := c.raw[1].(string)
	c.vals = append(c.vals[:0], u, pos)
	for i, col := range c.t.cols {
		v, err := ToRemote(col.remote, c.raw[3+i])
		if err != nil {
			c.err = msg.Wrap(err, "workbook.columnError", msg.Args{"column": col.name, "err": err})
			c.rs.Close()
			return false
		}
		c.vals = append(c.vals, v)
	}
	if *c.sent++; *c.sent%progressEvery == 0 {
		work.Progress(c.ctx, *c.sent, c.total)
	}
	return true
}

// Values rend les valeurs de la ligne courante ; pgx les encode avant de
// demander la suivante : la tranche est réutilisée.
func (c *copySource) Values() ([]any, error) { return c.vals, nil }
func (c *copySource) Err() error             { return c.err }

// publishIDs donne à la mise en page publiée les identifiants partagés des
// lignes : ceux posés par la publication (added, dans l'ordre des rid),
// sinon ceux déjà notés dans le classeur, lus un à un (seules les lignes
// citées par la mise en page sont cherchées).
type publishIDs struct {
	q      txOrDB
	tables map[int64]bool
	added  map[int64][]newRowID
}

func (p *publishIDs) rowID(ctx context.Context, tbl, rid int64) (string, bool, bool, error) {
	if !p.tables[tbl] {
		return "", false, false, nil
	}
	a := p.added[tbl]
	if i, ok := slices.BinarySearchFunc(a, rid, func(x newRowID, rid int64) int { return cmp.Compare(x.rid, rid) }); ok {
		return a[i].u.String(), true, true, nil
	}
	var u string
	switch err := p.q.QueryRowContext(ctx, `SELECT lattis_id FROM _lattis_row_ids WHERE tbl = ? AND rid = ?`, tbl, rid).Scan(&u); {
	case errors.Is(err, sql.ErrNoRows):
		return "", true, false, nil
	case err != nil:
		return "", true, false, err
	}
	return u, true, true, nil
}

// record note la publication dans le classeur : tableaux publiés et
// correspondance des colonnes, identifiants partagés, types figés (§9),
// description du classeur partagé, mise en page publiée.
func (p *published) record(ctx context.Context, tx *sql.Tx) error {
	// Tableaux nés avant cette publication (classeur partagé auparavant) :
	// le choix des tableaux publiés fait foi, aucun n'est ajouté ensuite
	// d'office (oplog.BornKey).
	if err := oplog.ForgetBornTx(ctx, tx); err != nil {
		return err
	}
	// Tableaux qui ne sont plus publiés : ils redeviennent privés.
	keep := map[int64]bool{}
	for _, t := range p.tables {
		keep[t.id] = true
	}
	rs, err := tx.QueryContext(ctx, `SELECT tbl FROM _lattis_remote_tables WHERE published = 1`)
	if err != nil {
		return err
	}
	var drop []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return err
		}
		if !keep[id] {
			drop = append(drop, id)
		}
	}
	rs.Close()
	for _, id := range drop {
		if _, err := tx.ExecContext(ctx, `UPDATE _lattis_remote_tables SET published = 0, remote_id = NULL, columns = NULL, since = NULL WHERE tbl = ?`, id); err != nil {
			return err
		}
	}
	for _, t := range p.tables {
		cols, err := json.Marshal(p.columns[t.id])
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_remote_tables (tbl, published, remote_name, remote_id, columns, since) VALUES (?, 1, ?, ?, ?, NULL)
			ON CONFLICT (tbl) DO UPDATE SET published = 1, remote_name = excluded.remote_name, remote_id = excluded.remote_id, columns = excluded.columns, since = NULL`,
			t.id, t.name, p.remote[t.id], string(cols)); err != nil {
			return err
		}
		if err := insertRowIDs(ctx, tx, t.id, p.newIDs[t.id], p.info.User); err != nil {
			return err
		}
		// Types figés : plus d'élargissement automatique (§9).
		if _, err := tx.ExecContext(ctx, `UPDATE _lattis_columns SET fixed = 1 WHERE tbl = ? AND coalesce(formula, '') = ''`, t.id); err != nil {
			return err
		}
	}
	if p.info.Mode == Frozen {
		// Rien ne part d'un classeur figé.
		if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_outbox`); err != nil {
			return err
		}
	}
	if err := writeCapabilities(ctx, tx, p.info, nil, nil); err != nil {
		return err
	}
	if err := writeInfo(ctx, tx, p.info); err != nil {
		return err
	}
	// Identifiants de la mise en page : ceux du poste (livelayout.go).
	if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key = ?`, idsKey); err != nil {
		return err
	}
	return writeBase(ctx, tx, p.base)
}

// insertRowIDs note les identifiants partagés posés par la publication
// (ids dans l'ordre des rid), par paquets : une instruction par paquet de
// lignes passées en JSON plutôt qu'une par ligne (à un million de lignes,
// 30 s → quelques secondes : chaque instruction passe par le Store).
func insertRowIDs(ctx context.Context, tx *sql.Tx, tbl int64, ids []newRowID, author string) error {
	if len(ids) == 0 {
		return nil
	}
	work.Loop(ctx, "record", int64(len(ids)))
	var text [36]byte
	return writeRowIDs(ctx, tx, tbl, len(ids), func(i int) (int64, []byte) {
		encodeUUID(text[:], ids[i].u)
		return ids[i].rid, text[:]
	}, author)
}

// insertRowIDMap : insertRowIDs pour des identifiants donnés par rid.
func insertRowIDMap(ctx context.Context, tx *sql.Tx, tbl int64, ids map[int64]string, author string) error {
	rids := slices.Sorted(maps.Keys(ids))
	return writeRowIDs(ctx, tx, tbl, len(rids), func(i int) (int64, []byte) { return rids[i], []byte(ids[rids[i]]) }, author)
}

// rowIDBatch : lignes par instruction.
const rowIDBatch = 20000

func writeRowIDs(ctx context.Context, tx *sql.Tx, tbl int64, n int, item func(int) (int64, []byte), author string) error {
	var a any
	if author != "" {
		a = author
	}
	q := `INSERT INTO _lattis_row_ids (tbl, rid, lattis_id, lattis_author)
		SELECT ?, j.value ->> 0, j.value ->> 1, ? FROM json_each(?) j WHERE true
		ON CONFLICT (tbl, rid) DO UPDATE SET lattis_id = excluded.lattis_id`
	var buf []byte
	for from := 0; from < n; from += rowIDBatch {
		to := min(from+rowIDBatch, n)
		buf = append(buf[:0], '[')
		for i := from; i < to; i++ {
			rid, u := item(i)
			if i > from {
				buf = append(buf, ',')
			}
			buf = append(buf, '[')
			buf = strconv.AppendInt(buf, rid, 10)
			buf = append(buf, ',', '"')
			buf = append(buf, u...)
			buf = append(buf, '"', ']')
		}
		buf = append(buf, ']')
		if _, err := tx.ExecContext(ctx, q, tbl, a, string(buf)); err != nil {
			return err
		}
		work.Progress(ctx, int64(to), int64(n))
	}
	return nil
}

// encodeUUID écrit u en texte (36 octets, forme de uuid.UUID.String) sans
// allouer.
func encodeUUID(dst []byte, u uuid.UUID) {
	hex.Encode(dst, u[:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], u[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], u[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], u[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:], u[10:])
}
