package workbook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
)

// NoticeACL est la charge de la notification d'un changement de droits du
// classeur (rôles, niveaux, membres, restrictions) sur son canal ; à sa
// réception, les capacités sont relues (migration 0008 du serveur).
const NoticeACL = "*acl"

var errUnchanged = errors.New("droits inchangés")

// writeCapabilities note les droits (_lattis_capabilities) : tbl 0 pour le
// classeur (niveau, rôles fournis, rôles personnalisés en JSON dans
// filters), un niveau par tableau publié avec « filtered » (filters) et les
// restrictions qui visent l'utilisateur (JSON).
func writeCapabilities(ctx context.Context, tx *sql.Tx, info *Info, caps *Capabilities, local map[string]int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_capabilities`); err != nil {
		return err
	}
	level := "editor"
	if !info.CanDesign() {
		level = "reader"
	}
	var custom any
	if caps != nil && len(caps.CustomRoles) > 0 {
		b, _ := json.Marshal(caps.CustomRoles)
		custom = string(b)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_capabilities (tbl, level, role, filters) VALUES (0, ?, ?, ?)`, level, strings.Join(info.Roles, ","), custom); err != nil {
		return err
	}
	if caps == nil {
		return nil
	}
	restr := map[string][]Restriction{}
	for _, r := range caps.Restrictions {
		restr[r.TableID] = append(restr[r.TableID], Restriction{Command: r.Command, Label: r.Label})
	}
	for rid, a := range caps.Tables {
		id, ok := local[rid]
		if !ok {
			continue
		}
		var filt, rs any
		if a.Filtered {
			filt = "filtered"
		}
		if r := restr[rid]; len(r) > 0 {
			b, _ := json.Marshal(r)
			rs = string(b)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_capabilities (tbl, level, filters, restrictions) VALUES (?, ?, ?, ?)
			ON CONFLICT (tbl) DO UPDATE SET level = excluded.level, filters = excluded.filters, restrictions = excluded.restrictions`, id, a.Level, filt, rs); err != nil {
			return err
		}
	}
	return nil
}

// capabilitiesSnapshot rend le contenu de _lattis_capabilities (pour
// savoir si une relecture a changé quelque chose).
func capabilitiesSnapshot(ctx context.Context, q interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}) (string, error) {
	rs, err := q.QueryContext(ctx, `SELECT tbl, level, coalesce(role, ''), coalesce(filters, ''), coalesce(restrictions, '') FROM _lattis_capabilities ORDER BY tbl`)
	if err != nil {
		return "", err
	}
	defer rs.Close()
	var b strings.Builder
	for rs.Next() {
		var tbl int64
		var a, c, d, e string
		if err := rs.Scan(&tbl, &a, &c, &d, &e); err != nil {
			return "", err
		}
		b.WriteString(strings.Join([]string{a, c, d, e}, "\x00"))
		b.WriteByte('\n')
	}
	return b.String(), rs.Err()
}

// RefreshCapabilities relit les droits de l'utilisateur sur le classeur
// partagé ouvert (lattis.my_capabilities) et les note dans
// _lattis_capabilities, avec ses rôles (Info.Roles), puis met link à jour.
// Appelé à la (re)connexion et sur la notification NoticeACL. Un
// utilisateur qui n'a plus accès au classeur (42501) voit tous ses
// tableaux publiés passer au niveau « aucun ». Rend vrai si les droits ont
// changé.
func RefreshCapabilities(ctx context.Context, srv Server, link *Link) (bool, error) {
	db := link.db
	info := link.Info()
	if info == nil {
		return false, nil
	}
	caps, err := srv.Capabilities(ctx, info.WorkbookID)
	revoked := Privilege(err)
	if err != nil && !revoked {
		return false, err
	}
	local := map[string]int64{}
	rs, err := db.Read().QueryContext(ctx, `SELECT tbl, coalesce(remote_id, '') FROM _lattis_remote_tables WHERE published = 1`)
	if err != nil {
		return false, err
	}
	for rs.Next() {
		var tbl int64
		var rid string
		if err := rs.Scan(&tbl, &rid); err != nil {
			rs.Close()
			return false, err
		}
		local[rid] = tbl
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return false, err
	}
	if revoked {
		caps = Capabilities{Tables: map[string]TableAccess{}}
		for rid := range local {
			caps.Tables[rid] = TableAccess{Level: LevelNone}
		}
	}
	changed := false
	err = db.Write(ctx, func(tx *sql.Tx) error {
		before, err := capabilitiesSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		cur, err := ReadInfo(ctx, tx)
		if err != nil || cur == nil {
			return err
		}
		roles := caps.Roles
		if roles == nil {
			roles = []string{}
		}
		if !slices.Equal(cur.Roles, roles) {
			cur.Roles = roles
			if err := writeInfo(ctx, tx, cur); err != nil {
				return err
			}
			changed = true
		}
		if err := writeCapabilities(ctx, tx, cur, &caps, local); err != nil {
			return err
		}
		after, err := capabilitiesSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		changed = changed || before != after
		if !changed {
			return errUnchanged // rien de changé : transaction défaite
		}
		return nil
	})
	if errors.Is(err, errUnchanged) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if changed {
		return true, link.Reload(ctx)
	}
	return false, nil
}
