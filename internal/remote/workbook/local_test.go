package workbook_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// derivedAuthor : un tableau Ventes et un tableau lié à une formule qui le
// filtre (Source_carte, comme une source de carte) : un tableau dérivé.
func derivedAuthor(t *testing.T) (*doc.Document, string) {
	t.Helper()
	ctx := context.Background()
	d := newDoc(t)
	paste(t, d, 1, 0, 0, [][]string{
		{"Produit", "Prix"},
		{"Café", "4"},
		{"Thé", "2"},
		{"Chocolat", "3"},
	})
	must(t, d.RenameTable(ctx, tableAt(t, d, 0, 0), "Ventes"))
	_, err := d.SetCell(ctx, 1, 1, 5, `=FILTRE(Ventes;Ventes[Prix]>2)`)
	must(t, err)
	name, err := d.ConvertSpill(ctx, 1, 1, 5, true)
	must(t, err)
	return d, name
}

// recompute rejoue les requêtes des tableaux recalculés sur chaque poste
// (ce que fait le pont à l'ouverture, bridge.recomputeLocal).
func recompute(t *testing.T, d *doc.Document) {
	t.Helper()
	ctx := context.Background()
	ids, err := workbook.LocalQueries(ctx, d.DB().Read())
	must(t, err)
	if len(ids) == 0 {
		t.Fatal("aucune requête à recalculer")
	}
	for _, id := range ids {
		_, err := query.Refresh(ctx, query.Env{Model: d.Model}, id)
		must(t, err)
	}
}

