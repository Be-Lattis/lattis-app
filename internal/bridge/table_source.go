package bridge

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/protocol"
)

// Clic droit sur un tableau › Partager au Data Lake Lattis… › « Publier
// uniquement ce tableau » : le tableau devient une source poussée du Data
// Lake (contrat de lattisd §9.1, migration 0027), liée au classeur, sans
// publier le classeur. lattisd crée un classeur technique qui ne contient
// que ce tableau, un pipeline « push » et la donnée du catalogue ; le
// poste envoie le contenu des colonnes choisies en CSV (POST
// /api/push/<pipeline>), à la demande (« Mettre à jour la source du Data
// Lake ») et à chaque enregistrement si demandé. Avant chaque envoi, les
// colonnes renommées, converties ou supprimées dans le classeur le sont
// aussi sur le serveur (fonctions de structure). Le reste du classeur
// (autres tableaux, feuilles, formules, scripts, colonnes non cochées) ne
// quitte pas le poste. Le lien est gardé dans le classeur (_lattis_info,
// clé table_sources). Voir docs/collaboration/pipeline-table-source.md.

// tableSourcesKey : clé des liens dans _lattis_info.
const tableSourcesKey = "table_sources"

// TableSourceEvent est émis après un envoi fait à l'enregistrement (charge
// TableSourceNotice) : la barre d'état le dit.
const TableSourceEvent = "data:source"

// TableSourceNotice est le résultat d'un envoi fait en arrière-plan.
type TableSourceNotice struct {
	Table   string `json:"table"`
	Message string `json:"message"`
	Failed  bool   `json:"failed"`
}

// SourceColumn est une colonne envoyée : colonne locale (stable au
// renommage), nom et type du serveur tels que le serveur les a.
type SourceColumn struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// TableSourceLink est le lien d'un tableau du classeur avec sa source
// poussée.
type TableSourceLink struct {
	// Tbl : tableau local (_lattis_tables.id, stable au renommage).
	Tbl int64 `json:"tbl"`
	// TableName : nom actuel du tableau ("" : tableau supprimé), relu à
	// chaque lecture.
	TableName string `json:"tableName"`
	Server    string `json:"server"`
	Pipeline  string `json:"pipeline"`
	Dataset   string `json:"dataset"`
	// WorkbookID, TableID : classeur technique et tableau sur le serveur
	// (fonctions de structure).
	WorkbookID string `json:"workbookId"`
	TableID    string `json:"tableId"`
	SQLName    string `json:"sqlName"`
	Name       string `json:"name"`
	// Mode : replace, append ou upsert (Keys) ; fixé à la création.
	Mode string         `json:"mode"`
	Cols []SourceColumn `json:"cols"`
	// Columns : noms des colonnes envoyées (Cols), pour l'interface.
	Columns []string `json:"columns"`
	Keys    []string `json:"keys"`
	// OnSave : envoyer à chaque enregistrement du classeur.
	OnSave bool `json:"onSave"`
	// Suspended : source suspendue (enabled = false) : rien n'est envoyé.
	Suspended bool `json:"suspended,omitempty"`
	// LastPush : date du dernier envoi écrit (RFC 3339) ; LastResult son
	// bilan ; LastError : échec du dernier essai ("" : réussi).
	LastPush   string            `json:"lastPush,omitempty"`
	LastResult *workbook.PushRun `json:"lastResult,omitempty"`
	LastError  string            `json:"lastError,omitempty"`
	// Blanked : valeurs envoyées vides au dernier envoi (saisie hors type,
	// erreur d'une formule).
	Blanked int `json:"blanked,omitempty"`
}

type tableSourcesDoc struct {
	Version int               `json:"version"`
	Links   []TableSourceLink `json:"links"`
}

// sourcesMu sérialise la lecture-modification-écriture des liens.
var sourcesMu sync.Mutex

// readSources lit les liens du classeur, avec le nom actuel des tableaux.
func readSources(ctx context.Context, d *doc.Document) ([]TableSourceLink, error) {
	var raw string
	err := d.DB().Read().QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, tableSourcesKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return []TableSourceLink{}, nil
	}
	if err != nil {
		return nil, err
	}
	var v tableSourcesDoc
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, msg.Wrap(err, "lake.sourcesUnreadable", msg.Args{"err": err})
	}
	out := v.Links
	if out == nil {
		out = []TableSourceLink{}
	}
	for i := range out {
		out[i].TableName = ""
		_ = d.DB().Read().QueryRowContext(ctx, `SELECT name FROM _lattis_tables WHERE id = ?`, out[i].Tbl).Scan(&out[i].TableName)
		out[i].Columns = []string{}
		for _, c := range out[i].Cols {
			out[i].Columns = append(out[i].Columns, c.Name)
		}
	}
	return out, nil
}

