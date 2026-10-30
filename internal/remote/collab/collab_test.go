package collab_test

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

const server = "https://lattis.example.test"

func useRoot(t *testing.T) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "work")
	old := archive.Root
	archive.Root = func() (string, error) { return root, nil }
	t.Cleanup(func() { archive.Root = old })
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// client est un poste : son classeur et sa synchronisation.
type client struct {
	t    *testing.T
	name string
	d    *doc.Document
	e    *collab.Engine
	// synced : fermé à la fin du premier tour du suivi (follow).
	synced chan struct{}
}

func newClient(t *testing.T, name string, d *doc.Document, srv workbook.Server) *client {
	e := collab.New(d.DB(), d.Model, d.Share)
	e.SetServer(srv)
	// Comme le pont (bridge.watchShare) : la structure partagée passe par
	// le serveur.
	d.Oplog.SetStructure(e.Structure)
	d.SetIndexer(e)
	return &client{t: t, name: name, d: d, e: e}
}

// setup publie un classeur collaboratif (tableau Ventes) depuis le poste
// d'Alice et l'ouvre sur celui de Bob (éditeur).
func setup(t *testing.T) (*workbooktest.Fake, *client, *client, string) {
	t.Helper()
	useRoot(t)
	ctx := context.Background()
	f := workbooktest.New()
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
	var tbl int64
	must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = 'Ventes'`).Scan(&tbl))
	info, err := workbook.Publish(ctx, f.As("alice"), d.DB(), workbook.Request{Server: server, User: "alice", Title: "Ventes 2026", Mode: workbook.Collaborative, Tables: []int64{tbl}})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	alice := newClient(t, "alice", d, f.As("alice"))

	must(t, f.Grant(info.WorkbookID, "bob", "editor"))
	bob := newClient(t, "bob", openAs(t, f, info.WorkbookID, "bob"), f.As("bob"))
	return f, alice, bob, info.WorkbookID
}

func openAs(t *testing.T, f *workbooktest.Fake, id, user string) *doc.Document {
	t.Helper()
	ctx := context.Background()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	_, err = workbook.Load(ctx, f.As(user), d.DB(), workbook.Source{Server: server, User: user, WorkbookID: id, Title: "Ventes 2026"}, true)
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	return d
}

// set saisit raw dans la cellule (ligne de données n, colonne i) de Ventes.
func (c *client) set(n, i int, raw string) error {
	_, err := c.d.SetCell(context.Background(), 1, 1+n, i, raw)
	return err
}

func (c *client) mustSet(n, i int, raw string) {
	c.t.Helper()
	must(c.t, c.set(n, i, raw))
}

func (c *client) send() collab.Report {
	c.t.Helper()
	r, err := c.e.Send(context.Background())
	must(c.t, err)
	return r
}

func (c *client) receive() collab.Report {
	c.t.Helper()
	r, err := c.e.Receive(context.Background(), nil)
	must(c.t, err)
	return r
}

// col lit une colonne de Ventes dans l'ordre d'affichage.
func (c *client) col(name string) []string {
	c.t.Helper()
	var tbl, cid int64
	must(c.t, c.d.DB().Read().QueryRow(`SELECT t.id, k.id FROM _lattis_tables t JOIN _lattis_columns k ON k.tbl = t.id WHERE t.name = 'Ventes' AND k.name = ?`, name).Scan(&tbl, &cid))
	rs, err := c.d.DB().Read().Query(`SELECT coalesce(CAST(c_` + strconv.FormatInt(cid, 10) + ` AS TEXT), '') FROM t_` + strconv.FormatInt(tbl, 10) + ` ORDER BY _pos, id`)
	must(c.t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var s string
		must(c.t, rs.Scan(&s))
		out = append(out, s)
	}
	return out
}

func (c *client) states() map[string]int {
	c.t.Helper()
	rs, err := c.d.DB().Read().Query(`SELECT state, count(*) FROM _lattis_outbox GROUP BY state`)
	must(c.t, err)
	defer rs.Close()
	out := map[string]int{}
	for rs.Next() {
		var s string
		var n int
		must(c.t, rs.Scan(&s, &n))
		out[s] = n
	}
	return out
}

func eq(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%s : %q, attendu %q", what, got, want)
	}
}

func TestSameRowDifferentColumns(t *testing.T) {
	_, alice, bob, _ := setup(t)
	alice.mustSet(0, 1, "5")
	bob.mustSet(0, 2, "9")
	if r := alice.send(); r.Sent != 1 || r.Conflicts != 0 {
		t.Fatalf("envoi d'Alice : %+v", r)
	}
	if r := bob.send(); r.Sent != 1 || r.Conflicts != 0 {
		t.Fatalf("envoi de Bob : %+v (deux colonnes d'une même ligne ne sont jamais en conflit)", r)
	}
	alice.receive()
	bob.receive()
	for _, c := range []*client{alice, bob} {
		eq(t, c.name+" Quantité", c.col("Quantité"), []string{"5", "10", "1"})
		eq(t, c.name+" Prix", c.col("Prix"), []string{"9.0", "2.0", "3.25"})
	}
}

func TestSameCellConflict(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	alice.mustSet(1, 2, "11")
	bob.mustSet(1, 2, "12")
	alice.send()
	// Reçu avant l'envoi : la saisie de Bob, pas encore confirmée, reste.
	bob.receive()
	eq(t, "Bob avant envoi", bob.col("Prix"), []string{"4.5", "12.0", "3.25"})
	r := bob.send()
	if r.Conflicts != 1 || r.Sent != 0 {
		t.Fatalf("envoi de Bob : %+v, un conflit attendu", r)
	}
	cs, err := bob.e.Conflicts(ctx)
	must(t, err)
	if len(cs) != 1 {
		t.Fatalf("conflits : %+v", cs)
	}
	c := cs[0]
	if c.Author != "alice" || c.Column != "Prix" || c.Table != "Ventes" || !c.Found || c.Row != 2 || c.Col != 2 {
		t.Fatalf("conflit : %+v", c)
	}
	if c.Mine != 12.0 || c.Theirs != 11.0 {
		t.Fatalf("valeurs du conflit : mienne %v, sienne %v", c.Mine, c.Theirs)
	}
	// Marque de conflit dans la grille.
	tbl := tableID(t, bob.d)
	marks, err := bob.e.Marks(ctx, tbl, []int64{1, 2, 3})
	must(t, err)
	if len(marks) != 1 {
		t.Fatalf("marques : %+v", marks)
	}
	for _, m := range marks {
		for _, v := range m {
			if v != collab.MarkConflict {
				t.Fatalf("marque %q, conflit attendu", v)
			}
		}
	}
	// Reçu pendant le conflit : la valeur de Bob reste affichée.
	bob.receive()
	eq(t, "Bob en conflit", bob.col("Prix"), []string{"4.5", "12.0", "3.25"})

	// Garder la mienne : renvoyée, conditionnée à la valeur du serveur.
	must(t, bob.e.Resolve(ctx, c.ID, true))
	if r := bob.send(); r.Sent != 1 || r.Conflicts != 0 {
		t.Fatalf("renvoi : %+v", r)
	}
	alice.receive()
	eq(t, "Alice", alice.col("Prix"), []string{"4.5", "12.0", "3.25"})
	if cs, _ := bob.e.Conflicts(ctx); len(cs) != 0 {
		t.Fatalf("conflits après décision : %+v", cs)
	}

	// Nouveau conflit, Garder la sienne : la valeur du serveur revient.
	alice.mustSet(1, 2, "20")
	bob.mustSet(1, 2, "21")
	alice.send()
	if r := bob.send(); r.Conflicts != 1 {
		t.Fatalf("second conflit : %+v", r)
	}
	cs, _ = bob.e.Conflicts(ctx)
	must(t, bob.e.Resolve(ctx, cs[0].ID, false))
	eq(t, "Bob, la sienne", bob.col("Prix"), []string{"4.5", "20.0", "3.25"})
	if r := bob.send(); r.Sent != 0 {
		t.Fatalf("garder la sienne n'envoie rien : %+v", r)
	}
	v, _, by, _ := f.Value(id, "Ventes", rowID(t, bob.d, 1), "Prix")
	if v != 20.0 || by != "alice" {
		t.Fatalf("serveur : %v par %s", v, by)
	}
}

func TestUndoAfterColleagueChange(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	// Une étape de deux cellules ; Bob change ensuite l'une d'elles.
	_, err := alice.d.Paste(ctx, 1, 1, 1, [][]string{{"7", "8"}})
	must(t, err)
	alice.send()
	bob.receive()
	bob.mustSet(0, 1, "70")
	bob.send()
	alice.receive()
	eq(t, "Alice avant annulation", alice.col("Quantité"), []string{"70", "10", "1"})

	// Ctrl+Z : la cellule modifiée par Bob n'est pas annulée, l'autre l'est.
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	if msg := collab.UndoNotice(alice.d.Oplog, false); !strings.Contains(msg, "1 cellule modifiée par un collègue") {
		t.Fatalf("message : %q", msg)
	}
	eq(t, "Alice Quantité", alice.col("Quantité"), []string{"70", "10", "1"})
	eq(t, "Alice Prix", alice.col("Prix"), []string{"4.5", "2.0", "3.25"})
	if r := alice.send(); r.Sent != 1 || r.Conflicts != 0 || r.Skipped != 0 {
		t.Fatalf("envoi de l'annulation : %+v", r)
	}
	bob.receive()
	eq(t, "Bob Quantité", bob.col("Quantité"), []string{"70", "10", "1"})
	eq(t, "Bob Prix", bob.col("Prix"), []string{"4.5", "2.0", "3.25"})

	// Rétablir : même règle.
	_, err = alice.d.Bus.Redo(ctx)
	must(t, err)
	collab.UndoNotice(alice.d.Oplog, true)
	alice.send()
	eq(t, "Alice rétablie", alice.col("Prix"), []string{"8.0", "2.0", "3.25"})

	// Modification du collègue pas encore reçue : refusée par le serveur,
	// la cellule reprend sa valeur.
	alice.mustSet(2, 1, "5")
	alice.send()
	bob.receive()
	bob.mustSet(2, 1, "6")
	bob.send()
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	r := alice.send()
	if r.Skipped != 1 || r.Conflicts != 0 {
		t.Fatalf("annulation refusée par le serveur : %+v", r)
	}
	eq(t, "Alice après refus", alice.col("Quantité"), []string{"70", "10", "6"})
	if v, _, _, _ := f.Value(id, "Ventes", rowID(t, alice.d, 2), "Quantité"); v != int64(6) {
		t.Fatalf("serveur : %v", v)
	}
}

func TestRightsRefusal(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	// Bob devient lecteur sur le serveur après l'ouverture : son écriture
	// est refusée (42501), la cellule reprend la valeur du serveur.
	f.SetLevel(id, "bob", "Ventes", "reader")
	bob.mustSet(0, 0, "Thé vert")
	r := bob.send()
	if r.Rejected != 1 || len(r.Notices) == 0 || !strings.Contains(r.Notices[0], "Droits insuffisants") {
		t.Fatalf("refus : %+v", r)
	}
	eq(t, "Bob", bob.col("Produit"), []string{"Café", "Thé", "Chocolat"})
	if bob.states()["rejected"] != 1 {
		t.Fatalf("file : %v", bob.states())
	}

	// Participant : ses lignes seulement (RLS) ; la ligne d'Alice est
	// refusée, sa propre ligne passe.
	f.SetLevel(id, "bob", "Ventes", "participant")
	bob.mustSet(1, 1, "99")
	r = bob.send()
	if r.Rejected != 1 {
		t.Fatalf("participant, ligne d'Alice : %+v", r)
	}
	eq(t, "Bob Quantité", bob.col("Quantité"), []string{"3", "10", "1"})
	bob.mustSet(3, 0, "Tisane") // ligne ajoutée sous le tableau : la sienne
	if r := bob.send(); r.Sent != 1 || r.Rejected != 0 {
		t.Fatalf("participant, nouvelle ligne : %+v", r)
	}
	bob.mustSet(3, 1, "4")
	if r := bob.send(); r.Sent != 1 {
		t.Fatalf("participant, sa ligne : %+v", r)
	}
	alice.receive()
	eq(t, "Alice", alice.col("Produit"), []string{"Café", "Thé", "Chocolat", "Tisane"})
	eq(t, "Alice Quantité", alice.col("Quantité"), []string{"3", "10", "1", "4"})
	var author string
	must(t, alice.d.DB().Read().QueryRow(`SELECT lattis_author FROM _lattis_row_ids WHERE lattis_id = ?`, rowID(t, alice.d, 3)).Scan(&author))
	if author != "bob" {
		t.Fatalf("auteur de la ligne reçue : %q", author)
	}
	_ = ctx
}

func TestOfflineQueue(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	bob.mustSet(0, 1, "30")
	bob.mustSet(1, 1, "31")
	// Coupure : rien ne part, la file est gardée.
	f.SetDown(true)
	if _, err := bob.e.Send(ctx); err == nil {
		t.Fatal("envoi sans serveur : erreur attendue")
	}
	bob.d.Share.SetOnline(false, "serveur injoignable")
	// Hors ligne, le classeur collaboratif reste modifiable (décision
	// 0014) : la saisie attend dans la file.
	if err := bob.set(2, 1, "32"); err != nil {
		t.Fatalf("saisie hors ligne : %v", err)
	}
	if bob.states()["pending"] != 3 {
		t.Fatalf("file : %v", bob.states())
	}
	marks, err := bob.e.Marks(ctx, tableID(t, bob.d), []int64{1, 2, 3})
	must(t, err)
	if len(marks) != 3 {
		t.Fatalf("marques en attente : %+v", marks)
	}
	// Pendant la coupure, Alice modifie la seconde cellule.
	f.SetDown(false)
	alice.mustSet(1, 1, "40")
	alice.send()
	// Reconnexion : la file part, avec la détection de conflit normale.
	bob.d.Share.SetOnline(true, "")
	r, err := bob.e.Sync(ctx)
	must(t, err)
	if r.Sent != 2 || r.Conflicts != 1 {
		t.Fatalf("reconnexion : %+v", r)
	}
	eq(t, "Bob", bob.col("Quantité"), []string{"30", "31", "32"})
	alice.receive()
	eq(t, "Alice", alice.col("Quantité"), []string{"30", "40", "32"})
	marks, _ = bob.e.Marks(ctx, tableID(t, bob.d), []int64{1, 2, 3})
	if len(marks) != 1 {
		t.Fatalf("marques après envoi : %+v", marks)
	}
}

func TestRowsInsertedDeletedMoved(t *testing.T) {
	ctx := context.Background()
	_, alice, bob, _ := setup(t)
	tbl := "Ventes"
	must(t, alice.d.InsertRow(ctx, tbl, 1))
	alice.mustSet(1, 0, "Sucre")
	alice.mustSet(1, 1, "2")
	must(t, alice.d.DeleteRow(ctx, tbl, 3)) // Chocolat
	alice.send()
	r := bob.receive()
	if !r.Changed() {
		t.Fatalf("réception : %+v", r)
	}
	eq(t, "Bob", bob.col("Produit"), []string{"Café", "Sucre", "Thé"})
	eq(t, "Bob Quantité", bob.col("Quantité"), []string{"3", "2", "10"})
	// Réception idempotente.
	r = bob.receive()
	if r.Changed() {
		t.Fatalf("seconde réception : %+v", r)
	}
	// Tri (ordre partagé).
	err := bob.d.SortTable(ctx, tbl, 0, false)
	must(t, err)
	bob.send()
	alice.receive()
	eq(t, "Alice triée", alice.col("Produit"), []string{"Café", "Sucre", "Thé"})
}

func TestFrozenTypesInSharedWorkbook(t *testing.T) {
	ctx := context.Background()
	_, alice, _, _ := setup(t)
	err := alice.set(0, 1, "beaucoup")
	var oot canvas.ErrOutOfType
	if !errors.As(err, &oot) || !strings.Contains(err.Error(), "nombre entier") {
		t.Fatalf("saisie hors type : %v", err)
	}
	ev, err := alice.d.Paste(ctx, 1, 1, 1, [][]string{{"5", "x"}, {"abc", "7"}})
	must(t, err)
	if ev.RefusedCount != 2 || len(ev.Refused) != 2 {
		t.Fatalf("collage : %+v", ev)
	}
	eq(t, "Quantité", alice.col("Quantité"), []string{"5", "10", "1"})
	eq(t, "Prix", alice.col("Prix"), []string{"4.5", "7.0", "3.25"})
	var n int
	must(t, alice.d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_invalid`).Scan(&n))
	if n != 0 {
		t.Fatalf("%d saisies invalides gardées", n)
	}
	// Structure d'un tableau partagé : sans la synchronisation, refusée en
	// local (§6.4) ; avec elle, voir structure_test.go.
	alice.d.Oplog.SetStructure(nil)
	if err := alice.d.InsertColumn(ctx, "Ventes", 1, "Remise"); !errors.Is(err, workbook.ErrStructure) {
		t.Fatalf("colonne ajoutée : %v", err)
	}
}

