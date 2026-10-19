package workbooktest

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/protocol"
)

// Sources poussées (contrat §9.1, migration 0027 du serveur) : un classeur
// technique (kind « source », dont le créateur est propriétaire, absent de
// my_workbooks) qui ne contient que le tableau, un pipeline « push » et la
// donnée du catalogue. L'envoi (Pusher) lit le CSV comme le pipeline du
// serveur : colonnes reconnues par leur nom (sans la casse), valeurs
// converties au type de la colonne, refus par valeur (ligne écartée), puis
// remplacement, ajout ou mise à jour par clé ; il notifie le canal du
// classeur technique.

type fsource struct {
	id, dataset, owner, book string
	mode                     string
	keys                     []string
	origin                   workbook.PushedOrigin
	runs                     []workbook.PushRun
	// suspended : enabled = false (update_pushed_source).
	suspended bool
}

func (s *session) UpdatePushedSource(ctx context.Context, id string, c workbook.PushedSourceChange) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("update_pushed_source"); err != nil {
		return err
	}
	if s.f.NoPushedSources || s.f.Before28 {
		return ErrServer{"42883", "function lattis.update_pushed_source(uuid, jsonb) does not exist"}
	}
	src, err := s.f.pushed(s.user, id)
	if err != nil {
		return err
	}
	mode, keys := src.mode, src.keys
	if c.Mode != nil {
		mode = *c.Mode
	}
	if c.KeyColumns != nil {
		keys = *c.KeyColumns
	}
	if !slices.Contains([]string{workbook.PushReplace, workbook.PushAppend, workbook.PushUpsert}, mode) {
		return ErrServer{"22023", "mode : replace, append ou upsert"}
	}
	if mode == workbook.PushUpsert && len(keys) == 0 {
		return ErrServer{"22023", "mise à jour par clé : choisissez au moins une colonne clé"}
	}
	b := s.f.books[src.book]
	t := b.schemas[b.schema].tables[0]
	for _, k := range keys {
		if !slices.ContainsFunc(t.cols, func(c workbook.RemoteColumn) bool { return c.Name == k }) {
			return ErrServer{"22023", "colonne clé inconnue : " + k}
		}
	}
	src.mode, src.keys = mode, slices.Clone(keys)
	if c.Enabled != nil {
		src.suspended = !*c.Enabled
	}
	s.f.pipelines = slices.DeleteFunc(s.f.pipelines, func(p *fpipeline) bool { return p.workbook == src.book })
	if len(keys) > 0 {
		name := src.id
		if _, _, ds := s.f.dataSource(src.dataset); ds != nil {
			name = ds.name
		}
		s.f.pipelines = append(s.f.pipelines, &fpipeline{b.id, t.id, name, slices.Clone(keys)})
	}
	return nil
}

func (s *session) noPush(name string) error {
	if s.f.NoPushedSources {
		return ErrServer{"42883", "function lattis." + name + " does not exist"}
	}
	return nil
}

func (s *session) PushedSourcesSupported(ctx context.Context) (bool, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("pushed_sources_supported"); err != nil {
		return false, err
	}
	return !s.f.NoPushedSources, nil
}

