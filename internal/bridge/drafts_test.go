package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// draftFast : délais courts pour la durée de l'essai.
func draftFast(t *testing.T) {
	t.Helper()
	oldD, oldR, oldM, oldF, oldG, oldT := draftDebounce, draftRetry, draftRetryMax, draftFlush, draftGrace, draftTick
	draftDebounce, draftRetry, draftRetryMax, draftFlush, draftGrace, draftTick = 5*time.Millisecond, 20*time.Millisecond, 80*time.Millisecond, 10*time.Second, 30*time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() {
		draftDebounce, draftRetry, draftRetryMax, draftFlush, draftGrace, draftTick = oldD, oldR, oldM, oldF, oldG, oldT
	})
}

// draftService : un service web, brouillons sur une réserve en mémoire.
func draftService(t *testing.T, store DraftStore) *DocumentService {
	t.Helper()
	draftFast(t)
	s := &DocumentService{}
	UseDrafts(s, store, t.TempDir())
	t.Cleanup(func() {
		s.stopDraft(false)
		s.CloseDocument(context.Background())
	})
	return s
}

func draftWaitState(t *testing.T, s *DocumentService, state string) {
	t.Helper()
	waitFor(t, "brouillon "+state, func() bool { return s.DraftStatus().State == state })
}

// draftCell lit une cellule libre d'un brouillon, restauré dans un autre service.
func draftCell(t *testing.T, data []byte, row, col int) string {
	t.Helper()
	const id = "00000000-0000-0000-0000-000000000001"
	store := workbooktest.NewDrafts()
	store.Save(context.Background(), id, "", data)
	s := draftService(t, store)
	_, err := s.Recover(context.Background(), draftPrefix+id)
	mustShare(t, err)
	got := freeCell(t, s, row, col)
	s.DraftForget()
	mustShare(t, s.CloseDocument(context.Background()))
	return got
}

// draftEngineGone : le moteur s'arrête (le dernier envoi part, comme dans
// ServiceShutdown) et son classeur est relâché : le répertoire peut être effacé.
func draftEngineGone(t *testing.T, s *DocumentService) {
	t.Helper()
	s.leaveDraft()
	s.mu.Lock()
	d := s.doc
	s.doc = nil
	s.mu.Unlock()
	mustShare(t, d.Release())
}

// Plusieurs modifications pendant un envoi lent : un seul envoi de plus, de
// l'état final ; les états intermédiaires sont sautés.
func TestDraftCoalesced(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	store := workbooktest.NewDrafts()
	store.Gate = make(chan struct{})
	store.Started = make(chan struct{}, 8)
	s := draftService(t, store)
	_, err := s.New(ctx)
	mustShare(t, err)
	_, err = s.SetCell(ctx, 1, 0, 0, "premier")
	mustShare(t, err)
	<-store.Started // l'envoi lent a commencé
	for i := range 30 {
		_, err = s.SetCell(ctx, 1, 1, 0, "v"+strings.Repeat("x", i))
		mustShare(t, err)
	}
	_, err = s.SetCell(ctx, 1, 1, 0, "final")
	mustShare(t, err)
	if st := s.DraftStatus().State; st != draftSaving {
		t.Fatalf("pendant l'envoi : %q", st)
	}
	close(store.Gate)
	draftWaitState(t, s, draftSaved)
	if tries, saves := store.Counts(); tries != 2 || saves != 2 {
		t.Fatalf("envois = %d (réussis %d), attendu 2", tries, saves)
	}
	if got := draftCell(t, store.Last(), 1, 0); got != "final" {
		t.Fatalf("dernier envoi : %q", got)
	}
}

// Échec : nouveaux essais espacés ; le serveur revenu, le dernier état part.
func TestDraftRetries(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	store := workbooktest.NewDrafts()
	store.Fail = errors.New("dial tcp : connexion refusée")
	s := draftService(t, store)
	_, err := s.New(ctx)
	mustShare(t, err)
	_, err = s.SetCell(ctx, 1, 0, 0, "a")
	mustShare(t, err)
	waitFor(t, "échec", func() bool { return s.DraftStatus() == DraftStatus{State: draftFailed, Reason: draftUnreachable, Active: true} })
	_, err = s.SetCell(ctx, 1, 0, 0, "b")
	mustShare(t, err)
	store.SetFail(nil)
	draftWaitState(t, s, draftSaved)
	if got := draftCell(t, store.Last(), 0, 0); got != "b" {
		t.Fatalf("après le retour du serveur : %q", got)
	}
}

