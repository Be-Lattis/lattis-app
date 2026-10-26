package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"slices"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Genres de connexion. Une connexion est enregistrée dans le classeur sans
// son mot de passe (voir secrets.go).
const (
	KindPostgres  = "postgres"  // PostgreSQL : Host, Port, Database, User, SSLMode
	KindMySQL     = "mysql"     // MySQL ou MariaDB : Host, Port, Database, User, SSLMode
	KindSQLServer = "sqlserver" // SQL Server : Host, Port, Instance, Database, User, SSLMode
	KindWeb       = "web"       // API web : URL (préfixe), Header, User
	// Serveur Lattis (classeurs partagés) : URL du serveur, Label (nom
	// affiché lu à l'ajout dans sa découverte). Pas de mot de passe : la
	// session du compte (voir lattis.go).
	KindLattis = "lattis"
)

// ConnectionKinds liste les genres de connexion, dans l'ordre de
// l'interface.
var ConnectionKinds = []string{KindPostgres, KindMySQL, KindSQLServer, KindWeb, KindLattis}

// DBConfig décrit une connexion, sans mot de passe. Les champs utilisés
// dépendent du genre (voir les constantes Kind*).
type DBConfig struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	User     string `json:"user,omitempty"`
	// SSLMode : "" (selon le serveur), disable, require, verify-full
	// (PostgreSQL accepte aussi prefer, verify-ca).
	SSLMode string `json:"sslmode,omitempty"`
	// SQL Server : instance nommée (SQLEXPRESS…).
	Instance string `json:"instance,omitempty"`
	// Web : adresse (préfixe) des requêtes autorisées à recevoir le secret,
	// et en-tête qui le porte (Authorization par défaut ; avec User :
	// authentification Basic).
	URL    string `json:"url,omitempty"`
	Header string `json:"header,omitempty"`
	// Lattis : nom affiché du serveur (« Serveur Acme »), qui remplace le
	// genre dans les listes.
	Label string `json:"label,omitempty"`
}

// Connection est une connexion du classeur, décodée.
type Connection struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Config DBConfig `json:"config"`
	// Remembered : le trousseau du système connaît son mot de passe.
	Remembered bool `json:"remembered"`
}

// KindLabel nomme un genre de connexion.
func KindLabel(kind string) string {
	switch kind {
	case KindPostgres:
		return "PostgreSQL"
	case KindMySQL:
		return "MySQL"
	case KindSQLServer:
		return "SQL Server"
	case KindWeb:
		return "Web"
	case KindLattis:
		return "Serveur Lattis"
	}
	return kind
}

// defaultPort donne le port par défaut d'un genre de serveur.
func defaultPort(kind string) int {
	switch kind {
	case KindPostgres:
		return 5432
	case KindMySQL:
		return 3306
	case KindSQLServer:
		return 1433
	}
	return 0
}

func (c DBConfig) port(kind string) int {
	if c.Port == 0 {
		return defaultPort(kind)
	}
	return c.Port
}

