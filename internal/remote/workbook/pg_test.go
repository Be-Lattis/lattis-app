package workbook_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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

type pgEnv struct {
	super string // URL du superutilisateur sur la base de test
}

func pgSetup(t *testing.T) *pgEnv {
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
	for _, q := range []string{
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'lattis_service') THEN CREATE ROLE lattis_service LOGIN CREATEROLE; END IF; END $$`,
		`ALTER ROLE lattis_service PASSWORD '` + pw + `'`,
		`GRANT CREATE, CONNECT ON DATABASE ` + pgx.Identifier{name}.Sanitize() + ` TO lattis_service`,
		`GRANT pg_monitor, pg_signal_backend TO lattis_service`,
	} {
		_, err := conn.Exec(ctx, q)
		must(t, err)
	}
	env := &pgEnv{super: withDB(t, admin, name, "", "")}
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

func withDB(t *testing.T, raw, db, user, pw string) string {
	u, err := url.Parse(raw)
	must(t, err)
	u.Path = "/" + db
	if user != "" {
		u.User = url.UserPassword(user, pw)
	}
	return u.String()
}

// user crée un utilisateur Lattis et rend son rôle PostgreSQL et son id.
func (e *pgEnv) user(t *testing.T, name string) (string, string) {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, e.super)
	must(t, err)
	defer c.Close(ctx)
	var role, id string
	email := fmt.Sprintf("%s.%s@example.test", name, uuid.NewString()[:8])
	must(t, c.QueryRow(ctx, `SELECT pg_role, id::text FROM lattis.create_user($1, $2, NULL, NULL)`, email, name).Scan(&role, &id))
	return role, id
}

// as rend le serveur vu par un utilisateur (une connexion par appel).
func (e *pgEnv) as(role string) workbook.Server {
	return workbook.PG(func(ctx context.Context, usage string) (*pgx.Conn, func(), error) {
		c, err := pgx.Connect(ctx, e.super)
		if err != nil {
			return nil, nil, err
		}
		if _, err := c.Exec(ctx, "SET application_name = 'Lattis/test/"+usage+"'; SET SESSION AUTHORIZATION "+pgx.Identifier{role}.Sanitize()); err != nil {
			c.Close(ctx)
			return nil, nil, err
		}
		return c, func() { c.Close(context.Background()) }, nil
	})
}

func (e *pgEnv) addMember(t *testing.T, wb, builtin, userID string) {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, e.super)
	must(t, err)
	defer c.Close(ctx)
	_, err = c.Exec(ctx, `SELECT lattis.add_member($1::uuid, $2, $3::uuid)`, wb, builtin, userID)
	must(t, err)
}

// Un classeur collaboratif se publie par le même chemin ; un éditeur
// l'ouvre (lecture sous RLS) et ses écritures vont dans la file d'envoi
// (la synchronisation est l'étape suivante).
func TestPostgresCollaborativePublish(t *testing.T) {
	env := pgSetup(t)
	useRoot(t)
	ctx := context.Background()
	alice, _ := env.user(t, "alice")
	carol, carolID := env.user(t, "carol")
	a := author(t)
	info, err := workbook.Publish(ctx, env.as(alice), a.DB(), workbook.Request{Server: server, User: alice, Title: "Suivi", Mode: workbook.Collaborative, Tables: []int64{tableID(t, a, "Ventes")}})
	must(t, err)
	if info.Mode != workbook.Collaborative || info.Version != 1 {
		t.Fatalf("info : %+v", info)
	}
	env.addMember(t, info.WorkbookID, "editor", carolID)
	r := open(t, env.as(carol), info.WorkbookID, carol)
	if got := column(t, r, "Ventes", "Produit"); !slices.Equal(got, []string{"Café", "Thé", "Chocolat"}) {
		t.Fatalf("produits : %v", got)
	}
	if st := r.Share.State(); st.ReadOnly != "" || st.Mode != workbook.Collaborative {
		t.Fatalf("état : %+v", st)
	}
	_, err = r.SetCell(ctx, 1, 1, 0, "Café moulu")
	must(t, err)
	var n int
	must(t, r.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_outbox WHERE state = 'pending'`).Scan(&n))
	if n != 1 {
		t.Fatalf("%d entrées en attente", n)
	}
	if _, err := workbook.Republish(ctx, env.as(alice), a.DB(), nil); err == nil {
		t.Fatal("un classeur collaboratif ne se republie pas")
	}
}

