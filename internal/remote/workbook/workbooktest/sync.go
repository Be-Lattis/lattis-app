package workbooktest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Synchronisation d'un classeur collaboratif (schema/0004_collaborative.sql
// du serveur) : écritures conditionnées sous les droits du tableau (lecteur :
// refus 42501 ; participant : seulement ses lignes, comme la RLS), pierres
// tombales, lattis.why_not, lattis.changes_since, une notification par
// tableau écrit.

// SetLevel impose à user le niveau level (none, reader, participant,
// editor) sur le tableau name d'un classeur collaboratif.
func (f *Fake) SetLevel(workbookID, user, name, level string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.levels[workbookID+"\x00"+user+"\x00"+name] = level
}

// Value lit une cellule de la version publiée (essais) : valeur du
// serveur, auteur de la ligne et dernier modificateur ; ok faux si la ligne
// n'existe pas.
func (f *Fake) Value(workbookID, table, id, col string) (v any, author, by string, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.books[workbookID]
	if b == nil {
		return nil, "", "", false
	}
	for _, t := range b.schemas[b.schema].tables {
		if t.name != table {
			continue
		}
		j := slices.IndexFunc(t.cols, func(c workbook.RemoteColumn) bool { return c.Name == col })
		for _, r := range t.rows {
			if r.id == id && j >= 0 {
				return r.vals[j], r.author, r.by, true
			}
		}
	}
	return nil, "", "", false
}

func (s *session) Delta(ctx context.Context, fn func(workbook.Reader) error) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("delta"); err != nil {
		return err
	}
	return fn(&reader{s: s})
}

