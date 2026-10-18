package workbook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/order"
	"github.com/lalamefine/lattis/internal/store"
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
	id   int64
	name string
	cols []localColumn
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
	newIDs  rowIDs                   // identifiants partagés posés par la publication
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
		return nil, fmt.Errorf("ce classeur est déjà publié sur %s : Fichier › Republier", old.Server)
	}
	if req.Mode != Frozen && req.Mode != Collaborative {
		return nil, fmt.Errorf("mode de partage inconnu : %q", req.Mode)
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("titre du classeur manquant")
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
		return nil, errors.New("ce classeur n'est pas publié : Fichier › Partager sur un serveur…")
	}
	if info.Mode != Frozen {
		return nil, errors.New("seul un classeur figé se republie (un classeur collaboratif se synchronise)")
	}
	if !info.CanDesign() {
		return nil, errors.New("seuls les propriétaires et concepteurs du classeur le republient")
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
		return nil, errors.New("aucun tableau à publier : cochez au moins un tableau")
	}
	check, err := Inspect(ctx, db)
	if err != nil {
		return nil, err
	}
	if b := check.Blocking(ids); len(b) > 0 {
		return nil, fmt.Errorf("publication impossible :\n%s", strings.Join(b, "\n"))
	}
	if err := spread(ctx, db, ids); err != nil {
		return nil, err
	}
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
	ids0, err := readRowIDs(ctx, rtx, ids)
	if err != nil {
		return nil, err
	}
	out := &published{info: info, tables: tables, remote: map[int64]string{}, columns: map[int64][]RemoteColumn{}, newIDs: rowIDs{}}
	err = srv.Publish(ctx, func(p Publisher) error {
		var schema string
		if again {
			if schema, err = p.Republish(ctx, info.WorkbookID); err != nil {
				return err
			}
		} else {
			id, err := p.CreateWorkbook(ctx, info.Title, info.Mode)
			if err != nil {
				return err
			}
			info.WorkbookID = id
			caps, err := p.Capabilities(ctx, id)
			if err != nil {
				return err
			}
			schema = caps.Schema
		}
		for _, t := range tables {
			tid, rcols, err := p.CreateTable(ctx, info.WorkbookID, schema, t.name, t.id, t.specs())
			if err != nil {
				return fmt.Errorf("tableau « %s » : %w", t.name, err)
			}
			out.remote[t.id], out.columns[t.id] = tid, rcols
			src := &copySource{ctx: ctx, q: rtx, t: t, ids: ids0[t.id], added: map[int64]string{}}
			names := []string{"lattis_id", "lattis_pos"}
			for _, c := range t.cols {
				names = append(names, c.name)
			}
			if _, err := p.Copy(ctx, schema, t.name, names, src); err != nil {
				return fmt.Errorf("tableau « %s » : %w", t.name, err)
			}
			out.newIDs[t.id] = src.added
		}
		all := rowIDs{}
		for _, t := range tables {
			m := map[int64]string{}
			for k, v := range ids0[t.id] {
				m[k] = v
			}
			for k, v := range out.newIDs[t.id] {
				m[k] = v
			}
			all[t.id] = m
		}
		pub := map[int64]bool{}
		for _, t := range tables {
			pub[t.id] = true
		}
		layout, err := dumpLayout(ctx, rtx, meta.Version, pub, all)
		if err != nil {
			return err
		}
		if out.base, err = json.Marshal(layout); err != nil {
			return err
		}
		if err := p.SaveMeta(ctx, info.WorkbookID, layoutKind, out.base); err != nil {
			return fmt.Errorf("mise en page : %w", err)
		}
		scripts, err := localScripts(ctx, rtx)
		if err != nil {
			return err
		}
		for _, s := range scripts {
			if _, err := p.PublishScript(ctx, info.WorkbookID, s[0], s[1]); err != nil {
				return fmt.Errorf("script « %s » : %w", s[0], err)
			}
		}
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
		return nil
	})
	if err != nil {
		return nil, err
	}
	rtx.Rollback()
	info.Synced = time.Now().UTC()
	if err := db.Write(ctx, func(tx *sql.Tx) error { return out.record(ctx, tx) }); err != nil {
		return nil, fmt.Errorf("le classeur est publié sur le serveur (version %d), mais ce poste n'a pas pu le noter : %w", info.Version, err)
	}
	return info, nil
}

