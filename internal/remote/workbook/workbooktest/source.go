package workbooktest

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/protocol"
)

// Sources du Data Lake tirées d'un classeur local (contrat attendu du
// serveur : docs/collaboration/pipeline-table-source.md). Le faux serveur
// garde la copie du tableau dans un classeur collaboratif caché (sans
// membre : absent de my_workbooks), partagé par une donnée ordinaire : le
// catalogue, la lecture et les relevés des tableaux liés marchent comme
// pour un tableau publié. Un envoi notifie le canal de ce classeur.

type fsource struct {
	id, dataset, owner, book string
	keys                     []string
	origin                   workbook.TableOrigin
	runs                     []workbook.PipelineRun
}

func (s *session) unsupported(name string) error {
	if s.f.NoTableSources {
		return ErrServer{"42883", "function lattis." + name + " does not exist"}
	}
	return nil
}

func (s *session) TableSourcesSupported(ctx context.Context) (bool, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("table_sources_supported"); err != nil {
		return false, err
	}
	return !s.f.NoTableSources, nil
}

func (s *session) source(id string) (*fsource, error) {
	for _, x := range s.f.sources {
		if x.id == id {
			if x.owner != s.user && !s.f.serverOwners[s.user] {
				return nil, ErrPrivilege{"permission denied: seul le créateur de la source l'alimente"}
			}
			return x, nil
		}
	}
	return nil, ErrServer{"P0002", "source inconnue"}
}

func checkKeys(cols []workbook.ColumnSpec, keys []string) error {
	for _, k := range keys {
		if !slices.ContainsFunc(cols, func(c workbook.ColumnSpec) bool { return c.Name == k }) {
			return ErrServer{"22023", "colonne clé inconnue : " + k}
		}
	}
	return nil
}

func (s *session) CreateTableSource(ctx context.Context, spec workbook.TableSourceSpec) (workbook.TableSourceCreated, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	var out workbook.TableSourceCreated
	if err := s.begin("create_table_source"); err != nil {
		return out, err
	}
	if err := s.unsupported("create_table_source(jsonb)"); err != nil {
		return out, err
	}
	if strings.TrimSpace(spec.Name) == "" || len(spec.Columns) == 0 {
		return out, ErrServer{"22023", "nom et colonnes obligatoires"}
	}
	if err := checkKeys(spec.Columns, spec.KeyColumns); err != nil {
		return out, err
	}
	d := s.f.data()
	sql := spec.SQLName
	if sql == "" {
		sql = strings.Trim(slugRe.ReplaceAllString(strings.ToLower(spec.Name), "_"), "_")
	}
	for _, ds := range d.datasets {
		if ds.sqlName == sql {
			return out, ErrServer{"23505", fmt.Sprintf("le nom SQL « %s » est déjà pris", sql)}
		}
	}
	slug := "src_" + sql
	b := &book{id: uuid.NewString(), slug: slug, title: spec.Origin.Title, mode: workbook.Collaborative, state: "published", version: 1, schema: "wb_" + slug,
		roles: map[string][]string{}, schemas: map[string]*schema{}}
	t := &table{id: uuid.NewString(), name: spec.Origin.Table, localID: 1}
	for i, c := range spec.Columns {
		t.cols = append(t.cols, workbook.RemoteColumn{ID: fmt.Sprintf("c%d", i+1), Name: c.Name, Type: c.Type, Position: i})
	}
	b.schemas[b.schema] = &schema{meta: map[string][]byte{}, tables: []*table{t}}
	s.f.books[b.id] = b
	src := &fsource{id: uuid.NewString(), owner: s.user, book: b.id, keys: slices.Clone(spec.KeyColumns), origin: spec.Origin}
	ds := &fdataset{id: d.next("d0000000"), folder: spec.FolderID, name: spec.Name, desc: spec.Description, sqlName: sql,
		workbook: b.id, table: t.id, inherit: true, pipeline: src.id}
	d.datasets = append(d.datasets, ds)
	src.dataset = ds.id
	s.f.sources = append(s.f.sources, src)
	return workbook.TableSourceCreated{PipelineID: src.id, DatasetID: ds.id, SQLName: sql}, nil
}

// keyOf : valeurs des colonnes clés d'une ligne, en texte.
func keyOf(vals []any, idx []int) string {
	var b strings.Builder
	for _, i := range idx {
		fmt.Fprintf(&b, "%v\x00", vals[i])
	}
	return b.String()
}

