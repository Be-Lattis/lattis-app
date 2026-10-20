package collab_test

// Essais tirés de l'audit du mode en ligne (docs/audits/audit-en-ligne-2026-10-20.md),
// faux serveur : chacun est un défaut corrigé, gardé en non-régression.

import (
	"context"
	"errors"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Constat 1 : une colonne renommée par une collègue pendant la coupure, alors
// qu'une saisie hors ligne la vise : l'envoi échoue (42703, errStale), la
// réception (qui relirait la correspondance) ne vient qu'après l'envoi.
func TestStaleColumnAfterColleague(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	f.SetDown(true)
	bob.d.Share.SetOnline(false, "serveur injoignable")
	bob.mustSet(0, 2, "9") // Prix
	f.SetDown(false)
	must(t, alice.header(2, "Tarif"))
	alice.send()

	bob.d.Share.SetOnline(true, "")
	for i := 0; i < 3; i++ {
		_, err := bob.e.Sync(ctx)
		t.Logf("essai %d : err=%v, file=%v", i+1, err, bob.states())
	}
	if bob.states()["pending"] > 0 {
		t.Fatalf("la file ne part pas (%v) ; colonnes de Bob : %v", bob.states(), colsOf(bob, "Ventes"))
	}
	// La saisie de Bob arrive sous le nouveau nom de la colonne.
	alice.receive()
	if got := alice.col("Tarif"); len(got) == 0 || got[0] != "9.0" {
		t.Errorf("Tarif chez Alice : %v, attendu 9.0 en premier", got)
	}
}

// Constat 1 : même chose avec un tableau supprimé par une conceptrice.
func TestStaleTableAfterColleague(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	f.SetDown(true)
	bob.d.Share.SetOnline(false, "serveur injoignable")
	bob.mustSet(0, 1, "99")
	f.SetDown(false)
	must(t, alice.d.DeleteTables(ctx, []string{"Ventes"}))
	alice.send()

	bob.d.Share.SetOnline(true, "")
	for i := 0; i < 3; i++ {
		_, err := bob.e.Sync(ctx)
		t.Logf("essai %d : err=%v, file=%v", i+1, err, bob.states())
	}
	if bob.states()["pending"] > 0 {
		t.Errorf("la file ne part pas (%v)", bob.states())
	}
}

// Même chose avec le faux serveur (l'erreur 42703/42P01 arrive à
// l'exécution, sans le problème du point de sauvegarde).
func TestStructureDuringInputFake(t *testing.T) {
	_, alice, bob, _ := setup(t)
	ctx := context.Background()
	bob.mustSet(1, 2, "8")
	must(t, alice.d.Model.RenameTable(ctx, "Ventes", "Achats"))
	alice.send()
	_, err1 := bob.e.Sync(ctx)
	_, err2 := bob.e.Sync(ctx)
	if err1 != nil || err2 != nil || bob.states()["pending"] != 0 {
		t.Fatalf("Bob : %v / %v, file %v", err1, err2, bob.states())
	}
}

// flaky : serveur dont la première mise à jour rend une erreur SQL
// passagère (interblocage 40P01).
type flaky struct {
	workbook.Server
	code string
	left int
}

type flakyWriter struct {
	workbook.Writer
	s *flaky
}

func (s *flaky) Write(ctx context.Context, fn func(workbook.Writer) error) error {
	return s.Server.Write(ctx, func(w workbook.Writer) error { return fn(flakyWriter{w, s}) })
}

func (w flakyWriter) Update(ctx context.Context, schema, table, id string, col workbook.RemoteColumn, old, new any) (workbook.Wrote, error) {
	if w.s.left > 0 {
		w.s.left--
		return workbook.Wrote{}, workbooktest.ErrServer{Code: w.s.code, Msg: "erreur passagère"}
	}
	return w.Writer.Update(ctx, schema, table, id, col, old, new)
}

// Constat 3 : une erreur SQL passagère (interblocage, délai dépassé) n'est
// pas un refus : la saisie reste en attente et part au tour suivant.
func TestTransientErrorKeepsEntry(t *testing.T) {
	for _, code := range []string{"40P01", "57014", "55P03"} {
		t.Run(code, func(t *testing.T) {
			ctx := context.Background()
			f, _, bob, _ := setup(t)
			f.SetDown(true)
			bob.d.Share.SetOnline(false, "serveur injoignable")
			bob.mustSet(0, 1, "77")
			f.SetDown(false)
			bob.e.SetServer(&flaky{Server: f.As("bob"), code: code, left: 1})
			bob.d.Share.SetOnline(true, "")
			// Premier essai : l'erreur passagère est une coupure, rien n'est défait.
			if _, err := bob.e.Sync(ctx); err == nil {
				t.Fatal("erreur passagère attendue")
			}
			if got := bob.col("Quantité")[0]; got != "77" || bob.states()["pending"] != 1 {
				t.Fatalf("après l'erreur passagère %s : Quantité %q, file %v (attendu 77 en attente)", code, got, bob.states())
			}
			// Essai suivant : l'étape part.
			r, err := bob.e.Sync(ctx)
			must(t, err)
			if r.Rejected != 0 || bob.col("Quantité")[0] != "77" || bob.states()["pending"] != 0 {
				t.Errorf("renvoi : %+v, Quantité %v, file %v", r, bob.col("Quantité"), bob.states())
			}
		})
	}
}

// Question 6 de l'audit : une étape que le serveur reçoit mais qui échoue
// toujours est mise de côté au bout de HeldAfter essais ; la réception
// continue, les modifications suivantes du même tableau l'attendent, et
// Réessayer les renvoie.
func TestHeldStep(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	fl := &flaky{Server: f.As("bob"), code: "40P01", left: 1000}
	bob.e.SetServer(fl)
	bob.mustSet(0, 1, "77")
	for i := 0; i < collab.HeldAfter; i++ {
		bob.e.Sync(ctx)
	}
	bob.e.Counts(ctx) // comme le suivi après chaque tour
	if st := bob.d.Share.State(); st.Held != 1 {
		t.Fatalf("étape mise de côté attendue : %+v, file %v", st, bob.states())
	}
	// Une modification suivante du même tableau attend avec elle.
	bob.mustSet(1, 1, "88")
	// La réception continue : Bob reçoit la saisie d'Alice.
	alice.mustSet(2, 2, "9")
	alice.send()
	_, err := bob.e.Sync(ctx)
	must(t, err)
	if got := bob.col("Prix")[2]; got != "9.0" {
		t.Errorf("réception bloquée : Prix de Bob %v", bob.col("Prix"))
	}
	if got := bob.col("Quantité"); got[0] != "77" || got[1] != "88" {
		t.Errorf("valeurs de Bob perdues : %v", got)
	}
	held, err := bob.e.HeldList(ctx)
	must(t, err)
	if len(held) != 1 || held[0].Name != "Ventes" || held[0].Steps != 2 {
		t.Fatalf("liste : %+v", held)
	}
	// Le serveur va mieux : Réessayer.
	fl.left = 0
	must(t, bob.e.RetryHeld(ctx, held[0].Table))
	_, err = bob.e.Sync(ctx)
	must(t, err)
	alice.receive()
	if got := alice.col("Quantité"); got[0] != "77" || got[1] != "88" {
		t.Errorf("Alice après Réessayer : %v", got)
	}
	bob.e.Counts(ctx)
	if st := bob.d.Share.State(); st.Held != 0 || bob.states()["pending"] != 0 {
		t.Errorf("encore de côté : %+v", st)
	}
}

// Abandonner : les modifications mises de côté sont défaites, le tableau
// reprend l'état du serveur.
func TestDropHeld(t *testing.T) {
	ctx := context.Background()
	f, _, bob, _ := setup(t)
	bob.e.SetServer(&flaky{Server: f.As("bob"), code: "40P01", left: 1000})
	bob.mustSet(0, 1, "77")
	for i := 0; i < collab.HeldAfter; i++ {
		bob.e.Sync(ctx)
	}
	held, err := bob.e.HeldList(ctx)
	must(t, err)
	if len(held) != 1 {
		t.Fatalf("liste : %+v", held)
	}
	_, err = bob.e.DropHeld(ctx, held[0].Table)
	must(t, err)
	if got := bob.col("Quantité")[0]; got != "3" {
		t.Errorf("Quantité après Abandonner : %q, attendu 3", got)
	}
}

// ambiguous : la transaction est validée sur le serveur, puis la réponse
// se perd (coupure pendant le COMMIT).
type ambiguous struct {
	workbook.Server
	left int
}

func (s *ambiguous) Write(ctx context.Context, fn func(workbook.Writer) error) error {
	err := s.Server.Write(ctx, fn)
	if err == nil && s.left > 0 {
		s.left--
		return errors.New("read tcp: connection reset by peer")
	}
	return err
}

// Constat 14 : coupure entre la validation du serveur et sa réponse : l'étape
// repart à la reconnexion ; pas de doublon. Une ligne ajoutée puis
// supprimée par une collègue entre les deux envois revient-elle ?
func TestAmbiguousCommit(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, _ := setup(t)
	f.SetDown(true)
	bob.d.Share.SetOnline(false, "serveur injoignable")
	must(t, bob.d.InsertRow(ctx, "Ventes", 3))
	bob.mustSet(3, 0, "Sucre")
	bob.mustSet(0, 1, "30")
	f.SetDown(false)
	bob.e.SetServer(&ambiguous{Server: f.As("bob"), left: 1})
	bob.d.Share.SetOnline(true, "")
	if _, err := bob.e.Sync(ctx); err == nil {
		t.Fatal("coupure attendue")
	}
	// Pendant la coupure (vue de Bob), Alice reçoit la ligne Sucre et la supprime.
	alice.receive()
	eq(t, "Alice reçoit", alice.col("Produit"), []string{"Café", "Thé", "Chocolat", ""})
	must(t, alice.d.DeleteRow(ctx, "Ventes", 3))
	alice.send()

	r, err := bob.e.Sync(ctx)
	must(t, err)
	t.Logf("renvoi : %+v", r)
	alice.receive()
	t.Logf("Alice : %v / %v ; Bob : %v / %v", alice.col("Produit"), alice.col("Quantité"), bob.col("Produit"), bob.col("Quantité"))
	if n := len(alice.col("Produit")); n != 3 {
		t.Errorf("la ligne supprimée par Alice revient (%d lignes)", n)
	}
	if r.Conflicts > 0 {
		t.Errorf("faux conflit sur sa propre écriture : %+v", r)
	}
}

// Constat 20 : un script enregistré par la propriétaire après la
// publication part chez les autres membres ; un éditeur ne modifie pas les
// scripts.
func TestScriptsShared(t *testing.T) {
	ctx := context.Background()
	_, alice, bob, _ := setup(t)
	_, err := alice.d.Model.SaveScript(ctx, 0, "Bonjour", `print("bonjour")`)
	must(t, err)
	_, err = alice.e.SyncScripts(ctx, false)
	must(t, err)
	rep, err := bob.e.SyncScripts(ctx, true)
	must(t, err)
	var src string
	must(t, bob.d.DB().Read().QueryRow(`SELECT source FROM _lattis_scripts WHERE name = 'Bonjour'`).Scan(&src))
	if src != `print("bonjour")` || len(rep.Notices) == 0 {
		t.Fatalf("Bob : %q, %v", src, rep.Notices)
	}
	// Alice le corrige : Bob reçoit la nouvelle version.
	var id int64
	must(t, alice.d.DB().Read().QueryRow(`SELECT id FROM _lattis_scripts WHERE name = 'Bonjour'`).Scan(&id))
	_, err = alice.d.Model.SaveScript(ctx, id, "Bonjour", `print("salut")`)
	must(t, err)
	_, err = alice.e.SyncScripts(ctx, false)
	must(t, err)
	_, err = bob.e.SyncScripts(ctx, true)
	must(t, err)
	must(t, bob.d.DB().Read().QueryRow(`SELECT source FROM _lattis_scripts WHERE name = 'Bonjour'`).Scan(&src))
	if src != `print("salut")` {
		t.Errorf("Bob après la correction : %q", src)
	}
	// Bob, éditeur : refusé.
	if _, err := bob.d.Model.SaveScript(ctx, 0, "À moi", `print(1)`); !errors.Is(err, canvas.ErrScriptsShared) {
		t.Errorf("éditeur : %v", err)
	}
}
