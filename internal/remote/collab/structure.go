package collab

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Structure des tableaux publiés d'un classeur collaboratif (spécification
// de l'application §6.4 ; fonctions du serveur §9.3, schema/0025) :
//
//   - envoi (écriture optimiste, décision 0012) : le relevé du bus
//     (oplog.Recorder.SetStructure) fait vérifier par Engine.Structure les
//     changements de structure d'une transaction locale (tableau renommé ou
//     supprimé, colonne ajoutée, renommée, supprimée ou convertie, clé) :
//     droits et connexion, en local, sans attendre le serveur. La
//     transaction est validée tout de suite ; les changements partent par
//     la file d'envoi (oplog.OpStructure), faits sur le serveur dans une
//     transaction avant les données de leur étape (sendStructure). Un refus
//     du serveur (une colonne citée par un filtre, un tableau qui alimente
//     le Data Lake…) défait le changement en local, hors du journal
//     d'annulation (revertStructure : forme et données du serveur), et
//     son message s'affiche. L'annulation est une transaction comme une
//     autre : elle envoie l'opération inverse ;
//   - réception : sur « *structure:<table_id> » (clé comprise depuis la
//     migration 0026 ; « *meta » et la connexion : tous), la structure du serveur est comparée à celle que le
//     poste connaît (_lattis_remote_tables.columns) ; le tableau local
//     prend la forme du serveur hors du journal, puis il est relu en
//     entier. L'historique d'annulation qui le touche est effacé : ses
//     étapes ne s'appliqueraient plus.
//
// Voir docs/collaboration/synchronisation.md.

// StructureTimeout borne l'appel au serveur d'un changement de structure
// envoyé par la file, et de l'ajout d'un tableau au classeur partagé.
var StructureTimeout = 30 * time.Second

// RefusedError est un changement de structure refusé par le serveur ; Msg
// est le texte du serveur.
type RefusedError struct {
	Kind string // command.KindDo, KindUndo, KindRedo
	Msg  string
}

func (e *RefusedError) Error() string {
	switch e.Kind {
	case command.KindUndo:
		return "Annulation refusée par le serveur : " + e.Msg
	case command.KindRedo:
		return "Rétablissement refusé par le serveur : " + e.Msg
	}
	return "Refusé par le serveur : " + e.Msg
}

// serverError rend l'erreur d'un appel de structure : refus du serveur
// (RefusedError, son texte) ou coupure.
func serverError(kind string, err error) error {
	if _, ok := workbook.Rejected(err); ok {
		return &RefusedError{Kind: kind, Msg: workbook.ServerMessage(err)}
	}
	return fmt.Errorf("Serveur injoignable, modification de structure annulée : %s", errText(err))
}

func cloneTables(in map[int64]*rtable) map[int64]*rtable {
	out := make(map[int64]*rtable, len(in))
	for k, t := range in {
		c := *t
		c.remote.Columns = slices.Clone(t.remote.Columns)
		out[k] = &c
	}
	return out
}

func (t *rtable) column(local int64) (int, bool) {
	i := slices.IndexFunc(t.remote.Columns, func(c workbook.RemoteColumn) bool { return c.LocalID == local && local != 0 })
	return i, i >= 0
}

// Structure vérifie les changements de structure d'une transaction locale
// (oplog.StructureFunc) avant sa validation, sans appel au serveur :
// tableau verrouillé, propriétaire ou concepteur (en ligne ou non : hors
// ligne, ils attendent dans la file, décision 0014). Acceptés, ils
// partent par la file d'envoi (oplog.OpStructure), faits sur le serveur
// par Send avant les données de leur étape (sendStructure). Classeur
// figé : rien (l'auteur prépare la version suivante en local).
func (e *Engine) Structure(ctx context.Context, tx *sql.Tx, kind string, changes []oplog.Change) error {
	info := e.link.Info()
	if info == nil || info.Mode != workbook.Collaborative || len(changes) == 0 {
		return nil
	}
	for _, ch := range changes {
		if err := e.link.TableLock(ch.Table); err != nil {
			return err
		}
	}
	if !info.CanDesign() {
		return workbook.ErrDesign
	}
	// Hors ligne (ou serveur pas encore joint depuis l'ouverture) : permis
	// aussi, le changement attend dans la file et part à la reconnexion,
	// avant les données qui le suivent ; un refus du serveur le défait
	// alors comme en ligne (décision 0014).
	return nil
}

