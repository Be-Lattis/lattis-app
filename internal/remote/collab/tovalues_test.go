package collab_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Convertir en valeurs une colonne calculée d'un tableau partagé : la
// colonne (mise en page jusque-là) devient une colonne du tableau sur le
// serveur, avec ses valeurs ; les collègues la reçoivent à la place de
// leur colonne calculée, sans doublon.

// setupCalc : comme setup, avec une colonne calculée Total
// (=[@Quantité]*[@Prix]) publiée dans la mise en page ; Bob (éditeur) et
// Carol (lectrice) ouvrent le classeur.
func setupCalc(t *testing.T) (f *workbooktest.Fake, alice, bob, carol *client, id string) {
	t.Helper()
	useRoot(t)
	ctx := context.Background()
	fk := workbooktest.New()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	_, err = d.Paste(ctx, 1, 0, 0, [][]string{
		{"Produit", "Quantité", "Prix"},
		{"Café", "3", "4,5"},
		{"Thé", "10", "2"},
		{"Chocolat", "1", "3,25"},
	})
	must(t, err)
	_, err = d.SetCell(ctx, 1, 0, 3, "Total")
	must(t, err)
	_, err = d.SetCell(ctx, 1, 1, 3, "=[@Quantité]*[@Prix]")
	must(t, err)
	var first string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = 1 AND row = 0 AND col = 0`).Scan(&first))
	must(t, d.RenameTable(ctx, first, "Ventes"))
	var tbl int64
	must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = 'Ventes'`).Scan(&tbl))
	info, err := workbook.Publish(ctx, fk.As("alice"), d.DB(), workbook.Request{Server: server, User: "alice", Title: "Ventes 2026", Mode: workbook.Collaborative, Tables: []int64{tbl}})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	alice = newClient(t, "alice", d, fk.As("alice"))
	id = info.WorkbookID
	if formulaOf(t, alice, "Total") == "" {
		t.Fatalf("Alice : pas de colonne calculée Total : %v", colsOf(alice, "Ventes"))
	}
	if cols, _ := serverCols(fk, id, "Ventes"); len(cols) != 3 {
		t.Fatalf("colonne calculée publiée : %v", cols)
	}
	must(t, fk.Grant(id, "bob", "editor"))
	bob = newClient(t, "bob", openAs(t, fk, id, "bob"), fk.As("bob"))
	must(t, fk.Grant(id, "carol", "reader"))
	carol = newClient(t, "carol", openAs(t, fk, id, "carol"), fk.As("carol"))
	for _, c := range []*client{bob, carol} {
		if formulaOf(t, c, "Total") == "" {
			t.Fatalf("%s : pas de colonne calculée Total : %v", c.name, colsOf(c, "Ventes"))
		}
	}
	return fk, alice, bob, carol, id
}

// formulaOf : formule d'une colonne de Ventes ("" : colonne saisie).
func formulaOf(t *testing.T, c *client, col string) string {
	t.Helper()
	var f string
	must(t, c.d.DB().Read().QueryRow(`SELECT coalesce(k.formula, '') FROM _lattis_columns k JOIN _lattis_tables t ON t.id = k.tbl WHERE t.name = 'Ventes' AND k.name = ?`, col).Scan(&f))
	return f
}

func TestConvertToValuesShared(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, carol, id := setupCalc(t)
	want := []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Total:decimal"}

	// Éditeur : refusé en local (structure et mise en page réservées).
	if err := bob.d.ColumnsToValues(ctx, "Ventes", []int{3}); !errors.Is(err, workbook.ErrDesign) && !errors.Is(err, workbook.ErrLayout) {
		t.Fatalf("éditeur : %v", err)
	}
	if formulaOf(t, bob, "Total") == "" || bob.states()["pending"] != 0 {
		t.Fatal("éditeur : colonne convertie")
	}

	// Propriétaire : une étape locale ; la colonne puis ses valeurs partent.
	must(t, alice.d.ColumnsToValues(ctx, "Ventes", []int{3}))
	eq(t, "Alice", colsOf(alice, "Ventes"), want)
	eq(t, "Alice : Total", alice.col("Total"), []string{"13.5", "20.0", "3.25"})
	if r := alice.send(); r.Rejected != 0 {
		t.Fatalf("envoi : %+v", r)
	}
	cols, _ := serverCols(f, id, "Ventes")
	eq(t, "serveur", cols, want)
	for n, v := range []float64{13.5, 20, 3.25} {
		if got, _, _, _ := f.Value(id, "Ventes", rowID(t, alice.d, n), "Total"); got != v {
			t.Fatalf("Total ligne %d sur le serveur : %v", n, got)
		}
	}
	alice.syncLayout(false)

	// Bob : la structure d'abord, puis la mise en page ; Carol : l'inverse.
	// Les deux gardent une seule colonne Total, saisie, avec les valeurs.
	_, err := bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	bob.syncLayout(true)
	carol.syncLayout(true)
	_, err = carol.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	for _, c := range []*client{bob, carol} {
		eq(t, c.name, colsOf(c, "Ventes"), want)
		eq(t, c.name+" : Total", c.col("Total"), []string{"13.5", "20.0", "3.25"})
		if formulaOf(t, c, "Total") != "" {
			t.Fatalf("%s : Total encore calculée", c.name)
		}
	}
	// Une valeur saisie ensuite par Bob (éditeur) arrive chez Alice.
	bob.mustSet(1, 3, "21")
	bob.send()
	alice.receive()
	eq(t, "Alice : Total saisi par Bob", alice.col("Total"), []string{"13.5", "21.0", "3.25"})

	// Annulation par Alice : la colonne redevient calculée, quitte le
	// serveur ; chez les autres, la colonne calculée revient (structure et
	// mise en page, dans un ordre ou dans l'autre).
	_, err = alice.d.Bus.Undo(ctx) // la saisie de Bob n'est pas dans l'historique d'Alice
	must(t, err)
	if formulaOf(t, alice, "Total") == "" {
		t.Fatalf("Alice après l'annulation : %v", colsOf(alice, "Ventes"))
	}
	if r := alice.send(); r.Rejected != 0 {
		t.Fatalf("envoi de l'annulation : %+v", r)
	}
	cols, _ = serverCols(f, id, "Ventes")
	eq(t, "serveur après l'annulation", cols, want[:3])
	alice.syncLayout(false)
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	bob.syncLayout(true)
	carol.syncLayout(true)
	_, err = carol.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	for _, c := range []*client{bob, carol} {
		got := colsOf(c, "Ventes")
		if len(got) != 4 || !strings.HasPrefix(got[3], "Total:") || formulaOf(t, c, "Total") == "" {
			t.Fatalf("%s après l'annulation : %v (formule %q)", c.name, got, formulaOf(t, c, "Total"))
		}
	}

	// Rétablie : de nouveau sur le serveur, avec les valeurs calculées.
	_, err = alice.d.Bus.Redo(ctx)
	must(t, err)
	alice.send()
	cols, _ = serverCols(f, id, "Ventes")
	eq(t, "serveur après le rétablissement", cols, want)
	if got, _, _, _ := f.Value(id, "Ventes", rowID(t, alice.d, 1), "Total"); got != 20.0 {
		t.Fatalf("Total rétabli sur le serveur : %v", got)
	}
}

