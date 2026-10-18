// Package oplog tient le journal logique d'un classeur partagé : il relève,
// pour chaque transaction du bus de commandes (internal/command), les
// opérations logiques (command.Op) faites sur les tableaux publiés, et les
// range dans la file d'envoi _lattis_outbox, dans la même transaction
// SQLite que l'étape (rien ne se perd en cas de plantage).
//
// Relevé par différence plutôt que par instrumentation de chaque opération
// (voir docs/collaboration/journal-logique.md) : des déclencheurs
// temporaires, posés sur la seule connexion d'écriture et sur les seuls
// tableaux publiés, notent la clé (id) de chaque ligne écrite ; à la fin de
// la transaction, l'état d'avant est relu dans l'instantané des lecteurs
// (WAL : ils voient la dernière transaction validée) et comparé à l'état
// d'après. Toute écriture est donc vue, quel que soit son chemin : saisie,
// collage, tri, SQL arbitraire, scripts, annulation… Un tableau non publié
// n'a pas de déclencheur et ne coûte rien.
package oplog

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/store"
)

// chunk : lignes relues par requête, et opérations par entrée de la file.
const chunk = 5000

// Recorder relève les opérations logiques des tableaux publiés d'un
// document (command.Recorder).
type Recorder struct {
	db *store.Store
	// armed : des objets temporaires de relevé ont pu être posés sur la
	// connexion d'écriture (Begin les remet en ordre).
	armed bool
	// Gate, s'il est fixé, décide de chaque opération relevée sur un
	// tableau publié : la garder dans la file d'envoi (send), l'écarter
	// (classeur figé : rien n'est renvoyé au serveur), ou refuser toute
	// l'étape (err : tableau publié en lecture seule). Classeur partagé :
	// voir internal/remote/workbook.
	Gate func(op command.Op) (send bool, err error)
}

// New crée le Recorder d'un document ; le brancher sur son bus
// (Bus.Recorder).
func New(db *store.Store) *Recorder { return &Recorder{db: db} }

