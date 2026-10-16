package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/grid"
)

// KindPostgres est le genre d'une connexion PostgreSQL.
const KindPostgres = "postgres"

// PGConfig décrit une connexion PostgreSQL, sans mot de passe (voir
// secrets.go).
type PGConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	// SSLMode : disable, prefer (défaut), require, verify-ca, verify-full.
	SSLMode string `json:"sslmode,omitempty"`
}

// Account désigne la base pour le trousseau : la même base utilisée par
// deux classeurs partage son mot de passe.
func (c PGConfig) Account() string {
	return fmt.Sprintf("postgres://%s@%s:%d/%s", c.User, c.Host, c.port(), c.Database)
}

func (c PGConfig) port() int {
	if c.Port == 0 {
		return 5432
	}
	return c.Port
}

// Connection est une connexion du classeur, décodée.
type Connection struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Postgres PGConfig `json:"postgres"`
	// Remembered : le trousseau du système connaît son mot de passe.
	Remembered bool `json:"remembered"`
}

// DecodeConnection lit une connexion enregistrée.
func DecodeConnection(d canvas.ConnectionDef) (Connection, error) {
	c := Connection{ID: d.ID, Name: d.Name, Kind: d.Kind}
	if d.Kind != KindPostgres {
		return c, fmt.Errorf("connexion « %s » : type %q non pris en charge", d.Name, d.Kind)
	}
	if err := json.Unmarshal([]byte(d.Config), &c.Postgres); err != nil {
		return c, fmt.Errorf("connexion « %s » : configuration illisible : %w", d.Name, err)
	}
	return c, nil
}

// EncodeConnection prépare une connexion pour l'enregistrement (jamais de
// mot de passe).
func EncodeConnection(c Connection) (canvas.ConnectionDef, error) {
	if c.Kind == "" {
		c.Kind = KindPostgres
	}
	if c.Kind != KindPostgres {
		return canvas.ConnectionDef{}, fmt.Errorf("type de connexion %q non pris en charge", c.Kind)
	}
	if strings.TrimSpace(c.Postgres.Host) == "" || strings.TrimSpace(c.Postgres.Database) == "" {
		return canvas.ConnectionDef{}, errors.New("connexion PostgreSQL : hôte et base sont obligatoires")
	}
	b, err := json.Marshal(c.Postgres)
	if err != nil {
		return canvas.ConnectionDef{}, err
	}
	return canvas.ConnectionDef{ID: c.ID, Name: c.Name, Kind: c.Kind, Config: string(b)}, nil
}

// ErrPassword signale une connexion qui demande un mot de passe (inconnu ou
// refusé) : l'interface le demande puis recommence.
type ErrPassword struct {
	Connection string
	Account    string
	Message    string
}

func (e *ErrPassword) Error() string {
	return fmt.Sprintf("connexion « %s » : mot de passe requis (%s)", e.Connection, e.Message)
}

// dsnValue écrit une valeur du format clé='valeur' de libpq.
func dsnValue(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// openPG ouvre une connexion (sans la tester). Un mot de passe vide laisse
// pgx chercher PGPASSWORD et le fichier .pgpass.
func openPG(c PGConfig, password string) (*sql.DB, error) {
	parts := []string{
		"host=" + dsnValue(c.Host),
		"port=" + strconv.Itoa(c.port()),
		"dbname=" + dsnValue(c.Database),
		"connect_timeout=10",
		"application_name=Slitable",
	}
	if c.User != "" {
		parts = append(parts, "user="+dsnValue(c.User))
	}
	if c.SSLMode != "" {
		parts = append(parts, "sslmode="+dsnValue(c.SSLMode))
	}
	if password != "" {
		parts = append(parts, "password="+dsnValue(password))
	}
	cfg, err := pgx.ParseConfig(strings.Join(parts, " "))
	if err != nil {
		return nil, err
	}
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(1)
	return db, nil
}

// authFailed reconnaît un refus d'authentification.
func authFailed(err error) bool {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code == "28P01" || pe.Code == "28000"
	}
	return strings.Contains(err.Error(), "password authentication failed") || strings.Contains(err.Error(), "no password")
}

