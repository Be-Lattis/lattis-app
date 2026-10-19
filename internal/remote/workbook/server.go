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
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	WorkbookID string `json:"workbook_id"`
	// Title : titre du classeur (serveur à partir de la migration 0007 ;
	// vide avant).
	Title     string                 `json:"title"`
	Mode      string                 `json:"mode"`
	Version   int                    `json:"version"`
	Schema    string                 `json:"schema"`
	Channel   string                 `json:"channel"`
	Roles     []string               `json:"roles"`
	CanDesign bool                   `json:"can_design"`
	CanManage bool                   `json:"can_manage"`
	Tables    map[string]TableAccess `json:"tables"`
	// CustomRoles : rôles personnalisés de l'utilisateur (noms).
	CustomRoles []string `json:"custom_roles"`
	// Restrictions : restrictions qui visent l'utilisateur (§8.2).
	Restrictions []Restriction `json:"restrictions"`
}

// TableAccess est le niveau effectif sur un tableau du serveur.
type TableAccess struct {
	Name  string `json:"name"`
	Level string `json:"level"` // none, reader, participant, editor
	// Filtered : un filtre de lecture ou d'écriture d'un de ses rôles
	// s'applique au tableau (gardé par le serveur pour compatibilité).
	Filtered bool `json:"filtered"`
	// SheetID : feuille du tableau pour le serveur (SheetKey de la feuille
	// locale à la publication ; vide : inconnue). Migration 0009.
	SheetID string `json:"sheet_id"`
	// ReadFiltered : un filtre de lecture ou une restriction select/all me
	// vise (« Vue filtrée par vos droits ») ; WriteFiltered : un filtre
	// d'écriture ou une restriction insert/update/delete/all. nil : serveur
	// sans la migration 0009 (voir ReadsFiltered).
	ReadFiltered  *bool `json:"read_filtered"`
	WriteFiltered *bool `json:"write_filtered"`
}

// Restriction est une restriction qui vise l'utilisateur
// (my_capabilities.restrictions) : commande (select, insert, update,
// delete, all) et libellé ; le prédicat n'est connu que du propriétaire.
type Restriction struct {
	TableID string `json:"table_id"`
	Command string `json:"command"`
	Label   string `json:"label"`
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
	// Key : clé du tableau sur le serveur (key_column), telle que le poste
	// l'a vue en dernier (gardée dans _lattis_remote_tables.columns : un
	// changement de clé fait ailleurs se reconnaît par différence).
	Key bool `json:"key,omitempty"`
}

// RemoteTable est un tableau du serveur (<schéma>.lattis_meta_tables).
type RemoteTable struct {
	ID       string
	Name     string
	LocalID  int64
	Position int
	Columns  []RemoteColumn
	// Key : identifiant de la colonne clé sur le serveur (key_column, posée
	// par lattis.set_key), vide sans clé.
	Key string
}

// Index est un index posé par lattis.create_index (lattis.table_indexes).
type Index struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Columns []string `json:"columns"` // noms des colonnes sur le serveur
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
	// CanCreateWorkbooks dit si l'utilisateur peut publier un nouveau
	// classeur (lattis.login_summary, can_create_workbooks : rôle « créateur
	// de classeurs », propriétaire du serveur, ou publication ouverte à tous).
	// Vrai pour un serveur qui ne connaît pas encore ce droit.
	CanCreateWorkbooks(ctx context.Context) (bool, error)
	// RecordOpen note que l'utilisateur a ouvert le classeur id (« dernier
	// lecteur » de l'administration du serveur) ; sans effet sur un serveur
	// d'avant cette fonction.
	RecordOpen(ctx context.Context, id string) error
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
	// Delta exécute fn dans une transaction de lecture cohérente
	// (REPEATABLE READ) sur rw : lecture des changements d'un classeur
	// collaboratif (contrat client de lattisd, §6). Read peut lire un
	// réplica en retard ; Delta, jamais.
	Delta(ctx context.Context, fn func(Reader) error) error
	// Write exécute fn dans une transaction d'écriture sur rw : une étape
	// de la file d'envoi (tout ou rien).
	Write(ctx context.Context, fn func(Writer) error) error
	// Rights : gestion des droits d'un classeur (écran « Partager »,
	// propriétaires ; voir rights.go).
	Rights
	// Data : données du Data Lake (catalogue, partage, lecture ; pg_data.go).
	Data
}

