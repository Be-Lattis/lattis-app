// Package collab synchronise un classeur collaboratif ouvert avec son
// Serveur Lattis (spécification de l'application collaborative, §6.2, §6.3,
// §7, §10 ; contrat client de lattisd, §6) :
//
//   - envoi : la file d'envoi (_lattis_outbox, remplie par le journal
//     logique, internal/remote/oplog) part dans l'ordre, une transaction
//     PostgreSQL par étape, écritures conditionnées à la valeur vue ; une
//     écriture sans effet est expliquée par lattis.why_not (conflit, droits,
//     ligne supprimée) ;
//   - réception : les changements des autres (lattis.changes_since, sur
//     notification) s'appliquent hors du journal d'annulation
//     (canvas.ApplyRemote), sans toucher aux cellules dont une modification
//     locale attend encore le serveur ;
//   - conflits : notés dans _lattis_conflicts, réglés par « Garder la
//     mienne » (renvoi conditionné à la valeur du serveur) ou « Garder la
//     sienne ».
//
// Voir docs/collaboration/synchronisation.md.
package collab

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/store"
)

// Engine synchronise un classeur collaboratif ouvert.
type Engine struct {
	db    *store.Store
	model *canvas.Model
	link  *workbook.Link

	// run : un envoi ou une réception à la fois.
	run sync.Mutex

	mu      sync.Mutex
	srv     workbook.Server
	editing *cell // cellule en cours de saisie (réception différée)
	marks   markCache
	// kicker : réveil du suivi (Follow) pour un envoi (Kick).
	kicker chan struct{}
	// retry : délais des nouveaux essais de connexion (SetBackoff ; nil :
	// le délai fixe passé à Follow).
	retry *workbook.Backoff
	// choices : catégories des onglets à choix déjà examinées
	// (workbook.ChoiceSignature, PublishChoices) ; sous run.
	choices string
	// Mise en page (layout.go, sous mu) : écritures déjà synchronisées
	// (Store.WatchGen), dernière génération vue et quand.
	layoutDone, layoutSeen int64
	layoutAt               time.Time
	layoutTheme            string
}

// SetBackoff règle les nouveaux essais de connexion du suivi (délais
// croissants, Wake pour essayer tout de suite).
func (e *Engine) SetBackoff(b *workbook.Backoff) {
	e.mu.Lock()
	e.retry = b
	e.mu.Unlock()
}

func (e *Engine) backoff() *workbook.Backoff {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.retry
}

// cell désigne une cellule d'un tableau (ligne par sa clé locale).
type cell struct {
	tbl, rid, col int64
}

// New crée la synchronisation d'un classeur (sans serveur tant que
// SetServer n'est pas appelé : rien ne part, rien n'arrive).
func New(db *store.Store, model *canvas.Model, link *workbook.Link) *Engine {
	// Écritures de la mise en page comptées : une modification locale est
	// une mise en page à envoyer (layout.go).
	db.Watch(workbook.LayoutTables()...)
	e := &Engine{db: db, model: model, link: link, layoutDone: db.WatchGen()}
	e.layoutTheme = e.theme()
	return e
}

// SetServer donne le serveur (sous l'identité de l'utilisateur).
func (e *Engine) SetServer(srv workbook.Server) {
	e.mu.Lock()
	e.srv = srv
	e.mu.Unlock()
}

func (e *Engine) server() workbook.Server {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.srv
}

// active : classeur collaboratif relié à son serveur.
func (e *Engine) active() (*workbook.Info, workbook.Server) {
	info := e.link.Info()
	srv := e.server()
	if info == nil || info.Mode != workbook.Collaborative || srv == nil {
		return nil, nil
	}
	return info, srv
}

// SetEditing note la cellule en cours de saisie (tbl 0 : aucune) : une
// modification reçue pour elle attend la validation (§6.3), qui peut
// produire un conflit.
func (e *Engine) SetEditing(tbl, rid, col int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if tbl == 0 {
		e.editing = nil
		return
	}
	e.editing = &cell{tbl, rid, col}
}

