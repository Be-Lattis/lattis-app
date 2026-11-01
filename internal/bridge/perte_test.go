package bridge

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Tableaux perdus après « publié puis rouvert » : ce qui n'est pas sur le
// serveur (tableaux non cochés) ne doit jamais disparaître sans question.

const perteServer = "https://lattis.example.test"

// perteSetup : un service sur un faux serveur, cache et récents à part, et un
// classeur de deux tableaux : Ventes (publié ensuite) et Notes.
func perteSetup(t *testing.T) (*DocumentService, *workbooktest.Fake, string) {
	t.Helper()
	ctx := context.Background()
	fake := workbooktest.New()
	oldServer, oldDir, oldRetry, oldRecent := shareServer, shareCacheDir, shareRetry, recentFile
	cache := t.TempDir()
	shareCacheDir = func() (string, error) { return cache, nil }
	shareRetry = 20 * time.Millisecond
	recent := filepath.Join(t.TempDir(), "recent.json")
	recentFile = func() (string, error) { return recent, nil }
	shareServer = func(context.Context, string) (workbook.Server, auth.User, string, error) {
		return fake.As("alice"), auth.User{PGRole: "alice"}, "Serveur", nil
	}
	t.Cleanup(func() { shareServer, shareCacheDir, shareRetry, recentFile = oldServer, oldDir, oldRetry, oldRecent })
	s, _ := shareService(t)
	_, err := s.New(ctx)
	mustShare(t, err)
	_, err = s.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4,5"}})
	mustShare(t, err)
	_, err = s.Paste(ctx, 1, 0, 8, [][]string{{"Note", "Valeur"}, {"a", "1"}, {"b", "2"}})
	mustShare(t, err)
	d, err := s.current()
	mustShare(t, err)
	mustShare(t, d.RenameTable(ctx, perteAt(t, d, 0), "Ventes"))
	mustShare(t, d.RenameTable(ctx, perteAt(t, d, 8), "Notes"))
	return s, fake, cache
}

