package collab_test

// Essais tirés de l'audit du mode en ligne (docs/audits/audit-en-ligne-2026-10-20.md),
// contre PostgreSQL (LATTIS_PG_TEST=1, voir publication.md §6).

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Structure changée par Alice pendant que Bob saisit (sa saisie attend
// dans la file) : Bob doit rester en ligne et sa file se vider.
func TestPostgresStructureDuringInput(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(ctx context.Context, a *client) error
	}{
		{"tableau renommé", func(ctx context.Context, a *client) error { return a.d.Model.RenameTable(ctx, "Ventes", "Achats") }},
		{"colonne supprimée", func(ctx context.Context, a *client) error { return a.d.Model.DeleteColumn(ctx, "Ventes", 2) }},
		{"colonne renommée", func(ctx context.Context, a *client) error { _, err := a.d.SetCell(ctx, 1, 0, 2, "Tarif"); return err }},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := pgSetupClients(t)
			ctx := context.Background()
			alice, bob := p.alice, p.bob
			bob.mustSet(1, 2, "8")
			must(t, c.change(ctx, alice))
			alice.send()
			// Comme le suivi : envoi puis réception, deux tours.
			_, err1 := bob.e.Sync(ctx)
			_, err2 := bob.e.Sync(ctx)
			if err1 != nil || err2 != nil || bob.states()["pending"] != 0 {
				t.Fatalf("Bob : %v / %v, file %v", err1, err2, bob.states())
			}
		})
	}
}

// Décimal saisi ailleurs (formulaire, SQL, pipeline) avec plus de 17
// chiffres : une modification de Bob ne doit pas tourner en conflit.
func TestPostgresDecimalPrecision(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	// D'abord un décimal ordinaire saisi dans l'application.
	alice.mustSet(0, 2, "0,1")
	alice.send()
	bob.receive()
	bob.mustSet(0, 2, "0,2")
	if r := bob.send(); r.Sent != 1 || r.Conflicts != 0 {
		t.Fatalf("0,1 puis 0,2 : %+v", r)
	}
	info := bob.d.Share.Info()
	id := rowID(t, bob.d, 1)
	p.env.Exec(t, `UPDATE `+pgx.Identifier{info.Schema, "Ventes"}.Sanitize()+` SET "Prix" = 0.12345678901234567891 WHERE lattis_id = $1::uuid`, id)
	bob.receive()
	bob.mustSet(1, 2, "7")
	r := bob.send()
	if r.Conflicts == 0 {
		return
	}
	cs, err := bob.e.Conflicts(ctx)
	must(t, err)
	must(t, bob.e.Resolve(ctx, cs[0].ID, true))
	r2 := bob.send()
	t.Fatalf("modification d'un décimal long : %+v (conflit %+v), puis « Garder la mienne » : %+v", r, cs[0], r2)
}

// Conflit, puis la collègue change encore la cellule ; « Garder la sienne »
// doit donner la valeur actuelle du serveur.
func TestPostgresStaleTheirs(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	alice.mustSet(1, 2, "11")
	bob.mustSet(1, 2, "12")
	alice.send()
	if r := bob.send(); r.Conflicts != 1 {
		t.Fatalf("conflit attendu : %+v", r)
	}
	alice.mustSet(1, 2, "15")
	alice.send()
	bob.receive()
	cs, err := bob.e.Conflicts(ctx)
	must(t, err)
	must(t, bob.e.Resolve(ctx, cs[0].ID, false))
	bob.receive()
	eq(t, "Bob après « Garder la sienne »", bob.col("Prix"), []string{"4.5", "15.0", "3.25"})
}

// Ligne supprimée puis rétablie (Ctrl+Z) avant que Bob ne relise : Bob doit
// garder la ligne (elle existe sur le serveur).
func TestPostgresDeleteUndoSameDelta(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	must(t, alice.d.DeleteRow(ctx, "Ventes", 1))
	alice.send()
	_, err := alice.d.Bus.Undo(ctx)
	must(t, err)
	alice.send()
	eq(t, "Alice", alice.col("Produit"), []string{"Café", "Thé", "Chocolat"})
	bob.receive()
	eq(t, "Bob (suppression et rétablissement lus ensemble)", bob.col("Produit"), []string{"Café", "Thé", "Chocolat"})
}

// lossy valide la transaction puis rend une coupure (réponse perdue).
type lossy struct {
	workbook.Server
	arm atomic.Bool
}

func (l *lossy) Write(ctx context.Context, fn func(workbook.Writer) error) error {
	err := l.Server.Write(ctx, fn)
	if err == nil && l.arm.CompareAndSwap(true, false) {
		return errNet
	}
	return err
}

