package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// recentHas dit si la liste des récents propose encore le classeur id du
// serveur.
func recentHas(s *DocumentService, id string) bool {
	for _, r := range s.Recent(context.Background()) {
		if r.WorkbookID == id {
			return true
		}
	}
	return false
}

// outbox compte les étapes en attente d'envoi du classeur ouvert.
func outbox(t *testing.T, s *DocumentService) int {
	t.Helper()
	d, err := s.current()
	mustShare(t, err)
	var n int
	mustShare(t, d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_outbox`).Scan(&n))
	return n
}

// TestShareGoneDeleted : un serveur coupé met le classeur hors ligne ; un
// classeur supprimé du serveur n'est pas « hors ligne » : son cache se
// rouvre avec l'état « supprimé », sans nouveaux essais, ses saisies ne
// partent plus ; « Enregistrer une copie locale… » garde la modification
// jamais envoyée, retire le cache et l'entrée des récents.
func TestShareGoneDeleted(t *testing.T) {
	ctx := context.Background()
	s, h, fake, _, tbl, id, cache := offlineSetup(t)

	fake.SetDown(true)
	waitShare(t, h.shareHost, "hors ligne", func(s workbook.State) bool { return !s.Online && s.Offline != "" && s.Gone == "" })
	_, err := s.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	mustShare(t, s.CloseDocument(ctx))

	fake.DropWorkbook(id)
	fake.SetDown(false)
	s2, h2 := shareService(t)
	sh2 := &saveHost{shareHost: h2}
	UseHost(sh2, nil, s2)
	_, err = s2.ShareOpen(ctx, "https://lattis.example.test", id, "")
	mustShare(t, err)
	waitShare(t, h2, "supprimé", func(s workbook.State) bool { return s.Gone == workbook.GoneDeleted })
	if st := s2.ShareState(); st.Online || st.Offline != "" || st.Pending != 1 {
		t.Fatalf("supprimé : %+v", st)
	}
	calls := fake.Count("my_capabilities") + fake.Count("listen")
	time.Sleep(200 * time.Millisecond) // shareRetry : 20 ms
	if n := fake.Count("my_capabilities") + fake.Count("listen"); n != calls {
		t.Fatalf("nouveaux essais : %d appels de plus", n-calls)
	}
	_, err = s2.SetCell(ctx, 1, 2, 1, "8")
	mustShare(t, err)
	if n := outbox(t, s2); n != 1 {
		t.Fatalf("file : %d", n)
	}

	sh2.path = filepath.Join(t.TempDir(), "Prix.lattis")
	r, err := s2.ShareLocalCopy(ctx)
	mustShare(t, err)
	if r.State.Path != sh2.path || r.State.Remote != nil || !strings.Contains(r.State.Notice, "1 modification n'avait pas été envoyée") {
		t.Fatalf("copie : %+v", r.State)
	}
	if got := prices(t, s2, tbl); got != "[[5] [8]]" {
		t.Fatalf("copie : %s", got)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("cache gardé : %v", err)
	}
	if recentHas(s2, id) {
		t.Fatal("classeur supprimé toujours dans les récents")
	}
}

// TestShareGoneForget : supprimé pendant qu'il est ouvert (vu au premier
// envoi), la saisie reste en file ; « Retirer des récents » le ferme,
// efface son cache et le retire de la liste ; rouvert sans cache depuis le
// serveur : une erreur qui dit la suppression, pas une coupure.
func TestShareGoneForget(t *testing.T) {
	ctx := context.Background()
	s, h, fake, _, _, id, cache := offlineSetup(t)
	if !recentHas(s, id) {
		t.Fatal("récents")
	}
	fake.DropWorkbook(id)
	_, err := s.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	waitShare(t, h.shareHost, "supprimé", func(s workbook.State) bool { return s.Gone == workbook.GoneDeleted })
	if n := outbox(t, s); n != 1 {
		t.Fatalf("file : %d", n)
	}
	mustShare(t, s.ShareForget(ctx))
	if _, err := s.current(); err == nil {
		t.Fatal("classeur toujours ouvert")
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("cache gardé : %v", err)
	}
	if recentHas(s, id) {
		t.Fatal("toujours dans les récents")
	}
	_, err = s.ShareOpen(ctx, "https://lattis.example.test", id, "")
	if err == nil || err.Error() != "Ce classeur a été supprimé de Serveur de test." {
		t.Fatalf("ouverture : %v", err)
	}
}
