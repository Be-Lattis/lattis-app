package workbook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

// CanDesign : propriétaire ou concepteur (publie, republie).
func (i *Info) CanDesign() bool {
	return i != nil && (slices.Contains(i.Roles, "owner") || slices.Contains(i.Roles, "designer"))
}

const (
	infoKey = "remote"
	// baseKey : mise en page telle que reçue du serveur (ou publiée), pour
	// reconnaître au rechargement ce que l'utilisateur a ajouté en local.
	baseKey = "remote_base"
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

// Manifest donne la section remote du manifeste (nil : classeur local).
func Manifest(ctx context.Context, db *sql.DB) (*archive.Remote, error) {
	i, err := ReadInfo(ctx, db)
	if err != nil || i == nil {
		return nil, err
	}
	r := &archive.Remote{Server: i.Server, WorkbookID: i.WorkbookID, Schema: i.Schema, Mode: i.Mode, Version: i.Version, Since: map[string]string{}}
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
	return r, rs.Err()
}
