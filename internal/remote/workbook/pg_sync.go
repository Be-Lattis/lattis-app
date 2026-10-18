package workbook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Synchronisation d'un classeur collaboratif avec un vrai serveur (contrat
// client de lattisd, §6) : écritures conditionnées, lattis.why_not,
// lattis.changes_since. Voir docs/collaboration/synchronisation.md.

func (s pgServer) Delta(ctx context.Context, fn func(Reader) error) error {
	return s.with(ctx, usageListen, func(c *pgx.Conn) error {
		return pgx.BeginTxFunc(ctx, c, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
			return fn(pgReader{tx: tx, v8: new(int8)})
		})
	})
}

func (s pgServer) Write(ctx context.Context, fn func(Writer) error) error {
	return s.with(ctx, usageWrite, func(c *pgx.Conn) error {
		return pgx.BeginFunc(ctx, c, func(tx pgx.Tx) error { return fn(pgWriter{tx}) })
	})
}

// pgType donne le type PostgreSQL d'une colonne du serveur, pour typer les
// paramètres d'une écriture.
func pgType(t string) string {
	switch t {
	case "integer":
		return "int8"
	case "decimal":
		return "numeric"
	case "date":
		return "date"
	case "datetime":
		return "timestamptz"
	case "boolean":
		return "bool"
	}
	return "text"
}

type pgWriter struct{ tx pgx.Tx }

// savepoint : nom du point de sauvegarde de chaque écriture.
const savepoint = "lattis_op"

// write exécute une écriture sous son point de sauvegarde (contrat client
// §6) : SAVEPOINT, l'écriture et RELEASE partent dans un même lot, sans
// aller-retour de plus. Une erreur SQL (filtre d'écriture, droits) ne
// défait que cette écriture (ROLLBACK TO) et est rendue telle quelle ; la
// transaction continue. read lit le résultat de l'écriture (deuxième ordre
// du lot).
func (w pgWriter) write(ctx context.Context, q string, args []any, read func(pgx.BatchResults) error) error {
	b := &pgx.Batch{}
	b.Queue(`SAVEPOINT ` + savepoint)
	b.Queue(q, args...)
	b.Queue(`RELEASE SAVEPOINT ` + savepoint)
	br := w.tx.SendBatch(ctx, b)
	_, err := br.Exec()
	if err == nil {
		err = read(br)
	}
	if err == nil {
		_, err = br.Exec()
	}
	if cerr := br.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		return nil
	}
	if _, rejected := Rejected(err); rejected {
		if _, rerr := w.tx.Exec(ctx, `ROLLBACK TO SAVEPOINT `+savepoint); rerr != nil {
			// La transaction n'est plus utilisable : une coupure, pas un refus.
			return fmt.Errorf("retour au point de sauvegarde après « %v » : %s", err, rerr.Error())
		}
	}
	return err
}

// returning lit la ligne rendue par RETURNING (aucune : 0 ligne touchée).
func returning(br pgx.BatchResults, out *string) (int64, error) {
	var v *string
	err := br.QueryRow().Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if v != nil {
		*out = *v
	}
	return 1, nil
}

func (w pgWriter) Update(ctx context.Context, schema, table, id string, col RemoteColumn, old, new any) (Wrote, error) {
	o, err := ToRemote(col.Type, old)
	if err != nil {
		return Wrote{}, err
	}
	n, err := ToRemote(col.Type, new)
	if err != nil {
		return Wrote{}, err
	}
	c, typ := pgx.Identifier{col.Name}.Sanitize(), pgType(col.Type)
	var out Wrote
	err = w.write(ctx, `UPDATE `+pgx.Identifier{schema, table}.Sanitize()+` SET `+c+` = $1::`+typ+
		` WHERE lattis_id = $2::uuid AND `+c+` IS NOT DISTINCT FROM $3::`+typ+` RETURNING old.lattis_xid::text`, []any{n, id, o},
		func(br pgx.BatchResults) (err error) {
			out.Rows, err = returning(br, &out.Prev)
			return err
		})
	return out, err
}