// Ligne ajoutée, réponse perdue ; Bob la supprime ; le renvoi d'Alice ne
// doit pas la recréer.
func TestPostgresInsertResentAfterDelete(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	l := &lossy{Server: p.env.As(p.aliceRole)}
	l.arm.Store(true)
	alice.e.SetServer(l)
	alice.mustSet(3, 0, "Tisane")
	if _, err := alice.e.Send(ctx); err == nil {
		t.Fatal("coupure attendue")
	}
	bob.receive()
	eq(t, "Bob reçoit", bob.col("Produit"), []string{"Café", "Thé", "Chocolat", "Tisane"})
	must(t, bob.d.DeleteRow(ctx, "Ventes", 3))
	bob.send()
	alice.send() // renvoi après la coupure
	bob.receive()
	eq(t, "Bob après le renvoi d'Alice", bob.col("Produit"), []string{"Café", "Thé", "Chocolat"})
}

// hookServer accroche la lecture de la mise en page dans une transaction
// d'écriture (Writer.Meta).
type hookServer struct {
	workbook.Server
	before, after func()
}

type hookWriter struct {
	workbook.Writer
	h *hookServer
}

func (w hookWriter) Meta(ctx context.Context, schema, kind, key string) ([]byte, error) {
	if w.h.before != nil {
		w.h.before()
	}
	raw, err := w.Writer.Meta(ctx, schema, kind, key)
	if w.h.after != nil {
		w.h.after()
	}
	return raw, err
}

func (h *hookServer) Write(ctx context.Context, fn func(workbook.Writer) error) error {
	return h.Server.Write(ctx, func(w workbook.Writer) error { return fn(hookWriter{w, h}) })
}

// Deux concepteurs envoient leur mise en page au même moment : aucune
// cellule libre ne doit se perdre.
func TestPostgresLayoutConcurrentSave(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	alice := p.alice
	carolRole, carolID := p.env.User(t, "carol")
	p.env.AddMember(t, p.workbookID, "designer", carolID)
	carolDoc := pgOpen(t, p.env.As(carolRole), p.workbookID, carolRole)

	aRead, cRead := make(chan struct{}), make(chan struct{})
	var aOnce, cOnce, cBefore sync.Once
	aSrv := &hookServer{Server: p.env.As(p.aliceRole)}
	aSrv.after = func() { aOnce.Do(func() { close(aRead); <-cRead }) }
	cSrv := &hookServer{Server: p.env.As(carolRole)}
	cSrv.before = func() { cBefore.Do(func() { <-aRead }) }
	cSrv.after = func() { cOnce.Do(func() { close(cRead) }) }
	alice.e.SetServer(aSrv)
	carol := newClient(t, "carol", carolDoc, cSrv)

	_, err := alice.d.SetCell(ctx, 1, 10, 5, "note d'Alice")
	must(t, err)
	_, err = carol.d.SetCell(ctx, 1, 12, 5, "note de Carol")
	must(t, err)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, c := range []*client{alice, carol} {
		wg.Add(1)
		go func(i int, c *client) {
			defer wg.Done()
			_, errs[i] = c.e.SyncLayout(ctx, false)
		}(i, c)
	}
	wg.Wait()
	must(t, errs[0])
	must(t, errs[1])
	alice.syncLayout(true)
	carol.syncLayout(true)
	for _, c := range []*client{alice, carol} {
		wantFree(t, c.name+" (note d'Alice)", c, 1, 10, 5, "note d'Alice", "note d'Alice")
		wantFree(t, c.name+" (note de Carol)", c, 1, 12, 5, "note de Carol", "note de Carol")
	}
}

// Droits de lecture changés en cours de session (restriction de lecture
// posée puis retirée) : Bob doit perdre puis retrouver la ligne.
func TestPostgresReadFilterChanged(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	bob := p.bob
	c, err := pgx.Connect(ctx, p.env.Super)
	must(t, err)
	defer c.Close(ctx)
	_, err = c.Exec(ctx, `SET SESSION AUTHORIZATION `+pgx.Identifier{p.aliceRole}.Sanitize())
	must(t, err)
	var tid, rid string
	tid = remoteTableID(t, bob)
	must(t, c.QueryRow(ctx, `SELECT lattis.add_restriction($1::uuid, $2::uuid, 'select', '{}', $3, 'sans thé')::text`, p.workbookID, tid, `"Produit" <> 'Thé'`).Scan(&rid))
	_, err = bob.e.RefreshRights(ctx)
	must(t, err)
	_, err = bob.e.Sync(ctx)
	must(t, err)
	got := bob.col("Produit")
	_, err = c.Exec(ctx, `SELECT lattis.drop_restriction($1::uuid)`, rid)
	must(t, err)
	_, err = bob.e.RefreshRights(ctx)
	must(t, err)
	_, err = bob.e.Sync(ctx)
	must(t, err)
	got2 := bob.col("Produit")
	if len(got) != 2 || len(got2) != 3 {
		t.Fatalf("Bob avec la restriction : %q (attendu sans Thé) ; après son retrait : %q (attendu les 3)", got, got2)
	}
}

