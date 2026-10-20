package collab_test

// Essais tirés de l'audit du mode en ligne (docs/audits/audit-en-ligne-2026-10-20.md),
// faux serveur : chacun est un défaut corrigé, gardé en non-régression.

import (
	"context"
	"testing"

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
