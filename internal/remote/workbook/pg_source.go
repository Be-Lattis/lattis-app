package workbook

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Sources du Data Lake tirées d'un classeur local (clic droit sur un
// tableau › Partager au Data Lake Lattis… › « Publier uniquement ce
// tableau ») : le classeur n'est pas publié ; le serveur garde une copie du
// tableau, alimentée par un pipeline dont le classeur local est la source
// (le poste envoie les lignes, à la demande ou à l'enregistrement), et
// partagée au catalogue comme toute donnée.
//
// Ces fonctions ne sont pas encore dans le schéma lattis du serveur :
// contrat attendu dans docs/collaboration/pipeline-table-source.md. Un
// serveur qui ne les a pas répond 42883 (Unsupported).

// TableSourceSpec décrit une source à créer (lattis.create_table_source).
type TableSourceSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// FolderID : dossier du catalogue ("" : la racine).
	FolderID string `json:"folder_id,omitempty"`
	// SQLName vide : proposé par le serveur à partir du nom.
	SQLName string       `json:"sql_name,omitempty"`
	Columns []ColumnSpec `json:"columns"`
	// KeyColumns : colonnes qui identifient une ligne d'un envoi à l'autre
	// (vide : chaque envoi remplace toutes les lignes).
	KeyColumns []string    `json:"key_columns"`
	Origin     TableOrigin `json:"origin"`
}

// TableOrigin dit d'où vient une source (catalogue, administration des
// pipelines) : rien du contenu du classeur hors du tableau.
type TableOrigin struct {
	// Workbook : identifiant aléatoire du classeur local (_lattis_info, id).
	Workbook string `json:"workbook"`
	Title    string `json:"title"`
	Table    string `json:"table"`
}

// TableSourceCreated est le résultat de lattis.create_table_source.
type TableSourceCreated struct {
	PipelineID string `json:"pipeline_id"`
	DatasetID  string `json:"dataset_id"`
	SQLName    string `json:"sql_name"`
}

// TablePush est un envoi : toutes les lignes du tableau (instantané
// complet), colonnes et clé du moment. Rows : valeurs du type du serveur
// (ToRemote : int64, float64, bool, string, time.Time pour une date, nil).
type TablePush struct {
	Columns    []ColumnSpec
	KeyColumns []string
	Origin     TableOrigin
	Rows       [][]any
}

// PushReject est une ligne refusée par le serveur (numéro à partir de 1).
type PushReject struct {
	Row   int    `json:"row"`
	Error string `json:"error"`
}

// PushResult est le compte rendu d'un envoi (lattis.finish_table_push,
// une ligne de lattis.pipeline_runs).
type PushResult struct {
	RunID string `json:"run_id"`
	// Status : ok ou warning (lignes refusées).
	Status   string       `json:"status"`
	Rows     int          `json:"rows"`
	Inserted int          `json:"inserted"`
	Updated  int          `json:"updated"`
	Deleted  int          `json:"deleted"`
	Rejected int          `json:"rejected"`
	Rejects  []PushReject `json:"rejects,omitempty"`
}

