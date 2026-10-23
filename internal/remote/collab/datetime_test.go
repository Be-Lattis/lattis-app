package collab_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Dates avec heure dans un classeur partagé (décision de Louis,
// 23/10/2026) : une colonne Date qui a des heures est publiée en datetime
// (timestamptz), relue à l'identique sur un poste d'un autre fuseau.

// dateDoc prépare le classeur d'Alice : un tableau Ventes avec une colonne
// de dates avec heure (Quand) et une colonne de dates seules (Jour).
func dateDoc(t *testing.T) *doc.Document {
	t.Helper()
	ctx := context.Background()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	_, err = d.Paste(ctx, 1, 0, 0, [][]string{
		{"Produit", "Quand", "Jour"},
		{"Café", "23/10/2026 14:30", "23/10/2026"},
		{"Thé", "24/10/2026", "24/10/2026"},
		{"Sucre", "25/10/2026 08:05:30", ""},
	})
	must(t, err)
	var first string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = 1 AND row = 0 AND col = 0`).Scan(&first))
	must(t, d.RenameTable(ctx, first, "Ventes"))
	return d
}

// inZone change le fuseau du poste le temps de l'essai.
func inZone(t *testing.T, name string) {
	t.Helper()
	loc, err := time.LoadLocation(name)
	must(t, err)
	old := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = old })
}

// remoteTypes : types du serveur des colonnes de Ventes connus du poste.
func remoteTypes(t *testing.T, c *client) map[string]string {
	t.Helper()
	rt, _ := remoteTable(t, c)
	out := map[string]string{}
	for _, col := range rt.Columns {
		out[col.Name] = col.Type
	}
	return out
}

// localType : type local d'une colonne de Ventes.
func localType(t *testing.T, d *doc.Document, name string) string {
	t.Helper()
	var typ string
	must(t, d.DB().Read().QueryRow(`SELECT k.type FROM _lattis_columns k JOIN _lattis_tables t ON t.id = k.tbl WHERE t.name = 'Ventes' AND k.name = ?`, name).Scan(&typ))
	return typ
}

// dateTimeScenario : publication collaborative, ouverture sur un poste
// d'un autre fuseau, modifications dans les deux sens, date seule dans une
// colonne datetime, heure refusée dans une colonne date, conflit, colonne
// ajoutée avec des heures.
func dateTimeScenario(t *testing.T, publisher workbook.Server, aliceUser string, grant func(id string), bobOpen func(id string) *client) {
	ctx := context.Background()
	d := dateDoc(t)
	check, err := workbook.Inspect(ctx, d.DB())
	must(t, err)
	if cols := check.Tables[0].Columns; cols[1].Remote != "datetime" || cols[2].Remote != "date" || len(check.Blocking([]int64{tableID(t, d)})) != 0 {
		t.Fatalf("vérification : %+v", check.Tables[0])
	}
	info, err := workbook.Publish(ctx, publisher, d.DB(), workbook.Request{Server: server, User: aliceUser, Title: "Ventes 2026", Mode: workbook.Collaborative, Tables: []int64{tableID(t, d)}})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	alice := newClient(t, "alice", d, publisher)
	if ty := remoteTypes(t, alice); ty["Quand"] != "datetime" || ty["Jour"] != "date" {
		t.Fatalf("types publiés : %v", ty)
	}
	grant(info.WorkbookID)

	// Bob, à New York : les mêmes valeurs, dans des colonnes Date.
	inZone(t, "America/New_York")
	bob := bobOpen(info.WorkbookID)
	quand := []string{"2026-10-23 14:30", "2026-10-24", "2026-10-25 08:05:30"}
	eq(t, "Alice Quand", alice.col("Quand"), quand)
	eq(t, "Bob Quand", bob.col("Quand"), quand)
	eq(t, "Bob Jour", bob.col("Jour"), []string{"2026-10-23", "2026-10-24", ""})
	if localType(t, bob.d, "Quand") != "date" || localType(t, bob.d, "Jour") != "date" {
		t.Fatal("types locaux de Bob")
	}
	if ty := remoteTypes(t, bob); ty["Quand"] != "datetime" || ty["Jour"] != "date" {
		t.Fatalf("types de Bob : %v", ty)
	}

	// Modifications dans les deux sens ; une date seule dans la colonne
	// datetime reste une date seule.
	bob.mustSet(0, 1, "23/10/2026 18:45")
	bob.mustSet(1, 1, "26/10/2026")
	if r := bob.send(); r.Sent != 2 || r.Conflicts != 0 {
		t.Fatalf("Bob : %+v", r)
	}
	inZone(t, "Asia/Tokyo")
	alice.receive()
	eq(t, "Alice reçoit", alice.col("Quand"), []string{"2026-10-23 18:45", "2026-10-26", "2026-10-25 08:05:30"})
	// Ancienne valeur sans heure (minuit) : pas de faux conflit.
	alice.mustSet(1, 1, "26/10/2026 12:00")
	alice.mustSet(2, 1, "27/10/2026 23:59")
	if r := alice.send(); r.Sent != 2 || r.Conflicts != 0 {
		t.Fatalf("Alice : %+v", r)
	}
	bob.receive()
	eq(t, "Bob reçoit", bob.col("Quand"), []string{"2026-10-23 18:45", "2026-10-26 12:00", "2026-10-27 23:59"})

	// Heure dans une colonne publiée en dates seules : refusée, clairement.
	err = bob.set(0, 2, "23/10/2026 10:00")
	var oot canvas.ErrOutOfType
	if !errors.As(err, &oot) || !oot.NoTime || !strings.Contains(err.Error(), "dates sans heure") {
		t.Fatalf("heure dans Jour : %v", err)
	}
	bob.mustSet(0, 2, "30/10/2026")

	// Même cellule : conflit comparé en instants.
	alice.mustSet(0, 1, "23/10/2026 07:00")
	bob.mustSet(0, 1, "23/10/2026 08:00")
	alice.send()
	if r := bob.send(); r.Conflicts != 1 {
		t.Fatalf("conflit : %+v", r)
	}
	cs, err := bob.e.Conflicts(ctx)
	must(t, err)
	if len(cs) != 1 {
		t.Fatalf("conflits : %+v", cs)
	}
	must(t, bob.e.Resolve(ctx, cs[0].ID, true))
	if r := bob.send(); r.Sent != 1 || r.Conflicts != 0 {
		t.Fatalf("garder la mienne : %+v", r)
	}
	alice.receive()
	eq(t, "Alice après conflit", alice.col("Quand"), []string{"2026-10-23 08:00", "2026-10-26 12:00", "2026-10-27 23:59"})
	eq(t, "Alice Jour", alice.col("Jour"), []string{"2026-10-30", "2026-10-24", ""})

	// Colonne ajoutée par la conceptrice, remplie de dates avec heure :
	// datetime sur le serveur.
	must(t, alice.d.InsertColumn(ctx, "Ventes", 3, "Fin"))
	alice.send()
	ev, err := alice.d.Paste(ctx, 1, 1, 3, [][]string{{"23/10/2026 17:00"}, {"24/10/2026"}})
	must(t, err)
	if ev.RefusedCount != 0 {
		t.Fatalf("collage : %+v", ev)
	}
	if r := alice.send(); r.Rejected != 0 {
		t.Fatalf("Alice Fin : %+v", r)
	}
	if ty := remoteTypes(t, alice); ty["Fin"] != "datetime" {
		t.Fatalf("type de Fin : %v", ty)
	}
	if localType(t, alice.d, "Fin") != "date" {
		t.Fatal("type local de Fin")
	}
	_, err = bob.e.ReceiveStructure(ctx, nil, map[int64]bool{tableID(t, bob.d): true})
	must(t, err)
	if localType(t, bob.d, "Fin") != "date" || remoteTypes(t, bob)["Fin"] != "datetime" {
		t.Fatal("type de Fin chez Bob")
	}
	// Les valeurs du collage qui a typé la colonne vide partent avec lui.
	bob.receive()
	eq(t, "Bob Fin collée", bob.col("Fin"), []string{"2026-10-23 17:00", "2026-10-24", ""})
	alice.mustSet(2, 3, "25/10/2026 06:00")
	bob.mustSet(0, 3, "01/11/2026 09:00")
	alice.send()
	bob.send()
	alice.receive()
	bob.receive()
	fin := []string{"2026-11-01 09:00", "2026-10-24", "2026-10-25 06:00"}
	eq(t, "Bob Fin", bob.col("Fin"), fin)
	eq(t, "Alice Fin", alice.col("Fin"), fin)

	// La conceptrice tape une heure dans la colonne de dates seules : la
	// colonne passe en date et heure sur le serveur, l'éditeur peut alors en
	// taper aussi.
	alice.mustSet(1, 2, "24/10/2026 16:00")
	if r := alice.send(); r.Rejected != 0 || r.Conflicts != 0 {
		t.Fatalf("Alice Jour : %+v", r)
	}
	if ty := remoteTypes(t, alice); ty["Jour"] != "datetime" {
		t.Fatalf("Jour élargie : %v", ty)
	}
	_, err = bob.e.ReceiveStructure(ctx, nil, map[int64]bool{tableID(t, bob.d): true})
	must(t, err)
	bob.receive()
	if ty := remoteTypes(t, bob); ty["Jour"] != "datetime" {
		t.Fatalf("Jour chez Bob : %v", ty)
	}
	eq(t, "Bob Jour élargie", bob.col("Jour"), []string{"2026-10-30", "2026-10-24 16:00", ""})
	bob.mustSet(0, 2, "30/10/2026 11:11")
	if r := bob.send(); r.Sent != 1 || r.Conflicts != 0 {
		t.Fatalf("Bob Jour : %+v", r)
	}
	alice.receive()
	eq(t, "Alice Jour élargie", alice.col("Jour"), []string{"2026-10-30 11:11", "2026-10-24 16:00", ""})
}

func TestDateTimeCollaborative(t *testing.T) {
	useRoot(t)
	f := workbooktest.New()
	dateTimeScenario(t, f.As("alice"), "alice",
		func(id string) { must(t, f.Grant(id, "bob", "editor")) },
		func(id string) *client { return newClient(t, "bob", openAs(t, f, id, "bob"), f.As("bob")) })
}

// zoned : serveur PostgreSQL vu par un utilisateur dont la session est
// dans le fuseau zone (heure du serveur ou du poste différente de UTC).
func zoned(env *workbooktest.PGEnv, role, zone string) workbook.Server {
	return workbook.PG(func(ctx context.Context, usage string) (*pgx.Conn, func(), error) {
		c, err := pgx.Connect(ctx, env.Super)
		if err != nil {
			return nil, nil, err
		}
		if _, err := c.Exec(ctx, "SET TIME ZONE '"+zone+"'; SET SESSION AUTHORIZATION "+pgx.Identifier{role}.Sanitize()); err != nil {
			c.Close(ctx)
			return nil, nil, err
		}
		return c, func() { c.Close(context.Background()) }, nil
	})
}

func TestPostgresDateTimeCollaborative(t *testing.T) {
	env := workbooktest.PGSetup(t)
	useRoot(t)
	aliceRole, _ := env.User(t, "alice")
	bobRole, bobID := env.User(t, "bob")
	dateTimeScenario(t, zoned(env, aliceRole, "Asia/Tokyo"), aliceRole,
		func(id string) { env.AddMember(t, id, "editor", bobID) },
		func(id string) *client {
			srv := zoned(env, bobRole, "America/New_York")
			return newClient(t, "bob", pgOpen(t, srv, id, bobRole), srv)
		})
}

// frozenDateTimes : publication figée, ouverture sur un autre poste,
// heure ajoutée par la conceptrice dans la colonne de dates seules et
// republication (la colonne passe en datetime), colonne datetime gardée
// quand elle n'a plus que des dates.
func frozenDateTimes(t *testing.T, publisher workbook.Server, aliceUser string, grant func(id string), reader func() (workbook.Server, string)) {
	ctx := context.Background()
	d := dateDoc(t)
	info, err := workbook.Publish(ctx, publisher, d.DB(), workbook.Request{Server: server, User: aliceUser, Title: "Ventes 2026", Mode: workbook.Frozen, Tables: []int64{tableID(t, d)}})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	grant(info.WorkbookID)
	inZone(t, "America/Los_Angeles")
	srv, user := reader()
	r := &client{t: t, name: "bob", d: pgOpen(t, srv, info.WorkbookID, user)}
	eq(t, "Bob Quand", r.col("Quand"), []string{"2026-10-23 14:30", "2026-10-24", "2026-10-25 08:05:30"})
	eq(t, "Bob Jour", r.col("Jour"), []string{"2026-10-23", "2026-10-24", ""})
	if ty := remoteTypes(t, r); ty["Quand"] != "datetime" || ty["Jour"] != "date" {
		t.Fatalf("types : %v", ty)
	}

	// Classeur figé : la conceptrice garde l'heure (prochaine version).
	_, err = d.SetCell(ctx, 1, 1, 2, "23/10/2026 09:15")
	must(t, err)
	for i := range 3 {
		_, err = d.SetCell(ctx, 1, 1+i, 1, "2"+string(rune('3'+i))+"/10/2026")
		must(t, err)
	}
	_, err = workbook.Republish(ctx, publisher, d.DB(), nil)
	must(t, err)
	r2 := &client{t: t, name: "bob", d: pgOpen(t, srv, info.WorkbookID, user)}
	if ty := remoteTypes(t, r2); ty["Quand"] != "datetime" || ty["Jour"] != "datetime" {
		t.Fatalf("types republiés : %v", ty)
	}
	eq(t, "Bob Quand v2", r2.col("Quand"), []string{"2026-10-23", "2026-10-24", "2026-10-25"})
	eq(t, "Bob Jour v2", r2.col("Jour"), []string{"2026-10-23 09:15", "2026-10-24", ""})
}

func TestDateTimeFrozen(t *testing.T) {
	useRoot(t)
	f := workbooktest.New()
	frozenDateTimes(t, f.As("alice"), "alice",
		func(id string) { must(t, f.Grant(id, "bob", "reader")) },
		func() (workbook.Server, string) { return f.As("bob"), "bob" })
}

func TestPostgresDateTimeFrozen(t *testing.T) {
	env := workbooktest.PGSetup(t)
	useRoot(t)
	aliceRole, _ := env.User(t, "alice")
	bobRole, bobID := env.User(t, "bob")
	frozenDateTimes(t, zoned(env, aliceRole, "Europe/Paris"), aliceRole,
		func(id string) { env.AddMember(t, id, "reader", bobID) },
		func() (workbook.Server, string) { return zoned(env, bobRole, "Pacific/Auckland"), bobRole })
}