func TestPostgresFrozenLifecycle(t *testing.T) {
	env := pgSetup(t)
	useRoot(t)
	ctx := context.Background()
	alice, _ := env.user(t, "alice")
	bob, bobID := env.user(t, "bob")

	a := author(t)
	var ids []int64
	ids = append(ids, tableID(t, a, "Ventes"))
	info, err := workbook.Publish(ctx, env.as(alice), a.DB(), workbook.Request{Server: server, User: alice, Title: "Ventes 2026 — Été", Mode: workbook.Frozen, Tables: ids})
	must(t, err)
	must(t, a.ShareChanged(ctx))
	a.Share.SetOnline(true, "")
	if info.Version != 1 || !strings.HasPrefix(info.Schema, "wb_ventes_2026_ete") || info.Channel == "" || !info.CanDesign() {
		t.Fatalf("info : %+v", info)
	}

	// Le lecteur ne voit rien tant qu'il n'est pas membre.
	list, err := env.as(bob).Workbooks(ctx)
	must(t, err)
	if len(list) != 0 {
		t.Fatalf("liste d'un non-membre : %+v", list)
	}
	env.addMember(t, info.WorkbookID, "reader", bobID)
	list, err = env.as(bob).Workbooks(ctx)
	must(t, err)
	if len(list) != 1 || list[0].Title != "Ventes 2026 — Été" || list[0].State != "published" || !slices.Equal(list[0].Roles, []string{"reader"}) {
		t.Fatalf("liste : %+v", list)
	}
	r := open(t, env.as(bob), info.WorkbookID, bob)
	if got := column(t, r, "Ventes", "Produit"); !slices.Equal(got, []string{"Café", "Thé", "Chocolat"}) {
		t.Fatalf("produits : %v", got)
	}
	if got := column(t, r, "Ventes", "Prix"); !slices.Equal(got, []string{"4.5", "2.0", "3.25"}) {
		t.Fatalf("prix : %v", got)
	}
	if got := column(t, r, "Ventes", "Livré le"); !slices.Equal(got, []string{"2026-02-01", "2026-03-15", ""}) {
		t.Fatalf("dates : %v", got)
	}
	if got := column(t, r, "Ventes", "Payé"); !slices.Equal(got, []string{"1", "0", "1"}) {
		t.Fatalf("booléens : %v", got)
	}
	var author string
	must(t, r.DB().Read().QueryRow(`SELECT lattis_author FROM _lattis_row_ids LIMIT 1`).Scan(&author))
	if author != alice {
		t.Fatalf("auteur des lignes : %q", author)
	}
	scripts, err := r.Scripts(ctx)
	must(t, err)
	if len(scripts) != 1 {
		t.Fatalf("scripts : %+v", scripts)
	}
	if _, err := workbook.Republish(ctx, env.as(bob), r.DB(), nil); err == nil {
		t.Fatal("un lecteur republie")
	}

	// Republication, notifiée sur le canal du classeur.
	lctx, stop := context.WithCancel(ctx)
	defer stop()
	ready := make(chan struct{})
	got := make(chan string, 4)
	go env.as(bob).Listen(lctx, info.Channel, func() { close(ready) }, func(p string) { got <- p })
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("écoute non posée")
	}
	_, err = a.SetCell(ctx, 1, 1, 2, "5")
	must(t, err)
	info2, err := workbook.Republish(ctx, env.as(alice), a.DB(), nil)
	must(t, err)
	if info2.Version != 2 || info2.Schema == info.Schema {
		t.Fatalf("republication : %+v", info2)
	}
	// La version de travail notifie aussi ses écritures (« *meta »,
	// identifiants de tableaux) sur le même canal : seule *version compte
	// pour un classeur figé.
	var seen []string
	for done := false; !done; {
		select {
		case p := <-got:
			seen = append(seen, p)
			if v, ok := workbook.VersionNotice(p); ok {
				if v != 2 {
					t.Fatalf("notification : %q", p)
				}
				done = true
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("pas de notification de version : %q", seen)
		}
	}
	caps, err := env.as(bob).Capabilities(ctx, info.WorkbookID)
	must(t, err)
	if caps.Version != 2 {
		t.Fatalf("capacités : %+v", caps)
	}
	_, err = workbook.Load(ctx, env.as(bob), r.DB(), workbook.Source{Server: server, User: bob, WorkbookID: info.WorkbookID}, false)
	must(t, err)
	must(t, r.ShareChanged(ctx))
	if got := column(t, r, "Ventes", "Prix"); !slices.Equal(got, []string{"5.0", "2.0", "3.25"}) {
		t.Fatalf("prix rechargés : %v", got)
	}
	if st := r.Share.State(); st.Version != 2 {
		t.Fatalf("état : %+v", st)
	}
}
