package bridge

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// TestShareCollaborativeWorkbook : parcours d'un classeur collaboratif par
// le pont, côté Bob (suivi du serveur, envoi après une saisie, réception sur
// notification, marques de la grille, conflit et sa résolution) ; Alice
// écrit depuis son propre classeur, directement par la synchronisation.
func TestShareCollaborativeWorkbook(t *testing.T) {
	ctx := context.Background()
	fake := workbooktest.New()
	oldServer, oldDir, oldRetry, oldDebounce := shareServer, shareCacheDir, shareRetry, collab.Debounce
	cache := t.TempDir()
	shareCacheDir = func() (string, error) { return cache, nil }
	shareRetry, collab.Debounce = 20*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() {
		shareServer, shareCacheDir, shareRetry, collab.Debounce = oldServer, oldDir, oldRetry, oldDebounce
	})
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		return fake.As("bob"), auth.User{PGRole: "bob"}, "Serveur de test", nil
	}

	// Alice publie, hors du pont.
	ad, err := doc.New(ctx)
	mustShare(t, err)
	t.Cleanup(func() { ad.Close() })
	_, err = ad.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4,5"}, {"Thé", "2"}})
	mustShare(t, err)
	var tbl int64
	mustShare(t, ad.DB().Read().QueryRow(`SELECT id FROM _lattis_tables`).Scan(&tbl))
	info, err := workbook.Publish(ctx, fake.As("alice"), ad.DB(), workbook.Request{Server: "https://lattis.example.test", User: "alice", Title: "Prix", Mode: workbook.Collaborative, Tables: []int64{tbl}})
	mustShare(t, err)
	mustShare(t, ad.ShareChanged(ctx))
	ad.Share.SetOnline(true, "")
	alice := collab.New(ad.DB(), ad.Model, ad.Share)
	alice.SetServer(fake.As("alice"))
	mustShare(t, fake.Grant(info.WorkbookID, "bob", "editor"))

	// Bob l'ouvre.
	b, hb := shareService(t)
	_, err = b.ShareOpen(ctx, "https://lattis.example.test", info.WorkbookID, "")
	mustShare(t, err)
	if st := b.ShareState(); st.Mode != workbook.Collaborative || st.Title != "Prix" {
		t.Fatalf("état : %+v", st)
	}
	waitShare(t, hb, "Bob en ligne", func(s workbook.State) bool { return s.Online })

	// Saisie de Bob : marquée en attente tant que le serveur est coupé…
	fake.SetDown(true)
	waitShare(t, hb, "Bob hors ligne", func(s workbook.State) bool { return !s.Online })
	if _, err := b.SetCell(ctx, 1, 1, 1, "5"); err == nil || !strings.Contains(err.Error(), "Hors ligne") {
		t.Fatalf("saisie hors ligne : %v", err)
	}
	fake.SetDown(false)
	waitShare(t, hb, "Bob de retour", func(s workbook.State) bool { return s.Online })
	_, err = b.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	// … puis envoyée (Kick) : Alice la reçoit.
	waitFor(t, "saisie de Bob sur le serveur", func() bool {
		v, _, _, ok := fake.Value(info.WorkbookID, "Feuille1", rowAt(t, ad, tbl, 0), "Prix")
		return ok && v == 5.0
	})
	_, err = alice.Receive(ctx, nil)
	mustShare(t, err)

	// Alice écrit : Bob reçoit sur notification.
	_, err = ad.SetCell(ctx, 1, 2, 1, "8")
	mustShare(t, err)
	_, err = alice.Send(ctx)
	mustShare(t, err)
	waitFor(t, "saisie d'Alice chez Bob", func() bool {
		w, err := b.Window(ctx, 1, tbl, 0, 10)
		return err == nil && len(w.Cells) == 2 && fmt.Sprint(w.Cells[1][1]) == "8"
	})

	// Conflit : la même cellule, Bob envoie après Alice.
	b.share.mu.Lock()
	bob := b.share.sync
	b.share.mu.Unlock()
	if bob == nil {
		t.Fatal("pas de synchronisation pour le classeur collaboratif")
	}
	_, err = ad.SetCell(ctx, 1, 2, 1, "9")
	mustShare(t, err)
	fake.SetDown(true) // Bob ne reçoit pas la modification d'Alice avant de saisir
	waitShare(t, hb, "Bob hors ligne", func(s workbook.State) bool { return !s.Online })
	b.share.doc.Share.SetOnline(true, "") // saisie permise : la file attend
	_, err = b.SetCell(ctx, 1, 2, 1, "10")
	mustShare(t, err)
	w, err := b.Window(ctx, 1, tbl, 0, 10)
	mustShare(t, err)
	if len(w.Sync) != 1 || w.Sync[0].Kind != collab.MarkPending || w.Sync[0].Row != 1 || w.Sync[0].Col != 1 {
		t.Fatalf("marques : %+v", w.Sync)
	}
	fake.SetDown(false)
	_, err = alice.Send(ctx)
	mustShare(t, err)
	waitShare(t, hb, "conflit signalé", func(s workbook.State) bool { return s.Online && s.Conflicts == 1 })
	cs, err := b.ShareConflicts(ctx)
	mustShare(t, err)
	if len(cs) != 1 || cs[0].Author != "alice" || cs[0].Mine != 10.0 || cs[0].Theirs != 9.0 {
		t.Fatalf("conflits : %+v", cs)
	}
	w, err = b.Window(ctx, 1, tbl, 0, 10)
	mustShare(t, err)
	if len(w.Sync) != 1 || w.Sync[0].Kind != collab.MarkConflict {
		t.Fatalf("marque de conflit : %+v", w.Sync)
	}
	_, err = b.ShareResolve(ctx, cs[0].ID, true)
	mustShare(t, err)
	waitFor(t, "valeur de Bob gardée sur le serveur", func() bool {
		v, _, by, _ := fake.Value(info.WorkbookID, "Feuille1", rowAt(t, ad, tbl, 1), "Prix")
		return v == 10.0 && by == "bob"
	})
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal(what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func rowAt(t *testing.T, d *doc.Document, tbl int64, n int) string {
	t.Helper()
	var id string
	mustShare(t, d.DB().Read().QueryRow(fmt.Sprintf(`SELECT r.lattis_id FROM t_%d AS t JOIN _lattis_row_ids AS r ON r.tbl = %d AND r.rid = t.id ORDER BY t._pos LIMIT 1 OFFSET ?`, tbl, tbl), n).Scan(&id))
	return id
}
