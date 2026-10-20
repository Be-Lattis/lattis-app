package coedit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/remote/oplog"
)

// cellKey désigne une cellule (col 0 : la position de la ligne).
type cellKey struct {
	tbl int64
	row string
	col int64
}

// stamp date une écriture : horloge de Lamport, journal (départage), ligne.
type stamp struct {
	clock   int64
	journal string
	seq     int64
}

func (a stamp) less(b stamp) bool {
	if a.clock != b.clock {
		return a.clock < b.clock
	}
	return a.journal < b.journal
}

// stampLocal note les cellules écrites par une ligne de ce poste.
func (e *Engine) stampLocal(l Line) {
	if len(l.Ops) == 0 {
		return
	}
	ops, err := command.DecodeOps(string(l.Ops))
	if err != nil {
		return
	}
	st := stamp{l.Clock, e.journal, l.Seq}
	for _, op := range ops {
		for _, k := range opCells(op) {
			e.cells[k] = st
		}
	}
}

// opCells : cellules écrites par une opération.
func opCells(op command.Op) []cellKey {
	switch op.Kind {
	case command.OpSet:
		return []cellKey{{op.Table, op.Row, op.Col}}
	case command.OpMove:
		return []cellKey{{op.Table, op.Row, 0}}
	case command.OpInsert:
		out := []cellKey{{op.Table, op.Row, 0}}
		if vals, ok := op.New.(command.Values); ok {
			for c := range vals {
				out = append(out, cellKey{op.Table, op.Row, c})
			}
		}
		return out
	}
	return nil
}

// --- Réception -----------------------------------------------------------------

// receive lit les journaux des autres sessions et applique les lignes
// prêtes, dans l'ordre causal.
func (e *Engine) receive(ctx context.Context, ev *Event, entries []os.DirEntry) error {
	e.files = map[string]bool{}
	for _, en := range entries {
		n := en.Name()
		if en.IsDir() || !strings.HasSuffix(n, journalExt) {
			continue
		}
		j := strings.TrimSuffix(n, journalExt)
		e.files[j] = true
		if e.mine(j) {
			continue
		}
		lines, bad, err := e.rd.read(e.dir, j)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		if bad > 0 {
			// Ligne illisible : peut-être une écriture interrompue puis
			// reprise ; relu depuis le début au tour suivant.
			e.rd.rewind[j] = true
		}
		for _, l := range lines {
			if l.Seq <= e.applied[j] || slices.ContainsFunc(e.pending[j], func(p Line) bool { return p.Seq == l.Seq }) {
				continue
			}
			e.pending[j] = append(e.pending[j], l)
			if l.Seq > e.maxSeq[j] {
				e.maxSeq[j] = l.Seq
			}
		}
		slices.SortFunc(e.pending[j], func(a, b Line) int { return int(a.Seq - b.Seq) })
	}
	// Journaux disparus (compactés) : plus rien à en attendre.
	for j := range e.pending {
		if !e.files[j] {
			delete(e.pending, j)
		}
	}
	for j := range e.rd.off {
		if !e.files[j] {
			e.rd.forget(j)
		}
	}
	return e.applyReady(ctx, ev)
}

// depsOK : les lignes dont dépend l sont appliquées (ou ne viendront
// plus : journal disparu).
func (e *Engine) depsOK(l Line, applied map[string]int64) bool {
	for j, s := range l.Deps {
		if e.mine(j) || applied[j] >= s {
			continue
		}
		if !e.files[j] {
			continue // journal compacté : ce qu'il avait est dans le fichier
		}
		return false
	}
	return true
}

// plan prépare l'application d'un lot de lignes prêtes.
type plan struct {
	tables  []canvas.RemoteTable
	index   map[int64]int
	cells   map[cellKey]stamp
	applied map[string]int64
	clock   int64
	cands   []candidate
	by      map[string]int // auteur → opérations reçues
	barrier []string
	lines   int
}

// candidate : cellule saisie ici que la valeur reçue remplace sans que son
// auteur l'ait vue (conflit si les valeurs diffèrent).
type candidate struct {
	k      cellKey
	theirs any
	author string
	at     string
	mine   any
	keep   bool
}

