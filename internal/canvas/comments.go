package canvas

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
)

// Commentaires en fil (Révision › Nouveau commentaire, Ctrl+Alt+M), à côté
// des notes (nt, cellformat.go) qui restent ce qu'elles sont : un fil par
// cellule, le premier message et ses réponses, chacun avec son auteur et sa
// date ; un fil se résout et se rouvre.
//
// Ancrage :
//
//	cellule libre        _lattis_comment_cells (sheet, row, col) : suit
//	                     l'insertion et la suppression de lignes et de
//	                     colonnes (sheetrows.go), le déplacement des cellules
//	                     (cellmove.go), part avec la feuille
//	cellule de tableau   _lattis_comment_rows (tbl, rid, col = colonne du
//	                     tableau ; rid -1 : en-tête) : suit sa ligne (tri,
//	                     filtre), part avec elle
//
// Les messages sont dans _lattis_comments (thread : identifiant du fil, celui
// de son premier message). Un fil dont l'ancre a disparu (ligne, colonne ou
// tableau supprimés) n'est plus montré ; l'annulation le rend.
//
// Copier une cellule ne copie pas son fil (une discussion tient à sa
// place) ; la couper et la coller ailleurs (déplacement) l'emmène.
//
// Classeur partagé : les trois tables font partie de la mise en page en
// direct (remote/workbook/layout.go) ; seuls propriétaires et concepteurs la
// modifient, donc commentent (ErrCommentsShared pour les autres).

// Who désigne l'auteur d'un commentaire : le nom affiché, et un identifiant
// stable (compte du Serveur Lattis, ou utilisateur du poste) qui dit quels
// messages il peut modifier.
type Who struct {
	Name string
	ID   string
}

// Comment est un message d'un fil.
type Comment struct {
	ID       string `json:"id"`
	Author   string `json:"author"`
	AuthorID string `json:"authorId"`
	// At, Edited : dates en UTC (RFC 3339) ; Edited vide : jamais modifié.
	At     string `json:"at"`
	Edited string `json:"edited,omitempty"`
	Text   string `json:"text"`
}

// Thread est un fil de commentaires, à sa place actuelle sur la feuille.
type Thread struct {
	ID        string    `json:"id"`
	Sheet     int64     `json:"sheet"`
	SheetName string    `json:"sheetName"`
	Row       int       `json:"row"`
	Col       int       `json:"col"`
	Table     string    `json:"table,omitempty"`
	Resolved  bool      `json:"resolved"`
	Comments  []Comment `json:"comments"`
}

// MaxCommentText borne la longueur d'un message.
const MaxCommentText = 10_000

var (
	// ErrCommentsShared : commentaire d'un classeur partagé par qui n'en
	// est ni propriétaire ni concepteur (les commentaires suivent la mise
	// en page, que le serveur leur réserve).
	ErrCommentsShared  = msg.New("comment.shared", nil)
	errCommentEmpty    = msg.New("comment.empty", nil)
	errCommentTooLong  = msg.New("comment.tooLong", msg.Args{"max": MaxCommentText})
	errCommentNotFound = msg.New("comment.notFound", nil)
	errCommentNotYours = msg.New("comment.notYours", nil)
)

// commentsLock refuse de commenter un classeur partagé à qui n'en est ni
// propriétaire ni concepteur (même règle que la mise en page).
func (t *tx) commentsLock() error {
	if t.layoutLock() != nil {
		return ErrCommentsShared
	}
	return nil
}

// newCommentID : identifiant aléatoire (16 octets en hexadécimal).
func newCommentID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// commentNow : date d'un message (UTC, à la seconde).
var commentNow = func() string { return time.Now().UTC().Format(time.RFC3339) }

func cleanComment(text string) (string, error) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	switch {
	case text == "":
		return "", errCommentEmpty
	case len([]rune(text)) > MaxCommentText:
		return "", errCommentTooLong
	}
	return text, nil
}

// anchor : place d'un fil (cellule libre, ou cellule de tableau par sa ligne).
type anchor struct {
	table           bool
	sheet           int64
	row, col        int
	tbl, rid, colID int64
}

func (a anchor) where() (string, string, []any) {
	if a.table {
		return `_lattis_comment_rows`, `tbl = ? AND rid = ? AND col = ?`, []any{a.tbl, a.rid, a.colID}
	}
	return `_lattis_comment_cells`, `sheet = ? AND row = ? AND col = ?`, []any{a.sheet, a.row, a.col}
}

