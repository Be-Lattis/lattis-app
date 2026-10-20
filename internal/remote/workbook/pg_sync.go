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
			return fn(pgReader{tx: tx, v8: new(sync8)})
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
	if code, rejected := Refusal(err); rejected {
		if _, rerr := w.tx.Exec(ctx, `ROLLBACK TO SAVEPOINT `+savepoint); rerr != nil {
			// Colonne, table ou schéma inconnus : pgx prépare tout le lot
			// avant de l'exécuter, l'erreur arrive avant le SAVEPOINT (le
			// retour échoue alors en 3B001). L'erreur d'origine est rendue
			// avec son code : l'appelant y reconnaît une structure changée
			// et refait l'étape dans une nouvelle transaction.
			if code == "42703" || code == "42P01" || code == "3F000" {
				return err
			}
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
	// Condition : la valeur que le poste avait lue. Un décimal est lu en
	// float8 (selectExpr) : comparé en float8, sinon une valeur de plus de
	// 17 chiffres écrite hors de Lattis ne serait jamais égale à sa lecture
	// arrondie (conflit à chaque modification).
	cond := c + ` IS NOT DISTINCT FROM $3::` + typ
	if typ == "numeric" {
		cond = c + `::float8 IS NOT DISTINCT FROM $3::float8`
	}
	return w.exec(ctx, `UPDATE `+pgx.Identifier{schema, table}.Sanitize()+` SET `+c+` = $1::`+typ+
		` WHERE lattis_id = $2::uuid AND `+cond, []any{n, id, o})
}

// exec exécute une écriture et rend le nombre de lignes touchées.
func (w pgWriter) exec(ctx context.Context, q string, args []any) (Wrote, error) {
	var out Wrote
	err := w.write(ctx, q, args, func(br pgx.BatchResults) error {
		tag, err := br.Exec()
		out.Rows = tag.RowsAffected()
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
	return w.exec(ctx, `DELETE FROM `+pgx.Identifier{schema, table}.Sanitize()+` WHERE lattis_id = $1::uuid`, []any{id})
}

func (w pgWriter) Move(ctx context.Context, schema, table, id, old, pos string) (Wrote, error) {
	return w.exec(ctx, `UPDATE `+pgx.Identifier{schema, table}.Sanitize()+` SET lattis_pos = $1 WHERE lattis_id = $2::uuid AND lattis_pos = $3`, []any{pos, id, old})
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

// sync8Query lit ce que le serveur sait de la migration 0008 ; lu une fois
// par transaction (avec SyncPoint) :
//   - since_covered existe ;
//   - changes_since(…, skip) ne masque pas l'écriture d'un autre recouverte
//     par le poste (lattis_other_xid, version finale de 0008). Sans cela,
//     skip n'est pas passé : les échos reviennent, sans effet, plutôt que
//     de perdre une valeur d'un collègue ;
//   - since_covered(since, pg_snapshot) existe (migration 0010) : le témoin
//     d'un rechargement est son instantané (Probe, CoveredBy).
const sync8Query = `to_regprocedure('lattis.since_covered(xid8)') IS NOT NULL,
	coalesce((SELECT prosrc LIKE '%lattis_other_xid%' FROM pg_catalog.pg_proc
		WHERE oid = to_regprocedure('lattis.changes_since(uuid,uuid,xid8,xid8[])')), false),
	to_regprocedure('lattis.since_covered(xid8,pg_snapshot)') IS NOT NULL`

// sync8 : fonctions de la migration 0008 du serveur.
type sync8 struct {
	read    bool // lu dans cette transaction
	covered bool // lattis.since_covered
	skip    bool // changes_since(…, skip) avec lattis_other_xid
	// snapshot : lattis.since_covered(since, pg_snapshot) (migration 0010).
	snapshot bool
}

func (r pgReader) has8(ctx context.Context) (sync8, error) {
	if r.v8 != nil && r.v8.read {
		return *r.v8, nil
	}
	var s sync8
	if err := r.tx.QueryRow(ctx, `SELECT `+sync8Query).Scan(&s.covered, &s.skip, &s.snapshot); err != nil {
		return sync8{}, err
	}
	r.note8(s)
	return s, nil
}

func (r pgReader) note8(s sync8) {
	if r.v8 != nil {
		s.read = true
		*r.v8 = s
	}
}

// Probe rend le témoin de la lecture (voir Reader.Probe) : son instantané
// (pg_current_snapshot, le même pendant toute la transaction REPEATABLE
// READ, donc celui de sync_point) avec la migration 0010 ; sinon la fin de
// cet instantané (pg_snapshot_xmax) : toute transaction d'identifiant
// inférieur est validée ou en cours à son début.
func (r pgReader) Probe(ctx context.Context) (string, error) {
	s8, err := r.has8(ctx)
	if err != nil {
		return "", err
	}
	q := `SELECT pg_snapshot_xmax(pg_current_snapshot())::text`
	if s8.snapshot {
		q = `SELECT pg_current_snapshot()::text`
	}
	var s string
	err = r.tx.QueryRow(ctx, q).Scan(&s)
	return s, err
}

// probeSnapshot dit si un témoin est un instantané (xmin:xmax:xip) plutôt
// qu'un point de contrôle (xmax seul, serveur sans la migration 0010).
func probeSnapshot(probe string) bool { return strings.Contains(probe, ":") }

func (r pgReader) CoveredBy(ctx context.Context, since, probe string) (bool, error) {
	if !probeSnapshot(probe) {
		covered, _, err := r.Covered(ctx, probe)
		return covered, err
	}
	s8, err := r.has8(ctx)
	if err != nil || !s8.snapshot {
		return false, err // instantané d'un serveur qui n'en a plus l'usage : recharger
	}
	if since == "" {
		since = "0"
	}
	var covered bool
	err = r.tx.QueryRow(ctx, `SELECT lattis.since_covered($1::xid8, $2::pg_snapshot)`, since, probe).Scan(&covered)
	return covered, err
}

func (r pgReader) Covered(ctx context.Context, since string) (bool, bool, error) {
	if since == "" || since == "0" {
		return true, true, nil
	}
	s, err := r.has8(ctx)
	if err != nil || !s.covered {
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
		s, err := r.has8(ctx)
		if err != nil {
			return err
		}
		if s.skip {
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
