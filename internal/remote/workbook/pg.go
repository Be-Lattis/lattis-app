package workbook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/protocol"
)

// Dialer donne une connexion PostgreSQL au serveur pour un usage
// (pgpool.Usage*), sous l'identité de l'utilisateur, et la fonction qui la
// rend.
type Dialer func(ctx context.Context, usage string) (*pgx.Conn, func(), error)

// Usages des connexions (application_name, voir pgpool).
const (
	usageWrite   = "write"
	usageLoad    = "initial_load"
	usageListen  = "delta"
	usageCatalog = "initial_load"
)

// PG est le Server d'un vrai Serveur Lattis (API SQL du schéma lattis,
// contrat client de lattisd §5).
func PG(dial Dialer) Server { return pgServer{dial} }

type pgServer struct{ dial Dialer }

// with exécute fn sur une connexion de l'usage donné. Une lecture qui peut
// aller sur un réplica (ro) et qui reçoit un refus de droits (42501) est
// refaite (contrat client de lattisd, §3.1) : une session ouverte sur un
// réplica pendant qu'il rejoue la création d'un classeur peut garder une
// vue périmée des rôles et refuser à tort ; la connexion fautive est fermée
// (le pool ne la redonne pas), un nouvel essai a lieu sur une nouvelle
// connexion, puis un dernier sur rw. Les lectures sont sans effet de bord :
// les refaire est sûr.
func (s pgServer) with(ctx context.Context, usage string, fn func(*pgx.Conn) error) error {
	err := s.once(ctx, usage, fn)
	if usage != usageLoad || !Privilege(err) {
		return err
	}
	if err = s.once(ctx, usage, fn); !Privilege(err) {
		return err
	}
	return s.once(ctx, usageListen, fn) // rw
}

func (s pgServer) once(ctx context.Context, usage string, fn func(*pgx.Conn) error) error {
	c, release, err := s.dial(ctx, usage)
	if err != nil {
		return err
	}
	defer release()
	err = fn(c)
	if Privilege(err) && usage == usageLoad {
		// Fermée avant d'être rendue : le pool la jette au lieu de la
		// réutiliser.
		c.Close(context.WithoutCancel(ctx))
	}
	return err
}

func (s pgServer) Workbooks(ctx context.Context) ([]Listing, error) {
	var out []Listing
	err := s.with(ctx, usageCatalog, func(c *pgx.Conn) error {
		var raw []byte
		if err := c.QueryRow(ctx, `SELECT lattis.my_workbooks()::text`).Scan(&raw); err != nil {
			return err
		}
		return decodeListings(raw, &out)
	})
	return out, err
}