func (s *session) CreatePushedSource(ctx context.Context, spec workbook.PushedSourceSpec) (workbook.PushedSourceCreated, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	var out workbook.PushedSourceCreated
	if err := s.begin("create_pushed_source"); err != nil {
		return out, err
	}
	if err := s.noPush("create_pushed_source(jsonb)"); err != nil {
		return out, err
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return out, ErrServer{"22023", "nom de la donnée manquant"}
	}
	mode := cmpOr(spec.Mode, workbook.PushReplace)
	if !slices.Contains([]string{workbook.PushReplace, workbook.PushAppend, workbook.PushUpsert}, mode) {
		return out, ErrServer{"22023", "mode : replace, append ou upsert"}
	}
	if mode == workbook.PushUpsert && len(spec.KeyColumns) == 0 {
		return out, ErrServer{"23514", "un pipeline upsert a des colonnes clés"}
	}
	for _, k := range spec.KeyColumns {
		if !slices.ContainsFunc(spec.Columns, func(c workbook.ColumnSpec) bool { return c.Name == k }) {
			return out, ErrServer{"22023", "colonne clé inconnue : " + k}
		}
	}
	d := s.f.data()
	sql := spec.SQLName
	if sql == "" {
		sql = strings.Trim(slugRe.ReplaceAllString(strings.ToLower(name), "_"), "_")
	}
	for _, ds := range d.datasets {
		if ds.sqlName == sql {
			return out, ErrServer{"23505", fmt.Sprintf("le nom SQL « %s » est déjà pris", sql)}
		}
	}
	slug := "src_" + sql
	b := &book{id: uuid.NewString(), slug: slug, title: "Source : " + name, mode: workbook.Collaborative, state: "published", version: 1,
		schema: "wb_" + slug, roles: map[string][]string{s.user: {"owner"}}, schemas: map[string]*schema{}, kind: "source"}
	t := &table{id: uuid.NewString(), name: cmpOr(strings.TrimSpace(spec.TableName), name), localID: 1}
	for i, c := range spec.Columns {
		t.cols = append(t.cols, workbook.RemoteColumn{ID: uuid.NewString(), Name: c.Name, Type: c.Type, Position: i})
	}
	b.schemas[b.schema] = &schema{meta: map[string][]byte{}, tables: []*table{t}}
	s.f.books[b.id] = b
	src := &fsource{id: uuid.NewString(), owner: s.user, book: b.id, mode: mode, keys: slices.Clone(spec.KeyColumns), origin: spec.Origin}
	ds := &fdataset{id: d.next("d0000000"), folder: spec.FolderID, name: name, desc: spec.Description, sqlName: sql,
		workbook: b.id, table: t.id, inherit: true}
	d.datasets = append(d.datasets, ds)
	src.dataset = ds.id
	s.f.sources = append(s.f.sources, src)
	if len(src.keys) > 0 {
		s.f.pipelines = append(s.f.pipelines, &fpipeline{b.id, t.id, name, slices.Clone(src.keys)})
	}
	return workbook.PushedSourceCreated{PipelineID: src.id, DatasetID: ds.id, WorkbookID: b.id, TableID: t.id, SQLName: sql}, nil
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// pushed : la source de l'utilisateur (propriétaire du classeur technique) ; mu tenu.
func (f *Fake) pushed(user, id string) (*fsource, error) {
	for _, x := range f.sources {
		if x.id == id {
			if b := f.books[x.book]; b == nil || !slices.Contains(b.roles[user], "owner") {
				return nil, ErrPrivilege{"permission denied: propriétaire de la source seulement"}
			}
			return x, nil
		}
	}
	return nil, ErrServer{"P0002", "source poussée inconnue"}
}

func (f *Fake) sourceItem(x *fsource) workbook.PushedSource {
	b := f.books[x.book]
	item := workbook.PushedSource{PipelineID: x.id, Mode: x.mode, KeyColumns: slices.Clone(x.keys), Origin: x.origin, WorkbookID: x.book, Enabled: !x.suspended}
	if b != nil {
		item.TableID = b.schemas[b.schema].tables[0].id
	}
	if _, _, ds := f.dataSource(x.dataset); ds != nil {
		item.Name = ds.name
		item.Datasets = append(item.Datasets, struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			SQLName string `json:"sql_name"`
		}{ds.id, ds.name, ds.sqlName})
	}
	if n := len(x.runs); n > 0 {
		r := x.runs[n-1]
		item.LastRun = &r
	}
	return item
}

func (s *session) PushedSources(ctx context.Context) ([]workbook.PushedSource, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("my_pushed_sources"); err != nil {
		return nil, err
	}
	if err := s.noPush("my_pushed_sources()"); err != nil {
		return nil, err
	}
	out := []workbook.PushedSource{}
	for _, x := range s.f.sources {
		if _, err := s.f.pushed(s.user, x.id); err == nil {
			out = append(out, s.f.sourceItem(x))
		}
	}
	return out, nil
}

func (s *session) DropPushedSource(ctx context.Context, id string) error {
	s.f.mu.Lock()
	if err := s.begin("drop_pushed_source"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	if err := s.noPush("drop_pushed_source(uuid)"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	src, err := s.f.pushed(s.user, id)
	if err != nil {
		s.f.mu.Unlock()
		return err
	}
	d := s.f.data()
	d.datasets = slices.DeleteFunc(d.datasets, func(ds *fdataset) bool { return ds.id == src.dataset })
	s.f.pipelines = slices.DeleteFunc(s.f.pipelines, func(p *fpipeline) bool { return p.workbook == src.book })
	delete(s.f.books, src.book)
	s.f.sources = slices.DeleteFunc(s.f.sources, func(x *fsource) bool { return x.id == id })
	s.f.mu.Unlock()
	s.f.DataNotify(protocol.DataChannel, protocol.DataWithdrawn+src.dataset)
	return nil
}

// Pusher rend l'envoi HTTP (POST /api/push) vu par un utilisateur.
func (f *Fake) Pusher(user string) workbook.Pusher { return &fpusher{f: f, user: user} }

type fpusher struct {
	f    *Fake
	user string
}

// pushConvert lit une valeur du CSV comme le pipeline (pipeline/write.go convert).
func pushConvert(raw, typ string) (any, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil, nil
	}
	switch typ {
	case "integer":
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("entier attendu")
		}
		return n, nil
	case "decimal":
		if strings.Count(v, ",") == 1 && !strings.Contains(v, ".") {
			v = strings.Replace(v, ",", ".", 1)
		}
		x, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("nombre attendu")
		}
		return x, nil
	case "boolean":
		switch strings.ToLower(v) {
		case "true", "vrai", "oui", "1":
			return true, nil
		case "false", "faux", "non", "0":
			return false, nil
		}
		return nil, fmt.Errorf("booléen attendu (oui/non, vrai/faux, 1/0)")
	case "date":
		for _, l := range []string{"2006-01-02", "02/01/2006"} {
			if t, err := time.Parse(l, v); err == nil {
				return t, nil
			}
		}
		return nil, fmt.Errorf("date attendue (AAAA-MM-JJ ou JJ/MM/AAAA)")
	}
	return raw, nil
}

