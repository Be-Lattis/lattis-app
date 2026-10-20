package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// saveHost : shareHost dont la boîte Enregistrer sous répond path.
type saveHost struct {
	*shareHost
	path string
}

func (h *saveHost) SaveFile(_ FileDialog, write func(string) error) (string, error) {
	if h.path == "" {
		return "", nil
	}
	return h.path, write(h.path)
}

// offlineSetup publie (Alice, hors du pont) un classeur collaboratif Prix
// et l'ouvre par le pont, depuis le serveur, sous l'identité de son
// propriétaire ; rend le service, son hôte, le faux serveur, le classeur
// d'Alice, le tableau et le chemin du cache.
func offlineSetup(t *testing.T) (*DocumentService, *saveHost, *workbooktest.Fake, *doc.Document, int64, string, string) {
	t.Helper()
	ctx := context.Background()
	fake := workbooktest.New()
	oldServer, oldDir, oldRetry, oldDebounce, oldRecent := shareServer, shareCacheDir, shareRetry, collab.Debounce, recentFile
	cache := t.TempDir()
	shareCacheDir = func() (string, error) { return cache, nil }
	recent := filepath.Join(t.TempDir(), "recent.json")
	recentFile = func() (string, error) { return recent, nil }
	shareRetry, collab.Debounce = 20*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() {
		shareServer, shareCacheDir, shareRetry, collab.Debounce, recentFile = oldServer, oldDir, oldRetry, oldDebounce, oldRecent
	})
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		return fake.As("alice"), auth.User{PGRole: "alice"}, "Serveur de test", nil
	}
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

	s, h := shareService(t)
	sh := &saveHost{shareHost: h}
	UseHost(sh, nil, s)
	r, err := s.ShareOpen(ctx, "https://lattis.example.test", info.WorkbookID, "")
	mustShare(t, err)
	waitShare(t, h, "en ligne", func(s workbook.State) bool { return s.Online })
	return s, sh, fake, ad, tbl, info.WorkbookID, r.State.Path
}

// prices lit la colonne Prix du classeur ouvert.
func prices(t *testing.T, s *DocumentService, tbl int64) string {
	t.Helper()
	res, err := s.Query(context.Background(), fmt.Sprintf(`SELECT c_2 FROM t_%d ORDER BY _pos`, tbl))
	mustShare(t, err)
	return fmt.Sprint(res.Rows)
}

// TestShareOfflineEditReopen : hors ligne, le classeur collaboratif reste
// modifiable ; fermé puis rouvert hors ligne, ses modifications attendent
// toujours (cache enregistré avec sa file) ; le serveur revenu, elles
// partent.
func TestShareOfflineEditReopen(t *testing.T) {
	ctx := context.Background()
	s, h, fake, ad, tbl, id, cache := offlineSetup(t)

	fake.SetDown(true)
	waitShare(t, h.shareHost, "hors ligne", func(s workbook.State) bool { return !s.Online })
	_, err := s.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	if st := s.ShareState(); st.ReadOnly != "" || st.Pending != 1 {
		t.Fatalf("hors ligne : %+v", st)
	}
	// Fermé sans enregistrer : le cache garde la file.
	mustShare(t, s.CloseDocument(ctx))
	if m, err := archive.ReadManifest(cache); err != nil || m.Remote == nil {
		t.Fatalf("cache : %+v, %v", m, err)
	}

	// Rouvert plus tard, toujours hors ligne.
	s2, h2 := shareService(t)
	or, err := s2.Open(ctx, cache)
	mustShare(t, err)
	if or.State.Remote == nil {
		t.Fatalf("réouverture : %+v", or.State)
	}
	waitShare(t, h2, "hors ligne à la réouverture", func(s workbook.State) bool { return !s.Online && s.Pending == 1 && s.ReadOnly == "" })
	if got := prices(t, s2, tbl); got != "[[5] [2]]" {
		t.Fatalf("saisie gardée : %s", got)
	}
	_, err = s2.SetCell(ctx, 1, 2, 1, "9")
	mustShare(t, err)

	// Le serveur revient : « Réessayer », tout part.
	fake.SetDown(false)
	s2.ShareRetry()
	waitShare(t, h2, "de retour", func(s workbook.State) bool { return s.Online })
	waitFor(t, "saisies hors ligne sur le serveur", func() bool {
		a, _, _, _ := fake.Value(id, "Feuille1", rowAt(t, ad, tbl, 0), "Prix")
		b, _, _, _ := fake.Value(id, "Feuille1", rowAt(t, ad, tbl, 1), "Prix")
		return a == 5.0 && b == 9.0
	})
	waitFor(t, "file vide", func() bool { return s2.ShareState().Pending == 0 })
}

