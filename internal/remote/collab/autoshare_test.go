package collab_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Partage automatique : dans un classeur collaboratif, un tableau né sur
// le poste d'un propriétaire ou d'un concepteur (saisi, collé, inséré…)
// est ajouté au classeur partagé de lui-même ; hors ligne, à la
// reconnexion ; celui d'un éditeur reste sur son poste (marqué, message
// une fois).

func born(t *testing.T, c *client) []int64 {
	t.Helper()
	ids, err := oplog.Born(context.Background(), c.d.DB())
	must(t, err)
	return ids
}

func joined(r collab.Report) string { return strings.Join(r.Notices, " ; ") }

// TestAutoShareFollowed : Alice (propriétaire) colle un tableau, Bob le
// reçoit sans que personne ne l'ajoute ; ses saisies suivent.
func TestAutoShareFollowed(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 20 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f, alice, bob, id := setup(t)
	var notices = make(chan string, 64)
	bob.follow(ctx, f.As("bob"), time.Hour, nil)
	alice.follow(ctx, f.As("alice"), time.Hour, func(ev collab.Event) {
		for _, n := range ev.Notices {
			notices <- n
		}
	})
	waitOnline(t, bob, alice)

	tbl := newLocalTable(t, alice, "Stock", 10)
	alice.e.Kick()
	waitFor(t, "Stock chez Bob", func() bool { return hasTable(bob, "Stock") })
	waitFor(t, "Stock avec ses lignes chez Bob", func() bool { return len(values(t, bob, "Stock", "Article")) == 2 })
	eq(t, "Bob Article", values(t, bob, "Stock", "Article"), []string{"Vis", "Écrou"})
	if alice.d.Share.TableShare(tbl) != "published" {
		t.Fatal("Stock pas partagé chez Alice")
	}
	if _, _, _, tables := f.Book(id); strings.Join(tables, ",") != "Ventes,Stock" {
		t.Fatalf("tableaux du serveur : %v", tables)
	}
	if b := born(t, alice); len(b) != 0 {
		t.Fatalf("tableaux nés encore en attente : %v", b)
	}
	deadline := time.After(5 * time.Second)
	for found := false; !found; {
		select {
		case n := <-notices:
			found = strings.Contains(n, "ajouté au classeur partagé")
		case <-deadline:
			t.Fatal("pas de message d'ajout chez Alice")
		}
	}

	_, err := alice.d.SetCell(ctx, 1, 11, 1, "121")
	must(t, err)
	alice.e.Kick()
	waitFor(t, "valeur d'Alice chez Bob", func() bool { return values(t, bob, "Stock", "Stock")[0] == "121" })
}

// TestAutoShareOffline : un tableau créé hors ligne attend (gardé dans le
// classeur, même rouvert) ; il part à la reconnexion.
func TestAutoShareOffline(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	f.SetDown(true)
	alice.d.Share.SetOnline(false, "serveur injoignable")
	tbl := newLocalTable(t, alice, "Stock", 10)
	if b := born(t, alice); len(b) != 1 || b[0] != tbl {
		t.Fatalf("tableaux nés : %v", b)
	}
	r, err := alice.e.AutoShare(ctx)
	must(t, err)
	if len(r.Shared) != 0 || len(r.Notices) != 0 {
		t.Fatalf("hors ligne : %+v", r)
	}
	if alice.d.Share.TableShare(tbl) != "private" {
		t.Fatal("Stock partagé hors ligne")
	}
	// Une autre synchronisation (poste redémarré) le retrouve.
	again := newClient(t, "alice", alice.d, f.As("alice"))

	f.SetDown(false)
	alice.d.Share.SetOnline(true, "")
	r, err = again.e.AutoShare(ctx)
	must(t, err)
	if len(r.Shared) != 1 || !strings.Contains(joined(r), "Tableau « Stock » ajouté au classeur partagé") {
		t.Fatalf("reconnexion : %+v", r)
	}
	if _, _, _, tables := f.Book(id); strings.Join(tables, ",") != "Ventes,Stock" {
		t.Fatalf("tableaux du serveur : %v", tables)
	}
	if b := born(t, alice); len(b) != 0 {
		t.Fatalf("tableaux nés après l'ajout : %v", b)
	}
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	eq(t, "Bob", values(t, bob, "Stock", "Stock"), []string{"120", "75"})
}

