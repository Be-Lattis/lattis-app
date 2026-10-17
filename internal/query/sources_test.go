package query

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/parquet-go/parquet-go"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/grid"
)

// previewOf calcule l'aperçu de la source seule (relue).
func previewOf(t *testing.T, e Env, src Source) Preview {
	t.Helper()
	s := newSession(t)
	return s.Preview(ctx, e, Query{Name: "Q", Source: src}, -1, true)
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const clientsJSON = `{"total": 3, "data": [
	{"id": 1, "nom": "Durand", "solde": 10.5, "actif": true, "inscrit": "2026-01-15", "adresse": {"ville": "Paris", "cp": "75001"}, "tags": ["a", "b"]},
	{"id": 2, "nom": "Martin", "solde": 20, "actif": false, "inscrit": "2026-02-01", "adresse": {"ville": "Lyon"}},
	{"id": 3, "nom": "Petit", "solde": null, "inscrit": null, "adresse": {"ville": "Nice", "cp": "06000"}, "extra": {"x": 1}}
]}`

func TestJSONFile(t *testing.T) {
	d := newDoc(t)
	p := writeFile(t, "clients.json", clientsJSON)
	// Sans chemin : le seul tableau de la racine.
	pv := previewOf(t, env(d), Source{Kind: SourceJSON, Path: p})
	if pv.Error != "" {
		t.Fatal(pv.Error)
	}
	want := []Column{{"id", grid.Integer}, {"nom", grid.Text}, {"solde", grid.Decimal}, {"actif", grid.Boolean}, {"inscrit", grid.Date},
		{"adresse.ville", grid.Text}, {"adresse.cp", grid.Text}, {"tags", grid.Text}, {"extra.x", grid.Integer}}
	if !reflect.DeepEqual(pv.Columns, want) {
		t.Fatalf("colonnes %v", pv.Columns)
	}
	if got := fmt.Sprint(pv.Rows); got != `[[1 Durand 10,5 VRAI 15/01/2026 Paris 75001 ["a","b"] ] [2 Martin 20 FAUX 01/02/2026 Lyon   ] [3 Petit    Nice 06000  1]]` {
		t.Fatalf("lignes %s", got)
	}
	// Chemin explicite, et erreur lisible si la clé manque.
	pv = previewOf(t, env(d), Source{Kind: SourceJSON, Path: p, RecordPath: "$.data[1].adresse"})
	if pv.Error != "" || fmt.Sprint(pv.Rows) != "[[Lyon]]" {
		t.Fatalf("chemin : %v %s", pv.Rows, pv.Error)
	}
	pv = previewOf(t, env(d), Source{Kind: SourceJSON, Path: p, RecordPath: "donnees"})
	if !strings.Contains(pv.Error, "clé « donnees » introuvable") || !strings.Contains(pv.Error, "total, data") {
		t.Fatalf("erreur attendue : %q", pv.Error)
	}
	// Une valeur par ligne (NDJSON), tableau de valeurs simples.
	nd := writeFile(t, "l.ndjson", "{\"a\": 1, \"b\": \"x\"}\n{\"a\": 2.5, \"c\": true}\n")
	pv = previewOf(t, env(d), Source{Kind: SourceJSON, Path: nd})
	if fmt.Sprint(pv.Columns) != "[{a decimal} {b text} {c boolean}]" || fmt.Sprint(pv.Rows) != "[[1 x ] [2,5  VRAI]]" {
		t.Fatalf("NDJSON : %v %v %s", pv.Columns, pv.Rows, pv.Error)
	}
	sc := writeFile(t, "s.json", `[3, "4", null]`)
	pv = previewOf(t, env(d), Source{Kind: SourceJSON, Path: sc})
	if fmt.Sprint(pv.Columns) != "[{Valeur text}]" || fmt.Sprint(pv.Rows) != "[[3] [4] []]" {
		t.Fatalf("valeurs simples : %v %v %s", pv.Columns, pv.Rows, pv.Error)
	}
	bad := writeFile(t, "bad.json", `{"a": [1, 2`)
	if pv = previewOf(t, env(d), Source{Kind: SourceJSON, Path: bad}); !strings.Contains(pv.Error, "JSON illisible") {
		t.Fatalf("JSON tronqué : %q", pv.Error)
	}
}

func TestWebSource(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		switch r.URL.Path {
		case "/api/clients":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, clientsJSON)
		case "/api/prive":
			if r.Header.Get("Authorization") != "Bearer jeton" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"resultats": {"lignes": [{"k": "v"}]}}`)
		case "/api/cle":
			if r.Header.Get("X-Api-Key") != "secret" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			fmt.Fprint(w, `[{"ok": 1}]`)
		default:
			http.Error(w, "page absente", http.StatusNotFound)
		}
	}))
	defer srv.Close()
	d := newDoc(t)
	e := env(d)

	pv := previewOf(t, e, Source{Kind: SourceWeb, URL: srv.URL + "/api/clients", Headers: []Header{{Name: "X-Trace", Value: "42"}}, RecordPath: "data"})
	if pv.Error != "" || pv.Count != 3 || len(pv.Columns) != 9 {
		t.Fatalf("API : %v %s", pv.Columns, pv.Error)
	}
	if gotHeaders.Get("X-Trace") != "42" || gotHeaders.Get("Accept") != "application/json" {
		t.Fatalf("en-têtes %v", gotHeaders)
	}
	if pv = previewOf(t, e, Source{Kind: SourceWeb, URL: srv.URL + "/absent"}); !strings.Contains(pv.Error, "HTTP 404") || !strings.Contains(pv.Error, "page absente") {
		t.Fatalf("404 : %q", pv.Error)
	}
	if pv = previewOf(t, e, Source{Kind: SourceWeb, URL: "ftp://exemple"}); !strings.Contains(pv.Error, "http://") {
		t.Fatalf("adresse : %q", pv.Error)
	}
	// Sans connexion, une API protégée dit quoi faire.
	if pv = previewOf(t, e, Source{Kind: SourceWeb, URL: srv.URL + "/api/prive"}); !strings.Contains(pv.Error, "connexion Web") {
		t.Fatalf("401 sans connexion : %q", pv.Error)
	}

	// Connexion Web : le jeton vient du coffre, jamais du classeur.
	conn := Connection{Name: "API", Kind: KindWeb, Config: DBConfig{URL: srv.URL + "/api"}}
	def, err := EncodeConnection(conn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveConnection(ctx, def); err != nil {
		t.Fatal(err)
	}
	src := Source{Kind: SourceWeb, URL: srv.URL + "/api/prive", RecordPath: "resultats.lignes", Connection: "API"}
	pv = previewOf(t, e, src)
	if pv.NeedPassword == nil || pv.NeedPassword.Connection != "API" {
		t.Fatalf("secret non demandé : %+v", pv)
	}
	if err := TestConnection(ctx, e.Secrets, conn, "faux", false); err == nil {
		// La racine /api répond 404 : un serveur joignable, sans refus.
		t.Log("racine joignable")
	}
	var pe *ErrPassword
	if err := TestConnection(ctx, e.Secrets, Connection{Name: "API", Kind: KindWeb, Config: DBConfig{URL: srv.URL + "/api/prive"}}, "faux", false); !errors.As(err, &pe) {
		t.Fatalf("mauvais jeton accepté : %v", err)
	}
	if err := e.Secrets.Set(conn.Account(), "jeton", false); err != nil {
		t.Fatal(err)
	}
	if pv = previewOf(t, e, src); pv.Error != "" || fmt.Sprint(pv.Rows) != "[[v]]" {
		t.Fatalf("avec jeton : %v %s", pv.Rows, pv.Error)
	}
	// Le secret n'est pas envoyé hors du préfixe de la connexion.
	other := src
	other.URL = strings.Replace(srv.URL, "127.0.0.1", "localhost", 1) + "/api/prive"
	if pv = previewOf(t, e, other); !strings.Contains(pv.Error, "n'est pas couverte") {
		t.Fatalf("préfixe : %q", pv.Error)
	}
	// En-tête personnalisé : la valeur est envoyée telle quelle.
	key := Connection{Name: "Cle", Kind: KindWeb, Config: DBConfig{URL: srv.URL + "/api/cle", Header: "X-Api-Key"}}
	if err := TestConnection(ctx, e.Secrets, key, "secret", false); err != nil {
		t.Fatal(err)
	}
}

func TestUnderPrefix(t *testing.T) {
	for _, c := range []struct {
		url, prefix string
		ok          bool
	}{
		{"https://api.x.com/v1/a", "https://api.x.com", true},
		{"https://api.x.com/v1/a", "https://api.x.com/v1/", true},
		{"https://api.x.com/v10/a", "https://api.x.com/v1", false},
		{"https://api.x.com.evil.org/v1", "https://api.x.com", false},
		{"http://api.x.com/v1", "https://api.x.com", false},
	} {
		u, _ := webURL(c.url)
		if got := underPrefix(u, c.prefix); got != c.ok {
			t.Errorf("%s sous %s : %v", c.url, c.prefix, got)
		}
	}
}

type pqAdresse struct {
	Ville string `parquet:"ville"`
	CP    string `parquet:"cp,optional"`
}

type pqClient struct {
	ID      int64     `parquet:"id"`
	Nom     string    `parquet:"nom"`
	Solde   float64   `parquet:"solde"`
	Actif   bool      `parquet:"actif"`
	Inscrit int32     `parquet:"inscrit,date"` // jours depuis 1970
	Vu      time.Time `parquet:"vu,timestamp(millisecond)"`
	Adresse pqAdresse `parquet:"adresse"`
	Tags    []string  `parquet:"tags,list"`
	Note    *string   `parquet:"note,optional"`
}

func days(y int, m time.Month, d int) int32 {
	return int32(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
}

func TestParquetFile(t *testing.T) {
	note := "à rappeler"
	rows := []pqClient{
		{ID: 1, Nom: "Durand", Solde: 10.5, Actif: true, Inscrit: days(2026, 1, 15), Vu: time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC), Adresse: pqAdresse{"Paris", "75001"}, Tags: []string{"a", "b"}, Note: &note},
		{ID: 2, Nom: "Martin", Solde: 20, Inscrit: days(2026, 2, 1), Vu: time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC), Adresse: pqAdresse{Ville: "Lyon"}},
	}
	p := filepath.Join(t.TempDir(), "clients.parquet")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := parquet.Write(f, rows); err != nil {
		t.Fatal(err)
	}
	f.Close()
	d := newDoc(t)
	pv := previewOf(t, env(d), Source{Kind: SourceParquet, Path: p})
	if pv.Error != "" {
		t.Fatal(pv.Error)
	}
	want := []Column{{"id", grid.Integer}, {"nom", grid.Text}, {"solde", grid.Decimal}, {"actif", grid.Boolean}, {"inscrit", grid.Date}, {"vu", grid.Text},
		{"adresse.ville", grid.Text}, {"adresse.cp", grid.Text}, {"tags", grid.Text}, {"note", grid.Text}}
	if !reflect.DeepEqual(pv.Columns, want) {
		t.Fatalf("colonnes %v", pv.Columns)
	}
	if got := fmt.Sprint(pv.Rows); got != "[[1 Durand 10,5 VRAI 15/01/2026 2026-03-01 08:30:00 Paris 75001 a, b à rappeler] [2 Martin 20 FAUX 01/02/2026 2026-03-02 09:00:00 Lyon   ]]" {
		t.Fatalf("lignes %s", got)
	}
	bad := writeFile(t, "faux.parquet", "pas du parquet")
	if pv = previewOf(t, env(d), Source{Kind: SourceParquet, Path: bad}); !strings.Contains(pv.Error, "Parquet illisible") {
		t.Fatalf("fichier invalide : %q", pv.Error)
	}
}

func TestSQLiteFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ventes.db")
	db, err := driver.Open("file:" + p)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE ventes (id INTEGER PRIMARY KEY, ville TEXT, montant REAL, jour TEXT)`,
		`INSERT INTO ventes VALUES (1, 'Paris', 10.5, '2026-02-01'), (2, 'Lyon', 20, '2026-02-02'), (3, 'Paris', 4.25, NULL)`,
		`CREATE VIEW parville AS SELECT ville, sum(montant) AS total FROM ventes GROUP BY ville`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	ts, err := FileTables(ctx, KindSQLite, p)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ts) != "[{ parville true} { ventes false}]" {
		t.Fatalf("tables %v", ts)
	}
	if TableSQL(KindSQLite, "", "ventes") != `SELECT * FROM "ventes"` {
		t.Fatal(TableSQL(KindSQLite, "", "ventes"))
	}
	d := newDoc(t)
	pv := previewOf(t, env(d), Source{Kind: SourceSQLite, Path: p, SQL: "SELECT * FROM ventes ORDER BY id;"})
	if pv.Error != "" {
		t.Fatal(pv.Error)
	}
	if got := fmt.Sprint(pv.Rows); got != "[[1 Paris 10,5 01/02/2026] [2 Lyon 20 02/02/2026] [3 Paris 4,25 ]]" {
		t.Fatalf("lignes %s (%v)", got, pv.Columns)
	}
	// Étapes et chargement.
	id := save(t, d, Query{Name: "SQLite", Source: Source{Kind: SourceSQLite, Path: p, SQL: "SELECT * FROM parville"}, Steps: []Step{{Op: OpSort, Sort: []SortKey{{Column: "total"}}}}})
	l, err := Refresh(ctx, env(d), id)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(dump(t, d, l.Table)); got != "[[ville:text total:decimal] [Paris 14,75] [Lyon 20]]" {
		t.Fatalf("tableau %s", got)
	}
	// Lecture seule.
	pv = previewOf(t, env(d), Source{Kind: SourceSQLite, Path: p, SQL: "DELETE FROM ventes"})
	if pv.Error == "" {
		t.Fatal("écriture acceptée")
	}
	pv = previewOf(t, env(d), Source{Kind: SourceSQLite, Path: p, SQL: "SELECT count(*) AS n FROM ventes"})
	if fmt.Sprint(pv.Rows) != "[[3]]" {
		t.Fatalf("après écriture refusée : %v %s", pv.Rows, pv.Error)
	}
	if pv = previewOf(t, env(d), Source{Kind: SourceSQLite, Path: filepath.Join(t.TempDir(), "absent.db"), SQL: "SELECT 1"}); !strings.Contains(pv.Error, "introuvable") {
		t.Fatalf("fichier absent : %q", pv.Error)
	}
}