// commentAnchor : ancre de la cellule (row, col) de la feuille ; dans un
// tableau, la ligne affichée à cette place (comme une note, cellPatch).
func (t *tx) commentAnchor(sheet int64, row, col int) (anchor, error) {
	tables, err := t.tables(sheet)
	if err != nil {
		return anchor{}, err
	}
	for i := range tables {
		tb := &tables[i]
		if row < tb.Row || row > tb.Bottom() || col < tb.Col || col > tb.Right() {
			continue
		}
		a := anchor{table: true, tbl: tb.ID, rid: -1, colID: tb.Cols[col-tb.Col].ID}
		if row > tb.Row {
			ids, err := t.ridsAt(tb, row, row)
			if err != nil {
				return anchor{}, err
			}
			if len(ids) == 0 {
				return anchor{}, msg.Wrap(ErrTable, "table.rowHiddenByFilter", msg.Args{"table": tb.Name})
			}
			a.rid = ids[0]
		}
		return a, nil
	}
	return anchor{sheet: sheet, row: row, col: col}, nil
}

// threadAt : fil ancré à a (ok faux : aucun).
func (t *tx) threadAt(a anchor) (id string, resolved, ok bool, err error) {
	table, where, args := a.where()
	err = t.row(`SELECT thread, resolved <> 0 FROM `+table+` WHERE `+where, args...).Scan(&id, &resolved)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, false, nil
	}
	return id, resolved, err == nil, err
}

