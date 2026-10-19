// Package oplogtest rejoue la file d'envoi d'un classeur comme le ferait le
// Serveur Lattis (écritures conditionnées aux valeurs vues), pour vérifier
// dans les tests que les opérations logiques décrivent exactement ce qui a
// changé dans les tableaux publiés.
package oplogtest

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/store"
)

// Mirror est la copie « serveur » des tableaux publiés, tenue à jour par les
// seules opérations de la file d'envoi.
type Mirror struct {
	Tables map[int64]map[string]*Row // tableau → lattis_id → ligne
	// stale : colonnes dont le type a changé (le serveur les convertit
	// lui-même) : Check ne les compare plus.
	stale map[int64]map[int64]bool
	last  int64 // dernière entrée de la file rejouée
	// Ops compte les opérations rejouées, par genre.
	Ops map[string]int
}

// Row est une ligne du miroir.
type Row struct {
	Pos  string
	Vals map[int64]string // valeurs normalisées (voir Norm)
}

// New crée un miroir vide.
func New() *Mirror {
	return &Mirror{Tables: map[int64]map[string]*Row{}, stale: map[int64]map[int64]bool{}, Ops: map[string]int{}}
}

// Norm normalise une valeur (SQLite ou JSON) pour la comparer : nombre, texte
// ou vide.
func Norm(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case int64:
		return "n:" + strconv.FormatFloat(float64(x), 'g', -1, 64), true
	case float64:
		return "n:" + strconv.FormatFloat(x, 'g', -1, 64), true
	case json.Number:
		f, _ := x.Float64()
		return "n:" + strconv.FormatFloat(f, 'g', -1, 64), true
	case []byte:
		return "s:" + string(x), true
	case string:
		return "s:" + x, true
	}
	return fmt.Sprintf("?:%v", v), true
}

// Sync rejoue les nouvelles entrées de la file. Une condition non tenue
// (valeur vue différente de celle du miroir, ligne absente ou déjà là) est
// une erreur : sans écriture concurrente, le serveur l'aurait refusée.
func (m *Mirror) Sync(ctx context.Context, db *store.Store) error {
	rs, err := db.Read().QueryContext(ctx, `SELECT id, ops FROM _lattis_outbox WHERE id > ? ORDER BY id`, m.last)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		var id int64
		var data string
		if err := rs.Scan(&id, &data); err != nil {
			return err
		}
		ops, err := command.DecodeOps(data)
		if err != nil {
			return err
		}
		for _, op := range ops {
			if err := m.apply(op); err != nil {
				return fmt.Errorf("entrée %d : %s : %w", id, describe(op), err)
			}
		}
		m.last = id
	}
	return rs.Err()
}

func describe(op command.Op) string {
	b, _ := json.Marshal(op)
	return string(b)
}

func (m *Mirror) table(id int64) map[string]*Row {
	t := m.Tables[id]
	if t == nil {
		t = map[string]*Row{}
		m.Tables[id] = t
	}
	return t
}

func (m *Mirror) apply(op command.Op) error {
	m.Ops[op.Kind]++
	t := m.table(op.Table)
	switch op.Kind {
	case command.OpInsert:
		if _, ok := t[op.Row]; ok {
			return fmt.Errorf("ligne déjà présente")
		}
		r := &Row{Pos: op.Pos, Vals: map[int64]string{}}
		vals, _ := op.New.(command.Values)
		for c, v := range vals {
			if s, ok := Norm(v); ok {
				r.Vals[c] = s
			}
		}
		t[op.Row] = r
	case command.OpDelete:
		if _, ok := t[op.Row]; !ok {
			return fmt.Errorf("ligne absente")
		}
		delete(t, op.Row)
	case command.OpSet:
		r, ok := t[op.Row]
		if !ok {
			return fmt.Errorf("ligne absente")
		}
		old, _ := Norm(op.Old)
		if r.Vals[op.Col] != old {
			return fmt.Errorf("valeur vue %q, le serveur a %q", old, r.Vals[op.Col])
		}
		if s, ok := Norm(op.New); ok {
			r.Vals[op.Col] = s
		} else {
			delete(r.Vals, op.Col)
		}
	case command.OpMove:
		r, ok := t[op.Row]
		if !ok {
			return fmt.Errorf("ligne absente")
		}
		if old, _ := op.Old.(string); old != r.Pos {
			return fmt.Errorf("position vue %q, le serveur a %q", old, r.Pos)
		}
		r.Pos = op.Pos
	case command.OpMeta:
		if op.Meta == nil {
			return fmt.Errorf("meta sans changement")
		}
		switch op.Meta.Object + "." + op.Meta.Action {
		case "table.drop":
			delete(m.Tables, op.Table)
			delete(m.stale, op.Table)
		case "table.create":
			if len(t) > 0 {
				return fmt.Errorf("tableau déjà présent")
			}
		case "column.remove":
			for _, r := range t {
				delete(r.Vals, op.Meta.ID)
			}
		case "column.retype":
			if m.stale[op.Table] == nil {
				m.stale[op.Table] = map[int64]bool{}
			}
			m.stale[op.Table][op.Meta.ID] = true
		}
	default:
		return fmt.Errorf("genre inconnu")
	}
	return nil
}

