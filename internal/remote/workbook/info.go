package workbook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
)

// Info décrit un classeur partagé : le classeur du serveur dont le classeur
// local est le cache (§5.1). Gardée dans la base (_lattis_info, clé
// « remote », transactionnelle avec les données) et recopiée dans la
// section remote du manifeste à chaque enregistrement.
type Info struct {
	// Server : adresse du Serveur Lattis (auth.NormalizeURL).
	Server     string `json:"server"`
	ServerName string `json:"serverName,omitempty"`
	WorkbookID string `json:"workbookId"`
	Title      string `json:"title"`
	// Schema : schéma PostgreSQL de la version lue (wb_<slug>, ou
	// wb_<slug>__v<n> après une republication).
	Schema string `json:"schema"`
	// Channel : canal LISTEN du classeur (wb_<slug>).
	Channel string `json:"channel,omitempty"`
	Mode    string `json:"mode"`
	// Version : version publiée chargée (classeur figé).
	Version int      `json:"version"`
	Roles   []string `json:"roles,omitempty"`
	// User : rôle PostgreSQL de l'utilisateur au chargement.
	User   string    `json:"user,omitempty"`
	Synced time.Time `json:"synced"`
}

// CanManage : propriétaire (gère les droits : écran « Partager »).
func (i *Info) CanManage() bool {
	return i != nil && slices.Contains(i.Roles, "owner")
}

// CanDesign : propriétaire ou concepteur (publie, republie).
func (i *Info) CanDesign() bool {
	return i != nil && (slices.Contains(i.Roles, "owner") || slices.Contains(i.Roles, "designer"))
}

const (
	infoKey = "remote"
	// baseKey : mise en page telle que reçue du serveur (ou publiée), pour
	// reconnaître au rechargement ce que l'utilisateur a ajouté en local.
	baseKey = "remote_base"
	// localKey : tableaux dérivés recalculés sur chaque poste (identifiants
	// locaux, JSON) : ni stockés sur le serveur ni privés ; leur requête est
	// publiée avec la mise en page (localTablesKey, localQueriesKey).
	localKey = "remote_local"
)

type queryer interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// ReadInfo lit la description du classeur partagé (nil : classeur local).
func ReadInfo(ctx context.Context, q queryer) (*Info, error) {
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, infoKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var i Info
	if err := json.Unmarshal([]byte(raw), &i); err != nil {
		return nil, err
	}
	return &i, nil
}

func writeInfo(ctx context.Context, tx *sql.Tx, i *Info) error {
	b, err := json.Marshal(i)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, infoKey, string(b))
	return err
}

func writeBase(ctx context.Context, tx *sql.Tx, base []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, baseKey, string(base))
	return err
}

func readBase(ctx context.Context, q queryer) ([]byte, error) {
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, baseKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return []byte(raw), err
}

// LocalTables lit les tableaux recalculés sur chaque poste (localKey).
func LocalTables(ctx context.Context, q queryer) (map[int64]bool, error) {
	out := map[int64]bool{}
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, localKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []int64
	if json.Unmarshal([]byte(raw), &ids) == nil {
		for _, id := range ids {
			out[id] = true
		}
	}
	return out, nil
}

// LocalQueries donne les requêtes qui chargent les tableaux recalculés sur
// chaque poste (à rejouer à l'ouverture et après un chargement).
func LocalQueries(ctx context.Context, db *sql.DB) ([]int64, error) {
	local, err := LocalTables(ctx, db)
	if err != nil || len(local) == 0 {
		return nil, err
	}
	var out []int64
	for _, tbl := range slices.Sorted(maps.Keys(local)) {
		var id int64
		switch err := db.QueryRowContext(ctx, `SELECT id FROM _lattis_queries WHERE target = ?`, tbl).Scan(&id); {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return nil, err
		default:
			out = append(out, id)
		}
	}
	return out, nil
}

func writeLocal(ctx context.Context, tx *sql.Tx, ids []int64) error {
	if len(ids) == 0 {
		_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key = ?`, localKey)
		return err
	}
	ids = slices.Sorted(slices.Values(ids))
	b, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, localKey, string(b))
	return err
}

// PrivateTables compte les tableaux qui ne sont que dans ce classeur, pas sur
// le serveur.
func PrivateTables(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT count(*) FROM _lattis_tables t
		WHERE NOT EXISTS (SELECT 1 FROM _lattis_remote_tables r WHERE r.tbl = t.id AND r.published = 1)`).Scan(&n)
	if err != nil {
		return n, err
	}
	// Recalculés sur chaque poste : pas privés (leur requête est publiée).
	local, err := LocalTables(ctx, db)
	if err != nil {
		return n, err
	}
	for id := range local {
		var ok int
		if db.QueryRowContext(ctx, `SELECT count(*) FROM _lattis_tables WHERE id = ?`, id).Scan(&ok) == nil && ok > 0 {
			n--
		}
	}
	return max(n, 0), nil
}

// Manifest donne la section remote du manifeste (nil : classeur local).
func Manifest(ctx context.Context, db *sql.DB) (*archive.Remote, error) {
	i, err := ReadInfo(ctx, db)
	if err != nil || i == nil {
		return nil, err
	}
	r := &archive.Remote{Server: i.Server, WorkbookID: i.WorkbookID, Schema: i.Schema, Mode: i.Mode, Version: i.Version, Since: map[string]string{}}
	if r.Private, err = PrivateTables(ctx, db); err != nil {
		return nil, err
	}
	rs, err := db.QueryContext(ctx, `SELECT tbl, coalesce(since, '') FROM _lattis_remote_tables WHERE published = 1 ORDER BY tbl`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var tbl int64
		var since string
		if err := rs.Scan(&tbl, &since); err != nil {
			return nil, err
		}
		r.Since[strconv.FormatInt(tbl, 10)] = since
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	if p, c, err := Unsent(ctx, db); err == nil {
		r.Unsent = p + c
	}
	return r, nil
}