func decodeListings(raw []byte, out *[]Listing) error {
	var rows []struct {
		Listing
		PublishedAt *string `json:"published_at"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("lattis.my_workbooks : %w", err)
	}
	for _, r := range rows {
		l := r.Listing
		if r.PublishedAt != nil {
			l.PublishedAt = *r.PublishedAt
		}
		*out = append(*out, l)
	}
	return nil
}

func (s pgServer) Capabilities(ctx context.Context, id string) (Capabilities, error) {
	var caps Capabilities
	err := s.with(ctx, usageCatalog, func(c *pgx.Conn) error {
		var err error
		caps, err = capabilities(ctx, c, id)
		return err
	})
	return caps, err
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func capabilities(ctx context.Context, q querier, id string) (Capabilities, error) {
	var raw []byte
	if err := q.QueryRow(ctx, `SELECT lattis.my_capabilities($1::uuid)::text`, id).Scan(&raw); err != nil {
		return Capabilities{}, err
	}
	var caps Capabilities
	if err := json.Unmarshal(raw, &caps); err != nil {
		return caps, fmt.Errorf("lattis.my_capabilities : %w", err)
	}
	return caps, nil
}

func (s pgServer) Publish(ctx context.Context, fn func(Publisher) error) error {
	return s.with(ctx, usageWrite, func(c *pgx.Conn) error {
		return pgx.BeginFunc(ctx, c, func(tx pgx.Tx) error { return fn(pgPublisher{tx: tx, v9: new(int8)}) })
	})
}

func (s pgServer) Read(ctx context.Context, fn func(Reader) error) error {
	return s.with(ctx, usageLoad, func(c *pgx.Conn) error {
		return pgx.BeginTxFunc(ctx, c, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
			return fn(pgReader{tx: tx, v8: new(sync8)})
		})
	})
}

func (s pgServer) Listen(ctx context.Context, channel string, ready func(), fn func(string)) error {
	if channel == "" {
		return msg.New("workbook.unknownChannel", nil)
	}
	return s.with(ctx, usageListen, func(c *pgx.Conn) error {
		if _, err := c.Exec(ctx, "LISTEN "+pgx.Identifier{channel}.Sanitize()); err != nil {
			return err
		}
		// La connexion retourne au pool : plus d'écoute dessus.
		defer c.Exec(context.WithoutCancel(ctx), "UNLISTEN *")
		if ready != nil {
			ready()
		}
		for {
			n, err := c.WaitForNotification(ctx)
			if err != nil {
				return err
			}
			fn(n.Payload)
		}
	})
}

// pgPublisher : v9 retient si le serveur a la migration 0009
// (create_table avec la feuille ; 0 : pas encore lu, 1 : oui, -1 : non).
type pgPublisher struct {
	tx pgx.Tx
	v9 *int8
}

// sheet9Query dit si le serveur a la migration 0009 (create_table reçoit
// la feuille du tableau).
const sheet9Query = `SELECT to_regprocedure('lattis.create_table(uuid,text,jsonb,text)') IS NOT NULL`

func (p pgPublisher) has9(ctx context.Context) (bool, error) {
	if *p.v9 == 0 {
		var ok bool
		if err := p.tx.QueryRow(ctx, sheet9Query).Scan(&ok); err != nil {
			return false, err
		}
		*p.v9 = -1
		if ok {
			*p.v9 = 1
		}
	}
	return *p.v9 > 0, nil
}

func (p pgPublisher) CreateWorkbook(ctx context.Context, title, mode string) (string, error) {
	var id string
	err := p.tx.QueryRow(ctx, `SELECT lattis.create_workbook($1, $2)::text`, title, mode).Scan(&id)
	return id, err
}

func (p pgPublisher) Republish(ctx context.Context, id string) (string, error) {
	var schema string
	err := p.tx.QueryRow(ctx, `SELECT lattis.republish($1::uuid)`, id).Scan(&schema)
	return schema, err
}

func (p pgPublisher) Capabilities(ctx context.Context, id string) (Capabilities, error) {
	return capabilities(ctx, p.tx, id)
}

// columnsJSON écrit le paramètre columns de lattis.create_table. Le serveur
// lit l'identifiant local du tableau dans le premier élément
// (table_local_id) : il est mis dans chacun.
func columnsJSON(localID int64, cols []ColumnSpec) ([]byte, error) {
	type col struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		LocalID string `json:"local_id"`
		Table   string `json:"table_local_id"`
	}
	out := make([]col, len(cols))
	for i, c := range cols {
		out[i] = col{c.Name, c.Type, strconv.FormatInt(c.LocalID, 10), strconv.FormatInt(localID, 10)}
	}
	return json.Marshal(out)
}

func (p pgPublisher) CreateTable(ctx context.Context, id, schema, name string, localID int64, sheet string, cols []ColumnSpec) (string, []RemoteColumn, error) {
	js, err := columnsJSON(localID, cols)
	if err != nil {
		return "", nil, err
	}
	v9, err := p.has9(ctx)
	if err != nil {
		return "", nil, err
	}
	var tid string
	q, args := `SELECT lattis.create_table($1::uuid, $2, $3::jsonb)::text`, []any{id, name, string(js)}
	if v9 && sheet != "" {
		q, args = `SELECT lattis.create_table($1::uuid, $2, $3::jsonb, $4)::text`, append(args, sheet)
	}
	if err := p.tx.QueryRow(ctx, q, args...).Scan(&tid); err != nil {
		return "", nil, err
	}
	rc, err := readColumns(ctx, p.tx, schema, tid)
	return tid, rc, err
}

func readColumns(ctx context.Context, tx pgx.Tx, schema, tid string) ([]RemoteColumn, error) {
	rs, err := tx.Query(ctx, `SELECT column_id::text, name, coalesce(local_id, ''), type, position FROM `+pgx.Identifier{schema, "lattis_meta_columns"}.Sanitize()+` WHERE table_id = $1::uuid ORDER BY position`, tid)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []RemoteColumn
	for rs.Next() {
		var c RemoteColumn
		var local string
		if err := rs.Scan(&c.ID, &c.Name, &local, &c.Type, &c.Position); err != nil {
			return nil, err
		}
		c.LocalID, _ = strconv.ParseInt(local, 10, 64)
		out = append(out, c)
	}
	return out, rs.Err()
}

func (p pgPublisher) Copy(ctx context.Context, schema, table string, cols []string, rows CopySource) (int64, error) {
	return p.tx.CopyFrom(ctx, pgx.Identifier{schema, table}, cols, rows)
}

func (p pgPublisher) SaveMeta(ctx context.Context, id, kind string, payload []byte) error {
	_, err := p.tx.Exec(ctx, `SELECT lattis.save_meta($1::uuid, $2, $3::jsonb)`, id, kind, string(payload))
	return err
}

func (p pgPublisher) PublishScript(ctx context.Context, id, name, source string) (int, error) {
	var v int
	err := p.tx.QueryRow(ctx, `SELECT lattis.publish_script($1::uuid, $2, $3)`, id, name, source).Scan(&v)
	return v, err
}

func (p pgPublisher) FinishPublish(ctx context.Context, id string) (int, error) {
	var v int
	err := p.tx.QueryRow(ctx, `SELECT lattis.finish_publish($1::uuid)`, id).Scan(&v)
	return v, err
}

// pgReader : v8 retient ce que le serveur a de la migration 0008 (lu une
// fois par transaction).
type pgReader struct {
	tx pgx.Tx
	v8 *sync8
}

func (r pgReader) Capabilities(ctx context.Context, id string) (Capabilities, error) {
	return capabilities(ctx, r.tx, id)
}

func (r pgReader) SyncPoint(ctx context.Context) (string, error) {
	var s string
	var v8 sync8
	err := r.tx.QueryRow(ctx, `SELECT lattis.sync_point()::text, `+sync8Query).Scan(&s, &v8.covered, &v8.skip, &v8.snapshot)
	if err == nil {
		r.note8(v8)
	}
	return s, err
}

func (r pgReader) Tables(ctx context.Context, schema string) ([]RemoteTable, error) {
	rs, err := r.tx.Query(ctx, `SELECT table_id::text, name, coalesce(local_id, ''), position, coalesce(key_column::text, '') FROM `+pgx.Identifier{schema, "lattis_meta_tables"}.Sanitize()+` ORDER BY position`)
	if err != nil {
		return nil, err
	}
	var out []RemoteTable
	for rs.Next() {
		var t RemoteTable
		var local string
		if err := rs.Scan(&t.ID, &t.Name, &local, &t.Position, &t.Key); err != nil {
			rs.Close()
			return nil, err
		}
		t.LocalID, _ = strconv.ParseInt(local, 10, 64)
		out = append(out, t)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Columns, err = readColumns(ctx, r.tx, schema, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r pgReader) Meta(ctx context.Context, schema, kind, key string) ([]byte, error) {
	var raw []byte
	err := r.tx.QueryRow(ctx, `SELECT payload::text FROM `+pgx.Identifier{schema, "lattis_meta_objects"}.Sanitize()+` WHERE kind = $1 AND key = $2`, kind, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return raw, err
}

func (r pgReader) Scripts(ctx context.Context, schema string) ([]Script, error) {
	rs, err := r.tx.Query(ctx, `SELECT DISTINCT ON (name) name, version, source, sha256 FROM `+pgx.Identifier{schema, "lattis_meta_script_versions"}.Sanitize()+` ORDER BY name, version DESC`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []Script
	for rs.Next() {
		var s Script
		if err := rs.Scan(&s.Name, &s.Version, &s.Source, &s.SHA256); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rs.Err()
}

// selectExpr lit une colonne du serveur sous une forme que FromRemote
// convertit sans perte : décimal en double précision (le stockage local
// est un REAL), date en texte ISO.
func selectExpr(c RemoteColumn) string {
	id := pgx.Identifier{c.Name}.Sanitize()
	switch c.Type {
	case "decimal":
		return id + "::float8"
	case "date":
		return "to_char(" + id + ", 'YYYY-MM-DD')"
	case "datetime":
		return "to_char(" + id + " AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"')"
	case "integer":
		return id + "::int8"
	case "boolean":
		return id + "::bool"
	}
	return id + "::text"
}

func (r pgReader) Rows(ctx context.Context, schema string, t RemoteTable, fn func(id, pos, author string, vals []any) error) error {
	sel := []string{"lattis_id::text", "lattis_pos", "lattis_author"}
	for _, c := range t.Columns {
		sel = append(sel, selectExpr(c))
	}
	rs, err := r.tx.Query(ctx, `SELECT `+strings.Join(sel, ", ")+` FROM `+pgx.Identifier{schema, t.Name}.Sanitize()+` ORDER BY lattis_pos, lattis_id`)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		v, err := rs.Values()
		if err != nil {
			return err
		}
		id, _ := v[0].(string)
		pos, _ := v[1].(string)
		author, _ := v[2].(string)
		vals := make([]any, len(t.Columns))
		for i, c := range t.Columns {
			vals[i] = FromRemote(c.Type, v[3+i])
		}
		if err := fn(id, pos, author, vals); err != nil {
			return err
		}
	}
	return rs.Err()
}

func (s pgServer) CanCreateWorkbooks(ctx context.Context) (bool, error) {
	can := true
	err := s.with(ctx, usageCatalog, func(c *pgx.Conn) error {
		var raw []byte
		if err := c.QueryRow(ctx, `SELECT lattis.login_summary()::text`).Scan(&raw); err != nil {
			if code, ok := Rejected(err); ok && code == "42883" {
				return nil // serveur d'avant login_summary : tout le monde publie
			}
			return err
		}
		var sum protocol.LoginSummary
		if err := json.Unmarshal(raw, &sum); err != nil {
			return fmt.Errorf("lattis.login_summary : %w", err)
		}
		can = sum.CanCreate()
		return nil
	})
	return can, err
}

func (s pgServer) RecordOpen(ctx context.Context, id string) error {
	// Sur rw (usageWrite), hors transaction : une fonction absente n'y laisse
	// pas de transaction en échec.
	return s.with(ctx, usageWrite, func(c *pgx.Conn) error {
		_, err := c.Exec(ctx, `SELECT lattis.record_open($1::uuid)`, id)
		if code, ok := Rejected(err); ok && code == "42883" {
			return nil // serveur d'avant record_open
		}
		return err
	})
}