func (p *fpusher) Push(ctx context.Context, id string, body []byte) (workbook.PushRun, error) {
	f := p.f
	f.mu.Lock()
	f.Calls["push"]++
	if f.down {
		f.mu.Unlock()
		return workbook.PushRun{}, ErrDown
	}
	src, err := f.pushed(p.user, id)
	if err != nil {
		f.mu.Unlock()
		if code, _ := workbook.Rejected(err); code == "P0002" {
			return workbook.PushRun{}, &workbook.PushError{Status: http.StatusNotFound, Code: "not_found", Message: "source poussée inconnue"}
		}
		return workbook.PushRun{}, &workbook.PushError{Status: http.StatusForbidden, Code: "forbidden", Message: "propriétaire de la source seulement"}
	}
	if f.PushBusy || src.suspended {
		f.mu.Unlock()
		if src.suspended {
			return workbook.PushRun{}, &workbook.PushError{Status: http.StatusConflict, Code: "conflict", Message: "source poussée suspendue"}
		}
		return workbook.PushRun{}, &workbook.PushError{Status: http.StatusConflict, Code: "conflict", Message: "un envoi attend déjà d'être traité : patientez la fin de son exécution"}
	}
	if f.MaxPush > 0 && len(body) > f.MaxPush {
		f.mu.Unlock()
		return workbook.PushRun{}, &workbook.PushError{Status: http.StatusRequestEntityTooLarge, Code: "too_large", Message: "tableau trop gros (plus de 100 Mo)"}
	}
	f.LastPush = string(body)
	run := f.applyPush(src, body)
	src.runs = append(src.runs, run)
	b := f.books[src.book]
	channel, payload := "wb_"+b.slug, b.schemas[b.schema].tables[0].id
	slow, before := f.PushSlow, f.Before28
	f.mu.Unlock()
	if run.Status != "error" {
		f.notify(channel, payload)
	}
	if slow {
		if before {
			return workbook.PushRun{Status: "running"}, nil
		}
		return workbook.PushRun{Status: "running", ID: run.ID, RunID: run.ID}, nil
	}
	return run, nil
}

