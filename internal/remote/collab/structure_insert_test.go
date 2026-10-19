package collab_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Suite de la structure partagée (migration 0026 du serveur) : colonne
// insérée à sa place (insert_column, add_column sur un serveur plus
// ancien), clé reçue par « *structure:<id> ».

// TestStructureInsertPosition : une colonne insérée en premier, au milieu
// ou à la fin arrive à la même place chez un collègue ; une valeur saisie
// ensuite va dans la bonne colonne.
func TestStructureInsertPosition(t *testing.T) {
	for _, c := range []struct {
		name string
		at   int
		want []string
	}{
		{"premier", 0, []string{"Remise:text", "Produit:text", "Quantité:integer", "Prix:decimal"}},
		{"milieu", 2, []string{"Produit:text", "Quantité:integer", "Remise:text", "Prix:decimal"}},
		{"fin", 3, []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Remise:text"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			f, alice, bob, id := setup(t)
			must(t, alice.d.InsertColumn(ctx, "Ventes", c.at, "Remise"))
			eq(t, "Alice", colsOf(alice, "Ventes"), c.want)
			cols, _ := serverCols(f, id, "Ventes")
			eq(t, "serveur", cols, c.want)
			if f.Calls["insert_column"] != 1 || f.Calls["add_column"] != 0 {
				t.Fatalf("appels : insert_column %d, add_column %d", f.Calls["insert_column"], f.Calls["add_column"])
			}
			tbl := map[int64]bool{tableID(t, bob.d): true}
			_, err := bob.e.ReceiveStructure(ctx, tbl, tbl)
			must(t, err)
			eq(t, "Bob", colsOf(bob, "Ventes"), c.want)

			alice.mustSet(0, c.at, "5 %")
			if r := alice.send(); r.Sent != 1 || r.Rejected != 0 {
				t.Fatalf("envoi : %+v", r)
			}
			bob.receive()
			eq(t, "Bob Remise", bob.col("Remise"), []string{"5 %", "", ""})
			eq(t, "Bob Produit", bob.col("Produit"), []string{"Café", "Thé", "Chocolat"})
		})
	}
}

// TestStructureInsertBefore26 : serveur sans insert_column (42883) : la
// colonne passe par add_column, après la colonne publiée qui la précède,
// à la fin sans elle ; la correspondance suit l'ordre du serveur.
func TestStructureInsertBefore26(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	f.Before26 = true
	must(t, alice.d.InsertColumn(ctx, "Ventes", 0, "Remise"))
	eq(t, "Alice", colsOf(alice, "Ventes"), []string{"Remise:text", "Produit:text", "Quantité:integer", "Prix:decimal"})
	cols, _ := serverCols(f, id, "Ventes")
	eq(t, "serveur", cols, []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Remise:text"})
	must(t, alice.d.InsertColumn(ctx, "Ventes", 2, "Note"))
	cols, _ = serverCols(f, id, "Ventes")
	eq(t, "serveur (milieu)", cols, []string{"Produit:text", "Note:text", "Quantité:integer", "Prix:decimal", "Remise:text"})
	if f.Calls["insert_column"] != 0 || f.Calls["add_column"] != 2 {
		t.Fatalf("appels : insert_column %d, add_column %d", f.Calls["insert_column"], f.Calls["add_column"])
	}
	tbl := map[int64]bool{tableID(t, bob.d): true}
	_, err := bob.e.ReceiveStructure(ctx, tbl, tbl)
	must(t, err)
	eq(t, "Bob", colsOf(bob, "Ventes"), []string{"Produit:text", "Note:text", "Quantité:integer", "Prix:decimal", "Remise:text"})
	// Alice n'a rien à reprendre : sa correspondance est celle du serveur.
	r, err := alice.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if len(r.Notices) != 0 {
		t.Fatalf("Alice : %v", r.Notices)
	}
	alice.mustSet(0, 0, "5 %")
	alice.mustSet(0, 2, "bien")
	if r := alice.send(); r.Rejected != 0 {
		t.Fatalf("envoi : %+v", r)
	}
	bob.receive()
	eq(t, "Bob Remise", bob.col("Remise"), []string{"5 %", "", ""})
	eq(t, "Bob Note", bob.col("Note"), []string{"bien", "", ""})
}

// TestStructureKeyByStructureNotice : la clé posée ou retirée par un
// collègue arrive par « *structure:<id> » seule (set_key, migration 0026),
// « *meta » écartée.
func TestStructureKeyByStructureNotice(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 20 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f, alice, bob, _ := setup(t)
	var mu sync.Mutex
	var seen []string
	f.Quiet(func(_, payload string) bool {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, payload)
		return payload == workbook.NoticeMeta
	})
	go bob.e.Follow(ctx, f.As("bob"), time.Hour, nil)
	go alice.e.Follow(ctx, f.As("alice"), time.Hour, nil)
	waitOnline(t, bob, alice)

	must(t, alice.d.SetKey(ctx, "Ventes", 0))
	waitFor(t, "clé chez Bob", func() bool { return keyOf(bob, "Ventes") == "Produit" })
	must(t, alice.d.ClearKey(ctx, "Ventes"))
	waitFor(t, "clé retirée chez Bob", func() bool { return keyOf(bob, "Ventes") == "" })
	tid, _ := remoteID(t, alice, "Produit")
	mu.Lock()
	defer mu.Unlock()
	if !slices.Contains(seen, workbook.NoticeStructure+tid) {
		t.Fatalf("notifications : %v", seen)
	}
}