func (p *plan) row(tbl int64, r canvas.RemoteRow) {
	i, ok := p.index[tbl]
	if !ok {
		i = len(p.tables)
		p.index[tbl] = i
		p.tables = append(p.tables, canvas.RemoteTable{Table: tbl})
	}
	p.tables[i].Rows = append(p.tables[i].Rows, r)
}

// applyReady applique, dans l'ordre causal (puis de l'horloge), les lignes
// dont les dépendances sont satisfaites.
func (e *Engine) applyReady(ctx context.Context, ev *Event) error {
	p := &plan{index: map[int64]int{}, cells: map[cellKey]stamp{}, applied: map[string]int64{}, clock: e.clock, by: map[string]int{}}
	for k, v := range e.applied {
		p.applied[k] = v
	}
	cell := func(k cellKey) (stamp, bool) {
		if s, ok := p.cells[k]; ok {
			return s, true
		}
		s, ok := e.cells[k]
		return s, ok
	}
	for {
		var next *Line
		for j, ls := range e.pending {
			// Lignes déjà prises dans ce lot.
			for len(ls) > 0 && ls[0].Seq <= p.applied[j] {
				ls = ls[1:]
			}
			if len(ls) == 0 || ls[0].Seq != p.applied[j]+1 || !e.depsOK(ls[0], p.applied) {
				if len(ls) > 0 && ls[0].Seq > p.applied[j]+1 {
					e.gaps[j]++
					if e.gaps[j] > 20 {
						e.rd.rewind[j] = true // trou qui dure : relire le journal
						e.gaps[j] = 0
					}
				}
				continue
			}
			l := ls[0]
			if next == nil || l.Clock < next.Clock || l.Clock == next.Clock && l.journal < next.journal {
				next = &l
			}
		}
		if next == nil {
			break
		}
		l := *next
		j := l.journal
		p.applied[j] = l.Seq
		p.lines++
		e.gaps[j] = 0
		if l.Clock > p.clock {
			p.clock = l.Clock
		}
		if l.V != LineVersion || l.Kind == kindBarrier {
			p.barrier = append(p.barrier, l.By)
			continue
		}
		if len(l.Ops) == 0 {
			continue
		}
		ops, err := command.DecodeOps(string(l.Ops))
		if err != nil {
			p.barrier = append(p.barrier, l.By)
			continue
		}
		st := stamp{l.Clock, j, l.Seq}
		for _, op := range ops {
			switch op.Kind {
			case command.OpSet, command.OpMove:
				col := op.Col
				if op.Kind == command.OpMove {
					col = 0
				}
				k := cellKey{op.Table, op.Row, col}
				cur, had := cell(k)
				if had && st.less(cur) {
					continue // une écriture plus récente l'emporte
				}
				if had && e.mine(cur.journal) && l.Deps[cur.journal] < cur.seq && op.Kind == command.OpSet {
					p.cands = append(p.cands, candidate{k: k, theirs: localValue(op.New), author: l.By, at: l.At})
				}
				p.cells[k] = st
				r := canvas.RemoteRow{ID: op.Row, Author: l.By, NoCreate: true}
				if op.Kind == command.OpSet {
					r.Values = map[int64]any{op.Col: localValue(op.New)}
				} else {
					r.Pos = op.Pos
				}
				p.row(op.Table, r)
			case command.OpInsert:
				vals := map[int64]any{}
				if v, ok := op.New.(command.Values); ok {
					for c, x := range v {
						vals[c] = localValue(x)
					}
				}
				for _, k := range opCells(op) {
					p.cells[k] = st
				}
				p.row(op.Table, canvas.RemoteRow{ID: op.Row, Pos: op.Pos, Author: l.By, Values: vals})
			case command.OpDelete:
				p.row(op.Table, canvas.RemoteRow{ID: op.Row, Deleted: true})
			default:
				continue
			}
			p.by[l.By]++
		}
	}
	if p.lines == 0 {
		return nil
	}
	var conflicts []candidate
	res, err := e.d.ApplyRemote(ctx, p.tables, func(tx *sql.Tx) (canvas.Keep, error) {
		k, err := pendingKeep(ctx, tx)
		if err != nil {
			return nil, err
		}
		conflicts = conflicts[:0]
		for i := range p.cands {
			c := &p.cands[i]
			if k.Has(c.k.tbl, c.k.row, c.k.col) {
				continue // ma valeur part avec une horloge plus grande
			}
			mine, ok, err := cellValue(ctx, tx, c.k)
			if err != nil {
				return nil, err
			}
			if ok && !oplog.SameValue(mine, c.theirs) {
				c.mine = mine
				conflicts = append(conflicts, *c)
			}
		}
		return k, nil
	}, func(tx *sql.Tx) error {
		for _, c := range conflicts {
			local, _ := json.Marshal(c.mine)
			theirs, _ := json.Marshal(c.theirs)
			if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_conflicts WHERE tbl = ? AND row_id = ? AND col = ?`, c.k.tbl, c.k.row, c.k.col); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_conflicts (outbox, tbl, row_id, col, local, server, author, at) VALUES (NULL, ?, ?, ?, ?, ?, ?, ?)`,
				c.k.tbl, c.k.row, c.k.col, string(local), string(theirs), c.author, c.at); err != nil {
				return err
			}
		}
		all := map[string]int64{}
		for k, v := range p.applied {
			all[k] = v
		}
		for k, v := range e.own {
			all[k] = v
		}
		all[e.journal] = e.seq
		return writeApplied(ctx, tx, all)
	})
	if err != nil {
		return err
	}
	// Lot appliqué : il fait foi.
	for k, v := range p.cells {
		e.cells[k] = v
	}
	e.applied = p.applied
	for j, ls := range e.pending {
		i := 0
		for i < len(ls) && ls[i].Seq <= e.applied[j] {
			i++
		}
		if i == len(ls) {
			delete(e.pending, j)
		} else {
			e.pending[j] = ls[i:]
		}
	}
	if p.clock > e.clock {
		e.clock = p.clock
	}
	n := res.Inserted + res.Updated + res.Deleted
	if n > 0 || len(conflicts) > 0 {
		e.d.MarkChanged()
		ev.Data = true
	}
	if n > 0 {
		e.note(ev, receivedNotice(p.by))
	}
	if len(conflicts) > 0 {
		ev.State = true
		e.note(ev, fmt.Sprintf("%d conflit(s) : des cellules que vous aviez modifiées ont pris la valeur d'un collègue (volet Conflits)", len(conflicts)))
	}
	if len(res.Failed) > 0 {
		e.note(ev, fmt.Sprintf("%d ligne(s) reçue(s) n'ont pas pu être appliquées : %s", len(res.Failed), res.Failed[0]))
	}
	if len(p.barrier) > 0 {
		e.mu.Lock()
		first := e.stale == ""
		e.stale = fmt.Sprintf("%s a modifié le classeur hors des données des tableaux pendant votre déconnexion : rouvrez-le pour voir ses changements ; d'ici là, enregistrer par-dessus le fichier demandera confirmation", p.barrier[0])
		msg := e.stale
		e.mu.Unlock()
		if first {
			ev.State = true
			e.note(ev, msg)
		}
	}
	return nil
}