// connectPG ouvre et teste la connexion name, avec le mot de passe connu
// (session, trousseau) ou sans ; *ErrPassword si le serveur le refuse.
func connectPG(ctx context.Context, sec *Secrets, name string, c PGConfig) (*sql.DB, error) {
	pw, _ := sec.Get(c.Account())
	db, err := openPG(c, pw)
	if err != nil {
		return nil, fmt.Errorf("connexion « %s » : %w", name, err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		if authFailed(err) {
			msg := "aucun mot de passe enregistré"
			if pw != "" {
				msg = "mot de passe refusé"
			}
			return nil, &ErrPassword{Connection: name, Account: c.Account(), Message: msg}
		}
		return nil, fmt.Errorf("connexion « %s » : %w", name, err)
	}
	return db, nil
}

// TestConnection essaie une connexion avec password (vide : celui qui est
// connu) ; s'il réussit et n'est pas vide, il est gardé pour la session et,
// si remember, dans le trousseau.
func TestConnection(ctx context.Context, sec *Secrets, c Connection, password string, remember bool) error {
	if password == "" {
		db, err := connectPG(ctx, sec, c.Name, c.Postgres)
		if err != nil {
			return err
		}
		return db.Close()
	}
	db, err := openPG(c.Postgres, password)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		if authFailed(err) {
			return &ErrPassword{Connection: c.Name, Account: c.Postgres.Account(), Message: "mot de passe refusé"}
		}
		return fmt.Errorf("connexion « %s » : %w", c.Name, err)
	}
	return sec.Set(c.Postgres.Account(), password, remember)
}

// pgType donne le type Slitable d'un type PostgreSQL (nom de
// database/sql : INT4, NUMERIC, TIMESTAMPTZ…).
func pgType(name string) grid.Type {
	switch strings.ToUpper(name) {
	case "INT2", "INT4", "INT8", "OID", "XID", "CID":
		return grid.Integer
	case "FLOAT4", "FLOAT8", "NUMERIC", "MONEY":
		return grid.Decimal
	case "BOOL":
		return grid.Boolean
	case "DATE":
		return grid.Date
	}
	return grid.Text
}

// pgValue ramène une valeur lue par pgx au stockage de son type Slitable.
func pgValue(v any, t grid.Type) any {
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
		s := x.Format("2006-01-02 15:04:05.999999")
		return s
	case bool:
		if x {
			return int64(1)
		}
		return int64(0)
	case []byte:
		v = string(x)
	}
	switch t {
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
	case grid.Text:
		switch x := v.(type) {
		case int64:
			return strconv.FormatInt(x, 10)
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		}
	}
	return v
}

// trimSQL retire les espaces et points-virgules de fin d'une requête.
func trimSQL(q string) string {
	return strings.TrimRight(strings.TrimSpace(q), "; \t\r\n")
}

// readPG exécute une requête en lecture seule (transaction READ ONLY) ;
// limit > 0 n'en lit que les premières lignes.
func readPG(ctx context.Context, db *sql.DB, q string, limit int, header func([]Column) error, each func([]any) error) error {
	q = trimSQL(q)
	if q == "" {
		return errors.New("requête SQL vide")
	}
	if limit > 0 {
		q = "SELECT * FROM (" + q + "\n) AS _slitable_apercu LIMIT " + strconv.Itoa(limit)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
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
	for i, ct := range cts {
		cols[i] = Column{Name: ct.Name(), Type: pgType(ct.DatabaseTypeName())}
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
			row[i] = pgValue(v, cols[i].Type)
		}
		if err := each(row); err != nil {
			return err
		}
	}
	return rs.Err()
}

// PGTable est une table ou une vue d'un serveur PostgreSQL (navigateur).
type PGTable struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	View   bool   `json:"view"`
}

// PGTables liste les tables et vues visibles d'une connexion.
func PGTables(ctx context.Context, sec *Secrets, c Connection) ([]PGTable, error) {
	db, err := connectPG(ctx, sec, c.Name, c.Postgres)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rs, err := db.QueryContext(ctx, `SELECT table_schema, table_name, table_type = 'VIEW'
		FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema = 'public' DESC, table_schema, table_name LIMIT 2000`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []PGTable{}
	for rs.Next() {
		var t PGTable
		if err := rs.Scan(&t.Schema, &t.Name, &t.View); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rs.Err()
}

// PGIdent écrit un identifiant PostgreSQL entre guillemets si besoin.
func PGIdent(s string) string {
	simple := s != ""
	for i, r := range s {
		if !(r == '_' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			simple = false
		}
	}
	if simple {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