// --- Tableaux publiés ------------------------------------------------------------

// rtable est un tableau publié et sa correspondance avec le serveur.
type rtable struct {
	local   int64
	remote  workbook.RemoteTable
	byLocal map[int64]workbook.RemoteColumn
	byName  map[string]int64
	since   string
}

// remoteTables lit les tableaux publiés et leurs colonnes du serveur
// (_lattis_remote_tables).
func remoteTables(ctx context.Context, q interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}) (map[int64]*rtable, error) {
	rs, err := q.QueryContext(ctx, `SELECT tbl, coalesce(remote_name, ''), remote_id, coalesce(columns, '[]'), coalesce(since, '') FROM _lattis_remote_tables WHERE published = 1 AND remote_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[int64]*rtable{}
	for rs.Next() {
		t := &rtable{byLocal: map[int64]workbook.RemoteColumn{}, byName: map[string]int64{}}
		var cols string
		if err := rs.Scan(&t.local, &t.remote.Name, &t.remote.ID, &cols, &t.since); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(cols), &t.remote.Columns); err != nil {
			return nil, fmt.Errorf("tableau %d : colonnes du serveur illisibles : %w", t.local, err)
		}
		t.remote.LocalID = t.local
		for _, c := range t.remote.Columns {
			if c.LocalID != 0 {
				t.byLocal[c.LocalID] = c
				t.byName[c.Name] = c.LocalID
			}
		}
		out[t.local] = t
	}
	return out, rs.Err()
}

// toLocal convertit une ligne du serveur pour canvas.ApplyRemote.
func (t *rtable) toLocal(c workbook.Change) canvas.RemoteRow {
	r := canvas.RemoteRow{ID: c.ID, Deleted: c.Deleted, Pos: c.Pos, Author: c.Author}
	if !c.Deleted {
		r.Values = map[int64]any{}
		for name, v := range c.Values {
			if id, ok := t.byName[name]; ok {
				r.Values[id] = v
			}
		}
	}
	return r
}

// rowToLocal convertit une ligne lue en entier (Reader.Rows : valeurs
// dans l'ordre des colonnes du serveur) pour canvas.ApplyRemote.
func (t *rtable) rowToLocal(id, pos, author string, vals []any) canvas.RemoteRow {
	r := canvas.RemoteRow{ID: id, Pos: pos, Author: author, Values: map[int64]any{}}
	for i, c := range t.remote.Columns {
		if c.LocalID != 0 && i < len(vals) {
			r.Values[c.LocalID] = vals[i]
		}
	}
	return r
}

// --- File d'envoi ------------------------------------------------------------------

// entry est une entrée de la file, décodée.
type entry struct {
	id, seq int64
	kind    string
	ops     []command.Op
}

// nextBatch lit la première étape en attente (toutes ses entrées).
func (e *Engine) nextBatch(ctx context.Context) ([]entry, error) {
	rs, err := e.db.Read().QueryContext(ctx, `SELECT id, coalesce(seq, 0), kind, ops FROM _lattis_outbox
		WHERE state = 'pending' AND batch = (SELECT batch FROM _lattis_outbox WHERE state = 'pending' ORDER BY id LIMIT 1) ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []entry
	for rs.Next() {
		var en entry
		var data string
		if err := rs.Scan(&en.id, &en.seq, &en.kind, &data); err != nil {
			return nil, err
		}
		if en.ops, err = command.DecodeOps(data); err != nil {
			return nil, fmt.Errorf("file d'envoi, entrée %d : %w", en.id, err)
		}
		out = append(out, en)
	}
	return out, rs.Err()
}

// keep relève, dans la transaction de réception, les cellules à ne pas
// remplacer : modifications locales en attente d'envoi, conflits ouverts,
// cellule en cours de saisie.
func (e *Engine) keep(tx *sql.Tx) (canvas.Keep, error) {
	k := canvas.Keep{}
	if err := pendingCells(context.Background(), tx, func(tbl int64, id string, col int64) { k.Add(tbl, id, col) }); err != nil {
		return nil, err
	}
	rs, err := tx.Query(`SELECT tbl, row_id, col FROM _lattis_conflicts`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var tbl, col int64
		var id string
		if err := rs.Scan(&tbl, &id, &col); err != nil {
			rs.Close()
			return nil, err
		}
		k.Add(tbl, id, col)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	e.mu.Lock()
	ed := e.editing
	e.mu.Unlock()
	if ed != nil {
		var id string
		if err := tx.QueryRow(`SELECT lattis_id FROM _lattis_row_ids WHERE tbl = ? AND rid = ?`, ed.tbl, ed.rid).Scan(&id); err == nil {
			k.Add(ed.tbl, id, ed.col)
		}
	}
	return k, nil
}

// Colonnes spéciales de Keep et des marques : 0 la position de la ligne,
// -1 la ligne entière (supprimée en local, ou ajoutée et pas encore
// envoyée).
const (
	posCol int64 = 0
	rowCol int64 = -1
)

// pendingCells parcourt les cellules des entrées en attente d'envoi.
func pendingCells(ctx context.Context, q interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}, fn func(tbl int64, id string, col int64)) error {
	rs, err := q.QueryContext(ctx, `SELECT ops FROM _lattis_outbox WHERE state = 'pending' ORDER BY id`)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		var data string
		if err := rs.Scan(&data); err != nil {
			return err
		}
		ops, err := command.DecodeOps(data)
		if err != nil {
			return err
		}
		for _, op := range ops {
			switch op.Kind {
			case command.OpSet:
				fn(op.Table, op.Row, op.Col)
			case command.OpMove:
				fn(op.Table, op.Row, posCol)
			case command.OpInsert:
				fn(op.Table, op.Row, rowCol)
				fn(op.Table, op.Row, posCol)
				if vals, ok := op.New.(command.Values); ok {
					for c := range vals {
						fn(op.Table, op.Row, c)
					}
				}
			case command.OpDelete:
				fn(op.Table, op.Row, rowCol)
			}
		}
	}
	return rs.Err()
}

