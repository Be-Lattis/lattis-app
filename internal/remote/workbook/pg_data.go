package workbook

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lalamefine/lattis/protocol"
)

// Données du Data Lake (contrat client de lattisd, §9 ; migration 0018) :
// catalogue, partage d'un tableau, audience, et lecture des vues
// donnees."<sql_name>".

// DatasetSpec décrit le partage d'un tableau (lattis.create_dataset).
type DatasetSpec struct {
	WorkbookID  string `json:"workbookId"`
	TableID     string `json:"tableId"`
	FolderID    string `json:"folderId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// SQLName vide : proposé par le serveur à partir du nom.
	SQLName string `json:"sqlName"`
	// Hidden : colonnes non exposées.
	Hidden []string `json:"hidden"`
}

// DatasetChange modifie une donnée (lattis.update_dataset) ; nil : inchangé.
type DatasetChange struct {
	Name            *string  `json:"name,omitempty"`
	Description     *string  `json:"description,omitempty"`
	FolderID        *string  `json:"folder_id,omitempty"`
	HiddenColumns   []string `json:"hidden_columns,omitempty"`
	InheritAudience *bool    `json:"inherit_audience,omitempty"`
}

// DataRows est le contenu d'une donnée : colonnes exposées (sans les
// colonnes lattis_*), identifiants des lignes, valeurs en texte (vide :
// NULL), et point de reprise pris avant la lecture.
type DataRows struct {
	Columns []string
	IDs     []string
	Rows    [][]string
	Since   string
}

// DataRow est une ligne d'une donnée pour un tableau lié : identifiant
// (lattis_id), clé d'ordre (lattis_pos) et valeurs des colonnes exposées,
// telles que les rend database/sql avec le pilote pgx (nombres décimaux en
// texte, dates en time.Time…), pour être converties comme le premier
// chargement de la requête.
type DataRow struct {
	ID, Pos string
	Values  []any
}

// DataDelta est le relevé d'une donnée depuis un point de reprise.
type DataDelta struct {
	// Columns, Types : colonnes exposées (sans lattis_*) et nom de leur type
	// PostgreSQL en majuscules (INT8, NUMERIC, TEXT…, comme
	// DatabaseTypeName de database/sql).
	Columns []string
	Types   []string
	// Rows : lignes modifiées depuis since, ou toutes (Full), par lattis_pos.
	Rows []DataRow
	// Deleted : lignes supprimées ou sorties de l'audience depuis since
	// (vide si Full).
	Deleted []string
	// Full : toute la donnée relue (since vide ou plus couvert).
	Full bool
	// Count : lignes visibles dans la vue, pour vérifier la copie locale (relevé manqué).
	Count int
	// Since : prochain point de reprise (sync_point pris avant la lecture).
	Since string
}

// DataResult est le résultat d'une requête de l'onglet SQL sur le serveur.
type DataResult struct {
	Columns   []string
	Rows      [][]any
	Truncated bool
}

// Data est l'API des données du Data Lake.
type Data interface {
	DataCatalog(ctx context.Context) (protocol.DataCatalog, error)
	CreateDataFolder(ctx context.Context, name, parentID string) (string, error)
	RenameDataFolder(ctx context.Context, id, name string) error
	MoveDataFolder(ctx context.Context, id, parentID string) error
	DropDataFolder(ctx context.Context, id string) error
	CreateDataset(ctx context.Context, d DatasetSpec) (string, error)
	UpdateDataset(ctx context.Context, id string, c DatasetChange) error
	// AddDataAudience : datasetID ou folderID ; predicate vide : toutes les lignes.
	AddDataAudience(ctx context.Context, datasetID, folderID, principal, predicate string) (string, error)
	// AddDataAudienceByEmail : une personne par son adresse (migration 0019) ;
	// adresse sans compte actif : erreur P0002 du serveur, à montrer telle quelle.
	AddDataAudienceByEmail(ctx context.Context, datasetID, folderID, email, predicate string) (string, error)
	RemoveDataAudience(ctx context.Context, id string) error
	DropDataset(ctx context.Context, id string) error
	LinkDataset(ctx context.Context, id, workbookID string) error
	UnlinkDataset(ctx context.Context, id, workbookID string) error
	// ReadDataset lit toutes les lignes d'une donnée (sur un réplica s'il y
	// en a un), après sync_point().
	ReadDataset(ctx context.Context, sqlName string) (DataRows, error)
	// DatasetDelta lit ce qui a changé dans une donnée depuis since (relevé
	// d'un tableau lié) : lignes modifiées (lattis_xid ≥ since), supprimées
	// (dataset_tombstones), dans une même lecture cohérente. since vide, ou
	// que les pierres tombales ne couvrent plus (since_covered) : toutes les
	// lignes (Full).
	DatasetDelta(ctx context.Context, id, sqlName, since string) (DataDelta, error)
	// DataQuery exécute une requête de lecture de l'utilisateur (onglet SQL :
	// vues donnees.*) dans une transaction en lecture seule ; au plus limit
	// lignes.
	DataQuery(ctx context.Context, q string, limit int) (DataResult, error)
	// ListenData écoute les canaux donnés (LISTEN) et appelle fn(canal,
	// charge) à chaque notification, jusqu'à l'annulation de ctx ou une
	// coupure ; ready est appelé une fois l'écoute posée.
	ListenData(ctx context.Context, channels []string, ready func(), fn func(channel, payload string)) error
}

func nullUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s pgServer) DataCatalog(ctx context.Context) (protocol.DataCatalog, error) {
	var c protocol.DataCatalog
	err := s.jsonCall(ctx, `SELECT lattis.data_catalog()::text`, &c)
	return c, err
}

func (s pgServer) CreateDataFolder(ctx context.Context, name, parentID string) (string, error) {
	var id string
	err := s.tx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT lattis.create_data_folder($1, $2::uuid)::text`, name, nullUUID(parentID)).Scan(&id)
	})
	return id, err
}

