package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// TestShareRemove : « Retirer du serveur » par le propriétaire d'un
// classeur ouvert depuis le serveur. Boîte Enregistrer sous annulée : rien
// n'est retiré. Sinon le classeur disparaît du serveur (drop_workbook) ; la
// copie ouverte devient un classeur local ordinaire, enregistré ailleurs,
// ses données gardées, modifiable, sans plus rien envoyer ; le cache et
// l'entrée des récents sont retirés.
func TestShareRemove(t *testing.T) {
	ctx := context.Background()
	s, h, fake, _, tbl, id, cache := offlineSetup(t)
	_, err := s.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	waitFor(t, "saisie envoyée", func() bool { return s.ShareState().Pending == 0 })

	// Boîte annulée : le classeur reste sur le serveur, toujours suivi.
	h.path = ""
	r, err := s.ShareRemove(ctx)
	mustShare(t, err)
	if !r.Cancelled || fake.Count("drop_workbook") != 0 {
		t.Fatalf("annulé : %+v, %d appels", r, fake.Count("drop_workbook"))
	}
	if st, _, _, _ := fake.Book(id); st == "" || !s.ShareState().Shared {
		t.Fatalf("annulé : classeur %q, partagé %v", st, s.ShareState().Shared)
	}

	h.path = filepath.Join(t.TempDir(), "Prix.lattis")
	r, err = s.ShareRemove(ctx)
	mustShare(t, err)
	if st, _, _, _ := fake.Book(id); st != "" {
		t.Fatalf("toujours sur le serveur : %q", st)
	}
	if r.State.Path != h.path || r.State.Remote != nil || s.ShareState().Shared || !strings.HasPrefix(r.State.Notice, "Classeur retiré du serveur : « Prix.lattis »") {
		t.Fatalf("retiré : %+v", r.State)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("cache encore là : %v", err)
	}
	if recentHas(s, id) {
		t.Fatal("toujours dans les récents")
	}
	if got := prices(t, s, tbl); got != "[[5] [2]]" {
		t.Fatalf("données : %s", got)
	}
	// Modifiable, rien ne part ni ne se suit.
	calls := fake.Count("my_capabilities") + fake.Count("listen")
	_, err = s.SetCell(ctx, 1, 2, 1, "8")
	mustShare(t, err)
	if n := outbox(t, s); n != 0 {
		t.Fatalf("file : %d", n)
	}
	time.Sleep(100 * time.Millisecond) // shareRetry : 20 ms
	if n := fake.Count("my_capabilities") + fake.Count("listen"); n != calls {
		t.Fatalf("suivi : %d appels de plus", n-calls)
	}
	mustShare(t, s.CloseDocument(ctx))
	if m, err := archive.ReadManifest(h.path); err != nil || m.Remote != nil {
		t.Fatalf("manifeste : %+v, %v", m, err)
	}
}

// TestShareRemoveMember : un membre (éditeur) ne peut pas retirer le
// classeur du serveur, ni par le pont ni par le serveur ; quand le
// propriétaire le retire (depuis son poste), la copie ouverte du membre
// passe à « supprimé du serveur » et se garde en classeur local.
func TestShareRemoveMember(t *testing.T) {
	ctx := context.Background()
	s, _, fake, _, tbl, id, _ := offlineSetup(t)
	mustShare(t, s.CloseDocument(ctx))
	mustShare(t, fake.Grant(id, "bob", "editor"))
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		return fake.As("bob"), auth.User{PGRole: "bob"}, "Serveur de test", nil
	}
	s2, h2 := shareService(t)
	sh2 := &saveHost{shareHost: h2}
	UseHost(sh2, nil, s2)
	_, err := s2.ShareOpen(ctx, "https://lattis.example.test", id, "")
	mustShare(t, err)
	waitShare(t, h2, "en ligne", func(s workbook.State) bool { return s.Online })
	if st := s2.ShareState(); st.CanManage {
		t.Fatalf("Bob propriétaire ? %+v", st)
	}

	sh2.path = filepath.Join(t.TempDir(), "Prix.lattis")
	if _, err := s2.ShareRemove(ctx); err == nil || !strings.Contains(err.Error(), "seul le propriétaire du classeur peut le retirer du serveur") {
		t.Fatalf("Bob : %v", err)
	}
	if err := fake.As("bob").RemoveWorkbook(ctx, id); !workbook.Privilege(err) {
		t.Fatalf("Bob sur le serveur : %v", err)
	}
	if st, _, _, _ := fake.Book(id); st == "" || !s2.ShareState().Shared {
		t.Fatal("retiré par un membre")
	}

	// Alice le retire depuis son poste : Bob le voit à son prochain envoi.
	mustShare(t, fake.As("alice").RemoveWorkbook(ctx, id))
	_, err = s2.SetCell(ctx, 1, 1, 1, "5")
	mustShare(t, err)
	waitShare(t, h2, "supprimé", func(s workbook.State) bool { return s.Gone == workbook.GoneDeleted })
	r, err := s2.ShareLocalCopy(ctx)
	mustShare(t, err)
	if r.State.Path != sh2.path || r.State.Remote != nil || s2.ShareState().Shared {
		t.Fatalf("copie gardée : %+v", r.State)
	}
	if got := prices(t, s2, tbl); got != "[[5] [2]]" {
		t.Fatalf("copie gardée : %s", got)
	}
}