func remoteTableID(t *testing.T, c *client) string {
	t.Helper()
	var id string
	must(t, c.d.DB().Read().QueryRow(`SELECT remote_id FROM _lattis_remote_tables WHERE published = 1`).Scan(&id))
	return id
}

// Filtre élargi : une ligne devenue visible doit arriver.
func TestPostgresReadFilterWidened(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	c, err := pgx.Connect(ctx, p.env.Super)
	must(t, err)
	defer c.Close(ctx)
	_, err = c.Exec(ctx, `SET SESSION AUTHORIZATION `+pgx.Identifier{p.aliceRole}.Sanitize())
	must(t, err)
	var rid string
	must(t, c.QueryRow(ctx, `SELECT lattis.add_restriction($1::uuid, $2::uuid, 'select', '{}', $3, 'sans thé')::text`, p.workbookID, remoteTableID(t, p.bob), `"Produit" <> 'Thé'`).Scan(&rid))
	carolRole, carolID := p.env.User(t, "carol")
	p.env.AddMember(t, p.workbookID, "editor", carolID)
	carol := newClient(t, "carol", pgOpen(t, p.env.As(carolRole), p.workbookID, carolRole), p.env.As(carolRole))
	before := carol.col("Produit")
	_, err = c.Exec(ctx, `SELECT lattis.drop_restriction($1::uuid)`, rid)
	must(t, err)
	_, err = carol.e.RefreshRights(ctx)
	must(t, err)
	_, err = carol.e.Sync(ctx)
	must(t, err)
	if got := carol.col("Produit"); len(got) != 3 {
		t.Fatalf("Carol : %q avec la restriction, %q après son retrait (attendu les 3)", before, got)
	}
}

// Ligne qui sort du filtre de lecture de Bob parce qu'Alice change sa
// valeur : Bob ne doit plus la voir.
func TestPostgresRowLeavesFilter(t *testing.T) {
	p := pgSetupClients(t)
	pgAs(t, p, p.aliceRole, `SELECT lattis.add_restriction($1::uuid, $2::uuid, 'select', '{}', $3, 'pas interdit')`, p.workbookID, remoteTableID(t, p.bob), `"Produit" <> 'Interdit'`)
	p.alice.mustSet(1, 0, "Interdit")
	p.alice.send()
	p.bob.receive()
	if got := p.bob.col("Produit"); len(got) != 2 {
		t.Fatalf("Bob : %q (la ligne « Interdit » ne lui est plus visible)", got)
	}
}

// Montée en charge : N postes suivent le classeur (LISTEN direct), Alice
// écrit ; délai jusqu'au dernier poste et connexions tenues.
func TestPostgresLoadFollowers(t *testing.T) {
	const n = 40
	p := pgSetupClients(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var fs []*client
	for i := 0; i < n; i++ {
		role, id := p.env.User(t, "poste")
		p.env.AddMember(t, p.workbookID, "editor", id)
		srv := p.env.As(role)
		c := newClient(t, "poste", pgOpen(t, srv, p.workbookID, role), srv)
		go c.e.Follow(ctx, srv, time.Hour, nil)
		fs = append(fs, c)
	}
	waitOnline(t, fs...)
	conns := p.env.Text(t, `SELECT count(*)::text FROM pg_stat_activity WHERE datname = current_database()`)
	var worst time.Duration
	for k := 0; k < 10; k++ {
		v := strconv.Itoa(100 + k)
		start := time.Now()
		p.alice.mustSet(2, 1, v)
		p.alice.send()
		for _, c := range fs {
			for c.col("Quantité")[2] != v {
				if time.Since(start) > 20*time.Second {
					t.Fatalf("poste en retard")
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
		if d := time.Since(start); d > worst {
			worst = d
		}
	}
	t.Logf("%d postes : connexions ouvertes sur la base %s, pire délai jusqu'au dernier poste %v", n, conns, worst)
}

// Constat 20, contre PostgreSQL : publish_script et relecture des scripts.
func TestPostgresScriptsShared(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	_, err := p.alice.d.Model.SaveScript(ctx, 0, "Bonjour", `print("bonjour")`)
	must(t, err)
	_, err = p.alice.e.SyncScripts(ctx, false)
	must(t, err)
	_, err = p.bob.e.SyncScripts(ctx, true)
	must(t, err)
	var src string
	must(t, p.bob.d.DB().Read().QueryRow(`SELECT source FROM _lattis_scripts WHERE name = 'Bonjour'`).Scan(&src))
	if src != `print("bonjour")` {
		t.Fatalf("Bob : %q", src)
	}
}