func perteAt(t *testing.T, d *doc.Document, col int) string {
	t.Helper()
	var n string
	mustShare(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE col = ?`, col).Scan(&n))
	return n
}

func perteTables(t *testing.T, s *DocumentService) []string {
	t.Helper()
	d, err := s.current()
	mustShare(t, err)
	rs, err := d.DB().Read().Query(`SELECT name FROM _lattis_tables ORDER BY name`)
	mustShare(t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var n string
		mustShare(t, rs.Scan(&n))
		out = append(out, n)
	}
	return out
}

func pertePublish(t *testing.T, s *DocumentService, mode string, names ...string) workbook.State {
	t.Helper()
	ctx := context.Background()
	check, err := s.ShareCheck(ctx)
	mustShare(t, err)
	var ids []int64
	for _, tb := range check.Tables {
		if slices.Contains(names, tb.Name) {
			ids = append(ids, tb.ID)
		}
	}
	st, err := s.SharePublish(ctx, ShareRequest{Server: perteServer, Title: "Essai", Mode: mode, Tables: ids})
	mustShare(t, err)
	return st
}

// Fichier d'auteur d'un classeur en direct qui a un tableau non coché : rouvert
// tel quel, pas depuis le serveur (le tableau disparaissait de la vue). Sans
// tableau privé, il s'ouvre toujours depuis le serveur.
func TestAuthorFileWithPrivateTableOpensItself(t *testing.T) {
	ctx := context.Background()
	workRoot(t)
	s, _, _ := perteSetup(t)
	d, err := s.current()
	mustShare(t, err)
	path := filepath.Join(t.TempDir(), "auteur.lattis")
	mustShare(t, saveDoc(ctx, d, path))
	pertePublish(t, s, workbook.Collaborative, "Ventes")
	if m, err := archive.ReadManifest(path); err != nil || m.Remote == nil || m.Remote.Private != 1 {
		t.Fatalf("manifeste : %+v %v", m.Remote, err)
	}
	mustShare(t, s.CloseDocument(ctx))
	r, err := s.Open(ctx, path)
	mustShare(t, err)
	if r.Shared != nil {
		t.Fatalf("rouvert depuis le serveur : Notes disparaît de la vue (%+v)", r.Shared)
	}
	if got := perteTables(t, s); !slices.Equal(got, []string{"Notes", "Ventes"}) {
		t.Fatalf("tableaux : %v", got)
	}
	// Plus de tableau privé : de nouveau depuis le serveur.
	mustShare(t, d2Delete(ctx, s, "Notes"))
	_, err = s.Save(ctx, false)
	mustShare(t, err)
	mustShare(t, s.CloseDocument(ctx))
	if r, err = s.Open(ctx, path); err != nil || r.Shared == nil {
		t.Fatalf("sans tableau privé : %+v %v", r, err)
	}
}

func d2Delete(ctx context.Context, s *DocumentService, table string) error {
	d, err := s.current()
	if err != nil {
		return err
	}
	return d.DeleteTables(ctx, []string{table})
}

// Édition web : publié sans fichier avec un tableau non coché, puis un autre
// classeur ouvert par un lien (/app/?workbook=<id>, autre onglet : aucune
// question possible). Le classeur quitté était effacé ; il reste récupérable.
func TestReplaceUnsavedKeepsRecovery(t *testing.T) {
	ctx := context.Background()
	workRoot(t)
	s, _, _ := perteSetup(t)
	st := pertePublish(t, s, workbook.Frozen, "Ventes")
	ds, err := s.State(ctx)
	mustShare(t, err)
	if !ds.Dirty || ds.PrivateTables != 1 {
		t.Fatalf("publié sans fichier : %+v", ds)
	}
	r, err := s.ShareOpen(ctx, perteServer, st.WorkbookID, "Essai")
	mustShare(t, err)
	if r.State.PrivateTables != 0 {
		t.Fatalf("copie du serveur : %+v", r.State)
	}
	if got := perteTables(t, s); !slices.Equal(got, []string{"Ventes"}) {
		t.Fatalf("copie du serveur : %v", got)
	}
	orphans, err := archive.Orphans()
	mustShare(t, err)
	if len(orphans) != 1 {
		t.Fatalf("classeur remplacé non gardé : %+v", orphans)
	}
	if _, err := s.Recover(ctx, orphans[0].Dir); err != nil {
		t.Fatal(err)
	}
	if got := perteTables(t, s); !slices.Equal(got, []string{"Notes", "Ventes"}) {
		t.Fatalf("récupéré : %v", got)
	}
}

// Moteur web arrêté (onglet fermé depuis 5 min) : le classeur non enregistré
// reste dans le répertoire de travail. Il n'est récupérable que si ce
// répertoire survit au moteur (voir docs/collaboration/version-web.md).
func TestEngineStopKeepsUnsaved(t *testing.T) {
	workRoot(t)
	s, _, _ := perteSetup(t)
	pertePublish(t, s, workbook.Collaborative, "Ventes")
	d, err := s.current()
	mustShare(t, err)
	mustShare(t, s.ServiceShutdown())
	// Fin du processus : le verrou du répertoire de travail tombe.
	s.mu.Lock()
	s.doc = nil
	s.mu.Unlock()
	d.Release()
	orphans, err := archive.Orphans()
	mustShare(t, err)
	if len(orphans) != 1 {
		t.Fatalf("orphelins : %+v", orphans)
	}
}

// « Ne pas enregistrer » : fermé pour de bon, rien à récupérer.
func TestCloseDontSaveLeavesNothing(t *testing.T) {
	ctx := context.Background()
	workRoot(t)
	s, _, _ := perteSetup(t)
	mustShare(t, s.CloseDocument(ctx))
	orphans, err := archive.Orphans()
	mustShare(t, err)
	if len(orphans) != 0 {
		t.Fatalf("orphelins : %+v", orphans)
	}
}
