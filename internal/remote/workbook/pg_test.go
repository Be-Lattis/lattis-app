package workbook_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Essais contre un vrai PostgreSQL 18 et le schéma lattis du serveur,
// activés par LATTIS_PG_TEST=1 (voir workbooktest.PGSetup).

type pgEnv struct{ *workbooktest.PGEnv }

func pgSetup(t *testing.T) *pgEnv { return &pgEnv{workbooktest.PGSetup(t)} }

func (e *pgEnv) user(t *testing.T, name string) (string, string) { return e.User(t, name) }

func (e *pgEnv) as(role string) workbook.Server { return e.As(role) }

func (e *pgEnv) addMember(t *testing.T, wb, builtin, userID string) {
	e.AddMember(t, wb, builtin, userID)
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

// Refus inattendu sur un réplica (contrat client de lattisd, §3.1) : une
// lecture refusée (42501) est refaite sur une nouvelle connexion, la
// fautive étant fermée. Le réplica en retard est imité par une première
// connexion ouverte sous un rôle qui n'est pas encore membre.
func TestPostgresReplicaRetry(t *testing.T) {
	env := pgSetup(t)
	useRoot(t)
	ctx := context.Background()
	alice, _ := env.user(t, "alice")
	stale, _ := env.user(t, "stale")
	a := author(t)
	info, err := workbook.Publish(ctx, env.as(alice), a.DB(), workbook.Request{Server: server, User: alice, Title: "Réplica", Mode: workbook.Frozen, Tables: []int64{tableID(t, a, "Ventes")}})
	must(t, err)
	var dials []string
	var first *pgx.Conn
	srv := workbook.PG(func(ctx context.Context, usage string) (*pgx.Conn, func(), error) {
		role := alice
		if len(dials) == 0 {
			role = stale
		}
		dials = append(dials, usage)
		c, err := pgx.Connect(ctx, env.Super)
		if err != nil {
			return nil, nil, err
		}
		if first == nil {
			first = c
		}
		if _, err := c.Exec(ctx, "SET SESSION AUTHORIZATION "+pgx.Identifier{role}.Sanitize()); err != nil {
			c.Close(ctx)
			return nil, nil, err
		}
		return c, func() {}, nil
	})
	caps, err := srv.Capabilities(ctx, info.WorkbookID)
	must(t, err)
	if caps.Schema != info.Schema || len(dials) != 2 || dials[1] != "initial_load" {
		t.Fatalf("capacités : %+v, connexions %v", caps, dials)
	}
	if !first.IsClosed() {
		t.Fatal("la connexion qui a refusé à tort doit être fermée")
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
	// Seule *version compte pour un classeur figé (avant la migration 0007
	// du serveur, la version de travail notifiait aussi ses écritures).
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
	if caps.Title != "" { // serveur à partir de la migration 0007
		if caps.Title != "Ventes 2026 — Été" {
			t.Fatalf("titre : %q", caps.Title)
		}
		if len(seen) != 1 {
			t.Fatalf("un lecteur ne reçoit que *version pendant une republication : %q", seen)
		}
	}
	_, err = workbook.Load(ctx, env.as(bob), r.DB(), workbook.Source{Server: server, User: bob, WorkbookID: info.WorkbookID}, false)
	must(t, err)
	must(t, r.ShareChanged(ctx))
	if got := column(t, r, "Ventes", "Prix"); !slices.Equal(got, []string{"5.0", "2.0", "3.25"}) {
		t.Fatalf("prix rechargés : %v", got)
	}
	if st := r.Share.State(); st.Version != 2 || st.Title != "Ventes 2026 — Été" {
		t.Fatalf("état : %+v", st)
	}
}

// Formulaires contre le vrai schéma (lot 1, §10 du contrat client) :
// disponibilité, création avec une définition complète (intertitre, liste
// verrouillée par le lien, valeurs posée par le serveur),
// version, ouverture, aperçu, liste vue par un lecteur, suppression.
func TestPostgresForms(t *testing.T) {
	env := pgSetup(t)
	useRoot(t)
	ctx := context.Background()
	alice, _ := env.user(t, "alice")
	bob, bobID := env.user(t, "bob")
	a := author(t)
	info, err := workbook.Publish(ctx, env.as(alice), a.DB(), workbook.Request{Server: server, User: alice, Title: "Commandes", Mode: workbook.Collaborative, Tables: []int64{tableID(t, a, "Ventes")}})
	must(t, err)
	env.addMember(t, info.WorkbookID, "reader", bobID)
	srv := env.as(alice)
	set, err := srv.FormsSettings(ctx)
	must(t, err)
	if !set.Forms || set.FormsPublic {
		t.Fatalf("réglages : %+v", set)
	}
	tbl, cols := remoteColumns(t, srv, info, "Ventes")
	def := formDef(cols)
	def.ServerValues = append(def.ServerValues, workbook.FormServerValue{Column: cols["Payé"], Value: workbook.FormValue{Fixed: true}})
	created, err := srv.CreateForm(ctx, info.WorkbookID, tbl, workbook.FormSpec{Definition: def, Access: workbook.FormMembers})
	must(t, workbook.FormError(err))
	if created.ID == "" || len(created.Code) != 10 || !strings.HasSuffix(created.URL, "/f/"+created.Code) {
		t.Fatalf("création : %+v", created)
	}
	def.Appearance.Title = "Commandes 2026"
	v, err := srv.UpdateForm(ctx, created.ID, workbook.FormChange{Definition: &def}, 1)
	must(t, workbook.FormError(err))
	if _, err := srv.UpdateForm(ctx, created.ID, workbook.FormChange{Definition: &def}, 1); !errors.Is(workbook.FormError(err), workbook.ErrFormModified) {
		t.Fatalf("version périmée : %v", err)
	}
	must(t, workbook.FormError(srv.SetFormState(ctx, created.ID, workbook.FormOpen)))
	forms, err := srv.ListForms(ctx, info.WorkbookID)
	must(t, workbook.FormError(err))
	if len(forms) != 1 || forms[0].State != workbook.FormOpen || forms[0].Version < v || forms[0].Definition == nil || forms[0].Definition.Appearance.Title != "Commandes 2026" || forms[0].Definition.Fields[1].Param != "produit" {
		t.Fatalf("liste : %+v", forms)
	}
	if u, err := srv.FormPreviewURL(ctx, created.ID); err != nil || !strings.Contains(u, "/f/"+created.Code+"?p=") {
		t.Fatalf("aperçu : %q %v", u, err)
	}
	if l, err := env.as(bob).ListForms(ctx, info.WorkbookID); err != nil || len(l) != 1 || l[0].Manageable || l[0].Definition != nil {
		t.Fatalf("liste du lecteur : %+v %v", l, err)
	}
	must(t, workbook.FormError(srv.DropForm(ctx, created.ID)))
	if l, err := srv.ListForms(ctx, info.WorkbookID); err != nil || len(l) != 0 {
		t.Fatalf("après suppression : %+v %v", l, err)
	}
}
