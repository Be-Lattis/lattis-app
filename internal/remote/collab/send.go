package collab

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Report décrit un envoi ou une réception.
type Report struct {
	// Sent : étapes confirmées par le serveur.
	Sent int
	// Conflicts : cellules en conflit (modifiées par un collègue entre-temps).
	Conflicts int
	// Rejected : opérations refusées (droits, valeur), défaites en local.
	Rejected int
	// Skipped : cellules qu'une annulation n'a pas touchées (un collègue les
	// avait modifiées entre-temps).
	Skipped int
	// Lost : modifications de lignes supprimées entre-temps par un collègue.
	Lost int
	// Applied : changements appliqués en local (réception, remise à l'état
	// du serveur après un refus).
	Applied canvas.RemoteApplied
	// Reloaded : tableaux relus en entier (point de reprise que les
	// pierres tombales du serveur ne couvrent plus).
	Reloaded int
	// Notices : messages pour la barre d'état.
	Notices []string
}

// Changed : des données locales ont changé (la grille est à relire).
func (r Report) Changed() bool { return len(r.Applied.Tables) > 0 }

func (r *Report) notice(msg string) {
	if msg != "" && !slices.Contains(r.Notices, msg) {
		r.Notices = append(r.Notices, msg)
	}
}

func (r *Report) add(o Report) {
	r.Sent += o.Sent
	r.Conflicts += o.Conflicts
	r.Rejected += o.Rejected
	r.Skipped += o.Skipped
	r.Lost += o.Lost
	r.Reloaded += o.Reloaded
	r.Applied.Inserted += o.Applied.Inserted
	r.Applied.Updated += o.Applied.Updated
	r.Applied.Deleted += o.Applied.Deleted
	r.Applied.Kept += o.Applied.Kept
	r.Applied.Failed = append(r.Applied.Failed, o.Applied.Failed...)
	for _, t := range o.Applied.Tables {
		if !slices.Contains(r.Applied.Tables, t) {
			r.Applied.Tables = append(r.Applied.Tables, t)
		}
	}
	for _, n := range o.Notices {
		r.notice(n)
	}
}

// Send vide la file d'envoi dans l'ordre, une transaction du serveur par
// étape. Une coupure (erreur rendue) laisse le reste en attente : il
// repartira à la reconnexion, avec la détection de conflit normale (§10).
func (e *Engine) Send(ctx context.Context) (Report, error) {
	e.run.Lock()
	defer e.run.Unlock()
	return e.send(ctx)
}

func (e *Engine) send(ctx context.Context) (Report, error) {
	var rep Report
	info, srv := e.active()
	if info == nil || !e.link.Online() {
		return rep, nil
	}
	tables, err := remoteTables(ctx, e.db.Read())
	if err != nil {
		return rep, err
	}
	retried := false
	for ctx.Err() == nil {
		entries, err := e.nextBatch(ctx)
		if err != nil {
			return rep, err
		}
		if len(entries) == 0 {
			break
		}
		o, err := e.sendBatch(ctx, srv, info, tables, entries)
		if errors.Is(err, errStale) && !retried {
			// Structure changée pendant l'envoi (colonne ou tableau renommé
			// par ce poste) : correspondance relue, l'étape repart une fois.
			retried = true
			if tables, err = remoteTables(ctx, e.db.Read()); err != nil {
				return rep, err
			}
			continue
		}
		retried = false
		if err != nil {
			return rep, err
		}
		if err := e.finish(ctx, srv, info, tables, o, &rep); err != nil {
			return rep, err
		}
	}
	if err := ctx.Err(); err != nil {
		return rep, err
	}
	return rep, e.purge(ctx)
}

// item est une opération à envoyer, avec sa correspondance sur le serveur.
type item struct {
	entry int // rang de l'entrée dans l'étape
	op    command.Op
	t     *rtable
	col   workbook.RemoteColumn   // set
	cols  []workbook.RemoteColumn // insert
	vals  []any                   // insert
}

// outcome est le résultat de l'envoi d'une étape.
type outcome struct {
	entries   []entry
	state     []string // par entrée : sent, conflict, rejected
	errs      []string
	conflicts []conflict
	resync    map[int64]map[string]bool // tableau → lignes à relire sur le serveur
	// authors : auteur des lignes ajoutées (tableau → lattis_id →
	// lattis_author), noté en local : leur écho ne revient plus.
	authors map[int64]map[string]string
	// xid : transaction du serveur de l'étape (Writer.Xact), écartée des
	// changements reçus ensuite (échos).
	xid string
	rep Report
}

