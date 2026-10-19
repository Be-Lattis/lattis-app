package bridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/protocol"
)

// Partager un tableau au Data Lake Lattis (clic droit sur un tableau),
// et gérer les données qu'on gère (contrat de lattisd, §9).

// DataShareInfo décrit ce qu'il faut pour partager un tableau : possible
// seulement pour un tableau publié d'un classeur du serveur, par un
// publieur de données propriétaire ou concepteur du classeur.
type DataShareInfo struct {
	// Reason : pourquoi le partage est impossible ("" : possible).
	Reason     string   `json:"reason"`
	Server     string   `json:"server"`
	WorkbookID string   `json:"workbookId"`
	TableID    string   `json:"tableId"`
	Table      string   `json:"table"`
	Columns    []string `json:"columns"`
	// Fields : colonnes du tableau avec leur type (formulaire des filtres de
	// l'audience, comme ceux des rôles).
	Fields  []RightsColumn       `json:"fields"`
	Catalog protocol.DataCatalog `json:"catalog"`
	Teams   []DataTeam           `json:"teams"`
	// Dataset : la donnée qui partage déjà ce tableau (nil : aucune).
	Dataset *protocol.Dataset `json:"dataset"`
	// Choice : le tableau n'est pas dans un classeur publié, deux voies
	// s'offrent : « workbook » (classeur non publié : le publier avec ses
	// droits) ou « table » (classeur publié sans ce tableau), ou publier
	// le tableau seul comme source (TableSourceInfo) ; « collab » : classeur
	// partagé en direct sans ce tableau, à y ajouter (ShareAddTable) ou à
	// publier seul. Reason dit alors
	// pourquoi la première voie est fermée ("" : ouverte).
	Choice string `json:"choice"`
	// Source : le tableau est déjà une source du Data Lake (sans le classeur).
	Source *TableSourceLink `json:"source"`
}

// DataTeam est une équipe proposée pour l'audience.
type DataTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DataShareInfo lit de quoi partager le tableau table du classeur ouvert.
func (s *DocumentService) DataShareInfo(ctx context.Context, table string) (DataShareInfo, error) {
	out := DataShareInfo{Table: table, Columns: []string{}, Fields: []RightsColumn{}, Teams: []DataTeam{},
		Catalog: protocol.DataCatalog{Folders: []protocol.DataFolder{}, Datasets: []protocol.Dataset{}}}
	d, err := s.current()
	if err != nil {
		return out, err
	}
	var tbl int64
	err = d.DB().Read().QueryRowContext(ctx, `SELECT t.id, coalesce(r.remote_id, '') FROM _lattis_tables t
		LEFT JOIN _lattis_remote_tables r ON r.tbl = t.id AND r.published = 1 WHERE t.name = ?`, table).Scan(&tbl, &out.TableID)
	if err != nil {
		return out, fmt.Errorf("tableau « %s » introuvable : %w", table, err)
	}
	// Déjà une source du Data Lake sans le classeur : on la gère.
	if out.Source, err = sourceOf(ctx, d, tbl); err != nil || out.Source != nil {
		return out, err
	}
	info := d.Share.Info()
	if info == nil {
		out.Choice = "workbook"
		return out, nil
	}
	if !info.CanDesign() {
		out.Reason = "Seuls les propriétaires et concepteurs du classeur partagent ses tableaux."
		return out, nil
	}
	if out.TableID == "" {
		out.Choice = "table"
		if info.Mode == workbook.Frozen {
			out.Reason = ""
		} else if eng := s.collab(); eng != nil {
			// Classeur partagé en direct : le tableau s'y ajoute
			// (ShareAddTable) ; Reason dit pourquoi pas maintenant.
			out.Choice = "collab"
			out.Reason = eng.CanAdd(tbl)
		} else {
			out.Reason = "Un classeur collaboratif ne se republie pas : ce tableau n'y entre plus avec le classeur."
		}
		return out, nil
	}
	l, err := d.Model.TableColumns(ctx, tbl)
	if err != nil {
		return out, err
	}
	out.Columns = l
	if rs, err := d.DB().Read().QueryContext(ctx, `SELECT name, type FROM _lattis_columns WHERE tbl = ? ORDER BY pos`, tbl); err == nil {
		for rs.Next() {
			var c RightsColumn
			if rs.Scan(&c.Name, &c.Type) == nil {
				out.Fields = append(out.Fields, c)
			}
		}
		rs.Close()
	}
	out.Server, out.WorkbookID = info.Server, info.WorkbookID
	srv, _, _, err := remoteServer(ctx, info.Server)
	if err != nil {
		return out, err
	}
	if out.Catalog, err = s.DataCatalog(ctx, info.Server); err != nil {
		return out, err
	}
	if !out.Catalog.CanPublish {
		out.Reason = "Votre compte ne partage pas au Data Lake Lattis sur ce serveur : demandez à son propriétaire le rôle « publieur de données »."
		return out, nil
	}
	for i := range out.Catalog.Datasets {
		ds := &out.Catalog.Datasets[i]
		if ds.TableID == out.TableID && ds.Manageable {
			out.Dataset = ds
		}
	}
	if teams, err := srv.Teams(ctx); err == nil {
		for _, t := range teams {
			out.Teams = append(out.Teams, DataTeam{ID: t.ID, Name: t.Name})
		}
	}
	return out, nil
}