// Change est une ligne modifiée ou supprimée depuis un point de reprise
// (lattis.changes_since), ou relue par Reader.RowsByID.
type Change struct {
	ID      string // lattis_id
	Deleted bool
	Pos     string // lattis_pos
	Author  string // lattis_author
	// UpdatedBy, UpdatedAt : dernier modificateur (lattis_updated_*).
	UpdatedBy string
	UpdatedAt string
	// Values : valeurs par nom de colonne du serveur, converties en valeurs
	// de stockage locales (FromJSON) ; une colonne absente de la ligne n'y
	// est pas.
	Values map[string]any
}

// Raisons d'un refus d'écriture (lattis.why_not).
const (
	WhyModified  = "modified"  // ligne modifiée entre-temps (Row : valeur actuelle)
	WhyDeleted   = "deleted"   // ligne supprimée entre-temps
	WhyForbidden = "forbidden" // droits insuffisants (ligne invisible ou non modifiable)
	WhyMissing   = "missing"   // ligne inconnue du serveur
)

// WhyNot explique une écriture qui n'a touché aucune ligne
// (lattis.why_not).
type WhyNot struct {
	Reason string
	// Row : la ligne actuelle (WhyModified).
	Row *Change
	// By, At : auteur et date de la modification ou de la suppression.
	By, At string
}

// Wrote est le résultat d'une écriture.
type Wrote struct {
	// Rows : lignes touchées (0 : conflit ou refus, voir WhyNot).
	Rows int64
	// Author : lattis_author de la ligne ajoutée (Insert).
	Author string
}

// Writer est la transaction d'une étape de la file d'envoi. Les valeurs
// sont des valeurs de stockage locales (entier, réel, texte, json.Number),
// converties au type de la colonne du serveur (ToRemote). Chaque écriture
// rend le nombre de lignes touchées : 0 est un conflit ou un refus
// (WhyNot), pas une erreur. Chaque écriture a son point de sauvegarde
// (contrat client §6) : une erreur SQL (Rejected : filtre d'écriture,
// droits, contrainte) ne défait qu'elle, la transaction continue ; toute
// autre erreur (coupure) la défait en entier.
type Writer interface {
	// Update : UPDATE … SET col = new WHERE lattis_id = id AND col IS NOT
	// DISTINCT FROM old.
	Update(ctx context.Context, schema, table, id string, col RemoteColumn, old, new any) (Wrote, error)
	// Insert : INSERT (lattis_id, lattis_pos, cols…) ; une ligne déjà
	// présente (renvoi après une coupure) n'est pas une erreur : 0.
	Insert(ctx context.Context, schema, table, id, pos string, cols []RemoteColumn, vals []any) (Wrote, error)
	// Delete : DELETE … WHERE lattis_id = id.
	Delete(ctx context.Context, schema, table, id string) (Wrote, error)
	// Move : UPDATE … SET lattis_pos = pos WHERE lattis_id = id AND
	// lattis_pos = old.
	Move(ctx context.Context, schema, table, id, old, pos string) (Wrote, error)
	// WhyNot explique une écriture sans effet (lattis.why_not) ; t donne
	// les colonnes pour relire la ligne actuelle.
	WhyNot(ctx context.Context, workbookID string, t RemoteTable, id string) (WhyNot, error)
	// Structure d'un tableau (concepteurs et propriétaires ; spécification
	// du serveur §9.3, schema/0025_structure.sql) : sans point de
	// sauvegarde, un refus défait toute la transaction (une opération de
	// structure se fait en entier ou pas du tout). Le serveur refuse de
	// renommer, supprimer ou convertir une colonne citée par un filtre
	// (rôle, restriction, audience) et de supprimer un tableau qui alimente
	// le Data Lake ou un pipeline ; il notifie « *structure:<table_id> ».
	RenameTable(ctx context.Context, workbookID, tableID, name string) error
	DropTable(ctx context.Context, workbookID, tableID string) error
	// AddColumn ajoute une colonne après la colonne after (vide : à la fin)
	// et rend son identifiant ; localID est noté par le serveur.
	AddColumn(ctx context.Context, workbookID, tableID, name, typ, after string, localID int64) (string, error)
	RenameColumn(ctx context.Context, workbookID, tableID, columnID, name string) error
	DropColumn(ctx context.Context, workbookID, tableID, columnID string) error
	AlterColumnType(ctx context.Context, workbookID, tableID, columnID, typ string) error
	// SetKey fait de la colonne la clé du tableau (index unique) ; vide :
	// plus de clé.
	SetKey(ctx context.Context, workbookID, tableID, columnID string) error
	// CreateIndex pose un index sur des colonnes et rend son nom, DropIndex
	// le retire, Indexes liste ceux du tableau.
	CreateIndex(ctx context.Context, workbookID, tableID string, columnIDs []string, unique bool) (string, error)
	DropIndex(ctx context.Context, workbookID, tableID, name string) error
	Indexes(ctx context.Context, workbookID, tableID string) ([]Index, error)
	// Xact rend l'identifiant (xid8, en texte) de la transaction, "" si
	// elle n'a rien écrit (pg_current_xact_id_if_assigned) : le poste le
	// garde pour écarter ses propres écritures des changements reçus
	// (Reader.Changes, skip).
	Xact(ctx context.Context) (string, error)
}

