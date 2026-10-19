package bridge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/protocol"
)

// Données du Data Lake (contrat de lattisd, §9) côté application.
//
// Une donnée insérée dans un classeur est une requête Power Query sur une
// connexion « Serveur Lattis » (SELECT … FROM donnees."<sql_name>"), dont le
// champ Source.Dataset garde l'identifiant au catalogue : son tableau est en
// lecture seule (tableau d'une requête), formules, tableaux croisés et
// graphiques s'en servent comme d'un autre tableau, et hors ligne la
// dernière copie reste. Tant que le classeur est ouvert, watchData écoute le
// canal du classeur source et lattis_data : à chaque changement, seules les
// lignes changées ou supprimées depuis le dernier relevé sont lues et
// appliquées, hors du journal d'annulation (data_sync.go) ; un partage
// retiré garde la copie et le signale. Un classeur d'un serveur annonce
// les données qu'il utilise (« Utilisée par ») et retire l'annonce quand le
// tableau lié ou sa requête est supprimé (dataChanged).

// DataCatalog lit le catalogue du Data Lake Lattis d'un serveur.
func (s *DocumentService) DataCatalog(ctx context.Context, server string) (protocol.DataCatalog, error) {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return protocol.DataCatalog{}, err
	}
	c, err := srv.DataCatalog(ctx)
	if c.Folders == nil {
		c.Folders = []protocol.DataFolder{}
	}
	if c.Datasets == nil {
		c.Datasets = []protocol.Dataset{}
	}
	return c, err
}

// DataInsert est une donnée à insérer à la cellule (Row, Col) d'une feuille.
type DataInsert struct {
	Server  string `json:"server"`
	Dataset string `json:"dataset"`
	Sheet   int64  `json:"sheet"`
	Row     int    `json:"row"`
	Col     int    `json:"col"`
}

// InsertDataset insère une donnée du Data Lake comme tableau lié (Insérer ›
// Donnée du Data Lake) : connexion au serveur (créée au besoin), requête,
// premier chargement à la cellule donnée ; dans un classeur du même serveur,
// le lien est annoncé (« Utilisée par », voir dataChanged).
func (s *DocumentService) InsertDataset(ctx context.Context, in DataInsert) (QueryRefresh, error) {
	d, err := s.current()
	if err != nil {
		return QueryRefresh{}, err
	}
	server, err := auth.NormalizeURL(in.Server)
	if err != nil {
		return QueryRefresh{}, err
	}
	srv, _, name, err := remoteServer(ctx, server)
	if err != nil {
		return QueryRefresh{}, err
	}
	cat, err := srv.DataCatalog(ctx)
	if err != nil {
		return QueryRefresh{}, err
	}
	var ds *protocol.Dataset
	for i := range cat.Datasets {
		if cat.Datasets[i].ID == in.Dataset {
			ds = &cat.Datasets[i]
		}
	}
	if ds == nil || !ds.Readable || !ds.Available {
		return QueryRefresh{}, errors.New("donnée du Data Lake introuvable ou indisponible")
	}
	conn, err := s.lattisConnection(ctx, d, server, name)
	if err != nil {
		return QueryRefresh{}, err
	}
	qname, err := s.NewQueryName(ctx, ds.Name)
	if err != nil {
		return QueryRefresh{}, err
	}
	id, err := s.SaveQuery(ctx, query.Query{
		Name: qname,
		Source: query.Source{
			Kind: query.SourceLattis, Connection: conn, SQL: datasetSQL(*ds), Dataset: ds.ID,
			Place: &canvas.QueryAnchor{Sheet: in.Sheet, Row: in.Row, Col: in.Col},
		},
		Steps: []query.Step{},
	})
	if err != nil {
		return QueryRefresh{}, err
	}
	// Chargé : changed (dans RefreshQuery) annonce le lien au serveur du
	// classeur (« Utilisée par ») et relance le suivi (dataChanged).
	return s.RefreshQuery(ctx, id)
}

// datasetSQL : la requête d'une donnée, colonnes exposées dans l'ordre.
func datasetSQL(ds protocol.Dataset) string {
	cols := make([]string, 0, len(ds.Columns))
	for _, c := range ds.Columns {
		if strings.HasPrefix(c.Name, "lattis_") {
			continue
		}
		cols = append(cols, pgx.Identifier{c.Name}.Sanitize())
	}
	list := "*"
	if len(cols) > 0 {
		list = strings.Join(cols, ", ")
	}
	return "SELECT " + list + " FROM " + protocol.DataSchema + "." + pgx.Identifier{ds.SQLName}.Sanitize() + " ORDER BY lattis_pos"
}

