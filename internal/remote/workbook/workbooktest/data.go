package workbooktest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/protocol"
)

// Données du Data Lake du serveur en mémoire : catalogue, dossiers et vues
// donnees.<sql_name> lues dans le tableau source. L'audience n'est pas
// contrôlée (tout le monde lit) ; DataNotify envoie une notification.

type fdataset struct {
	id, folder, name, desc, sqlName, workbook, table string
	hidden                                           []string
	inherit                                          bool
	audience                                         []protocol.DataAudience
	links                                            []string
	// gone : lignes sorties de l'audience (DataHide).
	gone map[string]bool
}

type fdata struct {
	folders  []protocol.DataFolder
	datasets []*fdataset
	n        int
}

func (f *Fake) data() *fdata {
	if f.dataStore == nil {
		f.dataStore = &fdata{}
	}
	return f.dataStore
}

func (d *fdata) next(prefix string) string {
	d.n++
	return fmt.Sprintf("%s-0000-0000-0000-%012d", prefix, d.n)
}

// sourceTable : tableau source d'une donnée (mu tenu).
func (s *session) sourceTable(ds *fdataset) *table {
	b := s.f.books[ds.workbook]
	if b == nil {
		return nil
	}
	for _, t := range b.schemas[b.schema].tables {
		if t.id == ds.table {
			return t
		}
	}
	return nil
}

func (s *session) DataCatalog(ctx context.Context) (protocol.DataCatalog, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("data_catalog"); err != nil {
		return protocol.DataCatalog{}, err
	}
	d := s.f.data()
	out := protocol.DataCatalog{CanPublish: true, Folders: slices.Clone(d.folders), Datasets: []protocol.Dataset{}}
	for _, ds := range d.datasets {
		t := s.sourceTable(ds)
		item := protocol.Dataset{ID: ds.id, FolderID: ds.folder, Name: ds.name, Description: ds.desc, SQLName: ds.sqlName,
			Readable: true, Manageable: true, Available: t != nil, Columns: []protocol.DataColumn{}, Audience: ds.audience,
			Source: &protocol.DataSource{WorkbookID: ds.workbook, Table: ds.table, HiddenColumns: ds.hidden, InheritAudience: ds.inherit}}
		if b := s.f.books[ds.workbook]; b != nil {
			item.Source.Title = b.title
		}
		if t != nil {
			item.TableID = t.id
			item.Channel = "wb_" + s.f.books[ds.workbook].slug
			for _, c := range t.cols {
				if !slices.Contains(ds.hidden, c.Name) {
					item.Columns = append(item.Columns, protocol.DataColumn{Name: c.Name, Type: c.Type})
				}
			}
		}
		for _, w := range ds.links {
			item.UsedBy = append(item.UsedBy, protocol.DataUse{WorkbookID: w, Title: s.f.books[w].title})
		}
		out.Datasets = append(out.Datasets, item)
	}
	return out, nil
}

func (s *session) CreateDataFolder(ctx context.Context, name, parentID string) (string, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	d := s.f.data()
	id := d.next("f0000000")
	path, depth := name, 0
	for _, p := range d.folders {
		if p.ID == parentID {
			path, depth = p.Path+" › "+name, p.Depth+1
		}
	}
	d.folders = append(d.folders, protocol.DataFolder{ID: id, ParentID: parentID, Name: name, Path: path, Depth: depth})
	return id, nil
}

func (s *session) RenameDataFolder(ctx context.Context, id, name string) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	for i, p := range s.f.data().folders {
		if p.ID == id {
			s.f.dataStore.folders[i].Name = name
		}
	}
	return nil
}

func (s *session) MoveDataFolder(ctx context.Context, id, parentID string) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	for i, p := range s.f.data().folders {
		if p.ID == id {
			s.f.dataStore.folders[i].ParentID = parentID
		}
	}
	return nil
}

func (s *session) DropDataFolder(ctx context.Context, id string) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	d := s.f.data()
	d.folders = slices.DeleteFunc(d.folders, func(p protocol.DataFolder) bool { return p.ID == id })
	return nil
}

func (s *session) CreateDataset(ctx context.Context, spec workbook.DatasetSpec) (string, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("create_dataset"); err != nil {
		return "", err
	}
	b := s.f.books[spec.WorkbookID]
	if b == nil || !s.has(b, "owner", "designer") {
		return "", ErrPrivilege{"permission denied: propriétaire ou concepteur du classeur"}
	}
	d := s.f.data()
	ds := &fdataset{id: d.next("d0000000"), folder: spec.FolderID, name: spec.Name, desc: spec.Description,
		sqlName: spec.SQLName, workbook: spec.WorkbookID, table: spec.TableID, hidden: spec.Hidden, inherit: true}
	if ds.sqlName == "" {
		ds.sqlName = strings.Trim(slugRe.ReplaceAllString(strings.ToLower(spec.Name), "_"), "_")
	}
	if s.sourceTable(ds) == nil {
		return "", errors.New("tableau inconnu")
	}
	d.datasets = append(d.datasets, ds)
	return ds.id, nil
}

func (s *session) dataset(id string) (*fdataset, error) {
	for _, ds := range s.f.data().datasets {
		if ds.id == id {
			return ds, nil
		}
	}
	return nil, errors.New("donnée inconnue")
}