func TestConnectionStrings(t *testing.T) {
	pg := pgDSN(DBConfig{Host: "db", Database: "ventes", User: "l'oui", SSLMode: "require"}, `p\w'd`)
	if pg != `host='db' port=5432 dbname='ventes' connect_timeout=10 application_name=Lattis user='l\'oui' sslmode='require' password='p\\w\'d'` {
		t.Fatal(pg)
	}
	my := mysqlDSN(DBConfig{Host: "srv", Database: "ventes", User: "louis", SSLMode: "disable"}, "p@ss:/w")
	if !strings.HasPrefix(my, "louis:p@ss:/w@tcp(srv:3306)/ventes?") || !strings.Contains(my, "tls=false") || !strings.Contains(my, "parseTime=true") {
		t.Fatal(my)
	}
	if cfg, err := mysql.ParseDSN(my); err != nil || cfg.Passwd != "p@ss:/w" || cfg.Addr != "srv:3306" {
		t.Fatalf("DSN MySQL relu : %+v %v", cfg, err)
	}
	ms := sqlServerDSN(DBConfig{Host: "srv", Instance: "SQLEXPRESS", Database: "Ventes", User: `DOM\louis`, SSLMode: "require"}, "p&ss w")
	if ms != "sqlserver://DOM%5Clouis:p&ss%20w@srv/SQLEXPRESS?TrustServerCertificate=true&app+name=Lattis&connection+timeout=10&database=Ventes&encrypt=true" {
		t.Fatal(ms)
	}
	if ms := sqlServerDSN(DBConfig{Host: "srv"}, ""); ms != "sqlserver://srv:1433?app+name=Lattis&connection+timeout=10" {
		t.Fatal(ms)
	}
	accounts := map[string]Connection{
		"postgres://u@h:5432/b":        {Kind: KindPostgres, Config: DBConfig{Host: "h", Database: "b", User: "u"}},
		"mysql://u@h:3306/b":           {Kind: KindMySQL, Config: DBConfig{Host: "h", Database: "b", User: "u"}},
		`sqlserver://u@h\SQLEXPRESS/b`: {Kind: KindSQLServer, Config: DBConfig{Host: "h", Instance: "SQLEXPRESS", Database: "b", User: "u"}},
		"web://@https://api.x.com/v1":  {Kind: KindWeb, Config: DBConfig{URL: "https://api.x.com/v1"}},
		"postgres://u@h:5433/b":        {Config: DBConfig{Host: "h", Port: 5433, Database: "b", User: "u"}},
	}
	for want, c := range accounts {
		if got := c.Account(); got != want {
			t.Errorf("compte %s, attendu %s", got, want)
		}
	}
}

