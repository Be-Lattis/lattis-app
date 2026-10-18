// Package workbook publie un classeur sur un Serveur Lattis, l'ouvre depuis
// le serveur dans un cache local et tient l'état d'un classeur partagé
// ouvert (mode figé ou collaboratif, en ligne ou hors ligne, lecture seule).
// Spécification de l'application collaborative, §5.1, §5.3, §5.4, §9 et
// §10 ; contrat client de lattisd, §5. Voir docs/collaboration/publication.md.
//
// Tout ce qui parle au serveur passe par l'interface Server (API SQL du
// schéma lattis) : PG l'implémente avec pgx, workbooktest.Fake en mémoire
// pour les essais.
package workbook

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Modes d'un classeur partagé (lattis.workbooks.mode).
const (
	Frozen        = "frozen"
	Collaborative = "collaborative"
)

// Listing est un classeur accessible (lattis.my_workbooks()).
type Listing struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Mode        string   `json:"mode"`
	State       string   `json:"state"` // draft, published, republishing
	Version     int      `json:"version"`
	Schema      string   `json:"schema"`
	Roles       []string `json:"roles"`
	PublishedAt string   `json:"publishedAt,omitempty"`
}

// Capabilities sont les droits de l'utilisateur sur un classeur
// (lattis.my_capabilities(id)).
type Capabilities struct {
	WorkbookID string                 `json:"workbook_id"`
	Mode       string                 `json:"mode"`
	Version    int                    `json:"version"`
	Schema     string                 `json:"schema"`
	Channel    string                 `json:"channel"`
	Roles      []string               `json:"roles"`
	CanDesign  bool                   `json:"can_design"`
	CanManage  bool                   `json:"can_manage"`
	Tables     map[string]TableAccess `json:"tables"`
}

// TableAccess est le niveau effectif sur un tableau du serveur.
type TableAccess struct {
	Name  string `json:"name"`
	Level string `json:"level"` // none, reader, participant, editor
}

// ColumnSpec décrit une colonne à créer (lattis.create_table).
type ColumnSpec struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // text, integer, decimal, date, datetime, boolean
	LocalID int64  `json:"-"`
}

// RemoteColumn est une colonne d'un tableau du serveur
// (<schéma>.lattis_meta_columns).
type RemoteColumn struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	LocalID  int64  `json:"local"`
	Type     string `json:"type"`
	Position int    `json:"-"`
}

// RemoteTable est un tableau du serveur (<schéma>.lattis_meta_tables).
type RemoteTable struct {
	ID       string
	Name     string
	LocalID  int64
	Position int
	Columns  []RemoteColumn
}

// Script est la dernière version d'un script publié
// (<schéma>.lattis_meta_script_versions).
type Script struct {
	Name    string
	Version int
	Source  string
	SHA256  string
}

// Server est l'API SQL d'un Serveur Lattis, sous l'identité de
// l'utilisateur connecté.
type Server interface {
	// Workbooks liste les classeurs accessibles (« Ouvrir depuis un
	// serveur »).
	Workbooks(ctx context.Context) ([]Listing, error)
	// Capabilities lit les droits sur un classeur (version courante).
	Capabilities(ctx context.Context, id string) (Capabilities, error)
	// Publish exécute fn dans une seule transaction (tout ou rien).
	Publish(ctx context.Context, fn func(Publisher) error) error
	// Read exécute fn dans une transaction de lecture cohérente
	// (REPEATABLE READ).
	Read(ctx context.Context, fn func(Reader) error) error
	// Listen écoute le canal d'un classeur (LISTEN) et appelle fn à chaque
	// notification, jusqu'à l'annulation de ctx ou une coupure (erreur).
	// ready est appelé une fois l'écoute posée.
	Listen(ctx context.Context, channel string, ready func(), fn func(payload string)) error
}

// CopySource fournit les lignes d'une copie (pgx.CopyFromSource).
type CopySource = pgx.CopyFromSource

// Publisher est la transaction d'une publication ou d'une republication.
type Publisher interface {
	CreateWorkbook(ctx context.Context, title, mode string) (string, error)
	// Republish ouvre la version de travail d'un classeur figé et rend
	// son schéma.
	Republish(ctx context.Context, id string) (string, error)
	Capabilities(ctx context.Context, id string) (Capabilities, error)
	// CreateTable crée un tableau dans le schéma où l'on publie (schema :
	// le brouillon ou la version de travail) et rend son identifiant et ses
	// colonnes (identifiants du serveur).
	CreateTable(ctx context.Context, id, schema, name string, localID int64, cols []ColumnSpec) (string, []RemoteColumn, error)
	// Copy copie des lignes dans "<schéma>"."<tableau>" (COPY) ; cols
	// commence par lattis_id et lattis_pos.
	Copy(ctx context.Context, schema, table string, cols []string, rows CopySource) (int64, error)
	SaveMeta(ctx context.Context, id, kind string, payload []byte) error
	PublishScript(ctx context.Context, id, name, source string) (int, error)
	// FinishPublish rend le classeur visible (ou bascule vers la nouvelle
	// version) et rend son numéro de version.
	FinishPublish(ctx context.Context, id string) (int, error)
}

// Reader est une lecture cohérente d'un classeur.
type Reader interface {
	Capabilities(ctx context.Context, id string) (Capabilities, error)
	// SyncPoint rend le point de reprise (xid8, en texte) à mémoriser
	// comme prochain since (lattis.sync_point()).
	SyncPoint(ctx context.Context) (string, error)
	Tables(ctx context.Context, schema string) ([]RemoteTable, error)
	// Meta lit un objet de mise en page (nil s'il n'existe pas).
	Meta(ctx context.Context, schema, kind, key string) ([]byte, error)
	Scripts(ctx context.Context, schema string) ([]Script, error)
	// Rows lit les lignes d'un tableau dans l'ordre (lattis_pos, puis
	// lattis_id) ; vals suit t.Columns, converties en valeurs locales
	// (voir FromRemote).
	Rows(ctx context.Context, schema string, t RemoteTable, fn func(id, pos, author string, vals []any) error) error
}