func (s *session) UpdateDataset(ctx context.Context, id string, c workbook.DatasetChange) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	ds, err := s.dataset(id)
	if err != nil {
		return err
	}
	if c.Name != nil {
		ds.name = *c.Name
	}
	if c.Description != nil {
		ds.desc = *c.Description
	}
	if c.FolderID != nil {
		ds.folder = *c.FolderID
	}
	if c.HiddenColumns != nil {
		ds.hidden = c.HiddenColumns
	}
	if c.InheritAudience != nil {
		ds.inherit = *c.InheritAudience
	}
	return nil
}

func (s *session) AddDataAudience(ctx context.Context, datasetID, folderID, principal, predicate string) (string, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	d := s.f.data()
	id := d.next("a0000000")
	a := protocol.DataAudience{ID: id, Kind: "user", Principal: principal, Label: principal, Predicate: predicate}
	if datasetID != "" {
		ds, err := s.dataset(datasetID)
		if err != nil {
			return "", err
		}
		ds.audience = append(ds.audience, a)
		return id, nil
	}
	for i, p := range d.folders {
		if p.ID == folderID {
			d.folders[i].Audience = append(d.folders[i].Audience, a)
		}
	}
	return id, nil
}

func (s *session) AddDataAudienceByEmail(ctx context.Context, datasetID, folderID, email, predicate string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return "", fmt.Errorf("aucun compte pour %s", email)
	}
	return s.AddDataAudience(ctx, datasetID, folderID, email, predicate)
}

func (s *session) RemoveDataAudience(ctx context.Context, id string) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	for _, ds := range s.f.data().datasets {
		ds.audience = slices.DeleteFunc(ds.audience, func(a protocol.DataAudience) bool { return a.ID == id })
	}
	return nil
}

func (s *session) DropDataset(ctx context.Context, id string) error {
	s.f.mu.Lock()
	d := s.f.data()
	d.datasets = slices.DeleteFunc(d.datasets, func(ds *fdataset) bool { return ds.id == id })
	s.f.mu.Unlock()
	s.f.DataNotify(protocol.DataChannel, protocol.DataWithdrawn+id)
	return nil
}

func (s *session) LinkDataset(ctx context.Context, id, workbookID string) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	ds, err := s.dataset(id)
	if err != nil {
		return err
	}
	if !slices.Contains(ds.links, workbookID) {
		ds.links = append(ds.links, workbookID)
	}
	return nil
}

func (s *session) UnlinkDataset(ctx context.Context, id, workbookID string) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	ds, err := s.dataset(id)
	if err != nil {
		return err
	}
	ds.links = slices.DeleteFunc(ds.links, func(w string) bool { return w == workbookID })
	return nil
}

func (s *session) ReadDataset(ctx context.Context, sqlName string) (workbook.DataRows, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("read_dataset"); err != nil {
		return workbook.DataRows{}, err
	}
	for _, ds := range s.f.data().datasets {
		if ds.sqlName != sqlName {
			continue
		}
		t := s.sourceTable(ds)
		if t == nil {
			return workbook.DataRows{}, ErrPrivilege{"donnée indisponible"}
		}
		out := workbook.DataRows{Since: strconv.FormatInt(s.f.xid, 10)}
		var keep []int
		for i, c := range t.cols {
			if !slices.Contains(ds.hidden, c.Name) {
				keep = append(keep, i)
				out.Columns = append(out.Columns, c.Name)
			}
		}
		for _, r := range t.sorted() {
			vals := make([]string, len(keep))
			for j, i := range keep {
				if i < len(r.vals) && r.vals[i] != nil {
					vals[j] = fmt.Sprint(r.vals[i])
				}
			}
			out.IDs = append(out.IDs, r.id)
			out.Rows = append(out.Rows, vals)
		}
		return out, nil
	}
	return workbook.DataRows{}, ErrPrivilege{"permission denied: donnée " + sqlName}
}

func (s *session) ListenData(ctx context.Context, channels []string, ready func(), fn func(channel, payload string)) error {
	s.f.mu.Lock()
	if err := s.begin("listen"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	type sub struct {
		ch string
		c  chan string
	}
	var subs []sub
	for _, ch := range channels {
		c := make(chan string, 16)
		s.f.listeners[ch] = append(s.f.listeners[ch], c)
		subs = append(subs, sub{ch, c})
	}
	s.f.mu.Unlock()
	defer func() {
		s.f.mu.Lock()
		for _, x := range subs {
			s.f.listeners[x.ch] = slices.DeleteFunc(s.f.listeners[x.ch], func(c chan string) bool { return c == x.c })
		}
		s.f.mu.Unlock()
	}()
	if ready != nil {
		ready()
	}
	merged := make(chan [2]string, 16)
	for _, x := range subs {
		go func() {
			for p := range x.c {
				merged <- [2]string{x.ch, p}
			}
		}()
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m := <-merged:
			fn(m[0], m[1])
		}
	}
}

// DataNotify envoie une notification sur un canal (lattis_data, canal d'un
// classeur).
func (f *Fake) DataNotify(channel, payload string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.listeners[channel] {
		select {
		case c <- payload:
		default:
		}
	}
}