func TestConnectionChecks(t *testing.T) {
	for _, c := range []struct {
		conn Connection
		msg  string
	}{
		{Connection{Kind: KindPostgres, Config: DBConfig{Host: "h"}}, "hôte et base"},
		{Connection{Kind: KindMySQL}, "MySQL : le serveur"},
		{Connection{Kind: KindSQLServer}, "SQL Server : le serveur"},
		{Connection{Kind: KindWeb, Config: DBConfig{URL: "api.x.com"}}, "http://"},
		{Connection{Kind: "oracle"}, "non pris en charge"},
	} {
		if _, err := EncodeConnection(c.conn); err == nil || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s : %v", c.conn.Kind, err)
		}
	}
	def, err := EncodeConnection(Connection{Name: "M", Kind: KindMySQL, Config: DBConfig{Host: "srv", User: "louis"}})
	if err != nil || def.Config != `{"host":"srv","user":"louis"}` {
		t.Fatalf("%+v %v", def, err)
	}
	back, err := DecodeConnection(def)
	if err != nil || back.Kind != KindMySQL || back.Config.Host != "srv" {
		t.Fatalf("%+v %v", back, err)
	}
	// Ancien format PostgreSQL (champs sans omitempty).
	old, err := DecodeConnection(canvas.ConnectionDef{Name: "P", Kind: KindPostgres, Config: `{"host":"db","port":5432,"database":"ventes","user":"louis","sslmode":""}`})
	if err != nil || old.Config.Database != "ventes" || old.Account() != "postgres://louis@db:5432/ventes" {
		t.Fatalf("%+v %v", old, err)
	}
}