// Check compare le miroir aux tableaux publiés du classeur : mêmes lignes
// (par lattis_id), mêmes clés d'ordre, mêmes valeurs saisies.
func (m *Mirror) Check(ctx context.Context, db *store.Store) error {
	rs, err := db.Read().QueryContext(ctx, `SELECT tbl FROM _lattis_remote_tables WHERE published = 1 ORDER BY tbl`)
	if err != nil {
		return err
	}
	var tables []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return err
		}
		tables = append(tables, id)
	}
	rs.Close()
	for _, id := range tables {
		local, err := localRows(ctx, db, id)
		if err != nil {
			return err
		}
		mirror := m.Tables[id]
		if local == nil {
			if len(mirror) > 0 {
				return fmt.Errorf("tableau %d supprimé, %d lignes sur le serveur", id, len(mirror))
			}
			continue
		}
		if len(local) != len(mirror) {
			return fmt.Errorf("tableau %d : %d lignes, %d sur le serveur\nlocal : %s\nserveur : %s", id, len(local), len(mirror), dump(local), dump(mirror))
		}
		for uid, l := range local {
			r, ok := mirror[uid]
			if !ok {
				return fmt.Errorf("tableau %d : ligne %s absente du serveur", id, uid)
			}
			if r.Pos != l.Pos {
				return fmt.Errorf("tableau %d, ligne %s : position %q, %q sur le serveur", id, uid, l.Pos, r.Pos)
			}
			for c, v := range l.Vals {
				if !m.stale[id][c] && r.Vals[c] != v {
					return fmt.Errorf("tableau %d, ligne %s, colonne %d : %q, %q sur le serveur", id, uid, c, v, r.Vals[c])
				}
			}
			for c, v := range r.Vals {
				if _, ok := l.Vals[c]; !ok && !m.stale[id][c] {
					return fmt.Errorf("tableau %d, ligne %s, colonne %d : vide, %q sur le serveur", id, uid, c, v)
				}
			}
		}
	}
	return nil
}

func dump(rows map[string]*Row) string {
	var out []string
	for uid, r := range rows {
		var cols []string
		for c, v := range r.Vals {
			cols = append(cols, fmt.Sprintf("%d=%s", c, v))
		}
		slices.Sort(cols)
		out = append(out, fmt.Sprintf("%s@%s{%s}", uid[len(uid)-6:], r.Pos, strings.Join(cols, " ")))
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

// localRows lit un tableau publié (nil s'il n'existe pas) ; chaque ligne
// doit avoir son identifiant partagé.
func localRows(ctx context.Context, db *store.Store, id int64) (map[string]*Row, error) {
	name := "t_" + strconv.FormatInt(id, 10)
	var n int
	if err := db.Read().QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	rs, err := db.Read().QueryContext(ctx, `SELECT id FROM _lattis_columns WHERE tbl = ? AND (formula IS NULL OR formula = '') ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	var cols []int64
	for rs.Next() {
		var c int64
		if err := rs.Scan(&c); err != nil {
			rs.Close()
			return nil, err
		}
		cols = append(cols, c)
	}
	rs.Close()
	sel := []string{"t.id", "t._pos", "r.lattis_id"}
	for _, c := range cols {
		sel = append(sel, "t.c_"+strconv.FormatInt(c, 10))
	}
	rs, err = db.Read().QueryContext(ctx, `SELECT `+strings.Join(sel, ", ")+` FROM `+name+` AS t LEFT JOIN _lattis_row_ids AS r ON r.tbl = ? AND r.rid = t.id`, id)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[string]*Row{}
	for rs.Next() {
		var rid int64
		var pos string
		var uid *string
		vals := make([]any, len(cols))
		dest := []any{&rid, &pos, &uid}
		for i := range vals {
			dest = append(dest, &vals[i])
		}
		if err := rs.Scan(dest...); err != nil {
			return nil, err
		}
		if uid == nil {
			return nil, fmt.Errorf("tableau %d : la ligne %d n'a pas d'identifiant partagé", id, rid)
		}
		r := &Row{Pos: pos, Vals: map[int64]string{}}
		for i, v := range vals {
			if s, ok := Norm(v); ok {
				r.Vals[cols[i]] = s
			}
		}
		out[*uid] = r
	}
	return out, rs.Err()
}
