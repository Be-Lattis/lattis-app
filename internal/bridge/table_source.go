package bridge

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/protocol"
)

// Clic droit sur un tableau › Partager au Data Lake Lattis… › « Publier
// uniquement ce tableau » : le tableau devient une source du Data Lake liée
// au classeur, sans publier le classeur. Le serveur en garde une copie,
// alimentée par un pipeline dont ce classeur est la source : le poste
// envoie toutes les lignes des colonnes choisies (« Mettre à jour la
// source du Data Lake », et à chaque enregistrement si demandé). Le reste
// du classeur (autres tableaux, feuilles, formules, scripts, colonnes non
// cochées) ne quitte pas le poste. Le lien est gardé dans le classeur
// (_lattis_info, clé table_sources). Contrat du serveur :
// docs/collaboration/pipeline-table-source.md.

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

// TableSourceLink est le lien d'un tableau du classeur avec sa source du
// Data Lake.
type TableSourceLink struct {
	// Tbl : tableau local (_lattis_tables.id, stable au renommage).
	Tbl int64 `json:"tbl"`
	// TableName : nom actuel du tableau ("" : tableau supprimé), relu à
	// chaque lecture.
	TableName string `json:"tableName"`
	Server    string `json:"server"`
	Pipeline  string `json:"pipeline"`
	Dataset   string `json:"dataset"`
	SQLName   string `json:"sqlName"`
	Name      string `json:"name"`
	// Columns : colonnes envoyées (les autres ne quittent pas le poste) ;
	// Keys : colonnes qui identifient une ligne d'un envoi à l'autre.
	Columns []string `json:"columns"`
	Keys    []string `json:"keys"`
	// OnSave : envoyer à chaque enregistrement du classeur.
	OnSave bool `json:"onSave"`
	// LastPush : date du dernier envoi réussi (RFC 3339) ; LastResult son
	// compte rendu ; LastError : échec du dernier essai ("" : réussi).
	LastPush   string               `json:"lastPush,omitempty"`
	LastResult *workbook.PushResult `json:"lastResult,omitempty"`
	LastError  string               `json:"lastError,omitempty"`
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
		return nil, fmt.Errorf("sources du Data Lake du classeur illisibles : %w", err)
	}
	out := v.Links
	if out == nil {
		out = []TableSourceLink{}
	}
	for i := range out {
		out[i].TableName = ""
		_ = d.DB().Read().QueryRowContext(ctx, `SELECT name FROM _lattis_tables WHERE id = ?`, out[i].Tbl).Scan(&out[i].TableName)
	}
	return out, nil
}