func (s *session) PushTableSource(ctx context.Context, id string, p workbook.TablePush) (workbook.PushResult, error) {
	s.f.mu.Lock()
	var out workbook.PushResult
	if err := s.begin("push_table_source"); err != nil {
		s.f.mu.Unlock()
		return out, err
	}
	if err := s.unsupported("begin_table_push(uuid, jsonb)"); err != nil {
		s.f.mu.Unlock()
		return out, err
	}
	src, err := s.source(id)
	if err != nil {
		s.f.mu.Unlock()
		return out, err
	}
	if err := checkKeys(p.Columns, p.KeyColumns); err != nil {
		s.f.mu.Unlock()
		return out, err
	}
	b := s.f.books[src.book]
	t := b.schemas[b.schema].tables[0]
	// Colonnes : celles de l'envoi, dans son ordre ; une colonne gardée
	// garde son identifiant (et ses valeurs, converties si le type change).
	old := t.cols
	cols := make([]workbook.RemoteColumn, len(p.Columns))
	for i, c := range p.Columns {
		cols[i] = workbook.RemoteColumn{ID: uuid.NewString(), Name: c.Name, Type: c.Type, Position: i}
		for _, o := range old {
			if o.Name == c.Name {
				cols[i].ID = o.ID
			}
		}
	}
	idx := make([]int, len(p.KeyColumns))
	for i, k := range p.KeyColumns {
		idx[i] = slices.IndexFunc(p.Columns, func(c workbook.ColumnSpec) bool { return c.Name == k })
	}
	if len(idx) > 0 {
		seen := map[string]int{}
		for n, r := range p.Rows {
			k := keyOf(r, idx)
			if m, ok := seen[k]; ok {
				s.f.mu.Unlock()
				return out, ErrServer{"23505", fmt.Sprintf("clé en double aux lignes %d et %d : l'envoi est refusé", m+1, n+1)}
			}
			seen[k] = n
		}
	}
	// Lignes actuelles, relues dans les colonnes de l'envoi.
	cur := map[string]row{}
	var order []string
	for _, r := range t.rows {
		vals := make([]any, len(cols))
		for i, c := range cols {
			for j, o := range old {
				if o.ID == c.ID && j < len(r.vals) {
					vals[i] = r.vals[j]
				}
			}
		}
		r.vals = vals
		k := r.id
		if len(idx) > 0 {
			k = keyOf(vals, idx)
		}
		cur[k] = r
		order = append(order, k)
	}
	s.f.xid++
	xid, now := s.f.xid, s.f.Clock()
	var rows []row
	kept := map[string]bool{}
	for n, vals := range p.Rows {
		pos := fmt.Sprintf("p%09d", n)
		if len(idx) > 0 {
			k := keyOf(vals, idx)
			if r, ok := cur[k]; ok {
				kept[k] = true
				same := r.pos == pos
				for i := range vals {
					same = same && workbook.SameRemote(r.vals[i], vals[i])
				}
				if !same {
					r.pos, r.vals, r.xid, r.at, r.by = pos, slices.Clone(vals), xid, now, "pipeline"
					out.Updated++
				}
				rows = append(rows, r)
				continue
			}
		}
		rows = append(rows, row{id: uuid.NewString(), pos: pos, vals: slices.Clone(vals), xid: xid, at: now, by: "pipeline"})
		out.Inserted++
	}
	for _, k := range order {
		if !kept[k] {
			t.tombs = append(t.tombs, tomb{id: cur[k].id, xid: xid, at: now})
			out.Deleted++
		}
	}
	t.cols, t.rows = cols, rows
	src.keys = slices.Clone(p.KeyColumns)
	out.RunID, out.Status, out.Rows = uuid.NewString(), "ok", len(p.Rows)
	src.runs = append(src.runs, workbook.PipelineRun{ID: out.RunID, Trigger: "workbook", Status: "ok", StartedAt: now.Format(time.RFC3339),
		FinishedAt: now.Format(time.RFC3339), Inserted: out.Inserted, Updated: out.Updated, Deleted: out.Deleted})
	channel, payload := "wb_"+b.slug, t.id
	structure := len(old) != len(cols) || slices.ContainsFunc(cols, func(c workbook.RemoteColumn) bool {
		return !slices.ContainsFunc(old, func(o workbook.RemoteColumn) bool { return o.ID == c.ID && o.Type == c.Type })
	})
	if structure {
		b.version++
	}
	version := b.version
	s.f.mu.Unlock()
	if structure {
		// Colonnes changées : relecture complète des tableaux liés, comme
		// après une nouvelle version d'un classeur publié.
		s.f.notify(channel, protocol.DataVersion+fmt.Sprint(version))
		return out, nil
	}
	s.f.notify(channel, payload)
	return out, nil
}

func (s *session) TableSources(ctx context.Context) ([]workbook.TableSource, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("my_table_sources"); err != nil {
		return nil, err
	}
	if err := s.unsupported("my_table_sources()"); err != nil {
		return nil, err
	}
	out := []workbook.TableSource{}
	for _, x := range s.f.sources {
		if x.owner != s.user && !s.f.serverOwners[s.user] {
			continue
		}
		item := workbook.TableSource{PipelineID: x.id, DatasetID: x.dataset, KeyColumns: slices.Clone(x.keys), Origin: x.origin, Owner: x.owner}
		if _, t, ds := s.f.dataSource(x.dataset); ds != nil {
			item.Name, item.SQLName = ds.name, ds.sqlName
			if t != nil {
				for _, c := range t.cols {
					item.Columns = append(item.Columns, workbook.ColumnSpec{Name: c.Name, Type: c.Type})
				}
			}
		}
		if n := len(x.runs); n > 0 {
			r := x.runs[n-1]
			item.LastRun = &r
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *session) DropTableSource(ctx context.Context, id string) error {
	s.f.mu.Lock()
	if err := s.begin("drop_table_source"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	if err := s.unsupported("drop_table_source(uuid)"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	src, err := s.source(id)
	if err != nil {
		s.f.mu.Unlock()
		return err
	}
	d := s.f.data()
	d.datasets = slices.DeleteFunc(d.datasets, func(ds *fdataset) bool { return ds.id == src.dataset })
	delete(s.f.books, src.book)
	s.f.sources = slices.DeleteFunc(s.f.sources, func(x *fsource) bool { return x.id == id })
	s.f.mu.Unlock()
	s.f.DataNotify(protocol.DataChannel, protocol.DataWithdrawn+src.dataset)
	return nil
}

// SourceRows rend les lignes de la copie d'une source, dans l'ordre
// (valeurs du serveur), pour les essais.
func (f *Fake) SourceRows(id string) (cols []string, rows [][]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.sources {
		if x.id != id {
			continue
		}
		b := f.books[x.book]
		t := b.schemas[b.schema].tables[0]
		for _, c := range t.cols {
			cols = append(cols, c.Name)
		}
		for _, r := range t.sorted() {
			rows = append(rows, slices.Clone(r.vals))
		}
	}
	return cols, rows
}