func TestPublishDerivedRecomputedLocally(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	a, derived := derivedAuthor(t)

	check, err := workbook.Inspect(ctx, a.DB())
	must(t, err)
	var sawDerived, sawVentes bool
	for _, tb := range check.Tables {
		switch tb.Name {
		case derived:
			sawDerived = tb.Derived && tb.Query != ""
		case "Ventes":
			sawVentes = !tb.Derived
		}
	}
	if !sawDerived || !sawVentes {
		t.Fatalf("vérification : %+v", check.Tables)
	}

	// Un tableau saisi ne se recalcule pas : refusé.
	_, err = workbook.Publish(ctx, fake.As("alice"), a.DB(), workbook.Request{Server: server, User: "alice", Title: "Nids", Mode: workbook.Collaborative,
		Tables: []int64{tableID(t, a, derived)}, Local: []int64{tableID(t, a, "Ventes")}})
	if err == nil {
		t.Fatal("tableau saisi publié comme recalculé")
	}

	info, err := workbook.Publish(ctx, fake.As("alice"), a.DB(), workbook.Request{Server: server, User: "alice", Title: "Nids", Mode: workbook.Collaborative,
		Tables: []int64{tableID(t, a, "Ventes")}, Local: []int64{tableID(t, a, derived)}})
	must(t, err)
	must(t, a.ShareChanged(ctx))
	a.Share.SetOnline(true, "")

	// Sur le serveur : Ventes seul ; la mise en page décrit le tableau
	// dérivé et sa requête.
	_, _, _, tables := fake.Book(info.WorkbookID)
	if !slices.Equal(tables, []string{"Ventes"}) {
		t.Fatalf("tableaux du serveur : %v", tables)
	}
	lay := string(fake.Meta(info.WorkbookID, "layout"))
	if !strings.Contains(lay, "_lattis_local_tables") || !strings.Contains(lay, "FILTRE") || !strings.Contains(lay, derived) {
		t.Fatalf("mise en page publiée : %s", lay)
	}
	if got := a.Share.TableShare(tableID(t, a, derived)); got != "local" {
		t.Fatalf("auteur : tableau dérivé %q", got)
	}
	if n, err := workbook.PrivateTables(ctx, a.DB().Read()); err != nil || n != 0 {
		t.Fatalf("tableaux privés de l'auteur : %d %v", n, err)
	}

	// Un éditeur ouvre le classeur : le tableau dérivé est recréé à sa
	// place, puis recalculé chez lui.
	must(t, fake.Grant(info.WorkbookID, "bob", "editor"))
	b := open(t, fake.As("bob"), info.WorkbookID, "bob")
	if got := b.Share.TableShare(tableID(t, b, derived)); got != "local" {
		t.Fatalf("lecteur : tableau dérivé %q", got)
	}
	if got := column(t, b, derived, "Produit"); len(got) != 0 {
		t.Fatalf("tableau dérivé avant recalcul : %v", got)
	}
	recompute(t, b)
	if got := column(t, b, derived, "Produit"); !slices.Equal(got, []string{"Café", "Chocolat"}) {
		t.Fatalf("tableau dérivé recalculé : %v", got)
	}
	if tableID(t, b, derived) != tableID(t, a, derived) {
		t.Fatal("le tableau dérivé a changé d'identifiant")
	}

	// Une saisie de l'éditeur dans Ventes le recalcule chez lui (sans droit
	// de mise en page), rien du tableau dérivé ne part au serveur.
	_, err = b.SetCell(ctx, 1, 2, 1, "5") // Thé : 2 → 5
	must(t, err)
	if got := column(t, b, derived, "Produit"); !slices.Equal(got, []string{"Café", "Thé", "Chocolat"}) {
		t.Fatalf("tableau dérivé après saisie : %v", got)
	}
	rs, err := b.DB().Read().Query(`SELECT ops FROM _lattis_outbox`)
	must(t, err)
	var sent []string
	for rs.Next() {
		var ops string
		must(t, rs.Scan(&ops))
		sent = append(sent, ops)
	}
	rs.Close()
	if len(sent) != 1 || strings.Contains(strings.Join(sent, " "), `"t":`+itoa(tableID(t, b, derived))+`,`) {
		t.Fatalf("file d'envoi : %v", sent)
	}

	// Rechargement : toujours là, toujours recalculé sur le poste.
	_, err = workbook.Load(ctx, fake.As("bob"), b.DB(), workbook.Source{Server: server, User: "bob", WorkbookID: info.WorkbookID}, false)
	must(t, err)
	must(t, b.ShareChanged(ctx))
	recompute(t, b)
	if got := b.Share.TableShare(tableID(t, b, derived)); got != "local" {
		t.Fatalf("après rechargement : %q", got)
	}
	var q int
	must(t, b.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_queries WHERE target = ?`, tableID(t, b, derived)).Scan(&q))
	if q != 1 {
		t.Fatalf("%d requêtes pour le tableau dérivé", q)
	}
}

func TestRepublishKeepsDerivedLocal(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	a, derived := derivedAuthor(t)
	info, err := workbook.Publish(ctx, fake.As("alice"), a.DB(), workbook.Request{Server: server, User: "alice", Title: "Nids", Mode: workbook.Frozen,
		Tables: []int64{tableID(t, a, "Ventes")}, Local: []int64{tableID(t, a, derived)}})
	must(t, err)
	must(t, a.ShareChanged(ctx))
	a.Share.SetOnline(true, "")
	// Republication sans dire : le tableau reste recalculé sur chaque poste.
	_, err = workbook.Republish(ctx, fake.As("alice"), a.DB(), nil, nil)
	must(t, err)
	_, _, v, tables := fake.Book(info.WorkbookID)
	if v != 2 || !slices.Equal(tables, []string{"Ventes"}) {
		t.Fatalf("v%d %v", v, tables)
	}
	// Puis stocké sur le serveur.
	_, err = workbook.Republish(ctx, fake.As("alice"), a.DB(), []int64{tableID(t, a, "Ventes"), tableID(t, a, derived)}, []int64{})
	must(t, err)
	must(t, a.ShareChanged(ctx))
	_, _, _, tables = fake.Book(info.WorkbookID)
	if len(tables) != 2 || a.Share.TableShare(tableID(t, a, derived)) != "published" {
		t.Fatalf("tableaux : %v, %q", tables, a.Share.TableShare(tableID(t, a, derived)))
	}
}
