package oplog

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"

	"github.com/lalamefine/lattis/internal/store"
)

// Tableaux nés dans un classeur collaboratif suivi (SetStructure) : un
// tableau qui apparaît par une action ou un rétablissement (saisi, collé,
// Insertion › Tableau, plage convertie, importé, copié…) est noté dans
// _lattis_info (clé BornKey), dans la transaction de l'action : la
// synchronisation l'ajoute au classeur partagé dès que possible (partage
// automatique, internal/remote/collab, AutoShare), même après une coupure
// ou un redémarrage. Une annulation ne fait naître aucun tableau : un
// tableau supprimé que Ctrl+Z rétablit reste sur ce poste.

// BornKey : clé de _lattis_info des tableaux nés à ajouter au classeur
// partagé (liste JSON de leurs identifiants locaux).
const BornKey = "shareBorn"

// tableIDs lit les identifiants des tableaux dans un état de la base.
func tableIDs(ctx context.Context, db snapshot) (map[int64]bool, error) {
	rs, err := db.QueryContext(ctx, `SELECT id FROM _lattis_tables`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[int64]bool{}
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rs.Err()
}

// births note les tableaux apparus pendant la transaction.
func (s *session) births() error {
	before, err := s.r.db.Read().BeginTx(s.ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer before.Rollback()
	b, err := tableIDs(s.ctx, before)
	if err != nil {
		return err
	}
	a, err := tableIDs(s.ctx, s.tx)
	if err != nil {
		return err
	}
	var born []int64
	for id := range a {
		if !b[id] {
			born = append(born, id)
		}
	}
	if len(born) == 0 {
		return nil
	}
	have, err := readBorn(s.ctx, s.tx)
	if err != nil {
		return err
	}
	for _, id := range born {
		if !slices.Contains(have, id) {
			have = append(have, id)
		}
	}
	return writeBorn(s.ctx, s.tx, have)
}

type querier interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

func readBorn(ctx context.Context, q querier) ([]int64, error) {
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, BornKey).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []int64
	if json.Unmarshal([]byte(raw), &out) != nil {
		return nil, nil // illisible : oublié
	}
	return out, nil
}

func writeBorn(ctx context.Context, tx *sql.Tx, ids []int64) error {
	if len(ids) == 0 {
		_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key = ?`, BornKey)
		return err
	}
	slices.Sort(ids)
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, BornKey, string(raw))
	return err
}

// Born rend les tableaux nés en attente d'ajout au classeur partagé.
func Born(ctx context.Context, db *store.Store) ([]int64, error) {
	return readBorn(ctx, db.Read())
}

// ForgetBorn retire des tableaux de la liste des tableaux nés (ajoutés au
// classeur partagé, ou laissés sur ce poste pour de bon).
func ForgetBorn(ctx context.Context, db *store.Store, ids ...int64) error {
	if len(ids) == 0 {
		return nil
	}
	return db.Write(ctx, func(tx *sql.Tx) error { return ForgetBornTx(ctx, tx, ids...) })
}

// ForgetBornTx : ForgetBorn dans une transaction (nil ids : toute la liste,
// à la publication d'un classeur).
func ForgetBornTx(ctx context.Context, tx *sql.Tx, ids ...int64) error {
	if ids == nil {
		_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key = ?`, BornKey)
		return err
	}
	have, err := readBorn(ctx, tx)
	if err != nil || len(have) == 0 {
		return err
	}
	have = slices.DeleteFunc(have, func(id int64) bool { return slices.Contains(ids, id) })
	return writeBorn(ctx, tx, have)
}
