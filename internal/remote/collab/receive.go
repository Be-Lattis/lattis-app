package collab

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Receive lit les changements des tableaux publiés depuis leur point de
// reprise (lattis.changes_since, une lecture cohérente sur rw) et les
// applique hors du journal d'annulation, avec le nouveau point de reprise
// dans la même transaction locale. Les écritures de ce poste n'en
// reviennent pas (skip : ses transactions, notées à l'envoi). Un point de
// reprise que les pierres tombales ne couvrent plus
// (lattis.since_covered) fait recharger le tableau en entier. only :
// tableaux locaux à relire (nil : tous).
func (e *Engine) Receive(ctx context.Context, only map[int64]bool) (Report, error) {
	e.run.Lock()
	defer e.run.Unlock()
	return e.receive(ctx, only, nil)
}

// receive : full, tableaux à relire en entier (structure changée).
func (e *Engine) receive(ctx context.Context, only, full map[int64]bool) (Report, error) {
	var rep Report
	info, srv := e.active()
	if info == nil {
		return rep, nil
	}
	tables, err := remoteTables(ctx, e.db.Read())
	if err != nil {
		return rep, err
	}
	sent, err := readSkip(ctx, e.db.Read())
	if err != nil {
		return rep, err
	}
	probes, err := readProbes(ctx, e.db.Read())
	if err != nil {
		return rep, err
	}
	// Structure changée ici et pas encore faite sur le serveur : le tableau
	// est relu après son envoi (sa correspondance ne la reflète pas encore).
	waiting, err := pendingStructure(ctx, e.db.Read())
	if err != nil {
		return rep, err
	}
	var batches []canvas.RemoteTable
	since := map[int64]string{}
	probe := ""
	reloaded := 0
	err = srv.Delta(ctx, func(r workbook.Reader) error {
		batches, since, probe, reloaded = nil, map[int64]string{}, "", 0
		withProbe := false
		// Point de reprise lu au début de la lecture : une transaction
		// validée après lui sera relue la prochaine fois (contrat §6).
		sp, err := r.SyncPoint(ctx)
		if err != nil {
			return err
		}
		for _, tbl := range sortedKeys(tables) {
			if (only != nil && !only[tbl]) || waiting[tbl] {
				continue
			}
			t := tables[tbl]
			rt := canvas.RemoteTable{Table: tbl}
			// Serveur sans since_covered (known faux) : le point de reprise
			// est supposé couvert.
			covered, _, err := r.Covered(ctx, t.since)
			if err == nil && !covered && probes[tbl] != "" {
				// Rechargé alors qu'une transaction longue retenait since sous
				// la purge : seules comptent les purges que le rechargement
				// n'a pas vues (voir Reader.Probe et Reader.CoveredBy).
				withProbe = true
				covered, err = r.CoveredBy(ctx, t.since, probes[tbl])
			}
			if err != nil {
				return fmt.Errorf("tableau « %s » : %w", t.remote.Name, err)
			}
			if full[tbl] {
				covered = false // structure changée : tout est relu
			}
			if covered {
				err = r.Changes(ctx, info.WorkbookID, t.remote, t.since, skipSince(sent, t.since), func(c workbook.Change) error {
					rt.Rows = append(rt.Rows, t.toLocal(c))
					return nil
				})
			} else {
				// Des suppressions ont pu être purgées depuis since : tout le
				// tableau est relu, les lignes absentes sont supprimées.
				rt.Full = true
				reloaded++
				err = r.Rows(ctx, info.Schema, t.remote, func(id, pos, author string, vals []any) error {
					rt.Rows = append(rt.Rows, t.rowToLocal(id, pos, author, vals))
					return nil
				})
			}
			if err != nil {
				return fmt.Errorf("tableau « %s » : %w", t.remote.Name, err)
			}
			batches = append(batches, rt)
			since[tbl] = sp
		}
		// Le nouveau point de reprise est-il couvert ? Non tant qu'une
		// transaction commencée avant la dernière purge dure (sync_point
		// est le plus petit xid en cours) : le prochain Covered(since)
		// serait faux et ferait recharger encore. L'instantané de cette
		// lecture (ou sa fin, sans la migration 0010) sert alors de témoin
		// (Reader.Probe), oublié dès que Covered(since) seul redevient vrai.
		if reloaded > 0 || withProbe {
			ok, _, err := r.Covered(ctx, sp)
			if err == nil && !ok {
				probe, err = r.Probe(ctx)
			}
			return err
		}
		return nil
	})
	if err != nil {
		return rep, err
	}
	dirty := false
	for tbl := range since {
		if probes[tbl] != probe {
			dirty = true
			if probe != "" {
				probes[tbl] = probe
			} else {
				delete(probes, tbl)
			}
		}
	}
	applied, err := e.model.ApplyRemote(ctx, batches, e.keep, func(tx *sql.Tx) error {
		for tbl, sp := range since {
			if _, err := tx.ExecContext(ctx, `UPDATE _lattis_remote_tables SET since = ? WHERE tbl = ?`, sp, tbl); err != nil {
				return err
			}
		}
		if dirty {
			if err := writeProbes(ctx, tx, probes); err != nil {
				return err
			}
		}
		return pruneSkip(ctx, tx)
	})
	rep.Reloaded = reloaded
	rep.Applied = applied
	if len(applied.Failed) > 0 {
		rep.notice(fmt.Sprintf("%d ligne(s) reçue(s) du serveur n'ont pas pu être appliquées : %s", len(applied.Failed), applied.Failed[0]))
	}
	return rep, err
}