func TestEditingCellWaitsForValidation(t *testing.T) {
	ctx := context.Background()
	_, alice, bob, _ := setup(t)
	tbl, rid, col, ok, err := bob.d.TableCell(ctx, 1, 1, 2)
	must(t, err)
	if !ok {
		t.Fatal("cellule introuvable")
	}
	bob.e.SetEditing(tbl, rid, col)
	alice.mustSet(0, 2, "6")
	alice.send()
	bob.receive()
	eq(t, "Bob pendant la saisie", bob.col("Prix"), []string{"4.5", "2.0", "3.25"})
	bob.mustSet(0, 2, "7")
	bob.e.SetEditing(0, 0, 0)
	if r := bob.send(); r.Conflicts != 1 {
		t.Fatalf("validation après une modification reçue : %+v", r)
	}
}

func TestFollowNotifications(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 20 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	f, alice, bob, _ := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var got []collab.Event
	data := make(chan struct{}, 8)
	bob.follow(ctx, f.As("bob"), time.Hour, func(ev collab.Event) {
		mu.Lock()
		got = append(got, ev)
		mu.Unlock()
		if ev.Data {
			data <- struct{}{}
		}
	})
	alice.follow(ctx, f.As("alice"), time.Hour, nil)
	waitOnline(t, bob, alice)
	alice.mustSet(2, 1, "42")
	alice.e.Kick()
	select {
	case <-data:
	case <-time.After(5 * time.Second):
		t.Fatal("modification d'Alice jamais reçue par Bob")
	}
	eq(t, "Bob", bob.col("Quantité"), []string{"3", "10", "42"})
	// Coupure : hors ligne ; reprise : en ligne, file envoyée.
	f.SetDown(true)
	deadline := time.Now().Add(5 * time.Second)
	for bob.d.Share.Online() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if bob.d.Share.Online() {
		t.Fatal("Bob devrait être hors ligne")
	}
	// Classeur collaboratif : pas en lecture seule hors ligne (décision
	// 0014), le bandeau dit seulement la coupure.
	if st := bob.d.Share.State(); st.ReadOnly != "" || st.Offline == "" {
		t.Fatalf("bandeau : %+v", st)
	}
}