func (s pgServer) RenameDataFolder(ctx context.Context, id, name string) error {
	return s.exec(ctx, `SELECT lattis.rename_data_folder($1::uuid, $2)`, id, name)
}

func (s pgServer) MoveDataFolder(ctx context.Context, id, parentID string) error {
	return s.exec(ctx, `SELECT lattis.move_data_folder($1::uuid, $2::uuid)`, id, nullUUID(parentID))
}

func (s pgServer) DropDataFolder(ctx context.Context, id string) error {
	return s.exec(ctx, `SELECT lattis.drop_data_folder($1::uuid)`, id)
}

func (s pgServer) CreateDataset(ctx context.Context, d DatasetSpec) (string, error) {
	var id string
	hidden := d.Hidden
	if hidden == nil {
		hidden = []string{}
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT lattis.create_dataset($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7::text[])::text`,
			d.WorkbookID, d.TableID, nullUUID(d.FolderID), d.Name, d.Description, d.SQLName, hidden).Scan(&id)
	})
	return id, err
}

func (s pgServer) UpdateDataset(ctx context.Context, id string, c DatasetChange) error {
	return s.exec(ctx, `SELECT lattis.update_dataset($1::uuid, $2::jsonb)`, id, c)
}

func (s pgServer) AddDataAudience(ctx context.Context, datasetID, folderID, principal, predicate string) (string, error) {
	var id string
	err := s.tx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT lattis.add_data_audience($1::uuid, $2::uuid, $3::uuid, $4)::text`,
			nullUUID(datasetID), nullUUID(folderID), principal, predicate).Scan(&id)
	})
	return id, err
}