// --- Échos ----------------------------------------------------------------------------

// skipKey : transactions du serveur de ce poste (xid8 en texte, JSON),
// dans _lattis_info, écartées des changements reçus (contrat client §6 :
// changes_since … skip). Gardées tant qu'un point de reprise peut les
// précéder.
const skipKey = "sync_skip"

// maxSkip borne la liste (les plus récentes sont gardées).
const maxSkip = 2000

func readSkip(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}) ([]string, error) {
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, skipKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	if json.Unmarshal([]byte(raw), &out) != nil {
		return nil, nil // illisible : les échos reviennent, sans effet
	}
	return out, nil
}

func writeSkip(ctx context.Context, tx *sql.Tx, xs []string) error {
	if len(xs) > maxSkip {
		xs = xs[len(xs)-maxSkip:]
	}
	raw, _ := json.Marshal(xs)
	_, err := tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, skipKey, string(raw))
	return err
}

// addSkip note une transaction d'écriture de ce poste.
func addSkip(ctx context.Context, tx *sql.Tx, xid string) error {
	xs, err := readSkip(ctx, tx)
	if err != nil {
		return err
	}
	if slices.Contains(xs, xid) {
		return nil
	}
	return writeSkip(ctx, tx, append(xs, xid))
}

// pruneSkip oublie les transactions antérieures à tous les points de
// reprise (elles ne peuvent plus revenir).
func pruneSkip(ctx context.Context, tx *sql.Tx) error {
	xs, err := readSkip(ctx, tx)
	if err != nil || len(xs) == 0 {
		return err
	}
	rs, err := tx.QueryContext(ctx, `SELECT coalesce(since, '') FROM _lattis_remote_tables WHERE published = 1 AND remote_id IS NOT NULL`)
	if err != nil {
		return err
	}
	var all []string
	for rs.Next() {
		var s string
		if err := rs.Scan(&s); err != nil {
			rs.Close()
			return err
		}
		if s == "" {
			rs.Close()
			return nil // tableau jamais lu : tout peut revenir
		}
		all = append(all, s)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	keep := xs[:0:0]
	for _, x := range xs {
		for _, s := range all {
			if workbook.XidAfter(x, s) {
				keep = append(keep, x)
				break
			}
		}
	}
	if len(keep) == len(xs) {
		return nil
	}
	return writeSkip(ctx, tx, keep)
}

// --- Témoins de couverture ----------------------------------------------------------

// probeKey : témoin de couverture par tableau (Reader.Probe ; JSON, tableau
// local → instantané pg_snapshot en texte, ou xid8 en texte sans la
// migration 0010), dans _lattis_info. Noté quand le point de reprise d'un
// tableau rechargé n'est pas couvert, oublié dès qu'il l'est.
const probeKey = "sync_probe"

func readProbes(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}) (map[int64]string, error) {
	out := map[int64]string{}
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, probeKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if json.Unmarshal([]byte(raw), &out) != nil {
		return map[int64]string{}, nil // illisible : au pire, un rechargement de plus
	}
	return out, nil
}

