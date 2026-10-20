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
// filters), un niveau par tableau publié avec « filtered » (filters : la
// lecture est filtrée, voir TableAccess.ReadsFiltered) et les restrictions
// qui visent l'utilisateur (JSON).
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
		if a.ReadsFiltered(restr[rid]) {
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

// ReadsFiltered dit si la lecture du tableau est filtrée pour
// l'utilisateur (« Vue filtrée par vos droits ») : read_filtered du serveur
// (migration 0009 : filtre de lecture ou restriction select/all qui le
// vise, hors propriétaires et concepteurs) ; avant, filtered (filtre de
// lecture ou d'écriture) ou une restriction select/all de mine.
func (a TableAccess) ReadsFiltered(mine []Restriction) bool {
	if a.ReadFiltered != nil {
		return *a.ReadFiltered
	}
	if a.Filtered {
		return true
	}
	for _, r := range mine {
		if r.Command == "select" || r.Command == "all" {
			return true
		}
	}
	return false
}

// capabilitiesSnapshot rend le contenu de _lattis_capabilities (pour
// savoir si une relecture a changé quelque chose).
func capabilitiesSnapshot(ctx context.Context, q interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}) (string, map[int64]tableCaps, error) {
	rs, err := q.QueryContext(ctx, `SELECT tbl, level, coalesce(role, ''), coalesce(filters, ''), coalesce(restrictions, '') FROM _lattis_capabilities ORDER BY tbl`)
	if err != nil {
		return "", nil, err
	}
	defer rs.Close()
	var b strings.Builder
	per := map[int64]tableCaps{}
	for rs.Next() {
		var tbl int64
		var a, c, d, e string
		if err := rs.Scan(&tbl, &a, &c, &d, &e); err != nil {
			return "", nil, err
		}
		key := strings.Join([]string{a, c, d, e}, "\x00")
		b.WriteString(key)
		b.WriteByte('\n')
		per[tbl] = tableCaps{level: a, filtered: d != "" || e != "", key: key}
	}
	return b.String(), per, rs.Err()
}

// tableCaps : droits notés d'un tableau (capabilitiesSnapshot).
type tableCaps struct {
	level    string
	filtered bool // filtre de lecture ou restriction : les lignes visibles dépendent de prédicats
	key      string
}

// readsChanged : tableaux publiés dont les lignes visibles ont pu changer
// avec les droits (niveau, filtre, restriction). Le prédicat d'un filtre ou
// d'une restriction n'est pas connu du client : un tableau filtré avant ou
// après est relu. Un tableau devenu inaccessible n'est pas relu (le serveur
// refuserait).
func readsChanged(before, after map[int64]tableCaps) map[int64]bool {
	out := map[int64]bool{}
	for tbl, a := range after {
		if tbl == 0 || a.level == string(LevelNone) {
			continue
		}
		b, ok := before[tbl]
		if !ok || b.key != a.key || b.filtered || a.filtered {
			out[tbl] = true
		}
	}
	return out
}

// RefreshCapabilities relit les droits de l'utilisateur sur le classeur
// partagé ouvert (lattis.my_capabilities) et les note dans
// _lattis_capabilities, avec ses rôles (Info.Roles), puis met link à jour.
// Appelé à la (re)connexion et sur la notification NoticeACL. Un
// utilisateur qui n'a plus accès au classeur (42501) voit tous ses
// tableaux publiés passer au niveau « aucun ». Le titre du classeur
// (renommé sur le serveur) est relu au passage : link est mis à jour, sans
// compter comme un changement de droits. Rend vrai si les droits ont
// changé.
func RefreshCapabilities(ctx context.Context, srv Server, link *Link) (bool, error) {
	changed, _, err := RefreshCapabilitiesReads(ctx, srv, link)
	return changed, err
}

// RefreshCapabilitiesReads fait RefreshCapabilities et rend aussi les
// tableaux à relire en entier (readsChanged) : « *acl » ne dit pas quelles
// lignes deviennent visibles ou invisibles, et changes_since ne rend que
// les lignes modifiées depuis le point de reprise.
func RefreshCapabilitiesReads(ctx context.Context, srv Server, link *Link) (bool, map[int64]bool, error) {
	db := link.db
	info := link.Info()
	if info == nil {
		return false, nil, nil
	}
	caps, err := srv.Capabilities(ctx, info.WorkbookID)
	// Classeur supprimé du serveur (P0002) : noté sur link, le suivi s'arrête ;
	// accès retiré (42501) : noté aussi, rendu à la prochaine lecture réussie.
	gone := GoneReason(err)
	if gone == GoneDeleted {
		link.SetGone(gone)
		return false, nil, err
	}
	revoked := gone == GoneRevoked
	if err != nil && !revoked {
		return false, nil, err
	}
	linkChanged := link.SetGone(gone)
	local := map[string]int64{}
	rs, err := db.Read().QueryContext(ctx, `SELECT tbl, coalesce(remote_id, '') FROM _lattis_remote_tables WHERE published = 1`)
	if err != nil {
		return false, nil, err
	}
	for rs.Next() {
		var tbl int64
		var rid string
		if err := rs.Scan(&tbl, &rid); err != nil {
			rs.Close()
			return false, nil, err
		}
		local[rid] = tbl
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return false, nil, err
	}
	if revoked {
		caps = Capabilities{Tables: map[string]TableAccess{}}
		for rid := range local {
			caps.Tables[rid] = TableAccess{Level: LevelNone}
		}
	}
	changed, titled := false, false
	var reload map[int64]bool
	err = db.Write(ctx, func(tx *sql.Tx) error {
		before, beforeT, err := capabilitiesSnapshot(ctx, tx)
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
			changed = true
		}
		// Titre changé sur le serveur (vide : serveur d'avant la migration
		// 0007, ou accès retiré).
		if caps.Title != "" && caps.Title != cur.Title {
			cur.Title = caps.Title
			titled = true
		}
		if changed || titled {
			if err := writeInfo(ctx, tx, cur); err != nil {
				return err
			}
		}
		if err := writeCapabilities(ctx, tx, cur, &caps, local); err != nil {
			return err
		}
		after, afterT, err := capabilitiesSnapshot(ctx, tx)
		if err != nil {
			return err
		}
		reload = readsChanged(beforeT, afterT)
		changed = changed || before != after
		if !changed && !titled {
			return errUnchanged // rien de changé : transaction défaite
		}
		return nil
	})
	if errors.Is(err, errUnchanged) {
		return linkChanged, reload, nil
	}
	if err != nil {
		return false, nil, err
	}
	if changed || titled {
		return changed || linkChanged, reload, link.Reload(ctx)
	}
	return linkChanged, reload, nil
}