// conflict est un conflit à noter.
type conflict struct {
	outbox     int64
	tbl        int64
	row        string
	col        int64
	local, srv any
	author, at string
}

func newOutcome(entries []entry) *outcome {
	o := &outcome{entries: entries, state: make([]string, len(entries)), errs: make([]string, len(entries)), resync: map[int64]map[string]bool{},
		authors: map[int64]map[string]string{}}
	for i := range o.state {
		o.state[i] = "sent"
	}
	return o
}

func (o *outcome) reset() {
	for i := range o.state {
		o.state[i], o.errs[i] = "sent", ""
	}
	o.conflicts, o.resync, o.rep = nil, map[int64]map[string]bool{}, Report{}
	o.authors, o.xid = map[int64]map[string]string{}, ""
}

func (o *outcome) refetch(tbl int64, id string) {
	if o.resync[tbl] == nil {
		o.resync[tbl] = map[string]bool{}
	}
	o.resync[tbl][id] = true
}

// reject note une opération refusée : l'entrée est refusée, la ligne
// reprend l'état du serveur.
func (o *outcome) reject(it item, msg string) {
	if o.state[it.entry] != "rejected" {
		o.state[it.entry] = "rejected"
		o.errs[it.entry] = msg
	}
	o.rep.Rejected++
	o.refetch(it.op.Table, it.op.Row)
}

