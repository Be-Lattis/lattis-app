package query

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/slitable/internal/grid"
)

// Tests d'intégration PostgreSQL : seulement si SLITABLE_PG_DSN désigne une
// base (par exemple postgres://postgres:secret@localhost:5432/postgres).
// Ils ne créent rien sur le serveur : tout passe par des requêtes en
// lecture seule.
func pgEnv(t *testing.T) (PGConfig, string) {
	t.Helper()
	dsn := os.Getenv("SLITABLE_PG_DSN")
	if dsn == "" {
		t.Skip("SLITABLE_PG_DSN non défini : tests PostgreSQL ignorés")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	mode := "disable"
	if cfg.TLSConfig != nil {
		mode = "require"
	}
	return PGConfig{Host: cfg.Host, Port: int(cfg.Port), Database: cfg.Database, User: cfg.User, SSLMode: mode}, cfg.Password
}

const pgSample = `SELECT * FROM (VALUES
	(1, 'Paris'::text, 10.5::numeric, DATE '2026-02-01', true, TIMESTAMP '2026-02-01 08:30:00', 3::bigint),
	(2, 'Lyon', 20, DATE '2026-02-02', false, NULL, 4),
	(3, 'Paris', 4.25, NULL, NULL, TIMESTAMP '2026-02-03 10:00:00', 5)
) AS t(id, ville, montant, jour, actif, horodatage, n);`

func TestPostgres(t *testing.T) {
	cfg, password := pgEnv(t)
	d := newDoc(t)
	s := newSession(t)
	e := env(d)
	def, err := EncodeConnection(Connection{Name: "Test", Postgres: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(def.Config, "password") || (password != "" && strings.Contains(def.Config, password)) {
		t.Fatalf("mot de passe dans la configuration : %s", def.Config)
	}
	if _, err := d.SaveConnection(ctx, def); err != nil {
		t.Fatal(err)
	}
	q := Query{Name: "PG", Source: Source{Kind: SourcePostgres, Connection: "Test", SQL: pgSample}}

	// Sans mot de passe connu : l'interface doit le demander.
	if password != "" && os.Getenv("PGPASSWORD") == "" {
		pv := s.Preview(ctx, e, q, -1, true)
		if pv.NeedPassword == nil || pv.NeedPassword.Connection != "Test" {
			t.Fatalf("mot de passe non demandé : %+v", pv)
		}
		conn := Connection{Name: "Test", Postgres: cfg}
		if err := TestConnection(ctx, e.Secrets, conn, "faux", false); err == nil {
			t.Fatal("mauvais mot de passe accepté")
		} else {
			var pe *ErrPassword
			if !errors.As(err, &pe) {
				t.Fatalf("erreur inattendue : %v", err)
			}
		}
		if err := TestConnection(ctx, e.Secrets, conn, password, false); err != nil {
			t.Fatal(err)
		}
	}

	pv := s.Preview(ctx, e, q, -1, true)
	if pv.Error != "" {
		t.Fatal(pv.Error)
	}
	want := []Column{{"id", grid.Integer}, {"ville", grid.Text}, {"montant", grid.Decimal}, {"jour", grid.Date}, {"actif", grid.Boolean}, {"horodatage", grid.Text}, {"n", grid.Integer}}
	if !reflect.DeepEqual(pv.Columns, want) {
		t.Fatalf("types %v", pv.Columns)
	}
	if got := fmt.Sprint(pv.Rows); got != "[[1 Paris 10,5 01/02/2026 VRAI 2026-02-01 08:30:00 3] [2 Lyon 20 02/02/2026 FAUX  4] [3 Paris 4,25   2026-02-03 10:00:00 5]]" {
		t.Fatalf("lignes %s", got)
	}

	// Étapes après la source, puis chargement.
	q.Steps = []Step{{Op: OpGroup, Columns: []string{"ville"}, Aggs: []Agg{{Name: "Total", Op: "sum", Column: "montant"}}}}
	id := save(t, d, q)
	l, err := s.Refresh(ctx, e, id)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(dump(t, d, l.Table)); got != "[[ville:text Total:decimal] [Lyon 20] [Paris 14,75]]" {
		t.Fatalf("tableau %s", got)
	}

	// Lecture seule : une écriture est refusée par le serveur.
	bad := save(t, d, Query{Name: "Écriture", Source: Source{Kind: SourcePostgres, Connection: "Test", SQL: "CREATE TABLE slitable_interdit (x int)"}})
	if _, err := s.Refresh(ctx, e, bad); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("écriture acceptée : %v", err)
	}

	// Navigateur.
	tables, err := PGTables(ctx, e.Secrets, Connection{Name: "Test", Postgres: cfg})
	if err != nil {
		t.Fatal(err)
	}
	_ = tables
}
