package collab_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// derivedCol lit la colonne Produit du tableau name (ordre d'affichage).
func derivedCol(t *testing.T, d *doc.Document, name string) []string {
	t.Helper()
	var tbl, cid int64
	must(t, d.DB().Read().QueryRow(`SELECT t.id, k.id FROM _lattis_tables t JOIN _lattis_columns k ON k.tbl = t.id WHERE t.name = ? AND k.name = 'Produit'`, name).Scan(&tbl, &cid))
	rs, err := d.DB().Read().Query(`SELECT coalesce(CAST(c_` + strconv.FormatInt(cid, 10) + ` AS TEXT), '') FROM t_` + strconv.FormatInt(tbl, 10) + ` ORDER BY _pos, id`)
	must(t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var s string
		must(t, rs.Scan(&s))
		out = append(out, s)
	}
	return out
}

// Un tableau lié à une formule, publié « recalculé sur chaque poste » :
// les changements reçus d'un collègue le recalculent sur le poste, et la
// mise en page en direct d'un concepteur ne le retire pas du serveur.
func TestDerivedRecomputedOnReceive(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	f := workbooktest.New()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	_, err = d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4"}, {"Thé", "2"}, {"Chocolat", "3"}})
	must(t, err)
	var first string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = 1 AND row = 0 AND col = 0`).Scan(&first))
	must(t, d.RenameTable(ctx, first, "Ventes"))
	_, err = d.SetCell(ctx, 1, 1, 5, `=FILTRE(Ventes;Ventes[Prix]>2)`)
	must(t, err)
	derived, err := d.ConvertSpill(ctx, 1, 1, 5, true)
	must(t, err)
	id := func(d *doc.Document, name string) int64 {
		var n int64
		must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = ?`, name).Scan(&n))
		return n
	}
	info, err := workbook.Publish(ctx, f.As("alice"), d.DB(), workbook.Request{Server: server, User: "alice", Title: "Ventes 2026", Mode: workbook.Collaborative,
		Tables: []int64{id(d, "Ventes")}, Local: []int64{id(d, derived)}})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	alice := newClient(t, "alice", d, f.As("alice"))
	must(t, f.Grant(info.WorkbookID, "bob", "editor"))
	bob := newClient(t, "bob", openAs(t, f, info.WorkbookID, "bob"), f.As("bob"))
	ids, err := workbook.LocalQueries(ctx, bob.d.DB().Read())
	must(t, err)
	for _, q := range ids {
		_, err := query.Refresh(ctx, query.Env{Model: bob.d.Model}, q)
		must(t, err)
	}
	eq(t, "bob, à l'ouverture", derivedCol(t, bob.d, derived), []string{"Café", "Chocolat"})

	// Alice change un prix : Bob le reçoit, son tableau dérivé suit.
	alice.mustSet(1, 1, "9") // Thé : 2 → 9
	alice.send()
	bob.receive()
	eq(t, "bob, après réception", derivedCol(t, bob.d, derived), []string{"Café", "Thé", "Chocolat"})

	// Mise en page en direct de la conceptrice : le tableau dérivé reste
	// dans la mise en page du serveur.
	_, err = d.SetCell(ctx, 1, 10, 0, "Note")
	must(t, err)
	_, err = alice.e.SyncLayout(ctx, true)
	must(t, err)
	lay := string(f.Meta(info.WorkbookID, "layout"))
	if !strings.Contains(lay, "Note") || !strings.Contains(lay, `"`+derived+`"`) || !strings.Contains(lay, "_lattis_local_queries") {
		t.Fatalf("mise en page après envoi : %s", lay)
	}
	_, err = bob.e.SyncLayout(ctx, true)
	must(t, err)
	if bob.d.Share.TableShare(id(bob.d, derived)) != "local" {
		t.Fatal("bob : tableau dérivé perdu après la mise en page en direct")
	}
	eq(t, "bob, après mise en page", derivedCol(t, bob.d, derived), []string{"Café", "Thé", "Chocolat"})
}
