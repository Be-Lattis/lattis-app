package oplog

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/store"
)

// Publish marque le tableau tbl comme publié : chacune de ses lignes reçoit
// un identifiant partagé (lattis_id, UUIDv7) et ses écritures suivantes
// sont relevées dans la file d'envoi. Hors du journal d'annulation, comme
// la protection : publier ne s'annule pas. Les identifiants ne sont posés
// qu'ici, tableau par tableau : un classeur non partagé n'est pas touché.
func Publish(ctx context.Context, db *store.Store, tbl int64) error {
	return db.Write(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM main.sqlite_master WHERE type = 'table' AND name = ?`, tableName(tbl)).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("pas de tableau %d", tbl)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_remote_tables (tbl, published) VALUES (?, 1) ON CONFLICT (tbl) DO UPDATE SET published = 1`, tbl); err != nil {
			return err
		}
		return assignIDs(ctx, tx, tbl)
	})
}

// assignIDs donne un identifiant aux lignes du tableau qui n'en ont pas.
func assignIDs(ctx context.Context, tx *sql.Tx, tbl int64) error {
	rs, err := tx.QueryContext(ctx, `SELECT id FROM `+tableName(tbl)+` AS t WHERE NOT EXISTS (SELECT 1 FROM _lattis_row_ids AS r WHERE r.tbl = ? AND r.rid = t.id) ORDER BY _pos`, tbl)
	if err != nil {
		return err
	}
	var rids []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return err
		}
		rids = append(rids, id)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	for start := 0; start < len(rids); start += chunk {
		var added [][2]any
		for _, r := range rids[start:min(start+chunk, len(rids))] {
			u, err := uuid.NewV7()
			if err != nil {
				return err
			}
			added = append(added, [2]any{r, u.String()})
		}
		if err := insertIDs(ctx, tx, tbl, added); err != nil {
			return err
		}
	}
	return nil
}

// Unpublish arrête le relevé des écritures du tableau tbl (il redevient
// local). Ses lignes gardent leurs identifiants.
func Unpublish(ctx context.Context, db *store.Store, tbl int64) error {
	return db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE _lattis_remote_tables SET published = 0 WHERE tbl = ?`, tbl)
		return err
	})
}

// Published dit si le tableau tbl est publié.
func Published(ctx context.Context, db *store.Store, tbl int64) (bool, error) {
	var n int
	err := db.Read().QueryRowContext(ctx, `SELECT count(*) FROM _lattis_remote_tables WHERE tbl = ? AND published = 1`, tbl).Scan(&n)
	return n > 0, err
}

// RowID donne l'identifiant partagé de la ligne rid du tableau tbl (vide si
// elle n'en a pas).
func RowID(ctx context.Context, db *store.Store, tbl, rid int64) (string, error) {
	var u string
	err := db.Read().QueryRowContext(ctx, `SELECT lattis_id FROM _lattis_row_ids WHERE tbl = ? AND rid = ?`, tbl, rid).Scan(&u)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return u, err
}

// Entry est une entrée de la file d'envoi.
type Entry struct {
	ID    int64
	Seq   int64 // 0 : la commande n'est plus dans le journal
	Kind  string
	Batch int64
	Ops   string // JSON (command.DecodeOps)
	State string
}

// Pending lit les entrées en attente, dans l'ordre d'envoi.
func Pending(ctx context.Context, db *store.Store) ([]Entry, error) {
	return entries(ctx, db, `WHERE state = 'pending'`)
}

// Entries lit toute la file, dans l'ordre d'envoi.
func Entries(ctx context.Context, db *store.Store) ([]Entry, error) { return entries(ctx, db, "") }

func entries(ctx context.Context, db *store.Store, where string) ([]Entry, error) {
	rs, err := db.Read().QueryContext(ctx, `SELECT id, coalesce(seq, 0), kind, batch, ops, state FROM _lattis_outbox `+where+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []Entry
	for rs.Next() {
		var e Entry
		if err := rs.Scan(&e.ID, &e.Seq, &e.Kind, &e.Batch, &e.Ops, &e.State); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rs.Err()
}