// TestConvertToValuesRefused : le serveur refuse la colonne (un nom pris
// entre-temps) : elle redevient calculée chez l'auteur, la colonne du
// serveur arrive à côté. Une annulation refusée rend la colonne saisie.
func TestConvertToValuesRefused(t *testing.T) {
	ctx := context.Background()
	f, alice, _, _, id := setupCalc(t)
	tid, _ := remoteID(t, alice, "Produit")
	must(t, f.As("alice").Write(ctx, func(w workbook.Writer) error {
		_, err := w.AddColumn(ctx, id, tid, "Total", "text", "", 0)
		return err
	}))
	must(t, alice.d.ColumnsToValues(ctx, "Ventes", []int{3}))
	r := alice.send()
	if all := strings.Join(r.Notices, " ; "); r.Rejected != 1 || !strings.Contains(all, "Refusé par le serveur") || !strings.Contains(all, "déjà une colonne « Total »") {
		t.Fatalf("refus : %+v", r)
	}
	if formulaOf(t, alice, "Total") == "" {
		t.Fatalf("Total pas redevenue calculée : %v", colsOf(alice, "Ventes"))
	}
	got := strings.Join(colsOf(alice, "Ventes"), "|")
	if !strings.Contains(got, "Total 2:text") || strings.Count(got, "Total") != 2 {
		t.Fatalf("Alice après le refus : %s", got)
	}
	if alice.states()["pending"] != 0 {
		t.Fatalf("file : %v", alice.states())
	}
}

func TestConvertToValuesUndoRefused(t *testing.T) {
	ctx := context.Background()
	f, alice, _, _, id := setupCalc(t)
	must(t, alice.d.ColumnsToValues(ctx, "Ventes", []int{3}))
	alice.send()
	tid, _ := remoteID(t, alice, "Total")
	_, err := f.As("alice").AddRestriction(ctx, id, workbook.RestrictionRule{TableID: tid, Command: "select", Predicate: `"Total" > 0`, Label: "Positifs"})
	must(t, err)
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	r := alice.send()
	if all := strings.Join(r.Notices, " ; "); r.Rejected != 1 || !strings.Contains(all, "Annulation refusée par le serveur") {
		t.Fatalf("annulation refusée : %+v", r)
	}
	if formulaOf(t, alice, "Total") != "" {
		t.Fatal("Total toujours calculée après le refus")
	}
	eq(t, "Alice", colsOf(alice, "Ventes"), []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Total:decimal"})
	eq(t, "Total", alice.col("Total"), []string{"13.5", "20.0", "3.25"})
}

// Hors ligne (décision 0014) : la conversion attend dans la file et part à
// la reconnexion, colonne puis valeurs.
func TestConvertToValuesOffline(t *testing.T) {
	ctx := context.Background()
	f, alice, _, _, id := setupCalc(t)
	f.SetDown(true)
	alice.d.Share.SetOnline(false, "serveur injoignable")
	must(t, alice.d.ColumnsToValues(ctx, "Ventes", []int{3}))
	if alice.states()["pending"] == 0 {
		t.Fatalf("file hors ligne : %v", alice.states())
	}
	f.SetDown(false)
	alice.d.Share.SetOnline(true, "")
	if r := alice.send(); r.Rejected != 0 {
		t.Fatalf("reconnexion : %+v", r)
	}
	cols, _ := serverCols(f, id, "Ventes")
	eq(t, "serveur", cols, []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Total:decimal"})
	if got, _, _, _ := f.Value(id, "Ventes", rowID(t, alice.d, 2), "Total"); got != 3.25 {
		t.Fatalf("Total sur le serveur : %v", got)
	}
}