// writeSources range les liens ; mark : le classeur est à enregistrer
// (lien créé ou retiré, réglage changé), pas pour un compte rendu d'envoi.
func writeSources(ctx context.Context, d *doc.Document, links []TableSourceLink, mark bool) error {
	err := d.DB().Write(ctx, func(tx *sql.Tx) error {
		if len(links) == 0 {
			_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key = ?`, tableSourcesKey)
			return err
		}
		b, err := json.Marshal(tableSourcesDoc{Version: 1, Links: links})
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
		return TableSourceLink{}, errors.New("ce tableau n'est plus une source du Data Lake")
	}
	fn(&links[i])
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
		return 0, fmt.Errorf("tableau « %s » introuvable", table)
	}
	return tbl, nil
}

// unsupportedMessage : serveur sans le contrat des sources.
const unsupportedMessage = "Ce Serveur Lattis ne reçoit pas encore de tableau sans son classeur (pipeline alimenté par un classeur local) : demandez sa mise à jour à son administrateur, ou publiez le classeur."

// sourceError rend une erreur du serveur lisible.
func sourceError(err error) error {
	if err == nil {
		return nil
	}
	if workbook.Unsupported(err) {
		return errors.New(unsupportedMessage)
	}
	if _, ok := workbook.Rejected(err); ok {
		return errors.New(workbook.ServerMessage(err))
	}
	return err
}

// TableSources liste les sources du Data Lake liées au classeur ouvert.
func (s *DocumentService) TableSources(ctx context.Context) ([]TableSourceLink, error) {
	d, err := s.current()
	if err != nil {
		return []TableSourceLink{}, nil
	}
	return readSources(ctx, d)
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
			out.Reason = "Seuls les propriétaires et concepteurs du classeur partagent ses tableaux."
			return out, nil
		case published > 0:
			out.Reason = "Ce tableau est publié avec le classeur : il se partage directement au Data Lake Lattis."
			return out, nil
		}
	}
	if server == "" {
		out.Reason = "Connectez-vous d'abord à un Serveur Lattis."
		return out, nil
	}
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return out, err
	}
	ok, err := srv.TableSourcesSupported(ctx)
	if err != nil {
		return out, err
	}
	if !ok {
		out.Reason = unsupportedMessage
		return out, nil
	}
	if out.Catalog, err = s.DataCatalog(ctx, server); err != nil {
		return out, err
	}
	if !out.Catalog.CanPublish {
		out.Reason = "Votre compte ne partage pas au Data Lake Lattis sur ce serveur : demandez à son propriétaire le rôle « publieur de données »."
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
	// Columns : colonnes envoyées ; Keys : colonnes qui identifient une ligne.
	Columns []string `json:"columns"`
	Keys    []string `json:"keys"`
	OnSave  bool     `json:"onSave"`
	Inherit bool     `json:"inherit"`
	// Add : entrées d'audience à ajouter ; Remove : identifiants à retirer.
	Add    []DataAudienceEntry `json:"add"`
	Remove []string            `json:"remove"`
}

// sourceColumns vérifie les colonnes demandées (dans l'ordre du tableau)
// et la clé.
func sourceColumns(fields []RightsColumn, cols, keys []string) ([]string, []string, error) {
	var out []string
	for _, f := range fields {
		if slices.Contains(cols, f.Name) {
			out = append(out, f.Name)
		}
	}
	if len(out) == 0 {
		return nil, nil, errors.New("cochez au moins une colonne à envoyer")
	}
	for _, c := range cols {
		if !slices.Contains(out, c) {
			return nil, nil, fmt.Errorf("colonne « %s » inconnue", c)
		}
	}
	k := []string{}
	for _, c := range keys {
		if !slices.Contains(out, c) {
			return nil, nil, fmt.Errorf("la clé « %s » doit être une colonne envoyée", c)
		}
		k = append(k, c)
	}
	return out, k, nil
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

// CreateTableSource publie le tableau seul comme source du Data Lake : la
// copie, le pipeline et la donnée sont créés sur le serveur, le lien est
// gardé dans le classeur, puis les lignes sont envoyées une première fois.
// Le classeur n'est pas publié. Un premier envoi refusé laisse la source
// créée (LastError dit pourquoi).
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
		return TableSourceLink{}, errors.New("ce tableau est déjà une source du Data Lake")
	}
	if info.Reason != "" {
		return TableSourceLink{}, errors.New(info.Reason)
	}
	cols, keys, err := sourceColumns(info.Fields, req.Columns, req.Keys)
	if err != nil {
		return TableSourceLink{}, err
	}
	tbl, err := tableID(ctx, d, req.Table)
	if err != nil {
		return TableSourceLink{}, err
	}
	srv, _, _, err := remoteServer(ctx, req.Server)
	if err != nil {
		return TableSourceLink{}, err
	}
	specs := make([]workbook.ColumnSpec, len(cols))
	for i, c := range cols {
		for _, f := range info.Fields {
			if f.Name == c {
				specs[i] = workbook.ColumnSpec{Name: c, Type: workbook.RemoteType(grid.Type(f.Type))}
			}
		}
	}
	name := strings.TrimSpace(req.Name)
	created, err := srv.CreateTableSource(ctx, workbook.TableSourceSpec{Name: name, Description: strings.TrimSpace(req.Description),
		FolderID: req.FolderID, SQLName: strings.TrimSpace(req.SQLName), Columns: specs, KeyColumns: keys,
		Origin: workbook.TableOrigin{Workbook: docID(ctx, d), Title: s.docTitle(ctx), Table: req.Table}})
	if err != nil {
		return TableSourceLink{}, sourceError(err)
	}
	link := TableSourceLink{Tbl: tbl, TableName: req.Table, Server: req.Server, Pipeline: created.PipelineID, Dataset: created.DatasetID,
		SQLName: created.SQLName, Name: name, Columns: cols, Keys: keys, OnSave: req.OnSave}
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
	pushed, _ := s.pushSource(ctx, d, tbl)
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
		return fmt.Errorf("source enregistrée, mais une partie de l'audience est refusée : %s", strings.Join(refused, " ; "))
	}
	return nil
}

// UpdateTableSource modifie la source d'un tableau (nom, dossier,
// audience au serveur ; colonnes, clé, envoi à l'enregistrement dans le
// classeur), puis envoie les lignes.
func (s *DocumentService) UpdateTableSource(ctx context.Context, req TableSourceRequest) (TableSourceLink, error) {
	d, err := s.current()
	if err != nil {
		return TableSourceLink{}, err
	}
	info, err := s.TableSourceInfo(ctx, req.Table, "")
	if err != nil {
		return TableSourceLink{}, err
	}
	if info.Link == nil {
		return TableSourceLink{}, errors.New("ce tableau n'est pas une source du Data Lake")
	}
	cols, keys, err := sourceColumns(info.Fields, req.Columns, req.Keys)
	if err != nil {
		return TableSourceLink{}, err
	}
	srv, _, _, err := remoteServer(ctx, info.Link.Server)
	if err != nil {
		return TableSourceLink{}, err
	}
	name := strings.TrimSpace(req.Name)
	desc := strings.TrimSpace(req.Description)
	folder := req.FolderID
	if err := srv.UpdateDataset(ctx, info.Link.Dataset, workbook.DatasetChange{Name: &name, Description: &desc, FolderID: &folder,
		InheritAudience: &req.Inherit}); err != nil {
		return *info.Link, sourceError(err)
	}
	if _, err := updateSource(ctx, d, info.Link.Tbl, true, func(l *TableSourceLink) {
		l.Name, l.Columns, l.Keys, l.OnSave = name, cols, keys, req.OnSave
	}); err != nil {
		return *info.Link, err
	}
	if err := sourceAudience(ctx, srv, info.Link.Dataset, req, false); err != nil {
		return *info.Link, err
	}
	return s.pushSource(ctx, d, info.Link.Tbl)
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
	return s.pushSource(ctx, d, tbl)
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
		return errors.New("ce tableau n'est pas une source du Data Lake")
	}
	srv, _, _, err := remoteServer(ctx, link.Server)
	if err != nil {
		return err
	}
	if err := srv.DropTableSource(ctx, link.Pipeline); err != nil {
		if code, ok := workbook.Rejected(err); !ok || code != "P0002" {
			return sourceError(err)
		}
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

// sourceRows lit les colonnes envoyées du tableau, converties au type du
// serveur ; blanked compte les valeurs envoyées vides (saisie hors type,
// erreur d'une formule, valeur inconvertible).
func sourceRows(ctx context.Context, m *canvas.Model, table string, want []string) (cols []workbook.ColumnSpec, rows [][]any, blanked int, err error) {
	var idx []int
	err = m.ExportTable(ctx, table, func(h []canvas.ExportColumn) error {
		for _, w := range want {
			i := slices.IndexFunc(h, func(c canvas.ExportColumn) bool { return c.Name == w })
			if i < 0 {
				return fmt.Errorf("la colonne « %s » envoyée par la source n'existe plus : modifiez la source", w)
			}
			idx = append(idx, i)
			cols = append(cols, workbook.ColumnSpec{Name: w, Type: workbook.RemoteType(h[i].Type)})
		}
		return nil
	}, func(r []any) error {
		row := make([]any, len(idx))
		for j, i := range idx {
			v, ok := sourceValue(cols[j].Type, r[i])
			if !ok {
				blanked++
			}
			row[j] = v
		}
		rows = append(rows, row)
		return nil
	})
	return cols, rows, blanked, err
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

// pushSource envoie les lignes du tableau tbl à sa source et note le
// résultat dans le lien (sans rendre le classeur « modifié »).
func (s *DocumentService) pushSource(ctx context.Context, d *doc.Document, tbl int64) (TableSourceLink, error) {
	link, err := sourceOf(ctx, d, tbl)
	if err != nil {
		return TableSourceLink{}, err
	}
	if link == nil {
		return TableSourceLink{}, errors.New("ce tableau n'est pas une source du Data Lake")
	}
	fail := func(err error) (TableSourceLink, error) {
		l, werr := updateSource(ctx, d, tbl, false, func(l *TableSourceLink) { l.LastError = err.Error() })
		if werr != nil {
			return *link, err
		}
		return l, err
	}
	if link.TableName == "" {
		return fail(errors.New("le tableau de cette source a été supprimé du classeur"))
	}
	cols, rows, blanked, err := sourceRows(ctx, d.Model, link.TableName, link.Columns)
	if err != nil {
		return fail(err)
	}
	srv, _, _, err := remoteServer(ctx, link.Server)
	if err != nil {
		return fail(err)
	}
	res, err := srv.PushTableSource(ctx, link.Pipeline, workbook.TablePush{Columns: cols, KeyColumns: link.Keys, Rows: rows,
		Origin: workbook.TableOrigin{Workbook: docID(ctx, d), Title: s.docTitle(ctx), Table: link.TableName}})
	if err != nil {
		return fail(sourceError(err))
	}
	return updateSource(ctx, d, tbl, false, func(l *TableSourceLink) {
		r := res
		l.LastPush, l.LastResult, l.LastError, l.Blanked = time.Now().UTC().Format(time.RFC3339), &r, "", blanked
	})
}

// PushSummary écrit le compte rendu d'un envoi pour la barre d'état.
func PushSummary(table string, r *workbook.PushResult, blanked int) string {
	if r == nil {
		return fmt.Sprintf("Source « %s » mise à jour", table)
	}
	msg := fmt.Sprintf("Source « %s » mise à jour : %d ligne%s", table, r.Rows, plural(r.Rows))
	var parts []string
	if r.Inserted > 0 {
		parts = append(parts, fmt.Sprintf("%d ajoutée%s", r.Inserted, plural(r.Inserted)))
	}
	if r.Updated > 0 {
		parts = append(parts, fmt.Sprintf("%d modifiée%s", r.Updated, plural(r.Updated)))
	}
	if r.Deleted > 0 {
		parts = append(parts, fmt.Sprintf("%d supprimée%s", r.Deleted, plural(r.Deleted)))
	}
	if r.Rejected > 0 {
		parts = append(parts, fmt.Sprintf("%d refusée%s", r.Rejected, plural(r.Rejected)))
	}
	if len(parts) > 0 {
		msg += " (" + strings.Join(parts, ", ") + ")"
	} else {
		msg += ", rien n'a changé"
	}
	if blanked > 0 {
		msg += fmt.Sprintf(" ; %d valeur%s envoyée%s vide%s (erreur ou saisie hors type)", blanked, plural(blanked), plural(blanked), plural(blanked))
	}
	return msg
}

func plural(n int) string {
	if n > 1 {
		return "s"
	}
	return ""
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
		if l.OnSave {
			due = append(due, l)
		}
	}
	if len(due) == 0 {
		return
	}
	go func() {
		for _, l := range due {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			got, err := s.pushSource(ctx, d, l.Tbl)
			cancel()
			name := cmp.Or(got.TableName, l.TableName)
			n := TableSourceNotice{Table: name}
			if err != nil {
				n.Failed = true
				n.Message = fmt.Sprintf("Source « %s » non mise à jour : %s", name, err.Error())
			} else {
				n.Message = PushSummary(name, got.LastResult, got.Blanked)
			}
			s.sys().Emit(TableSourceEvent, n)
		}
	}()
}
