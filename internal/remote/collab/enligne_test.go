package collab_test

// Essais tirés de l'audit du mode en ligne (docs/audits/audit-en-ligne-2026-10-20.md),
// faux serveur : chacun est un défaut corrigé, gardé en non-régression.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"strings"
	"testing"
	"time"

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

// queryRows : résultat d'une requête (Produit, Prix) pour LoadQuery.
func queryRows(rows [][]any) canvas.ImportTable {
	return canvas.ImportTable{
		Headers: []string{"Produit", "Prix"},
		Types:   []grid.Type{grid.Text, grid.Decimal},
		Count:   len(rows),
		Rows: func(yield func([]any) error) error {
			for _, r := range rows {
				if err := yield(r); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// Constat 21 : un tableau chargé par une requête et publié s'actualise
// (lignes remplacées sur place, le tableau reste publié) et les autres
// membres reçoivent les nouvelles lignes.
func TestRefreshSharedQuery(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	f := workbooktest.New()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	qid, err := d.Model.SaveQuery(ctx, canvas.QueryDef{Name: "Tarifs", Source: `{"kind":"table","table":"X"}`})
	must(t, err)
	it := queryRows([][]any{{"Café", 4.5}, {"Thé", 2.0}})
	it.At = &canvas.QueryAnchor{Sheet: 1}
	l, err := d.Model.LoadQuery(ctx, qid, it)
	must(t, err)
	var tbl int64
	must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = ?`, l.Table).Scan(&tbl))
	info, err := workbook.Publish(ctx, f.As("alice"), d.DB(), workbook.Request{Server: server, User: "alice", Title: "Tarifs", Mode: workbook.Collaborative, Tables: []int64{tbl}})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	alice := newClient(t, "alice", d, f.As("alice"))
	must(t, f.Grant(info.WorkbookID, "bob", "editor"))
	bob := newClient(t, "bob", openAs(t, f, info.WorkbookID, "bob"), f.As("bob"))

	// Actualisation : trois lignes, dont une nouvelle.
	_, err = d.Model.LoadQuery(ctx, qid, queryRows([][]any{{"Café", 5.0}, {"Thé", 2.0}, {"Chocolat", 3.25}}))
	must(t, err)
	var same int64
	must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = ?`, l.Table).Scan(&same))
	if same != tbl {
		t.Fatalf("le tableau a été remplacé (%d → %d) au lieu de ses lignes", tbl, same)
	}
	alice.send()
	bob.receive()
	var n int
	must(t, bob.d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_tables WHERE name = ?`, l.Table).Scan(&n))
	rows := tableCol(t, bob, l.Table, "Produit")
	if strings.Join(rows, ",") != "Café,Thé,Chocolat" {
		t.Errorf("Bob reçoit %v", rows)
	}
	// Ctrl+Z rend les anciennes lignes.
	_, err = d.Bus.Undo(ctx)
	must(t, err)
	if got := tableCol(t, alice, l.Table, "Produit"); strings.Join(got, ",") != "Café,Thé" {
		t.Errorf("après Ctrl+Z : %v", got)
	}
}

// tableCol lit une colonne d'un tableau, dans l'ordre des lignes.
func tableCol(t *testing.T, c *client, table, col string) []string {
	t.Helper()
	var tbl, cid int64
	must(t, c.d.DB().Read().QueryRow(`SELECT t.id, k.id FROM _lattis_tables t JOIN _lattis_columns k ON k.tbl = t.id WHERE t.name = ? AND k.name = ?`, table, col).Scan(&tbl, &cid))
	rs, err := c.d.DB().Read().Query(fmt.Sprintf(`SELECT coalesce(c_%d, '') FROM t_%d ORDER BY _pos`, cid, tbl))
	must(t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var v string
		must(t, rs.Scan(&v))
		out = append(out, v)
	}
	return out
}

// freeRaw lit une cellule libre de la feuille 1.
func freeRaw(t *testing.T, c *client, row, col int) string {
	t.Helper()
	var raw string
	if err := c.d.DB().Read().QueryRow(`SELECT raw FROM _lattis_cells WHERE sheet = 1 AND row = ? AND col = ?`, row, col).Scan(&raw); err != nil {
		return ""
	}
	return raw
}

// Question 3 de l'audit : au retour d'une longue coupure, une cellule libre
// changée des deux côtés garde la version du collègue (plus récente), avec
// un message qui l'explique ; après une coupure courte, le poste qui
// envoie l'emporte, comme avant.
func TestLayoutLongAway(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name string
		away time.Duration
		want string
	}{{"courte", time.Minute, "Alice"}, {"longue", 48 * time.Hour, "Bob"}} {
		t.Run(c.name, func(t *testing.T) {
			f, alice, bob, id := setup(t)
			must(t, f.Grant(id, "bob", "designer"))
			_, err := bob.e.RefreshRights(ctx)
			must(t, err)
			_, err = alice.e.SyncLayout(ctx, true)
			must(t, err)
			_, err = bob.e.SyncLayout(ctx, true)
			must(t, err)
			// Alice hors ligne depuis c.away : elle change la cellule ; Bob aussi.
			must(t, alice.d.DB().Write(ctx, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `UPDATE _lattis_info SET value = ? WHERE key = 'layout_synced'`, time.Now().Add(-c.away).UTC().Format(time.RFC3339))
				return err
			}))
			_, err = bob.d.SetCell(ctx, 1, 10, 5, "Bob")
			must(t, err)
			_, err = bob.e.SyncLayout(ctx, true)
			must(t, err)
			_, err = alice.d.SetCell(ctx, 1, 10, 5, "Alice")
			must(t, err)
			rep, err := alice.e.SyncLayout(ctx, true)
			must(t, err)
			if got := freeRaw(t, alice, 10, 5); got != c.want {
				t.Errorf("Alice : %q, attendu %q (%v)", got, c.want, rep.Notices)
			}
			if c.want == "Bob" && !strings.Contains(strings.Join(rep.Notices, " "), "24 h") {
				t.Errorf("message : %v", rep.Notices)
			}
		})
	}
}

// Question 11 de l'audit : une suppression l'emporte sur une modification,
// mais celui dont la ligne modifiée disparaît en est prévenu.
func TestDeletedAfterMyEdit(t *testing.T) {
	ctx := context.Background()
	_, alice, bob, _ := setup(t)
	alice.mustSet(0, 1, "50")
	alice.send()
	must(t, bob.d.DeleteRow(ctx, "Ventes", 0))
	bob.send()
	rep, err := alice.e.Receive(ctx, nil)
	must(t, err)
	if !strings.Contains(strings.Join(rep.Notices, " "), "que vous aviez modifiée a été supprimée") {
		t.Errorf("message : %v", rep.Notices)
	}
}
