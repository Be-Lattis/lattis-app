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

// Données d'entreprise (contrat de lattisd, §9) côté application.
//
// Une donnée insérée dans un classeur est une requête Power Query sur une
// connexion « Serveur Lattis » (SELECT … FROM donnees."<sql_name>"), dont le
// champ Source.Dataset garde l'identifiant au catalogue : son tableau est en
// lecture seule (tableau d'une requête), formules, tableaux croisés et
// graphiques s'en servent comme d'un autre tableau, et hors ligne la
// dernière copie reste. Tant que le classeur est ouvert, watchData écoute le
// canal du classeur source et lattis_data : à chaque changement, la requête
// est actualisée (toute la donnée relue) ; un partage retiré garde la copie
// et le signale.

// DataCatalog lit le catalogue des données d'entreprise d'un serveur.
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

// InsertDataset insère une donnée d'entreprise comme tableau lié (Insérer ›
// Donnée d'entreprise) : connexion au serveur (créée au besoin), requête,
// premier chargement à la cellule donnée ; dans un classeur du même serveur,
// le lien est annoncé (« Utilisée par »).
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
		return QueryRefresh{}, errors.New("donnée d'entreprise introuvable ou indisponible")
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
	out, err := s.RefreshQuery(ctx, id)
	if err == nil && out.NeedPassword == nil && len(out.Errors) == 0 {
		if info := d.Share.Info(); info != nil && info.Server == server && info.CanDesign() {
			srv.LinkDataset(ctx, ds.ID, info.WorkbookID) // « Utilisée par » : sans effet bloquant
		}
		s.watchData(d)
	}
	return out, err
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
}

// linkedData : requêtes du classeur qui lisent une donnée d'entreprise, par
// serveur.
type linkedData struct {
	query   int64
	name    string
	dataset string
}

func (s *DocumentService) linkedDatasets(ctx context.Context, d *doc.Document) (map[string][]linkedData, error) {
	defs, err := d.Queries(ctx)
	if err != nil {
		return nil, err
	}
	conns, err := s.Connections(ctx)
	if err != nil {
		return nil, err
	}
	urls := map[string]string{}
	for _, c := range conns {
		if c.Kind == query.KindLattis {
			if u, err := auth.NormalizeURL(c.Config.URL); err == nil {
				urls[c.Name] = u
			}
		}
	}
	out := map[string][]linkedData{}
	for _, def := range defs {
		q, err := query.Decode(def)
		if err != nil || q.Source.Kind != query.SourceLattis || q.Source.Dataset == "" {
			continue
		}
		if u := urls[q.Source.Connection]; u != "" {
			out[u] = append(out[u], linkedData{query: q.ID, name: q.Name, dataset: q.Source.Dataset})
		}
	}
	return out, nil
}

// dataRetry : nouvel essai après une coupure du suivi.
var dataRetry = 30 * time.Second

// watchData (re)lance le suivi des données d'entreprise du classeur d.
func (s *DocumentService) watchData(d *doc.Document) {
	s.data.mu.Lock()
	if s.data.cancel != nil {
		s.data.cancel()
		s.data.cancel = nil
	}
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
	// Catalogue relu (republication, vue refaite) : nouveaux table_id.
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
	refresh := func(ls []linkedData) {
		if cur, err := s.current(); err != nil || cur != d {
			return
		}
		for _, l := range ls {
			s.RefreshQuery(ctx, l.query)
		}
	}
	// Une coupure a pu faire manquer des changements : tout relire en ligne.
	refresh(list)
	return srv.ListenData(ctx, channels, nil, func(channel, payload string) {
		if channel == protocol.DataChannel {
			switch {
			case strings.HasPrefix(payload, protocol.DataWithdrawn):
				id := strings.TrimPrefix(payload, protocol.DataWithdrawn)
				for _, l := range list {
					if l.dataset == id {
						d.Notify(fmt.Sprintf("Donnée d'entreprise « %s » : partage retiré par son propriétaire, la dernière copie reste", l.name))
					}
				}
			case payload == protocol.DataCatalogChanged:
			default:
				var hit []linkedData
				for _, l := range list {
					if l.dataset == payload {
						hit = append(hit, l)
					}
				}
				if len(hit) > 0 {
					reread()
				}
				refresh(hit)
			}
			return
		}
		if strings.HasPrefix(payload, protocol.DataVersion) {
			reread()
		}
		var hit []linkedData
		mu.Lock()
		defer mu.Unlock()
		for _, l := range bySource[channel] {
			ds := byID[l.dataset]
			if payload == ds.TableID || strings.HasPrefix(payload, protocol.DataVersion) {
				hit = append(hit, l)
			}
		}
		refresh(hit)
	})
}