func (w pgWriter) Insert(ctx context.Context, schema, table, id, pos string, cols []RemoteColumn, vals []any) (Wrote, error) {
	names := []string{"lattis_id", "lattis_pos"}
	marks := []string{"$1::uuid", "$2"}
	args := []any{id, pos}
	for i, c := range cols {
		v, err := ToRemote(c.Type, vals[i])
		if err != nil {
			return Wrote{}, fmt.Errorf("colonne « %s » : %w", c.Name, err)
		}
		names = append(names, pgx.Identifier{c.Name}.Sanitize())
		args = append(args, v)
		marks = append(marks, "$"+strconv.Itoa(len(args))+"::"+pgType(c.Type))
	}
	var out Wrote
	err := w.write(ctx, `INSERT INTO `+pgx.Identifier{schema, table}.Sanitize()+` (`+strings.Join(names, ", ")+`) VALUES (`+strings.Join(marks, ", ")+`) ON CONFLICT (lattis_id) DO NOTHING RETURNING lattis_author::text`, args,
		func(br pgx.BatchResults) (err error) {
			out.Rows, err = returning(br, &out.Author)
			return err
		})
	return out, err
}

func (w pgWriter) Delete(ctx context.Context, schema, table, id string) (Wrote, error) {
	var out Wrote
	err := w.write(ctx, `DELETE FROM `+pgx.Identifier{schema, table}.Sanitize()+` WHERE lattis_id = $1::uuid`, []any{id},
		func(br pgx.BatchResults) error {
			tag, err := br.Exec()
			out.Rows = tag.RowsAffected()
			return err
		})
	return out, err
}

func (w pgWriter) Move(ctx context.Context, schema, table, id, old, pos string) (Wrote, error) {
	var out Wrote
	err := w.write(ctx, `UPDATE `+pgx.Identifier{schema, table}.Sanitize()+` SET lattis_pos = $1 WHERE lattis_id = $2::uuid AND lattis_pos = $3 RETURNING old.lattis_xid::text`, []any{pos, id, old},
		func(br pgx.BatchResults) (err error) {
			out.Rows, err = returning(br, &out.Prev)
			return err
		})
	return out, err
}

func (w pgWriter) Xact(ctx context.Context) (string, error) {
	var x *string
	if err := w.tx.QueryRow(ctx, `SELECT pg_current_xact_id_if_assigned()::text`).Scan(&x); err != nil || x == nil {
		return "", err
	}
	return *x, nil
}

func (w pgWriter) WhyNot(ctx context.Context, workbookID string, t RemoteTable, id string) (WhyNot, error) {
	var raw []byte
	if err := w.tx.QueryRow(ctx, `SELECT lattis.why_not($1::uuid, $2::uuid, $3::uuid)::text`, workbookID, t.ID, id).Scan(&raw); err != nil {
		return WhyNot{}, err
	}
	return decodeWhyNot(raw, t)
}

// decodeWhyNot lit la réponse de lattis.why_not :
// {"reason": "modified", "row", "updated_by", "updated_at"},
// {"reason": "deleted", "deleted_by", "deleted_at"}, {"reason": "forbidden"}
// ou {"reason": "missing"}.
func decodeWhyNot(raw []byte, t RemoteTable) (WhyNot, error) {
	m, err := decodeRow(raw)
	if err != nil {
		return WhyNot{}, fmt.Errorf("lattis.why_not : %w", err)
	}
	str := func(k string) string {
		if s, ok := m[k].(string); ok {
			return s
		}
		return ""
	}
	out := WhyNot{Reason: str("reason"), By: str("updated_by"), At: str("updated_at")}
	if out.Reason == WhyDeleted {
		out.By, out.At = str("deleted_by"), str("deleted_at")
	}
	if row, ok := m["row"].(map[string]any); ok {
		c := ChangeFromJSON(row, t)
		out.Row = &c
	}
	return out, nil
}

