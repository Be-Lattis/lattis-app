package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/order"
)

// Classeur collaboratif (docs/collaboration/synchronisation.md) : les
// changements reçus du serveur s'appliquent ici, hors du journal
// d'annulation (les modifications des autres ne s'annulent pas par
// Ctrl+Z), et les types des tableaux publiés sont figés (§9).

// RemoteRow est une ligne d'un tableau publié reçue du serveur.
type RemoteRow struct {
	ID      string // lattis_id
	Deleted bool
	Pos     string // lattis_pos ("" : inchangée)
	Author  string // lattis_author
	// Values : valeurs de stockage locales par colonne locale ; une colonne
	// absente garde sa valeur.
	Values map[int64]any
	// NoCreate : une ligne inconnue ici (supprimée en local) n'est pas
	// créée ; seules les lignes existantes sont mises à jour (modification
	// d'une cellule reçue d'un autre poste, internal/coedit).
	NoCreate bool
}

// RemoteTable regroupe les lignes reçues pour un tableau publié.
type RemoteTable struct {
	Table int64
	Rows  []RemoteRow
	// Full : Rows est tout le tableau (rechargé en entier) ; une ligne
	// locale absente est supprimée, sauf si une modification locale l'attend
	// (Keep).
	Full bool
}

// Keep donne, par tableau et lattis_id, les cellules à ne pas remplacer
// (colonne locale ; 0 : la position de la ligne ; -1 : toute la ligne,
// supprimée en local et pas encore envoyée) : modifications locales pas
// encore confirmées par le serveur, conflits ouverts, cellule en cours de
// saisie.
type Keep map[int64]map[string]map[int64]bool

// Has dit si la cellule (tableau, ligne, colonne) est à garder.
func (k Keep) Has(tbl int64, id string, col int64) bool { return k[tbl][id][col] }

// Add note une cellule à garder.
func (k Keep) Add(tbl int64, id string, col int64) {
	if k[tbl] == nil {
		k[tbl] = map[string]map[int64]bool{}
	}
	if k[tbl][id] == nil {
		k[tbl][id] = map[int64]bool{}
	}
	k[tbl][id][col] = true
}

// RemoteApplied décrit ce qu'ApplyRemote a fait.
type RemoteApplied struct {
	Inserted, Updated, Deleted int
	// Kept : cellules gardées telles quelles (voir Keep).
	Kept int
	// Failed : lignes refusées par le classeur local (clé en double…).
	Failed []string
	// Tables : tableaux modifiés.
	Tables []int64
}

// ApplyRemote applique des lignes reçues du serveur dans les tableaux
// publiés : t_<id>, _pos, _lattis_row_ids (auteur compris), saisies
// invalides et mises en forme des lignes supprimées. Hors du journal
// d'annulation et du relevé des opérations (rien n'est renvoyé). Une ligne
// peut revenir plusieurs fois : l'application est idempotente.
//
// keep, appelé dans la transaction, donne les cellules à ne pas remplacer ;
// after, s'il est fixé, termine la transaction (point de reprise noté avec
// les données).
func (m *Model) ApplyRemote(ctx context.Context, tables []RemoteTable, keep func(*sql.Tx) (Keep, error), after func(*sql.Tx) error) (RemoteApplied, error) {
	var out RemoteApplied
	err := m.db.Write(ctx, func(sqlTx *sql.Tx) error {
		out = RemoteApplied{}
		var k Keep
		if keep != nil {
			var err error
			if k, err = keep(sqlTx); err != nil {
				return err
			}
		}
		t := &tx{Tx: &command.Tx{Tx: sqlTx, Ctx: ctx}, m: m}
		for _, rt := range tables {
			changed, err := t.applyRemote(publishedIDs, rt, k, &out)
			if err != nil {
				return err
			}
			if changed {
				out.Tables = append(out.Tables, rt.Table)
			}
		}
		if after != nil {
			return after(sqlTx)
		}
		return nil
	})
	return out, err
}

// rowIDs désigne la table des identifiants partagés des lignes : celle des
// tableaux publiés (_lattis_row_ids, avec l'auteur), ou celle des tableaux
// liés à une donnée du Data Lake (_lattis_data_rows, voir dataset.go).
type rowIDs struct {
	table  string
	author bool
}

