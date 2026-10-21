package bridge

// Essais tirés de l'audit du mode en ligne (docs/audits/audit-en-ligne-2026-10-20.md).

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	mustShare(t, os.MkdirAll(to, 0o755))
	es, err := os.ReadDir(from)
	mustShare(t, err)
	for _, e := range es {
		if e.IsDir() || e.Name() == "lattis.lock" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(from, e.Name()))
		mustShare(t, err)
		mustShare(t, os.WriteFile(filepath.Join(to, e.Name()), b, 0o644))
	}
}

// Constat 2 : plantage pendant une session hors ligne, puis réouverture
// depuis le serveur (récents) une fois celui-ci revenu : la session
// interrompue est proposée en récupération, le cache n'est pas rechargé du
// serveur par-dessus sa file.
func TestShareOpenAfterCrash(t *testing.T) {
	ctx := context.Background()
	root := workRoot(t)
	s, h, fake, _, tbl, id, cache := offlineSetup(t)

	// Session 1 : hors ligne, saisie A, fermeture : le cache garde la file.
	fake.SetDown(true)
	waitShare(t, h.shareHost, "hors ligne", func(s workbook.State) bool { return !s.Online })
	_, err := s.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	mustShare(t, s.CloseDocument(ctx))

	// Session 2 : rouvert hors ligne, saisie B, puis « plantage ».
	s2, h2 := shareService(t)
	_, err = s2.Open(ctx, cache)
	mustShare(t, err)
	waitShare(t, h2, "hors ligne", func(s workbook.State) bool { return !s.Online })
	_, err = s2.SetCell(ctx, 1, 2, 1, "9")
	mustShare(t, err)
	before, err := os.ReadFile(cache)
	mustShare(t, err)
	sum := sha256.Sum256([]byte(archive.PathKey(cache)))
	wd := filepath.Join(root, hex.EncodeToString(sum[:8]))
	backup := filepath.Join(t.TempDir(), "wd")
	copyTree(t, wd, backup)
	mustShare(t, s2.CloseDocument(ctx))
	// État après un plantage : cache d'avant la session, répertoire orphelin.
	mustShare(t, os.WriteFile(cache, before, 0o644))
	copyTree(t, backup, wd)
	if os, _ := archive.Orphans(); len(os) != 1 {
		t.Fatalf("orphelins : %+v", os)
	}

	// Le serveur revient ; l'utilisateur rouvre depuis les récents.
	fake.SetDown(false)
	s3, h3 := shareService(t)
	UseHost(h3, nil, s3)
	r, err := s3.ShareOpen(ctx, "https://lattis.example.test", id, "")
	mustShare(t, err)
	if r.Recoverable == nil {
		t.Fatalf("récupération non proposée : en attente=%d, prix=%s", s3.ShareState().Pending, prices(t, s3, tbl))
	}
	after, err := os.ReadFile(cache)
	mustShare(t, err)
	if string(after) != string(before) {
		t.Error("le cache a été réécrit")
	}
	// La session récupérée garde ses deux saisies et sa file.
	_, err = s3.Recover(ctx, r.Recoverable.Dir)
	mustShare(t, err)
	if st := s3.ShareState(); st.Pending == 0 {
		t.Errorf("file perdue après récupération : %+v", st)
	}
}

// freeCell lit une cellule libre de la feuille 1 du classeur ouvert.
func freeCell(t *testing.T, s *DocumentService, row, col int) string {
	t.Helper()
	d, err := s.current()
	mustShare(t, err)
	var raw string
	err = d.DB().Read().QueryRow(`SELECT raw FROM _lattis_cells WHERE sheet = 1 AND row = ? AND col = ?`, row, col).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ""
	}
	mustShare(t, err)
	return raw
}

