package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// shareHost relève les évènements de partage.
type shareHost struct {
	NoHost
	events chan workbook.State
}

func (h *shareHost) Emit(event string, data any) bool {
	if event == ShareEvent {
		select {
		case h.events <- data.(workbook.State):
		default:
		}
	}
	return true
}

func mustShare(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func shareService(t *testing.T) (*DocumentService, *shareHost) {
	t.Helper()
	s := &DocumentService{}
	h := &shareHost{events: make(chan workbook.State, 64)}
	UseHost(h, nil, s)
	t.Cleanup(func() { s.CloseDocument(context.Background()) })
	return s, h
}

func waitShare(t *testing.T, h *shareHost, what string, ok func(workbook.State) bool) {
	t.Helper()
	for {
		select {
		case st := <-h.events:
			if ok(st) {
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal(what)
		}
	}
}

func TestShareFrozenWorkbook(t *testing.T) {
	ctx := context.Background()
	fake := workbooktest.New()
	users := map[string]string{}
	oldServer, oldDir, oldRetry, oldRecent := shareServer, shareCacheDir, shareRetry, recentFile
	cache := t.TempDir()
	shareCacheDir = func() (string, error) { return cache, nil }
	shareRetry = 20 * time.Millisecond
	recent := filepath.Join(t.TempDir(), "recent.json")
	recentFile = func() (string, error) { return recent, nil }
	t.Cleanup(func() { shareServer, shareCacheDir, shareRetry, recentFile = oldServer, oldDir, oldRetry, oldRecent })
	// L'adresse du serveur dit l'utilisateur : l'auteur publie sur
	// auteur.example.test, le lecteur ouvre depuis lattis.example.test (même
	// faux serveur). Une identité « courante » globale serait lue aussi par
	// le suivi en arrière-plan de l'autre poste, qui relirait alors les
	// droits du mauvais utilisateur (essai instable).
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		u := users["b"]
		if strings.Contains(server, "auteur.example.test") {
			u = users["a"]
		}
		return fake.As(u), auth.User{PGRole: u}, "Serveur de test", nil
	}

	// L'auteur publie.
	a, _ := shareService(t)
	users["a"], users["b"] = "alice", "bob"
	_, err := a.New(ctx)
	mustShare(t, err)
	_, err = a.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4,5"}, {"Thé", "2"}})
	mustShare(t, err)
	check, err := a.ShareCheck(ctx)
	mustShare(t, err)
	if len(check.Tables) != 1 || len(check.Tables[0].Problems) != 0 {
		t.Fatalf("vérifications : %+v", check)
	}
	st, err := a.SharePublish(ctx, ShareRequest{Server: "https://auteur.example.test", Title: "Prix", Mode: workbook.Frozen, Tables: []int64{check.Tables[0].ID}})
	mustShare(t, err)
	if !st.Shared || !st.CanDesign || st.Version != 1 || st.ServerName != "Serveur de test" {
		t.Fatalf("état de l'auteur : %+v", st)
	}
	l, err := a.Layout(ctx, 1)
	mustShare(t, err)
	if l.Tables[0].Shared != "published" || l.Tables[0].ReadOnly != "" {
		t.Fatalf("tableau de l'auteur : %+v", l.Tables[0])
	}
	if ds, _ := a.State(ctx); !ds.Dirty {
		t.Fatal("classeur publié non enregistré : il doit être à enregistrer")
	}

	// Le lecteur l'ouvre depuis le serveur.
	list, err := fake.As("alice").Workbooks(ctx)
	mustShare(t, err)
	mustShare(t, fake.Grant(list[0].ID, "bob", "reader"))
	b, hb := shareService(t)
	got, err := b.ShareWorkbooks(ctx, "https://lattis.example.test")
	mustShare(t, err)
	if len(got) != 1 || got[0].Title != "Prix" {
		t.Fatalf("liste : %+v", got)
	}
	r, err := b.ShareOpen(ctx, "https://lattis.example.test", got[0].ID, got[0].Title)
	mustShare(t, err)
	path := filepath.Join(cache, "lattis.example.test", got[0].ID+".lattis")
	if r.State.Path != path || r.State.Dirty {
		t.Fatalf("cache : %+v (attendu %s)", r.State, path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	// Le cache n'est jamais montré : titre du serveur, serveur, adresse.
	link := "https://lattis.example.test/w/" + got[0].ID
	if rm := r.State.Remote; r.State.Title != "Prix" || rm == nil || rm.Title != "Prix" || rm.Link != link || rm.ServerName != "Serveur de test" || rm.Mode != workbook.Frozen {
		t.Fatalf("classeur du serveur : %+v / %+v", r.State, r.State.Remote)
	}
	if rf := b.Recent(ctx); len(rf) != 1 || rf[0].Title != "Prix" || rf[0].WorkbookID != got[0].ID || rf[0].Link != link || rf[0].Server != "https://lattis.example.test" {
		t.Fatalf("récents : %+v", rf)
	}
	if ds, err := b.Save(ctx, false); err != nil || ds.Notice != "« Prix » est en lecture seule sur Serveur de test : rien à enregistrer. Pour une copie modifiable : Fichier › Enregistrer sous" {
		t.Fatalf("enregistrement : %+v, %v", ds, err)
	}
	if rf := b.Recent(ctx); len(rf) != 1 || rf[0].Title != "Prix" {
		t.Fatalf("récents après enregistrement : %+v", rf)
	}
	if st := b.ShareState(); st.ReadOnly != "Classeur publié en lecture seule" || st.Title != "Prix" {
		t.Fatalf("état du lecteur : %+v", st)
	}
	if _, err := b.SetCell(ctx, 1, 1, 1, "9"); err == nil || !strings.Contains(err.Error(), "Classeur publié en lecture seule") {
		t.Fatalf("saisie : %v", err)
	}
	if _, err := b.Call(ctx, "cells.set", map[string]any{"sheet": "Feuille1", "row": 2, "col": 2, "value": "9"}); err == nil || !strings.Contains(err.Error(), "lecture seule") {
		t.Fatalf("API : %v", err)
	}
	waitShare(t, hb, "lecteur en ligne", func(s workbook.State) bool { return s.Online })

	// L'auteur republie : le lecteur est prévenu, puis recharge.
	if st := a.ShareState(); st.ReadOnly != "" {
		t.Fatalf("auteur en lecture seule : %+v", st)
	}
	_, err = a.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	_, err = a.ShareRepublish(ctx, nil)
	mustShare(t, err)
	waitShare(t, hb, "nouvelle version", func(s workbook.State) bool { return s.NewVersion == 2 })
	_, err = b.ShareReload(ctx)
	mustShare(t, err)
	if st := b.ShareState(); st.Version != 2 || st.NewVersion != 0 {
		t.Fatalf("après rechargement : %+v", st)
	}
	res, err := b.Query(ctx, fmt.Sprintf(`SELECT c_2 FROM t_%d ORDER BY 1`, check.Tables[0].ID))
	mustShare(t, err)
	if got := fmt.Sprint(res.Rows); got != "[[2] [5]]" {
		t.Fatalf("prix rechargés : %s", got)
	}

	// Hors ligne : le cache se rouvre, en lecture seule.
	_ = b.CloseDocument(ctx)
	fake.SetDown(true)
	b2, hb2 := shareService(t)
	or, err := b2.Open(ctx, path)
	mustShare(t, err)
	if or.State.Path != path || or.State.Title != "Prix" || or.State.Remote == nil {
		t.Fatalf("réouverture : %+v", or)
	}
	waitShare(t, hb2, "hors ligne", func(s workbook.State) bool { return !s.Online && s.ReadOnly == "Hors ligne, lecture seule" })
	if _, err := b2.SetCell(ctx, 1, 1, 1, "9"); err == nil || !strings.Contains(err.Error(), "Hors ligne") {
		t.Fatalf("saisie hors ligne : %v", err)
	}
	// Renommé sur le serveur pendant la coupure : le titre suit à la
	// reconnexion.
	fake.SetTitle(got[0].ID, "Prix 2026")
	fake.SetDown(false)
	waitShare(t, hb2, "de retour", func(s workbook.State) bool { return s.Online })
	if ds, _ := b2.State(ctx); ds.Title != "Prix 2026" || ds.Remote.Title != "Prix 2026" {
		t.Fatalf("titre renommé : %+v", ds)
	}
	if rf := b2.Recent(ctx); len(rf) != 1 || rf[0].WorkbookID != got[0].ID {
		t.Fatalf("récents : %+v", rf)
	}
}