// receivedNotice : « 3 modifications reçues de Bob ».
func receivedNotice(by map[string]int) string {
	names := make([]string, 0, len(by))
	for n := range by {
		names = append(names, n)
	}
	sort.Strings(names)
	var parts []string
	for _, n := range names {
		who := n
		if who == "" {
			who = "un autre poste"
		}
		if by[n] == 1 {
			parts = append(parts, "1 modification reçue de "+who)
		} else {
			parts = append(parts, fmt.Sprintf("%d modifications reçues de %s", by[n], who))
		}
	}
	return strings.Join(parts, ", ")
}

// localValue convertit une valeur lue dans un journal (JSON) en valeur de
// stockage SQLite : nombres entiers ou réels.
func localValue(v any) any {
	if n, ok := v.(json.Number); ok {
		if i, err := strconv.ParseInt(string(n), 10, 64); err == nil {
			return i
		}
		if f, err := n.Float64(); err == nil {
			return f
		}
		return string(n)
	}
	return v
}

// cellValue lit la valeur d'une cellule d'un tableau (ok faux : ligne
// absente).
func cellValue(ctx context.Context, tx *sql.Tx, k cellKey) (any, bool, error) {
	var v any
	err := tx.QueryRowContext(ctx, `SELECT t.c_`+strconv.FormatInt(k.col, 10)+` FROM t_`+strconv.FormatInt(k.tbl, 10)+` AS t JOIN _lattis_row_ids AS r ON r.tbl = ? AND r.rid = t.id WHERE r.lattis_id = ?`, k.tbl, k.row).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, nil // colonne disparue : pas de conflit
	}
	if b, ok := v.([]byte); ok {
		v = string(b)
	}
	return v, true, nil
}

