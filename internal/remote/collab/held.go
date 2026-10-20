package collab

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Étapes mises de côté. Une étape que le serveur reçoit mais qui échoue à
// chaque essai (erreur SQL qui n'est pas un refus, structure que même la
// réception ne remet pas d'accord) ne doit pas bloquer la file : la
// réception ne vient qu'après l'envoi, le poste ne recevrait plus rien des
// autres. Au bout de HeldAfter échecs de suite, elle est mise de côté avec
// les étapes suivantes des mêmes tableaux (elles la supposent) : entrées
// « pending » qui ont une erreur (heldWhere ; le format du classeur ne
// change pas), que la file n'envoie plus ; leurs cellules gardent la
// valeur du poste (pendingCells) et le cache les garde (Unsent).
// Le volet Conflits la montre : Réessayer (Retry) ou Abandonner (Drop :
// les tableaux reprennent l'état du serveur).

// heldWhere : entrées mises de côté (en attente, avec une erreur).
const heldWhere = `state = 'pending' AND error IS NOT NULL`

// HeldAfter : échecs de suite d'une même étape, serveur joignable, avant
// de la mettre de côté.
var HeldAfter = 5

// stuckStep compte les échecs de la première étape de la file.
type stuckStep struct {
	batch int64
	n     int
}

// Held est un tableau dont des modifications sont mises de côté.
type Held struct {
	// Table : identifiant local du tableau, Name : son nom.
	Table int64  `json:"table"`
	Name  string `json:"name"`
	// Steps : étapes mises de côté ; Error : erreur de la première.
	Steps int    `json:"steps"`
	Error string `json:"error"`
}

// answered dit si err vient d'un serveur qui répond (erreur SQL, structure
// périmée) plutôt que d'une coupure : seules celles-là comptent.
func answered(err error) bool {
	if errors.Is(err, errStale) {
		return true
	}
	_, ok := workbook.Rejected(err)
	return ok
}

// stepTables : tableaux touchés par une étape.
func stepTables(entries []entry) []int64 {
	var out []int64
	for _, en := range entries {
		for _, op := range en.ops {
			if op.Table != 0 && !slices.Contains(out, op.Table) {
				out = append(out, op.Table)
			}
		}
	}
	return out
}

// hold compte un échec de l'étape entries ; au HeldAfter-ième de suite
// (serveur joignable), elle est mise de côté et hold rend vrai : la file
// continue avec l'étape suivante.
func (e *Engine) hold(ctx context.Context, entries []entry, cause error, rep *Report) (bool, error) {
	if len(entries) == 0 || !answered(cause) {
		return false, nil
	}
	first := entries[0].id
	if e.stuck.batch != first {
		e.stuck = stuckStep{batch: first}
	}
	e.stuck.n++
	if e.stuck.n < HeldAfter {
		return false, nil
	}
	e.stuck = stuckStep{}
	if err := e.setHeld(ctx, entries, errText(cause)); err != nil {
		return false, err
	}
	rep.notice(e.say("collab.held", msg.Args{"err": errText(cause)}))
	return true, nil
}

// heldTable met de côté, sans l'envoyer, une étape qui touche un tableau
// dont des modifications sont déjà mises de côté : elle les suppose
// (ligne ajoutée, colonne changée), l'ordre de la file est gardé.
func (e *Engine) heldTable(ctx context.Context, entries []entry, rep *Report) (bool, error) {
	tables := stepTables(entries)
	if len(tables) == 0 {
		return false, nil
	}
	held, err := e.heldTables(ctx)
	if err != nil {
		return false, err
	}
	for _, t := range tables {
		if msg, ok := held[t]; ok {
			return true, e.setHeld(ctx, entries, msg)
		}
	}
	return false, nil
}

// setHeld met les entrées d'une étape de côté.
func (e *Engine) setHeld(ctx context.Context, entries []entry, msg string) error {
	return e.db.Write(ctx, func(tx *sql.Tx) error {
		for _, en := range entries {
			if _, err := tx.ExecContext(ctx, `UPDATE _lattis_outbox SET error = ? WHERE id = ?`, msg, en.id); err != nil {
				return err
			}
		}
		return nil
	})
}

