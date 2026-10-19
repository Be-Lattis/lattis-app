package workbooktest

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/protocol"
)

// Relevé incrémental des données d'entreprise (DatasetDelta), requêtes de
// l'onglet SQL (DataQuery), et de quoi les essayer sans publier de classeur :
// SeedDataset crée un classeur, son tableau et la donnée qui le partage ;
// DataPut, DataDelete et DataHide écrivent dans le tableau source comme une
// transaction du serveur (xid, pierre tombale, notification sur le canal du
// classeur).

// pgTypes : nom du type PostgreSQL (DatabaseTypeName) d'une colonne du
// serveur (lattis.create_table).
var pgTypes = map[string]string{"integer": "INT8", "decimal": "NUMERIC", "boolean": "BOOL", "date": "DATE", "datetime": "TIMESTAMPTZ", "text": "TEXT"}

func (s *session) DatasetDelta(ctx context.Context, id, sqlName, since string) (workbook.DataDelta, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("dataset_delta"); err != nil {
		return workbook.DataDelta{}, err
	}
	ds, err := s.dataset(id)
	if err != nil || ds.sqlName != sqlName {
		return workbook.DataDelta{}, ErrPrivilege{"permission denied: donnée " + sqlName}
	}
	t := s.sourceTable(ds)
	if t == nil {
		return workbook.DataDelta{}, ErrPrivilege{"donnée indisponible"}
	}
	sp := s.f.xid + 1
	for _, h := range s.f.held {
		sp = min(sp, h)
	}
	out := workbook.DataDelta{Since: strconv.FormatInt(sp, 10)}
	from, _ := strconv.ParseInt(since, 10, 64)
	// since_covered : faux quand des pierres tombales d'après since ont été
	// purgées.
	out.Full = since == "" || (from != 0 && from <= s.f.purged)
	if !out.Full {
		s.f.Calls["since_covered"]++
	}
	var keep []int
	for i, c := range t.cols {
		if !slices.Contains(ds.hidden, c.Name) {
			keep = append(keep, i)
			out.Columns = append(out.Columns, c.Name)
			out.Types = append(out.Types, pgTypes[c.Type])
		}
	}
	for _, r := range t.sorted() {
		if ds.gone[r.id] {
			// dataset_changed_ids : changée depuis since, hors de la vue.
			if !out.Full && r.xid >= from {
				out.Deleted = append(out.Deleted, r.id)
			}
			continue
		}
		out.Count++
		if !out.Full && r.xid < from {
			continue
		}
		vals := make([]any, len(keep))
		for j, i := range keep {
			if i < len(r.vals) {
				vals[j] = r.vals[i]
			}
		}
		out.Rows = append(out.Rows, workbook.DataRow{ID: r.id, Pos: r.pos, Values: vals})
	}
	if !out.Full {
		for _, x := range t.tombs {
			if x.xid >= from {
				out.Deleted = append(out.Deleted, x.id)
			}
		}
	}
	return out, nil
}

var dataSelect = regexp.MustCompile(`(?i)^\s*select\s+\*\s+from\s+donnees\.("?)([a-z0-9_]+)"?\s*;?\s*$`)

// DataQuery ne comprend que SELECT * FROM donnees.<nom>.
func (s *session) DataQuery(ctx context.Context, q string, limit int) (workbook.DataResult, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("data_query"); err != nil {
		return workbook.DataResult{}, err
	}
	m := dataSelect.FindStringSubmatch(q)
	if m == nil {
		return workbook.DataResult{}, fmt.Errorf("requête non comprise par le serveur de test : %s", q)
	}
	for _, ds := range s.f.data().datasets {
		if ds.sqlName != m[2] {
			continue
		}
		t := s.sourceTable(ds)
		if t == nil {
			break
		}
		out := workbook.DataResult{Columns: []string{}, Rows: [][]any{}}
		var keep []int
		for i, c := range t.cols {
			if !slices.Contains(ds.hidden, c.Name) {
				keep = append(keep, i)
				out.Columns = append(out.Columns, c.Name)
			}
		}
		for _, r := range t.sorted() {
			if ds.gone[r.id] {
				continue
			}
			if limit > 0 && len(out.Rows) == limit {
				out.Truncated = true
				break
			}
			row := make([]any, len(keep))
			for j, i := range keep {
				if i < len(r.vals) {
					row[j] = r.vals[i]
				}
			}
			out.Rows = append(out.Rows, row)
		}
		return out, nil
	}
	return workbook.DataResult{}, ErrPrivilege{"la relation « donnees." + m[2] + " » n'existe pas"}
}

