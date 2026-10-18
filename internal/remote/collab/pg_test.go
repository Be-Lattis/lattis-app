package collab_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Essais de la synchronisation contre un vrai PostgreSQL 18 et le schéma du
// serveur (LATTIS_PG_TEST=1, voir workbooktest.PGSetup) : écritures
// conditionnées, lattis.why_not, lattis.changes_since, RLS, LISTEN.

// downable coupe un serveur à la demande (essai de la file hors ligne).
type downable struct {
	workbook.Server
	down atomic.Bool
}

var errNet = errors.New("dial tcp 127.0.0.1:5432: connexion refusée")

func (d *downable) Write(ctx context.Context, fn func(workbook.Writer) error) error {
	if d.down.Load() {
		return errNet
	}
	return d.Server.Write(ctx, fn)
}

func (d *downable) Delta(ctx context.Context, fn func(workbook.Reader) error) error {
	if d.down.Load() {
		return errNet
	}
	return d.Server.Delta(ctx, fn)
}

type pgClients struct {
	env                  *workbooktest.PGEnv
	alice, bob, dave     *client
	aliceRole, bobRole   string
	daveRole, workbookID string
	bobServer            *downable
}

func pgSetupClients(t *testing.T) *pgClients {
	t.Helper()
	env := workbooktest.PGSetup(t)
	useRoot(t)
	ctx := context.Background()
	p := &pgClients{env: env}
	var bobID, daveID string
	p.aliceRole, _ = env.User(t, "alice")
	p.bobRole, bobID = env.User(t, "bob")
	p.daveRole, daveID = env.User(t, "dave")

	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	_, err = d.Paste(ctx, 1, 0, 0, [][]string{
		{"Produit", "Quantité", "Prix"},
		{"Café", "3", "4,5"},
		{"Thé", "10", "2"},
		{"Chocolat", "1", "3,25"},
	})
	must(t, err)
	var first string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = 1 AND row = 0 AND col = 0`).Scan(&first))
	must(t, d.RenameTable(ctx, first, "Ventes"))
	info, err := workbook.Publish(ctx, env.As(p.aliceRole), d.DB(), workbook.Request{Server: server, User: p.aliceRole, Title: "Ventes 2026", Mode: workbook.Collaborative, Tables: []int64{tableID(t, d)}})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	p.workbookID = info.WorkbookID
	p.alice = newClient(t, "alice", d, env.As(p.aliceRole))

	env.AddMember(t, info.WorkbookID, "editor", bobID)
	env.AddMember(t, info.WorkbookID, "participant", daveID)
	p.bobServer = &downable{Server: env.As(p.bobRole)}
	p.bob = newClient(t, "bob", pgOpen(t, env.As(p.bobRole), info.WorkbookID, p.bobRole), p.bobServer)
	p.dave = newClient(t, "dave", pgOpen(t, env.As(p.daveRole), info.WorkbookID, p.daveRole), env.As(p.daveRole))
	return p
}

func pgOpen(t *testing.T, srv workbook.Server, id, user string) *doc.Document {
	t.Helper()
	ctx := context.Background()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	_, err = workbook.Load(ctx, srv, d.DB(), workbook.Source{Server: server, User: user, WorkbookID: id}, true)
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	return d
}

func TestPostgresSync(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	if st := bob.d.Share.State(); st.Title != "Ventes 2026" {
		t.Fatalf("titre (my_capabilities) : %+v", st)
	}

	// Même ligne, colonnes différentes : jamais en conflit.
	alice.mustSet(0, 1, "5")
	bob.mustSet(0, 2, "9")
	if r := alice.send(); r.Sent != 1 {
		t.Fatalf("Alice : %+v", r)
	}
	if r := bob.send(); r.Sent != 1 || r.Conflicts != 0 {
		t.Fatalf("Bob : %+v", r)
	}
	alice.receive()
	bob.receive()
	for _, c := range []*client{alice, bob} {
		eq(t, c.name+" Quantité", c.col("Quantité"), []string{"5", "10", "1"})
		eq(t, c.name+" Prix", c.col("Prix"), []string{"9.0", "2.0", "3.25"})
	}

	// Même cellule : conflit expliqué par lattis.why_not.
	alice.mustSet(1, 2, "11")
	bob.mustSet(1, 2, "12")
	alice.send()
	r := bob.send()
	if r.Conflicts != 1 {
		t.Fatalf("conflit : %+v", r)
	}
	cs, err := bob.e.Conflicts(ctx)
	must(t, err)
	if len(cs) != 1 || cs[0].Author != p.aliceRole || cs[0].Mine != 12.0 || cs[0].Theirs != 11.0 || cs[0].At == "" {
		t.Fatalf("conflits : %+v", cs)
	}
	must(t, bob.e.Resolve(ctx, cs[0].ID, true))
	if r := bob.send(); r.Sent != 1 || r.Conflicts != 0 {
		t.Fatalf("garder la mienne : %+v", r)
	}
	alice.receive()
	eq(t, "Alice après conflit", alice.col("Prix"), []string{"9.0", "12.0", "3.25"})

	// Lignes ajoutées, supprimées ; réception idempotente.
	must(t, alice.d.InsertRow(ctx, "Ventes", 1))
	alice.mustSet(1, 0, "Sucre")
	must(t, alice.d.DeleteRow(ctx, "Ventes", 3))
	alice.send()
	bob.receive()
	eq(t, "Bob lignes", bob.col("Produit"), []string{"Café", "Sucre", "Thé"})
	if r := bob.receive(); r.Changed() {
		t.Fatalf("seconde réception : %+v", r)
	}

	// Annulation après la modification d'un collègue (pas encore reçue) :
	// refusée par le serveur, la cellule garde la valeur du collègue.
	alice.mustSet(2, 1, "50")
	alice.send()
	bob.receive()
	bob.mustSet(2, 1, "60")
	bob.send()
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	if r := alice.send(); r.Skipped != 1 {
		t.Fatalf("annulation : %+v", r)
	}
	eq(t, "Alice après annulation", alice.col("Quantité"), []string{"5", "", "60"})

	// File gardée pendant une coupure, envoyée à la reconnexion.
	bob.mustSet(0, 0, "Café en grains")
	p.bobServer.down.Store(true)
	if _, err := bob.e.Send(ctx); err == nil {
		t.Fatal("envoi coupé : erreur attendue")
	}
	if bob.states()["pending"] != 1 {
		t.Fatalf("file : %v", bob.states())
	}
	p.bobServer.down.Store(false)
	if r, err := bob.e.Sync(ctx); err != nil || r.Sent != 1 {
		t.Fatalf("reconnexion : %+v %v", r, err)
	}
	alice.receive()
	eq(t, "Alice", alice.col("Produit"), []string{"Café en grains", "Sucre", "Thé"})
}

func TestPostgresRights(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	dave, alice := p.dave, p.alice

	// Participant : la ligne d'Alice est refusée par la RLS (0 ligne,
	// why_not « forbidden »), la cellule reprend la valeur du serveur.
	dave.mustSet(0, 1, "99")
	r := dave.send()
	if r.Rejected != 1 || len(r.Notices) == 0 || !strings.Contains(r.Notices[0], "Droits insuffisants") {
		t.Fatalf("participant : %+v", r)
	}
	eq(t, "Dave", dave.col("Quantité"), []string{"3", "10", "1"})
	// Sa propre ligne passe.
	dave.mustSet(3, 0, "Tisane")
	if r := dave.send(); r.Sent != 1 || r.Rejected != 0 {
		t.Fatalf("nouvelle ligne : %+v", r)
	}
	dave.mustSet(3, 1, "4")
	if r := dave.send(); r.Sent != 1 {
		t.Fatalf("sa ligne : %+v", r)
	}
	alice.receive()
	eq(t, "Alice", alice.col("Produit"), []string{"Café", "Thé", "Chocolat", "Tisane"})
	var author string
	must(t, alice.d.DB().Read().QueryRow(`SELECT lattis_author FROM _lattis_row_ids WHERE lattis_id = ?`, rowID(t, alice.d, 3)).Scan(&author))
	if author != p.daveRole {
		t.Fatalf("auteur : %q", author)
	}

	// Droit retiré sur tout le tableau : 42501, toute l'étape est défaite.
	var schema, table, group string
	c, err := pgx.Connect(ctx, p.env.Super)
	must(t, err)
	defer c.Close(ctx)
	info := p.bob.d.Share.Info()
	schema = info.Schema
	must(t, c.QueryRow(ctx, `SELECT pg_role FROM lattis.roles WHERE workbook_id = $1::uuid AND builtin = 'editor'`, p.workbookID).Scan(&group))
	table = "Ventes"
	p.env.Exec(t, `REVOKE UPDATE ON `+pgx.Identifier{schema, table}.Sanitize()+` FROM `+pgx.Identifier{group}.Sanitize())
	p.bob.mustSet(1, 0, "Thé vert")
	r = p.bob.send()
	if r.Rejected != 1 || p.bob.states()["rejected"] != 1 {
		t.Fatalf("refus du tableau : %+v %v", r, p.bob.states())
	}
	eq(t, "Bob", p.bob.col("Produit"), []string{"Café", "Thé", "Chocolat"})
}

func TestPostgresFollow(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 20 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	p := pgSetupClients(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	data := make(chan struct{}, 8)
	go p.bob.e.Follow(ctx, p.bobServer, time.Hour, func(ev collab.Event) {
		if ev.Data {
			select {
			case data <- struct{}{}:
			default:
			}
		}
	})
	waitOnline(t, p.bob)
	p.alice.mustSet(2, 1, "42")
	p.alice.send()
	deadline := time.After(10 * time.Second)
	for {
		if got := p.bob.col("Quantité"); len(got) == 3 && got[2] == "42" {
			break
		}
		select {
		case <-data:
		case <-deadline:
			t.Fatalf("notification non reçue : %v", p.bob.col("Quantité"))
		}
	}
}
