package workbooktest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/remote/pgpool"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Essais contre un vrai PostgreSQL 18 et le schéma lattis du serveur
// (be-lattis-server, schema/*.sql), activés par LATTIS_PG_TEST=1
// (spécification de l'application collaborative, §14) :
//
//	LATTIS_TEST_PG_URL : superutilisateur, par exemple
//	                     postgres://postgres:…@127.0.0.1:5432/postgres
//	LATTIS_SERVER_SCHEMA : dossier des migrations NNNN_*.sql du serveur
//
// Une base jetable est créée et migrée comme le fait lattisd (rôle
// lattis_service), les utilisateurs par lattis.create_user ; chaque
// connexion « utilisateur » est une connexion du superutilisateur passée
// sous son rôle (SET SESSION AUTHORIZATION), ce que voit PostgreSQL après
// une connexion OAUTHBEARER (session_user = pg_role).

// PGEnv est une base de test migrée.
type PGEnv struct {
	// Super : URL du superutilisateur sur la base de test.
	Super string
}

// PGSetup crée et migre une base jetable (essai ignoré sans
// LATTIS_PG_TEST=1).
func PGSetup(t *testing.T) *PGEnv {
	t.Helper()
	if os.Getenv("LATTIS_PG_TEST") != "1" {
		t.Skip("LATTIS_PG_TEST=1 non défini : essai contre PostgreSQL ignoré")
	}
	admin, dir := os.Getenv("LATTIS_TEST_PG_URL"), os.Getenv("LATTIS_SERVER_SCHEMA")
	if admin == "" || dir == "" {
		t.Fatal("LATTIS_TEST_PG_URL et LATTIS_SERVER_SCHEMA sont requis avec LATTIS_PG_TEST=1")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, admin)
	must(t, err)
	defer conn.Close(ctx)
	name := "lattis_app_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	must(t, err)
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), admin)
		if err == nil {
			c.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
			c.Close(context.Background())
		}
	})
	const pw = "lattis-service-test"
	// Rôles communs à tout le serveur PostgreSQL : les paquets d'essais (ceux
	// de be-lattis-server compris, internal/testpg, même verrou) s'amorcent
	// l'un après l'autre, sinon « tuple concurrently updated ». Le verrou
	// tombe avec la connexion si un essai s'arrête avant de le rendre.
	_, err = conn.Exec(ctx, `SELECT pg_advisory_lock(7357)`)
	must(t, err)
	for _, q := range []string{
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'lattis_service') THEN CREATE ROLE lattis_service LOGIN CREATEROLE; END IF; END $$`,
		`ALTER ROLE lattis_service PASSWORD '` + pw + `'`,
		`GRANT CREATE, CONNECT ON DATABASE ` + pgx.Identifier{name}.Sanitize() + ` TO lattis_service`,
		`GRANT pg_monitor, pg_signal_backend TO lattis_service`,
	} {
		_, err := conn.Exec(ctx, q)
		must(t, err)
	}
	_, err = conn.Exec(ctx, `SELECT pg_advisory_unlock(7357)`)
	must(t, err)
	env := &PGEnv{Super: withDB(t, admin, name, "", "")}
	svc, err := pgx.Connect(ctx, withDB(t, admin, name, "lattis_service", pw))
	must(t, err)
	defer svc.Close(ctx)
	files, err := filepath.Glob(filepath.Join(dir, "[0-9][0-9][0-9][0-9]_*.sql"))
	must(t, err)
	if len(files) == 0 {
		t.Fatalf("aucune migration dans %s", dir)
	}
	sort.Strings(files)
	_, err = svc.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS lattis; CREATE TABLE IF NOT EXISTS lattis.schema_version (version integer PRIMARY KEY, name text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`)
	must(t, err)
	for i, f := range files {
		body, err := os.ReadFile(f)
		must(t, err)
		err = pgx.BeginFunc(ctx, svc, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(body)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO lattis.schema_version (version, name) VALUES ($1, $2)`, i+1, filepath.Base(f))
			return err
		})
		if err != nil {
			t.Fatalf("migration %s : %v", filepath.Base(f), err)
		}
	}
	return env
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func withDB(t *testing.T, raw, db, user, pw string) string {
	u, err := url.Parse(raw)
	must(t, err)
	u.Path = "/" + db
	if user != "" {
		u.User = url.UserPassword(user, pw)
	}
	return u.String()
}

// User crée un utilisateur Lattis et rend son rôle PostgreSQL et son id.
func (e *PGEnv) User(t *testing.T, name string) (string, string) {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, e.Super)
	must(t, err)
	defer c.Close(ctx)
	var role, id string
	email := fmt.Sprintf("%s.%s@example.test", name, uuid.NewString()[:8])
	must(t, c.QueryRow(ctx, `SELECT pg_role, id::text FROM lattis.create_user($1, $2, NULL, NULL)`, email, name).Scan(&role, &id))
	return role, id
}

// As rend le serveur vu par un utilisateur (une connexion par appel), sans
// identifiant de poste.
func (e *PGEnv) As(role string) workbook.Server { return e.AsDevice(role, "") }

// AsDevice rend le serveur vu par un utilisateur depuis un poste : comme
// pgpool, lattis.device est posé sur les connexions de rw (écritures,
// deltas, LISTEN).
func (e *PGEnv) AsDevice(role, device string) workbook.Server {
	return workbook.PG(func(ctx context.Context, usage string) (*pgx.Conn, func(), error) {
		c, err := pgx.Connect(ctx, e.Super)
		if err != nil {
			return nil, nil, err
		}
		q := "SET application_name = 'Lattis/test/" + usage + "'; SET SESSION AUTHORIZATION " + pgx.Identifier{role}.Sanitize()
		if device != "" && !pgpool.ReadUsage(usage) {
			q += "; " + pgpool.DeviceSQL(device)
		}
		if _, err := c.Exec(ctx, q); err != nil {
			c.Close(ctx)
			return nil, nil, err
		}
		return c, func() { c.Close(context.Background()) }, nil
	})
}

// AddMember donne un rôle fourni (reader, participant, editor…) sur un
// classeur.
func (e *PGEnv) AddMember(t *testing.T, wb, builtin, userID string) {
	t.Helper()
	e.Exec(t, `SELECT lattis.add_member($1::uuid, $2, $3::uuid)`, wb, builtin, userID)
}

// Exec exécute une instruction en superutilisateur (essais).
func (e *PGEnv) Exec(t *testing.T, q string, args ...any) {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, e.Super)
	must(t, err)
	defer c.Close(ctx)
	_, err = c.Exec(ctx, q, args...)
	must(t, err)
}