func TestAuthFailures(t *testing.T) {
	if !mysqlAuthFailed(&mysql.MySQLError{Number: 1045, Message: "Access denied for user 'x'"}) || mysqlAuthFailed(&mysql.MySQLError{Number: 1146}) {
		t.Fatal("MySQL")
	}
	if !sqlServerAuthFailed(fmt.Errorf("ouverture : %w", mssql.Error{Number: 18456, Message: "Login failed for user 'x'."})) || sqlServerAuthFailed(mssql.Error{Number: 208}) {
		t.Fatal("SQL Server")
	}
}

func TestReadGuards(t *testing.T) {
	for q, want := range map[string]string{
		"  select 1": "SELECT",
		"-- note\n/* x */ (WITH a AS (SELECT 1) SELECT * FROM a)": "WITH",
		"DELETE FROM t": "DELETE",
		"exec sp_who":   "EXEC",
	} {
		if got := firstKeyword(q); got != want {
			t.Errorf("%q : %s", q, got)
		}
	}
	if got := sqlServerLimit("SELECT a FROM t", 5); got != "SELECT TOP (5) * FROM (SELECT a FROM t\n) AS _lattis_apercu" {
		t.Fatal(got)
	}
	for _, q := range []string{"SELECT a FROM t ORDER BY a", "WITH x AS (SELECT 1 AS a) SELECT a FROM x"} {
		if got := sqlServerLimit(q, 5); got != q {
			t.Fatalf("%q enveloppée : %s", q, got)
		}
	}
	if TableSQL(KindSQLServer, "dbo", "Ventes]2") != "SELECT * FROM [dbo].[Ventes]]2]" || TableSQL(KindMySQL, "db", "t`x") != "SELECT * FROM `db`.`t``x`" || TableSQL(KindPostgres, "public", "Ventes") != `SELECT * FROM public."Ventes"` {
		t.Fatal("identifiants")
	}
	// Types SQL des différents pilotes.
	for name, want := range map[string]grid.Type{"INT4": grid.Integer, "UNSIGNED BIGINT": grid.Integer, "DECIMAL(10,2)": grid.Decimal, "BIT": grid.Boolean, "DATE": grid.Date, "DATETIME2": grid.Text, "NVARCHAR": grid.Text, "": ""} {
		if got := sqlType(name); got != want {
			t.Errorf("%s : %s", name, got)
		}
	}
	if dbValue([]byte("12.50"), grid.Decimal) != 12.5 || dbValue([]byte("42"), grid.Integer) != int64(42) || dbValue([]byte{1}, grid.Boolean) != int64(1) || dbValue("2026-02-01 00:00:00", grid.Date) != "2026-02-01" {
		t.Fatal("valeurs")
	}
	if v := sqlServerValue("UNIQUEIDENTIFIER", []byte{0x67, 0x45, 0x23, 0x01, 0xAB, 0x89, 0xEF, 0xCD, 0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF}); v != "01234567-89AB-CDEF-0123-456789ABCDEF" {
		t.Fatal(v)
	}
}

// Un serveur injoignable donne une erreur qui nomme la connexion (pas une
// demande de mot de passe).
func TestUnreachableServers(t *testing.T) {
	sec := NewSecrets(false)
	for _, kind := range []string{KindMySQL, KindSQLServer, KindPostgres} {
		c := Connection{Name: "Hors ligne", Kind: kind, Config: DBConfig{Host: "127.0.0.1", Port: 1, Database: "x", User: "u"}}
		err := TestConnection(ctx, sec, c, "", false)
		var pe *ErrPassword
		if err == nil || errors.As(err, &pe) || !strings.Contains(err.Error(), "connexion « Hors ligne »") {
			t.Errorf("%s : %v", kind, err)
		}
	}
}