// SeedDataset crée un classeur publié par owner avec un tableau de colonnes
// cols (types du serveur : text, integer, decimal…), et la donnée sqlName
// qui le partage ; rend l'identifiant de la donnée.
func (f *Fake) SeedDataset(owner, sqlName string, cols []workbook.RemoteColumn) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.data()
	slug := "src_" + sqlName
	b := &book{id: uuid.NewString(), slug: slug, title: "Source " + sqlName, mode: workbook.Collaborative, state: "published", version: 1, schema: "wb_" + slug,
		roles: map[string][]string{owner: {"owner"}}, schemas: map[string]*schema{}}
	t := &table{id: uuid.NewString(), name: sqlName, localID: 1}
	for i, c := range cols {
		if c.ID == "" {
			c.ID = fmt.Sprintf("c%d", i+1)
		}
		c.Position = i
		t.cols = append(t.cols, c)
	}
	b.schemas[b.schema] = &schema{meta: map[string][]byte{}, tables: []*table{t}}
	f.books[b.id] = b
	ds := &fdataset{id: d.next("d0000000"), name: sqlName, sqlName: sqlName, workbook: b.id, table: t.id, inherit: true}
	d.datasets = append(d.datasets, ds)
	return ds.id
}

// dataSource : classeur et tableau source d'une donnée (mu tenu).
func (f *Fake) dataSource(id string) (*book, *table, *fdataset) {
	for _, ds := range f.data().datasets {
		if ds.id != id {
			continue
		}
		b := f.books[ds.workbook]
		if b == nil {
			return nil, nil, ds
		}
		for _, t := range b.schemas[b.schema].tables {
			if t.id == ds.table {
				return b, t, ds
			}
		}
		return b, nil, ds
	}
	return nil, nil, nil
}

// dataWrite applique fn au tableau source d'une donnée dans une nouvelle
// transaction, puis notifie le canal du classeur (charge : table_id).
func (f *Fake) dataWrite(id string, fn func(t *table, ds *fdataset, xid int64)) {
	f.mu.Lock()
	b, t, ds := f.dataSource(id)
	if t == nil {
		f.mu.Unlock()
		panic("donnée inconnue : " + id)
	}
	f.xid++
	fn(t, ds, f.xid)
	channel, payload := "wb_"+b.slug, t.id
	f.mu.Unlock()
	f.notify(channel, payload)
}

// DataPut ajoute ou remplace une ligne du tableau source d'une donnée
// (valeurs du serveur, une par colonne).
func (f *Fake) DataPut(id, rowID, pos string, vals ...any) {
	f.dataWrite(id, func(t *table, ds *fdataset, xid int64) {
		r := row{id: rowID, pos: pos, vals: slices.Clone(vals), xid: xid, at: f.Clock()}
		if i := t.row(rowID); i >= 0 {
			if pos == "" {
				r.pos = t.rows[i].pos
			}
			t.rows[i] = r
			return
		}
		t.rows = append(t.rows, r)
	})
}

// DataDelete supprime une ligne du tableau source (pierre tombale).
func (f *Fake) DataDelete(id, rowID string) {
	f.dataWrite(id, func(t *table, ds *fdataset, xid int64) {
		if i := t.row(rowID); i >= 0 {
			t.rows = slices.Delete(t.rows, i, i+1)
			t.tombs = append(t.tombs, tomb{id: rowID, xid: xid, at: f.Clock()})
		}
	})
}

// DataHide fait sortir une ligne de l'audience (une modification la rend
// invisible par le filtre de la vue) : elle change de transaction mais ne
// laisse pas de pierre tombale.
func (f *Fake) DataHide(id, rowID string) {
	f.dataWrite(id, func(t *table, ds *fdataset, xid int64) {
		if i := t.row(rowID); i >= 0 {
			t.rows[i].xid = xid
		}
		if ds.gone == nil {
			ds.gone = map[string]bool{}
		}
		ds.gone[rowID] = true
	})
}

// DataNotifyView notifie lattis_data que la vue d'une donnée est refaite.
func (f *Fake) DataNotifyView(id string) { f.DataNotify(protocol.DataChannel, id) }