func writeProbes(ctx context.Context, tx *sql.Tx, m map[int64]string) error {
	if len(m) == 0 {
		_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key = ?`, probeKey)
		return err
	}
	raw, _ := json.Marshal(m)
	_, err := tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, probeKey, string(raw))
	return err
}

// skipSince garde les transactions postérieures à since (contrat §6).
func skipSince(xs []string, since string) []string {
	var out []string
	for _, x := range xs {
		if workbook.XidAfter(x, since) {
			out = append(out, x)
		}
	}
	return out
}

// Sync envoie la file puis lit la structure et les changements de tous
// les tableaux publiés (ouverture, reconnexion).
func (e *Engine) Sync(ctx context.Context) (Report, error) {
	e.run.Lock()
	defer e.run.Unlock()
	rep, err := e.send(ctx)
	if err != nil {
		return rep, err
	}
	r, err := e.receiveStructure(ctx, nil, nil)
	rep.add(r)
	return rep, err
}

// TableOf donne le tableau local d'un identifiant de tableau du serveur
// (charge d'une notification) ; 0 s'il n'est pas publié ici.
func (e *Engine) TableOf(ctx context.Context, remoteID string) (int64, error) {
	var tbl int64
	err := e.db.Read().QueryRowContext(ctx, `SELECT tbl FROM _lattis_remote_tables WHERE published = 1 AND remote_id = ?`, remoteID).Scan(&tbl)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return tbl, err
}

// --- Conflits ---------------------------------------------------------------------

// Conflict est un conflit en attente de décision (volet Conflits) :
// « Modifié par Alice à 10:42 : 1 100 (vous : 1 200) ».
type Conflict struct {
	ID     int64  `json:"id"`
	Table  string `json:"table"`
	Column string `json:"column"`
	// Type : type de la colonne (affichage des valeurs).
	Type string `json:"type"`
	// Mine, Theirs : ma valeur et celle du serveur (valeurs de stockage).
	Mine   any    `json:"mine"`
	Theirs any    `json:"theirs"`
	Author string `json:"author"`
	// At : date de la modification du collègue (RFC 3339), "" si inconnue.
	At string `json:"at"`
	// Sheet, Row, Col : la cellule sur sa feuille (Found faux : ligne
	// supprimée ou masquée par un filtre).
	Sheet int64 `json:"sheet"`
	Row   int   `json:"row"`
	Col   int   `json:"col"`
	Found bool  `json:"found"`
}

// Conflicts liste les conflits ouverts.
func (e *Engine) Conflicts(ctx context.Context) ([]Conflict, error) {
	rs, err := e.db.Read().QueryContext(ctx, `SELECT c.id, c.tbl, c.row_id, c.col, coalesce(c.local, 'null'), coalesce(c.server, 'null'), coalesce(c.author, ''), coalesce(c.at, ''),
		coalesce(t.name, ''), coalesce(k.name, ''), coalesce(k.type, 'text'), coalesce(r.rid, 0)
		FROM _lattis_conflicts c
		LEFT JOIN _lattis_tables t ON t.id = c.tbl
		LEFT JOIN _lattis_columns k ON k.tbl = c.tbl AND k.id = c.col
		LEFT JOIN _lattis_row_ids r ON r.tbl = c.tbl AND r.lattis_id = c.row_id
		ORDER BY c.id`)
	if err != nil {
		return nil, err
	}
	type located struct {
		c             Conflict
		tbl, rid, col int64
	}
	var all []located
	for rs.Next() {
		var l located
		var row, mine, theirs string
		if err := rs.Scan(&l.c.ID, &l.tbl, &row, &l.col, &mine, &theirs, &l.c.Author, &l.c.At, &l.c.Table, &l.c.Column, &l.c.Type, &l.rid); err != nil {
			rs.Close()
			return nil, err
		}
		l.c.Mine, l.c.Theirs = typed(l.c.Type, decodeValue(mine)), typed(l.c.Type, decodeValue(theirs))
		all = append(all, l)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	out := make([]Conflict, 0, len(all))
	for _, l := range all {
		if l.rid != 0 {
			s, r, c, ok, err := e.model.CellOf(ctx, l.tbl, l.rid, l.col)
			if err != nil {
				return nil, err
			}
			l.c.Sheet, l.c.Row, l.c.Col, l.c.Found = s, r, c, ok
		}
		out = append(out, l.c)
	}
	return out, nil
}

// typed rend une valeur décodée au type de stockage de sa colonne (un
// décimal entier, 12.0, s'écrit 12 en JSON).
func typed(typ string, v any) any {
	if n, ok := v.(int64); ok && typ == "decimal" {
		return float64(n)
	}
	return v
}

func decodeValue(s string) any {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil {
		return nil
	}
	if n, ok := v.(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			return i
		}
		f, _ := n.Float64()
		return f
	}
	return v
}

// Resolve règle un conflit : mine, « Garder la mienne » (ma valeur repart,
// conditionnée à la valeur actuelle du serveur) ; sinon « Garder la
// sienne » (la valeur du serveur remplace la mienne en local, hors du
// journal d'annulation).
func (e *Engine) Resolve(ctx context.Context, id int64, mine bool) error {
	e.run.Lock()
	defer e.run.Unlock()
	var tbl, col int64
	var row, local, server string
	err := e.db.Read().QueryRowContext(ctx, `SELECT tbl, row_id, col, coalesce(local, 'null'), coalesce(server, 'null') FROM _lattis_conflicts WHERE id = ?`, id).Scan(&tbl, &row, &col, &local, &server)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("conflit %d introuvable (déjà réglé ?)", id)
	}
	if err != nil {
		return err
	}
	theirs := decodeValue(server)
	if mine {
		return e.db.Write(ctx, func(tx *sql.Tx) error {
			// Ma valeur, telle qu'elle est maintenant dans la cellule.
			var cur any
			err := tx.QueryRowContext(ctx, `SELECT t.c_`+fmt.Sprint(col)+` FROM t_`+fmt.Sprint(tbl)+` AS t JOIN _lattis_row_ids AS r ON r.tbl = ? AND r.rid = t.id WHERE r.lattis_id = ?`, tbl, row).Scan(&cur)
			if errors.Is(err, sql.ErrNoRows) {
				cur = decodeValue(local)
			} else if err != nil {
				return err
			}
			if b, ok := cur.([]byte); ok {
				cur = string(b)
			}
			ops, err := command.EncodeOps([]command.Op{{Kind: command.OpSet, Table: tbl, Row: row, Col: col, Old: theirs, New: cur}})
			if err != nil {
				return err
			}
			var oid int64
			if err := tx.QueryRowContext(ctx, `INSERT INTO _lattis_outbox (seq, kind, batch, ops) VALUES (NULL, 'do', 0, ?) RETURNING id`, ops).Scan(&oid); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE _lattis_outbox SET batch = id WHERE id = ?`, oid); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `DELETE FROM _lattis_conflicts WHERE id = ?`, id)
			return err
		})
	}
	rows := []canvas.RemoteTable{{Table: tbl, Rows: []canvas.RemoteRow{{ID: row, Values: map[int64]any{col: theirs}}}}}
	_, err = e.model.ApplyRemote(ctx, rows, func(tx *sql.Tx) (canvas.Keep, error) {
		if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_conflicts WHERE id = ?`, id); err != nil {
			return nil, err
		}
		k, err := e.keep(tx)
		if err == nil {
			delete(k[tbl][row], col) // la cellule en conflit prend la valeur du serveur
		}
		return k, err
	}, nil)
	return err
}

// --- Suivi du serveur ----------------------------------------------------------------

// Event décrit ce que le suivi a changé, pour l'interface.
type Event struct {
	// Data : des données locales ont changé (grille à relire).
	Data bool
	// State : connexion, file d'attente ou conflits (bandeau, volet).
	State bool
	// Sync : file d'envoi en cours ou changée (barre d'état :
	// « Synchronisation… »), sans relire la mise en page.
	Sync bool
	// Notices : messages pour la barre d'état.
	Notices []string
}

// Debounce : délai de regroupement des notifications reçues avant de lire
// les changements (spécification §16, point 2 : une rafale d'écritures ne
// recalcule les formules qu'une fois).
var Debounce = 200 * time.Millisecond

// Follow suit un classeur collaboratif ouvert jusqu'à l'annulation de ctx
// : écoute de son canal (LISTEN), envoi de la file et lecture de tous les
// changements à chaque (re)connexion, lecture des changements d'un tableau
// sur notification (regroupées sur Debounce), envoi à chaque Kick. Le
// serveur coupé, le classeur passe hors ligne (lecture seule, §10) et un
// nouvel essai a lieu toutes les retry. changed est appelé hors de tout
// verrou, depuis un seul fil.
func (e *Engine) Follow(ctx context.Context, srv workbook.Server, retry time.Duration, changed func(Event)) {
	info := e.link.Info()
	if info == nil {
		return
	}
	e.SetServer(srv)
	if changed == nil {
		changed = func(Event) {}
	}
	w := &worker{e: e, kick: make(chan struct{}, 1), due: map[int64]bool{}, structure: map[int64]bool{}, changed: changed, retry: retry}
	e.mu.Lock()
	e.kicker = w.kick
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.kicker = nil
		e.mu.Unlock()
	}()
	go w.loop(ctx)
	for ctx.Err() == nil {
		err := srv.Listen(ctx, info.Channel, func() {
			w.connected()
		}, func(p string) {
			w.notified(ctx, p)
		})
		if ctx.Err() != nil {
			return
		}
		reason := "serveur injoignable"
		if err != nil {
			reason = err.Error()
		}
		w.disconnected(reason)
		select {
		case <-ctx.Done():
			return
		case <-time.After(retry):
		}
	}
}

// Kick demande l'envoi de la file (après une modification locale) ; sans
// effet si le classeur n'est pas suivi.
func (e *Engine) Kick() {
	e.mu.Lock()
	k := e.kicker
	e.mu.Unlock()
	if k != nil {
		select {
		case k <- struct{}{}:
		default:
		}
	}
}