// lattisConnection rend le nom de la connexion du classeur à ce serveur,
// créée au besoin.
func (s *DocumentService) lattisConnection(ctx context.Context, d *doc.Document, server, label string) (string, error) {
	conns, err := s.Connections(ctx)
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, c := range conns {
		taken[strings.ToLower(c.Name)] = true
		if c.Kind == query.KindLattis {
			if u, err := auth.NormalizeURL(c.Config.URL); err == nil && u == server {
				return c.Name, nil
			}
		}
	}
	name := strings.TrimSpace(label)
	if name == "" {
		name = auth.Host(server)
	}
	base := name
	for n := 2; taken[strings.ToLower(name)]; n++ {
		name = fmt.Sprintf("%s %d", base, n)
	}
	if _, err := s.SaveConnection(ctx, query.Connection{Name: name, Kind: query.KindLattis, Config: query.DBConfig{URL: server}}, "", false); err != nil {
		return "", err
	}
	return name, nil
}

// --- Suivi en direct --------------------------------------------------------------

type dataWatch struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	// doc, used : classeur suivi et données qu'il utilise (serveur et
	// donnée, voir dataUses), pour annoncer les liens qui changent.
	doc  *doc.Document
	used map[dataUse]bool
}

// dataUse : une donnée d'un serveur utilisée par le classeur.
type dataUse struct{ server, dataset string }

// linkedData : requêtes du classeur qui lisent une donnée du Data Lake, par
// serveur.
type linkedData struct {
	query   int64
	name    string
	dataset string
}

// linkedDatasets : requêtes chargées (tableau présent) qui lisent une donnée
// d'entreprise, par serveur. Une requête dont le tableau est supprimé n'est
// plus suivie (et son lien est retiré, voir dataChanged).
func (s *DocumentService) linkedDatasets(ctx context.Context, d *doc.Document) (map[string][]linkedData, error) {
	defs, err := d.Queries(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string][]linkedData{}
	var qs []query.Query
	for _, def := range defs {
		if def.Table == "" || !strings.Contains(def.Source, `"dataset"`) {
			continue
		}
		q, err := query.Decode(def)
		if err != nil || q.Source.Kind != query.SourceLattis || q.Source.Dataset == "" {
			continue
		}
		qs = append(qs, q)
	}
	if len(qs) == 0 {
		return out, nil
	}
	// Connexions lues sans le trousseau (Connections) : appelé à chaque
	// modification.
	conns, err := d.Connections(ctx)
	if err != nil {
		return nil, err
	}
	urls := map[string]string{}
	for _, def := range conns {
		c, err := query.DecodeConnection(def)
		if err != nil || c.Kind != query.KindLattis {
			continue
		}
		if u, err := auth.NormalizeURL(c.Config.URL); err == nil {
			urls[strings.ToLower(c.Name)] = u
		}
	}
	for _, q := range qs {
		if u := urls[strings.ToLower(q.Source.Connection)]; u != "" {
			out[u] = append(out[u], linkedData{query: q.ID, name: q.Name, dataset: q.Source.Dataset})
		}
	}
	return out, nil
}

// dataUses : données utilisées par le classeur d (requêtes chargées) ; nil
// si le classeur ne se lit pas.
func (s *DocumentService) dataUses(ctx context.Context, d *doc.Document) map[dataUse]bool {
	links, err := s.linkedDatasets(ctx, d)
	if err != nil {
		return nil
	}
	out := map[dataUse]bool{}
	for server, list := range links {
		for _, l := range list {
			out[dataUse{server, l.dataset}] = true
		}
	}
	return out
}

// dataChanged suit, après chaque modification du classeur d, les données
// qu'il utilise : une donnée insérée, ou dont le tableau lié ou la requête
// est supprimé (ou rétabli par Annuler), relance le suivi et, dans un
// classeur d'un serveur qu'on peut modifier, est annoncée à ce serveur ou
// retirée (link_dataset, unlink_dataset : « Utilisée par »).
func (s *DocumentService) dataChanged(d *doc.Document) {
	if d == nil {
		return
	}
	ctx := context.Background()
	s.data.mu.Lock()
	known := s.data.doc == d
	s.data.mu.Unlock()
	if !known {
		return
	}
	now := s.dataUses(ctx, d)
	if now == nil {
		return // lecture impossible : rien ne change
	}
	s.data.mu.Lock()
	if s.data.doc != d {
		s.data.mu.Unlock()
		return
	}
	before := s.data.used
	same := len(before) == len(now)
	for u := range now {
		same = same && before[u]
	}
	s.data.used = now
	s.data.mu.Unlock()
	if same {
		return
	}
	s.watchData(d)
	info := d.Share.Info()
	if info == nil || !info.CanDesign() {
		return
	}
	var link, unlink []string
	for u := range now {
		if !before[u] && u.server == info.Server {
			link = append(link, u.dataset)
		}
	}
	for u := range before {
		if !now[u] && u.server == info.Server {
			unlink = append(unlink, u.dataset)
		}
	}
	if len(link)+len(unlink) == 0 {
		return
	}
	go s.announceData(info.Server, info.WorkbookID, link, unlink)
}