var publishedIDs = rowIDs{table: "_lattis_row_ids", author: true}

func (t *tx) applyRemote(idt rowIDs, rt RemoteTable, keep Keep, out *RemoteApplied) (bool, error) {
	tb, err := t.table(rt.Table)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil // tableau supprimé en local entre-temps
	}
	if err != nil {
		return false, err
	}
	cols := map[int64]Column{}
	for _, c := range tb.Cols {
		if c.Formula == "" {
			cols[c.ID] = c
		}
	}
	before := tb.Rows
	var next int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM ` + tb.sql()).Scan(&next); err != nil {
		return false, err
	}
	rows := rt.Rows
	if rt.Full {
		gone, err := t.missingRows(idt, tb.ID, rt.Rows, keep)
		if err != nil {
			return false, err
		}
		rows = append(slices.Clip(rows), gone...)
	}
	changed := false
	for _, r := range rows {
		if keep.Has(tb.ID, r.ID, -1) {
			out.Kept++
			continue
		}
		var rid int64
		err := t.row(`SELECT rid FROM `+idt.table+` WHERE tbl = ? AND lattis_id = ?`, tb.ID, r.ID).Scan(&rid)
		known := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return changed, err
		}
		if known {
			// La ligne a pu disparaître du tableau sans que son identifiant
			// soit oublié (relevé pas encore passé) : elle est recréée.
			var n int
			if err := t.row(`SELECT count(*) FROM `+tb.sql()+` WHERE id = ?`, rid).Scan(&n); err != nil {
				return changed, err
			}
			if n == 0 {
				if _, err := t.ExecContext(t.Ctx, `DELETE FROM `+idt.table+` WHERE tbl = ? AND rid = ?`, tb.ID, rid); err != nil {
					return changed, err
				}
				known = false
			}
		}
		switch {
		case r.Deleted:
			if !known {
				continue
			}
			for _, q := range []string{
				`DELETE FROM ` + tb.sql() + ` WHERE id = ?`,
				`DELETE FROM _lattis_invalid WHERE tbl = ` + itoa64(tb.ID) + ` AND rid = ?`,
				`DELETE FROM _lattis_styles WHERE tbl = ` + itoa64(tb.ID) + ` AND rid = ?`,
				`DELETE FROM ` + idt.table + ` WHERE tbl = ` + itoa64(tb.ID) + ` AND rid = ?`,
			} {
				if _, err := t.ExecContext(t.Ctx, q, rid); err != nil {
					return changed, err
				}
			}
			out.Deleted++
			changed = true
		case known:
			sets, args, kept := remoteSets(tb, cols, r, keep)
			out.Kept += kept
			var ids []int64
			for _, c := range sortedCols(r.Values) {
				if _, ok := cols[c]; ok && !keep.Has(tb.ID, r.ID, c) {
					ids = append(ids, c)
				}
			}
			if r.Pos != "" {
				if keep.Has(tb.ID, r.ID, 0) {
					out.Kept++
				} else {
					sets = append(sets, "_pos = ?")
					args = append(args, r.Pos)
				}
			}
			if len(sets) > 0 {
				// Ne réécrire que si quelque chose change (une ligne revient
				// souvent telle quelle : sa propre modification, renvoyée).
				where := make([]string, 0, len(sets))
				for _, s := range sets {
					where = append(where, strings.Replace(s, " = ?", " IS NOT ?", 1))
				}
				q := `UPDATE ` + tb.sql() + ` SET ` + strings.Join(sets, ", ") + ` WHERE id = ? AND (` + strings.Join(where, " OR ") + `)`
				all := append(append(append([]any{}, args...), rid), args...)
				res, err := t.ExecContext(t.Ctx, q, all...)
				if err != nil {
					out.Failed = append(out.Failed, fmt.Sprintf("%s : %v", tb.Name, err))
					continue
				}
				if n, _ := res.RowsAffected(); n > 0 {
					out.Updated++
					changed = true
				}
			}
			for _, c := range ids {
				res, err := t.ExecContext(t.Ctx, `DELETE FROM _lattis_invalid WHERE tbl = ? AND rid = ? AND col = ?`, tb.ID, rid, c)
				if err != nil {
					return changed, err
				}
				if n, _ := res.RowsAffected(); n > 0 {
					changed = true
				}
			}
			if idt.author {
				if _, err := t.ExecContext(t.Ctx, `UPDATE _lattis_row_ids SET lattis_author = ? WHERE tbl = ? AND rid = ? AND lattis_author IS NOT ?`, remoteAuthor(r.Author), tb.ID, rid, remoteAuthor(r.Author)); err != nil {
					return changed, err
				}
			}
		case r.NoCreate:
			continue
		default:
			names := []string{"id", "_pos"}
			args := []any{next, r.Pos}
			for _, c := range sortedCols(r.Values) {
				if _, ok := cols[c]; ok {
					names = append(names, colSQL(c))
					args = append(args, r.Values[c])
				}
			}
			if _, err := t.ExecContext(t.Ctx, `INSERT INTO `+tb.sql()+` (`+strings.Join(names, ", ")+`) VALUES (`+marks(len(names))+`)`, args...); err != nil {
				out.Failed = append(out.Failed, fmt.Sprintf("%s : %v", tb.Name, err))
				continue
			}
			ins, args := `INSERT INTO `+idt.table+` (tbl, rid, lattis_id) VALUES (?, ?, ?)`, []any{tb.ID, next, r.ID}
			if idt.author {
				ins, args = `INSERT INTO _lattis_row_ids (tbl, rid, lattis_id, lattis_author) VALUES (?, ?, ?, ?)`, append(args, remoteAuthor(r.Author))
			}
			if _, err := t.ExecContext(t.Ctx, ins, args...); err != nil {
				return changed, err
			}
			next++
			out.Inserted++
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	// Tableau agrandi : ce qui est dessous descend, comme pour une saisie
	// sous le tableau.
	var after int
	if err := t.row(`SELECT count(*) FROM ` + tb.sql()).Scan(&after); err != nil {
		return true, err
	}
	if after > before {
		if err := t.ensureGapBelow(tb.Sheet, tb.ID, tb.Bottom(), tb.Row+after, tb.Col, tb.Right()); err != nil {
			return true, err
		}
	}
	return true, nil
}

// missingRows rend, pour un tableau rechargé en entier, les lignes locales
// absentes du serveur (à supprimer), hors de celles qu'une modification
// locale attend (Keep : ajoutée et pas encore envoyée, modifiée, en
// conflit).
func (t *tx) missingRows(idt rowIDs, tbl int64, rows []RemoteRow, keep Keep) ([]RemoteRow, error) {
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		seen[r.ID] = true
	}
	rs, err := t.query(`SELECT lattis_id FROM `+idt.table+` WHERE tbl = ? ORDER BY rid`, tbl)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []RemoteRow
	for rs.Next() {
		var id string
		if err := rs.Scan(&id); err != nil {
			return nil, err
		}
		if !seen[id] && len(keep[tbl][id]) == 0 {
			out = append(out, RemoteRow{ID: id, Deleted: true})
		}
	}
	return out, rs.Err()
}

// remoteSets prépare la mise à jour des valeurs d'une ligne reçue ; rend
// aussi le nombre de cellules gardées.
func remoteSets(tb Table, cols map[int64]Column, r RemoteRow, keep Keep) ([]string, []any, int) {
	var sets []string
	var args []any
	kept := 0
	for _, c := range sortedCols(r.Values) {
		if _, ok := cols[c]; !ok {
			continue
		}
		if keep.Has(tb.ID, r.ID, c) {
			kept++
			continue
		}
		sets = append(sets, colSQL(c)+" = ?")
		args = append(args, r.Values[c])
	}
	return sets, args, kept
}

func sortedCols(v map[int64]any) []int64 {
	out := make([]int64, 0, len(v))
	for c := range v {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func remoteAuthor(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func itoa64(n int64) string { return fmt.Sprint(n) }

// --- Types figés (§9) ---------------------------------------------------------

// ErrOutOfType refuse une saisie hors du type figé d'une colonne d'un
// tableau publié (classeur partagé).
type ErrOutOfType struct {
	Table, Column string
	Type          grid.Type
	Raw           string
}

func (e ErrOutOfType) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e ErrOutOfType) Msg() msg.Message {
	return msg.M("workbook.outOfType", msg.Args{"value": e.Raw, "type": typeArticle(e.Type), "column": e.Column, "table": e.Table})
}

// typeArticle : « un nombre entier », « une date »… (message).
func typeArticle(t grid.Type) msg.Message {
	switch t {
	case grid.Integer:
		return msg.M("type.anInteger", nil)
	case grid.Decimal:
		return msg.M("type.aNumber", nil)
	case grid.Boolean:
		return msg.M("type.aBoolean", nil)
	case grid.Date:
		return msg.M("type.aDate", nil)
	}
	return msg.M("type.aText", nil)
}

// rowKey donne la clé d'ordre (_pos) d'une nouvelle ligne entre a et b.
// Dans un tableau publié d'un classeur partagé, la clé reçoit un suffixe
// aléatoire (order.Distinct) : deux postes qui ajoutent une ligne au même
// endroit en même temps n'ont pas la même clé, et l'ordre des lignes est le
// même sur tous les postes (le serveur départage les clés égales par
// lattis_id, le classeur local ne le peut pas).
func (t *tx) rowKey(tb Table, a, b string) (string, error) {
	if t.frozenTypes(tb.ID) {
		return order.Distinct(a, b, order.Suffix())
	}
	return order.Between(a, b)
}

// frozenTypes : le tableau est publié dans un classeur partagé, ses types
// sont figés et une valeur hors type est refusée (§9).
func (t *tx) frozenTypes(tbl int64) bool {
	return t.m != nil && t.m.sharing != nil && t.m.sharing.TableShare(tbl) == "published"
}

// refuse traite une saisie hors du type figé d'un tableau publié : un
// collage la note (Event.Refused) et continue, une saisie est refusée.
func (t *tx) refuse(tb Table, c Column, raw string, ev *Event) error {
	e := ErrOutOfType{Table: tb.Name, Column: c.Name, Type: c.Type, Raw: raw}
	if t.collect && ev != nil {
		if len(ev.Refused) < maxRefused {
			ev.Refused = append(ev.Refused, t.say("workbook.refusedValue", msg.Args{"column": c.Name, "value": raw}))
		}
		ev.RefusedCount++
		return nil
	}
	return e
}

// maxRefused : valeurs refusées citées dans le message d'un collage.
const maxRefused = 20

// Notify retient un message pour la barre d'état (voir TakeNotice).
func (m *Model) Notify(msg string) {
	if msg != "" {
		m.note([]string{msg})
	}
}

// TableCell situe une cellule de feuille dans les données d'un tableau :
// tableau, clé de la ligne et colonne ; ok faux hors des données d'un
// tableau.
func (m *Model) TableCell(ctx context.Context, sheet int64, row, col int) (tbl, rid, colID int64, ok bool, err error) {
	err = m.read(ctx, func(t *tx) error {
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			if row <= tb.Row || row > tb.Bottom() || col < tb.Col || col > tb.Right() {
				continue
			}
			id, _, err := t.rowID(tb, row-tb.Row-1)
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return nil // ligne masquée par un filtre : rien à suivre
			}
			tbl, rid, colID, ok = tb.ID, id, tb.Cols[col-tb.Col].ID, true
			return nil
		}
		return nil
	})
	return
}

// CellOf situe sur sa feuille la cellule (ligne rid, colonne colID) d'un
// tableau : feuille, ligne et colonne de la feuille ; ok faux si elle
// n'existe plus ou si un filtre la masque.
func (m *Model) CellOf(ctx context.Context, tbl, rid, colID int64) (sheet int64, row, col int, ok bool, err error) {
	err = m.read(ctx, func(t *tx) error {
		tb, err := t.table(tbl)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		i := -1
		for k, c := range tb.Cols {
			if c.ID == colID {
				i = k
			}
		}
		if i < 0 {
			return nil
		}
		var pos string
		if err := t.row(`SELECT _pos FROM `+tb.view()+` WHERE id = ?`, rid).Scan(&pos); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		var n int
		if err := t.row(`SELECT count(*) FROM `+tb.view()+` WHERE _pos < ?`, pos).Scan(&n); err != nil {
			return err
		}
		sheet, row, col, ok = tb.Sheet, tb.Row+1+n, tb.Col+i, true
		return nil
	})
	return
}