// AddComment ajoute un message au fil de la cellule (row, col), ou ouvre
// le fil s'il n'y en a pas ; rend l'identifiant du message. Une étape
// d'annulation.
func (m *Model) AddComment(ctx context.Context, sheet int64, row, col int, text string, who Who) (string, error) {
	text, err := cleanComment(text)
	if err != nil {
		return "", err
	}
	id := newCommentID()
	at := commentNow()
	label := "Nouveau commentaire"
	err = m.Do(ctx, label, func(t *tx) error {
		if err := t.commentsLock(); err != nil {
			return err
		}
		a, err := t.commentAnchor(sheet, row, col)
		if err != nil {
			return err
		}
		thread, _, ok, err := t.threadAt(a)
		if err != nil {
			return err
		}
		if !ok {
			thread = id
			table, where, args := a.where()
			var ins stmt
			if a.table {
				ins = st(`INSERT INTO _lattis_comment_rows (tbl, rid, col, thread) VALUES (?, ?, ?, ?)`, a.tbl, a.rid, a.colID, thread)
			} else {
				ins = st(`INSERT INTO _lattis_comment_cells (sheet, row, col, thread) VALUES (?, ?, ?, ?)`, a.sheet, a.row, a.col, thread)
			}
			if err := t.apply([]stmt{ins}, []stmt{st(`DELETE FROM `+table+` WHERE `+where, args...)}); err != nil {
				return err
			}
		}
		return t.apply(
			[]stmt{st(`INSERT INTO _lattis_comments (id, thread, author, author_id, at, text) VALUES (?, ?, ?, ?, ?, ?)`, id, thread, who.Name, who.ID, at, text)},
			[]stmt{st(`DELETE FROM _lattis_comments WHERE id = ?`, id)},
		)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// comment lit un message (errCommentNotFound s'il n'existe plus).
func (t *tx) comment(id string) (Comment, string, error) {
	var c Comment
	var thread string
	var edited sql.NullString
	err := t.row(`SELECT id, thread, author, author_id, at, edited, text FROM _lattis_comments WHERE id = ?`, id).Scan(&c.ID, &thread, &c.Author, &c.AuthorID, &c.At, &edited, &c.Text)
	if errors.Is(err, sql.ErrNoRows) {
		return c, "", errCommentNotFound
	}
	c.Edited = edited.String
	return c, thread, err
}

// mine dit si le message est de who (un message sans identifiant d'auteur
// est à tous).
func mine(c Comment, who Who) bool {
	return c.AuthorID == "" || who.ID == "" || c.AuthorID == who.ID
}

// EditComment remplace le texte de son propre message.
func (m *Model) EditComment(ctx context.Context, id, text string, who Who) error {
	text, err := cleanComment(text)
	if err != nil {
		return err
	}
	return m.Do(ctx, "Modifier le commentaire", func(t *tx) error {
		if err := t.commentsLock(); err != nil {
			return err
		}
		c, _, err := t.comment(id)
		if err != nil {
			return err
		}
		if !mine(c, who) {
			return errCommentNotYours
		}
		if c.Text == text {
			return nil
		}
		var edited any
		if c.Edited != "" {
			edited = c.Edited
		}
		q := `UPDATE _lattis_comments SET text = ?, edited = ? WHERE id = ?`
		return t.apply([]stmt{st(q, text, commentNow(), id)}, []stmt{st(q, c.Text, edited, id)})
	})
}

// DeleteComment supprime son propre message ; le premier message d'un fil
// emporte le fil entier (réponses comprises).
func (m *Model) DeleteComment(ctx context.Context, id string, who Who) error {
	return m.Do(ctx, "Supprimer le commentaire", func(t *tx) error {
		if err := t.commentsLock(); err != nil {
			return err
		}
		c, thread, err := t.comment(id)
		if err != nil {
			return err
		}
		if !mine(c, who) {
			return errCommentNotYours
		}
		if thread == id {
			return t.dropThread(thread)
		}
		return t.deleteWhere(`_lattis_comments`, `id = ?`, id)
	})
}

// dropThread supprime un fil : ses messages et son ancre (rendus par
// l'annulation).
func (t *tx) dropThread(thread string) error {
	for _, table := range []string{`_lattis_comments`, `_lattis_comment_cells`, `_lattis_comment_rows`} {
		if err := t.deleteWhere(table, `thread = ?`, thread); err != nil {
			return err
		}
	}
	return nil
}

// DeleteThread supprime le fil de la cellule (row, col) (Révision ›
// Supprimer le commentaire).
func (m *Model) DeleteThread(ctx context.Context, sheet int64, row, col int) error {
	return m.Do(ctx, "Supprimer le fil", func(t *tx) error {
		if err := t.commentsLock(); err != nil {
			return err
		}
		a, err := t.commentAnchor(sheet, row, col)
		if err != nil {
			return err
		}
		thread, _, ok, err := t.threadAt(a)
		if err != nil {
			return err
		}
		if !ok {
			return errCommentNotFound
		}
		return t.dropThread(thread)
	})
}

// ResolveThread résout (ou rouvre) un fil.
func (m *Model) ResolveThread(ctx context.Context, thread string, resolved bool) error {
	label := "Rouvrir le fil"
	if resolved {
		label = "Résoudre le fil"
	}
	return m.Do(ctx, label, func(t *tx) error {
		if err := t.commentsLock(); err != nil {
			return err
		}
		v := 0
		if resolved {
			v = 1
		}
		for _, table := range []string{`_lattis_comment_cells`, `_lattis_comment_rows`} {
			var old int
			err := t.row(`SELECT resolved FROM `+table+` WHERE thread = ?`, thread).Scan(&old)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if (old != 0) == resolved {
				return nil
			}
			q := `UPDATE ` + table + ` SET resolved = ? WHERE thread = ?`
			return t.apply([]stmt{st(q, v, thread)}, []stmt{st(q, old, thread)})
		}
		return errCommentNotFound
	})
}

// Threads rend les fils d'une feuille (sheet 0 : de tout le classeur), à
// leur place actuelle, dans l'ordre des feuilles puis des cellules. Un fil
// d'une ligne masquée par un filtre, ou dont l'ancre a disparu, n'y est pas.
func (m *Model) Threads(ctx context.Context, sheet int64) ([]Thread, error) {
	out := []Thread{}
	err := m.read(ctx, func(t *tx) error {
		var sheets []int64
		names := map[int64]string{}
		rs, err := t.query(`SELECT id, name FROM _lattis_sheets WHERE ? = 0 OR id = ? ORDER BY pos, id`, sheet, sheet)
		if err != nil {
			return err
		}
		for rs.Next() {
			var id int64
			var name string
			if err := rs.Scan(&id, &name); err != nil {
				rs.Close()
				return err
			}
			sheets = append(sheets, id)
			names[id] = name
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
		for _, sh := range sheets {
			th, err := t.sheetThreads(sh)
			if err != nil {
				return err
			}
			for i := range th {
				th[i].SheetName = names[sh]
			}
			out = append(out, th...)
		}
		return t.threadComments(out)
	})
	return out, err
}

// sheetThreads : fils d'une feuille (sans leurs messages), triés par place.
func (t *tx) sheetThreads(sheet int64) ([]Thread, error) {
	var out []Thread
	rs, err := t.query(`SELECT thread, row, col, resolved <> 0 FROM _lattis_comment_cells WHERE sheet = ?`, sheet)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		th := Thread{Sheet: sheet}
		if err := rs.Scan(&th.ID, &th.Row, &th.Col, &th.Resolved); err != nil {
			rs.Close()
			return nil, err
		}
		out = append(out, th)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	tables, err := t.tables(sheet)
	if err != nil {
		return nil, err
	}
	for i := range tables {
		th, err := t.tableThreads(&tables[i])
		if err != nil {
			return nil, err
		}
		out = append(out, th...)
	}
	sortThreads(out)
	return out, nil
}

// tableThreads : fils d'un tableau, à la place affichée de leur ligne.
func (t *tx) tableThreads(tb *Table) ([]Thread, error) {
	at := map[int64]int{}
	for i, c := range tb.Cols {
		at[c.ID] = i
	}
	type hit struct {
		th  Thread
		rid int64
	}
	var hits []hit
	rs, err := t.query(`SELECT thread, rid, col, resolved <> 0 FROM _lattis_comment_rows WHERE tbl = ?`, tb.ID)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var h hit
		var colID int64
		if err := rs.Scan(&h.th.ID, &h.rid, &colID, &h.th.Resolved); err != nil {
			rs.Close()
			return nil, err
		}
		i, ok := at[colID]
		if !ok {
			continue // colonne supprimée
		}
		h.th.Sheet, h.th.Table, h.th.Col = tb.Sheet, tb.Name, tb.Col+i
		hits = append(hits, h)
	}
	rs.Close()
	if err := rs.Err(); err != nil || len(hits) == 0 {
		return nil, err
	}
	// Rang affiché des lignes (absentes : supprimées ou masquées par un filtre).
	var rids []any
	for _, h := range hits {
		if h.rid > 0 {
			rids = append(rids, h.rid)
		}
	}
	rank := map[int64]int{}
	for k := 0; k < len(rids); k += 500 {
		part := rids[k:min(k+500, len(rids))]
		rs, err := t.query(`SELECT id, n FROM (SELECT id, row_number() OVER (ORDER BY _pos) AS n FROM `+tb.view()+`)
			WHERE id IN (`+strings.TrimSuffix(strings.Repeat("?, ", len(part)), ", ")+`)`, part...)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var id int64
			var n int
			if err := rs.Scan(&id, &n); err != nil {
				rs.Close()
				return nil, err
			}
			rank[id] = n
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return nil, err
		}
	}
	var out []Thread
	for _, h := range hits {
		switch n, ok := rank[h.rid]; {
		case h.rid < 0:
			h.th.Row = tb.Row
		case ok:
			h.th.Row = tb.Row + n
		default:
			continue
		}
		out = append(out, h.th)
	}
	return out, nil
}

func sortThreads(th []Thread) {
	for i := 1; i < len(th); i++ {
		for j := i; j > 0 && (th[j].Row < th[j-1].Row || th[j].Row == th[j-1].Row && th[j].Col < th[j-1].Col); j-- {
			th[j], th[j-1] = th[j-1], th[j]
		}
	}
}

// threadComments remplit les messages des fils, dans l'ordre de leur date.
func (t *tx) threadComments(th []Thread) error {
	if len(th) == 0 {
		return nil
	}
	at := map[string]int{}
	ids := make([]any, 0, len(th))
	for i := range th {
		at[th[i].ID] = i
		th[i].Comments = []Comment{}
		ids = append(ids, th[i].ID)
	}
	for k := 0; k < len(ids); k += 500 {
		part := ids[k:min(k+500, len(ids))]
		rs, err := t.query(`SELECT thread, id, author, author_id, at, coalesce(edited, ''), text FROM _lattis_comments
			WHERE thread IN (`+strings.TrimSuffix(strings.Repeat("?, ", len(part)), ", ")+`) ORDER BY at, id <> thread, id`, part...)
		if err != nil {
			return err
		}
		for rs.Next() {
			var thread string
			var c Comment
			if err := rs.Scan(&thread, &c.ID, &c.Author, &c.AuthorID, &c.At, &c.Edited, &c.Text); err != nil {
				rs.Close()
				return err
			}
			if i, ok := at[thread]; ok {
				th[i].Comments = append(th[i].Comments, c)
			}
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
	}
	return nil
}

// dropSheetComments supprime les fils des cellules libres d'une feuille
// supprimée et des tableaux qu'elle contient (deleteSheet ; rendus par
// l'annulation) : un identifiant de feuille ou de tableau repris plus tard
// ne les retrouve pas.
func (t *tx) dropSheetComments(sheet int64, tables []Table) error {
	if err := t.deleteWhere(`_lattis_comments`, `thread IN (SELECT thread FROM _lattis_comment_cells WHERE sheet = ?)`, sheet); err != nil {
		return err
	}
	if err := t.deleteWhere(`_lattis_comment_cells`, `sheet = ?`, sheet); err != nil {
		return err
	}
	for _, tb := range tables {
		if err := t.dropTableComments(tb.ID); err != nil {
			return err
		}
	}
	return nil
}

// dropTableComments supprime les fils d'un tableau supprimé.
func (t *tx) dropTableComments(tbl int64) error {
	if err := t.deleteWhere(`_lattis_comments`, `thread IN (SELECT thread FROM _lattis_comment_rows WHERE tbl = ?)`, tbl); err != nil {
		return err
	}
	return t.deleteWhere(`_lattis_comment_rows`, `tbl = ?`, tbl)
}

// dropLooseComments supprime les messages des fils dont l'ancre vient de
// partir (lignes ou colonnes de la feuille supprimées).
func (t *tx) dropLooseComments() error {
	return t.deleteWhere(`_lattis_comments`, `thread NOT IN (SELECT thread FROM _lattis_comment_cells UNION ALL SELECT thread FROM _lattis_comment_rows)`)
}