// Constat 13 : hors ligne, une cellule libre saisie par la conceptrice puis
// un autre classeur ouvert (« Ne pas enregistrer ») : la mise en page en
// attente est gardée dans le cache et revient à la réouverture.
func TestOpenOtherKeepsLayout(t *testing.T) {
	ctx := context.Background()
	s, h, fake, _, _, _, cache := offlineSetup(t)
	fake.SetDown(true)
	waitShare(t, h.shareHost, "hors ligne", func(s workbook.State) bool { return !s.Online })
	_, err := s.SetCell(ctx, 1, 10, 5, "note")
	mustShare(t, err)
	_, err = s.New(ctx)
	mustShare(t, err)
	_, err = s.Open(ctx, cache)
	mustShare(t, err)
	if got := freeCell(t, s, 10, 5); got != "note" {
		t.Errorf("cellule libre après réouverture : %q, attendu « note »", got)
	}
}

// Constat 6 : le cache d'un classeur hors ligne ne peut pas être enregistré
// à la fermeture (fichier en lecture seule) : la file n'est pas perdue, le
// répertoire de travail reste proposé en récupération, et l'utilisateur le
// sait.
func TestCloseKeepsQueueWhenCacheFails(t *testing.T) {
	ctx := context.Background()
	root := workRoot(t)
	s, h, fake, _, _, _, cache := offlineSetup(t)
	fake.SetDown(true)
	waitShare(t, h.shareHost, "hors ligne", func(s workbook.State) bool { return !s.Online })
	_, err := s.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	mustShare(t, os.Chmod(cache, 0o444))
	t.Cleanup(func() { os.Chmod(cache, 0o644) })
	// Hors Windows, un fichier en lecture seule se remplace encore (écriture
	// dans un fichier voisin puis renommage) : c'est son dossier qui l'empêche.
	if runtime.GOOS != "windows" {
		mustShare(t, os.Chmod(filepath.Dir(cache), 0o555))
		t.Cleanup(func() { os.Chmod(filepath.Dir(cache), 0o755) })
	}
	err = s.CloseDocument(ctx)
	if err == nil {
		t.Fatal("aucun message alors que le cache n'a pas pu être enregistré")
	}
	t.Log(err)
	if len(workDirs(t, root)) == 0 {
		t.Fatal("répertoire de travail supprimé : la file est perdue")
	}
	if o, _ := archive.Orphans(); len(o) != 1 {
		t.Errorf("session à récupérer : %+v", o)
	}
}

// Question 7 de l'audit : un classeur collaboratif ouvert depuis le serveur
// n'est jamais « modifié » (ses modifications partent d'elles-mêmes) ; à la
// fermeture, son cache est enregistré sans question, y compris ce qui ne
// part pas au serveur (cellule d'un tableau privé, ici une cellule libre
// hors ligne).
func TestLiveCacheNeverDirty(t *testing.T) {
	ctx := context.Background()
	s, h, fake, _, _, _, cache := offlineSetup(t)
	_, err := s.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	if st, _ := s.State(ctx); st.Dirty {
		t.Fatal("classeur du serveur marqué modifié")
	}
	if s.windowClosing(time.Now()) != closeAllow {
		t.Error("la fermeture de la fenêtre pose une question")
	}
	// Hors ligne, une modification en attente : la fenêtre demande.
	fake.SetDown(true)
	waitShare(t, h.shareHost, "hors ligne", func(s workbook.State) bool { return !s.Online })
	_, err = s.SetCell(ctx, 1, 10, 5, "note")
	mustShare(t, err)
	_, err = s.SetCell(ctx, 1, 2, 1, "7")
	mustShare(t, err)
	if s.windowClosing(time.Now()) == closeAllow {
		t.Error("fermeture sans prévenir des modifications en attente")
	}
	s.CloseRequestSeen()
	mustShare(t, s.CloseDocument(ctx))
	_, err = s.Open(ctx, cache)
	mustShare(t, err)
	if got := freeCell(t, s, 10, 5); got != "note" {
		t.Errorf("cache pas enregistré à la fermeture : cellule libre %q", got)
	}
}