// Begin ouvre le relevé d'une transaction du bus.
func (r *Recorder) Begin(ctx context.Context, tx *sql.Tx) (command.Capture, error) {
	pub, err := published(ctx, tx)
	if err != nil {
		return nil, err
	}
	s := &session{r: r, ctx: ctx, tx: tx, pub: pub, ddl: map[int64]bool{}, replaced: map[int64]bool{}}
	if len(pub) > 0 || r.armed {
		if err := s.arm(nil, false); err != nil {
			return nil, err
		}
	}
	if len(pub) > 0 {
		// Lignes notées par une écriture hors du bus (modifications reçues du
		// serveur, appliquées hors journal) : elles ne sont pas à renvoyer.
		if _, err := tx.ExecContext(ctx, `DELETE FROM temp._lattis_cap`); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// published lit les tableaux publiés.
func published(ctx context.Context, tx *sql.Tx) (map[int64]bool, error) {
	rs, err := tx.QueryContext(ctx, `SELECT tbl FROM _lattis_remote_tables WHERE published = 1`)
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

// session suit une transaction.
type session struct {
	r   *Recorder
	ctx context.Context
	tx  *sql.Tx
	pub map[int64]bool
	// ddl : tableaux publiés touchés par une instruction de structure ;
	// replaced : tableaux reconstruits ou remplacés (déclencheurs reposés),
	// comparés en entier.
	ddl, replaced map[int64]bool
}

func tableName(id int64) string { return "t_" + strconv.FormatInt(id, 10) }

func triggerName(id int64, k string) string { return fmt.Sprintf("_lattis_cap_%d_%s", id, k) }

var triggerID = regexp.MustCompile(`^_lattis_cap_(\d+)_[iud]$`)

// arm pose les déclencheurs de relevé des tableaux publiés (ids : ceux-là
// seulement, nil : tous) et retire ceux qui ne servent plus (tableau dépublié,
// ou renommé : un déclencheur suit sa table). mark : un tableau dont les
// déclencheurs manquaient a été reconstruit ou remplacé pendant la
// transaction (DROP puis RENAME) : il sera comparé en entier.
func (s *session) arm(ids map[int64]bool, mark bool) error {
	if len(s.pub) > 0 {
		if _, err := s.tx.ExecContext(s.ctx, `CREATE TEMP TABLE IF NOT EXISTS _lattis_cap (t INTEGER NOT NULL, r INTEGER NOT NULL, PRIMARY KEY (t, r)) WITHOUT ROWID`); err != nil {
			return err
		}
	}
	rs, err := s.tx.QueryContext(s.ctx, `SELECT name, tbl_name FROM temp.sqlite_master WHERE type = 'trigger' AND name LIKE '\_lattis\_cap\_%' ESCAPE '\'`)
	if err != nil {
		return err
	}
	have := map[string]string{}
	for rs.Next() {
		var n, t string
		if err := rs.Scan(&n, &t); err != nil {
			rs.Close()
			return err
		}
		have[n] = t
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	for n, t := range have {
		m := triggerID.FindStringSubmatch(n)
		if m == nil {
			continue
		}
		id, _ := strconv.ParseInt(m[1], 10, 64)
		if ids != nil && !ids[id] {
			continue
		}
		if !s.pub[id] || !strings.EqualFold(t, tableName(id)) {
			if _, err := s.tx.ExecContext(s.ctx, `DROP TRIGGER IF EXISTS temp.`+n); err != nil {
				return err
			}
			delete(have, n)
		}
	}
	for id := range s.pub {
		if ids != nil && !ids[id] {
			continue
		}
		var exists int
		if err := s.tx.QueryRowContext(s.ctx, `SELECT count(*) FROM main.sqlite_master WHERE type = 'table' AND name = ?`, tableName(id)).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			continue
		}
		name := tableName(id)
		defs := map[string]string{
			"i": fmt.Sprintf(`AFTER INSERT ON main.%s BEGIN INSERT OR IGNORE INTO _lattis_cap (t, r) VALUES (%d, NEW.id); END`, name, id),
			"u": fmt.Sprintf(`AFTER UPDATE ON main.%s BEGIN INSERT OR IGNORE INTO _lattis_cap (t, r) VALUES (%d, OLD.id); INSERT OR IGNORE INTO _lattis_cap (t, r) VALUES (%d, NEW.id); END`, name, id, id),
			"d": fmt.Sprintf(`AFTER DELETE ON main.%s BEGIN INSERT OR IGNORE INTO _lattis_cap (t, r) VALUES (%d, OLD.id); END`, name, id),
		}
		for _, k := range []string{"i", "u", "d"} {
			if _, ok := have[triggerName(id, k)]; ok {
				continue
			}
			if _, err := s.tx.ExecContext(s.ctx, `CREATE TEMP TRIGGER `+triggerName(id, k)+` `+defs[k]); err != nil {
				return err
			}
			if mark {
				s.replaced[id] = true
			}
		}
	}
	s.r.armed = len(s.pub) > 0
	return nil
}

var tableRef = regexp.MustCompile(`\bt_(\d+)\b`)

// Statement suit les instructions de structure : un tableau publié
// reconstruit (DROP puis RENAME) ou dont les colonnes changent.
func (s *session) Statement(q string) error {
	if len(s.pub) == 0 || !isDDL(q) {
		return nil
	}
	var ids map[int64]bool
	for _, m := range tableRef.FindAllStringSubmatch(q, -1) {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		if s.pub[id] {
			if ids == nil {
				ids = map[int64]bool{}
			}
			ids[id] = true
			s.ddl[id] = true
		}
	}
	if ids == nil {
		return nil
	}
	return s.arm(ids, true)
}

func isDDL(q string) bool {
	q = strings.TrimLeft(q, " \t\r\n")
	if len(q) < 4 {
		return false
	}
	switch q[0] {
	case 'C', 'c', 'A', 'a', 'D', 'd':
	default:
		return false
	}
	head := strings.ToUpper(q[:min(len(q), 6)])
	return strings.HasPrefix(head, "CREATE") || strings.HasPrefix(head, "ALTER") || strings.HasPrefix(head, "DROP")
}

// Finish range dans la file d'envoi les opérations de la transaction.
func (s *session) Finish(c command.Commit) error {
	// Un numéro de commande peut resservir (les commandes annulées sont
	// effacées par une nouvelle action) : la file ne rattache plus à ce
	// numéro les étapes de l'ancienne commande.
	if c.New && c.Seq != 0 {
		if _, err := s.tx.ExecContext(s.ctx, `UPDATE _lattis_outbox SET seq = NULL WHERE seq = ?`, c.Seq); err != nil {
			return err
		}
	}
	if len(s.pub) == 0 {
		return nil
	}
	w := &writer{s: s, c: c}
	if err := s.diff(w); err != nil {
		return err
	}
	for _, op := range c.Ops {
		if op.Table == 0 || s.pub[op.Table] {
			if err := w.add(op); err != nil {
				return err
			}
		}
	}
	if err := w.flush(); err != nil {
		return err
	}
	_, err := s.tx.ExecContext(s.ctx, `DELETE FROM temp._lattis_cap`)
	return err
}

// --- Comparaison avant / après ------------------------------------------------

// column décrit une colonne de données d'un tableau dans un état.
type column struct {
	typ     string
	formula bool
}

// snapshot est un état de la base : avant (lecteur) ou après (écriture).
type snapshot interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// columns lit les colonnes d'un tableau (nil si le tableau n'existe pas).
func columns(ctx context.Context, db snapshot, id int64) (map[int64]column, error) {
	rs, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, tableName(id))
	if err != nil {
		return nil, err
	}
	phys := map[int64]bool{}
	n := 0
	for rs.Next() {
		var name string
		if err := rs.Scan(&name); err != nil {
			rs.Close()
			return nil, err
		}
		n++
		if c, ok := strings.CutPrefix(name, "c_"); ok {
			if cid, err := strconv.ParseInt(c, 10, 64); err == nil {
				phys[cid] = true
			}
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	out := map[int64]column{}
	rs, err = db.QueryContext(ctx, `SELECT id, type, formula IS NOT NULL AND formula <> '' FROM _lattis_columns WHERE tbl = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var cid int64
		var c column
		if err := rs.Scan(&cid, &c.typ, &c.formula); err != nil {
			return nil, err
		}
		if phys[cid] {
			out[cid] = c
		}
	}
	return out, rs.Err()
}

// row est une ligne relue : clé d'ordre et valeurs des colonnes comparées.
type row struct {
	pos  string
	vals []any
}

func (s *session) diff(w *writer) error {
	captured := map[int64][]int64{}
	rs, err := s.tx.QueryContext(s.ctx, `SELECT t, r FROM temp._lattis_cap ORDER BY t, r`)
	if err != nil {
		return err
	}
	for rs.Next() {
		var t, r int64
		if err := rs.Scan(&t, &r); err != nil {
			rs.Close()
			return err
		}
		captured[t] = append(captured[t], r)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	var tables []int64
	for id := range s.pub {
		if len(captured[id]) > 0 || s.ddl[id] || s.replaced[id] {
			tables = append(tables, id)
		}
	}
	if len(tables) == 0 {
		return nil
	}
	slices.Sort(tables)
	// Instantané d'avant : un lecteur voit la dernière transaction validée,
	// l'écrivain (unique) étant au milieu de celle-ci.
	before, err := s.r.db.Read().BeginTx(s.ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer before.Rollback()
	for _, id := range tables {
		if err := s.diffTable(w, before, id, captured[id]); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) diffTable(w *writer, before *sql.Tx, id int64, rids []int64) error {
	bcols, err := columns(s.ctx, before, id)
	if err != nil {
		return err
	}
	acols, err := columns(s.ctx, s.tx, id)
	if err != nil {
		return err
	}
	full := s.replaced[id]
	switch {
	case bcols == nil && acols == nil:
		return nil
	case acols == nil:
		// Tableau supprimé (ou mis à la corbeille) : une opération de
		// structure ; ses lignes gardent leurs identifiants (l'annulation le
		// rétablit tel quel).
		return w.add(command.Op{Kind: command.OpMeta, Table: id, Meta: &command.MetaChange{Object: "table", Action: "drop"}})
	case bcols == nil:
		if err := w.add(command.Op{Kind: command.OpMeta, Table: id, Meta: &command.MetaChange{Object: "table", Action: "create", After: colTypes(acols)}}); err != nil {
			return err
		}
		full = true
	}
	// Colonnes comparées : celles d'après, saisies (pas calculées), au même
	// type qu'avant. Un changement de structure est une opération « meta » ;
	// une colonne apparue vaut NULL avant (rétablissement de ses valeurs).
	var cols []int64
	for cid, c := range acols {
		b, had := bcols[cid]
		if bcols != nil && (s.ddl[id] || full) {
			switch {
			case !had:
				if err := w.add(command.Op{Kind: command.OpMeta, Table: id, Meta: &command.MetaChange{Object: "column", Action: "add", ID: cid, After: c.typ}}); err != nil {
					return err
				}
			case b.typ != c.typ:
				if err := w.add(command.Op{Kind: command.OpMeta, Table: id, Meta: &command.MetaChange{Object: "column", Action: "retype", ID: cid, Before: b.typ, After: c.typ}}); err != nil {
					return err
				}
			}
		}
		if c.formula || had && b.typ != c.typ {
			continue
		}
		cols = append(cols, cid)
	}
	if bcols != nil && (s.ddl[id] || full) {
		var gone []int64
		for cid := range bcols {
			if _, ok := acols[cid]; !ok {
				gone = append(gone, cid)
			}
		}
		slices.Sort(gone)
		for _, cid := range gone {
			if err := w.add(command.Op{Kind: command.OpMeta, Table: id, Meta: &command.MetaChange{Object: "column", Action: "remove", ID: cid, Before: bcols[cid].typ}}); err != nil {
				return err
			}
		}
	}
	slices.Sort(cols)

	name := tableName(id)
	if full {
		rids, err = s.allIDs(before, bcols != nil, name)
		if err != nil {
			return err
		}
	}
	// Au-delà de la plus grande clé d'avant, une ligne est forcément nouvelle :
	// un grand collage ne relit rien dans l'instantané d'avant.
	var bmax int64 = -1 << 63
	if bcols != nil {
		var m sql.NullInt64
		if err := before.QueryRowContext(s.ctx, `SELECT max(id) FROM `+name).Scan(&m); err != nil {
			return err
		}
		if !m.Valid {
			bmax = -1 << 63
		} else {
			bmax = m.Int64
		}
	}
	var mmax sql.NullInt64
	if err := s.tx.QueryRowContext(s.ctx, `SELECT max(rid) FROM _lattis_row_ids WHERE tbl = ?`, id).Scan(&mmax); err != nil {
		return err
	}

	bsel := []string{"id", "_pos"}
	asel := []string{"id", "_pos"}
	for _, cid := range cols {
		c := "c_" + strconv.FormatInt(cid, 10)
		asel = append(asel, c)
		if _, ok := bcols[cid]; ok {
			bsel = append(bsel, c)
		} else {
			bsel = append(bsel, "NULL")
		}
	}
	for start := 0; start < len(rids); start += chunk {
		part := rids[start:min(start+chunk, len(rids))]
		var old []int64
		for _, r := range part {
			if r <= bmax {
				old = append(old, r)
			}
		}
		var brows map[int64]row
		if bcols != nil && len(old) > 0 {
			if brows, err = s.readRows(before, name, bsel, old); err != nil {
				return err
			}
		}
		arows, err := s.readRows(s.tx, name, asel, part)
		if err != nil {
			return err
		}
		ids := map[int64]string{}
		if mmax.Valid && part[0] <= mmax.Int64 {
			if ids, err = s.rowIDs(id, part); err != nil {
				return err
			}
		}
		var added [][2]any
		var removed []int64
		for _, r := range part {
			b, inB := brows[r]
			a, inA := arows[r]
			uid, known := ids[r]
			switch {
			case inB && !inA:
				if known {
					if err := w.add(command.Op{Kind: command.OpDelete, Table: id, Row: uid, Old: values(cols, b.vals)}); err != nil {
						return err
					}
					removed = append(removed, r)
				}
			case inA && (!inB || !known):
				// Ligne nouvelle, ou ligne sans identifiant (jamais envoyée) ; une
				// ligne nouvelle qui a déjà un identifiant vient d'un tableau
				// rétabli par une annulation : elle le garde.
				if !known {
					u, err := uuid.NewV7()
					if err != nil {
						return err
					}
					uid = u.String()
					added = append(added, [2]any{r, uid})
				}
				if err := w.add(command.Op{Kind: command.OpInsert, Table: id, Row: uid, New: values(cols, a.vals), Pos: a.pos}); err != nil {
					return err
				}
			case inA && inB:
				for i, cid := range cols {
					if !same(b.vals[i], a.vals[i]) {
						if err := w.add(command.Op{Kind: command.OpSet, Table: id, Row: uid, Col: cid, Old: b.vals[i], New: a.vals[i]}); err != nil {
							return err
						}
					}
				}
				if b.pos != a.pos {
					if err := w.add(command.Op{Kind: command.OpMove, Table: id, Row: uid, Old: b.pos, Pos: a.pos}); err != nil {
						return err
					}
				}
			}
		}
		if err := s.saveIDs(id, removed, added); err != nil {
			return err
		}
	}
	return nil
}

func colTypes(cols map[int64]column) map[int64]string {
	out := map[int64]string{}
	for id, c := range cols {
		if !c.formula {
			out[id] = c.typ
		}
	}
	return out
}

// allIDs donne les clés des lignes d'avant et d'après (tableau comparé en
// entier).
func (s *session) allIDs(before *sql.Tx, existed bool, name string) ([]int64, error) {
	seen := map[int64]bool{}
	var out []int64
	read := func(db snapshot) error {
		rs, err := db.QueryContext(s.ctx, `SELECT id FROM `+name)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				return err
			}
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
		return rs.Err()
	}
	if existed {
		if err := read(before); err != nil {
			return nil, err
		}
	}
	if err := read(s.tx); err != nil {
		return nil, err
	}
	slices.Sort(out)
	return out, nil
}

// idList encode des clés pour json_each.
func idList(ids []int64) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatInt(id, 10))
	}
	b.WriteByte(']')
	return b.String()
}

func (s *session) readRows(db snapshot, name string, sel []string, ids []int64) (map[int64]row, error) {
	rs, err := db.QueryContext(s.ctx, `SELECT `+strings.Join(sel, ", ")+` FROM `+name+` WHERE id IN (SELECT value FROM json_each(?))`, idList(ids))
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := make(map[int64]row, len(ids))
	n := len(sel) - 2
	for rs.Next() {
		var id int64
		var pos string
		vals := make([]any, n)
		dest := make([]any, n+2)
		dest[0], dest[1] = &id, &pos
		for i := range vals {
			dest[i+2] = &vals[i]
		}
		if err := rs.Scan(dest...); err != nil {
			return nil, err
		}
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		out[id] = row{pos, vals}
	}
	return out, rs.Err()
}

func (s *session) rowIDs(tbl int64, rids []int64) (map[int64]string, error) {
	rs, err := s.tx.QueryContext(s.ctx, `SELECT rid, lattis_id FROM _lattis_row_ids WHERE tbl = ? AND rid IN (SELECT value FROM json_each(?))`, tbl, idList(rids))
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[int64]string{}
	for rs.Next() {
		var r int64
		var u string
		if err := rs.Scan(&r, &u); err != nil {
			return nil, err
		}
		out[r] = u
	}
	return out, rs.Err()
}

// saveIDs oublie les identifiants des lignes supprimées et range ceux des
// lignes nouvelles.
func (s *session) saveIDs(tbl int64, removed []int64, added [][2]any) error {
	if len(removed) > 0 {
		if _, err := s.tx.ExecContext(s.ctx, `DELETE FROM _lattis_row_ids WHERE tbl = ? AND rid IN (SELECT value FROM json_each(?))`, tbl, idList(removed)); err != nil {
			return err
		}
	}
	return insertIDs(s.ctx, s.tx, tbl, added)
}

func insertIDs(ctx context.Context, tx *sql.Tx, tbl int64, added [][2]any) error {
	if len(added) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, a := range added {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `[%d,%q]`, a[0], a[1])
	}
	b.WriteByte(']')
	_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO _lattis_row_ids (tbl, rid, lattis_id) SELECT ?, value->>0, value->>1 FROM json_each(?)`, tbl, b.String())
	return err
}

// values rend les valeurs non vides d'une ligne.
func values(cols []int64, vals []any) command.Values {
	out := command.Values{}
	for i, v := range vals {
		if v != nil {
			out[cols[i]] = v
		}
	}
	return out
}

func same(a, b any) bool { return a == b }

// --- File d'envoi ------------------------------------------------------------

// writer range les opérations dans _lattis_outbox par paquets (une entrée
// par paquet de chunk opérations, toutes de la même transaction : batch).
type writer struct {
	s     *session
	c     command.Commit
	buf   []command.Op
	batch int64
	fwd   *forward
}

func (w *writer) add(op command.Op) error {
	if g := w.s.r.Gate; g != nil {
		send, err := g(op)
		if err != nil {
			return err
		}
		if !send {
			return nil
		}
	}
	if w.c.Kind != command.KindDo && w.c.Seq != 0 && (op.Kind == command.OpSet || op.Kind == command.OpMove) {
		if w.fwd == nil {
			f, err := loadForward(w.s.ctx, w.s.tx, w.c.Seq, w.c.Kind)
			if err != nil {
				return err
			}
			w.fwd = f
		}
		w.fwd.condition(&op)
	}
	w.buf = append(w.buf, op)
	if len(w.buf) >= chunk {
		return w.flush()
	}
	return nil
}

func (w *writer) flush() error {
	if len(w.buf) == 0 {
		return nil
	}
	data, err := command.EncodeOps(w.buf)
	if err != nil {
		return err
	}
	w.buf = w.buf[:0]
	var seq any
	if w.c.Seq != 0 {
		seq = w.c.Seq
	}
	var id int64
	if err := w.s.tx.QueryRowContext(w.s.ctx, `INSERT INTO _lattis_outbox (seq, kind, batch, ops) VALUES (?, ?, ?, ?) RETURNING id`, seq, w.c.Kind, w.batch, data).Scan(&id); err != nil {
		return err
	}
	if w.batch == 0 {
		w.batch = id
		_, err = w.s.tx.ExecContext(w.s.ctx, `UPDATE _lattis_outbox SET batch = id WHERE id = ?`, id)
	}
	return err
}

// forward donne, pour une annulation ou un rétablissement, ce que la
// commande avait écrit : l'opération inverse est conditionnée à « la
// cellule vaut toujours ce que j'y ai mis », même si une modification
// reçue d'un collègue a changé la valeur locale entre-temps.
type forward struct {
	cells map[cellKey]any
	pos   map[rowKey]string
}

type cellKey struct {
	t   int64
	r   string
	col int64
}

type rowKey struct {
	t int64
	r string
}

func loadForward(ctx context.Context, tx *sql.Tx, seq int64, kind string) (*forward, error) {
	f := &forward{cells: map[cellKey]any{}, pos: map[rowKey]string{}}
	rs, err := tx.QueryContext(ctx, `SELECT ops FROM _lattis_outbox WHERE seq = ? AND kind = 'do' ORDER BY id`, seq)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var data string
		if err := rs.Scan(&data); err != nil {
			return nil, err
		}
		ops, err := command.DecodeOps(data)
		if err != nil {
			return nil, err
		}
		for _, op := range ops {
			switch op.Kind {
			case command.OpSet:
				k := cellKey{op.Table, op.Row, op.Col}
				if kind == command.KindUndo {
					f.cells[k] = op.New // la dernière valeur écrite
				} else if _, ok := f.cells[k]; !ok {
					f.cells[k] = op.Old // la valeur d'avant la commande
				}
			case command.OpMove:
				k := rowKey{op.Table, op.Row}
				if kind == command.KindUndo {
					f.pos[k] = op.Pos
				} else if _, ok := f.pos[k]; !ok {
					f.pos[k], _ = op.Old.(string)
				}
			}
		}
	}
	return f, rs.Err()
}

func (f *forward) condition(op *command.Op) {
	switch op.Kind {
	case command.OpSet:
		if v, ok := f.cells[cellKey{op.Table, op.Row, op.Col}]; ok {
			op.Old = v
		}
	case command.OpMove:
		if p, ok := f.pos[rowKey{op.Table, op.Row}]; ok {
			op.Old = p
		}
	}
}