// sendBatch envoie une étape dans une transaction du serveur. Une erreur
// rendue est une coupure (rien n'est noté) ; un refus du serveur est dans
// le résultat. Chaque écriture a son point de sauvegarde : une opération
// refusée par une erreur SQL (filtre d'écriture d'un participant, droits)
// est seule refusée, le reste de l'étape s'applique.
func (e *Engine) sendBatch(ctx context.Context, srv workbook.Server, info *workbook.Info, tables map[int64]*rtable, entries []entry) (*outcome, error) {
	o := newOutcome(entries)
	var items []item
	for ei, en := range entries {
		for _, op := range en.ops {
			t := tables[op.Table]
			if t == nil || op.Kind == command.OpMeta {
				// Mise en page : étape 7. La structure ne passe pas par la file :
				// elle est faite sur le serveur avant la validation locale
				// (Engine.Structure).
				continue
			}
			it := item{entry: ei, op: op, t: t}
			switch op.Kind {
			case command.OpSet:
				c, ok := t.byLocal[op.Col]
				if !ok {
					continue // colonne privée (non publiée)
				}
				it.col = c
				if _, err := workbook.ToRemote(c.Type, op.New); err != nil {
					o.reject(it, err.Error())
					continue
				}
			case command.OpInsert:
				vals, _ := op.New.(command.Values)
				cols := make([]int64, 0, len(vals))
				for c := range vals {
					cols = append(cols, c)
				}
				slices.Sort(cols)
				bad := false
				for _, c := range cols {
					rc, ok := t.byLocal[c]
					if !ok {
						continue
					}
					if _, err := workbook.ToRemote(rc.Type, vals[c]); err != nil {
						o.reject(it, fmt.Sprintf("%s : %v", rc.Name, err))
						bad = true
						break
					}
					it.cols = append(it.cols, rc)
					it.vals = append(it.vals, vals[c])
				}
				if bad {
					continue
				}
			}
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		return o, nil
	}
	schema := info.Schema
	err := srv.Write(ctx, func(w workbook.Writer) error {
		o.reset()
		for _, it := range items {
			op := it.op
			var wr workbook.Wrote
			var err error
			switch op.Kind {
			case command.OpSet:
				wr, err = w.Update(ctx, schema, it.t.remote.Name, op.Row, it.col, op.Old, op.New)
			case command.OpInsert:
				wr, err = w.Insert(ctx, schema, it.t.remote.Name, op.Row, op.Pos, it.cols, it.vals)
				if err == nil && wr.Author != "" {
					if o.authors[op.Table] == nil {
						o.authors[op.Table] = map[string]string{}
					}
					o.authors[op.Table][op.Row] = wr.Author
				}
				wr.Rows = 1 // déjà présente : renvoi après une coupure
			case command.OpDelete:
				wr, err = w.Delete(ctx, schema, it.t.remote.Name, op.Row)
			case command.OpMove:
				old, _ := op.Old.(string)
				wr, err = w.Move(ctx, schema, it.t.remote.Name, op.Row, old, op.Pos)
			}
			if err != nil {
				if staleStructure(err) {
					// Colonne ou tableau inconnu du serveur : la correspondance
					// est périmée (structure changée), pas un refus de la
					// valeur ; l'étape reste en attente.
					return fmt.Errorf("%w : %s", errStale, errText(err))
				}
				if _, rejected := workbook.Rejected(err); !rejected {
					return err
				}
				// Refusée seule (point de sauvegarde) : la ligne reprend
				// l'état du serveur, le reste de l'étape continue.
				o.reject(it, errText(err))
				if workbook.Privilege(err) {
					o.rep.notice("Droits insuffisants sur « " + it.t.remote.Name + " » : modification annulée")
				} else {
					o.rep.notice("Modification refusée par le serveur, annulée : " + errText(err))
				}
				continue
			}
			if wr.Rows > 0 {
				// Si la ligne portait l'écriture d'un autre pas encore reçue,
				// elle reviendra par changes_since malgré skip : le serveur
				// garde lattis_other_xid.
				continue
			}
			why, err := w.WhyNot(ctx, info.WorkbookID, it.t.remote, op.Row)
			if err != nil {
				return err
			}
			o.refused(it, why)
		}
		x, err := w.Xact(ctx)
		o.xid = x
		return err
	})
	if err == nil {
		return o, nil
	}
	if _, rejected := workbook.Rejected(err); !rejected {
		return nil, err // coupure : l'étape reste en attente
	}
	// Refus de toute l'étape (droits sur le tableau, contrainte) : défaite
	// en local, chaque ligne reprend l'état du serveur.
	o.reset()
	for _, it := range items {
		o.reject(it, err.Error())
	}
	if workbook.Privilege(err) {
		o.rep.notice("Droits insuffisants sur un tableau partagé : modification annulée")
	} else {
		o.rep.notice("Modification refusée par le serveur, annulée : " + err.Error())
	}
	return o, nil
}

// refused traite une écriture sans effet, expliquée par lattis.why_not.
func (o *outcome) refused(it item, why workbook.WhyNot) {
	op := it.op
	kind := o.entries[it.entry].kind
	switch why.Reason {
	case workbook.WhyModified:
		if op.Kind == command.OpSet && why.Row != nil {
			cur := why.Row.Values[it.col.Name]
			if oplog.SameValue(cur, op.New) {
				return // déjà écrite (renvoi après une coupure)
			}
			if kind != command.KindDo {
				// Annulation ou rétablissement d'une cellule qu'un collègue a
				// modifiée depuis : elle garde sa valeur (§7).
				o.rep.Skipped++
				o.refetch(op.Table, op.Row)
				return
			}
			if o.state[it.entry] == "sent" {
				o.state[it.entry] = "conflict"
			}
			o.conflicts = append(o.conflicts, conflict{outbox: o.entries[it.entry].id, tbl: op.Table, row: op.Row, col: op.Col,
				local: op.New, srv: cur, author: why.By, at: why.At})
			o.rep.Conflicts++
			return
		}
		if op.Kind == command.OpMove && why.Row != nil && why.Row.Pos == op.Pos {
			return
		}
		// Ordre des lignes : celui du serveur l'emporte.
		o.refetch(op.Table, op.Row)
	case workbook.WhyDeleted:
		if op.Kind == command.OpDelete {
			return // déjà supprimée
		}
		o.rep.Lost++
		who := why.By
		if who == "" {
			who = "un collègue"
		}
		o.rep.notice("Ligne supprimée entre-temps par " + who + " : modification abandonnée")
		o.refetch(op.Table, op.Row)
	case workbook.WhyMissing:
		if op.Kind == command.OpDelete {
			return
		}
		o.reject(it, "ligne inconnue du serveur")
	default: // forbidden
		o.reject(it, "droits insuffisants")
		o.rep.notice("Droits insuffisants sur « " + it.t.remote.Name + " » : modification annulée")
	}
}

// finish note le résultat d'une étape : états de la file, conflits, et
// lignes remises à l'état du serveur (refus, annulations sans effet), dans
// une seule transaction locale.
func (e *Engine) finish(ctx context.Context, srv workbook.Server, info *workbook.Info, tables map[int64]*rtable, o *outcome, rep *Report) error {
	var rows []canvas.RemoteTable
	if len(o.resync) > 0 {
		var err error
		if rows, err = e.fetchRows(ctx, srv, info, tables, o.resync); err != nil {
			return err
		}
	}
	mark := func(tx *sql.Tx) (canvas.Keep, error) {
		for i, en := range o.entries {
			var msg any
			if o.errs[i] != "" {
				msg = o.errs[i]
			}
			if _, err := tx.ExecContext(ctx, `UPDATE _lattis_outbox SET state = ?, error = ? WHERE id = ?`, o.state[i], msg, en.id); err != nil {
				return nil, err
			}
		}
		for tbl, rows := range o.authors {
			for id, author := range rows {
				if _, err := tx.ExecContext(ctx, `UPDATE _lattis_row_ids SET lattis_author = ? WHERE tbl = ? AND lattis_id = ? AND lattis_author IS NOT ?`, author, tbl, id, author); err != nil {
					return nil, err
				}
			}
		}
		if o.xid != "" {
			if err := addSkip(ctx, tx, o.xid); err != nil {
				return nil, err
			}
		}
		for _, c := range o.conflicts {
			local, _ := json.Marshal(c.local)
			server, _ := json.Marshal(c.srv)
			if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_conflicts WHERE tbl = ? AND row_id = ? AND col = ?`, c.tbl, c.row, c.col); err != nil {
				return nil, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO _lattis_conflicts (outbox, tbl, row_id, col, local, server, author, at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				c.outbox, c.tbl, c.row, c.col, string(local), string(server), c.author, c.at); err != nil {
				return nil, err
			}
		}
		return e.keep(tx)
	}
	applied, err := e.model.ApplyRemote(ctx, rows, mark, nil)
	if err != nil {
		return err
	}
	o.rep.Applied = applied
	for i := range o.entries {
		if o.state[i] == "sent" {
			o.rep.Sent++
		}
	}
	if o.rep.Conflicts > 0 {
		o.rep.notice(conflictNotice(o.rep.Conflicts))
	}
	if o.rep.Skipped > 0 {
		o.rep.notice(skippedNotice(o.rep.Skipped, o.entries[0].kind == command.KindRedo))
	}
	rep.add(o.rep)
	return nil
}

func conflictNotice(n int) string {
	if n == 1 {
		return "Conflit : une cellule a été modifiée par un collègue entre-temps (voir le volet Conflits)"
	}
	return fmt.Sprintf("Conflits : %d cellules ont été modifiées par un collègue entre-temps (voir le volet Conflits)", n)
}

// fetchRows relit des lignes sur le serveur ; une ligne absente
// (supprimée, ou invisible) est à supprimer en local.
func (e *Engine) fetchRows(ctx context.Context, srv workbook.Server, info *workbook.Info, tables map[int64]*rtable, want map[int64]map[string]bool) ([]canvas.RemoteTable, error) {
	var out []canvas.RemoteTable
	err := srv.Delta(ctx, func(r workbook.Reader) error {
		out = nil
		for _, tbl := range sortedKeys(want) {
			t := tables[tbl]
			if t == nil {
				continue
			}
			ids := make([]string, 0, len(want[tbl]))
			for id := range want[tbl] {
				ids = append(ids, id)
			}
			slices.Sort(ids)
			rt := canvas.RemoteTable{Table: tbl}
			seen := map[string]bool{}
			if err := r.RowsByID(ctx, info.Schema, t.remote, ids, func(c workbook.Change) error {
				seen[c.ID] = true
				rt.Rows = append(rt.Rows, t.toLocal(c))
				return nil
			}); err != nil {
				return err
			}
			for _, id := range ids {
				if !seen[id] {
					rt.Rows = append(rt.Rows, canvas.RemoteRow{ID: id, Deleted: true})
				}
			}
			out = append(out, rt)
		}
		return nil
	})
	return out, err
}

func sortedKeys[V any](m map[int64]V) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// errStale : une écriture vise une colonne ou un tableau que le serveur ne
// connaît plus sous ce nom (structure changée pendant l'envoi).
var errStale = errors.New("structure du tableau changée sur le serveur")

// staleStructure : colonne ou table inconnue (42703, 42P01).
func staleStructure(err error) bool {
	code, ok := workbook.Rejected(err)
	return ok && (code == "42703" || code == "42P01")
}

// errText résume une erreur pour la barre d'état.
func errText(err error) string {
	s := err.Error()
	if i := strings.Index(s, "\n"); i > 0 {
		s = s[:i]
	}
	return s
}