// heldTables : tableau → erreur de sa première étape mise de côté.
func (e *Engine) heldTables(ctx context.Context) (map[int64]string, error) {
	rs, err := e.db.Read().QueryContext(ctx, `SELECT ops, error FROM _lattis_outbox WHERE `+heldWhere+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[int64]string{}
	for rs.Next() {
		var data, msg string
		if err := rs.Scan(&data, &msg); err != nil {
			return nil, err
		}
		ops, err := command.DecodeOps(data)
		if err != nil {
			return nil, err
		}
		for _, op := range ops {
			if _, ok := out[op.Table]; !ok && op.Table != 0 {
				out[op.Table] = msg
			}
		}
	}
	return out, rs.Err()
}

// HeldList liste les tableaux dont des modifications sont mises de côté.
func (e *Engine) HeldList(ctx context.Context) ([]Held, error) {
	rs, err := e.db.Read().QueryContext(ctx, `SELECT batch, ops, error FROM _lattis_outbox WHERE `+heldWhere+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var out []Held
	steps := map[int64]map[int64]bool{} // tableau → étapes
	for rs.Next() {
		var batch int64
		var data, msg string
		if err := rs.Scan(&batch, &data, &msg); err != nil {
			rs.Close()
			return nil, err
		}
		ops, err := command.DecodeOps(data)
		if err != nil {
			rs.Close()
			return nil, err
		}
		for _, op := range ops {
			if op.Table == 0 {
				continue
			}
			i := slices.IndexFunc(out, func(h Held) bool { return h.Table == op.Table })
			if i < 0 {
				out = append(out, Held{Table: op.Table, Error: msg})
				steps[op.Table] = map[int64]bool{}
				i = len(out) - 1
			}
			steps[op.Table][batch] = true
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Steps = len(steps[out[i].Table])
		_ = e.db.Read().QueryRowContext(ctx, `SELECT name FROM _lattis_tables WHERE id = ?`, out[i].Table).Scan(&out[i].Name)
	}
	return out, nil
}

// heldEntries : entrées mises de côté qui touchent le tableau tbl.
func (e *Engine) heldEntries(ctx context.Context, tx *sql.Tx, tbl int64) ([]int64, error) {
	rs, err := tx.QueryContext(ctx, `SELECT id, ops FROM _lattis_outbox WHERE `+heldWhere+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var ids []int64
	for rs.Next() {
		var id int64
		var data string
		if err := rs.Scan(&id, &data); err != nil {
			return nil, err
		}
		ops, err := command.DecodeOps(data)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(ops, func(op command.Op) bool { return op.Table == tbl }) {
			ids = append(ids, id)
		}
	}
	return ids, rs.Err()
}

// RetryHeld remet en file les modifications mises de côté du tableau tbl
// (Réessayer) ; elles repartent au prochain envoi.
func (e *Engine) RetryHeld(ctx context.Context, tbl int64) error {
	e.run.Lock()
	defer e.run.Unlock()
	e.stuck = stuckStep{}
	return e.db.Write(ctx, func(tx *sql.Tx) error {
		ids, err := e.heldEntries(ctx, tx, tbl)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := tx.ExecContext(ctx, `UPDATE _lattis_outbox SET state = 'pending', error = NULL WHERE id = ?`, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// DropHeld abandonne les modifications mises de côté du tableau tbl
// (Abandonner, en ligne) : elles sont notées refusées et le tableau
// reprend l'état du serveur (relu en entier ; les autres modifications en
// attente sont gardées).
func (e *Engine) DropHeld(ctx context.Context, tbl int64) (Report, error) {
	e.run.Lock()
	defer e.run.Unlock()
	if info, _ := e.active(); info == nil || !e.link.Online() {
		return Report{}, msg.New("collab.discardOffline", nil)
	}
	err := e.db.Write(ctx, func(tx *sql.Tx) error {
		ids, err := e.heldEntries(ctx, tx, tbl)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := tx.ExecContext(ctx, `UPDATE _lattis_outbox SET state = 'rejected', error = error || ' (abandonnée)' WHERE id = ?`, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}
	return e.receiveStructure(ctx, map[int64]bool{tbl: true}, map[int64]bool{tbl: true})
}