func (s *session) Write(ctx context.Context, fn func(workbook.Writer) error) error {
	s.f.mu.Lock()
	if err := s.begin("write"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	saved := s.f.snapshot()
	w := &writer{s: s, xid: s.f.xid + 1, at: s.f.Clock(), touched: map[string]string{}}
	if err := fn(w); err != nil {
		s.f.books = saved
		s.f.mu.Unlock()
		return err
	}
	s.f.xid = w.xid
	touched := w.touched
	s.f.mu.Unlock()
	for id, channel := range touched {
		s.f.notify(channel, id)
	}
	return nil
}

// find donne le classeur, le tableau et le schéma visés par une écriture.
func (s *session) find(schemaName, name string) (*book, *table, error) {
	for _, b := range s.f.books {
		sc, ok := b.schemas[schemaName]
		if !ok {
			continue
		}
		for _, t := range sc.tables {
			if t.name == name {
				return b, t, nil
			}
		}
		return nil, nil, fmt.Errorf("la relation « %s.%s » n'existe pas", schemaName, name)
	}
	return nil, nil, fmt.Errorf("le schéma « %s » n'existe pas", schemaName)
}

type writer struct {
	s       *session
	xid     int64
	at      time.Time
	touched map[string]string // tableau → canal
}

// target vérifie le droit d'écrire dans un tableau et rend son niveau.
func (w *writer) target(schemaName, name string) (*book, *table, string, error) {
	b, t, err := w.s.find(schemaName, name)
	if err != nil {
		return nil, nil, "", err
	}
	if b.mode != workbook.Collaborative || b.state != "published" || schemaName != b.schema {
		return nil, nil, "", ErrPrivilege{"droit refusé pour la table " + name}
	}
	lv := w.s.level(b, t)
	if lv != "editor" && lv != "participant" {
		return nil, nil, "", ErrPrivilege{"droit refusé pour la table " + name}
	}
	return b, t, lv, nil
}

func (w *writer) touch(b *book, t *table, r *row) {
	if r != nil {
		r.xid, r.by, r.at = w.xid, w.s.user, w.at
	}
	w.touched[t.id] = "wb_" + b.slug
}

func (t *table) row(id string) int {
	return slices.IndexFunc(t.rows, func(r row) bool { return r.id == id })
}

func (t *table) col(name string) int {
	return slices.IndexFunc(t.cols, func(c workbook.RemoteColumn) bool { return c.Name == name })
}

func (w *writer) Update(ctx context.Context, schemaName, name, id string, col workbook.RemoteColumn, old, new any) (int64, error) {
	b, t, lv, err := w.target(schemaName, name)
	if err != nil {
		return 0, err
	}
	o, err := workbook.ToRemote(col.Type, old)
	if err != nil {
		return 0, err
	}
	n, err := workbook.ToRemote(col.Type, new)
	if err != nil {
		return 0, err
	}
	j := t.col(col.Name)
	if j < 0 {
		return 0, fmt.Errorf("la colonne « %s » n'existe pas", col.Name)
	}
	i := t.row(id)
	if i < 0 || (lv == "participant" && t.rows[i].author != w.s.user) || !workbook.SameRemote(t.rows[i].vals[j], o) {
		return 0, nil
	}
	t.rows[i].vals[j] = n
	w.touch(b, t, &t.rows[i])
	return 1, nil
}

func (w *writer) Insert(ctx context.Context, schemaName, name, id, pos string, cols []workbook.RemoteColumn, vals []any) (int64, error) {
	b, t, _, err := w.target(schemaName, name)
	if err != nil {
		return 0, err
	}
	if t.row(id) >= 0 {
		return 0, nil // ON CONFLICT DO NOTHING
	}
	r := row{id: id, pos: pos, author: w.s.user, vals: make([]any, len(t.cols))}
	for k, c := range cols {
		j := t.col(c.Name)
		if j < 0 {
			return 0, fmt.Errorf("la colonne « %s » n'existe pas", c.Name)
		}
		if r.vals[j], err = workbook.ToRemote(c.Type, vals[k]); err != nil {
			return 0, err
		}
	}
	w.touch(b, t, &r)
	t.rows = append(t.rows, r)
	return 1, nil
}

func (w *writer) Delete(ctx context.Context, schemaName, name, id string) (int64, error) {
	b, t, lv, err := w.target(schemaName, name)
	if err != nil {
		return 0, err
	}
	i := t.row(id)
	if i < 0 || (lv == "participant" && t.rows[i].author != w.s.user) {
		return 0, nil
	}
	t.rows = slices.Delete(t.rows, i, i+1)
	t.tombs = append(t.tombs, tomb{id: id, xid: w.xid, by: w.s.user, at: w.at})
	w.touch(b, t, nil)
	return 1, nil
}

func (w *writer) Move(ctx context.Context, schemaName, name, id, old, pos string) (int64, error) {
	b, t, lv, err := w.target(schemaName, name)
	if err != nil {
		return 0, err
	}
	i := t.row(id)
	if i < 0 || (lv == "participant" && t.rows[i].author != w.s.user) || t.rows[i].pos != old {
		return 0, nil
	}
	t.rows[i].pos = pos
	w.touch(b, t, &t.rows[i])
	return 1, nil
}

func (w *writer) WhyNot(ctx context.Context, workbookID string, rt workbook.RemoteTable, id string) (workbook.WhyNot, error) {
	w.s.f.Calls["why_not"]++
	b := w.s.f.books[workbookID]
	if b == nil {
		return workbook.WhyNot{}, errors.New("classeur inconnu")
	}
	var t *table
	for _, x := range b.schemas[b.schema].tables {
		if x.id == rt.ID {
			t = x
		}
	}
	if t == nil {
		return workbook.WhyNot{}, errors.New("tableau inconnu")
	}
	i := t.row(id)
	if i < 0 {
		for k := len(t.tombs) - 1; k >= 0; k-- {
			if t.tombs[k].id == id {
				return workbook.WhyNot{Reason: workbook.WhyDeleted, By: t.tombs[k].by, At: t.tombs[k].at.Format(time.RFC3339)}, nil
			}
		}
		return workbook.WhyNot{Reason: workbook.WhyMissing}, nil
	}
	r := t.rows[i]
	lv := w.s.level(b, t)
	if lv == "reader" || lv == "none" || (lv == "participant" && r.author != w.s.user) {
		return workbook.WhyNot{Reason: workbook.WhyForbidden}, nil
	}
	c := change(t, rt, r)
	return workbook.WhyNot{Reason: workbook.WhyModified, Row: &c, By: r.by, At: r.at.Format(time.RFC3339)}, nil
}

// change rend une ligne du serveur en valeurs locales, colonnes de rt.
func change(t *table, rt workbook.RemoteTable, r row) workbook.Change {
	c := workbook.Change{ID: r.id, Pos: r.pos, Author: r.author, UpdatedBy: r.by, UpdatedAt: r.at.Format(time.RFC3339), Values: map[string]any{}}
	for _, col := range rt.Columns {
		if j := t.col(col.Name); j >= 0 {
			c.Values[col.Name] = workbook.FromRemote(col.Type, r.vals[j])
		}
	}
	return c
}

func (r *reader) table(workbookID, tableID string) (*table, error) {
	b := r.s.f.books[workbookID]
	if b == nil {
		return nil, errors.New("classeur inconnu")
	}
	if len(r.s.roles(b)) == 0 {
		return nil, ErrPrivilege{"aucun accès à ce classeur"}
	}
	for _, t := range b.schemas[b.schema].tables {
		if t.id == tableID {
			if lv := r.s.level(b, t); lv == "none" {
				return nil, ErrPrivilege{"droit refusé pour la table " + t.name}
			}
			return t, nil
		}
	}
	return nil, errors.New("tableau inconnu")
}

func (r *reader) Changes(ctx context.Context, workbookID string, rt workbook.RemoteTable, since string, fn func(workbook.Change) error) error {
	r.s.f.Calls["changes_since"]++
	t, err := r.table(workbookID, rt.ID)
	if err != nil {
		return err
	}
	from, _ := strconv.ParseInt(since, 10, 64)
	for _, x := range t.sorted() {
		if x.xid >= from {
			if err := fn(change(t, rt, x)); err != nil {
				return err
			}
		}
	}
	for _, x := range t.tombs {
		if x.xid >= from {
			if err := fn(workbook.Change{ID: x.id, Deleted: true}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *reader) RowsByID(ctx context.Context, schemaName string, rt workbook.RemoteTable, ids []string, fn func(workbook.Change) error) error {
	s, err := r.schema(schemaName)
	if err != nil {
		return err
	}
	for _, t := range s.tables {
		if t.id != rt.ID {
			continue
		}
		for _, x := range t.rows {
			if slices.Contains(ids, x.id) {
				if err := fn(change(t, rt, x)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