// DataAudienceEntry est une entrée d'audience à ajouter : une adresse
// (Email) ou une équipe (Team), avec un filtre facultatif.
type DataAudienceEntry struct {
	Email     string `json:"email"`
	Team      string `json:"team"`
	Predicate string `json:"predicate"`
}

// DataShare est le partage (création ou modification) d'un tableau.
type DataShare struct {
	Server string `json:"server"`
	// Dataset : donnée à modifier ("" : partager le tableau).
	Dataset     string   `json:"dataset"`
	WorkbookID  string   `json:"workbookId"`
	TableID     string   `json:"tableId"`
	FolderID    string   `json:"folderId"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	SQLName     string   `json:"sqlName"`
	Hidden      []string `json:"hidden"`
	Inherit     bool     `json:"inherit"`
	// Add : entrées d'audience à ajouter ; Remove : identifiants à retirer.
	Add    []DataAudienceEntry `json:"add"`
	Remove []string            `json:"remove"`
}

// ShareDataset partage un tableau au Data Lake Lattis, ou modifie la
// donnée qui le partage ; rend son identifiant. Une entrée d'audience
// refusée (adresse sans compte) est signalée après coup : le reste est
// enregistré.
func (s *DocumentService) ShareDataset(ctx context.Context, req DataShare) (string, error) {
	srv, _, _, err := remoteServer(ctx, req.Server)
	if err != nil {
		return "", err
	}
	hidden := req.Hidden
	if hidden == nil {
		hidden = []string{}
	}
	id := req.Dataset
	if id == "" {
		if id, err = srv.CreateDataset(ctx, workbook.DatasetSpec{WorkbookID: req.WorkbookID, TableID: req.TableID, FolderID: req.FolderID,
			Name: req.Name, Description: req.Description, SQLName: req.SQLName, Hidden: hidden}); err != nil {
			return "", err
		}
		if !req.Inherit {
			inherit := false
			if err := srv.UpdateDataset(ctx, id, workbook.DatasetChange{InheritAudience: &inherit}); err != nil {
				return id, err
			}
		}
	} else {
		folder := req.FolderID
		if err := srv.UpdateDataset(ctx, id, workbook.DatasetChange{Name: &req.Name, Description: &req.Description, FolderID: &folder,
			HiddenColumns: hidden, InheritAudience: &req.Inherit}); err != nil {
			return id, err
		}
	}
	for _, a := range req.Remove {
		if err := srv.RemoveDataAudience(ctx, a); err != nil {
			return id, err
		}
	}
	var refused []string
	for _, a := range req.Add {
		var err error
		switch {
		case a.Team != "":
			_, err = srv.AddDataAudience(ctx, id, "", a.Team, a.Predicate)
		case strings.TrimSpace(a.Email) != "":
			_, err = srv.AddDataAudienceByEmail(ctx, id, "", a.Email, a.Predicate)
		}
		if err != nil {
			refused = append(refused, remoteMessage(err))
		}
	}
	if len(refused) > 0 {
		return id, fmt.Errorf("donnée enregistrée, mais une partie de l'audience est refusée : %s", strings.Join(refused, " ; "))
	}
	return id, nil
}

// remoteMessage : le message d'une erreur du serveur, sans le code SQL.
func remoteMessage(err error) string {
	m := err.Error()
	if i := strings.Index(m, " (SQLSTATE"); i > 0 {
		m = m[:i]
	}
	return strings.TrimPrefix(m, "ERROR: ")
}

// DropDataset retire le partage d'une donnée (la vue disparaît ; les
// classeurs qui l'utilisent gardent leur dernière copie).
func (s *DocumentService) DropDataset(ctx context.Context, server, id string) error {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return err
	}
	return srv.DropDataset(ctx, id)
}

// CreateDataFolder crée un dossier du catalogue (parent "" : à la racine).
func (s *DocumentService) CreateDataFolder(ctx context.Context, server, name, parent string) (string, error) {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return "", err
	}
	return srv.CreateDataFolder(ctx, strings.TrimSpace(name), parent)
}

// MoveDataItem range une donnée (kind dataset) ou un dossier (kind folder)
// dans le dossier parent ("" : la racine) : glisser-déposer dans l'arbre.
func (s *DocumentService) MoveDataItem(ctx context.Context, server, kind, id, parent string) error {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return err
	}
	if kind == "folder" {
		return srv.MoveDataFolder(ctx, id, parent)
	}
	return srv.UpdateDataset(ctx, id, workbook.DatasetChange{FolderID: &parent})
}
