package workbook_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// sqlDerivedAuthor : un tableau Ventes et un tableau chargé par une requête
// SQL sur le classeur qui le filtre (Référencer depuis l'onglet SQL).
func sqlDerivedAuthor(t *testing.T) (*doc.Document, int64, int64) {
	t.Helper()
	ctx := context.Background()
	d := newDoc(t)
	paste(t, d, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4"}, {"Thé", "2"}, {"Chocolat", "3"}})
	must(t, d.RenameTable(ctx, tableAt(t, d, 0, 0), "Ventes"))
	src := tableID(t, d, "Ventes")
	def, err := query.Encode(query.Query{Name: "Chers", Source: query.Source{Kind: query.SourceSQL, SQL: `SELECT c_1 AS Produit, c_2 AS Prix FROM t_` + itoa(src) + ` WHERE c_2 > 2`}})
	must(t, err)
	id, err := d.SaveQuery(ctx, def)
	must(t, err)
	l, err := query.Refresh(ctx, query.Env{Model: d.Model}, id)
	must(t, err)
	return d, src, tableID(t, d, l.Table)
}

func sqlDerivedQuery(t *testing.T, d *doc.Document, tbl int64) string {
	t.Helper()
	var src string
	must(t, d.DB().Read().QueryRow(`SELECT coalesce((SELECT source FROM _lattis_queries WHERE target = ?), '')`, tbl).Scan(&src))
	return src
}

// Contre le vrai serveur : un tableau calculé par une requête SQL sur le
// classeur, publié avant avec ses lignes (figé chez le lecteur), devient
// dynamique à la republication : sa requête arrive chez le lecteur, qui le
// recalcule, sans tableau du serveur.
func TestPostgresSQLDerivedBecomesDynamic(t *testing.T) {
	env := pgSetup(t)
	useRoot(t)
	ctx := context.Background()
	alice, _ := env.user(t, "alice")
	bob, bobID := env.user(t, "bob")

	a, src, derived := sqlDerivedAuthor(t)
	check, err := workbook.Inspect(ctx, a.DB())
	must(t, err)
	if i := slices.IndexFunc(check.Tables, func(tb workbook.TableCheck) bool { return tb.ID == derived }); i < 0 || !check.Tables[i].Derived {
		t.Fatalf("requête SQL sur le classeur non dérivée : %+v", check.Tables)
	}

	// Stocké sur le serveur (l'ancien choix par défaut) : figé chez le lecteur.
	info, err := workbook.Publish(ctx, env.as(alice), a.DB(), workbook.Request{Server: server, User: alice, Title: "Chers " + t.Name(), Mode: workbook.Frozen, Tables: []int64{src, derived}})
	must(t, err)
	must(t, a.ShareChanged(ctx))
	a.Share.SetOnline(true, "")
	env.addMember(t, info.WorkbookID, "reader", bobID)
	r := open(t, env.as(bob), info.WorkbookID, bob)
	if q := sqlDerivedQuery(t, r, derived); q != "" {
		t.Fatalf("stocké : requête %s", q)
	}

	// Republié recalculé sur chaque poste.
	_, err = workbook.Republish(ctx, env.as(alice), a.DB(), []int64{src}, []int64{derived})
	must(t, err)
	must(t, a.ShareChanged(ctx))
	_, err = workbook.Load(ctx, env.as(bob), r.DB(), workbook.Source{Server: server, User: bob, WorkbookID: info.WorkbookID}, false)
	must(t, err)
	must(t, r.ShareChanged(ctx))
	if q := sqlDerivedQuery(t, r, derived); !strings.Contains(q, `"sql"`) {
		t.Fatalf("lecteur : requête %q", q)
	}
	if got := r.Share.TableShare(derived); got != "local" {
		t.Fatalf("lecteur : tableau %q", got)
	}
	recompute(t, r)
	if got := column(t, r, nameOf(t, r, derived), "Produit"); !slices.Equal(got, []string{"Café", "Chocolat"}) {
		t.Fatalf("lecteur, recalculé : %v", got)
	}

	// Un poste neuf aussi.
	n := open(t, env.as(bob), info.WorkbookID, bob)
	recompute(t, n)
	if got := column(t, n, nameOf(t, n, derived), "Produit"); !slices.Equal(got, []string{"Café", "Chocolat"}) {
		t.Fatalf("poste neuf : %v", got)
	}
}