// ChangeFromJSON lit une ligne du serveur en JSON (to_jsonb : colonnes par
// leur nom, colonnes système comprises), décodée avec UseNumber.
func ChangeFromJSON(row map[string]any, t RemoteTable) Change {
	str := func(k string) string {
		switch v := row[k].(type) {
		case string:
			return v
		case nil:
			return ""
		default:
			return fmt.Sprint(v)
		}
	}
	c := Change{ID: str("lattis_id"), Pos: str("lattis_pos"), Author: str("lattis_author"),
		UpdatedBy: str("lattis_updated_by"), UpdatedAt: str("lattis_updated_at"), Values: map[string]any{}}
	for _, col := range t.Columns {
		if v, ok := row[col.Name]; ok {
			c.Values[col.Name] = FromJSON(col.Type, v)
		}
	}
	return c
}

func decodeRow(raw []byte) (map[string]any, error) {
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	err := dec.Decode(&m)
	return m, err
}

// sync8Query dit si le serveur a la migration 0008 (changes_since avec
// skip, since_covered) ; lu une fois par transaction (avec SyncPoint).
const sync8Query = `to_regprocedure('lattis.since_covered(xid8)') IS NOT NULL`

func (r pgReader) has8(ctx context.Context) (bool, error) {
	if r.v8 != nil && *r.v8 != 0 {
		return *r.v8 > 0, nil
	}
	var ok bool
	if err := r.tx.QueryRow(ctx, `SELECT `+sync8Query).Scan(&ok); err != nil {
		return false, err
	}
	r.note8(ok)
	return ok, nil
}

func (r pgReader) note8(ok bool) {
	if r.v8 == nil {
		return
	}
	*r.v8 = -1
	if ok {
		*r.v8 = 1
	}
}

func (r pgReader) Covered(ctx context.Context, since string) (bool, bool, error) {
	if since == "" || since == "0" {
		return true, true, nil
	}
	ok, err := r.has8(ctx)
	if err != nil || !ok {
		return true, false, err
	}
	var covered bool
	err = r.tx.QueryRow(ctx, `SELECT lattis.since_covered($1::xid8)`, since).Scan(&covered)
	return covered, true, err
}

func (r pgReader) Changes(ctx context.Context, workbookID string, t RemoteTable, since string, skip []string, fn func(Change) error) error {
	if since == "" {
		since = "0"
	}
	q, args := `SELECT row_data::text, deleted FROM lattis.changes_since($1::uuid, $2::uuid, $3::xid8)`, []any{workbookID, t.ID, since}
	if len(skip) > 0 {
		ok, err := r.has8(ctx)
		if err != nil {
			return err
		}
		if ok {
			q, args = `SELECT row_data::text, deleted FROM lattis.changes_since($1::uuid, $2::uuid, $3::xid8, $4::text[]::xid8[])`, append(args, skip)
		}
	}
	rs, err := r.tx.Query(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		var raw []byte
		var deleted bool
		if err := rs.Scan(&raw, &deleted); err != nil {
			return err
		}
		row, err := decodeRow(raw)
		if err != nil {
			return fmt.Errorf("lattis.changes_since : %w", err)
		}
		c := ChangeFromJSON(row, t)
		c.Deleted = deleted
		if err := fn(c); err != nil {
			return err
		}
	}
	return rs.Err()
}

func (r pgReader) RowsByID(ctx context.Context, schema string, t RemoteTable, ids []string, fn func(Change) error) error {
	if len(ids) == 0 {
		return nil
	}
	rs, err := r.tx.Query(ctx, `SELECT to_jsonb(t)::text FROM `+pgx.Identifier{schema, t.Name}.Sanitize()+` AS t WHERE lattis_id = ANY ($1::text[]::uuid[])`, ids)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		var raw []byte
		if err := rs.Scan(&raw); err != nil {
			return err
		}
		row, err := decodeRow(raw)
		if err != nil {
			return err
		}
		if err := fn(ChangeFromJSON(row, t)); err != nil {
			return err
		}
	}
	return rs.Err()
}
