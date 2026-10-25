package collab_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Commentaires en fil d'un classeur partagé (canvas/comments.go) : ils
// suivent la mise en page en direct ; seuls propriétaires et concepteurs
// commentent (comme ils sont seuls à la changer).

func threads(t *testing.T, c *client) []canvas.Thread {
	t.Helper()
	th, err := c.d.Model.Threads(context.Background(), 0)
	must(t, err)
	return th
}

func TestCommentsLive(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	a := canvas.Who{Name: "Alice", ID: "alice"}
	b := canvas.Who{Name: "Bob", ID: "bob"}
	// Un fil sur une cellule libre, un autre sur la ligne « Thé » de Ventes.
	free, err := alice.d.AddComment(ctx, 1, 8, 1, "Total à revoir @Bob", a)
	must(t, err)
	row, err := alice.d.AddComment(ctx, 1, 2, 1, "Quantité juste ?", a)
	must(t, err)
	alice.syncLayout(false)
	bob.syncLayout(true)
	got := threads(t, bob)
	if len(got) != 2 || got[0].ID != row || got[0].Row != 2 || got[0].Col != 1 || got[0].Table != "Ventes" || got[1].ID != free || got[1].Comments[0].Text != "Total à revoir @Bob" {
		t.Fatalf("fils reçus par Bob : %+v", got)
	}
	// Bob, éditeur : refusé en local (le serveur lui refuse la mise en page).
	if _, err := bob.d.AddComment(ctx, 1, 8, 1, "Je regarde", b); !errors.Is(err, canvas.ErrCommentsShared) {
		t.Fatalf("éditeur : %v", err)
	}
	// Concepteur : il répond et résout ; Alice le reçoit.
	designer(t, f, id, bob)
	_, err = bob.d.AddComment(ctx, 1, 8, 1, "Corrigé", b)
	must(t, err)
	must(t, bob.d.ResolveThread(ctx, free, true))
	bob.syncLayout(false)
	alice.syncLayout(true)
	for _, x := range threads(t, alice) {
		if x.ID == free && (!x.Resolved || len(x.Comments) != 2 || x.Comments[1].Author != "Bob") {
			t.Fatalf("réponse de Bob chez Alice : %+v", x)
		}
	}
	// Alice trie Ventes (le tri reste à chacun) : le fil suit la ligne
	// « Thé », en tête chez elle, à sa place chez Bob.
	must(t, alice.d.SortTable(ctx, "Ventes", 1, true))
	alice.syncLayout(false)
	bob.syncLayout(true)
	for c, want := range map[*client]int{alice: 1, bob: 2} {
		for _, x := range threads(t, c) {
			if x.ID == row && x.Row != want {
				t.Fatalf("%s : fil de « Thé » ligne %d, attendu %d", c.name, x.Row, want)
			}
		}
	}
	// Alice supprime le fil de la cellule libre : parti chez Bob.
	must(t, alice.d.DeleteThread(ctx, 1, 8, 1))
	alice.syncLayout(false)
	bob.syncLayout(true)
	if got := threads(t, bob); len(got) != 1 || got[0].ID != row {
		t.Fatalf("fil supprimé, chez Bob : %+v", got)
	}
}