// Account désigne le compte d'une connexion ; le trousseau le range par
// classeur (Secrets.For) : la même base utilisée par deux classeurs demande
// son mot de passe à chacun.
func (c Connection) Account() string {
	k := c.Kind
	if k == "" {
		k = KindPostgres
	}
	cfg := c.Config
	switch k {
	case KindWeb:
		return "web://" + cfg.User + "@" + cfg.URL
	case KindLattis:
		return lattisAccount(cfg.URL)
	case KindSQLServer:
		host := fmt.Sprintf("%s:%d", cfg.Host, cfg.port(k))
		if cfg.Instance != "" {
			host = cfg.Host + `\` + cfg.Instance
		}
		return fmt.Sprintf("sqlserver://%s@%s/%s", cfg.User, host, cfg.Database)
	}
	return fmt.Sprintf("%s://%s@%s:%d/%s", k, cfg.User, cfg.Host, cfg.port(k), cfg.Database)
}

// DecodeConnection lit une connexion enregistrée.
func DecodeConnection(d canvas.ConnectionDef) (Connection, error) {
	c := Connection{ID: d.ID, Name: d.Name, Kind: d.Kind}
	if !knownKind(d.Kind) {
		return c, msg.New("connection.unsupportedKind", msg.Args{"connection": d.Name, "kind": fmt.Sprintf("%q", d.Kind)})
	}
	if err := json.Unmarshal([]byte(d.Config), &c.Config); err != nil {
		return c, msg.Wrap(err, "connection.unreadableConfig", msg.Args{"connection": d.Name, "err": err})
	}
	return c, nil
}

func knownKind(k string) bool {
	for _, x := range ConnectionKinds {
		if x == k {
			return true
		}
	}
	return false
}

// EncodeConnection prépare une connexion pour l'enregistrement (jamais de
// mot de passe).
func EncodeConnection(c Connection) (canvas.ConnectionDef, error) {
	if c.Kind == "" {
		c.Kind = KindPostgres
	}
	if err := checkConfig(c.Kind, c.Config); err != nil {
		return canvas.ConnectionDef{}, err
	}
	b, err := json.Marshal(c.Config)
	if err != nil {
		return canvas.ConnectionDef{}, err
	}
	return canvas.ConnectionDef{ID: c.ID, Name: c.Name, Kind: c.Kind, Config: string(b)}, nil
}

// checkConfig vérifie les champs obligatoires d'une connexion.
func checkConfig(kind string, c DBConfig) error {
	blank := func(s string) bool { return strings.TrimSpace(s) == "" }
	switch kind {
	case KindPostgres:
		if blank(c.Host) || blank(c.Database) {
			return msg.New("connection.postgresRequired", nil)
		}
	case KindMySQL, KindSQLServer:
		if blank(c.Host) {
			return msg.New("connection.serverRequired", msg.Args{"kind": KindLabel(kind)})
		}
	case KindWeb:
		u, err := url.Parse(strings.TrimSpace(c.URL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return msg.New("connection.webUrl", nil)
		}
	case KindLattis:
		return checkLattis(c)
	default:
		return msg.New("connection.unsupportedType", msg.Args{"kind": fmt.Sprintf("%q", kind)})
	}
	return nil
}

// ErrPassword signale une connexion qui demande un mot de passe (inconnu ou
// refusé) : l'interface le demande puis recommence.
type ErrPassword struct {
	Connection string
	Account    string
	Message    string
	// Stored : un mot de passe est enregistré sur ce poste, mais ce classeur
	// n'a pas encore l'accord de s'en servir (Secrets.Grant).
	Stored bool
	// Lattis : adresse du Serveur Lattis où se connecter (pas de mot de
	// passe : la page de connexion du serveur).
	Lattis string
}

func (e *ErrPassword) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e *ErrPassword) Msg() msg.Message {
	if e.Lattis != "" {
		return msg.M("connection.error", msg.Args{"connection": e.Connection, "err": e.Message})
	}
	return msg.M("connection.passwordRequired", msg.Args{"connection": e.Connection, "detail": e.Message})
}

// DBTable est une table ou une vue d'une base (navigateur).
type DBTable struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	View   bool   `json:"view"`
}

// dialect décrit un SGBD : ouverture, reconnaissance d'un refus
// d'authentification, limite de l'aperçu, lecture seule, liste des tables.
type dialect struct {
	label string
	// open ouvre une connexion à un serveur (sans la tester) ; openFile
	// ouvre un fichier (SQLite) en lecture seule.
	open     func(c DBConfig, password string, dial DialFunc) (*sql.DB, error)
	openFile func(path string) (*sql.DB, error)
	// authFailed reconnaît un refus d'authentification.
	authFailed func(err error) bool
	// limit enveloppe une requête pour n'en lire que n lignes (nil : la
	// lecture s'arrête après n lignes).
	limit func(q string, n int) string
	// readOnly : transaction en lecture seule garantie par le serveur ;
	// sinon la transaction est toujours annulée et seules les requêtes de
	// lecture (SELECT, WITH…) sont acceptées.
	readOnly bool
	// inferTypes : types des colonnes inférés des valeurs (typage
	// dynamique de SQLite) plutôt que des types déclarés.
	inferTypes bool
	// tables liste les tables et vues (nil : pas de navigateur).
	tables func(ctx context.Context, db *sql.DB) ([]DBTable, error)
	// ident écrit un identifiant entre guillemets.
	ident func(s string) string
	// value convertit une valeur lue avant la conversion commune
	// (identifiants uniques de SQL Server…).
	value func(typeName string, v any) any
}

// dialects : un par genre de connexion et de fichier de base.
var dialects = map[string]*dialect{}

func dialectOf(kind string) (*dialect, error) {
	d, ok := dialects[kind]
	if !ok {
		return nil, msg.New("connection.unsupportedDb", msg.Args{"kind": fmt.Sprintf("%q", kind)})
	}
	return d, nil
}

// limitClause : SELECT * FROM (…) LIMIT n (PostgreSQL, MySQL, SQLite).
func limitClause(q string, n int) string {
	return "SELECT * FROM (" + q + "\n) AS _lattis_apercu LIMIT " + strconv.Itoa(n)
}

// quoteANSI écrit un identifiant entre guillemets doubles.
func quoteANSI(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

// infoSchemaTables liste les tables par information_schema (sauf les
// schémas système).
func infoSchemaTables(q string) func(ctx context.Context, db *sql.DB) ([]DBTable, error) {
	return func(ctx context.Context, db *sql.DB) ([]DBTable, error) {
		rs, err := db.QueryContext(ctx, q)
		if err != nil {
			return nil, err
		}
		defer rs.Close()
		out := []DBTable{}
		for rs.Next() {
			var t DBTable
			var kind string
			if err := rs.Scan(&t.Schema, &t.Name, &kind); err != nil {
				return nil, err
			}
			t.View = strings.Contains(strings.ToUpper(kind), "VIEW")
			out = append(out, t)
		}
		return out, rs.Err()
	}
}

// connectDB ouvre et teste la connexion c, avec le mot de passe connu
// (session, trousseau) ou sans ; *ErrPassword si le serveur le refuse.
func connectDB(ctx context.Context, sec *Secrets, c Connection) (*sql.DB, *dialect, error) {
	if g := GuardOf(ctx); g != nil {
		if err := g.Check(ctx, Access{Host: ConnectionHost(c)}); err != nil {
			return nil, nil, msg.Wrap(err, "connection.error", msg.Args{"connection": c.Name, "err": err})
		}
	}
	if c.Kind == KindLattis {
		return connectLattis(ctx, c)
	}
	d, err := dialectOf(c.Kind)
	if err != nil || d.open == nil {
		return nil, nil, msg.New("connection.unsupportedKind", msg.Args{"connection": c.Name, "kind": fmt.Sprintf("%q", c.Kind)})
	}
	pw, _ := sec.Get(c.Account())
	if pw == "" && sec.Pending(c.Account()) {
		// Mot de passe enregistré, pas encore autorisé pour ce classeur.
		return nil, nil, &ErrPassword{Connection: c.Name, Account: c.Account(), Message: "mot de passe enregistré à autoriser", Stored: true}
	}
	db, err := d.open(c.Config, pw, dialOf(ctx))
	if err != nil {
		return nil, nil, msg.Wrap(err, "connection.error", msg.Args{"connection": c.Name, "err": err})
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		if d.authFailed(err) {
			msg := "aucun mot de passe enregistré"
			if pw != "" {
				msg = "mot de passe refusé"
			}
			return nil, nil, &ErrPassword{Connection: c.Name, Account: c.Account(), Message: msg}
		}
		return nil, nil, msg.Wrap(driverFrench(err), "connection.error", msg.Args{"connection": c.Name, "err": driverFrench(err)})
	}
	return db, d, nil
}

// TestConnection essaie une connexion avec password (vide : celui qui est
// connu) ; s'il réussit et n'est pas vide, il est gardé pour la session et,
// si remember, dans le trousseau.
func TestConnection(ctx context.Context, sec *Secrets, c Connection, password string, remember bool) error {
	if c.Kind == "" {
		c.Kind = KindPostgres
	}
	if err := checkConfig(c.Kind, c.Config); err != nil {
		return err
	}
	if c.Kind == KindWeb {
		return testWeb(ctx, sec, c, password, remember)
	}
	if c.Kind == KindLattis {
		db, _, err := connectLattis(ctx, c)
		if err != nil {
			return err
		}
		return db.Close()
	}
	if password == "" {
		db, _, err := connectDB(ctx, sec, c)
		if err != nil {
			return err
		}
		return db.Close()
	}
	d, err := dialectOf(c.Kind)
	if err != nil {
		return err
	}
	if g := GuardOf(ctx); g != nil {
		if err := g.Check(ctx, Access{Host: ConnectionHost(c)}); err != nil {
			return msg.Wrap(err, "connection.error", msg.Args{"connection": c.Name, "err": err})
		}
	}
	db, err := d.open(c.Config, password, dialOf(ctx))
	if err != nil {
		return msg.Wrap(err, "connection.error", msg.Args{"connection": c.Name, "err": err})
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		if d.authFailed(err) {
			return &ErrPassword{Connection: c.Name, Account: c.Account(), Message: "mot de passe refusé"}
		}
		return msg.Wrap(driverFrench(err), "connection.error", msg.Args{"connection": c.Name, "err": driverFrench(err)})
	}
	return sec.Set(c.Account(), password, remember)
}

// sqlType donne le type Lattis d'un type SQL (nom rendu par le pilote :
// INT4, NUMERIC, VARCHAR, DATETIME2…) ; "" : à inférer des valeurs.
func sqlType(name string) grid.Type {
	n := strings.ToUpper(strings.TrimSpace(name))
	if i := strings.IndexByte(n, '('); i >= 0 {
		n = strings.TrimSpace(n[:i])
	}
	n = strings.TrimPrefix(n, "UNSIGNED ")
	switch n {
	case "":
		return ""
	case "INT2", "INT4", "INT8", "OID", "XID", "CID", "INT", "INTEGER", "SMALLINT", "TINYINT", "MEDIUMINT", "BIGINT", "YEAR",
		"UTINYINT", "USMALLINT", "UINTEGER", "INT1", "INT16", "INT32", "INT64", "SERIAL", "BIGSERIAL":
		return grid.Integer
	case "FLOAT4", "FLOAT8", "NUMERIC", "DECIMAL", "MONEY", "SMALLMONEY", "REAL", "FLOAT", "DOUBLE", "DOUBLE PRECISION",
		"HUGEINT", "UBIGINT", "UHUGEINT", "NUMBER":
		return grid.Decimal
	case "BOOL", "BOOLEAN", "BIT", "LOGICAL":
		return grid.Boolean
	case "DATE":
		return grid.Date
	}
	return grid.Text
}

// SQLType donne le type Lattis d'un type SQL tel que le rend
// DatabaseTypeName (INT8, NUMERIC, TEXT…) ; "" : à inférer des valeurs.
func SQLType(name string) grid.Type { return sqlType(name) }

// StorageValue ramène une valeur lue par database/sql au stockage d'une
// colonne de type t, comme le chargement d'une requête (relevés des
// tableaux liés aux données du Data Lake).
func StorageValue(v any, t grid.Type) any { return dbValue(v, t) }

// dbValue ramène une valeur lue par un pilote au stockage de son type
// Lattis.
func dbValue(v any, t grid.Type) any {
	switch x := v.(type) {
	case nil:
		return nil
	case time.Time:
		if t == grid.Date {
			return x.Format("2006-01-02")
		}
		if x.Location() != time.UTC {
			x = x.Local()
		}
		return x.Format("2006-01-02 15:04:05.999999")
	case bool:
		if x {
			return int64(1)
		}
		return int64(0)
	case []byte:
		if t == grid.Boolean && len(x) == 1 && x[0] <= 1 {
			return int64(x[0]) // BIT(1) de MySQL
		}
		v = string(x)
	case int:
		v = int64(x)
	case int32:
		v = int64(x)
	case int16:
		v = int64(x)
	case int8:
		v = int64(x)
	case uint64:
		if x > 1<<63-1 {
			v = float64(x)
		} else {
			v = int64(x)
		}
	case uint32:
		v = int64(x)
	case uint16:
		v = int64(x)
	case uint8:
		v = int64(x)
	case float32:
		v = float64(x)
	}
	switch t {
	case grid.Integer:
		if s, ok := v.(string); ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				return n
			}
			if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				return f
			}
			return nil
		}
	case grid.Decimal:
		if s, ok := v.(string); ok {
			s = strings.NewReplacer("$", "", "€", "", ",", "").Replace(s)
			if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				return f
			}
			return nil
		}
		if n, ok := v.(int64); ok {
			return float64(n)
		}
	case grid.Boolean:
		if s, ok := v.(string); ok {
			switch strings.ToLower(strings.TrimSpace(s)) {
			case "1", "t", "true", "y", "yes":
				return int64(1)
			case "0", "f", "false", "n", "no":
				return int64(0)
			}
			return nil
		}
	case grid.Date:
		if s, ok := v.(string); ok && len(s) >= 10 {
			return s[:10]
		}
	case grid.Text:
		switch x := v.(type) {
		case int64:
			return strconv.FormatInt(x, 10)
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		}
	}
	switch v.(type) {
	case nil, int64, float64, string:
		return v
	}
	return fmt.Sprint(v)
}

// trimSQL retire les espaces et points-virgules de fin d'une requête.
func trimSQL(q string) string {
	return strings.TrimRight(strings.TrimSpace(q), "; \t\r\n")
}

// readKeywords : premiers mots d'une requête de lecture, pour les bases
// sans transaction en lecture seule.
var readKeywords = map[string]bool{"SELECT": true, "WITH": true, "VALUES": true, "TABLE": true, "SHOW": true, "DESCRIBE": true, "DESC": true, "EXPLAIN": true}

// firstKeyword donne le premier mot d'une requête, commentaires et
// parenthèses sautés.
func firstKeyword(q string) string {
	for {
		q = strings.TrimLeft(q, " \t\r\n(")
		switch {
		case strings.HasPrefix(q, "--"):
			if i := strings.IndexByte(q, '\n'); i >= 0 {
				q = q[i+1:]
				continue
			}
			return ""
		case strings.HasPrefix(q, "/*"):
			if i := strings.Index(q, "*/"); i >= 0 {
				q = q[i+2:]
				continue
			}
			return ""
		}
		break
	}
	end := strings.IndexFunc(q, func(r rune) bool {
		return !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
	})
	if end < 0 {
		end = len(q)
	}
	return strings.ToUpper(q[:end])
}

// readDB exécute une requête en lecture seule ; limit > 0 n'en lit que
// les premières lignes (poussé au serveur quand le dialecte le permet).
func readDB(ctx context.Context, db *sql.DB, d *dialect, q string, limit int, header func([]Column) error, each func([]any) error) error {
	q = trimSQL(q)
	if q == "" {
		return msg.New("sql.empty", nil)
	}
	if !d.readOnly && !readKeywords[firstKeyword(q)] {
		return msg.New("query.readOnly", msg.Args{"keyword": firstKeyword(q)})
	}
	if limit > 0 && d.limit != nil {
		q = d.limit(q, limit)
	}
	var opts *sql.TxOptions
	if d.readOnly {
		opts = &sql.TxOptions{ReadOnly: true}
	}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	// Toujours annulée : rien de ce que la requête aurait écrit ne reste.
	defer tx.Rollback()
	rs, err := tx.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rs.Close()
	cts, err := rs.ColumnTypes()
	if err != nil {
		return err
	}
	cols := make([]Column, len(cts))
	names := make([]string, len(cts))
	for i, ct := range cts {
		names[i] = ct.DatabaseTypeName()
		cols[i] = Column{Name: ct.Name(), Type: sqlType(names[i])}
		if d.inferTypes {
			cols[i].Type = ""
		}
	}
	if err := header(cols); err != nil {
		return err
	}
	dest := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range dest {
		ptrs[i] = &dest[i]
	}
	row := make([]any, len(cols))
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return err
		}
		for i, v := range dest {
			if d.value != nil {
				v = d.value(names[i], v)
			}
			row[i] = dbValue(v, cols[i].Type)
		}
		if err := each(row); err != nil {
			return err
		}
	}
	return rs.Err()
}

// DBTables liste les tables et vues d'une connexion (navigateur).
func DBTables(ctx context.Context, sec *Secrets, c Connection) ([]DBTable, error) {
	if c.Kind == KindWeb {
		return []DBTable{}, nil
	}
	db, d, err := connectDB(ctx, sec, c)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return listTables(ctx, db, d)
}

func listTables(ctx context.Context, db *sql.DB, d *dialect) ([]DBTable, error) {
	if d.tables == nil {
		return []DBTable{}, nil
	}
	ts, err := d.tables(ctx, db)
	if err != nil {
		return nil, err
	}
	ts = slices.DeleteFunc(ts, internalTable)
	if len(ts) > 2000 {
		ts = ts[:2000]
	}
	return ts, nil
}

// internalTable : table propre à Lattis, cachée des listes de tables d'une
// source (un classeur .lattis ouvert comme base, un Serveur Lattis : ses
// tables lattis_meta_* ; son schéma interne lattis est déjà caché par le
// dialecte Serveur Lattis, et reste visible d'une base PostgreSQL quelconque).
func internalTable(t DBTable) bool {
	n := strings.ToLower(t.Name)
	return strings.HasPrefix(n, "_lattis_") || strings.HasPrefix(n, "lattis_meta_")
}

// FileTables liste les tables et vues d'un fichier de base (SQLite).
func FileTables(ctx context.Context, kind, path string) ([]DBTable, error) {
	if g := GuardOf(ctx); g != nil {
		if err := g.CheckPath(path); err != nil {
			return nil, err
		}
	}
	db, d, err := openFileDB(kind, path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return listTables(ctx, db, d)
}

// openFileDB ouvre un fichier de base en lecture seule.
func openFileDB(kind, path string) (*sql.DB, *dialect, error) {
	d, err := dialectOf(kind)
	if err != nil || d.openFile == nil {
		return nil, nil, msg.New("connection.unsupportedFileDb", msg.Args{"kind": fmt.Sprintf("%q", kind)})
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil, msg.New("connection.noFileDb", nil)
	}
	db, err := d.openFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%s %s : %w", d.label, baseName(path), err)
	}
	return db, d, nil
}

// TableSQL écrit la requête qui lit une table entière d'une base du genre
// kind (connexion ou fichier).
func TableSQL(kind, schema, table string) string {
	ident := quoteANSI
	if d, ok := dialects[kind]; ok && d.ident != nil {
		ident = d.ident
	}
	if schema == "" {
		return "SELECT * FROM " + ident(table)
	}
	return "SELECT * FROM " + ident(schema) + "." + ident(table)
}

// sortTables range les tables : schéma préféré d'abord, puis par nom.
func sortTables(ts []DBTable, preferred string) {
	sort.SliceStable(ts, func(i, j int) bool {
		pi, pj := strings.EqualFold(ts[i].Schema, preferred), strings.EqualFold(ts[j].Schema, preferred)
		if pi != pj {
			return pi
		}
		if ts[i].Schema != ts[j].Schema {
			return ts[i].Schema < ts[j].Schema
		}
		return ts[i].Name < ts[j].Name
	})
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `\/`); i >= 0 {
		return p[i+1:]
	}
	return p
}
