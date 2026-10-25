package collab_test

import (
	"context"
	"fmt"
	"testing"
)

// outlineOf résume le plan des lignes de la feuille 1 (ligne:niveau,
// * : réduit) et les lignes masquées.
func outlineOf(t *testing.T, c *client) string {
	t.Helper()
	l, err := c.d.Model.Layout(context.Background(), 1)
	must(t, err)
	if l.Outline == nil {
		return "-"
	}
	s := ""
	for _, x := range l.Outline.Rows {
		s += fmt.Sprintf("%d:%d", x.Index, x.Level)
		if x.Folded != 0 {
			s += "*"
		}
		s += " "
	}
	return s + fmt.Sprint(l.Outline.HiddenRows)
}

// Plan d'un classeur partagé (canvas/outline.go) : les groupes suivent la
// mise en page en direct ; réduire ou rouvrir reste sur chaque poste,
// permis à un éditeur, sans rien envoyer.
func TestLayoutOutline(t *testing.T) {
	ctx := context.Background()
	_, alice, bob, _ := setup(t)
	must(t, alice.d.GroupLines(ctx, 1, true, 20, 22))
	must(t, alice.d.FoldGroup(ctx, 1, true, 1, 20, true))
	alice.syncLayout(false)
	bob.syncLayout(true)
	if got := outlineOf(t, bob); got != "20:1 21:1 22:1* [20 21 22]" {
		t.Fatalf("Bob reçoit : %s", got)
	}
	// Bob (éditeur) rouvre le groupe chez lui ; il ne groupe pas.
	must(t, bob.d.FoldGroup(ctx, 1, true, 1, 21, false))
	if err := bob.d.GroupLines(ctx, 1, true, 30, 31); err == nil {
		t.Fatal("un éditeur a groupé des lignes")
	}
	bob.syncLayout(false)
	alice.syncLayout(true)
	if got := outlineOf(t, alice); got != "20:1 21:1 22:1* [20 21 22]" {
		t.Fatalf("Alice après Bob : %s", got)
	}
	// Alice imbrique un groupe : Bob le reçoit et garde son état.
	must(t, alice.d.GroupLines(ctx, 1, true, 21, 21))
	alice.syncLayout(false)
	bob.syncLayout(true)
	if got := outlineOf(t, bob); got != "20:1 21:2 22:1 []" {
		t.Fatalf("Bob après le groupe d'Alice : %s", got)
	}
	if got := outlineOf(t, alice); got != "20:1 21:2 22:1* [20 21 22]" {
		t.Fatalf("Alice : %s", got)
	}
}