// TestAutoShareEditor : un éditeur ne crée pas de tableau sur la feuille
// (décision 0018) ; celui qu'il a créé hors ligne comme concepteur, avant
// de perdre ce rôle, reste sur son poste : marqué « private », un message
// une seule fois par session, rien sur le serveur.
func TestAutoShareEditor(t *testing.T) {
	ctx := context.Background()
	f, _, bob, id := setup(t)
	if _, err := bob.d.Paste(ctx, 1, 10, 0, [][]string{{"Article", "Stock"}, {"Vis", "120"}}); !errors.Is(err, workbook.ErrLayout) {
		t.Fatalf("tableau créé par un éditeur : %v", err)
	}

	must(t, f.Grant(id, "bob", "designer"))
	_, err := workbook.RefreshCapabilities(ctx, f.As("bob"), bob.d.Share)
	must(t, err)
	bob.d.Share.SetOnline(false, "coupure")
	one := newLocalTable(t, bob, "Notes", 10)
	two := newLocalTable(t, bob, "Idées", 20)
	f.Revoke(id, "bob", "designer")
	bob.d.Share.SetOnline(true, "")
	_, err = workbook.RefreshCapabilities(ctx, f.As("bob"), bob.d.Share)
	must(t, err)

	r, err := bob.e.AutoShare(ctx)
	must(t, err)
	if len(r.Shared) != 0 || len(r.Notices) != 1 || !strings.Contains(r.Notices[0], "resté sur ce poste : seuls le propriétaire et les concepteurs") {
		t.Fatalf("éditeur : %+v", r)
	}
	for _, tbl := range []int64{one, two} {
		if bob.d.Share.TableShare(tbl) != "private" {
			t.Fatalf("tableau %d de Bob partagé", tbl)
		}
	}
	if _, _, _, tables := f.Book(id); strings.Join(tables, ",") != "Ventes" {
		t.Fatalf("tableaux du serveur : %v", tables)
	}
	if b := born(t, bob); len(b) != 0 {
		t.Fatalf("tableaux nés de Bob : %v", b)
	}
	if s := bob.e.CanAdd(one); s == "" {
		t.Fatal("CanAdd d'un éditeur")
	}
}

// TestAutoShareRefused : un tableau que le serveur refuse reste sur ce
// poste, avec un message, et n'est plus réessayé ; l'action à la main
// reste possible.
func TestAutoShareRefused(t *testing.T) {
	ctx := context.Background()
	f, alice, _, id := setup(t)
	tbl := newLocalTable(t, alice, "Stock", 10)
	f.Fail = "create_table"
	r, err := alice.e.AutoShare(ctx)
	f.Fail = ""
	if err != nil {
		// Un échec simulé du serveur sans code SQL est une coupure : le
		// tableau attend la prochaine fois.
		if b := born(t, alice); len(b) != 1 {
			t.Fatalf("après une coupure : %v", b)
		}
		r, err = alice.e.AutoShare(ctx)
		must(t, err)
		if len(r.Shared) != 1 {
			t.Fatalf("nouvel essai : %+v", r)
		}
		return
	}
	if len(r.Shared) != 0 || !strings.Contains(joined(r), "resté sur ce poste") {
		t.Fatalf("refus : %+v", r)
	}
	if alice.d.Share.TableShare(tbl) != "private" {
		t.Fatal("Stock partagé malgré le refus")
	}
	if _, _, _, tables := f.Book(id); strings.Join(tables, ",") != "Ventes" {
		t.Fatalf("tableaux du serveur : %v", tables)
	}
	r, err = alice.e.AutoShare(ctx)
	must(t, err)
	if len(r.Shared)+len(r.Notices) != 0 {
		t.Fatalf("réessayé : %+v", r)
	}
	_, err = alice.e.AddTable(ctx, tbl)
	must(t, err)
}

// TestAutoShareUndoRedo : Ctrl+Z de la création d'un tableau partagé
// d'office le retire du serveur ; Ctrl+Y le fait renaître, et il repart.
func TestAutoShareUndoRedo(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	_, err := alice.d.Paste(ctx, 1, 10, 0, [][]string{{"Article", "Stock"}, {"Vis", "120"}})
	must(t, err)
	r, err := alice.e.AutoShare(ctx)
	must(t, err)
	if len(r.Shared) != 1 {
		t.Fatalf("partage : %+v", r)
	}
	tbl := r.Shared[0]
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if tableCount(t, bob) != 2 {
		t.Fatal("tableau pas reçu par Bob")
	}

	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	if alive(t, alice, tbl) {
		t.Fatal("tableau toujours là après Ctrl+Z")
	}
	if r := alice.send(); r.Rejected != 0 {
		t.Fatalf("envoi : %+v", r)
	}
	if _, _, _, tables := f.Book(id); len(tables) != 1 {
		t.Fatalf("serveur après Ctrl+Z : %v", tables)
	}
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if tableCount(t, bob) != 1 {
		t.Fatal("tableau toujours chez Bob")
	}

	_, err = alice.d.Bus.Redo(ctx)
	must(t, err)
	if !alive(t, alice, tbl) || alice.d.Share.TableShare(tbl) != "private" {
		t.Fatal("rétablissement")
	}
	r, err = alice.e.AutoShare(ctx)
	must(t, err)
	if len(r.Shared) != 1 {
		t.Fatalf("partage après Ctrl+Y : %+v", r)
	}
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if tableCount(t, bob) != 2 {
		t.Fatal("tableau pas reçu de nouveau par Bob")
	}
}