// Rejected dit si err est un refus du serveur (erreur SQL : droits,
// contrainte, valeur) plutôt qu'une coupure, et rend son code SQLSTATE.
// Un refus ne se règle pas en renvoyant la même étape.
func Rejected(err error) (string, bool) {
	var s interface{ SQLState() string }
	if errors.As(err, &s) {
		return s.SQLState(), true
	}
	return "", false
}

// XidAfter dit si l'xid8 a (en texte) est postérieur ou égal à since ("" :
// 0) ; faux si a est illisible.
func XidAfter(a, since string) bool {
	x, err := strconv.ParseUint(a, 10, 64)
	if err != nil {
		return false
	}
	if since == "" {
		return true
	}
	s, err := strconv.ParseUint(since, 10, 64)
	return err != nil || x >= s
}

// ServerMessage rend le texte d'un refus du serveur, sans « ERROR: » ni
// le code SQLSTATE (en français pour les fonctions du schéma lattis) ;
// err.Error() pour une autre erreur.
func ServerMessage(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Message != "" {
		return pg.Message
	}
	s := strings.TrimPrefix(err.Error(), "ERROR: ")
	if i := strings.LastIndex(s, " (SQLSTATE "); i > 0 && strings.HasSuffix(s, ")") {
		s = s[:i]
	}
	return s
}

// Privilege dit si err est un refus de droits (42501).
func Privilege(err error) bool {
	code, ok := Rejected(err)
	return ok && code == "42501"
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
	// colonnes (identifiants du serveur). sheet : feuille du tableau
	// (SheetKey), notée par un serveur qui a la migration 0009 (niveaux par
	// feuille), ignorée par les autres.
	CreateTable(ctx context.Context, id, schema, name string, localID int64, sheet string, cols []ColumnSpec) (string, []RemoteColumn, error)
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
	// Changes lit les lignes modifiées ou supprimées d'un tableau depuis
	// since (lattis.changes_since, filtré par les droits de l'utilisateur),
	// sauf celles dont la dernière écriture vient d'une des transactions
	// skip (ses propres écritures, Writer.Xact), sauf si un autre auteur
	// (rôle et poste, lattis.device) l'a écrite depuis since
	// (lattis_other_xid) : le poste a pu recouvrir une valeur pas encore
	// reçue. skip est ignoré par un serveur sans la version finale de la
	// migration 0008 (sans lattis_other_xid). Une ligne peut revenir deux
	// fois : l'application est idempotente.
	Changes(ctx context.Context, workbookID string, t RemoteTable, since string, skip []string, fn func(Change) error) error
	// Covered dit si les changements depuis since sont complets
	// (lattis.since_covered) : faux, des pierres tombales ont été purgées
	// depuis, le tableau est à recharger en entier (Rows). known faux : le
	// serveur ne sait pas le dire (sans la migration 0008).
	Covered(ctx context.Context, since string) (covered, known bool, err error)
	// Probe rend le témoin d'un rechargement, à garder tant que
	// Covered(since) seul reste faux à cause d'une transaction longue
	// (since, pg_snapshot_xmin, ne dépasse pas la purge tant qu'elle dure) :
	// l'instantané de la lecture (pg_current_snapshot, migration 0010), ou,
	// sans elle, sa fin (pg_snapshot_xmax : une transaction d'identifiant
	// inférieur est validée et lue, ou était en cours).
	Probe(ctx context.Context) (string, error)
	// CoveredBy dit si les changements depuis since sont complets compte
	// tenu du témoin probe d'un rechargement : instantané,
	// lattis.since_covered(since, probe) (la dernière purge y est visible :
	// le rechargement a vu ces suppressions) ; point de contrôle,
	// Covered(probe). Un instantané sur un serveur sans 0010 : faux.
	CoveredBy(ctx context.Context, since, probe string) (bool, error)
	// RowsByID relit des lignes par leur identifiant (absentes : invisibles ou
	// supprimées) : remise à l'état du serveur après un refus.
	RowsByID(ctx context.Context, schema string, t RemoteTable, ids []string, fn func(Change) error) error
}
