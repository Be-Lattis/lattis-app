package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
	"github.com/lalamefine/lattis/internal/work"
)

// progressServer : faux serveur (alice publie, bob lit) et un classeur figé
// de 400 lignes publié ; rend aussi l'adresse de lecture et le cache.
func progressServer(t *testing.T) (fake *workbooktest.Fake, author *DocumentService, id, cache string) {
	t.Helper()
	ctx := context.Background()
	fake = workbooktest.New()
	oldServer, oldDir, oldRetry, oldRecent := shareServer, shareCacheDir, shareRetry, recentFile
	cache = t.TempDir()
	shareCacheDir = func() (string, error) { return cache, nil }
	shareRetry = 20 * time.Millisecond
	recent := filepath.Join(t.TempDir(), "recent.json")
	recentFile = func() (string, error) { return recent, nil }
	t.Cleanup(func() { shareServer, shareCacheDir, shareRetry, recentFile = oldServer, oldDir, oldRetry, oldRecent })
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		u := "bob"
		if server == authorServer {
			u = "alice"
		}
		return fake.As(u), auth.User{PGRole: u}, "Serveur de test", nil
	}
	author = &DocumentService{}
	UseHost(&NoHost{}, nil, author)
	t.Cleanup(func() { author.CloseDocument(ctx) })
	_, err := author.New(ctx)
	mustShare(t, err)
	block := [][]string{{"Produit", "Prix"}}
	for range 400 {
		block = append(block, []string{"Café", "4"})
	}
	_, err = author.Paste(ctx, 1, 0, 0, block)
	mustShare(t, err)
	check, err := author.ShareCheck(ctx)
	mustShare(t, err)
	_, err = author.SharePublish(ctx, ShareRequest{Server: authorServer, Title: "Prix", Mode: workbook.Frozen, Tables: []int64{check.Tables[0].ID}})
	mustShare(t, err)
	list, err := fake.As("alice").Workbooks(ctx)
	mustShare(t, err)
	mustShare(t, fake.Grant(list[0].ID, "bob", "reader"))
	return fake, author, list[0].ID, cache
}

const (
	authorServer = "https://auteur.example.test"
	readerServer = "https://lattis.example.test"
)

// progressReader : poste du lecteur, opérations suivies sans délai.
func progressReader(t *testing.T) (*DocumentService, *workHost) {
	t.Helper()
	d, e := work.Delay, work.Every
	work.Delay, work.Every = 0, 0
	t.Cleanup(func() { work.Delay, work.Every = d, e })
	s := &DocumentService{}
	h := &workHost{}
	UseHost(h, nil, s)
	t.Cleanup(func() { s.CloseDocument(context.Background()) })
	return s, h
}

func cancelOnRows(s *DocumentService, h *workHost) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onStep = func(st work.Step) {
		if st.Phase == "rows" && st.Done > 0 {
			s.CancelWork(st.ID)
		}
	}
}

func noCancel(h *workHost) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onStep = nil
	h.steps = nil
}

func wantCancelled(t *testing.T, err error) {
	t.Helper()
	var m *msg.Error
	if !errors.As(err, &m) || m.ID != "work.cancelled" {
		t.Fatalf("erreur attendue : opération annulée ; reçu %v", err)
	}
}

// Ouverture depuis le serveur : la barre d'état dit la connexion, la mise en
// page, les lignes de chaque tableau et l'enregistrement, sans « Annuler »
// une fois le chargement validé. Annulée pendant la lecture des lignes : pas
// de cache créé, aucun classeur ouvert ; ouverte ensuite normalement.
func TestShareOpenProgressAndCancel(t *testing.T) {
	ctx := context.Background()
	_, _, id, cache := progressServer(t)
	s, h := progressReader(t)
	path := filepath.Join(cache, "lattis.example.test", id+".lattis")

	cancelOnRows(s, h)
	_, err := s.ShareOpen(ctx, readerServer, id, "Prix")
	wantCancelled(t, err)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cache créé par une ouverture annulée : %v", err)
	}
	if _, err := s.current(); err == nil {
		t.Fatal("classeur ouvert après l'annulation")
	}

	noCancel(h)
	r, err := s.ShareOpen(ctx, readerServer, id, "Prix")
	mustShare(t, err)
	if r.State.Path != path {
		t.Fatalf("cache : %+v", r.State)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	last := map[string]work.Step{}
	for _, st := range h.steps {
		if st.Op == "" {
			continue // sans délai (essais) : émise avant que l'action soit nommée
		}
		if st.Op != "openRemote" {
			t.Fatalf("action : %+v", st)
		}
		last[st.Phase] = st
	}
	for _, p := range []string{"connect", "readLayout", "rows", "commit", "save"} {
		if _, ok := last[p]; !ok {
			t.Fatalf("étape %s absente : %+v", p, last)
		}
	}
	if r := last["rows"]; r.Done != 400 || r.Item == "" || !r.Cancel {
		t.Fatalf("lignes lues : %+v", r)
	}
	if last["save"].Cancel {
		t.Fatalf("« Annuler » encore proposé à l'enregistrement : %+v", last["save"])
	}
}