// pendingKeep : cellules d'une modification locale pas encore envoyée
// (elle partira avec une horloge plus grande et l'emportera partout).
func pendingKeep(ctx context.Context, tx *sql.Tx) (canvas.Keep, error) {
	k := canvas.Keep{}
	rs, err := tx.QueryContext(ctx, `SELECT ops FROM _lattis_outbox WHERE state = 'pending' ORDER BY id`)
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
			case command.OpDelete:
				k.Add(op.Table, op.Row, -1)
			default:
				for _, c := range opCells(op) {
					k.Add(c.tbl, c.row, c.col)
				}
			}
		}
	}
	return k, rs.Err()
}

// --- Compactage -------------------------------------------------------------------

// manifestCache garde les positions du fichier enregistré tant qu'il ne
// change pas.
type manifestCache struct {
	size    int64
	mod     time.Time
	journal map[string]int64
	ok      bool
}

func (e *Engine) fileJournal() map[string]int64 {
	st, err := os.Stat(e.path)
	if err != nil {
		return nil
	}
	c := &e.manifest
	if c.ok && c.size == st.Size() && c.mod.Equal(st.ModTime()) {
		return c.journal
	}
	m, err := archive.ReadManifest(e.path)
	if err != nil {
		return nil
	}
	*c = manifestCache{size: st.Size(), mod: st.ModTime(), journal: m.Journal, ok: true}
	return m.Journal
}

// compact supprime les segments que le fichier enregistré couvre et que
// toutes les sessions vivantes ont appliqués, ainsi que les sessions
// coupées encore attendues (roster.away, AwayKeep), quand leur session est
// partie ou en a ouvert un autre ; puis les présences mortes sans journal.
func (e *Engine) compact(live []Presence, entries []os.DirEntry) {
	saved := e.fileJournal()
	if saved == nil {
		return
	}
	waiting := append(slices.Clone(live), e.roster.away(time.Now())...)
	current := map[string]string{} // session vivante → segment en cours
	for _, p := range live {
		current[p.Session] = p.Journal
	}
	current[e.sid] = e.journal
	left := map[string]bool{}
	for _, en := range entries {
		n := en.Name()
		if !strings.HasSuffix(n, journalExt) {
			continue
		}
		j := strings.TrimSuffix(n, journalExt)
		sid := sessionOf(j)
		if cur, ok := current[sid]; ok && (cur == j || cur == "") {
			left[sid] = true
			continue
		}
		var last int64
		switch {
		case e.mine(j):
			last = e.own[j]
		case e.rd.complete(j) && len(e.pending[j]) == 0:
			last = e.applied[j]
		default:
			left[sid] = true
			continue
		}
		ok := saved[j] >= last && e.appliedAll()[j] >= last
		for _, p := range waiting {
			if p.Session != sid && p.Applied[j] < last {
				ok = false
			}
		}

		if !ok {
			left[sid] = true
			continue
		}
		if os.Remove(filepath.Join(e.dir, n)) == nil {
			e.rd.forget(j)
			delete(e.pending, j)
		}
	}
	for _, sid := range e.roster.staleSessions(time.Now()) {
		if !left[sid] {
			os.Remove(filepath.Join(e.dir, sid+presenceExt))
		}
	}
}