// isStructure : l'entrée de la file porte des changements de structure.
func (en entry) isStructure() bool {
	return len(en.ops) > 0 && en.ops[0].Kind == oplog.OpStructure
}

// structureChanges relit les changements d'une entrée de structure.
func (en entry) structureChanges() ([]oplog.Change, error) {
	out := make([]oplog.Change, 0, len(en.ops))
	for _, op := range en.ops {
		if op.Kind != oplog.OpStructure {
			continue
		}
		ch, err := oplog.DecodeStructure(op)
		if err != nil {
			return nil, fmt.Errorf("file d'envoi, entrée %d : %w", en.id, err)
		}
		out = append(out, ch)
	}
	return out, nil
}

// RefusedNotice : message de la barre d'état quand le serveur refuse un
// changement de structure déjà appliqué en local (écriture optimiste) : il
// est défait sur ce poste.
func RefusedNotice(kind, msg string) string {
	return (&RefusedError{Kind: kind, Msg: msg}).Error() + " — la modification a été annulée"
}

// sendStructure fait sur le serveur les changements de structure d'une
// entrée de la file, dans une transaction du serveur, et note la nouvelle
// correspondance. Une coupure (erreur rendue) laisse l'entrée en attente.
// Un refus défait le changement en local, hors du journal d'annulation
// (revertStructure), avec les changements de structure suivants des mêmes
// tableaux, encore en attente : ils supposaient celui-ci.
func (e *Engine) sendStructure(ctx context.Context, srv workbook.Server, info *workbook.Info, en entry, rep *Report) error {
	changes, err := en.structureChanges()
	if err != nil {
		return err
	}
	tables, err := remoteTables(ctx, e.db.Read())
	if err != nil {
		return err
	}
	var work map[int64]*rtable
	dropped := map[int64]bool{}
	cctx, cancel := context.WithTimeout(ctx, StructureTimeout)
	defer cancel()
	err = srv.Write(cctx, func(w workbook.Writer) error {
		work, dropped = cloneTables(tables), map[int64]bool{}
		for _, ch := range changes {
			t := work[ch.Table]
			if t == nil || dropped[ch.Table] {
				continue // tableau non publié
			}
			if err := e.structureOne(cctx, w, info.WorkbookID, t, ch); err != nil {
				return err
			}
			if ch.Action == oplog.DropTable {
				dropped[ch.Table] = true
			}
		}
		return nil
	})
	if err != nil {
		if _, ok := workbook.Refusal(err); !ok {
			return err // coupure ou erreur passagère : l'entrée reste en attente
		}
		return e.refuseStructure(ctx, srv, info, en, changes, workbook.ServerMessage(err), rep)
	}
	restored := []string{}
	err = e.db.Write(ctx, func(tx *sql.Tx) error {
		restored = restored[:0]
		for tbl, t := range work {
			if dropped[tbl] {
				if err := forgetTable(ctx, tx, tbl); err != nil {
					return err
				}
				// Rétabli entre-temps par une annulation (avant l'envoi) : il
				// reste sur ce poste seulement.
				var name string
				if tx.QueryRowContext(ctx, `SELECT name FROM _lattis_tables WHERE id = ?`, tbl).Scan(&name) == nil {
					restored = append(restored, name)
				}
				continue
			}
			if err := saveMapping(ctx, tx, tbl, t.remote.Name, t.remote.Columns); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE _lattis_outbox SET state = 'sent' WHERE id = ?`, en.id)
		return err
	})
	if err != nil {
		return err
	}
	for tbl := range dropped {
		e.link.Unpublish(tbl)
	}
	for _, name := range restored {
		rep.notice("Le tableau « " + name + " » est rétabli sur ce poste seulement : il a été supprimé du serveur et n'est plus partagé")
	}
	rep.Sent++
	return nil
}

// refuseStructure traite le refus d'une entrée de structure : elle est
// notée refusée, avec les changements de structure suivants des mêmes
// tableaux encore en attente, et les tableaux reprennent la forme et les
// données du serveur en local (hors du journal d'annulation ; les données
// en attente d'envoi restent et partent ensuite : une valeur d'une colonne
// qui n'est plus sur le serveur est écartée).
func (e *Engine) refuseStructure(ctx context.Context, srv workbook.Server, info *workbook.Info, en entry, changes []oplog.Change, msg string, rep *Report) error {
	tbls := map[int64]bool{}
	added := map[int64]bool{} // colonnes ajoutées en local, à retirer
	calc := map[int64]string{} // colonnes converties en valeurs (ou l'inverse)
	seen := func(ch oplog.Change) {
		if ch.Action == oplog.AddColumn {
			added[ch.Column] = true
		}
		if ch.Formula != "" && (ch.Action == oplog.AddColumn || ch.Action == oplog.DropColumn) {
			calc[ch.Column] = ch.Formula
		}
	}
	for _, ch := range changes {
		tbls[ch.Table] = true
		seen(ch)
	}
	dropped := 0
	err := e.db.Write(ctx, func(tx *sql.Tx) error {
		dropped = 0
		if _, err := tx.ExecContext(ctx, `UPDATE _lattis_outbox SET state = 'rejected', error = ? WHERE id = ?`, msg, en.id); err != nil {
			return err
		}
		rs, err := tx.QueryContext(ctx, `SELECT id, ops FROM _lattis_outbox WHERE state = 'pending' AND id > ? AND ops LIKE ? ORDER BY id`, en.id, `%"k":"`+oplog.OpStructure+`"%`)
		if err != nil {
			return err
		}
		var later []int64
		for rs.Next() {
			var id int64
			var data string
			if err := rs.Scan(&id, &data); err != nil {
				rs.Close()
				return err
			}
			ops, err := command.DecodeOps(data)
			if err != nil {
				rs.Close()
				return err
			}
			hit := false
			for _, op := range ops {
				if op.Kind == oplog.OpStructure && tbls[op.Table] {
					hit = true
					if ch, err := oplog.DecodeStructure(op); err == nil {
						seen(ch)
					}
				}
			}
			if hit {
				later = append(later, id)
			}
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
		for _, id := range later {
			if _, err := tx.ExecContext(ctx, `UPDATE _lattis_outbox SET state = 'rejected', error = ? WHERE id = ?`, "abandonnée : "+msg, id); err != nil {
				return err
			}
			dropped++
		}
		return nil
	})
	if err != nil {
		return err
	}
	rep.Rejected++
	note := RefusedNotice(en.kind, msg)
	if dropped > 0 {
		note += fmt.Sprintf(" (et %d changement(s) de structure suivant(s) du même tableau)", dropped)
	}
	rep.notice(note)
	r, err := e.revertStructure(ctx, srv, info, tbls, added, calc)
	rep.add(r)
	return err
}

// revertStructure redonne aux tableaux tbls la forme et les données du
// serveur, hors du journal d'annulation : un changement de structure
// refusé par le serveur est défait (colonnes ajoutées en local, added,
// retirées ; colonnes supprimées recréées et relues ; noms, types et clé
// du serveur). Un tableau supprimé en local revient comme un tableau reçu
// (receiveArrivals). Une colonne convertie en valeurs (calc) redevient
// calculée au lieu d'être retirée ; une conversion annulée et refusée rend
// sa colonne saisie. L'historique d'annulation qui touche ces tableaux est
// effacé : ses étapes ne s'appliqueraient plus.
func (e *Engine) revertStructure(ctx context.Context, srv workbook.Server, info *workbook.Info, tbls, added map[int64]bool, calc map[int64]string) (Report, error) {
	var rep Report
	var server []workbook.RemoteTable
	err := srv.Delta(ctx, func(r workbook.Reader) error {
		var err error
		server, err = r.Tables(ctx, info.Schema)
		return err
	})
	if err != nil {
		return rep, err
	}
	byID := map[string]workbook.RemoteTable{}
	for _, t := range server {
		byID[t.ID] = t
	}
	type planned struct {
		shape  canvas.RemoteShape
		server workbook.RemoteTable
	}
	var plans []planned
	var gone []int64 // supprimés en local, à recevoir de nouveau
	plan := func(tx *sql.Tx) ([]canvas.RemoteShape, error) {
		plans, gone = nil, nil
		tables, err := remoteTables(ctx, tx)
		if err != nil {
			return nil, err
		}
		for _, tbl := range sortedKeys(tbls) {
			t := tables[tbl]
			if t == nil {
				continue
			}
			st, ok := byID[t.remote.ID]
			if !ok {
				plans = append(plans, planned{shape: canvas.RemoteShape{Table: tbl, Gone: true}})
				continue
			}
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM _lattis_tables WHERE id = ?`, tbl).Scan(&n); err != nil {
				return nil, err
			}
			if n == 0 {
				gone = append(gone, tbl)
				continue
			}
			types, err := localTypes(ctx, tx, tbl)
			if err != nil {
				return nil, err
			}
			var key int64
			if err := tx.QueryRowContext(ctx, `SELECT coalesce(key_col, 0) FROM _lattis_tables WHERE id = ?`, tbl).Scan(&key); err != nil {
				return nil, err
			}
			sh := canvas.RemoteShape{Table: tbl, Name: st.Name, KeyAt: -1, Calc: calc}
			for _, c := range t.remote.Columns {
				if c.LocalID != 0 {
					sh.Published = append(sh.Published, c.LocalID)
				}
			}
			for c := range added {
				if _, ok := types[c]; ok && !slices.Contains(sh.Published, c) {
					sh.Published = append(sh.Published, c)
				}
			}
			old := map[string]workbook.RemoteColumn{}
			for _, c := range t.remote.Columns {
				old[c.ID] = c
			}
			keyLocal := int64(0)
			for i, c := range st.Columns {
				sc := canvas.RemoteShapeColumn{Name: c.Name}
				// Colonne connue : la colonne locale qui lui correspond (recréée
				// si elle a été supprimée en local) ; inconnue (ajoutée par un
				// collègue) : nouvelle.
				o, known := old[c.ID]
				lt, here := types[o.LocalID]
				if known && o.LocalID != 0 && here {
					sc.ID = o.LocalID
					if workbook.RemoteType(lt) != c.Type {
						sc.Type = workbook.LocalType(c.Type)
					}
				} else {
					sc.Want, sc.Type = c.LocalID, workbook.LocalType(c.Type)
					if known && o.LocalID != 0 {
						sc.Want = o.LocalID
					}
				}
				if c.ID == st.Key {
					sh.KeyAt = i
					keyLocal = sc.ID
				}
				sh.Columns = append(sh.Columns, sc)
			}
			sh.KeySet = keyLocal != key || (st.Key != "" && keyLocal == 0)
			plans = append(plans, planned{shape: sh, server: st})
		}
		out := make([]canvas.RemoteShape, len(plans))
		for i, p := range plans {
			out[i] = p.shape
		}
		return out, nil
	}
	forgot := false
	after := func(tx *sql.Tx, cols map[int64][]int64) error {
		forgot = false
		// Les étapes d'annulation qui touchent ces tableaux ne
		// s'appliqueraient plus à leur forme rétablie.
		touched := func(tbl int64) error {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM _lattis_journal WHERE steps GLOB ?`, "*t_"+strconv.FormatInt(tbl, 10)+"[^0-9]*").Scan(&n); err != nil {
				return err
			}
			if n > 0 && !forgot {
				if err := command.Forget(ctx, tx); err != nil {
					return err
				}
				forgot = true
			}
			return nil
		}
		for _, p := range plans {
			tbl := p.shape.Table
			if p.shape.Gone {
				if err := forgetTable(ctx, tx, tbl); err != nil {
					return err
				}
			} else {
				mapped := slices.Clone(p.server.Columns)
				ids := cols[tbl]
				for i := range mapped {
					mapped[i].LocalID = 0
					if i < len(ids) {
						mapped[i].LocalID = ids[i]
					}
					mapped[i].Key = p.server.Key != "" && mapped[i].ID == p.server.Key
				}
				if err := saveMapping(ctx, tx, tbl, p.server.Name, mapped); err != nil {
					return err
				}
			}
			if err := touched(tbl); err != nil {
				return err
			}
		}
		// Tableau supprimé en local et refusé : oublié ici, il revient comme
		// un tableau reçu, avec ses lignes.
		for _, tbl := range gone {
			if err := touched(tbl); err != nil {
				return err
			}
			for _, q := range []string{
				`DELETE FROM _lattis_remote_tables WHERE tbl = ?`,
				`DELETE FROM _lattis_row_ids WHERE tbl = ?`,
				`DELETE FROM _lattis_conflicts WHERE tbl = ?`,
			} {
				if _, err := tx.ExecContext(ctx, q, tbl); err != nil {
					return err
				}
			}
		}
		return nil
	}
	changed, err := e.model.ApplyRemoteShapes(ctx, plan, after)
	if err != nil {
		return rep, err
	}
	if forgot {
		rep.notice("historique d'annulation effacé")
	}
	full := map[int64]bool{}
	for _, p := range plans {
		if p.shape.Gone {
			e.link.Unpublish(p.shape.Table)
			continue
		}
		full[p.shape.Table] = true
	}
	for _, tbl := range changed {
		if !slices.Contains(rep.Applied.Tables, tbl) {
			rep.Applied.Tables = append(rep.Applied.Tables, tbl)
		}
	}
	if len(gone) > 0 {
		for _, tbl := range gone {
			e.link.Unpublish(tbl)
		}
		r, err := e.receiveArrivals(ctx)
		r.Notices = nil // le message du refus suffit
		rep.add(r)
		if err != nil {
			return rep, err
		}
	}
	if len(full) > 0 {
		r, err := e.receive(ctx, full, full)
		rep.add(r)
		if err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// pendingStructure : tableaux dont un changement de structure attend
// encore le serveur. Leur réception attend son envoi : la correspondance
// des colonnes (_lattis_remote_tables) ne le reflète pas encore.
func pendingStructure(ctx context.Context, q interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}) (map[int64]bool, error) {
	rs, err := q.QueryContext(ctx, `SELECT ops FROM _lattis_outbox WHERE state = 'pending' AND ops LIKE ?`, `%"k":"`+oplog.OpStructure+`"%`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out map[int64]bool
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
			if op.Kind == oplog.OpStructure {
				if out == nil {
					out = map[int64]bool{}
				}
				out[op.Table] = true
			}
		}
	}
	return out, rs.Err()
}

// structureOne fait un changement sur le serveur et le reporte dans la
// correspondance t.
func (e *Engine) structureOne(ctx context.Context, w workbook.Writer, wb string, t *rtable, ch oplog.Change) error {
	tid := t.remote.ID
	cols := &t.remote.Columns
	switch ch.Action {
	case oplog.DropTable:
		return w.DropTable(ctx, wb, tid)
	case oplog.RenameTable:
		if err := w.RenameTable(ctx, wb, tid, ch.Name); err != nil {
			return err
		}
		t.remote.Name = ch.Name
	case oplog.DropColumn:
		i, ok := t.column(ch.Column)
		if !ok {
			return nil // colonne non publiée
		}
		if err := w.DropColumn(ctx, wb, tid, (*cols)[i].ID); err != nil {
			return err
		}
		*cols = slices.Delete(*cols, i, i+1)
	case oplog.RenameColumn:
		i, ok := t.column(ch.Column)
		if !ok {
			return nil
		}
		if err := w.RenameColumn(ctx, wb, tid, (*cols)[i].ID, ch.Name); err != nil {
			return err
		}
		(*cols)[i].Name = ch.Name
	case oplog.RetypeColumn:
		i, ok := t.column(ch.Column)
		typ := workbook.RemoteType(grid.Type(ch.Type))
		if !ok || (*cols)[i].Type == typ {
			return nil
		}
		if err := w.AlterColumnType(ctx, wb, tid, (*cols)[i].ID, typ); err != nil {
			return err
		}
		(*cols)[i].Type = typ
	case oplog.AddColumn:
		if _, ok := t.column(ch.Column); ok {
			return nil
		}
		// Place parmi les colonnes publiées : après la plus proche colonne
		// publiée qui la précède en local, en premier sans elle.
		after, at := "", 0
		for _, p := range ch.Prev {
			if i, ok := t.column(p); ok {
				after, at = (*cols)[i].ID, i+1
				break
			}
		}
		typ := workbook.RemoteType(grid.Type(ch.Type))
		id, err := w.InsertColumn(ctx, wb, tid, ch.Name, typ, at+1, ch.Column)
		if errors.Is(err, workbook.ErrUnsupported) {
			// Serveur sans insert_column (migration 0026) : add_column ne
			// sait pas mettre en premier, la colonne va à la fin.
			if after == "" {
				at = len(*cols)
			}
			id, err = w.AddColumn(ctx, wb, tid, ch.Name, typ, after, ch.Column)
		}
		if err != nil {
			return err
		}
		*cols = slices.Insert(*cols, at, workbook.RemoteColumn{ID: id, Name: ch.Name, LocalID: ch.Column, Type: typ})
	case oplog.SetKey:
		key := ""
		if ch.Key != 0 {
			i, ok := t.column(ch.Key)
			if !ok {
				return nil // clé sur une colonne non publiée : locale
			}
			key = (*cols)[i].ID
		}
		if err := w.SetKey(ctx, wb, tid, key); err != nil {
			return err
		}
		for i := range *cols {
			(*cols)[i].Key = key != "" && (*cols)[i].ID == key
		}
	}
	return nil
}

// saveMapping note le nom et les colonnes d'un tableau sur le serveur.
func saveMapping(ctx context.Context, tx *sql.Tx, tbl int64, name string, cols []workbook.RemoteColumn) error {
	if cols == nil {
		cols = []workbook.RemoteColumn{}
	}
	raw, err := json.Marshal(cols)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE _lattis_remote_tables SET remote_name = ?, columns = ? WHERE tbl = ?`, name, string(raw), tbl)
	return err
}

// forgetTable note qu'un tableau n'est plus sur le serveur : il n'est plus
// publié (son nom reste, pour le message d'une annulation qui le rétablit
// en local : oplog.Recorder.TakeNotice), ses lignes oublient leurs
// identifiants partagés, ses conflits sont sans objet.
func forgetTable(ctx context.Context, tx *sql.Tx, tbl int64) error {
	for _, q := range []string{
		`UPDATE _lattis_remote_tables SET published = 0, remote_id = NULL, columns = NULL, since = NULL WHERE tbl = ?`,
		`DELETE FROM _lattis_row_ids WHERE tbl = ?`,
		`DELETE FROM _lattis_conflicts WHERE tbl = ?`,
	} {
		if _, err := tx.ExecContext(ctx, q, tbl); err != nil {
			return err
		}
	}
	return nil
}

// --- Réception ---------------------------------------------------------------------

// StructureNotice : message de la barre d'état quand la structure d'un
// tableau a changé sur le serveur.
func structureNotice(name string, gone, forgot bool) string {
	msg := "Structure du tableau « " + name + " » modifiée sur le serveur"
	if gone {
		msg = "Tableau « " + name + " » supprimé sur le serveur"
	}
	if forgot {
		msg += " : historique d'annulation effacé"
	}
	return msg
}

// sameShape dit si la structure du serveur st est celle que le poste
// connaît (t).
func sameShape(t *rtable, st workbook.RemoteTable) bool {
	if st.Name != t.remote.Name || len(st.Columns) != len(t.remote.Columns) {
		return false
	}
	for i, c := range st.Columns {
		o := t.remote.Columns[i]
		if c.ID != o.ID || c.Name != o.Name || c.Type != o.Type || (st.Key != "" && c.ID == st.Key) != o.Key {
			return false
		}
	}
	return true
}

// ReceiveStructure relit la structure des tableaux publiés only (nil :
// tous) sur le serveur et l'applique en local hors du journal d'annulation,
// puis lit les changements de données ; les tableaux de reload
// (notification « *structure:<id> ») et ceux dont la structure a changé
// sont relus en entier.
func (e *Engine) ReceiveStructure(ctx context.Context, only, reload map[int64]bool) (Report, error) {
	e.run.Lock()
	defer e.run.Unlock()
	return e.receiveStructure(ctx, only, reload)
}

func (e *Engine) receiveStructure(ctx context.Context, only, reload map[int64]bool) (Report, error) {
	var rep Report
	if only == nil {
		// Catégories d'onglets à choix publiées par un concepteur, puis
		// tableaux ajoutés au classeur par un collègue (« *meta »,
		// connexion).
		r, err := e.receiveChoices(ctx)
		rep.add(r)
		if err != nil {
			return rep, err
		}
		r, err = e.receiveArrivals(ctx)
		rep.add(r)
		if err != nil {
			return rep, err
		}
	}
	r, changed, err := e.syncStructure(ctx, only)
	rep.add(r)
	if err != nil {
		return rep, err
	}
	full := map[int64]bool{}
	for tbl := range reload {
		full[tbl] = true
	}
	for _, tbl := range changed {
		full[tbl] = true
	}
	r, err = e.receive(ctx, only, full)
	rep.add(r)
	return rep, err
}

// syncStructure applique en local la structure du serveur ; rend les
// tableaux changés.
func (e *Engine) syncStructure(ctx context.Context, only map[int64]bool) (Report, []int64, error) {
	var rep Report
	info, srv := e.active()
	if info == nil {
		return rep, nil, nil
	}
	var server []workbook.RemoteTable
	err := srv.Delta(ctx, func(r workbook.Reader) error {
		var err error
		server, err = r.Tables(ctx, info.Schema)
		return err
	})
	if err != nil {
		return rep, nil, err
	}
	byID := map[string]workbook.RemoteTable{}
	for _, t := range server {
		byID[t.ID] = t
	}
	type planned struct {
		shape  canvas.RemoteShape
		server workbook.RemoteTable
		name   string
	}
	var plans []planned
	plan := func(tx *sql.Tx) ([]canvas.RemoteShape, error) {
		plans = nil
		tables, err := remoteTables(ctx, tx)
		if err != nil {
			return nil, err
		}
		waiting, err := pendingStructure(ctx, tx)
		if err != nil {
			return nil, err
		}
		for _, tbl := range sortedKeys(tables) {
			if (only != nil && !only[tbl]) || waiting[tbl] {
				continue // structure changée ici, pas encore envoyée
			}
			t := tables[tbl]
			var local string
			if err := tx.QueryRowContext(ctx, `SELECT name FROM _lattis_tables WHERE id = ?`, tbl).Scan(&local); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}
			st, ok := byID[t.remote.ID]
			if !ok {
				plans = append(plans, planned{shape: canvas.RemoteShape{Table: tbl, Gone: true}, name: local})
				continue
			}
			if sameShape(t, st) {
				continue
			}
			types, err := localTypes(ctx, tx, tbl)
			if err != nil {
				return nil, err
			}
			sh := canvas.RemoteShape{Table: tbl, KeyAt: -1}
			if st.Name != t.remote.Name {
				sh.Name = st.Name
			}
			old := map[string]workbook.RemoteColumn{}
			oldKey := ""
			for _, c := range t.remote.Columns {
				old[c.ID] = c
				if c.LocalID != 0 {
					sh.Published = append(sh.Published, c.LocalID)
				}
				if c.Key {
					oldKey = c.ID
				}
			}
			for i, c := range st.Columns {
				o, known := old[c.ID]
				sc := canvas.RemoteShapeColumn{}
				if known && o.LocalID != 0 {
					sc.ID = o.LocalID
					if c.Name != o.Name {
						sc.Name = c.Name
					}
					if lt := workbook.LocalType(c.Type); c.Type != o.Type && types[o.LocalID] != lt {
						sc.Type = lt
					}
				} else {
					sc.Want, sc.Name, sc.Type = c.LocalID, c.Name, workbook.LocalType(c.Type)
				}
				if c.ID == st.Key {
					sh.KeyAt = i
				}
				sh.Columns = append(sh.Columns, sc)
			}
			sh.KeySet = st.Key != oldKey
			plans = append(plans, planned{shape: sh, server: st, name: local})
		}
		out := make([]canvas.RemoteShape, len(plans))
		for i, p := range plans {
			out[i] = p.shape
		}
		return out, nil
	}
	forgot := false
	after := func(tx *sql.Tx, cols map[int64][]int64) error {
		forgot = false
		for _, p := range plans {
			tbl := p.shape.Table
			if p.shape.Gone {
				if err := forgetTable(ctx, tx, tbl); err != nil {
					return err
				}
			} else {
				mapped := slices.Clone(p.server.Columns)
				ids := cols[tbl]
				for i := range mapped {
					mapped[i].LocalID = 0
					if i < len(ids) {
						mapped[i].LocalID = ids[i]
					}
					mapped[i].Key = p.server.Key != "" && mapped[i].ID == p.server.Key
				}
				if err := saveMapping(ctx, tx, tbl, p.server.Name, mapped); err != nil {
					return err
				}
			}
			// Les étapes d'annulation qui touchent ce tableau ne
			// s'appliqueraient plus à sa nouvelle forme.
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM _lattis_journal WHERE steps GLOB ?`, "*t_"+strconv.FormatInt(tbl, 10)+"[^0-9]*").Scan(&n); err != nil {
				return err
			}
			if n > 0 && !forgot {
				if err := command.Forget(ctx, tx); err != nil {
					return err
				}
				forgot = true
			}
		}
		return nil
	}
	changed, err := e.model.ApplyRemoteShapes(ctx, plan, after)
	if err != nil {
		return rep, nil, err
	}
	for _, p := range plans {
		if p.shape.Gone {
			e.link.Unpublish(p.shape.Table)
		}
		rep.notice(structureNotice(p.name, p.shape.Gone, forgot))
	}
	var reload []int64
	for _, tbl := range changed {
		if !slices.ContainsFunc(plans, func(p planned) bool { return p.shape.Table == tbl && p.shape.Gone }) {
			reload = append(reload, tbl)
		}
		if !slices.Contains(rep.Applied.Tables, tbl) {
			rep.Applied.Tables = append(rep.Applied.Tables, tbl)
		}
	}
	// Correspondance changée sans changement local (colonne d'un autre
	// poste déjà connue…) : relue aussi.
	for _, p := range plans {
		if !p.shape.Gone && !slices.Contains(reload, p.shape.Table) {
			reload = append(reload, p.shape.Table)
		}
	}
	return rep, reload, nil
}

// localTypes lit les types des colonnes d'un tableau local.
func localTypes(ctx context.Context, tx *sql.Tx, tbl int64) (map[int64]grid.Type, error) {
	rs, err := tx.QueryContext(ctx, `SELECT id, type FROM _lattis_columns WHERE tbl = ?`, tbl)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[int64]grid.Type{}
	for rs.Next() {
		var id int64
		var typ string
		if err := rs.Scan(&id, &typ); err != nil {
			return nil, err
		}
		out[id] = grid.Type(typ)
	}
	return out, rs.Err()
}

// --- Index (serveur seulement) ------------------------------------------------------

// indexTable vérifie qu'un index peut être posé ou retiré sur un tableau
// publié (concepteur, en ligne) et rend sa correspondance.
func (e *Engine) indexTable(ctx context.Context, tbl int64) (*workbook.Info, workbook.Server, *rtable, error) {
	info, srv := e.active()
	if info == nil {
		return nil, nil, nil, ErrNotCollaborative
	}
	if !e.link.Online() {
		return nil, nil, nil, workbook.ErrOffline
	}
	if !info.CanDesign() {
		return nil, nil, nil, workbook.ErrDesign
	}
	tables, err := remoteTables(ctx, e.db.Read())
	if err != nil {
		return nil, nil, nil, err
	}
	t := tables[tbl]
	if t == nil {
		return nil, nil, nil, errors.New("ce tableau n'est pas partagé sur le serveur : ses index se posent sur le serveur seulement")
	}
	return info, srv, t, nil
}

// Indexes liste les index posés sur le serveur pour un tableau publié.
func (e *Engine) Indexes(ctx context.Context, tbl int64) ([]workbook.Index, error) {
	info, srv, t, err := e.indexTable(ctx, tbl)
	if err != nil {
		return nil, err
	}
	var out []workbook.Index
	err = srv.Write(ctx, func(w workbook.Writer) error {
		var err error
		out, err = w.Indexes(ctx, info.WorkbookID, t.remote.ID)
		return err
	})
	if err != nil {
		return nil, serverError(command.KindDo, err)
	}
	return out, nil
}

// CreateIndex pose sur le serveur un index sur des colonnes (locales)
// d'un tableau publié ; rend son nom. Un index ne change ni les données
// ni la structure vue des autres : il ne s'annule pas par Ctrl+Z.
func (e *Engine) CreateIndex(ctx context.Context, tbl int64, cols []int64, unique bool) (string, error) {
	info, srv, t, err := e.indexTable(ctx, tbl)
	if err != nil {
		return "", err
	}
	var ids []string
	for _, c := range cols {
		rc, ok := t.byLocal[c]
		if !ok {
			return "", errors.New("colonne non partagée : un index ne porte que sur les colonnes publiées")
		}
		ids = append(ids, rc.ID)
	}
	var name string
	err = srv.Write(ctx, func(w workbook.Writer) error {
		var err error
		name, err = w.CreateIndex(ctx, info.WorkbookID, t.remote.ID, ids, unique)
		return err
	})
	if err != nil {
		return "", serverError(command.KindDo, err)
	}
	return name, nil
}

// DropIndex retire un index posé sur le serveur.
func (e *Engine) DropIndex(ctx context.Context, tbl int64, name string) error {
	info, srv, t, err := e.indexTable(ctx, tbl)
	if err != nil {
		return err
	}
	err = srv.Write(ctx, func(w workbook.Writer) error {
		return w.DropIndex(ctx, info.WorkbookID, t.remote.ID, name)
	})
	if err != nil {
		return serverError(command.KindDo, err)
	}
	return nil
}