// announceData annonce au serveur les données que le classeur workbookID
// utilise désormais (link) ou plus (unlink). Sans effet bloquant :
// « Utilisée par » est une information.
func (s *DocumentService) announceData(server, workbookID string, link, unlink []string) {
	ctx := context.Background()
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return
	}
	for _, id := range link {
		srv.LinkDataset(ctx, id, workbookID)
	}
	for _, id := range unlink {
		srv.UnlinkDataset(ctx, id, workbookID)
	}
}

// dataRetry : nouvel essai après une coupure du suivi.
var dataRetry = 30 * time.Second

// watchData (re)lance le suivi des données du Data Lake du classeur d.
func (s *DocumentService) watchData(d *doc.Document) {
	var used map[dataUse]bool
	if d != nil {
		used = s.dataUses(context.Background(), d)
	}
	s.data.mu.Lock()
	if s.data.cancel != nil {
		s.data.cancel()
		s.data.cancel = nil
	}
	if s.data.doc != d || used != nil {
		s.data.used = used
	}
	s.data.doc = d
	if d == nil {
		s.data.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.data.cancel = cancel
	s.data.mu.Unlock()
	go func() {
		links, err := s.linkedDatasets(ctx, d)
		if err != nil || len(links) == 0 {
			return
		}
		for server, list := range links {
			go s.followData(ctx, d, server, list)
		}
	}()
}

// followData écoute un serveur pour les données liées list, tant que ctx
// court et que d reste le classeur ouvert.
func (s *DocumentService) followData(ctx context.Context, d *doc.Document, server string, list []linkedData) {
	for ctx.Err() == nil {
		err := s.followDataOnce(ctx, d, server, list)
		if ctx.Err() != nil {
			return
		}
		_ = err // hors ligne : la dernière copie reste ; nouvel essai plus tard
		select {
		case <-ctx.Done():
			return
		case <-time.After(dataRetry):
		}
	}
}

func (s *DocumentService) followDataOnce(ctx context.Context, d *doc.Document, server string, list []linkedData) error {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return err
	}
	cat, err := srv.DataCatalog(ctx)
	if err != nil {
		return err
	}
	var mu sync.Mutex
	byID := map[string]protocol.Dataset{}
	for _, ds := range cat.Datasets {
		byID[ds.ID] = ds
	}
	// Catalogue relu (republication, vue refaite) : nouveaux table_id,
	// colonnes.
	reread := func() {
		if c, err := srv.DataCatalog(ctx); err == nil {
			mu.Lock()
			for _, ds := range c.Datasets {
				byID[ds.ID] = ds
			}
			mu.Unlock()
		}
	}
	// Canal du classeur source → données liées (charge : table_id ou *version).
	channels := []string{protocol.DataChannel}
	bySource := map[string][]linkedData{}
	for _, l := range list {
		ds, ok := byID[l.dataset]
		if !ok || ds.Channel == "" {
			continue
		}
		if len(bySource[ds.Channel]) == 0 {
			channels = append(channels, ds.Channel)
		}
		bySource[ds.Channel] = append(bySource[ds.Channel], l)
	}
	// relève les données ls ; full : tout relire (vue refaite, republication).
	relieve := func(ls []linkedData, full bool) {
		for _, l := range ls {
			mu.Lock()
			ds, ok := byID[l.dataset]
			mu.Unlock()
			if ok {
				s.syncDataset(ctx, d, srv, l, ds, full)
			}
		}
	}
	// Une coupure a pu faire manquer des changements : relevé depuis le
	// dernier point de reprise (tout relire s'il n'est plus couvert).
	relieve(list, false)
	return srv.ListenData(ctx, channels, nil, func(channel, payload string) {
		if channel == protocol.DataChannel {
			switch {
			case strings.HasPrefix(payload, protocol.DataWithdrawn):
				id := strings.TrimPrefix(payload, protocol.DataWithdrawn)
				for _, l := range list {
					if l.dataset == id {
						d.Notify(fmt.Sprintf("Data Lake Lattis · « %s » : partage retiré par son propriétaire, la dernière copie reste", l.name))
					}
				}
			case payload == protocol.DataCatalogChanged:
			default:
				// Vue refaite : colonnes ou audience ont pu changer.
				var hit []linkedData
				for _, l := range list {
					if l.dataset == payload {
						hit = append(hit, l)
					}
				}
				if len(hit) > 0 {
					reread()
				}
				relieve(hit, true)
			}
			return
		}
		version := strings.HasPrefix(payload, protocol.DataVersion)
		if version {
			reread()
		}
		var hit []linkedData
		mu.Lock()
		for _, l := range bySource[channel] {
			if version || payload == byID[l.dataset].TableID {
				hit = append(hit, l)
			}
		}
		mu.Unlock()
		relieve(hit, version)
	})
}