func tableCount(t *testing.T, c *client) int {
	t.Helper()
	var n int
	must(t, c.d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_tables`).Scan(&n))
	return n
}

// TestAutoShareKeepsPublishChoice : un tableau laissé de côté à la
// publication n'est pas ajouté d'office ; seul un tableau né ensuite l'est.
func TestAutoShareKeepsPublishChoice(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	f := workbooktest.New()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	_, err = d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4"}})
	must(t, err)
	_, err = d.Paste(ctx, 1, 10, 0, [][]string{{"Note", "Qui"}, {"privée", "moi"}})
	must(t, err)
	var pub, kept int64
	must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE row = 0`).Scan(&pub))
	must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE row = 10`).Scan(&kept))
	_, err = workbook.Publish(ctx, f.As("alice"), d.DB(), workbook.Request{Server: server, User: "alice", Title: "Choix", Mode: workbook.Collaborative, Tables: []int64{pub}})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	alice := newClient(t, "alice", d, f.As("alice"))
	r, err := alice.e.AutoShare(ctx)
	must(t, err)
	if len(r.Shared) != 0 || d.Share.TableShare(kept) != "private" {
		t.Fatalf("tableau laissé de côté partagé : %+v", r)
	}
	_, err = d.Paste(ctx, 1, 20, 0, [][]string{{"Nouveau", "N"}, {"a", "1"}})
	must(t, err)
	r, err = alice.e.AutoShare(ctx)
	must(t, err)
	if len(r.Shared) != 1 || r.Shared[0] == kept {
		t.Fatalf("tableau né : %+v", r)
	}
}

// TestAutoShareRelayAndListen : le tableau ajouté d'office arrive chez Bob
// par le relais de lattisd (SSE) comme par l'écoute de PostgreSQL quand le
// serveur n'a pas de relais (404, lattisd v0.3.3-beta).
func TestAutoShareRelayAndListen(t *testing.T) {
	for _, relay := range []bool{true, false} {
		t.Run(fmt.Sprintf("relais=%v", relay), func(t *testing.T) {
			old := collab.Debounce
			collab.Debounce = 20 * time.Millisecond
			t.Cleanup(func() { collab.Debounce = old })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f, alice, bob, _ := setup(t)
			info := bob.d.Share.Info()
			var hits atomic.Int32
			hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if !relay {
					http.NotFound(w, r)
					return
				}
				// Relais : lattisd écoute PostgreSQL pour le poste et suit
				// les notifications en SSE.
				w.Header().Set("Content-Type", "text/event-stream")
				fl := w.(http.Flusher)
				send := func(kind, data string) {
					fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, data)
					fl.Flush()
				}
				ch := make(chan string, 64)
				go f.As("bob").Listen(r.Context(), info.Channel, func() { ch <- "" }, func(p string) { ch <- p })
				for {
					select {
					case <-r.Context().Done():
						return
					case p := <-ch:
						if p == "" {
							send("ready", `{}`)
							continue
						}
						send("change", fmt.Sprintf(`{"workbook":%q,"payload":%q}`, info.WorkbookID, p))
					}
				}
			}))
			// Fermé après l'arrêt du suivi de Bob (le flux se termine).
			defer func() {
				cancel()
				hs.CloseClientConnections()
				hs.Close()
			}()
			es := workbook.WithEvents(f.As("bob"), hs.URL+"/api/events", info.WorkbookID,
				func(context.Context) (string, error) { return "jeton", nil }, nil)
			bob.follow(ctx, es, time.Hour, nil)
			alice.follow(ctx, f.As("alice"), time.Hour, nil)
			waitOnline(t, bob, alice)
			if hits.Load() == 0 {
				t.Fatal("relais jamais demandé")
			}
			newLocalTable(t, alice, "Stock", 10)
			alice.e.Kick()
			waitFor(t, "Stock chez Bob", func() bool { return hasTable(bob, "Stock") })
			waitFor(t, "lignes chez Bob", func() bool { return len(values(t, bob, "Stock", "Stock")) == 2 })
			_, err := alice.d.SetCell(ctx, 1, 11, 1, "121")
			must(t, err)
			alice.e.Kick()
			waitFor(t, "valeur d'Alice chez Bob", func() bool { return values(t, bob, "Stock", "Stock")[0] == "121" })
		})
	}
}