// patience : délai des attentes des essais. Une attente réussie n'attend
// pas : large, il ne coûte rien et tient sous -race sur une machine lente.
const patience = 20 * time.Second

// follow lance le suivi de c (Engine.Follow) et rend un canal fermé à son
// arrêt. La fin du premier tour (connexion : file envoyée, tout relu) est
// notée pour waitOnline : c'est le premier évènement d'état reçu en ligne
// (le passage en ligne est toujours dit).
func (c *client) follow(ctx context.Context, srv workbook.Server, retry time.Duration, changed func(collab.Event)) <-chan struct{} {
	synced := make(chan struct{})
	c.synced = synced
	// Hors ligne d'abord : le premier tour le remet en ligne, et le dit
	// (Event.State) à sa fin.
	c.d.Share.SetOnline(false, "")
	var once sync.Once
	done := make(chan struct{})
	// Le suivi s'arrête avant la fin de l'essai (ctx annulé par son defer) :
	// sinon il lirait encore les réglages (collab.Debounce…) que l'essai
	// remet en place, et le classeur fermé.
	c.t.Cleanup(func() {
		select {
		case <-done:
		case <-time.After(patience):
			c.t.Errorf("%s : suivi pas arrêté", c.name)
		}
	})
	go func() {
		defer close(done)
		c.e.Follow(ctx, srv, retry, func(ev collab.Event) {
			if changed != nil {
				changed(ev)
			}
			if ev.State && c.d.Share.Online() {
				once.Do(func() { close(synced) })
			}
		})
	}()
	return done
}