// Moteur neuf, répertoire effacé : le classeur revient tel quel, historique
// d'annulation compris, et continue sur le même brouillon.
func TestDraftRestoreAfterWipe(t *testing.T) {
	root := workRoot(t)
	ctx := context.Background()
	store := workbooktest.NewDrafts()
	s := draftService(t, store)
	_, err := s.New(ctx)
	mustShare(t, err)
	_, err = s.Paste(ctx, 1, 0, 3, [][]string{{"Produit", "Prix"}, {"Café", "4,5"}})
	mustShare(t, err)
	_, err = s.SetCell(ctx, 1, 0, 0, "gardé")
	mustShare(t, err)
	draftEngineGone(t, s)
	ids := store.IDs()
	if len(ids) != 1 {
		t.Fatalf("brouillons = %v", ids)
	}
	mustShare(t, os.RemoveAll(root))

	s2 := draftService(t, store)
	o, err := s2.Orphans()
	mustShare(t, err)
	if len(o) != 1 || !strings.HasPrefix(o[0].Dir, draftPrefix) || o[0].Modified.IsZero() {
		t.Fatalf("proposés = %+v", o)
	}
	st, err := s2.Recover(ctx, o[0].Dir)
	mustShare(t, err)
	if !st.Dirty || st.Undo == "" {
		t.Fatalf("état restauré : %+v", st)
	}
	if got := freeCell(t, s2, 0, 0); got != "gardé" {
		t.Fatalf("cellule restaurée : %q", got)
	}
	if got := perteTables(t, s2); len(got) != 1 {
		t.Fatalf("tableaux restaurés : %v", got)
	}
	if s2.DraftStatus().State != draftSaved {
		t.Fatalf("état : %+v", s2.DraftStatus())
	}
	// Ctrl+Z, puis le même brouillon est mis à jour.
	_, saves := store.Counts()
	_, err = s2.Undo(ctx)
	mustShare(t, err)
	waitFor(t, "envoi après Ctrl+Z", func() bool { _, n := store.Counts(); return n > saves })
	if got := store.IDs(); len(got) != 1 || got[0] != ids[0] {
		t.Fatalf("brouillons après reprise = %v", got)
	}
	// Le brouillon du classeur ouvert n'est plus proposé.
	if o, _ := s2.Orphans(); len(o) != 0 {
		t.Fatalf("proposés avec le classeur ouvert = %+v", o)
	}
}

// Le fichier de la session revient au nom qu'il avait.
func TestDraftKeepsFileName(t *testing.T) {
	root := workRoot(t)
	ctx := context.Background()
	store := workbooktest.NewDrafts()
	s := draftService(t, store)
	_, err := s.New(ctx)
	mustShare(t, err)
	_, err = s.SetCell(ctx, 1, 0, 0, "x")
	mustShare(t, err)
	d, _ := s.current()
	mustShare(t, saveDoc(ctx, d, filepath.Join(t.TempDir(), "Ventes.lattis")))
	s.kickDraft()
	waitFor(t, "titre", func() bool {
		l, _ := store.List(ctx)
		return len(l) == 1 && l[0].Title == "Ventes.lattis" && s.DraftStatus().State == draftSaved
	})
	draftEngineGone(t, s)
	mustShare(t, os.RemoveAll(root))
	s2 := draftService(t, store)
	o, _ := s2.Orphans()
	if len(o) != 1 || o[0].Title != "Ventes.lattis" {
		t.Fatalf("proposés = %+v", o)
	}
	st, err := s2.Recover(ctx, o[0].Dir)
	mustShare(t, err)
	if st.Title != "Ventes" {
		t.Fatalf("titre = %q", st.Title)
	}
}