// Purge retire de la file les entrées envoyées dont l'annulation n'a plus
// besoin (commande sortie du journal, ou renvoi d'un conflit).
func (e *Engine) purge(ctx context.Context) error {
	return e.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_outbox WHERE state <> 'pending'
			AND (seq IS NULL OR seq NOT IN (SELECT seq FROM _lattis_journal))
			AND id NOT IN (SELECT coalesce(outbox, 0) FROM _lattis_conflicts)`)
		return err
	})
}

// Counts compte les étapes en attente d'envoi et les conflits ouverts, et
// les note dans l'état de partage. state : l'état de partage a changé
// (conflits, file hors ligne) ; sync : la file a changé (barre d'état).
func (e *Engine) Counts(ctx context.Context) (pending, conflicts int, state, sync bool, err error) {
	err = e.db.Read().QueryRowContext(ctx, `SELECT (SELECT count(DISTINCT batch) FROM _lattis_outbox WHERE state = 'pending'), (SELECT count(*) FROM _lattis_conflicts)`).Scan(&pending, &conflicts)
	if err == nil {
		state, sync = e.link.SetQueue(pending, conflicts)
	}
	return
}

// --- Marques de la grille -----------------------------------------------------------

// Marques d'une cellule (grille) : modification en attente du serveur, ou
// conflit.
const (
	MarkPending  = "pending"
	MarkConflict = "conflict"
)

// markCache garde les cellules marquées tant que la file et les conflits
// ne changent pas.
type markCache struct {
	version string
	cells   map[int64]map[string]map[int64]string // tbl → lattis_id → colonne → marque
}

// Marks donne les marques des lignes rids d'un tableau : clé locale →
// colonne (0 : la position, -1 : toute la ligne) → MarkPending ou
// MarkConflict.
func (e *Engine) Marks(ctx context.Context, tbl int64, rids []int64) (map[int64]map[int64]string, error) {
	if info := e.link.Info(); info == nil || info.Mode != workbook.Collaborative || len(rids) == 0 {
		return nil, nil
	}
	var maxID, n, nc int64
	if err := e.db.Read().QueryRowContext(ctx, `SELECT coalesce((SELECT max(id) FROM _lattis_outbox WHERE state = 'pending'), 0),
		(SELECT count(*) FROM _lattis_outbox WHERE state = 'pending'), (SELECT coalesce(max(id), 0) * 1000003 + count(*) FROM _lattis_conflicts)`).Scan(&maxID, &n, &nc); err != nil {
		return nil, err
	}
	version := fmt.Sprint(maxID, ":", n, ":", nc)
	e.mu.Lock()
	cache := e.marks
	e.mu.Unlock()
	if cache.version != version {
		cells := map[int64]map[string]map[int64]string{}
		add := func(tbl int64, id string, col int64, mark string) {
			if cells[tbl] == nil {
				cells[tbl] = map[string]map[int64]string{}
			}
			if cells[tbl][id] == nil {
				cells[tbl][id] = map[int64]string{}
			}
			if cells[tbl][id][col] != MarkConflict {
				cells[tbl][id][col] = mark
			}
		}
		if err := pendingCells(ctx, e.db.Read(), func(tbl int64, id string, col int64) { add(tbl, id, col, MarkPending) }); err != nil {
			return nil, err
		}
		rs, err := e.db.Read().QueryContext(ctx, `SELECT tbl, row_id, col FROM _lattis_conflicts`)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var t, c int64
			var id string
			if err := rs.Scan(&t, &id, &c); err != nil {
				rs.Close()
				return nil, err
			}
			add(t, id, c, MarkConflict)
		}
		rs.Close()
		cache = markCache{version: version, cells: cells}
		e.mu.Lock()
		e.marks = cache
		e.mu.Unlock()
	}
	marked := cache.cells[tbl]
	if len(marked) == 0 {
		return nil, nil
	}
	ids, err := lattisIDs(ctx, e.db.Read(), tbl, rids)
	if err != nil {
		return nil, err
	}
	out := map[int64]map[int64]string{}
	for rid, id := range ids {
		if m := marked[id]; len(m) > 0 {
			out[rid] = m
		}
	}
	return out, nil
}

func lattisIDs(ctx context.Context, db *sql.DB, tbl int64, rids []int64) (map[int64]string, error) {
	var b strings.Builder
	b.WriteByte('[')
	for i, r := range rids {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprint(&b, r)
	}
	b.WriteByte(']')
	rs, err := db.QueryContext(ctx, `SELECT rid, lattis_id FROM _lattis_row_ids WHERE tbl = ? AND rid IN (SELECT value FROM json_each(?))`, tbl, b.String())
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[int64]string{}
	for rs.Next() {
		var r int64
		var id string
		if err := rs.Scan(&r, &id); err != nil {
			return nil, err
		}
		out[r] = id
	}
	return out, rs.Err()
}

// --- Annulation -----------------------------------------------------------------------

// UndoNotice rend le message de la barre d'état après une annulation (ou
// un rétablissement, redo) dont des cellules, modifiées par un collègue
// depuis, n'ont pas été touchées (§7), ou qui a rétabli sur ce poste
// seulement un tableau supprimé du serveur ; "" sinon.
func UndoNotice(rec *oplog.Recorder, redo bool) string {
	if rec == nil {
		return ""
	}
	var out []string
	for _, s := range []string{skippedNotice(rec.TakeSkipped(), redo), rec.TakeNotice()} {
		if s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " ; ")
}

func skippedNotice(n int, redo bool) string {
	verb, verbs := "annulée", "annulées"
	if redo {
		verb, verbs = "rétablie", "rétablies"
	}
	switch {
	case n == 1:
		return "1 cellule modifiée par un collègue depuis n'a pas été " + verb
	case n > 1:
		return fmt.Sprintf("%d cellules modifiées par un collègue depuis n'ont pas été %s", n, verbs)
	}
	return ""
}

// ErrNotCollaborative : l'action ne vaut que pour un classeur collaboratif.
var ErrNotCollaborative = errors.New("ce classeur n'est pas un classeur collaboratif ouvert depuis son serveur")