// waitOnline attend que chaque poste suivi (follow) ait fini son premier
// tour : avant, une saisie ou une suppression faite par l'essai croise la
// synchronisation de la connexion.
func waitOnline(t *testing.T, cs ...*client) {
	t.Helper()
	deadline := time.After(patience)
	for _, c := range cs {
		if c.synced == nil {
			t.Fatalf("%s : suivi lancé sans follow", c.name)
		}
		select {
		case <-c.synced:
		case <-deadline:
			t.Fatalf("%s pas en ligne : %+v", c.name, c.d.Share.State())
		}
	}
}

func tableID(t *testing.T, d *doc.Document) int64 {
	t.Helper()
	var id int64
	must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = 'Ventes'`).Scan(&id))
	return id
}

// rowID donne le lattis_id de la ligne de données n de Ventes.
func rowID(t *testing.T, d *doc.Document, n int) string {
	t.Helper()
	tbl := tableID(t, d)
	var id string
	must(t, d.DB().Read().QueryRow(`SELECT r.lattis_id FROM t_`+strconv.FormatInt(tbl, 10)+` AS t JOIN _lattis_row_ids AS r ON r.tbl = ? AND r.rid = t.id ORDER BY t._pos, t.id LIMIT 1 OFFSET ?`, tbl, n).Scan(&id))
	return id
}
