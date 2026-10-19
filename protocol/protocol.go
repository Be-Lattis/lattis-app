// Package protocol décrit ce que l'application Lattis et le serveur
// (lattisd) s'échangent hors PostgreSQL : découverte, jetons OAuth, résumé
// de connexion et liens vers un classeur (docs/contrat-client.md du
// serveur, §1, §2, §4 et §8). Il ne dépend que de la bibliothèque standard,
// pour être importé des deux côtés.
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Chemins par défaut de lattisd (spécification du serveur, §11).
const (
	PathDiscovery = "/.well-known/lattis.json"
	PathAuthorize = "/oauth/authorize"
	PathToken     = "/oauth/token"
	PathDevice    = "/oauth/device"
	PathRevoke    = "/oauth/revoke"
	// PathWorkbook préfixe le lien partagé vers un classeur (§8).
	PathWorkbook = "/w/"
)

// DesktopClientID est l'identifiant de client public de l'application de
// bureau.
const DesktopClientID = "lattis-desktop"

// DefaultScope est le scope demandé quand la découverte n'en donne pas.
const DefaultScope = "openid lattis"

// ProviderPassword est le genre du fournisseur « mot de passe Lattis ».
const ProviderPassword = "password"

// DefaultDatabase : base des classeurs quand la découverte ne la nomme pas.
const DefaultDatabase = "lattis"

// Discovery est le document /.well-known/lattis.json (§1).
type Discovery struct {
	// Name : nom affiché de la source (« Serveur Acme »).
	Name string `json:"name"`
	// Logo : adresse (absolue ou relative au serveur) ou URI data: d'une
	// image.
	Logo string `json:"logo,omitempty"`
	// Version de lattisd.
	Version string `json:"version,omitempty"`
	// MinClientVersion : version minimale de Lattis acceptée.
	MinClientVersion string `json:"min_client_version,omitempty"`
	// Edition : admin ou web.
	Edition string `json:"edition,omitempty"`
	// AccessMode : invitation ou open.
	AccessMode string `json:"access_mode,omitempty"`
	// Issuer : émetteur des jetons, comparé au paramètre iss du retour
	// (RFC 9207).
	Issuer string `json:"issuer,omitempty"`
	// ClientID, Scope : client public et scope à demander (DesktopClientID
	// et DefaultScope par défaut).
	ClientID string `json:"client_id,omitempty"`
	Scope    string `json:"scope,omitempty"`
	// Points OAuth (chemins Path* du serveur par défaut).
	AuthorizationEndpoint string `json:"authorization_endpoint,omitempty"`
	TokenEndpoint         string `json:"token_endpoint,omitempty"`
	RevocationEndpoint    string `json:"revocation_endpoint,omitempty"`
	DeviceEndpoint        string `json:"device_authorization_endpoint,omitempty"`
	// SourceURL : code source du serveur (AGPL-3.0 §13 : offert aux
	// utilisateurs réseau, affiché dans « À propos » de l'édition web).
	SourceURL string `json:"source_url,omitempty"`
	// PreviewEndpoint : « Voir en tant que… » (POST, jeton d'accès ; §7).
	PreviewEndpoint string     `json:"preview_endpoint,omitempty"`
	Providers       []Provider `json:"providers"`
	Postgres        Postgres   `json:"postgres"`
}

// Provider est un fournisseur d'identité actif, dans l'ordre du serveur.
type Provider struct {
	// ID : identifiant passé à /oauth/authorize (paramètre provider).
	ID string `json:"id"`
	// Kind : password, google, google_workspace, microsoft, keycloak, oidc.
	Kind  string `json:"kind"`
	Label string `json:"label"`
	// Icon : image, ou le genre (« password ») que le client traduit en icône.
	Icon string `json:"icon,omitempty"`
}

// Postgres décrit les points de connexion PostgreSQL annoncés.
type Postgres struct {
	// RW : parent.
	RW Endpoint `json:"rw"`
	// RO : réplicas en lecture (vide : tout passe par RW).
	RO []Endpoint `json:"ro,omitempty"`
	// Port : port par défaut des points qui n'en donnent pas.
	Port     int    `json:"port,omitempty"`
	SSLMode  string `json:"sslmode,omitempty"`
	Database string `json:"database,omitempty"`
}