// Rouverture d'un cache (classeur figé) annulée : le cache reste identique
// octet pour octet, et s'ouvre ensuite.
func TestShareOpenCancelKeepsCache(t *testing.T) {
	ctx := context.Background()
	_, _, id, cache := progressServer(t)
	s, h := progressReader(t)
	path := filepath.Join(cache, "lattis.example.test", id+".lattis")
	_, err := s.ShareOpen(ctx, readerServer, id, "Prix")
	mustShare(t, err)
	mustShare(t, s.CloseDocument(ctx))
	before, err := os.ReadFile(path)
	mustShare(t, err)

	cancelOnRows(s, h)
	_, err = s.ShareOpen(ctx, readerServer, id, "Prix")
	wantCancelled(t, err)
	after, err := os.ReadFile(path)
	mustShare(t, err)
	if string(before) != string(after) {
		t.Fatal("cache modifié par une rouverture annulée")
	}

	noCancel(h)
	_, err = s.ShareOpen(ctx, readerServer, id, "Prix")
	mustShare(t, err)
}

// « Nouvelle version disponible » : suivi aussi. Annulé, le classeur ouvert
// reste à l'ancienne version, intact ; relancé, il passe à la nouvelle.
func TestShareReloadProgressAndCancel(t *testing.T) {
	ctx := context.Background()
	_, author, id, _ := progressServer(t)
	s, h := progressReader(t)
	_, err := s.ShareOpen(ctx, readerServer, id, "Prix")
	mustShare(t, err)

	_, err = author.SetCell(ctx, 1, 1, 0, "Thé")
	mustShare(t, err)
	check, err := author.ShareCheck(ctx)
	mustShare(t, err)
	_, err = author.ShareRepublish(ctx, []int64{check.Tables[0].ID})
	mustShare(t, err)

	first := func() string {
		d, err := s.current()
		mustShare(t, err)
		var v string
		mustShare(t, d.DB().Read().QueryRow(`SELECT c_1 FROM t_1 ORDER BY _pos LIMIT 1`).Scan(&v))
		return v
	}
	if v := first(); v != "Café" {
		t.Fatalf("ancienne version : %q", v)
	}
	cancelOnRows(s, h)
	_, err = s.ShareReload(ctx)
	wantCancelled(t, err)
	if v := first(); v != "Café" {
		t.Fatalf("après l'annulation : %q", v)
	}
	if info := s.ShareState(); info.Version != 1 {
		t.Fatalf("version après l'annulation : %d", info.Version)
	}

	noCancel(h)
	_, err = s.ShareReload(ctx)
	mustShare(t, err)
	if v := first(); v != "Thé" {
		t.Fatalf("nouvelle version : %q", v)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	seen := map[string]bool{}
	for _, st := range h.steps {
		if st.Op == "" {
			continue
		}
		if st.Op != "reloadRemote" {
			t.Fatalf("action : %+v", st)
		}
		seen[st.Phase] = true
	}
	if !seen["rows"] || !seen["save"] {
		t.Fatalf("étapes : %v", seen)
	}
}

// Serveur lent (latence simulée) : les étapes arrivent, avec le vrai délai
// de la première émission ; serveur rapide (délai jamais atteint) : rien.
func TestShareOpenSlowServerShows(t *testing.T) {
	ctx := context.Background()
	fake, _, id, _ := progressServer(t)
	s, h := progressReader(t)
	work.Delay = 5 * time.Millisecond
	fake.Latency = 20 * time.Millisecond
	_, err := s.ShareOpen(ctx, readerServer, id, "Prix")
	mustShare(t, err)
	fake.Latency = 0
	h.mu.Lock()
	n := len(h.steps)
	h.steps = nil
	h.mu.Unlock()
	if n == 0 {
		t.Fatal("serveur lent : aucune étape montrée")
	}
	mustShare(t, s.CloseDocument(ctx))

	work.Delay = time.Minute
	_, err = s.ShareOpen(ctx, readerServer, id, "Prix")
	mustShare(t, err)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.steps) != 0 {
		t.Fatalf("opération courte montrée : %+v", h.steps)
	}
}