func (s pgServer) AddDataAudienceByEmail(ctx context.Context, datasetID, folderID, email, predicate string) (string, error) {
	var id string
	err := s.tx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT lattis.add_data_audience_by_email($1::uuid, $2::uuid, $3, $4)::text`,
			nullUUID(datasetID), nullUUID(folderID), email, predicate).Scan(&id)
	})
	return id, err
}

func (s pgServer) RemoveDataAudience(ctx context.Context, id string) error {
	return s.exec(ctx, `SELECT lattis.remove_data_audience($1::uuid)`, id)
}

func (s pgServer) DropDataset(ctx context.Context, id string) error {
	return s.exec(ctx, `SELECT lattis.drop_dataset($1::uuid)`, id)
}

func (s pgServer) LinkDataset(ctx context.Context, id, workbookID string) error {
	return s.exec(ctx, `SELECT lattis.link_dataset($1::uuid, $2::uuid)`, id, workbookID)
}

func (s pgServer) UnlinkDataset(ctx context.Context, id, workbookID string) error {
	return s.exec(ctx, `SELECT lattis.unlink_dataset($1::uuid, $2::uuid)`, id, workbookID)
}

func (s pgServer) ReadDataset(ctx context.Context, sqlName string) (DataRows, error) {
	var out DataRows
	err := s.with(ctx, usageLoad, func(c *pgx.Conn) error {
		return pgx.BeginTxFunc(ctx, c, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
			if err := tx.QueryRow(ctx, `SELECT lattis.sync_point()::text`).Scan(&out.Since); err != nil {
				return err
			}
			rows, err := tx.Query(ctx, `SELECT * FROM `+protocol.DataSchema+`.`+pgx.Identifier{sqlName}.Sanitize()+` ORDER BY lattis_pos`)
			if err != nil {
				return err
			}
			defer rows.Close()
			fields := rows.FieldDescriptions()
			idCol := -1
			var keep []int
			for i, f := range fields {
				switch f.Name {
				case "lattis_id":
					idCol = i
				case "lattis_pos", "lattis_xid", "lattis_updated_at", "lattis_author", "lattis_updated_by":
				default:
					keep = append(keep, i)
					out.Columns = append(out.Columns, f.Name)
				}
			}
			for rows.Next() {
				vals, err := rows.Values()
				if err != nil {
					return err
				}
				row := make([]string, len(keep))
				for j, i := range keep {
					row[j] = dataText(vals[i])
				}
				out.Rows = append(out.Rows, row)
				if idCol >= 0 {
					out.IDs = append(out.IDs, dataText(vals[idCol]))
				}
			}
			return rows.Err()
		})
	})
	return out, err
}

// dataText écrit une valeur de PostgreSQL comme une saisie de Lattis :
// nombres avec un point, dates ISO, booléens VRAI/FAUX ; "" pour NULL.
func dataText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "VRAI"
		}
		return "FAUX"
	case int64:
		return strconv.FormatInt(x, 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int16:
		return strconv.FormatInt(int64(x), 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case time.Time:
		if x.Hour() == 0 && x.Minute() == 0 && x.Second() == 0 && x.Nanosecond() == 0 {
			return x.Format("2006-01-02")
		}
		return x.Format("2006-01-02 15:04:05")
	case pgtype.Numeric:
		if !x.Valid || x.NaN {
			return ""
		}
		r := new(big.Rat).SetInt(x.Int)
		if x.Exp > 0 {
			r.Mul(r, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(x.Exp)), nil)))
		} else if x.Exp < 0 {
			r.Quo(r, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-x.Exp)), nil)))
		}
		return r.FloatString(max(0, int(-x.Exp)))
	case [16]byte:
		return fmt.Sprintf("%x-%x-%x-%x-%x", x[0:4], x[4:6], x[6:8], x[8:10], x[10:16])
	}
	return fmt.Sprint(v)
}

func (s pgServer) ListenData(ctx context.Context, channels []string, ready func(), fn func(channel, payload string)) error {
	return s.with(ctx, usageListen, func(c *pgx.Conn) error {
		for _, ch := range channels {
			if _, err := c.Exec(ctx, `LISTEN `+pgx.Identifier{ch}.Sanitize()); err != nil {
				return err
			}
		}
		defer c.Exec(context.WithoutCancel(ctx), "UNLISTEN *")
		if ready != nil {
			ready()
		}
		for {
			n, err := c.WaitForNotification(ctx)
			if err != nil {
				return err
			}
			fn(n.Channel, n.Payload)
		}
	})
}