// Endpoint est un point de connexion PostgreSQL, « hôte » ou « hôte:port ».
// Dans la découverte : un objet {"host", "port"} ou un texte.
type Endpoint string

func (e *Endpoint) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*e = Endpoint(s)
		return nil
	}
	var o struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return fmt.Errorf("point de connexion PostgreSQL illisible : %s", b)
	}
	if o.Port > 0 {
		*e = Endpoint(net.JoinHostPort(o.Host, strconv.Itoa(o.Port)))
	} else {
		*e = Endpoint(o.Host)
	}
	return nil
}

// Strings donne des points sous forme de textes.
func Strings(eps []Endpoint) []string {
	out := make([]string, len(eps))
	for i, e := range eps {
		out[i] = string(e)
	}
	return out
}

// TokenResponse est la réponse de /oauth/token (§2).
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	// PGRole : rôle PostgreSQL de l'utilisateur, nom d'utilisateur de la
	// connexion PostgreSQL.
	PGRole string `json:"pg_role"`
}

// LoginSummary est le résultat de lattis.login_summary() (§4).
type LoginSummary struct {
	Server SummaryServer `json:"server"`
	User   *SummaryUser  `json:"user"`
	// ServerOwner : propriétaire du serveur.
	ServerOwner bool `json:"server_owner"`
	// WorkbookCreator : rôle « créateur de classeurs ».
	WorkbookCreator bool `json:"workbook_creator"`
	// CanCreateWorkbooks : peut publier un classeur (créateur, propriétaire,
	// ou publication ouverte à tous). Nil : serveur d'avant ce droit.
	CanCreateWorkbooks *bool `json:"can_create_workbooks,omitempty"`
	// DataPublisher : rôle « publieur de données » ; CanPublishData : peut
	// partager des tableaux comme données d'entreprise (§9).
	DataPublisher  bool  `json:"data_publisher"`
	CanPublishData *bool `json:"can_publish_data,omitempty"`
	// Workbooks : lattis.my_workbooks(), tel quel.
	Workbooks json.RawMessage `json:"workbooks,omitempty"`
}

// SummaryServer est le serveur de LoginSummary.
type SummaryServer struct {
	DisplayName string `json:"display_name"`
	AccessMode  string `json:"access_mode"`
}

// SummaryUser est l'utilisateur de LoginSummary.
type SummaryUser struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	PGRole      string `json:"pg_role"`
	LastLoginAt string `json:"last_login_at,omitempty"`
}

// CanCreate dit si le compte peut publier ; vrai pour un serveur qui ne
// connaît pas encore ce droit.
func (s LoginSummary) CanCreate() bool {
	return s.CanCreateWorkbooks == nil || *s.CanCreateWorkbooks
}

// DesktopScheme est le schéma des liens qui ouvrent l'application (§8).
const DesktopScheme = "lattis"

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// WorkbookLink donne l'adresse à partager d'un classeur : <issuer>/w/<id>.
func WorkbookLink(issuer, id string) string {
	return strings.TrimRight(issuer, "/") + PathWorkbook + url.PathEscape(id)
}

// DesktopLink donne le lien qui ouvre un classeur dans l'application :
// lattis://open?server=<issuer encodé>&workbook=<id>.
func DesktopLink(issuer, id string) string {
	q := url.Values{"server": {strings.TrimRight(issuer, "/")}, "workbook": {id}}
	return DesktopScheme + "://open?" + q.Encode()
}

// ErrDesktopLink signale un lien lattis:// refusé.
var ErrDesktopLink = errors.New("lien Lattis invalide")

