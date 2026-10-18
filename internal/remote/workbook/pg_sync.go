package workbook

import (
	"context"
	"encoding/json"
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
			return fn(pgReader{tx})
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

func (w pgWriter) Update(ctx context.Context, schema, table, id string, col RemoteColumn, old, new any) (int64, error) {
	o, err := ToRemote(col.Type, old)
	if err != nil {
		return 0, err
	}
	n, err := ToRemote(col.Type, new)
	if err != nil {
		return 0, err
	}
	c, typ := pgx.Identifier{col.Name}.Sanitize(), pgType(col.Type)
	tag, err := w.tx.Exec(ctx, `UPDATE `+pgx.Identifier{schema, table}.Sanitize()+` SET `+c+` = $1::`+typ+
		` WHERE lattis_id = $2::uuid AND `+c+` IS NOT DISTINCT FROM $3::`+typ, n, id, o)
	return tag.RowsAffected(), err
}

func (w pgWriter) Insert(ctx context.Context, schema, table, id, pos string, cols []RemoteColumn, vals []any) (int64, error) {
	names := []string{"lattis_id", "lattis_pos"}
	marks := []string{"$1::uuid", "$2"}
	args := []any{id, pos}
	for i, c := range cols {
		v, err := ToRemote(c.Type, vals[i])
		if err != nil {
			return 0, fmt.Errorf("colonne « %s » : %w", c.Name, err)
		}
		names = append(names, pgx.Identifier{c.Name}.Sanitize())
		args = append(args, v)
		marks = append(marks, "$"+strconv.Itoa(len(args))+"::"+pgType(c.Type))
	}
	tag, err := w.tx.Exec(ctx, `INSERT INTO `+pgx.Identifier{schema, table}.Sanitize()+` (`+strings.Join(names, ", ")+`) VALUES (`+strings.Join(marks, ", ")+`) ON CONFLICT (lattis_id) DO NOTHING`, args...)
	return tag.RowsAffected(), err
}

func (w pgWriter) Delete(ctx context.Context, schema, table, id string) (int64, error) {
	tag, err := w.tx.Exec(ctx, `DELETE FROM `+pgx.Identifier{schema, table}.Sanitize()+` WHERE lattis_id = $1::uuid`, id)
	return tag.RowsAffected(), err
}

func (w pgWriter) Move(ctx context.Context, schema, table, id, old, pos string) (int64, error) {
	tag, err := w.tx.Exec(ctx, `UPDATE `+pgx.Identifier{schema, table}.Sanitize()+` SET lattis_pos = $1 WHERE lattis_id = $2::uuid AND lattis_pos = $3`, pos, id, old)
	return tag.RowsAffected(), err
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

func (r pgReader) Changes(ctx context.Context, workbookID string, t RemoteTable, since string, fn func(Change) error) error {
	if since == "" {
		since = "0"
	}
	rs, err := r.tx.Query(ctx, `SELECT row_data::text, deleted FROM lattis.changes_since($1::uuid, $2::uuid, $3::xid8)`, workbookID, t.ID, since)
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