// Une session vide, ou un fichier seulement ouvert, n'écrit rien ; et rien
// n'écrase le brouillon d'un classeur laissé de côté, même rouvert ailleurs.
func TestDraftNeverOverwrites(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	store := workbooktest.NewDrafts()
	s := draftService(t, store)
	_, err := s.New(ctx)
	mustShare(t, err)
	time.Sleep(30 * time.Millisecond)
	if tries, _ := store.Counts(); tries != 0 {
		t.Fatalf("classeur vide : %d envois", tries)
	}
	_, err = s.SetCell(ctx, 1, 0, 0, "précieux")
	mustShare(t, err)
	draftWaitState(t, s, draftSaved)
	path := filepath.Join(t.TempDir(), "f.lattis")
	d, _ := s.current()
	mustShare(t, saveDoc(ctx, d, path))
	ids := store.IDs()
	draftEngineGone(t, s)

	// Autre session : le même fichier (même identifiant de classeur) rouvert.
	s2 := draftService(t, store)
	_, err = s2.Open(ctx, path)
	mustShare(t, err)
	time.Sleep(30 * time.Millisecond)
	if got := store.IDs(); len(got) != 1 {
		t.Fatalf("simple ouverture : %v", got)
	}
	_, err = s2.SetCell(ctx, 1, 0, 0, "autre")
	mustShare(t, err)
	draftWaitState(t, s2, draftSaved)
	got := store.IDs()
	if len(got) != 2 {
		t.Fatalf("brouillons = %v (avant : %v)", got, ids)
	}
	data, err := store.Read(ctx, ids[0])
	mustShare(t, err)
	if v := draftCell(t, data, 0, 0); v != "précieux" {
		t.Fatalf("brouillon écrasé : %q", v)
	}
}

// Au bureau (pas de réserve) et en démo publique : aucun brouillon.
func TestDraftNotInDesktopOrDemo(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	desk := &DocumentService{}
	t.Cleanup(func() { desk.CloseDocument(ctx) })
	_, err := desk.New(ctx)
	mustShare(t, err)
	_, err = desk.SetCell(ctx, 1, 0, 0, "a")
	mustShare(t, err)
	if desk.drafts.run != nil || desk.DraftStatus().State != "" {
		t.Fatal("brouillon au bureau")
	}
	UsePublicDemo(true)
	t.Cleanup(func() { UsePublicDemo(false) })
	store := workbooktest.NewDrafts()
	s := draftService(t, store)
	_, err = s.New(ctx)
	mustShare(t, err)
	_, err = s.SetCell(ctx, 1, 0, 0, "a")
	mustShare(t, err)
	time.Sleep(30 * time.Millisecond)
	if tries, _ := store.Counts(); tries != 0 || s.drafts.run != nil {
		t.Fatalf("démo publique : %d envois", tries)
	}
}

// Limite dépassée : second essai sans l'historique d'annulation, puis « trop
// volumineux » ; le brouillon précédent reste.
func TestDraftOverflow(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	store := workbooktest.NewDrafts()
	s := draftService(t, store)
	_, err := s.New(ctx)
	mustShare(t, err)
	_, err = s.SetCell(ctx, 1, 0, 0, "petit")
	mustShare(t, err)
	draftWaitState(t, s, draftSaved)
	store.Quota = 1 // plus aucune place
	before, _ := store.Read(ctx, store.IDs()[0])
	_, err = s.SetCell(ctx, 1, 0, 0, "grand")
	mustShare(t, err)
	waitFor(t, "trop gros", func() bool { return s.DraftStatus() == DraftStatus{State: draftFailed, Reason: draftTooBig, Active: true} })
	if tries, _ := store.Counts(); tries < 3 {
		t.Fatalf("essais = %d : un second, sans historique, était attendu", tries)
	}
	after, _ := store.Read(ctx, store.IDs()[0])
	if string(before) != string(after) {
		t.Fatal("brouillon précédent changé")
	}
	if !workbook.DraftOverflow(workbooktest.ErrSQL{Code: "LT001"}) || workbook.DraftOverflow(errors.New("x")) {
		t.Fatal("DraftOverflow")
	}
}

// Serveur d'avant les brouillons : rien à dire, les avertissements restent.
func TestDraftServerWithout(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	store := workbooktest.NewDrafts()
	store.Fail = workbooktest.ErrSQL{Code: "42883"}
	s := draftService(t, store)
	_, err := s.New(ctx)
	mustShare(t, err)
	_, err = s.SetCell(ctx, 1, 0, 0, "a")
	mustShare(t, err)
	waitFor(t, "abandon", func() bool { return s.drafts.missing.Load() })
	waitFor(t, "aucun état", func() bool { return s.DraftStatus().State == "" })
}