// ParseDesktopLink lit un lien lattis://open : l'émetteur (adresse http ou
// https, sans identifiants, requête ni fragment) et l'identifiant (UUID, en
// minuscules) du classeur. Tout autre hôte que open est refusé ; aucun
// autre paramètre n'est lu.
func ParseDesktopLink(raw string) (issuer, id string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(u.Scheme, DesktopScheme) {
		return "", "", ErrDesktopLink
	}
	if !strings.EqualFold(u.Host, "open") || (u.Path != "" && u.Path != "/") || u.User != nil {
		return "", "", fmt.Errorf("%w : action inconnue", ErrDesktopLink)
	}
	q := u.Query()
	id = q.Get("workbook")
	if !uuidRe.MatchString(id) {
		return "", "", fmt.Errorf("%w : classeur inconnu", ErrDesktopLink)
	}
	s, err := url.Parse(q.Get("server"))
	if err != nil || (s.Scheme != "https" && s.Scheme != "http") || s.Host == "" || s.User != nil || s.Fragment != "" || s.RawQuery != "" || s.Opaque != "" {
		return "", "", fmt.Errorf("%w : adresse du serveur invalide", ErrDesktopLink)
	}
	return strings.TrimRight(s.String(), "/"), strings.ToLower(id), nil
}

// --- Données d'entreprise (§9) -------------------------------------------------

// DataCatalog est le résultat de lattis.data_catalog().
type DataCatalog struct {
	// CanPublish : la personne peut partager des tableaux et gérer les dossiers.
	CanPublish bool         `json:"can_publish"`
	Folders    []DataFolder `json:"folders"`
	Datasets   []Dataset    `json:"datasets"`
}

// DataFolder est un dossier du catalogue.
type DataFolder struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id,omitempty"`
	Name     string `json:"name"`
	// Path : chemin lisible (« Ventes › Europe »).
	Path  string `json:"path"`
	Depth int    `json:"depth"`
	// Audience : pour un publieur seulement.
	Audience []DataAudience `json:"audience,omitempty"`
}

// Dataset est une donnée d'entreprise : un tableau d'un classeur publié,
// lisible par son audience dans la vue donnees."<SQLName>".
type Dataset struct {
	ID          string `json:"id"`
	FolderID    string `json:"folder_id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SQLName     string `json:"sql_name"`
	// Readable : la personne lit ses lignes ; Manageable : elle la gère.
	Readable   bool `json:"readable"`
	Manageable bool `json:"manageable"`
	// Available faux : source supprimée ou tableau introuvable.
	Available bool `json:"available"`
	// TableID et Channel : notifications du classeur source (LISTEN Channel,
	// charge TableID ou *version:<n>).
	TableID   string       `json:"table_id,omitempty"`
	Channel   string       `json:"channel,omitempty"`
	Columns   []DataColumn `json:"columns"`
	UpdatedAt string       `json:"updated_at,omitempty"`
	// Source, Audience, UsedBy : pour qui gère la donnée seulement.
	Source   *DataSource    `json:"source,omitempty"`
	Audience []DataAudience `json:"audience,omitempty"`
	UsedBy   []DataUse      `json:"used_by,omitempty"`
}

// DataColumn est une colonne de la vue (type PostgreSQL).
type DataColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DataSource est l'origine d'une donnée.
type DataSource struct {
	WorkbookID      string   `json:"workbook_id"`
	Title           string   `json:"title"`
	Table           string   `json:"table"`
	HiddenColumns   []string `json:"hidden_columns"`
	InheritAudience bool     `json:"inherit_audience"`
}

// DataAudience est une entrée de l'audience d'une donnée ou d'un dossier.
type DataAudience struct {
	ID string `json:"id"`
	// Kind : user ou team.
	Kind      string `json:"kind"`
	Principal string `json:"principal"`
	Label     string `json:"label"`
	Predicate string `json:"predicate,omitempty"`
}

// DataUse est un classeur qui utilise une donnée (« Utilisée par »).
type DataUse struct {
	WorkbookID string          `json:"workbook_id"`
	Title      string          `json:"title"`
	LinkedAt   string          `json:"linked_at,omitempty"`
	LastEdit   json.RawMessage `json:"last_edit,omitempty"`
	LastRead   *DataRead       `json:"last_read,omitempty"`
}

// DataRead est la dernière lecture d'un classeur.
type DataRead struct {
	Email string `json:"email"`
	At    string `json:"at"`
}

// Canal des notifications du catalogue et charges particulières.
const (
	DataChannel        = "lattis_data"
	DataCatalogChanged = "*catalog"
	DataWithdrawn      = "*withdrawn:"
	DataVersion        = "*version:"
)

// DataSchema est le schéma PostgreSQL des vues des données d'entreprise.
const DataSchema = "donnees"