// PipelineRun est une exécution d'un pipeline (lattis.pipeline_runs).
type PipelineRun struct {
	ID         string `json:"id"`
	Trigger    string `json:"trigger"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
	Inserted   int    `json:"inserted"`
	Updated    int    `json:"updated"`
	Deleted    int    `json:"deleted"`
	Rejected   int    `json:"rejected"`
	Error      string `json:"error,omitempty"`
}

// TableSource est une source du Data Lake alimentée par un classeur local
// (lattis.my_table_sources).
type TableSource struct {
	PipelineID string       `json:"pipeline_id"`
	DatasetID  string       `json:"dataset_id"`
	Name       string       `json:"name"`
	SQLName    string       `json:"sql_name"`
	Columns    []ColumnSpec `json:"columns"`
	KeyColumns []string     `json:"key_columns"`
	Origin     TableOrigin  `json:"origin"`
	// Owner : adresse de qui a créé la source (lui seul l'alimente).
	Owner   string       `json:"owner"`
	LastRun *PipelineRun `json:"last_run,omitempty"`
}

// TableSources est l'API des sources tirées d'un classeur local.
type TableSources interface {
	// TableSourcesSupported dit si le serveur a le contrat (fonction
	// lattis.create_table_source).
	TableSourcesSupported(ctx context.Context) (bool, error)
	// CreateTableSource crée la copie du tableau sur le serveur, le
	// pipeline qui l'alimente et la donnée du catalogue (publieurs de
	// données), sans ligne : le premier envoi suit.
	CreateTableSource(ctx context.Context, spec TableSourceSpec) (TableSourceCreated, error)
	// PushTableSource envoie toutes les lignes en une transaction (tout ou
	// rien) : begin_table_push, push_table_rows par paquets,
	// finish_table_push. Créateur de la source seulement.
	PushTableSource(ctx context.Context, pipelineID string, p TablePush) (PushResult, error)
	// TableSources liste les sources de l'utilisateur (toutes pour un
	// propriétaire du serveur).
	TableSources(ctx context.Context) ([]TableSource, error)
	// DropTableSource retire la source : pipeline, donnée et copie (les
	// classeurs qui l'utilisent gardent leur dernière copie).
	DropTableSource(ctx context.Context, pipelineID string) error
}

// Unsupported dit si err vient d'un serveur qui n'a pas la fonction
// appelée (42883 : undefined_function).
func Unsupported(err error) bool {
	code, ok := Rejected(err)
	return ok && code == "42883"
}

// pushChunk : lignes par appel de lattis.push_table_rows.
const pushChunk = 2000

// PushJSON écrit une valeur du serveur pour la charge JSON d'un envoi :
// dates en aaaa-mm-jj, le reste tel quel.
func PushJSON(v any) any {
	if t, ok := v.(time.Time); ok {
		if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
			return t.Format("2006-01-02")
		}
		return t.Format(time.RFC3339)
	}
	return v
}

func (s pgServer) TableSourcesSupported(ctx context.Context) (bool, error) {
	var ok bool
	err := s.with(ctx, usageWrite, func(c *pgx.Conn) error {
		return c.QueryRow(ctx, `SELECT to_regprocedure('lattis.create_table_source(jsonb)') IS NOT NULL`).Scan(&ok)
	})
	return ok, err
}

func (s pgServer) CreateTableSource(ctx context.Context, spec TableSourceSpec) (TableSourceCreated, error) {
	var out TableSourceCreated
	if spec.KeyColumns == nil {
		spec.KeyColumns = []string{}
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT lattis.create_table_source($1::jsonb)::text`, spec).Scan(&raw); err != nil {
			return err
		}
		return json.Unmarshal(raw, &out)
	})
	return out, err
}

func (s pgServer) PushTableSource(ctx context.Context, pipelineID string, p TablePush) (PushResult, error) {
	var out PushResult
	keys := p.KeyColumns
	if keys == nil {
		keys = []string{}
	}
	head, err := json.Marshal(map[string]any{"columns": p.Columns, "key_columns": keys, "origin": p.Origin, "rows": len(p.Rows)})
	if err != nil {
		return out, err
	}
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var run string
		if err := tx.QueryRow(ctx, `SELECT lattis.begin_table_push($1::uuid, $2::jsonb)::text`, pipelineID, head).Scan(&run); err != nil {
			return err
		}
		for i := 0; i < len(p.Rows); i += pushChunk {
			chunk := p.Rows[i:min(i+pushChunk, len(p.Rows))]
			rows := make([][]any, len(chunk))
			for j, r := range chunk {
				row := make([]any, len(r))
				for k, v := range r {
					row[k] = PushJSON(v)
				}
				rows[j] = row
			}
			b, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `SELECT lattis.push_table_rows($1::uuid, $2::jsonb)`, run, b); err != nil {
				return err
			}
		}
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT lattis.finish_table_push($1::uuid)::text`, run).Scan(&raw); err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return fmt.Errorf("finish_table_push : %w", err)
		}
		return nil
	})
	return out, err
}

func (s pgServer) TableSources(ctx context.Context) ([]TableSource, error) {
	var out []TableSource
	err := s.jsonCall(ctx, `SELECT lattis.my_table_sources()::text`, &out)
	return out, err
}

func (s pgServer) DropTableSource(ctx context.Context, pipelineID string) error {
	return s.exec(ctx, `SELECT lattis.drop_table_source($1::uuid)`, pipelineID)
}