// TestShareDetach : « Travailler sur une copie locale » (copie sans lien,
// modifications pas envoyées comprises ; le cache et le serveur
// inchangés), puis « Détacher du serveur » d'un classeur ouvert depuis le
// serveur (enregistré ailleurs, cache retiré), et d'un fichier publié.
func TestShareDetach(t *testing.T) {
	ctx := context.Background()
	s, h, fake, ad, tbl, id, cache := offlineSetup(t)
	fake.SetDown(true)
	waitShare(t, h.shareHost, "hors ligne", func(s workbook.State) bool { return !s.Online })
	_, err := s.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)

	// Copie locale.
	dir := t.TempDir()
	h.path = filepath.Join(dir, "Prix (copie locale).lattis")
	r, err := s.ShareLocalCopy(ctx)
	mustShare(t, err)
	if r.State.Path != h.path || r.State.Remote != nil || s.ShareState().Shared || !strings.Contains(r.State.Notice, "1 modification n'avait pas été envoyée") {
		t.Fatalf("copie : %+v", r.State)
	}
	if got := prices(t, s, tbl); got != "[[5] [2]]" {
		t.Fatalf("copie : %s", got)
	}
	if m, err := archive.ReadManifest(h.path); err != nil || m.Remote != nil {
		t.Fatalf("manifeste de la copie : %+v, %v", m, err)
	}
	var n int
	d, err := s.current()
	mustShare(t, err)
	mustShare(t, d.DB().Read().QueryRow(`SELECT (SELECT count(*) FROM _lattis_outbox) + (SELECT count(*) FROM _lattis_remote_tables) + (SELECT count(*) FROM _lattis_row_ids) + (SELECT count(*) FROM _lattis_info WHERE key LIKE 'remote%')`).Scan(&n))
	if n != 0 {
		t.Fatalf("copie : %d traces du partage", n)
	}
	// La copie se modifie librement, rien ne part.
	_, err = s.SetCell(ctx, 1, 2, 1, "7")
	mustShare(t, err)
	if m, err := archive.ReadManifest(cache); err != nil || m.Remote == nil {
		t.Fatalf("cache après la copie : %+v, %v", m, err)
	}

	// Le cache se rouvre partagé, sa file intacte ; son propriétaire le
	// détache : enregistré ailleurs, cache retiré.
	or, err := s.Open(ctx, cache)
	mustShare(t, err)
	if or.State.Remote == nil {
		t.Fatalf("cache rouvert : %+v", or.State)
	}
	waitShare(t, h.shareHost, "cache rouvert", func(s workbook.State) bool { return s.Shared && s.Pending == 1 })
	h.path = filepath.Join(dir, "Prix.lattis")
	r, err = s.ShareDetach(ctx)
	mustShare(t, err)
	if r.State.Path != h.path || s.ShareState().Shared || !strings.Contains(r.State.Notice, "Classeur détaché du serveur") {
		t.Fatalf("détaché : %+v", r.State)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("cache encore là : %v", err)
	}
	if got := prices(t, s, tbl); got != "[[5] [2]]" {
		t.Fatalf("détaché : %s", got)
	}
	// Rouvert : un classeur local ordinaire.
	mustShare(t, s.CloseDocument(ctx))
	or, err = s.Open(ctx, h.path)
	mustShare(t, err)
	if or.State.Remote != nil || s.ShareState().Shared {
		t.Fatalf("rouvert : %+v", or.State)
	}

	// Le serveur n'a rien reçu.
	fake.SetDown(false)
	if v, _, _, _ := fake.Value(id, "Feuille1", rowAt(t, ad, tbl, 0), "Prix"); v != 4.5 {
		t.Fatalf("serveur : %v", v)
	}

	// Fichier publié (pas un cache) : détaché sur place.
	pub := filepath.Join(dir, "Publié.lattis")
	mustShare(t, ad.Save(ctx, pub))
	mustShare(t, s.CloseDocument(ctx))
	// Ouvrir le fichier d'un classeur collaboratif renvoie vers le serveur
	// (Shared) ; OpenLocalCopy l'ouvre tel quel.
	if or, err := s.Open(ctx, pub); err != nil || or.Shared == nil {
		t.Fatalf("Open du fichier publié : %+v %v", or, err)
	}
	_, err = s.OpenLocalCopy(ctx, pub)
	mustShare(t, err)
	if !s.ShareState().Shared || !s.ShareState().CanManage {
		t.Fatalf("fichier publié : %+v", s.ShareState())
	}
	h.path = ""
	r, err = s.ShareDetach(ctx)
	mustShare(t, err)
	if r.State.Path != pub || s.ShareState().Shared {
		t.Fatalf("détaché sur place : %+v", r.State)
	}
	if m, err := archive.ReadManifest(pub); err != nil || m.Remote != nil {
		t.Fatalf("manifeste : %+v, %v", m, err)
	}
}