// « Ne pas enregistrer » retire le brouillon ; un classeur téléchargé tel quel
// aussi, à sa fermeture ; une fermeture simple le garde.
func TestDraftForget(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	store := workbooktest.NewDrafts()
	s := draftService(t, store)
	edit := func(v string) {
		t.Helper()
		_, err := s.New(ctx)
		mustShare(t, err)
		_, err = s.SetCell(ctx, 1, 0, 0, v)
		mustShare(t, err)
		draftWaitState(t, s, draftSaved)
	}
	edit("a")
	s.DraftForget()
	mustShare(t, s.CloseDocument(ctx))
	if got := store.IDs(); len(got) != 0 {
		t.Fatalf("après « Ne pas enregistrer » : %v", got)
	}

	edit("b")
	mustShare(t, s.CloseDocument(ctx))
	got := store.IDs()
	if len(got) != 1 {
		t.Fatalf("après fermeture simple : %v", got)
	}
	mustShare(t, s.Discard(draftPrefix+got[0]))
	if got := store.IDs(); len(got) != 0 {
		t.Fatalf("après abandon : %v", got)
	}

	edit("c")
	s.draftFetched()
	mustShare(t, s.CloseDocument(ctx))
	if got := store.IDs(); len(got) != 0 {
		t.Fatalf("téléchargé puis fermé : %v", got)
	}
}

// Un même contenu n'est pas renvoyé ; au-delà de la taille libre, une pause
// proportionnelle, que FlushDraft (plus d'onglet) coupe.
func TestDraftSpacing(t *testing.T) {
	for _, c := range []struct {
		size int
		want time.Duration
	}{{1000, 0}, {1 << 20, 2 * time.Second}, {5 << 20, 10 * time.Second}, {100 << 20, 60 * time.Second}} {
		if got := draftSpacing(c.size); got != c.want {
			t.Errorf("draftSpacing(%d) = %v, attendu %v", c.size, got, c.want)
		}
	}
	workRoot(t)
	ctx := context.Background()
	store := workbooktest.NewDrafts()
	s := draftService(t, store)
	_, err := s.New(ctx)
	mustShare(t, err)
	_, err = s.SetCell(ctx, 1, 0, 0, "a")
	mustShare(t, err)
	draftWaitState(t, s, draftSaved)
	_, first := store.Counts()
	s.kickDraft() // rien de changé
	time.Sleep(50 * time.Millisecond)
	draftWaitState(t, s, draftSaved)
	if _, n := store.Counts(); n != first {
		t.Fatalf("même contenu renvoyé : %d envois, avant %d", n, first)
	}
	// Pause longue après chaque envoi : le suivant attend, puis part sur FlushDraft.
	oldFree, oldPer, oldMax := draftFreeSize, draftPerMB, draftSpaceMax
	draftFreeSize, draftPerMB, draftSpaceMax = 0, time.Hour*1000, time.Hour
	t.Cleanup(func() { draftFreeSize, draftPerMB, draftSpaceMax = oldFree, oldPer, oldMax })
	_, err = s.SetCell(ctx, 1, 0, 0, "b")
	mustShare(t, err)
	waitFor(t, "b envoyé", func() bool { _, n := store.Counts(); return n == first+1 })
	_, err = s.SetCell(ctx, 1, 0, 0, "c")
	mustShare(t, err)
	time.Sleep(50 * time.Millisecond)
	if _, n := store.Counts(); n != first+1 {
		t.Fatalf("envoi pendant la pause : %d", n)
	}
	if st := s.DraftStatus().State; st != draftSaving {
		t.Fatalf("pendant la pause : %q", st)
	}
	FlushDraft(s)
	waitFor(t, "c envoyé", func() bool { _, n := store.Counts(); return n == first+2 })
	draftWaitState(t, s, draftSaved)
	if got := draftCell(t, store.Last(), 0, 0); got != "c" {
		t.Fatalf("dernier envoi : %q", got)
	}
}

// Classeur collaboratif : un tableau privé fait un brouillon ; tout sur le
// serveur, le brouillon est retiré.
func TestDraftCollaborative(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	draftFast(t)
	s, _, _ := perteSetup(t)
	store := workbooktest.NewDrafts()
	UseDrafts(s, store, t.TempDir())
	t.Cleanup(func() { s.stopDraft(false) })
	d, err := s.current()
	mustShare(t, err)
	s.attachDraft(d)
	draftWaitState(t, s, draftSaved)
	// Publié avec Ventes seulement : Notes n'est que dans le brouillon.
	pertePublish(t, s, workbook.Collaborative, "Ventes")
	waitFor(t, "brouillon après publication", func() bool { return s.DraftStatus().State == draftSaved && len(store.IDs()) == 1 })
	// Notes supprimé : plus rien que le serveur n'ait.
	_, err = s.DeleteTables(ctx, []string{"Notes"})
	mustShare(t, err)
	waitFor(t, "brouillon retiré", func() bool { return len(store.IDs()) == 0 && s.DraftStatus().State == "" })
}