// applyPush écrit le CSV dans le tableau technique (mu tenu).
func (f *Fake) applyPush(src *fsource, body []byte) workbook.PushRun {
	now := f.Clock()
	run := workbook.PushRun{ID: uuid.NewString(), Trigger: "push", StartedAt: now.Format(time.RFC3339), FinishedAt: now.Format(time.RFC3339)}
	fail := func(msg string) workbook.PushRun {
		run.Status, run.Error = "error", msg
		return run
	}
	b := f.books[src.book]
	t := b.schemas[b.schema].tables[0]
	cr := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))))
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	head, err := cr.Read()
	if err != nil {
		return fail("CSV vide")
	}
	// Colonne du tableau → colonne du CSV (même nom, sans la casse).
	src2 := make([]int, len(t.cols))
	found := 0
	for i, c := range t.cols {
		src2[i] = slices.IndexFunc(head, func(h string) bool { return strings.EqualFold(strings.TrimSpace(h), c.Name) })
		if src2[i] >= 0 {
			found++
		}
	}
	if found == 0 {
		return fail(fmt.Sprintf("aucune colonne de la source ne correspond au tableau « %s » (en-têtes lus : %s)", t.name, strings.Join(head, ", ")))
	}
	keyIdx := []int{}
	for _, k := range src.keys {
		i := slices.IndexFunc(t.cols, func(c workbook.RemoteColumn) bool { return c.Name == k })
		if i < 0 || src2[i] < 0 {
			return fail(fmt.Sprintf("la colonne clé « %s » n'est pas dans la source", k))
		}
		keyIdx = append(keyIdx, i)
	}
	var rows [][]any
	for line := 2; ; line++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fail("CSV : " + err.Error())
		}
		run.RowsRead++
		vals := make([]any, len(t.cols))
		ok := true
		for i, c := range t.cols {
			if src2[i] < 0 || src2[i] >= len(rec) {
				continue
			}
			v, err := pushConvert(rec[src2[i]], c.Type)
			if err != nil {
				run.Rejected++
				run.Rejects = append(run.Rejects, workbook.PushReject{Row: line, Column: c.Name, Value: rec[src2[i]], Error: err.Error()})
				ok = false
				break
			}
			vals[i] = v
		}
		if ok {
			rows = append(rows, vals)
		}
	}
	f.xid++
	xid := f.xid
	key := func(v []any) string { return keyOf(v, keyIdx) }
	switch src.mode {
	case workbook.PushAppend:
		for _, v := range rows {
			t.rows = append(t.rows, row{id: uuid.NewString(), pos: fmt.Sprintf("p%09d", len(t.rows)), vals: v, xid: xid, at: now, by: "pipeline"})
			run.Inserted++
		}
	case workbook.PushUpsert:
		seen := map[string]bool{}
		for _, v := range rows {
			k := key(v)
			if seen[k] {
				return fail("clé en double : " + k)
			}
			seen[k] = true
			i := slices.IndexFunc(t.rows, func(r row) bool { return key(r.vals) == k })
			if i < 0 {
				t.rows = append(t.rows, row{id: uuid.NewString(), pos: fmt.Sprintf("p%09d", len(t.rows)), vals: v, xid: xid, at: now, by: "pipeline"})
				run.Inserted++
				continue
			}
			same := true
			for j := range v {
				same = same && workbook.SameRemote(t.rows[i].vals[j], v[j])
			}
			if !same {
				t.rows[i].vals, t.rows[i].xid, t.rows[i].at = v, xid, now
				run.Updated++
			}
		}
	default:
		for _, r := range t.rows {
			t.tombs = append(t.tombs, tomb{id: r.id, xid: xid, at: now})
		}
		run.Deleted = len(t.rows)
		t.rows = nil
		for n, v := range rows {
			t.rows = append(t.rows, row{id: uuid.NewString(), pos: fmt.Sprintf("p%09d", n), vals: v, xid: xid, at: now, by: "pipeline"})
		}
		run.Inserted = len(rows)
	}
	run.Status = "ok"
	if run.Rejected > 0 {
		run.Status = "warning"
	}
	return run
}

func keyOf(vals []any, idx []int) string {
	var sb strings.Builder
	for _, i := range idx {
		fmt.Fprintf(&sb, "%v\x00", vals[i])
	}
	return sb.String()
}

func (p *fpusher) PushStatus(ctx context.Context, id string) (workbook.PushedSource, error) {
	f := p.f
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls["push_status"]++
	src, err := f.pushed(p.user, id)
	if err != nil {
		return workbook.PushedSource{}, &workbook.PushError{Status: http.StatusNotFound, Code: "not_found", Message: "source poussée inconnue"}
	}
	return f.sourceItem(src), nil
}

// PushRun lit une exécution (GET ?run=) ; le premier appel répond
// « pending », comme un envoi pas encore pris par lattisd.
func (p *fpusher) PushRun(ctx context.Context, id, runID string) (workbook.PushRun, error) {
	f := p.f
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls["pushed_run"]++
	src, err := f.pushed(p.user, id)
	if err != nil {
		return workbook.PushRun{}, &workbook.PushError{Status: http.StatusNotFound, Code: "not_found", Message: "envoi inconnu"}
	}
	if f.Before28 {
		return workbook.PushRun{}, nil // état de la source, sans « status »
	}
	if f.Calls["pushed_run"] == 1 {
		return workbook.PushRun{ID: runID, Status: "pending"}, nil
	}
	for _, r := range src.runs {
		if r.ID == runID {
			return r, nil
		}
	}
	return workbook.PushRun{}, &workbook.PushError{Status: http.StatusNotFound, Code: "not_found", Message: "envoi inconnu"}
}

// SourceRows rend les colonnes et lignes du tableau d'une source poussée,
// dans l'ordre (valeurs du serveur), pour les essais.
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
			cols = append(cols, c.Name+":"+c.Type)
		}
		for _, r := range t.sorted() {
			rows = append(rows, slices.Clone(r.vals))
		}
	}
	return cols, rows
}