// spread répartit les clés d'ordre (_pos) des tableaux à publier : des
// clés courtes et régulières, dans l'ordre actuel (§5.2 : après des
// insertions répétées au même endroit, order.Between allonge les clés).
func spread(ctx context.Context, db *store.Store, ids []int64) error {
	return db.Write(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			name := "t_" + strconv.FormatInt(id, 10)
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM `+name).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				continue
			}
			q := `WITH r AS (SELECT id, row_number() OVER (ORDER BY _pos, id) AS n FROM ` + name + `)
				UPDATE ` + name + ` SET _pos = ` + order.SpreadSQL(n, "r.n") + ` FROM r WHERE r.id = ` + name + `.id`
			if _, err := tx.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}

func readTables(ctx context.Context, q txOrDB, ids []int64) ([]localTable, error) {
	var out []localTable
	for _, id := range ids {
		t := localTable{id: id}
		if err := q.QueryRowContext(ctx, `SELECT name FROM _lattis_tables WHERE id = ?`, id).Scan(&t.name); err != nil {
			return nil, fmt.Errorf("tableau %d : %w", id, err)
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

// copySource lit les lignes d'un tableau pour COPY : lattis_id (posé ici
// pour une ligne qui n'en a pas), lattis_pos, puis les colonnes saisies
// converties au type du serveur.
type copySource struct {
	ctx   context.Context
	q     txOrDB
	t     localTable
	ids   map[int64]string
	added map[int64]string
	rs    *sql.Rows
	vals  []any
	err   error
}

func (c *copySource) Next() bool {
	if c.err != nil {
		return false
	}
	if c.rs == nil {
		cols := []string{"id", "_pos"}
		for _, col := range c.t.cols {
			cols = append(cols, "c_"+strconv.FormatInt(col.id, 10))
		}
		c.rs, c.err = c.q.QueryContext(c.ctx, `SELECT `+strings.Join(cols, ", ")+` FROM t_`+strconv.FormatInt(c.t.id, 10)+` ORDER BY _pos, id`)
		if c.err != nil {
			return false
		}
	}
	if !c.rs.Next() {
		c.err = c.rs.Err()
		c.rs.Close()
		return false
	}
	raw := make([]any, 2+len(c.t.cols))
	ptrs := make([]any, len(raw))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	if c.err = c.rs.Scan(ptrs...); c.err != nil {
		c.rs.Close()
		return false
	}
	rid, _ := asInt(raw[0])
	u, ok := c.ids[rid]
	if !ok {
		v7, err := uuid.NewV7()
		if err != nil {
			c.err = err
			c.rs.Close()
			return false
		}
		u = v7.String()
		c.added[rid] = u
	}
	pos, _ := raw[1].(string)
	c.vals = append(c.vals[:0], u, pos)
	for i, col := range c.t.cols {
		v, err := ToRemote(col.remote, raw[2+i])
		if err != nil {
			c.err = fmt.Errorf("colonne « %s » : %w", col.name, err)
			c.rs.Close()
			return false
		}
		c.vals = append(c.vals, v)
	}
	return true
}

func (c *copySource) Values() ([]any, error) { return slices.Clone(c.vals), nil }
func (c *copySource) Err() error             { return c.err }

// record note la publication dans le classeur : tableaux publiés et
// correspondance des colonnes, identifiants partagés, types figés (§9),
// description du classeur partagé, mise en page publiée.
func (p *published) record(ctx context.Context, tx *sql.Tx) error {
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
	return writeBase(ctx, tx, p.base)
}

func insertRowIDs(ctx context.Context, tx *sql.Tx, tbl int64, ids map[int64]string, author string) error {
	if len(ids) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO _lattis_row_ids (tbl, rid, lattis_id, lattis_author) VALUES (?, ?, ?, ?)
		ON CONFLICT (tbl, rid) DO UPDATE SET lattis_id = excluded.lattis_id`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	var a any
	if author != "" {
		a = author
	}
	for rid, u := range ids {
		if _, err := stmt.ExecContext(ctx, tbl, rid, u, a); err != nil {
			return err
		}
	}
	return nil
}

// writeCapabilities note les droits (_lattis_capabilities) : tbl 0 pour le
// classeur (rôles), un niveau par tableau publié.
func writeCapabilities(ctx context.Context, tx *sql.Tx, info *Info, caps *Capabilities, local map[string]int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_capabilities`); err != nil {
		return err
	}
	level := "editor"
	if !info.CanDesign() {
		level = "reader"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_capabilities (tbl, level, role) VALUES (0, ?, ?)`, level, strings.Join(info.Roles, ",")); err != nil {
		return err
	}
	if caps == nil {
		return nil
	}
	for rid, a := range caps.Tables {
		id, ok := local[rid]
		if !ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_capabilities (tbl, level) VALUES (?, ?) ON CONFLICT (tbl) DO UPDATE SET level = excluded.level`, id, a.Level); err != nil {
			return err
		}
	}
	return nil
}