// writeSources range les liens ; mark : le classeur est à enregistrer
// (lien créé ou retiré, réglage changé), pas pour un bilan d'envoi.
func writeSources(ctx context.Context, d *doc.Document, links []TableSourceLink, mark bool) error {
	err := d.DB().Write(ctx, func(tx *sql.Tx) error {
		if len(links) == 0 {
			_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key = ?`, tableSourcesKey)
			return err
		}
		b, err := json.Marshal(tableSourcesDoc{Version: 2, Links: links})
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, tableSourcesKey, string(b))
		return err
	})
	if err == nil && mark {
		d.MarkChanged()
	}
	return err
}

// updateSource applique fn au lien du tableau tbl et le range.
func updateSource(ctx context.Context, d *doc.Document, tbl int64, mark bool, fn func(*TableSourceLink)) (TableSourceLink, error) {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	links, err := readSources(ctx, d)
	if err != nil {
		return TableSourceLink{}, err
	}
	i := slices.IndexFunc(links, func(l TableSourceLink) bool { return l.Tbl == tbl })
	if i < 0 {
		return TableSourceLink{}, msg.New("lake.noLongerSource", nil)
	}
	fn(&links[i])
	links[i].Columns = []string{}
	for _, c := range links[i].Cols {
		links[i].Columns = append(links[i].Columns, c.Name)
	}
	return links[i], writeSources(ctx, d, links, mark)
}

// sourceOf rend le lien du tableau tbl (nil : aucun).
func sourceOf(ctx context.Context, d *doc.Document, tbl int64) (*TableSourceLink, error) {
	links, err := readSources(ctx, d)
	if err != nil {
		return nil, err
	}
	for _, l := range links {
		if l.Tbl == tbl {
			return &l, nil
		}
	}
	return nil, nil
}

// tableID rend l'identifiant local du tableau nommé.
func tableID(ctx context.Context, d *doc.Document, table string) (int64, error) {
	var tbl int64
	if err := d.DB().Read().QueryRowContext(ctx, `SELECT id FROM _lattis_tables WHERE name = ?`, table).Scan(&tbl); err != nil {
		return 0, msg.New("table.notFound", msg.Args{"table": table})
	}
	return tbl, nil
}

// localColumn est une colonne du tableau local.
type localColumn struct {
	ID   int64
	Name string
	Type string // type du serveur (RemoteType)
}

// localColumns lit les colonnes du tableau tbl, dans l'ordre.
func localColumns(ctx context.Context, d *doc.Document, tbl int64) ([]localColumn, error) {
	rs, err := d.DB().Read().QueryContext(ctx, `SELECT id, name, type FROM _lattis_columns WHERE tbl = ? ORDER BY pos`, tbl)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []localColumn
	for rs.Next() {
		var c localColumn
		var typ string
		if err := rs.Scan(&c.ID, &c.Name, &typ); err != nil {
			return nil, err
		}
		c.Type = workbook.RemoteType(grid.Type(typ))
		out = append(out, c)
	}
	return out, rs.Err()
}

// errPushUnsupported : serveur sans les sources poussées (migration 0027).
var errPushUnsupported = msg.New("lake.pushUnsupported", nil)

// unsupportedMessage : le texte français de errPushUnsupported.
var unsupportedMessage = errPushUnsupported.Error()

// sourceError rend une erreur du serveur lisible.
func sourceError(err error) error {
	if err == nil {
		return nil
	}
	if workbook.Unsupported(err) {
		return errPushUnsupported
	}
	if _, ok := workbook.Rejected(err); ok {
		return errors.New(workbook.ServerMessage(err))
	}
	return err
}

// sharePusher remplace l'envoi HTTP (essais).
var sharePusher func(ctx context.Context, server string) (workbook.Pusher, error)

// sourcePoll : intervalle de relecture de l'état d'un envoi encore en cours (202).
var sourcePoll = 2 * time.Second

// remotePusher ouvre l'envoi HTTP vers l'émetteur du serveur (issuer de la
// découverte), avec le jeton d'accès de la session PostgreSQL.
func remotePusher(ctx context.Context, server string) (workbook.Pusher, error) {
	if sharePusher != nil {
		return sharePusher(ctx, server)
	}
	m, err := sessions()
	if err != nil {
		return nil, err
	}
	_, disc, err := m.Session(ctx, server)
	if err != nil {
		return nil, err
	}
	issuer := disc.Issuer
	if issuer == "" {
		issuer = server
	}
	return workbook.PushClient{
		Issuer:     issuer,
		Token:      func(ctx context.Context) (string, error) { return m.Token(ctx, server) },
		Invalidate: func() { m.Invalidate(server) },
	}, nil
}

// TableSources liste les sources du Data Lake liées au classeur ouvert.
func (s *DocumentService) TableSources(ctx context.Context) ([]TableSourceLink, error) {
	d, err := s.current()
	if err != nil {
		return []TableSourceLink{}, nil
	}
	return readSources(ctx, d)
}

// PushedSources liste les sources poussées de l'utilisateur sur un serveur
// (lattis.my_pushed_sources), quel que soit le classeur d'où elles
// viennent : vide pour un serveur sans sources poussées.
func (s *DocumentService) PushedSources(ctx context.Context, server string) ([]workbook.PushedSource, error) {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return []workbook.PushedSource{}, err
	}
	out, err := srv.PushedSources(ctx)
	if workbook.Unsupported(err) {
		return []workbook.PushedSource{}, nil
	}
	if out == nil {
		out = []workbook.PushedSource{}
	}
	return out, sourceError(err)
}

// TableSourceInfo décrit de quoi publier un tableau seul (ou gérer sa
// source) sur un serveur.
type TableSourceInfo struct {
	// Reason : pourquoi c'est impossible sur ce serveur ("" : possible).
	Reason string `json:"reason"`
	Server string `json:"server"`
	Table  string `json:"table"`
	// Fields : colonnes du tableau et leur type local.
	Fields []RightsColumn `json:"fields"`
	// Keys : clé proposée (colonne clé du tableau, s'il en a une).
	Keys    []string             `json:"keys"`
	Catalog protocol.DataCatalog `json:"catalog"`
	Teams   []DataTeam           `json:"teams"`
	// Link : la source existante (nil : à créer) ; Dataset : sa donnée au
	// catalogue (nil : retirée ou illisible).
	Link    *TableSourceLink  `json:"link"`
	Dataset *protocol.Dataset `json:"dataset"`
}

// TableSourceInfo lit de quoi publier le tableau table seul sur server
// (ignoré si le tableau a déjà une source : son serveur).
func (s *DocumentService) TableSourceInfo(ctx context.Context, table, server string) (TableSourceInfo, error) {
	out := TableSourceInfo{Table: table, Fields: []RightsColumn{}, Keys: []string{}, Teams: []DataTeam{},
		Catalog: protocol.DataCatalog{Folders: []protocol.DataFolder{}, Datasets: []protocol.Dataset{}}}
	d, err := s.current()
	if err != nil {
		return out, err
	}
	tbl, err := tableID(ctx, d, table)
	if err != nil {
		return out, err
	}
	if out.Link, err = sourceOf(ctx, d, tbl); err != nil {
		return out, err
	}
	if out.Link != nil {
		server = out.Link.Server
	}
	if rs, err := d.DB().Read().QueryContext(ctx, `SELECT name, type FROM _lattis_columns WHERE tbl = ? ORDER BY pos`, tbl); err == nil {
		for rs.Next() {
			var c RightsColumn
			if rs.Scan(&c.Name, &c.Type) == nil {
				out.Fields = append(out.Fields, c)
			}
		}
		rs.Close()
	}
	var key string
	if d.DB().Read().QueryRowContext(ctx, `SELECT c.name FROM _lattis_tables t JOIN _lattis_columns c ON c.id = t.key_col WHERE t.id = ?`, tbl).Scan(&key) == nil {
		out.Keys = []string{key}
	}
	out.Server = server
	// Classeur partagé : ses données ne se republient pas par un autre
	// chemin que ses droits (lecteurs, éditeurs), et un tableau publié se
	// partage directement.
	if info := d.Share.Info(); info != nil && out.Link == nil {
		var published int
		_ = d.DB().Read().QueryRowContext(ctx, `SELECT count(*) FROM _lattis_remote_tables WHERE tbl = ? AND published = 1`, tbl).Scan(&published)
		switch {
		case !info.CanDesign():
			out.Reason = s.say("lake.shareOwnersDesigners", nil)
			return out, nil
		case published > 0:
			out.Reason = s.say("lake.publishedWithWorkbook", nil)
			return out, nil
		}
	}
	if server == "" {
		out.Reason = s.say("lake.connectFirst", nil)
		return out, nil
	}
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return out, err
	}
	ok, err := srv.PushedSourcesSupported(ctx)
	if err != nil {
		return out, err
	}
	if !ok {
		out.Reason = s.say("lake.pushUnsupported", nil)
		return out, nil
	}
	if out.Catalog, err = s.DataCatalog(ctx, server); err != nil {
		return out, err
	}
	if !out.Catalog.CanPublish {
		out.Reason = s.say("lake.notPublisher", nil)
		return out, nil
	}
	if out.Link != nil {
		for i := range out.Catalog.Datasets {
			if out.Catalog.Datasets[i].ID == out.Link.Dataset {
				out.Dataset = &out.Catalog.Datasets[i]
			}
		}
	}
	if teams, err := srv.Teams(ctx); err == nil {
		for _, t := range teams {
			out.Teams = append(out.Teams, DataTeam{ID: t.ID, Name: t.Name})
		}
	}
	return out, nil
}

// TableSourceRequest crée ou modifie la source d'un tableau.
type TableSourceRequest struct {
	Server      string `json:"server"`
	Table       string `json:"table"`
	FolderID    string `json:"folderId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SQLName     string `json:"sqlName"`
	// Columns : colonnes envoyées ; Mode : replace (défaut), append ou
	// upsert (Keys : colonnes qui identifient une ligne).
	Columns []string `json:"columns"`
	Mode    string   `json:"mode"`
	Keys    []string `json:"keys"`
	OnSave  bool     `json:"onSave"`
	Inherit bool     `json:"inherit"`
	// Suspended : suspendre la source (modification seulement).
	Suspended bool `json:"suspended"`
	// Add : entrées d'audience à ajouter ; Remove : identifiants à retirer.
	Add    []DataAudienceEntry `json:"add"`
	Remove []string            `json:"remove"`
}

// sourceColumns vérifie les colonnes demandées (rendues dans l'ordre du
// tableau), le mode et la clé.
func sourceColumns(fields []localColumn, cols []string, mode string, keys []string) ([]localColumn, string, []string, error) {
	var out []localColumn
	for _, f := range fields {
		if slices.Contains(cols, f.Name) {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return nil, "", nil, msg.New("lake.chooseColumn", nil)
	}
	for _, c := range cols {
		if !slices.ContainsFunc(out, func(f localColumn) bool { return f.Name == c }) {
			return nil, "", nil, msg.New("lake.unknownColumn", msg.Args{"column": c})
		}
	}
	mode = cmp.Or(mode, workbook.PushReplace)
	switch mode {
	case workbook.PushReplace, workbook.PushAppend:
		return out, mode, []string{}, nil
	case workbook.PushUpsert:
	default:
		return nil, "", nil, msg.New("lake.unknownMode", msg.Args{"mode": mode})
	}
	if len(keys) == 0 {
		return nil, "", nil, msg.New("lake.keyNeeded", nil)
	}
	for _, c := range keys {
		if !slices.ContainsFunc(out, func(f localColumn) bool { return f.Name == c }) {
			return nil, "", nil, msg.New("lake.keyNotSent", msg.Args{"column": c})
		}
	}
	return out, mode, slices.Clone(keys), nil
}

// docTitle : titre du classeur (nom du fichier), pour l'origine de la source.
func (s *DocumentService) docTitle(ctx context.Context) string {
	st, err := s.State(ctx)
	if err != nil || st.Title == "" {
		return "Classeur"
	}
	return strings.TrimSuffix(st.Title, " (lecture seule)")
}

func docID(ctx context.Context, d *doc.Document) string {
	var id string
	_ = d.DB().Read().QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = 'id'`).Scan(&id)
	return id
}

// CreateTableSource publie le tableau seul comme source poussée : classeur
// technique, pipeline et donnée créés sur le serveur, lien gardé dans le
// classeur, puis premier envoi. Le classeur n'est pas publié. Un premier
// envoi refusé laisse la source créée (LastError dit pourquoi).
func (s *DocumentService) CreateTableSource(ctx context.Context, req TableSourceRequest) (TableSourceLink, error) {
	d, err := s.current()
	if err != nil {
		return TableSourceLink{}, err
	}
	info, err := s.TableSourceInfo(ctx, req.Table, req.Server)
	if err != nil {
		return TableSourceLink{}, err
	}
	if info.Link != nil {
		return TableSourceLink{}, msg.New("lake.alreadySource", nil)
	}
	if info.Reason != "" {
		return TableSourceLink{}, errors.New(info.Reason)
	}
	tbl, err := tableID(ctx, d, req.Table)
	if err != nil {
		return TableSourceLink{}, err
	}
	all, err := localColumns(ctx, d, tbl)
	if err != nil {
		return TableSourceLink{}, err
	}
	cols, mode, keys, err := sourceColumns(all, req.Columns, req.Mode, req.Keys)
	if err != nil {
		return TableSourceLink{}, err
	}
	srv, _, _, err := remoteServer(ctx, req.Server)
	if err != nil {
		return TableSourceLink{}, err
	}
	specs := make([]workbook.ColumnSpec, len(cols))
	scols := make([]SourceColumn, len(cols))
	for i, c := range cols {
		specs[i] = workbook.ColumnSpec{Name: c.Name, Type: c.Type}
		scols[i] = SourceColumn(c)
	}
	name := strings.TrimSpace(req.Name)
	created, err := srv.CreatePushedSource(ctx, workbook.PushedSourceSpec{Name: name, Description: strings.TrimSpace(req.Description),
		FolderID: req.FolderID, SQLName: strings.TrimSpace(req.SQLName), TableName: req.Table, Columns: specs, Mode: mode, KeyColumns: keys,
		Origin: workbook.PushedOrigin{Workbook: s.docTitle(ctx), WorkbookLocalID: docID(ctx, d), Table: req.Table, TableLocalID: strconv.FormatInt(tbl, 10)}})
	if err != nil {
		return TableSourceLink{}, sourceError(err)
	}
	link := TableSourceLink{Tbl: tbl, TableName: req.Table, Server: req.Server, Pipeline: created.PipelineID, Dataset: created.DatasetID,
		WorkbookID: created.WorkbookID, TableID: created.TableID, SQLName: created.SQLName, Name: name, Mode: mode, Cols: scols, Keys: keys, OnSave: req.OnSave}
	sourcesMu.Lock()
	links, err := readSources(ctx, d)
	if err == nil {
		err = writeSources(ctx, d, append(links, link), true)
	}
	sourcesMu.Unlock()
	if err != nil {
		return link, err
	}
	if err := sourceAudience(ctx, srv, created.DatasetID, req, true); err != nil {
		return link, err
	}
	// Créée : un premier envoi refusé est dit par LastError.
	pushed, _ := s.pushSource(ctx, d, tbl, nil)
	return pushed, nil
}

// sourceAudience règle l'audience de la donnée (comme ShareDataset).
func sourceAudience(ctx context.Context, srv workbook.Server, dataset string, req TableSourceRequest, created bool) error {
	if created && !req.Inherit {
		inherit := false
		if err := srv.UpdateDataset(ctx, dataset, workbook.DatasetChange{InheritAudience: &inherit}); err != nil {
			return sourceError(err)
		}
	}
	for _, a := range req.Remove {
		if err := srv.RemoveDataAudience(ctx, a); err != nil {
			return sourceError(err)
		}
	}
	var refused []string
	for _, a := range req.Add {
		var err error
		switch {
		case a.Team != "":
			_, err = srv.AddDataAudience(ctx, dataset, "", a.Team, a.Predicate)
		case strings.TrimSpace(a.Email) != "":
			_, err = srv.AddDataAudienceByEmail(ctx, dataset, "", a.Email, a.Predicate)
		}
		if err != nil {
			refused = append(refused, remoteMessage(err))
		}
	}
	if len(refused) > 0 {
		return msg.New("lake.audienceRefused", msg.Args{"refused": strings.Join(refused, " ; ")})
	}
	return nil
}

// UpdateTableSource modifie la source d'un tableau (nom, description,
// dossier, audience ; colonnes envoyées, ajoutées ou retirées sur le
// serveur ; mode, clé, suspension ; envoi à l'enregistrement), puis
// envoie les lignes (sauf suspendue). Un serveur sans la migration 0028
// refuse le changement de mode, de clé ou la suspension.
func (s *DocumentService) UpdateTableSource(ctx context.Context, req TableSourceRequest) (TableSourceLink, error) {
	d, err := s.current()
	if err != nil {
		return TableSourceLink{}, err
	}
	info, err := s.TableSourceInfo(ctx, req.Table, "")
	if err != nil {
		return TableSourceLink{}, err
	}
	link := info.Link
	if link == nil {
		return TableSourceLink{}, msg.New("lake.notSource", nil)
	}
	all, err := localColumns(ctx, d, link.Tbl)
	if err != nil {
		return *link, err
	}
	mode, keys := link.Mode, link.Keys
	if req.Mode != "" {
		mode = req.Mode
	}
	if req.Keys != nil && mode == workbook.PushUpsert {
		keys = req.Keys
	}
	cols, mode, keys, err := sourceColumns(all, req.Columns, mode, keys)
	if err != nil {
		return *link, err
	}
	srv, _, _, err := remoteServer(ctx, link.Server)
	if err != nil {
		return *link, err
	}
	want := make([]int64, len(cols))
	for i, c := range cols {
		want[i] = c.ID
	}
	// Mode, clé, suspension (update_pushed_source, migration 0028) : les
	// colonnes sont alignées d'abord, la clé doit exister sur le serveur.
	modeChanged := mode != cmp.Or(link.Mode, workbook.PushReplace) || !slices.Equal(keys, link.Keys)
	if modeChanged || req.Suspended != link.Suspended {
		if modeChanged {
			if _, err := alignSource(ctx, d, *link, want); err != nil {
				return *link, err
			}
		}
		change := workbook.PushedSourceChange{}
		if modeChanged {
			change.Mode, change.KeyColumns = &mode, &keys
		}
		if req.Suspended != link.Suspended {
			on := !req.Suspended
			change.Enabled = &on
		}
		if err := srv.UpdatePushedSource(ctx, link.Pipeline, change); err != nil {
			if workbook.Unsupported(err) {
				return *link, msg.New("lake.serverTooOld", nil)
			}
			return *link, sourceError(err)
		}
		if _, err := updateSource(ctx, d, link.Tbl, true, func(l *TableSourceLink) { l.Mode, l.Keys, l.Suspended = mode, keys, req.Suspended }); err != nil {
			return *link, err
		}
	}
	name := strings.TrimSpace(req.Name)
	desc := strings.TrimSpace(req.Description)
	folder := req.FolderID
	if err := srv.UpdateDataset(ctx, link.Dataset, workbook.DatasetChange{Name: &name, Description: &desc, FolderID: &folder,
		InheritAudience: &req.Inherit}); err != nil {
		return *link, sourceError(err)
	}
	if _, err := updateSource(ctx, d, link.Tbl, true, func(l *TableSourceLink) { l.Name, l.OnSave = name, req.OnSave }); err != nil {
		return *link, err
	}
	if err := sourceAudience(ctx, srv, link.Dataset, req, false); err != nil {
		return *link, err
	}
	if req.Suspended {
		l, err := sourceOf(ctx, d, link.Tbl)
		if err != nil || l == nil {
			return *link, err
		}
		return *l, nil
	}
	return s.pushSource(ctx, d, link.Tbl, want)
}

// SetTableSourceOnSave règle l'envoi à chaque enregistrement.
func (s *DocumentService) SetTableSourceOnSave(ctx context.Context, table string, on bool) (TableSourceLink, error) {
	d, err := s.current()
	if err != nil {
		return TableSourceLink{}, err
	}
	tbl, err := tableID(ctx, d, table)
	if err != nil {
		return TableSourceLink{}, err
	}
	l, err := updateSource(ctx, d, tbl, true, func(l *TableSourceLink) { l.OnSave = on })
	if err == nil {
		_, err = s.changed(ctx)
	}
	return l, err
}

// PushTableSource envoie maintenant les lignes du tableau à sa source
// (« Mettre à jour la source du Data Lake »).
func (s *DocumentService) PushTableSource(ctx context.Context, table string) (TableSourceLink, error) {
	d, err := s.current()
	if err != nil {
		return TableSourceLink{}, err
	}
	tbl, err := tableID(ctx, d, table)
	if err != nil {
		return TableSourceLink{}, err
	}
	return s.pushSource(ctx, d, tbl, nil)
}

// DropTableSource retire la source du tableau sur le serveur (la donnée
// disparaît du catalogue ; les classeurs qui l'utilisent gardent leur
// dernière copie) et oublie le lien. Une source déjà retirée du serveur
// est seulement oubliée.
func (s *DocumentService) DropTableSource(ctx context.Context, table string) error {
	d, err := s.current()
	if err != nil {
		return err
	}
	tbl, err := tableID(ctx, d, table)
	if err != nil {
		return err
	}
	return s.dropSource(ctx, d, tbl)
}

// DropTableSourceByID retire une source par son tableau local (liste des
// sources : aussi celle d'un tableau supprimé).
func (s *DocumentService) DropTableSourceByID(ctx context.Context, tbl int64) error {
	d, err := s.current()
	if err != nil {
		return err
	}
	return s.dropSource(ctx, d, tbl)
}

func (s *DocumentService) dropSource(ctx context.Context, d *doc.Document, tbl int64) error {
	link, err := sourceOf(ctx, d, tbl)
	if err != nil {
		return err
	}
	if link == nil {
		return msg.New("lake.notSource", nil)
	}
	srv, _, _, err := remoteServer(ctx, link.Server)
	if err != nil {
		return err
	}
	if err := srv.DropPushedSource(ctx, link.Pipeline); err != nil && !workbook.PushNotFound(err) {
		return sourceError(err)
	}
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	links, err := readSources(ctx, d)
	if err != nil {
		return err
	}
	links = slices.DeleteFunc(links, func(l TableSourceLink) bool { return l.Tbl == tbl })
	if err := writeSources(ctx, d, links, true); err != nil {
		return err
	}
	_, err = s.changed(ctx)
	return err
}

// syncColumns aligne le tableau de la source sur le classeur avant un
// envoi : une colonne envoyée renommée, convertie ou supprimée en local
// l'est aussi sur le serveur ; want (nil : les colonnes du lien encore
// présentes) donne les colonnes à envoyer, une colonne nouvelle est
// ajoutée. Le tout dans une transaction (refus du serveur : rien ne
// change, son message est rendu). Rend les colonnes à jour.
func syncColumns(ctx context.Context, srv workbook.Server, link TableSourceLink, local []localColumn, want []int64) ([]SourceColumn, error) {
	byID := map[int64]localColumn{}
	for _, c := range local {
		byID[c.ID] = c
	}
	if want == nil {
		for _, c := range link.Cols {
			if _, ok := byID[c.ID]; ok {
				want = append(want, c.ID)
			}
		}
	}
	type op struct {
		kind       string // drop, rename, type, add
		name, to   string
		typ        string
		localID    int64
		remoteName string
	}
	var ops []op
	next := []SourceColumn{}
	for _, c := range link.Cols {
		l, ok := byID[c.ID]
		if !ok || !slices.Contains(want, c.ID) {
			ops = append(ops, op{kind: "drop", remoteName: c.Name})
			continue
		}
		if l.Name != c.Name {
			ops = append(ops, op{kind: "rename", remoteName: c.Name, to: l.Name})
		}
		if l.Type != c.Type {
			ops = append(ops, op{kind: "type", remoteName: c.Name, typ: l.Type})
		}
		next = append(next, SourceColumn{ID: c.ID, Name: l.Name, Type: l.Type})
	}
	for _, id := range want {
		if !slices.ContainsFunc(link.Cols, func(c SourceColumn) bool { return c.ID == id }) {
			l := byID[id]
			ops = append(ops, op{kind: "add", name: l.Name, typ: l.Type, localID: id})
			next = append(next, SourceColumn(l))
		}
	}
	if len(next) == 0 {
		return nil, msg.New("lake.noColumnsLeft", nil)
	}
	if len(ops) == 0 {
		return link.Cols, nil
	}
	// Ajouts avant suppressions : le serveur garde au moins une colonne.
	order := map[string]int{"rename": 0, "type": 1, "add": 2, "drop": 3}
	slices.SortStableFunc(ops, func(a, b op) int { return order[a.kind] - order[b.kind] })
	caps, err := srv.Capabilities(ctx, link.WorkbookID)
	if err != nil {
		return nil, sourceError(err)
	}
	var remote *workbook.RemoteTable
	if err := srv.Delta(ctx, func(r workbook.Reader) error {
		ts, err := r.Tables(ctx, caps.Schema)
		for i := range ts {
			if ts[i].ID == link.TableID {
				remote = &ts[i]
			}
		}
		return err
	}); err != nil {
		return nil, sourceError(err)
	}
	if remote == nil {
		return nil, msg.New("lake.tableNotFoundOnServer", nil)
	}
	colID := func(name string) string {
		for _, c := range remote.Columns {
			if c.Name == name {
				return c.ID
			}
		}
		return ""
	}
	// Renommages en deux temps quand deux colonnes échangent leurs noms :
	// rare, refusé par le serveur comme nom pris ; le message le dit.
	err = srv.Write(ctx, func(w workbook.Writer) error {
		for _, o := range ops {
			var err error
			switch o.kind {
			case "drop":
				if id := colID(o.remoteName); id != "" {
					err = w.DropColumn(ctx, link.WorkbookID, link.TableID, id)
				}
			case "rename":
				err = w.RenameColumn(ctx, link.WorkbookID, link.TableID, colID(o.remoteName), o.to)
			case "type":
				err = w.AlterColumnType(ctx, link.WorkbookID, link.TableID, colID(o.remoteName), o.typ)
			case "add":
				_, err = w.AddColumn(ctx, link.WorkbookID, link.TableID, o.name, o.typ, "", o.localID)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, msg.Wrap(sourceError(err), "lake.columnsNotUpdated", msg.Args{"err": sourceError(err)})
	}
	return next, nil
}

// sourceCSV écrit les colonnes envoyées du tableau en CSV (séparateur ;,
// décimales avec un point, dates AAAA-MM-JJ, booléens true/false) ;
// blanked compte les valeurs envoyées vides (saisie hors type, erreur
// d'une formule, valeur inconvertible).
func sourceCSV(ctx context.Context, m *canvas.Model, table string, cols []SourceColumn) ([]byte, int, int, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	var idx []int
	rows, blanked := 0, 0
	err := m.ExportTable(ctx, table, func(h []canvas.ExportColumn) error {
		head := make([]string, len(cols))
		for i, c := range cols {
			j := slices.IndexFunc(h, func(x canvas.ExportColumn) bool { return x.Name == c.Name })
			if j < 0 {
				return msg.New("lake.columnGone", msg.Args{"column": c.Name})
			}
			idx = append(idx, j)
			head[i] = c.Name
		}
		return w.Write(head)
	}, func(r []any) error {
		rec := make([]string, len(idx))
		for k, j := range idx {
			v, ok := sourceValue(cols[k].Type, r[j])
			if !ok {
				blanked++
			}
			rec[k] = csvText(v)
		}
		rows++
		return w.Write(rec)
	})
	if err != nil {
		return nil, 0, 0, err
	}
	w.Flush()
	return buf.Bytes(), rows, blanked, w.Error()
}

// csvText écrit une valeur du serveur (ToRemote) pour le CSV d'un envoi.
func csvText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case time.Time:
		if x.Hour() == 0 && x.Minute() == 0 && x.Second() == 0 && x.Nanosecond() == 0 {
			return x.Format("2006-01-02")
		}
		return x.Format("2006-01-02 15:04:05")
	}
	return fmt.Sprint(v)
}

// excelEpoch : jour 0 des numéros de série de dates (30/12/1899).
var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// sourceValue convertit une valeur lue par ExportTable au type du serveur ;
// faux : envoyée vide.
func sourceValue(typ string, v any) (any, bool) {
	switch x := v.(type) {
	case canvas.Invalid:
		return nil, false
	case formula.Value:
		switch x.Kind {
		case formula.Empty:
			return nil, true
		case formula.Error:
			return nil, false
		case formula.Number:
			v = x.Num
		case formula.Text:
			v = x.Str
		case formula.Bool:
			if typ == "text" {
				v = x.String()
			} else {
				v = x.Bool
			}
		case formula.Date:
			if typ == "date" || typ == "text" {
				v = excelEpoch.AddDate(0, 0, int(x.Num)).Format("2006-01-02")
			} else {
				v = x.Num
			}
		}
	}
	r, err := workbook.ToRemote(typ, v)
	if err != nil {
		return nil, false
	}
	return r, true
}

// waitRun suit un envoi encore en cours (202) jusqu'à son bilan : par son
// exécution (run, migration 0028 : GET ?run=), sinon par l'état de la
// source (dernier envoi terminé, plus rien en attente).
func waitRun(ctx context.Context, p workbook.Pusher, id, run string) (workbook.PushRun, error) {
	for {
		if run != "" {
			r, err := p.PushRun(ctx, id, run)
			if err != nil {
				return workbook.PushRun{}, err
			}
			switch r.Status {
			case "pending", "running":
			case "":
				run = "" // serveur d'avant 0028 : ?run= ignoré
				continue
			default:
				return r, nil
			}
		} else {
			st, err := p.PushStatus(ctx, id)
			if err != nil {
				return workbook.PushRun{}, err
			}
			if r := st.LastRun; !st.Pending && r != nil && r.FinishedAt != "" && r.Status != "running" {
				return *r, nil
			}
		}
		select {
		case <-ctx.Done():
			return workbook.PushRun{Status: "running"}, msg.New("lake.stillWriting", nil)
		case <-time.After(sourcePoll):
		}
	}
}

// alignSource aligne les colonnes de la source sur le classeur
// (syncColumns) et note les colonnes et les clés à jour dans le lien.
func alignSource(ctx context.Context, d *doc.Document, link TableSourceLink, want []int64) ([]SourceColumn, error) {
	local, err := localColumns(ctx, d, link.Tbl)
	if err != nil {
		return nil, err
	}
	srv, _, _, err := remoteServer(ctx, link.Server)
	if err != nil {
		return nil, err
	}
	cols, err := syncColumns(ctx, srv, link, local, want)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(cols, link.Cols) {
		// Les clés suivent le renommage de leur colonne.
		keys := slices.Clone(link.Keys)
		for i, k := range keys {
			for _, c := range link.Cols {
				if c.Name == k {
					for _, n := range cols {
						if n.ID == c.ID {
							keys[i] = n.Name
						}
					}
				}
			}
		}
		if _, err := updateSource(ctx, d, link.Tbl, true, func(l *TableSourceLink) { l.Cols, l.Keys = cols, keys }); err != nil {
			return nil, err
		}
	}
	return cols, nil
}

// pushSource aligne les colonnes, envoie les lignes du tableau tbl à sa
// source et note le bilan dans le lien (sans rendre le classeur
// « modifié », sauf si les colonnes ont changé). want : colonnes à
// envoyer (nil : celles du lien).
func (s *DocumentService) pushSource(ctx context.Context, d *doc.Document, tbl int64, want []int64) (TableSourceLink, error) {
	link, err := sourceOf(ctx, d, tbl)
	if err != nil {
		return TableSourceLink{}, err
	}
	if link == nil {
		return TableSourceLink{}, msg.New("lake.notSource", nil)
	}
	fail := func(err error, run *workbook.PushRun) (TableSourceLink, error) {
		l, werr := updateSource(ctx, d, tbl, false, func(l *TableSourceLink) {
			l.LastError = err.Error()
			if run != nil {
				l.LastResult = run
			}
		})
		if werr != nil {
			return *link, err
		}
		return l, err
	}
	if link.TableName == "" {
		return fail(msg.New("lake.tableDeleted", nil), nil)
	}
	if link.Suspended {
		return fail(msg.New("lake.suspended", nil), nil)
	}
	cols, err := alignSource(ctx, d, *link, want)
	if err != nil {
		return fail(err, nil)
	}
	body, _, blanked, err := sourceCSV(ctx, d.Model, link.TableName, cols)
	if err != nil {
		return fail(err, nil)
	}
	p, err := remotePusher(ctx, link.Server)
	if err != nil {
		return fail(err, nil)
	}
	run, err := p.Push(ctx, link.Pipeline, body)
	if err == nil && run.Status == "running" {
		wctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		run, err = waitRun(wctx, p, link.Pipeline, cmp.Or(run.RunID, run.ID))
		cancel()
	}
	if err != nil {
		return fail(err, nil)
	}
	if run.Status == "error" {
		if run.Error != "" {
			return fail(errors.New(run.Error), &run)
		}
		return fail(msg.New("lake.pipelineRefused", nil), &run)
	}
	return updateSource(ctx, d, tbl, false, func(l *TableSourceLink) {
		r := run
		l.LastPush, l.LastResult, l.LastError, l.Blanked = time.Now().UTC().Format(time.RFC3339), &r, "", blanked
	})
}

// PushSummary écrit le bilan d'un envoi pour la barre d'état, en français
// (PushSummaryIn : dans une langue).
func PushSummary(table string, r *workbook.PushRun, blanked int) string {
	return PushSummaryIn(msg.FR, table, r, blanked)
}

// PushSummaryIn écrit le bilan d'un envoi dans la langue l.
func PushSummaryIn(l msg.Lang, table string, r *workbook.PushRun, blanked int) string {
	say := func(id msg.ID, args msg.Args) string { return msg.Format(l, id, args) }
	if r == nil {
		return say("lake.pushDone", msg.Args{"table": table})
	}
	text := say("lake.pushRead", msg.Args{"table": table, "n": r.RowsRead})
	var parts []string
	if r.Inserted > 0 {
		parts = append(parts, say("lake.pushInserted", msg.Args{"n": r.Inserted}))
	}
	if r.Updated > 0 {
		parts = append(parts, say("lake.pushUpdated", msg.Args{"n": r.Updated}))
	}
	if r.Deleted > 0 {
		parts = append(parts, say("lake.pushDeleted", msg.Args{"n": r.Deleted}))
	}
	if r.Rejected > 0 {
		parts = append(parts, say("lake.pushRejected", msg.Args{"n": r.Rejected}))
	}
	if len(parts) > 0 {
		text += " (" + strings.Join(parts, ", ") + ")"
	} else {
		text += say("lake.pushNothing", nil)
	}
	if len(r.Rejects) > 0 {
		x := r.Rejects[0]
		text += say("lake.pushReject", msg.Args{"row": x.Row, "column": x.Column, "value": x.Value, "err": x.Error})
	}
	if blanked > 0 {
		text += say("lake.pushBlanked", msg.Args{"n": blanked})
	}
	return text
}

// pushOnSave envoie, en arrière-plan, les tableaux dont la source le
// demande à l'enregistrement ; chaque résultat est émis (TableSourceEvent).
func (s *DocumentService) pushOnSave(d *doc.Document) {
	links, err := readSources(context.Background(), d)
	if err != nil {
		return
	}
	var due []TableSourceLink
	for _, l := range links {
		if l.OnSave && !l.Suspended {
			due = append(due, l)
		}
	}
	if len(due) == 0 {
		return
	}
	go func() {
		for _, l := range due {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			got, err := s.pushSource(ctx, d, l.Tbl, nil)
			cancel()
			name := cmp.Or(got.TableName, l.TableName)
			n := TableSourceNotice{Table: name}
			if err != nil {
				n.Failed = true
				n.Message = s.say("lake.pushFailed", msg.Args{"table": name, "err": err})
			} else {
				n.Message = PushSummaryIn(s.sessionLang(), name, got.LastResult, got.Blanked)
			}
			s.sys().Emit(TableSourceEvent, n)
		}
	}()
}
